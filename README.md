# StakeWars

Repository and Go module: `github.com/karamble/dcrgaming-stakewars`.
The player-facing name, executable, existing profile locations and bridge
protocol identifiers are unchanged by the repository rename.

Turn-based peer-to-peer artillery combat with Stakey squads, destructible
terrain and Decred stakes. Asynchronous Bison Relay messages carry completed
turns; every participant independently replays them.

## Current build

The desktop connects to the SDK for invitation acceptance, bonded seating,
signed map agreement, wallet-approved stakes, asynchronous signed turns,
verified replay and cooperative payouts. `make acceptance` plays the whole path
against two wallets and two dcrpulse bridges on simnet; see
[docs/simnet.md](docs/simnet.md).

**Payout requires every player to sign.** A refusing player can veto winnings.
Each owner reclaims their original unspent deposits in dcrpulse, under Gaming
then Recovery, after the locks mature and less fees.
Offline-skip consensus is not implemented; a missing turn can stall the match.
See [first-playable instructions and limits](docs/first-playable.md).

Run independent identities with `-datadir PATH`, matching dcrpoker's parameter:

```sh
make desktop
/tmp/stakewars-bin/stakewars -datadir "$HOME/.dcrstakewars-player1" -settings bridge
/tmp/stakewars-bin/stakewars -datadir "$HOME/.dcrstakewars-player2" -settings bridge
```

`--appdata` / `-A` is the Decred-style alias for `-datadir`. First launch creates
`dcrstakewars.conf`; existing files are preserved and CLI options override INI
settings. Decred `slog` output goes to stdout and `logs/dcrstakewars.log` with
bounded rotation. Use `--debuglevel SESS=debug,SDK=debug` for protocol diagnostics.
See [OS defaults and configuration details](docs/first-playable.md).

`make multiplayer-demo` opens two isolated desktop peers with simulated funds.

- [Payment model](docs/payment-model.md)
- [Simnet acceptance harness](docs/simnet.md)
- [Implementation specification](impl-spec.md)

## Build and run

Requires Go 1.25 or newer, Ebiten's platform development libraries for desktop
builds, and the SDK sibling checkout:

```text
karamble/
  dcrgaming-sdk/
  dcrgaming-stakewars/
```

```sh
make check                 # race tests, vet, sim analyzer, both desktop builds
make play                 # build and launch the internal arena with seed 42
make play SEED=0          # choose a fresh random map seed
```

Internal arena controls: A/D walk, W/S aim, hold/release Space to fire charged
weapons (tap for instant weapons/tools), J jump, Backspace high jump, Shift+J
backflip. Hold left Shift for precise aiming or turn in place with A/D.
Keys 1–5 select the grenade/bouncing/cluster fuse in seconds.
Q/E cycle weapons; R selects the rope and Space attaches/detaches it. While
attached, W shortens the rope and S lengthens it instead of aiming; Shift does
not change rope speed. W provides jetpack thrust while selected, replacing aim-up. T selects teleport. Click the battlefield to choose a teleport, homing,
airstrike or girder target. Space confirms targeted attacks/construction.
Enter ends the turn; P or Esc opens the internal pause menu.

Right-click opens the full 24-item arsenal. The bottom tray has three pages,
with arrow buttons beside it. K opens key bindings: click an action and press
its new key; conflicting actions exchange bindings. Settings persist in the
user config directory under `stakewars/controls.json`, or a path supplied with
`-controls`. Camera/menu/fuse keys stay reserved. Opening the arsenal or
controls does not pause the turn timer.

The menu offers Resume and Back to lobby; the internal build can resume from
the lobby with Enter.
Water, lava or toxic slime animates beneath the island. Falling into it kills
Stakey immediately; the appearance is chosen from the match seed. Internal
fixtures use a fresh seed on restart; pass `-dev-seed 43` for reproducible lava
or `-dev-seed 44` for slime.

The default map is 4,096 pixels wide, with built-in arches, caves, overhangs,
isolated shelves and multiple spawn levels. Generated slate, basalt and mossy
rock textures follow the actual collision mask. Background layers scroll more
slowly than terrain; sparse foreground shapes scroll faster.

Click the top-right or bottom-right chevron to collapse that bar. F1/F2 toggle
them independently; H hides/shows both. Hidden bars expand the playable viewport
while a small timer strip stays visible. Move the mouse to the stage edges to
scroll in all four directions; PgUp/PgDn also pan vertically.

The map is horizontally scrollable. Left/Right pan horizontally, middle-drag
pans the camera, the wheel zooms, and C returns to Stakey. Click the map strip
to jump across the battlefield. The camera tracks the active character and
projectiles until manually panned; each turn starts focused on the next Stakey.
The initial window can be resized. The normal `desktop` build rejects `-dev-arena`.

```sh
make preview               # render lobby and arena PNGs without a display
make replay                # verify frozen two- and six-squad recordings
go run ./cmd/stakewars-sim -seats 6 -record /tmp/stakewars-run.json
go run ./cmd/stakewars-sim -verify /tmp/stakewars-run.json
```

Previews: [lobby](artifacts/stakewars-lobby.png), [arena](artifacts/stakewars-arena.png).
No wallet, bridge or network access occurs in these commands.

The local SDK replacement is development-only: `go.mod` replaces the module
with a sibling checkout, so the build follows whatever is in that working tree.
A portable immutable SDK dependency is still required for releases.
`-buildvcs=false` permits building this workspace, whose `.git` is empty.

## Verification boundaries

`internal/turnbatch` authenticates bounded binary completed-turn messages and
replays them against the agreed previous state. The tests exercise six peer
states over a whole recorded match. This is an experimental codec/verifier,
not production consensus, durable signing, BR transport, or escrow authorization.
`internal/payout` validates explicit custom gross allocations for every winner
and draw; production signed terms and the payout UI are still outstanding.

Tests named `TestCounterexample...` pass when they reproduce a limitation.
A green test suite does **not** mean responder-only timeout voting establishes
safe agreement, or that an absent player's payout signature is unnecessary.

## Artwork

The renderer creates vector Stakey characters with a notched ticket silhouette,
blue fold, expressive eyes and sci-fi equipment. References were inspected in
`../../pLabarta/dcrwebcomic`, especially `Proposal2/newchars/stakey.png` and
`Assets/characters/`. No comic bitmap has been copied. Generated comic weapon sprites are embedded
in the inventory, hover previews, and thrown-explosive rendering. Matching
rocket, pellet, Uzi-round and bomblet sprites are also embedded. Instant-hit
weapons emit visual shot effects along the simulated path; damage timing is
unchanged. Production
character animation and final effects remain outstanding.

Walking and jumping share the 45-second action timer; there is no separate
distance budget. Movement remains available during the 3-second retreat after
the attack ends. Press J once per jump; holding it does not auto-jump.

## Arsenal expansion

Twelve new weapons/tools join the original eleven: mortar, baseball bat,
bouncing bomb, drill rocket, parachute, homing rocket, airstrike, flamethrower,
remote charge, heavy cluster, bison bomb and girder. Each has comic inventory
art and matching projectile/effect art where applicable. Homing rockets (3),
airstrikes (2), remote charges (3), heavy clusters (1), bison bombs (2) and
girders (4) have a shared per-squad supply. Other equipment is repeatable in
this built-in prototype scheme.

Remote charges detonate on a second fire press or turn end. Girders require a
clear 80×12 area within 240 pixels, with no living Stakey or projectile inside.
Flames burn briefly; parachutes slow descent and prevent landing damage while
open. These are simulation-version-4 rules, covered by signed-turn replay tests;
old-version recordings are rejected. This does not resolve the outstanding
paid-match protocol and settlement gates.

[Full arsenal preview](artifacts/stakewars-arsenal-menu.png).

## Animation, sound and environmental rules

Stakey now has a walking foot cycle, blink, jump squash/stretch, backflip,
landing squash, weapon recoil and hurt reactions. Floating numbers show damage
and healing. The selected weapon is visible in Stakey's hands. Weapon-specific
fire sounds, blast sounds, footsteps, jumps, landings, pickups and ready beeps
are original synthesized effects. Press M to mute/unmute, or launch with
`-mute`. [Sound sample](artifacts/stakewars-sfx.wav).

Each turn starts with a three-second ready countdown. Any valid gameplay input
starts the action phase early; camera movement and opening panels do not.
The 45-second action budget remains intact. Homing rockets and airstrikes unlock
in round 2, heavy clusters in round 3; inventory tiles show the remaining wait.
A round here means the initial number of table seats in completed turns.

Mines arm after two seconds of active simulation and trigger within 32 pixels,
with a half-second flashing warning. Idle mines persist across turns without
preventing resolution. Barrels explode and can chain into other barrels/mines.
Health crates restore up to 25 HP, capped at starting HP; ammunition crates add
one charge of the weapon shown. Crates and barrels fall when their support is
destroyed and disappear in the bottom hazard. Crates cannot be built over.

The built-in environment starts with barrels and two crates. At most one extra
crate is placed per round, with at most four live crates and 128 total objects.
All placement uses a separate deterministic generator derived from the agreed
map seed. These drops are predictable, not secret randomness. Production seed
agreement and paid-match deadline/settlement work remain release gates.

The simulation is version 5. Readiness, unlock settings, all objects and their
random-generator state are included in replay hashes; old recordings are rejected.
[Environment preview](artifacts/stakewars-environment.png).

## Faster movement presentation (version 6)

Visual animation runs at 3× speed: walking feet, recoil, reactions, backflips,
explosion/splash effects and animated water. Stakey gravity is tripled, shortening
normal/high jump arcs to roughly one-third their former height and airtime;
jetpack thrust is adjusted to retain lift. Parachutes still cap descent speed.
Walking or precision-turning mirrors the aim horizontally while preserving
shot elevation. W/S aim correctly on either side. These gameplay changes use
simulation version 6 and refreshed replay fixtures. Run `make play` to try them.

## Slope walking and faster weapons (version 7)

Grounded walking now follows drops of up to four pixels per movement step,
matching the uphill step allowance. Bigger ledges, missing ground and jumps
still use falling physics. Projectile flight runs at 5× speed using five swept
movement steps per game tick. Selected grenade fuses, dynamite/remote timers,
and lingering flame damage keep their real-time duration. Internal flight
lifetimes advance with projectile motion. Camera following reacts faster to
shots. W/S aim is 3× faster, including the finer precision adjustment.

## Projectile and precision tuning (version 8)

Projectile flight is restored to its original 1× rate, including homing and
internal flight timers. Swept collision and explosive fuse timing remain in
place. Normal W/S aiming stays at 480 angle units per tick; Shift precision
aiming is reduced to 24 (one-third of the previous precision speed and 1/20
of normal aim). Downhill walking and the current jump/animation tuning remain
active. Replay fixtures are refreshed for simulation version 8.

### Mining drill (simulation version 9)

Select MINING DRILL from the right-click arsenal or the last slot on tray page 3.
Hold Space to dig, W/S to aim, and A/D to walk through the cleared passage.
The reusable drill clears an embedded Stakey's body and cuts up to 30 pixels
from its body center in the aim direction, at ten pulses per second. It causes
no direct damage and keeps the attack available; the turn timer keeps running.
Release Space to stop, then switch weapons to attack. Removing support can
cause falls or drops into the bottom hazard. All peers must use version 9;
older simulation replays are incompatible.

### Movement and recovery follow-up (simulation version 10)

Ropes release when supporting terrain is destroyed. The arena now highlights the
active Stakey and shows ready, retreat, result and urgent countdown banners even
with collapsed bars. Weapon tiles explain selection restrictions.

`make reliability` runs offline recovery and payment-script checks. Experimental
journals now support persisted turn replay, restart/resend, bounded reordered
messages, signing reservations and duplicate-funding prevention. Off-chain skip
agreement and an independent security review remain release blockers; this is
not a real-money-ready build.

### Lobby settings and bridge setup

Click the cogwheel in the lobby, or run `make settings`. The **Keyboard & Mouse**
tab shows current bindings and lets you click an action to rebind it. The
**Bridge Connection** tab uses the current dcrpulse/SDK mutual-TLS transport:

1. Register `stakewars` in dcrpulse and obtain its client certificate, client
   private key, and the bridge certificate.
2. Enter the bridge IP and gaming port (dcrpulse defaults to `8443`).
3. Click each credential field and paste the complete PEM contents with Ctrl+V
   (Command+V on macOS). Paste text, not a filename. Tab advances fields; Ctrl+A
   selects the field contents for replacement.
4. Click **Connect** to verify the secure handshake, game identity and network.
   **Save Settings** remembers credentials locally; **Disconnect** closes the
   connection. Use `-connect` to connect automatically at startup.

The network defaults to mainnet and there is no selector; for simnet or
testnet3, set `network` in the saved `bridge.json`. Registration in dcrpulse
also needs a gaming policy naming the account to spend from, and that account
cannot be a mixing account.

The private key is always masked in the UI. Settings are stored unencrypted in
`APPDATA/bridge.json` (default on Linux: `~/.dcrstakewars/bridge.json`)
with owner-only file permissions on Unix. Protect this file as a bridge credential.
Linux clipboard paste uses `wl-paste`, `xclip`, or `xsel`; macOS uses `pbpaste` and
Windows uses PowerShell's clipboard command.

The desktop now runs the SDK session rather than the invitation-preview handler.
Click Accept in dcrpulse chat to start bonded seating; the table room reports
verification and offers stake funding once all peers agree on the world.
See [the playable walkthrough](docs/first-playable.md) for funding and recovery.
No live operator wallet has been exercised by these local tests.

Shots automatically take camera focus even after manually scrolling the map.
The camera follows the projectile (or volley center) and returns to the active
Stakey after impact.


### Table room and scenery

`make table-lobby` opens the **fictional, interactive preparation demo**. Click a
player card for admission-bond and stake details. The demo also shows a
table-bond stage; live tables have no table bond, and an invitation carrying one
is refused. Use the
bottom controls to inspect 2/4/6-player layouts and preparation states, including
map verification and connection loss. Live invitations arrive by clicking Accept
in dcrpulse chat; no invitation copy/paste is needed. Live tables use the SDK
seating and funding flow described above.

`make play` uses the generated multi-layer comic scenery. Press H to collapse
both bars and fit the battlefield vertically; wide maps still scroll horizontally.
Sky, distant mountains, middle scenery and sparse foreground scroll at different
speeds. Water, lava and slime seeds choose matching scenery variants.

Full-power bazooka, drill rocket, mortar and homing shots now reach across most
of a 4096-pixel arena in clear conditions. This tuning is simulation version 15;
peers and replays must use the same simulation version.

### Fullscreen HUD and pickups

The map now fills the window behind translucent HUD overlays. F1/F2 toggle the
header/arsenal; H toggles both without resizing the map. Lobby branding uses the
standalone comic logo from the loading cover. Health and ammo crates collect on
body contact, including when already full (normal caps still apply). Rope corners
release when the direct path clears, and slack rope permits free motion.
These pickup/rope rules use simulation version 16 and updated replay fixtures.

Rope steering was further tuned in simulation version 17: A/D pumps along the
swing arc with stronger acceleration; opposite input brakes and reverses it.

### Accepted payment model

All gameplay and accusation consensus stay off-chain over Bison Relay. Payouts
require every participant’s transaction signature. If someone refuses, winnings
are not guaranteed; each participant must be able to recover its own unspent
deposits after the agreed locks, less fees. See [payment model](docs/payment-model.md).
