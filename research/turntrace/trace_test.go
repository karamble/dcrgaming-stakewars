package turntrace

import (
	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	"os"
	"testing"
)

func TestRealSimulationDispute(t *testing.T) {
	for _, name := range []string{"duel", "wide-six-squads"} {
		t.Run(name, func(t *testing.T) {
			f, e := os.Open("../../pkg/replay/testdata/" + name + ".json")
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			r, e := replay.Read(f)
			if e != nil {
				t.Fatal(e)
			}
			honest, size, e := Prefix(r, 512)
			if e != nil {
				t.Fatal(e)
			}
			lie := append([]Hash(nil), honest.States...)
			for i := 260; i < len(lie); i++ {
				lie[i][0] ^= 1
			}
			bad, e := New(honest.Context, lie)
			if e != nil {
				t.Fatal(e)
			}
			d, e := Narrow(honest.Commitment, bad.Commitment, honest.Open, bad.Open)
			if e != nil {
				t.Fatal(e)
			}
			if d.Before != 259 || d.After != 260 || d.Rounds != 9 || d.ResultA != honest.States[260] || d.ResultB == honest.States[260] {
				t.Fatal("wrong disputed transition", d)
			}
			t.Logf("%s: 512 actual simulation ticks; disagreement narrowed in%d midpoint rounds to state%d->%d; largest full state%d bytes; one opening%d bytes", name, d.Rounds, d.Before, d.After, size, 4+32+32*len(honest.Open(260).Siblings))
		})
	}
}
func TestAdversarialOpenings(t *testing.T) {
	st := make([]Hash, 18)
	for i := range st {
		st[i][0] = byte(i)
	}
	a, _ := New(Hash{7}, st)
	st[17][0] ^= 1
	st[3][0] ^= 1 // non-monotonic divergence is permitted.
	b, _ := New(Hash{7}, st)
	if d, e := Narrow(a.Commitment, b.Commitment, a.Open, b.Open); e != nil || d.After != 17 {
		t.Fatal(d, e)
	}
	for _, edit := range []func(*Opening){func(o *Opening) { o.State[0] ^= 1 }, func(o *Opening) { o.Index++ }, func(o *Opening) { o.Siblings = o.Siblings[:len(o.Siblings)-1] }, func(o *Opening) { o.Siblings[0][0] ^= 1 }} {
		_, e := Narrow(a.Commitment, b.Commitment, a.Open, func(i uint32) Opening { o := b.Open(i); edit(&o); return o })
		if e == nil {
			t.Fatal("forged opening accepted")
		}
	}
	c := b.Commitment
	c.Context[0] ^= 1
	if _, e := Narrow(a.Commitment, c, a.Open, b.Open); e == nil {
		t.Fatal("different context accepted")
	}
	c = b.Commitment
	c.Count--
	if _, e := Narrow(a.Commitment, c, a.Open, b.Open); e == nil {
		t.Fatal("different length accepted")
	}
	if Verify(b.Commitment, b.Open(b.Count)) {
		t.Fatal("padding accepted as state")
	}
}
func TestMaximumLengthNarrowing(t *testing.T) {
	st := make([]Hash, MaxTicks+1)
	a, _ := New(Hash{1}, st)
	st[MaxTicks] = Hash{2}
	b, _ := New(Hash{1}, st)
	d, e := Narrow(a.Commitment, b.Commitment, a.Open, b.Open)
	if e != nil || d.Rounds > 15 || d.After != MaxTicks {
		t.Fatal(d, e)
	}
	t.Logf("20000-step synthetic trace: %d midpoint rounds; NOT a transaction count", d.Rounds)
}

func TestRejoiningTracesAndMidpointForgery(t *testing.T) {
	// Divergent traces can rejoin. Do not rely on a monotonic equality predicate.
	for mask := 0; mask < 128; mask++ {
		states := make([]Hash, 9)
		a, _ := New(Hash{9}, states)
		for i := 1; i < 8; i++ {
			if mask&(1<<(i-1)) != 0 {
				states[i][0] = 1
			}
		}
		states[8][0] = 1
		b, _ := New(Hash{9}, states)
		d, e := Narrow(a.Commitment, b.Commitment, a.Open, b.Open)
		if e != nil || d.After != d.Before+1 || a.States[d.Before] != b.States[d.Before] || a.States[d.After] == b.States[d.After] {
			t.Fatal("invalid adjacent dispute", mask, d, e)
		}
		_, e = Narrow(a.Commitment, b.Commitment, a.Open, func(i uint32) Opening {
			o := b.Open(i)
			if i != 0 && i != 8 {
				o.State[0] ^= 1
			}
			return o
		})
		if e == nil {
			t.Fatal("forged midpoint accepted")
		}
	}
}
