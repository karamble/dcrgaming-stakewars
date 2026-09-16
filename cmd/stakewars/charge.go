//go:build desktop

package main

// chargeShot emits one shot on release or at full power. Holding fire after an
// automatic shot cannot start another charge, even across a turn transition.
func (g *game) chargeShot(held bool) int {
	if !held {
		g.fireReleaseRequired = false
	}
	if g.fireReleaseRequired {
		return 0
	}
	if held {
		g.charging = true
		g.power += 12
		if g.power < 1000 {
			return 0
		}
		g.fireReleaseRequired = true
		g.power = 1000
	} else if !g.charging {
		return 0
	}
	power := g.power
	if power < 1 {
		power = 1
	}
	g.charging, g.power = false, 0
	return power
}
