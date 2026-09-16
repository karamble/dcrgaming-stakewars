package session

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/karamble/dcrgaming-sdk/pkg/forfeit"
	"github.com/karamble/dcrstakewars/internal/durable"
)

// World preparation occupies checkpoint sequence zero. Gameplay must start its
// checkpoints at one. This book persists the exact digest before SDK signing.
type signingBook struct {
	mu    sync.Mutex
	store *durable.Store
	fault error
}

func positionKey(p forfeit.Position) [32]byte {
	raw, _ := json.Marshal(p)
	return sha256.Sum256(append([]byte("StakeWars/signing-position/v1\x00"), raw...))
}
func (b *signingBook) Used(p forfeit.Position) ([32]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var hash [32]byte
	raw, err := b.store.Get(positionKey(p))
	if errors.Is(err, os.ErrNotExist) {
		return hash, false
	}
	if err != nil {
		b.fault = err
		return hash, false
	}
	if len(raw) != 32 {
		b.fault = errors.New("invalid signing reservation")
		return hash, false
	}
	copy(hash[:], raw)
	return hash, true
}
func (b *signingBook) Record(p forfeit.Position, hash [32]byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fault != nil {
		return b.fault
	}
	return b.store.Put(positionKey(p), hash[:])
}
