package turnbatch

import (
	"bytes"
	"testing"
	"time"

	sdkwire "github.com/karamble/dcrgaming-sdk/pkg/gaming/wire"
	"github.com/karamble/dcrstakewars/pkg/replay"
)

// Bison Relay carries these as discrete stored messages. Chunking a completed
// batch is not a live action stream. This test exercises the actual SDK codec.
func TestSDKFragmentsReorderAndDuplicate(t *testing.T) {
	c, before, after, in, key := fixture(t)
	b, err := Sign(c, before, after, in, key)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := b.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	frames, err := sdkwire.Encode("stakewars", 1, "0123456789abcdef", payload, sdkwire.ClassState.Deadline(now), 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) < 2 {
		t.Fatal("not fragmented")
	}
	assembler := sdkwire.NewAssembler(sdkwire.AssemblerConfig{MaxMessageBytes: maxWireBytes, MaxTotalBytes: maxWireBytes * 2, MaxPendingPerSender: 2})
	var completed []byte
	for i := len(frames) - 1; i >= 0; i-- {
		part, ok := sdkwire.Parse(frames[i])
		if !ok {
			t.Fatal("SDK rejected its frame")
		}
		got, err := assembler.Add("authorized-fixture-sender", part, now)
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			if got != nil {
				t.Fatal("delivered incomplete message")
			}
			duplicate, err := assembler.Add("authorized-fixture-sender", part, now)
			if err != nil || duplicate != nil {
				t.Fatal("duplicate fragment changed assembly")
			}
		} else {
			completed = got
		}
	}
	if !bytes.Equal(completed, payload) {
		t.Fatal("reassembly changed signed bytes")
	}
	received, err := Decode(bytes.NewReader(completed))
	if err != nil {
		t.Fatal(err)
	}
	s, err := received.Verify(c, before)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Hash(s) != replay.Hash(after) {
		t.Fatal("transport replay diverged")
	}
	dir := t.TempDir()
	journal, err := OpenJournal(dir, c, before)
	if err != nil {
		t.Fatal(err)
	}
	if err = journal.Receive(received); err != nil {
		t.Fatal(err)
	}
	journal, err = OpenJournal(dir, c, before)
	if err != nil {
		t.Fatal(err)
	}
	if err = journal.Receive(received); err != nil {
		t.Fatal("redelivery after reconnect", err)
	}
	if replay.Hash(journal.Head()) != replay.Hash(after) {
		t.Fatal("durable SDK delivery diverged")
	}
	if assembler.Pending() != 0 {
		t.Fatal("completed fragments retained")
	}
}
