# Contributing to Scrip

Scrip specifies controls on the issuance of securities and other real-world assets. A
change that weakens a control has consequences that are not recoverable by a later patch,
so the bar for changes to the normative sections is deliberately high.

## Before opening a pull request

**Changes to §3 (the five properties) and §5 (anchoring)** need a rationale that answers
three questions:

1. What failure does the current requirement prevent?
2. Does the proposed change still prevent it, or does it accept the failure?
3. If it accepts the failure, what makes that acceptable?

"It is difficult to implement" is a legitimate answer to (3) only alongside a demonstration
that the difficulty is inherent rather than incidental to one architecture. Most
implementation difficulty in this area comes from putting a control in application code
where it belongs in the storage layer.

**Changes to §6 (asset class profiles)** need the seven dimensions filled in, plus the two
determining questions from §6.4 answered explicitly.

**New profiles** are welcome and are the most useful contribution right now. Treasuries,
private credit, carbon credits, art and collectibles, and equipment leasing all have
requirements the three existing profiles do not express.

## What a good profile contribution looks like

The value is in the **additional requirements** section, not the table. The table is
mostly mechanical. The requirements are where you record what an implementation built for
a different asset class gets wrong — the way gold's proof-of-reserve and real estate's
*lex situs* are missed by anything shaped like equity.

If your profile has no additional requirements, it is probably an existing profile with
different words.

## Failure modes are the documentation

Every normative requirement in SPEC.md is paired with the failure it prevents. This is not
a stylistic preference. A control whose failure mode is not written down gets removed by
the next person who finds it inconvenient, and they will be right to, because nothing told
them what it was for.

Keep this pairing in anything you add.

## Honesty about status

The specification currently over-describes the reference implementation in one important
respect: anchoring (§5) is normative and unimplemented, which means nothing is
Scrip-conformant today, including the reference implementation. This is stated plainly in
several places.

Do not remove those statements as part of an unrelated change. If you close the gap,
change them by demonstrating it.

## Scope

Contributions that expand Scrip into token mechanics, custody, trading, or DeFi
composability will be declined. Those are well served elsewhere, and a specification that
grows to cover everything specifies nothing. See SPEC.md §1.

## Licence

By contributing you agree your contribution is licensed under Apache 2.0, including the
patent grant in §3 of that licence.
