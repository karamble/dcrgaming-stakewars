// Package sound creates original short PCM effects. It has no connection to
// simulation randomness, clocks, inputs or state hashing.
package sound

import (
	"encoding/binary"
	"math"
)

const SampleRate = 22050
const (
	Step = iota
	Jump
	Land
	Fire
	Blast
	Hurt
	Pickup
	Ready
)

// PCM returns stereo signed 16-bit little-endian samples at SampleRate.
func PCM(kind, weapon int) []byte {
	duration := []float64{.055, .18, .09, .22, .65, .13, .3, .09}
	if kind < 0 || kind >= len(duration) {
		return nil
	}
	n := int(duration[kind] * SampleRate)
	out := make([]byte, n*4)
	seed := uint32(721 + weapon*97)
	phase := 0.0
	filtered := 0.0
	for i := 0; i < n; i++ {
		t := float64(i) / SampleRate
		u := float64(i) / float64(n)
		seed = seed*1664525 + 1013904223
		noise := float64(seed>>8)/8388608 - 1
		filtered = filtered*.8 + noise*.2
		envelope := math.Sin(math.Min(1, u*20)*math.Pi/2) * math.Pow(1-u, 2)
		v := 0.0
		switch kind {
		case Step:
			v = filtered*.9 + math.Sin(2*math.Pi*180*t)*.1
		case Jump:
			phase += 2 * math.Pi * (180 + 600*u) / SampleRate
			v = math.Sin(phase) * .55
		case Land:
			v = filtered*.8 + math.Sin(2*math.Pi*80*t)*.3
		case Fire:
			phase += 2 * math.Pi * (float64(90+weapon*13)*(1-u) + 35) / SampleRate
			v = noise*.32 + math.Sin(phase)*.45
		case Blast:
			phase += 2 * math.Pi * (65 - 40*u) / SampleRate
			v = filtered*1.5 + math.Sin(phase)*.3
		case Hurt:
			phase += 2 * math.Pi * (350 - 220*u) / SampleRate
			v = math.Sin(phase) * .5
		case Pickup:
			freq := 660.0
			if u > .33 {
				freq = 880
			}
			if u > .66 {
				freq = 1100
			}
			phase += 2 * math.Pi * freq / SampleRate
			v = math.Sin(phase) * .5
		case Ready:
			v = math.Sin(2*math.Pi*660*t) * .4
		}
		sample := int16(math.Max(-1, math.Min(1, v*envelope)) * 18000)
		binary.LittleEndian.PutUint16(out[i*4:], uint16(sample))
		binary.LittleEndian.PutUint16(out[i*4+2:], uint16(sample))
	}
	return out
}
