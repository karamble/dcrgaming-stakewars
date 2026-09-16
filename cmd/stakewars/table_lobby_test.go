//go:build desktop

package main

import (
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"testing"
)

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
