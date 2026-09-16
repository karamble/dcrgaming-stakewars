package sim

import "encoding/binary"

// AppendCanonical encodes every gameplay field in a fixed order. Hashing lives
// outside sim so its import boundary remains stdlib plus sim subpackages.
// Bump Version whenever this layout or simulation behavior changes.
func (s *State) AppendCanonical(b []byte) []byte {
	b = append(b, []byte("StakeWars/state\x00")...)
	u32 := func(v uint32) { b = binary.LittleEndian.AppendUint32(b, v) }
	u16 := func(v uint16) { b = binary.LittleEndian.AppendUint16(b, v) }
	u8 := func(v uint8) { b = append(b, v) }
	flag := func(v bool) {
		if v {
			u8(1)
		} else {
			u8(0)
		}
	}
	u32(Version)
	b = binary.LittleEndian.AppendUint64(b, s.Config.Seed)
	u16(s.Config.Width)
	u16(s.Config.Height)
	u8(s.Config.Seats)
	u8(s.Config.Units)
	u32(uint32(s.Config.HP))
	u32(s.Config.TurnTicks)
	u32(s.Config.RetreatTicks)
	u32(s.Config.ResolutionTicks)
	u32(s.Config.SuddenDeathTurn)
	u16(s.Config.WaterRise)
	u32(s.Config.ReadyTicks)
	flag(s.Config.Environment)
	flag(s.Config.WeaponDelays)
	u32(s.Tick)
	u32(s.Turn)
	u32(s.PhaseTick)
	u8(uint8(s.Phase))
	u8(uint8(s.Winner))
	b = s.Terrain.AppendCanonical(b)
	u32(uint32(s.WaterY))
	u32(uint32(s.Wind))
	u32(uint32(len(s.Worms)))
	for _, w := range s.Worms {
		u32(uint32(w.Pos.X))
		u32(uint32(w.Pos.Y))
		u32(uint32(w.Vel.X))
		u32(uint32(w.Vel.Y))
		u32(uint32(w.Apex))
		u32(uint32(w.HP))
		u8(w.Seat)
		u8(uint8(w.Facing))
		flag(w.Grounded)
		flag(w.Chute)
	}
	u32(uint32(len(s.Projectiles)))
	for _, p := range s.Projectiles {
		u32(uint32(p.Pos.X))
		u32(uint32(p.Pos.Y))
		u32(uint32(p.Vel.X))
		u32(uint32(p.Vel.Y))
		u32(p.Fuse)
		u32(uint32(p.Damage))
		u32(uint32(p.Radius))
		u8(p.Owner)
		u8(uint8(p.Weapon))
		flag(p.Burning)
		flag(p.Dead)
		u32(p.Age)
		u16(p.Drill)
		u8(p.DrillSlow)
		u32(uint32(p.Target.X))
		u32(uint32(p.Target.Y))
	}
	u16(s.Active)
	u32(uint32(len(s.Next)))
	for _, v := range s.Next {
		u8(v)
	}
	u16(uint16(s.Aim))
	u8(uint8(s.Weapon))
	u8(uint8(s.Walk))
	u8(uint8(s.Thrust))
	u8(s.Shots)
	u8(s.FuseSeconds)
	u32(uint32(s.Target.X))
	u32(uint32(s.Target.Y))
	flag(s.HasTarget)
	u32(uint32(len(s.Ammo)))
	for _, inventory := range s.Ammo {
		for _, count := range inventory {
			u8(count)
		}
	}
	u32(uint32(len(s.Rope.Pivots)))
	for _, p := range s.Rope.Pivots {
		u32(uint32(p.Pos.X))
		u32(uint32(p.Pos.Y))
		u8(uint8(p.Side))
	}
	u32(uint32(s.Rope.Length))
	u32(uint32(len(s.Objects)))
	for _, o := range s.Objects {
		u8(uint8(o.Kind))
		u32(uint32(o.Pos.X))
		u32(uint32(o.Pos.Y))
		u32(uint32(o.VelY))
		u32(uint32(o.HP))
		u32(o.Arm)
		u32(o.Fuse)
		u8(o.Owner)
		u8(uint8(o.Weapon))
		flag(o.Settled)
	}
	envState, envInc := s.EnvironmentRng.Words()
	b = binary.LittleEndian.AppendUint64(b, envState)
	b = binary.LittleEndian.AppendUint64(b, envInc)
	state, inc := s.Rng.Words()
	b = binary.LittleEndian.AppendUint64(b, state)
	b = binary.LittleEndian.AppendUint64(b, inc)
	return b
}
