package turnbatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"sync"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
	"github.com/karamble/dcrstakewars/internal/durable"
	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

const MaxPendingTurns = 6

var ErrMissingTurn = errors.New("missing earlier turn; retransmit from sync point")
var ErrConflictingTurn = durable.ErrConflict

type SyncPoint struct {
	Session, Head [32]byte
	Turn, Tick    uint32
}

// Journal persists signing intent and verified turns. It does not decide skip
// consensus or use forfeitable signatures. One match starts at an agreed genesis.
// Filesystem durability requires a local filesystem supporting hard links and
// directory fsync. Storage errors fail closed; no signature is returned.
type Journal struct {
	mu      sync.Mutex
	store   *durable.Store
	context Context
	head    *sim.State
	pending map[uint32]Batch
}

func journalKey(kind byte, session [32]byte, turn uint32) [32]byte {
	b := append([]byte("StakeWars/experiment/journal/v1\x00"), kind)
	b = append(b, session[:]...)
	b = binary.LittleEndian.AppendUint32(b, turn)
	return sha256.Sum256(b)
}
func OpenJournal(dir string, c Context, genesis *sim.State) (*Journal, error) {
	id, err := c.ID()
	if err != nil {
		return nil, err
	}
	if genesis == nil || genesis.Turn != 0 || genesis.Tick != 0 || int(genesis.Config.Seats) != len(c.Keys) {
		return nil, errors.New("journal needs agreed genesis")
	}
	store, err := durable.Open(dir)
	if err != nil {
		return nil, err
	}
	// Bind genesis, including version/config/terrain, before accepting any history.
	hash := replay.Hash(genesis)
	if err = store.Put(journalKey('g', id, 0), hash[:]); err != nil {
		return nil, err
	}
	c.Keys = append([][33]byte(nil), c.Keys...)
	j := &Journal{store: store, context: c, head: genesis.Clone(), pending: map[uint32]Batch{}}
	for j.head.Phase != sim.Ended {
		data, err := store.Get(journalKey('a', id, j.head.Turn))
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return nil, err
		}
		b, err := Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		next, err := b.Verify(c, j.head)
		if err != nil {
			return nil, err
		}
		j.head = next
	}
	return j, nil
}
func (j *Journal) Head() *sim.State { j.mu.Lock(); defer j.mu.Unlock(); return j.head.Clone() }
func (j *Journal) SyncPoint() SyncPoint {
	j.mu.Lock()
	defer j.mu.Unlock()
	id, _ := j.context.ID()
	return SyncPoint{Session: id, Head: replay.Hash(j.head), Turn: j.head.Turn, Tick: j.head.Tick}
}

// SignedTurn reserves the exact unsigned body durably BEFORE signing. A changed
// state, input or end tick at the same session/turn is refused, even after restart.
// Returned bytes can be resent unchanged after an uncertain transport result.
func (j *Journal) SignedTurn(after *sim.State, inputs []sim.Input, key *secp256k1.PrivateKey) (Batch, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	b, err := prepare(j.context, j.head, after, inputs, key)
	if err != nil {
		return Batch{}, err
	}
	data, err := b.body()
	if err != nil {
		return Batch{}, err
	}
	if err = j.store.Put(journalKey('s', b.Session, b.Turn), data); err != nil {
		return Batch{}, err
	}
	hash := sha256.Sum256(data)
	sig, err := schnorr.Sign(key, hash[:])
	if err != nil {
		return Batch{}, err
	}
	copy(b.Signature[:], sig.Serialize())
	return b, nil
}

// Receive authenticates before buffering, replays against the local head, and
// persists before advancing it. Future turns are bounded and volatile: reconnect
// requests retransmission from SyncPoint. It never infers absence from a timeout.
func (j *Journal) Receive(b Batch) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := b.authenticate(j.context); err != nil {
		return err
	}
	data, err := b.MarshalBinary()
	if err != nil {
		return err
	}
	b.Inputs = append([]sim.Input(nil), b.Inputs...)
	key := journalKey('a', b.Session, b.Turn)
	if b.Turn < j.head.Turn || j.head.Phase == sim.Ended {
		old, err := j.store.Get(key)
		if err != nil {
			return err
		}
		if !bytes.Equal(old, data) {
			return ErrConflictingTurn
		}
		return nil
	}
	if b.Turn-j.head.Turn > MaxPendingTurns {
		return ErrMissingTurn
	}
	if old, ok := j.pending[b.Turn]; ok {
		oldData, _ := old.MarshalBinary()
		if !bytes.Equal(oldData, data) {
			return ErrConflictingTurn
		}
	}
	if b.Turn > j.head.Turn {
		j.pending[b.Turn] = b
		return ErrMissingTurn
	}
	for {
		next, err := b.Verify(j.context, j.head)
		if err != nil {
			delete(j.pending, b.Turn)
			return err
		}
		data, _ := b.MarshalBinary()
		if err = j.store.Put(journalKey('a', b.Session, b.Turn), data); err != nil {
			return err
		}
		j.head = next
		delete(j.pending, b.Turn)
		if j.head.Phase == sim.Ended {
			return nil
		}
		var ok bool
		b, ok = j.pending[j.head.Turn]
		if !ok {
			return nil
		}
	}
}

// AcceptedTurn returns the exact durable frame for retransmission.
func (j *Journal) AcceptedTurn(turn uint32) ([]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	id, _ := j.context.ID()
	return j.store.Get(journalKey('a', id, turn))
}

// ResumeSignedTurn recovers the reserved complete turn after a crash, even if
// the caller lost its in-memory inputs and post-state. It returns no signature
// when there is no reservation or it cannot independently replay that body.
func (j *Journal) ResumeSignedTurn(key *secp256k1.PrivateKey) (Batch, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	id, _ := j.context.ID()
	data, err := j.store.Get(journalKey('s', id, j.head.Turn))
	if err != nil {
		return Batch{}, err
	}
	b, err := Decode(bytes.NewReader(append(append([]byte(nil), data...), make([]byte, 64)...)))
	if err != nil {
		return Batch{}, err
	}
	if b.Session != id {
		return Batch{}, errors.New("reserved session mismatch")
	}
	after, err := b.replay(j.head)
	if err != nil {
		return Batch{}, err
	}
	checked, err := prepare(j.context, j.head, after, b.Inputs, key)
	if err != nil {
		return Batch{}, err
	}
	body, err := checked.body()
	if err != nil {
		return Batch{}, err
	}
	if !bytes.Equal(body, data) {
		return Batch{}, errors.New("reservation differs from verified intent")
	}
	// Retrying sync also handles an earlier ambiguous directory-sync failure.
	if err = j.store.Put(journalKey('s', id, b.Turn), body); err != nil {
		return Batch{}, err
	}
	h := sha256.Sum256(body)
	sig, err := schnorr.Sign(key, h[:])
	if err != nil {
		return Batch{}, err
	}
	copy(checked.Signature[:], sig.Serialize())
	return checked, nil
}
