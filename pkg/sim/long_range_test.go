package sim

import (
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
	"testing"
)

// Exercise actual launch and swept flight, not just tuning constants. Full
// power at 45 degrees should cross most of the 4096-pixel map in still air.
func TestLongRangeWeaponsCrossWideMap(t *testing.T) {
	for _, weapon := range []Weapon{Bazooka, DrillRocket, Mortar, HomingRocket} {
		for _, left := range []bool{false, true} {
			cfg := DefaultConfig()
			cfg.Width = 4096
			cfg.Height = 2048
			cfg.ReadyTicks = 0
			cfg.Environment = false
			cfg.WeaponDelays = false
			s, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			s.Terrain = terrain.New(4096, 2048, false)
			s.Terrain.StampRect(terrain.Rect{X0: 0, Y0: 1800, X1: 4096, Y1: 2048}, true)
			for i := range s.Worms {
				s.Worms[i].Pos.Y = fixed.FromInt(1800)
			}
			w := &s.Worms[s.Active]
			w.Pos.X = fixed.FromInt(32)
			w.Pos.Y = fixed.FromInt(1800)
			s.Aim = fixed.Angle(57344)
			w.Facing = 1
			targetX := 3900
			if left {
				w.Pos.X = fixed.FromInt(4064)
				w.Facing = -1
				s.Aim = fixed.Angle(40960)
				targetX = 196
			}
			s.Wind = 0
			s.Weapon = weapon
			s.HasTarget = true
			s.Target = fixed.Vec{X: fixed.FromInt(targetX), Y: fixed.FromInt(1500)}
			var effects []Effect
			s.fire(1000, &effects)
			if len(s.Projectiles) != 1 {
				t.Fatal("launch failed", weapon)
			}
			start := s.Projectiles[0].Pos.X
			maxDistance := 0
			for tick := 0; tick < 600 && len(s.Projectiles) > 0; tick++ {
				distance := fixed.Abs(s.Projectiles[0].Pos.X - start).Int()
				if distance > maxDistance {
					maxDistance = distance
				}
				s.moveProjectiles(&effects)
			}
			if maxDistance < 3500 {
				t.Fatalf("weapon %d left=%v only covered %d px", weapon, left, maxDistance)
			}
		}
	}
}
