// Package tablelobby derives read-only preparation views. It never reserves a
// seat, accepts a peer claim as verification, or initiates a payment.
package tablelobby

import (
	"fmt"
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
)

type PaymentPhase uint8

const (
	Unknown PaymentPhase = iota
	Approval
	Announced
	Confirming
	Verified
	Rejected
)

type Payment struct {
	Phase                   PaymentPhase
	Confirmations, Required uint32
	// Checked means the local verifier checked the expected script, value and
	// unspent output. A peer announcement must never set it.
	Checked bool
}

func (p Payment) Complete() bool {
	return p.Phase == Verified && p.Checked && p.Required > 0 && p.Confirmations >= p.Required
}
func (p Payment) Label() string {
	if p.Complete() {
		return fmt.Sprintf("Verified · %d/%d confirmations", p.Confirmations, p.Required)
	}
	switch p.Phase {
	case Approval:
		return "Approve in dcrpulse"
	case Announced:
		return "Broadcast · awaiting chain check"
	case Confirming, Verified:
		if !p.Checked {
			return "Awaiting local output verification"
		}
		if p.Required == 0 {
			return "Confirmation policy not set"
		}
		return fmt.Sprintf("Confirming · %d/%d blocks", p.Confirmations, p.Required)
	case Rejected:
		return "Output rejected · review required"
	default:
		return "Not requested"
	}
}

// BondCardLabel is the compact, per-player admission status shown while the
// roster is assembling. It describes only locally checked chain evidence.
func (p Payment) BondCardLabel() string {
	if p.Complete() {
		return fmt.Sprintf("BOND VERIFIED · %d/%d CONFIRMATIONS", p.Confirmations, p.Required)
	}
	switch p.Phase {
	case Approval:
		return "BOND APPROVAL REQUIRED"
	case Announced:
		return "BOND POSTED · VERIFYING OUTPUT"
	case Confirming, Verified:
		if p.Checked && p.Required > 0 {
			return fmt.Sprintf("BOND POSTED · %d/%d CONFIRMATIONS", p.Confirmations, p.Required)
		}
		return "BOND POSTED · VERIFYING OUTPUT"
	case Rejected:
		return "BOND OUTPUT REJECTED"
	default:
		return ""
	}
}

type Seat struct {
	Name                                      string
	Ours, Joined, IdentityVerified, Committed bool
	Admission, Stake, Bond                    Payment
	BondOptional                              bool
}

func (s Seat) Status() string {
	if !s.Joined {
		return "NO VERIFIED PLAYER"
	}
	if !s.IdentityVerified {
		return "CHECKING IDENTITY"
	}
	if !s.Admission.Complete() {
		return "ADMISSION BOND"
	}
	if !s.Committed {
		return "AGREEING ROSTER"
	}
	if !s.Stake.Complete() {
		return "WAITING FOR STAKE"
	}
	if !s.BondOptional && !s.Bond.Complete() {
		return "WAITING FOR TABLE BOND"
	}
	return "FUNDS VERIFIED"
}

type Table struct {
	PayoutsKnown               bool
	Invite                     bridgeconn.Invitation
	Reviewed                   bool
	Seats                      []Seat // canonical roster positions; not a claim of drawn turn order
	RosterAgreed, DrawVerified bool
	WorldVerified              bool
	Connected, ChainKnown      bool
	Height                     uint32
	Stale                      bool
	Demo                       bool
	Live                       bool
	Status, Error              string
	RefundStatus               string
	CanFund, Ready             bool
}

func New(inv bridgeconn.Invitation) Table {
	if inv.Seats < 2 || inv.Seats > 6 {
		return Table{Invite: inv}
	}
	return Table{Invite: inv, Seats: make([]Seat, inv.Seats)}
}
func (t Table) Expired() bool { return t.ChainKnown && t.Height >= t.Invite.Until }
func (t Table) Counts() (joined, funded int) {
	for _, s := range t.Seats {
		if s.Joined && s.IdentityVerified {
			joined++
		}
		if s.IdentityVerified && s.Joined && s.Admission.Complete() && s.Committed && s.Stake.Complete() && (s.BondOptional || s.Bond.Complete()) {
			funded++
		}
	}
	return
}

// Stage is the first unmet condition. Unknown, stale or invalid evidence never
// unlocks progress. Even the final stage does not authorize a match or payment.
func (t Table) Stage() int {
	if !t.Reviewed {
		return 0
	}
	if t.Stale || !t.Connected || !t.ChainKnown {
		return 1
	}
	if len(t.Seats) != int(t.Invite.Seats) || len(t.Seats) < 2 || len(t.Seats) > 6 {
		return 1
	}
	all := func(fn func(Seat) bool) bool {
		for _, s := range t.Seats {
			if !fn(s) {
				return false
			}
		}
		return true
	}
	if !all(func(s Seat) bool { return s.Joined && s.IdentityVerified && s.Admission.Complete() }) {
		return 1
	}
	if !t.RosterAgreed || !all(func(s Seat) bool { return s.Committed }) {
		return 2
	}
	if !t.DrawVerified {
		return 3
	}
	if t.Live {
		if !t.WorldVerified {
			return 4
		}
		if !all(func(s Seat) bool { return s.Stake.Complete() }) {
			return 5
		}
		if !t.PayoutsKnown {
			return 6
		}
		return 7
	}
	if !all(func(s Seat) bool { return s.Stake.Complete() }) {
		return 4
	}
	if !all(func(s Seat) bool { return s.BondOptional || s.Bond.Complete() }) {
		return 5
	}
	if !t.WorldVerified {
		return 6
	}
	return 7
}
func (t Table) Guidance() (string, string) {
	if t.Live {
		if t.Error != "" {
			return t.Status, t.Error
		}
		return t.Status, "All players sign the payout. If cooperation fails, recover your own deposits after their locks."
	}
	if t.Stale {
		return "Refresh the table evidence", "Delivery was interrupted. Earlier checks cannot establish current readiness."
	}
	if !t.Connected {
		return "Connect your bridge", "Open connection settings. Reviewing an invitation never reserves a seat."
	}
	if !t.ChainKnown {
		return "Waiting for chain data", "The bridge is reachable, but the current block height is unavailable."
	}
	if t.Expired() && t.Stage() < 3 {
		return "Admission deadline reached", "Ask the host for a new invitation. No payment has been requested here."
	}
	if !t.Reviewed {
		return "Review the invitation", "Check the buy-in, refund delay and admission deadline before continuing."
	}
	if !t.Demo {
		return "SDK admission recovery blocked", "Pending bond recovery must be fixed in the SDK before joining. No funds requested."
	}
	switch t.Stage() {
	case 1:
		return "Waiting for admission bonds", "A broadcast is not a confirmed bond. Do not send a second payment while waiting."
	case 2:
		return "Waiting for roster agreement", "Every participant must commit to the same membership. Nothing to approve here."
	case 3:
		return "Waiting for the seat draw", "Turn order is assigned only after the agreed block can be independently checked."
	case 4:
		return "Waiting for stakes to confirm", "Approval happens in dcrpulse. A pending payment should not be submitted again."
	case 5:
		return "Waiting for table bonds", "Stake and table bond are separate outputs. Both must pass local checks."
	case 6:
		return "Waiting for battlefield agreement", "Every peer must generate the same map and Stakey positions and sign the initial-state hash."
	default:
		return "Preparation verified in this demo", "This is a UI example. It cannot start a paid match or move funds."
	}
}
func DCR(atoms uint64) string { return fmt.Sprintf("%d.%08d", atoms/100000000, atoms%100000000) }

// Demo constructs explicit, disconnected UI fixtures, never bridge observations.
func Demo(seats, scenario int) Table {
	if seats != 2 && seats != 4 && seats != 6 {
		seats = 4
	}
	inv := bridgeconn.Invitation{SID: "a7c39e12", GCID: "0000000000000000000000000000000000000000000000000000000000000001", Seats: uint32(seats), BuyInAtoms: 10000000, CSVBlocks: 288, Until: 1100120}
	t := New(inv)
	t.Demo = true
	t.Reviewed = true
	t.Connected = true
	t.ChainKnown = true
	t.Height = 1100104
	verified := Payment{Phase: Verified, Checked: true, Confirmations: 2, Required: 2}
	names := []string{"You", "Mint Maverick", "Blue Comet", "Crater Maker", "Ticket Titan", "Last Stakey"}
	for i := range t.Seats {
		t.Seats[i] = Seat{Name: names[i], Ours: i == 0, Joined: true, IdentityVerified: true, Committed: true, Admission: verified, Stake: verified, Bond: verified}
	}
	t.RosterAgreed, t.DrawVerified = true, true
	switch scenario {
	case 0:
		t.RosterAgreed, t.DrawVerified = false, false
		t.Seats[seats-1] = Seat{}
		t.Seats[0].Admission = Payment{Phase: Confirming, Checked: true, Confirmations: 1, Required: 2}
		for i := range t.Seats {
			t.Seats[i].Committed = false
			t.Seats[i].Stake = Payment{}
			t.Seats[i].Bond = Payment{}
		}
	case 1:
		t.DrawVerified = false
		for i := range t.Seats {
			t.Seats[i].Stake = Payment{}
			t.Seats[i].Bond = Payment{}
		}
	case 2:
		t.Seats[0].Stake = Payment{Phase: Confirming, Checked: true, Confirmations: 1, Required: 2}
		t.Seats[1].Stake = Payment{Phase: Approval}
		for i := range t.Seats {
			t.Seats[i].Bond = Payment{}
		}
	case 3:
		t.Seats[seats-1].Bond = Payment{Phase: Confirming, Checked: true, Confirmations: 1, Required: 2}
	case 5:
		t.WorldVerified = true
	case 6:
		t.Stale = true
		t.Connected = false
		t.ChainKnown = false
	}
	return t
}
