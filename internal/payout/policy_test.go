package payout

import "testing"

func TestEveryOutcomeConservesPot(t *testing.T) {
	for seats := uint8(2); seats <= 6; seats++ {
		p, err := WinnerTakesAll(seats, 1234567)
		if err != nil {
			t.Fatal(err)
		}
		for winner := int8(-1); winner < int8(seats); winner++ {
			shares, err := p.Allocation(winner)
			if err != nil {
				t.Fatal(err)
			}
			var sum int64
			for _, n := range shares {
				sum += n
			}
			if sum != int64(seats)*p.StakeAtoms {
				t.Fatal("money lost")
			}
			if winner >= 0 && shares[winner] != sum {
				t.Fatal("wrong winner")
			}
			if winner == -1 {
				for _, n := range shares {
					if n != p.StakeAtoms {
						t.Fatal("draw changed contributions")
					}
				}
			}
			shares[0]++
			if err := p.Validate(); err != nil {
				t.Fatal("caller mutated terms")
			}
		}
	}
}
func TestCustomAllocationBoundToPolicy(t *testing.T) {
	p, _ := WinnerTakesAll(3, 10000)
	before, _ := p.ID()
	p.Gross[0] = []int64{20000, 7500, 2500}
	after, err := p.ID()
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("custom split not bound")
	}
	p.Gross[3] = []int64{10001, 10000, 9999}
	draw, err := p.ID()
	if err != nil || draw == after {
		t.Fatal("draw split not bound")
	}
}
func TestRejectMalformedAndOverflow(t *testing.T) {
	for _, p := range []Policy{
		{Seats: 6, StakeAtoms: 1 << 62}, {Seats: 0, StakeAtoms: 1}, {Seats: 2, StakeAtoms: -1},
		{Seats: 2, StakeAtoms: 1, Gross: [][]int64{{2, 0}, {0, 2}}},
		{Seats: 2, StakeAtoms: 1, Gross: [][]int64{{2, 0}, {0, 2}, {1, -1}}},
		{Seats: 2, StakeAtoms: 1, Gross: [][]int64{{2, 0}, {0, 2}, {1 << 62, 1 << 62}}},
		{Seats: 2, StakeAtoms: 1, Gross: [][]int64{{1, 0}, {0, 2}, {1, 1}}},
	} {
		if p.Validate() == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
	p, _ := WinnerTakesAll(2, 1)
	for _, winner := range []int8{-2, 2, 127} {
		if _, err := p.Allocation(winner); err == nil {
			t.Fatal("accepted nonexistent outcome")
		}
	}
}
