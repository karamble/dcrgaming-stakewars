package rng

import "testing"

func TestPCGReferencePrefix(t *testing.T) {
	p := New(42, 54)
	// PCG reference seed/sequence example; catches output variant and seeding changes.
	want := []uint32{0xa15c02b7, 0x7b47f409, 0xba1d3330, 0x83d2f293, 0xbfa4784b, 0xcbed606e}
	for i, v := range want {
		if got := p.Uint32(); got != v {
			t.Fatalf("output %d: %08x want %08x", i, got, v)
		}
	}
	state, inc := p.Words()
	copy := Restore(state, inc)
	for i := 0; i < 1000; i++ {
		if p.Uint32() != copy.Uint32() {
			t.Fatal("restore diverged")
		}
	}
}
func TestBoundedValues(t *testing.T) {
	p := New(99, 4)
	for i := 0; i < 10000; i++ {
		if n := p.Intn(7); n < 0 || n >= 7 {
			t.Fatal(n)
		}
		if f := p.Fixed(); f < 0 || f >= 65536 {
			t.Fatal(f)
		}
	}
}
