package sim

import (
	"testing"

	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"github.com/karamble/dcrstakewars/pkg/sim/terrain"
)

func TestWalkingFollowsDownhillSurfaceBothDirections(t *testing.T) {
	for _, direction := range []int8{-1, 1} {
		s := flatArena(t)
		s.Terrain = terrain.New(640, 384, false)
		height := func(x int) int {
			if direction < 0 {
				x = 639 - x
			}
			return 100 + x/3
		}
		for x := 0; x < 640; x++ {
			for y := height(x); y < 384; y++ {
				s.Terrain.Set(x, y)
			}
		}
		x := 200
		if direction < 0 {
			x = 439
		}
		y := height(x - int(direction)*halfWidth)
		w := &s.Worms[0]
		w.Pos = fixed.Vec{X: fixed.FromInt(x), Y: fixed.FromInt(y)}
		w.Apex = w.Pos.Y
		s.Walk = direction
		hp := w.HP
		for tick := 0; tick < 60; tick++ {
			s.moveWorm(0)
			if !w.Grounded || !s.supported(w.Pos) || s.blocked(w.Pos) || w.Vel.Y != 0 {
				t.Fatalf("lost slope contact: direction=%d tick=%d pos=%v", direction, tick, w.Pos)
			}
		}
		if w.Pos.Y <= fixed.FromInt(y) || w.HP != hp || w.Pos.X != fixed.FromInt(x+int(direction)*90) {
			t.Fatal("downhill walking stalled or caused fall damage")
		}
	}
}

func TestWalkingStepDownLimitAndRealLedges(t *testing.T) {
	for _, drop := range []int{4, 5, 60} {
		s := flatArena(t)
		s.Terrain = terrain.New(640, 384, false)
		s.Terrain.StampRect(terrain.Rect{X0: 0, Y0: 180, X1: 100, Y1: 384}, true)
		s.Terrain.StampRect(terrain.Rect{X0: 100, Y0: 180 + drop, X1: 640, Y1: 384}, true)
		w := &s.Worms[0]
		w.Pos = fixed.Vec{X: fixed.FromInt(105), Y: fixed.FromInt(180)}
		w.Apex = w.Pos.Y
		s.Walk = 1
		s.moveWorm(0)
		if drop == 4 {
			if !w.Grounded || w.Pos.Y != fixed.FromInt(184) {
				t.Fatal("small step was not followed")
			}
		} else {
			if w.Grounded || w.Pos.Y >= fixed.FromInt(180+drop) {
				t.Fatal("snapped down a real ledge")
			}
		}
	}
}

func TestDownhillFollowingDoesNotCatchJumpOrDestroyedFloor(t *testing.T) {
	s := flatArena(t)
	w := &s.Worms[0]
	w.Pos.Y = fixed.FromInt(298)
	w.Grounded = false
	w.Vel.Y = -fixed.FromInt(5)
	s.Walk = 1
	s.moveWorm(0)
	if w.Grounded || w.Pos.Y >= fixed.FromInt(298) {
		t.Fatal("ground following caught a jump")
	}
	s = flatArena(t)
	w = &s.Worms[0]
	s.Terrain.StampRect(terrain.Rect{X0: 80, Y0: 300, X1: 120, Y1: 303}, false)
	s.Walk = 1
	s.moveWorm(0)
	if w.Grounded || w.Pos.Y >= fixed.FromInt(303) {
		t.Fatal("ground following caught destroyed support")
	}
}
