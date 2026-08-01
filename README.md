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

**Nothing may currently be described as Scrip-conformant, including the reference
implementation.** The specification is complete enough to build against; the conformance
suite is not written.

What exists today in the reference implementation (Alef Markets):

| Property | Status |
|---|---|
| 1 · Issuance traces to a corporate act | Implemented |
| 2 · Dual-party authorisation | Implemented, enforced at storage layer |
| 3 · Atomic supply drawdown | Implemented, enforced by trigger |
| 4 · The chain is the register | Implemented |
| 5 · DvP with unwind | Implemented, exactly-once by unique index |
| **On-chain anchoring of authorisations** | **Not implemented** — see below |

### The gap that matters most

Scrip's central claim is that issuance authority is *independently verifiable*. Today the
authorisation chain lives in the venue's database. A holder cannot check it without asking
the venue — which is the trust assumption the protocol exists to remove.

Closing it requires anchoring each authorisation on-chain: at minimum a hash commitment of
the authorisation record and both signatures, written before the mint that draws on it.
That is specified in [SPEC.md § Anchoring](./SPEC.md#anchoring) and is the first
requirement for a v1.0 that means what it says.

Until then, Scrip is best described as *a specification of the controls, with a reference
implementation that enforces them locally*. That is useful and it is honest. It is not yet
a trust-minimised protocol.

## Repository layout

```
README.md        — this file: the problem, how Scrip differs, asset classes
SPEC.md          — the normative specification
CONTRIBUTING.md  — how to propose changes; how to write a new asset profile
LICENSE          — Apache 2.0
```

The Go reference module is not yet extracted; the specification is the deliverable today.
See [SPEC.md § Reference implementation](./SPEC.md#reference-implementation) for what
extraction requires.

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
