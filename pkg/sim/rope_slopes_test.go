package sim

import (
	"bytes"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
	"testing"
)

func TestRopeReelsOffSlopes(t *testing.T) {
	for _, direction := range []int{-1, 1} {
		s := flatArena(t)
		s.Terrain = terrain.New(640, 384, false)
		floor := func(x int) int { return 280 - direction*(x-200) }
		for x := 100; x < 300; x++ {
			for y := floor(x); y < 384; y++ {
				s.Terrain.Set(x, y)
			}
		}
		w := &s.Worms[0]
		w.Pos = fixed.Vec{X: fixed.FromInt(200), Y: fixed.FromInt(274)}
		w.Apex = w.Pos.Y
		// The torso-to-anchor ray is clear, but the feet's diagonal pull meets
		// the staircase pixels at the uphill edge of the body's collision box.
		anchor := fixed.Vec{X: fixed.FromInt(230), Y: fixed.FromInt(248)}
		if direction < 0 {
			anchor.X = fixed.FromInt(170)
		}
		s.Terrain.StampRect(terrain.Rect{X0: anchor.X.Int() - 2, Y0: anchor.Y.Int() - 5, X1: anchor.X.Int() + 3, Y1: anchor.Y.Int() - 2}, true)
		s.Rope = Rope{Pivots: []Pivot{{Pos: anchor}}, Length: w.Pos.Sub(anchor).Len()}
		w.Grounded = false
		peer := s.Clone()
		start := w.Pos
		for tick := 0; tick < 12; tick++ {
			eventStep(t, s, Input{Kind: RopeAdjust, Param: -1})
			eventStep(t, peer, Input{Kind: RopeAdjust, Param: -1})
			if !bytes.Equal(s.AppendCanonical(nil), peer.AppendCanonical(nil)) {
				t.Fatal("slope reeling replay diverged")
			}
			if s.blocked(w.Pos) {
				t.Fatal("reeled through terrain", direction, tick)
			}
		}
		if start.Y-w.Pos.Y < fixed.FromInt(8) {
			t.Fatalf("reeling stuck on slope %d: start=%v end=%v", direction, start, w.Pos)
		}
	}
}

func TestRopeBlockedReelingDoesNotBankShortening(t *testing.T) {
	s := flatArena(t)
	w := &s.Worms[0]
	w.Pos = fixed.Vec{X: fixed.FromInt(100), Y: fixed.FromInt(270)}
	s.Terrain.StampRect(terrain.Rect{X0: 108, Y0: 100, X1: 120, Y1: 300}, true)
	anchor := fixed.Vec{X: fixed.FromInt(106), Y: fixed.FromInt(250)}
	s.Rope = Rope{Pivots: []Pivot{{Pos: anchor}}, Length: w.Pos.Sub(anchor).Len()}
	for tick := 0; tick < 60; tick++ {
		eventStep(t, s, Input{Kind: RopeAdjust, Param: -1})
		if s.blocked(w.Pos) {
			t.Fatal("rope pulled body through wall")
		}
		if len(s.Rope.Pivots) == 0 {
			t.Fatal("valid anchor detached")
		}
		last := s.Rope.Pivots[len(s.Rope.Pivots)-1].Pos
		if w.Pos.Sub(last).Len()-s.Rope.Length > fixed.Ratio(1, 8) {
			t.Fatal("blocked shortening accumulated")
		}
	}
}
