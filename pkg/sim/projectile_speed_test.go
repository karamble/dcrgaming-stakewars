package sim

import (
	"testing"

	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

func TestProjectileNormalFlightWithSweptCollision(t *testing.T) {
	s := flatArena(t)
	s.Wind = 0
	s.Projectiles = []Projectile{{Weapon: Bazooka, Pos: fixed.Vec{X: fixed.FromInt(100), Y: fixed.FromInt(100)}, Vel: fixed.Vec{X: fixed.FromInt(10)}}}
	var effects []Effect
	s.moveProjectiles(&effects)
	if len(s.Projectiles) != 1 || s.Projectiles[0].Pos.X != fixed.FromInt(110) || s.Projectiles[0].Pos.Y != fixed.FromInt(100)+fixed.Ratio(1, 16) {
		t.Fatal("flight did not advance one trajectory step")
	}
	s.Projectiles[0].Pos = fixed.Vec{X: fixed.FromInt(100), Y: fixed.FromInt(100)}
	s.Projectiles[0].Vel = fixed.Vec{X: fixed.FromInt(10)}
	s.Projectiles[0].Radius = 8
	s.Terrain.StampRect(terrain.Rect{X0: 105, Y0: 0, X1: 106, Y1: 300}, true)
	s.moveProjectiles(&effects)
	if len(s.Projectiles) != 0 || len(effects) != 1 || effects[0].Pos.X.Int() >= 105 {
		t.Fatal("projectile tunneled through a one-pixel wall")
	}
}

func TestProjectileFusesUseRealTimeSeconds(t *testing.T) {
	for _, weapon := range []Weapon{Grenade, BouncingBomb, Dynamite, RemoteCharge} {
		s := flatArena(t)
		s.Projectiles = []Projectile{{Weapon: weapon, Pos: fixed.Vec{X: fixed.FromInt(50), Y: fixed.FromInt(299)}, Fuse: 180, Radius: 8}}
		var effects []Effect
		for i := 0; i < 179; i++ {
			s.moveProjectiles(&effects)
		}
		if len(s.Projectiles) != 1 || s.Projectiles[0].Fuse != 1 || len(effects) != 0 {
			t.Fatal("flight speed shortened explosive fuse", weapon)
		}
		s.moveProjectiles(&effects)
		if len(s.Projectiles) != 0 || len(effects) != 1 {
			t.Fatal("fuse failed to expire at three seconds", weapon)
		}
	}
}
