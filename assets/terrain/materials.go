// Package terrain provides embedded visual materials, never collision masks.
package terrain

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
)

//go:embed slate-v1.png
var slate []byte

//go:embed basalt-v1.png
var basalt []byte

//go:embed moss-v1.png
var moss []byte
var materials = load()

func load() [3]image.Image {
	var images [3]image.Image
	for i, data := range [][]byte{slate, basalt, moss} {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		images[i] = img
	}
	return images
}
func mirror(v int) int {
	v %= 1024
	if v < 0 {
		v += 1024
	}
	if v >= 512 {
		v = 1023 - v
	}
	return v
}

// Color samples world-anchored mirrored tiles, preserving seams without
// changing the generated source PNGs. Camera motion never changes the fill.
func Color(seed uint64, x, y int) color.RGBA {
	material := materials[seed%3]
	bounds := material.Bounds()
	u := mirror(x+int((seed>>8)%512)) * bounds.Dx() / 512
	v := mirror(y+int((seed>>16)%512)) * bounds.Dy() / 512
	r, g, b, _ := material.At(u, v).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
}
