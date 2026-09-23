package sim

import (
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
	"testing"
)

func TestDrillSlowsInMaterialAndRecoversOutside(t *testing.T) {
	s := flatArena(t)
	s.Terrain.StampRect(terrain.Rect{X0: 200, Y0: 60, X1: 255, Y1: 220}, true)
	s.Projectiles = []Projectile{{Weapon: DrillRocket, Pos: fixed.Vec{X: fixed.FromInt(194), Y: fixed.FromInt(100)}, Vel: fixed.Vec{X: fixed.FromInt(12)}, Drill: 100, Fuse: 200}}
	var effects []Effect
	slowed, recovered := false, false
	for i := 0; i < 20; i++ {
		before := s.Projectiles[0].Pos.X
		s.moveProjectiles(&effects)
		if len(s.Projectiles) != 1 {
			t.Fatal("rocket lost while drilling")
		}
		p := s.Projectiles[0]
		distance := p.Pos.X - before
		if p.Pos.X > fixed.FromInt(210) && p.Pos.X < fixed.FromInt(248) {
			if distance >= fixed.FromInt(12) || distance < fixed.FromInt(8) {
				t.Fatal("material did not moderately slow rocket", distance)
			}
			slowed = true
		}
		if p.Pos.X > fixed.FromInt(330) && distance == fixed.FromInt(12) {
			recovered = true
		}
	}
	if !slowed || !recovered {
		t.Fatal("missing drag or restored flight", slowed, recovered)
	}
}
