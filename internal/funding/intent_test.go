package funding

import (
	"errors"
	"github.com/karamble/dcrgaming-stakewars/internal/durable"
	"sync"
	"sync/atomic"
	"testing"
)

func intent() Intent {
	return Intent{Request: [32]byte{1}, Match: [32]byte{2}, Terms: [32]byte{3}, Seat: 1, Kind: ForfeitBond, Atoms: 200000, Script: []byte{1, 2, 3}}
}
func TestUnknownPaymentNeverDispatchesAgain(t *testing.T) {
	dir := t.TempDir()
	j, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	i := intent()
	send, e := j.Begin(i)
	if e != nil || !send {
		t.Fatal(e)
	}
	// Crash after reservation or uncertain wallet reply: both are unknown.
	j, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	send, e = j.Begin(i)
	if e != nil || send {
		t.Fatal("uncertain payment retried", e)
	}
	i.Atoms++
	if _, e = j.Begin(i); !errors.Is(e, durable.ErrConflict) {
		t.Fatal("amount silently changed", e)
	}
}
func TestConcurrentRequestsDispatchOnlyOnce(t *testing.T) {
	j, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 20; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			send, e := j.Begin(intent())
			if e != nil {
				t.Error(e)
			}
			if send {
				calls.Add(1)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate funding dispatch", calls.Load())
	}
}
func TestFundingObligationCannotChange(t *testing.T) {
	for _, mutate := range []func(*Intent){func(i *Intent) { i.Kind = Stake }, func(i *Intent) { i.Terms[0]++ }, func(i *Intent) { i.Match[0]++ }, func(i *Intent) { i.Seat++ }, func(i *Intent) { i.Script[0]++ }} {
		j, e := Open(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		i := intent()
		if _, e = j.Begin(i); e != nil {
			t.Fatal(e)
		}
		mutate(&i)
		if _, e = j.Begin(i); !errors.Is(e, durable.ErrConflict) {
			t.Fatal("funding obligation rebound", e)
		}
	}
}
