package sim

import (
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"testing"
)

func TestJetpackControlledLiftAndRelease(t *testing.T) {
	s := flatArena(t)
	start := s.Worms[0].Pos.Y
	eventStep(t, s, Input{Kind: SelectWeapon, Param: int32(Jetpack)}, Input{Kind: Thrust, Param: 1})
	for tick := 1; tick < 60; tick++ {
		eventStep(t, s)
		if s.Worms[0].Vel.Y < -fixed.FromInt(3) {
			t.Fatal("jetpack exceeded climb speed")
		}
	}
	rise := start - s.Worms[0].Pos.Y
	if rise < fixed.FromInt(100) || rise > fixed.FromInt(160) {
		t.Fatal("jetpack lift outside controllable range", rise)
	}
	eventStep(t, s, Input{Kind: Thrust, Param: 0})
	for tick := 0; tick < 9; tick++ {
		eventStep(t, s)
	}
	if s.Worms[0].Vel.Y <= 0 {
		t.Fatal("release did not stop ascent promptly")
	}
}
