package scrip

import "time"

// SettlementState is where a trade sits between being struck and being finished.
type SettlementState string

const (
	// Pending — cash is held, the asset has not moved.
	Pending SettlementState = "pending"
	// Delivering — the transfer has been submitted to the register.
	Delivering SettlementState = "delivering"
	// Settled — delivery confirmed, cash released. Terminal.
	Settled SettlementState = "settled"
	// Unwound — delivery failed terminally, cash returned to the buyer. Terminal.
	Unwound SettlementState = "unwound"
	// Indeterminate — the transfer was submitted and its outcome is unknown. Terminal
	// until a human resolves it, and deliberately NOT retryable.
	//
	// This state exists because the alternative is worse in a way that is easy to miss.
	// A transfer that was broadcast but not confirmed may have succeeded. Retrying it
	// moves the asset twice; unwinding it returns cash for an asset that did move.
	// Nothing in the record distinguishes those, so the only correct action is to stop
	// and look at the register.
	Indeterminate SettlementState = "indeterminate"
)

// Settlement is one delivery-versus-payment obligation.
//
// Property 5. Cash is held from the moment the trade is struck and leaves only on
// confirmed delivery or on return to the buyer — never on optimism about a transfer
// that has been submitted.
type Settlement struct {
	ID           string
	InstrumentID string
	BuyerID      string
	SellerID     string
	Quantity     uint64
	// Cash is in the asset's minor unit — cents, fils. Integer, because a settlement
	// amount that cannot be paid should never be recorded as owed.
	Cash     uint64
	Currency string

	State SettlementState

	// TransferRef identifies the register transfer, once submitted. Present in
	// Indeterminate is the difference between "we know which transfer to go and check"
	// and an afternoon of work.
	TransferRef string
	Reason      string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewSettlement opens an obligation with the cash already held.
func NewSettlement(id, instrumentID, buyerID, sellerID string, quantity, cash uint64, currency string, now time.Time) (*Settlement, error) {
	if quantity == 0 || cash == 0 {
		return nil, ErrZeroQuantity
	}
	return &Settlement{
		ID: id, InstrumentID: instrumentID,
		BuyerID: buyerID, SellerID: sellerID,
		Quantity: quantity, Cash: cash, Currency: currency,
		State:     Pending,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Submit records that the register transfer has been sent.
//
// The transfer reference is stored BEFORE the outcome is known, which is the point: if
// the process dies here, the record says a transfer with this reference may exist. A
// design that stores the reference on success loses exactly the case where it matters.
func (s *Settlement) Submit(transferRef string, now time.Time) error {
	if s.State != Pending {
		return s.terminalOr(ErrNotPending)
	}
	s.State = Delivering
	s.TransferRef = transferRef
	s.UpdatedAt = now
	return nil
}

// Confirm records confirmed inclusion in the register and releases the cash.
//
// Property 4: settled only on confirmed inclusion. Not on submission, not on a
// hopeful timeout.
func (s *Settlement) Confirm(now time.Time) error {
	if s.State != Delivering {
		return s.terminalOr(ErrNotPending)
	}
	s.State = Settled
	s.UpdatedAt = now
	return nil
}

// Fail records that delivery failed terminally and returns the cash to the buyer.
//
// Only valid from Pending or Delivering where the failure is *known* — a rejected
// transaction, a compliance refusal. If the outcome is unknown, the caller must use
// [Settlement.MarkIndeterminate] instead, and the distinction is the caller's to make
// honestly because only they saw the response.
func (s *Settlement) Fail(reason string, now time.Time) error {
	if s.State != Pending && s.State != Delivering {
		return s.terminalOr(ErrNotPending)
	}
	s.State = Unwound
	s.Reason = reason
	s.UpdatedAt = now
	return nil
}

// MarkIndeterminate parks the settlement for human reconciliation.
//
// Reached when a transfer was submitted and the outcome cannot be determined — a
// timeout waiting for inclusion, a node that went away mid-call, a process killed
// between broadcast and record. Neither confirming nor unwinding is safe, so neither
// happens automatically.
func (s *Settlement) MarkIndeterminate(reason string, now time.Time) error {
	if s.State != Delivering {
		return s.terminalOr(ErrNotPending)
	}
	s.State = Indeterminate
	s.Reason = reason
	s.UpdatedAt = now
	return nil
}

// Resolve closes an indeterminate settlement after a person has checked the register.
//
// The only transition out of Indeterminate, and it takes an explicit outcome because
// the whole point is that the system could not work it out. `settled` true means the
// transfer was found in the register; false means it was not and the cash goes back.
func (s *Settlement) Resolve(settled bool, resolution string, now time.Time) error {
	if s.State != Indeterminate {
		return ErrNotPending
	}
	if settled {
		s.State = Settled
	} else {
		s.State = Unwound
	}
	s.Reason = resolution
	s.UpdatedAt = now
	return nil
}

// terminalOr returns the specific already-finished error when the settlement is in a
// terminal state, so a caller can tell "this finished" from "wrong state". Exactly-once
// release and unwind depend on callers being able to make that distinction.
func (s *Settlement) terminalOr(fallback error) error {
	switch s.State {
	case Settled:
		return ErrAlreadySettled
	case Unwound:
		return ErrAlreadyUnwound
	case Indeterminate:
		return ErrIndeterminate
	default:
		return fallback
	}
}

// IsTerminal reports whether no further automatic transition is possible.
func (s *Settlement) IsTerminal() bool {
	return s.State == Settled || s.State == Unwound || s.State == Indeterminate
}
