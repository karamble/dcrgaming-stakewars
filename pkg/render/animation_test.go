package render

import (
	"testing"

	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
)

func TestAnimationSoundAndDamageFeedbackDoNotChangeReplay(t *testing.T) {
	s, err := sim.New(sim.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	r := New(s)
	s.Worms[0].HP -= 20
	s.Worms[0].Grounded = false
	s.Worms[0].Pos.Y -= fixed.FromInt(3)
	effects := []sim.Effect{{Jumped: true, Unit: 0, Variant: 2, Pos: s.Worms[0].Pos}, {Fired: true, Unit: 0, Weapon: sim.Bazooka, Pos: s.Worms[0].Pos}}
	before := replay.Hash(s)
	r.Update(s, effects)
	if len(r.floaters) != 1 || r.floaters[0].amount != -20 || r.motion[0].hurt == 0 || r.motion[0].recoil == 0 || !r.motion[0].backflip {
		t.Fatal("missing damage/jump/recoil feedback")
	}
	cues := r.DrainSounds()
	if len(cues) < 3 || len(r.DrainSounds()) != 0 {
		t.Fatal("sound cues missing or played twice")
	}
	canvas := NewRaster()
	defer canvas.Close()
	r.Draw(canvas, s, View{Arena: true})
	if replay.Hash(s) != before {
		t.Fatal("presentation changed authoritative state")
	}
	for i := 0; i < 80; i++ {
		r.Update(s, nil)
	}
	if len(r.floaters) != 0 || r.motion[0].hurt != 0 || r.motion[0].recoil != 0 {
		t.Fatal("expired animation retained")
	}
}
func TestWalkingFeetAndLandingSquash(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	r := New(s)
	s.Worms[0].Pos.X += fixed.FromInt(2)
	r.Update(s, nil)
	if r.actorPose(0, s).stride == 0 {
		t.Fatal("walking has no foot cycle")
	}
	s.Worms[0].Grounded = false
	r.Update(s, nil)
	s.Worms[0].Grounded = true
	r.Update(s, nil)
	r.Update(s, nil)
	if r.actorPose(0, s).sy >= 1 {
		t.Fatal("landing does not squash")
	}
}

func TestVisualAnimationRunsAtTripleSpeed(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	r := New(s)
	s.Worms[0].Pos.X += fixed.FromInt(2)
	r.Update(s, nil)
	if r.motion[0].walk != 2*.24*3 {
		t.Fatal("walking animation not tripled")
	}
	r.motion[0].recoil = 12
	r.motion[0].hurt = 18
	r.motion[0].land = 10
	for i := 0; i < 4; i++ {
		r.Update(s, nil)
	}
	if r.motion[0].recoil != 0 || r.motion[0].land != 0 || r.motion[0].hurt != 6 {
		t.Fatal("visual timers not tripled or underflowed")
	}
}

func TestTurnChargeAndExpressionsStayCosmetic(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	s.Phase = sim.Playing
	r := New(s)
	i := int(s.Active)
	s.Worms[i].Facing *= -1
	s.Aim = sim.FacingAim(s.Aim, s.Worms[i].Facing)
	before := replay.Hash(s)
	r.Update(s, nil)
	turning := r.actorPose(i, s)
	if turning.sx >= 1 || turning.lookX*float64(s.Worms[i].Facing) <= 0 {
		t.Fatal("pivot must animate while eyes already track the new aim")
	}
	if p := r.presentedPose(i, s, 1000); p.face != faceGrit || p.sy >= turning.sy {
		t.Fatal("full charge should brace and grit teeth")
	}
	for j := 0; j < 10; j++ {
		r.Update(s, nil)
	}
	if r.motion[i].turn != 0 || r.actorPose(i, s).sx != 1 {
		t.Fatal("pivot did not settle promptly")
	}
	c := NewRaster()
	defer c.Close()
	r.Draw(c, s, View{Arena: true, Power: 1000})
	if replay.Hash(s) != before {
		t.Fatal("turning/charging visuals changed replay state")
	}
	s.Worms[i].HP -= 10
	r.Update(s, nil)
	if r.presentedPose(i, s, 1000).face != faceHurt {
		t.Fatal("damage must override charging")
	}
	for j := 0; j < 6; j++ {
		r.Update(s, nil)
	}
	if r.actorPose(i, s).face != faceAngry {
		t.Fatal("hurt should recover into an angry glance")
	}
	for j := 0; j < 22; j++ {
		r.Update(s, nil)
	}
	if r.motion[i].mood != 0 {
		t.Fatal("reaction did not expire")
	}
}

func TestWeaponReactionsAndPickup(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	r := New(s)
	i := int(s.Active)
	r.Update(s, []sim.Effect{{Fired: true, Unit: s.Active, Weapon: sim.Bazooka}})
	rocket := r.actorPose(i, s).recoil
	r.Update(s, []sim.Effect{{Fired: true, Unit: s.Active, Weapon: sim.Uzi}})
	if r.actorPose(i, s).recoil >= rocket {
		t.Fatal("heavy weapon should recoil more strongly")
	}
	r.Update(s, []sim.Effect{{Fired: true, Unit: s.Active, Weapon: sim.Grenade}})
	if r.actorPose(i, s).recoil >= 0 {
		t.Fatal("throw should follow through toward target")
	}
	for j := 0; j < 12; j++ {
		r.Update(s, nil)
	}
	r.Update(s, []sim.Effect{{Pickup: true, Unit: s.Active}})
	if r.actorPose(i, s).face != faceHappy {
		t.Fatal("pickup should trigger a grin")
	}
	for j := 0; j < 20; j++ {
		r.Update(s, nil)
	}
	if r.actorPose(i, s).face == faceHappy {
		t.Fatal("pickup grin did not expire")
	}
}

func TestAirborneTransportPosesDoNotChangeReplay(t *testing.T) {
	s, _ := sim.New(sim.DefaultConfig())
	s.Phase = sim.Playing
	i := int(s.Active)
	s.Worms[i].Grounded = false
	s.Worms[i].Vel = fixed.Vec{X: fixed.FromInt(6), Y: fixed.FromInt(-3)}
	r := New(s)
	r.Update(s, nil)
	if p := r.actorPose(i, s); p.trail >= 0 || p.tuck <= 0 {
		t.Fatal("jump legs should tuck and trail motion")
	}
	s.Rope.Pivots = []sim.Pivot{{Pos: fixed.Vec{X: s.Worms[i].Pos.X + fixed.FromInt(80), Y: s.Worms[i].Pos.Y - fixed.FromInt(80)}}}
	p := r.actorPose(i, s)
	if p.handLift < 10 || p.angle <= 0 || p.face != faceHappy {
		t.Fatal("fast rope swing should reach up, lean toward anchor and grin")
	}
	s.Rope = sim.Rope{}
	s.Weapon, s.Thrust, s.Walk = sim.Jetpack, 1, 1
	before := replay.Hash(s)
	p = r.actorPose(i, s)
	if !p.jetpack || !p.thrust || p.angle <= 0 || p.tuck != 0 {
		t.Fatal("jetpack pose missing steering lean or dangling legs")
	}
	c := NewRaster()
	defer c.Close()
	r.Draw(c, s, View{Arena: true})
	if replay.Hash(s) != before {
		t.Fatal("transport animation changed replay state")
	}
	s.Thrust = 0
	if r.actorPose(i, s).thrust {
		t.Fatal("jet exhaust continued after releasing thrust")
	}
	s.Worms[i].Chute = true
	if r.actorPose(i, s).handLift < 10 {
		t.Fatal("parachuting should reach for suspension lines")
	}
}
