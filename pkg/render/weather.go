package render

import (
	"github.com/karamble/dcrstakewars/assets/scenery"
	"github.com/karamble/dcrstakewars/pkg/sim"
	"github.com/karamble/dcrstakewars/pkg/sim/fixed"
	"math"
)

type weatherMotion struct{ drift, fall float64 }

func weatherDrift(wind fixed.F) float64 { return float64(wind) / 65536 * 12 }
func (r *Scene) advanceWeather(s *sim.State) {
	r.weather.drift += weatherDrift(s.Wind)
	r.weather.fall += 1
}
func (r *Scene) drawWeather(c Canvas, s *sim.State, cam Camera, near bool) {
	speed, count := .3, 22
	if near {
		speed, count = 1.15, 16
	}
	b := cam.Viewport()
	wind := float64(s.Wind) / 65536
	for i := 0; i < count; i++ {
		salt := uint64(i+1)*6364136223846793005 ^ s.Config.Seed
		sprite := i % 2
		switch s.Config.Seed % 3 {
		case 1:
			sprite = 4 + i%2
		case 2:
			sprite = []int{2, 3, 7}[i%3]
		default:
			if i%5 == 0 {
				sprite = 2
			}
		}
		baseX := float64(salt % 4096)
		baseY := float64((salt >> 16) % 2048)
		x := float64(b.Min.X) - 40 + repeat(baseX+r.weather.drift*speed-cam.X*cam.Zoom*speed, float64(b.Dx()+80))
		fallRate := 2.5
		if sprite >= 2 {
			fallRate = .65
		}
		if sprite == 4 {
			fallRate = -.65
		}
		y := float64(b.Min.Y) - 40 + repeat(baseY+r.weather.fall*fallRate*speed-cam.Y*cam.Zoom*speed, float64(b.Dy()+80))
		size := 18.0
		if near {
			size = 31
		}
		if sprite >= 2 {
			size *= .75
		}
		angle := math.Atan2(-wind*12, fallRate)
		if sprite >= 2 {
			angle = math.Sin(r.weather.fall*.025+float64(i))*.6 + wind*2
		}
		c.RotatedImage(scenery.Weather(sprite), x, y, size, size*1.3, angle)
	}
}
