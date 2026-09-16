package render

import (
	"image"
	"image/color"
	"math"
)

// actorCanvas rotates the existing vector character around its body centre.
// Terrain collision continues to use the unmodified simulation coordinates.
type actorCanvas struct {
	Canvas
	x, y, angle float64
}

func (a *actorCanvas) point(x, y float64) Point {
	dx, dy := x-a.x, y-a.y
	c, s := math.Cos(a.angle), math.Sin(a.angle)
	return Point{a.x + dx*c - dy*s, a.y + dx*s + dy*c}
}
func (a *actorCanvas) Poly(points []Point, c color.RGBA) {
	out := make([]Point, len(points))
	for i, p := range points {
		out[i] = a.point(p.X, p.Y)
	}
	a.Canvas.Poly(out, c)
}
func (a *actorCanvas) Rect(x, y, w, h float64, c color.RGBA) {
	a.Poly([]Point{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}, c)
}
func (a *actorCanvas) Circle(x, y, r float64, c color.RGBA) {
	p := a.point(x, y)
	a.Canvas.Circle(p.X, p.Y, r, c)
}
func (a *actorCanvas) Line(x, y, xx, yy, w float64, c color.RGBA) {
	p, q := a.point(x, y), a.point(xx, yy)
	a.Canvas.Line(p.X, p.Y, q.X, q.Y, w, c)
}
func (a *actorCanvas) Image(src image.Image, x, y, w, h float64) {
	p := a.point(x+w/2, y+h/2)
	a.Canvas.RotatedImage(src, p.X, p.Y, w, h, a.angle)
}

func (a *actorCanvas) RotatedImage(src image.Image, x, y, w, h, angle float64) {
	p := a.point(x, y)
	a.Canvas.RotatedImage(src, p.X, p.Y, w, h, angle+a.angle)
}
