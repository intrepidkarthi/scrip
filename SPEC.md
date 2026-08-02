# Scrip Protocol — Specification

Version 0.1 (draft) · August 2026

The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are to be interpreted as in
RFC 2119.

---

## 1 · Scope

Scrip specifies the **authority chain for tokenized real-world assets**: the record of who
authorised each unit's creation, under what legal act, who counter-signed, and how that
authority is consumed as units are minted.

**In scope**

- Issuance authorisation and its enforcement
- Supply drawdown against authorisation
- Settlement atomicity between asset and cash
- The attestation that the underlying asset exists
- Asset-class profiles carrying what differs between instrument types

**Out of scope, deliberately**

- **Token mechanics.** Scrip assumes a permissioned token with an identity registry and a
  compliance hook. ERC-3643 satisfies this; so do others. Scrip does not specify one.
- **Custody.** Who holds signing keys is orthogonal. Scrip specifies who must *authorise*,
  not who signs.
- **Trading and market structure.** Order books, auctions and RFQ are venue concerns.
- **Yield, pooling and DeFi composability.** Different problem.

## 2 · Roles

| Role | Definition | Constraint |
|---|---|---|
| **Issuer** | The legal entity whose asset or equity the instrument represents | MUST be the entity with authority to create the units under the applicable law |
| **Register-keeper** | The party maintaining the authoritative record of holdings | MUST be a distinct entity from the issuer |
| **Attestor** | The party evidencing that the underlying exists and is as described | MUST be independent of the issuer for asset classes requiring it (§6) |
| **Holder** | The party with beneficial ownership of units | — |

An implementation where issuer and register-keeper are the same entity does not conform.
Self-registered issuance is a legitimate arrangement; it is not this protocol, because the
counter-signature is the control that makes the authority chain meaningful.

## 3 · The five properties

A system conforms only with all five.

### 3.1 · Issuance traces to a corporate act

Every mint MUST reference a specific instrument by which the issuer created the units: a
board resolution, an offering round, a subscription agreement, an SPV formation, a vault
deposit receipt.

The reference MUST carry the act's **type**, **identifier**, and **date**. It MUST NOT be
satisfied by an operator's name, a ticket number, or a free-text note.

> **Failure prevented.** The venue mints on its own initiative and calls it
> issuer-sponsored. In the reference implementation, minting was originally triggered by
> an admin endpoint guarded by a single permission — one operator could create securities
> representing real assets with the issuing company involved at no point.

### 3.2 · The issuer authorises; the register-keeper counter-signs

Two signatures, from two distinct entities.

- The issuer's signatory MUST be verifiably associated with the issuing entity. This MUST
  be enforced, not assumed from a role name.
- The register-keeper's counter-signature MUST come from a different entity and MUST be
  recorded before any mint draws on the authorisation.

> **Failure prevented.** Four-eyes collapsing to one pair. If both signatures can come
> from the same side, the control is theatre.

### 3.3 · Minting draws down the authorisation atomically

An authorisation is for a quantity. Every mint decrements it. Minting beyond it MUST be
refused.

The drawdown and the supply check MUST occur in the same atomic operation as the mint
record. They MUST NOT be a read-then-write in application code.

> **Failure prevented.** An authorisation for 1,000,000 units used to mint 1,000,000 units
> repeatedly. This is the control most often implemented where a retry, a race, or a
> second service can bypass it.

### 3.4 · The register is authoritative and singular

There MUST be exactly one authoritative record of holdings, and the implementation MUST
state which it is.

Where the register is on-chain, an off-chain balance is a projection and MUST NOT be
treated as truth. Settlement MUST mark a transfer complete only on confirmed on-chain
inclusion.

An **indeterminate** result — broadcast but unconfirmed — MUST be a distinct terminal state
requiring human reconciliation. It MUST NOT be retried automatically.

> **Failure prevented.** A database and a chain that disagree with no rule for which wins;
> and automatic retry of a transfer that actually succeeded, moving the asset twice.

### 3.5 · Delivery versus payment, with an unwind path

Where the protocol governs a transfer against payment, cash and asset MUST move together
or not at all.

Cash MUST be held in a settlement account from the moment the trade is struck, released on
confirmed delivery, and returned to the buyer on terminal failure. Release and unwind MUST
each be exactly-once, enforced structurally rather than by an application flag.

> **Failure prevented.** The buyer's cash leaving before the asset arrives, and the venue
> absorbing the difference when the chain leg fails.

## 4 · Supporting requirements

Not distinguishing properties — a system can have all of these and still not be Scrip —
but a conforming implementation without them is not safe to operate.

**4.1 Double-entry accounting.** Every movement of value MUST post a balanced journal,
with the balance check enforced by the storage layer. Balance columns are projections and
MUST be reconciled on a schedule.

Amounts MUST be quantized to the currency's minor unit before being written to either the
ledger or its projection. *Money that cannot be paid MUST NOT be recorded as owed* — a
ledger at higher precision than its projection diverges permanently, a little more with
every distribution.

**4.2 Privileged operations behind dual approval.** Freeze, unfreeze, forced transfer and
identity removal are the powers that make a permissioned token permissioned. Every
compliant security token needs them. Each MUST require two approvers and leave an
immutable record.

**4.3 Append-only audit.** Authorisation and detection records MUST be immutable. Review
state MAY change; the recorded facts MUST NOT. A record that can be edited is not evidence.

**4.4 Issuer verification at admission.** An instrument MUST NOT be admitted to trading
unless its issuer is currently verified and identifiable. Verification at the time the
instrument was drafted is a different claim from verification at the moment the public can
buy.

**4.5 Market abuse surveillance.** Where the implementation operates a secondary market,
detection is insufficient alone: alerts MUST be durable and reviewable. An alert written to
a buffer nobody drains is the same as no alert.

## 5 · Anchoring

**Normative. Implemented in the Go module; not yet deployed against a live chain.**

Scrip's central claim is that issuance authority is verifiable independently of the venue.
An authorisation chain held only in the venue's database does not satisfy that claim: a
holder must ask the venue, which is the trust assumption the protocol exists to remove.

A conforming implementation MUST, before any mint draws on an authorisation, publish
on-chain a commitment containing at minimum:

- a hash of the authorisation record, including the corporate act reference
- the issuer signatory's identity commitment
- the register-keeper counter-signatory's identity commitment
- the authorised quantity

The commitment MUST be verifiable against the authorisation record disclosed to a holder,
auditor or regulator. It SHOULD NOT publish the act's contents, which are frequently
confidential — a hash commitment satisfies verifiability without disclosure.

The commitment MUST be salted with at least 128 bits of entropy. The preimage is
otherwise a small set of low-entropy fields — a quantity that is usually round, a date, an
identifier that may be sequential — and an unsalted digest discloses them to anyone
willing to enumerate.

The encoding MUST be canonical and MUST length-prefix every variable-length field.
Without prefixing, ("AB","C") and ("A","BC") encode identically, so two different
authorisations can share a digest — and the fields in question are chosen by the party
being constrained.

An implementation that has not published its anchors to a chain a third party can read
MAY be described as *enforcing the Scrip controls locally*. It MUST NOT be described as
Scrip-conformant.

### 5.1 · Commitment encoding (normative)

This section exists so that an implementation in any language produces byte-identical
digests. Without it the encoding is defined by whichever implementation you happen to
have, and a verifier written independently of the venue — the only kind whose
verification means anything — cannot be written at all.

**Digest.** SHA-256 over the canonical encoding below.

**Canonical encoding.** The concatenation, in exactly this order, of:

| # | Field | Encoding |
|---|---|---|
| 1 | Domain separator | `field("scrip.authorization.v1")` |
| 2 | Salt | `field(salt)` — 32 bytes |
| 3 | Authorisation ID | `field(id)` |
| 4 | Instrument ID | `field(instrument_id)` |
| 5 | Act type | `field(act.type)` |
| 6 | Act reference | `field(act.reference)` |
| 7 | Act date | `uint64(unix seconds, UTC)` |
| 8 | Quantity | `uint64` |
| 9 | Issuer entity | `field(authorized_by.entity_id)` |
| 10 | Issuer signatory | `field(authorized_by.id)` |
| 11 | Keeper entity | `field(counter_signed_by.entity_id)` |
| 12 | Keeper signatory | `field(counter_signed_by.id)` |
| 13 | Attestation count | `uint64` |
| 14 | Each attestation, in recorded order | `field(kind)`, `field(reference)`, `uint64(as_of unix, UTC)`, `uint64(quantity)` |

Where:

- `uint64(n)` is 8 bytes, **big-endian**.
- `field(b)` is `uint64(len(b))` followed by the bytes of `b`. Strings are UTF-8.

**Length prefixing is mandatory.** Without it `("AB","C")` and `("A","BC")` encode
identically, so two different authorisations can share a digest. The fields concerned —
act references, party identifiers — are chosen by the party the commitment constrains, so
this is a forgery and not a curiosity.

**Field order is frozen.** Reordering invalidates every commitment ever published. A
future revision MUST change the domain separator rather than the order.

**Salt.** At least 128 bits from a cryptographically secure source; 256 bits is
RECOMMENDED and is what the reference implementation uses. A fresh salt per commitment.
The preimage is otherwise a small set of low-entropy fields — a quantity that is usually
round, a date, an identifier that may be sequential — and an unsalted digest discloses
them to anyone willing to enumerate.

**What is committed and what is not.** The act's *identity* is committed (type,
reference, date); its *contents* are not. Attestation references are committed; the
documents are not. This is what makes anchoring usable for issuers whose board
resolutions are confidential.

### 5.2 · Test vector

An implementation is encoding correctly if it reproduces this digest.

```
salt          = 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
id            = "auth-0001"
instrument_id = "inst-0001"
act.type      = "board_resolution"
act.reference = "BR-2026-04"
act.date      = 1776211200          # 2026-04-15T00:00:00Z
quantity      = 1000000
issuer        = entity "entity-issuer", signatory "u-cfo"
keeper        = entity "entity-venue",  signatory "u-registrar"
attestations  = 1
  [0] kind "audited_financials", reference "AUD-2025",
      as_of 1769817600             # 2026-01-31T00:00:00Z
      quantity 0

SHA-256 = d0388a3f447f0dee4ac782aa6c49b5a60fc9198d8d32db461ae79e61f4c4c28c
```

### 5.3 · What a verifier does

1. Obtain the authorisation record and its salt from the venue.
2. Read the digest from the register, and note which address published it.
3. Recompute per §5.1 and compare.

A match establishes that the venue committed to *this* record before minting against it.
It does not establish that the record is true — that the board actually resolved what it
says — which is what the attestations and the counter-signature are for. Anchoring proves
the venue has not changed its story.

## 6 · Asset class profiles

The five properties are asset-class agnostic. What differs is carried in a **profile**,
which declares the requirements the base protocol cannot state generically.

A profile MUST specify: valid issuance instruments, required attestations and their
cadence, the redemption model, the income model, the governing jurisdiction rule, and how
supply changes.

### 6.1 · Profile: operating company equity

*Agricultural companies, SMEs — the reference implementation's original case.*

| | |
|---|---|
| Token represents | Equity in the operating company |
| Issuance instrument | Board resolution; offering round; subscription agreement |
| Attestation | Audited financial statements |
| Cadence | Annual, with the instrument suspended if attestation lapses beyond one period |
| Income | Dividends, distributed pro rata to the register at a record date |
| Redemption | None. Exit is by transfer in the secondary market |
| Jurisdiction | The issuer's place of incorporation |
| Supply | Increases only by a new authorised offering round |

### 6.2 · Profile: real estate

| | |
|---|---|
| Token represents | Fractional interest in a property-holding SPV |
| Issuance instrument | SPV formation document; deed of transfer into the SPV |
| Attestation | Title deed **and** independent valuation survey |
| Cadence | Title on transfer; valuation at least annually |
| Income | Rental distribution, net of costs, at a stated frequency |
| Redemption | None. Exit is by transfer |
| Jurisdiction | **The property's location — *lex situs*.** MAY differ from the issuer's incorporation, and where it does, the property's law governs transfer restrictions |
| Supply | Fixed at SPV formation. Additional units require a new instrument and a revaluation |

**Additional requirements**

- **R1.** The instrument MUST record the property's jurisdiction separately from the
  issuer's. An implementation that assumes one jurisdiction per instrument cannot
  represent a Dubai SPV holding a London property, which is the common case.
- **R2.** A valuation older than the profile's cadence MUST be surfaced to holders and
  SHOULD gate primary issuance. Stale valuations are the principal mis-selling risk in
  tokenized real estate.
- **R3.** Where the SPV holds a single property, the implementation MUST NOT present the
  instrument as diversified.

### 6.3 · Profile: gold and precious metals

| | |
|---|---|
| Token represents | Allocated bullion, by weight and fineness |
| Issuance instrument | Vault deposit receipt |
| Attestation | Assay certificate (fineness) **and** vault attestation (existence, allocation) |
| Cadence | **Continuous proof-of-reserve** |
| Income | None. Storage fees are negative income and MUST be disclosed as such |
| Redemption | **Physical delivery**, subject to a stated minimum |
| Jurisdiction | The vault's location |
| Supply | **Mint on deposit, burn on withdrawal.** Not fixed |

**Additional requirements**

- **G1. Proof of reserve.** Total units outstanding MUST NOT exceed attested vault
  holdings, checked continuously rather than periodically. This is the requirement most
  frequently weakened in practice, and the one whose failure is unrecoverable: units in
  excess of metal are unbacked, and no subsequent audit restores the metal.
- **G2.** Units MUST be denominated in weight and fineness, not in currency. A token
  representing "one gram of 99.99% fine gold" is a claim on an object; one representing
  "$X of gold" is a derivative and outside this profile.
- **G3. Allocated, not pooled.** Bullion MUST be allocated to the instrument and
  identifiable. Unallocated claims are a credit exposure to the vault operator and MUST NOT
  be presented as ownership of metal.
- **G4.** Redemption terms — minimum quantity, delivery jurisdictions, fees — MUST be
  disclosed at issuance, not at the point of request.

### 6.4 · Defining a new profile

A new asset class requires a profile answering all seven dimensions above, plus any
additional requirements. Two questions determine most of it:

1. **Does the underlying exist independently of the issuer?** If yes — gold, real estate —
   the profile needs independent attestation and a cadence. If no — operating equity — the
   issuer's own audited accounts suffice.
2. **Can the holder redeem for the underlying?** If yes, the profile MUST specify supply
   burn on redemption and disclose the terms up front.

## 7 · Reference implementation

Alef Markets, August 2026.

| Requirement | Location |
|---|---|
| 3.1 Corporate act | `migrations/030_issuance_authorization.sql` |
| 3.2 Dual-party | `030` — `issuance_assert_parties()` |
| 3.3 Atomic drawdown | `030` — `issuance_assert_within_supply()` |
| 3.4 Register | `services/trading-server/internal/services/settlement.go` |
| 3.5 DvP + unwind | `migrations/028_settlement_dvp.sql` |
| 4.1 Double-entry | `migrations/027_double_entry_ledger.sql`, `ledger/` |
| 4.2 Privileged ops | `migrations/031_privileged_operations.sql` |
| 4.4 Issuer gate | `migrations/036_issuer_verification_gate.sql` |
| 4.5 Surveillance | `migrations/035_surveillance_alerts.sql` |
| 5 Anchoring | `anchor.go` — not yet wired into Alef |
| 6.2 Real estate profile | Not implemented |
| 6.3 Gold profile | Not implemented |

### The Go module

This repository also carries a reference implementation of the invariants themselves,
independent of Alef: `authorization.go`, `settlement.go`, `profile.go`, `store.go`. All
three profiles in §6 are implemented there, including the two requirements Alef cannot
currently satisfy — real estate's separate asset jurisdiction, and gold's reserve ceiling
checked on every mint.

Storage sits behind the `Store` interface, so the invariants are no longer bound to one
database. The package checks every rule and expects the `Store` to enforce them too;
`memstore.go` is the reference for what that means, most importantly that drawdown and
mint commit atomically.

This does not close §5. Anchoring remains unimplemented and nothing is conformant.

### What extraction requires

The reference implementation is Alef-shaped in ways a protocol must not be:

- Controls live in PostgreSQL triggers, binding them to one database engine.
- Role names (`admin`, `staff`, `issuer`) are Alef's, not the protocol's.
- ERC-3643 is assumed in the blockchain service rather than expressed behind an interface.
- Only the operating-company profile exists; the profile concept is implicit.

A genuine extraction produces a `scrip` module alongside the existing `ledger` module: the
authorisation chain, the DvP state machine, and the profile definitions, with storage
behind an interface and its own invariant tests. `ledger/` — already a standalone module
consumed by two services — is the model.

## 8 · Conformance

A conformance suite does not yet exist. Until it does, **no implementation may be described
as Scrip-conformant**, including the reference implementation.

When written, the suite MUST verify at minimum:

- An authorisation cannot be created without a typed, identified, dated corporate act
- An authorisation signed by a party not associated with the issuer is refused
- A counter-signature from the same entity as the authoriser is refused
- Minting beyond the authorised quantity is refused under concurrent load
- A settlement whose chain leg is indeterminate does not retry
- A release and an unwind for the same trade cannot both succeed
- The profile's attestation requirements are enforced at issuance

## 9 · Open questions

1. **Anchoring granularity.** Per authorisation, or a periodic Merkle root of all
   authorisations? Per-authorisation is simpler to verify and more expensive; a root is
   cheap and requires the venue to serve inclusion proofs, reintroducing a dependency on
   the venue being available.
2. **Verification lapse during trading.** Admission is gated (§4.4); continued listing is
   not. Automatic suspension on a lapsed attestation is operationally dangerous —
   a bureaucratic delay would halt a market. Manual review is slow. Neither is obviously
   right.
3. **Corporate act taxonomy.** Should the protocol define a closed set of instrument types,
   or accept any typed reference? A closed set aids machine verification and invites
   jurisdictional mismatch.
4. **Multi-jurisdiction instruments.** §6.2 R1 requires the property's jurisdiction be
   recorded separately. Where three jurisdictions are involved — issuer, asset, holder —
   the protocol does not currently state which governs a transfer restriction.

---

*Licensed under Apache 2.0. See [LICENSE](./LICENSE).*
