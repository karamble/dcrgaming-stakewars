# Hedgewars reference analysis

Inspected the official GitHub mirror at commit
`85685825d924cf08b2becfdac61bccbaf4dbd9dd`, cloned to
`/home/user/go/src/github.com/hedgewars/hw`. This is source analysis; Hedgewars
itself was not built or played during this review. The Worms screenshots in
Downloads were inspected separately as visual references.

## Turn flow: ready, move/aim, attack, retreat, resolution

Hedgewars initializes a distinct ready countdown for human turns. Its engine
initialization sets `cReadyDelay` to 5000 milliseconds, although game settings
can override this. During readiness, the clock decrements readiness instead
of the main action time. Camera-only movement does not end readiness; other
bound inputs can send an explicit ready-abort command. This is a short chance
to locate the active character, not an unlimited movement phase.
[Turn initialization](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uTeams.pas#L470),
[clock](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uGears.pas#L582),
[ready input](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uInputHandler.pas#L236).

Walking and aiming belong to the action phase. Left/right controls facing and
walking; up/down changes aim. A precision modifier slows aiming and permits
turning without walking. Normal, high and back jumps provide different ways
to traverse terrain. Weapons distinguish charged attacks from immediate ones.
[Movement and aiming](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uGearsHedgehog.pas#L847).

Attack completion is weapon-dependent. Metadata includes shots per turn,
whether using the item ends the round, and retreat duration. Multiple-shot
weapons can remain in the action phase. A final attack normally switches to
get-away time. Once objects become inactive, the engine goes through damage,
cleanup, reactions, win checks, water rise and other between-turn processing.
[AfterAttack](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uGearsHedgehog.pas#L603),
[resolution](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uGears.pas#L343).

**StakeWars application:** label the existing action phase “Move & aim”, keep
retreat and resolution visually distinct, and add a bounded ready phase as a
future rules change. Ready duration and early-start events must be included
in deterministic replay and in the agreed deadline rules. Do not pause a paid
turn because a local menu is open. The prototype currently retains its agreed
45-second action / 3-second retreat defaults; this review does not silently
add time to the financial protocol.

## Terrain structure and presentation

Hedgewars selects among outline-template, maze, Perlin and wave-collapse map
generators. The outline generator starts with templates, can mirror/flip,
distorts outlines, optionally smooths them with curves, then draws and fills
them. This naturally supports islands and caves instead of assigning exactly
one surface height to every X coordinate.
[Generator selection](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uLand.pas#L817),
[outline pipeline](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/rust/landgen/src/outline_template_based/template_based.rs#L24).

Land filling, exposed borders, background material and theme objects are
separate pieces. Some theme objects contribute collision, so decoration cannot
universally be treated as non-solid. Texture updates are tiled and visible
areas are updated for drawing.
[Material fill](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uLand.pas#L183),
[theme-object collision](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uLandObjects.pas#L122),
[texture updates](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uLandTexture.pas#L233).

**Implemented in StakeWars:** original built-in polygon silhouettes and carved
openings provide hooked towers, arches, cave floors, overhangs, detached shelves
and gaps. Spawn search considers exposed floors at multiple heights, validates
the entire body and avoids overlap. Three newly generated comic materials fill
the actual mask; exposed rims and cave undersides remain legible. These are
our own assets and implementation, not copied Hedgewars maps or textures.
Further authored courses, smoother contour variation and balance playtesting
remain necessary.

## Camera and parallax

Hedgewars follows an active object with smoothing and movement-direction
look-ahead. Manual cursor movement can cancel follow; target-selection and
ammo-menu states affect automatic following. Bounds account for map edges.
[MoveCamera](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uWorld.pas#L1943).

Its sky and horizon scroll at different fractions of world displacement
(3/8 and 3/5 in the inspected drawing path), with additional water-wave layers.
This creates depth through relative movement, rather than moving one large
background image at terrain speed.
[Parallax drawing](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uWorld.pas#L1207).

**Implemented in StakeWars:** stars, moon, distant silhouettes and buildings
use successively larger fractions of camera displacement; terrain stays at
world speed; sparse foreground shapes move faster. All are presentation-only.
Manual camera override, wheel zoom, map-strip navigation and character/shot
following already exist. Look-ahead tuning, occlusion-aware framing and camera
comfort settings still need work.

## Broader feature comparison

| Area | Hedgewars reference behavior | StakeWars work needed |
|---|---|---|
| Arsenal | Many projectile, melee, remote/targeted weapons and mobility tools | Complete the approved arsenal beyond the eleven initial tools |
| Inventory | Counts, delayed availability, per-turn shot limits, utility properties | Versioned weapon definitions, ammo, cooldowns and disabled-state explanations |
| Aiming | Power, precision, target selection, timer/bounce options by weapon | Weapon-specific input modes and visible fuse/power/target feedback |
| Traversal | Walking, multiple jumps, rope, parachute and flight tools | Movement/rope courses, back/high jump behavior, clear controls |
| Environment | Destruction, drowning, world-edge behavior, mines, crates, sudden death | Finish proximity mines, pickups and consistent hazard rules |
| Feedback | Active-character marker, timer states, captions, sounds, health tags | Distinct ready/action/retreat cues, audio, damage and outcome presentation |
| Camera | Follow with manual override, zoom, target-mode behavior, parallax | Refine camera priorities and comfort across multi-projectile turns |
| Modes | Training, campaigns, scripts and custom schemes | Keep StakeWars built-in and paid-only; these do not override approved scope |

Inventory evidence: [ammo counts and delays](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uAmmos.pas#L306),
[weapon metadata](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uVariables.pas#L911).
The repository [README](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/README.md)
provides the broad weapon, control and mode overview.

## Networking and money stay separate

Hedgewars has tick-stamped engine commands, IPC and record loading, which are
useful references for replay and command handling. They do not supply StakeWars
with trustless table seating, bond validation, Bison Relay completed-turn
agreement, or absent-player settlement.
[Command transport and records](https://github.com/hedgewars/hw/blob/85685825d924cf08b2becfdac61bccbaf4dbd9dd/hedgewars/uIO.pas#L250).
The SDK/escrow release gates in `protocol-findings.md` still apply.

## Next implementation priorities from this review

1. Validate complex terrain across seeds, squad sizes and traversal routes.
2. Add explicit weapon capabilities so UI, inventory and input rules agree.
3. Add and replay a bounded ready phase; retain separate action and retreat clocks.
4. Improve aim precision, jump variety, rope behavior and camera framing.
5. Finish audio, environmental interactions and built-in match configuration.

## Follow-up implementation (simulation version 5)

The previously recommended readiness, jump variants, precision controls, fuse
selection, weapon grid, ammunition, unlock delays, proximity mines, barrels and
pickups are now implemented in the internal fixture. Stakey animation and
synthesized audio also exist. The reference descriptions above record the
original review; consult `implementation-status.md` for current validation and
remaining paid-release gates.
