package render

import (
	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"testing"
)

func TestGraveRecoveryAndHazardExclusion(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	w := &s.Worms[0]
	w.HP = 0
	r := New(s)
	if r.motion[0].deathAge != 60 {
		t.Fatal("restored grave should already be visible")
	}
	x, y, ok := gravePosition(s, 0)
	if !ok || x != float64(w.Pos.X.Int()) || y < float64(w.Pos.Y.Int()) {
		t.Fatal("grave not on terrain below dead unit")
	}
	before := replay.Hash(s)
	c := NewRaster()
	defer c.Close()
	r.drawGrave(c, s, 0, 0, 0, 1)
	if before != replay.Hash(s) {
		t.Fatal("grave changed simulation")
	}
	w.Pos.Y = s.WaterY
	if _, _, ok := gravePosition(s, 0); ok {
		t.Fatal("hazard death left floating grave")
	}
	w.Pos.Y = fixed.FromInt(-20)
	if _, _, ok := gravePosition(s, 0); ok {
		t.Fatal("sky death left floating grave")
	}
}
