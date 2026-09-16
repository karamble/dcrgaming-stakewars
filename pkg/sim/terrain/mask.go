// Package terrain owns the collision mask, never the visual terrain texture.
package terrain

import "encoding/binary"

const MaxWidth, MaxHeight = 4096, 2048

type Rect struct{ X0, Y0, X1, Y1 int }

func (r Rect) Empty() bool { return r.X0 >= r.X1 || r.Y0 >= r.Y1 }

type Mask struct {
	w, h, stride int
	border       bool
	bits         []uint64
}

func New(w, h int, border bool) Mask {
	if w < 1 || h < 1 || w > MaxWidth || h > MaxHeight {
		panic("terrain: invalid dimensions")
	}
	stride := (w + 63) / 64
	return Mask{w: w, h: h, stride: stride, border: border, bits: make([]uint64, stride*h)}
}
func (m Mask) Width() int  { return m.w }
func (m Mask) Height() int { return m.h }
func (m Mask) Solid(x, y int) bool {
	if x < 0 || y < 0 || x >= m.w || y >= m.h {
		return m.border
	}
	return m.bits[y*m.stride+x/64]&(uint64(1)<<uint(x%64)) != 0
}
func (m *Mask) Set(x, y int) {
	if x >= 0 && y >= 0 && x < m.w && y < m.h {
		m.bits[y*m.stride+x/64] |= uint64(1) << uint(x%64)
	}
}
func (m *Mask) Clear(x, y int) {
	if x >= 0 && y >= 0 && x < m.w && y < m.h {
		m.bits[y*m.stride+x/64] &^= uint64(1) << uint(x%64)
	}
}
func (m *Mask) StampRect(r Rect, solid bool) {
	r = m.clip(r)
	for y := r.Y0; y < r.Y1; y++ {
		for x := r.X0; x < r.X1; x++ {
			if solid {
				m.Set(x, y)
			} else {
				m.Clear(x, y)
			}
		}
	}
}
func (m *Mask) StampCircle(cx, cy, radius int, solid bool) Rect {
	// Bounded arithmetic, even when used by a malformed external fixture.
	if radius < 0 || radius > MaxWidth || cx < -MaxWidth || cx > MaxWidth*2 || cy < -MaxHeight || cy > MaxHeight*2 {
		panic("terrain: invalid circle")
	}
	r := m.clip(Rect{cx - radius, cy - radius, cx + radius + 1, cy + radius + 1})
	for y := r.Y0; y < r.Y1; y++ {
		for x := r.X0; x < r.X1; x++ {
			dx, dy := int64(x-cx), int64(y-cy)
			if dx*dx+dy*dy <= int64(radius)*int64(radius) {
				if solid {
					m.Set(x, y)
				} else {
					m.Clear(x, y)
				}
			}
		}
	}
	return r
}
func (m Mask) clip(r Rect) Rect {
	if r.X0 < 0 {
		r.X0 = 0
	}
	if r.Y0 < 0 {
		r.Y0 = 0
	}
	if r.X1 > m.w {
		r.X1 = m.w
	}
	if r.Y1 > m.h {
		r.Y1 = m.h
	}
	return r
}
func (m Mask) Clone() Mask { m.bits = append([]uint64(nil), m.bits...); return m }
func (m Mask) AppendCanonical(b []byte) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(m.w))
	b = binary.LittleEndian.AppendUint32(b, uint32(m.h))
	if m.border {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	for _, word := range m.bits {
		b = binary.LittleEndian.AppendUint64(b, word)
	}
	return b
}
