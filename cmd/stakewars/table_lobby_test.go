//go:build desktop

package main

import (
	"github.com/karamble/dcrgaming-sdk/pkg/membership"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
	"github.com/karamble/dcrgaming-stakewars/internal/bridgeconn"
	"testing"
)

func TestLiveInvitationPreservesAcceptedAdmissionTerms(t *testing.T) {
	r := sdk.TableRecord{Match: "abc123", GCID: "group", Terms: membership.Terms{Seats: 2, BuyInAtoms: 100000, CSVBlocks: 288, Until: 1000, BondAtoms: 10000, BondLockBlocks: 2016}}
	inv := liveInvitation(r)
	if inv.AdmissionAtoms != r.Terms.BondAtoms || inv.AdmissionBlocks != r.Terms.BondLockBlocks {
		t.Fatalf("live view dropped accepted admission terms: %+v", inv)
	}
}

func TestBridgeAcceptanceOpensPreparationNotFundedSeats(t *testing.T) {
	g := &game{arena: true, settingsOpen: true}
	inv := &bridgeconn.Invitation{SID: "abc123", Seats: 4, Until: 1000}
	u := connectionUpdate{invite: inv, inviteSerial: 1, connected: true, chainKnown: true, height: 900}
	g.receiveTableUpdate(u)
	if !g.tableOpen || g.arena || g.settingsOpen || g.tableState == nil {
		t.Fatal("accepted request did not open table room")
	}
	if joined, funded := g.tableState.Counts(); joined != 0 || funded != 0 {
		t.Fatal("invitation fabricated seats")
	}
	g.tableOpen = false
	g.receiveTableUpdate(u)
	if g.tableOpen {
		t.Fatal("heartbeat reopened table room")
	}
	u.inviteSerial++
	g.receiveTableUpdate(u)
	if !g.tableOpen {
		t.Fatal("new request did not reopen room")
	}
	u.gap = true
	g.receiveTableUpdate(u)
	if !g.tableState.Stale {
		t.Fatal("gap did not invalidate evidence")
	}
	g.stopBridge()
	if g.tableState.Connected || g.tableState.ChainKnown {
		t.Fatal("disconnect retained live status")
	}
}
