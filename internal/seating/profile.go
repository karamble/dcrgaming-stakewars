// Package seating defines StakeWars' invitation-derived preparation policy.
// It does not authorize money movement or replace SDK escrow construction.
package seating

import (
	"fmt"
	"github.com/karamble/dcrgaming-sdk/pkg/membership"
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"github.com/karamble/dcrstakewars/internal/payout"
)

const ProtocolVersion = 5
const MinRefundBlocks uint32 = 288
const BondLockBlocks uint32 = 2016
const AccuseFeeAtoms uint64 = 10000

type Costs struct {
	Stake, Admission, Liveness, Honesty, Total uint64
}

// Derive preserves all bridge-approved financial terms. StakeWars does not
// choose hidden bond amounts or refund delays. Fees are shown by the bridge.
func Derive(inv bridgeconn.Invitation, height uint32) (membership.Terms, Costs, error) {
	if inv.SID == "" || inv.Seats < 2 || inv.Seats > 6 || inv.BuyInAtoms == 0 || inv.CSVBlocks < MinRefundBlocks || inv.Until <= height || inv.Until > ^uint32(0)-64 {
		return membership.Terms{}, Costs{}, fmt.Errorf("invalid or expired seating terms")
	}
	if inv.AdmissionAtoms == 0 || inv.AdmissionAtoms > uint64(payout.MaxAtoms) || inv.AdmissionBlocks == 0 || inv.AdmissionBlocks > 65535 || inv.TableBondAtoms != 0 || inv.TableBondBlocks != 0 {
		return membership.Terms{}, Costs{}, fmt.Errorf("StakeWars requires explicit refundable admission terms and no additional table bond")
	}
	if inv.BuyInAtoms < 100000 {
		return membership.Terms{}, Costs{}, fmt.Errorf("stake must be at least 0.001 DCR to cover settlement and refund fees")
	}
	c := Costs{Stake: inv.BuyInAtoms, Admission: inv.AdmissionAtoms}
	// Bound aggregate pot and every participant's complete obligation before sums.
	if c.Stake > uint64(payout.MaxAtoms)/uint64(inv.Seats) || c.Stake > (uint64(payout.MaxAtoms)-c.Admission-c.Liveness)/2 {
		return membership.Terms{}, Costs{}, fmt.Errorf("deposit amounts exceed monetary bounds")
	}
	c.Total = c.Stake + c.Admission + c.Liveness + c.Honesty
	terms := membership.Terms{Game: bridgeconn.GameID, GameVer: ProtocolVersion, SID: inv.SID, BuyInAtoms: inv.BuyInAtoms, Seats: inv.Seats, CSVBlocks: inv.CSVBlocks, Until: inv.Until, BondAtoms: c.Admission, BondLockBlocks: inv.AdmissionBlocks}
	// GameVer freezes the built-in preset; the SDK binds it in the signed terms.
	// Custom rules must receive an explicit terms commitment before support.
	if _, err := Preset(inv.Seats, inv.BuyInAtoms); err != nil {
		return membership.Terms{}, Costs{}, err
	}
	if err := terms.Validate(); err != nil {
		return membership.Terms{}, Costs{}, err
	}
	return terms, c, nil
}
