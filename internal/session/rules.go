// Package session owns StakeWars' SDK lifecycle and independently verified
// preparation. It does not treat seating or a world signature as a paid result.
package session

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/connect"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/gamingpb"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/schema"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/transport"
	"github.com/karamble/dcrgaming-sdk/pkg/membership"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
	"github.com/karamble/dcrgaming-stakewars/internal/bridgeconn"
	"github.com/karamble/dcrgaming-stakewars/internal/durable"
	"github.com/karamble/dcrgaming-stakewars/internal/seating"
)

type worldMessage struct {
	World     seating.World
	Signer    string
	Signature string
}
type rules struct {
	mu        sync.Mutex
	bridge    *transport.Bridge
	runtime   *sdk.Runtime
	records   map[string]sdk.TableRecord
	worlds    map[string]worldMessage
	approvals map[string]map[string]worldMessage
	problems  map[string]string
	state     sdk.State
	accepted  chan string
	inbox     chan sdk.Message
	allowed   map[string]bool
	settled   map[string]string
	receipts  *durable.Store
}

func Identity() connect.Identity {
	return connect.Identity{GameID: bridgeconn.GameID, GameVer: seating.ProtocolVersion, ClientVersion: "stakewars-playable-v3", Capabilities: []gamingpb.Capability{gamingpb.Capability_CAP_ACCEPT_INVITE, gamingpb.Capability_CAP_SET_NAMES}, MinRefundBlocks: seating.MinRefundBlocks, BondLockBlocks: seating.BondLockBlocks}
}
func (r *rules) Identity() connect.Identity { return Identity() }
func (r *rules) Terms(sid string) (membership.Terms, error) {
	return membership.Terms{Game: bridgeconn.GameID, GameVer: seating.ProtocolVersion, SID: sid}, nil
}
func (r *rules) ResolveInvite(ctx context.Context, inv schema.Invite, _ membership.Terms) (membership.Terms, error) {
	tip, err := r.bridge.ChainTip(ctx)
	if err != nil {
		return membership.Terms{}, err
	}
	if tip.Height <= 0 || tip.Height > int64(^uint32(0)) {
		return membership.Terms{}, fmt.Errorf("chain height unavailable")
	}
	terms, _, err := seating.Derive(bridgeconn.Invitation{SID: inv.SID, Seats: inv.Seats, BuyInAtoms: inv.BuyInAtoms, CSVBlocks: inv.CSVBlocks, Until: inv.Until, AdmissionAtoms: inv.AdmissionAtoms, AdmissionBlocks: inv.AdmissionBlocks, TableBondAtoms: inv.TableBondAtoms, TableBondBlocks: inv.TableBondBlocks}, uint32(tip.Height))
	if err == nil {
		select {
		case r.accepted <- inv.SID:
		default:
		}
	}
	return terms, err
}
func (r *rules) State(context.Context) sdk.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := sdk.State{Summary: r.state.Summary}
	for _, t := range r.state.Tables {
		copy := t
		copy.Detail = map[string]string{}
		for k, v := range t.Detail {
			copy.Detail[k] = v
		}
		out.Tables = append(out.Tables, copy)
	}
	return out
}

// No settlement or cooperative release can be authorized by a peer claim,
// a UI flag, or preparation alone. Verified terminal replay is still required.
func (r *rules) WillCoSign(match string, _ uint32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.allowed[match]
}
func (r *rules) Handle(ctx context.Context, in sdk.Message) error {
	select {
	case r.inbox <- in:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("game inbox full; local consumer is not keeping up")
	}
}
func (r *rules) handleWorld(in sdk.Message) error {
	if in.Kind != "w.world" {
		return fmt.Errorf("gameplay unavailable before verified match protocol")
	}
	if len(in.Body) > 4096 {
		return fmt.Errorf("oversized world agreement")
	}
	var msg worldMessage
	if err := json.Unmarshal(in.Body, &msg); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[in.Match]
	if !ok || in.GCID != rec.GCID {
		return fmt.Errorf("world for unknown table or chat")
	}
	if msg.World.Match != rec.Roster {
		return fmt.Errorf("world names another roster")
	}
	var logKey string
	for _, join := range rec.Joins {
		if join.LogKey == msg.Signer {
			logKey = join.LogKey
			break
		}
	}
	if logKey == "" {
		return fmt.Errorf("signer is not in the committed roster")
	}
	raw, err := hex.DecodeString(logKey)
	if err != nil {
		return err
	}
	pub, err := secp256k1.ParsePubKey(raw)
	if err != nil {
		return err
	}
	raw, err = hex.DecodeString(msg.Signature)
	if err != nil {
		return err
	}
	sig, err := schnorr.ParseSignature(raw)
	if err != nil {
		return err
	}
	hash := msg.World.Hash()
	if !sig.Verify(hash[:], pub) {
		return fmt.Errorf("invalid world signature")
	}
	if r.approvals[in.Match] == nil {
		r.approvals[in.Match] = map[string]worldMessage{}
	}
	if old, exists := r.approvals[in.Match][msg.Signer]; exists && old.World != msg.World {
		r.problems[in.Match] = "A participant signed conflicting battlefields"
		return fmt.Errorf("conflicting world signatures")
	}
	r.approvals[in.Match][msg.Signer] = msg
	return nil
}
