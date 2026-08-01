package scrip

import "time"

// PartyKind is which side of the authorisation a signatory acts for.
type PartyKind string

const (
	Issuer         PartyKind = "issuer"
	RegisterKeeper PartyKind = "register_keeper"
)

// Party is a signatory.
//
// EntityID is the load-bearing field, and the reason this is not just a user ID. The
// dual-signature requirement is that two *entities* sign, not two accounts — two
// logins at the same firm are one pair of eyes, and an implementation that compares
// user IDs will happily accept them.
type Party struct {
	ID       string
	Kind     PartyKind
	EntityID string
}

// CorporateAct is the legal instrument by which the issuer created the units.
//
// All three fields are required. An act missing any of them cannot be produced in an
// audit, which is the only thing it is for.
type CorporateAct struct {
	Type      ActType
	Reference string
	Date      time.Time
}

func (a CorporateAct) valid() error {
	if a.Type == "" || a.Reference == "" || a.Date.IsZero() {
		return ErrIncompleteCorporateAct
	}
	return nil
}

// Instrument is the thing being issued. The protocol needs very little of it: who
// issues it, under which profile, and — where the profile says so — where the asset is.
type Instrument struct {
	ID string
	// IssuerEntityID is the legal entity whose asset or equity this represents. An
	// authorisation is valid only if signed by a party acting for this entity.
	IssuerEntityID string
	Profile        Profile

	// AssetJurisdiction is where the underlying asset sits, when that differs from the
	// issuer's incorporation. Required when Profile.JurisdictionFollowsAsset — a Dubai
	// SPV holding a London property must record London, and an implementation with one
	// jurisdiction field cannot say that.
	AssetJurisdiction string
}

// Authorization is the record of authority to create units: which act, how many, who
// signed, who counter-signed, and how much of it has been drawn.
//
// The drawdown lives on the authorisation rather than being derived from a sum of
// mints, so that "have we issued past authority" is answerable from one row under one
// lock. Deriving it invites a read-then-write race, which is exactly how an
// authorisation for a million units gets used to mint a million units repeatedly.
type Authorization struct {
	ID           string
	InstrumentID string
	Act          CorporateAct
	Quantity     uint64

	AuthorizedBy    Party
	CounterSignedBy *Party

	// Drawn is the total minted against this authorisation.
	Drawn uint64

	Attestations    []Attestation
	CreatedAt       time.Time
	CounterSignedAt *time.Time
}

// Remaining is how many units may still be minted under this authorisation.
func (a *Authorization) Remaining() uint64 {
	if a.Drawn >= a.Quantity {
		return 0
	}
	return a.Quantity - a.Drawn
}

// IsCounterSigned reports whether the register-keeper has confirmed the authorisation.
func (a *Authorization) IsCounterSigned() bool { return a.CounterSignedBy != nil }

// NewAuthorization validates and constructs an authorisation. It is not counter-signed
// yet — that is a separate act by a separate entity, and collapsing the two into one
// call would make it possible to do both in one transaction from one session, which is
// the whole failure being guarded against.
//
// `now` is passed rather than read from the clock so that attestation staleness is
// testable and so a caller running a batch gets one consistent moment.
func NewAuthorization(
	id string,
	inst Instrument,
	act CorporateAct,
	quantity uint64,
	by Party,
	attestations []Attestation,
	now time.Time,
) (*Authorization, error) {
	if quantity == 0 {
		return nil, ErrZeroQuantity
	}

	// Property 1 — issuance traces to a corporate act.
	if err := act.valid(); err != nil {
		return nil, err
	}
	if !inst.Profile.permitsAct(act.Type) {
		return nil, ErrActNotValidForProfile
	}

	// Property 2 (first half) — the signatory must act for the issuer. Checked against
	// the instrument's issuing entity, not against a role name: a role name says what
	// someone is called, not who they are entitled to bind.
	if by.Kind != Issuer || by.EntityID == "" || by.EntityID != inst.IssuerEntityID {
		return nil, ErrNotIssuer
	}

	// Profile requirements — evidence the underlying exists.
	if err := inst.Profile.checkAttestations(attestations, now); err != nil {
		return nil, err
	}

	return &Authorization{
		ID:           id,
		InstrumentID: inst.ID,
		Act:          act,
		Quantity:     quantity,
		AuthorizedBy: by,
		Attestations: attestations,
		CreatedAt:    now,
	}, nil
}

// CounterSign records the register-keeper's confirmation.
//
// Property 2 (second half). The signature must come from a different legal entity than
// the issuer's — compared on EntityID, so two accounts at the same firm are refused.
func (a *Authorization) CounterSign(by Party, now time.Time) error {
	if a.IsCounterSigned() {
		return ErrAlreadyCounterSigned
	}
	if by.Kind != RegisterKeeper || by.EntityID == "" {
		return ErrNotRegisterKeeper
	}
	if by.EntityID == a.AuthorizedBy.EntityID {
		return ErrSameEntity
	}
	// A different entity but the same human, where one person holds accounts at both,
	// is not detectable from here. It is a governance problem, and the protocol says so
	// rather than pretending the check covers it.
	if by.ID == a.AuthorizedBy.ID {
		return ErrSameEntity
	}

	a.CounterSignedBy = &by
	a.CounterSignedAt = &now
	return nil
}

// Draw consumes `quantity` units of authority, returning the new drawn total.
//
// Property 3. This is pure arithmetic on purpose: it is the check, and it must be
// callable inside whatever transaction the Store uses so that the drawdown and the
// mint commit together. A Store that calls this and then writes in a separate
// transaction has implemented the check and not the guarantee.
func (a *Authorization) Draw(quantity uint64) error {
	if quantity == 0 {
		return ErrZeroQuantity
	}
	if !a.IsCounterSigned() {
		return ErrNotCounterSigned
	}
	// Written as a subtraction against Remaining rather than `Drawn+quantity >
	// Quantity` so that a caller passing a quantity near the integer limit cannot wrap
	// the addition and pass the check.
	if quantity > a.Remaining() {
		return ErrExceedsAuthorization
	}
	a.Drawn += quantity
	return nil
}
