package bridgeconn

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/karamble/dcrgaming-sdk/pkg/gaming/bridgetest"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/gamingpb"
)

const validInvite = "gaming://stakewars/table?sid=abc123&seats=4&buyin=10000000&csv=288&until=1000000&fv=2&bond=1000000&bondcsv=2016&tablebond=0&tablebondcsv=0"

func TestPreviewRejectsAmbiguousOrIncompleteTerms(t *testing.T) {
	gcid := strings.Repeat("a", 64)
	inv, err := PreviewInvite(validInvite, gcid)
	if err != nil || inv.Seats != 4 || inv.BuyInAtoms != 10000000 {
		t.Fatal(inv, err)
	}
	for _, link := range []string{
		strings.Replace(validInvite, "stakewars", "poker", 1),
		strings.Replace(validInvite, "seats=4", "seats=7", 1),
		strings.Replace(validInvite, "buyin=10000000", "buyin=0", 1),
		strings.Replace(validInvite, "buyin=10000000", "buyin=2100000000000000", 1),
		strings.Replace(validInvite, "csv=288", "csv=0", 1),
		strings.Replace(validInvite, "sid=abc123", "sid=", 1),
		validInvite + "&buyin=2", validInvite + "&unknown=1", validInvite + "&bad=%zz", validInvite + "#fragment",
	} {
		if _, err := PreviewInvite(link, gcid); err == nil {
			t.Fatal("accepted invalid invitation", link)
		}
	}
	if _, err := PreviewInvite(validInvite, "wrong-chat"); err == nil {
		t.Fatal("invalid chat accepted")
	}
}

func TestLobbyReceivesRequestsWithoutFundingAndRecovers(t *testing.T) {
	fake := bridgetest.New(bridgetest.Options{Game: GameID, Network: Network, Fees: bridgetest.RelayFees()})
	server, err := fake.Serve("stakewars")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	credentials, err := server.Config("stakewars")
	if err != nil {
		t.Fatal(err)
	}
	host, port, err := net.SplitHostPort(credentials.Addr)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Network: "mainnet", Host: host, Port: port, ClientCert: string(credentials.ClientCert), ClientKey: string(credentials.ClientKey), BridgeCert: string(credentials.BridgeCert)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan Update, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runLobby(ctx, cfg, func(u Update) {
			select {
			case updates <- u:
			case <-ctx.Done():
			}
		}, 20*time.Millisecond, 10*time.Millisecond)
	}()
	waitUpdate := func(match func(Update) bool) Update {
		t.Helper()
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for {
			select {
			case u := <-updates:
				if match(u) {
					return u
				}
			case <-timer.C:
				t.Fatal("missing connection update")
				return Update{}
			}
		}
	}
	waitUpdate(func(u Update) bool { return u.Connected })
	deadline := time.Now().Add(3 * time.Second)
	for fake.Subscribers() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no subscription")
		}
		time.Sleep(time.Millisecond)
	}
	requests := []*gamingpb.BridgeRequest{
		{RequestId: "preview", Req: &gamingpb.BridgeRequest_AcceptInvite{AcceptInvite: &gamingpb.AcceptInvite{Invite: validInvite, Gcid: strings.Repeat("a", 64)}}},
		{RequestId: "refresh", Req: &gamingpb.BridgeRequest_RefreshState{RefreshState: &gamingpb.RefreshState{}}},
		{RequestId: "unsupported"},
		{RequestId: "expired", DeadlineUnix: time.Now().Add(-time.Minute).Unix(), Req: &gamingpb.BridgeRequest_AcceptInvite{AcceptInvite: &gamingpb.AcceptInvite{Invite: validInvite, Gcid: strings.Repeat("b", 64)}}},
	}
	for _, req := range requests {
		if fake.Ask(req) != 1 {
			t.Fatal("request not delivered")
		}
	}
	u := waitUpdate(func(u Update) bool { return u.Invite != nil })
	if u.InviteSerial != 1 {
		t.Fatal("accepted request did not have a unique UI event")
	}
	if u.Invite.GCID != strings.Repeat("a", 64) {
		t.Fatal("wrong preview")
	}
	deadline = time.Now().Add(3 * time.Second)
	for len(fake.Replies()) < len(requests) {
		if time.Now().After(deadline) {
			t.Fatal("requests unanswered")
		}
		time.Sleep(time.Millisecond)
	}
	replies := fake.Replies()
	for _, reply := range replies {
		if reply.RequestId == "refresh" {
			if !reply.Ok || reply.GetState() == nil || len(reply.GetState().Tables) != 0 {
				t.Fatal("false table state")
			}
		} else if reply.Ok || reply.Error == "" {
			t.Fatal("unsupported request accepted")
		}
	}
	fake.SetUnreachable(true)
	waitUpdate(func(u Update) bool { return !u.Connected && !u.Busy })
	fake.SetUnreachable(false)
	waitUpdate(func(u Update) bool { return u.Connected })
	if len(fake.Spends()) != 0 || len(fake.Sent()) != 0 || len(fake.Broadcasts()) != 0 {
		t.Fatal("lobby caused financial or peer side effects")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not cancel worker")
	}
}
