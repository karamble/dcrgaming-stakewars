# Bridge financial authority implementation

Status: **in progress**, updated 2026-09-17. The full system is not finished.
No live wallet or deposit was changed during the 2026-09-17 wire implementation.

## 2026-09-17 one-shot BR wire implementation

The tested changes are now installed in the actual `dcrgaming-sdk`, dcrpulse
`gaming-standalone`, and dcrstakewars working trees. They are not committed or
deployed yet.

- Gaming envelope framing is version 2 with a stable full BLAKE-256 message ID.
- Deployed brclientd `gc-message` notifications now enter the gaming bridge;
  protocol frames are withheld from browser chat.
- dcrpulse persists inbound gameplay frames before delivery, deduplicates exact
  BR-history repeats, replays after the SDK cursor, rebuilds from local BR group
  history after a notification gap, and closes a stalled stream for replay
  instead of dropping a frame.
- Bridge-reserved financial frames use the same durable inbox but are replayed
  only to the financial worker and never exposed to the untrusted game. The old
  authority `want` request/reply flag was removed; participant and roster state
  publish only on their own transitions.
- dcrpulse durably claims each outgoing `(game, gcid, mid, part)` before its
  only BR send attempt. SDK retries and bridge restarts cannot republish it.
- SDK peer resync/request/reply messages and StakeWars `w.sync` were deleted.
- Generic formation kinds now use the reserved `table.*` and `finance.*`
  namespaces. A game cannot publish those through `Runtime.Send`.
- Generic table/finance events and StakeWars world/turn transitions use
  `exp=0`; protocol reducers decide whether replayed state still applies.
- A repeated two-seat test proves exactly two seat, two roster-prepare and two
  roster-commit messages. Funding adds exactly two stake and two payout
  destination records. Seat draw and idle chain ticks add zero messages.

Verified after installation: full dcrgaming-sdk tests, dcrpulse
`internal/gamingbridge` and `internal/services`, and all StakeWars packages.
This does not complete the real two-wallet acceptance gate or authorize a
real-money test.

## Required trust boundary

The game and SDK are untrusted clients of dcrpulse. Only dcrpulse may retain
spending keys, independently validate financial proposals, ask the operator for
approval, sign, and publish. Game identity/log keys remain in the game and may
never authorize spending. A game-reported outcome is a payout proposal, not
financial authorization. The bridge does not adjudicate gameplay.

Before offering an approval the bridge must persist the complete immutable
deposit descriptor, independently rebuild the allowlisted script, prove it owns
the recovery authority, construct and inspect the funding transaction, and bind
the request to the authenticated game, network, wallet fingerprint, and account.
Address-only funding and unrestricted game transaction broadcasting must fail
closed. Approval binds exact bytes, amount, fee, and destinations; signing and
publishing recheck current chain state. Unknown templates fail closed.

The financial ledger survives failed seating, closed tables, a stopped game,
bridge restarts, ambiguous broadcasts, and reorganizations. Recovery is operated
from Gaming → Recovery. Active tables require local closure before recovery;
closure is not permission to forget records or revoke already-issued signatures.

## Existing funded output

The local Omarchy/player2 admission bond is legacy, 0.01 DCR, with its original
2016-block lock. Outpoint:
`cdaf64d5cb5e1e8eb382366e753596bd12acda9a63de302ce2f525a5d8497b31:0`.
Preserve the original SDK profile and signing seed. Do not pretend the bridge
controls this existing output, migrate its script, or issue a test mainnet refund.
JACKIN's unpaid attempt must not be shown as recoverable funds.

## Implementation work

Development copies for cross-repository changes:

- SDK: `/tmp/stakewars-recovery-work/sdk`
- dcrpulse: `/tmp/stakewars-recovery-work/pulse/dashboard`
- StakeWars: `/tmp/stakewars-recovery-work/game`

These are editable staging trees. **Most integration changes remain staged**;
they are not automatically installed in the sibling repositories, built into
running applications, or deployed. Preserve these trees before clearing `/tmp`.
The staged game was copied as Go source only: assets and non-Go test fixtures
are missing. A missing fixture there does not establish a product regression.

### Installed source changes

- Shared SDK public financial contracts (`pkg/finance`) and version-2 invitation
  schema. These contracts do not hold wallet keys or authorize spending.
- An earlier iteration of the dcrpulse financial ledger and cooperative payout
  core, plus its SDK dependency. Latest service/protocol integration is staged.
- StakeWars invitation/seating plumbing for explicit admission bond terms and
  messages directing recovery to the dcrpulse dashboard.
- SDK payout input keys are normalized before sorting; equivalent hexadecimal
  encodings produce identical transaction bytes. Corresponding regression tests
  were installed. No further production-source installation occurred in the
  latest continuation; only these project notes were updated in the game tree.

### Staged integration and latest changes

- Wallet-owned financial keys, authenticated financial roster commitments,
  descriptor-bound funding requests, exact transaction validation, and durable
  funding/payout/refund records. Bridge approval/recovery/payout handlers and
  dashboard components, protocol version 2, and transaction reconciliation.
- SDK settlement only proposes a payout for dashboard approval. Old financial
  claim/ladder/sweep/signing libraries were retired from staging. `pkg/forfeit`
  remains because game-log proofs still use its off-chain cryptography.
- Removed the runtime's identity-wide bond option and `BondScope` configuration.
  Admission is always table-specific, using explicit invitation amounts/delays.
  Removed obsolete runtime table/forfeit bond state. Updated staged StakeWars
  controller to the new SDK configuration. Runtime package builds, but its old
  tests still reference removed APIs and require deliberate migration.
- Fixed table-store version checking so version-2 records survive restart.
  Formation wire terms now carry admission amount and refund delay.
- Financial BR messages use a bounded queue instead of blocking chat delivery.
  Duplicate routing fields, unknown financial JSON fields, and trailing JSON
  are rejected. Periodic gossip repairs missed financial messages.
- Duplicate settlement signatures no longer reset a confirmed operation.
  Closed tables refuse new payout assembly, while already-issued signatures
  remain recorded. Reorganization observations clear spent state appropriately.
- Ledger initialization marker is synced to disk and its directory.
- Gaming payment checks reject unknown, missing, null, malformed, or watch-only
  wallet metadata rather than treating it as spendable. Imported accounts are
  excluded. This uses cached wallet-loader metadata plus account/key checks;
  a live wallet integration test is still required.
- Ledger rejects two simultaneous funding previews for the same deposit, even
  with distinct wallet inputs. Funding commit rejects trailing transaction bytes.
- Approval UI shows verified table, deposit type, funding fee, total debit and
  refund delay. Latest staged dashboard TypeScript/Vite build passed.
- Game bridge config supports explicit `mainnet`, `testnet3`, or `simnet` and
  rejects mismatched Hello networks. Session controller selects chain parameters
  from the authenticated bridge. A settings UI network selector is still needed.
- Replaced game protocol-check tests for retired game-side spending APIs with
  public finance/ECDSA script-engine tests: 2–6-player winner payouts, absent loser,
  forged substitute signature, changed amounts/destinations, owner refunds,
  insufficient sequence, and reuse of previously collected signatures.

### Latest verified results

- Staged SDK `go build ./...`: **passed**, including removal of identity-wide bonds.
- SDK race tests previously passed for membership, schema, escrow, finance,
  bridgetest, transport and spend; connect tests passed. Full SDK suite is still
  failing, including runtime tests importing retired `pkg/punish` and outdated
  protobuf contract expectations. Do not restore old APIs to satisfy those tests.
- Staged game race test of `TestCooperativeMatch` (2 and 6 players) and
  `TestRecoveryRemainsBridgeOwnedAfterRestart`: **passed**, 49.759 s, after latest
  runtime cleanup. Includes explicit payout approval and restart handling.
  **This is an in-process simulated bridge, not independent real wallets.**
- Game `internal/protocolcheck` and `internal/bridgeconn` race tests: **passed**
  after the latest fixture and ECDSA test migration.
- `internal/seating` race run: **failed** only at
  `TestInvitationControlsStakeSeatsAndDeadline` in `profile_test.go:14`, whose
  fixture still omits required explicit admission terms. Pending-admission
  restart tests did not report failures in that run.
- Earlier full game internal run: failed on old escrow APIs, invitation fixtures,
  removed BondScope, and missing staged `pkg/replay/testdata/six-squads.json`.
  Several failures were subsequently fixed as above; full run not repeated.
- Latest targeted bridge race run passed `internal/gamingfunds` and
  `internal/services`, including signing-metadata refusal, duplicate funding
  approval, malformed funding commit and payout regressions. Earlier broader
  gamingfunds race checks and service/RPC/handler compile checks passed.
- Full dcrpulse suite is **not green**. Old spend/invite/RPC tests use the retired
  contract; a previous full service run timed out after 10 minutes. Use explicit
  shorter timeouts and fix fixtures before repeating that run.

### Upcoming work, in order

1. Finish test/API migration. Fix the explicit-terms fixture in game seating;
   copy required non-Go test fixtures/assets into game staging. Migrate SDK
   runtime lifecycle/wire/restart tests while preserving useful coverage;
   replace tests for intentionally removed spending APIs with bridge-authority
   assertions. Remove remaining obsolete handlers/protobuf messages/capabilities,
   regenerate both protocol copies together, and update contract pins only after
   verifying they match. Update SDK examples/docs to the new generic interface.
2. Finish authoritative approval persistence. Funding previews live in the ledger
   but the pending dashboard request still lives in a separate spend log. Handle
   a crash between those writes; recover pending approvals without duplicate
   charges. Make descriptor-based retries recover the original request even
   after funding or response loss. Resolve expired/rejected payout retry behavior.
3. Finish reconciliation/security integration: fresh wallet ownership/signability
   checks; external spends, mempool conflicts and reorgs; portable chain lookup
   without assuming transaction indexing; backup/restore checks; worker shutdown
   cancellation (currently starts with a background context). Review payout fee
   policy across supported player counts and unknown ledger fields.
4. Add network selection to game settings. Complete SDK financial-state refresh
   after bridge closure/recovery. Review exact approval details and availability
   in the dashboard with the game disconnected.
5. Review and install a consistent change set into all three actual repositories,
   including intentional file removals. Preserve unrelated pre-existing changes;
   do not blindly overlay staging trees. Run full Go tests/race checks and builds,
   plus the dashboard production build. No binaries have been deployed yet.
6. Run **two independent wallets, bridges and games on simnet**: seating approval,
   confirmation, identical game world/replay, exact cooperative payout approvals,
   expired seating, unilateral mature refunds, restart, ambiguous publication and
   reorg handling. The simulated harness cannot satisfy this acceptance gate.

### Continuation artifacts and review boundaries

- `/tmp/implement-gaming-authority.py` is a scoped source installer; inspect it
  before use. It does not handle retired-file deletion.
- `/tmp/stakewars-sdk-before-financial-retirement.tar.gz` backs up staging SDK
  before the reviewed retirement patch. Exact patch and manifest:
  `/tmp/stakewars-financial-retirement.patch` and
  `/tmp/stakewars-financial-retirement-review.json`.
- Automatic review initially rejected a broad retirement command; a later exact,
  backed-up patch was approved and applied **to staging only**.
- A separate proposed membership hash rewrite was rejected and **not applied**.
  Production terms hash remains v1 with its existing bonded encoding; obsolete
  `AccuseFeeAtoms` / `ForfeitBondAtoms` fields remain in membership terms. Do not
  mistake the updated tests for an approved hash-format change, or silently retry
  the rejected rewrite. Any necessary change needs its own concrete review.
- Latest continuation scripts: `/tmp/stakewars-table-bonds.py` (already applied;
  not idempotent), and staged new tests
  `internal/gamingfunds/funding_test.go`,
  `internal/services/gaming_wallet_authority_test.go` in dcrpulse.
- Logs: `/tmp/stakewars-game-internal-current.log`,
  `/tmp/stakewars-authority-web-build.log`,
  `/tmp/stakewars-authority-pulse-tests.log`. Some logs predate latest fixes;
  use the verified results above rather than assuming every log is current.

### Completion gate

The new system is **not ready for a real-money play test**. The original local
bond recovery is separate, manual work using its preserved backup; nothing in
this implementation changes that output or moves its funds.

## Validation requirements

Exercise missing/foreign keys, wrong account/network/wallet, malformed and altered
scripts, duplicate obligations, replayed approvals, insufficient fees, changed
outputs, corrupt storage, publishing timeouts, conflicting payouts/refunds,
script-engine execution and maturity boundaries. Never infer spend authorization
from the number or size of signature-shaped stack pushes.

## Explicit scope correction — no compatibility

The operator explicitly rejected backward compatibility and legacy migration.
Do not add old-format RPC shims, legacy ledger import/reporting, key migration,
or an optional game-held spending mode. The new financial protocol requires
bridge authority. Existing application payment records must not be upgraded into
verified descriptors by assumption. The one real bond is recovered manually
using the separately verified backup and non-secret recovery note.

Removed from staging: reported-legacy-deposit protocol fields, legacy ledger flag,
legacy recovery UI/state handling, the old bridge payment callback, and the SDK's
optional bridge-authority mode and old-funding fallback.
