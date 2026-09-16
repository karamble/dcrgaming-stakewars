package sim

import (
	"bytes"
	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"github.com/karamble/dcrstakewars/pkg/sim/terrain"
	"testing"
)

func TestRopeReleasesClearCornersWithoutCrossingWindingSide(t *testing.T) {
	s := flatArena(t)
	v := func(x, y int) fixed.Vec { return fixed.Vec{X: fixed.FromInt(x), Y: fixed.FromInt(y)} }
	s.Worms[0].Pos = v(300, 250)
	s.Rope = Rope{Pivots: []Pivot{{Pos: v(100, 100)}, {Pos: v(150, 150), Side: -1}, {Pos: v(200, 160), Side: 1}}, Length: fixed.FromInt(120)}
	total := s.Rope.Length + s.Rope.Pivots[1].Pos.Sub(s.Rope.Pivots[0].Pos).Len() + s.Rope.Pivots[2].Pos.Sub(s.Rope.Pivots[1].Pos).Len()
	s.unwrapRope(s.Worms[0].Pos)
	if len(s.Rope.Pivots) != 1 || s.Rope.Length != total {
		t.Fatal("clear corners retained or rope length lost")
	}
}

func TestRopeKeepsObstructedCornerThenReleasesWhenClear(t *testing.T) {
	s := flatArena(t)
	v := func(x, y int) fixed.Vec { return fixed.Vec{X: fixed.FromInt(x), Y: fixed.FromInt(y)} }
	s.Terrain.StampRect(terrain.Rect{X0: 190, Y0: 140, X1: 210, Y1: 220}, true)
	s.Rope = Rope{Pivots: []Pivot{{Pos: v(100, 100)}, {Pos: v(188, 222), Side: 1}}, Length: fixed.FromInt(150)}
	s.unwrapRope(v(300, 250))
	if len(s.Rope.Pivots) != 2 {
		t.Fatal("rope cut through obstructing terrain")
	}
	s.unwrapRope(v(120, 250))
	if len(s.Rope.Pivots) != 1 {
		t.Fatal("rope stuck after moving past corner")
	}
}

func TestSlackRopeDoesNotPushBodyOutward(t *testing.T) {
	s := flatArena(t)
	w := &s.Worms[0]
	w.Pos = fixed.Vec{X: fixed.FromInt(200), Y: fixed.FromInt(180)}
	w.Vel = fixed.Vec{Y: -fixed.FromInt(2)}
	s.Rope = Rope{Pivots: []Pivot{{Pos: fixed.Vec{X: fixed.FromInt(200), Y: fixed.FromInt(100)}}}, Length: fixed.FromInt(140)}
	peer := s.Clone()
	s.swing(w)
	peer.swing(&peer.Worms[0])
	if w.Pos.Y >= fixed.FromInt(180) {
		t.Fatal("slack rope pushed instead of allowing inward motion")
	}
	if !bytes.Equal(s.AppendCanonical(nil), peer.AppendCanonical(nil)) {
		t.Fatal("rope replay diverged")
	}
}
