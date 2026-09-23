package render

import (
	"github.com/karamble/dcrgaming-stakewars/assets/scenery"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
	"math"
)

// sceneryStrip mirrors alternate tiles at render time, making identical edges
// meet without editing the generated alpha assets. Offset is world anchored.
func sceneryStrip(c Canvas, name string, cam Camera, seed uint64, speed, bottom, width, height float64) {
	im := scenery.Layer(name)
	offset := cam.X*cam.Zoom*speed + float64((seed>>8)%997)
	first := int(math.Floor(offset/width)) - 1
	for i := first; i <= first+int(math.Ceil(float64(cam.Viewport().Dx())/width))+2; i++ {
		x := float64(cam.Viewport().Min.X) + float64(i)*width - offset
		w := width
		if i%2 != 0 {
			w = -w
		}
		c.RotatedImage(im, x+width/2, bottom-height/2, w, height, 0)
	}
}
func drawScenery(c Canvas, s *sim.State, cam Camera, front bool) {
	b := cam.Viewport()
	water := float64(b.Min.Y) + (float64(s.WaterY)/65536-cam.Y)*cam.Zoom
	if front {
		// Fast near layer, sparse and confined to the hazard edge; it never covers
		// the central play area or becomes fake collision geometry.
		sceneryStrip(c, "front", cam, s.Config.Seed, 1.3, water+62, 1700, 210)
		return
	}
	// An opaque sky always covers the stage, including above-map projectile views.
	skyBottom := float64(b.Max.Y) + 160 - math.Min(100, math.Max(0, cam.Y*cam.Zoom*.03))
	sceneryStrip(c, "sky", cam, s.Config.Seed, .04, skyBottom, 2200, float64(b.Dy())+260)
	far, mid := "far", "mid"
	if s.Config.Seed%3 == 1 {
		far = "far-volcanic"
	}
	if s.Config.Seed%3 == 2 {
		mid = "mid-grove"
	}
	scale := .8 + cam.Zoom*.2
	farBottom := float64(b.Max.Y) + 55 - cam.Y*cam.Zoom*.12
	midBottom := float64(b.Max.Y) + 85 - cam.Y*cam.Zoom*.28
	sceneryStrip(c, far, cam, s.Config.Seed, .18, farBottom, 1900*scale, 1080*scale)
	sceneryStrip(c, mid, cam, s.Config.Seed, .43, midBottom, 1800*scale, 1020*scale)
}
