package scrip

import (
	"errors"
	"testing"
)

func pending(t *testing.T) *Settlement {
	t.Helper()
	s, err := NewSettlement("s1", "inst-1", "buyer", "seller", 100, 250_00, "USD", now)
	if err != nil {
		t.Fatalf("valid settlement refused: %v", err)
	}
	return s
}

// ── Property 5: exactly-once release and unwind ───────────────────────────────

func TestCashIsNotReleasedOnSubmissionOnly(t *testing.T) {
	s := pending(t)
	if err := s.Submit("0xabc", now); err != nil {
		t.Fatalf("submit refused: %v", err)
	}
	if s.State == Settled {
		t.Fatal("settlement completed on submission — cash must not leave until delivery is confirmed")
	}
}

// The transfer reference must be recorded before the outcome is known, or the one case
// where it matters is the one case where it is missing.
func TestTransferReferenceIsRecordedBeforeOutcome(t *testing.T) {
	s := pending(t)
	if err := s.Submit("0xabc", now); err != nil {
		t.Fatalf("submit refused: %v", err)
	}
	if s.TransferRef != "0xabc" {
		t.Fatal("transfer reference not stored at submission — an interrupted process would leave nothing to reconcile against")
	}
}

func TestSettlingTwiceIsRefused(t *testing.T) {
	s := pending(t)
	_ = s.Submit("0xabc", now)
	if err := s.Confirm(now); err != nil {
		t.Fatalf("confirm refused: %v", err)
	}
	if err := s.Confirm(now); !errors.Is(err, ErrAlreadySettled) {
		t.Fatalf("a settled trade settled again — the seller would be paid twice: %v", err)
	}
}

func TestUnwindingTwiceIsRefused(t *testing.T) {
	s := pending(t)
	if err := s.Fail("compliance refusal", now); err != nil {
		t.Fatalf("fail refused: %v", err)
	}
	if err := s.Fail("again", now); !errors.Is(err, ErrAlreadyUnwound) {
		t.Fatalf("a trade unwound twice — the buyer's cash would be returned twice: %v", err)
	}
}

func TestASettledTradeCannotBeUnwound(t *testing.T) {
	s := pending(t)
	_ = s.Submit("0xabc", now)
	_ = s.Confirm(now)
	if err := s.Fail("changed my mind", now); !errors.Is(err, ErrAlreadySettled) {
		t.Fatalf("a settled trade was unwound — cash returned for an asset that moved: %v", err)
	}
}

// ── Property 4: indeterminate is terminal, never retried ──────────────────────

func TestIndeterminateIsNotAutomaticallyResolvable(t *testing.T) {
	s := pending(t)
	_ = s.Submit("0xabc", now)
	if err := s.MarkIndeterminate("timed out awaiting inclusion", now); err != nil {
		t.Fatalf("mark indeterminate refused: %v", err)
	}

	// Neither outcome may be assumed: confirming pays a seller whose transfer may never
	// have landed, unwinding refunds a buyer who may already hold the asset.
	if err := s.Confirm(now); !errors.Is(err, ErrIndeterminate) {
		t.Fatalf("an indeterminate settlement was confirmed automatically: %v", err)
	}
	if err := s.Fail("giving up", now); !errors.Is(err, ErrIndeterminate) {
		t.Fatalf("an indeterminate settlement was unwound automatically: %v", err)
	}
	if !s.IsTerminal() {
		t.Fatal("indeterminate must be terminal until a human resolves it")
	}
}

func TestIndeterminateResolvesOnlyWithAnExplicitOutcome(t *testing.T) {
	t.Run("found in register", func(t *testing.T) {
		s := pending(t)
		_ = s.Submit("0xabc", now)
		_ = s.MarkIndeterminate("timeout", now)
		if err := s.Resolve(true, "operator confirmed 0xabc in block 41,201,993", now); err != nil {
			t.Fatalf("resolve refused: %v", err)
		}
		if s.State != Settled {
			t.Fatalf("state = %s, want settled", s.State)
		}
	})

	t.Run("absent from register", func(t *testing.T) {
		s := pending(t)
		_ = s.Submit("0xabc", now)
		_ = s.MarkIndeterminate("timeout", now)
		if err := s.Resolve(false, "operator confirmed 0xabc never mined", now); err != nil {
			t.Fatalf("resolve refused: %v", err)
		}
		if s.State != Unwound {
			t.Fatalf("state = %s, want unwound", s.State)
		}
	})
}

func TestResolveOnlyAppliesToIndeterminate(t *testing.T) {
	s := pending(t)
	if err := s.Resolve(true, "no", now); !errors.Is(err, ErrNotPending) {
		t.Fatalf("a pending settlement was resolved as though it were indeterminate: %v", err)
	}
}

// ── The path that should work ─────────────────────────────────────────────────

func TestTheHappyPath(t *testing.T) {
	s := pending(t)
	if err := s.Submit("0xabc", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Confirm(now); err != nil {
		t.Fatal(err)
	}
	if s.State != Settled || !s.IsTerminal() {
		t.Fatalf("state = %s", s.State)
	}
}
