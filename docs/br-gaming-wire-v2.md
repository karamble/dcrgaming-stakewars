# BR gaming wire v2

Status: wire v2 transport and one-shot table formation are implemented in the
three working repositories as of 2026-09-17. Financial authority integration is
still in progress. This is a breaking replacement; no compatibility or migration
layer exists.

Implemented and tested now:

- stable full BLAKE-256 message IDs and `v=2` framing;
- `gc-message` and `gaming-frame` ingress routing without visible chat traffic;
- dcrpulse persistence before game delivery, exact duplicate suppression,
  reconnect replay, local BR-history recovery and stream restart on backpressure;
- durable bridge-only financial inbox replay; financial frames never enter the
  untrusted game stream;
- durable one-attempt outgoing claims, preventing repeated physical publication;
- one-shot `table.seat`, `table.roster_prepare` and `table.roster_commit` events;
- non-expiring durable framing for table, financial, StakeWars world and turn
  transitions, so local history replay does not discard an old valid event;
- removal of SDK and StakeWars peer resync/request/reply paths;
- removal of the financial authority `want` request/reply flag; authority state
  is announced only when the local participant or roster state changes;
- two-seat tests proving exactly six formation events and zero events from idle
  chain ticks.
- funded two-seat tests proving exactly two additional stake records and two
  payout-destination records before cooperative payout approval.

The Bison Relay carriers remain unchanged:

```text
--gaming[...]--<base64 payload>
--mcp[...]--<base64 payload>
```

Either envelope may travel in a direct message or a group chat. Shared table
records normally use the table group chat so every participant receives the same
announcement. Invitations and explicitly private operations may use direct
messages. The carrier does not decide protocol authority.

## Goals

- Publish each logical state transition to BR once.
- Never publish because a timer, UI refresh, block poll or clean reconnect ran.
- Accept arbitrary asynchronous ordering without discarding valid future state.
- Persist before acknowledging or advancing security-sensitive state.
- Keep games unable to authorize, sign or broadcast financial operations.
- Let every participant independently verify membership, chain deposits,
  deterministic seating and payouts, and let each game verify its own protocol
  state through game-defined events.
- Recover from dcrpulse durable state and BR history without peer recovery
  messages.
- Keep gaming frames out of visible chat while retaining them in BR storage.

## Non-goals

- The wire does not make BR synchronous or globally ordered.
- It does not infer absence from silence.
- It does not put gameplay on chain.
- It does not solve an uncooperative winner-payout signer. Owner refunds remain
  the financial backstop.
- It does not retain the current payload protocol for compatibility.

## Core model

A BR gaming message is immutable and has a stable identity within one table.
There are two message classes:

- standard control events, whose schemas, verification and transitions belong
  to the SDK's generic table protocol;
- opaque application events, whose kinds, bodies, dependencies and transitions
  belong only to the importing game.

A receiver stores a message before dispatching it. Missing prerequisites for an
standard control event make that event pending; they do not make it invalid.
dcrpulse deduplicates raw frames by stable event identity and preserves their BR
sender and group-chat attribution. The SDK verifies and reduces generic table
events, and delivers game events without interpreting their bodies.

The SDK defines logical slots for generic table coordination. dcrpulse defines
and enforces financial obligations and exposes their verified state:

- one seat claim per participant;
- one roster preparation per participant and candidate roster;
- one roster commitment per participant;
- one stake claim per seat;
- one payout signature per participant and payout proposal.

Games may define their own application-level slots under their game and
game-version namespace. Those slots are outside dcrpulse and the SDK.

Receiving the same event again is a no-op. For standard control events, different
events for the same irreversible slot are a conflict. Both are retained and the
affected generic transition stops. Application conflict rules belong to the
game.

## Envelope

The text envelope remains `--gaming[...]--`. Version 2 requires:

```text
--gaming[v=2,game=<game>,gv=<game-version>,sid=<table>,mid=<stable-id>,seq=<part>/<total>,exp=<unix-or-zero>]--<base64>
```

- `game` routes to an installed game.
- `gv` selects that game's payload contract.
- `sid` is the table/session identifier.
- `mid` is derived from the immutable event ID. It is not randomly regenerated
  for retries or restarts.
- `seq` fragments one immutable event. Every fragment must agree on all routing
  fields, total length and content hash.
- `exp=0` is used for durable protocol evidence. Expiration is reserved for
  requests and optional presence hints.

The authenticated BR sender and GC/DM destination are supplied by BR metadata,
never by a payload field. Security-sensitive signatures bind the network,
destination, table, game, game version, message class, kind, author and body.
Standard control signatures also bind their generic logical slot and dependencies.
Applications define any additional signed fields needed by their own protocol.

The full event ID is:

```text
BLAKE256("dcrgaming/event/v2" || canonical unsigned event)
```

The canonical signing encoding will use fixed field order, fixed integer widths,
length-prefixed byte strings and explicit domain tags. JSON may render the body
for inspection, but generic JSON serialization is not a signature preimage.

## Component contract

| Component | Owns |
|---|---|
| BR | `--gaming[...]--` and `--mcp[...]--` delivery through DM or GC and message history |
| dcrpulse | Durable inbox/outbox, BR sender attribution, deduplication, wallet keys, independent financial validation, approvals, signing, broadcasting and refunds |
| dcrgaming-sdk | Generic table formation, deterministic seating, canonical encodings, transport, subscriptions, financial requests/views and verification helpers |
| game | All application payload schemas and all game state/rules |

dcrgaming-sdk is a client library running in the untrusted game process. Its
generic table reducer independently verifies peer statements. It is never the
financial authority and never holds wallet spending keys.

Wire kinds are reserved:

- `table.*`: generic SDK table-control records;
- `finance.*`: generic SDK records that refer to bridge-verified financial state;
- all other game/version-scoped kinds: opaque application records.

The SDK application API cannot publish `table.*` or `finance.*`. dcrpulse does
not interpret game payloads, and the SDK does not interpret application bodies.

The SDK contains no world, battlefield, board, card, move, turn, timer,
readiness, winner or outcome model.

## Delivery contract

### Incoming

1. dcrpulse recognizes `--gaming[` inside both current `gc-message` events and
   any future dedicated `gaming-frame` event.
2. It validates only envelope bounds and routing at ingress.
3. It durably stores the raw event, authenticated BR sender, destination,
   receive time, event ID and processing state before fan-out.
4. Gaming traffic is consumed by the gaming bridge and is not forwarded to the
   browser chat event bus.
5. The local game stream replays records after the SDK's last consumed sequence
   before crossing into live delivery.
6. The SDK reduces known generic kinds and delivers all other kinds plus
   authenticated sender/table metadata to the game.

The local gRPC stream is delivery plumbing, not the source of truth. Reconnecting
replays from dcrpulse's durable inbox and sends no BR messages. A game-process
restart currently replays the retained inbox from its beginning; reducers must
remain idempotent. A persistent explicit ACK remains future hardening.

### Early events

dcrpulse keeps bounded quarantine storage for correctly framed events belonging
to a known invitation/GC that the local operator has not accepted yet. Acceptance
moves matching records into the authorized table inbox. Quarantine cannot
authorize a payment, create an approval request or expose another game's data.

dcrpulse creates its table reducer immediately after durable invitation
acceptance, before the local bond is paid. Peer seat claims can therefore be
stored and verified while the local operator is still approving or confirming
their bond.

### Outgoing

Games call the SDK application send API and cannot select a reserved
control/finance kind. The SDK creates generic table events from verified local
transitions. A finance event may only describe state independently returned by
dcrpulse; it never authorizes dcrpulse to spend.

dcrpulse persists the canonical immutable event before publication:

- a new event ID is durably journaled and submitted to BR once;
- the same event ID returns the prior result without another BR publication;
- a definite pre-enqueue failure may retry the same event;
- an ambiguous enqueue result becomes `submission uncertain` and is reconciled
  against BR's local outgoing record before any retry;
- restarting recovers the original bytes, event ID and fragments.

Strict physical once-only publication across a crash requires an idempotent BR
enqueue key or a durable enqueue receipt. Until BR exposes that, uncertainty is
shown honestly and is never handled by inventing a fresh message ID.

## Generic SDK table and payment contract

The SDK scope is limited to identities, BR wire transport, table bonds, seating,
deterministic seat draw, stakes and payouts. These are game-agnostic services.
The invitation terms supply amounts, player count, confirmation policy and
financial templates; the SDK does not hardcode one game's values.

The game-facing SDK surface is limited to:

```text
Connect(config) -> Client
Hello(gameProtocolVersion, capabilities) -> BridgeIdentity
GetLocalIdentity() -> IdentityView
Subscribe(cursor) -> BridgeEvent stream
RespondTableRequest(requestID, response)
AcknowledgeDelivery(cursor)
GetTable(tableID) -> TableView
GetParticipants(tableID) -> []ParticipantView
RequestFinancialOperation(tableID, obligationID)
ProposePayout(tableID, exactOutputs, evidenceHash)
GetFinancialState(tableID) -> FinancialView
SendApplicationMessage(tableID, protocolVersion, payloadBytes)
SubscribeApplicationMessages(tableID, cursor) -> attributed payload bytes
```

`CreateTable`, `Invite`, `AcceptInvitation`, `ApproveFinancialOperation`,
`ApprovePayout` and `RequestRefund` are dcrpulse dashboard operations. They are
not game-facing SDK methods. No SDK method approves, signs or broadcasts money.

The SDK types are game-neutral:

```text
TableTerms {
  schema_version
  game_id
  game_protocol_version
  network
  participant_count
  admission_deadline
  seat_policy              // join_order | explicit | block_hash_shuffle
  draw_height              // required only for block_hash_shuffle
  draw_confirmations
  bond: FinancialTerms     // amount may be zero
  stake: FinancialTerms    // amount may be zero
  payout_policy            // all_participants_sign
}

FinancialTerms {
  template                 // bridge allowlisted template identifier
  amount_atoms
  confirmations
  refund_delay_blocks
}

ParticipantView {
  authenticated_br_uid
  application_session_key
  bridge_protocol_key
  financial_key_commitment
  bond_state
  seat
  stake_state
}
```

The standard one-shot control records are:

```text
table.seat
table.roster_prepare
table.roster_commit
finance.stake
finance.payout_proposal
finance.payout_signature
finance.payout_published
finance.refund_published
```

| Record | Author | Emit condition | Cardinality |
|---|---|---|---|
| `table.seat` | participant's dcrpulse | accepted terms and required bond outpoint validated | once per participant/table |
| `table.roster_prepare` | participant's dcrpulse | complete candidate participant set | once per participant/candidate hash |
| `table.roster_commit` | participant's dcrpulse | all preparations match | once per participant/table |
| `finance.stake` | stake owner's dcrpulse | exact stake outpoint validated | once per owner/obligation |
| `finance.payout_proposal` | proposer's dcrpulse | exact proposal passes bridge validation | once per proposal hash |
| `finance.payout_signature` | signer's dcrpulse | exact proposal approved in dashboard | once per signer/proposal hash |
| `finance.payout_published` | designated dcrpulse | signed transaction broadcast | once per proposal hash |
| `finance.refund_published` | deposit owner's dcrpulse | mature refund approved and broadcast | once per obligation |

Seat draw is derived locally from the committed roster, immutable seat policy
and selected block hash. Confirmation changes are local chain observations.
Neither produces periodic BR traffic.

### 1. Invitation and identity binding

The invitation chip and dashboard acceptance remain the user-facing entry point.
Acceptance persists the exact terms and `(game, network, sid, gcid)` locally.
The game registers an application-session public key. This local registration
does not authorize spending. UI refreshes emit no gaming events.

### 2. Table bond and seat claim

When the accepted terms require a table bond, dcrpulse constructs, validates and
broadcasts it only after dashboard approval. Once the bridge locates the exact
output in the mempool or chain, it publishes one `seat` control event.

The event contains the terms hash, application-session public key, bridge
protocol key, financial key identifier, admission descriptor ID and exact bond
outpoint. Application and bridge signatures bind their respective keys and the
financial evidence to the same participant and table.

Every peer independently verifies the generic record. A temporarily unseen
transaction remains pending until a local chain or mempool update; it is never
discarded or republished.

### 3. Seating and roster agreement

When a peer holds the configured number of valid seat records, it publishes one
`roster_prepare` containing their ordered event IDs and candidate roster hash.
After all participants prepare the same candidate, each publishes one
`roster_commit`. Matching commits form the roster certificate. Conflicting
commitments are retained as evidence and stop seating.

The admission deadline closes only new seat claims. A full committed roster does
not expire while already published table bonds gain confirmations.

### 4. Deterministic seat draw

The invitation fixes the draw height before its block hash is knowable. After the
roster certificate and configured confirmation depth exist, every peer derives
the same seat ordering with a frozen domain-separated algorithm. The draw creates
no BR message.

A block offset and confirmation depth are separate values. A reorganization
pauses or invalidates the draw according to one documented rule; the SDK never
silently substitutes a new block hash.

### 5. Stake

The game requests the standard stake operation through the SDK when its own rules
permit. dcrpulse does not interpret those rules. It independently validates the
request against the roster, accepted terms, descriptor, wallet/account policy
and spending limits, then requires dashboard approval. dcrpulse alone prepares,
signs and broadcasts the transaction.

After locating the exact output, the owning bridge publishes one `stake` control
event with the roster reference, descriptor ID and outpoint. Every peer verifies
the script, amount, owner and refund terms using its own node. Confirmations are
local observations and cause no BR publication. The SDK reports financial state;
only the game decides what that state means for gameplay.

### 6. Payout and refund

A game may submit a payout proposal through the SDK. The proposal is untrusted
input, not proof that the reported result is correct. dcrpulse independently
verifies the table, funded inputs, participant destinations, amounts, fees and
absence of conflicting spends. It does not adjudicate game rules or trust a
reported winner.

Each bridge publishes at most one `payout_signature` for an exact proposal and
only after explicit dashboard approval. Cooperative payout requires every
participant's required financial signature. A designated bridge publishes one
`payout_published` after broadcast. If cooperation fails, each owner retains
unilateral timelocked recovery of their own deposit.

## Opaque game traffic

The SDK exposes an authenticated application-message API over the same BR
carrier. Its contract is bytes in and bytes out, scoped by game ID, game version,
table ID, destination and author. dcrpulse assigns a stable message ID, persists
before publishing, deduplicates on receive and replays until acknowledged. The
SDK delivers the bytes and does not decode or reduce them.

StakeWars defines its own readiness, map seed, terrain hash, turn, timeout, skip
and outcome messages in the StakeWars repository. Poker, Battleships,
Four-in-a-Row and future games define unrelated payloads. None of those schemas
or state machines belongs in dcrgaming-sdk or dcrpulse.

## Recovery

Normal recovery is local replay from dcrpulse's durable inbox. It emits no BR
traffic.

There is no automatic peer `fetch`, `want`, resync request or response. dcrpulse
rebuilds its inbox from BR history and resumes its local delivery cursor. An
explicit operator retry of an uncertain outgoing publication uses the same event
ID; it never creates a new logical event. Clean reconnects, duplicates, chain
ticks and dashboard refreshes emit nothing.

## Generic SDK reducer behavior

Every stored `table.*` or `finance.*` event receives one status:

- `pending`: valid shape/signature but missing local facts or dependencies;
- `applied`: durably included in table state;
- `duplicate`: identical event or already applied event ID;
- `conflict`: same irreversible logical slot with different contents;
- `rejected`: permanently invalid signature, binding, bounds or table identity.

Temporary node unavailability, mempool propagation lag, an unavailable parent
event, or an unconfirmed transaction are `pending`. They are never permanent
rejections.

The SDK reducer verifies that envelope game/version/SID, authenticated
destination, event bindings and stored table `(game, network, sid, gcid)` all
agree before application. Opaque application events have bridge transport state;
their semantic state exists only inside the game.

## Bounds

- Table and financial evidence use `exp=0`.
- Control records must fit one BR frame.
- Current outgoing events must fit one BR frame, which lets dcrpulse
  independently recompute the stable MID before accepting a hostile game's
  publication request. Fragmented application events require bridge-side full
  payload verification before they can be enabled.
- Incomplete fragment storage has a bounded assembly timeout independent from
  event validity.
- Per-game, per-table, per-sender and quarantine byte/event limits are enforced
  before allocation.
- Unknown versions and unknown security-sensitive fields fail closed.

## Wire-count invariants

When no protocol state changes, each of these must add exactly zero BR messages:

- one or one thousand UI refreshes;
- chain-tip or confirmation polling;
- a clean game-to-bridge reconnect;
- restarting the game after all events were acknowledged;
- receiving duplicate events;
- waiting for another participant;
- rendering a lobby, countdown or progress indicator.

For an ordinary two-player bonded and staked table, each bridge publishes one
seat claim, roster preparation, roster commitment and stake claim: eight generic
state events total. Seat drawing emits zero. Application-message counts are
defined by the game and are not part of this invariant.

## Current defects this replaces

- dcrpulse currently expects `gaming-frame`, while deployed brclientd emits
  `gc-message`.
- The dcrpulse gaming bus and financial inbox are transient and may drop frames.
- The bridge can compute a stream gap before installing its frame subscriber.
- The SDK drops unknown-table messages and rejects peer joins while the local
  bond is still pending.
- Chain-dependent validation permanently drops valid events during propagation
  lag.
- Seating can run from roster assertions before every irreversible commitment.
- StakeWars publishes `w.world` from a refresh path until all approvals arrive.
- World approvals are held only in memory.
- Random framing IDs provide neither semantic deduplication nor crash recovery.
- Short wall-clock TTLs can erase durable turns and financial state.
- SDK, game and financial layers have separate resync mechanisms that can all
  answer the same missing-state condition.

## Implementation order

1. Freeze canonical event encodings, signature preimages, logical slots, size
   limits and golden fixtures.
2. Implement dcrpulse durable ingress, quarantine, replay barrier and consumer
   acknowledgement. Route both `gc-message` and `gaming-frame` through it.
3. Implement dcrpulse idempotent outgoing publication and uncertain-enqueue
   reconciliation.
4. Implement dcrpulse's dependency-aware table/finance reducers before local
   bond payment. Make the SDK a typed client over their views.
5. Implement the combined signed seat record and the two-phase roster
   prepare/commit certificate. Require every commit before seating.
6. Implement deterministic draw confirmation and reorg handling with fixed
   vectors.
7. Add the bounded opaque `app.data` publish/subscription API. Implement
   StakeWars payload parsing and reducers only in dcrstakewars.
8. Delete all peer resync/want/fetch paths. Recover exclusively from dcrpulse's
   durable inbox and BR history.
9. Convert payout coordination to dependency-aware one-shot events while keeping
   all keys, approval, signing and broadcast authority in dcrpulse.
10. Delete the retired payload kinds, periodic replay code and old TTL behavior.
    Do not add compatibility shims.

## Acceptance

The test harness must model independent asynchronous BR histories rather than a
synchronous shared fan-out. Run every stage with two and six participants under
delay, reordering, duplication and restart:

- remote bond before local approval;
- seat claim before invitation acceptance;
- roster preparation before a missing seat claim;
- stake before local node propagation;
- payout signature before payout proposal;
- crash before/after inbox persistence, state commit, BR enqueue and local ACK;
- malicious GCID/SID/network/key substitution;
- same event ID replay and same logical slot conflict;
- queue/quarantine exhaustion;
- draw-block reorganization;
- local BR-history catch-up without peer-generated recovery traffic.

Tests must count physical BR publications by event ID. No-state-change operations
must publish zero messages. Real-money testing remains disabled until the full
contract passes this suite and a two-wallet, two-bridge, two-game simnet run.
