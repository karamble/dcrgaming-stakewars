package render

import (
	"github.com/karamble/dcrstakewars/pkg/sim"
	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"testing"
)

func TestWeatherFollowsWindAndDoesNotChangeSimulation(t *testing.T) {
	s, err := sim.New(sim.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	r := new(Scene)
	for _, wind := range []fixed.F{fixed.Ratio(1, 5), 0, -fixed.Ratio(1, 5)} {
		s.Wind = wind
		before := string(s.AppendCanonical(nil))
		prior := r.weather.drift
		r.advanceWeather(s)
		delta := r.weather.drift - prior
		if wind > 0 && delta <= 0 || wind < 0 && delta >= 0 || wind == 0 && delta != 0 {
			t.Fatal("weather contradicts wind", wind, delta)
		}
		if string(s.AppendCanonical(nil)) != before {
			t.Fatal("weather mutated gameplay")
		}
	}
}
