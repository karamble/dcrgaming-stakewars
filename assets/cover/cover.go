package cover

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"
)

//go:embed stakewars-cover-v1.png
var data []byte
var once sync.Once
var art image.Image

func Image() image.Image {
	once.Do(func() {
		var e error
		art, e = png.Decode(bytes.NewReader(data))
		if e != nil {
			panic(e)
		}
	})
	return art
}

//go:embed stakewars-logo-v1.png
var logoData []byte
var logoOnce sync.Once
var logo image.Image

// Logo returns the standalone transparent wordmark derived from the cover.
func Logo() image.Image {
	logoOnce.Do(func() {
		im, e := png.Decode(bytes.NewReader(logoData))
		if e != nil {
			panic(e)
		}
		b := im.Bounds()
		tight := image.Rectangle{}
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, a := im.At(x, y).RGBA()
				if a > 256 {
					tight = tight.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
		logo = im.(interface {
			SubImage(image.Rectangle) image.Image
		}).SubImage(tight)
	})
	return logo
}
