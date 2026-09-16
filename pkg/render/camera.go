package render

import (
	"github.com/karamble/dcrstakewars/pkg/sim"
	"image"
	"math"
)

var CameraBar = image.Rect(1020, 14, 1390, 44)

var ArenaRect = image.Rect(0, 0, Width, Height)

type Camera struct {
	X, Y, Zoom                   float64
	Bounds                       image.Rectangle
	fitTop                       float64
	fitValid                     bool
	trackingShot                 bool
	impactX, impactY             float64
	impactHold, returnFrame      int
	returnX, returnY, returnZoom float64
}

func (c *Camera) Clamp(s *sim.State) {
	c.Zoom = math.Max(0.4, math.Min(2.5, c.Zoom))
	c.X = math.Max(0, math.Min(c.X, math.Max(0, float64(s.Config.Width)-float64(c.Viewport().Dx())/c.Zoom)))
	// Flights can rise above the terrain's top edge. Preserve that sky in all
	// camera clamps, including the renderer's copy of the camera.
	// Allow manual sky headroom even when no projectile is airborne.
	minY := -float64(c.Viewport().Dy()) / c.Zoom * .75
	if c.impactHold > 0 || c.returnFrame > 0 {
		minY = math.Min(minY, c.impactY-float64(c.Viewport().Dy())/c.Zoom/2)
	}
	for _, p := range s.Projectiles {
		if !p.Dead && !p.Burning && p.Weapon != sim.RemoteCharge {
			minY = math.Min(minY, float64(p.Pos.Y)/65536-float64(c.Viewport().Dy())/c.Zoom/2)
		}
	}
	c.Y = math.Max(minY, math.Min(c.Y, math.Max(0, float64(s.Config.Height)-float64(c.Viewport().Dy())/c.Zoom)))
}
func (c *Camera) Focus(s *sim.State) {
	if c.Zoom == 0 {
		c.Zoom = 1.25
	}
	if c.Viewport() == Stage(true, true) {
		if !c.fitValid {
			c.measureBattlefield(s)
		}
		c.fitBattlefield(s)
	}
	w := s.Worms[s.Active]
	c.X = float64(w.Pos.X)/65536 - float64(c.Viewport().Dx())/c.Zoom/2
	if c.Viewport() == Stage(true, true) {
		c.Clamp(s)
		return
	}
	c.Y = float64(s.Config.Height) - float64(c.Viewport().Dy())/c.Zoom
	// Keep the lethal floor visible unless following a character high above it.
	c.Y = math.Min(c.Y, float64(w.Pos.Y)/65536-80/c.Zoom)
	c.Clamp(s)
}
func (c Camera) World(x, y float64) (float64, float64) {
	return c.X + (x-float64(c.Viewport().Min.X))/c.Zoom, c.Y + (y-float64(c.Viewport().Min.Y))/c.Zoom
}
func (c Camera) Origin() (float64, float64) {
	return float64(c.Viewport().Min.X) - c.X*c.Zoom, float64(c.Viewport().Min.Y) - c.Y*c.Zoom
}

func (c Camera) Viewport() image.Rectangle {
	if c.Bounds.Empty() {
		return ArenaRect
	}
	return c.Bounds
}
func Stage(hideTop, hideBottom bool) image.Rectangle {
	return ArenaRect
}
func (c *Camera) SetViewport(bounds image.Rectangle, s *sim.State) {
	old := c.Viewport()
	if old == bounds {
		return
	}
	centerX, centerY := c.X+float64(old.Dx())/c.Zoom/2, c.Y+float64(old.Dy())/c.Zoom/2
	c.Bounds = bounds
	c.fitValid = false
	if bounds == Stage(true, true) {
		c.measureBattlefield(s)
		c.fitBattlefield(s)
		c.X = centerX - float64(bounds.Dx())/c.Zoom/2
		c.Clamp(s)
		return
	}
	c.X = centerX - float64(bounds.Dx())/c.Zoom/2
	c.Y = centerY - float64(bounds.Dy())/c.Zoom/2
	c.Clamp(s)
}

// EdgePan returns screen-pixel motion per tick inside the stage edge zones.
func EdgePan(bounds image.Rectangle, x, y int) (float64, float64) {
	active := image.Rect(0, bounds.Min.Y, Width, bounds.Max.Y)
	if bounds.Min.Y <= 12 {
		active.Min.Y = 0
	}
	if bounds.Max.Y >= Height-12 {
		active.Max.Y = Height
	}
	if !image.Pt(x, y).In(active) {
		return 0, 0
	}
	axis := func(p, lo, hi int) float64 {
		if p < lo+24 {
			return -12 * math.Min(1, float64(lo+24-p)/24)
		}
		if p >= hi-24 {
			return 12 * math.Min(1, float64(p-(hi-24)+1)/24)
		}
		return 0
	}
	return axis(x, bounds.Min.X, bounds.Max.X), axis(y, bounds.Min.Y, bounds.Max.Y)
}

// FollowProjectiles keeps the live shot centered, regardless of prior manual
// panning. A volley follows its center; lingering fire and placed charges must
// not hold the camera away from the next player. This is presentation only.
func (c *Camera) FollowProjectiles(s *sim.State) bool {
	var x, y float64
	n := 0
	for _, p := range s.Projectiles {
		if p.Dead || p.Burning || p.Weapon == sim.RemoteCharge {
			continue
		}
		x += float64(p.Pos.X) / 65536
		y += float64(p.Pos.Y) / 65536
		n++
	}
	if n == 0 {
		return false
	}
	if c.Zoom == 0 {
		c.Zoom = 1.25
	}
	c.X = x/float64(n) - float64(c.Viewport().Dx())/c.Zoom/2
	c.Y = y/float64(n) - float64(c.Viewport().Dy())/c.Zoom/2
	c.Clamp(s)
	return true
}

// Fit the whole vertical battlefield while preserving horizontal exploration.
// Measure on viewport transitions, not on every animation tick.
func (c *Camera) measureBattlefield(s *sim.State) {
	top := s.WaterY.Int()
	found := false
	for y := 0; y < top && !found; y++ {
		for x := 0; x < int(s.Config.Width); x++ {
			if s.Terrain.Solid(x, y) {
				top = y
				found = true
				break
			}
		}
	}
	c.fitTop = math.Max(0, float64(top)-72)
	c.fitValid = true
}
func (c *Camera) fitBattlefield(s *sim.State) {
	bottom := float64(s.WaterY)/65536 + 28
	span := math.Max(100, bottom-c.fitTop)
	c.Zoom = math.Max(.4, math.Min(2.5, float64(c.Viewport().Dy())/span))
	c.Y = bottom - float64(c.Viewport().Dy())/c.Zoom
}

// HUD overlays consume pointer input without changing the world viewport.
func HUDContains(x, y int, hideTop, hideBottom bool) bool {
	return (!hideTop && y < 104) || (!hideBottom && y >= 708) ||
		image.Pt(x, y).In(TopToggleRect) || image.Pt(x, y).In(BottomToggleRect)
}

// ObserveImpact receives replay effects; the cinematic runs on UI updates even
// while waiting for the next asynchronous turn. It never advances game time.
func (c *Camera) ObserveImpact(effects []sim.Effect) {
	for _, e := range effects {
		if e.Radius > 0 || e.Shot || e.HazardDeath {
			c.impactX, c.impactY = float64(e.Pos.X)/65536, float64(e.Pos.Y)/65536
			c.impactHold = 90
			c.returnFrame = 0
		}
	}
}

// ReviewingImpact also delays the victory overlay until the final hit is seen.
func (c Camera) ReviewingImpact() bool { return c.impactHold > 0 || c.trackingShot }

// Advance follows flight, reviews impact, then accelerates and decelerates into
// the next actor. Returns true while cinematic tracking owns the camera.
func (c *Camera) Advance(s *sim.State, follow bool) bool {
	if c.FollowProjectiles(s) {
		c.trackingShot = true
		c.returnFrame = 0
		if c.impactHold == 0 {
			c.impactX = c.X + float64(c.Viewport().Dx())/c.Zoom/2
			c.impactY = c.Y + float64(c.Viewport().Dy())/c.Zoom/2
		}
		return true
	}
	if c.trackingShot {
		c.trackingShot = false
		c.impactHold = 90
	}
	if c.impactHold > 0 {
		c.X = c.impactX - float64(c.Viewport().Dx())/c.Zoom/2
		c.Y = c.impactY - float64(c.Viewport().Dy())/c.Zoom/2
		c.Clamp(s)
		// Keep the aftermath on screen while physics/retreat finishes, rather
		// than briefly returning to the shooter before the next turn.
		if c.impactHold > 1 || (s.Phase != sim.Retreating && s.Phase != sim.Resolving) {
			c.impactHold--
		}
		if c.impactHold == 0 {
			c.returnFrame = 1
			c.returnX, c.returnY, c.returnZoom = c.X, c.Y, c.Zoom
		}
		return true
	}
	if s.Phase == sim.Ended {
		return false
	}
	if c.returnFrame > 0 {
		target := *c
		target.Focus(s)
		t := math.Min(1, float64(c.returnFrame)/60)
		ease := t * t * (3 - 2*t)
		c.X = c.returnX + (target.X-c.returnX)*ease
		c.Y = c.returnY + (target.Y-c.returnY)*ease
		c.Zoom = c.returnZoom + (target.Zoom-c.returnZoom)*ease
		c.returnFrame++
		if t == 1 {
			c.returnFrame = 0
		}
		return true
	}
	if follow {
		target := *c
		target.Focus(s)
		c.X += (target.X - c.X) * .12
		c.Y += (target.Y - c.Y) * .12
		c.Zoom += (target.Zoom - c.Zoom) * .12
	}
	return false
}
