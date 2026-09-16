package bridgeconn

import (
	"context"
	"errors"
	"github.com/decred/slog"
	"strings"

	sdkconnect "github.com/karamble/dcrgaming-sdk/pkg/gaming/connect"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/transport"
)

var log = slog.Disabled

// UseLogger configures the subsystem before any bridge connections start.
func UseLogger(l slog.Logger) { log = l }

const Network = "mainnet"
const GameID = "stakewars"

// Connect proves the credentials against the current SDK mTLS Hello contract.
// No invitations, table capabilities, payments or peer messages are sent.
func Connect(ctx context.Context, c Config) (*transport.Bridge, error) {
	return connect(ctx, c, nil)
}

func connect(ctx context.Context, c Config, onGap func([]string)) (*transport.Bridge, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	addr, _ := c.Address()
	b, err := transport.Dial(ctx, transport.BridgeConfig{Log: log, Addr: addr, ClientCert: []byte(c.ClientCert), ClientKey: []byte(c.ClientKey), BridgeCert: []byte(c.BridgeCert), GameID: GameID, GameVer: 1, ClientVersion: "stakewars-dev", OnGap: onGap})
	if err != nil {
		return nil, errors.New("Could not initialize the secure bridge connection.")
	}
	if err = Check(ctx, b, c.Network); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}
func Check(ctx context.Context, b *transport.Bridge, network string) error {
	reply, err := b.Hello(ctx, network)
	if err != nil {
		if strings.Contains(err.Error(), "this game is set up for") {
			return errors.New("Network mismatch: the bridge must match the network selected in settings.")
		}
		return errors.New("Bridge unavailable or authentication refused. Check address, port and certificates.")
	}
	if reply.GetGame() != GameID {
		return errors.New("The bridge credential is not registered for StakeWars.")
	}
	if reply.GetNetwork() != network {
		return errors.New("Network mismatch: the bridge must match the network selected in settings.")
	}
	return nil
}

// ConnectGame connects a runtime-owned subscription with explicit capabilities.
func ConnectGame(ctx context.Context, c Config, id sdkconnect.Identity) (*transport.Bridge, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	addr, _ := c.Address()
	cfg := transport.BridgeConfig{Log: log, Addr: addr, ClientCert: []byte(c.ClientCert), ClientKey: []byte(c.ClientKey), BridgeCert: []byte(c.BridgeCert)}
	sdkconnect.Stamp(&cfg, id)
	b, err := transport.Dial(ctx, cfg)
	if err != nil {
		return nil, errors.New("Secure bridge connection failed")
	}
	if err = Check(ctx, b, c.Network); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}
