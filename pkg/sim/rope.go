package sim

import "github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"

func (s *State) attach(w *Worm) {
	origin := w.Pos
	origin.Y -= fixed.FromInt(10)
	dir := fixed.Vec{X: fixed.Cos(s.Aim), Y: fixed.Sin(s.Aim)}
	for n := 12; n <= 600; n++ {
		p := origin.Add(dir.Scale(fixed.FromInt(n)))
		if s.Terrain.Solid(p.X.Int(), p.Y.Int()) {
			anchor := origin.Add(dir.Scale(fixed.FromInt(n - 2)))
			s.Rope = Rope{Pivots: []Pivot{{Pos: anchor}}, Length: w.Pos.Sub(anchor).Len()}
			w.Grounded = false
			w.Apex = w.Pos.Y
			return
		}
	}
}

// obstruction excludes the endpoints so a terrain-adjacent pivot does not
// immediately wrap around itself. The last free pixel is the next pivot.
func (s *State) obstruction(a, b fixed.Vec) (fixed.Vec, bool) {
	d := b.Sub(a)
	steps := int((fixed.Max(fixed.Abs(d.X), fixed.Abs(d.Y)) + fixed.One - 1) / fixed.One)
	last := a
	for n := 2; n < steps-2; n++ {
		p := fixed.Vec{X: a.X + fixed.F(int64(d.X)*int64(n)/int64(steps)), Y: a.Y + fixed.F(int64(d.Y)*int64(n)/int64(steps))}
		if s.Terrain.Solid(p.X.Int(), p.Y.Int()) {
			return last, true
		}
		last = p
	}
	return fixed.Vec{}, false
}
func crossSign(a, b fixed.Vec) int8 {
	v := int64(a.X)*int64(b.Y) - int64(a.Y)*int64(b.X)
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

func (s *State) swing(w *Worm) {
	r := &s.Rope
	s.unwrapRope(w.Pos)
	last := r.Pivots[len(r.Pivots)-1].Pos
	w.Vel.Y += fixed.Ratio(1, 8)
	normal := w.Pos.Sub(last).Unit()
	// Pump along the swing arc, so steering remains useful near its sides.
	// Slack rope still allows ordinary horizontal air control.
	taut := w.Pos.Sub(last).Len() >= r.Length-fixed.Ratio(1, 8)
	steer := fixed.Vec{X: fixed.One}
	if taut {
		steer = fixed.Vec{X: normal.Y, Y: -normal.X}
		if steer.X < 0 {
			steer = steer.Scale(-fixed.One)
		}
	}
	w.Vel = w.Vel.Add(steer.Scale(fixed.Ratio(int32(s.Walk), 4)))
	// Tension can pull but never push: slack rope permits free motion.
	if taut && fixed.Dot(w.Vel, normal) > 0 {
		w.Vel = w.Vel.Sub(normal.Scale(fixed.Dot(w.Vel, normal)))
	}
	w.Vel.X = fixed.Clamp(w.Vel.X, -fixed.FromInt(18), fixed.FromInt(18))
	w.Vel.Y = fixed.Clamp(w.Vel.Y, -fixed.FromInt(18), fixed.FromInt(18))
	target := w.Pos.Add(w.Vel)
	if target.Sub(last).Len() > r.Length {
		target = last.Add(target.Sub(last).Unit().Scale(r.Length))
	}
	reeling := r.Length < w.Pos.Sub(last).Len()-fixed.Ratio(1, 8)
	delta := target.Sub(w.Pos)
	steps := int((fixed.Max(fixed.Abs(delta.X), fixed.Abs(delta.Y)) + fixed.One - 1) / fixed.One)
	if steps < 1 {
		steps = 1
	}
	start := w.Pos
	previous := start
	collided := false
	for n := 1; n <= steps; n++ {
		desired := fixed.Vec{X: start.X + fixed.F(int64(delta.X)*int64(n)/int64(steps)), Y: start.Y + fixed.F(int64(delta.Y)*int64(n)/int64(steps))}
		increment := desired.Sub(previous)
		previous = desired
		p := w.Pos.Add(increment)
		if !s.blocked(p) {
			w.Pos = p
			continue
		}
		collided = true
		// Reeling along a floor needs the same small-step clearance as
		// walking. Tangential gravity can cancel the tiny upward component
		// of a shallow pull; axis sliding alone then never clears a pixel.
		if reeling && increment.X != 0 && s.supported(w.Pos) {
			climbed := false
			for up := 1; up <= 4; up++ {
				lift := w.Pos.Sub(fixed.Vec{Y: fixed.FromInt(up)})
				if s.blocked(lift) {
					break
				}
				step := lift.Add(fixed.Vec{X: increment.X})
				if !s.blocked(step) {
					w.Pos = step
					w.Vel.Y = fixed.Min(w.Vel.Y, 0)
					climbed = true
					break
				}
			}
			if climbed {
				continue
			}
		}
		// Sweep both axes separately at the same <=1-pixel resolution. On a
		// slope, the free vertical component lifts the body clear of the next
		// terrain step instead of throwing away the entire diagonal pull.
		vertical := w.Pos.Add(fixed.Vec{Y: increment.Y})
		if !s.blocked(vertical) {
			w.Pos = vertical
		} else {
			w.Vel.Y = 0
		}
		horizontal := w.Pos.Add(fixed.Vec{X: increment.X})
		if !s.blocked(horizontal) {
			w.Pos = horizontal
		} else {
			w.Vel.X = 0
		}
	}
	if collided {
		// Do not bank impossible shortening against a wall. Subsequent reeling
		// starts at the reachable length, avoiding a snap when the wall ends.
		r.Length = fixed.Max(r.Length, w.Pos.Sub(last).Len())
	}
	if w.Pos.Y < w.Apex {
		w.Apex = w.Pos.Y
	}
	// Keep the total rope length when introducing or removing a corner.
	if len(r.Pivots) < 32 {
		if pivot, hit := s.obstruction(w.Pos, last); hit {
			consumed := pivot.Sub(last).Len()
			if consumed > fixed.FromInt(2) && r.Length-consumed >= fixed.FromInt(8) {
				side := crossSign(pivot.Sub(last), w.Pos.Sub(pivot))
				r.Pivots = append(r.Pivots, Pivot{Pos: pivot, Side: side})
				r.Length -= consumed
			}
		}
	}
	s.unwrapRope(w.Pos)
	w.Grounded = false
}

// ropeSupported drops a rope if any supporting terrain corner was destroyed.
// Anchors sit up to two ray steps outside terrain; a three-pixel neighborhood
// includes that contact for diagonal rays as well as horizontal/vertical ones.
func (s *State) ropeSupported() bool {
	for _, pivot := range s.Rope.Pivots {
		x, y := pivot.Pos.X.Int(), pivot.Pos.Y.Int()
		supported := false
		for yy := y - 3; yy <= y+3; yy++ {
			for xx := x - 3; xx <= x+3; xx++ {
				if s.Terrain.Solid(xx, yy) {
					supported = true
				}
			}
		}
		if !supported {
			return false
		}
	}
	return true
}

// Release every obsolete corner as soon as the direct segment clears terrain.
// A winding-side test alone traps a rope on a corner after sliding past it.
func (s *State) unwrapRope(pos fixed.Vec) {
	r := &s.Rope
	for len(r.Pivots) > 1 {
		n := len(r.Pivots) - 1
		p, prev := r.Pivots[n], r.Pivots[n-1]
		if _, hit := s.obstruction(pos, prev.Pos); hit {
			break
		}
		r.Length += p.Pos.Sub(prev.Pos).Len()
		r.Pivots = r.Pivots[:n]
	}
}
