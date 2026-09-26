package bridgeconn

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/karamble/dcrgaming-sdk/pkg/gaming/bridgetest"
)

func fixture(t *testing.T, game, network string) Config {
	t.Helper()
	server, err := bridgetest.New(bridgetest.Options{Game: game, Network: network, Fees: bridgetest.RelayFees()}).Serve("stakewars")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	cfg, err := server.Config("stakewars")
	if err != nil {
		t.Fatal(err)
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		t.Fatal(err)
	}
	return Config{Network: "mainnet", Host: host, Port: port, ClientCert: string(cfg.ClientCert), ClientKey: string(cfg.ClientKey), BridgeCert: string(cfg.BridgeCert)}
}
func TestAuthenticatedConnectionAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		game, network string
		ok            bool
	}{{GameID, Network, true}, {"poker", Network, false}, {GameID, "testnet3", false}} {
		t.Run(tc.game+tc.network, func(t *testing.T) {
			cfg := fixture(t, tc.game, tc.network)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			b, err := Connect(ctx, cfg)
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				b.Close()
			} else if err == nil {
				b.Close()
				t.Fatal("wrong identity/network accepted")
			}
		})
	}
}
func TestWrongPinnedCertificateFails(t *testing.T) {
	cfg := fixture(t, GameID, Network)
	other := fixture(t, GameID, Network)
	cfg.BridgeCert = other.BridgeCert
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if b, err := Connect(ctx, cfg); err == nil {
		b.Close()
		t.Fatal("wrong bridge certificate accepted")
	}
	cfg.ClientKey = other.ClientKey
	if err := cfg.Validate(); err == nil {
		t.Fatal("mismatched client key accepted")
	}
}
func TestSettingsPersistenceAndPermissions(t *testing.T) {
	cfg := fixture(t, GameID, Network)
	path := filepath.Join(t.TempDir(), "bridge.json")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded != cfg {
		t.Fatal("settings did not round-trip", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("private key permissions", info.Mode())
	}
	cfg.ClientKey = "SECRET-INVALID-KEY"
	if err := Save(path, cfg); err == nil || strings.Contains(err.Error(), cfg.ClientKey) {
		t.Fatal("invalid key saved or exposed")
	}
	loaded, err = Load(path)
	if err != nil || loaded.ClientKey == cfg.ClientKey {
		t.Fatal("failed save destroyed old settings")
	}
}
func TestAddressValidation(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "[::1]"} {
		c := Defaults()
		c.Host = host
		if _, err := c.Address(); err != nil {
			t.Fatal(err)
		}
	}
	for _, host := range []string{"http://127.0.0.1", "127.0.0.1/path", "localhost:8443", ""} {
		c := Defaults()
		c.Host = host
		if _, err := c.Address(); err == nil {
			t.Fatal("invalid IP accepted")
		}
	}
	for _, port := range []string{"0", "65536", "", "abc"} {
		c := Defaults()
		c.Port = port
		if _, err := c.Address(); err == nil {
			t.Fatal("invalid port accepted")
		}
	}
}

func TestExplicitNonMainnetConnections(t *testing.T) {
	for _, network := range []string{"testnet3", "simnet"} {
		t.Run(network, func(t *testing.T) {
			cfg := fixture(t, GameID, network)
			cfg.Network = network
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			bridge, err := Connect(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer bridge.Close()
			if bridge.Network() != network {
				t.Fatal("network not retained")
			}
		})
	}
	cfg := fixture(t, GameID, Network)
	cfg.Network = "unrecognized"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unknown network accepted")
	}
}
