package sim

import (
	"bytes"
	"testing"

	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

func TestDrillEscapesBurialAndKeepsAttack(t *testing.T) {
	for _, facing := range []int8{-1, 1} {
		s := flatArena(t)
		s.Terrain.StampRect(terrain.Rect{X0: 75, Y0: 250, X1: 126, Y1: 300}, true)
		s.Aim = FacingAim(0, facing)
		if !s.blocked(s.Worms[0].Pos) {
			t.Fatal("fixture is not buried")
		}
		eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(MiningDrill)}, Input{Kind: Fire, Param: 1})
		if s.blocked(s.Worms[0].Pos) || !s.Worms[0].Grounded {
			t.Fatal("drill failed to clear body or removed horizontal support")
		}
		for i := 0; i < 30; i++ {
			eventStep(t, s, Input{Kind: Move, Param: int32(facing)}, Input{Kind: Fire, Param: 1})
		}
		if fixed.Abs(s.Worms[0].Pos.X-fixed.FromInt(100)) < fixed.FromInt(30) {
			t.Fatal("could not walk out of burial")
		}
		if s.Phase != Playing || s.Shots != 0 || s.Worms[0].HP != s.Config.HP || s.PhaseTick == 0 {
			t.Fatal("dig changed attack, health or clock")
		}
		eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(Bazooka)}, Input{Kind: Fire, Param: 1000})
		if len(s.Projectiles) == 0 {
			t.Fatal("attack unavailable after digging")
		}
	}
}

func TestDrillCadenceBoundsAndReplay(t *testing.T) {
	s := flatArena(t)
	s.Weapon = MiningDrill
	s.Aim = 16384 // down
	peer := s.Clone()
	for i := 0; i < 18; i++ {
		input := []Input{{Kind: Fire, Param: 1}}
		eventStep(t, s, input...)
		eventStep(t, peer, input...)
		if !bytes.Equal(s.AppendCanonical(nil), peer.AppendCanonical(nil)) {
			t.Fatal("drill replay diverged")
		}
	}
	if s.Terrain.Solid(100, 300) || s.Worms[0].Pos.Y <= fixed.FromInt(300) {
		t.Fatal("downward digging did not remove floor")
	}
	if !s.Terrain.Solid(140, 300) {
		t.Fatal("drill exceeded local reach")
	}
	s.Tick = 19
	before := s.Terrain.AppendCanonical(nil)
	var effects []Effect
	s.dig(&effects)
	if len(effects) != 0 || !bytes.Equal(before, s.Terrain.AppendCanonical(nil)) {
		t.Fatal("drill ignored cadence")
	}
	s.Tick = 24
	s.Shots = 1
	s.dig(&effects)
	if len(effects) != 0 {
		t.Fatal("drill usable after attack")
	}
}
