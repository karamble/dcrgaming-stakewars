package main

import (
	tscript "github.com/decred/dcrd/txscript/v4"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/fixed"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
	"testing"
)

// Compare against the actual full Step entry point in a controlled empty arena,
// not a second Go implementation of the same arithmetic expression.
func actualFlight(t *testing.T, vy int64) int64 {
	t.Helper()
	c := sim.DefaultConfig()
	c.Width = 640
	c.Height = 384
	c.Seats = 2
	c.Units = 1
	c.ReadyTicks = 0
	c.Environment = false
	s, e := sim.New(c)
	if e != nil {
		t.Fatal(e)
	}
	s.Terrain = terrain.New(640, 384, false)
	s.Terrain.StampRect(terrain.Rect{X0: 0, Y0: 300, X1: 640, Y1: 340}, true)
	for i := range s.Worms {
		s.Worms[i].Pos = fixed.Vec{X: fixed.FromInt(50 + i*500), Y: fixed.FromInt(300)}
		s.Worms[i].Vel = fixed.Vec{}
		s.Worms[i].Grounded = true
	}
	s.Wind = 0
	s.Projectiles = []sim.Projectile{{Weapon: sim.Bazooka, Pos: fixed.Vec{X: fixed.FromInt(200), Y: fixed.FromInt(100)}, Vel: fixed.Vec{Y: fixed.F(vy)}}}
	if _, e = sim.Step(s, nil); e != nil {
		t.Fatal(e)
	}
	if len(s.Projectiles) != 1 || s.Projectiles[0].Dead {
		t.Fatal("flight fixture collided or disappeared")
	}
	return int64(s.Projectiles[0].Vel.Y)
}
func TestArithmeticAgainstRealSimulation(t *testing.T) {
	values := []int64{-2097152, -2097151, -65536, -4096, -1, 0, 1, 4096, 2093055, 2093056, 2093057, 2097151, 2097152}
	for i := int64(0); i < 32; i++ {
		values = append(values, (i*7919*17)%4194305-2097152)
	}
	for _, vy := range values {
		actual := actualFlight(t, vy)
		honest, pk, _ := fixture(vy, actual)
		if check(honest, pk) == nil {
			t.Fatalf("honest result challenged: vy%d ->%d", vy, actual)
		}
		for _, delta := range []int64{-1, 1} {
			bad, pk, _ := fixture(vy, actual+delta)
			if err := check(bad, pk); err != nil {
				t.Fatalf("false result not provable: vy%d ->%d: %v", vy, actual+delta, err)
			}
		}
	}
	t.Logf("%d input vectors matched actual sim.Step, including negative values and upper clamp", len(values))
}
func TestInputCommitmentAndBound(t *testing.T) {
	tx, pk, _ := fixture(-65536, -65536)
	args := arguments(tx)
	args[2] = tscript.ScriptNum(-65535).Bytes()
	setArguments(tx, args)
	if check(tx, pk) == nil {
		t.Fatal("different input satisfies commitment")
	}
	for _, vy := range []int64{-2097153, 2097153, -2147483648, 2147483647} {
		tx, pk, _ := fixture(vy, 0)
		if check(tx, pk) == nil {
			t.Fatal("outside input domain accepted", vy)
		}
	}
	_, _, script := fixture(-65536, -65536)
	tok := tscript.MakeScriptTokenizer(0, script)
	ops := 0
	for tok.Next() {
		if tok.Opcode() > tscript.OP_16 {
			ops++
		}
	}
	if tok.Err() != nil {
		t.Fatal(tok.Err())
	}
	if ops > 255 {
		t.Fatal("operation limit")
	}
	t.Logf("fraud script %d bytes, %d non-push operations", len(script), ops)
}
