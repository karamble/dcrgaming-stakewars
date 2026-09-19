package seating

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/bridgetest"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/connect"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/gamingpb"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/transport"
	"github.com/karamble/dcrgaming-sdk/pkg/identity"
	"github.com/karamble/dcrgaming-sdk/pkg/membership"
	rt "github.com/karamble/dcrgaming-sdk/pkg/runtime"
	"github.com/karamble/dcrgaming-sdk/pkg/spend"
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
)

type fixtureRules struct{ terms membership.Terms }

func (r fixtureRules) Identity() connect.Identity {
	return connect.Identity{GameID: "stakewars", GameVer: ProtocolVersion, ClientVersion: "seating-check", Capabilities: []gamingpb.Capability{gamingpb.Capability_CAP_ACCEPT_INVITE}}
}
func (r fixtureRules) Terms(string) (membership.Terms, error) { return r.terms, nil }
func (fixtureRules) Handle(context.Context, rt.Message) error { return nil }
func (fixtureRules) State(context.Context) rt.State {
	return rt.State{Summary: "SDK compatibility fixture"}
}
func (fixtureRules) WillCoSign(string, uint32) bool { return false }

// This exercises the real SDK against a local mTLS bridge with simulated funds.
func TestSDKPendingAdmissionRecovery(t *testing.T) {
	for _, lostID := range []bool{false, true} {
		name := "known-request"
		if lostID {
			name = "reconciled-request"
		}
		t.Run(name, func(t *testing.T) { checkSDKPendingAdmissionRecovery(t, lostID) })
	}
}

func checkSDKPendingAdmissionRecovery(t *testing.T, lostID bool) {
	terms, _, err := Derive(bridgeconn.Invitation{SID: "abcdef01", Seats: 2, BuyInAtoms: 10000000, CSVBlocks: 288, Until: 900, AdmissionAtoms: 1000000, AdmissionBlocks: 2016}, 800)
	if err != nil {
		t.Fatal(err)
	}
	rules := fixtureRules{terms}
	fake := bridgetest.New(bridgetest.Options{Game: "stakewars", Network: "testnet3", Params: chaincfg.TestNet3Params(), Height: 800})
	fake.SetVerdict(bridgetest.Hold, "")
	srv, err := fake.Serve("seat0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	seed, err := identity.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	// Params explicitly: the fake bridge has not said hello. TickEvery is off
	// because this test moves the height itself.
	config := rt.Config{Rules: rules, Identity: seed, Dir: directory, Params: chaincfg.TestNet3Params(), TickEvery: -1, SeatTags: identity.SeatTags{Session: "StakeWars/session/v1", Log: "StakeWars/log/v1", Bond: "StakeWars/bond/v1"}}
	start := func(resume bool) (*rt.Runtime, func()) {
		ctx, cancel := context.WithCancel(context.Background())
		conn, err := srv.Dial(ctx, "seat0", func(c *transport.BridgeConfig) { connect.Stamp(c, rules.Identity()) })
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		config.Bridge = conn
		runtime, err := rt.Open(config)
		if err != nil {
			cancel()
			conn.Close()
			t.Fatal(err)
		}
		if resume {
			if report := runtime.Resumed(); len(report.Restored) != 1 {
				t.Fatalf("resume: %+v", report)
			}
		}
		done := make(chan error, 1)
		go func() { done <- runtime.Run(ctx) }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("runtime did not stop")
				}
				conn.Close()
			})
		}
		t.Cleanup(stop)
		eventually(t, func() bool { state, _ := conn.ConnectionStatus(); return state == transport.Subscribed })
		return runtime, stop
	}
	runtime, stop := start(false)
	ctx := context.Background()
	link := "gaming://stakewars/table?sid=abcdef01&seats=2&buyin=10000000&csv=288&until=900&fv=2&bond=1000000&bondcsv=2016&tablebond=0&tablebondcsv=0"
	if _, err = runtime.AcceptInvite(ctx, link, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		records := runtime.Book().All()
		return len(fake.Spends()) == 1 && len(records) == 1 && records[0].ID != ""
	})
	records, err := runtime.Tables().LoadTables()
	if err != nil || len(records) != 1 || records[0].Terms != terms {
		t.Fatalf("pending admission missing: %+v %v", records, err)
	}
	if err = runtime.Fund(ctx, "abcdef01"); !errors.Is(err, rt.ErrNotSeated) {
		t.Fatalf("pending funding: %v", err)
	}
	stop()
	eventually(t, func() bool { return fake.Subscribers() == 0 })
	if lostID {
		// The runtime is stopped, so its book can be edited underneath it.
		store, err := spend.FileStore(directory + "/spends.json")
		if err != nil {
			t.Fatal(err)
		}
		records, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		records[0].ID = "" // Simulate losing the response before its ID was saved.
		if err = store.Save(records); err != nil {
			t.Fatal(err)
		}
	}
	restarted, _ := start(true)
	if restarted.Terms("abcdef01") != terms {
		t.Fatal("restored terms changed")
	}
	if lostID {
		for id := range fake.Spends() {
			if err = restarted.ReconcileSpend(ctx, "abcdef01", "seatbond", id); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = restarted.AcceptInvite(ctx, link, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	for id := range fake.Spends() {
		if !fake.Settle(id, bridgetest.Approve, "") {
			t.Fatal("approval failed")
		}
	}
	fake.Mine(2)
	eventually(t, func() bool {
		restarted.Tick(ctx, fake.Height())
		snap, err := restarted.Snapshot("abcdef01")
		return err == nil && len(snap.Record.Joins) == 1
	})
	if len(fake.Spends()) != 1 {
		t.Fatal("restart duplicated the admission payment")
	}
	snap, err := restarted.RefreshDeposits(ctx, "abcdef01")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Deposits) != 1 || snap.Deposits[0].Check != "verified" {
		t.Fatalf("bond verification: %+v", snap.Deposits)
	}
	if snap.Phase == "seated" {
		t.Fatal("one player incorrectly seated a two-player table")
	}
}
func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for SDK lifecycle")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
