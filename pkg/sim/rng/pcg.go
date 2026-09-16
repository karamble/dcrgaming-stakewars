// Package rng contains the frozen PCG-XSH-RR generator used by the simulation.
package rng

import "github.com/karamble/dcrstakewars/pkg/sim/fixed"

type PCG struct{ state, inc uint64 }

func New(seed, sequence uint64) PCG {
	p := PCG{inc: sequence<<1 | 1}
	p.Uint32()
	p.state += seed
	p.Uint32()
	return p
}

func (p *PCG) Uint32() uint32 {
	old := p.state
	p.state = old*6364136223846793005 + p.inc
	x := uint32(((old >> 18) ^ old) >> 27)
	r := uint32(old >> 59)
	return x>>r | x<<((-r)&31)
}

func (p *PCG) Intn(n int) int {
	if n <= 0 || uint64(n) > 4294967295 {
		panic("rng: invalid bound")
	}
	bound := uint32(n)
	threshold := -bound % bound
	for {
		if v := p.Uint32(); v >= threshold {
			return int(v % bound)
		}
	}
}
func (p *PCG) Fixed() fixed.F         { return fixed.F(p.Uint32() >> 16) }
func (p PCG) Words() (uint64, uint64) { return p.state, p.inc }
func Restore(state, increment uint64) PCG {
	if increment&1 == 0 {
		panic("rng: increment must be odd")
	}
	return PCG{state, increment}
}
