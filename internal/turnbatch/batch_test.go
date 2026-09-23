package turnbatch

import (
	"bytes"
	"os"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
)

func fixture(t *testing.T) (Context, *sim.State, *sim.State, []sim.Input, *secp256k1.PrivateKey) {
	t.Helper()
	c := Context{Match: [32]byte{1}, Terms: [32]byte{2}}
	var key *secp256k1.PrivateKey
	for i := 1; i <= 3; i++ {
		k := secp256k1.PrivKeyFromBytes([]byte{byte(i)})
		var pub [33]byte
		copy(pub[:], k.PubKey().SerializeCompressed())
		c.Keys = append(c.Keys, pub)
		if i == 1 {
			key = k
		}
	}
	cfg := sim.DefaultConfig()
	cfg.Seats = 3
	cfg.TurnTicks = 60
	cfg.RetreatTicks = 3
	before, err := sim.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	in := []sim.Input{{Tick: 0, Seat: 0, Kind: sim.Move, Param: 1}, {Tick: 20, Seat: 0, Kind: sim.Move, Param: 0}}
	next := 0
	for after.Turn == before.Turn && after.Phase != sim.Ended {
		var tick []sim.Input
		if next < len(in) && in[next].Tick == after.Tick {
			tick = in[next : next+1]
			next++
		}
		if _, err := sim.Step(after, tick); err != nil {
			t.Fatal(err)
		}
		if after.Tick > MaxTicks {
			t.Fatal("unbounded turn")
		}
	}
	return c, before, after, in, key
}
func TestVerifyAndDecode(t *testing.T) {
	c, before, after, in, key := fixture(t)
	old := replay.Hash(before)
	b, err := Sign(c, before, after, in, key)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := b.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := decoded.Verify(c, before)
		if err != nil || replay.Hash(got) != replay.Hash(after) {
			t.Fatalf("verify: %v", err)
		}
	}
	if replay.Hash(before) != old {
		t.Fatal("verification mutated agreed state")
	}
	for _, bad := range [][]byte{wire[:len(wire)-1], append(append([]byte(nil), wire...), 0)} {
		if _, err := Decode(bytes.NewReader(bad)); err == nil {
			t.Fatal("accepted noncanonical length")
		}
	}
}
func TestRejectTamperingAndOtherContexts(t *testing.T) {
	c, before, after, in, key := fixture(t)
	b, err := Sign(c, before, after, in, key)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*Batch){"signature": func(b *Batch) { b.Signature[5] ^= 1 }, "post-state": func(b *Batch) { b.After[0] ^= 1 }, "input": func(b *Batch) { b.Inputs[0].Param = -1 }, "head": func(b *Batch) { b.Before[0] ^= 1 }, "turn": func(b *Batch) { b.Turn++ }, "author": func(b *Batch) { b.Seat = 1 }, "session": func(b *Batch) { b.Session[0] ^= 1 }}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			v := b
			v.Inputs = append([]sim.Input(nil), b.Inputs...)
			edit(&v)
			if _, err := v.Verify(c, before); err == nil {
				t.Fatal("accepted tampering")
			}
		})
	}
	other := c
	other.Terms[0]++
	if _, err := b.Verify(other, before); err == nil {
		t.Fatal("accepted different terms")
	}
	other = c
	other.Keys = append([][33]byte(nil), c.Keys...)
	other.Keys[0], other.Keys[1] = other.Keys[1], other.Keys[0]
	if _, err := b.Verify(other, before); err == nil {
		t.Fatal("accepted different seating")
	}
	if _, err := b.Verify(c, after); err == nil {
		t.Fatal("accepted stale replay")
	}
}
func TestSignerCannotChooseFalseResultOrCrossTurns(t *testing.T) {
	c, before, after, in, key := fixture(t)
	bad := after.Clone()
	bad.Worms[1].HP--
	if _, err := Sign(c, before, bad, in, key); err == nil {
		t.Fatal("signed false result")
	}
	short := before.Clone()
	sim.Step(short, in[:1])
	if _, err := Sign(c, before, short, in[:1], key); err == nil {
		t.Fatal("signed partial turn")
	}
	long := after.Clone()
	sim.Step(long, nil)
	if _, err := Sign(c, before, long, in, key); err == nil {
		t.Fatal("signed past boundary")
	}
	if _, err := Sign(c, before, after, in, secp256k1.PrivKeyFromBytes([]byte{9})); err == nil {
		t.Fatal("signed as wrong seat")
	}
}
func FuzzDecode(f *testing.F) {
	valid, err := (Batch{End: 1}).MarshalBinary()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add([]byte{})
	f.Add(append([]byte(nil), magic...))
	f.Fuzz(func(t *testing.T, data []byte) {
		b, err := Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		wire, err := b.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, wire) {
			t.Fatal("noncanonical round trip")
		}
	})
}

func TestSixPeersReplayCompleteMatch(t *testing.T) {
	f, err := os.Open("../../pkg/replay/testdata/six-squads.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := replay.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	c := Context{Match: [32]byte{4}, Terms: [32]byte{5}}
	var keys []*secp256k1.PrivateKey
	var peers []*sim.State
	for i := 0; i < int(r.Config.Seats); i++ {
		k := secp256k1.PrivKeyFromBytes([]byte{byte(i + 1)})
		keys = append(keys, k)
		var pub [33]byte
		copy(pub[:], k.PubKey().SerializeCompressed())
		c.Keys = append(c.Keys, pub)
		s, err := sim.New(r.Config)
		if err != nil {
			t.Fatal(err)
		}
		peers = append(peers, s)
	}
	author := peers[0].Clone()
	before := author.Clone()
	cursor := 0
	start := 0
	turns := 0
	for author.Tick < r.Ticks {
		at := cursor
		for cursor < len(r.Inputs) && r.Inputs[cursor].Tick == author.Tick {
			cursor++
		}
		if _, err := sim.Step(author, r.Inputs[at:cursor]); err != nil {
			t.Fatal(err)
		}
		if author.Turn == before.Turn && author.Phase != sim.Ended {
			continue
		}
		batch, err := Sign(c, before, author, r.Inputs[start:cursor], keys[before.ActiveSeat()])
		if err != nil {
			t.Fatal(err)
		}
		wire, err := batch.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		for i := len(peers) - 1; i >= 0; i-- {
			received, err := Decode(bytes.NewReader(wire))
			if err != nil {
				t.Fatal(err)
			}
			next, err := received.Verify(c, peers[i])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := received.Verify(c, next); err == nil {
				t.Fatal("duplicate accepted against advanced head")
			}
			peers[i] = next
		}
		before = author.Clone()
		start = cursor
		turns++
	}
	if turns < 10 {
		t.Fatal("not enough turn boundaries exercised")
	}
	for _, p := range peers {
		if replay.HexHash(p) != r.FinalHash {
			t.Fatal("peer diverged")
		}
	}
}
