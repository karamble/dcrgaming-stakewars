// Command stakewars-sim is an offline engineering harness, not a public game.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
)

func main() {
	verify := flag.String("verify", "", "verify an internal replay file")
	record := flag.String("record", "", "write the generated internal replay")
	seed := flag.Uint64("seed", 42, "deterministic map seed")
	seats := flag.Uint("seats", 2, "fixture seats, 2..6")
	ticks := flag.Uint("ticks", 20000, "maximum fixture ticks")
	width := flag.Uint("width", 4096, "fixture map width, 640..4096")
	flag.Parse()
	if *verify != "" {
		f, err := os.Open(*verify)
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		r, err := replay.Read(f)
		if err != nil {
			fatal(err)
		}
		s, err := r.Run()
		if err != nil {
			fatal(err)
		}
		fmt.Printf("verified ticks=%d turn=%d winner=%d hash=%s\n", s.Tick, s.Turn, s.Winner, replay.HexHash(s))
		return
	}
	if *width < 640 || *width > 4096 || *seats < 2 || *seats > 6 || *ticks > replay.MaxTicks {
		fatal(fmt.Errorf("fixture outside supported limits"))
	}
	c := sim.DefaultConfig()
	c.Seed = *seed
	c.Width = uint16(*width)
	c.Seats = uint8(*seats)
	c.TurnTicks = 120
	c.RetreatTicks = 30
	c.SuddenDeathTurn = 8
	c.WaterRise = 20
	s, err := sim.New(c)
	if err != nil {
		fatal(err)
	}
	r := replay.Recording{Version: sim.Version, Config: c}
	for s.Tick < uint32(*ticks) && s.Phase != sim.Ended {
		var in []sim.Input
		if s.Phase == sim.Playing && s.PhaseTick == 60 {
			in = []sim.Input{
				{Tick: s.Tick, Seat: s.ActiveSeat(), Sequence: 0, Kind: sim.SelectWeapon, Param: int32(sim.Grenade)},
				{Tick: s.Tick, Seat: s.ActiveSeat(), Sequence: 1, Kind: sim.Fire, Param: 650},
			}
		}
		r.Inputs = append(r.Inputs, in...)
		if _, err = sim.Step(s, in); err != nil {
			fatal(err)
		}
	}
	r.Ticks = s.Tick
	r.FinalHash = replay.HexHash(s)
	if _, err = r.Run(); err != nil {
		fatal(err)
	}
	if *record != "" {
		f, err := os.Create(*record)
		if err != nil {
			fatal(err)
		}
		err = replay.Write(f, r)
		closeErr := f.Close()
		if err != nil {
			fatal(err)
		}
		if closeErr != nil {
			fatal(closeErr)
		}
	}
	fmt.Printf("internal fixture ticks=%d turn=%d winner=%d hash=%s\n", s.Tick, s.Turn, s.Winner, r.FinalHash)
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
