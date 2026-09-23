# Decred settlement experiments

Offline cryptography research using the local dcrd script engine. All private
keys are public test constants. **Do not fund these scripts.** Nothing here is
connected to the gaming bridge or enabled in production.

From the StakeWars repository root:

```sh
make crypto-check
make crypto-policy-check
```

The first command runs adversarial tests with the race detector and `go vet`.
The second generates synthetic fixtures and adds tests to dcrd using a Go source
overlay. It does not edit dcrd. It checks stock mempool admission, fees and input
maturity with mainnet parameters and dcrd's existing fake-chain harness. It does
not broadcast, mine blocks or connect to a wallet. Temporary fixtures are kept
in the directory printed by the command.

The nested module uses the sibling layout `github.com/karamble/dcrgaming-stakewars`
and `github.com/decred/dcrd`. It is separate from StakeWars' application module.
The tested dcrd checkout is `b9634e01`; dependency replacements explicitly use
its txscript, wire and secp256k1 modules.

## Experiments

| Directory | What it establishes | What it does not establish |
|---|---|---|
| `fixed` | Exact output amount/destination enforced by CAT and a synthetic Decred Schnorr signature | Player authorization or gameplay |
| `recursive` | Current script authentication, enforced successor state, alternating real actor signatures, CSV timeout branch | A calculated game winner; cooperative recipient is predetermined |
| `takegame` | Choice-dependent winner, empty-turn timeout, bounded draw, exact payout without loser settlement signature | Irreversible off-chain history or StakeWars physics verification |
| `takegame/checkpoint.go` | Newer fully endorsed checkpoint replaces a stale claim, then resumes without the loser | Arbitrary state-hash authorization; only four predefined snapshots |
| `wots` | RFC WOTS+ arbitrary-digest reference signatures and real Script chain steps | Whole-signature covenant, persistent one-time keys or checkpoint protocol |
| `wotsserial` | Fifteen linked pot-constrained WOTS+ steps; two pass stock mempool admission | Full signer authentication, completion, abort or payout |
| `memoryproof` | Merkle-authenticated memory read and rejection of a false fixed arithmetic result | Full VM, authoritative execution trace or complete dispute escrow |
| `claimmodel` | Bounded abstract verification attempts, plus reproduced starvation attacks | Script enforcement, chain inclusion or affordable worst-case fees |
| `policy/testdata` | Stock contextual mempool tests injected through the source overlay | Live-network propagation, actual block construction or reorg behavior |

The take game starts with three items. A and B alternate taking one or two;
taking the last pays that player. A CSV delay of two blocks permits anyone to
skip without changing the pile. Turn eight without a winner divides the
remaining amount equally. These are **toy rules**, not new StakeWars rules.
The correct initial escrow is 1,020,000 atoms; each transition uses 20,000 atoms
for fees. Funding must verify the exact agreed script and amount. Script itself
does not authenticate transaction witness `ValueIn`.

## Evidence

Tests cover 128 valid state/action combinations, linked transitions, real
winner payouts, empty-signature skips, fresh-signature output redirection,
forged state/code, incorrect actor and sighash type, noncanonical actions,
expiry, self-loops and manipulated draws. Contextual tests reject a two-block
timeout at input age one and accept it at age two. A separate test confirms
expiry-parent outputs require 256-block maturity on mainnet.

Measured signature-script sizes are 334 bytes for the fixed constraint,
approximately 1,242 for recursive progression, and 1,411–1,468 for the take
game. The tested transactions meet the stock 1,650-byte input-script limit and
default relay fee. Sizes can vary by an ECDSA signature byte.

**A passing rollback counterexample is intentionally retained:** B can sign
take-one, observe A's winning continuation, then sign an alternative earlier
take-two that pays B. Both B transactions are legal spends of the same output.
The first confirmed branch is authoritative; earlier BR acceptance alone does
not revoke the alternative. Never change this test to imply off-chain finality.

## Bounded checkpoint recovery

Each player commits independent secrets for four exact snapshots. Both players'
openings certify a snapshot independently of the spending outpoint. Funding
enters a fresh claim; strictly newer certificates replace it; CSV maturity
permits exit into the exact game state. A linked test resumes after B's endorsed
move and lets A finish and receive the pot without another B signature.

**Required conditions:** every funder must hold the complete initial certificate
before funding; the honest watcher must confirm a newer claim before stale exit
maturity; and only a complete certificate establishes off-chain endorsement.
Tests demonstrate that missing the initial certificate can lock funds, and that
a mature stale exit submitted first blocks a conflicting newer update.

This is four-snapshot research, not general StakeWars checkpoint verification.
The claim script is 462 bytes; update/exit signature scripts are approximately
1,439/1,375 bytes. Funding is 1,060,000 atoms. A fixed fee schedule ensures every
route to a snapshot funds the same resumed game amount; jumping over snapshots
spends the corresponding reserve as fees. This is not an SDK fee policy.

The policy suite also admits 102 ordinary parent-first transactions without a
new block. That is dependency-policy evidence, not a cryptographic verifier or
same-block mining demonstration.

These constructions are not used by the game. Payouts are cooperative.

## Practical Poker baseline

`make poker-exit-check` runs the exit audit against the sibling Poker scripts
and a stock dcrd mempool. It reproduces the payout veto and a bond answer that
redirects to the owner wallet.

## Real simulation arithmetic

The adjacent [flightproof experiment](../flightproof/README.md) compares a
bazooka gravity/clamp fraud predicate against actual simulation results. Run
`make flightproof-check`; `make crypto-policy-check` includes its node-admission
fixture. This operation is not yet bound to a canonical game-state proof.
