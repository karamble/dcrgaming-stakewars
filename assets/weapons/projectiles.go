package weapons

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
)

//go:embed comic-projectiles-v1.png
var projectilesPNG []byte
var projectiles = loadProjectiles()

func loadProjectiles() [4]*image.NRGBA {
	atlas, err := png.Decode(bytes.NewReader(projectilesPNG))
	if err != nil {
		panic(err)
	}
	boxes := [...]image.Rectangle{image.Rect(25, 165, 755, 510), image.Rect(811, 265, 1176, 436), image.Rect(75, 874, 623, 1019), image.Rect(858, 664, 1173, 1110)}
	var result [4]*image.NRGBA
	for i, b := range boxes {
		result[i] = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(result[i], result[i].Bounds(), atlas, b.Min, draw.Src)
	}
	return result
}

// Projectile returns rocket, shotgun pellet, Uzi round, or cluster bomblet.
func Projectile(index int) image.Image { return projectiles[index] }
