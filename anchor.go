package scrip

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"
)

// Anchoring — SPEC.md §5.
//
// Everything else in this package enforces the authority chain. Anchoring is what makes
// it checkable by someone who does not trust the venue, which is the entire claim: an
// authorisation chain held only in the venue's database requires a holder to ask the
// venue, and that is the trust assumption the protocol exists to remove.
//
// The shape is a commitment, not a disclosure. A corporate act is routinely
// confidential — a board resolution names parties, prices and intentions that are
// nobody else's business until they are. Publishing a hash lets a holder who has been
// shown the record prove it is the one that was committed to, without the venue
// publishing the record to the world.

var (
	// ErrNotAnchored — a mint was attempted against an authorisation whose commitment
	// has not been published. The mint is refused, because an authority chain that is
	// only verifiable for some units is not verifiable.
	ErrNotAnchored = errors.New("scrip: authorisation has not been anchored")
	// ErrAlreadyAnchored — anchoring twice would leave two commitments for one
	// authorisation and no rule for which is authoritative.
	ErrAlreadyAnchored = errors.New("scrip: authorisation is already anchored")
	// ErrCommitmentMismatch — the record does not hash to the published commitment.
	// Either the record was altered after anchoring, or this is not the record that was
	// anchored. Both are the same problem for whoever is holding the units.
	ErrCommitmentMismatch = errors.New("scrip: record does not match the published commitment")
	// ErrNotCommittable — the authorisation is not yet in a state that can be committed
	// to, which in practice means it has not been counter-signed.
	ErrNotCommittable = errors.New("scrip: authorisation cannot be anchored before it is counter-signed")
)

// SaltLen is the length of the commitment salt.
//
// The salt is not decoration. Without it the preimage is a handful of low-entropy
// fields — a quantity that is probably a round number, a date, an identifier that may
// be sequential — and anyone holding the digest could recover them by enumeration.
// That would turn a privacy-preserving commitment into a slow disclosure of every
// issuance the venue has ever made.
const SaltLen = 32

// Commitment is what gets published. The digest is derived from the authorisation
// record and the salt; the salt is disclosed only alongside the record, to whoever is
// entitled to see it.
type Commitment struct {
	// Digest is the SHA-256 over the canonical encoding. This is the only part that
	// goes on-chain.
	Digest [32]byte
	// Salt must be retained. Losing it makes the authorisation unverifiable forever —
	// the commitment stands, but nobody can ever demonstrate what it commits to.
	Salt [SaltLen]byte
}

// Hex renders the digest for publication.
func (c Commitment) Hex() string { return hex.EncodeToString(c.Digest[:]) }

// Anchor is the record that a commitment was published, and where.
type Anchor struct {
	AuthorizationID string
	Digest          [32]byte
	// Reference locates the commitment in the register — a transaction hash. This is
	// what a verifier follows to find the digest independently of the venue.
	Reference string
	At        time.Time
}

// Anchorer publishes a commitment to a register and returns where it landed.
//
// Implementations write to a chain. The interface is this small on purpose: the
// protocol cares that a commitment is durably published somewhere a verifier can reach
// without the venue's cooperation, and not at all which chain or contract that is.
type Anchorer interface {
	// Publish writes the digest and returns a reference to it. It MUST NOT return until
	// the commitment is durable — a reference to a transaction that may still be
	// dropped is exactly the indeterminate state §3.4 refuses to guess about, and here
	// it would mean minting against an anchor that never existed.
	Publish(ctx context.Context, digest [32]byte) (reference string, err error)
}

// Commit produces the commitment for an authorisation.
//
// A fresh salt is generated per call, so committing the same authorisation twice yields
// different digests. That is correct — a deterministic digest would let an observer
// confirm a guess about the contents by recomputing it, which is the property the salt
// exists to deny.
func Commit(a *Authorization) (Commitment, error) {
	if !a.IsCounterSigned() {
		return Commitment{}, ErrNotCommittable
	}

	var salt [SaltLen]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return Commitment{}, err
	}
	return Commitment{Digest: digest(a, salt), Salt: salt}, nil
}

// Verify checks a disclosed record against a published digest.
//
// This is the function a holder, auditor or regulator runs. They obtain the
// authorisation record and salt from the venue, read the digest from the chain, and
// call this. If it returns nil, the venue has shown them the record it committed to
// before minting — which it could not have altered afterwards without the digests
// diverging.
func Verify(a *Authorization, salt [SaltLen]byte, published [32]byte) error {
	computed := digest(a, salt)
	// Constant-time, because a verifier may run this against attacker-influenced input
	// and there is no reason to leak where the first differing byte is.
	if subtle.ConstantTimeCompare(computed[:], published[:]) != 1 {
		return ErrCommitmentMismatch
	}
	return nil
}

// digest computes SHA-256 over a canonical encoding of the fields that constitute the
// authority to issue.
//
// Canonical means one record has exactly one encoding, reproducible by anyone. Every
// variable-length field is length-prefixed before its bytes: without that,
// ("AB", "C") and ("A", "BC") encode identically, and two different authorisations —
// different act references, different signatories — could share a digest. That is a
// real forgery, not a theoretical one, because the attacker chooses the strings.
//
// Field order is fixed by this function and must never be reordered: doing so would
// invalidate every commitment ever published.
func digest(a *Authorization, salt [SaltLen]byte) [32]byte {
	h := sha256.New()

	// Domain separator. Stops a digest produced here from being replayed as a
	// commitment to something else that happens to hash the same bytes.
	writeField(h, []byte("scrip.authorization.v1"))

	writeField(h, salt[:])
	writeField(h, []byte(a.ID))
	writeField(h, []byte(a.InstrumentID))

	// The corporate act — the answer to "under what authority".
	writeField(h, []byte(a.Act.Type))
	writeField(h, []byte(a.Act.Reference))
	writeUint(h, uint64(a.Act.Date.UTC().Unix()))

	writeUint(h, a.Quantity)

	// Both signatories, entity first. Entity is what makes the dual-signature
	// requirement meaningful, so it is what a verifier most needs to be able to check.
	writeField(h, []byte(a.AuthorizedBy.EntityID))
	writeField(h, []byte(a.AuthorizedBy.ID))
	writeField(h, []byte(a.CounterSignedBy.EntityID))
	writeField(h, []byte(a.CounterSignedBy.ID))

	// Attestation references, in the order recorded. Their contents stay private; their
	// identity is committed to, so a venue cannot later claim a different valuation or
	// vault report backed the issuance.
	writeUint(h, uint64(len(a.Attestations)))
	for _, at := range a.Attestations {
		writeField(h, []byte(at.Kind))
		writeField(h, []byte(at.Reference))
		writeUint(h, uint64(at.AsOf.UTC().Unix()))
		writeUint(h, at.Quantity)
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// writeField length-prefixes before writing. See the note in digest.
func writeField(h interface{ Write([]byte) (int, error) }, b []byte) {
	writeUint(h, uint64(len(b)))
	_, _ = h.Write(b)
}

func writeUint(h interface{ Write([]byte) (int, error) }, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	_, _ = h.Write(buf[:])
}
