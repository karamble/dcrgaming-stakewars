package sim

import (
	"bytes"
	"testing"

	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"github.com/karamble/dcrstakewars/pkg/sim/terrain"
)

func flatArena(t *testing.T) *State {
	t.Helper()
	s := testState(t)
	s.Terrain = terrain.New(640, 384, false)
	s.Terrain.StampRect(terrain.Rect{X0: 0, Y0: 300, X1: 640, Y1: 340}, true)
	for i := range s.Worms {
		s.Worms[i].Pos = fixed.Vec{X: fixed.FromInt(100 + i*100), Y: fixed.FromInt(300)}
		s.Worms[i].Vel = fixed.Vec{}
		s.Worms[i].Apex = s.Worms[i].Pos.Y
		s.Worms[i].Grounded = true
		s.Worms[i].Facing = 1
	}
	return s
}
func eventStep(t *testing.T, s *State, events ...Input) []Effect {
	t.Helper()
	for i := range events {
		events[i].Tick = s.Tick
		events[i].Seat = s.ActiveSeat()
		events[i].Sequence = uint16(i)
	}
	e, err := Step(s, events)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestJumpVariantsAndPreciseFacing(t *testing.T) {
	for mode := int32(0); mode <= 2; mode++ {
		s := flatArena(t)
		eventStep(t, s, Input{Kind: Jump, Param: mode})
		w := s.Worms[0]
		if w.Grounded || w.Vel.Y >= 0 {
			t.Fatal("jump did not launch")
		}
		if mode == 1 && w.Vel.X != 0 {
			t.Fatal("high jump moved horizontally")
		}
		if mode == 2 && w.Vel.X >= 0 {
			t.Fatal("back jump moved forward")
		}
	}
	s := flatArena(t)
	pos := s.Worms[0].Pos
	eventStep(t, s, Input{Kind: Face, Param: -1})
	if s.Worms[0].Pos != pos || s.Worms[0].Facing != -1 {
		t.Fatal("precision facing walked")
	}
}
func TestFuseSelectionAndAtomicValidation(t *testing.T) {
	for seconds := int32(1); seconds <= 5; seconds++ {
		s := flatArena(t)
		eventStep(t, s, Input{Kind: SetFuse, Param: seconds}, Input{Kind: SelectWeapon, Param: int32(BouncingBomb)}, Input{Kind: Fire, Param: 800})
		if len(s.Projectiles) != 1 || s.Projectiles[0].Fuse != uint32(seconds)*60-1 {
			t.Fatal("fuse selection ignored")
		}
	}
	for _, bad := range []Input{{Kind: SetFuse, Param: 0}, {Kind: SetFuse, Param: 6}, {Kind: Jump, Param: 3}, {Kind: Face, Param: 2}, {Kind: SetTarget, Param: 640}, {Kind: SetTarget, Param: -1}} {
		s := flatArena(t)
		before := s.AppendCanonical(nil)
		_, err := Step(s, []Input{{Tick: s.Tick, Seat: s.ActiveSeat(), Kind: Move, Param: 1}, {Tick: s.Tick, Seat: s.ActiveSeat(), Sequence: 1, Kind: bad.Kind, Param: bad.Param}})
		if err == nil || !bytes.Equal(before, s.AppendCanonical(nil)) {
			t.Fatal("invalid new input partly applied", bad)
		}
	}
}
func TestParachuteControlsDescentAndLandingDamage(t *testing.T) {
	s := flatArena(t)
	w := &s.Worms[0]
	w.Pos.Y = fixed.FromInt(70)
	w.Apex = w.Pos.Y
	w.Grounded = false
	w.Vel.Y = fixed.FromInt(12)
	hp := w.HP
	eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(Parachute)}, Input{Kind: Fire, Param: 1}, Input{Kind: Move, Param: 1})
	if !w.Chute || w.Vel.Y > fixed.Ratio(3, 2) {
		t.Fatal("canopy failed to slow descent")
	}
	startX := w.Pos.X
	for i := 0; i < 250 && !w.Grounded; i++ {
		eventStep(t, s)
	}
	if !w.Grounded || w.HP != hp || w.Pos.X <= startX {
		t.Fatal("unsafe parachute landing or no steering")
	}
	eventStep(t, s)
	if w.Chute {
		t.Fatal("canopy stayed open on ground")
	}
}
func TestBatKnockback(t *testing.T) {
	s := flatArena(t)
	s.Worms[1].Pos.X = fixed.FromInt(138)
	s.Aim = 0
	s.Weapon = BaseballBat
	var effects []Effect
	s.fire(1000, &effects)
	if s.Worms[1].HP != 80 || s.Worms[1].Vel.X < fixed.FromInt(12) || s.Worms[1].Vel.Y >= 0 {
		t.Fatal("bat lacks directed knockback")
	}
	if len(effects) != 2 || !effects[1].Shot || effects[1].Weapon != BaseballBat {
		t.Fatal("missing bat visual")
	}
}
func TestTargetedWeaponsAndInventory(t *testing.T) {
	for _, weapon := range []Weapon{HomingRocket, Airstrike, Girder} {
		s := flatArena(t)
		s.Weapon = weapon
		n := s.AmmoCount(weapon)
		var effects []Effect
		s.fire(800, &effects)
		if s.AmmoCount(weapon) != n || s.Phase != Playing || len(s.Projectiles) > 0 {
			t.Fatal("untargeted weapon consumed")
		}
	}
	s := flatArena(t)
	eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(Airstrike)}, Input{Kind: SetTarget, Param: 250<<16 | 320}, Input{Kind: Fire, Param: 1000})
	if len(s.Projectiles) != 5 || s.AmmoCount(Airstrike) != 1 || s.Phase != Retreating {
		t.Fatal("airstrike not launched")
	}
	h := flatArena(t)
	h.Weapon = HeavyCluster
	var effects []Effect
	h.fire(1000, &effects)
	if h.AmmoCount(HeavyCluster) != 0 {
		t.Fatal("heavy ammo not consumed")
	}
	h.Phase = Playing
	h.fire(1000, &effects)
	if len(h.Projectiles) != 1 {
		t.Fatal("empty ammo fired")
	}
}
func TestRemoteChargeDetonatesManuallyAndOnTimeout(t *testing.T) {
	for _, manual := range []bool{false, true} {
		s := flatArena(t)
		s.Config.TurnTicks = 4
		eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(RemoteCharge)}, Input{Kind: Fire, Param: 1000})
		if s.Phase != Playing || s.Shots != 1 || len(s.Projectiles) != 1 {
			t.Fatal("remote planting ended turn")
		}
		eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(Bazooka)})
		if s.Weapon != RemoteCharge {
			t.Fatal("remote allowed weapon switch")
		}
		if manual {
			eventStep(t, s, Input{Kind: Fire, Param: 1000})
		}
		for i := 0; i < 8; i++ {
			eventStep(t, s)
		}
		if len(s.Projectiles) != 0 || s.Phase == Playing {
			t.Fatal("remote remained unresolved")
		}
	}
}
func TestGirderPlacementRejectsOverlapAndRange(t *testing.T) {
	s := flatArena(t)
	s.Weapon = Girder
	s.HasTarget = true
	for _, target := range []fixed.Vec{{X: fixed.FromInt(100), Y: fixed.FromInt(290)}, {X: fixed.FromInt(150), Y: fixed.FromInt(300)}, {X: fixed.FromInt(500), Y: fixed.FromInt(100)}} {
		s.Target = target
		before := s.Terrain.AppendCanonical(nil)
		var e []Effect
		if s.placeGirder(&e) || !bytes.Equal(before, s.Terrain.AppendCanonical(nil)) || s.AmmoCount(Girder) != 4 {
			t.Fatal("invalid girder changed state")
		}
	}
	s.Target = fixed.Vec{X: fixed.FromInt(180), Y: fixed.FromInt(240)}
	var e []Effect
	if !s.placeGirder(&e) || !s.Terrain.Solid(180, 240) || s.AmmoCount(Girder) != 3 || len(e) != 1 || e[0].Dirty.Empty() {
		t.Fatal("valid girder failed")
	}
}
func TestDrillPenetratesWallButBazookaDoesNot(t *testing.T) {
	for _, weapon := range []Weapon{Bazooka, DrillRocket} {
		s := flatArena(t)
		s.Terrain.StampRect(terrain.Rect{X0: 200, Y0: 100, X1: 208, Y1: 220}, true)
		s.Projectiles = []Projectile{{Pos: fixed.Vec{X: fixed.FromInt(190), Y: fixed.FromInt(150)}, Vel: fixed.Vec{X: fixed.FromInt(24)}, Weapon: weapon, Drill: 18, Fuse: 100, Radius: 8}}
		var e []Effect
		s.moveProjectiles(&e)
		if weapon == Bazooka {
			if len(s.Projectiles) > 0 {
				t.Fatal("bazooka crossed wall")
			}
		} else if len(s.Projectiles) != 1 || s.Projectiles[0].Pos.X <= fixed.FromInt(208) || s.Terrain.Solid(200, 150) {
			t.Fatal("drill failed to bore through")
		}
	}
}
func TestHomingSteersAndBounceRetainsEnergy(t *testing.T) {
	s := flatArena(t)
	s.Projectiles = []Projectile{{Pos: fixed.Vec{X: fixed.FromInt(150), Y: fixed.FromInt(100)}, Vel: fixed.Vec{X: fixed.FromInt(5)}, Weapon: HomingRocket, Target: fixed.Vec{X: fixed.FromInt(300), Y: fixed.FromInt(40)}, Age: 21, Fuse: 200}}
	var e []Effect
	s.moveProjectiles(&e)
	if s.Projectiles[0].Vel.Y >= 0 {
		t.Fatal("missile did not steer toward target")
	}
	speeds := []fixed.F{}
	for _, weapon := range []Weapon{Grenade, BouncingBomb} {
		s := flatArena(t)
		s.Projectiles = []Projectile{{Pos: fixed.Vec{X: fixed.FromInt(50), Y: fixed.FromInt(299)}, Vel: fixed.Vec{Y: fixed.FromInt(5)}, Weapon: weapon, Fuse: 100}}
		s.moveProjectiles(&e)
		speeds = append(speeds, fixed.Abs(s.Projectiles[0].Vel.Y))
	}
	if speeds[1] <= speeds[0] {
		t.Fatal("rubber bomb did not rebound harder")
	}
}
func TestBisonMovesOverSmallStepAndFlamesExpire(t *testing.T) {
	s := flatArena(t)
	s.Terrain.StampRect(terrain.Rect{X0: 125, Y0: 297, X1: 170, Y1: 300}, true)
	s.Projectiles = []Projectile{{Pos: fixed.Vec{X: fixed.FromInt(120), Y: fixed.FromInt(299)}, Vel: fixed.Vec{X: fixed.FromInt(3)}, Weapon: BisonBomb, Fuse: 100, Radius: 20}}
	var e []Effect
	for i := 0; i < 5; i++ {
		s.moveProjectiles(&e)
	}
	if len(s.Projectiles) != 1 || s.Projectiles[0].Pos.X < fixed.FromInt(130) {
		t.Fatal("bison failed small step")
	}
	s = flatArena(t)
	s.Projectiles = []Projectile{{Pos: fixed.Vec{X: fixed.FromInt(100), Y: fixed.FromInt(290)}, Weapon: Flamethrower, Burning: true, Fuse: 31}}
	hp := s.Worms[0].HP
	for i := 0; i < 32; i++ {
		s.moveProjectiles(&e)
	}
	if len(s.Projectiles) != 0 || s.Worms[0].HP >= hp {
		t.Fatal("flame did not burn and expire")
	}
}
func TestExpansionFieldsCanonicalAndCloneIndependent(t *testing.T) {
	s := flatArena(t)
	s.Projectiles = []Projectile{{Weapon: HomingRocket}}
	for _, mutate := range []func(*State){func(s *State) { s.FuseSeconds++ }, func(s *State) { s.Target.X++ }, func(s *State) { s.HasTarget = true }, func(s *State) { s.Ammo[0][HeavyCluster]-- }, func(s *State) { s.Worms[0].Chute = true }, func(s *State) { s.Projectiles[0].Age++ }, func(s *State) { s.Projectiles[0].Drill++ }, func(s *State) { s.Projectiles[0].DrillSlow++ }, func(s *State) { s.Projectiles[0].Target.Y++ }, func(s *State) { s.Projectiles[0].Burning = true }} {
		before := s.AppendCanonical(nil)
		clone := s.Clone()
		mutate(clone)
		if bytes.Equal(before, clone.AppendCanonical(nil)) || !bytes.Equal(before, s.AppendCanonical(nil)) {
			t.Fatal("new gameplay field omitted or clone aliased")
		}
	}
}

func TestMovementFacesAimWithoutChangingElevation(t *testing.T) {
	for _, kind := range []EventKind{Move, Face} {
		s := flatArena(t)
		s.Aim = 57344
		original := s.Aim
		eventStep(t, s, Input{Kind: kind, Param: -1})
		if fixed.Cos(s.Aim) >= 0 || fixed.Sin(s.Aim) != fixed.Sin(original) {
			t.Fatal("left movement did not reflect aim horizontally", s.Aim)
		}
		left := s.Aim
		eventStep(t, s, Input{Kind: kind, Param: -1})
		if s.Aim != left {
			t.Fatal("same direction flipped aim again")
		}
		eventStep(t, s, Input{Kind: kind, Param: 1})
		if s.Aim != original || fixed.Cos(s.Aim) <= 0 {
			t.Fatal("right movement failed to restore aim")
		}
		eventStep(t, s, Input{Kind: kind, Param: 0})
		if s.Aim != original {
			t.Fatal("stopping changed aim")
		}
	}
}

func TestJumpsHaveShortArcAndJetpackStillLifts(t *testing.T) {
	for _, mode := range []int32{0, 1, 2} {
		s := flatArena(t)
		s.Worms[0].Pos.X = fixed.FromInt(500)
		start := s.Worms[0].Pos.Y
		eventStep(t, s, Input{Kind: Jump, Param: mode})
		apex := s.Worms[0].Pos.Y
		ticks := 1
		for !s.Worms[0].Grounded && ticks < 80 {
			eventStep(t, s)
			apex = fixed.Min(apex, s.Worms[0].Pos.Y)
			ticks++
		}
		height := (start - apex).Int()
		if mode == 0 && (height < 34 || height > 41 || ticks > 32) {
			t.Fatalf("normal jump too floaty: height=%d ticks=%d", height, ticks)
		}
		if mode == 1 && (height < 55 || height > 68 || ticks > 42) {
			t.Fatalf("high jump too floaty: height=%d ticks=%d", height, ticks)
		}
		if mode == 2 && (height < 68 || height > 78 || ticks > 44) {
			t.Fatalf("backflip arc: height=%d ticks=%d", height, ticks)
		}
	}
	s := flatArena(t)
	y := s.Worms[0].Pos.Y
	eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(Jetpack)}, Input{Kind: Thrust, Param: 1})
	for i := 0; i < 10; i++ {
		eventStep(t, s)
	}
	if s.Worms[0].Pos.Y >= y || s.Worms[0].Vel.Y >= 0 {
		t.Fatal("stronger gravity broke jetpack lift")
	}
}
