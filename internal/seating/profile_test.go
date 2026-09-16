package seating

import (
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"testing"
)

func TestInvitationControlsStakeSeatsAndDeadline(t *testing.T) {
	for _, seats := range []uint32{2, 4, 6} {
		for _, buyin := range []uint64{100000, 10000000} {
			inv := bridgeconn.Invitation{SID: "abcdef01", Seats: seats,
				BuyInAtoms: buyin, CSVBlocks: 400, Until: 900,
				AdmissionAtoms: 1_000_000, AdmissionBlocks: BondLockBlocks}
			terms, c, err := Derive(inv, 800)
			if err != nil {
				t.Fatal(err)
			}
			if terms.BuyInAtoms != buyin || terms.Seats != seats || terms.Until != 900 || terms.CSVBlocks != 400 {
				t.Fatal("overrode invitation")
			}
			if c.Admission != 1000000 || c.Liveness != 0 || c.Honesty != 0 || c.Total != buyin+1000000 {
				t.Fatal("incorrect deposit policy")
			}
		}
	}
}
func TestInvalidPreparationTerms(t *testing.T) {
	inv := bridgeconn.Invitation{SID: "abcdef01", Seats: 2,
		BuyInAtoms: 10_000_000, CSVBlocks: 288, Until: 900,
		AdmissionAtoms: 1_000_000, AdmissionBlocks: BondLockBlocks}
	for _, change := range []func(*bridgeconn.Invitation){func(i *bridgeconn.Invitation) { i.Until = 800 }, func(i *bridgeconn.Invitation) { i.Until = ^uint32(0) }, func(i *bridgeconn.Invitation) { i.BuyInAtoms = ^uint64(0) }, func(i *bridgeconn.Invitation) { i.CSVBlocks = 1 }, func(i *bridgeconn.Invitation) { i.Seats = 7 }} {
		copy := inv
		change(&copy)
		if _, _, err := Derive(copy, 800); err == nil {
			t.Fatal("accepted invalid terms")
		}
	}
}
