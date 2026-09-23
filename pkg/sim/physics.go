package sim

import "github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"

const halfWidth, bodyHeight = 6, 20

func (s *State) blocked(p fixed.Vec) bool {
	x, y := p.X.Int(), p.Y.Int()
	for yy := y - bodyHeight; yy < y; yy++ {
		for xx := x - halfWidth; xx <= x+halfWidth; xx++ {
			if s.Terrain.Solid(xx, yy) {
				return true
			}
		}
	}
	return false
}
func (s *State) supported(p fixed.Vec) bool {
	x, y := p.X.Int(), p.Y.Int()
	for xx := x - halfWidth; xx <= x+halfWidth; xx++ {
		if s.Terrain.Solid(xx, y) {
			return true
		}
	}
	return false
}
func (s *State) moveWorm(i int) {
	w := &s.Worms[i]
	if w.HP <= 0 {
		return
	}
	if w.Pos.Y >= s.WaterY || w.Pos.Y < -fixed.FromInt(int(s.Config.Height)) || w.Pos.X < 0 || w.Pos.X >= fixed.FromInt(int(s.Config.Width)) {
		w.HP = 0
		return
	}
	active := i == int(s.Active) && s.Phase != Resolving
	if active && len(s.Rope.Pivots) > 0 {
		if s.ropeSupported() {
			s.swing(w)
			return
		}
		s.Rope = Rope{}
		w.Grounded = false
	}
	if w.Grounded && !s.supported(w.Pos) {
		w.Grounded = false
		w.Apex = w.Pos.Y
	}
	if w.Grounded && active && s.Walk != 0 {
		dx := fixed.Ratio(int32(s.Walk)*3, 2)
		for step := 0; step < 2; step++ {
			p := w.Pos
			p.X += dx / 2
			for up := 0; up <= 4; up++ {
				candidate := p
				candidate.Y -= fixed.FromInt(up)
				if !s.blocked(candidate) {
					w.Pos = candidate
					break
				}
			}
			// Follow walkable downhill steps just as we climb small steps.
			// Only a unit that entered this step grounded can follow the floor.
			if !s.supported(w.Pos) {
				for down := 1; down <= 4; down++ {
					candidate := w.Pos
					candidate.Y += fixed.FromInt(down)
					if s.blocked(candidate) {
						break
					}
					if s.supported(candidate) {
						w.Pos = candidate
						w.Vel.Y = 0
						break
					}
				}
			}
			if !s.supported(w.Pos) {
				w.Grounded = false
				w.Apex = w.Pos.Y
				break // A genuine ledge: stop walking and let gravity take over.
			}
			w.Apex = w.Pos.Y
		}
	}
	if active && s.Weapon == Jetpack && s.Thrust != 0 {
		w.Vel.Y -= fixed.Ratio(int32(s.Thrust), 2)
		w.Vel.X += fixed.Ratio(int32(s.Walk), 10)
		w.Grounded = false
	}
	if w.Grounded {
		w.Chute = false
		return
	}
	w.Vel.Y += fixed.Ratio(3, 8)
	if active && s.Weapon == Jetpack && s.Thrust > 0 {
		w.Vel.Y = fixed.Max(w.Vel.Y, -fixed.FromInt(3))
	}
	if w.Chute && w.Vel.Y > 0 {
		w.Vel.Y = fixed.Min(w.Vel.Y, fixed.Ratio(3, 2))
		if active {
			w.Vel.X = fixed.Clamp(w.Vel.X+fixed.Ratio(int32(s.Walk), 12), -fixed.FromInt(3), fixed.FromInt(3))
		}
		w.Apex = w.Pos.Y
	}
	w.Vel.X = fixed.Clamp(w.Vel.X, -fixed.FromInt(24), fixed.FromInt(24))
	w.Vel.Y = fixed.Clamp(w.Vel.Y, -fixed.FromInt(24), fixed.FromInt(24))
	steps := int((fixed.Max(fixed.Abs(w.Vel.X), fixed.Abs(w.Vel.Y)) + fixed.One - 1) / fixed.One)
	if steps < 1 {
		steps = 1
	}
	start := w.Pos
	for step := 1; step <= steps; step++ {
		x := start.X + fixed.F(int64(w.Vel.X)*int64(step)/int64(steps))
		p := w.Pos
		p.X = x
		if !s.blocked(p) {
			w.Pos.X = x
		} else {
			w.Vel.X = 0
			start.X = w.Pos.X
		}
		y := start.Y + fixed.F(int64(w.Vel.Y)*int64(step)/int64(steps))
		p = w.Pos
		p.Y = y
		if !s.blocked(p) {
			w.Pos.Y = y
		} else {
			if w.Vel.Y > 0 {
				drop := (w.Pos.Y - w.Apex).Int()
				if drop > 64 {
					damage := (drop - 64) / 4
					if damage > 50 {
						damage = 50
					}
					w.HP -= int32(damage)
				}
				w.Grounded = true
				w.Apex = w.Pos.Y
			}
			w.Vel.Y = 0
			start.Y = w.Pos.Y
		}
		if w.Pos.Y < w.Apex {
			w.Apex = w.Pos.Y
		}
	}
}

// Projectile flight advances at the normal 60 Hz rate with swept collision.
func (s *State) moveProjectiles(effects *[]Effect) {
	n := len(s.Projectiles)
	for i := 0; i < n; i++ {
		p := s.Projectiles[i]
		if p.Dead {
			continue
		}
		p.Age++
		if p.Burning {
			if p.Age%15 == 0 {
				for j := range s.Worms {
					w := &s.Worms[j]
					center := w.Pos
					center.Y -= fixed.FromInt(10)
					if w.HP > 0 && center.Sub(p.Pos).Len() < fixed.FromInt(18) {
						w.HP -= 2
					}
				}
			}
			if p.Fuse > 0 {
				p.Fuse--
			}
			if p.Fuse == 0 {
				p.Dead = true
			}
			s.Projectiles[i] = p
			continue
		}
		if p.Weapon == HomingRocket && p.Age > 20 {
			desired := p.Target.Sub(p.Pos).Unit().Scale(fixed.FromInt(14))
			p.Vel = p.Vel.Scale(fixed.Ratio(15, 16)).Add(desired.Scale(fixed.Ratio(1, 16)))
		}
		if p.Fuse > 0 {
			p.Fuse--
			if p.Fuse == 0 {
				if p.Weapon == Flamethrower {
					p.Burning = true
					p.Vel = fixed.Vec{}
					p.Fuse = 120
					s.Projectiles[i] = p
					continue
				}
				s.Projectiles[i].Dead = true
				s.explode(p, effects)
				continue
			}
		}
		switch p.Weapon {
		case HomingRocket:
			// Propelled flight follows its target instead of a ballistic arc.
		case Bazooka, DrillRocket:
			p.Vel.Y += fixed.Ratio(1, 16)
		case Mortar:
			p.Vel.Y += fixed.Ratio(1, 12)
		default:
			p.Vel.Y += fixed.Ratio(1, 8)
		}
		if p.Weapon == Bazooka {
			p.Vel.X += s.Wind
		}
		p.Vel.X = fixed.Clamp(p.Vel.X, -fixed.FromInt(32), fixed.FromInt(32))
		p.Vel.Y = fixed.Clamp(p.Vel.Y, -fixed.FromInt(32), fixed.FromInt(32))
		travel := p.Vel
		if p.Weapon == DrillRocket && p.DrillSlow > 0 {
			// Preserve ballistic velocity; material reduces distance traveled this
			// tick, then six clear ticks smoothly restore full flight speed.
			travel = travel.Scale(fixed.Ratio(24-int32(p.DrillSlow), 24))
		}
		steps := int((fixed.Max(fixed.Abs(travel.X), fixed.Abs(travel.Y)) + fixed.One - 1) / fixed.One)
		if steps < 1 {
			steps = 1
		}
		start := p.Pos
		limit := steps
		drilled := false
		for step := 1; step <= limit; step++ {
			next := fixed.Vec{X: start.X + fixed.F(int64(travel.X)*int64(step)/int64(steps)), Y: start.Y + fixed.F(int64(travel.Y)*int64(step)/int64(steps))}
			if next.Y >= s.WaterY || next.X < 0 || next.X >= fixed.FromInt(int(s.Config.Width)) {
				p.Dead = true
				break
			}
			landHit := s.Terrain.Solid(next.X.Int(), next.Y.Int())
			if landHit && p.Weapon == DrillRocket && p.Drill > 0 {
				if !drilled && p.DrillSlow == 0 {
					limit = step + (steps-step)*3/4
				}
				drilled = true
				p.DrillSlow = 6
				p.Drill--
				r := s.Terrain.StampCircle(next.X.Int(), next.Y.Int(), 5, false)
				*effects = append(*effects, Effect{Dirty: r})
				p.Pos = next
				continue
			}
			if landHit && p.Weapon == BisonBomb {
				climbed := false
				for up := 1; up <= 5; up++ {
					if !s.Terrain.Solid(next.X.Int(), next.Y.Int()-up) {
						next.Y -= fixed.FromInt(up)
						p.Pos = next
						p.Vel.Y = 0
						climbed = true
						break
					}
				}
				if climbed {
					break
				}
			}
			hit := landHit
			if !hit {
				for j := range s.Objects {
					o := &s.Objects[j]
					if o.HP > 0 && fixed.Abs(next.X-o.Pos.X) < fixed.FromInt(10) && next.Y > o.Pos.Y-fixed.FromInt(20) && next.Y < o.Pos.Y {
						o.HP = 0
						hit = true
						break
					}
				}
			}
			if !hit {
				for _, w := range s.Worms {
					if w.HP > 0 && fixed.Abs(next.X-w.Pos.X) < fixed.FromInt(halfWidth) && next.Y > w.Pos.Y-fixed.FromInt(bodyHeight) && next.Y < w.Pos.Y {
						hit = true
						break
					}
				}
			}
			if hit {
				if p.Weapon == Flamethrower {
					p.Burning = true
					p.Vel = fixed.Vec{}
					p.Fuse = 120
					break
				}
				if p.Weapon == Bazooka || p.Weapon == Mortar || p.Weapon == DrillRocket || p.Weapon == HomingRocket || p.Weapon == Airstrike || p.Weapon == BisonBomb {
					p.Dead = true
					s.explode(p, effects)
				} else {
					// Approximate mask gradient; fixed fallback for flat/ambiguous contacts.
					x, y := next.X.Int(), next.Y.Int()
					nx, ny := int32(0), int32(0)
					for d := -1; d <= 1; d++ {
						if s.Terrain.Solid(x-1, y+d) {
							nx++
						}
						if s.Terrain.Solid(x+1, y+d) {
							nx--
						}
						if s.Terrain.Solid(x+d, y-1) {
							ny++
						}
						if s.Terrain.Solid(x+d, y+1) {
							ny--
						}
					}
					normal := fixed.Vec{X: fixed.FromInt(int(nx)), Y: fixed.FromInt(int(ny))}.Unit()
					if normal.X == 0 && normal.Y == 0 {
						normal = fixed.Vec{Y: -fixed.One}
					}
					bounce := fixed.Ratio(3, 5)
					if p.Weapon == BouncingBomb {
						bounce = fixed.Ratio(9, 10)
					}
					p.Vel = p.Vel.Sub(normal.Scale(fixed.Dot(p.Vel, normal) * 2)).Scale(bounce)
					if p.Weapon == Dynamite || p.Weapon == Mine || p.Weapon == RemoteCharge || p.Weapon == Flamethrower {
						p.Vel = fixed.Vec{}
					}
				}
				break
			}
			p.Pos = next
		}
		if p.Weapon == DrillRocket && !drilled && p.DrillSlow > 0 {
			p.DrillSlow--
		}
		s.Projectiles[i] = p
	}
	out := s.Projectiles[:0]
	for _, p := range s.Projectiles {
		if !p.Dead {
			out = append(out, p)
		}
	}
	s.Projectiles = out
}

func (s *State) explode(p Projectile, effects *[]Effect) {
	r := s.Terrain.StampCircle(p.Pos.X.Int(), p.Pos.Y.Int(), int(p.Radius), false)
	*effects = append(*effects, Effect{Pos: p.Pos, Radius: p.Radius, Dirty: r, Weapon: p.Weapon})
	for i := range s.Worms {
		w := &s.Worms[i]
		if w.HP <= 0 {
			continue
		}
		center := w.Pos
		center.Y -= fixed.FromInt(bodyHeight / 2)
		delta := center.Sub(p.Pos)
		distance := delta.Len()
		if distance >= fixed.FromInt(int(p.Radius)) {
			continue
		}
		strength := fixed.One - fixed.Div(distance, fixed.FromInt(int(p.Radius)))
		if p.Damage > 0 {
			damage := fixed.Mul(fixed.FromInt(int(p.Damage)), strength).Round()
			if damage < 1 {
				damage = 1
			}
			w.HP -= int32(damage)
		}
		if delta.X == 0 && delta.Y == 0 {
			delta.Y = -fixed.One
		}
		w.Vel = w.Vel.Add(delta.Unit().Scale(fixed.Mul(strength, fixed.FromInt(9))))
		w.Grounded = false
		w.Apex = w.Pos.Y
	}
	s.damageObjects(p)
	// Chain already-existing explosives, not children of this explosion.
	for i := range s.Projectiles {
		if s.Projectiles[i].Dead {
			continue
		}
		if s.Projectiles[i].Pos.Sub(p.Pos).Len() < fixed.FromInt(int(p.Radius)) {
			s.Projectiles[i].Fuse = 1
		}
	}
	if p.Weapon == ClusterBomb || p.Weapon == HeavyCluster {
		count := 5
		if p.Weapon == HeavyCluster {
			count = 9
		}
		for n := 0; n < count; n++ {
			a := fixed.Angle(32768 + uint16(s.Rng.Intn(32768)))
			s.Projectiles = append(s.Projectiles, Projectile{Pos: p.Pos, Vel: fixed.Vec{X: fixed.Cos(a) * 4, Y: fixed.Sin(a) * 4}, Fuse: 90, Damage: 22, Radius: 24, Owner: p.Owner, Weapon: Grenade})
		}
	}
}

func (s *State) fire(power int32, effects *[]Effect) {
	w := &s.Worms[s.Active]
	if s.Weapon == Parachute {
		w.Chute = !w.Chute
		return
	}
	if s.Weapon == RemoteCharge && s.Shots > 0 {
		s.retreat()
		return
	}
	if s.Weapon == Mine && len(s.Objects) >= MaxObjects {
		return
	}
	if !s.WeaponAvailable(s.Weapon) {
		return
	}
	if UsesTarget(s.Weapon) && !s.HasTarget {
		return
	}
	if s.Weapon == MiningDrill {
		s.dig(effects)
		return
	}
	if s.Weapon == Girder {
		s.placeGirder(effects)
		return
	}
	if s.Weapon == NinjaRope {
		if len(s.Rope.Pivots) > 0 {
			s.Rope = Rope{}
			return
		}
		s.attach(w)
		return
	}
	if s.Weapon == Teleport || s.Weapon == Jetpack {
		return
	}
	if !s.consume(s.Weapon) {
		return
	}
	*effects = append(*effects, Effect{Fired: true, Unit: s.Active, Weapon: s.Weapon, Pos: w.Pos})
	dir := fixed.Vec{X: fixed.Cos(s.Aim), Y: fixed.Sin(s.Aim)}
	origin := w.Pos
	origin.Y -= fixed.FromInt(12)
	origin = origin.Add(dir.Scale(fixed.FromInt(14)))
	if s.Weapon == Shotgun || s.Weapon == Uzi || s.Weapon == FirePunch || s.Weapon == BaseballBat {
		count, damage, distance := 1, 25, 1200
		if s.Weapon == Uzi {
			count, damage = 7, 5
		}
		if s.Weapon == FirePunch {
			damage, distance = 30, 42
		}
		if s.Weapon == BaseballBat {
			damage, distance = 20, 48
		}
		for n := 0; n < count; n++ {
			d := dir
			if s.Weapon == Uzi {
				a := s.Aim + fixed.Angle(int16(s.Rng.Intn(1001)-500))
				d = fixed.Vec{X: fixed.Cos(a), Y: fixed.Sin(a)}
			}
			end := origin
			for k := 0; k < distance; k++ {
				p := origin.Add(d.Scale(fixed.FromInt(k)))
				end = p
				if s.Terrain.Solid(p.X.Int(), p.Y.Int()) {
					s.explode(Projectile{Pos: p, Radius: 8, Damage: 0}, effects)
					break
				}
				hit := false
				for j := range s.Objects {
					o := &s.Objects[j]
					if o.HP > 0 && fixed.Abs(p.X-o.Pos.X) < fixed.FromInt(10) && p.Y > o.Pos.Y-fixed.FromInt(20) && p.Y < o.Pos.Y {
						o.HP = 0
						hit = true
						break
					}
				}
				if hit {
					break
				}
				for i := range s.Worms {
					target := &s.Worms[i]
					if target.HP > 0 && i != int(s.Active) && fixed.Abs(p.X-target.Pos.X) < fixed.FromInt(halfWidth) && p.Y > target.Pos.Y-fixed.FromInt(bodyHeight) && p.Y < target.Pos.Y {
						target.HP -= int32(damage)
						target.Vel = d.Scale(fixed.FromInt(4))
						if s.Weapon == BaseballBat {
							target.Vel = d.Scale(fixed.FromInt(14))
							target.Vel.Y -= fixed.FromInt(5)
						}
						target.Grounded = false
						target.Apex = target.Pos.Y
						hit = true
						break
					}
				}
				if hit {
					break
				}
			}
			*effects = append(*effects, Effect{Shot: true, From: origin, Pos: end, Weapon: s.Weapon})
		}
		if s.Weapon == Shotgun && s.Shots == 0 {
			s.Shots = 1
			return
		}
		s.retreat()
		return
	}
	if s.Weapon == Airstrike {
		for n := 0; n < 5; n++ {
			x := fixed.Clamp(s.Target.X+fixed.FromInt((n-2)*40), fixed.FromInt(2), fixed.FromInt(int(s.Config.Width)-3))
			s.Projectiles = append(s.Projectiles, Projectile{Pos: fixed.Vec{X: x, Y: fixed.FromInt(-80 - n*18)}, Vel: fixed.Vec{Y: fixed.FromInt(5)}, Fuse: 600, Damage: 35, Radius: 32, Weapon: Airstrike, Owner: w.Seat})
		}
		s.retreat()
		return
	}
	if s.Weapon == Flamethrower {
		for n := 0; n < 12; n++ {
			a := s.Aim + fixed.Angle(int16(s.Rng.Intn(3001)-1500))
			d := fixed.Vec{X: fixed.Cos(a), Y: fixed.Sin(a)}
			s.Projectiles = append(s.Projectiles, Projectile{Pos: origin, Vel: d.Scale(fixed.Ratio(int32(25+n), 5)), Fuse: uint32(15 + n*2), Damage: 7, Radius: 15, Owner: w.Seat, Weapon: Flamethrower})
		}
		s.retreat()
		return
	}
	p := Projectile{Pos: origin, Vel: dir.Scale(fixed.Ratio(power*12, 1000)), Damage: 50, Radius: 42, Owner: w.Seat, Weapon: s.Weapon}
	if UsesFuse(s.Weapon) {
		p.Fuse = uint32(s.FuseSeconds) * TickRate
	}
	switch s.Weapon {
	case Bazooka:
		p.Vel = dir.Scale(fixed.Ratio(power*16, 1000))
	case Mortar:
		p.Vel = dir.Scale(fixed.Ratio(power*20, 1000))
		p.Damage = 55
		p.Radius = 48
		p.Fuse = 600
	case BouncingBomb:
		p.Damage = 45
		p.Radius = 38
	case DrillRocket:
		p.Vel = dir.Scale(fixed.Ratio(power*16, 1000))
		p.Drill = 18
		p.Fuse = 480
	case HomingRocket:
		p.Vel = dir.Scale(fixed.Ratio(power*16, 1000))
		p.Target = s.Target
		p.Fuse = 480
		p.Damage = 45
	case HeavyCluster:
		p.Damage = 65
		p.Radius = 58
	case BisonBomb:
		p.Pos = w.Pos
		p.Pos.X += fixed.FromInt(int(w.Facing) * 18)
		p.Pos.Y -= fixed.FromInt(3)
		p.Vel = fixed.Vec{X: fixed.FromInt(int(w.Facing) * 3)}
		p.Fuse = 240
		p.Damage = 65
		p.Radius = 52
	}
	if s.Weapon == Dynamite || s.Weapon == Mine || s.Weapon == RemoteCharge {
		p.Pos = w.Pos
		p.Pos.X += fixed.FromInt(int(w.Facing) * 15)
		p.Pos.Y -= fixed.FromInt(2)
		p.Vel = fixed.Vec{}
		p.Fuse = 180
		if s.Weapon == Dynamite {
			p.Damage = 75
			p.Radius = 64
			p.Fuse = 240
		}
	}
	if s.Weapon == Mine {
		// Persistent proximity mines do not hold the turn open while idle.
		s.Objects = append(s.Objects, Object{Kind: ProximityMine, Pos: p.Pos, HP: 1, Arm: 120, Owner: w.Seat})
		s.retreat()
		return
	}
	if s.Weapon == RemoteCharge {
		p.Fuse = s.RemainingTicks() + 1
		p.Damage = 65
		p.Radius = 50
	}
	s.Projectiles = append(s.Projectiles, p)
	if s.Weapon == RemoteCharge {
		s.Shots = 1
		return
	}
	s.retreat()
}
