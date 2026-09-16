// Package objects embeds the original generated comic pickup atlas.
package objects

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"
)

//go:embed crates-v1.png
var atlas []byte
var once sync.Once
var sprites [3]image.Image

// Sprite returns health, ammunition, or barrel art. Alpha margins are trimmed
// at runtime so the visible feet and bounds align with the simulation.
func Sprite(index int) image.Image {
	once.Do(func() {
		im, err := png.Decode(bytes.NewReader(atlas))
		if err != nil {
			panic(err)
		}
		sub := im.(interface {
			SubImage(image.Rectangle) image.Image
		})
		b := im.Bounds()
		for i := range sprites {
			cell := image.Rect(b.Min.X+i*b.Dx()/3, b.Min.Y, b.Min.X+(i+1)*b.Dx()/3, b.Max.Y)
			bounds := image.Rectangle{}
			for y := cell.Min.Y; y < cell.Max.Y; y++ {
				for x := cell.Min.X; x < cell.Max.X; x++ {
					_, _, _, a := im.At(x, y).RGBA()
					if a > 256 {
						bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			sprites[i] = sub.SubImage(bounds)
		}
	})
	return sprites[index]
}
