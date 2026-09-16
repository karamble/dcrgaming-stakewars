package replay

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/karamble/dcrstakewars/pkg/sim"
)

func TestRoundTripAndTamperDetection(t *testing.T) {
	c := sim.DefaultConfig()
	c.Width = 640
	c.Height = 384
	r := Recording{Version: sim.Version, Config: c, Ticks: 600, Inputs: []sim.Input{{Tick: 100, Seat: 0, Kind: sim.Fire, Param: 750}}}
	s, err := r.Run()
	if err != nil {
		t.Fatal(err)
	}
	r.FinalHash = HexHash(s)
	var b bytes.Buffer
	if err = Write(&b, r); err != nil {
		t.Fatal(err)
	}
	loaded, err := Read(&b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loaded.Run(); err != nil {
		t.Fatal(err)
	}
	loaded.Inputs[0].Kind = sim.Move
	loaded.Inputs[0].Param = 1
	if _, err = loaded.Run(); err == nil {
		t.Fatal("tampered input matched final state")
	}
}
func TestMalformedReplayRejected(t *testing.T) {
	for _, raw := range []string{`{} {}`, `{"Unknown":1}`, `not-json`} {
		if _, err := Read(strings.NewReader(raw)); err == nil {
			t.Fatal("malformed replay accepted")
		}
	}
	c := sim.DefaultConfig()
	for _, r := range []Recording{
		{Version: 999, Config: c}, {Version: sim.Version, Config: c, Ticks: MaxTicks + 1},
		{Version: sim.Version, Config: c, Ticks: 10, Inputs: []sim.Input{{Tick: 10}}},
		{Version: sim.Version, Config: c, Ticks: 10, Inputs: []sim.Input{{Tick: 2}, {Tick: 1}}},
	} {
		if _, err := r.Run(); err == nil {
			t.Fatal("invalid replay accepted")
		}
	}
}

func TestFrozenReplays(t *testing.T) {
	for _, name := range []string{"duel", "six-squads", "wide-six-squads"} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open("testdata/" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r, err := Read(f)
			if err != nil {
				t.Fatal(err)
			}
			if r.FinalHash == "" {
				t.Fatal("golden hash missing")
			}
			s, err := r.Run()
			if err != nil {
				t.Fatal(err)
			}
			if s.Phase != sim.Ended {
				t.Fatal("fixture did not finish")
			}
		})
	}
}
