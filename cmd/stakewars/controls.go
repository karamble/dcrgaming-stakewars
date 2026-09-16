//go:build desktop

package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/karamble/dcrstakewars/pkg/render"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

var controlActions = []string{"left", "right", "aimUp", "aimDown", "jump", "highJump", "fire", "precision", "previous", "next", "rope", "teleport", "skip"}
var controlNames = []string{"Walk left", "Walk right", "Aim up / reel in / jetpack lift", "Aim down / lengthen attached rope", "Jump / precision: backflip", "High jump", "Fire / use tool", "Precision modifier", "Previous weapon", "Next weapon", "Rope shortcut", "Teleport shortcut", "End turn"}

func defaultControls() map[string]ebiten.Key {
	return map[string]ebiten.Key{"left": ebiten.KeyA, "right": ebiten.KeyD, "aimUp": ebiten.KeyW, "aimDown": ebiten.KeyS, "jump": ebiten.KeyJ, "highJump": ebiten.KeyBackspace, "fire": ebiten.KeySpace, "precision": ebiten.KeyShiftLeft, "previous": ebiten.KeyQ, "next": ebiten.KeyE, "rope": ebiten.KeyR, "teleport": ebiten.KeyT, "skip": ebiten.KeyEnter}
}
func reservedKey(k ebiten.Key) bool {
	switch k {
	case ebiten.KeyM, ebiten.KeyEscape, ebiten.KeyK, ebiten.KeyP, ebiten.KeyH, ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyC, ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyPageUp, ebiten.KeyPageDown, ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5:
		return true
	}
	return false
}
func (g *game) loadControls(path string) error {
	g.keys = defaultControls()
	g.controlsPath = path
	g.bindingIndex = -1
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var loaded map[string]ebiten.Key
	if err = json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	for action, key := range loaded {
		// Older settings had separate arrow bindings for rope and jetpack.
		if action == "lower" || action == "rise" {
			continue
		}
		if _, ok := g.keys[action]; !ok {
			return fmt.Errorf("unknown control %q", action)
		}
		g.keys[action] = key
	}
	used := map[ebiten.Key]bool{}
	for _, key := range g.keys {
		if reservedKey(key) || used[key] {
			return fmt.Errorf("reserved or duplicated key %s", key)
		}
		used[key] = true
	}
	return nil
}
func (g *game) pressed(action string) bool     { return ebiten.IsKeyPressed(g.keys[action]) }
func (g *game) justPressed(action string) bool { return inpututil.IsKeyJustPressed(g.keys[action]) }
func (g *game) controlLabels() map[string]string {
	key := func(a string) string { return strings.ToUpper(g.keys[a].String()) }
	return map[string]string{"rise": key("aimUp"), "liftHint": "Hold " + key("aimUp"), "ropeLength": key("aimUp") + " / " + key("aimDown"), "fire": key("fire"), "precision": key("precision"), "skip": key("skip"), "move": key("left") + " / " + key("right"), "aim": key("aimUp") + " / " + key("aimDown"), "cycle": key("previous") + " / " + key("next"), "jump": key("jump") + " jump", "jumpSuffix": "+" + key("jump")}
}
func (g *game) controlRows() [][2]string {
	var rows [][2]string
	for i, a := range controlActions {
		rows = append(rows, [2]string{controlNames[i], g.keys[a].String()})
	}
	return rows
}
func (g *game) updateControls() {
	if g.bindingIndex >= 0 {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.bindingIndex = -1
			return
		}
		for _, key := range inpututil.AppendJustPressedKeys(nil) {
			if reservedKey(key) {
				g.controlsMessage = "That key is reserved for camera, menus or fuse selection."
				return
			}
			action := controlActions[g.bindingIndex]
			old := g.keys[action]
			for a, k := range g.keys {
				if a != action && k == key {
					g.keys[a] = old
				}
			}
			g.keys[action] = key
			g.bindingIndex = -1
			data, err := json.MarshalIndent(g.keys, "", "  ")
			if err == nil {
				err = os.MkdirAll(filepath.Dir(g.controlsPath), 0700)
			}
			if err == nil {
				err = os.WriteFile(g.controlsPath, data, 0600)
			}
			g.controlsMessage = "Saved. Conflicting actions exchange keys."
			if err != nil {
				g.controlsMessage = "Active for this session; save failed: " + err.Error()
			}
			return
		}
	} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		for i := range controlActions {
			box := render.ControlRect(i)
			if g.settingsOpen {
				box = render.SettingsControlRect(i)
			}
			if image.Pt(x, y).In(box) {
				g.bindingIndex = i
				break
			}
		}
	}
}

func (g *game) cycleWeapon(direction int) sim.Weapon {
	if g.s.Shots > 0 {
		return g.s.Weapon
	}
	for n := 1; n <= int(sim.WeaponCount); n++ {
		w := sim.Weapon((int(g.s.Weapon) + int(sim.WeaponCount) + direction*n) % int(sim.WeaponCount))
		if g.s.WeaponAvailable(w) {
			return w
		}
	}
	return g.s.Weapon
}
