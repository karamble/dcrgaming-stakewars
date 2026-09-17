//go:build desktop

package main

import (
	"fmt"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"github.com/karamble/dcrstakewars/internal/tablelobby"
	"github.com/karamble/dcrstakewars/pkg/render"
	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

func liveInvitation(r sdk.TableRecord) bridgeconn.Invitation {
	return bridgeconn.Invitation{
		SID: r.Match, GCID: r.GCID, Seats: r.Terms.Seats,
		BuyInAtoms: r.Terms.BuyInAtoms, CSVBlocks: r.Terms.CSVBlocks, Until: r.Terms.Until,
		AdmissionAtoms: r.Terms.BondAtoms, AdmissionBlocks: r.Terms.BondLockBlocks,
	}
}

func (g *game) pollSession() {
	if g.session == nil {
		return
	}
	v := g.session.Snapshot()
	g.sessionView = v
	if v.Status != "" {
		g.bridgeConnected = v.Connected
	}
	if v.Status != "" {
		g.bridgeNotice = v.Status
	}
	if v.Match == "" {
		return
	}
	for _, s := range v.Tables {
		if s.Record.Match != v.Match {
			continue
		}
		r := s.Record
		fresh := g.tableState == nil || g.tableState.Invite.SID != v.Match
		t := tablelobby.New(liveInvitation(r))
		t.Live = true
		t.Closed = r.RecoveryOnly || r.Aborted
		t.Reviewed = true
		t.Connected = g.bridgeConnected
		t.Height = v.Height
		t.ChainKnown = v.Height > 0
		t.RosterAgreed = r.Bound
		t.DrawVerified = len(s.Seats) == len(t.Seats)
		t.WorldVerified = v.WorldAgreed || v.Head != nil
		t.PayoutsKnown = len(r.Payouts) == len(t.Seats)
		t.Status = v.Status
		t.Error = v.Error
		t.CanFund = v.CanFund
		t.Ready = v.Head != nil
		for seat, key := range s.Seats {
			if int(seat) >= len(t.Seats) {
				continue
			}
			a := &t.Seats[seat]
			a.Name = fmt.Sprintf("Player %d · %.8s", seat+1, key)
			a.Ours = uint8(seat) == v.Mine
			a.Joined = true
			a.IdentityVerified = true
			a.Committed = true
			a.BondOptional = true
			if v.AdmissionChecked {
				a.Admission = tablelobby.Payment{Phase: tablelobby.Verified, Checked: true, Confirmations: 2, Required: 2}
			}
		}
		if len(s.Seats) == 0 {
			for i, j := range r.Joins {
				if i >= len(t.Seats) {
					break
				}
				t.Seats[i] = tablelobby.Seat{Ours: j.BondOutpoint == r.SeatBond.Outpoint, Name: fmt.Sprintf("Player · %.8s", j.Key), Joined: true, IdentityVerified: true, BondOptional: true}
			}
		}
		for _, d := range s.Deposits {
			seat := int(d.Seat)
			if d.Purpose == "seatbond" {
				if len(r.Joins) == 0 {
					// Before the join is published there is no roster position
					// yet. Show the local bond in a provisional card without
					// claiming that the player has joined.
					seat = 0
				} else if len(s.Seats) == 0 {
					for i, j := range r.Joins {
						if j.BondOutpoint == d.Outpoint {
							seat = i
							break
						}
					}
				}
			}
			if seat >= len(t.Seats) {
				continue
			}
			p := tablelobby.Payment{Confirmations: uint32(max(0, d.Confirmations)), Required: uint32(max(0, d.RequiredConfirmations)), Checked: d.Check == "verified" || d.Check == "confirming", Phase: tablelobby.Announced}
			switch d.Check {
			case "confirming":
				p.Phase = tablelobby.Confirming
			case "verified":
				p.Phase = tablelobby.Verified
			case "missing", "mismatch":
				p.Phase = tablelobby.Rejected
			}
			if d.Purpose == "stake" {
				t.Seats[seat].Stake = p
			}
			if d.Purpose == "seatbond" {
				t.Seats[seat].Admission = p
				if len(r.Joins) == 0 {
					t.Seats[seat].Ours = true
					t.Seats[seat].Name = "You · bond pending"
				}
			}
		}
		t.RefundStatus = "Refunds use your dcrpulse payout address."
		for _, d := range s.Deposits {
			if d.Purpose == "seatbond" || (d.Purpose == "stake" && d.Seat == uint32(v.Mine)) {
				lock := r.Terms.CSVBlocks
				label := "Stake"
				if d.Purpose == "seatbond" {
					lock = r.Terms.BondLockBlocks
					label = "Entry"
				}
				remaining := max(int64(0), int64(lock)-d.Confirmations)
				if d.Check == "verified" || d.Check == "confirming" {
					t.RefundStatus += fmt.Sprintf(" %s: %d blocks left.", label, remaining)
				} else {
					t.RefundStatus += " " + label + ": recheck required."
				}
			}
		}
		g.tableState = &t
		if fresh {
			g.tableOpen = true
			g.settingsOpen = false
			g.arena = false
			g.tableSelected = -1
		}
	}
	// Initialize only at a durable replay boundary. Never overwrite an active shot.
	if v.Head != nil && g.networkMatch != v.Match {
		g.networkMatch = v.Match
		g.s = v.Head.Clone()
		g.turnInputs = nil
		g.remoteBatch = nil
		g.submitted = false
		g.scene = render.New(g.s)
		g.gpu = render.NewGPU(g.scene)
		g.follow = true
		g.camera = render.Camera{Zoom: 1.25, Bounds: render.Stage(g.hideTop, g.hideBottom)}
		g.camera.Focus(g.s)
	}
	if g.submitted && v.Head != nil && v.Head.Tick >= g.s.Tick {
		g.submitted = false
	}
}
func (g *game) localTurn() bool {
	if g.session == nil || !g.bridgeConnected || g.sessionView.Head == nil || g.submitted {
		return false
	}
	g.message = fmt.Sprintf("You are player %d", g.sessionView.Mine+1)
	return g.s.ActiveSeat() == g.sessionView.Mine && g.sessionView.Head.Turn == g.s.Turn
}
func (g *game) updateRemoteTurn() error {
	g.charging = false
	g.power = 0
	if g.session == nil || !g.bridgeConnected {
		g.message = "Bridge disconnected · reconnect to resume"
		return nil
	}
	if g.submitted {
		g.message = "Saving and sending your signed turn…"
		return nil
	}
	if g.remoteBatch == nil {
		b, e := g.session.Accepted(g.networkMatch, g.s.Turn)
		if e != nil {
			g.message = fmt.Sprintf("Waiting for player %d · turns arrive over Bison Relay", g.s.ActiveSeat()+1)
			return nil
		}
		g.remoteBatch = &b
	}
	b := g.remoteBatch
	var in []sim.Input
	for _, i := range b.Inputs {
		if i.Tick == g.s.Tick {
			in = append(in, i)
		}
	}
	effects, e := sim.Step(g.s, in)
	if e != nil {
		return e
	}
	g.scene.Update(g.s, effects)
	g.speaker.play(g.scene.DrainSounds(), g.camera.X)
	g.camera.ObserveImpact(effects)
	if g.s.Tick == b.End {
		if replay.Hash(g.s) != b.After {
			return fmt.Errorf("remote replay diverged; payout withheld")
		}
		g.remoteBatch = nil
		g.message = ""
		g.follow = true
	}
	return nil
}

// Menus and the lobby suppress inputs, not the multiplayer turn countdown.
func (g *game) advanceNetworkIdle() error {
	if g.networkMatch == "" || g.s == nil || g.s.Phase == sim.Ended {
		return nil
	}
	if !g.localTurn() {
		return g.updateRemoteTurn()
	}
	turn := g.s.Turn
	effects, err := sim.Step(g.s, nil)
	if err != nil {
		return err
	}
	g.scene.Update(g.s, effects)
	g.camera.ObserveImpact(effects)
	if g.s.Turn != turn || g.s.Phase == sim.Ended {
		if err = g.session.Submit(g.networkMatch, g.s, g.turnInputs); err != nil {
			g.message = err.Error()
			return nil
		}
		g.submitted = true
		g.turnInputs = nil
	}
	return nil
}
