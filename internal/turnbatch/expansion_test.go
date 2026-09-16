package turnbatch

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

func TestEveryExpansionWeaponSignedTurnReplaysOnPeers(t *testing.T) {
	for weapon := sim.Mortar; weapon < sim.WeaponCount; weapon++ {
		t.Run(fmt.Sprint(weapon), func(t *testing.T) {
			context, before, _, _, key := fixture(t)
			before.Config.WeaponDelays = false
			after := before.Clone()
			var inputs []sim.Input
			for after.Turn == before.Turn && after.Phase != sim.Ended {
				var tick []sim.Input
				add := func(kind sim.EventKind, param int32) {
					tick = append(tick, sim.Input{Tick: after.Tick, Seat: after.ActiveSeat(), Sequence: uint16(len(tick)), Kind: kind, Param: param})
				}
				switch after.Tick {
				case 0:
					add(sim.Face, -1)
					add(sim.Jump, 2)
					add(sim.SetFuse, 2)
					add(sim.SetTarget, 200<<16|500)
					add(sim.SelectWeapon, int32(weapon))
				case 5:
					if after.Phase == sim.Playing {
						add(sim.Fire, 700)
					}
				case 15:
					if weapon == sim.RemoteCharge && after.Phase == sim.Playing {
						add(sim.Fire, 1000)
					}
				}
				if weapon == sim.MiningDrill && after.Phase == sim.Playing && after.Tick >= 6 && after.Tick < 36 {
					add(sim.Fire, 1000)
				}
				inputs = append(inputs, tick...)
				if _, err := sim.Step(after, tick); err != nil {
					t.Fatal(err)
				}
				if after.Tick > MaxTicks {
					t.Fatal("weapon prevented turn completion")
				}
			}
			batch, err := Sign(context, before, after, inputs, key)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := batch.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			for peer := 0; peer < 3; peer++ {
				decoded, err := Decode(bytes.NewReader(wire))
				if err != nil {
					t.Fatal(err)
				}
				got, err := decoded.Verify(context, before.Clone())
				if err != nil {
					t.Fatal(err)
				}
				if replay.Hash(got) != replay.Hash(after) {
					t.Fatal("peer diverged")
				}
			}
			tampered := batch
			tampered.Inputs = append([]sim.Input(nil), batch.Inputs...)
			tampered.Inputs[2].Param = 5
			if _, err := tampered.Verify(context, before); err == nil {
				t.Fatal("modified fuse passed signature verification")
			}
		})
	}
}

func TestReadyCountdownCannotBeOmittedFromTurnHead(t *testing.T) {
	context, before, after, inputs, key := fixture(t)
	before.Phase = sim.Playing // Same turn number, but this skips the agreed readiness.
	if _, err := Sign(context, before, after, inputs, key); err == nil {
		t.Fatal("accepted action-phase head with readiness omitted")
	}
}
