package sim

import (
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

// dig removes only nearby terrain. It never damages units, spends the attack,
// moves a unit through solid ground, or resets the turn clock. The cadence is
// simulation time, independent of frame rate and the sender's input frequency.
func (s *State) dig(effects *[]Effect) {
	if s.Shots != 0 || s.Tick%6 != 0 {
		return
	}
	w := s.Worms[s.Active]
	x, y := w.Pos.X.Int(), w.Pos.Y.Int()
	// Clear the occupied body even when completely entombed, preserving the
	// supporting floor unless the drill is aimed downward.
	body := terrain.Rect{X0: x - halfWidth - 1, Y0: y - bodyHeight - 1, X1: x + halfWidth + 2, Y1: y}
	s.Terrain.StampRect(body, false)
	origin := w.Pos.Sub(fixed.Vec{Y: fixed.FromInt(13)})
	dir := fixed.Vec{X: fixed.Cos(s.Aim), Y: fixed.Sin(s.Aim)}
	// Overlapping cuts make a body-sized continuous passage in any direction.
	*effects = append(*effects, Effect{Dirty: body})
	for distance := 6; distance <= 18; distance += 6 {
		p := origin.Add(dir.Scale(fixed.FromInt(distance)))
		dirty := s.Terrain.StampCircle(p.X.Int(), p.Y.Int(), 12, false)
		*effects = append(*effects, Effect{Dirty: dirty})
	}
	*effects = append(*effects, Effect{Fired: true, Unit: s.Active, Weapon: MiningDrill, Pos: origin})
}
