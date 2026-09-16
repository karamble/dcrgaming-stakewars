package weapons

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
)

//go:embed comic-mining-drill-v2.png
var miningDrillPNG []byte

var miningDrill = func() image.Image {
	img, err := png.Decode(bytes.NewReader(miningDrillPNG))
	if err != nil {
		panic(err)
	}
	return img
}()
