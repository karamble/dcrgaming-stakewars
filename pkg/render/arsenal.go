package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/karamble/dcrstakewars/assets/weapons"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

var hudGlass = color.RGBA{10, 22, 39, 174}

const TraySlots = 8
const WeaponPages = (int(sim.WeaponCount) + TraySlots - 1) / TraySlots

var TrayPrev = image.Rect(1276, 780, 1332, 836)
var TrayNext = image.Rect(1340, 780, 1396, 836)

func MenuOrigin(x, y int) image.Point {
	return image.Pt(max(12, min(x, Width-756)), max(12, min(y, Height-450)))
}
func MenuRect(origin image.Point) image.Rectangle {
	return image.Rect(origin.X, origin.Y, origin.X+744, origin.Y+438)
}
func MenuWeaponRect(origin image.Point, w sim.Weapon) image.Rectangle {
	x, y := origin.X+12+int(w)%6*120, origin.Y+46+int(w)/6*96
	return image.Rect(x, y, x+112, y+88)
}
func MenuWeaponAt(origin image.Point, x, y int) (sim.Weapon, bool) {
	for w := sim.Weapon(0); w < sim.WeaponCount; w++ {
		if image.Pt(x, y).In(MenuWeaponRect(origin, w)) {
			return w, true
		}
	}
	return 0, false
}
func weaponTile(c Canvas, s *sim.State, w sim.Weapon, b image.Rectangle, v View) {
	x, y, width := float64(b.Min.X), float64(b.Min.Y), float64(b.Dx())
	bg, fg := hudGlass, White
	if image.Pt(v.MouseX, v.MouseY).In(b) {
		bg = color.RGBA{31, 55, 75, 225}
	}
	if WeaponUnavailable(s, w) != "" {
		fg = Muted
	}
	if w == s.Weapon {
		bg = color.RGBA{22, 71, 77, 220}
	}
	c.Rect(x, y, width, float64(b.Dy()), bg)
	if w == s.Weapon {
		c.Rect(x, y, width, 3, Mint)
	}
	sprite := weapons.Sprite(int(w))
	bounds := sprite.Bounds()
	scale := math.Min((width-24)/float64(bounds.Dx()), 52/float64(bounds.Dy()))
	sw, sh := float64(bounds.Dx())*scale, float64(bounds.Dy())*scale
	c.Image(sprite, x+(width-sw)/2, y+7+(52-sh)/2, sw, sh)
	if reason := WeaponUnavailable(s, w); reason != "" {
		c.Rect(x+3, y+3, width-6, 15, Navy)
		c.Text(reason, x+5, y+4, 10, Muted)
	}
	name := WeaponName(w)
	size := math.Min(12, (width-12)/math.Max(1, float64(len(name)))*1.6)
	c.Text(name, x+6, y+66, size, fg)
	if n := s.AmmoCount(w); n != sim.Unlimited {
		c.Text(fmt.Sprintf("%d", n), x+width-17, y+5, 12, fg)
	}
}
func drawWeaponMenu(c Canvas, s *sim.State, v View) {
	b := MenuRect(v.MenuOrigin)
	c.Rect(float64(b.Min.X), float64(b.Min.Y), float64(b.Dx()), float64(b.Dy()), color.RGBA{8, 18, 34, 252})
	title := "ARSENAL · CLICK TO EQUIP · RIGHT-CLICK TO CLOSE"
	if s.Shots > 0 {
		title = "ARSENAL · FINISH CURRENT WEAPON FIRST"
	}
	c.Text(title, float64(b.Min.X+16), float64(b.Min.Y+14), 15, Mint)
	for w := sim.Weapon(0); w < sim.WeaponCount; w++ {
		weaponTile(c, s, w, MenuWeaponRect(v.MenuOrigin, w), v)
	}
}
func controlText(text string, v View) string {
	if v.KeyLabels == nil {
		return text
	}
	replacements := []string{}
	for _, p := range [][2]string{{"Hold W", "liftHint"}, {"SPACE", "fire"}, {"SHIFT", "precision"}, {"ENTER", "skip"}, {"A / D", "move"}, {"W / S", "aim"}, {"Q / E", "cycle"}, {"J jump", "jump"}, {"+J", "jumpSuffix"}} {
		if label, ok := v.KeyLabels[p[1]]; ok {
			replacements = append(replacements, p[0], label)
		}
	}
	return strings.NewReplacer(replacements...).Replace(text)
}
func ControlRect(index int) image.Rectangle {
	x, y := 250+(index/10)*480, 190+(index%10)*51
	return image.Rect(x, y, x+450, y+44)
}
func drawControls(c Canvas, v View) {
	c.Rect(225, 100, 990, 670, color.RGBA{8, 18, 34, 253})
	c.Text("CONTROLS", 250, 122, 28, Mint)
	c.Text("Click an action, then press its new key. ESC cancels; K closes.", 250, 160, 15, White)
	for i, row := range v.ControlRows {
		b := ControlRect(i)
		col := Panel
		if i == v.BindingIndex {
			col = color.RGBA{22, 71, 77, 220}
		}
		c.Rect(float64(b.Min.X), float64(b.Min.Y), 450, 44, col)
		c.Text(row[0], float64(b.Min.X+12), float64(b.Min.Y+10), 15, White)
		key := row[1]
		if i == v.BindingIndex {
			key = "PRESS KEY"
		}
		c.Text(key, float64(b.Min.X+280), float64(b.Min.Y+10), 14, Mint)
	}
	c.Text("Precision + jump = backflip. Keys 1–5 set grenade fuse.", 250, 712, 15, White)
	c.Text(v.ControlsMessage, 250, 742, 13, Muted)
}
