package render

import (
	"fmt"
	"github.com/karamble/dcrstakewars/pkg/sim"
	"image/color"
	"math"
)

type hazardStyle struct {
	name                 string
	deep, surface, light color.RGBA
}

func hazardFor(seed uint64) hazardStyle {
	switch seed % 3 {
	case 1:
		return hazardStyle{"LAVA", color.RGBA{103, 25, 27, 255}, color.RGBA{241, 80, 28, 255}, color.RGBA{255, 218, 104, 255}}
	case 2:
		return hazardStyle{"TOXIC SLIME", color.RGBA{26, 63, 49, 255}, color.RGBA{83, 183, 58, 255}, color.RGBA{200, 250, 104, 255}}
	default:
		return hazardStyle{"WATER", color.RGBA{10, 47, 83, 255}, color.RGBA{24, 118, 163, 255}, color.RGBA{115, 230, 245, 255}}
	}
}

type splash struct {
	x, y float64
	age  int
	seat uint8
}

func (r *Scene) hazard(c Canvas, s *sim.State, v View, cam Camera) {
	style := hazardFor(s.Config.Seed)
	left, top := cam.Origin()
	surface := top + float64(s.WaterY)/65536*cam.Zoom
	bottom := top + float64(s.Config.Height)*cam.Zoom
	x0, x1 := float64(cam.Viewport().Min.X), float64(cam.Viewport().Max.X)
	// The straight bright line is the exact lethal boundary. Waves, bubbles and
	// spray are decorative and never change independently verified collisions.
	if surface < bottom && surface < float64(cam.Viewport().Max.Y)+8 {
		c.Rect(x0, surface, x1-x0, bottom-surface, style.deep)
		points := []Point{{x0, bottom}, {x0, surface}}
		for x := x0; x <= x1; x += 8 {
			worldX := cam.X + (x-x0)/cam.Zoom
			wave := math.Sin(worldX*0.031+float64(v.Frame)*AnimationSpeed*0.065)*3 + math.Sin(worldX*0.071-float64(v.Frame)*AnimationSpeed*0.038)*1.5
			points = append(points, Point{x, surface + wave*cam.Zoom})
		}
		points = append(points, Point{x1, bottom})
		c.Poly(points, style.surface)
		for y := surface + 16; y < bottom; y += 13 {
			c.Rect(x0, y, x1-x0, 5, color.RGBA{style.deep.R, style.deep.G, style.deep.B, 100})
		}
		c.Line(x0, surface, x1, surface, 1, style.light)
		// World-anchored animated ripples remain stable while the camera pans.
		for i := 0; i < 90; i++ {
			worldX := float64(i)*53 + float64(s.Config.Seed%41)
			x := left + worldX*cam.Zoom
			if x < x0-30 || x > x1+30 {
				continue
			}
			cycle := math.Mod(float64(v.Frame)*AnimationSpeed*0.7+float64(i*17), 60) / 60
			y := surface + 25*(1-cycle)*cam.Zoom
			if s.Config.Seed%3 == 0 {
				c.Line(x, y, x+18*cam.Zoom, y, 1.5, style.light)
			} else {
				c.Circle(x, y, (2+cycle*3)*cam.Zoom, color.RGBA{style.light.R, style.light.G, style.light.B, uint8(160 * (1 - cycle))})
			}
		}
		c.Text(style.name+"  ·  LETHAL", x0+12, math.Max(surface+10, float64(cam.Viewport().Min.Y)+8), 12, style.light)
	}
	for _, sp := range r.splashes {
		x, y := left+sp.x*cam.Zoom, top+sp.y*cam.Zoom
		t := float64(sp.age) / 50
		for n := 0; n < 12; n++ {
			a := math.Pi + float64(n)*math.Pi/11
			dx := math.Cos(a) * 55 * t
			dy := math.Sin(a)*75*t + 65*t*t
			c.Circle(x+dx*cam.Zoom, y+dy*cam.Zoom, (4-3*t)*cam.Zoom, color.RGBA{style.light.R, style.light.G, style.light.B, uint8(255 * (1 - t))})
		}
		c.Text(fmt.Sprintf("SQUAD %d · STAKEY LOST", sp.seat+1), math.Max(x0+10, math.Min(x-100, x1-260)), y-35-t*40, 13, White)
	}
}
