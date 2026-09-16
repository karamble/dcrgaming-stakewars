package render

import (
	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
	"testing"
)

func TestWeaponAvailabilityExplainsSelectionLock(t *testing.T) {
	s, e := sim.New(sim.DefaultConfig())
	if e != nil {
		t.Fatal(e)
	}
	if WeaponUnavailable(s, sim.Airstrike) != "WAIT 1R" {
		t.Fatal("missing unlock reason")
	}
	s.Ammo[s.ActiveSeat()][sim.BisonBomb] = 0
	if WeaponUnavailable(s, sim.BisonBomb) != "EMPTY" {
		t.Fatal("missing empty reason")
	}
	s.Phase = sim.Playing
	s.Shots = 1
	s.Weapon = sim.Shotgun
	if WeaponUnavailable(s, sim.Shotgun) != "" || WeaponUnavailable(s, sim.Bazooka) != "ATTACK USED" {
		t.Fatal("second shotgun shot mislabeled")
	}
	s.Weapon = sim.RemoteCharge
	s.Ammo[s.ActiveSeat()][sim.RemoteCharge] = 0
	if WeaponUnavailable(s, sim.RemoteCharge) != "" {
		t.Fatal("last remote charge cannot be detonated in HUD")
	}
	s.Phase = sim.Retreating
	if WeaponUnavailable(s, sim.Bazooka) != "TURN LOCKED" {
		t.Fatal("missing phase reason")
	}
}
func TestFeedbackAllPhasesPreservesState(t *testing.T) {
	for _, phase := range []sim.Phase{sim.Ready, sim.Playing, sim.Retreating, sim.Resolving, sim.Ended} {
		s, e := sim.New(sim.DefaultConfig())
		if e != nil {
			t.Fatal(e)
		}
		s.Phase = phase
		if phase == sim.Playing {
			s.PhaseTick = s.Config.TurnTicks - 60
		}
		before := replay.Hash(s)
		r := New(s)
		c := NewRaster()
		r.Draw(c, s, View{Arena: true, HideTop: true, HideBottom: true})
		c.Close()
		if replay.Hash(s) != before {
			t.Fatal("feedback mutated game")
		}
	}
}
