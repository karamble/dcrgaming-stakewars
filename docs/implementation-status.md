# Implementation status — 2026-09-16

## First cooperative playable build

The desktop now uses the real SDK runtime for invitations, admission bonds,
seating, stake funding, world agreement, signed turn delivery/replay, cooperative
winner settlement and independent owner refunds. `-datadir` isolates bridge
credentials, identity, controls and durable match state for multi-instance tests.
Decred-style startup uses `dcrutil.AppDataDir`, creates `dcrstakewars.conf` on
first run, supports `--appdata`/`--configfile`, and uses `slog` subsystem loggers
with rotating per-profile logs. `make multiplayer-demo` runs two desktop peers
with a local simulated bridge.

See [the current walkthrough, verification and limits](first-playable.md).
Responding-peer skip consensus is still unresolved; missing players can stall
play and veto payouts. Owner refund locks remain the financial fallback.
Historical milestones below describe the implementation at their dates; earlier
preview-only statements are superseded by this section.

## Historical prototype status — 2026-09-15

This is a development prototype, not the completed paid game. The approved
scope in [decisions](decisions.md) remains the target.

## Implemented and locally exercised

- Q16.16 arithmetic using widened intermediates; integer square root, trig,
  vector helpers, PCG-XSH-RR with reference seeding/output checks.
- Pure simulation package with no wall clock, network, rendering or floating
  point imports. Static analyzer rejects forbidden imports, floats, map
  iteration, goroutines/channels and reference-bearing state fields.
- Single-threaded tick simulation, canonical state bytes, BLAKE3 hashes outside
  the pure simulation package, bounded internal JSON recording/replay.
- Animated water, lava and toxic slime chosen from the seed, a visible void
  below the island, immediate death on crossing the lethal surface, and
  splash/death feedback. Cosmetic waves never alter the collision boundary.
- Terrain mask and crater updates, swept pixel probes, fall/explosion damage,
  turn rotation, retreat and resolution, elimination, sudden-death water.
- Initial bazooka, grenade, cluster, shotgun, uzi, punch, dynamite, timed mine,
  teleport, rope and jetpack implementations. These are initial mechanics;
  names do not imply every original PRD behavior is complete.
- Embedded comic sprites for eleven weapons/tools, clickable inventory with
  large hover previews, and thrown-explosive sprites. Static sprite textures
  have their own GPU cache; terrain updates cannot overwrite them.
- A 4,096-pixel default arena with a clipped scrollable viewport, pan/zoom,
  map-strip navigation, character/shot tracking and pointer-to-world mapping.
- Matching rocket, shotgun-pellet, Uzi-round and bomblet sprites. Instant-hit
  ray endpoints are transient effects and do not enter canonical state.
- Stakey vector renderer, navy/blue/turquoise lobby and arena, GPU terrain cache,
  dirty rectangles accumulated across simulation updates, offline PNG renderer.
- Internal desktop controls behind `desktop,dev` build tags. Ordinary desktop
  build has no route to the internal arena.
- Two- and six-squad frozen completed-match recordings. Tests compare stored
  hashes; updating simulation behavior requires reviewing fixture changes.
- Experimental binary turn codec with fixed domain/version, match/terms/roster
  binding, signature validation before replay, strict input order, bounded size
  and ticks, exact one-turn boundary and mandatory before/after hashes.
- Six peer states replay a whole signed fixture independently. Other tests cover
  tampering, wrong key/seat/context, stale messages and false supplied outcomes.
- SDK wire fragmentation/reassembly exercised with reversed and duplicate fragments
  of a completed signed turn. This is an offline codec test, not a BR connection.
- Experimental explicit gross payout matrix: every sole-winner outcome and draw
  has a pre-agreed allocation conserving the pot, with overflow checks and a
  policy hash. Custom allocations are tested through the SDK settlement builder.
- SDK script-engine characterization of 2–6-seat settlements, custom gross
  allocations, fees, missing signatures, and owner refund script branches.

## Engineering choices made explicit

The quarter-sine lookup table has **1025** samples: 1024 intervals plus an exact
endpoint for interpolation. Its generator is outside `pkg/sim` and is not run
automatically. Only committed integer samples enter the simulation.

The pure simulator emits canonical bytes; `pkg/replay` performs BLAKE3 hashing.
This resolves the original specification's conflicting requirement for BLAKE3
inside sim and no non-stdlib dependencies inside sim. Event order is explicit
through `(Tick, Sequence)`, never arrival order.

Version 3 is still experimental and not a published protocol. Neither JSON
fixture files nor ordinary Schnorr turn signatures authorize monetary actions.
`Terms` in the experimental turn context is a required opaque hash; it is not
proof that a complete production terms document has been agreed or funded.

Simulated input ticks cannot prove that a remote player respected a wall-clock
countdown. Real-time delivery and timeout agreement remain separate protocol
questions. A normal connected client can submit its expired turn; the verifier
does not invent another player's signature to handle absence.

## Outstanding before a complete game

- Complete weapon inventory, ammunition, utility items, crates, collapse rules,
  built-in maps, authored rope courses and balance qualification from the PRD.
- Rope corner behavior and terrain contact need broader gameplay fixtures.
  Mines are currently timed explosives; proximity mines are not implemented.
- Final animation, effects, audio, camera work, squad identification/accessibility,
  cosmetic setup, paid lobby and complete table screens.
- Actual SDK runtime/bridge adapter: invitations, UID/key binding, bond admission,
  seating beacon, approved funding, refunds and durable recovery.
- Production signed terms, payout UI, ranking/team outcomes, and fee/dust
  admission. The experimental matrix covers sole winners and explicit draws;
  it does not authenticate a final outcome or replace a complete terms schema.
- Persist-before-send signing journal and recovery from ambiguous sends/payments;
  asynchronous inbox persistence, reconciliation and session management.
- Safe off-chain missing-player agreement, cooperative payout signing and
  end-to-end independent deposit refunds. Guaranteed absent-loser winnings are
  explicitly outside the accepted release model; see payment-model.md.
- Any future forfeitable-bond scheme is separate from the accepted owner-refund
  model and must not undermine independent deposit recovery.
- Actual amd64/arm64 replay execution on independent runners, desktop interaction
  testing, Windows/macOS packaging, performance tests and independent security review.

## Local validation

`make check` runs race tests, vet, the simulation analyzer and compiles ordinary
and internal desktop builds. `make preview` and `make replay` exercise headless
visual output and frozen recordings. The actual GPU desktop window was also launched, captured to
`artifacts/stakewars-desktop.png`, visually inspected, and exited successfully.
This checks startup/drawing, not manual control coverage. Native amd64 and
32-bit x86 simulation/replay tests pass with the same frozen hashes; 32-bit
execution required an approved run outside the syscall-restricting sandbox.
ARM64 execution remains outstanding. Cross-compilation alone is not evidence
of cross-architecture determinism.

## Simulation version 2

The terrain now ends at seven-eighths of world height, with a lethal surface
at fifteen-sixteenths, leaving visible open space beneath the island. Crossing
that surface kills on the same tick. These gameplay changes bump the simulation
version and regenerate all three fixtures; version-1 recordings are rejected
rather than silently replayed under new rules. Tests cover falling through a
destroyed column, exactly one death effect, and immediate boundary crossing.

Internal desktop fixtures now choose a local random seed on restart unless
`-dev-seed` is supplied. This is not production multiplayer seed agreement.
The same seed determines terrain and hazard appearance for every replay.

## Simulation version 3 and terrain presentation

The single height profile has been replaced by original built-in polygon
silhouettes plus carved openings. Templates include hooked towers, arches,
cave floors, overhangs and detached platforms. Seeded mirroring/variation keeps
map creation reproducible. Spawn sites are chosen from exposed floors at
multiple levels, with full-body clearance and separation checks. The new map
geometry/spawn behavior bumps the simulation version to 3; the three completed
replay fixtures have been regenerated and older versions are rejected.

Three generated comic rock textures are embedded under `assets/terrain/` with
their prompts. They are sampled in world coordinates and clipped by the mask;
craters reveal the same material and remain exact. Stars, moon, distant shapes
and buildings use increasing parallax factors; sparse near-field silhouettes
move faster than terrain. These visual layers never feed back into simulation.

Top and bottom HUD bars collapse independently (chevrons or F1/F2); H toggles
both. Camera viewport bounds, mouse targeting, clipping and camera clamping
adapt together. Four-direction edge scroll and PgUp/PgDn support tall maps;
manual motion overrides automatic follow. Tests cover camera-coordinate
round trips, expanding the viewport, HUD exclusion and edge motion.

The Hedgewars clone/source review is recorded in `hedgewars-analysis.md`.
The separate ready-phase design is a recommendation, not implemented yet.

## Simulation version 4: arsenal and controls

The prototype now has 23 weapons/tools. The added twelve are mortar, bat,
bouncing bomb, drill rocket, parachute, homing rocket, airstrike, flamethrower,
remote charge, heavy cluster, bison bomb and girder. Heavy/targeted equipment
has per-squad ammunition; the UI shows remaining supply and skips empty items
when cycling. New state includes inventory, chosen fuse/target, projectile
age/drill budget/burning state and deployed parachutes. Canonical serialization
and cloning cover every added field. Three completed-match frozen fixtures were
regenerated for version 4; their turn counts/winners stayed the same.

J jumps, Backspace jumps higher, precision+J jumps backward. The precision
modifier slows aim and changes facing without walking. Space remains fire.
Keys 1–5 choose fuses. The paged tray and right-click grid expose every item;
target markers and invalid-girder feedback explain placement. K opens bindings;
settings save to the user config directory or `-controls` path. Opening either
panel keeps the simulation clock running. Instant weapons use a press; charged
weapons use hold/release. HUD labels follow customized keys.

Tests cover invalid-input atomicity, jump directions, parachute steering and
safe landing, bat knockback, ammunition depletion, missing targets, remote
manual/timeout detonation, girder exclusion, drilling, homing, bounce energy,
bison traversal and flame expiry. Each expansion weapon is carried through a
signed completed turn, binary encoding/decoding and independent verification
by three simulated peers; tampered fuse input is rejected. This remains a
local protocol test, not evidence of production network or settlement safety.

The new art and original generation prompts live under `assets/weapons/`.
Ammo quantities and weapon balance are provisional built-in rules requiring
playtesting before a paid release. Ready countdown, pickups, proximity mines,
full character animation and audio remain separate follow-up work.

Version-4 simulation, replay and signed-turn tests also pass when executed as
32-bit x86 binaries, using the same frozen state hashes as amd64. Execution
required the approved syscall-unrestricted runner. Desktop binding-load and
inventory-cycle tests pass with display access. The GPU build launched and
captured successfully; this is not comprehensive manual playtesting.

## Simulation version 5: readiness, environment and feedback

Completed the remaining requested features: a bounded ready phase with early
start, round-based weapon locks, persistent proximity mines, explosive barrels,
health/ammo pickups and seeded replenishment. Defaults are ReadyTicks=180,
Environment=true, WeaponDelays=true. Built-in desktop fixtures use these defaults;
some focused unit fixtures explicitly disable them. This does not add public
custom schemes or free-play modes.

The completed-turn verifier now requires the configured ready-phase head when
readiness is enabled. It rejects a substituted action-phase head. Its replay
still must terminate exactly at the next turn boundary. Frozen two/six-seat
matches and all expansion-weapon signed-turn tests were regenerated/revalidated.
New tests cover early readiness, preserved action time, blocked locked weapons,
idle/triggered mines, reverse-order barrel chains, pickup caps, falling objects,
construction exclusion, separate RNG consumption, canonical state and cloning.

Stakey animation is built on the existing vector renderer: stepping feet,
blinking, squash/stretch, rotating backflip, recoil, damage flash and floating
health deltas. Cosmetic state is isolated in render.Scene; tests check that
updates, sound cue draining and drawing do not alter replay hashes. Explosions
use weapon-family palettes and heavier weapons add a shockwave. The held sprite
matches the equipped item. Original synthesized PCM effects use Ebiten audio,
with 16 simultaneous voices maximum, distance attenuation and an M mute toggle.

The environment random stream is separate from projectile randomness, but its
seed is public/predictable. No claim of unpredictable or bias-resistant paid
loot drops is made. Deployment still needs agreed terms/seed/deadline binding,
settlement safety and independent review; these local improvements do not
resolve the protocol findings.

Validation for version 5: full race tests, vet, simulation analyzer and desktop
builds pass. Native amd64 and executed 32-bit x86 replay/turn verification pass
against the same regenerated fixture hashes. The desktop ran through a ready
beep with audio enabled and captured at frame 75 without an audio/render error.
Automated sound tests validate non-silent, bounded stereo PCM; speaker output
quality and full manual playtesting are not covered by those checks.

## Simulation version 6: faster jumps and facing

Visual timers advance at 3× speed, including the distance-driven walking cycle.
Character gravity is 3/8 per tick, up from 1/8; jump launch velocity stays the
same, reducing arc height/airtime to about one-third. Jetpack thrust is 5/8 to
retain lift. Tests bound normal jump height to 25–35 pixels and airtime to at
most 30 ticks; high/back jumps to 55–68 pixels and at most 42 ticks on flat ground.

Move/Face inputs now mirror aim horizontally toward the requested direction,
preserving elevation. Desktop input accounts for the mirrored angle before
applying simultaneous aim keys. Left-facing W/S input uses the corresponding
sign. Tests cover both directions, repeated direction, stop, elevation,
shortened arcs, jetpack lift and 3× visual timers. Frozen fixtures were regenerated
because changed trajectories/landing times can change match outcomes.

## Simulation version 7: downhill contact and projectile speed

Grounded walkers follow nearby floor down by at most four pixels per horizontal
substep. This occurs only after supported walking; airborne characters and
characters whose support was destroyed are never snapped to a floor. Tests
cover continuous diagonal descent in both directions, small steps, real ledges,
jumps and destroyed support.

Projectile motion now advances five swept steps per simulation tick. Explicit
explosive fuses and burning damage remain on the normal clock; internal flight
budgets and homing guidance advance with the motion steps. Tests verify exact
five-step displacement, one-pixel wall collision, three-second fuse duration,
and the existing weapon/environment behaviors. Frozen recordings are regenerated.
Desktop W/S angle adjustment increases from 160 to 480 units per tick, precision
from 24 to 72. Projectile camera smoothing increases independently of simulation.

## Projectile and precision tuning (version 8)

Projectile flight is restored to its original 1× rate, including homing and
internal flight timers. Swept collision and explosive fuse timing remain in
place. Normal W/S aiming stays at 480 angle units per tick; Shift precision
aiming is reduced to 24 (one-third of the previous precision speed and 1/20
of normal aim). Downhill walking and the current jump/animation tuning remain
active. Replay fixtures are refreshed for simulation version 8.

## Mining drill — simulation version 9

Added reusable MiningDrill as appended weapon ID 23, preserving previous IDs.
Holding fire emits existing Fire inputs; deterministic six-tick cadence removes
only a bounded body clearance and overlapping directional terrain circles.
No direct damage, ammunition cost, automatic movement or attack consumption.
Normal support, falling, hazard and turn-time rules apply. Inventory growth and
terrain behavior require version 9; all three frozen replays were regenerated.
The 24-slot arsenal, held sprite, recoil feedback and hints include the drill.
The generated sprite is embedded; no runtime asset downloads.

## Movement, feedback and recovery follow-up — simulation version 10

- Movement integration covers buried escape, shelf walking, jumping, rope attach,
  length changes, release and landing with two independent deterministic states.
  Ropes detach when their terrain support is destroyed. This behavior bumps the
  simulation to 10; the three frozen fixture transcripts were regenerated.
- Active Stakey markers, ready/retreat/result banners and a last-five-seconds
  warning remain visible with collapsed bars. Weapon tiles explain empty ammo,
  unlock delays and turn/attack locks. Noncharged tools no longer show power bars.
- Experimental turn journals durably reserve signing intent and retain verified
  frames. Recovery replays from the bound genesis. Future delivery is bounded to
  six turns, duplicates are idempotent, and signatures authenticate before queues.
  Lost future buffers are recovered by requesting frames from the local sync point.
- Recovery tests cover 2–6 peers, SDK fragment reassembly followed by restart,
  changed genesis, corruption, storage failure, exact resend and conflicting
  signing attempts. Immutable record publication has concurrent-writer tests.
- Funding intent tests cover crash/unknown outcomes, concurrent dispatch attempts
  and changed obligations. These are local components, not a connected funding UI.
- Poker's actual payout path was rechecked: N-of-N transaction signatures, distinct
  from checkpoint log signatures. See protocol-findings.md for the source trace.

Live dcrpulse integration, safe off-chain gameplay agreement, cooperative payout
and end-to-end owner refunds are still pending. Enforced absent-loser winnings
are no longer a release requirement; see payment-model.md. `make reliability` runs the offline recovery and payment checks;
a passing counterexample test means a known protocol problem was reproduced,
not that it was solved.

## Rope slope collision fix — simulation version 11

Reproduced reeling stalls on mirrored diagonal terrain. Rope motion now sweeps
free vertical and horizontal components when a diagonal substep is blocked,
rather than cancelling all motion. Each substep remains at most one pixel per
axis and checks the full body against terrain. Blocked reeling restores the
reachable rope length so failed shortening cannot accumulate into a later snap.

Regression tests cover both slope directions, independent replay agreement,
wall collision without clipping or stored shortening, and existing attach,
lengthen, release, landing, destroyed-anchor and open-air swing constraints.
Simulation version 11 invalidates older physics replays; all three frozen
fixtures were regenerated.

## Shallow-slope rope follow-up — simulation version 12

The version 11 axis-slide fix was insufficient on gentle inclines. New regression
fixtures use actual SelectWeapon/Aim/Fire attachment on 1-in-4 and 1-in-8 slopes
in both directions; all four failed against version 11. Tangential gravity could
cancel the reel's small upward displacement, leaving the body caught on a pixel.
While shortening against supported ground, rope motion now uses walking's bounded
1–4 pixel step clearance. Every intermediate vertical position and the horizontal
substep checks the full body, so walls and low ceilings still block movement.
Tests include independent peer replay and a too-short tunnel that must remain
impassable without changing terrain. All previous rope tests remain in place.

## Jetpack control tuning — simulation version 13

Reduced powered acceleration from 5/8 to 1/2 pixel per tick squared. Against
3/8 gravity this halves net upward acceleration from 1/4 to 1/8. Powered ascent
is capped at 3 pixels per tick instead of reaching the general 24-pixel cap.
Releasing thrust keeps normal gravity, stopping maximum powered ascent in eight
ticks. Tests cover sustained lift, speed cap and release. Frozen replays updated.

## Lobby settings and certificate bridge setup

Added a lobby cogwheel and settings tabs for keyboard/mouse bindings and bridge
credentials. The desktop accepts pasted IPv4/IPv6 address, port, client PEM
certificate/private key and pinned bridge PEM certificate. The private key never
enters the renderer; it displays a fixed masked summary. Explicit save uses an
atomic replacement and owner-only temporary file; startup loads settings but
never auto-connects. Clipboard reading runs off the render thread with bounded
output and timeout. Late clipboard and connection results cannot overwrite a
newer selection/configuration.

Uses the current SDK gRPC/mTLS Dial + Hello, not Poker's older bearer-token HTTP
transport. Hello verifies the returned game ID and mainnet network. An 8-second
connection deadline and periodic bounded Hello calls prevent a frozen UI and
clear connected status on failure. Disconnect cancels outstanding work and closes
the connection. No invite capabilities, event subscription, peer traffic or
spend RPCs are enabled in this milestone.

Validation: actual SDK loopback mTLS handshake; wrong pinned bridge certificate,
mismatched key pair, wrong game/network, config persistence and 0600 permissions;
desktop private-key masking, field replacement and stale-result tests. Full race
checks, static checks and desktop builds pass. Captured native Controls and Bridge
screens under artifacts. Live operator credentials/network were not available.

### Bridge event reception and projectile camera

The desktop now subscribes once to the SDK bridge event channel after mTLS
identity/network validation. Requests are answered with bounded deadlines;
refresh reports an empty joined-table list. Incoming invitations are strictly
parsed and shown as unverified previews, never reported as seats. Expired and
unsupported requests are rejected. No accept-invite/payment capabilities are
advertised, so dcrpulse may hide its accept action; actual table acceptance is
still gated. Invitations delivered through the bridge request API can be
previewed, but receive an explicit failure explaining that seating is disabled.
The latest preview is memory-only and clears on disconnect or credential edits.

Initial connection failures retry with bounded backoff. The SDK reconnects its
subscription and reports delivery gaps; the UI warns that invitation history
may be incomplete. Periodic ReportState calls indicate bridge RPC reachability,
not proof of an uninterrupted subscription. Hello is no longer called repeatedly
alongside frame reception (the SDK writes its identity fields during Hello).
No admitted tables exist yet, so unsolicited peer frames are discarded without
allocating assemblers or changing the simulation. Wiring admitted tables to the
durable turn journal remains outstanding.

Projectile tracking now overrides prior manual pan and pointer-edge scrolling
while a shot exists. Volleys track their center; burning remnants and placed
remote charges are excluded. After impact, normal character follow resumes.
This changes presentation only, with no simulation version or replay change.

### Shot controls, sky tracking and match results (simulation version 14)

Charged weapons fire automatically at 100%; a release latch prevents a held
Space key from charging another shot until released. Early release still fires
at the selected power. Collapsing the bottom bar keeps a compact power meter
and precision-key indicator over the stage for charged weapons.

Camera clamps now allow sky above the terrain while following projectiles,
including the renderer's camera copy. J launch speed increases from 5 to 5.5
vertical and 2 to 2.25 horizontal; Shift+J increases from 7 to 7.5 vertical and
2 to 2.25 backward. Backspace high jump is unchanged. Gravity is unchanged.

Drill rockets retain their ballistic velocity but travel up to 25% slower during
terrain penetration. A canonical six-tick drag counter restores flight speed
outside the surface. Collision sweeps, drilling budget and fuse remain active.
Simulation version 14 includes this state and the jump changes; frozen fixtures
were regenerated and replay-verified.

A full-screen match result now shows animated rays/confetti, a crowned winning
Stakey, squad survivors and remaining HP, turn count and simulated battle time.
Draws have their own presentation. Development matches offer Play Again; all
result screens offer Back to Lobby. The result does not claim a payment.
Preview: `go run ./cmd/stakewars-preview -victory -output /tmp/victory.png`
(or `-draw`).

### Table preparation UX and layered scenery

The lobby has a Table Lobby entry and a complete preparation screen: eight-stage
rail (invitation, admission bond, roster, seat draw, stake, table bond, map check,
ready), selectable 2–6 player cards, separate payment indicators, confirmation
counts, terms and advertised buy-in total, block-based deadline, bridge/chain
status and contextual next step. Verification, confirmation loss and stale data
remain distinct. A payment is verified only with the local-check flag, a positive
required depth, sufficient confirmations and the verified phase. Map agreement
is a separate readiness condition.

Live AcceptInvite requests automatically open this screen, carrying their GC and
terms. Heartbeats do not repeatedly reopen it; another operator request does.
ChainTip supplies the actual displayed height; inability to read the chain does
not become a zero-height success. The game still refuses paid admission and
reports no joined tables, and no live roster/funding verifier is wired here yet.
The visual confirmation and funded-player examples are explicitly fictional:
`make table-lobby` opens the interactive dev-only demo with next-state and 2/4/6
player controls. This is not an unpaid public match mode. Creation and invite
publication remain in dcrpulse and cannot complete until admission is implemented.

Generated scenery now includes six original PNG assets spanning four depth
layers and water/lava/slime theme variants. Assets and exact generation prompts
are in `assets/scenery/`. Horizontal speed differs per layer, vertical scrolling
moves distant scenery more slowly, and foreground decoration sits at the hazard
edge. Scenery is seed-selected and has no simulation side effects.

Collapsing both bars fits the highest terrain plus sky headroom down to the
hazard, retaining horizontal scrolling. Terrain extents are measured when the
viewport changes, not per frame. Manual camera controls and above-map projectile
tracking remain available. No simulation version change is needed for this work.

### Wide-map weapon range (simulation version 15)

Long-range weapons now match the 4096-pixel battlefield. Bazooka and drill rocket
launch at up to 16 px/tick with gravity 1/16 px/tick² (previously 12 and 1/8).
Mortar launches at up to 20 with gravity 1/12 (previously 16 and 1/8). Homing
rockets launch at up to 16 and steer toward 14 px/tick (previously 12 and 8).
Grenades, close-range weapons and fuse seconds retain their existing tuning.
Drill material drag still applies. End-to-end launch/flight tests require all
four weapons to cover more than 3500 pixels in both directions on a clear
4096-pixel test map at full power; wind and obstacles can reduce practical range.
Frozen replays are regenerated for version 15.

### Opening cover and wind particles

Generated and embedded a comic fisheye Stakey-soldier cover in `assets/cover/`.
Desktop startup draws it before building the local scene and decoding scenery;
when preparation completes it shows Enter/click to continue. No fake percentage
or artificial delay. Direct settings/table-demo launches bypass it; development
can use `-skip-cover`. `-cover -screenshot ...` captures the actual ready cover.

A generated transparent weather atlas supplies rain, leaves, embers and ash.
Far particles render behind terrain; near particles render in front. Accumulated
horizontal drift follows the sign and strength of simulation wind, including
zero wind and direction changes, while particle positions remain cosmetic.
The code does not change replay state or require another simulation version.

### Fullscreen overlay HUD, pickups and rope release (simulation version 16)

- The battlefield always renders edge to edge. HUD visibility no longer changes
  the viewport, zoom or world coordinate mapping. Compact translucent squad,
  status and arsenal panels overlay it; pointer actions over HUD panels do not
  target terrain or pan/zoom the map. The in-game HUD has no logo.
- A standalone transparent wordmark derived from the loading cover now brands
  the lobby, table room and victory view. Original assets and exact built-in
  imagegen prompts are kept under `assets/cover` and `assets/objects`.
- Generated health/ammo crates and explosive barrel replace geometric objects.
  Live Stakeys collect crates on body contact in the same simulation tick,
  including at full health or ammo capacity; HP/ammo limits and unlimited ammo
  remain intact. One pickup event is emitted per consumed crate.
- Rope tension only pulls when taut; slack no longer pushes the body outward.
  Terrain corners unwrap as soon as the direct segment clears, without requiring
  a winding-side crossing. Multiple obsolete corners release together while
  preserving total length; obstructed segments still retain their corner.
- Simulation version 16 invalidates older replay rules. Frozen fixtures were
  regenerated. Regression coverage includes immediate walking pickup, caps,
  single consumption, dead units, slack motion, retained blocked corners,
  multiple clear corners, slope reeling and independent canonical replay.

### Responsive rope steering (simulation version 17)

A/D now applies 1/4 pixel/tick² of steering acceleration (previously 1/24),
along the swing tangent while taut and horizontally while slack. This restores
prompt swing buildup and makes steering effective near the sides of the arc.
Outward velocity is removed within fixed-point constraint tolerance, preventing
rounding from retaining tension that the position constraint already resolved.
Corner release, slack motion, swept collision and existing speed limits remain.
Tests exercise left/right startup, opposite-input reversal, side-arc pumping,
constraint bounds and independent replay, alongside existing slope/corner tests.
Version 17 and regenerated replay fixtures bind peers to the changed rules.
