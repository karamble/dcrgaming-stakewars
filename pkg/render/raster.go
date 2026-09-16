package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Raster is the offline renderer for reproducible visual inspection. The
// desktop has a separate GPU canvas implementing the same drawing operations.
type Raster struct {
	Target       *image.RGBA
	faces        map[int]font.Face
	normal, bold *opentype.Font
}

func NewRaster() *Raster {
	n, _ := opentype.Parse(goregular.TTF)
	b, _ := opentype.Parse(gobold.TTF)
	return &Raster{Target: image.NewRGBA(image.Rect(0, 0, Width, Height)), faces: map[int]font.Face{}, normal: n, bold: b}
}
func (r *Raster) Rect(x, y, w, h float64, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	draw.Draw(r.Target, image.Rect(int(x), int(y), int(math.Ceil(x+w)), int(math.Ceil(y+h))), image.NewUniform(color.NRGBA(c)), image.Point{}, draw.Over)
}
func (r *Raster) Poly(p []Point, c color.RGBA) {
	if len(p) < 3 {
		return
	}
	x0, y0, x1, y1 := p[0].X, p[0].Y, p[0].X, p[0].Y
	for _, v := range p {
		x0 = math.Min(x0, v.X)
		y0 = math.Min(y0, v.Y)
		x1 = math.Max(x1, v.X)
		y1 = math.Max(y1, v.Y)
	}
	box := image.Rect(int(math.Floor(x0)), int(math.Floor(y0)), int(math.Ceil(x1))+1, int(math.Ceil(y1))+1).Intersect(r.Target.Bounds())
	if box.Empty() {
		return
	}
	v := vector.NewRasterizer(box.Dx(), box.Dy())
	v.MoveTo(float32(p[0].X-float64(box.Min.X)), float32(p[0].Y-float64(box.Min.Y)))
	for _, pt := range p[1:] {
		v.LineTo(float32(pt.X-float64(box.Min.X)), float32(pt.Y-float64(box.Min.Y)))
	}
	v.ClosePath()
	v.Draw(r.Target, box, image.NewUniform(color.NRGBA(c)), image.Point{})
}
func (r *Raster) Circle(x, y, radius float64, c color.RGBA) {
	n := int(radius * 3)
	if n < 16 {
		n = 16
	}
	if n > 100 {
		n = 100
	}
	p := make([]Point, n)
	for i := range p {
		a := float64(i) * math.Pi * 2 / float64(n)
		p[i] = Point{x + math.Cos(a)*radius, y + math.Sin(a)*radius}
	}
	r.Poly(p, c)
}
func (r *Raster) Line(x0, y0, x1, y1, width float64, c color.RGBA) {
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	nx, ny := -dy/length*width/2, dx/length*width/2
	r.Poly([]Point{{x0 + nx, y0 + ny}, {x1 + nx, y1 + ny}, {x1 - nx, y1 - ny}, {x0 - nx, y0 - ny}}, c)
}
func (r *Raster) Text(s string, x, y, size float64, c color.RGBA) {
	key := int(size * 10)
	face, ok := r.faces[key]
	if !ok {
		f := r.normal
		if size >= 26 {
			f = r.bold
		}
		face, _ = opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		r.faces[key] = face
	}
	d := font.Drawer{Dst: r.Target, Src: image.NewUniform(color.NRGBA(c)), Face: face, Dot: fixed.P(int(x), int(y+size))}
	d.DrawString(s)
}
func (r *Raster) Image(src image.Image, x, y, w, h float64) {
	xdraw.ApproxBiLinear.Scale(r.Target, image.Rect(int(x), int(y), int(x+w), int(y+h)), src, src.Bounds(), draw.Over, nil)
}
func (r *Raster) Close() {
	for _, f := range r.faces {
		f.Close()
	}
}

func (r *Raster) Clip(rect image.Rectangle) func() {
	old := r.Target
	r.Target = old.SubImage(rect.Intersect(old.Bounds())).(*image.RGBA)
	return func() { r.Target = old }
}

func (r *Raster) RotatedImage(src image.Image, x, y, w, h, angle float64) {
	sx, sy := w/float64(src.Bounds().Dx()), h/float64(src.Bounds().Dy())
	a, b := math.Cos(angle)*sx, -math.Sin(angle)*sy
	d, e := math.Sin(angle)*sx, math.Cos(angle)*sy
	cx, cy := float64(src.Bounds().Min.X)+float64(src.Bounds().Dx())/2, float64(src.Bounds().Min.Y)+float64(src.Bounds().Dy())/2
	transform := f64.Aff3{a, b, x - a*cx - b*cy, d, e, y - d*cx - e*cy}
	xdraw.ApproxBiLinear.Transform(r.Target, transform, src, src.Bounds(), draw.Over, nil)
}
