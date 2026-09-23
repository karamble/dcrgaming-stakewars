// Package turnbatch verifies experimental asynchronous completed-turn messages.
// It does not establish consensus or authorize escrow. Journal provides local
// durable turn history and signing reservations for this experimental codec.
package turnbatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
)

const MaxInputs = 65536
const MaxTicks = 20000
const maxWireBytes = 256 + MaxInputs*12

var magic = []byte("StakeWars/experiment/turn/v1\x00")

type Context struct {
	Match, Terms [32]byte
	// Keys are in canonical seated order. BR routing identity is not authority.
	Keys [][33]byte
}

func (c Context) ID() ([32]byte, error) {
	if c.Match == [32]byte{} || c.Terms == [32]byte{} || len(c.Keys) < 2 || len(c.Keys) > 6 {
		return [32]byte{}, errors.New("invalid match context")
	}
	b := []byte("StakeWars/experiment/session/v1\x00")
	b = append(b, c.Match[:]...)
	b = append(b, c.Terms[:]...)
	b = append(b, byte(len(c.Keys)))
	for i, k := range c.Keys {
		if _, err := secp256k1.ParsePubKey(k[:]); err != nil {
			return [32]byte{}, err
		}
		for _, p := range c.Keys[:i] {
			if k == p {
				return [32]byte{}, errors.New("duplicate seat key")
			}
		}
		b = append(b, k[:]...)
	}
	return sha256.Sum256(b), nil
}

type Batch struct {
	Session, Before, After [32]byte
	Turn, Start, End       uint32
	Seat                   uint8
	Inputs                 []sim.Input
	Signature              [64]byte
}

func (b Batch) body() ([]byte, error) {
	if b.End <= b.Start || b.End-b.Start > MaxTicks || len(b.Inputs) > MaxInputs || b.Seat >= 6 {
		return nil, errors.New("turn batch exceeds bounds")
	}
	data := append([]byte(nil), magic...)
	data = append(data, b.Session[:]...)
	data = append(data, b.Before[:]...)
	data = append(data, b.After[:]...)
	data = binary.LittleEndian.AppendUint32(data, b.Turn)
	data = binary.LittleEndian.AppendUint32(data, b.Start)
	data = binary.LittleEndian.AppendUint32(data, b.End)
	data = append(data, b.Seat)
	data = binary.LittleEndian.AppendUint32(data, uint32(len(b.Inputs)))
	for i, v := range b.Inputs {
		if v.Tick < b.Start || v.Tick >= b.End || v.Seat != b.Seat || (i > 0 && (v.Tick < b.Inputs[i-1].Tick || (v.Tick == b.Inputs[i-1].Tick && v.Sequence <= b.Inputs[i-1].Sequence))) {
			return nil, errors.New("invalid input ordering or seat")
		}
		data = binary.LittleEndian.AppendUint32(data, v.Tick)
		data = append(data, v.Seat)
		data = binary.LittleEndian.AppendUint16(data, v.Sequence)
		data = append(data, byte(v.Kind))
		data = binary.LittleEndian.AppendUint32(data, uint32(v.Param))
	}
	return data, nil
}
func (b Batch) MarshalBinary() ([]byte, error) {
	data, err := b.body()
	if err != nil {
		return nil, err
	}
	return append(data, b.Signature[:]...), nil
}
func Decode(src io.Reader) (Batch, error) {
	var b Batch
	data, err := io.ReadAll(io.LimitReader(src, maxWireBytes+1))
	if err != nil {
		return b, err
	}
	if len(data) > maxWireBytes || len(data) < len(magic)+96+12+1+4+64 || !bytes.HasPrefix(data, magic) {
		return b, errors.New("invalid turn frame")
	}
	r := bytes.NewReader(data[len(magic):])
	r.Read(b.Session[:])
	r.Read(b.Before[:])
	r.Read(b.After[:])
	binary.Read(r, binary.LittleEndian, &b.Turn)
	binary.Read(r, binary.LittleEndian, &b.Start)
	binary.Read(r, binary.LittleEndian, &b.End)
	b.Seat, _ = r.ReadByte()
	var count uint32
	binary.Read(r, binary.LittleEndian, &count)
	if count > MaxInputs || r.Len() != int(count)*12+64 {
		return Batch{}, errors.New("invalid turn frame length")
	}
	b.Inputs = make([]sim.Input, count)
	for i := range b.Inputs {
		v := &b.Inputs[i]
		binary.Read(r, binary.LittleEndian, &v.Tick)
		v.Seat, _ = r.ReadByte()
		binary.Read(r, binary.LittleEndian, &v.Sequence)
		k, _ := r.ReadByte()
		v.Kind = sim.EventKind(k)
		binary.Read(r, binary.LittleEndian, &v.Param)
	}
	r.Read(b.Signature[:])
	_, err = b.body()
	return b, err
}

// Replay checks one complete turn, working on a private copy. No supplied
// post-state is trusted. Exactly the first turn boundary must end the batch.
func (b Batch) replay(before *sim.State) (*sim.State, error) {
	if before == nil || ((before.Config.ReadyTicks > 0 && before.Phase != sim.Ready) || (before.Config.ReadyTicks == 0 && before.Phase != sim.Playing)) || before.PhaseTick != 0 || b.Turn != before.Turn || b.Start != before.Tick || b.Seat != before.ActiveSeat() || b.Before != replay.Hash(before) {
		return nil, errors.New("batch does not extend this turn head")
	}
	if _, err := b.body(); err != nil {
		return nil, err
	}
	s := before.Clone()
	next := 0
	for s.Tick < b.End {
		start := next
		for next < len(b.Inputs) && b.Inputs[next].Tick == s.Tick {
			next++
		}
		if _, err := sim.Step(s, b.Inputs[start:next]); err != nil {
			return nil, fmt.Errorf("tick %d: %w", s.Tick, err)
		}
		if s.Turn != before.Turn || s.Phase == sim.Ended {
			if s.Tick != b.End {
				return nil, errors.New("batch crosses a turn boundary")
			}
			if b.After != replay.Hash(s) {
				return nil, errors.New("post-state mismatch")
			}
			return s, nil
		}
	}
	return nil, errors.New("incomplete turn")
}
func (b Batch) authenticate(c Context) error {
	id, err := c.ID()
	if err != nil {
		return err
	}
	if b.Session != id || int(b.Seat) >= len(c.Keys) {
		return errors.New("wrong session or seat")
	}
	data, err := b.body()
	if err != nil {
		return err
	}
	pub, err := secp256k1.ParsePubKey(c.Keys[b.Seat][:])
	if err != nil {
		return err
	}
	sig, err := schnorr.ParseSignature(b.Signature[:])
	if err != nil {
		return err
	}
	h := sha256.Sum256(data)
	if !sig.Verify(h[:], pub) {
		return errors.New("invalid turn signature")
	}
	return nil
}
func (b Batch) Verify(c Context, before *sim.State) (*sim.State, error) {
	if before == nil || int(before.Config.Seats) != len(c.Keys) {
		return nil, errors.New("wrong seat count")
	}
	if err := b.authenticate(c); err != nil {
		return nil, err
	}
	return b.replay(before)
}

func prepare(c Context, before, after *sim.State, inputs []sim.Input, key *secp256k1.PrivateKey) (Batch, error) {
	id, err := c.ID()
	if err != nil {
		return Batch{}, err
	}
	if before == nil || after == nil || key == nil || int(before.Config.Seats) != len(c.Keys) {
		return Batch{}, errors.New("missing state/key or wrong seats")
	}
	seat := before.ActiveSeat()
	if int(seat) >= len(c.Keys) || !bytes.Equal(key.PubKey().SerializeCompressed(), c.Keys[seat][:]) {
		return Batch{}, errors.New("key is not active seat")
	}
	b := Batch{Session: id, Before: replay.Hash(before), After: replay.Hash(after), Turn: before.Turn, Start: before.Tick, End: after.Tick, Seat: seat, Inputs: append([]sim.Input(nil), inputs...)}
	if _, err = b.replay(before); err != nil {
		return Batch{}, err
	}
	return b, nil
}

// Sign is for internal fixtures only. Production use needs a durable
// persist-before-send signing journal and the SDK's reviewed signing scheme.
func Sign(c Context, before, after *sim.State, inputs []sim.Input, key *secp256k1.PrivateKey) (Batch, error) {
	b, err := prepare(c, before, after, inputs, key)
	if err != nil {
		return Batch{}, err
	}
	data, err := b.body()
	if err != nil {
		return Batch{}, err
	}
	h := sha256.Sum256(data)
	sig, err := schnorr.Sign(key, h[:])
	if err != nil {
		return Batch{}, err
	}
	copy(b.Signature[:], sig.Serialize())
	_, err = b.Verify(c, before)
	return b, err
}
