//go:build desktop

package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/karamble/dcrstakewars/internal/tablelobby"
	"github.com/karamble/dcrstakewars/pkg/render"
	"image"
)

func (g *game) receiveTableUpdate(u connectionUpdate) {
	if u.invite != nil && u.inviteSerial != g.tableSerial {
		t := tablelobby.New(*u.invite)
		// Acceptance was already requested by the person clicking in dcrpulse.
		// This build opens preparation but refuses paid admission back to the host.
		t.Reviewed = true
		g.tableState = &t
		g.tableSelected = -1
		g.tableSerial = u.inviteSerial
		g.tableOpen = true
		g.settingsOpen = false
		g.arena = false
		g.paused = false
		g.weaponMenu = false
	}
	if g.tableState != nil && !g.tableState.Demo {
		g.tableState.Connected = u.connected
		g.tableState.ChainKnown = u.chainKnown
		g.tableState.Height = u.height
		g.tableState.Stale = u.gap
	}
}
func (g *game) tableLobbyView() render.TableLobbyView {
	return render.TableLobbyView{Open: g.tableOpen, Table: g.tableState, Selected: g.tableSelected, Help: g.tableHelp, DemoScenario: g.tableDemoScenario}
}
func (g *game) updateTableLobby() {
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	x, y := ebiten.CursorPosition()
	p := image.Pt(x, y)
	if g.tableHelp {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (click && p.In(render.TableHelpCloseRect)) {
			g.tableHelp = false
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (click && p.In(render.TableBackRect)) {
		g.tableOpen = false
		return
	}
	if g.session != nil && g.tableState != nil && g.tableState.Live {
		if inpututil.IsKeyJustPressed(ebiten.KeyF) && g.sessionView.CanFund {
			_ = g.session.Action("fund")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			_ = g.session.Action("refund")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyB) {
			_ = g.session.Action("bond")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyTab) && len(g.sessionView.Tables) > 1 {
			for i, s := range g.sessionView.Tables {
				if s.Record.Match == g.sessionView.Match {
					_ = g.session.Action("select:" + g.sessionView.Tables[(i+1)%len(g.sessionView.Tables)].Record.Match)
					break
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && g.sessionView.Head != nil {
			g.arena = true
			g.paused = false
			return
		}
		if click && p.In(render.TableReviewRect) {
			if g.sessionView.Head != nil {
				g.arena = true
				g.paused = false
			} else if g.sessionView.CanFund {
				_ = g.session.Action("fund")
			}
			return
		}
		if click && p.In(render.TableRefundRect) {
			_ = g.session.Action("refund")
			return
		}
		if click && p.In(render.TableBondRefundRect) {
			_ = g.session.Action("bond")
			return
		}
	}
	if !click {
		return
	}
	if p.In(render.TableCreateHelpRect) {
		g.tableHelp = true
		return
	}
	if g.tableState == nil {
		return
	}
	t := g.tableState
	for i := range t.Seats {
		if p.In(render.TableSeatRect(len(t.Seats), i)) {
			g.tableSelected = i
			return
		}
	}
	if p.In(render.TableTermsRect) {
		g.tableSelected = -1
	}
	if p.In(render.TableSeatTabRect) && g.tableSelected < 0 {
		g.tableSelected = 0
	}
	if p.In(render.TableReviewRect) {
		t.Reviewed = true
	}
	if t.Demo {
		count := int(t.Invite.Seats)
		if p.In(render.TableDemoNextRect) {
			g.tableDemoScenario = (g.tableDemoScenario + 1) % 7
		}
		if p.In(render.TableDemoSizeRect) {
			count += 2
			if count > 6 {
				count = 2
			}
		}
		if p.In(render.TableDemoNextRect) || p.In(render.TableDemoSizeRect) {
			next := tablelobby.Demo(count, g.tableDemoScenario)
			g.tableState = &next
			if g.tableSelected >= count {
				g.tableSelected = -1
			}
		}
	}
}
