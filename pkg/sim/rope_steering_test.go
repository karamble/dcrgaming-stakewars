package sim

import (
	"bytes"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
	"testing"
)

func TestRopeSteeringStartsAndReversesPromptly(t *testing.T) {
	for _, direction := range []int{-1, 1} {
		s := flatArena(t)
		s.Terrain = terrain.New(640, 384, false)
		anchor := fixed.Vec{X: fixed.FromInt(320), Y: fixed.FromInt(80)}
		s.Terrain.StampRect(terrain.Rect{X0: 318, Y0: 76, X1: 323, Y1: 79}, true)
		w := &s.Worms[0]
		w.Pos = anchor.Add(fixed.Vec{Y: fixed.FromInt(140)})
		w.Vel = fixed.Vec{}
		w.Grounded = false
		w.Apex = w.Pos.Y
		s.Rope = Rope{Pivots: []Pivot{{Pos: anchor}}, Length: fixed.FromInt(140)}
		peer := s.Clone()
		start := w.Pos
		step := func(dir int) {
			eventStep(t, s, Input{Kind: Move, Param: int32(dir)})
			eventStep(t, peer, Input{Kind: Move, Param: int32(dir)})
			if !bytes.Equal(s.AppendCanonical(nil), peer.AppendCanonical(nil)) {
				t.Fatal("steering replay diverged")
			}
			if len(s.Rope.Pivots) != 1 || w.Pos.Sub(anchor).Len() > s.Rope.Length+fixed.Ratio(1, 8) {
				t.Fatal("rope lost constraint")
			}
		}
		for tick := 0; tick < 18; tick++ {
			step(direction)
		}
		travel := fixed.Mul(w.Pos.X-start.X, fixed.FromInt(direction))
		if travel < fixed.FromInt(28) {
			t.Fatalf("slow swing startup: travel %v", travel)
		}
		for tick := 0; tick < 24; tick++ {
			step(-direction)
		}
		if fixed.Mul(w.Vel.X, fixed.FromInt(direction)) >= -fixed.One {
			t.Fatalf("opposite input failed to reverse swing: %v", w.Vel)
		}
	}
}

func TestRopeSteeringPumpsAtSideOfArc(t *testing.T) {
	for _, direction := range []int{-1, 1} {
		s := flatArena(t)
		w := &s.Worms[0]
		anchor := fixed.Vec{X: fixed.FromInt(320), Y: fixed.FromInt(100)}
		w.Pos = anchor.Add(fixed.Vec{X: fixed.FromInt(direction * 120), Y: fixed.FromInt(20)})
		w.Vel = fixed.Vec{}
		s.Rope = Rope{Pivots: []Pivot{{Pos: anchor}}, Length: w.Pos.Sub(anchor).Len()}
		s.Walk = int8(direction)
		s.swing(w)
		if w.Vel.Y >= 0 {
			t.Fatal("steering cannot pump upward at side of arc")
		}
	}
}
