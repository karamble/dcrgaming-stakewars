# Project continuity

## Existing mainnet bond

Before changing financial recovery or user profiles, read
[the non-secret legacy recovery note](docs/legacy-bond-recovery.md).
The local Omarchy/player2 admission bond is 0.01 DCR; its 0.001 DCR stake was
never funded. Verified recovery material is backed up outside the repository.
Never print, commit, or copy its private key/seed into chat or assistant memory.
Do not confuse this with JACKIN/player1's unpaid attempt.

## Financial authority work

The game is untrusted. dcrpulse must control spending keys, independently verify
financial proposals, persist recovery data before offering a payment, and require
dashboard approval for financial operations. See
[implementation status](docs/bridge-financial-authority-implementation.md).

No backward compatibility or legacy migration code is wanted. Recover the single
old bond manually from its separate backup; keep that exception out of the new
SDK/bridge financial protocol and application UI.

## Latest handoff — 2026-09-16

Read [the dated implementation status and upcoming work](docs/bridge-financial-authority-implementation.md)
before continuing. Most bridge/SDK/game integration is staged under
`/tmp/stakewars-recovery-work/{sdk,pulse/dashboard,game}` and is not deployed.
Two- and six-player simulated flows pass; full suites and real two-wallet simnet
acceptance are unfinished. Preserve the staging trees and distinguish tested
source from installed source. The status document records remaining failures,
review boundaries, and the next work in order.
