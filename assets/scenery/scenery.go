// Package scenery embeds generated comic layers; these never affect collision.
package scenery

import (
	"embed"
	"image"
	"image/png"
	"sync"
)

//go:embed *-v1.png
var files embed.FS
var once sync.Once
var layers map[string]image.Image

func Layer(name string) image.Image {
	once.Do(func() {
		layers = make(map[string]image.Image)
		for _, n := range []string{"sky", "far", "mid", "front", "far-volcanic", "mid-grove"} {
			f, e := files.Open(n + "-v1.png")
			if e != nil {
				panic(e)
			}
			im, e := png.Decode(f)
			f.Close()
			if e != nil {
				panic(e)
			}
			layers[n] = im
		}
	})
	return layers[name]
}

var weatherOnce sync.Once
var weather [8]image.Image

// Weather exposes sprite cells directly, preserving the original PNG alpha.
func Weather(index int) image.Image {
	weatherOnce.Do(func() {
		f, e := files.Open("weather-v1.png")
		if e != nil {
			panic(e)
		}
		im, e := png.Decode(f)
		f.Close()
		if e != nil {
			panic(e)
		}
		sub := im.(interface {
			SubImage(image.Rectangle) image.Image
		})
		b := im.Bounds()
		w, h := b.Dx()/4, b.Dy()/2
		for i := range weather {
			weather[i] = sub.SubImage(image.Rect((i%4)*w, (i/4)*h, (i%4+1)*w, (i/4+1)*h))
		}
	})
	return weather[index%8]
}
