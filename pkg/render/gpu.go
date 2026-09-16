//go:build desktop

package render

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

type GPU struct {
	Target       *ebiten.Image
	Scene        *Scene
	texture      *ebiten.Image
	sprites      map[image.Image]*ebiten.Image
	revision     uint64
	faces        map[int]font.Face
	normal, bold *opentype.Font
}

func NewGPU(scene *Scene) *GPU {
	n, _ := opentype.Parse(goregular.TTF)
	b, _ := opentype.Parse(gobold.TTF)
	return &GPU{Scene: scene, sprites: make(map[image.Image]*ebiten.Image), faces: map[int]font.Face{}, normal: n, bold: b}
}
func (g *GPU) Rect(x, y, w, h float64, c color.RGBA) {
	if w > 0 && h > 0 {
		vector.DrawFilledRect(g.Target, float32(x), float32(y), float32(w), float32(h), color.NRGBA(c), false)
	}
}
func (g *GPU) Circle(x, y, r float64, c color.RGBA) {
	vector.DrawFilledCircle(g.Target, float32(x), float32(y), float32(r), color.NRGBA(c), true)
}
func (g *GPU) Line(x0, y0, x1, y1, width float64, c color.RGBA) {
	vector.StrokeLine(g.Target, float32(x0), float32(y0), float32(x1), float32(y1), float32(width), color.NRGBA(c), true)
}
func (g *GPU) Poly(p []Point, c color.RGBA) {
	if len(p) < 3 {
		return
	}
	var path vector.Path
	path.MoveTo(float32(p[0].X), float32(p[0].Y))
	for _, v := range p[1:] {
		path.LineTo(float32(v.X), float32(v.Y))
	}
	path.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(color.NRGBA(c))
	vector.FillPath(g.Target, &path, nil, op)
}
func (g *GPU) Text(s string, x, y, size float64, c color.RGBA) {
	key := int(size * 10)
	face, ok := g.faces[key]
	if !ok {
		f := g.normal
		if size >= 26 {
			f = g.bold
		}
		face, _ = opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		g.faces[key] = face
	}
	text.Draw(g.Target, s, face, int(x), int(y+size), color.NRGBA(c))
}
func (g *GPU) Image(src image.Image, x, y, w, h float64) {
	if src != g.Scene.Terrain {
		texture := g.sprites[src]
		if texture == nil {
			texture = ebiten.NewImageFromImage(src)
			g.sprites[src] = texture
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(w/float64(src.Bounds().Dx()), h/float64(src.Bounds().Dy()))
		op.GeoM.Translate(x, y)
		op.Filter = ebiten.FilterLinear
		g.Target.DrawImage(texture, op)
		return
	}
	if g.texture == nil {
		g.texture = ebiten.NewImageFromImage(src)
		g.revision = g.Scene.revision
		g.Scene.dirty = image.Rectangle{}
	}
	if g.revision != g.Scene.revision {
		// One coalesced dirty upload per rendered update, never a full texture
		// replacement for an individual explosion.
		r := g.Scene.dirty.Intersect(src.Bounds())
		pixels := make([]byte, r.Dx()*r.Dy()*4)
		for yy := r.Min.Y; yy < r.Max.Y; yy++ {
			at := (yy - r.Min.Y) * r.Dx() * 4
			off := g.Scene.Terrain.PixOffset(r.Min.X, yy)
			copy(pixels[at:at+r.Dx()*4], g.Scene.Terrain.Pix[off:off+r.Dx()*4])
		}
		if !r.Empty() {
			g.texture.SubImage(r).(*ebiten.Image).WritePixels(pixels)
		}
		g.revision = g.Scene.revision
		g.Scene.dirty = image.Rectangle{}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w/float64(src.Bounds().Dx()), h/float64(src.Bounds().Dy()))
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	g.Target.DrawImage(g.texture, op)
}

func (g *GPU) Clip(rect image.Rectangle) func() {
	old := g.Target
	g.Target = old.SubImage(rect.Intersect(old.Bounds())).(*ebiten.Image)
	return func() { g.Target = old }
}

func (g *GPU) RotatedImage(src image.Image, x, y, w, h, angle float64) {
	texture := g.sprites[src]
	if texture == nil {
		texture = ebiten.NewImageFromImage(src)
		g.sprites[src] = texture
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(src.Bounds().Dx())/2, -float64(src.Bounds().Dy())/2)
	op.GeoM.Scale(w/float64(src.Bounds().Dx()), h/float64(src.Bounds().Dy()))
	op.GeoM.Rotate(angle)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	g.Target.DrawImage(texture, op)
}
