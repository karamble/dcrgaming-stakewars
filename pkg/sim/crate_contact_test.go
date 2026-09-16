package sim

import (
	"bytes"
	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"testing"
)

func TestWalkingCollectsCratesOnFirstBodyContact(t *testing.T) {
	for _, direction := range []int{-1, 1} {
		s := flatArena(t)
		w := &s.Worms[0]
		w.Pos.X = fixed.FromInt(250)
		pos := w.Pos
		pos.X += fixed.FromInt(direction * 30)
		s.Objects = []Object{{Kind: HealthCrate, Pos: pos, HP: 20, Settled: true}}
		peer := s.Clone()
		collected := false
		for tick := 0; tick < 20; tick++ {
			eventStep(t, s, Input{Kind: Move, Param: int32(direction)})
			eventStep(t, peer, Input{Kind: Move, Param: int32(direction)})
			if !bytes.Equal(s.AppendCanonical(nil), peer.AppendCanonical(nil)) {
				t.Fatal("pickup replay diverged")
			}
			if crateContact(w.Pos, pos) {
				if len(s.Objects) != 0 {
					t.Fatal("overlapping crate not collected in walking tick")
				}
				collected = true
				break
			}
			if len(s.Objects) == 0 {
				t.Fatal("collected before body contact")
			}
		}
		if !collected {
			t.Fatal("did not reach crate")
		}
	}
}

func TestCrateContactUsesBodyAndCapsInventory(t *testing.T) {
	s := flatArena(t)
	pos := s.Worms[0].Pos
	above := pos
	above.Y -= fixed.FromInt(23)
	if crateContact(above, pos) {
		t.Fatal("collected across air gap")
	}
	below := pos
	below.Y += fixed.FromInt(21)
	if crateContact(below, pos) {
		t.Fatal("collected through floor below crate")
	}
	for _, ammo := range []uint8{19, 20, 255} {
		s.Ammo[0][HeavyCluster] = ammo
		s.Objects = []Object{{Kind: AmmoCrate, Pos: pos, HP: 20, Settled: true, Weapon: HeavyCluster}}
		var effects []Effect
		s.moveObjects(&effects)
		s.moveObjects(&effects)
		want := ammo
		if want < 20 {
			want++
		}
		if len(s.Objects) != 0 || len(effects) != 1 || s.Ammo[0][HeavyCluster] != want {
			t.Fatal("pickup duplicated or ammo cap corrupted")
		}
	}
	s.Worms[0].HP = 0
	s.Objects = []Object{{Kind: HealthCrate, Pos: pos, HP: 20, Settled: true}}
	var effects []Effect
	s.moveObjects(&effects)
	if len(s.Objects) != 1 {
		t.Fatal("dead unit collected crate")
	}
}
