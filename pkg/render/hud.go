package render

import (
	"fmt"
	"github.com/karamble/dcrstakewars/assets/weapons"
	"github.com/karamble/dcrstakewars/pkg/sim"
	"image"
	"image/color"
	"math"
)

func WeaponRect(w sim.Weapon) image.Rectangle {
	x := 40 + (int(w)%TraySlots)*154
	return image.Rect(x, 780, x+146, 864)
}
func WeaponAt(x, y int, pages ...int) (sim.Weapon, bool) {
	page := 0
	if len(pages) > 0 {
		page = pages[0]
	}
	for w := sim.Weapon(page * TraySlots); w < sim.WeaponCount && int(w) < (page+1)*TraySlots; w++ {
		if image.Pt(x, y).In(WeaponRect(w)) {
			return w, true
		}
	}
	return 0, false
}

var TopToggleRect = image.Rect(1398, 8, 1432, 40)
var BottomToggleRect = image.Rect(1398, 860, 1432, 892)

var LobbyResumeRect = image.Rect(65, 675, 456, 731)
var LobbyQuitRect = image.Rect(800, 675, 950, 731)

var ResumeRect = image.Rect(520, 464, 920, 518)
var LeaveRect = image.Rect(520, 532, 920, 586)

func WeaponHint(w sim.Weapon) string {
	switch w {
	case sim.HomingRocket, sim.Airstrike:
		return "Click to mark target   ·   SPACE fire"
	case sim.MiningDrill:
		return "Hold SPACE to dig   ·   W / S aim   ·   A / D walk out"
	case sim.Girder:
		return "Click a clear site within 240 px   ·   SPACE build"
	case sim.BisonBomb:
		return "SPACE release bison   ·   Runs until impact or timeout"
	case sim.RemoteCharge:
		return "SPACE place   ·   SPACE again to detonate   ·   Auto at timeout"
	case sim.Parachute:
		return "SPACE open / close canopy   ·   A / D steer"
	case sim.Teleport:
		return "Click a clear destination on the battlefield."
	case sim.NinjaRope:
		return "SPACE attach / detach   ·   W / S change length   ·   A / D swing"
	case sim.Jetpack:
		return "Hold W to rise   ·   A / D steer   ·   Choose a weapon when ready"
	case sim.Shotgun:
		return "SPACE fire   ·   Two shots per turn"
	case sim.Uzi, sim.FirePunch, sim.BaseballBat, sim.Flamethrower:
		return "SPACE attack in the aiming direction"
	case sim.Mine:
		return "SPACE place proximity mine   ·   Arms after 2s; keep clear"
	case sim.Dynamite:
		return "SPACE place explosive   ·   Move away during retreat"
	default:
		return "Hold SPACE to charge, release to fire   ·   W / S aim"
	}
}
func (r *Scene) hud(c Canvas, s *sim.State, v View) {
	col := teams[s.ActiveSeat()]
	if !v.HideTop {
		c.Rect(40, 14, 650, 30, hudGlass)
		c.Text(fmt.Sprintf("TURN %02d   ·   SQUAD %d ACTIVE", s.Turn+1, s.ActiveSeat()+1), 54, 20, 14, col)
		wind := float64(s.Wind) / 65536
		arrow := "→"
		if wind < 0 {
			arrow = "←"
		}
		c.Text(fmt.Sprintf("WIND %s %.1f", arrow, math.Abs(wind)), 470, 20, 13, Mint)
		if v.Camera != nil {
			c.Rect(1020, 14, 370, 30, hudGlass)
			c.Text("MAP", 1030, 21, 10, Muted)
			for _, unit := range s.Worms {
				if unit.HP > 0 {
					c.Circle(1020+float64(unit.Pos.X)/65536/float64(s.Config.Width)*370, 29, 3, teams[unit.Seat])
				}
			}
			left := 1020 + v.Camera.X/float64(s.Config.Width)*370
			width := math.Min(370, float64(v.Camera.Viewport().Dx())/v.Camera.Zoom/float64(s.Config.Width)*370)
			c.Rect(left, 15, width, 28, color.RGBA{79, 168, 178, 45})
			c.Line(left, 15, left+width, 15, 2, Mint)
			c.Line(left, 42, left+width, 42, 2, Mint)
		}
		for seat := uint8(0); seat < s.Config.Seats; seat++ {
			x := 40 + float64(seat)*float64(1360)/float64(s.Config.Seats)
			width := float64(1360)/float64(s.Config.Seats) - 10
			hp, alive := int32(0), 0
			for _, w := range s.Worms {
				if w.Seat == seat && w.HP > 0 {
					hp += w.HP
					alive++
				}
			}
			c.Rect(x, 52, width, 46, hudGlass)
			if seat == s.ActiveSeat() {
				c.Rect(x, 52, width, 3, teams[seat])
			}
			c.Text(fmt.Sprintf("%02d  SQUAD %d", seat+1, seat+1), x+12, 58, 14, teams[seat])
			c.Text(fmt.Sprintf("%d alive  /  %d HP", alive, hp), x+12, 78, 12, White)
		}
	}
	if !v.HideBottom {
		c.Rect(40, 710, 1360, 65, hudGlass)
		c.Rect(40, 710, 4, 65, col)
		phase := fmt.Sprintf("SQUAD %d · MOVE & AIM", s.ActiveSeat()+1)
		hint := controlText(WeaponHint(s.Weapon), v)
		if sim.UsesTarget(s.Weapon) && !s.HasTarget {
			hint = "MARK A TARGET · Click the battlefield first"
		}
		if s.Weapon == sim.Girder && s.HasTarget && !s.CanPlaceGirder() {
			hint = "CANNOT BUILD HERE · Choose a clear site within 240 pixels"
		}
		if sim.UsesFuse(s.Weapon) {
			hint += fmt.Sprintf("   ·   FUSE %ds [1–5]", s.FuseSeconds)
		}
		ticks := s.RemainingTicks()
		if s.Phase == sim.Ready {
			phase = "GET READY"
			hint = "Move, aim or use a weapon to start early"
			ticks = s.Config.ReadyTicks - s.PhaseTick
		}
		if s.Phase == sim.Retreating {
			phase = "RETREAT · MOVE TO COVER"
			hint = controlText("A / D move   ·   J jump", v)
			if s.PhaseTick < s.Config.RetreatTicks {
				ticks = s.Config.RetreatTicks - s.PhaseTick
			}
		}
		if s.Phase == sim.Retreating && s.Worms[s.Active].HP <= 0 {
			phase = "STAKEY LOST · TURN ENDING"
			hint = "Your remaining squad members stay in the match."
		}
		if s.Phase == sim.Resolving {
			phase = "WAITING FOR THE BATTLEFIELD"
			hint = "Explosions and movement are resolving."
		}
		if s.Phase == sim.Ended {
			phase = "MATCH COMPLETE"
			hint = "Press ENTER to restart this internal fixture."
			if s.Winner < 0 {
				phase = "DRAW"
			} else {
				phase = fmt.Sprintf("SQUAD %d WINS", s.Winner+1)
			}
		}
		if ticks > 0 && ticks <= 600 && s.Phase == sim.Playing {
			col = color.RGBA{255, 173, 81, 255}
		}
		c.Text(phase, 60, 718, 20, col)
		c.Text(hint, 60, 749, 12, White)
		if s.Phase == sim.Playing || s.Phase == sim.Retreating || s.Phase == sim.Ready {
			c.Text(fmt.Sprintf("%02d", (ticks+59)/60), 1310, 713, 38, col)
		}
		if s.Phase == sim.Playing && sim.Charged(s.Weapon) {
			c.Text(fmt.Sprintf("POWER %d%%", v.Power/10), 1000, 720, 13, Muted)
			c.Rect(1000, 746, 245, 6, Navy)
			c.Rect(1000, 746, 245*float64(v.Power)/1000, 6, col)
		}
		for w := sim.Weapon(v.WeaponPage * TraySlots); w < sim.WeaponCount && int(w) < (v.WeaponPage+1)*TraySlots; w++ {
			weaponTile(c, s, w, WeaponRect(w), v)
		}
		for i, box := range []image.Rectangle{TrayPrev, TrayNext} {
			c.Rect(float64(box.Min.X), float64(box.Min.Y), float64(box.Dx()), float64(box.Dy()), hudGlass)
			label := "<"
			if i == 1 {
				label = ">"
			}
			c.Text(label, float64(box.Min.X+20), float64(box.Min.Y+20), 24, Mint)
		}
		c.Text(fmt.Sprintf("%d / %d", v.WeaponPage+1, WeaponPages), 1285, 846, 14, White)
		c.Rect(40, 870, 1348, 24, hudGlass)
		c.Text(controlText("A / D move   ·   J jump   ·   SHIFT precision   ·   Right-click arsenal   ·   K controls", v), 52, 875, 12, White)
		c.Text("C follow   ·   H hide HUD   ·   M sound", 1020, 875, 11, Muted)
		weaponPreview(c, s, v)
	}
	r.turnBanner(c, s, v)
	panelToggles(c, s, v)
	if v.Muted && !v.HideBottom {
		c.Text("MUTED", 930, 875, 10, Muted)
	}
	if v.WeaponMenu {
		drawWeaponMenu(c, s, v)
	}
	if v.ControlsOpen {
		drawControls(c, v)
	}
	if v.Paused {
		c.Rect(0, 0, Width, Height, color.RGBA{5, 12, 26, 220})
		c.Rect(480, 275, 480, 350, Panel)
		c.Text("TAKE A BREATHER", 520, 307, 30, White)
		label := "The internal fixture is paused."
		if v.Network {
			label = "The multiplayer turn timer keeps running."
		}
		c.Text(label, 520, 359, 17, Muted)
		c.Text("ESC or P to return to the battlefield.", 520, 396, 16, Muted)
		c.Rect(520, 464, 400, 54, Mint)
		c.Text("RESUME", 670, 478, 18, Navy)
		c.Rect(520, 532, 400, 54, color.RGBA{31, 47, 69, 255})
		c.Text("BACK TO LOBBY", 638, 546, 18, White)
	}
}

func weaponPreview(c Canvas, s *sim.State, v View) {
	w, ok := WeaponAt(v.MouseX, v.MouseY, v.WeaponPage)
	if !ok || v.Paused || v.WeaponMenu || v.ControlsOpen {
		return
	}
	descriptions := [][2]string{
		{"Wind affects its flight.", "Explodes on contact."},
		{"Bounces before exploding.", "Choose fuse with keys 1–5."},
		{"Bursts into five bomblets.", "Keep your distance."},
		{"Two direct shots per turn.", "Move between your shots."},
		{"A short burst of seven shots.", "Aim before firing."},
		{"A close-range strike.", "Get beside your target."},
		{"Place it, then move to cover.", "Four-second fuse."},
		{"Arms two seconds after placement.", "Proximity trigger; half-second warning."},
		{"Click a clear landing spot.", "Retreat starts on arrival."},
		{"Hook terrain to swing across it.", "Adjust length with W / S."},
		{"Hold W for lift; A / D to steer.", "Switch tools when ready."},
		{"Fast shell for high-angle shots.", "Explodes on contact."},
		{"Strong knockback at close range.", "Send enemies into the hazard."},
		{"High rebound for cave shots.", "Choose fuse with keys 1–5."},
		{"Bores through a bounded depth.", "Explodes after penetration."},
		{"Open canopy to slow descent.", "Steer with movement keys."},
		{"Mark a target, then launch.", "Guidance starts after launch."},
		{"Mark the ground, then fire.", "Five bombs descend from above."},
		{"A short cone of burning fuel.", "Keep clear of the flames."},
		{"Place, move, then detonate.", "Detonates on turn timeout too."},
		{"Nine bomblets; one per squad.", "Choose fuse with keys 1–5."},
		{"Runs over small terrain steps.", "Explodes on impact or timeout."},
		{"Build a horizontal bridge.", "Clear area within 240 pixels."},
		{"Hold SPACE to clear a passage.", "No direct damage; keeps your attack."},
	}
	x := math.Min(float64(WeaponRect(w).Min.X), 1100)
	y := float64(454)
	c.Rect(x, y, 300, 216, color.RGBA{12, 23, 43, 252})
	c.Rect(x, y, 300, 3, Mint)
	c.Text(WeaponName(w), x+16, y+14, 18, White)
	sprite := weapons.Sprite(int(w))
	bounds := sprite.Bounds()
	scale := math.Min(160/float64(bounds.Dx()), 100/float64(bounds.Dy()))
	sw, sh := float64(bounds.Dx())*scale, float64(bounds.Dy())*scale
	c.Image(sprite, x+(300-sw)/2, y+43+(100-sh)/2, sw, sh)
	c.Text(descriptions[w][0], x+16, y+149, 14, White)
	c.Text(descriptions[w][1], x+16, y+170, 14, Muted)
	action := "CLICK TO EQUIP"
	if w == s.Weapon {
		action = "EQUIPPED"
	} else if s.UnlockTurns(w) > 0 {
		action = fmt.Sprintf("UNLOCKS IN %d ROUND(S)", s.UnlockTurns(w))
	} else if s.Phase != sim.Playing && s.Phase != sim.Ready {
		action = "AVAILABLE NEXT TURN"
	} else if s.AmmoCount(w) == 0 {
		action = "OUT OF AMMO"
	} else if s.Shots > 0 {
		action = "FINISH CURRENT WEAPON"
	}
	c.Text(action, x+16, y+195, 11, Mint)
}

func panelToggles(c Canvas, s *sim.State, v View) {
	for i, box := range []image.Rectangle{TopToggleRect, BottomToggleRect} {
		hidden := v.HideTop
		if i == 1 {
			hidden = v.HideBottom
		}
		x, y := float64(box.Min.X), float64(box.Min.Y)
		c.Rect(x, y, float64(box.Dx()), float64(box.Dy()), hudGlass)
		direction := -1.0
		if i == 1 {
			direction = 1
		}
		if hidden {
			direction = -direction
		}
		c.Line(x+9, y+16-direction*3, x+17, y+16+direction*3, 2, Mint)
		c.Line(x+17, y+16+direction*3, x+25, y+16-direction*3, 2, Mint)
	}
	if v.HideBottom {
		b := Stage(v.HideTop, v.HideBottom)
		c.Rect(float64(b.Min.X+8), float64(b.Max.Y-38), 340, 30, Panel)
		text := fmt.Sprintf("SQUAD %d   %s   %ds", s.ActiveSeat()+1, WeaponName(s.Weapon), (s.RemainingTicks()+59)/60)
		if s.Phase == sim.Ready {
			text = fmt.Sprintf("GET READY · %ds · Act to start", (s.Config.ReadyTicks-s.PhaseTick+59)/60)
		} else if s.Phase != sim.Playing {
			text = "TURN RESOLVING  ·  H TO SHOW CONTROLS"
		}
		c.Text(text, float64(b.Min.X+18), float64(b.Max.Y-33), 13, White)
	}
}
