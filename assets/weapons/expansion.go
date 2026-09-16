package weapons

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
)

//go:embed comic-expansion-v1.png
var expansionPNG []byte

//go:embed comic-expansion-projectiles-v1.png
var expansionProjectilesPNG []byte
var expansion = grid(expansionPNG, 4, 3)
var expansionShots = loadExpansionShots()

func grid(data []byte, columns, rows int) []*image.NRGBA {
	atlas, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	var result []*image.NRGBA
	for y := 0; y < rows; y++ {
		for x := 0; x < columns; x++ {
			b := image.Rect(x*atlas.Bounds().Dx()/columns, y*atlas.Bounds().Dy()/rows, (x+1)*atlas.Bounds().Dx()/columns, (y+1)*atlas.Bounds().Dy()/rows)
			result = append(result, extract(atlas, b))
		}
	}
	return result
}
func extract(atlas image.Image, b image.Rectangle) *image.NRGBA {
	// Trim only transparent margins at runtime; retain the original atlas/alpha.
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := atlas.At(x, y).RGBA()
			if a > 0 {
				minX = min(minX, x)
				minY = min(minY, y)
				maxX = max(maxX, x+1)
				maxY = max(maxY, y+1)
			}
		}
	}
	if maxX <= minX || maxY <= minY {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	b = image.Rect(minX, minY, maxX, maxY)
	result := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(result, result.Bounds(), atlas, b.Min, draw.Src)
	return result
}
func loadExpansionShots() []*image.NRGBA {
	atlas, err := png.Decode(bytes.NewReader(expansionProjectilesPNG))
	if err != nil {
		panic(err)
	}
	// Authored regions account for the generated silhouettes crossing grid guides.
	boxes := []image.Rectangle{image.Rect(0, 0, 480, 440), image.Rect(480, 0, 875, 440), image.Rect(890, 0, 1325, 440), image.Rect(1330, 0, 1776, 440), image.Rect(0, 445, 475, 888), image.Rect(475, 445, 935, 888), image.Rect(945, 445, 1320, 888), image.Rect(1325, 445, 1776, 888)}
	var result []*image.NRGBA
	for _, b := range boxes {
		result = append(result, extract(atlas, b.Intersect(atlas.Bounds())))
	}
	return result
}
func ExpansionProjectile(index int) image.Image { return expansionShots[index] }
