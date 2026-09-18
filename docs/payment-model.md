# Accepted payment model

There is no enforcement of the winner's payout against an uncooperative loser.
A payout needs every seat's signature.

## Normal settlement

1. Gameplay, deterministic replay, cheat detection and skip agreement take place
   entirely off-chain over the BR gaming wire.
2. Each peer independently verifies the result and proposes the same outcome to
   its own dcrpulse. Shares are gross and must total the funded pot exactly;
   dcrpulse deducts the transaction fee.
3. Each dcrpulse builds the payout, asks its operator to approve it, and signs
   with the wallet it controls. Signatures are exchanged over the wire. The game
   holds no spending key and signs nothing.
4. Once every seat has signed, one bridge broadcasts. That happens on a
   reconcile pass rather than at the moment of approval, so a payout can be
   fully signed for up to thirty seconds before it reaches the network.

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

## What dcrpulse guarantees

These are the bridge's obligations, not the game's. The game holds identity and
log keys only.

- Every deposit script, amount, owner, roster, network and lock is verified
  against the agreed terms before the deposit is offered.
- Outpoints, scripts, derivation data and terms are persisted before a payment
  is offered, so recovery works after a restart, an abandoned invitation, a
  dissolved table, and when every other participant is offline.
- Maturity comes from the chain, including confirmation height and reorgs.
- Refunds are built from the owner's authorization alone. Exact signed bytes are
  persisted before broadcast, and an ambiguous result is reconciled rather than
  retried.
- A confirmed settlement and a refund cannot both spend the same output. At
  refund maturity conflicting spends can race, and nothing promises the intended
  winner's transaction priority.

## What the player is told before funding

**“Payout requires all players to sign. If settlement fails, you can reclaim
your original deposit after its lock, minus fees. Winnings are not
guaranteed.”** Each deposit's unlock status and its recovery action are shown
outside the game screen, in dcrpulse under Gaming then Recovery.

Settlement runs through the SDK and the desktop. The refund path does not: the
game has no refund or reclaim call, and deposits are recovered in dcrpulse under
Gaming then Recovery.

## Current evidence

`internal/protocolcheck/escrow_test.go` exercises every owner at table sizes 2
through 6, rejects another player's key and insufficient sequence, and checks
refund amounts. These are script-engine tests and do not establish wallet
recovery, confirmation age or UI availability.

`simnet/run-financial-authority.sh` runs the whole path against two wallets and
two bridges: bond, seat draw, stake, a played match, a cooperative payout and a
mature unilateral recovery. See [simnet.md](simnet.md).
