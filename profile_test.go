package scrip

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The profiles for real estate and gold were written from those asset classes'
// requirements rather than from any existing implementation's schema. These tests pin
// the two requirements that an equity-shaped system has nowhere to put — and which are
// therefore the ones that get missed.

// ── Gold: continuous proof-of-reserve ─────────────────────────────────────────

func goldInstrument() Instrument {
	return Instrument{ID: "gold-1", IssuerEntityID: "entity-vault-op", Profile: GoldProfile()}
}

func vaultAttestations(grams uint64, asOf time.Time) []Attestation {
	return []Attestation{
		{Kind: AssayCertificate, Reference: "ASSAY-1", AsOf: asOf},
		{Kind: VaultAttestation, Reference: "VAULT-1", AsOf: asOf, Quantity: grams},
	}
}

func TestGoldRequiresBothAssayAndVaultAttestation(t *testing.T) {
	assayOnly := []Attestation{{Kind: AssayCertificate, Reference: "ASSAY-1", AsOf: now}}
	act := CorporateAct{Type: VaultDepositReceipt, Reference: "VD-1", Date: now}
	_, err := NewAuthorization("g1", goldInstrument(), act, 1000,
		Party{ID: "u-vault", Kind: Issuer, EntityID: "entity-vault-op"}, assayOnly, now)
	if !errors.Is(err, ErrAttestationRequired) {
		t.Fatalf("gold was authorised on an assay with no vault attestation — purity proven, existence not: %v", err)
	}
}

// Continuous means continuous. An attestation from yesterday does not prove the metal
// is in the vault now.
func TestGoldAttestationMustBeCurrent(t *testing.T) {
	act := CorporateAct{Type: VaultDepositReceipt, Reference: "VD-1", Date: now}
	stale := vaultAttestations(1000, now.Add(-time.Hour))
	_, err := NewAuthorization("g1", goldInstrument(), act, 100,
		Party{ID: "u-vault", Kind: Issuer, EntityID: "entity-vault-op"}, stale, now)
	if !errors.Is(err, ErrAttestationStale) {
		t.Fatalf("an hour-old vault attestation satisfied a continuous requirement: %v", err)
	}
}

// The unrecoverable failure: units in excess of metal. No later audit puts the metal
// back.
func TestGoldIssuanceCannotExceedAttestedReserves(t *testing.T) {
	st := newMemStore()
	inst := goldInstrument()
	st.instruments[inst.ID] = inst

	act := CorporateAct{Type: VaultDepositReceipt, Reference: "VD-1", Date: now}
	a, err := NewAuthorization("g1", inst, act, 10_000,
		Party{ID: "u-vault", Kind: Issuer, EntityID: "entity-vault-op"},
		vaultAttestations(1_000, now), now) // 1,000 grams in the vault
	if err != nil {
		t.Fatalf("valid gold authorisation refused: %v", err)
	}
	if err := a.CounterSign(keeperParty(), now); err != nil {
		t.Fatal(err)
	}
	st.auths[a.ID] = a

	ctx := context.Background()

	// §5 — nothing mints until the commitment is published.
	if _, err := AnchorAuthorization(ctx, st, &fakeAnchorer{}, a.ID, now); err != nil {
		t.Fatalf("anchoring refused: %v", err)
	}

	// Up to the attested reserve is fine.
	if _, err := Issue(ctx, st, a.ID, "m1", "holder", 1_000, now); err != nil {
		t.Fatalf("issuing exactly the attested reserve was refused: %v", err)
	}

	// One gram past it is not — even though the authorisation has 9,000 units left.
	if _, err := Issue(ctx, st, a.ID, "m2", "holder", 1, now); !errors.Is(err, ErrReserveShortfall) {
		t.Fatalf("minted gold beyond attested vault holdings: %v", err)
	}
}

// ── Real estate: lex situs ────────────────────────────────────────────────────

// The common case the protocol must be able to express: a Dubai SPV holding a London
// property. A schema with one jurisdiction per instrument cannot say this.
func TestRealEstateRecordsTheAssetJurisdictionSeparately(t *testing.T) {
	p := RealEstateProfile()
	if !p.JurisdictionFollowsAsset {
		t.Fatal("real estate must follow lex situs — the property's law governs, not the issuer's")
	}

	inst := Instrument{
		ID:                "prop-1",
		IssuerEntityID:    "entity-dubai-spv",
		Profile:           p,
		AssetJurisdiction: "GB-LND",
	}
	if inst.AssetJurisdiction == "" {
		t.Fatal("asset jurisdiction is unrecorded")
	}
	// The point of the test: these differ, and both are representable.
	if inst.AssetJurisdiction == inst.IssuerEntityID {
		t.Fatal("issuer and asset jurisdiction collapsed into one value")
	}
}

func TestRealEstateRequiresTitleAndValuation(t *testing.T) {
	inst := Instrument{ID: "prop-1", IssuerEntityID: "entity-spv", Profile: RealEstateProfile()}
	act := CorporateAct{Type: SPVFormation, Reference: "SPV-1", Date: now}
	titleOnly := []Attestation{{Kind: TitleDeed, Reference: "TD-1", AsOf: now}}

	_, err := NewAuthorization("p1", inst, act, 100,
		Party{ID: "u-spv", Kind: Issuer, EntityID: "entity-spv"}, titleOnly, now)
	if !errors.Is(err, ErrAttestationRequired) {
		t.Fatalf("property was authorised with a deed but no valuation — ownership proven, price not: %v", err)
	}
}

// Stale valuations are the principal mis-selling risk in tokenized real estate.
func TestRealEstateValuationGoesStale(t *testing.T) {
	inst := Instrument{ID: "prop-1", IssuerEntityID: "entity-spv", Profile: RealEstateProfile()}
	act := CorporateAct{Type: SPVFormation, Reference: "SPV-1", Date: now}
	old := []Attestation{
		{Kind: TitleDeed, Reference: "TD-1", AsOf: now.AddDate(-3, 0, 0)},
		{Kind: ValuationSurvey, Reference: "VS-1", AsOf: now.AddDate(-3, 0, 0)},
	}
	_, err := NewAuthorization("p1", inst, act, 100,
		Party{ID: "u-spv", Kind: Issuer, EntityID: "entity-spv"}, old, now)
	if !errors.Is(err, ErrAttestationStale) {
		t.Fatalf("a three-year-old valuation priced a new issuance: %v", err)
	}
}

// ── Redemption model ──────────────────────────────────────────────────────────

func TestOnlyGoldRedeems(t *testing.T) {
	if GoldProfile().Redemption != PhysicalDelivery {
		t.Fatal("gold must be redeemable for metal, or it is a derivative")
	}
	if EquityProfile().Redemption != NoRedemption {
		t.Fatal("equity does not redeem — exit is by transfer")
	}
	if RealEstateProfile().Redemption != NoRedemption {
		t.Fatal("a fractional property interest does not redeem")
	}
}
