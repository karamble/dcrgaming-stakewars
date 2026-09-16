package terrain

import (
	"bytes"
	"testing"
)

func TestWordBoundaryAndClone(t *testing.T) {
	m := New(65, 10, false)
	m.Set(64, 3)
	m.Set(0, 4)
	if m.Solid(63, 3) || !m.Solid(64, 3) || !m.Solid(0, 4) || m.Solid(65, 3) {
		t.Fatal("word aligned rows alias")
	}
	c := m.Clone()
	c.Clear(64, 3)
	if !m.Solid(64, 3) {
		t.Fatal("clone aliases original")
	}
	if bytes.Equal(c.AppendCanonical(nil), m.AppendCanonical(nil)) {
		t.Fatal("canonical mask omits bits")
	}
}
func TestCraterAndBorder(t *testing.T) {
	m := New(80, 80, true)
	m.StampRect(Rect{0, 0, 80, 80}, true)
	r := m.StampCircle(40, 40, 10, false)
	if r != (Rect{30, 30, 51, 51}) || m.Solid(40, 40) || m.Solid(50, 40) || !m.Solid(51, 40) || !m.Solid(-1, 1) {
		t.Fatal("crater or border")
	}
	m.StampCircle(40, 40, 10, true)
	if !m.Solid(40, 40) {
		t.Fatal("additive stamp")
	}
}
