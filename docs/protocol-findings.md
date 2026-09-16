# Protocol characterization — first milestone

> Scope update (2026-09-16): [cooperative payouts and independent refunds](payment-model.md)
> are the approved model. The refusal counterexamples below document an accepted
> limitation, not a requirement to add on-chain game verification.

This is an offline test report, not a mainnet security audit. It records what
the current SDK and the proposed timeout behavior actually establish.

## Existing payout machinery

The tests call the SDK's `BuildSettlement`, `SignSettlement`,
`FinishSettlement`, `CheckSettleDraft` and `BuildTimelockedSpend`. Finalization
runs Decred's actual script engine against synthetic deposits.

| Scenario | Expected result |
|---|---|
| All 2–6 seats sign a winner-takes-all settlement | Valid, winner receives pot less fee |
| Eliminated player receives zero but never signs | Settlement cannot finalize |
| Survivor duplicates its signature to stand in for absent seat | Script validation rejects it |
| Three-seat custom allocation | Conserves gross pot, deducts fee deterministically |
| Another destination or allocation is proposed | Local draft verification rejects it |
| Caller deducts fees before passing shares | Builder rejects the inconsistent total |
| Owner signs the refund branch with sufficient sequence | Script-valid refund |
| Wrong owner or insufficient sequence | Script validation rejects it |

Refund tests do not verify deposit age: actual maturity requires chain data.
The absence test concerns a player who has not already supplied the necessary
signature. A previously collected signature remains usable after disconnect.

**Conclusion:** reuse the established payout implementation. The remaining
question is how the correct settlement becomes spendable when a required
signer never authorized it. Killing their characters or voting to skip their
turn does not remove their key from the funded script.

## Timed skip experiment

`internal/protocolcheck` models the requested behavior using synthetic time and
signed proposals, votes and answers. Signatures are standard Decred-compatible
secp256k1 Schnorr, with explicit experiment-only domain tags. They are not
forfeitable log signatures and do not expose a key on conflicting votes.

The signature binds the match, turn, agreed state head, accused seat and active
roster. Only roster keys authenticate messages; mapping a BR sender identity to
a seat is deliberately outside this experiment.

Explicit harness conventions (not approved production wire/timing defaults):

- A proposal can open only after countdown plus delivery grace.
- Every observer opens a local response window when it receives that proposal.
- The proposer casts a separate explicit vote. Zero votes never authorize a skip.
- Nonresponders abstain. Any observed negative vote prevents a skip.
- An observed accused answer cancels the proposal.
- The window includes its opening instant and excludes its closing instant.
- Authenticated messages arriving after local closure cannot rewrite that
  observer's outcome.
- Conflicting signed votes retain both statements and block the local decision.

### Counterexample: delayed dissent

Three active seats: accused **0**, observer/proposer **1**, observer **2**.
Both observers hold the same signed proposal and seat 1's affirmative vote.
Seat 2 signs a negative vote. Seat 1 receives that vote after its deadline.

| Delivery | Observer 1 | Observer 2 |
|---|---|---|
| Affirmative vote before close | Received | Received |
| Negative vote before close | Not received | Received |
| Local outcome at close | Skip | Do not skip |

No forged signature or double-signing is needed. The same divergence occurs
when the accused's authentic answer reaches only one observer before closure.
Increasing a finite grace period alone does not rule out a longer delay.

**Conclusion:** this models the requested user experience, but it is not a safe
finality protocol for paid outcomes. The counterexample tests intentionally
pass when they reproduce these disagreements. They must not be changed to
silently count unseen votes or assume a shared wall clock.

## Required follow-up

| Item | Next evidence needed |
|---|---|
| Skip finality | A construction or explicit network assumptions that prevent conflicting final continuations while satisfying the required liveness behavior |
| Absent-player payout | Outcome authorization that remains available without a fresh absent-player signature, without giving survivors arbitrary spending power |
| More than two players | Runtime extensions and adversarial tests for accusation and forfeitable-bond paths, not merely six-member escrow scripts |
| Bond admission | Trace and test every join/roster path for on-chain existence, amount, script and confirmation checks |
| Forfeitable amount | Reconcile `FundForfeitBond` using `BondAtoms` with receivers honoring `ForfeitBondAtoms` |
| Replay/signing persistence | Persist-before-send signing journal and crash/restart tests; never sign different state at a used forfeitable position |
| Match formation | Bind content/rules/payout terms before deposits; resolve UID/session-key binding and seating-beacon reorg handling |
| Randomness | Evaluate future-event predictability and last-contributor/anchor selection, not just deterministic seed derivation |

The skip harness does not implement production wire serialization, bridge
integration, persistence, chain consensus, six-player bond extensions, or a
solution to these counterexamples. Separate experimental turn-codec and payout
packages are tracked in [implementation status](implementation-status.md).
Paid release remains disabled.

## Covenant research update (2026-09-16)

An isolated experiment now demonstrates output constraints and recursive state
transitions using existing Decred CAT/Schnorr operations. A small take-one-or-two
game verifies its winner and pays without a loser settlement signature. Stock
mempool tests with synthetic UTXOs check standardness, fees and timeout maturity.

Additional isolated probes execute real WOTS+ signature chain steps, check a
Merkle memory proof for one fixed arithmetic instruction, and model bounded
claim attempts against repeated aborts. They do not yet compose into a complete
or affordable settlement protocol.

This does not resolve StakeWars' asynchronous finality requirement. A retained
counterexample shows an actor replacing an earlier off-chain move with another
legal move before confirmation, changing the winner. The current SDK escrow
and release gate are unchanged. See [the cryptography research report](settlement-cryptography-research.md)
and run `make crypto-check` / `make crypto-policy-check` to reproduce the bounded
results.

## Poker loser-refusal tests (2026-09-16)

The [exit audit](poker-exit-audit.md) now reproduces refusal against Poker's actual
escrow code. Its claimed-bond owner can redirect an answer into a wallet output,
breaking the pre-signed ladder; stock mempool checks accept that spend. A second
refuser at a six-player table also prevents collection of an unanswered bond.
Run `make poker-exit-check` for script and contextual maturity tests. This
confirms that the existing ladder does not enforce StakeWars winner payouts.

## Poker settlement source recheck (2026-09-15)

Inspected the local `../dcrpoker` checkout, especially
`pkg/escrow/script.go`, `pkg/gamelog/checkpoint.go`, and
`cmd/pokerplugin/settle.go` (`settleDraft`, `signSettlement`,
`adoptSettlement`, `broadcastSettlement`).

- The stake settlement branch is **N-of-N**, including members paid zero.
  It uses one Schnorr `OP_CHECKSIGALTVERIFY` per member, not N−1 voting.
- Poker derives balances from mutually signed hand-boundary checkpoints.
  Those log-key signatures are not transaction signatures. Actual settlement
  signatures come from the session keys named by the deposit scripts.
- The plugin builds its own draft from the agreed stacks, funded outpoints and
  payout destinations; `CheckSettleDraft` rejects another payout before signing.
- `broadcastSettlement` returns early unless every member has supplied a
  transaction signature for every input. It then calls `FinishSettlement`, which
  validates the script before broadcasting. A received signature remains useful
  after its author disconnects; a missing signature cannot be replaced by a vote.
- Some source comments and the rejected-alternatives section mention adaptor
  signatures or pre-signatures for all outcomes. The current settlement section
  explicitly says it uses checkpoints, and the plugin path above signs the
  settlement transaction once the table is over. The earlier wording must not
  be treated as evidence of an implemented winner-dependent N−1 spend.

StakeWars can reuse this cooperative transaction machinery. It cannot infer that
an unsigned final result becomes payable merely because a gameplay majority—or
all currently responding peers—agrees. Poker's option to void a hand and use an
older boundary also does not establish StakeWars' requirement that abandoning
a losing game cannot undo the loss.

New characterization verifies that collected SDK transaction signatures remain
sufficient without retaining private keys and fail if the payout is changed.
The existing 2–6-seat missing-signer and duplicate-signature rejection tests and
asynchronous skip divergence counterexamples remain unchanged.

## Recovery and funding groundwork

`internal/turnbatch.Journal` persists validated turns and unsigned signing intent
before returning a signature. It supports exact retransmission, bounded future
turn buffering, duplicate handling, independent replay after restart and explicit
sync points. Its signatures remain experimental ordinary Schnorr, not the SDK's
forfeitable log signature scheme. It supplies no skip or payout finality.

`internal/funding` persists one immutable approval/payment request intent per
stable request ID. Only the first successful creation authorizes dispatch; an
identical retry is unresolved and must be reconciled through the bridge/chain.
A changed amount, purpose, terms, script, seat or match under the same request ID
is refused. Caller-generated new IDs must not bypass this obligation mapping.
This package is not a chain verifier and has no wallet or bridge calls. Recovery
currently errs on the side of holding an uncertain payment for reconciliation.

Both journals require trusted local storage supporting hard links and directory
fsync; unsupported filesystems fail closed. They do not protect against malicious
local state rollback, deletion, compromised keys or a caller using the raw fixture
signer instead. Desktop/bridge integration, obligation-to-request ID derivation,
receipt reconciliation, and crash injection at every filesystem boundary remain
release work. Real-money matches remain disabled.

The Poker recheck also ran the local checkout's targeted tests successfully:
`TestASettlementNeedsEverySeat`, `TestATablePaysOutWhatItEndedHolding`, and
`TestASeatChecksASettlementBeforeSigningIt` in `pkg/escrow` (no Poker source edits).
