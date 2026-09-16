package render

import (
	"fmt"
	"image/color"
	"math"

	"github.com/karamble/dcrstakewars/assets/objects"
	"github.com/karamble/dcrstakewars/assets/weapons"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

const AnimationSpeed = 3

type SoundKind uint8

const (
	SoundStep SoundKind = iota
	SoundJump
	SoundLand
	SoundFire
	SoundBlast
	SoundHurt
	SoundPickup
	SoundReady
)

type SoundCue struct {
	Kind   SoundKind
	Weapon sim.Weapon
	X      float64
}
type motion struct {
	walk                     float64
	jump, land, hurt, recoil int
	backflip                 bool
	turn, mood, fireFace     int
	firedWeapon              sim.Weapon
	deathAge                 int
}

// Expressions are presentation-only and never enter simulation or replay state.
type expression uint8

const (
	faceSmile expression = iota
	faceSmirk
	faceFocus
	faceGrit
	faceHurt
	faceAngry
	faceStartled
	faceHappy
)

type floater struct {
	label  string
	x, y   float64
	amount int32
	age    int
}
type pose struct {
	stride, sx, sy, angle, recoil float64
	blink, hurt                   bool
	face                          expression
	lookX, lookY, brace           float64
	bob, tuck, trail, handLift    float64
	jetpack, thrust               bool
}

func (r *Scene) initMotion(s *sim.State) {
	r.previous = append([]sim.Worm(nil), s.Worms...)
	r.motion = make([]motion, len(s.Worms))
	for i, w := range s.Worms {
		if w.HP <= 0 {
			r.motion[i].deathAge = 60
		}
	}
	r.lastPhase = s.Phase
}
func (r *Scene) updateMotion(s *sim.State, effects []sim.Effect) {
	r.cues = r.cues[:0]
	if len(r.previous) != len(s.Worms) {
		r.initMotion(s)
	}
	out := r.floaters[:0]
	for _, f := range r.floaters {
		f.age++
		if f.age < 70 {
			out = append(out, f)
		}
	}
	r.floaters = out
	for i, w := range s.Worms {
		old := r.previous[i]
		m := &r.motion[i]
		if w.HP <= 0 {
			m.deathAge = min(60, m.deathAge+1)
		}
		m.turn = max(0, m.turn-AnimationSpeed)
		m.fireFace = max(0, m.fireFace-AnimationSpeed)
		if m.mood > 0 {
			m.mood = max(0, m.mood-AnimationSpeed)
		} else if m.mood < 0 {
			m.mood = min(0, m.mood+AnimationSpeed)
		}
		if w.Facing != old.Facing && w.HP > 0 {
			m.turn = 24
		}
		if m.land > 0 {
			m.land = max(0, m.land-AnimationSpeed)
		}
		if m.hurt > 0 {
			m.hurt = max(0, m.hurt-AnimationSpeed)
		}
		if m.recoil > 0 {
			m.recoil = max(0, m.recoil-AnimationSpeed)
		}
		if !w.Grounded {
			m.jump += AnimationSpeed
		} else {
			m.jump = 0
			m.backflip = false
		}
		if w.Grounded && !old.Grounded {
			m.land = 10
			r.cues = append(r.cues, SoundCue{Kind: SoundLand, X: float64(w.Pos.X) / 65536})
		}
		dx := math.Abs(float64(w.Pos.X-old.Pos.X) / 65536)
		if w.HP > 0 && w.Grounded && dx > 0.01 {
			before := int(m.walk / math.Pi)
			m.walk += dx * 0.24 * AnimationSpeed
			if int(m.walk/math.Pi) != before {
				r.cues = append(r.cues, SoundCue{Kind: SoundStep, X: float64(w.Pos.X) / 65536})
			}
		} else {
			m.walk = 0
		}
		if w.HP != old.HP {
			delta := max(int32(0), w.HP) - max(int32(0), old.HP)
			if delta != 0 && len(r.floaters) < 96 {
				r.floaters = append(r.floaters, floater{x: float64(w.Pos.X) / 65536, y: float64(w.Pos.Y)/65536 - 35, amount: delta})
			}
			if delta < 0 {
				m.hurt = 18
				m.mood = 66
				r.cues = append(r.cues, SoundCue{Kind: SoundHurt, X: float64(w.Pos.X) / 65536})
			}
		}
		r.previous[i] = w
	}
	for _, e := range effects {
		x := float64(e.Pos.X) / 65536
		if e.Jumped && int(e.Unit) < len(r.motion) {
			r.motion[e.Unit].jump = 1
			r.motion[e.Unit].backflip = e.Variant == 2
			r.cues = append(r.cues, SoundCue{Kind: SoundJump, X: x})
		}
		if e.Fired && int(e.Unit) < len(r.motion) {
			r.motion[e.Unit].recoil = 12
			r.motion[e.Unit].fireFace = 30
			r.motion[e.Unit].firedWeapon = e.Weapon
			r.cues = append(r.cues, SoundCue{Kind: SoundFire, Weapon: e.Weapon, X: x})
		}
		if e.Radius > 0 {
			r.cues = append(r.cues, SoundCue{Kind: SoundBlast, Weapon: e.Weapon, X: x})
		}
		if e.Pickup {
			if int(e.Unit) < len(r.motion) && r.motion[e.Unit].mood == 0 {
				r.motion[e.Unit].mood = -48
			}
			if e.Variant == int32(sim.AmmoCrate) && len(r.floaters) < 96 {
				r.floaters = append(r.floaters, floater{x: x, y: float64(e.Pos.Y)/65536 - 35, label: "AMMO · " + WeaponName(e.Weapon)})
			}
			r.cues = append(r.cues, SoundCue{Kind: SoundPickup, X: x})
		}
	}
	if s.Phase == sim.Ready && (r.lastPhase != sim.Ready || s.PhaseTick%60 == 0) {
		r.cues = append(r.cues, SoundCue{Kind: SoundReady, X: float64(s.Worms[s.Active].Pos.X) / 65536})
	}
	r.lastPhase = s.Phase
}

// DrainSounds is presentation-only; neither device timing nor mute feeds back.
func (r *Scene) DrainSounds() []SoundCue {
	cues := append([]SoundCue(nil), r.cues...)
	r.cues = r.cues[:0]
	return cues
}
func (r *Scene) actorPose(i int, s *sim.State) pose {
	p := pose{sx: 1, sy: 1, blink: (s.Tick*AnimationSpeed+uint32(i)*37)%211 < 7}
	if i >= len(r.motion) {
		return p
	}
	m := r.motion[i]
	p.stride = math.Sin(m.walk) * 3
	p.hurt = m.hurt > 0
	w := s.Worms[i]
	p.lookX = float64(w.Facing) * .8
	idle := (s.Tick + uint32(i)*97) % 360
	if m.walk != 0 {
		p.face = faceFocus
		p.angle = float64(w.Facing) * .055
		p.bob = -math.Abs(math.Sin(m.walk)) * .8
	} else if idle > 260 && w.Grounded {
		p.face = faceSmirk
		p.lookX *= -1
	}
	if i == int(s.Active) && s.Phase != sim.Ended {
		a := float64(s.Aim) * 2 * math.Pi / 65536
		p.lookX, p.lookY = math.Cos(a)*1.1, math.Sin(a)*1.1
	}
	if !w.Grounded && w.Vel.Y > 0 && m.jump > 24 {
		p.face = faceStartled
	}
	// Transport poses follow verified position/velocity, never input timing.
	vx, vy := float64(w.Vel.X)/65536, float64(w.Vel.Y)/65536
	if !w.Grounded {
		p.trail = math.Max(-3, math.Min(3, -vx*.6))
		p.tuck = math.Max(0, math.Min(3, -vy*.35))
		p.handLift = 2
		p.angle += math.Max(-.18, math.Min(.18, vx*.035))
		if w.Chute {
			p.handLift = 13
			p.face = faceFocus
		}
	}
	if i == int(s.Active) && s.Phase != sim.Ended {
		if len(s.Rope.Pivots) > 0 && !w.Grounded {
			anchor := s.Rope.Pivots[len(s.Rope.Pivots)-1].Pos
			dx, dy := float64(anchor.X-w.Pos.X), float64(anchor.Y-w.Pos.Y)
			p.angle = math.Max(-.55, math.Min(.55, math.Atan2(dx, -dy)*.45))
			p.handLift = 13
			p.tuck = 1.5
			p.face = faceFocus
			if math.Abs(vx) > 5 {
				p.face = faceHappy
			}
		}
		p.jetpack = s.Weapon == sim.Jetpack
		p.thrust = p.jetpack && s.Thrust > 0 && s.Phase == sim.Playing
		if p.thrust {
			p.angle = math.Max(-.25, math.Min(.25, vx*.055+float64(s.Walk)*.08))
			p.face = faceFocus
			p.tuck = 0
			p.handLift = 3
		}
	}
	if m.mood < 0 {
		p.face = faceHappy
	} else if m.mood > 0 {
		p.face = faceAngry
	}
	if m.fireFace > 0 {
		p.face = faceGrit
	}
	if p.hurt {
		p.face = faceHurt
		p.blink = false
	}
	if m.jump > 0 && m.jump <= 3 {
		p.sx = 1.12
		p.sy = .8
	} else if m.jump > 3 && m.jump < 12 {
		p.sx = .86
		p.sy = 1.12
	}
	if m.backflip && m.jump > 3 && m.jump < 42 {
		p.angle = -float64(s.Worms[i].Facing) * math.Min(1, float64(m.jump-3)/32) * 2 * math.Pi
	}
	if m.land > 0 {
		wave := math.Sin(float64(m.land) / 10 * math.Pi)
		p.sx += .2 * wave
		p.sy -= .25 * wave
	}
	// A quick narrow-and-open pivot; facing/aim already changed in the sim.
	if m.turn > 0 {
		pivot := math.Sin(float64(m.turn) / 27 * math.Pi)
		p.sx *= 1 - .38*pivot
		p.sy *= 1 + .06*pivot
		p.angle -= float64(w.Facing) * .09 * pivot
	}
	p.recoil = float64(m.recoil) / 12 * recoilStrength(m.firedWeapon)
	if p.hurt {
		p.sx *= 1.1
		p.sy *= .88
		p.angle += math.Sin(float64(m.hurt)*1.5) * .1
	}
	return p
}

// Charging belongs to the local view, not the authoritative game state.
func (r *Scene) presentedPose(i int, s *sim.State, power int) pose {
	p := r.actorPose(i, s)
	if i == int(s.Active) && s.Phase == sim.Playing && power > 0 && !p.hurt {
		p.brace = math.Min(1, float64(power)/1000)
		p.face = faceFocus
		if p.brace > .55 {
			p.face = faceGrit
		}
		p.blink = false
		p.sx *= 1 + .08*p.brace
		p.sy *= 1 - .08*p.brace
		p.angle -= float64(s.Worms[i].Facing) * .08 * p.brace
	}
	return p
}

func recoilStrength(w sim.Weapon) float64 {
	switch w {
	case sim.Bazooka, sim.Mortar, sim.DrillRocket, sim.HomingRocket, sim.Shotgun:
		return 4.5
	case sim.Grenade, sim.BouncingBomb, sim.BisonBomb, sim.Mine, sim.Dynamite:
		return -2 // A throwing follow-through, toward the target.
	case sim.Uzi:
		return 1.5
	default:
		return 2.5
	}
}

func (r *Scene) drawFloaters(c Canvas, margin, top, scale float64) {
	for _, f := range r.floaters {
		col := color.RGBA{255, 126, 111, uint8(255 * (70 - f.age) / 70)}
		if f.amount > 0 {
			col = color.RGBA{77, 245, 179, uint8(255 * (70 - f.age) / 70)}
		}
		label := fmt.Sprintf("%+d", f.amount)
		if f.label != "" {
			label = f.label
			col = color.RGBA{77, 245, 179, uint8(255 * (70 - f.age) / 70)}
		}
		c.Text(label, margin+f.x*scale-12, top+(f.y-float64(f.age)*.45)*scale, 18, col)
	}
}
func drawObject(c Canvas, o sim.Object, x, y, scale float64, tick uint32) {
	k := scale
	switch o.Kind {
	case sim.ProximityMine:
		sprite := weapons.Sprite(int(sim.Mine))
		c.Image(sprite, x-13*k, y-10*k, 26*k, 12*k)
		col := Muted
		if o.Arm == 0 {
			col = Mint
		}
		if o.Fuse > 0 && (tick/4)%2 == 0 {
			col = color.RGBA{255, 80, 60, 255}
		}
		c.Circle(x, y-8*k, 2*k, col)
	case sim.Barrel:
		c.Image(objects.Sprite(2), x-11*k, y-28*k, 22*k, 28*k)
	case sim.HealthCrate:
		c.Image(objects.Sprite(0), x-12*k, y-22*k, 24*k, 22*k)
	case sim.AmmoCrate:
		c.Image(objects.Sprite(1), x-12*k, y-22*k, 24*k, 22*k)
	}
}
