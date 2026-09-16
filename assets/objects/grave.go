package objects

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"
)

//go:embed grave-cross-v1.png
var gravePNG []byte
var graveOnce sync.Once
var grave image.Image

// Grave is the generated comic wooden cross, trimmed to its alpha bounds.
func Grave() image.Image {
	graveOnce.Do(func() {
		im, err := png.Decode(bytes.NewReader(gravePNG))
		if err != nil {
			panic(err)
		}
		bounds := image.Rectangle{}
		for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
			for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
				_, _, _, a := im.At(x, y).RGBA()
				if a > 256 {
					bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
		grave = im.(interface {
			SubImage(image.Rectangle) image.Image
		}).SubImage(bounds)
	})
	return grave
}
