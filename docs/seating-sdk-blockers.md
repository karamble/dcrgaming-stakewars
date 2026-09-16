# Seating SDK integration status

## SDK recovery work implemented

The SDK work was authorized on 2026-09-16. Poker and Battleships are unfinished
references; they impose no deployed-state migration or compatibility requirement.

The sibling `dcrgaming-sdk` now provides:

- Durable pending admission before acknowledgement or payment dispatch.
- Pending-table restore, safe resync and owned admission-worker shutdown.
- One funding request per immutable obligation, explicit unpaid retries and
  authoritative reconciliation of a supplied bridge request ID.
- Latched persistence faults, durable atomic writes and exclusive runtime
  ownership of file-backed payment/table stores.
- A journal of exact signed transaction bytes before broadcast and explicit
  retry of those bytes.
- Per-table admission refunds after expiry, even if no table formed.
- Invitation-aware policy resolution without changing advertised terms.
- Detached lifecycle snapshots, deposit verification and subscription status.
- Synchronized formation access and two-, four- and six-peer seating tests.

See the SDK's `docs/seating-recovery.md` for API contracts and lifecycle order.

## StakeWars acceptance check

Run `make seating-check`. `TestSDKPendingAdmissionRecovery` uses the actual
sibling SDK, a local mTLS mock bridge, temporary file-backed stores and simulated
money. It holds bond approval, stops the first runtime, restores the pending
admission, approves that same request, mines confirmations and checks that one
join and one payment request exist. Pending resync and funding guards are checked
as well. A second case simulates a lost request ID, verifies explicit
reconciliation, and checks that no second payment is dispatched. No saved
credentials or real wallet are used.

The old passing characterization of missing persistence and a resync panic has
been replaced by this recovery acceptance test.

## Remaining production integration

StakeWars still refuses paid invitations in its production lobby. Next attach
the runtime to that path, drive the lobby from lifecycle snapshots, verify every
participant's admission/stake/bond outputs, and complete agreement on the
battlefield and initial state before enabling gameplay.

A lost RequestSpend response remains explicitly unresolved until its bridge
request ID is supplied and verified. The current bridge offers no lookup by
client obligation ID; the SDK must not issue a second payment to guess.

Off-chain multiplayer agreement, cooperative N-of-N settlement and end-to-end
owner refunds remain integration work. Enforced payout against a refusing signer
is not required under the approved [payment model](payment-model.md).

## Dcrpulse follow-up

Refresh the Games list when a game connects so New table appears without a
browser reload. Table configuration and chat-chip invitation acceptance remain
in Dcrpulse.
