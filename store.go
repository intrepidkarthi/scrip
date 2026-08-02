package scrip

import (
	"context"
	"errors"
	"time"
)

// Store is the persistence contract.
//
// The specification requires the invariants be enforced at the storage layer, not in
// application code that a second service can bypass. This interface is how that
// requirement is expressed: an implementation is expected to enforce every rule in
// [Conformance] itself — with constraints and triggers where the backend has them —
// and this package checks the same rules again before calling it.
//
// The duplication is the design. Application checks give callers a diagnosable error;
// storage checks give the system a guarantee. Neither substitutes for the other.
type Store interface {
	// Instrument returns the instrument, or ErrNotFound.
	Instrument(ctx context.Context, id string) (Instrument, error)

	// SaveAuthorization persists a new authorisation.
	SaveAuthorization(ctx context.Context, a *Authorization) error

	// Authorization loads one by ID.
	Authorization(ctx context.Context, id string) (*Authorization, error)

	// CounterSign records the register-keeper's signature.
	//
	// MUST refuse if already counter-signed, and MUST do so atomically — two concurrent
	// counter-signatures must not both succeed, or the recorded signatory becomes
	// whichever write landed last.
	CounterSign(ctx context.Context, authorizationID string, by Party, at time.Time) error

	// DrawAndMint consumes authority and records the mint, atomically.
	//
	// This is the single most important method in the interface. The drawdown and the
	// mint MUST commit together: an implementation that reads the authorisation,
	// checks it in Go, and then writes has a race in which two callers both observe
	// enough remaining authority and both mint. Under Postgres this is one UPDATE with
	// the bound in its WHERE clause, or a trigger that recomputes it.
	//
	// MUST return ErrExceedsAuthorization if the mint would take total issuance past
	// the authorised quantity, ErrNotCounterSigned if it has not been counter-signed,
	// and ErrReserveShortfall where the profile bounds supply by attested reserves.
	DrawAndMint(ctx context.Context, m Mint) error

	// TotalIssued returns units currently outstanding for an instrument, which for a
	// redeemable profile is mints minus burns.
	TotalIssued(ctx context.Context, instrumentID string) (uint64, error)

	// SaveSettlement persists a new or updated settlement.
	//
	// MUST enforce exactly-once on the terminal transitions: a settlement that is
	// already Settled must not become Unwound, and neither release nor unwind may be
	// applied twice. Under Postgres a partial unique index on the settlement reference
	// does this without a flag anyone can forget to check.
	SaveSettlement(ctx context.Context, s *Settlement) error

	// Settlement loads one by ID.
	Settlement(ctx context.Context, id string) (*Settlement, error)

	// SaveAnchor records that a commitment was published.
	//
	// MUST refuse a second anchor for the same authorisation: two commitments for one
	// authorisation leaves no rule for which is authoritative, and a verifier checking
	// the wrong one gets a mismatch that looks like tampering.
	SaveAnchor(ctx context.Context, a Anchor, salt [SaltLen]byte) error

	// Anchor returns the published commitment for an authorisation, or ErrNotAnchored.
	Anchor(ctx context.Context, authorizationID string) (Anchor, error)
}

// AnchorAuthorization commits to an authorisation and publishes it, in that order.
//
// Called after counter-signature and before any mint. The ordering is the whole point
// of §5: a commitment published after minting proves nothing, because by then the venue
// has already had the opportunity to decide what it wished it had authorised.
//
// The salt is handed to the Store alongside the anchor. It MUST be retained — the
// digest stands forever, and without the salt nobody can ever demonstrate what it
// commits to.
func AnchorAuthorization(
	ctx context.Context,
	s Store,
	an Anchorer,
	authorizationID string,
	now time.Time,
) (Anchor, error) {
	auth, err := s.Authorization(ctx, authorizationID)
	if err != nil {
		return Anchor{}, err
	}

	if _, err := s.Anchor(ctx, authorizationID); err == nil {
		return Anchor{}, ErrAlreadyAnchored
	} else if !errors.Is(err, ErrNotAnchored) && !errors.Is(err, ErrNotFound) {
		return Anchor{}, err
	}

	c, err := Commit(auth)
	if err != nil {
		return Anchor{}, err
	}

	// Publish before recording. If publication succeeds and the write below fails, the
	// commitment exists on-chain with no local record — recoverable by reading the
	// chain. The reverse would be a local record pointing at a commitment that was
	// never published, which reads as anchored and is not.
	ref, err := an.Publish(ctx, c.Digest)
	if err != nil {
		return Anchor{}, err
	}

	anchor := Anchor{
		AuthorizationID: authorizationID,
		Digest:          c.Digest,
		Reference:       ref,
		At:              now,
	}
	if err := s.SaveAnchor(ctx, anchor, c.Salt); err != nil {
		return Anchor{}, err
	}
	return anchor, nil
}

// Mint is a record of units created against an authorisation.
type Mint struct {
	ID              string
	AuthorizationID string
	InstrumentID    string
	Quantity        uint64
	// HolderID is who receives the units.
	HolderID string
	// TransferRef ties the mint to its register transaction, once known.
	TransferRef string
	At          time.Time
}

// Issue is the full issuance path: load, validate every property, then hand a
// validated mint to the Store to commit atomically.
//
// This is the function an implementer should call rather than assembling the steps
// themselves, because the order matters and the ordering is not obvious. Attestation
// freshness is checked at mint time, not only at authorisation time — an authorisation
// raised in January against a valuation from the previous year is stale by December,
// and the units minted in December are the ones a buyer relies on.
func Issue(
	ctx context.Context,
	s Store,
	authorizationID string,
	mintID string,
	holderID string,
	quantity uint64,
	now time.Time,
) (*Mint, error) {
	if quantity == 0 {
		return nil, ErrZeroQuantity
	}

	auth, err := s.Authorization(ctx, authorizationID)
	if err != nil {
		return nil, err
	}

	inst, err := s.Instrument(ctx, auth.InstrumentID)
	if err != nil {
		return nil, err
	}

	// Property 2 — nothing is minted against an unconfirmed authorisation.
	if !auth.IsCounterSigned() {
		return nil, ErrNotCounterSigned
	}

	// §5 — nothing is minted against an authorisation whose commitment has not been
	// published. Without this the anchoring requirement is advisory: a venue could
	// anchor the authorisations it expects to be asked about and mint freely against
	// the rest, and an authority chain that is verifiable for some units is not
	// verifiable.
	if _, err := s.Anchor(ctx, authorizationID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotAnchored
		}
		return nil, err
	}

	// Property 3 — checked here for a diagnosable error, and again by the Store for the
	// guarantee. Draw mutates a local copy only; the Store owns the durable decrement.
	if quantity > auth.Remaining() {
		return nil, ErrExceedsAuthorization
	}

	// Profile — evidence must be current NOW, not when the authorisation was raised.
	if err := inst.Profile.checkAttestations(auth.Attestations, now); err != nil {
		return nil, err
	}

	// Profile — reserve ceiling, for gold and anything else supply-bounded. Checked
	// against units already outstanding, so it holds across authorisations rather than
	// within one.
	if ceiling, bounded := inst.Profile.reserveCeiling(auth.Attestations); bounded {
		outstanding, err := s.TotalIssued(ctx, inst.ID)
		if err != nil {
			return nil, err
		}
		if outstanding+quantity > ceiling {
			return nil, ErrReserveShortfall
		}
	}

	m := Mint{
		ID:              mintID,
		AuthorizationID: authorizationID,
		InstrumentID:    inst.ID,
		Quantity:        quantity,
		HolderID:        holderID,
		At:              now,
	}

	if err := s.DrawAndMint(ctx, m); err != nil {
		return nil, err
	}
	return &m, nil
}
