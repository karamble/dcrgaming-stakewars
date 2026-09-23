// Command stakewars-preview renders an internal scene to PNG without a display.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"

	"github.com/karamble/dcrgaming-stakewars/internal/tablelobby"
	"github.com/karamble/dcrgaming-stakewars/pkg/render"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim"
	"github.com/karamble/dcrgaming-stakewars/pkg/sim/terrain"
)

func main() {
	output := flag.String("output", "stakewars-preview.png", "PNG output path")
	coverPreview := flag.Bool("cover", false, "preview the opening cover")
	tablePreview := flag.Bool("table-lobby", false, "preview a fictional table lobby")
	liveTable := flag.Bool("live-table", false, "preview the SDK lobby layout with fictional data")
	tableStage := flag.Int("table-stage", 2, "demo stage 0..6")
	tableSeats := flag.Int("table-seats", 4, "demo player count: 2, 4, 6")
	tableSelected := flag.Int("table-player", -1, "show player details instead of terms")
	victory := flag.Bool("victory", false, "preview a fictional victory screen")
	draw := flag.Bool("draw", false, "preview a drawn match")
	bridge := flag.Bool("bridge-invite", false, "show a fictional bridge invitation preview")
	arena := flag.Bool("arena", false, "show the internal simulation scene")
	hover := flag.Int("hover", -1, "preview a weapon tooltip, 0..23")
	fire := flag.Int("fire", -1, "fire a weapon in the internal fixture, 0..23")
	flight := flag.Int("flight", 4, "ticks after firing, 0..600")
	pan := flag.Float64("pan", -1, "camera horizontal world position")
	seed := flag.Uint64("seed", 42, "match seed (also chooses hazard appearance)")
	drop := flag.Bool("hazard-drop", false, "internal fixture: open a hole under active Stakey")
	hideTop := flag.Bool("hide-top", false, "collapse header")
	hideBottom := flag.Bool("hide-bottom", false, "collapse weapon bar")
	zoom := flag.Float64("zoom", 1.25, "camera zoom")
	menu := flag.Bool("arsenal", false, "show right-click arsenal grid")
	panY := flag.Float64("pan-y", -1, "camera vertical world position")
	flag.Parse()
	if *flight < 0 || *flight > 600 {
		panic("flight outside 0..600")
	}
	config := sim.DefaultConfig()
	config.Seed = *seed
	s, err := sim.New(config)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 60; i++ {
		if _, err = sim.Step(s, nil); err != nil {
			panic(err)
		}
	}
	r := render.New(s)
	if *fire >= 0 && *fire < int(sim.WeaponCount) {
		effects, err := sim.Step(s, []sim.Input{{Tick: s.Tick, Seat: s.ActiveSeat(), Kind: sim.SelectWeapon, Param: int32(*fire)}, {Tick: s.Tick, Seat: s.ActiveSeat(), Sequence: 1, Kind: sim.Fire, Param: 800}})
		if err != nil {
			panic(err)
		}
		r.Update(s, effects)
		for i := 0; i < *flight; i++ {
			effects, err = sim.Step(s, nil)
			if err != nil {
				panic(err)
			}
			r.Update(s, effects)
		}
	}
	if *drop {
		w := s.Worms[s.Active]
		box := terrain.Rect{X0: w.Pos.X.Int() - 16, Y0: w.Pos.Y.Int(), X1: w.Pos.X.Int() + 17, Y1: int(s.Config.Height)}
		s.Terrain.StampRect(box, false)
		r.Update(s, []sim.Effect{{Dirty: box}})
		for i := 0; i < 180; i++ {
			effects, err := sim.Step(s, nil)
			if err != nil {
				panic(err)
			}
			r.Update(s, effects)
			hit := false
			for _, e := range effects {
				hit = hit || e.HazardDeath
			}
			if hit {
				for j := 0; j < 10; j++ {
					effects, err = sim.Step(s, nil)
					if err != nil {
						panic(err)
					}
					r.Update(s, effects)
				}
				break
			}
		}
	}
	c := render.NewRaster()
	defer c.Close()
	camera := render.Camera{Zoom: *zoom, Bounds: render.Stage(*hideTop, *hideBottom)}
	camera.Focus(s)
	if *pan >= 0 {
		camera.X = *pan
		camera.Clamp(s)
	}
	if *panY >= 0 {
		camera.Y = *panY
		camera.Clamp(s)
	}
	v := render.View{WeaponMenu: *menu, MenuOrigin: render.MenuOrigin(580, 220), WeaponPage: int(s.Weapon) / render.TraySlots, HideTop: *hideTop, HideBottom: *hideBottom, Camera: &camera, Arena: *arena, Frame: uint64(s.Tick), Power: 620}
	if *tablePreview {
		t := tablelobby.Demo(*tableSeats, *tableStage)
		if *liveTable {
			t.Live = true
			t.Demo = false
			t.WorldVerified = true
			t.CanFund = true
			t.Status = fmt.Sprintf("Battlefield agreed · 0/%d stakes confirmed", t.Invite.Seats)
			t.RefundStatus = "Stake: not funded. Entry refund: 1998 blocks remaining."
			for i := range t.Seats {
				t.Seats[i].BondOptional = true
				t.Seats[i].Stake = tablelobby.Payment{}
			}
		}
		v.TableLobby = render.TableLobbyView{Open: true, Table: &t, Selected: *tableSelected}
		v.Arena = false
	}
	if *victory || *draw {
		s.Phase = sim.Ended
		s.Turn, s.Tick = 27, 18640
		s.Winner = 0
		if *draw {
			s.Winner = -1
		}
		for i := range s.Worms {
			if s.Worms[i].Seat != 0 || *draw {
				s.Worms[i].HP = 0
			}
		}
		v.Arena, v.Dev = true, true
		v.Frame = 180
	}
	if *bridge {
		v.BridgeStatus = "Connected · StakeWars · mainnet (preview fixture)"
		v.BridgeLobby = render.BridgeLobbyView{Visible: true, Notice: "Unverified invitation · not seated · no funds requested", Terms: []string{"SESSION abc123", "4 seats · 0.10000000 DCR per seat", "Refund delay: 288 blocks", "Admission deadline: block 1000000"}}
	}
	if *hover >= 0 && *hover < int(sim.WeaponCount) {
		v.WeaponPage = *hover / render.TraySlots
		box := render.WeaponRect(sim.Weapon(*hover))
		v.MouseX = box.Min.X + 20
		v.MouseY = box.Min.Y + 20
	}
	if *coverPreview {
		render.DrawCover(c, 120, true)
	} else {
		r.Draw(c, s, v)
	}
	f, err := os.Create(*output)
	if err != nil {
		panic(err)
	}
	if err = png.Encode(f, c.Target); err != nil {
		f.Close()
		panic(err)
	}
	if err = f.Close(); err != nil {
		panic(err)
	}
	fmt.Println(*output)
}
