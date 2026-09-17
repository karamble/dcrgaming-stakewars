package bridgeconn

import (
	"context"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/karamble/dcrgaming-sdk/pkg/gaming/gamingpb"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/schema"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/transport"
	"github.com/karamble/dcrstakewars/internal/payout"
)

const PaidUnavailable = "Paid seating is not enabled in this build. SDK recovery is implemented; lobby integration is pending. No funds requested."

// Invitation is an untrusted offer, never a verified roster or a funded seat.
type Invitation struct {
	SID, GCID        string
	Seats            uint32
	BuyInAtoms       uint64
	CSVBlocks, Until uint32
	AdmissionAtoms   uint64
	AdmissionBlocks  uint32
	TableBondAtoms   uint64
	TableBondBlocks  uint32
}

func PreviewInvite(link, gcid string) (Invitation, error) {
	invalid := errors.New("Invalid StakeWars invitation: require a table, session, 2–6 seats, buy-in, refund delay and deadline.")
	if len(link) > 4096 || len(gcid) != 64 {
		return Invitation{}, invalid
	}
	decoded, err := hex.DecodeString(gcid)
	if err != nil || len(decoded) != 32 {
		return Invitation{}, invalid
	}
	u, err := url.Parse(link)
	if err != nil || u.User != nil || u.Fragment != "" {
		return Invitation{}, invalid
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Invitation{}, invalid
	}
	for key, values := range q {
		if len(values) != 1 {
			return Invitation{}, invalid
		}
		switch key {
		case "sid", "seats", "buyin", "csv", "until", "fv", "bond", "bondcsv", "tablebond", "tablebondcsv":
		default:
			return Invitation{}, invalid
		}
	}
	inv, err := schema.ParseInvite(link)
	if err != nil || inv.Game != GameID || inv.Kind != schema.InviteKindTable || inv.SID == "" || inv.Seats < 2 || inv.Seats > 6 || inv.BuyInAtoms == 0 || inv.BuyInAtoms > uint64(payout.MaxAtoms)/uint64(inv.Seats) || inv.CSVBlocks == 0 || inv.Until == 0 {
		return Invitation{}, invalid
	}
	return Invitation{SID: inv.SID, GCID: strings.ToLower(gcid), Seats: inv.Seats, BuyInAtoms: inv.BuyInAtoms, CSVBlocks: inv.CSVBlocks, Until: inv.Until, AdmissionAtoms: inv.AdmissionAtoms, AdmissionBlocks: inv.AdmissionBlocks, TableBondAtoms: inv.TableBondAtoms, TableBondBlocks: inv.TableBondBlocks}, nil
}

// Update is an immutable snapshot delivered to the main UI thread.
type Update struct {
	Status, Notice  string
	Connected, Busy bool
	Invite          *Invitation
	InviteSerial    uint64
	ChainKnown      bool
	Height          uint32
}

// RunLobby receives local bridge events. Peer gameplay remains asynchronous and
// is not admitted here until seating, identity and payment gates are implemented.
// emit must return promptly or respect ctx cancellation.
func RunLobby(ctx context.Context, cfg Config, emit func(Update)) {
	runLobby(ctx, cfg, emit, 10*time.Second, time.Second)
}
func runLobby(ctx context.Context, cfg Config, emit func(Update), heartbeat, retry time.Duration) {
	if err := cfg.Validate(); err != nil {
		emit(Update{Status: err.Error()})
		return
	}
	for ctx.Err() == nil {
		emit(Update{Status: "Connecting securely…", Busy: true})
		dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		b, err := connect(dialCtx, cfg)
		cancel()
		if err == nil {
			serveLobby(ctx, b, emit, heartbeat)
			b.Close()
			return
		}
		emit(Update{Status: err.Error() + " Retrying…", Busy: true})
		timer := time.NewTimer(retry)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if retry < 30*time.Second {
			retry *= 2
			if retry > 30*time.Second {
				retry = 30 * time.Second
			}
		}
	}
}

func serveLobby(ctx context.Context, b *transport.Bridge, emit func(Update), heartbeat time.Duration) {
	frames, err := b.Events(ctx)
	if err != nil {
		emit(Update{Status: "Could not subscribe to bridge events."})
		return
	}
	snapshot := Update{Status: "Connected · StakeWars · mainnet", Connected: true, Notice: "Waiting for invitations. Seating and payments are disabled."}
	publish := func() { emit(snapshot) }
	publish()
	var seq uint64
	state := func(requestID string) *gamingpb.GameState {
		seq++
		// Invitations are deliberately not reported as joined tables.
		return &gamingpb.GameState{RequestId: requestID, ReportedAt: time.Now().Unix(), Seq: seq}
	}
	report := func() {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := b.ReportState(callCtx, state(""))
		cancel()
		snapshot.Connected = err == nil
		snapshot.ChainKnown = false
		if err == nil {
			tipCtx, done := context.WithTimeout(ctx, 5*time.Second)
			tip, tipErr := b.ChainTip(tipCtx)
			done()
			if tipErr == nil && tip.Height > 0 && tip.Height <= int64(^uint32(0)) {
				snapshot.Height = uint32(tip.Height)
				snapshot.ChainKnown = true
			}
		}
		if err != nil {
			snapshot.Status = "Bridge unavailable. Retrying; event delivery reconnects automatically."
		} else {
			snapshot.Status = "Connected · StakeWars · mainnet"
		}
		publish()
	}
	report()
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report()
		case _, ok := <-frames:
			if !ok {
				return
			}
			// No admitted tables: never allocate peer assembly state or apply a frame.
		case req := <-b.Requests():
			if req == nil {
				continue
			}
			reply := &gamingpb.RespondRequest{RequestId: req.GetRequestId()}
			now := time.Now()
			switch {
			case req.GetRequestId() == "" || len(req.GetRequestId()) > 256:
				continue
			case req.GetDeadlineUnix() > 0 && req.GetDeadlineUnix() <= now.Unix():
				reply.Error = "Bridge request expired. Try again from dcrpulse."
			case req.GetAcceptInvite() != nil:
				invite, err := PreviewInvite(req.GetAcceptInvite().GetInvite(), req.GetAcceptInvite().GetGcid())
				if err != nil {
					reply.Error = err.Error()
				} else {
					snapshot.Invite = &invite
					snapshot.InviteSerial++
					snapshot.Notice = "Unverified invitation · not seated · no funds requested"
					reply.Error = PaidUnavailable
					publish()
				}
			case req.GetRefreshState() != nil:
				reply.Ok = true
				reply.Result = &gamingpb.RespondRequest_State{State: state(req.GetRequestId())}
			default:
				reply.Error = "This StakeWars build does not support that bridge action."
			}
			deadline := now.Add(5 * time.Second)
			if req.GetDeadlineUnix() > now.Unix() && time.Unix(req.GetDeadlineUnix(), 0).Before(deadline) {
				deadline = time.Unix(req.GetDeadlineUnix(), 0)
			}
			callCtx, cancel := context.WithDeadline(ctx, deadline)
			err := b.Respond(callCtx, reply)
			cancel()
			if err != nil {
				snapshot.Notice = "Could not deliver the bridge response. Try again from dcrpulse."
				publish()
			}
		}
	}
}
