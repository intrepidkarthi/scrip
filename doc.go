// Package scrip is the reference implementation of the Scrip Protocol: the authority
// chain for tokenized real-world assets.
//
// It answers, for any unit of any instrument: under what legal act was this created,
// who at the issuer authorised it, who at the register-keeper counter-signed, and does
// the total ever minted stay within what was authorised.
//
// # What this package is
//
// The invariants, expressed once, in code that can be tested. Storage sits behind
// [Store] so the protocol is not bound to one database — the specification requires
// enforcement at the storage layer, and this package's job is to define exactly what a
// storage layer must enforce, then verify that it does.
//
// # What this package is not
//
// It is not a token standard, a custody model, or a settlement network. It assumes a
// permissioned token with an identity registry and a compliance hook — ERC-3643
// satisfies this — and says nothing about which one you use or who holds the keys.
//
// # Defence in depth, not delegation
//
// Every invariant is checked here AND expected to be enforced by the Store. That
// duplication is deliberate. Application-layer checks are bypassable by the next
// service someone writes against the same database; storage-layer checks cannot
// explain themselves to a caller. Doing both means a violation is refused twice and
// diagnosed once.
//
// See SPEC.md for the normative specification.
package scrip
