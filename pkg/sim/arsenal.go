package sim

import (
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

// Unlimited is reserved for repeatable equipment in this built-in scheme.
const Unlimited uint8 = 255

type Inventory [WeaponCount]uint8

func initialInventory() Inventory {
	var a Inventory
	for i := range a {
		a[i] = Unlimited
	}
	a[HomingRocket], a[Airstrike], a[HeavyCluster] = 3, 2, 1
	a[RemoteCharge], a[BisonBomb], a[Girder] = 3, 2, 4
	return a
}
func (s *State) AmmoCount(w Weapon) uint8 { return s.Ammo[s.ActiveSeat()][w] }
func (s *State) consume(w Weapon) bool {
	a := &s.Ammo[s.ActiveSeat()][w]
	if !s.WeaponAvailable(w) {
		return false
	}
	if *a != Unlimited {
		*a--
	}
	return true
}
func UsesFuse(w Weapon) bool {
	return w == Grenade || w == ClusterBomb || w == BouncingBomb || w == HeavyCluster
}
func UsesTarget(w Weapon) bool { return w == HomingRocket || w == Airstrike || w == Girder }
func Charged(w Weapon) bool {
	return w == Bazooka || w == Grenade || w == ClusterBomb || w == Mortar || w == BouncingBomb || w == DrillRocket || w == HomingRocket || w == HeavyCluster
}

// placeGirder checks the whole volume before writing any terrain. Construction
// is bounded in range and cannot entomb living units or projectiles.
func (s *State) CanPlaceGirder() bool {
	if !s.HasTarget || s.Target.Sub(s.Worms[s.Active].Pos).Len() > fixed.FromInt(240) {
		return false
	}
	x, y := s.Target.X.Int(), s.Target.Y.Int()
	if x < 40 || x+40 >= int(s.Config.Width) || y < 6 || fixed.FromInt(y+6) >= s.WaterY {
		return false
	}
	for yy := y - 6; yy < y+6; yy++ {
		for xx := x - 40; xx < x+40; xx++ {
			if s.Terrain.Solid(xx, yy) {
				return false
			}
		}
	}
	for _, w := range s.Worms {
		if w.HP > 0 && w.Pos.X.Int()+halfWidth >= x-40 && w.Pos.X.Int()-halfWidth < x+40 && w.Pos.Y.Int() >= y-6 && w.Pos.Y.Int()-bodyHeight < y+6 {
			return false
		}
	}
	for _, o := range s.Objects {
		if o.HP > 0 && o.Pos.X.Int()+10 >= x-40 && o.Pos.X.Int()-10 < x+40 && o.Pos.Y.Int() >= y-6 && o.Pos.Y.Int()-20 < y+6 {
			return false
		}
	}
	for _, p := range s.Projectiles {
		if !p.Dead && fixed.Abs(p.Pos.X-s.Target.X) < fixed.FromInt(42) && fixed.Abs(p.Pos.Y-s.Target.Y) < fixed.FromInt(8) {
			return false
		}
	}
	return true
}
func (s *State) placeGirder(effects *[]Effect) bool {
	if !s.CanPlaceGirder() {
		return false
	}
	x, y := s.Target.X.Int(), s.Target.Y.Int()
	if !s.consume(Girder) {
		return false
	}
	for yy := y - 6; yy < y+6; yy++ {
		for xx := x - 40; xx < x+40; xx++ {
			s.Terrain.Set(xx, yy)
		}
	}
	*effects = append(*effects, Effect{Dirty: terrain.Rect{X0: x - 40, Y0: y - 6, X1: x + 40, Y1: y + 6}})
	s.retreat()
	return true
}

// Unlock rounds are counted from zero and apply equally to every squad.
func UnlockRound(w Weapon) uint32 {
	switch w {
	case HomingRocket, Airstrike:
		return 1
	case HeavyCluster:
		return 2
	}
	return 0
}
func (s *State) UnlockTurns(w Weapon) uint32 {
	if !s.Config.WeaponDelays {
		return 0
	}
	round := s.Turn / uint32(s.Config.Seats)
	unlock := UnlockRound(w)
	if round >= unlock {
		return 0
	}
	return unlock - round
}
func (s *State) WeaponAvailable(w Weapon) bool { return s.AmmoCount(w) > 0 && s.UnlockTurns(w) == 0 }
