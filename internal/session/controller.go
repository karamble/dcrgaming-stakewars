package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
	"github.com/decred/slog"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/schema"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/transport"
	gw "github.com/karamble/dcrgaming-sdk/pkg/gaming/wire"
	"github.com/karamble/dcrgaming-sdk/pkg/identity"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
	"github.com/karamble/dcrgaming-sdk/pkg/spend"
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"github.com/karamble/dcrstakewars/internal/durable"
	"github.com/karamble/dcrstakewars/internal/seating"
	"github.com/karamble/dcrstakewars/internal/turnbatch"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

var sessionLog = slog.Disabled
var sdkLog = slog.Disabled

// UseLoggers must be called before starting any sessions.
func UseLoggers(game, sdk slog.Logger) { sessionLog = game; sdkLog = sdk }

// View is detached from controller state. UI rendering cannot authorize payment.
type View struct {
	Connected                        bool
	Status, Error, Match, Settlement string
	Phase                            string
	Mine                             uint8
	Head                             *sim.State
	CanFund                          bool
	AdmissionChecked                 bool
	WorldAgreed                      bool
	Height                           uint32
	Tables                           []sdk.TableSnapshot
}
type turnCommand struct {
	Match  string
	After  *sim.State
	Inputs []sim.Input
}
type operation struct {
	Match, Kind string
	Err         error
}
type liveMatch struct {
	journal *turnbatch.Journal
	key     *secp256k1.PrivateKey
	world   seating.World
	mine    uint8
	funded  bool
	settled string
}
type Controller struct {
	mu           sync.Mutex
	view         View
	matches      map[string]*liveMatch
	rules        *rules
	runtime      *sdk.Runtime
	bridge       *transport.Bridge
	tables       sdk.TableStore
	dir          string
	reservations *durable.Store
	turns        chan turnCommand
	actions      chan string
	done         chan operation
	jobs         map[string]bool
	activeJobs   map[string]bool
	workers      sync.WaitGroup
	selected     string
	financialAt  map[string]time.Time
	financial    map[string]sdk.TableSnapshot
}

func Open(ctx context.Context, cfg bridgeconn.Config, dir string) (*Controller, error) {
	bridge, e := bridgeconn.ConnectGame(ctx, cfg, Identity())
	if e != nil {
		return nil, e
	}
	c, e := New(bridge, dir)
	if e != nil {
		bridge.Close()
		return nil, e
	}
	return c, nil
}

// New also supports local mTLS bridge fixtures; all durable state lives under dir.
func New(bridge *transport.Bridge, dir string) (*Controller, error) {
	if bridge == nil {
		return nil, fmt.Errorf("bridge required")
	}
	var params *chaincfg.Params
	switch bridge.Network() {
	case "mainnet":
		params = chaincfg.MainNetParams()
	case "testnet3":
		params = chaincfg.TestNet3Params()
	case "simnet":
		params = chaincfg.SimNetParams()
	default:
		return nil, fmt.Errorf("bridge network has not been authenticated")
	}

	id, e := identity.Load(filepath.Join(dir, "identity"))
	if e != nil {
		return nil, e
	}
	tables, e := sdk.NewFileTableStore(filepath.Join(dir, "tables"))
	if e != nil {
		return nil, e
	}
	ss, e := spend.FileStore(filepath.Join(dir, "spends.json"))
	if e != nil {
		return nil, e
	}
	book, e := spend.OpenBook(ss)
	if e != nil {
		return nil, e
	}
	reservations, e := durable.Open(filepath.Join(dir, "world-signatures"))
	if e != nil {
		return nil, e
	}
	r := &rules{bridge: bridge, records: map[string]sdk.TableRecord{}, worlds: map[string]worldMessage{}, approvals: map[string]map[string]worldMessage{}, problems: map[string]string{}, accepted: make(chan string, 64), inbox: make(chan sdk.Message, 256), allowed: map[string]bool{}, settled: map[string]string{}, receipts: reservations}
	rt, e := sdk.New(sdk.Config{Log: sdkLog, Rules: r, Bridge: bridge, Book: book, Identity: id, Params: params, Tables: tables, SeatTags: identity.SeatTags{Session: "StakeWars/session/v1", Log: "StakeWars/log/v1", Bond: "StakeWars/bond/v1"}})
	if e != nil {
		return nil, e
	}
	r.runtime = rt
	c := &Controller{runtime: rt, rules: r, bridge: bridge, tables: tables, dir: dir, reservations: reservations, matches: map[string]*liveMatch{}, turns: make(chan turnCommand, 16), actions: make(chan string, 16), done: make(chan operation, 32), jobs: map[string]bool{}, activeJobs: map[string]bool{}, financialAt: map[string]time.Time{}, financial: map[string]sdk.TableSnapshot{}}
	if _, e = rt.ResumeWithReport(); e != nil {
		rt.Close()
		return nil, e
	}
	return c, nil
}
func (c *Controller) Snapshot() View {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.view
	if v.Head != nil {
		v.Head = v.Head.Clone()
	}
	v.Tables = append([]sdk.TableSnapshot(nil), v.Tables...)
	return v
}
func (c *Controller) Submit(match string, after *sim.State, in []sim.Input) error {
	if after == nil {
		return errors.New("missing turn state")
	}
	cmd := turnCommand{match, after.Clone(), append([]sim.Input(nil), in...)}
	select {
	case c.turns <- cmd:
		return nil
	default:
		return errors.New("turn queue full")
	}
}
func (c *Controller) Action(action string) error {
	select {
	case c.actions <- action:
		return nil
	default:
		return errors.New("action queue full")
	}
}
func (c *Controller) Accepted(match string, turn uint32) (turnbatch.Batch, error) {
	c.mu.Lock()
	m := c.matches[match]
	c.mu.Unlock()
	if m == nil {
		return turnbatch.Batch{}, errors.New("unknown match")
	}
	raw, e := m.journal.AcceptedTurn(turn)
	if e != nil {
		return turnbatch.Batch{}, e
	}
	return turnbatch.Decode(bytes.NewReader(raw))
}
func (c *Controller) setError(e error) {
	if e == nil {
		return
	}
	message := e.Error()
	c.mu.Lock()
	same := c.view.Error == message
	c.view.Error = message
	c.mu.Unlock()
	if !same {
		sessionLog.Warnf("Session operation: %v", e)
	}
}
func (c *Controller) job(ctx context.Context, match, kind string, f func(context.Context) error) {
	id := match + ":" + kind
	if c.jobs[id] {
		return
	}
	c.jobs[id] = true
	c.activeJobs[id] = true
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		e := f(ctx)
		select {
		case c.done <- operation{match, kind, e}:
		case <-ctx.Done():
		}
	}()
}
func (c *Controller) Run(ctx context.Context) error {
	sessionLog.Info("Session connected; restoring tables and signed turns")
	// Install durable table routes before subscribing. BR replays group-chat
	// history immediately, and those one-shot frames must not race the first
	// periodic UI refresh after a restart.
	records, err := c.tables.LoadTables()
	if err != nil {
		return err
	}
	c.rules.mu.Lock()
	for _, record := range records {
		c.rules.records[record.Match] = record
	}
	c.rules.mu.Unlock()
	defer sessionLog.Info("Session stopped")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer c.bridge.Close()
	defer func() { cancel(); c.workers.Wait() }()
	result := make(chan error, 1)
	go func() { result <- c.runtime.Run(ctx) }()
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			cancel()
			<-result
			return nil
		case e := <-result:
			return e
		case <-timer.C:
			tickCtx, tickDone := context.WithTimeout(ctx, 8*time.Second)
			if tip, err := c.runtime.Chain(tickCtx); err == nil {
				c.runtime.Tick(tickCtx, tip.Height)
			}
			tickDone()
			c.refresh(ctx)
		case match := <-c.rules.accepted:
			c.selected = match
		case in := <-c.rules.inbox:
			call, done := context.WithTimeout(ctx, 5*time.Second)
			c.setError(c.receive(call, in))
			done()
		case cmd := <-c.turns:
			call, done := context.WithTimeout(ctx, 5*time.Second)
			c.setError(c.submit(call, cmd))
			done()
		case a := <-c.actions:
			c.action(ctx, a)
		case op := <-c.done:
			delete(c.activeJobs, op.Match+":"+op.Kind)
			if op.Kind == "settle" && op.Err != nil {
				delete(c.jobs, op.Match+":"+op.Kind)
			}
			c.setError(op.Err)
		}
	}
}
func (c *Controller) action(ctx context.Context, a string) {
	if strings.HasPrefix(a, "select:") {
		c.selected = strings.TrimPrefix(a, "select:")
		return
	}
	v := c.Snapshot()
	if c.activeJobs[v.Match+":"+a] {
		return
	}
	if v.Match == "" {
		c.setError(errors.New("select a table first"))
		return
	}
	switch a {
	case "fund":
		if !v.CanFund {
			c.setError(errors.New("funding requires verified battlefield agreement"))
			return
		}
		delete(c.jobs, v.Match+":fund")
		c.job(ctx, v.Match, "fund", func(ctx context.Context) error { return c.runtime.Fund(ctx, v.Match) })
	case "refund", "bond":
		c.setError(errors.New("Manage this deposit in dcrpulse → Gaming → Recovery"))
	}
}
func (c *Controller) refresh(ctx context.Context) {
	records, e := c.tables.LoadTables()
	if e != nil {
		c.setError(e)
		return
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Match < records[j].Match })
	c.rules.mu.Lock()
	for _, r := range records {
		c.rules.records[r.Match] = r
	}
	c.rules.mu.Unlock()
	connection, _ := c.bridge.ConnectionStatus()
	v := View{Connected: connection == transport.Subscribed, Status: "Connected · waiting for a dcrpulse invitation"}
	selected := c.selected
	if selected == "" {
		var latest uint32
		for _, rec := range records {
			if !rec.RecoveryOnly && !rec.Aborted && (selected == "" || rec.Terms.Until > latest) {
				selected, latest = rec.Match, rec.Terms.Until
			}
		}
	}
	chainCtx, chainDone := context.WithTimeout(ctx, 3*time.Second)
	if tip, err := c.runtime.Chain(chainCtx); err == nil && tip.Height > 0 {
		v.Height = uint32(tip.Height)
	}
	chainDone()
	for _, rec := range records {
		snap, e := c.runtime.Snapshot(rec.Match)
		if e != nil {
			c.setError(e)
			continue
		}
		if cached, ok := c.financial[rec.Match]; ok {
			snap.Deposits = cached.Deposits
		}
		if rec.Match == selected && time.Since(c.financialAt[rec.Match]) >= 5*time.Second {
			c.financialAt[rec.Match] = time.Now()
			call, done := context.WithTimeout(ctx, 4*time.Second)
			refreshed, refreshErr := c.runtime.RefreshDeposits(call, rec.Match)
			done()
			if refreshErr != nil {
				c.setError(refreshErr)
			} else {
				snap = refreshed
				c.financial[rec.Match] = refreshed
			}
		}
		v.Tables = append(v.Tables, snap)
		if v.Match != "" || selected != rec.Match {
			continue
		} // one foreground match; all deposits remain in dashboard
		v.Match = rec.Match
		v.Phase = snap.Phase
		v.Status = "Preparing table · " + snap.Phase
		receipt, readErr := c.reservations.Get(receiptKey(rec.Match))
		if readErr == nil {
			v.Settlement = string(receipt)
			v.Phase = "finished"
			v.Status = "Payout broadcast · " + v.Settlement
			c.mu.Lock()
			if m := c.matches[rec.Match]; m != nil {
				v.Head = m.journal.Head()
			}
			c.mu.Unlock()
			continue
		}
		if !errors.Is(readErr, os.ErrNotExist) {
			c.setError(readErr)
			continue
		}
		if snap.Record.RecoveryOnly || snap.Record.Aborted {
			v.Status = "Table closed · deposits remain recoverable"
			continue
		}
		if len(snap.Seats) != int(rec.Terms.Seats) {
			continue
		}
		mine, ok := c.runtime.Seat(rec.Match)
		if !ok {
			continue
		}
		v.Mine = uint8(mine)
		call, done := context.WithTimeout(ctx, 4*time.Second)
		e = c.runtime.CheckAdmissionBonds(call, rec.Match)
		if e == nil {
			v.AdmissionChecked = true
			e = c.prepareWorld(call, rec)
		}
		done()
		if e != nil {
			v.Status = e.Error()
			continue
		}
		c.mu.Lock()
		m := c.matches[rec.Match]
		c.mu.Unlock()
		if m == nil {
			v.Status = "Waiting for every player to verify the battlefield"
			continue
		}
		v.WorldAgreed = true
		c.rules.mu.Lock()
		m.settled = c.rules.settled[rec.Match]
		c.rules.mu.Unlock()
		if m.settled != "" {
			v.Head = m.journal.Head()
			v.Phase = "finished"
			v.Settlement = m.settled
			v.Status = "Payout broadcast · " + m.settled
			continue
		}
		count := 0
		for _, d := range snap.Deposits {
			if d.Purpose == "stake" && d.Check == "verified" {
				count++
			}
		}
		if c.runtime.PayoutFor(rec.Match) == "" {
			v.Status = "Waiting for the bridge-owned payout destination"
			continue
		}
		if count != int(rec.Terms.Seats) {
			v.CanFund = len(snap.Record.Funded) == 0
			if _, exists := snap.Record.Funded[uint32(m.mine)]; !exists {
				v.CanFund = true
			}
			for _, pay := range snap.Payments {
				if pay.Purpose == "stake" {
					v.CanFund = false
				}
			}
			v.Status = fmt.Sprintf("Battlefield agreed · %d/%d stakes confirmed · F to fund", count, rec.Terms.Seats)
			continue
		}
		if len(snap.Record.Payouts) != int(rec.Terms.Seats) {
			v.Status = "Waiting for payout addresses in dcrpulse"
			continue
		}
		m.funded = true
		v.Head = m.journal.Head()
		v.Phase = "playing"
		v.Status = "Playing · signed turns over Bison Relay"
		v.Settlement = m.settled
		if v.Head.Phase == sim.Ended {
			v.Phase = "finished"
			v.Status = "Match finished · collecting payout signatures"
			manifest, e := seating.Preset(rec.Terms.Seats, rec.Terms.BuyInAtoms)
			if e != nil {
				c.setError(e)
				continue
			}
			amounts, e := manifest.Payout.Allocation(v.Head.Winner)
			if e != nil {
				c.setError(e)
				continue
			}
			out := sdk.Outcome{Shares: map[uint32]int64{}}
			for seat, a := range amounts {
				out.Shares[uint32(seat)] = a
			}
			c.rules.mu.Lock()
			c.rules.allowed[rec.Match] = true
			c.rules.mu.Unlock()
			c.job(ctx, rec.Match, "settle", func(ctx context.Context) error { return c.runtime.Settle(ctx, rec.Match, out) })
		}
	}
	c.mu.Lock()
	v.Error = c.view.Error
	previousStatus := c.view.Status
	c.view = v
	c.mu.Unlock()
	if v.Status != previousStatus {
		sessionLog.Infof("Table %s: %s", v.Match, v.Status)
	}
	c.rules.mu.Lock()
	c.rules.state = sdk.State{Summary: v.Status}
	for _, s := range v.Tables {
		c.rules.state.Tables = append(c.rules.state.Tables, sdk.TableState{Match: s.Record.Match, Seats: s.Record.Terms.Seats, Status: v.Status, Detail: map[string]string{"paymentModel": "All players sign payouts; owner refunds after locks", "error": v.Error}})
	}
	c.rules.mu.Unlock()
}
func (c *Controller) prepareWorld(ctx context.Context, rec sdk.TableRecord) error {
	c.mu.Lock()
	existing := c.matches[rec.Match]
	c.mu.Unlock()
	if existing != nil {
		anchor, err := c.runtime.BlockHash(ctx, existing.world.AnchorHeight)
		if err != nil {
			return err
		}
		if anchor != rec.Beacon {
			return errors.New("seating block changed; recover deposits")
		}
		c.rules.mu.Lock()
		problem := c.rules.problems[rec.Match]
		c.rules.mu.Unlock()
		if problem != "" {
			return errors.New(problem)
		}
		return nil
	}
	tip, e := c.runtime.Chain(ctx)
	if e != nil {
		return e
	}
	// The SDK has already committed the roster to its beacon before this point.
	world, state, e := seating.BuildWorld(rec.Terms, rec.Roster, rec.Beacon)
	if e != nil {
		return e
	}
	if tip.Height < int64(world.AnchorHeight+seating.AnchorConfirmations-1) {
		return fmt.Errorf("Waiting for seating block: %d more confirmations", int64(world.AnchorHeight+seating.AnchorConfirmations-1)-tip.Height)
	}
	anchor, e := c.runtime.BlockHash(ctx, world.AnchorHeight)
	if e != nil {
		return e
	}
	if anchor != rec.Beacon {
		return errors.New("seating block changed; recover deposits and create a new table")
	}
	key, e := c.runtime.LogKey(rec.Match)
	if e != nil {
		return e
	}
	h := world.Hash()
	position := sha256.Sum256([]byte("StakeWars/world-reservation/v1/" + rec.Match))
	if e = c.reservations.Put(position, h[:]); e != nil {
		return e
	}
	sig, e := schnorr.Sign(key.Priv(), h[:])
	if e != nil {
		return e
	}
	own := worldMessage{world, hex.EncodeToString(key.Public().SerializeCompressed()), hex.EncodeToString(sig.Serialize())}
	c.rules.mu.Lock()
	if c.rules.approvals[rec.Match] == nil {
		c.rules.approvals[rec.Match] = map[string]worldMessage{}
	}
	c.rules.worlds[rec.Match] = own
	c.rules.approvals[rec.Match][own.Signer] = own
	all := len(c.rules.approvals[rec.Match]) == int(rec.Terms.Seats)
	for _, a := range c.rules.approvals[rec.Match] {
		if a.World != world {
			all = false
		}
	}
	problem := c.rules.problems[rec.Match]
	c.rules.mu.Unlock()
	if problem != "" {
		return errors.New(problem)
	}
	if e = c.runtime.Send(ctx, rec.Match, "w.world", own, gw.ClassState); e != nil {
		return e
	}
	if !all {
		return nil
	}
	logs, ok := c.runtime.LogSeats(rec.Match)
	if !ok {
		return errors.New("missing log roster")
	}
	matchID, e := hex.DecodeString(rec.Roster)
	if e != nil {
		return e
	}
	termsHash, e := rec.Terms.Hash()
	if e != nil {
		return e
	}
	tc := turnbatch.Context{Terms: termsHash, Keys: make([][33]byte, rec.Terms.Seats)}
	copy(tc.Match[:], matchID)
	for seat, pub := range logs {
		if int(seat) >= len(tc.Keys) || len(pub) != 33 {
			return errors.New("invalid log roster")
		}
		copy(tc.Keys[seat][:], pub)
	}
	journal, e := turnbatch.OpenJournal(filepath.Join(c.dir, "turns", hex.EncodeToString(tc.Match[:])), tc, state)
	if e != nil {
		return e
	}
	if journal.Head().Phase != sim.Ended {
		b, err := journal.ResumeSignedTurn(key.Priv())
		if err == nil {
			if err = journal.Receive(b); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	mine, ok := c.runtime.Seat(rec.Match)
	if !ok {
		return errors.New("local seat missing")
	}
	c.mu.Lock()
	c.matches[rec.Match] = &liveMatch{journal: journal, key: key.Priv(), world: world, mine: uint8(mine)}
	c.mu.Unlock()
	return nil
}
func (c *Controller) submit(ctx context.Context, cmd turnCommand) error {
	c.mu.Lock()
	m := c.matches[cmd.Match]
	c.mu.Unlock()
	if m == nil || !m.funded {
		return errors.New("match is not funded")
	}
	if m.journal.Head().ActiveSeat() != m.mine {
		return errors.New("not your turn")
	}
	b, e := m.journal.SignedTurn(cmd.After, cmd.Inputs, m.key)
	if e != nil {
		return e
	}
	if e = m.journal.Receive(b); e != nil {
		return e
	}
	c.publishHead(cmd.Match, m)
	return c.runtime.Send(ctx, cmd.Match, "w.turn", b, gw.ClassTurn)
}
func (c *Controller) receive(ctx context.Context, in sdk.Message) error {
	c.rules.mu.Lock()
	rec, known := c.rules.records[in.Match]
	c.rules.mu.Unlock()
	if !known || rec.GCID != in.GCID {
		return errors.New("message for unknown table or chat")
	}
	if in.Kind == "w.world" {
		return c.rules.handleWorld(in)
	}
	c.mu.Lock()
	m := c.matches[in.Match]
	c.mu.Unlock()
	if m == nil {
		return errors.New("game message before world agreement")
	}
	switch in.Kind {
	case "w.turn":
		var b turnbatch.Batch
		if len(in.Body) > 1<<20 {
			return errors.New("oversized turn")
		}
		if e := json.Unmarshal(in.Body, &b); e != nil {
			return e
		}
		err := m.journal.Receive(b)
		if err == nil {
			c.publishHead(in.Match, m)
		}
		return err
	case "w.sync":
		var q struct{ Turn uint32 }
		if e := json.Unmarshal(in.Body, &q); e != nil {
			return e
		}
		head := m.journal.Head()
		limit := head.Turn
		if head.Phase == sim.Ended {
			limit++
		}
		for turn := q.Turn; turn < limit && turn-q.Turn < 6; turn++ {
			raw, e := m.journal.AcceptedTurn(turn)
			if e != nil {
				return e
			}
			b, e := turnbatch.Decode(bytes.NewReader(raw))
			if e != nil {
				return e
			}
			if e = c.runtime.Send(ctx, in.Match, schema.Kind("w.turn"), b, gw.ClassState); e != nil {
				return e
			}
		}
	}
	return nil
}
func (r *rules) Settled(_ context.Context, match, txid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.receipts.Put(receiptKey(match), []byte(txid)); err != nil {
		r.problems[match] = err.Error()
		return
	}
	r.state.Summary = "Payout broadcast · " + txid
	r.allowed[match] = false
	r.settled[match] = txid
}

func (c *Controller) publishHead(match string, m *liveMatch) {
	if !m.funded {
		return
	}
	head := m.journal.Head()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.view.Match == match {
		c.view.Head = head
	}
}

func receiptKey(match string) [32]byte {
	return sha256.Sum256([]byte("StakeWars/payout-receipt/v1/" + match))
}
