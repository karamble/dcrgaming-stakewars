//go:build desktop

package main

import "testing"

func TestChargeAutomaticallyFiresOnceUntilReleased(t *testing.T) {
	g := new(game)
	for i := 0; i < 83; i++ {
		if g.chargeShot(true) != 0 {
			t.Fatal("fired below full power")
		}
	}
	if got := g.chargeShot(true); got != 1000 {
		t.Fatal("did not fire at full power", got)
	}
	for i := 0; i < 300; i++ {
		if g.chargeShot(true) != 0 {
			t.Fatal("held key fired twice")
		}
	}
	if g.chargeShot(false) != 0 {
		t.Fatal("release after auto fire shot again")
	}
	for i := 0; i < 10; i++ {
		g.chargeShot(true)
	}
	if got := g.chargeShot(false); got != 120 {
		t.Fatal("manual release lost charge", got)
	}
	if g.chargeShot(false) != 0 {
		t.Fatal("manual release fired twice")
	}
}
