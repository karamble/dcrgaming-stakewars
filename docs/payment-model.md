# Accepted payment model

**Approved by the project owner on 2026-09-16.** This supersedes earlier release
requirements for enforcing the winner's payout against an uncooperative loser.
The research remains available; it is not a product implementation dependency.

## Normal settlement

1. Gameplay, deterministic replay, cheat detection and accusation/skip agreement
   take place entirely off-chain over the BR gaming wire.
2. Each peer independently verifies the result and derives the same payout
   transaction from the agreed table terms, deposits, destinations and allocation.
3. All participants sign every required input, including eliminated players and
   zero-payout players. Signatures are checked and persisted before broadcast.
4. Once complete, any holder of the fully signed transaction can broadcast it.
   Signers need not remain online after supplying their signatures.

No game action, shot, heartbeat or accusation requires an on-chain transaction.
Off-chain votes never substitute for a missing escrow transaction signature.

## If someone refuses or disappears

There is no implemented guarantee that a winner can collect winnings without
all required settlement signatures. That is an accepted limitation of this
release model, not a claim that no cryptographic solution could ever exist.

Every participant must have an independent, owner-authorized route to recover
its own **unspent original deposit**, after the agreed relative lock, less the
refund transaction fee. This requires neither opponents' signatures nor their
continued participation. Refunds do not distribute the winner's pot, reimburse
opportunity costs or pay the latest off-chain balance. A refusing loser may
therefore recover its original stake instead of paying the loss.

Stake, admission bond and any other deposit must be listed separately with its
own amount, script, owner and unlock condition. Do not assume a stake refund
also releases a bond. New forfeiture/accusation machinery is outside this decision
and must not silently defeat the promised independent recovery route.

## Required before funding is enabled

- Verify every actual deposit script, amount, owner, roster, network and lock
  against the agreed terms. Do not fund a script without the intended owner exit.
- Durably retain outpoints, scripts/derivation data, terms and the information
  needed to recover the signing key before losing the in-memory session.
- Recovery must work after restart, an abandoned invitation, a dissolved table,
  and when every other participant is offline. A restore must expose deposits
  even when there is no active match.
- Determine maturity from the authoritative chain, including confirmation height
  and reorgs. A script-engine check alone does not establish input age.
- Build refunds using only the owner's authorization, estimate fees and dust,
  persist exact signed bytes before broadcast, and reconcile ambiguous results.
- Track already-spent deposits. A confirmed settlement and a refund cannot both
  spend the same output. At refund maturity, conflicting spends can race; no
  interface should promise that the intended winner's transaction has priority.
- Show the accepted risk before funding: **“Payout requires all players to sign.
  If settlement fails, you can reclaim your original deposit after its lock,
  minus fees. Winnings are not guaranteed.”** Show each deposit's unlock status
  and a recovery action independent of the game screen.

The first playable build integrates these cooperative settlement and refund
paths through the SDK and desktop. Local mTLS integration checks use simulated
funds; live wallet/BR validation and independent security review remain
outstanding. See [first-playable.md](first-playable.md) for current limits.

## Current evidence

The SDK script tests cover cooperative payouts and owner-only stake refunds.
`internal/protocolcheck/escrow_test.go` exercises every owner at table sizes
2 through 6, rejects another player's key and insufficient sequence, and checks
refund amounts. These are synthetic script-engine tests; they do not establish
wallet recovery, actual confirmation age or UI availability.

The Poker source audit separately exercises contextual node maturity and
illustrates why its bond ladder cannot guarantee winner settlement. See
[poker-exit-audit.md](poker-exit-audit.md). Historical research requirements and
proposed on-chain computation are superseded by this decision as product scope.
