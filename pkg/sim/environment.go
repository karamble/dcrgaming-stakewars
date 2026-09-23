package sim

import "github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"

type ObjectKind uint8

const (
	Barrel ObjectKind = iota
	ProximityMine
	HealthCrate
	AmmoCrate
)
const MaxObjects = 128

type Object struct {
	Kind      ObjectKind
	Pos       fixed.Vec
	VelY      fixed.F
	HP        int32
	Arm, Fuse uint32
	Owner     uint8
	Weapon    Weapon
	Settled   bool
}

func (s *State) objectsMoving() bool {
	for _, o := range s.Objects {
		if o.HP >= 0 && (!o.Settled || o.Fuse > 0 || o.HP == 0) {
			return true
		}
	}
	return false
}
func (s *State) objectSite() (fixed.Vec, bool) {
	for attempt := 0; attempt < 80; attempt++ {
		x := 24 + int(s.EnvironmentRng.Intn(int(s.Config.Width)-48))
		for y := 32; y < s.WaterY.Int()-20; y++ {
			p := fixed.Vec{X: fixed.FromInt(x), Y: fixed.FromInt(y)}
			if !s.supported(p) || s.blocked(p) {
				continue
			}
			clear := true
			for _, w := range s.Worms {
				if w.HP > 0 && w.Pos.Sub(p).Len() < fixed.FromInt(64) {
					clear = false
					break
				}
			}
			for _, o := range s.Objects {
				if o.HP > 0 && o.Pos.Sub(p).Len() < fixed.FromInt(40) {
					clear = false
					break
				}
			}
			if clear {
				return p, true
			}
		}
	}
	return fixed.Vec{}, false
}
func (s *State) addObject(kind ObjectKind) {
	if len(s.Objects) >= MaxObjects {
		return
	}
	p, ok := s.objectSite()
	if !ok {
		return
	}
	weapon := []Weapon{HomingRocket, Airstrike, RemoteCharge, HeavyCluster, BisonBomb, Girder}[s.EnvironmentRng.Intn(6)]
	s.Objects = append(s.Objects, Object{Kind: kind, Pos: p, HP: 20, Settled: true, Weapon: weapon})
}
func (s *State) seedObjects() {
	for i := 0; i < max(1, int(s.Config.Width)/800); i++ {
		s.addObject(Barrel)
	}
	s.addObject(HealthCrate)
	s.addObject(AmmoCrate)
}
func (s *State) spawnCrate() {
	count := 0
	for _, o := range s.Objects {
		if o.HP > 0 && (o.Kind == HealthCrate || o.Kind == AmmoCrate) {
			count++
		}
	}
	if count >= 4 {
		return
	}
	kind := HealthCrate
	if s.EnvironmentRng.Intn(2) == 1 {
		kind = AmmoCrate
	}
	s.addObject(kind)
}
func (s *State) damageObjects(p Projectile) {
	for i := range s.Objects {
		o := &s.Objects[i]
		center := o.Pos
		center.Y -= fixed.FromInt(8)
		if o.HP > 0 && center.Sub(p.Pos).Len() < fixed.FromInt(int(p.Radius)) {
			o.HP = 0
		}
	}
}
func (s *State) moveObjects(effects *[]Effect) {
	for i := range s.Objects {
		o := &s.Objects[i]
		if o.HP <= 0 {
			continue
		}
		if o.Pos.Y >= s.WaterY {
			o.HP = -1
			continue
		}
		if !s.supported(o.Pos) {
			o.Settled = false
		}
		if !o.Settled {
			o.VelY = fixed.Min(o.VelY+fixed.Ratio(1, 8), fixed.FromInt(12))
			steps := max(1, o.VelY.Int()+1)
			dy := o.VelY / fixed.F(steps)
			for n := 0; n < steps; n++ {
				p := o.Pos
				p.Y += dy
				if s.blocked(p) {
					o.Settled = true
					o.VelY = 0
					break
				}
				o.Pos = p
			}
			if o.Pos.Y >= s.WaterY {
				o.HP = -1
				continue
			}
		}
		if o.Kind == ProximityMine {
			if o.Arm > 0 {
				o.Arm--
			} else if o.Fuse == 0 {
				for _, w := range s.Worms {
					if w.HP > 0 && w.Pos.Sub(o.Pos).Len() < fixed.FromInt(32) {
						o.Fuse = 30
						break
					}
				}
			}
			if o.Fuse > 0 {
				o.Fuse--
				if o.Fuse == 0 {
					o.HP = 0
				}
			}
		}
		if o.Kind == HealthCrate || o.Kind == AmmoCrate {
			for j := range s.Worms {
				w := &s.Worms[j]
				if w.HP <= 0 || !crateContact(w.Pos, o.Pos) {
					continue
				}
				if o.Kind == HealthCrate {
					w.HP = min(s.Config.HP, w.HP+25)
				} else {
					a := &s.Ammo[w.Seat][o.Weapon]
					if *a < 20 {
						*a++
					}
				}
				o.HP = -1
				*effects = append(*effects, Effect{Pickup: true, Unit: uint16(j), Pos: o.Pos, Weapon: o.Weapon, Variant: int32(o.Kind)})
				break
			}
		}
	}
	// Bounded stable-order passes finish chains even when a later barrel destroys
	// an earlier one. Mark processed objects before explosions can revisit them.
	for pass := 0; pass < len(s.Objects); pass++ {
		changed := false
		for i := range s.Objects {
			o := s.Objects[i]
			if o.HP != 0 {
				continue
			}
			s.Objects[i].HP = -1
			changed = true
			if o.Kind == Barrel || o.Kind == ProximityMine {
				weapon := Dynamite
				damage, radius := int32(60), int32(52)
				if o.Kind == ProximityMine {
					weapon = Mine
					damage = 45
					radius = 38
				}
				pos := o.Pos
				pos.Y -= fixed.FromInt(6)
				s.explode(Projectile{Pos: pos, Damage: damage, Radius: radius, Weapon: weapon, Owner: o.Owner}, effects)
			}
		}
		if !changed {
			break
		}
	}
	out := s.Objects[:0]
	for _, o := range s.Objects {
		if o.HP > 0 {
			out = append(out, o)
		}
	}
	s.Objects = out
}

// Feet-anchored body bounds match the visible 24x22 crate footprint.
func crateContact(w, crate fixed.Vec) bool {
	return fixed.Abs(w.X-crate.X) <= fixed.FromInt(halfWidth+12) &&
		w.Y > crate.Y-fixed.FromInt(22) && w.Y-fixed.FromInt(bodyHeight) < crate.Y
}
