# StakeWars — approved decisions

Recorded 2026-09-15 from the project owner's instructions. This document takes
precedence over conflicting passages in the original PRD and implementation
specification. Technical proposals are not security guarantees.

## Product and visuals

- Product name: **StakeWars**. Go module: `github.com/karamble/dcrstakewars`.
- Go/Ebiten native desktop game for Linux, Windows and macOS.
- Full artillery-game mechanical scope from the PRD, with 2–6 players and up to
  eight characters per squad. First public release is not limited to duels.
- Every player-facing match requires stakes. No free play, hotseat product mode,
  solo training product mode or public unpaid demo. Offline fixtures and testnet
  builds are engineering tools, not player-facing game modes.
- Built-in content only. No map, weapon or scheme imports, modding support, or
  content editors. Custom payout allocations remain supported and are distinct
  from custom gameplay content. Cosmetic squad setup uses built-in choices.
- Stakey, the comic ticket character, is the squad character reference. Preserve
  the ticket silhouette, notched top and expressive face. The inspected local
  source is `pLabarta/dcrwebcomic`, not a repository named `dcrcomic`.
- Use Decred's turquoise, blue and deep navy visual identity. Add sci-fi gear,
  layered arenas, animation, lighting and destruction effects. Teams need
  symbols/nameplates as well as color differences.
- New artwork uses generated assets plus editing and animation work, grounded
  in these character references. This does not authorize copying unrelated
  crossover characters found in the comic collection.
- Rope and movement target original competitive feel, not exact WA replication.

## Turns and delivery

- Peer traffic is asynchronous Bison Relay messages through the SDK and the
  dcrpulse gaming bridge. No live input streaming or optional live-chunk mode.
- The local gRPC event subscription is only a bridge transport detail.
- The active player has a visible countdown. Retain the PRD's 45-second turn
  and 3-second retreat as initial defaults.
- The player submits a signed, tick-stamped turn batch. Others verify and replay
  it after receipt. An idle connected client submits an expired turn.
- A missing batch receives a delivery grace period, then a signed skip proposal.
- The proposal has its own response window. Responders must unanimously agree;
  nonresponders abstain rather than blocking the vote.
- An authenticated answer from the accused during the response window cancels
  the offline-skip proposal. Exact treatment of a late answer is a protocol
  design issue, not a reason to silently rewrite finalized history.
- A skipped player keeps their characters on the map. Others continue their
  turns and may eliminate that squad. Ordinary inactivity is handled through
  gameplay, not automatic bond seizure.
- Gameplay progression must not require acknowledgments from every absent or
  eliminated participant.

**Not yet established:** how peers agree on the responder set and skip outcome
despite delayed messages or partitions. Neither signatures nor local deadlines
prove that another peer did not receive an answer. See the executable
counterexamples in [protocol findings](protocol-findings.md).

## Money and verification

- Reuse the existing Poker/SDK escrow and multisig payout construction. Do not
  replace working Decred scripts with new game-specific financial plumbing.
- Every honest participant independently verifies inputs, state and outcome.
  No trusted referee and no assumption that a majority is honest.
- Custom payout allocations must be agreed before funding and bound to the
  match. Allocation validation must cover ties, team splits, integer rounding
  and fees; their production encoding is not implemented yet.
- SDK settlement takes gross shares totaling the funded pot. The SDK builder
  subtracts the fee from positive payouts; do not deduct it twice.
- Entry/seat bonds, stakes, liveness table bonds and forfeitable bonds are
  distinct deposits. Use the per-table entry-bond model for StakeWars.
- Real funding requests use the bridge's approval flow. Unknown payment status
  stays unknown until reconciled; no blind retry that asks for a second payment.
- **Updated 2026-09-16:** winner payouts use cooperative N-of-N transaction
  signing, including eliminated players and players receiving zero. Every peer
  verifies the result and exact payout locally before signing.
- Enforcing winnings against a refusing signer is not solved by the implemented
  protocol and is **not a requirement for this release**. Refusal can prevent a
  winner payout. This limitation must be disclosed before funding.
- If settlement cannot complete, every participant must be able to reclaim its
  own unspent original deposits independently after their agreed locks, less
  transaction fees. Refunds do not pay winnings or restore the latest game balance.
- Moves, shots, replay, cheat detection and accusation/skip consensus remain
  entirely off-chain over BR's gaming wire. No per-shot transactions or on-chain
  gameplay/dispute execution are part of the product plan.
- Funding remains gated on working cooperative settlement and end-to-end owner
  refunds, including persistence, restart, chain maturity, fees and spent-output
  reconciliation. Missing-player gameplay agreement still needs implementation;
  it does not authorize spending another participant's deposit.

See [the accepted payment model](payment-model.md). This decision supersedes
older requirements for guaranteed absent-loser winnings. Cryptographic research
remains documented but is not a dependency for implementing this payment model.

## Implementation order

1. Record these decisions and build the offline protocol characterization
   harness against the actual SDK. **Implemented characterization harness.**
2. Build the determinism foundation: fixed-point math, frozen RNG, canonical
   serialization, static checks and cross-architecture replay fixtures.
3. Build an internal gameplay prototype and a Stakey visual/movement showcase.
4. Complete built-in gameplay content and integrate the reviewed asynchronous
   match protocol and SDK lifecycle.
5. Test 2–6-seat funding, custom payouts, restarts and recovery on testnet; perform
   security review, performance qualification and desktop packaging.

Current implementation evidence is tracked in [implementation status](implementation-status.md).

The original week estimates are not commitments. Protocol research has explicit
results and blockers; it is not presumed to conclude with a safe construction.

## Seating anchor and battlefield agreement

Use the agreed seating-block hash as the randomness source for the battlefield.
The roster and full match settings must be bound before that block is known.
Derive a domain-separated hash of the seating block hash, roster hash and settings
hash (`StakeWars/map/v1`), using a specified canonical encoding. This is the
agreed design direction, not a deployed multiplayer handshake. Every peer must
verify the anchor, generate terrain and safe Stakey spawn positions locally, and
sign the same initial-state hash before the match can begin. Freeze the simulation
version, configuration, seed conversion and content version in the agreed terms.
A confirmation depth and reorganization policy remain to be settled; peers seeing
different anchor hashes must not proceed. The simulation currently takes a uint64
seed; the hash-to-seed conversion must be explicitly specified in that handshake.
Public anchor randomness makes future seeded events predictable, not secret.

## Dcrpulse owns table creation and chat invitations

The current host implements table creation in its Gaming tab: game, GC, buy-in,
2–6 seats and open-block window. It asks the creator's game to accept first, then
publishes the human-readable invitation via Bison Relay. Invite chips send the
same AcceptInvite request to joining games over the mTLS subscription. No manual
link/GC copy-paste is required. Current chip readiness checks the game's live
connection, not CAP_ACCEPT_INVITE. StakeWars receives these requests and opens
preparation; paid admission is still explicitly refused. Consequently creation
cannot yet complete/publish through dcrpulse. The current bridge contract has no
outgoing create-table/plain-chat RPC for StakeWars to call.

## Seating integration scope — 2026-09-16

The user explicitly deferred changes to `dcrgaming-sdk` and the Dcrpulse gaming
bridge/dashboard to separate later work. Do not silently expand StakeWars work
into those repositories. Keep Dcrpulse's existing New table dialog unchanged.
Buy-in, seats, admission deadline and advertised refund lock come from its
invitation; StakeWars must not substitute a hardcoded buy-in.

For the first preparation profile the user selected: a fixed gameplay preset,
winner-takes-all with equal gross shares on a draw, admission and liveness bonds
at the SDK minimum (0.01 DCR each), and an honesty bond of max(0.01 DCR, buy-in).
Use SDK baseline confirmations and review-then-fund for post-seating deposits.
Full funded preparation is the intended milestone, not permission to report an
unfunded seat as ready. Custom allocations remain a later supported feature.

SDK pending-admission recovery defects currently prevent activating that
integration under the unchanged-SDK constraint. Evidence and deferred work are
in `docs/seating-sdk-blockers.md`. Mainnet paid-game implementation is intended;
this dependency blocker must not be described as a refusal to implement it.

## 2026-09-16 — SDK recovery implementation

SDK changes are now authorized and implemented in the sibling repository.
Poker and Battleships are unfinished references, not compatibility constraints;
no deployed funds or legacy tables need migration. The SDK remains game-neutral:
games derive their economics from the invitation, while the runtime owns durable
admission, payment obligations, recovery and status reporting.

`make seating-check` now verifies pending admission across an actual SDK restart,
then approval and confirmed joining without a duplicate request. Production
StakeWars payment integration remains the next step; the previous missing-SDK
recovery error is replaced by an accurate integration status.
