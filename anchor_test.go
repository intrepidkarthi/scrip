package scrip

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// §5 is the requirement that turns the authority chain from something the venue asserts
// into something a holder can check. These tests are about what a verifier who does not
// trust the venue can establish.

// fakeAnchorer stands in for a chain. Records what it was asked to publish so a test
// can assert the venue committed before it minted, rather than after.
type fakeAnchorer struct {
	published [][32]byte
	fail      error
}

func (f *fakeAnchorer) Publish(_ context.Context, d [32]byte) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	f.published = append(f.published, d)
	return "0xanchor" + hex.EncodeToString(d[:4]), nil
}

func anchoredSetup(t *testing.T) (*memStore, *fakeAnchorer, *Authorization) {
	t.Helper()
	st := newMemStore()
	inst := equityInstrument()
	st.instruments[inst.ID] = inst

	a := mustCounterSigned(t, 1000)
	st.auths[a.ID] = a

	an := &fakeAnchorer{}
	if _, err := AnchorAuthorization(context.Background(), st, an, a.ID, now); err != nil {
		t.Fatalf("anchoring refused: %v", err)
	}
	return st, an, a
}

// The core requirement: no anchor, no mint. Without this the rule is advisory — a venue
// could anchor the authorisations it expects questions about and mint freely against
// the rest.
func TestMintingRequiresAnAnchor(t *testing.T) {
	st := newMemStore()
	inst := equityInstrument()
	st.instruments[inst.ID] = inst
	a := mustCounterSigned(t, 1000)
	st.auths[a.ID] = a

	_, err := Issue(context.Background(), st, a.ID, "m1", "holder", 10, now)
	if !errors.Is(err, ErrNotAnchored) {
		t.Fatalf("units were minted against an authorisation nobody can verify: %v", err)
	}
}

func TestMintingSucceedsOnceAnchored(t *testing.T) {
	st, an, a := anchoredSetup(t)
	if len(an.published) != 1 {
		t.Fatalf("expected one published commitment, got %d", len(an.published))
	}
	if _, err := Issue(context.Background(), st, a.ID, "m1", "holder", 10, now); err != nil {
		t.Fatalf("minting against an anchored authorisation was refused: %v", err)
	}
}

func TestCannotAnchorBeforeCounterSignature(t *testing.T) {
	st := newMemStore()
	inst := equityInstrument()
	st.instruments[inst.ID] = inst
	a := mustAuthorize(t) // not counter-signed
	st.auths[a.ID] = a

	_, err := AnchorAuthorization(context.Background(), st, &fakeAnchorer{}, a.ID, now)
	if !errors.Is(err, ErrNotCommittable) {
		t.Fatalf("a half-signed authorisation was committed to: %v", err)
	}
}

func TestAnchoringTwiceIsRefused(t *testing.T) {
	st, an, a := anchoredSetup(t)
	_, err := AnchorAuthorization(context.Background(), st, an, a.ID, now)
	if !errors.Is(err, ErrAlreadyAnchored) {
		t.Fatalf("a second commitment was published — a verifier would not know which is authoritative: %v", err)
	}
}

// If publication fails, nothing may be recorded as anchored. The reverse of the
// ordering guarantee in AnchorAuthorization.
func TestFailedPublicationDoesNotRecordAnAnchor(t *testing.T) {
	st := newMemStore()
	inst := equityInstrument()
	st.instruments[inst.ID] = inst
	a := mustCounterSigned(t, 1000)
	st.auths[a.ID] = a

	boom := errors.New("chain unreachable")
	if _, err := AnchorAuthorization(context.Background(), st, &fakeAnchorer{fail: boom}, a.ID, now); !errors.Is(err, boom) {
		t.Fatalf("expected the publication error, got %v", err)
	}
	if _, err := st.Anchor(context.Background(), a.ID); !errors.Is(err, ErrNotAnchored) {
		t.Fatal("an anchor was recorded for a commitment that was never published")
	}
	// And therefore minting is still refused.
	if _, err := Issue(context.Background(), st, a.ID, "m1", "holder", 1, now); !errors.Is(err, ErrNotAnchored) {
		t.Fatalf("minted after a failed anchor: %v", err)
	}
}

// ── What a verifier can establish ─────────────────────────────────────────────

func TestAHolderCanVerifyTheDisclosedRecord(t *testing.T) {
	st, _, a := anchoredSetup(t)

	// What the venue discloses to the holder, and what the holder reads from the chain.
	salt, ok := st.salt(a.ID)
	if !ok {
		t.Fatal("salt was not retained — the commitment can never be demonstrated")
	}
	anchor, err := st.Anchor(context.Background(), a.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := Verify(a, salt, anchor.Digest); err != nil {
		t.Fatalf("the disclosed record did not verify against the published commitment: %v", err)
	}
}

// The point of the whole exercise: a venue that alters the record after committing is
// caught.
func TestTamperingWithTheRecordIsDetected(t *testing.T) {
	st, _, a := anchoredSetup(t)
	salt, _ := st.salt(a.ID)
	anchor, _ := st.Anchor(context.Background(), a.ID)

	tampered := []struct {
		name  string
		apply func(*Authorization)
	}{
		{"quantity increased", func(x *Authorization) { x.Quantity = 2_000_000 }},
		{"corporate act reference swapped", func(x *Authorization) { x.Act.Reference = "BR-2026-99" }},
		{"act type changed", func(x *Authorization) { x.Act.Type = OfferingRound }},
		{"issuer signatory replaced", func(x *Authorization) { x.AuthorizedBy.ID = "u-someone-else" }},
		{"counter-signatory's entity changed", func(x *Authorization) { x.CounterSignedBy.EntityID = "entity-issuer" }},
		{"attestation reference swapped", func(x *Authorization) {
			if len(x.Attestations) > 0 {
				x.Attestations[0].Reference = "AUD-different"
			}
		}},
	}

	for _, tc := range tampered {
		t.Run(tc.name, func(t *testing.T) {
			// Work on a copy so each case starts from the committed record.
			clone := *a
			cs := *a.CounterSignedBy
			clone.CounterSignedBy = &cs
			clone.Attestations = append([]Attestation(nil), a.Attestations...)

			tc.apply(&clone)

			if err := Verify(&clone, salt, anchor.Digest); !errors.Is(err, ErrCommitmentMismatch) {
				t.Fatalf("%s went undetected — the venue could rewrite history after committing", tc.name)
			}
		})
	}
}

// Without the salt the preimage is a handful of low-entropy fields and the commitment
// discloses them by enumeration. Two commitments to the same record must differ.
func TestCommitmentsAreSalted(t *testing.T) {
	a := mustCounterSigned(t, 1000)

	c1, err := Commit(a)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := Commit(a)
	if err != nil {
		t.Fatal(err)
	}

	if c1.Salt == c2.Salt {
		t.Fatal("two commitments used the same salt")
	}
	if c1.Digest == c2.Digest {
		t.Fatal("committing the same record twice produced the same digest — an observer could confirm a guess by recomputing it")
	}
	// Each still verifies against its own salt.
	if err := Verify(a, c1.Salt, c1.Digest); err != nil {
		t.Fatalf("commitment did not verify against its own salt: %v", err)
	}
	if err := Verify(a, c2.Salt, c1.Digest); !errors.Is(err, ErrCommitmentMismatch) {
		t.Fatal("a commitment verified against the wrong salt")
	}
}

// Length-prefixing. Without it ("AB","C") and ("A","BC") encode identically, and an
// attacker who chooses the strings can make two different authorisations share a
// digest — a real forgery, since party identifiers and act references are
// attacker-supplied in the case that matters.
//
// The two fields varied here must be ADJACENT in the encoding. An earlier version of
// this test varied the act reference and the signatory ID, which are separated by the
// fixed-width date and quantity — so it passed with length-prefixing removed and
// tested nothing.
func TestFieldBoundariesCannotBeShifted(t *testing.T) {
	base := mustCounterSigned(t, 1000)
	base.AuthorizedBy.EntityID = "AB"
	base.AuthorizedBy.ID = "C"

	shifted := *base
	cs := *base.CounterSignedBy
	shifted.CounterSignedBy = &cs
	shifted.AuthorizedBy.EntityID = "A"
	shifted.AuthorizedBy.ID = "BC"

	var salt [SaltLen]byte
	if digest(base, salt) == digest(&shifted, salt) {
		t.Fatal("two different authorisations share a digest — field boundaries are not committed to")
	}
}

// The test vector published in SPEC.md §5.2.
//
// This is the load-bearing test for calling Scrip a protocol rather than a library. An
// implementation in another language is conformant on this point if it reproduces this
// digest; if this test ever fails, the encoding has changed and every commitment ever
// published under the old one has been invalidated.
//
// Changing it requires changing the domain separator in digest() and re-publishing the
// vector, not adjusting the constant below.
func TestSpecTestVector(t *testing.T) {
	var salt [SaltLen]byte
	for i := range salt {
		salt[i] = byte(i)
	}

	a := &Authorization{
		ID:           "auth-0001",
		InstrumentID: "inst-0001",
		Act: CorporateAct{
			Type:      BoardResolution,
			Reference: "BR-2026-04",
			Date:      time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		},
		Quantity:        1_000_000,
		AuthorizedBy:    Party{ID: "u-cfo", Kind: Issuer, EntityID: "entity-issuer"},
		CounterSignedBy: &Party{ID: "u-registrar", Kind: RegisterKeeper, EntityID: "entity-venue"},
		Attestations: []Attestation{{
			Kind:      AuditedFinancials,
			Reference: "AUD-2025",
			AsOf:      time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		}},
	}

	const want = "d0388a3f447f0dee4ac782aa6c49b5a60fc9198d8d32db461ae79e61f4c4c28c"
	got := digest(a, salt)
	if hex.EncodeToString(got[:]) != want {
		t.Fatalf("encoding has changed — every published commitment is now unverifiable.\n got  %s\n want %s",
			hex.EncodeToString(got[:]), want)
	}
}

// Dates are committed in UTC. A venue in Dubai and a verifier in London must compute the
// same digest from the same record, and a timezone-local encoding would silently give
// them different ones.
func TestEncodingIsTimezoneIndependent(t *testing.T) {
	var salt [SaltLen]byte
	gulf := time.FixedZone("GST", 4*3600)

	utc := &Authorization{
		ID: "a", InstrumentID: "i",
		Act:             CorporateAct{Type: BoardResolution, Reference: "R", Date: time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC)},
		Quantity:        1,
		AuthorizedBy:    Party{ID: "x", EntityID: "e1"},
		CounterSignedBy: &Party{ID: "y", EntityID: "e2"},
	}
	local := *utc
	cs := *utc.CounterSignedBy
	local.CounterSignedBy = &cs
	// The same instant, expressed in another zone.
	local.Act.Date = utc.Act.Date.In(gulf)

	if digest(utc, salt) != digest(&local, salt) {
		t.Fatal("the same instant in two timezones produced different digests")
	}
}
