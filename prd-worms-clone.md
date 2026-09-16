# PRD — StakeWars

> **2026-09-15 decisions:** [Approved decisions](docs/decisions.md) supersede
> conflicting passages below. StakeWars uses Stakey squads and Decred colors,
> paid-only matches, built-in content, asynchronous completed-turn replay, and
> timed unanimous votes among responders to propose skips. The original
> mechanical detail remains reference material; crypto claims are subject to
> the [protocol findings and release gates](docs/protocol-findings.md).

**Product name:** StakeWars
**Stack:** Go 1.23+ / Ebiten v2, native desktop (Linux, Windows, macOS)
**Multiplayer:** Peer-to-peer, no authoritative server, cryptographically verifiable match state
**Wire:** `dcrgaming-sdk` → dcrpulse `gaming-standalone` bridge (gRPC) → Bison Relay; reference impl `karamble/dcrbattleships`
**v1 scope:** Full artillery mechanics with built-in content and paid-only matches
**Status:** Draft 4 — approved decisions recorded; protocol validation in progress

---

## 1. Summary

A turn-based artillery game on fully destructible 2D terrain, in the lineage of *Worms Armageddon* and *Hogs of War*. Stakey ticket squads fight in polished sci-fi arenas using Decred's visual palette. The native Go/Ebiten client verifies signed completed-turn batches through deterministic replay. Randomness, skip finality and absent-player settlement require the protocol validation described in `docs/protocol-findings.md`; deterministic replay alone does not establish those guarantees.

The design bet: **turn-based play makes trustless P2P cheap.** Realtime netcode forces either an authoritative server or rollback with rich cheat surface. A turn game tolerates 300 ms RTT invisibly, needs ~50 input events per turn, and gives a natural synchronization barrier at every turn boundary for state-hash comparison. That is why this genre, not another, is the right vehicle for a serverless multiplayer title.

### Goals

- G1 — Full artillery mechanics: per-pixel destructible terrain, ninja rope, ~40 weapons, built-in rulesets, 6 teams × 8 Stakey characters. Rope targets original competitive feel.
- G2 — Bit-identical simulation across OS and CPU architecture, verifiable by state hash.
- G3 — Match integrity without a trusted party: signed input transcript, unbiasable randomness, reproducible replay.
- G4 — Single distributable binary per platform, assets embedded, no runtime installer.
- G5 — Built-in content shared identically by all peers. No modding, content imports or content editors. Custom payout allocations are agreed before funding.

### Non-goals (v1)

- 3D. The Worms 3D detour is the cautionary tale, not the target.
- Realtime modes. No "Shopper"-style live play; the determinism contract assumes discrete turns.
- Mobile / web builds. Ebiten supports both; deferred to avoid input-model and perf compromises leaking into v1 design.
- Matchmaking as a service, ranking ladders, accounts. Identity is a keypair; ladder is a post-v1 layer on top of signed replays.
- Single-player campaign, free play and training/challenge product modes. Internal fixtures are development tools only.

---

## 2. Positioning

| | Worms Armageddon | Worms W.M.D | Hogs of War | This |
|---|---|---|---|---|
| Engine | Proprietary, 1999 | Proprietary | Proprietary | Go/Ebiten, open |
| Netplay | WormNET (central) | Steam/P2P w/ host authority | Split-screen | Trustless P2P |
| Host advantage | Yes (host = arbiter) | Yes | n/a | None by construction |
| Replays verifiable | No | No | No | Yes, signed |
| Modding | Scheme editor, WA hacks | Limited | None | First-class |

Audience: the WA competitive remnant (still active, still on a 1999 binary because nothing replaced it), plus the self-hosted/crypto-adjacent crowd who want games that run without anyone's servers.

---

## 3. Match model

### 3.1 Setup

- 2–6 teams. Each team has 1–8 Stakey characters and corresponds to one peer. No public hotseat mode.
- Scheme (ruleset) agreed pre-match and hashed into the match parameters. Any scheme mismatch fails the handshake — it cannot be silently divergent.
- Map: procedurally generated from a jointly derived seed, or a pre-agreed map file identified by content hash.
- Worm placement: randomized from the same seed, or manual placement (scheme option) with placement order round-robin.

### 3.2 Turn structure

```
turn_start
  → wind sampled from turn beacon
  → active worm selected (round-robin or scheme-defined)
  → turn timer runs (default 45 s)
  → player acts: move / rope / select weapon / aim / fire
  → on fire or timeout: retreat window (default 3 s)
  → projectile & explosion resolution to quiescence
  → damage, fall damage, chain reactions, drowning
  → crate drop roll from turn beacon
  → state hash exchanged, compared
turn_end
```

Sudden death at scheme-defined turn count: water rise, or all worms to 1 HP, or both.

**Missing turn:** after countdown and delivery grace, a peer proposes skipping
the absent player. The proposal has its own timed response window. Responders
must unanimously approve; nonresponders abstain. An answer from the accused
during that window cancels the offline proposal. A skipped squad remains on the
map and can be eliminated by others. This is the required gameplay behavior;
the safe distributed finalization mechanism remains gated by protocol tests.
Do not require an absent player's acknowledgment to progress gameplay, and do
not automatically seize a bond for an ordinary missed turn.

### 3.3 Modes

- Deathmatch (last team standing)
- Fort / Darkside variants (fixed maps, no rope)
- Race (checkpoint-based, rope-heavy)
- Roper (single-weapon schemes)
- Shopper family
- No player-facing free, hotseat or training mode. Seeded internal fixtures and testnet builds support development.

---

## 4. Simulation spec

### 4.1 Determinism contract

This is the load-bearing section. Everything else is negotiable.

**Rule 1 — No floating point in `pkg/sim`.** Go's spec explicitly permits an implementation to fuse `x*y + z` into a single FMA with one rounding. arm64 and ppc64 do this; amd64 generally does not. Identical Go source therefore produces different float results across architectures. All simulation math is fixed-point **Q16.16** on `int32`/`int64`. A `float64` in the sim package is a build-breaking lint failure (custom `go vet` analyzer, CI-enforced).

**Rule 2 — One PRNG, vendored, versioned.** `math/rand` output is not stable across Go releases and `math/rand/v2` differs again. The sim carries its own PCG-XSH-RR implementation in `pkg/sim/rng`, frozen, with a golden-vector test. Never seeded from time.

**Rule 3 — No map iteration in sim.** Go randomizes map range order. Simulation state uses slices with stable ordering; maps are permitted only for lookup with no iteration, enforced by the same analyzer.

**Rule 4 — No wall clock, no goroutines, no `select` in sim.** The turn timer is a tick count. The sim is a single-threaded pure function `Step(State, []Input) State`. Rendering, audio, and networking run outside it and may never write to it.

**Rule 5 — Fixed tick rate.** 60 Hz simulation. Ebiten's `Update` is already fixed-timestep at 60 TPS by default; render frames may decouple and interpolate, sim ticks may not.

**Rule 6 — State hash every tick boundary is cheap; exchange it every turn.** FNV-1a or BLAKE3 over the canonical serialized state. Mismatch = desync = match halts and both transcripts are dumped for diffing.

### 4.2 Terrain

Dual representation:

- **Collision mask** — 1 bit per pixel, `[]uint64` bitset, row-major, word-aligned. The only thing the sim reads. ~4096×2048 map = 1 MB. Cheap to hash, cheap to snapshot.
- **Visual layer** — RGBA `*ebiten.Image` with texture fill, edge/grass decoration, and destruction rims. Purely presentational, never read by the sim.

Operations:
- **Circular blast:** stamp a circle into the mask; queue the dirty rect for visual re-upload. Never re-upload the whole texture.
- **Rim generation:** after a blast, the newly exposed boundary gets a darkened crater edge in the visual layer only.
- **Girders:** additive stamp, mask + visual.
- **Terrain physics:** WA-style "no collapse" by default; scheme option for detached-chunk collapse (flood fill from anchors, unanchored regions fall as debris).
- **Indestructible border** per scheme; water level as a scalar, not terrain.

Dirty-rect batching: accumulate per tick, coalesce overlapping rects, upload once per frame. Target ≤ 4 sub-image uploads/frame at 4096×2048.

### 4.3 Bodies and movement

- Worms are 1×2-cell AABBs (classic WA is effectively a point with a probe height). Movement is per-pixel stepping with a slope-climb threshold — no general rigid-body solver, which is the correct call: a physics engine here buys nothing and costs determinism.
- Fall damage proportional to fall distance above a threshold, capped.
- Drowning: below water level, worm is removed with a fixed animation, damage is lethal regardless of HP.
- Knockback from explosions: impulse from blast center, magnitude falls off linearly to blast radius, applied as a ballistic velocity.

### 4.4 Projectiles

- Integrator: semi-implicit Euler at sim tick, with **sub-stepping** so no projectile advances more than 1 px per collision probe. Tunneling through 1-px terrain is the classic bug; the sub-step count is `ceil(|v|)` capped at 32.
- Wind: constant horizontal acceleration for the turn, applies only to wind-affected weapons.
- Bounce weapons (grenade, cluster, mine): reflect velocity about the local terrain normal, estimated from the mask via a 3×3 gradient probe, with restitution and friction per weapon.
- Fuse timers in ticks, decremented in sim, not in wall time.
- Chain reactions resolve to quiescence with a hard tick cap (e.g. 60 s of sim time) to bound turn length in pathological mine fields.

### 4.5 Ninja rope

The single hardest mechanic and the one the competitive audience will judge the whole project on.

- Rope is a polyline of pivot points. Fire → raycast along aim vector until terrain hit → anchor.
- Swing phase: worm constrained to circle of current segment length about the last pivot; gravity projected onto the tangent.
- **Wrapping:** each tick, probe the segment from worm to last pivot for terrain intersection; on hit, insert a new pivot at the contact pixel. **Unwrapping:** when the angle at the last pivot goes convex, pop it. This pivot push/pop is what gives WA roping its feel; get it wrong and the entire skill ceiling evaporates.
- Rope length adjustable (up/down), clamped per scheme.
- Must be replay-tested against hand-authored rope courses as regression fixtures.

### 4.6 Weapon set (v1 target)

Tier 1 (must ship, MVP):
bazooka, grenade, cluster bomb, shotgun, uzi, fire punch, dynamite, mine, teleport, ninja rope, skip go, surrender.

Tier 2 (v1):
homing missile, mortar, holy hand grenade, banana bomb, air strike, napalm strike, mine strike, blowtorch, pneumatic drill, girder, parachute, bungee, kamikaze, sheep, super sheep, baseball bat, prod, sheep launcher, petrol bomb, handgun, longbow, minigun, homing pigeon, mad cow, old woman, concrete donkey, indian nuclear test, jetpack, low gravity, laser sight, fast walk, invisibility, freeze, earthquake.

Each weapon is a **declarative definition** (damage, radius, fuse, wind-affected, bounce params, cluster spawn, sprite, sounds) in an embedded TOML table, plus an optional behavior ID that maps to a Go implementation. Definitions and behaviors are built-in. Their content hash is part of match parameters; peers reject mismatched content before funding. There is no user modding interface.

---

## 5. Networking & trust model

### 5.1 The key simplification

Positions, HP and resolved events are public. That avoids Poker's hidden-card
protocol for ordinary play, but does not remove the requirements for safe
randomness, authenticated inputs, distributed agreement and enforceable
settlement. Weapons with hidden information need explicit review. Public future
randomness can also reveal future crates and wind to a modified client.

(Exception: a hidden-placement or fog-of-war scheme would reintroduce commitments. Deferred past v1, but the state layout should keep per-team visibility as a concept so it isn't a rewrite later.)

### 5.2 Randomness: block-hash anchor

**Candidate, not finalized for paid play.** The reference games record block-hash
anchors. The derivation below is a candidate deterministic seed mechanism. It
must be evaluated for anchor selection, miner influence, reorganizations and
predictability of all future events before adoption.

1. **Anchor.** Before funding, peers bind a future block height *h*. Its hash `A` is recorded as the match anchor. The height must not be selectable after its hash is known.
2. **Map seed** = `H(A ‖ match_id ‖ scheme_hash)`.
3. **Turn beacon** = `H(A ‖ match_id ‖ turn_no)`. Drives wind, crate type, crate position, crate contents, weapon-internal randomness.

This deletes an entire message class. No commitments, no reveal round, no reveals to withhold, no last-revealer bias, no per-turn traffic for randomness at all — the whole match schedule is derivable from one 32-byte anchor.

Turn cadence is faster than Decred block time, which is why the beacon is a deterministic expansion of one anchor rather than one block per turn. Two caveats that follow:

- **Miner grinding.** A miner could in principle grind `A` to shape the whole match's wind and crate schedule. For a friendly match this is absurd. For a staked match it is a real, if remote, concern — mitigate by re-anchoring every N turns to a newer block, or by mixing in per-peer commitments as a second input. Decide with the stakes question (open question 2).
- **Reorgs.** Battleships treats byte-disagreeing flipnotes as a reorg split and abandons rather than re-signing. Same rule here: require anchor depth before the match starts, and treat a disagreeing anchor as abandonment, never as something to resolve in-band.

### 5.3 Lockstep

- Input events (`MOVE_LEFT_DOWN@tick`, `AIM_UP@tick`, `FIRE@tick,power`) are included in a signed completed-turn batch.
- Non-active peers validate and replay inputs at their stated simulation ticks after receiving the batch.
- The active peer sees immediate local play; observers see delayed replay. No fixed RTT or live observation is promised.
- Peers exchange signed state hashes. Disagreement is retained for investigation; it is not automatic proof of cheating or an automatic refund trigger. Finality with missing peers depends on the reviewed protocol.

### 5.4 Identity and transcript

- Identity is an SDK concern: `connect.Identity`, `connect.Stamp/Prove/Load/Save`, `identity.json` beside `bridge.json`. No game-specific keygen, no key distribution. `transport.Frame` carries an authenticated sender, so seat-to-sender binding is checkable at the wire.
- **Transcript = a `bslog`-style hash chain.** Battleships' `pkg/bslog` is the pattern to copy: per-seat signers over a roster, `Next(seat, kind, body, height, anchor)` → `Append`, `Head()`, `AttestHead`, and `ConflictingHeads` → `RecoverKeyFromHeads` (equivocation publishes your own key — the dcrpoker position-nonce trick, generalized).
- A `TurnBatch` is just a chain entry of a new kind. Replays, forfeit evidence, and ladder anti-forgery all fall out of the chain rather than being designed.

### 5.5 Wire: dcrgaming-sdk over the dcrpulse bridge

**Decided.** BR is not integrated directly. dcrpulse's `gaming-standalone` branch exposes a gaming bridge, and standalone games link `github.com/karamble/dcrgaming-sdk` and speak gRPC (`gaming/gamingpb`) to it. `karamble/dcrbattleships` is the working reference implementation.

The SDK's `runtime.Game` interface is the plug-in contract, verified against master:

| Method | Gives |
|---|---|
| `Send(ctx, match, kind, body, class wire.Class)` | framing, chunking, routing. **`wire.Class` sets a message's delivery lifetime** — a move and a piece of evidence are not worth delivering equally long |
| `Settle(ctx, match, Outcome{Shares, Void})` | payout built from the terms every seat signed. `Void` unwinds the table, each seat taking its own stake back |
| `Seize(ctx, match, seat, *evidence.Exposed)` | spends a branch of a seat's forfeitable bond **using a key that seat's own signatures gave up** |
| `Accuse(ctx, match, seat, Lapsed)` | accusation chain against a seat that stopped answering. The game says what was owed and when; the chain says whether it is late |
| `Release` / `Reclaim` | cooperative bond return; timelock-matured self-claim |
| `Chain(ctx) → {Height, Hash}`, `BlockHash(ctx, height)` | tip and historical block hashes, explicitly "for a game anchoring its own moves in time" |
| `Seat`, `Seats`, `LogSeats`, `LogKey`, `MatchID` | roster, and this seat's log-signing key bound to the table |

Two properties of that contract worth internalizing:

- **A game cannot broadcast.** Deliberate — a game that could broadcast could pay somebody. All chain writes go through the runtime.
- **Log keys and session keys are separate.** The game signs what it says happened with `LogKey`; the session key holds the stake and never leaves the runtime.

Also imported: `escrow`, `forfeit`, `punish`, `evidence`, `spend`, `identity`, `membership`, `gaming/{connect,transport,wire,schema,gamingpb,bridgetest}`.

So the worms project writes **game logic only**: sim, terrain, rope, weapons, turn state machine, message schema, chain kinds, renderer. Everything in the original §6 `pkg/net` tree except lockstep and session handling is deleted and replaced by an import.

Mapping onto the SDK's existing concepts:

- match = table; 2–6 teams = 2–6 seats (`membership` handles formation/roster unchanged)
- ordinary turn timeout = signed skip-vote protocol; sustained chain silence is a separate `Accuse` case
- match end = `rt.Settle(ctx, match, runtime.Outcome{Shares: …})`
- bond scope = `runtime.BondPerTable`
- state hash = BLAKE3 (`lukechampine.com/blake3` is already in the line's dep tree)

The current runtime's accusation and forfeitable-bond paths are heads-up only.
The SDK's 6-member escrow limit does not establish 6-player punishment support.

### 5.5.1 Process model

Battleships runs as a daemon (`cmd/dcrbattleshipsd`) serving a JS UI out of `ui/dist`. That split exists because the UI is JavaScript. **Ebiten is Go, so the split collapses:** one binary, Ebiten window and SDK bridge client in the same process.

```
main goroutine   Ebiten Update()  → drain inbox → sim.Step() → Draw()
bg goroutine     runtime.Runtime tick loop, bridge gRPC, chain persistence
                 ↓ inbound frames land on a buffered channel
```

**Determinism hazard, and the only one this architecture introduces:** network messages must be drained at exactly one fixed point per tick, at the top of `Update`, never applied mid-step. `pkg/sim` stays single-threaded and never sees a channel. The `pkg/sim` import-graph test (§4.1) already enforces the boundary; add a race-detector CI job on top.

### 5.5.2 Message budget

Turn structure makes batching natural. Baseline: **one chain entry per peer per turn.**

Active peer at turn end emits one signed entry:
```
TurnBatch {
  turn_no
  inputs   []{tick, event, param}   // full turn, tick-stamped
  state_hash                        // post-resolution
}
```
Non-active peers emit one small `Ack{turn_no, state_hash}`. With the block anchor there are no reveals to carry. ~`k` entries per turn, ~80 turns → a few hundred messages per match. A `TurnBatch` should be well under 1 KB; confirm against the bridge's frame size limit before committing to the format.

**Cost of the baseline:** observers receive the batch after the active turn completes, then replay it. Delay includes the completed turn and asynchronous message delivery, not merely one bridge RTT.

**No live mode:** Bison Relay carries asynchronous completed-turn batches. SDK transport chunking for message size is distinct from streaming gameplay inputs. Observers replay after the batch arrives.

### 5.5.3 Order independence

Nothing needs a global order. Only the active peer's inputs are authoritative in a turn, and per-sender order holds. The beacon is derived from the anchor, not from peer messages. Acks are a set comparison. Stale arrivals resolve by `turn_no`; anything for a closed turn is dropped.

### 5.5.4 Two clocks

The rails measure duty in **block heights** — `Accuse` takes a lapse the chain can judge as late. A worms turn timer is 45 seconds. Decred blocks are ~5 minutes. These cannot be the same clock, and conflating them is a design trap:

- **Game clock (ticks).** Turn timer, retreat window, resolution cap. Enforced by peer consensus inside the sim. Missing it costs you your turn — the active worm just doesn't fire. No chain involvement whatsoever.
- **Duty clock (heights).** "This seat has stopped answering at all." Coarse, several blocks wide, and the only thing that reaches `Accuse`.

A missed 45-second turn is normal play, not a lapse. Only sustained silence is accusable. Get this wrong and every slow player gets an accusation chain opened against them.

### 5.5.5 Spectators and non-seat senders

The group chat is wider than the roster. `Message.From` is a Bison Relay UID;
`Seats(match)` contains session public keys. They are not interchangeable.
The current runtime bounds admission by known session and authenticates its
membership messages by signatures. It does not supply a verified UID-to-seat
lookup. Game messages must verify their claimed seat against the roster's
signing key before gameplay state changes. Any stricter transport identity
binding needs a separately specified and verified exchange.

**The determinism hazard is the real reason this matters.** A rejected frame must change nothing — not a sequence counter, not a held-frame queue, not a timer. Battleships keeps a `held` queue for frames arriving before a board exists; a junk frame accumulating there on one peer and not another is a desync with no cheating involved. So: filter before any queue, before any parse that mutates, and identically on every peer. Frame rejection is the one place where "obviously harmless" leniency produces a divergence.

Rate-limiting a spamming spectator is local policy, not sim state — peers may drop at different rates safely, precisely because dropped junk affects nothing.

Spectators do not sign gameplay votes. A verified transcript can support replay,
but late-join discovery, synchronization, content availability and finality
verification still require implementation. A signature alone cannot select the
canonical continuation if a timeout protocol has split honest peers.

### 5.6 Threat model

| Threat | Handled by |
|---|---|
| Favorable map selection | Seed from pre-committed chain (§5.2) |
| Re-rolling wind / crates | Same |
| Rewriting history after loss | Signed input transcript |
| Modified client, altered damage values | State-hash divergence at turn end |
| Modified weapon table | Table hash in match params |
| Rage-quit to deny a loss | Skip through the agreed protocol; squad stays vulnerable. Payout without a fresh absent signature remains a release gate |
| Withholding a randomness contribution | Depends on the final randomness protocol; not presumed solved by a liveness bond |
| Relay drops/delays messages | Indistinguishable from disconnect; timeout path. Relay cannot read or forge. |
| Sybil lobbies | LN message cost + BR identity friction; not free to spin up |
| Aimbot / trajectory solver | **Not preventable.** The inputs are legal inputs. Mitigation is social (invite-only GCs, which BR gives natively) or economic (staked matches). State this honestly rather than pretending otherwise. |
| Non-seat GC member injecting gaming frames | Bounded transport admission, then verify signed game messages against the roster before game-state mutation |
| Seated peer emitting frames claiming another seat | Verify the claimed seat's roster key; any additional BR UID binding must be explicit |
| Griefing via 60 s mine chains | Hard tick cap on resolution phase |

### 5.6.1 What the game owns vs what the rails own

The line's convention: each game brings its own crypto for its own logic, and imports the SDK for the rails. dcrpoker brings `pkg/deck` (Barnett–Smart on kyber). dcrbattleships brings `pkg/commitment` (Merkle over salted cells) and `pkg/bslog` (hash-chained signed log). Both import bridge, runtime, membership, escrow, forfeit, punish, evidence, spend.

Worms' game-side crypto surface is the thinnest of the three, because perfect information deletes the hard part:

- anchor → seed → per-turn beacon expansion (hashing, no protocol)
- hash-chained transcript with per-seat signers, head attestation, equivocation → key recovery (a `bslog` analogue)
- sim-version + weapon-table + scheme binding in match params

No deck, no shuffle proofs, no cell commitments. Essentially all project complexity sits in the simulation instead. That makes worms the best stress test of the rails specifically — it exercises the bridge, runtime, bonds and dispute path with almost no game crypto competing for attention.

### 5.6.2 Disputes are not self-contained here

This is the one place worms breaks an assumption both reference games satisfy.

Poker and battleships produce **verdicts computable from the complaint alone** — the deterministic shuffle dispute, equivocation key recovery, Merkle open verification, attestation checks. Cheap, self-contained, ladder-friendly.

Worms records *inputs*. Adjudicating "your state hash is wrong" requires re-running a 60 Hz sim over a 1 MB terrain mask. That is not computable from the complaint and not computable on-chain.

Consequences:

- **Hashes detect disagreement; majority does not prove correctness.** Replay may establish which claimed state follows a complete agreed transcript, but an honest-majority assumption is not allowed. On-chain enforcement is a separate requirement.
- **No automatic desync refund.** `Outcome{Void: true}` can cooperatively return stakes less fees, but letting any fabricated hash trigger a refund permits escape from a valid loss. The final dispute policy is gated on the settlement construction.
- **The SDK's own model confirms why divergence can't be slashed.** `Seize` spends a bond branch *using a key the cheating seat's own signatures gave up* — a game whose cheating exposes no key has nothing for it to spend. Sim divergence exposes no key. Equivocation does, via `RecoverKeyFromHeads`. So the split isn't a policy choice, it's structural.
- **The common cheats remain inside the existing ladder.** Equivocation (two signed heads) is seizable; sustained silence is accusable. Only "wrong simulation result" falls outside, and it lands on `Void`.
- **Narrow the diagnosis:** pin `sim_version`, content and rules hashes in match params, and checkpoint the state several times per turn. These checks prevent version mismatches; they do not prove the absence of implementation bugs.

---

## 6. Architecture

```
cmd/game/              entrypoint, Ebiten Game impl
pkg/sim/               PURE. fixed-point, deterministic, no I/O
  ├── fixed/           Q16.16 arithmetic, trig LUTs
  ├── rng/             vendored PCG, golden vectors
  ├── terrain/         bitset mask, stamps, flood fill
  ├── body/            worm movement, collision probes
  ├── projectile/      integrator, sub-stepping, bounce
  ├── rope/            pivot push/pop solver
  ├── weapon/          declarative table + behavior dispatch
  └── state.go         State, Input, Step(), canonical hash
pkg/render/            Ebiten draw, dirty-rect texture mgmt, camera, interpolation
pkg/audio/             Ebiten audio, positional mix
pkg/net/
  ├── lockstep/        TurnBatch apply, deterministic replay, catch-up
  ├── beacon/          anchor → map seed → per-turn expansion
  ├── wschema/         game message kinds over sdk gaming/schema
  └── wlog/            bslog-style hash chain: entries, head attest, equivocation
                       ── everything else comes from dcrgaming-sdk:
                          gaming/{connect,transport,wire,schema,gamingpb,bridgetest}
                          runtime, membership, identity
                          escrow, forfeit, punish, evidence, spend
pkg/mapgen/            built-in procedural generation from seed
pkg/scheme/            ruleset definition, validation, hashing
pkg/replay/            transcript read/write/verify
pkg/ui/                menus, built-in squad customization, paid-match lobby
assets/                go:embed — sprites, fonts, audio, default maps
```

`pkg/sim` imports nothing outside stdlib and its own subpackages. CI enforces this with an import graph test. That single constraint is what keeps determinism from eroding over 18 months of feature work.

---

## 7. Rendering

- Ebiten v2. Camera with zoom (0.5×–2×), edge-follow on active worm and on projectiles.
- Layers: sky gradient → parallax background → terrain visual → water (animated, foreground + background bands) → entities → particles → HUD.
- Terrain texture upload only on dirty rects (§4.2).
- Sprite atlas per team colour, generated at build time from a source palette; worm sprites are palette-swapped rather than duplicated.
- Particles (debris, smoke, water splash) are presentational only — spawned from sim events, never feeding back. They may be non-deterministic and frame-rate dependent; that is fine and should be explicit so nobody "fixes" it into the sim.
- Render decoupled from sim tick with position interpolation, so a 144 Hz display is smooth while the sim stays at 60.

**Budget:** 4096×2048 map, 48 worms, 200 particles, ≤ 4 ms CPU frame, ≤ 8 ms GPU frame on integrated graphics (Intel Iris Xe class).

---

## 8. Content pipeline

- **Procedural maps:** cavern and island generators. Perlin/simplex on the fixed-point RNG, thresholded to the mask, then erosion pass. Seeded, so identical across peers by construction.
- **Maps:** built-in authored arenas and deterministic generators only; no user import or map editor.
- **Rulesets:** built-in turn, weapon, sudden-death and terrain options, validated and hashed. No user scheme editor.
- **Teams:** up to 8 named Stakey characters, team name and built-in cosmetic/voice choices. No user asset or voice-pack imports.

---

## 9. Persistence

- Config, teams, schemes, maps, replays in the OS config dir (`os.UserConfigDir`).
- Storage: SQLite or bbolt for match history and stats; flat files for maps/schemes/replays so they are shareable by copy.
- Replays are the canonical artifact — small (inputs only, ~10 KB/match), verifiable, and the basis for any later ladder.

---

## 10. Milestones

Phased. Each phase ends in something playable.

**M0 — Determinism harness (3 wks).** Fixed-point lib, RNG, sim skeleton, golden-vector tests, CI lint analyzer for float/map/time in `pkg/sim`, cross-arch hash comparison job (amd64 + arm64 runners). Nothing playable; everything downstream depends on it.

**M1 — Internal prototype.** Terrain destruction, movement, bazooka, grenade, wind and turn loop. Automated matches and developer-controlled fixtures validate mechanics; no public free-play mode.

**M2 — Core feel (8 wks).** Ninja rope with pivot solver, jetpack, full Tier 1 weapon set, camera, particles, crates, water rise, real art direction pass.

**M3 — Netplay.** SDK integration, signed asynchronous turn batches, replay, reviewed skip finality and settlement. Use `gaming/bridgetest` and testnet lifecycle tests. Measure delivery delays to inform the approved timeout mechanism. Six-player runtime extensions are explicit work, not assumed imports.

**M4 — Full built-in content.** Tier 2 weapons, built-in maps and rulesets, Stakey squad customization and 6-peer matches. No content editors.

**M5 — Polish.** Built-in audio and voices, UI, cross-platform packaging, accessibility, performance and release security review.

~43 weeks of focused work. **Realistically 18–24 months solo.** "Full-feature Worms Armageddon clone" is a large target — WA itself accreted features for two decades, and a substantial part of its reputation rests on rope physics that were tuned over years. The phasing above is structured so M2 is a legitimate stopping point if the scope turns out to be wrong, and M3 is where the differentiator actually lands.

---

## 11. Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Rope feels unresponsive or shallow | High | Prototype early, test with experienced artillery-game players and maintain rope regression fixtures |
| Determinism erodes as features land | High | CI analyzer from M0; cross-arch hash job on every PR. Non-negotiable. |
| Scope: 40 weapons × art × audio | High | Declarative weapon table; behavior reuse; ship M2 set if needed |
| Terrain texture upload cost | Medium | Dirty rects from day one, not retrofitted |
| BR message latency makes observation feel dead | Medium | Batched turn playback is the baseline UX, not a degradation; chunk mode if latency allows |
| Audience gated behind dcrpulse adoption | Medium | Accepted trade: public matches require stakes and the bridge; make onboarding clear |
| `dcrgaming-sdk` is unpublished | Medium | dcrbattleships pins v0.3.0 but carries a `replace` to a local path, so nobody outside can build it. Publish the SDK before M3, or the game inherits the same problem. |
| Concurrency leaks into the sim via the inbox | High | Single drain point at the top of `Update`; import-graph test plus race-detector CI job |
| Solo art direction | Medium | Commission or use a coherent free asset base early; placeholder art masks feel problems |
| Audience never shows up | Medium | WA community engagement from M2. Being open-source and serverless is the pitch. |

---

## 12. Open questions

1. ~~Packaging vs dcrpulse.~~ **Resolved:** standalone binary linking `dcrgaming-sdk`, gRPC to the dcrpulse `gaming-standalone` bridge, per the dcrbattleships pattern. Ebiten collapses the daemon/UI split into one process (§5.5.1).
2. **`gamelog` generalization.** The SDK is public and complete on master, but `gamelog.Chain.Next(seat, hand, street Street, action Action, amount int64)` is still hold'em-shaped, which is why battleships carries its own `pkg/bslog`. Promote the generic form (`kind`, opaque `body []byte`, `height`, `anchor`) into the SDK before worms starts, or worms writes the third copy of equivocation recovery.
3. **Resolved:** all player-facing matches are paid; no unstaked product mode. Payment finality remains a technical release gate.
4. **Resolved:** asynchronous completed-turn replay only; no live-chunk mode.
5. **Resolved:** original competitive rope feel, evaluated with experienced players.
6. Map resolution ceiling — 4096×2048 fits WA; larger maps blow the dirty-rect budget and the per-turn hash cost.
7. **Resolved:** Stakey ticket squads, Decred turquoise/blue/navy, polished sci-fi arenas. Reference: local `pLabarta/dcrwebcomic` artwork; generated new art plus editing and animation.
8. ~~Spectators.~~ **Resolved:** the GC is wider than the roster, so passive observers work for free (§5.5.5); the work is edge-filtering non-seat and seat-spoofing senders so junk frames never reach sim state.

---

## 13. Legal

"Worms" and the worm character designs are Team17 trademarks and copyrighted assets. Game *mechanics* are not copyrightable; expression is. Therefore:

- No use of the name, logo, or any Team17 asset. No "Worms-like" in the product name.
- No ripped sprites, sounds, voice packs, or map files. `.lev` import reads user-supplied files; it does not bundle them.
- Original characters and original art. This is also the design opportunity — see open question 5.
- License the project under a permissive or copyleft OSS license from commit one; retrofitting is painful.
