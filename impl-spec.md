# Implementation Spec — StakeWars

> [Approved decisions](docs/decisions.md) take precedence over conflicting
> details below. `internal/protocolcheck` is an offline characterization harness
> over the real SDK escrow scripts; its tests do not authorize a mainnet release
> or establish safe consensus under partitions.

**Audience:** the coding agent implementing this. Read end to end before writing code.
**Companion:** the PRD covers *why*. This covers *what to build and in what order*. Where they disagree, this document wins on mechanics and the PRD wins on scope.
**Covers:** M0 through M3 in executable detail. M4/M5 are sketched at the end.

---

## 0. How to use this document

Work milestone by milestone, top to bottom. Each task has an acceptance criterion; do not move on until it passes. §19 is a list of mistakes that are cheap to avoid now and very expensive to find later — read it twice.

Two rules that override everything else in this document:

1. **If a change would put a floating-point value, a map iteration, a wall clock, or a goroutine inside `pkg/sim`, the change is wrong.** Find another way. There is always another way.
2. **If you are unsure whether something belongs in the simulation or outside it, it belongs outside.** The sim is the smallest thing that can produce a deterministic game state. Everything else — rendering, audio, particles, networking, logging, UI — is outside.

---

## 1. Repository and toolchain

```
module github.com/karamble/dcrstakewars
go 1.25.0
```

Dependencies, pinned:

```
github.com/hajimehoshi/ebiten/v2        # rendering, input, audio
github.com/karamble/dcrgaming-sdk       # rails — see §14
lukechampine.com/blake3                 # state hashing, already in the SDK line
github.com/decred/dcrd/dcrec/secp256k1/v4
```

Nothing else without justification. In particular: no physics engine, no ECS framework, no math library. The simulation is a few thousand lines of integer arithmetic and it must stay readable enough to audit for determinism.

Layout:

```
cmd/<name>/            main, Ebiten Game impl, inbox drain
pkg/sim/               PURE — no I/O, no floats, no maps-in-iteration, no clocks
  fixed/               Q16.16 arithmetic, trig tables
  rng/                 vendored PCG, golden vectors
  terrain/             collision mask, stamps, flood fill
  body/                worm movement, collision probes
  projectile/          integrator, sub-stepping, bounce
  rope/                pivot push/pop solver
  weapon/              declarative table + behavior dispatch
  state.go             State, Input, Step, CanonicalHash
pkg/render/            Ebiten draw, dirty rects, camera, interpolation
pkg/audio/
pkg/game/              turn state machine, glue between sim and net (impure)
pkg/net/
  wschema/             game message kinds
  wlog/                hash chain (see §13)
  beacon/              anchor → seed → per-turn expansion
  lockstep/            TurnBatch apply and replay
pkg/mapgen/
pkg/scheme/
pkg/replay/
pkg/ui/
assets/                go:embed
internal/analyzer/     the determinism vet checks (§2.4)
```

`pkg/sim` imports **only** stdlib and its own subpackages. This is enforced by a test, not a convention (§2.4).

---

## 2. The determinism contract

Everything in the project rests on this. A desync is not a bug you fix later; it is a bug that makes the entire networked product non-functional and is nearly impossible to localize once the codebase is large. Build the enforcement before the game.

### 2.1 No floating point in the simulation

Go's specification permits an implementation to fuse `x*y + z` into a single fused multiply-add with one rounding step. arm64 and ppc64 do this; amd64 generally does not. **Identical Go source therefore produces different `float64` results on different architectures.** There is no compiler flag that disables it portably.

All simulation arithmetic is fixed-point Q16.16 in `int32`, with `int64` intermediates.

### 2.2 One PRNG, vendored and frozen

`math/rand` output is not stable across Go releases, and `math/rand/v2` differs again. `pkg/sim/rng` contains a hand-written PCG-XSH-RR with a golden-vector test. It is never seeded from the clock. It is never replaced or "upgraded".

### 2.3 Banned constructs inside `pkg/sim`

| Banned | Why | Use instead |
|---|---|---|
| `float32`, `float64` | FMA contraction (§2.1) | `fixed.F` |
| `for range` over a map | Go randomizes map iteration order | sorted slice, or a slice with a parallel index map used only for lookup |
| `time.Now`, `time.Since`, any `time` value | wall clock differs per machine | tick counts |
| `go`, `chan`, `select`, `sync.*` | scheduling is non-deterministic | single-threaded `Step` |
| `math/rand`, `crypto/rand` | see §2.2 | `rng.PCG` |
| `unsafe`, `reflect` | layout and iteration order | plain structs |
| pointer values in hashed state | addresses vary per run | indices into slices |

### 2.4 Enforcement (build this first, in M0)

Three CI gates:

1. **Import-graph test.** A Go test that walks `pkg/sim/...` with `go/packages` and fails if any import falls outside stdlib + `pkg/sim/...`.
2. **`internal/analyzer`.** A `golang.org/x/tools/go/analysis` pass run over `pkg/sim/...` that reports every item in the §2.3 table. Wire it into `go vet -vettool=`. This is ~200 lines and it will save you months.
3. **Cross-architecture hash job.** Run the golden replay fixtures on `amd64` and `arm64` runners and compare `CanonicalHash` output byte for byte. Fail on mismatch. This is the only gate that catches a determinism break the analyzer cannot see (a subtly order-dependent algorithm, for instance).

Add a race-detector job over `cmd/` and `pkg/game` too, once the inbox exists (§14.4).

---

## 3. `pkg/sim/fixed` — Q16.16

```go
// F is a fixed-point number with 16 fractional bits.
type F int32

const (
    Shift = 16
    One   = F(1 << Shift)
    Half  = One / 2
)

func FromInt(i int) F              { return F(i) << Shift }
func (f F) Int() int               { return int(f >> Shift) }          // floor
func (f F) Round() int             { return int((f + Half) >> Shift) }
func Mul(a, b F) F                 { return F((int64(a) * int64(b)) >> Shift) }
func Div(a, b F) F                 { return F((int64(a) << Shift) / int64(b)) }
func Abs(a F) F
func Min(a, b F) F
func Max(a, b F) F
func Clamp(v, lo, hi F) F
```

Notes the implementation must respect:

- `Mul` and `Div` promote to `int64` before shifting. Multiplying two `F` values in `int32` overflows at ±32768 in the integer part; map coordinates reach 4096 and velocities are small, but intermediate products do not.
- `Div` by zero is a programming error, not a runtime condition. Panic; do not return a sentinel. A sentinel silently diverges.
- Rounding is truncation toward negative infinity for `Int()` (arithmetic right shift), which is what you want for pixel addressing on both sides of the origin. Do not "fix" this to truncate toward zero — it makes `x=-0.5` and `x=+0.5` land in the same pixel.

### 3.1 Trig

No `math.Sin`. Generate a quarter-wave sine table of 1024 entries at **build time** into a `.go` file, as `[1024]F`, and commit the generated file. Angle unit is a `Angle uint16` covering the full turn (65536 units), so wrapping is free and exact.

```go
type Angle uint16
func Sin(a Angle) F
func Cos(a Angle) F     // Sin(a + 16384)
func Atan2(y, x F) Angle
```

`Atan2` is used for aim readout and rope geometry. Implement with the standard CORDIC or a polynomial approximation on `F` — whatever you choose, it must be table- or integer-based and covered by a golden-vector test.

**Commit the generated table.** Regenerating it at build time on a machine with a different libm changes the game.

---

## 4. `pkg/sim/rng` — PCG-XSH-RR

```go
type PCG struct{ state, inc uint64 }

func New(seed, seq uint64) *PCG
func (p *PCG) Uint32() uint32
func (p *PCG) Intn(n int) int       // rejection sampling, not modulo
func (p *PCG) Fixed() fixed.F       // uniform in [0, 1)
func (p *PCG) Range(lo, hi fixed.F) fixed.F
```

`Intn` must use rejection sampling. Modulo introduces bias, and more importantly two implementations that disagree about bias handling diverge on the same seed.

Golden-vector test: first 64 outputs for seed `(42, 54)`, committed as a literal array. If this test ever fails, the RNG changed and every replay in existence is invalid.

---

## 5. State and the step function

### 5.1 Contract

```go
// Step advances the simulation by exactly one tick.
//
// It is a pure function of (state, inputs). It performs no I/O, allocates
// no goroutines, reads no clock, and must produce byte-identical output on
// every architecture for every input sequence.
func Step(s *State, in []Input)
```

`Step` mutates `*State` in place rather than returning a copy — copying a 1 MB terrain mask 60 times a second is wasteful and the purity that matters is "no external influence", not "no mutation".

Tick rate is 60 Hz, matching Ebiten's default `Update` cadence. Never couple the sim to render frames.

### 5.2 Input

```go
type Input struct {
    Tick  uint32
    Seat  uint8
    Event EventKind    // uint8
    Param int32        // meaning depends on Event
}
```

Events: `MoveLeftDown`, `MoveLeftUp`, `MoveRightDown`, `MoveRightUp`, `AimUpDown`, `AimUpUp`, `AimDownDown`, `AimDownUp`, `Jump`, `Backflip`, `FireDown`, `FireUp`, `SelectWeapon` (Param = weapon ID), `RopeAdjust` (Param = ±1), `Teleport` (Param = packed x,y), `Detonate`, `SkipGo`, `Surrender`.

Inputs arriving for a tick are applied in ascending `(Tick, Seat, Event)` order. Sort explicitly before applying; never rely on arrival order.

### 5.3 State

```go
type State struct {
    Tick      uint32
    Turn      uint32
    Phase     Phase        // Placing, Playing, Retreating, Resolving, Ended
    PhaseTick uint32       // ticks elapsed in the current phase

    Terrain   terrain.Mask
    WaterY    fixed.F
    Wind      fixed.F

    Teams     []Team       // index = seat
    Worms     []Worm       // flat, stable order; Team holds indices
    Active    uint16       // index into Worms
    Projectiles []Projectile
    Crates    []Crate
    Rope      Rope

    Rng       rng.PCG      // advanced only by the sim
    Scheme    scheme.Ruleset
}
```

Rules for the state:

- **Stable ordering everywhere.** Worms, projectiles and crates live in slices whose order never depends on a map. Removal is by tombstone-and-compact at a fixed point in the tick, not by `slices.Delete` mid-iteration.
- **No pointers between state objects.** A worm references its team by index. A projectile references its owner by index. A pointer's value is an address and addresses are not deterministic.
- **Nothing outside the sim writes to it, ever.** Not the renderer, not the netcode, not the UI. If something outside needs to know a thing, it reads.

### 5.4 Canonical hash

```go
func (s *State) CanonicalHash() [32]byte
```

BLAKE3 over an explicitly written byte layout — field order fixed by hand, integer widths stated, no `encoding/gob`, no `encoding/json`, no reflection. The same reasoning the SDK gives for `Entry.SigningBytes`: two implementations must agree byte for byte or the comparison is worthless, and general-purpose encoders leave too much room to disagree.

Include: tick, turn, phase, phase tick, terrain mask, water, wind, every worm (position, velocity, HP, flags), every projectile, every crate, rng state, active index.

Exclude: anything presentational. Particles, camera, sound state, animation frames. If it is excluded from the hash it must also be excluded from `State`.

Hash cost at 4096×2048 is ~1 MB per call, which is fine at turn boundaries and at a few intra-turn checkpoints (§12.3). Do not hash every tick in production; do hash every tick under a `-tags simdebug` build, because it turns "these two peers diverged somewhere in this turn" into "these two peers diverged at tick 1,447".

---

## 6. `pkg/sim/terrain`

### 6.1 Representation

```go
type Mask struct {
    W, H   int
    bits   []uint64        // row-major, W rounded up to a multiple of 64
    stride int             // words per row
}

func (m *Mask) Solid(x, y int) bool
func (m *Mask) Set(x, y int)
func (m *Mask) Clear(x, y int)
func (m *Mask) StampCircle(cx, cy, r int, solid bool) Rect
func (m *Mask) StampRect(r Rect, solid bool)
func (m *Mask) Hash() [32]byte
```

Out-of-bounds reads return the border rule from the scheme: either `true` (indestructible walls) or `false` (open sides). Decide once, at construction, and store it — do not branch on scheme inside the hot probe.

The visual layer is **not here**. It lives in `pkg/render` and is never consulted by the sim. Keeping them in separate packages is what prevents someone reaching for the pretty version during a collision check.

### 6.2 Destruction

`StampCircle` returns the dirty `Rect` so the renderer can re-upload only that region. The sim does not care about the return value; the caller in `pkg/game` forwards it to the renderer.

Crater rims, scorch marks and debris are all renderer concerns.

### 6.3 Collapse (scheme option)

When enabled, after each blast: flood fill from anchor rows (the map borders, or girders marked as anchors), and any solid region not reached falls. Implement falling chunks as a bounded number of debris bodies rather than as a moving mask region — a moving mask is a determinism and performance trap.

Default scheme has collapse **off**, matching Worms Armageddon. Implement it in M4, not M2.

---

## 7. `pkg/sim/body` — worm movement

Worms are not rigid bodies and there is no general solver. Movement is per-pixel stepping.

### 7.1 Walking

Per tick, while a move key is held:

1. Compute the target x (current ± walk speed, in whole pixels — accumulate the fractional part in `F` and step when it crosses 1).
2. For the step, probe upward from the target position: if the column is solid at foot level, try stepping up one pixel at a time to the slope-climb limit (default 4 px). If a clear standing position is found within the limit, move there.
3. If not, the step is blocked; the worm does not move.
4. If the target position has no ground beneath it within 1 px, the worm begins falling.

### 7.2 Falling

Gravity accumulates velocity in `F`. Each tick, sub-step the vertical movement one pixel at a time and probe for ground. On landing, compute fall damage from the total drop distance if it exceeds the threshold, cap it at the scheme maximum, and zero the velocity.

Fall damage is computed from **distance fallen**, tracked as a state field, not from impact velocity. Velocity-derived damage is more "physical" and much harder to keep identical across the sub-stepping boundary.

### 7.3 Water

A worm whose y exceeds `WaterY` drowns: removed from play, fixed animation, damage lethal regardless of HP. Drowning is resolved in the resolution phase, not mid-projectile.

### 7.4 Knockback

An explosion applies a ballistic velocity to every worm within the blast radius: direction from blast centre to worm centre, magnitude falling off linearly from full at the centre to zero at the radius. The worm then behaves as a falling body until it lands.

---

## 8. `pkg/sim/projectile`

### 8.1 Integration

Semi-implicit Euler at the sim tick:

```
v += (gravity + windIfAffected) * dt
p += v * dt
```

with `dt` = one tick, folded into the constants rather than carried as a variable.

### 8.2 Sub-stepping — required

A projectile moving faster than 1 px per tick will tunnel through thin terrain. Split the movement:

```go
steps := int(fixed.Abs(v.X).Max(fixed.Abs(v.Y)).Round()) + 1
if steps > 32 { steps = 32 }
```

Advance position by `v/steps` and probe collision after each sub-step. The cap at 32 bounds worst-case cost; a projectile fast enough to need more than 32 has other problems.

### 8.3 Bounce

On contact, estimate the surface normal by sampling the mask in a 3×3 (or 5×5 for smoother results) neighbourhood and taking the gradient of solidity. Reflect velocity about that normal, then apply per-weapon restitution and tangential friction.

The normal estimate is approximate and that is fine — what matters is that it is *identically* approximate everywhere. Write it once, in one function, and call it from every bouncing weapon.

### 8.4 Fuses and chains

Fuse timers count down in ticks. Explosions during the resolution phase can trigger further explosions; resolve to quiescence, with a hard cap of 60 seconds of sim time (3600 ticks) after which everything remaining detonates at once. Without the cap, a dense mine field can make a turn effectively infinite.

---

## 9. `pkg/sim/rope` — the hard part

The ninja rope is what competitive players judge the game on. Budget real time for it, prototype it in M2, and test it against fixtures.

### 9.1 Model

The rope is a polyline: an anchor point, zero or more intermediate pivots, and the worm at the free end. Only the last segment is dynamic.

```go
type Rope struct {
    Active  bool
    Pivots  []Point     // Pivots[0] is the anchor
    Length  fixed.F     // current length of the free segment
    Attached bool
}
```

### 9.2 Firing

Raycast from the worm along the aim angle, one pixel at a time, to the maximum rope range. First solid pixel becomes `Pivots[0]`. No hit within range = no attachment, and the rope retracts.

### 9.3 Swing

Each tick, with `P` = last pivot and `W` = worm position:

1. Apply gravity to the worm's velocity.
2. Project the velocity onto the tangent of the circle centred at `P` with radius `Length` — that is, remove the radial component. The rope is inextensible; it does not spring.
3. Advance the position, then correct it back onto the circle exactly (normalize the `P→W` vector to `Length`). Do the correction every tick without exception; drift accumulates and diverges.

### 9.4 Wrapping — push

After moving, probe the segment from `W` to `P` pixel by pixel. If it intersects terrain, insert a new pivot at the last non-solid pixel adjacent to the contact, set `Length` to the distance from that new pivot to the worm, and continue.

### 9.5 Unwrapping — pop

Check the angle at the last pivot between the incoming segment (`Pivots[n-1] → Pivots[n]`) and the outgoing one (`Pivots[n] → W`). When that turn becomes convex — the rope would leave the corner — pop the pivot and restore `Length` to the distance from the new last pivot to the worm.

This push/pop pair is the entire feel of roping. Get the pop condition wrong in either direction and the rope either sticks to corners it should release or slips off corners it should hold, and in both cases experienced players will reject it immediately.

### 9.6 Length adjustment

Up/down keys change `Length` at a fixed rate per tick, clamped to `[minLength, maxRange]`. Changing length does not change position; it changes the constraint the next tick enforces.

### 9.7 Fixtures

Author at least six rope courses as committed test maps, each with a recorded input sequence and an expected final state hash. Any change to the rope solver that alters a fixture hash is a feel change and must be reviewed as one, not merged as a refactor.

---

## 10. `pkg/sim/weapon`

Weapons are declarative data plus a small set of behaviors.

```toml
[bazooka]
id = 1
behavior = "ballistic"
damage = 50
radius = 40
wind_affected = true
sprite = "bazooka"
[bazooka.impact]
kind = "explode"
```

```go
type Def struct {
    ID           uint16
    Behavior     BehaviorID
    Damage       int32
    Radius       int32
    WindAffected bool
    Restitution  fixed.F
    Friction     fixed.F
    FuseTicks    uint32
    Cluster      *ClusterDef
    // ...
}
```

Behaviors are a fixed enum implemented in Go: `ballistic`, `bouncing`, `hitscan`, `melee`, `airstrike`, `placed`, `utility`, `guided`, `directed` (sheep, mad cow). Tier 2 weapons overwhelmingly reuse Tier 1 behaviors with different data.

The table is embedded with `go:embed`, parsed once at startup into a `[]Def` indexed by ID, and **its content hash goes into the match parameters** (§14.5). A peer with a modified table cannot join a vanilla match — this is what makes the whole declarative approach safe.

Parse at startup, outside the sim. The sim receives the parsed `[]Def` and only indexes into it.

---

## 11. Turn state machine

Lives in `pkg/game`, not `pkg/sim` — it decides *when* a turn ends, which requires knowing about the network. The sim exposes what it needs.

```
Placing    → (all worms placed) → Playing
Playing    → (fire | timeout | death) → Retreating
Retreating → (retreat timer | fire) → Resolving
Resolving  → (quiescent | 3600-tick cap) → turn_end
turn_end   → (win condition) → Ended
           → otherwise → Playing (next seat)
```

Quiescence test: no projectiles in flight, no worm airborne, no fuse burning, no terrain settling, no damage animation pending. Implement as a single `func (s *State) Quiescent() bool` in the sim so both the local player and the replaying observer agree on exactly when a turn ended.

Sudden death at the scheme's turn count: water rise per turn, or all worms to 1 HP, or both.

---

## 12. `pkg/net/beacon`

### 12.1 Anchor

At formation the table agrees on a future block height `h`. `Game.BlockHash(ctx, h)` returns its hash; that is the match anchor `A` (32 bytes). Record it in a chain entry.

Require a depth margin before the match starts — a handful of confirmations — so a reorg does not invalidate the anchor mid-match. A disagreeing anchor is abandonment, never something to renegotiate in-band.

### 12.2 Derivation

```go
func MapSeed(a [32]byte, matchID, schemeHash string) uint64
func TurnBeacon(a [32]byte, matchID string, turn uint32) [32]byte
```

`TurnBeacon = BLAKE3(A ‖ matchID ‖ turn)`, domain-separated with a fixed prefix string. It drives wind, crate type, crate position, crate contents, and any weapon-internal randomness for that turn.

This is a pure function of the anchor, so there is **no per-turn randomness traffic at all**. Every peer derives the same beacon independently. Do not send it.

### 12.3 Checkpoints

Hash the state at fixed intra-turn points — turn start, on fire, and every 600 ticks during resolution — and carry those hashes in the `TurnBatch`. A divergence then localizes to a window instead of to a whole turn.

---

## 13. `pkg/net/wlog` — the transcript chain

**Do not use `gamelog` from the SDK as it stands.** Its `Chain.Next(seat, hand uint64, street Street, action Action, amount int64)` and its `Entry` fields (`Hand`, `Street`, `Action`, `Amount`) are hold'em-specific. dcrbattleships hit the same wall and wrote `pkg/bslog`.

Build `wlog` on the battleships shape:

```go
func (c *Chain) Next(seat uint32, kind Kind, body []byte, height uint32, anchor [32]byte) *Entry
```

with `Entry` carrying `Version, PrevHash, Seq, Seat, Signer, Kind, Body, Height, Sig`. Everything else — `Append`, `Head`, `Signer`, `Entries`, `AttestHead`, `ConflictingHeads`, `EquivocationProof.Verify`, `RecoverKey`, `RecoverKeyFromHeads`, `Marshal`/`Unmarshal`, `GenesisHash` — copy the semantics from the SDK's `gamelog` exactly. It is the same construction with an opaque body.

`SigningBytes` must be a hand-written canonical layout with fixed field order and stated integer widths. No `gob`, no `json`, no reflection.

**Raise the generalization upstream.** This is the third copy of equivocation recovery in the ecosystem (poker, battleships, this). The right fix is promoting the generic form into the SDK's `gamelog` and expressing poker's `street/action/amount` as a body type. Flag it; do not silently fork and move on.

`Height` follows the SDK's own rule: recorded, not enforced. A verifier may check heights do not go backwards and nothing more. It is not a turn timer (§14.6).

### 13.1 Entry kinds

| Kind | Body | Class |
|---|---|---|
| `w.anchor` | agreed height + resulting hash | `ClassForm` |
| `w.params` | scheme hash, weapon-table hash, sim version, map descriptor | `ClassForm` |
| `w.place` | manual worm placement | `ClassTurn` |
| `w.turn` | `TurnBatch` (§14.5) | `ClassTurn` |
| `w.ack` | turn number + state hash | `ClassState` |
| `w.evidence` | equivocation proof or divergence report | `ClassDispute` |

---

## 14. SDK integration

### 14.1 What the game implements

The game implements `runtime.Rules`:

```go
type Rules interface {
    Identity() connect.Identity
    Terms(sid string) (membership.Terms, error)
    Handle(ctx context.Context, in Message) error
    State(ctx context.Context) State
}
```

`Handle` receives one frame. The runtime has already framed, routed, reassembled and **checked the sender** — `Message.From` is an authenticated identity taken from the channel and the envelope, never from anything inside the body. Errors returned from `Handle` are logged and the message dropped; the runtime never retries, because a runtime that retried on the game's behalf would replay moves. Missed traffic is recovered from Bison Relay group history, not by asking peers.

`State` is called from the bridge's request loop. It must not block on game locks for long and **must not call back into the runtime**.

### 14.2 What the game receives

`runtime.Game` is the runtime as the game sees it:

| Method | Use here |
|---|---|
| `Send(ctx, match, kind, body, class)` | every outbound entry, with the class from §13.1 |
| `Settle(ctx, match, Outcome{Shares, Void})` | match end; `Void: true` for a desync (§14.7) |
| `Chain(ctx)`, `BlockHash(ctx, height)` | anchor (§12.1) and duty clocks |
| `Seat`, `Seats`, `LogSeats` | the edge filter (§14.3) |
| `LogKey(match)` | signs every chain entry |
| `MatchID(match)` | binds the chain and all evidence |

Two properties to design around: **a game cannot broadcast** — every chain write goes through the runtime, deliberately, because a game that could broadcast could pay somebody. And **log keys and session keys are separate** — the game signs what it says happened with `LogKey`; the session key holds stake and never leaves the runtime.

### 14.3 The edge filter

The group chat is wider than the roster. `Message.From` is an authenticated BR
UID, while `Seats` holds session public keys. The original `seatOf(seats,
in.From)` pseudocode is invalid without an explicit verified binding.

Before gameplay mutation, require the expected match/turn context, bounded
decoding, a claimed seat in the agreed roster, and a valid signature under that
seat's appropriate key (`LogSeats` for the transcript). Keep transport limits
separate from gameplay state. The current runtime's session admission check is
not proof that a BR sender is seated. Any UID/session-key binding is additional
protocol work, not assumed SDK behavior.

**This is a determinism requirement, not just a security one.** A rejected frame must change *nothing*: no sequence counter, no held-frame queue, no timer. dcrbattleships buffers frames arriving before a board exists; a junk frame accumulating in that queue on one peer and not another is a desync with no cheating involved. Filter before any queue and before any parse that mutates, and run the identical filter on every peer.

Rate-limiting a spamming spectator is local policy and may differ per peer — safely, precisely because dropped junk affects nothing downstream.

### 14.4 The inbox drain — the one concurrency hazard

One process: Ebiten's loop on the main goroutine, the runtime and bridge on background goroutines.

```go
func (g *Game) Update() error {
    g.drainInbox()        // exactly here, exactly once
    g.applyLocalInput()
    sim.Step(&g.state, g.pending)
    return nil
}
```

`drainInbox` reads a buffered channel until empty and applies whole `TurnBatch`es. Nothing else in the program applies network state. `pkg/sim` never sees a channel, a mutex or a goroutine.

Get this wrong — apply a frame mid-step, or from the background goroutine — and you get a desync that reproduces once a week and cannot be bisected. The import-graph test (§2.4) plus a race-detector CI job are the guards.

### 14.5 `TurnBatch`

```go
type TurnBatch struct {
    Turn        uint32
    Inputs      []Input      // full turn, tick-stamped, sorted
    Checkpoints [][32]byte   // §12.3
    StateHash   [32]byte     // post-resolution
}
```

Baseline is **one batch per turn**, emitted at turn end with `ClassTurn`, plus a small `w.ack` from every other seat. There are no reveals to carry — the beacon is derived from the anchor (§12.2). A few hundred messages per match.

Observers receive the completed turn asynchronously and replay it deterministically. No live-chunk mode. SDK message-size chunking is a transport feature, not live gameplay streaming. Do not promise a fixed RTT or globally ordered deliveries.

Match parameters (`w.params`) must bind `sim_version`, built-in content/rules hashes, map descriptor and payout terms before funding. This prevents version mismatches but does not rule out simulation bugs. The binding to the funded membership requires implementation and tests.

### 14.6 Two clocks — do not conflate them

| | Unit | Enforced by | Missing it means |
|---|---|---|---|
| Game clock | ticks | peer consensus in the sim | you lose your turn; the worm does not fire |
| Duty clock | block heights | refusing to co-sign | you stopped playing |

Decred blocks average five minutes and arrive in a Poisson process. A 45-second turn timer cannot be derived from them — the SDK says so in `Entry.Height`'s own documentation, and it is right. A missed turn is ordinary play. Only sustained silence, several blocks wide, justifies withholding a signature. Conflate these and every slow player loses their stake to a refusal.

### 14.7 Disputes

Two failures look identical on the wire and have opposite verdicts:

| Symptom | Cause | Verdict |
|---|---|---|
| two conflicting heads signed by the **same** seat | equivocation | retain the recovered key as evidence; withhold co-signing |
| different state claims without same-seat equivocation | disagreement requiring replay and protocol diagnosis | retain evidence; no automatic refund or slash |

**Test for equivocation first**, and only fall through to divergence. Treating an honest desync as an attack is the worst available outcome.

There is no forfeiture execution. A seat that equivocates publishes a key, and that key is evidence, but nothing in the runtime spends against it: the only response available is refusing to co-sign a payout, which leaves each seat its own deposit at the refund lock. Replay of agreed inputs can check a state claim, but majority voting is not proof of correctness and does not create a spendable settlement. Automatic voiding on a fabricated disagreement is disallowed.

### 14.8 Spectators

A spectator has no seat and no `LogKey`, and needs neither. The chain self-verifies against the roster and `GenesisHash(matchID)`, and the sim is deterministic, so replaying the transcript yields the same state a player holds. Late joiners pull a marshalled transcript from anyone at all — the source need not be trusted, because a bad transcript fails verification rather than producing a wrong view.

Spectating requires synchronization, content and finality verification as well
as §14.3. It does not require a spectator to have a signing seat.

---

## 15. `pkg/render`

- Layers: sky → parallax background → terrain visual → water bands → entities → particles → HUD.
- Terrain visual is an `*ebiten.Image`. Re-upload only dirty rects returned by `StampCircle`; coalesce overlapping rects per frame; target ≤ 4 sub-image uploads per frame at 4096×2048.
- Camera follows the active worm and airborne projectiles, zoom 0.5×–2×.
- Sprites are palette-swapped per team from a single atlas generated at build time, not duplicated per colour.
- **Interpolate.** Sim runs at 60 Hz; draw at display rate with positions interpolated between the last two sim states. Keep the previous tick's positions in the renderer, never in `State`.
- Particles are presentational, spawned from sim events, never feeding back. They may be frame-rate dependent and non-deterministic — that is correct, and should carry a comment saying so, so nobody later "fixes" them into the sim.

Budget: 4096×2048, 48 worms, 200 particles, ≤ 4 ms CPU and ≤ 8 ms GPU per frame on Intel Iris Xe class integrated graphics.

---

## 16. Testing

| Layer | Test |
|---|---|
| `fixed` | golden vectors for `Mul`, `Div`, `Sin`, `Atan2` |
| `rng` | first 64 outputs for a fixed seed, committed literal |
| `terrain` | stamp/probe round-trips; mask hash stability |
| `projectile` | no tunnelling through 1-px walls at max velocity |
| `rope` | six committed courses, input sequences, expected final hashes |
| `sim` | replay fixtures: input transcript in, final `CanonicalHash` out |
| cross-arch | every replay fixture on amd64 **and** arm64, hashes compared |
| `wlog` | equivocation detection, key recovery, marshal/unmarshal round-trip |
| net | `gaming/bridgetest` — full match flow with no live bridge |
| filter | junk frames from non-seats and seat-spoofers change no state hash |

The replay fixture is the workhorse. Every bug that produces a wrong game state should become one.

---

## 17. Milestones and acceptance

### M0 — Determinism harness (~3 weeks)

1. `fixed` with golden vectors, committed generated sine table.
2. `rng` with golden vectors.
3. `State`, `Input`, `Step` skeleton (no gameplay), `CanonicalHash`.
4. Import-graph test.
5. `internal/analyzer` covering every row of §2.3, wired to `go vet`.
6. CI: amd64 + arm64 runners, replay fixture hash comparison.

**Accept when:** a trivial fixture (one falling worm, 600 ticks) produces an identical hash on both architectures, and a deliberately introduced `float64` in `pkg/sim` fails CI.

Nothing is playable. Everything downstream depends on this. Do not skip ahead.

### M1 — Local prototype (~6 weeks)

Terrain mask and destruction; walking, falling, fall damage; bazooka and grenade; wind; turn loop; internal two-team fixtures; placeholder Stakey art.

**Accept when:** an internal two-team match reaches a winner and replays from its input transcript to the same final hash. This is a development harness, not a public unpaid game mode.

This is the first "is this fun" checkpoint. Answer that question honestly before continuing.

### M2 — Core feel (~8 weeks)

Rope with the push/pop solver and its six fixtures; jetpack; full Tier 1 weapon set; camera; particles; crates; water rise; real art direction pass.

**Accept when:** experienced Worms Armageddon players try the rope and do not immediately reject it. This is a human acceptance test and there is no substitute for it.

M2 is a legitimate stopping point if scope proves wrong.

### M3 — Netplay (~4–5 weeks)

`connect` handshake and `runtime` wiring; `wschema` kinds; `wlog`; `TurnBatch` batching; anchor beacon; edge filter; inbox drain; desync detection; replay verify; `Settle` and co-signing paths.

**Accept when:** peers complete testnet matches with matching verified states;
corruption is detected and retained without automatically refunding a malicious
loser; the reviewed skip and payment finality rules hold under delayed messages
and absent players; a spectator cannot affect gameplay.

Short because transport, encryption, identity, discovery, bonds and dispute handling are all imported. The work is game protocol and the inbox boundary.

Measure asynchronous bridge delivery here to validate the reviewed countdown, grace and voting behavior. No live-chunk mode.

### M4 / M5 (sketch)

M4: Tier 2 weapons, built-in maps/rulesets, Stakey squad customization, 6-seat matches and terrain collapse. M5: built-in audio/voices, UI, packaging, accessibility, performance and release review. No content editors or public free-play/training modes.

---

## 18. Open items the agent must not invent answers to

These are decisions for the project owner. If one blocks you, stop and ask rather than picking.

1. `gamelog` generalization upstream vs a third local fork (§13).
2. Whether `persist-referee` in dcrpoker is the sim-replay adjudication mechanism to reuse.
3. **Resolved:** paid-only public matches. Safe payout and skip finality remain release gates.
4. **Resolved:** original competitive rope feel.
5. Map resolution ceiling above 4096×2048.
6. **Resolved:** Stakey squads, Decred colors, polished sci-fi artwork generated and refined against the comic references.
7. **Resolved:** StakeWars; module `github.com/karamble/dcrstakewars`.

---

## 19. Traps

Read this list again before each milestone.

1. **A `float64` in `pkg/sim`.** Even in a "harmless" helper. Even in a comment-out-later debug line.
2. **`for k := range someMap`** in simulation code. Go randomizes it. Your two peers will disagree.
3. **`time.Now()` anywhere in the sim.** Turn timers are tick counts.
4. **Applying a network frame outside the drain point.** §14.4.
5. **Letting a rejected frame touch a queue, counter or timer.** §14.3.
6. **Using the visual terrain layer for a collision probe.** They are different packages for this reason.
7. **Pointers between state objects.** Addresses are not deterministic. Use indices.
8. **Skipping sub-stepping** because the bazooka looked fine. It tunnels at high power with wind.
9. **Treating a missed turn timer as an accusable lapse.** §14.6.
10. **Treating an honest desync as cheating.** Test for equivocation first. §14.7.
11. **Hashing state with `gob`/`json`/reflection.** Hand-write the layout.
12. **Regenerating the sine table at build time.** Commit it.
13. **Feeding particles back into the sim.** They are decoration.
14. **"Fixing" `Int()` to truncate toward zero.** §3.
15. **Copying the SDK's `gamelog` wholesale** and discovering three weeks later that `Street` and `Hand` mean nothing here.
