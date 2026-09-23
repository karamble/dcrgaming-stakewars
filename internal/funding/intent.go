// Package funding tracks experimental funding requests. It never calls a wallet
// or infers a payment failure from silence. It is not yet wired to the bridge.
package funding

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/karamble/dcrgaming-stakewars/internal/durable"
	"github.com/karamble/dcrgaming-stakewars/internal/payout"
)

type Kind uint8

const (
	EntryBond Kind = iota
	Stake
	TableBond
	ForfeitBond
)

type Intent struct {
	// Request identifies one logical obligation, stable across retries/restarts.
	// Terms and Script bind exactly what the approval flow would fund.
	Request, Match, Terms [32]byte
	Seat                  uint8
	Kind                  Kind
	Atoms                 int64
	Script                []byte
}

func (i Intent) body() ([]byte, error) {
	if i.Request == [32]byte{} || i.Match == [32]byte{} || i.Terms == [32]byte{} || i.Seat >= 6 || i.Kind > ForfeitBond || i.Atoms <= 0 || i.Atoms > payout.MaxAtoms || len(i.Script) == 0 || len(i.Script) > 16384 {
		return nil, errors.New("invalid funding intent")
	}
	b := []byte("StakeWars/experiment/funding/v1\x00")
	b = append(b, i.Request[:]...)
	b = append(b, i.Match[:]...)
	b = append(b, i.Terms[:]...)
	b = append(b, i.Seat, byte(i.Kind))
	b = binary.LittleEndian.AppendUint64(b, uint64(i.Atoms))
	b = binary.LittleEndian.AppendUint32(b, uint32(len(i.Script)))
	b = append(b, i.Script...)
	return b, nil
}
func intentKey(id [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("StakeWars/funding-request/v1\x00"), id[:]...))
}

type Journal struct{ store *durable.Store }

func Open(dir string) (*Journal, error) {
	s, e := durable.Open(dir)
	if e != nil {
		return nil, e
	}
	return &Journal{store: s}, nil
}

// Begin must complete before calling the bridge approval/funding API. Only true
// permits that initial call. False means a previous attempt may have paid: query
// the bridge/chain by the same request, never dispatch another payment. This
// deliberately favors an unresolved payment over spending twice after a crash.
func (j *Journal) Begin(i Intent) (bool, error) {
	b, e := i.body()
	if e != nil {
		return false, e
	}
	return j.store.Create(intentKey(i.Request), b)
}
