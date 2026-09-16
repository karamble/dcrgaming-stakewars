package sim

import (
	"bytes"
	"testing"

	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"github.com/karamble/dcrstakewars/pkg/sim/terrain"
)

func testState(t *testing.T) *State {
	t.Helper()
	c := DefaultConfig()
	c.ReadyTicks = 0
	c.Environment = false
	c.WeaponDelays = false
	c.Width = 640
	c.Height = 384
	c.Units = 2
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestRejectedTickDoesNotMutateState(t *testing.T) {
	s := testState(t)
	before := s.AppendCanonical(nil)
	in := []Input{{Tick: 0, Seat: 0, Sequence: 0, Kind: Move, Param: 1}, {Tick: 0, Seat: 1, Sequence: 1, Kind: Fire, Param: 1000}}
	if _, err := Step(s, in); err == nil {
		t.Fatal("foreign-seat input accepted")
	}
	if !bytes.Equal(before, s.AppendCanonical(nil)) {
		t.Fatal("partially applied invalid tick")
	}
}
func TestIdleTurnRotatesSquadsAndCharacters(t *testing.T) {
	s := testState(t)
	s.Config.TurnTicks = 1
	s.Config.RetreatTicks = 0
	// Settle all bodies first; no input is needed to rotate turns.
	seen := []uint16{s.Active}
	for n := 0; n < 1500 && len(seen) < 5; n++ {
		old := s.Turn
		if _, err := Step(s, nil); err != nil {
			t.Fatal(err)
		}
		if s.Turn != old {
			seen = append(seen, s.Active)
		}
	}
	want := []uint16{0, 2, 1, 3, 0}
	if len(seen) != len(want) {
		t.Fatal("idle match stopped progressing")
	}
	for i, v := range want {
		if seen[i] != v {
			t.Fatalf("rotation %d: got %d want %d", i, seen[i], v)
		}
	}
}
func TestCanonicalStateAndCloneIncludeGameplay(t *testing.T) {
	s := testState(t)
	for _, mutate := range []func(*State){
		func(v *State) { v.Worms[0].HP-- }, func(v *State) { v.Worms[0].Apex++ }, func(v *State) { v.Config.TurnTicks++ },
		func(v *State) { v.Next[0]++ }, func(v *State) { v.Rng.Uint32() }, func(v *State) { v.Rope.Length++ },
		func(v *State) { v.Terrain.Set(0, 0) }, func(v *State) { v.Shots++ },
	} {
		c := s.Clone()
		before := s.AppendCanonical(nil)
		mutate(c)
		if bytes.Equal(before, c.AppendCanonical(nil)) {
			t.Fatal("gameplay field omitted")
		}
		if !bytes.Equal(before, s.AppendCanonical(nil)) {
			t.Fatal("clone aliases state")
		}
	}
}
func TestProjectileCannotTunnelThroughOnePixelWall(t *testing.T) {
	s := testState(t)
	s.Terrain = terrain.New(640, 384, false)
	s.Terrain.StampRect(terrain.Rect{X0: 200, Y0: 0, X1: 201, Y1: 384}, true)
	s.Projectiles = []Projectile{{Pos: fixed.Vec{X: fixed.FromInt(190), Y: fixed.FromInt(100)}, Vel: fixed.Vec{X: fixed.FromInt(32)}, Damage: 50, Radius: 8, Weapon: Bazooka}}
	var effects []Effect
	s.moveProjectiles(&effects)
	if len(s.Projectiles) != 0 || len(effects) != 1 || effects[0].Pos.X.Int() >= 200 {
		t.Fatal("projectile crossed thin wall")
	}
}
func TestClusterChildrenKeepTheirFuse(t *testing.T) {
	s := testState(t)
	var effects []Effect
	s.explode(Projectile{Pos: fixed.Vec{X: fixed.FromInt(300), Y: fixed.FromInt(100)}, Damage: 50, Radius: 30, Weapon: ClusterBomb}, &effects)
	if len(s.Projectiles) != 5 {
		t.Fatal("missing cluster children")
	}
	for _, p := range s.Projectiles {
		if p.Fuse != 90 {
			t.Fatal("parent explosion immediately detonated children")
		}
	}
}
func TestShotgunCannotResetFirstShotBySwitching(t *testing.T) {
	s := testState(t)
	s.Weapon = Shotgun
	s.Shots = 1
	s.apply(Input{Kind: SelectWeapon, Param: int32(Bazooka)}, nil)
	if s.Weapon != Shotgun || s.Shots != 1 {
		t.Fatal("switching reset shotgun allowance")
	}
}
func TestSurrenderReachesWinner(t *testing.T) {
	s := testState(t)
	if _, err := Step(s, []Input{{Tick: 0, Seat: 0, Kind: Surrender}}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 500 && s.Phase != Ended; n++ {
		if _, err := Step(s, nil); err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase != Ended || s.Winner != 1 {
		t.Fatal("surrender did not settle gameplay")
	}
}
func TestRopePreservesConstraintInOpenSpace(t *testing.T) {
	s := testState(t)
	s.Terrain = terrain.New(640, 384, false)
	w := &s.Worms[0]
	w.Pos = fixed.Vec{X: fixed.FromInt(300), Y: fixed.FromInt(220)}
	s.Rope = Rope{Pivots: []Pivot{{Pos: fixed.Vec{X: fixed.FromInt(300), Y: fixed.FromInt(100)}}}, Length: fixed.FromInt(120)}
	for n := 0; n < 600; n++ {
		s.Walk = 1
		s.swing(w)
		deviation := w.Pos.Sub(s.Rope.Pivots[0].Pos).Len() - s.Rope.Length
		if deviation > fixed.Ratio(1, 8) {
			t.Fatal("rope constraint drift")
		}
	}
}

func TestRepeatedSkipCannotExtendRetreat(t *testing.T) {
	c := DefaultConfig()
	c.ReadyTicks = 0
	c.Environment = false
	c.WeaponDelays = false
	c.TurnTicks = 10
	c.RetreatTicks = 3
	s, _ := New(c)
	for tick := 0; tick < 20 && s.Turn == 0; tick++ {
		_, err := Step(s, []Input{{Tick: s.Tick, Seat: s.ActiveSeat(), Kind: Skip}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.Turn == 0 {
		t.Fatal("repeated skip extended turn")
	}
}
func TestSpawnBodiesClearAndSettle(t *testing.T) {
	for seats := uint8(2); seats <= 6; seats++ {
		c := DefaultConfig()
		c.ReadyTicks = 0
		c.Environment = false
		c.WeaponDelays = false
		c.Seats = seats
		c.Units = 8
		s, err := New(c)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range s.Worms {
			if s.blocked(w.Pos) {
				t.Fatal("spawn embedded in terrain")
			}
		}
		for tick := 0; tick < 60; tick++ {
			if _, err := Step(s, nil); err != nil {
				t.Fatal(err)
			}
		}
		if !s.Quiescent() {
			t.Fatal("squad did not settle")
		}
	}
}

func TestShotEffectStopsAtHitAndDealsDamageImmediately(t *testing.T) {
	s := testState(t)
	s.Terrain = terrain.New(int(s.Config.Width), int(s.Config.Height), false)
	for i := range s.Worms {
		s.Worms[i].HP = 0
	}
	s.Worms[0].HP = 100
	s.Worms[0].Pos = fixed.Vec{X: fixed.FromInt(100), Y: fixed.FromInt(200)}
	s.Worms[2].HP = 100
	s.Worms[2].Pos = fixed.Vec{X: fixed.FromInt(200), Y: fixed.FromInt(200)}
	s.Aim = 0
	s.Weapon = Shotgun
	effects, err := Step(s, []Input{{Tick: s.Tick, Seat: 0, Kind: Fire, Param: 1000}})
	if err != nil {
		t.Fatal(err)
	}
	if s.Worms[2].HP != 75 {
		t.Fatal("visual effects delayed or changed shotgun damage")
	}
	found := false
	for _, effect := range effects {
		if effect.Shot {
			found = true
			if effect.Weapon != Shotgun || effect.From.X != fixed.FromInt(114) || effect.Pos.X < fixed.FromInt(194) || effect.Pos.X > fixed.FromInt(200) {
				t.Fatal("shot effect does not follow actual ray endpoint")
			}
		}
	}
	if !found {
		t.Fatal("missing shot effect")
	}
}

func TestFallingThroughDestroyedGroundIsLethal(t *testing.T) {
	s := testState(t)
	for i := 0; i < 60; i++ {
		if _, err := Step(s, nil); err != nil {
			t.Fatal(err)
		}
	}
	index := s.Active
	w := s.Worms[index]
	box := terrain.Rect{X0: w.Pos.X.Int() - 16, Y0: w.Pos.Y.Int(), X1: w.Pos.X.Int() + 17, Y1: int(s.Config.Height)}
	s.Terrain.StampRect(box, false)
	deaths := 0
	for i := 0; i < 120; i++ {
		effects, err := Step(s, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range effects {
			if e.HazardDeath {
				deaths++
				if e.Seat != w.Seat || e.Pos.Y != s.WaterY {
					t.Fatal("incorrect hazard death")
				}
			}
		}
	}
	if s.Worms[index].HP != 0 || deaths != 1 {
		t.Fatalf("expected one lethal fall, hp=%d deaths=%d", s.Worms[index].HP, deaths)
	}
	if s.Worms[2].HP <= 0 {
		t.Fatal("unaffected squad died")
	}
}
func TestHazardKillsOnBoundaryCrossing(t *testing.T) {
	s := testState(t)
	w := &s.Worms[s.Active]
	w.Pos.Y = s.WaterY - fixed.One
	w.Vel.Y = fixed.FromInt(4)
	w.Grounded = false
	effects, err := Step(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w.HP != 0 {
		t.Fatal("crossing lethal surface survived until a later tick")
	}
	count := 0
	for _, e := range effects {
		if e.HazardDeath {
			count++
		}
	}
	if count != 1 {
		t.Fatal("missing boundary-crossing effect")
	}
}
