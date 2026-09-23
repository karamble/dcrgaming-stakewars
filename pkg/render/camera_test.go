package render

import (
	"crypto/sha256"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"math"
	"testing"
)

func TestCameraCoordinatesAfterPanAndZoom(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	for _, zoom := range []float64{0.4, 1.25, 2.5} {
		c := Camera{X: 2200, Y: 200, Zoom: zoom}
		c.Clamp(s)
		x, y := c.Origin()
		wx, wy := c.World(x+123.75*c.Zoom, y+99.5*c.Zoom)
		if math.Abs(wx-123.75) > 0.0001 || math.Abs(wy-99.5) > 0.0001 {
			t.Fatal("pointer no longer maps to terrain")
		}
		c.X = 1e9
		c.Y = 1e9
		c.Clamp(s)
		if c.X+float64(ArenaRect.Dx())/c.Zoom > float64(s.Config.Width)+0.001 {
			t.Fatal("camera escaped right edge")
		}
		c.X = -100
		c.Y = -100
		c.Clamp(s)
		if c.X != 0 || c.Y != -100 {
			t.Fatal("camera escaped left edge")
		}
	}
}

func TestCollapsedViewportAndEdgePan(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	c := Camera{X: 1500, Y: 100, Zoom: 2}
	old := c.Viewport()
	cx, _ := c.World(float64(old.Min.X+old.Dx()/2), float64(old.Min.Y+old.Dy()/2))
	c.SetViewport(Stage(true, true), s)
	b := c.Viewport()
	wx, _ := c.World(float64(b.Min.X+b.Dx()/2), float64(b.Min.Y+b.Dy()/2))
	if math.Abs(wx-cx) > 0.001 {
		t.Fatal("panel toggle shifted horizontal camera target")
	}
	if b != old || b.Min.X != 0 || b.Min.Y != 0 || b.Dx() != Width || b.Dy() != Height {
		t.Fatal("HUD visibility changed fullscreen stage")
	}
	for _, test := range []struct {
		x, y   int
		dx, dy float64
	}{{0, 0, -12, -12}, {Width - 1, Height - 1, 12, 12}, {Width / 2, Height / 2, 0, 0}} {
		dx, dy := EdgePan(b, test.x, test.y)
		if dx != test.dx || dy != test.dy {
			t.Fatalf("edge scroll at %d,%d: %f,%f", test.x, test.y, dx, dy)
		}
	}
	if dx, dy := EdgePan(Stage(false, false), 100, 80); dx != 0 || dy != 0 {
		t.Fatal("hovering header scrolls map")
	}
	if dx, dy := EdgePan(b, -1, 500); dx != 0 || dy != 0 {
		t.Fatal("pointer outside window scrolls map")
	}
}

func TestTallMapVerticalCameraLimits(t *testing.T) {
	config := sim.DefaultConfig()
	config.Height = 2048
	s, err := sim.New(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []bool{false, true} {
		c := Camera{Zoom: 1.25, Bounds: Stage(hidden, hidden)}
		_, dy := EdgePan(c.Viewport(), Width/2, c.Viewport().Max.Y-1)
		c.Y += dy / c.Zoom
		c.Clamp(s)
		if c.Y <= 0 {
			t.Fatal("bottom edge did not scroll down tall map")
		}
		c.Y = 1e9
		c.Clamp(s)
		bottom := c.Y + float64(c.Viewport().Dy())/c.Zoom
		if math.Abs(bottom-float64(config.Height)) > 0.001 {
			t.Fatal("camera failed to stop at world bottom")
		}
		_, dy = EdgePan(c.Viewport(), Width/2, c.Viewport().Min.Y)
		previous := c.Y
		c.Y += dy / c.Zoom
		c.Clamp(s)
		if c.Y >= previous {
			t.Fatal("top edge did not scroll up tall map")
		}
	}
}

func TestProjectileFollowAfterManualPan(t *testing.T) {
	cfg := sim.DefaultConfig()
	cfg.Width, cfg.Height = 4096, 2048
	s, err := sim.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []bool{false, true} {
		c := Camera{X: 0, Y: 0, Zoom: 1.25, Bounds: Stage(hidden, hidden)}
		s.Projectiles = []sim.Projectile{{Weapon: sim.Bazooka}}
		for _, pos := range [][2]int{{2800, 1200}, {3100, 800}, {3100, -180}, {3100, -600}, {3500, 1600}, {4090, 2000}} {
			s.Projectiles[0].Pos = fixed.Vec{X: fixed.FromInt(pos[0]), Y: fixed.FromInt(pos[1])}
			before := sha256.Sum256(s.AppendCanonical(nil))
			if !c.FollowProjectiles(s) {
				t.Fatal("live shot not tracked")
			}
			c.Clamp(s) // The renderer also clamps its copy; it must preserve sky tracking.
			ox, oy := c.Origin()
			x, y := ox+float64(pos[0])*c.Zoom, oy+float64(pos[1])*c.Zoom
			if x < float64(c.Viewport().Min.X) || x > float64(c.Viewport().Max.X) || y < float64(c.Viewport().Min.Y) || y > float64(c.Viewport().Max.Y) {
				t.Fatal("shot escaped viewport", x, y)
			}
			if before != sha256.Sum256(s.AppendCanonical(nil)) {
				t.Fatal("camera changed simulation")
			}
		}
		s.Projectiles = []sim.Projectile{{Burning: true}, {Weapon: sim.RemoteCharge}, {Dead: true}}
		old := c
		if c.FollowProjectiles(s) || c != old {
			t.Fatal("lingering effects moved camera")
		}
		s.Projectiles = nil
		if c.FollowProjectiles(s) {
			t.Fatal("camera did not release after impact")
		}
	}
}

func TestCollapsedBattlefieldFitsHeight(t *testing.T) {
	for _, height := range []uint16{768, 2048} {
		cfg := sim.DefaultConfig()
		cfg.Height = height
		s, err := sim.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		c := Camera{Zoom: 1.25, Bounds: Stage(false, false)}
		c.Focus(s)
		c.SetViewport(Stage(true, true), s)
		c.Focus(s)
		_, oy := c.Origin()
		water := oy + float64(s.WaterY)/65536*c.Zoom
		bottom := float64(c.Viewport().Max.Y)
		if water > bottom-8 || water < bottom-80 {
			t.Fatal("hazard not at bottom", height, water, bottom)
		}
		top := int(height)
		for y := 0; y < int(height); y++ {
			found := false
			for x := 0; x < int(cfg.Width); x++ {
				if s.Terrain.Solid(x, y) {
					top = y
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		sky := oy + float64(top)*c.Zoom - float64(c.Viewport().Min.Y)
		if sky < 24 || sky > 180 {
			t.Fatal("terrain lacks sky headroom", height, sky)
		}
	}
}

func TestImpactHoldSurvivesTurnChangeAndEasesReturn(t *testing.T) {
	cfg := sim.DefaultConfig()
	cfg.Width = 4096
	s, _ := sim.New(cfg)
	c := Camera{Zoom: 1.25}
	s.Projectiles = []sim.Projectile{{Weapon: sim.Bazooka, Pos: fixed.Vec{X: fixed.FromInt(3200), Y: fixed.FromInt(300)}}}
	c.Advance(s, true)
	s.Projectiles = nil
	c.ObserveImpact([]sim.Effect{{Radius: 30, Pos: fixed.Vec{X: fixed.FromInt(3100), Y: fixed.FromInt(350)}}})
	s.Turn++
	s.Active = 1
	s.Worms[1].Pos.X = fixed.FromInt(600)
	before := sha256.Sum256(s.AppendCanonical(nil))
	c.Advance(s, true)
	x, y := c.X, c.Y
	for i := 0; i < 89; i++ {
		c.Advance(s, true)
		if c.X != x || c.Y != y {
			t.Fatal("left impact before review finished")
		}
	}
	if c.ReviewingImpact() {
		t.Fatal("impact review never ended")
	}
	c.Advance(s, true)
	first := math.Abs(c.X - x)
	if first == 0 || first > 10 {
		t.Fatal("return must begin gently", first)
	}
	for i := 0; i < 28; i++ {
		c.Advance(s, true)
	}
	previous := c.X
	c.Advance(s, true)
	if math.Abs(c.X-previous) <= first {
		t.Fatal("return did not accelerate")
	}
	for i := 0; i < 30; i++ {
		c.Advance(s, true)
	}
	target := c
	target.Focus(s)
	if math.Abs(c.X-target.X) > .001 || math.Abs(c.Y-target.Y) > .001 {
		t.Fatal("return did not reach next player")
	}
	if before != sha256.Sum256(s.AppendCanonical(nil)) {
		t.Fatal("camera changed game state")
	}
}

func TestSkyPanWithoutProjectileAndFinalImpactReview(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	c := Camera{Zoom: 1.25}
	for i := 0; i < 100; i++ {
		c.Y -= 14 / c.Zoom
		c.Clamp(s)
	}
	if c.Y >= -100 {
		t.Fatal("manual upward pan has no sky headroom")
	}
	c.ObserveImpact([]sim.Effect{{Radius: 30, Pos: fixed.Vec{X: fixed.FromInt(2000), Y: fixed.FromInt(-600)}}})
	s.Phase = sim.Ended
	c.Advance(s, true)
	c.Clamp(s)
	if !c.ReviewingImpact() || c.Y >= -600 {
		t.Fatal("final sky impact clamped or skipped")
	}
	x := c.X
	for i := 0; i < 100; i++ {
		c.Advance(s, true)
	}
	if c.ReviewingImpact() || c.X != x {
		t.Fatal("final impact should release victory without panning away")
	}
}

func TestImpactDoesNotReturnToShooterDuringRetreat(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	s.Phase = sim.Retreating
	c := Camera{Zoom: 1.25}
	c.ObserveImpact([]sim.Effect{{Radius: 30, Pos: fixed.Vec{X: fixed.FromInt(2800), Y: fixed.FromInt(400)}}})
	c.Advance(s, true)
	x := c.X
	for i := 0; i < 180; i++ {
		c.Advance(s, true)
	}
	if c.X != x || !c.ReviewingImpact() {
		t.Fatal("left aftermath before turn advanced")
	}
	s.Phase = sim.Ready
	s.Turn++
	c.Advance(s, true)
	if c.ReviewingImpact() {
		t.Fatal("new turn did not release aftermath")
	}
}
