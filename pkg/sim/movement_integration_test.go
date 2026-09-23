package sim

import (
	"bytes"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
	"testing"
)

func TestRopeDigJumpIntegrationReplays(t *testing.T) {
	s := flatArena(t)
	// Low tunnel ceiling, buried start, and a small shelf beyond the escape.
	s.Terrain.StampRect(terrain.Rect{X0: 60, Y0: 210, X1: 230, Y1: 230}, true)
	s.Terrain.StampRect(terrain.Rect{X0: 80, Y0: 265, X1: 125, Y1: 300}, true)
	s.Terrain.StampRect(terrain.Rect{X0: 145, Y0: 296, X1: 190, Y1: 300}, true)
	peer := s.Clone()
	step := func(in ...Input) {
		t.Helper()
		eventStep(t, s, in...)
		eventStep(t, peer, in...)
		if !bytes.Equal(s.AppendCanonical(nil), peer.AppendCanonical(nil)) {
			t.Fatal("movement replay diverged")
		}
	}
	step(Input{Kind: SelectWeapon, Param: int32(MiningDrill)}, Input{Kind: Aim, Param: 0}, Input{Kind: Fire, Param: 1})
	for i := 0; i < 35; i++ {
		step(Input{Kind: Move, Param: 1}, Input{Kind: Fire, Param: 1})
	}
	if s.blocked(s.Worms[0].Pos) || s.Worms[0].Pos.X < fixed.FromInt(145) {
		t.Fatal("failed to escape onto shelf")
	}
	step(Input{Kind: Move, Param: 0}, Input{Kind: Jump, Param: 0})
	if s.Worms[0].Grounded {
		t.Fatal("jump failed after digging")
	}
	step(Input{Kind: SelectWeapon, Param: int32(NinjaRope)}, Input{Kind: Aim, Param: 49152}, Input{Kind: Fire, Param: 1})
	if len(s.Rope.Pivots) == 0 {
		t.Fatal("rope missed tunnel ceiling")
	}
	length := s.Rope.Length
	for i := 0; i < 5; i++ {
		step(Input{Kind: RopeAdjust, Param: -1})
	}
	if s.Rope.Length >= length {
		t.Fatal("rope did not shorten")
	}
	for i := 0; i < 5; i++ {
		step(Input{Kind: RopeAdjust, Param: 1})
	}
	if s.Rope.Length != length {
		t.Fatal("rope did not lengthen")
	}
	step(Input{Kind: Fire, Param: 1})
	if len(s.Rope.Pivots) != 0 {
		t.Fatal("rope did not release")
	}
	for i := 0; i < 40; i++ {
		step()
	}
	if !s.Worms[0].Grounded || s.blocked(s.Worms[0].Pos) {
		t.Fatal("did not land cleanly")
	}
}
func TestDestroyedRopeSupportDetaches(t *testing.T) {
	s := flatArena(t)
	s.Terrain.StampRect(terrain.Rect{X0: 70, Y0: 220, X1: 130, Y1: 230}, true)
	eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(NinjaRope)}, Input{Kind: Aim, Param: 49152}, Input{Kind: Fire, Param: 1})
	if len(s.Rope.Pivots) == 0 {
		t.Fatal("rope missed")
	}
	s.Terrain.StampRect(terrain.Rect{X0: 70, Y0: 220, X1: 130, Y1: 230}, false)
	eventStep(t, s)
	if len(s.Rope.Pivots) != 0 || s.Worms[0].Vel.Y <= 0 {
		t.Fatal("rope hung from destroyed terrain")
	}
}
