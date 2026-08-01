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

**Normative for v1.0. Not implemented in the reference implementation.**

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

Until anchoring is implemented, an implementation MAY be described as *enforcing the Scrip
controls locally*. It MUST NOT be described as Scrip-conformant.

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
| 5 Anchoring | **Not implemented** |
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
