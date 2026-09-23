package render

import (
	"github.com/karamble/dcrgaming-stakewars/assets/cover"
	"github.com/karamble/dcrgaming-stakewars/assets/objects"
	"github.com/karamble/dcrgaming-stakewars/assets/scenery"
	"image/color"
	"math"
)

// PreloadVisuals performs actual asset decoding behind the loading cover.
func PreloadVisuals() { scenery.Layer("sky"); scenery.Weather(0); objects.Grave() }
func DrawCover(c Canvas, frame uint64, ready bool) {
	art := cover.Image()
	b := art.Bounds()
	scale := math.Max(float64(Width)/float64(b.Dx()), float64(Height)/float64(b.Dy()))
	w, h := float64(b.Dx())*scale, float64(b.Dy())*scale
	c.Rect(0, 0, Width, Height, Navy)
	c.Image(art, (Width-w)/2, (Height-h)/2, w, h)
	c.Rect(0, Height-120, Width, 120, color.RGBA{5, 12, 27, 215})
	label := "LOADING LOCAL GAME ASSETS"
	if ready {
		label = "PRESS ENTER OR CLICK TO CONTINUE"
	}
	c.Text(label, 480, Height-85, 19, White)
	c.Rect(480, Height-45, 480, 4, color.RGBA{35, 61, 77, 255})
	if ready {
		c.Rect(480, Height-45, 480, 4, Mint)
	} else {
		x := 480 + math.Mod(float64(frame)*5, 360)
		c.Rect(x, Height-45, 120, 4, Mint)
	}
	c.Text("TACTICAL COMBAT. SHARED CONSENSUS.", 40, Height-28, 11, Muted)
	c.Text("BUILT ON DECRED", 1240, Height-28, 11, Mint)
}
