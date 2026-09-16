package render

import (
	"image/color"
	"math"
)

// Facial features share the body's squash/turn coordinates. Strong silhouettes
// keep expressions readable at gameplay scale, without changing collision size.
func drawStakeyFace(c Canvas, px, py func(float64) float64, k float64, p pose, skin color.RGBA) {
	line := func(x, y, xx, yy, width float64) { c.Line(px(x), py(y), px(xx), py(yy), width*k, Navy) }
	radius := 3 * k * math.Sqrt(p.sx)
	for _, ex := range []float64{-4, 5} {
		if p.face == faceHurt {
			line(ex-2, -22, ex+1, -20, 1.2)
			line(ex+1, -20, ex-2, -18, 1.2)
		} else if p.blink {
			line(ex-2.5, -20, ex+2.5, -20, 1.2)
		} else {
			c.Circle(px(ex), py(-20), radius, White)
			c.Circle(px(ex+p.lookX), py(-20+p.lookY), 1.45*k*math.Sqrt(p.sx), Navy)
			c.Circle(px(ex+p.lookX-.4), py(-20+p.lookY-.6), .45*k, White)
			if p.face == faceFocus || p.face == faceGrit || p.face == faceAngry {
				// Skin-colored lids and inward brows make the squint more than an eyebrow.
				c.Poly([]Point{{px(ex - 3), py(-23.5)}, {px(ex + 3), py(-23.5)}, {px(ex + 3), py(-21.6)}, {px(ex - 3), py(-22)}}, skin)
			}
		}
	}
	switch p.face {
	case faceFocus, faceGrit, faceAngry:
		line(-7, -24, -1, -22.6, 1.2)
		line(2, -22.6, 8, -24, 1.2)
	case faceSmirk:
		line(-7, -24, -1, -24.7, 1)
		line(2, -23.3, 8, -24.1, 1)
	case faceStartled:
		line(-7, -25, -1, -25.5, 1)
		line(2, -25.5, 8, -25, 1)
	}
	switch p.face {
	case faceGrit, faceHurt:
		c.Poly([]Point{{px(-4), py(-15.5)}, {px(5), py(-15.5)}, {px(4), py(-11.5)}, {px(-3), py(-11.5)}}, Navy)
		c.Poly([]Point{{px(-3), py(-14.8)}, {px(4), py(-14.8)}, {px(3.5), py(-12.3)}, {px(-2.5), py(-12.3)}}, White)
		line(0, -14.8, 0, -12.3, .5)
		line(2, -14.8, 2, -12.3, .5)
	case faceSmirk:
		line(-3, -13.8, 1, -12.8, 1)
		line(1, -12.8, 5, -15.3, 1)
		line(4, -15.5, 6, -15.5, .8)
	case faceFocus, faceAngry:
		line(-2, -13, 2, -14, 1)
		line(2, -14, 4, -13.5, 1)
	case faceStartled:
		c.Circle(px(.5), py(-13.5), 2*k, Navy)
	case faceHappy:
		c.Poly([]Point{{px(-4), py(-15)}, {px(5), py(-15)}, {px(3), py(-11)}, {px(-2), py(-11)}}, Navy)
		line(-3, -14.5, 4, -14.5, 1)
		c.Line(px(-3), py(-14.5), px(4), py(-14.5), k, White)
	default:
		line(-2, -14, 0, -13, .8)
		line(0, -13, 3, -14, .8)
	}
}
