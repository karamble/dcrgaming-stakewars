package terrain

import "sort"

type Point struct{ X, Y int }

// StampPolygon fills a bounded authored outline using integer scan lines.
func (m *Mask) StampPolygon(points []Point, solid bool) {
	if len(points) < 3 || len(points) > 64 {
		panic("terrain: invalid polygon")
	}
	lo, hi := m.h, 0
	for _, p := range points {
		if p.X < -MaxWidth || p.X > MaxWidth*2 || p.Y < -MaxHeight || p.Y > MaxHeight*2 {
			panic("terrain: polygon outside bounds")
		}
		if p.Y < lo {
			lo = p.Y
		}
		if p.Y > hi {
			hi = p.Y
		}
	}
	if lo < 0 {
		lo = 0
	}
	if hi > m.h {
		hi = m.h
	}
	crossings := make([]int, 0, len(points))
	for y := lo; y < hi; y++ {
		crossings = crossings[:0]
		for i, a := range points {
			b := points[(i+1)%len(points)]
			if a.Y > b.Y {
				a, b = b, a
			}
			if y < a.Y || y >= b.Y {
				continue
			}
			x := a.X + int(int64(y-a.Y)*int64(b.X-a.X)/int64(b.Y-a.Y))
			crossings = append(crossings, x)
		}
		sort.Ints(crossings)
		for i := 0; i+1 < len(crossings); i += 2 {
			m.StampRect(Rect{crossings[i], y, crossings[i+1] + 1, y + 1}, solid)
		}
	}
}
func (m *Mask) StampEllipse(cx, cy, rx, ry int, solid bool) {
	if rx < 1 || ry < 1 || rx > MaxWidth || ry > MaxHeight || cx < -MaxWidth || cx > MaxWidth*2 || cy < -MaxHeight || cy > MaxHeight*2 {
		panic("terrain: invalid ellipse")
	}
	box := m.clip(Rect{cx - rx, cy - ry, cx + rx + 1, cy + ry + 1})
	xx, yy := int64(rx)*int64(rx), int64(ry)*int64(ry)
	for y := box.Y0; y < box.Y1; y++ {
		for x := box.X0; x < box.X1; x++ {
			dx, dy := int64(x-cx), int64(y-cy)
			if dx*dx*yy+dy*dy*xx <= xx*yy {
				if solid {
					m.Set(x, y)
				} else {
					m.Clear(x, y)
				}
			}
		}
	}
}
