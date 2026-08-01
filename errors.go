package scrip

import "errors"

// One error per invariant, so a caller can distinguish "you sent something malformed"
// from "this would have broken the protocol". The second kind is the reason this
// package exists, and a handler that cannot tell them apart will log both at the same
// level and notice neither.

var (
	// ── Property 1: issuance traces to a corporate act ──────────────────────────

	// ErrNoCorporateAct — the authorisation names no legal instrument.
	ErrNoCorporateAct = errors.New("scrip: issuance must reference a corporate act")
	// ErrIncompleteCorporateAct — an act without a type, reference or date is a note,
	// not an instrument. It cannot be produced in an audit.
	ErrIncompleteCorporateAct = errors.New("scrip: corporate act requires a type, a reference and a date")
	// ErrActNotValidForProfile — e.g. a vault deposit receipt authorising company equity.
	ErrActNotValidForProfile = errors.New("scrip: corporate act type is not valid for this asset profile")

	// ── Property 2: dual-party authorisation ────────────────────────────────────

	// ErrNotIssuer — the signatory does not act for the issuing entity.
	ErrNotIssuer = errors.New("scrip: authorisation must be signed by a party acting for the issuer")
	// ErrNotRegisterKeeper — the counter-signatory is not the register-keeper.
	ErrNotRegisterKeeper = errors.New("scrip: counter-signature must come from the register-keeper")
	// ErrSameEntity — both signatures come from the same legal entity. This is the
	// failure that turns four-eyes into theatre, and it is why parties carry an entity
	// identifier rather than only a user identifier: two accounts at the same firm are
	// one pair of eyes.
	ErrSameEntity = errors.New("scrip: the issuer and register-keeper signatures must come from different entities")
	// ErrNotCounterSigned — a mint was attempted against an authorisation nobody at the
	// register-keeper has confirmed.
	ErrNotCounterSigned = errors.New("scrip: authorisation has not been counter-signed")
	// ErrAlreadyCounterSigned — counter-signing twice would let a second signature
	// silently replace the recorded one.
	ErrAlreadyCounterSigned = errors.New("scrip: authorisation is already counter-signed")

	// ── Property 3: atomic drawdown ─────────────────────────────────────────────

	// ErrExceedsAuthorization — the mint would take total issuance past what was
	// authorised. The single most consequential refusal in the protocol.
	ErrExceedsAuthorization = errors.New("scrip: mint would exceed the authorised quantity")
	// ErrZeroQuantity — a zero-unit authorisation or mint. Almost always a bug in the
	// caller, and permitting it makes the drawdown arithmetic harder to reason about.
	ErrZeroQuantity = errors.New("scrip: quantity must be greater than zero")

	// ── Property 4: the register is authoritative ───────────────────────────────

	// ErrIndeterminate — a transfer was broadcast but its inclusion is unknown. NOT an
	// error to retry. Retrying a transfer that actually succeeded moves the asset
	// twice, and nothing in the database distinguishes the two cases.
	ErrIndeterminate = errors.New("scrip: transfer outcome is indeterminate and requires human reconciliation")

	// ── Property 5: delivery versus payment ─────────────────────────────────────

	// ErrAlreadySettled — release or unwind attempted on a settled trade.
	ErrAlreadySettled = errors.New("scrip: settlement is already complete")
	// ErrAlreadyUnwound — unwind attempted twice. Exactly-once matters here because the
	// second unwind would return the buyer's cash a second time.
	ErrAlreadyUnwound = errors.New("scrip: settlement is already unwound")
	// ErrNotPending — a state transition was attempted from the wrong state.
	ErrNotPending = errors.New("scrip: settlement is not in a state that permits this transition")

	// ── Attestation (asset profiles) ────────────────────────────────────────────

	// ErrAttestationRequired — the profile requires evidence the underlying exists and
	// none was supplied.
	ErrAttestationRequired = errors.New("scrip: asset profile requires an attestation")
	// ErrAttestationStale — evidence exists but is older than the profile permits. For
	// real estate this is the principal mis-selling risk; for gold it means the reserve
	// is unproven right now.
	ErrAttestationStale = errors.New("scrip: attestation is older than the profile permits")
	// ErrReserveShortfall — units outstanding would exceed attested holdings. Gold's
	// unrecoverable failure: no later audit restores metal that was never there.
	ErrReserveShortfall = errors.New("scrip: issuance would exceed attested reserves")

	// ── Structural ──────────────────────────────────────────────────────────────

	ErrNotFound       = errors.New("scrip: not found")
	ErrUnknownProfile = errors.New("scrip: unknown asset profile")
)
