package sim

import (
	"bytes"
	"testing"

	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

func TestReadyCountdownAndEarlyAction(t *testing.T) {
	s := flatArena(t)
	s.Config.ReadyTicks = 3
	s.Phase = Ready
	pos := s.Worms[0].Pos
	eventStep(t, s)
	eventStep(t, s)
	if s.Phase != Ready || s.PhaseTick != 2 || s.Worms[0].Pos != pos {
		t.Fatal("ready moved gameplay")
	}
	eventStep(t, s)
	if s.Phase != Playing || s.PhaseTick != 0 || s.RemainingTicks() != s.Config.TurnTicks {
		t.Fatal("ready spent action budget")
	}
	s.Phase = Ready
	s.PhaseTick = 1
	eventStep(t, s, Input{Kind: Move, Param: 1})
	if s.Phase != Playing || s.PhaseTick != 1 || s.Worms[0].Pos.X <= pos.X {
		t.Fatal("first action did not start turn")
	}
	s.Phase = Ready
	before := s.AppendCanonical(nil)
	_, err := Step(s, []Input{{Tick: s.Tick, Seat: s.ActiveSeat(), Kind: Jump, Param: 99}})
	if err == nil || !bytes.Equal(before, s.AppendCanonical(nil)) {
		t.Fatal("invalid action escaped readiness")
	}
}
func TestDelayedWeaponsCannotBeSelectedOrFired(t *testing.T) {
	s := flatArena(t)
	s.Config.WeaponDelays = true
	eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(HeavyCluster)})
	if s.Weapon != Bazooka {
		t.Fatal("locked weapon selected")
	}
	s.Weapon = HeavyCluster
	var e []Effect
	s.fire(1000, &e)
	if len(s.Projectiles) > 0 || s.AmmoCount(HeavyCluster) != 1 {
		t.Fatal("locked weapon fired")
	}
	s.Turn = uint32(s.Config.Seats)
	if !s.WeaponAvailable(Airstrike) || s.WeaponAvailable(HeavyCluster) {
		t.Fatal("first-round unlock incorrect")
	}
	s.Turn = 2 * uint32(s.Config.Seats)
	s.fire(1000, &e)
	if len(s.Projectiles) != 1 || s.AmmoCount(HeavyCluster) != 0 {
		t.Fatal("unlocked weapon failed")
	}
}
func TestIdleMinePersistsWithoutBlockingTurns(t *testing.T) {
	s := flatArena(t)
	s.Objects = []Object{{Kind: ProximityMine, Pos: fixed.Vec{X: fixed.FromInt(550), Y: fixed.FromInt(300)}, HP: 1, Settled: true, Arm: 120}}
	var e []Effect
	for i := 0; i < 180; i++ {
		s.moveObjects(&e)
	}
	if len(s.Objects) != 1 || !s.Quiescent() {
		t.Fatal("idle mine vanished or held turn open")
	}
	s.Worms[0].Pos.X = fixed.FromInt(540)
	s.moveObjects(&e)
	if s.Objects[0].Fuse == 0 || s.Quiescent() {
		t.Fatal("proximity mine did not start warning")
	}
	for i := 0; i < 30; i++ {
		s.moveObjects(&e)
	}
	if len(s.Objects) != 0 || s.Worms[0].HP == 100 {
		t.Fatal("mine did not explode")
	}
}
func TestBarrelChainProcessesEveryBarrelOnce(t *testing.T) {
	s := flatArena(t)
	s.Objects = []Object{{Kind: Barrel, Pos: fixed.Vec{X: fixed.FromInt(120), Y: fixed.FromInt(300)}, HP: 20, Settled: true}, {Kind: Barrel, Pos: fixed.Vec{X: fixed.FromInt(165), Y: fixed.FromInt(300)}, HP: 0, Settled: true}}
	var e []Effect
	s.moveObjects(&e)
	if len(s.Objects) != 0 || len(e) != 2 {
		t.Fatal("reverse-index barrel chain incomplete", len(e))
	}
	s.moveObjects(&e)
	if len(e) != 2 {
		t.Fatal("barrel exploded twice")
	}
}
func TestHealthAndAmmoCrates(t *testing.T) {
	s := flatArena(t)
	s.Worms[0].HP = 90
	pos := s.Worms[0].Pos
	s.Objects = []Object{{Kind: HealthCrate, Pos: pos, HP: 20, Settled: true}, {Kind: AmmoCrate, Pos: pos, HP: 20, Settled: true, Weapon: HeavyCluster}}
	var e []Effect
	s.moveObjects(&e)
	if len(s.Objects) != 0 || s.Worms[0].HP != 100 || s.AmmoCount(HeavyCluster) != 2 || len(e) != 2 {
		t.Fatal("pickup transfer or health cap incorrect")
	}
	s.Objects = []Object{{Kind: HealthCrate, Pos: pos, HP: 20, Settled: true}}
	s.moveObjects(&e)
	if len(s.Objects) != 0 || s.Worms[0].HP != 100 {
		t.Fatal("full-health contact must collect without overhealing")
	}
}
func TestObjectsFallIntoHazardAndBlockConstruction(t *testing.T) {
	s := flatArena(t)
	s.Objects = []Object{{Kind: Barrel, Pos: fixed.Vec{X: fixed.FromInt(180), Y: fixed.FromInt(240)}, HP: 20, Settled: true}}
	s.Target = s.Objects[0].Pos
	s.HasTarget = true
	if s.CanPlaceGirder() {
		t.Fatal("girder entombed barrel")
	}
	s.Terrain = terrain.New(640, 384, false)
	var e []Effect
	for i := 0; i < 120; i++ {
		s.moveObjects(&e)
	}
	if len(s.Objects) > 0 || len(e) > 0 {
		t.Fatal("hazard failed to swallow object without explosion")
	}
}
func TestEnvironmentSeedAndCanonicalState(t *testing.T) {
	config := DefaultConfig()
	a, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Objects) < 3 || !bytes.Equal(a.AppendCanonical(nil), b.AppendCanonical(nil)) {
		t.Fatal("environment placement not reproducible")
	}
	r, inc := a.Rng.Words()
	a.spawnCrate()
	rr, ii := a.Rng.Words()
	if r != rr || inc != ii {
		t.Fatal("environment consumed weapon RNG")
	}
	for _, mutate := range []func(*State){func(s *State) { s.Objects[0].HP-- }, func(s *State) { s.Objects[0].Pos.X++ }, func(s *State) { s.Objects[0].VelY++ }, func(s *State) { s.Objects[0].Arm++ }, func(s *State) { s.Objects[0].Fuse++ }, func(s *State) { s.Objects[0].Kind++ }, func(s *State) { s.Objects[0].Owner++ }, func(s *State) { s.Objects[0].Weapon++ }, func(s *State) { s.Objects[0].Settled = false }, func(s *State) { s.EnvironmentRng.Uint32() }, func(s *State) { s.Config.ReadyTicks++ }, func(s *State) { s.Config.Environment = false }, func(s *State) { s.Config.WeaponDelays = false }} {
		before := a.AppendCanonical(nil)
		clone := a.Clone()
		mutate(clone)
		if bytes.Equal(before, clone.AppendCanonical(nil)) || !bytes.Equal(before, a.AppendCanonical(nil)) {
			t.Fatal("new field omitted or aliased")
		}
	}
}
