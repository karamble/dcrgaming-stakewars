// Package protocolcheck contains executable protocol experiments, not a
// production consensus implementation. In particular, unanimity among locally
// observed responders does NOT guarantee agreement under asynchronous delivery.
package protocolcheck

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
)

// Tick is synthetic elapsed time used by the scheduler in tests. It is not a
// claim about chain time or a remotely verifiable wall clock.
type Tick uint64

type Seat struct {
	ID  uint8
	Key [33]byte
}

// Context binds votes to an agreed pre-turn state and its active membership.
// Active is ordered by seat ID. Eliminated players are not active voters.
type Context struct {
	Match  [32]byte
	Head   [32]byte
	Turn   uint32
	Absent uint8
	Active []Seat
}

func (c Context) digest() ([32]byte, error) {
	if len(c.Active) < 2 || len(c.Active) > 6 {
		return [32]byte{}, errors.New("a round requires 2..6 active seats")
	}
	b := []byte("stakewars/experiment/skip-context/v1\x00")
	b = append(b, c.Match[:]...)
	b = append(b, c.Head[:]...)
	b = binary.BigEndian.AppendUint32(b, c.Turn)
	b = append(b, c.Absent, byte(len(c.Active)))
	found := false
	for i, s := range c.Active {
		if s.ID >= 6 || (i > 0 && c.Active[i-1].ID >= s.ID) {
			return [32]byte{}, errors.New("active seats must have unique ascending IDs in 0..5")
		}
		if _, err := secp256k1.ParsePubKey(s.Key[:]); err != nil {
			return [32]byte{}, fmt.Errorf("seat %d: %w", s.ID, err)
		}
		for _, prev := range c.Active[:i] {
			if prev.Key == s.Key {
				return [32]byte{}, errors.New("two seats cannot share a signing key")
			}
		}
		found = found || s.ID == c.Absent
		b = append(b, s.ID)
		b = append(b, s.Key[:]...)
	}
	if !found {
		return [32]byte{}, errors.New("the absent player must be active")
	}
	return sha256.Sum256(b), nil
}

type Kind uint8

const (
	Propose Kind = iota + 1
	Vote
	Answer
)

// Message uses ordinary Schnorr signatures to authenticate this experiment's
// ballots. It is not a game-log signature, equivocation-recovery construction,
// escrow authorization, or a frozen production wire format.
type Message struct {
	Round     [32]byte
	Sender    uint8
	Kind      Kind
	Approve   bool
	Signature [64]byte
}

func (m Message) digest() [32]byte {
	b := []byte("stakewars/experiment/skip-message/v1\x00")
	b = append(b, m.Round[:]...)
	b = append(b, m.Sender, byte(m.Kind))
	if m.Approve {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	return sha256.Sum256(b)
}

func Sign(c Context, sender uint8, kind Kind, approve bool, key *secp256k1.PrivateKey) (Message, error) {
	id, err := c.digest()
	if err != nil {
		return Message{}, err
	}
	if key == nil {
		return Message{}, errors.New("missing signing key")
	}
	m := Message{Round: id, Sender: sender, Kind: kind, Approve: approve}
	digest := m.digest()
	sig, err := schnorr.Sign(key, digest[:])
	if err != nil {
		return Message{}, err
	}
	copy(m.Signature[:], sig.Serialize())
	if err := verify(c, m); err != nil {
		return Message{}, err
	}
	return m, nil
}

func verify(c Context, m Message) error {
	id, err := c.digest()
	if err != nil {
		return err
	}
	if m.Round != id {
		return errors.New("message names another match, turn, state or roster")
	}
	if m.Kind != Propose && m.Kind != Vote && m.Kind != Answer {
		return errors.New("unknown message kind")
	}
	if m.Kind != Vote && m.Approve {
		return errors.New("only votes carry an approval")
	}
	if (m.Kind == Answer) != (m.Sender == c.Absent) {
		return errors.New("only the accused may answer; only others may propose or vote")
	}
	for _, seat := range c.Active {
		if seat.ID != m.Sender {
			continue
		}
		pub, err := secp256k1.ParsePubKey(seat.Key[:])
		if err != nil {
			return err
		}
		sig, err := schnorr.ParseSignature(m.Signature[:])
		if err != nil {
			return err
		}
		digest := m.digest()
		if !sig.Verify(digest[:], pub) {
			return errors.New("invalid signature")
		}
		return nil
	}
	return errors.New("sender is not an active seat")
}

// Timing describes one observer's local view. Different observers can receive
// the same proposal at different times. Test values are not product defaults.
type Timing struct {
	TurnEndsAt Tick
	Grace      Tick
	VoteWindow Tick
}

type Round struct {
	context  Context
	opened   Tick
	closes   Tick
	votes    map[uint8]Message
	answer   *Message
	conflict *[2]Message
}

func Open(c Context, proposal Message, timing Timing, receivedAt Tick) (*Round, error) {
	if err := verify(c, proposal); err != nil {
		return nil, err
	}
	if proposal.Kind != Propose {
		return nil, errors.New("round must start with a proposal")
	}
	eligible := timing.TurnEndsAt + timing.Grace
	closes := receivedAt + timing.VoteWindow
	if eligible < timing.TurnEndsAt || timing.VoteWindow == 0 || closes < receivedAt {
		return nil, errors.New("invalid or overflowing timing")
	}
	if receivedAt < eligible {
		return nil, errors.New("countdown and delivery grace have not expired")
	}
	c.Active = append([]Seat(nil), c.Active...)
	return &Round{context: c, opened: receivedAt, closes: closes, votes: make(map[uint8]Message)}, nil
}

var (
	ErrLate         = errors.New("message arrived after the local response window")
	ErrEquivocation = errors.New("seat signed conflicting votes")
)

// Receive authenticates before changing the observer's state. This experiment
// treats the response window as [opened, closes); late messages cannot rewrite
// its decision. That explicit convention lets tests expose the partition gap.
func (r *Round) Receive(m Message, receivedAt Tick) error {
	if err := verify(r.context, m); err != nil {
		return err
	}
	if receivedAt < r.opened {
		return errors.New("message predates the local round")
	}
	if receivedAt >= r.closes {
		return ErrLate
	}
	switch m.Kind {
	case Vote:
		if prev, ok := r.votes[m.Sender]; ok && prev.Approve != m.Approve {
			r.conflict = &[2]Message{prev, m}
			return ErrEquivocation
		}
		r.votes[m.Sender] = m
	case Answer:
		copy := m
		r.answer = &copy
	default:
		return errors.New("a proposal is not a vote or answer")
	}
	return nil
}

type Decision string

const (
	Pending    Decision = "waiting"
	Cancelled  Decision = "accused answered"
	Rejected   Decision = "a responder voted no"
	NoVotes    Decision = "no affirmative voters"
	Skipped    Decision = "responders agreed to skip"
	Conflicted Decision = "conflicting signed votes"
)

func (r *Round) Decision(at Tick) Decision {
	if r.conflict != nil {
		return Conflicted
	}
	if r.answer != nil {
		return Cancelled
	}
	if at < r.closes {
		return Pending
	}
	if len(r.votes) == 0 {
		return NoVotes
	}
	for _, vote := range r.votes {
		if !vote.Approve {
			return Rejected
		}
	}
	return Skipped
}
