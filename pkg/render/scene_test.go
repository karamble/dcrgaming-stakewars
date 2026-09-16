package render

import (
	"image"
	"testing"

	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
	"github.com/karamble/dcrstakewars/pkg/sim/terrain"
)

func TestDrawingDoesNotChangeSimulation(t *testing.T) {
	s, err := sim.New(sim.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	before := replay.Hash(s)
	r := New(s)
	c := NewRaster()
	r.Draw(c, s, View{Arena: true, Frame: 123, Power: 500})
	if replay.Hash(s) != before {
		t.Fatal("renderer mutated game state")
	}
}
func TestDirtyUpdatesAccumulateUntilUpload(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	r := New(s)
	r.dirty = image.Rectangle{} // initial GPU upload
	a := terrain.Rect{X0: 10, Y0: 600, X1: 30, Y1: 620}
	b := terrain.Rect{X0: 900, Y0: 600, X1: 920, Y1: 620}
	s.Terrain.StampRect(a, false)
	r.Update(s, []sim.Effect{{Dirty: a}})
	s.Terrain.StampRect(b, false)
	r.Update(s, []sim.Effect{{Dirty: b}})
	if !image.Pt(10, 600).In(r.dirty) || !image.Pt(919, 619).In(r.dirty) {
		t.Fatal("missed earlier update before GPU draw")
	}
	fresh := New(s)
	if string(r.Terrain.Pix) != string(fresh.Terrain.Pix) {
		t.Fatal("partial update differs from full rebuild")
	}
}

func TestShotEffectsDoNotDirtyTerrain(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	r := New(s)
	revision := r.Revision()
	r.Update(s, []sim.Effect{{Shot: true, Weapon: sim.Uzi}})
	if r.Revision() != revision {
		t.Fatal("shot caused unrelated terrain upload")
	}
	for i := 0; i < 50; i++ {
		r.Update(s, nil)
	}
	if len(r.shots) != 0 {
		t.Fatal("expired shot retained")
	}
}

func TestArsenalPagesAndPopupHitTesting(t *testing.T) {
	for w := sim.Weapon(0); w < sim.WeaponCount; w++ {
		box := WeaponRect(w)
		got, ok := WeaponAt(box.Min.X+1, box.Min.Y+1, int(w)/TraySlots)
		if !ok || got != w {
			t.Fatal("tray selected wrong weapon", w, got)
		}
		origin := MenuOrigin(Width-1, Height-1)
		box = MenuWeaponRect(origin, w)
		got, ok = MenuWeaponAt(origin, box.Min.X+1, box.Min.Y+1)
		if !ok || got != w || !box.In(MenuRect(origin)) {
			t.Fatal("popup selected wrong weapon or clipped tile", w)
		}
	}
	if _, ok := WeaponAt(40, 751, WeaponPages); ok {
		t.Fatal("empty final-page slot selected weapon")
	}
	s, _ := sim.New(sim.DefaultConfig())
	before := replay.Hash(s)
	r := New(s)
	canvas := NewRaster()
	defer canvas.Close()
	r.Draw(canvas, s, View{Arena: true, WeaponMenu: true, MenuOrigin: MenuOrigin(900, 800), HideBottom: true, HideTop: true, Camera: &Camera{Zoom: 1, Bounds: Stage(true, true)}})
	if replay.Hash(s) != before {
		t.Fatal("arsenal popup changed simulation")
	}
}
