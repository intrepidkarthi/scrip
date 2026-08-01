package scrip

import "time"

// ActType is the kind of legal instrument by which an issuer created units.
type ActType string

const (
	BoardResolution       ActType = "board_resolution"
	OfferingRound         ActType = "offering_round"
	SubscriptionAgreement ActType = "subscription_agreement"
	SPVFormation          ActType = "spv_formation"
	DeedOfTransfer        ActType = "deed_of_transfer"
	VaultDepositReceipt   ActType = "vault_deposit_receipt"
)

// AttestationKind is evidence that the underlying asset exists and is as described.
type AttestationKind string

const (
	AuditedFinancials AttestationKind = "audited_financials"
	TitleDeed         AttestationKind = "title_deed"
	ValuationSurvey   AttestationKind = "valuation_survey"
	AssayCertificate  AttestationKind = "assay_certificate"
	VaultAttestation  AttestationKind = "vault_attestation"
)

// RedemptionModel is whether a holder can get the underlying back.
type RedemptionModel string

const (
	// NoRedemption — exit is by transfer in a secondary market. Equity and property.
	NoRedemption RedemptionModel = "none"
	// PhysicalDelivery — the holder may take the asset. Gold. Implies supply burns.
	PhysicalDelivery RedemptionModel = "physical_delivery"
)

// Attestation is one piece of evidence, with the date it speaks to.
type Attestation struct {
	Kind AttestationKind
	// Reference identifies the document: a certificate number, a deed reference, a
	// vault report identifier. Not a filename.
	Reference string
	// AsOf is the date the evidence describes, which is not the date it was filed. A
	// valuation surveyed in January and uploaded in June is six months old.
	AsOf time.Time
	// Quantity, where the attestation bounds supply — attested vault holdings for gold.
	// Zero means the attestation does not bound supply.
	Quantity uint64
}

// Profile carries what differs between asset classes. The five properties are
// asset-class agnostic; everything that is not is here.
//
// A profile built for one asset class and reused for another fails quietly: an
// equity-shaped implementation has nowhere to record a property's jurisdiction, and no
// concept of a reserve that must cover outstanding units. Those are the two omissions
// this type exists to make impossible.
type Profile struct {
	Name string

	// ValidActs — which corporate acts may create units under this profile. A vault
	// deposit receipt cannot authorise company equity.
	ValidActs []ActType

	// RequiredAttestations — evidence that must be present at issuance.
	RequiredAttestations []AttestationKind

	// MaxAttestationAge — how stale evidence may be at the moment of issuance.
	//
	// Must be positive. An earlier draft used zero to mean "continuous", which read
	// well and enforced nothing: with no window to compare against, any past-dated
	// attestation passed. "Continuous" is expressed by a short window here plus
	// ReserveBounded below — a feed that must be current, and a ceiling checked on
	// every mint rather than periodically.
	MaxAttestationAge time.Duration

	// ReserveBounded — total units outstanding may never exceed the attested quantity.
	// True for gold. This is checked on every mint rather than periodically, because a
	// periodic check can only tell you that units were unbacked, never prevent it.
	ReserveBounded bool

	Redemption RedemptionModel

	// JurisdictionFollowsAsset — the governing law is the asset's location, not the
	// issuer's incorporation. True for real estate: lex situs. A Dubai SPV holding a
	// London property is governed by English law for transfer purposes, and an
	// implementation that stores one jurisdiction per instrument cannot express that.
	JurisdictionFollowsAsset bool
}

// The three specified profiles. See SPEC.md §6.

// EquityProfile — operating company equity. Agricultural companies, SMEs.
func EquityProfile() Profile {
	return Profile{
		Name:                 "operating_company_equity",
		ValidActs:            []ActType{BoardResolution, OfferingRound, SubscriptionAgreement},
		RequiredAttestations: []AttestationKind{AuditedFinancials},
		MaxAttestationAge:    18 * 30 * 24 * time.Hour, // ~18 months: an annual audit, plus filing slack
		ReserveBounded:       false,
		Redemption:           NoRedemption,
	}
}

// RealEstateProfile — fractional interest in a property-holding SPV.
func RealEstateProfile() Profile {
	return Profile{
		Name:                 "real_estate",
		ValidActs:            []ActType{SPVFormation, DeedOfTransfer},
		RequiredAttestations: []AttestationKind{TitleDeed, ValuationSurvey},
		// A year. Longer than this and the price a buyer sees is not evidence of
		// anything — stale valuations are the principal mis-selling risk here.
		MaxAttestationAge:        365 * 24 * time.Hour,
		ReserveBounded:           false,
		Redemption:               NoRedemption,
		JurisdictionFollowsAsset: true,
	}
}

// GoldProfile — allocated bullion, by weight and fineness.
//
// One unit is a fixed weight at a stated fineness, declared by the instrument. Units
// are integers here deliberately: a token is a claim on a countable object, and
// representing it as a decimal invites the rounding drift that the ledger requirements
// in §4.1 exist to prevent.
func GoldProfile() Profile {
	return Profile{
		Name:                 "gold",
		ValidActs:            []ActType{VaultDepositReceipt},
		RequiredAttestations: []AttestationKind{AssayCertificate, VaultAttestation},
		// Fifteen minutes. This is what "continuous proof-of-reserve" means in
		// practice: a feed that republishes on a short cycle, not a quarterly audit.
		// The number is a policy choice and an implementer may tighten it; what is not
		// negotiable is that it is short and that ReserveBounded is set, because
		// together they mean every mint is covered by a reserve proven minutes ago.
		MaxAttestationAge: 15 * time.Minute,
		ReserveBounded:    true,
		Redemption:        PhysicalDelivery,
	}
}

func (p Profile) permitsAct(t ActType) bool {
	for _, a := range p.ValidActs {
		if a == t {
			return true
		}
	}
	return false
}

// checkAttestations verifies the profile's evidence requirements against what was
// supplied, as at `now`.
func (p Profile) checkAttestations(given []Attestation, now time.Time) error {
	for _, required := range p.RequiredAttestations {
		found := false
		for _, g := range given {
			if g.Kind != required {
				continue
			}
			found = true

			// Evidence dated in the future is rejected as stale rather than accepted as
			// very fresh — a clock or data-entry fault either way, and trusting it
			// would let a backdated document pass forever.
			if g.AsOf.After(now) || now.Sub(g.AsOf) > p.MaxAttestationAge {
				return ErrAttestationStale
			}
			break
		}
		if !found {
			return ErrAttestationRequired
		}
	}
	return nil
}

// reserveCeiling returns the largest supply the attestations support, and whether the
// profile bounds supply at all.
func (p Profile) reserveCeiling(given []Attestation) (uint64, bool) {
	if !p.ReserveBounded {
		return 0, false
	}
	var ceiling uint64
	for _, a := range given {
		if a.Quantity > ceiling {
			ceiling = a.Quantity
		}
	}
	return ceiling, true
}
