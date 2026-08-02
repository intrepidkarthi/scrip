# Scrip Protocol

**An open protocol for issuer-sponsored tokenization of real-world assets.**

`github.com/intrepidkarthi/scrip`

Scrip specifies how a token comes to exist: who authorised it, under what legal act, who
counter-signed, and how that authority is drawn down as units are minted. It is a protocol
for the *authority chain*, not a token standard and not a platform.

Status: **v0.1 draft.** Specification written, reference implementation partial. Not yet
released — see [Conformance](#conformance) for what is and is not claimable today.

---

## The problem

Ask who created a tokenized security and where its authority came from, and most stacks
cannot answer in a way you can verify.

**Token standards** — ERC-3643, ERC-1400, ERC-3525 — specify what a token may do once it
exists: who may hold it, what transfers are permitted, how compliance modules gate
movement. They say nothing about who may *create* it. A T-REX token minted by a platform
operator on their own initiative is indistinguishable, on-chain, from one minted under a
board resolution.

**Platforms** — Securitize, Tokeny, ADDX, Chintai, Brickken — implement authorisation
internally, as business logic. It may be excellent. It is also unauditable from outside:
you are trusting the platform's database and the platform's word about it.

**Asset issuers** — Paxos Gold, Tether Gold, Ondo, RealT — ask you to trust a named
counterparty. That is a legitimate model, and for many holders the right one. But the
trust is in the entity, not in a verifiable record.

The gap in all three: **the authority for each unit in existence is not a matter of record
you can check independently of the party that benefits from it.**

## What Scrip specifies

Five properties, each defined by the failure it prevents. A system implementing all five
can answer, for any unit of any instrument:

- Under what legal act was this created?
- Which party at the issuer authorised it, and were they entitled to?
- Who at the register-keeper counter-signed, and were they a different entity?
- Does the sum of all units ever minted stay within what was authorised?
- Did the asset and the cash move together, or is one of them owed?

The full normative specification is in **[SPEC.md](./SPEC.md)**.

## How Scrip differs

| | What it governs | Who verifies | Scrip's relationship |
|---|---|---|---|
| **ERC-3643 / T-REX** | Transfer permissions on an existing token | On-chain, anyone | **Uses it.** Scrip is the layer above: how the token came to exist |
| **ERC-1400 / 1404** | Partitioned securities, transfer restrictions | On-chain | Alternative substrate; Scrip is standard-agnostic |
| **Polymesh, Provenance** | Purpose-built securities chains | Chain validators | Substrate. Scrip runs on any chain with an identity registry |
| **Securitize, Tokeny** | Full issuance + transfer agency, commercially | The vendor | Overlapping scope, opposite posture: they are products, Scrip is a spec anyone may implement |
| **Ondo, Paxos, RealT** | A specific tokenized asset | The issuer's attestations | Instances. Any of them could conform to Scrip; none currently publishes an authority chain |
| **Centrifuge, Maple** | Pooled credit, DeFi-native | Pool mechanics on-chain | Different problem: pooling and yield, not issuance authority |

The honest summary: **Scrip is not competing with the token standards — it depends on
one.** It competes with the *implicit, unpublished* authorisation logic inside closed
platforms, by making that logic a specification with named failure modes that an auditor,
a regulator, or a holder can check against.

### What is genuinely novel

The **issuance authorisation chain as a first-class, enforced, auditable artifact**:
dual-party attestation bound to a named corporate act, with supply drawdown enforced at
the storage layer rather than in bypassable application code.

To our knowledge no RWA protocol publishes this. Platforms implement something like it
privately; standards omit it entirely.

### What is not novel, and should not be claimed as such

- Token mechanics — ERC-3643 does this, and Scrip uses it
- Delivery-versus-payment — standard settlement practice since the 1980s
- Double-entry accounting — standard since the 15th century
- Four-eyes approval — standard operational control

Scrip's contribution is specifying how these compose around issuance authority, and
refusing the shortcuts that make each one decorative.

## Asset classes

Scrip is asset-class agnostic at its core, with **profiles** carrying what differs. Three
are specified:

| | Agricultural / operating company | Real estate | Gold and precious metals |
|---|---|---|---|
| Token represents | Equity in the operating company | Fractional interest in a property SPV | Allocated bullion by weight and fineness |
| Issuance instrument | Board resolution, offering round | SPV formation, deed transfer | Vault deposit receipt |
| Proof of underlying | Audited financials | Title deed + valuation survey | Assay certificate + vault attestation |
| Attestation cadence | Annual | Annual valuation, on-transfer title | **Continuous** — proof-of-reserve |
| Income | Dividends | Rental distribution | None; storage fees are negative income |
| Redemption | None — equity | None — exit via market | **Physical delivery** |
| Governing jurisdiction | Issuer's incorporation | **Property location** (*lex situs*) | Vault location |
| Supply changes | New offering rounds | New SPV units | **Mint on deposit, burn on withdrawal** |

The bold entries are where a profile adds a requirement the base protocol does not have —
and where an implementation built only for one asset class quietly fails for another.
Gold's proof-of-reserve and real estate's *lex situs* are the two that most often get
missed, because an equity-shaped implementation has no place to put them.

See [SPEC.md § Asset class profiles](./SPEC.md#asset-class-profiles).

## Conformance

**No deployed system may currently be described as Scrip-conformant.** The Go module
implements and tests every property including anchoring; what is missing is a venue
running it against a real chain, and a conformance suite that an independent
implementation can be run through.

What exists today in the reference implementation (Alef Markets):

| Property | Status |
|---|---|
| 1 · Issuance traces to a corporate act | Implemented |
| 2 · Dual-party authorisation | Implemented, enforced at storage layer |
| 3 · Atomic supply drawdown | Implemented, enforced by trigger |
| 4 · The chain is the register | Implemented |
| 5 · DvP with unwind | Implemented, exactly-once by unique index |
| **On-chain anchoring of authorisations** | Implemented in the Go module; **not deployed by any venue** — see below |

### Anchoring — what a holder can now check

The Go module implements §5. Before any mint, the venue publishes a salted SHA-256
commitment over the authorisation record: the corporate act, the quantity, both
signatories and their entities, and every attestation reference. `Issue` refuses to mint
against an authorisation that has not been anchored, so the guarantee covers every unit
rather than the ones a venue expects questions about.

A holder, auditor or regulator obtains the record and salt from the venue, reads the
digest from the chain, and calls `Verify`. If the venue altered anything after committing
— the quantity, which board resolution, who signed, which valuation backed it — the
digests diverge and `Verify` says so.

It is a commitment, not a disclosure. A board resolution names parties, prices and
intentions that are nobody else's business; publishing a hash lets someone who has been
shown the record prove it is the one committed to, without the venue publishing it to the
world. The salt matters for the same reason: without it the preimage is a handful of
low-entropy fields — a round quantity, a date, a sequential identifier — and anyone
holding the digest could recover them by enumeration.

### Why this is a protocol and not just a library

The commitment encoding is specified normatively in [SPEC.md §5.1](./SPEC.md#51--commitment-encoding-normative)
— byte layout, field order, big-endian widths, mandatory length prefixing — with a
[test vector](./SPEC.md#52--test-vector) any implementation can check itself against.

That is the difference. Two independent implementations, in different languages, written
by parties who do not trust each other, can produce and verify the same commitment. A
verifier written by a regulator against the spec alone will agree with a venue's Go
implementation, or the venue has a problem. Without a specified encoding there is no
interoperation and the word "protocol" would be doing no work.

The Go module pins the vector in a test, so the encoding cannot drift from the
specification silently.

### What is still missing

**No venue has deployed this.** The module provides `Anchorer` as a one-method interface
and the protocol does not care which chain satisfies it, but a specification and a
reference implementation are not a running system. Alef, the implementation Scrip was
extracted from, does not yet anchor.

So: the controls are implemented and tested, and the verifiability claim is now
*implementable* rather than aspirational. It is not yet *demonstrated*, and no
implementation should be described as conformant until its anchors are on a chain someone
else can read.

## Repository layout

```
README.md        — this file: the problem, how Scrip differs, asset classes
SPEC.md          — the normative specification
CONTRIBUTING.md  — how to propose changes; how to write a new asset profile
LICENSE          — Apache 2.0

doc.go           — package overview
authorization.go — the authority chain: parties, corporate acts, drawdown
settlement.go    — the DvP state machine, including the indeterminate state
profile.go       — asset class profiles: equity, real estate, gold
store.go         — the persistence contract, and Issue()
errors.go        — one error per invariant
memstore.go      — in-memory reference Store, unexported by design
*_test.go        — the invariants, as tests
```

## The Go module

```
go get github.com/intrepidkarthi/scrip
```

Zero dependencies. A protocol that drags a dependency tree into every adopter's build is
one fewer reason to adopt it, and each dependency would impose a decision about how to
represent money, identity or time.

Quantities are integer units. What one unit *means* — a share, a gram of 99.99% fine gold,
a fraction of an SPV — is declared by the instrument's profile. This keeps decimal
rounding out of the protocol entirely, which matters because the ledger requirement in
§4.1 exists precisely to stop rounding drift.

```go
auth, err := scrip.NewAuthorization(id, instrument,
    scrip.CorporateAct{Type: scrip.BoardResolution, Reference: "BR-2026-04", Date: resolved},
    1_000_000, issuerSignatory, attestations, now)

err = auth.CounterSign(registerKeeperSignatory, now)  // must be a different entity

mint, err := scrip.Issue(ctx, store, auth.ID, mintID, holderID, 250_000, now)
```

Every invariant is checked in the package *and* expected to be enforced by your [Store].
That duplication is the design: application checks give callers a diagnosable error,
storage checks give the system a guarantee under concurrency, and neither substitutes for
the other. `memstore.go` shows what a conforming Store must do — most importantly that
drawdown and mint commit together.

The tests are the specification made executable, and they are almost all refusals: the
value of this package is not what it lets you do.

## Contributing

New **asset class profiles** are the most useful contribution right now — treasuries,
private credit, carbon credits, art, equipment leasing. See [CONTRIBUTING.md](./CONTRIBUTING.md);
the value is in the additional requirements, not the table.

## Licence

Apache 2.0. Chosen over MIT for the explicit patent grant — a protocol touching securities
issuance should not leave adopters exposed to a later patent claim by a contributor.

---

*Scrip: historically, the provisional certificate of ownership issued before the
definitive share certificate. A securities term, deliberately, rather than a crypto one.*
