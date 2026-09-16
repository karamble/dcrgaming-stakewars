package objects

import (
	"bytes"
	"image/png"
	"testing"
)

func TestGraveHasTransparentBackground(t *testing.T) {
	im, err := png.Decode(bytes.NewReader(gravePNG))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := im.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatal("grave background is not transparent")
	}
	if Grave().Bounds().Empty() {
		t.Fatal("empty grave sprite")
	}
}
