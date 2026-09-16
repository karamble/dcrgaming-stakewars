// Package weapons embeds the original comic atlas and exposes its authored
// sprite regions. Assets are available in packaged builds without a working
// directory dependency. The original PNG and alpha are preserved.
package weapons

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
)

//go:embed comic-weapons-v1.png
var atlasPNG []byte

var regions = [...]image.Rectangle{
	image.Rect(8, 50, 465, 365),
	image.Rect(472, 50, 706, 365),
	image.Rect(765, 50, 1045, 380),
	image.Rect(1050, 100, 1448, 355),
	image.Rect(12, 387, 389, 710),
	image.Rect(394, 386, 749, 711),
	image.Rect(756, 380, 1075, 713),
	image.Rect(1083, 442, 1448, 711),
	image.Rect(40, 715, 341, 1060),
	image.Rect(375, 715, 751, 1060),
	image.Rect(755, 717, 1082, 1070),
}
var sprites = load()

func load() [11]*image.NRGBA {
	atlas, err := png.Decode(bytes.NewReader(atlasPNG))
	if err != nil {
		panic(err)
	}
	var result [11]*image.NRGBA
	for i, box := range regions {
		result[i] = image.NewNRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
		draw.Draw(result[i], result[i].Bounds(), atlas, box.Min, draw.Src)
	}
	return result
}
func Sprite(index int) image.Image {
	if index == 23 {
		return miningDrill
	}
	if index >= len(sprites) && index < len(sprites)+len(expansion) {
		return expansion[index-len(sprites)]
	}
	if index < 0 || index >= len(sprites) {
		return nil
	}
	return sprites[index]
}
