package tablelobby

import (
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"testing"
)

func TestUnknownAndStaleEvidenceNeverReady(t *testing.T) {
	t0 := New(bridgeconn.Invitation{Seats: 4, Until: 100})
	if t0.Stage() != 0 {
		t.Fatal("unknown invitation advanced")
	}
	t0.Reviewed = true
	t0.Connected = true
	t0.ChainKnown = true
	if t0.Stage() != 1 {
		t.Fatal("empty roster advanced")
	}
	ready := Demo(4, 5)
	if ready.Stage() != 7 {
		t.Fatal("demo missing readiness")
	}
	ready.Stale = true
	if ready.Stage() != 1 {
		t.Fatal("stale checks retained readiness")
	}
	ready = Demo(4, 5)
	ready.Seats[0].Bond.Confirmations = 1
	if ready.Stage() != 5 {
		t.Fatal("confirmation rollback ignored")
	}
	ready = Demo(4, 5)
	ready.Seats[0].Admission.Checked = false
	if ready.Stage() != 1 {
		t.Fatal("peer claim treated as verified")
	}
	ready = Demo(4, 5)
	ready.WorldVerified = false
	if ready.Stage() != 6 {
		t.Fatal("map disagreement did not block readiness")
	}
}
func TestPaymentRequiresAllEvidence(t *testing.T) {
	for _, p := range []Payment{{}, {Phase: Verified}, {Phase: Verified, Checked: true}, {Phase: Verified, Checked: true, Required: 2, Confirmations: 1}, {Phase: Announced, Checked: true, Required: 2, Confirmations: 2}} {
		if p.Complete() {
			t.Fatal("incomplete payment marked verified", p)
		}
	}
	for _, n := range []int{2, 4, 6} {
		for stage := 0; stage < 7; stage++ {
			v := Demo(n, stage)
			if len(v.Seats) != n {
				t.Fatal("seat count")
			}
			a, b := v.Counts()
			if a > n || b > n {
				t.Fatal("bad counts")
			}
			if v.Stage() > 7 {
				t.Fatal("bad phase")
			}
		}
	}
}
