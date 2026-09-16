package turnbatch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

func nextIdle(t *testing.T, c Context, before *sim.State) (Batch, *sim.State) {
	t.Helper()
	after := before.Clone()
	for after.Turn == before.Turn && after.Phase != sim.Ended {
		if _, err := sim.Step(after, nil); err != nil {
			t.Fatal(err)
		}
	}
	key := secp256k1.PrivKeyFromBytes([]byte{before.ActiveSeat() + 1})
	b, err := Sign(c, before, after, nil, key)
	if err != nil {
		t.Fatal(err)
	}
	return b, after
}
func TestJournalReorderDuplicateRestartAndResend(t *testing.T) {
	c, genesis, after, in, key := fixture(t)
	dir := t.TempDir()
	j, err := OpenJournal(dir, c, genesis)
	if err != nil {
		t.Fatal(err)
	}
	first, err := j.SignedTurn(after, in, key)
	if err != nil {
		t.Fatal(err)
	}
	second, head := nextIdle(t, c, after)
	if err = j.Receive(second); !errors.Is(err, ErrMissingTurn) {
		t.Fatal(err)
	}
	if replay.Hash(j.Head()) != replay.Hash(genesis) {
		t.Fatal("future turn advanced head")
	}
	if err = j.Receive(first); err != nil {
		t.Fatal(err)
	}
	if replay.Hash(j.Head()) != replay.Hash(head) {
		t.Fatal("buffer was not replayed")
	}
	for _, b := range []Batch{first, second, first} {
		if err = j.Receive(b); err != nil {
			t.Fatal(err)
		}
	}
	recovered, err := OpenJournal(dir, c, genesis)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.SyncPoint() != j.SyncPoint() {
		t.Fatal("restart lost verified head")
	}
	wire, err := recovered.AcceptedTurn(0)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := first.MarshalBinary()
	if !bytes.Equal(wire, expected) {
		t.Fatal("resend changed frame")
	}
	copy := j.Head()
	copy.Worms[0].HP = 0
	if replay.Hash(j.Head()) != replay.Hash(head) {
		t.Fatal("head leaked mutable state")
	}
}
func TestJournalReservesBeforeSigningAndRejectsForkAfterRestart(t *testing.T) {
	c, genesis, after, in, key := fixture(t)
	dir := t.TempDir()
	j, err := OpenJournal(dir, c, genesis)
	if err != nil {
		t.Fatal(err)
	}
	b, err := j.SignedTurn(after, in, key)
	if err != nil {
		t.Fatal(err)
	}
	j, err = OpenJournal(dir, c, genesis)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := j.ResumeSignedTurn(key)
	if err != nil || retry.Signature != b.Signature {
		t.Fatal("identical retry changed", err)
	}
	_, alternative := nextIdle(t, c, genesis)
	if _, err = j.SignedTurn(alternative, nil, key); !errors.Is(err, ErrConflictingTurn) {
		t.Fatal("signed conflicting turn", err)
	}
	id, _ := c.ID()
	path := journalKey('s', id, 0)
	data, err := j.store.Get(path)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := b.body()
	if !bytes.Equal(data, body) {
		t.Fatal("intent not stored")
	}
}
func TestJournalRejectsForgeryCorruptionAndChangedGenesis(t *testing.T) {
	c, g, a, in, key := fixture(t)
	dir := t.TempDir()
	j, err := OpenJournal(dir, c, g)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Sign(c, g, a, in, key)
	if err != nil {
		t.Fatal(err)
	}
	bad := b
	bad.Signature[0] ^= 1
	if err = j.Receive(bad); err == nil {
		t.Fatal("forgery accepted")
	}
	if replay.Hash(j.Head()) != replay.Hash(g) {
		t.Fatal("invalid input advanced state")
	}
	if err = j.Receive(b); err != nil {
		t.Fatal(err)
	}
	changed := g.Clone()
	changed.Worms[0].HP--
	if _, err = OpenJournal(dir, c, changed); err == nil {
		t.Fatal("changed genesis accepted")
	}
	id, _ := c.ID()
	record := fmt.Sprintf("%s/%x.record", dir, journalKey('a', id, 0))
	if err = os.WriteFile(record, []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenJournal(dir, c, g); err == nil {
		t.Fatal("corrupt recovery accepted")
	}
}
func TestJournalStorageFailureReturnsNoSignature(t *testing.T) {
	c, g, a, in, key := fixture(t)
	dir := t.TempDir()
	j, err := OpenJournal(dir, c, g)
	if err != nil {
		t.Fatal(err)
	}
	// Rename the directory so journal writes fail without relying on permissions.
	if err = os.Rename(dir, dir+"-moved"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(dir+"-moved", dir)
	b, err := j.SignedTurn(a, in, key)
	if err == nil || b.Signature != [64]byte{} {
		t.Fatal("signature escaped failed persistence")
	}
}

func TestJournalRecoveryTwoToSixPeers(t *testing.T) {
	for seats := 2; seats <= 6; seats++ {
		c := Context{Match: [32]byte{byte(seats)}, Terms: [32]byte{9}}
		for seat := 0; seat < seats; seat++ {
			k := secp256k1.PrivKeyFromBytes([]byte{byte(seat + 1)})
			var pub [33]byte
			copy(pub[:], k.PubKey().SerializeCompressed())
			c.Keys = append(c.Keys, pub)
		}
		cfg := sim.DefaultConfig()
		cfg.Seats = uint8(seats)
		cfg.TurnTicks = 30
		cfg.RetreatTicks = 3
		cfg.ReadyTicks = 0
		genesis, err := sim.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		peers := make([]*Journal, seats)
		dirs := make([]string, seats)
		for i := range peers {
			dirs[i] = t.TempDir()
			peers[i], err = OpenJournal(dirs[i], c, genesis)
			if err != nil {
				t.Fatal(err)
			}
		}
		first, after := nextIdle(t, c, genesis)
		second, final := nextIdle(t, c, after)
		for i := len(peers) - 1; i >= 0; i-- {
			if err = peers[i].Receive(second); !errors.Is(err, ErrMissingTurn) {
				t.Fatal(err)
			}
			// Reconnect discards future buffer: retransmission is required, not guessed.
			peers[i], err = OpenJournal(dirs[i], c, genesis)
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range []Batch{first, first, second, second} {
				if err = peers[i].Receive(b); err != nil {
					t.Fatal(err)
				}
			}
			peers[i], err = OpenJournal(dirs[i], c, genesis)
			if err != nil {
				t.Fatal(err)
			}
			if replay.Hash(peers[i].Head()) != replay.Hash(final) {
				t.Fatal("reconnected peer disagrees", seats, i)
			}
		}
	}
}
