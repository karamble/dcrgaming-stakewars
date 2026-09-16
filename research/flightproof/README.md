# Bazooka gravity operation: executable fraud branch

Research only. **Do not fund this script.** This is one arithmetic operation and
a fixed-beneficiary fraud branch, not a complete challenge contract or game rule
verifier. It has no honest-claim exit/refund branch.

## Operation taken from the real game

[`sim.moveProjectiles`](../../pkg/sim/physics.go) adds gravity to a bazooka's
vertical velocity, then clamps it to the flight-speed limit. Q16.16 arithmetic
represents one pixel as 65,536 integer units. For the tested input domain:

```text
-2,097,152 <= oldVY <= 2,097,152
newVY = min(max(oldVY + 4,096, -2,097,152), 2,097,152)
```

The test calls the real exported `sim.Step` in a controlled empty arena, with a
live, non-burning bazooka projectile whose fuse does not expire. It extracts the
resulting velocity and compares the Script fraud predicate against it. Forty-five
inputs include fractional/negative values, zero and both clamp boundaries.

For each input, the actual result cannot satisfy the fraud branch; claimed
results one integer unit above and below it can. Other tests reject changed
input commitments, unsupported input ranges, output redirection, extra inputs or
outputs, expiry, forged witness data and nonfinal sequences. Transaction changes
are tested with freshly rebuilt witnesses, not only stale signatures.

## Script and transaction

The redeem script commits to `BLAKE256(ScriptNum(oldVY).Bytes())` and a claimed
result. It authenticates the supplied input, enforces the bounded domain, computes
the operation and requires a different claimed result. The existing CAT/Schnorr
construction then constrains the spending transaction's beneficiary and amount.
Its synthetic signature is not player authentication.

Measured representative fixture:

| Property | Value |
|---|---:|
| Redeem script | 273 bytes |
| Non-push operations | 44 of 255 |
| Signature script | 406 of 1,650 bytes |
| Transaction | 517 bytes |
| Synthetic input | 30,000 atoms |
| Fixed output | 10,000 atoms |
| Fixture fee | 20,000 atoms |
| Stock default minimum relay fee for this transaction | 5,170 atoms |

The stock dcrd mempool harness accepts the transaction with a synthetic UTXO and
mainnet parameters. This is not a live broadcast or block-mining test. Sizes can
vary slightly with the committed numeric values.

## Precisely what is missing

- The input commitment is to a scalar in ScriptNum encoding. It is **not** a
  Merkle proof from StakeWars' canonical state, nor interchangeable with the
  canonical format's four-byte little-endian signed integers.
- Neither old velocity nor claimed result is yet authenticated to the agreed
  game/dispute state. A future contract must establish those commitments.
- Script does not prove that this instruction is the next legal instruction:
  weapon, alive/burning status, fuse, phase and program position are fixture
  conditions. Their proofs/control flow remain absent.
- Other weapons, wind, position integration, collisions, drilling, explosions,
  terrain changes and damage are not checked by this operation.
- Inputs outside the specified old-velocity range are rejected. This does not
  establish every possible newly spawned projectile meets that precondition.
- No state write-back, multi-instruction execution, bisection contract, timeout,
  multi-party arbitration, key management or actual settlement is implemented.

This shows that **one relevant real-game arithmetic operation fits comfortably**.
It does not bound full dispute cost or solve the uncooperative-loser problem.

## Reproduce

From StakeWars:

```sh
make flightproof-check     # real-simulation comparisons, attacks, race, vet
make crypto-policy-check # stock node admission, plus existing research suite
```

The root-module tests use its pinned txscript dependency; the policy overlay also
executes the resulting fixture against the sibling dcrd checkout. Neither node,
wallet, SDK nor production simulation code is modified.
