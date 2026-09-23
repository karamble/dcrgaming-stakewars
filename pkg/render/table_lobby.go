package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/karamble/dcrgaming-stakewars/internal/seating"
	"github.com/karamble/dcrgaming-stakewars/internal/tablelobby"
)

var LobbyTablesRect = image.Rect(480, 675, 780, 731)
var TableCreateHelpRect = image.Rect(1090, 115, 1376, 157)
var TableHelpCloseRect = image.Rect(1090, 219, 1160, 261)
var TableBackRect = image.Rect(48, 115, 235, 150)

var TableRefundRect = image.Rect(48, 840, 260, 877)
var TableBondRefundRect = image.Rect(280, 840, 500, 877)
var TableReviewRect = image.Rect(1020, 784, 1376, 834)
var TableTermsRect = image.Rect(1010, 302, 1185, 340)
var TableSeatTabRect = image.Rect(1190, 302, 1376, 340)
var TableDemoNextRect = image.Rect(1110, 850, 1376, 887)
var TableDemoSizeRect = image.Rect(860, 850, 1090, 887)

func TableSeatRect(count, index int) image.Rectangle {
	cols := 3
	if count <= 4 {
		cols = 2
	}
	width := (928 - (cols-1)*16) / cols
	x, y := 48+(index%cols)*(width+16), 350+(index/cols)*177
	return image.Rect(x, y, x+width, y+161)
}

type TableLobbyView struct {
	Open         bool
	Help         bool
	Table        *tablelobby.Table
	Selected     int
	Error        string
	DemoScenario int
}

var tableAmber = color.RGBA{255, 195, 104, 255}

func tableButton(c Canvas, r image.Rectangle, label string, active bool, v View) {
	col := Panel
	ink := White
	if active {
		col = Mint
		ink = Navy
	}
	if image.Pt(v.MouseX, v.MouseY).In(r) {
		if active {
			col = color.RGBA{80, 239, 192, 255}
		} else {
			col = color.RGBA{28, 52, 74, 255}
		}
	}
	c.Rect(float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()), col)
	c.Text(label, float64(r.Min.X+16), float64(r.Min.Y+12), 14, ink)
}
func tableLines(c Canvas, text string, x, y, width, size float64, col color.RGBA) {
	words := strings.Fields(text)
	line := ""
	row := 0
	for _, w := range words {
		if len(line)+len(w)+1 > int(width/(size*.56)) && line != "" {
			c.Text(line, x, y+float64(row)*(size+7), size, col)
			row++
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	c.Text(line, x, y+float64(row)*(size+7), size, col)
}
func shortTableText(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
func drawTableLobby(c Canvas, v View) {
	l := v.TableLobby
	brand(c)
	settingsCog(c)
	tableButton(c, TableBackRect, "< BACK TO LOBBY", false, v)
	tableButton(c, TableCreateHelpRect, "CREATE / JOIN IN DCRPULSE", false, v)
	if l.Table == nil {
		c.Text("YOUR NEXT BATTLE", 65, 221, 48, White)
		c.Text("STARTS AT THE TABLE.", 65, 283, 48, Mint)
		tableLines(c, "Connect StakeWars to your bridge, then click Accept on a StakeWars invitation in your dcrpulse group chat. This table room opens automatically.", 68, 384, 590, 21, Muted)
		c.Text("01  REVIEW TERMS", 68, 512, 17, White)
		c.Text("02  VERIFY PLAYERS & BONDS", 68, 558, 17, White)
		c.Text("03  PREPARE YOUR SQUAD", 68, 604, 17, White)
		c.Circle(1090, 459, 175, color.RGBA{39, 211, 162, 16})
		stakey(c, 1080, 613, 6, Mint, 1, float64(v.Frame)/90, true)
		c.Rect(65, 713, 1309, 112, Panel)
		c.Text("No table selected", 87, 734, 21, White)
		c.Text("The chat supplies the invitation and group automatically. No copy/paste needed.", 87, 774, 16, Muted)
	} else {
		drawTableRoom(c, v, *l.Table)
	}
	if l.Help {
		drawTableHelp(c, v)
	}
}
func drawTableRoom(c Canvas, v View, t tablelobby.Table) {
	l := v.TableLobby
	joined, funded := t.Counts()
	c.Text("ASSEMBLE YOUR SQUAD", 48, 169, 34, White)
	tag := "INVITATION REVIEW"
	if t.Live {
		tag = "TABLE PREPARATION"
	}
	if t.Demo {
		tag = "INTERACTIVE UX DEMO · FICTIONAL PLAYERS"
	}
	c.Text(tag, 50, 210, 12, Mint)
	if t.Stale || !t.Connected || !t.ChainKnown {
		c.Text("CURRENT STATUS UNVERIFIED", 1010, 180, 16, tableAmber)
		c.Text("WAITING FOR FRESH LOCAL CHECKS", 1010, 207, 12, Muted)
	} else {
		c.Text(fmt.Sprintf("%d/%d PLAYERS VERIFIED", joined, t.Invite.Seats), 1010, 180, 16, White)
		c.Text(fmt.Sprintf("%d/%d FUNDING CHECKS COMPLETE", funded, t.Invite.Seats), 1010, 207, 12, Muted)
	}
	stage := t.Stage()
	labels := []string{"INVITATION", "ENTRY BOND", "ROSTER", "SEAT DRAW", "STAKE", "TABLE BOND", "MAP CHECK", "READY"}
	if t.Live {
		labels = []string{"INVITATION", "ENTRY BOND", "ROSTER", "SEATING BLOCK", "MAP CHECK", "STAKE", "PAYOUTS", "READY"}
	}
	for i, label := range labels {
		x := 48 + float64(i)*168
		col := Muted
		if i < stage {
			col = Mint
		}
		if i == stage {
			col = tableAmber
		}
		if t.Stale && i > 0 {
			col = Muted
		}
		c.Rect(x, 249, 156, 4, col)
		c.Circle(x+12, 277, 11, col)
		ink := Navy
		if i > stage {
			ink = White
		}
		c.Text(fmt.Sprintf("%d", i+1), x+8, 270, 12, ink)
		c.Text(label, x+29, 269, 11, col)
	}
	label := "ROSTER SLOTS · TURN ORDER NOT YET ASSIGNED"
	if t.DrawVerified {
		label = "ROSTER · SEAT DRAW VERIFIED"
	}
	if t.Stale {
		label = "ROSTER · PREVIOUS CHECKS REQUIRE REVALIDATION"
	}
	c.Text(label, 48, 318, 12, Muted)
	for i, s := range t.Seats {
		r := TableSeatRect(len(t.Seats), i)
		x, y := float64(r.Min.X), float64(r.Min.Y)
		col := teams[i]
		bg := Panel
		if l.Selected == i {
			bg = color.RGBA{24, 48, 65, 255}
		}
		c.Rect(x, y, float64(r.Dx()), float64(r.Dy()), bg)
		c.Rect(x, y, 4, float64(r.Dy()), col)
		title := fmt.Sprintf("ROSTER %02d", i+1)
		if t.DrawVerified {
			title = fmt.Sprintf("PLAYER %02d", i+1)
		}
		c.Text(title, x+17, y+13, 11, Muted)
		showPlayer := s.Joined || s.Ours || s.Admission.Phase != tablelobby.Unknown
		name := "Awaiting player"
		if showPlayer {
			name = shortTableText(s.Name, 23)
			if name == "" {
				name = "Unnamed participant"
			}
		}
		c.Text(name, x+17, y+38, 19, White)
		if s.Ours {
			c.Text("YOU", x+float64(r.Dx())-47, y+13, 11, col)
		}
		if showPlayer {
			stakey(c, x+float64(r.Dx())-42, y+105, 1.45, col, 1, float64(v.Frame)/80, false)
		} else {
			c.Circle(x+float64(r.Dx())-40, y+83, 20, color.RGBA{43, 62, 79, 255})
			c.Line(x+float64(r.Dx())-48, y+83, x+float64(r.Dx())-32, y+83, 2, Muted)
			c.Line(x+float64(r.Dx())-40, y+75, x+float64(r.Dx())-40, y+91, 2, Muted)
		}
		status := s.Status()
		statusCol := tableAmber
		if (!s.Joined || !s.Admission.Complete()) && s.Admission.BondCardLabel() != "" {
			status = s.Admission.BondCardLabel()
		}
		if !s.Joined {
			statusCol = Muted
		}
		if t.Stale {
			status = "RECHECK REQUIRED"
			statusCol = tableAmber
		}
		if s.Status() == "FUNDS VERIFIED" && !t.Stale {
			statusCol = Mint
		}
		c.Text(status, x+17, y+74, 11, statusCol)
		payments := []tablelobby.Payment{s.Admission, s.Stake, s.Bond}
		for j, p := range payments {
			if j == 2 && s.BondOptional {
				continue
			}
			bx := x + 17 + float64(j)*float64(r.Dx()-34)/3
			pc := Muted
			if p.Complete() && !t.Stale {
				pc = Mint
			} else if p.Phase != tablelobby.Unknown {
				pc = tableAmber
			}
			c.Circle(bx+4, y+124, 3, pc)
			c.Text([]string{"ENTRY", "STAKE", "BOND"}[j], bx+13, y+118, 10, pc)
		}
		c.Text("CLICK FOR DETAILS", x+17, y+144, 9, Muted)
	}
	tableButton(c, TableTermsRect, "TABLE TERMS", l.Selected < 0, v)
	tableButton(c, TableSeatTabRect, "PLAYER DETAILS", l.Selected >= 0, v)
	c.Rect(1010, 350, 366, 354, Panel)
	if l.Selected >= 0 && l.Selected < len(t.Seats) {
		drawSeatDetails(c, t, l.Selected)
	} else {
		drawTableTerms(c, t)
	}
	title, body := t.Guidance()
	c.Rect(48, 722, 928, 112, Panel)
	c.Rect(48, 722, 4, 112, tableAmber)
	c.Text("NEXT STEP", 68, 735, 11, tableAmber)
	c.Text(shortTableText(title, 68), 68, 758, 22, White)
	tableLines(c, body, 68, 794, 880, 13, Muted)
	if t.Live {
		label := "WAITING FOR VERIFICATION"
		if t.CanFund {
			label = "F · REQUEST STAKE FUNDING"
		}
		if t.Ready {
			label = "ENTER · JOIN THE BATTLE"
		}
		tableButton(c, TableReviewRect, label, t.CanFund || t.Ready, v)
		if t.Closed {
			tableButton(c, TableRefundRect, "R · REFUND STAKE", false, v)
			tableButton(c, TableBondRefundRect, "B · REFUND ENTRY", false, v)
			c.Text(shortTableText(t.RefundStatus, 155), 48, 884, 11, Muted)
		}
		c.Text("TAB · NEXT TABLE", 540, 852, 11, Muted)
	} else if !t.Reviewed && !t.Demo {
		tableButton(c, TableReviewRect, "MARK TERMS REVIEWED", true, v)
	} else {
		c.Rect(1020, 784, 356, 50, Panel)
		c.Text("JOIN & FUND · UNAVAILABLE", 1036, 802, 14, Muted)
	}
	if t.Live {
		c.Text("Payments need approval in dcrpulse", 1022, 747, 12, Muted)
	} else {
		c.Text("Review only · no payment requested", 1022, 747, 12, Muted)
	}
	if t.Demo {
		c.Text("DEMO ONLY · no wallet or peer activity", 48, 860, 12, tableAmber)
		tableButton(c, TableDemoSizeRect, fmt.Sprintf("%d PLAYERS · CHANGE", t.Invite.Seats), false, v)
		tableButton(c, TableDemoNextRect, "NEXT PREPARATION STATE >", false, v)
	} else if !t.Live {
		status := "BRIDGE OFFLINE"
		col := tableAmber
		if t.Connected {
			status = "BRIDGE CONNECTED"
			col = Mint
		}
		c.Circle(55, 868, 4, col)
		c.Text(status, 68, 861, 11, col)
		chain := "CHAIN HEIGHT UNKNOWN"
		if t.ChainKnown {
			chain = fmt.Sprintf("BLOCK %d", t.Height)
		}
		c.Text(chain, 285, 861, 11, Muted)
		c.Text("SESSION "+t.Invite.SID, 875, 861, 11, Muted)
	}
}
func drawTableTerms(c Canvas, t tablelobby.Table) {
	x := 1030.0
	c.Text("BUY-IN / PLAYER", x, 370, 11, Muted)
	c.Text(tablelobby.DCR(t.Invite.BuyInAtoms)+" DCR", x, 393, 25, Mint)
	c.Text("Match pot: "+tablelobby.DCR(t.Invite.BuyInAtoms*uint64(t.Invite.Seats))+" DCR", x, 432, 13, White)
	_, costs, err := seating.Derive(t.Invite, 0)
	if err != nil {
		tableLines(c, "Unsupported preparation terms: "+err.Error(), x, 468, 323, 13, tableAmber)
		return
	}
	c.Text("COOPERATIVE PAYOUT · OWNER REFUNDS", x, 464, 11, tableAmber)
	for i, row := range []struct {
		name  string
		atoms uint64
	}{
		{"Admission bond", costs.Admission},
	} {
		y := 490 + float64(i)*25
		c.Text(row.name, x, y, 12, Muted)
		c.Text(tablelobby.DCR(row.atoms)+" DCR", x+175, y, 12, White)
	}
	c.Line(x, 566, 1354, 566, 1, color.RGBA{43, 62, 79, 255})
	c.Text("Total / player: "+tablelobby.DCR(costs.Total)+" DCR", x, 580, 13, Mint)
	c.Text("Network fees calculated by dcrpulse", x, 603, 11, Muted)
	c.Text(fmt.Sprintf("Stake refund: %d blocks · bonds: %d", t.Invite.CSVBlocks, t.Invite.AdmissionBlocks), x, 631, 12, White)
	c.Text(fmt.Sprintf("Admission closes at block %d", t.Invite.Until), x, 655, 12, White)
	c.Text("Winner takes pot · draw returns equal shares", x, 682, 11, Muted)
	c.Text("Exact payout fee shown before approval", x, 702, 11, Muted)

}
func drawSeatDetails(c Canvas, t tablelobby.Table, index int) {
	s := t.Seats[index]
	x := 1030.0
	name := s.Name
	if name == "" {
		name = "Awaiting a verified participant"
	}
	c.Text(shortTableText(name, 28), x, 369, 18, White)
	status := "Identity not established"
	if s.IdentityVerified {
		status = "Session identity verified"
	}
	if t.Stale {
		status = "Previously checked · now stale"
	}
	c.Text(status, x, 399, 12, Muted)
	payments := []tablelobby.Payment{s.Admission, s.Stake, s.Bond}
	for j, p := range payments {
		if j == 2 && s.BondOptional {
			continue
		}
		y := 438 + float64(j)*78
		c.Text([]string{"01  ADMISSION BOND", "02  MATCH STAKE", "03  TABLE BOND"}[j], x, y, 12, White)
		label := p.Label()
		if t.Stale {
			label = "Recheck required after reconnect"
		}
		c.Text(label, x, y+23, 12, tableAmber)
		c.Rect(x, y+47, 321, 3, Navy)
		progress := 0.0
		if p.Checked && p.Required > 0 && !t.Stale {
			progress = math.Min(1, float64(p.Confirmations)/float64(p.Required))
		}
		c.Rect(x, y+47, 321*progress, 3, Mint)
	}
}
func drawTableHelp(c Canvas, v View) {
	c.Rect(0, 0, Width, Height, color.RGBA{3, 9, 20, 235})
	c.Rect(252, 212, 936, 486, Panel)
	c.Text("CREATE & JOIN THROUGH DCRPULSE", 282, 243, 25, White)
	tableButton(c, TableHelpCloseRect, "X", false, v)
	items := [][2]string{
		{"01  CONNECT STAKEWARS", "Use the lobby cogwheel to connect the certificate-based gaming bridge."},
		{"02  CREATE IN DCRPULSE / GAMING", "Choose StakeWars, the group chat, 2–6 players, the buy-in and admission window."},
		{"03  INVITATION APPEARS IN CHAT", "Dcrpulse asks the creator's game to join before posting the invitation chip."},
		{"04  OTHER PLAYERS CLICK ACCEPT", "The bridge delivers the terms and group directly. This room opens automatically."},
	}
	for i, item := range items {
		y := 306 + float64(i)*73
		c.Text(item[0], 282, y, 15, Mint)
		c.Text(item[1], 282, y+28, 14, White)
	}
	tableLines(c, "Current build: table requests open this room, but paid admission is refused. Dcrpulse will not publish a new table until creator admission succeeds.", 282, 614, 850, 14, tableAmber)
}
