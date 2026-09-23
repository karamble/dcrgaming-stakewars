// Package sim is the deterministic, single-threaded game. Only Step changes a
// running match. Inputs and synthetic ticks are its only external influences.
package sim

import (
	"errors"

	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/rng"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

const Version uint32 = 17
const TickRate = 60

type Phase uint8

const (
	Playing Phase = iota
	Retreating
	Resolving
	Ended
	Ready
)

type Weapon uint8

const (
	Bazooka Weapon = iota
	Grenade
	ClusterBomb
	Shotgun
	Uzi
	FirePunch
	Dynamite
	Mine
	Teleport
	NinjaRope
	Jetpack
	Mortar
	BaseballBat
	BouncingBomb
	DrillRocket
	Parachute
	HomingRocket
	Airstrike
	Flamethrower
	RemoteCharge
	HeavyCluster
	BisonBomb
	Girder
	MiningDrill
	WeaponCount
)

type EventKind uint8

const (
	Move EventKind = iota
	Aim
	Jump
	SelectWeapon
	Fire
	RopeAdjust
	Thrust
	TeleportTo
	Skip
	Surrender
	Face
	SetFuse
	SetTarget
)

type Input struct {
	Tick     uint32
	Seat     uint8
	Sequence uint16
	Kind     EventKind
	Param    int32
}

type Config struct {
	Seed                                     uint64
	Width, Height                            uint16
	Seats, Units                             uint8
	HP                                       int32
	TurnTicks, RetreatTicks, ResolutionTicks uint32
	SuddenDeathTurn                          uint32
	WaterRise                                uint16
	ReadyTicks                               uint32
	Environment                              bool
	WeaponDelays                             bool
}

func DefaultConfig() Config {
	return Config{Seed: 42, Width: 4096, Height: 768, Seats: 2, Units: 4, HP: 100,
		ReadyTicks: 180, Environment: true, WeaponDelays: true, TurnTicks: 2700, RetreatTicks: 180, ResolutionTicks: 3600, SuddenDeathTurn: 40, WaterRise: 6}
}

func (c Config) Validate() error {
	if c.Width < 640 || c.Width > 4096 || c.Height < 384 || c.Height > 2048 {
		return errors.New("map dimensions out of range")
	}
	if c.Seats < 2 || c.Seats > 6 || c.Units < 1 || c.Units > 8 {
		return errors.New("invalid squad count")
	}
	if c.HP < 1 || c.HP > 1000 {
		return errors.New("invalid health")
	}
	if c.ReadyTicks > 600 {
		return errors.New("invalid ready duration")
	}
	if c.TurnTicks < 1 || c.TurnTicks > 7200 || c.RetreatTicks > 600 || c.ResolutionTicks < 1 || c.ResolutionTicks > 3600 {
		return errors.New("invalid phase duration")
	}
	if c.SuddenDeathTurn > 10000 || c.WaterRise > 64 {
		return errors.New("invalid sudden death")
	}
	return nil
}

type Worm struct {
	Pos, Vel fixed.Vec // position is the centre of the feet
	Apex     fixed.F
	HP       int32
	Seat     uint8
	Facing   int8
	Grounded bool
	Chute    bool
}

type Projectile struct {
	Pos, Vel       fixed.Vec
	Fuse           uint32
	Damage, Radius int32
	Owner          uint8
	Weapon         Weapon
	Burning        bool
	Dead           bool
	Age            uint32
	Drill          uint16
	DrillSlow      uint8 // 0..6: terrain drag, recovers over six clear-flight ticks
	Target         fixed.Vec
}

type Pivot struct {
	Pos  fixed.Vec
	Side int8
}
type Rope struct {
	Pivots []Pivot
	Length fixed.F
}

type State struct {
	Config                Config
	Tick, Turn, PhaseTick uint32
	Phase                 Phase
	Winner                int8 // -1 until decided, or a draw
	Terrain               terrain.Mask
	WaterY, Wind          fixed.F
	Worms                 []Worm
	Projectiles           []Projectile
	Active                uint16
	Next                  []uint8
	Aim                   fixed.Angle
	Weapon                Weapon
	Walk, Thrust          int8
	Shots                 uint8
	Rope                  Rope
	Rng                   rng.PCG
	FuseSeconds           uint8
	Target                fixed.Vec
	HasTarget             bool
	Ammo                  []Inventory
	Objects               []Object
	EnvironmentRng        rng.PCG
}

// Effect is a transient description for the renderer, not part of State and
// never fed back to the simulation.
type Effect struct {
	Pickup      bool
	Fired       bool
	Jumped      bool
	Unit        uint16
	Variant     int32
	HazardDeath bool
	Seat        uint8
	Shot        bool
	From        fixed.Vec
	Weapon      Weapon
	Pos         fixed.Vec
	Radius      int32
	Dirty       terrain.Rect
}

func New(c Config) (*State, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	s := &State{Config: c, Winner: -1, Terrain: terrain.New(int(c.Width), int(c.Height), false),
		WaterY: fixed.FromInt(int(c.Height) * 15 / 16), Rng: rng.New(c.Seed, 54), Next: make([]uint8, c.Seats)}
	s.Ammo = make([]Inventory, c.Seats)
	for i := range s.Ammo {
		s.Ammo[i] = initialInventory()
	}
	s.buildTerrain()
	if err := s.spawnSquads(); err != nil {
		return nil, err
	}

	s.EnvironmentRng = rng.New(c.Seed, 73)
	if c.Environment {
		s.seedObjects()
	}
	s.begin(0)
	s.Next[0] = 1 % c.Units
	return s, nil
}

func (s *State) Clone() *State {
	v := *s
	v.Objects = append([]Object(nil), s.Objects...)
	v.Terrain = s.Terrain.Clone()
	v.Worms = append([]Worm(nil), s.Worms...)
	v.Projectiles = append([]Projectile(nil), s.Projectiles...)
	v.Ammo = append([]Inventory(nil), s.Ammo...)
	v.Next = append([]uint8(nil), s.Next...)
	v.Rope.Pivots = append([]Pivot(nil), s.Rope.Pivots...)
	return &v
}

func (s *State) ActiveSeat() uint8 { return s.Worms[s.Active].Seat }
func (s *State) RemainingTicks() uint32 {
	if s.Phase != Playing || s.PhaseTick >= s.Config.TurnTicks {
		return 0
	}
	return s.Config.TurnTicks - s.PhaseTick
}
func (s *State) Quiescent() bool {
	if s.objectsMoving() {
		return false
	}
	if len(s.Projectiles) > 0 {
		return false
	}
	for _, w := range s.Worms {
		if w.HP > 0 && !w.Grounded {
			return false
		}
	}
	return true
}

func validateInputs(s *State, in []Input) error {
	if len(in) > 64 {
		return errors.New("too many inputs in one tick")
	}
	for i, v := range in {
		if v.Tick != s.Tick || v.Seat != s.ActiveSeat() || (i > 0 && in[i-1].Sequence >= v.Sequence) {
			return errors.New("invalid tick, seat or input sequence")
		}
		if s.Phase == Ended || v.Kind > SetTarget {
			return errors.New("input outside active match")
		}
		switch v.Kind {
		case Move, RopeAdjust, Thrust, Face:
			if v.Param < -1 || v.Param > 1 {
				return errors.New("invalid direction")
			}
		case Aim:
			if v.Param < 0 || v.Param > 65535 {
				return errors.New("invalid angle")
			}
		case SelectWeapon:
			if v.Param < 0 || v.Param >= int32(WeaponCount) {
				return errors.New("unknown weapon")
			}
		case Fire:
			if v.Param < 1 || v.Param > 1000 {
				return errors.New("invalid power")
			}
		case Jump:
			if v.Param < 0 || v.Param > 2 {
				return errors.New("invalid jump")
			}
		case SetFuse:
			if v.Param < 1 || v.Param > 5 {
				return errors.New("invalid fuse")
			}
		case SetTarget:
			x, y := int(uint32(v.Param)&65535), int(uint32(v.Param)>>16)
			if x >= int(s.Config.Width) || y >= int(s.Config.Height) {
				return errors.New("target outside arena")
			}
		case TeleportTo:
			x, y := int(uint32(v.Param)&65535), int(uint32(v.Param)>>16)
			if x < 6 || x >= int(s.Config.Width)-6 || y < 24 || y >= int(s.Config.Height) {
				return errors.New("teleport outside arena")
			}
		default:
			if v.Param != 0 {
				return errors.New("unexpected input parameter")
			}
		}
	}
	return nil
}

// Step validates the complete tick before changing state. Input sequence numbers
// encode the author's order; receipt order must never decide gameplay.
func Step(s *State, in []Input) ([]Effect, error) {
	if err := validateInputs(s, in); err != nil {
		return nil, err
	}
	if s.Phase == Ended {
		return nil, nil
	}
	var effects []Effect
	if s.Phase == Ready {
		if len(in) == 0 {
			s.Tick++
			s.PhaseTick++
			if s.PhaseTick >= s.Config.ReadyTicks {
				s.Phase = Playing
				s.PhaseTick = 0
			}
			return nil, nil
		}
		s.Phase = Playing
		s.PhaseTick = 0
	}
	for _, v := range in {
		s.apply(v, &effects)
	}
	for i := range s.Worms {
		wasAlive := s.Worms[i].HP > 0
		s.moveWorm(i)
		w := &s.Worms[i]
		if w.HP > 0 && w.Pos.Y >= s.WaterY {
			w.HP = 0
		}
		if wasAlive && w.HP <= 0 && w.Pos.Y >= s.WaterY {
			effects = append(effects, Effect{HazardDeath: true, Seat: w.Seat, Pos: fixed.Vec{X: w.Pos.X, Y: s.WaterY}})
		}
	}
	s.moveProjectiles(&effects)
	s.moveObjects(&effects)
	s.Tick++
	s.PhaseTick++
	if s.Worms[s.Active].HP <= 0 && s.Phase == Playing {
		s.retreat()
	}
	switch s.Phase {
	case Playing:
		if s.PhaseTick >= s.Config.TurnTicks {
			s.retreat()
		}
	case Retreating:
		if s.PhaseTick >= s.Config.RetreatTicks {
			s.Phase = Resolving
			s.PhaseTick = 0
			s.Walk = 0
			s.Thrust = 0
			s.Rope = Rope{}
		}
	case Resolving:
		if s.PhaseTick >= s.Config.ResolutionTicks {
			// Remaining projectiles resolve once; spawned clusters resolve on
			// subsequent ticks before the next turn, preserving stable order.
			for i := range s.Projectiles {
				s.Projectiles[i].Fuse = 1
			}
		}
		if s.Quiescent() {
			s.nextTurn()
		}
	}
	return effects, nil
}

func (s *State) apply(v Input, effects *[]Effect) {
	w := &s.Worms[s.Active]
	if w.HP <= 0 || s.Phase == Resolving {
		return
	}
	switch v.Kind {
	case Move:
		s.Walk = int8(v.Param)
		if v.Param != 0 {
			w.Facing = int8(v.Param)
			s.Aim = FacingAim(s.Aim, w.Facing)
		}
	case Face:
		if v.Param != 0 {
			w.Facing = int8(v.Param)
			s.Aim = FacingAim(s.Aim, w.Facing)
		}
	case SetFuse:
		if s.Phase == Playing {
			s.FuseSeconds = uint8(v.Param)
		}
	case SetTarget:
		if s.Phase == Playing {
			s.Target = fixed.Vec{X: fixed.FromInt(int(uint32(v.Param) & 65535)), Y: fixed.FromInt(int(uint32(v.Param) >> 16))}
			s.HasTarget = true
		}
	case Aim:
		s.Aim = fixed.Angle(v.Param)
	case Jump:
		if w.Grounded {
			w.Vel.Y = -fixed.Ratio(11, 2)
			w.Vel.X = fixed.Ratio(int32(w.Facing)*9, 4)
			if v.Param == 1 {
				w.Vel.Y = -fixed.FromInt(7)
				w.Vel.X = 0
			}
			if v.Param == 2 {
				w.Vel.Y = -fixed.Ratio(15, 2)
				w.Vel.X = -fixed.Ratio(int32(w.Facing)*9, 4)
			}
			w.Chute = false
			w.Grounded = false
			w.Apex = w.Pos.Y
			*effects = append(*effects, Effect{Jumped: true, Unit: s.Active, Variant: v.Param, Pos: w.Pos})
		}
	case SelectWeapon:
		if s.Phase == Playing && s.Shots == 0 && s.WeaponAvailable(Weapon(v.Param)) {
			s.Weapon = Weapon(v.Param)
			s.Thrust = 0
			s.Shots = 0
		}
	case Fire:
		if s.Phase == Playing || (s.Phase == Retreating && s.Weapon == Parachute) {
			s.fire(v.Param, effects)
		}
	case RopeAdjust:
		if len(s.Rope.Pivots) > 0 {
			s.Rope.Length = fixed.Clamp(s.Rope.Length+fixed.FromInt(int(v.Param)*2), fixed.FromInt(16), fixed.FromInt(600))
		}
	case Thrust:
		if s.Weapon == Jetpack && s.Phase == Playing {
			s.Thrust = int8(v.Param)
		}
	case TeleportTo:
		if s.Phase == Playing && s.Weapon == Teleport {
			p := fixed.Vec{X: fixed.FromInt(int(uint32(v.Param) & 65535)), Y: fixed.FromInt(int(uint32(v.Param) >> 16))}
			if !s.blocked(p) {
				w.Pos = p
				w.Vel = fixed.Vec{}
				w.Apex = p.Y
				w.Grounded = false
				s.Rope = Rope{}
				s.retreat()
			}
		}
	case Skip:
		if s.Phase == Playing {
			s.retreat()
		}
	case Surrender:
		for i := range s.Worms {
			if s.Worms[i].Seat == w.Seat {
				s.Worms[i].HP = 0
			}
		}
		s.retreat()
	}
}
func (s *State) retreat() {
	if s.Phase != Playing {
		return
	}
	if s.Weapon == RemoteCharge {
		for i := range s.Projectiles {
			if s.Projectiles[i].Weapon == RemoteCharge && s.Projectiles[i].Owner == s.ActiveSeat() {
				s.Projectiles[i].Fuse = 1
			}
		}
	}
	s.Phase = Retreating
	s.PhaseTick = 0
	s.Thrust = 0
	s.Rope = Rope{}
}
func (s *State) begin(index int) {
	s.Active = uint16(index)
	s.Phase = Playing
	if s.Config.ReadyTicks > 0 {
		s.Phase = Ready
	}
	s.PhaseTick = 0
	s.Walk = 0
	s.Thrust = 0
	s.Shots = 0
	s.FuseSeconds = 3
	s.HasTarget = false
	s.Target = fixed.Vec{}
	s.Weapon = Bazooka
	s.Rope = Rope{}
	s.Aim = 57344 // 45 degrees above screen-right
	if s.Worms[index].Facing < 0 {
		s.Aim = 40960
	}
	s.Wind = fixed.Ratio(int32(s.Rng.Intn(41)-20), 1000)
}
func (s *State) nextTurn() {
	alive := make([]bool, s.Config.Seats)
	for _, w := range s.Worms {
		if w.HP > 0 {
			alive[w.Seat] = true
		}
	}
	count, last := 0, -1
	for i, v := range alive {
		if v {
			count++
			last = i
		}
	}
	if count <= 1 {
		s.Phase = Ended
		s.Winner = int8(last)
		return
	}
	s.Turn++
	if s.Config.Environment && s.Turn%uint32(s.Config.Seats) == 0 {
		s.spawnCrate()
	}
	if s.Config.SuddenDeathTurn > 0 && s.Turn >= s.Config.SuddenDeathTurn {
		s.WaterY -= fixed.FromInt(int(s.Config.WaterRise))
	}
	seat := s.ActiveSeat()
	for n := uint8(1); n <= s.Config.Seats; n++ {
		next := (seat + n) % s.Config.Seats
		if !alive[next] {
			continue
		}
		for j := uint8(0); j < s.Config.Units; j++ {
			u := (s.Next[next] + j) % s.Config.Units
			i := int(next)*int(s.Config.Units) + int(u)
			if s.Worms[i].HP > 0 {
				s.Next[next] = (u + 1) % s.Config.Units
				s.begin(i)
				return
			}
		}
	}
}

// FacingAim reflects horizontally while preserving the elevation of the shot.
func FacingAim(angle fixed.Angle, direction int8) fixed.Angle {
	x := fixed.Cos(angle)
	if (direction < 0 && x > 0) || (direction > 0 && x < 0) {
		return fixed.Angle(32768 - uint16(angle))
	}
	return angle
}
