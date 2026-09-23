// Package turntrace studies dispute narrowing. It is not an escrow verifier.
package turntrace

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
)

type Hash [32]byte

const MaxTicks = 20000

type Commitment struct {
	Context Hash
	Count   uint32
	Root    Hash
}
type Opening struct {
	Index    uint32
	State    Hash
	Siblings []Hash
}
type Trace struct {
	Commitment
	States   []Hash
	nodes    []Hash
	capacity int
}

func tagged(tag byte, parts ...[]byte) Hash {
	h := sha256.New()
	h.Write([]byte{tag})
	for _, p := range parts {
		h.Write(p)
	}
	var out Hash
	copy(out[:], h.Sum(nil))
	return out
}
func number(v uint32) []byte                       { return binary.LittleEndian.AppendUint32(nil, v) }
func leaf(ctx Hash, index uint32, state Hash) Hash { return tagged(0, ctx[:], number(index), state[:]) }
func parent(a, b Hash) Hash                        { return tagged(1, a[:], b[:]) }
func boundRoot(ctx Hash, count uint32, root Hash) Hash {
	return tagged(3, ctx[:], number(count), root[:])
}
func New(ctx Hash, states []Hash) (*Trace, error) {
	if len(states) < 2 || len(states) > MaxTicks+1 {
		return nil, errors.New("trace length out of bounds")
	}
	n := 1
	for n < len(states) {
		n *= 2
	}
	tr := &Trace{Commitment: Commitment{Context: ctx, Count: uint32(len(states))}, States: append([]Hash(nil), states...), nodes: make([]Hash, 2*n), capacity: n}
	for i := 0; i < n; i++ {
		if i < len(states) {
			tr.nodes[n+i] = leaf(ctx, uint32(i), states[i])
		} else {
			tr.nodes[n+i] = tagged(2, ctx[:], number(uint32(i)))
		}
	}
	for i := n - 1; i > 0; i-- {
		tr.nodes[i] = parent(tr.nodes[2*i], tr.nodes[2*i+1])
	}
	tr.Root = boundRoot(ctx, tr.Count, tr.nodes[1])
	return tr, nil
}
func (tr *Trace) Open(i uint32) Opening {
	if i >= tr.Count {
		return Opening{Index: i}
	}
	o := Opening{Index: i, State: tr.States[i]}
	p := tr.capacity + int(i)
	for p > 1 {
		o.Siblings = append(o.Siblings, tr.nodes[p^1])
		p /= 2
	}
	return o
}
func Verify(c Commitment, o Opening) bool {
	if c.Count < 2 || c.Count > MaxTicks+1 || o.Index >= c.Count {
		return false
	}
	n, depth := 1, 0
	for n < int(c.Count) {
		n *= 2
		depth++
	}
	if len(o.Siblings) != depth {
		return false
	}
	h := leaf(c.Context, o.Index, o.State)
	i := o.Index
	for _, s := range o.Siblings {
		if i&1 == 0 {
			h = parent(h, s)
		} else {
			h = parent(s, h)
		}
		i >>= 1
	}
	return boundRoot(c.Context, c.Count, h) == c.Root
}

type Dispute struct {
	Before, After              uint32
	Rounds                     int
	Prestate, ResultA, ResultB Hash
}

// Narrow preserves equal low endpoints and unequal high endpoints. It does not
// assume divergence is monotonic, and authenticates every opening to its root.
func Narrow(a, b Commitment, openA, openB func(uint32) Opening) (Dispute, error) {
	var out Dispute
	if a.Context != b.Context || a.Count != b.Count || a.Count < 2 || a.Count > MaxTicks+1 {
		return out, errors.New("incompatible claims")
	}
	pair := func(i uint32) (Opening, Opening, error) {
		x, y := openA(i), openB(i)
		if x.Index != i || y.Index != i || !Verify(a, x) || !Verify(b, y) {
			return x, y, errors.New("invalid committed opening")
		}
		return x, y, nil
	}
	lo, hi := uint32(0), a.Count-1
	left, right, e := pair(lo)
	if e != nil {
		return out, e
	}
	if left.State != right.State {
		return out, errors.New("initial states disagree")
	}
	low := left.State
	left, right, e = pair(hi)
	if e != nil {
		return out, e
	}
	if left.State == right.State {
		return out, errors.New("final results agree")
	}
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		x, y, e := pair(mid)
		if e != nil {
			return out, e
		}
		out.Rounds++
		if x.State == y.State {
			lo = mid
			low = x.State
		} else {
			hi = mid
			left, right = x, y
		}
	}
	out.Before, out.After, out.Prestate, out.ResultA, out.ResultB = lo, hi, low, left.State, right.State
	return out, nil
}

// Prefix runs actual StakeWars physics. SHA256 here is research-only; production
// replay.Hash uses BLAKE3. The context binds input bytes/config but does not
// establish participant authorization, availability or canonical game history.
func Prefix(r replay.Recording, ticks uint32) (*Trace, int, error) {
	if ticks == 0 || ticks > MaxTicks || ticks > r.Ticks || r.Version != sim.Version {
		return nil, 0, errors.New("invalid prefix")
	}
	for i, v := range r.Inputs {
		if v.Tick >= r.Ticks || (i > 0 && (v.Tick < r.Inputs[i-1].Tick || (v.Tick == r.Inputs[i-1].Tick && v.Sequence <= r.Inputs[i-1].Sequence))) {
			return nil, 0, errors.New("unordered inputs")
		}
	}
	end := 0
	for end < len(r.Inputs) && r.Inputs[end].Tick < ticks {
		end++
	}
	body, e := json.Marshal(struct {
		Domain  string
		Version uint32
		Config  sim.Config
		Ticks   uint32
		Inputs  []sim.Input
	}{"StakeWars/research/turntrace/v1", r.Version, r.Config, ticks, r.Inputs[:end]})
	if e != nil {
		return nil, 0, e
	}
	ctx := Hash(sha256.Sum256(body))
	s, e := sim.New(r.Config)
	if e != nil {
		return nil, 0, e
	}
	states := make([]Hash, 0, ticks+1)
	buf := s.AppendCanonical(nil)
	maxBytes := len(buf)
	states = append(states, Hash(sha256.Sum256(buf)))
	next := 0
	for tick := uint32(0); tick < ticks; tick++ {
		start := next
		for next < end && r.Inputs[next].Tick == tick {
			next++
		}
		if s.Phase == sim.Ended {
			return nil, 0, errors.New("prefix exceeds match end")
		}
		if _, e = sim.Step(s, r.Inputs[start:next]); e != nil {
			return nil, 0, fmt.Errorf("tick%d: %w", tick, e)
		}
		buf = s.AppendCanonical(buf[:0])
		if len(buf) > maxBytes {
			maxBytes = len(buf)
		}
		states = append(states, Hash(sha256.Sum256(buf)))
	}
	tr, e := New(ctx, states)
	return tr, maxBytes, e
}
