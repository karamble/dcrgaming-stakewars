// Package payout models explicit pre-agreed gross allocations for experimental
// match terms. It neither chooses the agreed outcome nor authorizes spending.
package payout

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const MaxAtoms int64 = 21_000_000 * 100_000_000

// Policy has one row for each possible sole-winning seat, followed by a draw
// row. Each row explicitly assigns the entire gross pot across all seats.
// Rank-based or team results need a different versioned outcome model.
type Policy struct {
	StakeAtoms int64 // same stake per seat, excluding separate bond deposits
	Seats      uint8
	Gross      [][]int64
}

func (p Policy) Validate() error {
	if p.Seats < 2 || p.Seats > 6 || p.StakeAtoms <= 0 || p.StakeAtoms > MaxAtoms/int64(p.Seats) {
		return errors.New("invalid stake or seats")
	}
	if len(p.Gross) != int(p.Seats)+1 {
		return errors.New("every winner and draw need an allocation")
	}
	pot := p.StakeAtoms * int64(p.Seats)
	for _, row := range p.Gross {
		if len(row) != int(p.Seats) {
			return errors.New("allocation must name every seat")
		}
		sum := int64(0)
		for _, atoms := range row {
			if atoms < 0 || atoms > pot-sum {
				return errors.New("allocation exceeds pot or is negative")
			}
			sum += atoms
		}
		if sum != pot {
			return errors.New("gross allocation must equal funded stake pot")
		}
	}
	return nil
}
func (p Policy) ID() ([32]byte, error) {
	if err := p.Validate(); err != nil {
		return [32]byte{}, err
	}
	b := []byte("StakeWars/experiment/payout-policy/v1\x00")
	b = append(b, p.Seats)
	b = binary.LittleEndian.AppendUint64(b, uint64(p.StakeAtoms))
	for _, row := range p.Gross {
		for _, atoms := range row {
			b = binary.LittleEndian.AppendUint64(b, uint64(atoms))
		}
	}
	return sha256.Sum256(b), nil
}

// Allocation returns a private copy of the gross SDK shares. Winner -1 is a
// draw. The SDK builder deducts fees; callers must not deduct them here too.
func (p Policy) Allocation(winner int8) ([]int64, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if winner < -1 || winner >= int8(p.Seats) {
		return nil, errors.New("unknown outcome")
	}
	row := int(winner)
	if winner == -1 {
		row = int(p.Seats)
	}
	return append([]int64(nil), p.Gross[row]...), nil
}
func WinnerTakesAll(seats uint8, stake int64) (Policy, error) {
	p := Policy{Seats: seats, StakeAtoms: stake}
	if seats < 2 || seats > 6 || stake <= 0 || stake > MaxAtoms/int64(seats) {
		return Policy{}, errors.New("invalid stake or seats")
	}
	for winner := uint8(0); winner < seats; winner++ {
		row := make([]int64, seats)
		row[winner] = stake * int64(seats)
		p.Gross = append(p.Gross, row)
	}
	draw := make([]int64, seats)
	for i := range draw {
		draw[i] = stake
	}
	p.Gross = append(p.Gross, draw)
	return p, p.Validate()
}
