package tablelobby

import (
	"strings"
	"testing"

	"github.com/karamble/dcrgaming-stakewars/internal/bridgeconn"
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

func TestBondCardLabelShowsConfirmationProgress(t *testing.T) {
	p := Payment{Phase: Confirming, Checked: true, Confirmations: 1, Required: 2}
	if got := p.BondCardLabel(); got != "BOND POSTED · 1/2 CONFIRMATIONS" {
		t.Fatalf("progress label = %q", got)
	}
	p.Phase = Verified
	p.Confirmations = 2
	if got := p.BondCardLabel(); got != "BOND VERIFIED · 2/2 CONFIRMATIONS" {
		t.Fatalf("verified label = %q", got)
	}
}

func TestFinishedTableNamesItsResult(t *testing.T) {
	table := Demo(2, 7)
	if table.Stage() != 8 {
		t.Fatalf("finished stage = %d, want 8", table.Stage())
	}
	table.Stale = true
	if table.Stage() != 8 {
		t.Fatal("stale chain checks took a finished match back to funding")
	}
	title, body := table.Guidance()
	if title != "Match finished · Player 2 won" {
		t.Fatalf("title = %q", title)
	}
	if !strings.HasPrefix(body, "Payout 5eed5eed5eed5eed… confirmed.") {
		t.Fatalf("confirmed payout guidance = %q", body)
	}
	if table.SeatStatus(1) != "WINNER" || table.SeatStatus(0) != "MATCH FINISHED" {
		t.Fatalf("seat lines = %q / %q", table.SeatStatus(0), table.SeatStatus(1))
	}
	table.Winner = -1
	if title, _ = table.Guidance(); title != "Match finished · draw" || table.SeatStatus(0) != "DRAW" {
		t.Fatalf("draw = %q / %q", title, table.SeatStatus(0))
	}
	for payout, want := range map[string]string{
		"published": "left escrow",
		"signing":   "signs the payout",
		"proposing": "Proposing the payout",
	} {
		table.Payout = payout
		if _, body = table.Guidance(); !strings.Contains(body, want) {
			t.Errorf("%s: guidance %q", payout, body)
		}
	}
}

func TestPaidOutStakeIsNeverRejected(t *testing.T) {
	paid := Payment{Phase: PaidOut, Checked: true}
	if got := paid.Label(); got != "Paid out · spent by the table's payout" {
		t.Fatalf("paid-out label = %q", got)
	}
	if got := (Payment{Phase: Locked, UnlockHeight: 1120857}).Label(); got != "Locked until block 1120857 · recover in dcrpulse" {
		t.Fatalf("locked label = %q", got)
	}
	if got := (Payment{Phase: Locked}).Label(); got != "Locked · recover in dcrpulse after its lock" {
		t.Fatalf("locked label without a height = %q", got)
	}
}

func TestOnlyAPaidOutOrClosedLiveTableCloses(t *testing.T) {
	table := Demo(2, 7)
	if table.Closable() {
		t.Fatal("a demo table can be closed")
	}
	table.Live = true
	if !table.Closable() {
		t.Fatal("a paid-out table cannot be closed")
	}
	table.Payout = "published"
	if table.Closable() {
		t.Fatal("a table closes before its payout confirmed")
	}
	table.Finished, table.Closed = false, true
	if !table.Closable() {
		t.Fatal("a table closed for recovery cannot be put away")
	}
}

func TestLiveGuidanceSeparatesSeatingFromFunding(t *testing.T) {
	table := Demo(2, 5)
	table.Live = true
	table.WorldVerified = false
	table.Status = "Waiting for seating block: 1 more confirmation"
	for i := range table.Seats {
		table.Seats[i].Stake = Payment{}
	}

	title, body := table.Guidance()
	if title != table.Status {
		t.Fatalf("title = %q, want live seating status %q", title, table.Status)
	}
	if body != "Players independently verify the same seating block, map and starting positions. No payment approval is needed." {
		t.Fatalf("unexpected map-check guidance: %q", body)
	}

	table.WorldVerified = true
	title, _ = table.Guidance()
	if title != "Waiting for match stakes" {
		t.Fatalf("title = %q after map agreement", title)
	}
}
