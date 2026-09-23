package sim

import (
	"bytes"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
	"testing"
)

func TestRopeActualAttachmentClimbsShallowSlope(t *testing.T) {
	for _, rise := range []int{4, 8} {
		for _, direction := range []int{-1, 1} {
			t.Run(map[int]string{4: "1in4", 8: "1in8"}[rise]+map[int]string{-1: "/left", 1: "/right"}[direction], func(t *testing.T) {
				s := flatArena(t)
				s.Terrain = terrain.New(640, 384, false)
				for x := 0; x < 640; x++ {
					y := 300 - direction*(x-200)/rise
					for ; y < 384; y++ {
						s.Terrain.Set(x, y)
					}
				}
				w := &s.Worms[0]
				w.Pos = fixed.Vec{X: fixed.FromInt(200), Y: fixed.FromInt(300 - halfWidth/rise)}
				w.Apex = w.Pos.Y
				aim := int32(0)
				if direction < 0 {
					aim = 32768
				}
				eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(NinjaRope)}, Input{Kind: Aim, Param: aim}, Input{Kind: Fire, Param: 1})
				if len(s.Rope.Pivots) == 0 {
					t.Fatal("failed to attach to slope")
				}
				peer := s.Clone()
				start := w.Pos
				for n := 0; n < 30; n++ {
					eventStep(t, s, Input{Kind: RopeAdjust, Param: -1})
					eventStep(t, peer, Input{Kind: RopeAdjust, Param: -1})
					if !bytes.Equal(s.AppendCanonical(nil), peer.AppendCanonical(nil)) {
						t.Fatal("shallow slope replay diverged")
					}
					if s.blocked(w.Pos) {
						t.Fatal("body clipped slope")
					}
				}
				t.Logf("travel=%v rise=%v pivots=%v length=%v", w.Pos.X-start.X, start.Y-w.Pos.Y, s.Rope.Pivots, s.Rope.Length)
				if fixed.Abs(w.Pos.X-start.X) < fixed.FromInt(20) || start.Y-w.Pos.Y < fixed.FromInt(2) {
					t.Fatal("reeling failed to climb shallow slope")
				}
			})
		}
	}
}

func TestRopeStepClearanceCannotPassLowCeiling(t *testing.T) {
	s := flatArena(t)
	s.Terrain.StampRect(terrain.Rect{X0: 70, Y0: 270, X1: 170, Y1: 280}, true)
	s.Terrain.StampRect(terrain.Rect{X0: 110, Y0: 299, X1: 170, Y1: 300}, true)
	s.Terrain.StampRect(terrain.Rect{X0: 160, Y0: 280, X1: 170, Y1: 300}, true)
	eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(NinjaRope)}, Input{Kind: Aim, Param: 0}, Input{Kind: Fire, Param: 1})
	if len(s.Rope.Pivots) == 0 {
		t.Fatal("rope failed to attach")
	}
	before := s.Terrain.AppendCanonical(nil)
	for n := 0; n < 60; n++ {
		eventStep(t, s, Input{Kind: RopeAdjust, Param: -1})
		if s.blocked(s.Worms[0].Pos) {
			t.Fatal("stepped through ceiling or floor")
		}
	}
	if s.Worms[0].Pos.X >= fixed.FromInt(104) {
		t.Fatal("squeezed into gap shorter than body")
	}
	if !bytes.Equal(before, s.Terrain.AppendCanonical(nil)) {
		t.Fatal("rope modified terrain")
	}
}
