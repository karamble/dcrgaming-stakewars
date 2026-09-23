package render

import (
	"math"

	"github.com/karamble/dcrgaming-stakewars/assets/objects"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
)

// Markers have no collision or simulation state. Reconstruct their position
// from the dead unit and current terrain, including when loading a replay.
func gravePosition(s *sim.State, i int) (float64, float64, bool) {
	w := s.Worms[i]
	x, y := w.Pos.X.Int(), w.Pos.Y.Int()
	if w.HP > 0 || x < 0 || x >= int(s.Config.Width) || y < 0 || y >= s.WaterY.Int() {
		return 0, 0, false
	}
	for y < int(s.Config.Height) && y < s.WaterY.Int() {
		if s.Terrain.Solid(x, y) {
			return float64(x), float64(y), true
		}
		y++
	}
	return 0, 0, false
}

func (r *Scene) drawGrave(c Canvas, s *sim.State, i int, margin, top, scale float64) {
	w := s.Worms[i]
	age := 60
	if i < len(r.motion) {
		age = r.motion[i].deathAge
	}
	if age < 18 && w.Pos.Y < s.WaterY && w.Pos.Y >= 0 {
		p := r.actorPose(i, s)
		p.hurt = true
		p.face = faceHurt
		p.angle += float64(w.Facing) * float64(age) / 18 * math.Pi / 2
		p.sy *= 1 - float64(age)/36
		stakey(c, margin+float64(w.Pos.X)/65536*scale, top+float64(w.Pos.Y)/65536*scale, scale, teams[w.Seat], int(w.Facing), 0, false, p)
		return
	}
	x, y, ok := gravePosition(s, i)
	if !ok {
		return
	}
	sprite := objects.Grave()
	growth := math.Min(1, math.Max(.15, float64(age-18)/12))
	h := 34 * scale * growth
	width := h * float64(sprite.Bounds().Dx()) / float64(sprite.Bounds().Dy())
	c.Image(sprite, margin+x*scale-width/2, top+y*scale-h, width, h)
}
