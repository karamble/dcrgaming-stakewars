package sim

import (
	"bytes"
	"testing"
)

func TestTerrainHasMultipleRoutesAndGaps(t *testing.T) {
	s, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	layered, gaps := 0, 0
	for x := 20; x < int(s.Config.Width)-20; x++ {
		runs := 0
		previous := false
		for y := 0; y < s.WaterY.Int(); y++ {
			solid := s.Terrain.Solid(x, y)
			if solid && !previous {
				runs++
			}
			previous = solid
		}
		if runs >= 2 {
			layered++
		}
		if runs == 0 {
			gaps++
		}
	}
	if layered < 100 || gaps < 20 {
		t.Fatalf("missing multi-level terrain: %d layered columns, %d gaps", layered, gaps)
	}
}
func TestComplexSpawnsAcrossSeeds(t *testing.T) {
	for seed := uint64(0); seed < 16; seed++ {
		c := DefaultConfig()
		c.Seed = seed
		c.Seats = 6
		c.Units = 8
		s, err := New(c)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		copy, err := New(c)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(s.AppendCanonical(nil), copy.AppendCanonical(nil)) {
			t.Fatal("generation nondeterministic")
		}
		for i, w := range s.Worms {
			if s.blocked(w.Pos) || !s.supported(w.Pos) || w.Pos.Y >= s.WaterY {
				t.Fatalf("seed %d unsafe spawn %d", seed, i)
			}
		}
	}
}
