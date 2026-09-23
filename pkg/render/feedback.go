package render

import (
	"fmt"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
	"image/color"
)

// WeaponUnavailable describes selection restrictions, not a change to the
// deterministic inventory. The selected remote charge and second shotgun shot
// remain usable while choosing a different weapon is locked.
func WeaponUnavailable(s *sim.State, w sim.Weapon) string {
	if s.Phase != sim.Playing && s.Phase != sim.Ready {
		return "TURN LOCKED"
	}
	if s.Shots > 0 {
		if w != s.Weapon {
			return "ATTACK USED"
		}
		return ""
	}
	if s.AmmoCount(w) == 0 {
		return "EMPTY"
	}
	if rounds := s.UnlockTurns(w); rounds > 0 {
		return fmt.Sprintf("WAIT %dR", rounds)
	}
	return ""
}
func (r *Scene) turnBanner(c Canvas, s *sim.State, v View) {
	if v.Paused || v.ControlsOpen || v.WeaponMenu {
		return
	}
	label := ""
	col := teams[s.ActiveSeat()]
	switch s.Phase {
	case sim.Ready:
		label = fmt.Sprintf("SQUAD %d · GET READY · %d", s.ActiveSeat()+1, (s.Config.ReadyTicks-s.PhaseTick+59)/60)
	case sim.Retreating:
		if s.PhaseTick < 60 {
			label = "RETREAT · MOVE TO COVER"
		}
	case sim.Playing:
		left := s.RemainingTicks()
		if left > 0 && left <= 5*60 {
			label = fmt.Sprintf("%d SECONDS · FINISH YOUR TURN", (left+59)/60)
			col = color.RGBA{255, 173, 81, 255}
		}
	case sim.Ended:
		label = "DRAW"
		if s.Winner >= 0 {
			label = fmt.Sprintf("SQUAD %d WINS", s.Winner+1)
		}
	}
	if label == "" {
		return
	}
	stage := Stage(v.HideTop, v.HideBottom)
	x, y := float64(stage.Min.X+stage.Dx()/2-230), float64(stage.Min.Y+12)
	if !v.HideTop {
		y = 112
	}
	c.Rect(x, y, 460, 36, hudGlass)
	c.Rect(x, y, 4, 36, col)
	c.Text(label, x+16, y+8, 18, col)
}
