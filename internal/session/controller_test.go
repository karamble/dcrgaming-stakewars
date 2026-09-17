package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/bridgetest"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/connect"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/gamingpb"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/transport"
	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

func waitSession(t *testing.T, label string, cs []*Controller, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(40 * time.Second)
	for !f() {
		if time.Now().After(deadline) {
			for _, c := range cs {
				v := c.Snapshot()
				t.Logf("%s phase=%s error=%s tables=%+v", v.Status, v.Phase, v.Error, v.Tables)
			}
			t.Fatal("timeout: " + label)
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func TestStateLockErrorNamesTheProfileOwnerProblem(t *testing.T) {
	err := friendlyOpenError(fmt.Errorf("state already in use (/secret/profile/spends.json.lock): resource temporarily unavailable"))
	if got := err.Error(); got != "This player profile is already open in another StakeWars window. Quit that window before reconnecting." {
		t.Fatalf("unexpected lock guidance: %q", got)
	}
	if strings.Contains(err.Error(), "/secret/") || strings.Contains(err.Error(), ".lock") {
		t.Fatal("raw lock path leaked into the player-facing error")
	}
}

func TestWaitForProfileContinuesWhenPreviousOwnerStops(t *testing.T) {
	dir := t.TempDir()
	release, busy, err := acquireProfileProbe(dir)
	if err != nil || busy {
		t.Fatalf("acquire profile: busy=%v err=%v", busy, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct {
		waited bool
		err    error
	}, 1)
	go func() {
		waited, err := WaitForProfile(ctx, dir)
		done <- struct {
			waited bool
			err    error
		}{waited, err}
	}()
	select {
	case got := <-done:
		t.Fatalf("wait returned while profile was owned: %+v", got)
	case <-time.After(400 * time.Millisecond):
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.err != nil || !got.waited {
			t.Fatalf("wait result: %+v", got)
		}
	case <-ctx.Done():
		t.Fatal("wait did not continue after release")
	}
}

func TestOnlyOneDesktopInstanceUsesAProfile(t *testing.T) {
	dir := t.TempDir()
	release, err := AcquireProfileInstance(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = AcquireProfileInstance(dir); !errors.Is(err, ErrProfileInUse) {
		t.Fatalf("second instance error = %v", err)
	}
	if err = release(); err != nil {
		t.Fatal(err)
	}
	next, err := AcquireProfileInstance(dir)
	if err != nil {
		t.Fatalf("profile remained owned after exit: %v", err)
	}
	if err = next(); err != nil {
		t.Fatal(err)
	}
}

// Full production controllers over local mTLS: admission, deterministic world,
// wallet-approved stake, signed/replayed turns, and all-seat settlement. No funds.
func TestCooperativeMatch(t *testing.T) {
	for _, n := range []int{2, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { cooperativeMatch(t, n, false) })
	}
}
func TestRecoveryRemainsBridgeOwnedAfterRestart(t *testing.T) { cooperativeMatch(t, 2, true) }
func cooperativeMatch(t *testing.T, players int, refunds bool) {
	names := make([]string, players)
	for i := range names {
		names[i] = fmt.Sprint("seat", i)
	}
	fake := bridgetest.New(bridgetest.Options{Game: "stakewars", Network: "simnet", Params: chaincfg.SimNetParams(), Height: 800})
	server, err := fake.Serve(names...)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := make([]*Controller, players)
	dirs := make([]string, players)
	stops := make([]func(), players)
	start := func(i int) {
		nodeCtx, nodeCancel := context.WithCancel(ctx)
		b, e := server.Dial(nodeCtx, names[i], func(cfg *transport.BridgeConfig) { connect.Stamp(cfg, Identity()) })
		if e != nil {
			t.Fatal(e)
		}
		if _, e := b.Hello(nodeCtx, "simnet"); e != nil {
			t.Fatal(e)
		}
		c, e := New(b, dirs[i])
		if e != nil {
			t.Fatal(e)
		}
		cs[i] = c
		done := make(chan error, 1)
		go func() { done <- c.Run(nodeCtx) }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				nodeCancel()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Error("controller shutdown timeout")
				}
			})
		}
		stops[i] = stop
		t.Cleanup(stop)
	}
	for i := range names {
		dirs[i] = t.TempDir()
		start(i)
	}
	defer func() {
		for _, stop := range stops {
			stop()
		}
	}()
	waitSession(t, "subscribers", cs, func() bool { return fake.Subscribers() == players })
	link := fmt.Sprintf("gaming://stakewars/table?sid=abcdef01&seats=%d&buyin=10000000&csv=288&until=900&fv=2&bond=1000000&bondcsv=2016&tablebond=0&tablebondcsv=0", players)
	// Exercise the same request emitted by dcrpulse's invitation chat chip.
	fake.Ask(&gamingpb.BridgeRequest{RequestId: "invite", Req: &gamingpb.BridgeRequest_AcceptInvite{AcceptInvite: &gamingpb.AcceptInvite{Invite: link, Gcid: strings.Repeat("a", 64)}}})
	waitSession(t, "admission spends", cs, func() bool { return len(fake.Spends()) == players })
	fake.Mine(2)
	waitSession(t, "roster", cs, func() bool {
		for _, c := range cs {
			s, e := c.runtime.Snapshot("abcdef01")
			if e != nil || len(s.Record.Commits) != players {
				return false
			}
		}
		return true
	})
	fake.SetHeight(906)
	waitSession(t, "world agreement", cs, func() bool {
		for _, c := range cs {
			if !c.Snapshot().CanFund {
				return false
			}
		}
		return true
	})
	for _, c := range cs {
		if e := c.Action("fund"); e != nil {
			t.Fatal(e)
		}
	}
	waitSession(t, "stake spends", cs, func() bool { return len(fake.Spends()) == players*2 })
	fake.Mine(2)
	waitSession(t, "ready", cs, func() bool {
		for _, c := range cs {
			if c.Snapshot().Head == nil {
				return false
			}
		}
		return true
	})
	if replay.Hash(cs[0].Snapshot().Head) != replay.Hash(cs[1].Snapshot().Head) {
		t.Fatal("genesis differs")
	}
	if players == 2 && !refunds {
		before := len(fake.Sent())
		time.Sleep(6 * time.Second)
		if after := len(fake.Sent()); after != before {
			t.Fatalf("idle table retransmitted %d BR frame(s)", after-before)
		}
	}
	if refunds {
		// Restart each game alone. Recovery remains an operator action in the bridge.
		for _, stop := range stops {
			stop()
		}
		waitSession(t, "all peers offline", cs, func() bool { return fake.Subscribers() == 0 })
		fake.Mine(2017)
		for i := range cs {
			start(i)
			waitSession(t, "sole recovery peer", cs, func() bool { return fake.Subscribers() == 1 })
			waitSession(t, "restored deposits", cs, func() bool { return cs[i].Snapshot().Match != "" && cs[i].runtime.PayoutFor("abcdef01") != "" })
			_ = cs[i].Action("refund")
			_ = cs[i].Action("bond")
			if len(fake.Broadcasts()) != 0 {
				t.Fatal("game action published a refund")
			}
			waitSession(t, "bridge recovery guidance", cs, func() bool { return strings.Contains(cs[i].Snapshot().Error, "Recovery") })
			stops[i]()
			waitSession(t, "recovery peer disconnected", cs, func() bool { return fake.Subscribers() == 0 })
		}
		if len(fake.Spends()) != players*2 {
			t.Fatal("recovery requested a new deposit")
		}
		return
	}

	// Ordinary turn countdowns and sudden death eventually finish an idle match.
	for n := 0; n < 300; n++ {
		head := cs[0].Snapshot().Head
		if head.Phase == sim.Ended {
			break
		}
		active := cs[0]
		for _, c := range cs {
			if c.Snapshot().Mine == head.ActiveSeat() {
				active = c
				break
			}
		}
		after := head.Clone()
		var inputs []sim.Input
		acted := false
		for tick := 0; tick < 20000 && after.Turn == head.Turn && after.Phase != sim.Ended; tick++ {
			var in []sim.Input
			if after.Phase == sim.Playing && !acted {
				acted = true
				kind, param := sim.Fire, int32(650)
				if n >= players {
					kind, param = sim.Surrender, 0
				}
				in = []sim.Input{{Tick: after.Tick, Seat: after.ActiveSeat(), Kind: kind, Param: param}}
				inputs = append(inputs, in...)
			}
			if _, e := sim.Step(after, in); e != nil {
				t.Fatal(e)
			}
		}
		if after.Turn == head.Turn && after.Phase != sim.Ended {
			t.Fatal("turn did not end")
		}
		if e := active.Submit("abcdef01", after, inputs); e != nil {
			t.Fatal(e)
		}
		want := replay.Hash(after)
		waitSession(t, fmt.Sprintf("turn %d", n), cs, func() bool {
			for _, c := range cs {
				v := c.Snapshot()
				if v.Head == nil || replay.Hash(v.Head) != want {
					return false
				}
			}
			return true
		})
		if n == 0 && players == 2 {
			stops[0]()
			waitSession(t, "disconnected instance", cs, func() bool { return fake.Subscribers() == players-1 })
			start(0)
			waitSession(t, "reconnected instance", cs, func() bool { return fake.Subscribers() == players })
			waitSession(t, "durable replay after restart", cs, func() bool { h := cs[0].Snapshot().Head; return h != nil && replay.Hash(h) == want })
			if len(fake.Spends()) != players*2 {
				t.Fatal("restart requested another deposit")
			}
		}

	}
	if cs[0].Snapshot().Head.Phase != sim.Ended {
		t.Fatal("match did not end")
	}
	if cs[0].Snapshot().Head.Winner < 0 {
		t.Fatal("expected winner, including zero-share losing signers")
	}
	if len(fake.Broadcasts()) != 0 {
		t.Fatal("payout occurred before dashboard approval")
	}
	if err := fake.SetPayoutVerdict(bridgetest.Approve); err != nil {
		t.Fatal(err)
	}
	waitSession(t, "payout", cs, func() bool { return len(fake.Broadcasts()) == 1 })
	if len(fake.Spends()) != players*2 {
		t.Fatal("gameplay requested an on-chain spend")
	}
}
