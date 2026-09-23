// Command stakewars-localbridge runs two desktop peers against an isolated
// simulated bridge. It cannot connect to a wallet or a Decred node.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/bridgetest"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/gamingpb"
	"github.com/karamble/dcrgaming-stakewars/internal/bridgeconn"
)

func run() error {
	binary := flag.String("desktop", "/tmp/stakewars-bin/stakewars-dev", "desktop dev binary")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	root, err := os.MkdirTemp("", "stakewars-demo-")
	if err != nil {
		return err
	}
	fmt.Printf("SIMULATED FUNDS ONLY. Demo state: %s\n", root)
	fake := bridgetest.New(bridgetest.Options{Game: "stakewars", Network: "mainnet", Params: chaincfg.MainNetParams(), Height: 800})
	server, err := fake.Serve("one", "two")
	if err != nil {
		return err
	}
	defer server.Close()
	exited := make(chan error, 2)
	for _, seat := range []string{"one", "two"} {
		cfg, err := server.Config(seat)
		if err != nil {
			return err
		}
		host, port, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			return err
		}
		dir := filepath.Join(root, seat)
		path := filepath.Join(dir, "bridge.json")
		err = bridgeconn.Save(path, bridgeconn.Config{Network: "mainnet", Host: host, Port: port, ClientCert: string(cfg.ClientCert), ClientKey: string(cfg.ClientKey), BridgeCert: string(cfg.BridgeCert)})
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, *binary, "-skip-cover", "-datadir", dir, "-connect", "-dev-network", "-mute")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Start(); err != nil {
			return err
		}
		go func() { exited <- cmd.Wait() }()
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	invited := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-exited:
			return err
		case <-ticker.C:
			if !invited {
				if fake.Subscribers() != 2 {
					continue
				}
				fake.Ask(&gamingpb.BridgeRequest{RequestId: "demo-invite", Req: &gamingpb.BridgeRequest_AcceptInvite{AcceptInvite: &gamingpb.AcceptInvite{Gcid: strings.Repeat("a", 64), Invite: "gaming://stakewars/table?sid=abcdef01&seats=2&buyin=10000000&csv=288&until=810&fv=2&bond=1000000&bondcsv=2016&tablebond=0&tablebondcsv=0"}}})
				invited = true
				fmt.Println("Invitation accepted in both windows. Wait for the map check, press F in each window to fund, then Enter to play. Space fires; J jumps; W/S aim; A/D walk.")
			}
			if fake.Height() < 816 || (len(fake.Spends()) == 4 && fake.Height() < 818) {
				fake.Mine(1)
			}
		}
	}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
