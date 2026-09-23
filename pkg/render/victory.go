package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
)

var VictoryAgainRect = image.Rect(80, 777, 390, 835)
var VictoryLobbyRect = image.Rect(410, 777, 720, 835)

func drawVictory(c Canvas, s *sim.State, v View) {
	c.Rect(0, 0, Width, Height, color.RGBA{5, 12, 27, 240})
	accent := Mint
	title, subtitle := "VICTORY", "THE LAST SQUAD STANDING."
	if s.Winner < 0 {
		title, subtitle = "DRAW", "NO ONE LEFT. NOTHING HELD BACK."
	} else {
		accent = teams[s.Winner]
	}
	t := float64(v.Frame) / 60
	// Orbiting light rays and drifting comic confetti stay entirely local.
	for i := 0; i < 18; i++ {
		a := float64(i)*math.Pi/9 + t*.035
		col := accent
		col.A = 14
		c.Poly([]Point{{1060, 350}, {1060 + 680*math.Cos(a), 350 + 680*math.Sin(a)}, {1060 + 680*math.Cos(a+.085), 350 + 680*math.Sin(a+.085)}}, col)
	}
	for i := 0; i < 65; i++ {
		x := repeat(float64(i*173)+18*math.Sin(t+float64(i)), Width)
		y := repeat(float64(i*89)+t*float64(18+i%23), Height)
		col := accent
		if i%3 == 0 {
			col = Blue
		}
		if i%7 == 0 {
			col = White
		}
		col.A = 150
		c.Line(x, y, x+5*math.Cos(t+float64(i)), y+9, 3, col)
	}
	brand(c)
	c.Text("BATTLE REPORT / MATCH COMPLETE", 80, 148, 15, accent)
	c.Text(title, 74, 192, 94, White)
	c.Rect(80, 309, 96, 5, accent)
	if s.Winner >= 0 {
		c.Text(fmt.Sprintf("SQUAD %02d", s.Winner+1), 80, 344, 44, accent)
	} else {
		c.Text("A BATTLE TO REMEMBER", 80, 344, 29, accent)
	}
	c.Text(subtitle, 80, 410, 18, White)
	c.Text(fmt.Sprintf("%d TURNS   /   %02d:%02d BATTLE TIME", s.Turn+1, s.Tick/3600, (s.Tick/60)%60), 80, 454, 16, Muted)
	c.Text("Every crater tells a story.", 80, 496, 18, Muted)
	c.Circle(1060, 345, 193, color.RGBA{39, 211, 162, 15})
	c.Circle(1060, 345, 154, color.RGBA{39, 211, 162, 17})
	c.Poly([]Point{{856, 526}, {930, 486}, {1190, 486}, {1264, 526}, {1222, 558}, {898, 558}}, Panel)
	c.Line(856, 526, 1264, 526, 3, accent)
	if s.Winner >= 0 {
		stakey(c, 1050, 495+3*math.Sin(t*3), 6.5, accent, 1, t, true)
		gold := color.RGBA{255, 207, 100, 255}
		c.Poly([]Point{{993, 271}, {981, 225}, {1018, 242}, {1049, 209}, {1080, 242}, {1117, 225}, {1105, 271}}, gold)
		c.Rect(993, 271, 112, 7, White)
	} else {
		stakey(c, 988, 495, 4.4, Mint, 1, t, false)
		stakey(c, 1145, 495, 4.4, Blue, -1, t, false)
	}
	c.Text("SQUAD RESULTS", 80, 576, 13, Muted)
	for seat := 0; seat < int(s.Config.Seats); seat++ {
		alive, hp := 0, int32(0)
		for _, w := range s.Worms {
			if int(w.Seat) == seat && w.HP > 0 {
				alive++
				hp += w.HP
			}
		}
		x := 80 + float64(seat)*214
		c.Rect(x, 608, 198, 127, Panel)
		c.Rect(x, 608, 198, 3, teams[seat])
		label := fmt.Sprintf("SQUAD %02d", seat+1)
		if int(s.Winner) == seat {
			label += " / WINNER"
		}
		c.Text(label, x+13, 626, 13, teams[seat])
		c.Text(fmt.Sprintf("%d", alive), x+13, 653, 32, White)
		c.Text("SURVIVORS", x+50, 669, 11, Muted)
		c.Text(fmt.Sprintf("%d HP REMAINING", hp), x+13, 709, 11, Muted)
	}
	button := func(r image.Rectangle, label string, primary bool) {
		col := Panel
		if primary {
			col = accent
		}
		if image.Pt(v.MouseX, v.MouseY).In(r) {
			col = Blue
		}
		c.Rect(float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()), col)
		ink := White
		if primary {
			ink = Navy
		}
		c.Text(label, float64(r.Min.X+20), float64(r.Min.Y+20), 15, ink)
	}
	if v.Dev {
		button(VictoryAgainRect, "PLAY AGAIN  /  ENTER", true)
	}
	button(VictoryLobbyRect, "BACK TO LOBBY  /  ESC", false)
	if v.Network {
		tableLines(c, v.Settlement, 944, 776, 440, 13, Mint)
	} else {
		c.Text("LOCAL MATCH RESULT · NO PAYOUT", 944, 800, 13, Muted)
	}
}

func drawCompactPower(c Canvas, s *sim.State, v View) {
	if s.Phase != sim.Playing && s.Phase != sim.Ready {
		return
	}
	if !sim.Charged(s.Weapon) {
		return
	}
	stage := Stage(v.HideTop, v.HideBottom)
	x, y := float64(stage.Min.X+stage.Dx()/2-180), float64(stage.Max.Y-83)
	c.Rect(x, y, 360, 66, Panel)
	label := "SHIFT · PRECISION"
	col := Muted
	if v.Precision {
		label = "SHIFT · PRECISION ON"
		col = Mint
	}
	c.Text(fmt.Sprintf("POWER %d%%", v.Power/10), x+14, y+12, 14, White)
	c.Text(controlText(label, v), x+155, y+13, 12, col)
	c.Rect(x+14, y+41, 332, 9, Navy)
	c.Rect(x+14, y+41, 332*math.Min(1, float64(v.Power)/1000), 9, Mint)
}
