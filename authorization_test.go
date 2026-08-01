package scrip

import (
	"errors"
	"testing"
	"time"
)

// Every test here asserts a refusal. That is the shape of this package: the value is
// not in what it lets you do, it is in what it will not let you do, and a test suite
// that mostly exercises happy paths would verify none of it.

var now = time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)

func equityInstrument() Instrument {
	return Instrument{ID: "inst-1", IssuerEntityID: "entity-issuer", Profile: EquityProfile()}
}

func goodAct() CorporateAct {
	return CorporateAct{Type: BoardResolution, Reference: "BR-2026-04", Date: now.AddDate(0, -1, 0)}
}

func issuerParty() Party {
	return Party{ID: "u-cfo", Kind: Issuer, EntityID: "entity-issuer"}
}

func keeperParty() Party {
	return Party{ID: "u-registrar", Kind: RegisterKeeper, EntityID: "entity-venue"}
}

func freshAudit() []Attestation {
	return []Attestation{{Kind: AuditedFinancials, Reference: "AUD-2025", AsOf: now.AddDate(0, -6, 0)}}
}

// ── Property 1 ────────────────────────────────────────────────────────────────

func TestAuthorizationRequiresACompleteCorporateAct(t *testing.T) {
	cases := map[string]CorporateAct{
		"no type":      {Reference: "BR-1", Date: now},
		"no reference": {Type: BoardResolution, Date: now},
		"no date":      {Type: BoardResolution, Reference: "BR-1"},
	}
	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewAuthorization("a1", equityInstrument(), act, 1000, issuerParty(), freshAudit(), now)
			if !errors.Is(err, ErrIncompleteCorporateAct) {
				t.Fatalf("an act with %s was accepted; it could not be produced in an audit. got %v", name, err)
			}
		})
	}
}

func TestActMustSuitTheProfile(t *testing.T) {
	// A vault deposit receipt cannot create company equity.
	act := CorporateAct{Type: VaultDepositReceipt, Reference: "VD-9", Date: now}
	_, err := NewAuthorization("a1", equityInstrument(), act, 1000, issuerParty(), freshAudit(), now)
	if !errors.Is(err, ErrActNotValidForProfile) {
		t.Fatalf("a vault deposit receipt authorised company equity: %v", err)
	}
}

// ── Property 2 ────────────────────────────────────────────────────────────────

func TestSignatoryMustActForTheIssuer(t *testing.T) {
	stranger := Party{ID: "u-x", Kind: Issuer, EntityID: "some-other-entity"}
	_, err := NewAuthorization("a1", equityInstrument(), goodAct(), 1000, stranger, freshAudit(), now)
	if !errors.Is(err, ErrNotIssuer) {
		t.Fatalf("an authorisation was signed by a party who does not act for the issuer: %v", err)
	}
}

func TestRegisterKeeperRoleIsRequiredToCounterSign(t *testing.T) {
	a := mustAuthorize(t)
	notKeeper := Party{ID: "u-other", Kind: Issuer, EntityID: "entity-venue"}
	if err := a.CounterSign(notKeeper, now); !errors.Is(err, ErrNotRegisterKeeper) {
		t.Fatalf("a non-register-keeper counter-signed: %v", err)
	}
}

// The failure that turns four-eyes into theatre.
func TestCounterSignatureMustComeFromADifferentEntity(t *testing.T) {
	a := mustAuthorize(t)
	sameFirm := Party{ID: "u-colleague", Kind: RegisterKeeper, EntityID: "entity-issuer"}
	if err := a.CounterSign(sameFirm, now); !errors.Is(err, ErrSameEntity) {
		t.Fatalf("both signatures came from the same entity and were accepted: %v", err)
	}
}

func TestCounterSignatureMustComeFromADifferentPerson(t *testing.T) {
	a := mustAuthorize(t)
	// Same human, second hat.
	sameHuman := Party{ID: "u-cfo", Kind: RegisterKeeper, EntityID: "entity-venue"}
	if err := a.CounterSign(sameHuman, now); !errors.Is(err, ErrSameEntity) {
		t.Fatalf("one person signed both sides: %v", err)
	}
}

func TestCounterSigningTwiceIsRefused(t *testing.T) {
	a := mustAuthorize(t)
	if err := a.CounterSign(keeperParty(), now); err != nil {
		t.Fatalf("valid counter-signature refused: %v", err)
	}
	second := Party{ID: "u-other-registrar", Kind: RegisterKeeper, EntityID: "entity-venue"}
	if err := a.CounterSign(second, now); !errors.Is(err, ErrAlreadyCounterSigned) {
		t.Fatalf("a second counter-signature silently replaced the first: %v", err)
	}
}

// ── Property 3 ────────────────────────────────────────────────────────────────

func TestNothingIsMintedBeforeCounterSignature(t *testing.T) {
	a := mustAuthorize(t)
	if err := a.Draw(1); !errors.Is(err, ErrNotCounterSigned) {
		t.Fatalf("units were drawn against an unconfirmed authorisation: %v", err)
	}
}

func TestDrawdownCannotExceedAuthority(t *testing.T) {
	a := mustCounterSigned(t, 1000)

	if err := a.Draw(600); err != nil {
		t.Fatalf("first draw refused: %v", err)
	}
	if err := a.Draw(500); !errors.Is(err, ErrExceedsAuthorization) {
		t.Fatalf("drew 1100 units against an authorisation for 1000: %v", err)
	}
	if a.Drawn != 600 {
		t.Fatalf("a refused draw still moved the counter: drawn = %d", a.Drawn)
	}
	if got := a.Remaining(); got != 400 {
		t.Fatalf("remaining = %d, want 400", got)
	}
}

// An authorisation for a million units used to mint a million units repeatedly is the
// failure this guards; exhausting it must leave nothing behind.
func TestExhaustedAuthorizationIsExhausted(t *testing.T) {
	a := mustCounterSigned(t, 100)
	if err := a.Draw(100); err != nil {
		t.Fatalf("draw refused: %v", err)
	}
	if err := a.Draw(1); !errors.Is(err, ErrExceedsAuthorization) {
		t.Fatalf("an exhausted authorisation minted another unit: %v", err)
	}
}

// Integer overflow is a real way to defeat a bound written as an addition.
func TestOverflowCannotDefeatTheBound(t *testing.T) {
	a := mustCounterSigned(t, 1000)
	a.Drawn = 999
	// Drawn + quantity wraps to a small number if the check is written as an addition.
	if err := a.Draw(^uint64(0)); !errors.Is(err, ErrExceedsAuthorization) {
		t.Fatalf("an overflowing quantity passed the authorisation bound: %v", err)
	}
}

// ── Profile requirements ──────────────────────────────────────────────────────

func TestAttestationIsRequired(t *testing.T) {
	_, err := NewAuthorization("a1", equityInstrument(), goodAct(), 1000, issuerParty(), nil, now)
	if !errors.Is(err, ErrAttestationRequired) {
		t.Fatalf("equity was authorised with no audited financials: %v", err)
	}
}

func TestStaleAttestationIsRefused(t *testing.T) {
	old := []Attestation{{Kind: AuditedFinancials, Reference: "AUD-2019", AsOf: now.AddDate(-5, 0, 0)}}
	_, err := NewAuthorization("a1", equityInstrument(), goodAct(), 1000, issuerParty(), old, now)
	if !errors.Is(err, ErrAttestationStale) {
		t.Fatalf("a five-year-old audit was accepted: %v", err)
	}
}

// Backdating forward is the same fault as backdating backward, and accepting it would
// let one document satisfy the freshness rule permanently.
func TestFutureDatedAttestationIsRefused(t *testing.T) {
	future := []Attestation{{Kind: AuditedFinancials, Reference: "AUD-2030", AsOf: now.AddDate(4, 0, 0)}}
	_, err := NewAuthorization("a1", equityInstrument(), goodAct(), 1000, issuerParty(), future, now)
	if !errors.Is(err, ErrAttestationStale) {
		t.Fatalf("an attestation dated in the future was accepted: %v", err)
	}
}

func TestZeroQuantityIsRefused(t *testing.T) {
	_, err := NewAuthorization("a1", equityInstrument(), goodAct(), 0, issuerParty(), freshAudit(), now)
	if !errors.Is(err, ErrZeroQuantity) {
		t.Fatalf("a zero-unit authorisation was created: %v", err)
	}
}

// ── The path that should work ─────────────────────────────────────────────────

func TestAValidAuthorizationSucceeds(t *testing.T) {
	a := mustCounterSigned(t, 500)
	if !a.IsCounterSigned() {
		t.Fatal("counter-signature not recorded")
	}
	if a.CounterSignedAt == nil {
		t.Fatal("counter-signature time not recorded — the audit trail needs when, not just who")
	}
	if err := a.Draw(500); err != nil {
		t.Fatalf("a valid full draw was refused: %v", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustAuthorize(t *testing.T) *Authorization {
	t.Helper()
	a, err := NewAuthorization("a1", equityInstrument(), goodAct(), 1000, issuerParty(), freshAudit(), now)
	if err != nil {
		t.Fatalf("valid authorisation refused: %v", err)
	}
	return a
}

func mustCounterSigned(t *testing.T, qty uint64) *Authorization {
	t.Helper()
	a, err := NewAuthorization("a1", equityInstrument(), goodAct(), qty, issuerParty(), freshAudit(), now)
	if err != nil {
		t.Fatalf("valid authorisation refused: %v", err)
	}
	if err := a.CounterSign(keeperParty(), now); err != nil {
		t.Fatalf("valid counter-signature refused: %v", err)
	}
	return a
}
