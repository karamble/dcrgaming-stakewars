package sim

import (
	"errors"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

// buildTerrain composes built-in silhouettes, not a single-valued height map.
// Solids and carved openings create several traversable surfaces at one X.
func (s *State) buildTerrain() {
	width, height := int(s.Config.Width), int(s.Config.Height)
	count := width / 700
	if count < 1 {
		count = 1
	}
	for island := 0; island < count; island++ {
		start, end := island*width/count, (island+1)*width/count
		span := end - start
		mirror := s.Rng.Intn(2) == 1
		variant := (island + int(s.Rng.Intn(3))) % 3
		shift := int(s.Rng.Intn(41)) - 20
		point := func(x, y int) terrain.Point {
			if mirror {
				x = 1000 - x
			}
			return terrain.Point{X: start + x*span/1000, Y: (y + shift) * height / 1000}
		}
		poly := func(raw []terrain.Point) {
			points := make([]terrain.Point, len(raw))
			for i, p := range raw {
				points[i] = point(p.X, p.Y)
			}
			s.Terrain.StampPolygon(points, true)
		}
		hole := func(x, y, rx, ry int) {
			p := point(x, y)
			s.Terrain.StampEllipse(p.X, p.Y, rx*span/1000, ry*height/1000, false)
		}
		// A stepped lower route with an irregular, tapering underside.
		poly([]terrain.Point{{X: 25, Y: 660}, {X: 65, Y: 555}, {X: 130, Y: 590}, {X: 190, Y: 515}, {X: 235, Y: 550}, {X: 310, Y: 485}, {X: 370, Y: 575}, {X: 445, Y: 520}, {X: 520, Y: 495}, {X: 605, Y: 580}, {X: 695, Y: 515}, {X: 760, Y: 575}, {X: 880, Y: 490}, {X: 965, Y: 575}, {X: 975, Y: 690}, {X: 935, Y: 850}, {X: 785, Y: 810}, {X: 680, Y: 865}, {X: 490, Y: 820}, {X: 355, Y: 870}, {X: 205, Y: 815}, {X: 65, Y: 855}})
		switch variant {
		case 0:
			// Hooked tower: an upper perch hangs over a hollow bowl.
			poly([]terrain.Point{{X: 100, Y: 610}, {X: 125, Y: 365}, {X: 85, Y: 255}, {X: 155, Y: 280}, {X: 185, Y: 185}, {X: 235, Y: 235}, {X: 310, Y: 160}, {X: 345, Y: 255}, {X: 390, Y: 280}, {X: 410, Y: 350}, {X: 345, Y: 375}, {X: 320, Y: 540}, {X: 275, Y: 620}})
			hole(230, 395, 105, 130)
			// Broad arch with a second playable floor beneath the opening.
			poly([]terrain.Point{{X: 400, Y: 590}, {X: 455, Y: 325}, {X: 520, Y: 285}, {X: 600, Y: 325}, {X: 700, Y: 260}, {X: 825, Y: 310}, {X: 900, Y: 365}, {X: 855, Y: 420}, {X: 810, Y: 590}})
			hole(660, 520, 155, 155)
		case 1:
			// Overhanging shelf, narrow supporting pillar, and a split summit.
			poly([]terrain.Point{{X: 125, Y: 560}, {X: 150, Y: 290}, {X: 80, Y: 230}, {X: 155, Y: 175}, {X: 270, Y: 215}, {X: 320, Y: 165}, {X: 360, Y: 220}, {X: 445, Y: 235}, {X: 405, Y: 300}, {X: 290, Y: 345}, {X: 285, Y: 570}})
			hole(375, 415, 135, 105)
			poly([]terrain.Point{{X: 585, Y: 590}, {X: 590, Y: 315}, {X: 650, Y: 290}, {X: 680, Y: 165}, {X: 740, Y: 235}, {X: 790, Y: 210}, {X: 810, Y: 315}, {X: 915, Y: 275}, {X: 945, Y: 380}, {X: 865, Y: 425}, {X: 850, Y: 590}})
			hole(710, 470, 100, 140)
		case 2:
			// Cave with a roof route and a detached high platform across a gap.
			poly([]terrain.Point{{X: 115, Y: 595}, {X: 100, Y: 385}, {X: 155, Y: 260}, {X: 240, Y: 245}, {X: 300, Y: 175}, {X: 350, Y: 270}, {X: 435, Y: 245}, {X: 485, Y: 335}, {X: 455, Y: 530}, {X: 390, Y: 600}})
			hole(285, 485, 115, 145)
			poly([]terrain.Point{{X: 635, Y: 265}, {X: 675, Y: 200}, {X: 765, Y: 225}, {X: 825, Y: 180}, {X: 915, Y: 255}, {X: 875, Y: 320}, {X: 780, Y: 340}, {X: 705, Y: 315}})
			poly([]terrain.Point{{X: 680, Y: 600}, {X: 730, Y: 440}, {X: 780, Y: 410}, {X: 830, Y: 485}, {X: 900, Y: 450}, {X: 940, Y: 550}, {X: 915, Y: 655}})
		}
		// Irregular underside alcoves and an occasional shaft to the hazard.
		hole(180, 850, 95, 110)
		hole(790, 845, 105, 105)
		hole(490, 785, 65, 105)
		// Small scallops break up long polygon edges without replacing topology.
		for n := 0; n < 7; n++ {
			hole(90+n*130, 850+int(s.Rng.Intn(25)), 22+int(s.Rng.Intn(18)), 20+int(s.Rng.Intn(20)))
		}
	}
}

func (s *State) spawnSquads() error {
	type site struct{ x, y int }
	var sites []site
	// Candidates include cave floors and shelves, not only the topmost surface.
	for x := halfWidth + 8; x < int(s.Config.Width)-halfWidth-8; x += 8 {
		for y := bodyHeight + 8; y < s.WaterY.Int()-16; y++ {
			edge := false
			for xx := x - halfWidth; xx <= x+halfWidth; xx++ {
				if s.Terrain.Solid(xx, y) && !s.Terrain.Solid(xx, y-1) {
					edge = true
					break
				}
			}
			if !edge {
				continue
			}
			p := fixed.Vec{X: fixed.FromInt(x), Y: fixed.FromInt(y)}
			if !s.blocked(p) && s.supported(p) {
				sites = append(sites, site{x, y})
			}
		}
	}
	total := int(s.Config.Seats) * int(s.Config.Units)
	positions := make([]site, 0, total)
	for i := 0; i < total; i++ {
		desiredX := 32 + (int(s.Config.Width)-64)*(i+1)/(total+1)
		// Interleaved seats receive a mix of high, middle, and lower routes.
		desiredY := int(s.Config.Height) * (3 + (i/int(s.Config.Seats))%3*2) / 10
		best, score := -1, int(^uint(0)>>1)
		for j, p := range sites {
			clear := true
			for _, used := range positions {
				if absInt(p.x-used.x) < 22 && absInt(p.y-used.y) < 28 {
					clear = false
					break
				}
			}
			if !clear {
				continue
			}
			cost := absInt(p.x-desiredX)*3 + absInt(p.y-desiredY)
			if cost < score {
				best, score = j, cost
			}
		}
		if best < 0 {
			return errors.New("built-in terrain has insufficient safe spawn sites")
		}
		positions = append(positions, sites[best])
	}
	for seat := uint8(0); seat < s.Config.Seats; seat++ {
		for unit := uint8(0); unit < s.Config.Units; unit++ {
			p := positions[int(unit)*int(s.Config.Seats)+int(seat)]
			pos := fixed.Vec{X: fixed.FromInt(p.x), Y: fixed.FromInt(p.y)}
			s.Worms = append(s.Worms, Worm{Pos: pos, Apex: pos.Y, HP: s.Config.HP, Seat: seat, Facing: 1, Grounded: true})
		}
	}
	return nil
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
