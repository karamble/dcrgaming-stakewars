package protocolcheck

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// All keys are deterministic fixtures, never production identities.
func fixture(n int) (Context, []*secp256k1.PrivateKey) {
	c := Context{Match: sha256.Sum256([]byte("test match")), Head: sha256.Sum256([]byte("agreed turn start")), Turn: 7, Absent: 0}
	keys := make([]*secp256k1.PrivateKey, n)
	for i := range n {
		seed := sha256.Sum256([]byte(fmt.Sprintf("stakewars test-only seat %d", i)))
		keys[i] = secp256k1.PrivKeyFromBytes(seed[:])
		s := Seat{ID: uint8(i)}
		copy(s.Key[:], keys[i].PubKey().SerializeCompressed())
		c.Active = append(c.Active, s)
	}
	return c, keys
}

func signed(t *testing.T, c Context, keys []*secp256k1.PrivateKey, seat uint8, kind Kind, yes bool) Message {
	t.Helper()
	m, err := Sign(c, seat, kind, yes, keys[seat])
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func round(t *testing.T, c Context, keys []*secp256k1.PrivateKey) *Round {
	t.Helper()
	r, err := Open(c, signed(t, c, keys, 1, Propose, false), Timing{TurnEndsAt: 45, Grace: 5, VoteWindow: 10}, 50)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func receive(t *testing.T, r *Round, m Message, at Tick) {
	t.Helper()
	if err := r.Receive(m, at); err != nil {
		t.Fatal(err)
	}
}

func TestRespondersAgreeWhileSilentPlayersAbstain(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprintf("%d seats", n), func(t *testing.T) {
			c, keys := fixture(n)
			r := round(t, c, keys)
			receive(t, r, signed(t, c, keys, 1, Vote, true), 51)
			// All other players are silent, including the accused.
			if got := r.Decision(59); got != Pending {
				t.Fatalf("before deadline: %s", got)
			}
			if got := r.Decision(60); got != Skipped {
				t.Fatalf("silent players blocked responders: %s", got)
			}
		})
	}
}

func TestProposalAloneDoesNotCountAsAnAffirmativeVote(t *testing.T) {
	c, keys := fixture(3)
	if got := round(t, c, keys).Decision(60); got != NoVotes {
		t.Fatalf("empty voter set authorized a skip: %s", got)
	}
}

func TestDissentAndAccusedAnswer(t *testing.T) {
	for _, tc := range []struct {
		name string
		seat uint8
		kind Kind
		want Decision
	}{
		{"responding player disagrees", 2, Vote, Rejected},
		{"accused answers", 0, Answer, Cancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, keys := fixture(3)
			r := round(t, c, keys)
			receive(t, r, signed(t, c, keys, 1, Vote, true), 51)
			receive(t, r, signed(t, c, keys, tc.seat, tc.kind, false), 59)
			if got := r.Decision(60); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDeliveryOrderAndDuplicatesDoNotChangeTheSameView(t *testing.T) {
	c, keys := fixture(6)
	a, b := round(t, c, keys), round(t, c, keys)
	for i := uint8(1); i < 6; i++ {
		receive(t, a, signed(t, c, keys, i, Vote, true), 52)
	}
	for i := uint8(5); i > 0; i-- {
		m := signed(t, c, keys, i, Vote, true)
		receive(t, b, m, 53)
		receive(t, b, m, 54)
	}
	if a.Decision(60) != Skipped || b.Decision(60) != Skipped || len(b.votes) != 5 {
		t.Fatal("order or duplicate delivery changed the result")
	}
}

func TestInvalidMessagesDoNotChangeRound(t *testing.T) {
	c, keys := fixture(3)
	good := signed(t, c, keys, 1, Vote, true)
	for _, tc := range []struct {
		name string
		edit func(*Message)
	}{
		{"seat spoof", func(m *Message) { m.Sender = 2 }},
		{"spectator", func(m *Message) { m.Sender = 5 }},
		{"changed decision", func(m *Message) { m.Approve = false }},
		{"changed context", func(m *Message) { m.Round[0] ^= 1 }},
		{"forged signature", func(m *Message) { m.Signature[10] ^= 1 }},
		{"invalid kind", func(m *Message) { m.Kind = 99 }},
		{"forged accused answer", func(m *Message) { m.Kind, m.Sender, m.Approve = Answer, 0, false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := round(t, c, keys)
			m := good
			tc.edit(&m)
			if err := r.Receive(m, 51); err == nil {
				t.Fatal("invalid message accepted")
			}
			if len(r.votes) != 0 || r.answer != nil || r.conflict != nil || r.Decision(60) != NoVotes {
				t.Fatal("invalid message changed state")
			}
		})
	}
}

func TestVotesAreBoundToMatchTurnHeadAndRoster(t *testing.T) {
	c, keys := fixture(3)
	m := signed(t, c, keys, 1, Vote, true)
	for _, edit := range []func(*Context){
		func(c *Context) { c.Match[0] ^= 1 },
		func(c *Context) { c.Head[0] ^= 1 },
		func(c *Context) { c.Turn++ },
		func(c *Context) { c.Absent = 2 },
		func(c *Context) { c.Active = c.Active[:2] },
	} {
		other := c
		edit(&other)
		if err := verify(other, m); err == nil {
			t.Fatal("vote replayed into a different context")
		}
	}
}

func TestConflictingVotesRetainEvidenceAndBlockLocalDecision(t *testing.T) {
	c, keys := fixture(3)
	r := round(t, c, keys)
	yes, no := signed(t, c, keys, 1, Vote, true), signed(t, c, keys, 1, Vote, false)
	receive(t, r, yes, 51)
	if err := r.Receive(no, 52); !errors.Is(err, ErrEquivocation) {
		t.Fatalf("conflicting vote: %v", err)
	}
	if r.conflict == nil || *r.conflict != [2]Message{yes, no} || r.Decision(60) != Conflicted {
		t.Fatal("lost conflicting statements or authorized a skip")
	}
}

func TestCountdownGraceAndResponseWindow(t *testing.T) {
	c, keys := fixture(3)
	p := signed(t, c, keys, 1, Propose, false)
	for _, tc := range []struct {
		name   string
		timing Timing
		at     Tick
	}{
		{"turn not over", Timing{45, 5, 10}, 44},
		{"grace not over", Timing{45, 5, 10}, 49},
		{"empty vote window", Timing{45, 5, 0}, 50},
		{"grace overflow", Timing{^Tick(0), 2, 1}, 50},
		{"vote window overflow", Timing{45, 5, 10}, ^Tick(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Open(c, p, tc.timing, tc.at); err == nil {
				t.Fatal("invalid timing accepted")
			}
		})
	}
	r := round(t, c, keys)
	receive(t, r, signed(t, c, keys, 1, Vote, true), 59)
	if err := r.Receive(signed(t, c, keys, 0, Answer, false), 60); !errors.Is(err, ErrLate) {
		t.Fatalf("deadline boundary: %v", err)
	}
	if r.Decision(60) != Skipped {
		t.Fatal("late answer rewrote the local result")
	}
}

// Passing this test means the unsafe execution is reproduced, NOT that the
// protocol is safe. Both observers receive authentic messages. No key is forged
// and no player equivocates. One simply cannot see the other's dissent in time.
func TestCounterexampleDelayedDissentSplitsHonestObservers(t *testing.T) {
	c, keys := fixture(3)
	a, b := round(t, c, keys), round(t, c, keys)
	yes := signed(t, c, keys, 1, Vote, true)
	no := signed(t, c, keys, 2, Vote, false)
	receive(t, a, yes, 51)
	receive(t, b, yes, 51)
	receive(t, b, no, 52)
	if a.Decision(60) != Skipped || b.Decision(60) != Rejected {
		t.Fatal("expected one observer to skip while the other records dissent")
	}
	if err := a.Receive(no, 61); !errors.Is(err, ErrLate) {
		t.Fatalf("delayed dissent: %v", err)
	}
	t.Log("same signed proposal: observer A skips, observer B rejects; local responder unanimity is not consensus")
}

func TestCounterexampleDelayedAnswerSplitsHonestObservers(t *testing.T) {
	c, keys := fixture(3)
	a, b := round(t, c, keys), round(t, c, keys)
	yes, answer := signed(t, c, keys, 1, Vote, true), signed(t, c, keys, 0, Answer, false)
	receive(t, a, yes, 51)
	receive(t, b, yes, 51)
	receive(t, b, answer, 59)
	if a.Decision(60) != Skipped || b.Decision(60) != Cancelled {
		t.Fatal("expected one observer to skip while the other accepts the answer")
	}
	if err := a.Receive(answer, 61); !errors.Is(err, ErrLate) {
		t.Fatal("late answer unexpectedly changed observer A")
	}
	t.Log("the accused answered, but only one observer received it before its deadline")
}

func TestOpeningCopiesMembership(t *testing.T) {
	c, keys := fixture(3)
	m := signed(t, c, keys, 1, Vote, true)
	r := round(t, c, keys)
	c.Active[1].ID = 5
	receive(t, r, m, 51)
}
