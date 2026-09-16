// Package replay records and verifies internal deterministic simulation runs.
// This file format is not a signed paid-match transcript.
package replay

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/karamble/dcrstakewars/pkg/sim"
	"lukechampine.com/blake3"
)

const MaxTicks = 2_000_000
const MaxBytes = 32 << 20

type Recording struct {
	Version   uint32
	Config    sim.Config
	Ticks     uint32
	Inputs    []sim.Input
	FinalHash string
}

func Hash(s *sim.State) [32]byte  { return blake3.Sum256(s.AppendCanonical(nil)) }
func HexHash(s *sim.State) string { h := Hash(s); return hex.EncodeToString(h[:]) }

func (r Recording) Run() (*sim.State, error) {
	if r.Version != sim.Version {
		return nil, errors.New("unsupported simulation version")
	}
	if r.Ticks > MaxTicks || len(r.Inputs) > 1_000_000 {
		return nil, errors.New("replay exceeds resource limits")
	}
	for i, v := range r.Inputs {
		if v.Tick >= r.Ticks || (i > 0 && (r.Inputs[i-1].Tick > v.Tick || (r.Inputs[i-1].Tick == v.Tick && r.Inputs[i-1].Sequence >= v.Sequence))) {
			return nil, errors.New("unordered or out-of-range replay input")
		}
	}
	s, err := sim.New(r.Config)
	if err != nil {
		return nil, err
	}
	next := 0
	for tick := uint32(0); tick < r.Ticks; tick++ {
		if s.Phase == sim.Ended {
			return nil, errors.New("replay continues after the match ended")
		}
		start := next
		for next < len(r.Inputs) && r.Inputs[next].Tick == tick {
			next++
		}
		if _, err = sim.Step(s, r.Inputs[start:next]); err != nil {
			return nil, fmt.Errorf("tick %d: %w", tick, err)
		}
	}
	if r.FinalHash != "" && HexHash(s) != r.FinalHash {
		return nil, errors.New("final state hash mismatch")
	}
	return s, nil
}

func Read(src io.Reader) (Recording, error) {
	limited := io.LimitReader(src, MaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Recording{}, err
	}
	if len(data) > MaxBytes {
		return Recording{}, errors.New("replay file too large")
	}
	var r Recording
	// Decode through a bounded byte reader and reject trailing documents.
	err = decode(data, &r)
	return r, err
}
func Write(dst io.Writer, r Recording) error {
	enc := json.NewEncoder(dst)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
