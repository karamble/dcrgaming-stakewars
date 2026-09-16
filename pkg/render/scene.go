// Package render draws StakeWars without ever modifying simulation state. Its
// canvas interface supports both the desktop GPU and offline PNG inspection.
package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/karamble/dcrstakewars/assets/cover"
	materials "github.com/karamble/dcrstakewars/assets/terrain"
	"github.com/karamble/dcrstakewars/assets/weapons"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

const Width, Height = 1440, 900

type Point struct{ X, Y float64 }
type Canvas interface {
	Clip(image.Rectangle) func()
	RotatedImage(image.Image, float64, float64, float64, float64, float64)
	Rect(x, y, w, h float64, c color.RGBA)
	Circle(x, y, r float64, c color.RGBA)
	Line(x0, y0, x1, y1, width float64, c color.RGBA)
	Poly(points []Point, c color.RGBA)
	Text(text string, x, y, size float64, c color.RGBA)
	Image(src image.Image, x, y, w, h float64)
}

var (
	Navy  = color.RGBA{9, 16, 35, 255}
	Panel = color.RGBA{14, 25, 46, 245}
	Muted = color.RGBA{125, 151, 178, 255}
	White = color.RGBA{233, 245, 248, 255}
	Mint  = color.RGBA{39, 211, 162, 255}
	Blue  = color.RGBA{47, 111, 255, 255}
	teams = []color.RGBA{Mint, Blue, {255, 173, 81, 255}, {221, 116, 235, 255}, {240, 104, 120, 255}, {119, 208, 243, 255}}
)

type Scene struct {
	weather   weatherMotion
	previous  []sim.Worm
	motion    []motion
	floaters  []floater
	cues      []SoundCue
	lastPhase sim.Phase
	Terrain   *image.RGBA
	revision  uint64
	dirty     image.Rectangle
	blasts    []blast
	splashes  []splash
	shots     []shot
}

type shot struct {
	from, to Point
	weapon   sim.Weapon
	age      int
}

type blast struct {
	weapon       sim.Weapon
	x, y, radius float64
	age          int
}

func New(s *sim.State) *Scene {
	r := &Scene{Terrain: image.NewRGBA(image.Rect(0, 0, int(s.Config.Width), int(s.Config.Height)))}
	r.update(s, r.Terrain.Bounds())
	r.initMotion(s)
	return r
}
func (r *Scene) Revision() uint64 { return r.revision }
func (r *Scene) Update(s *sim.State, effects []sim.Effect) {
	r.updateMotion(s, effects)
	r.advanceWeather(s)
	splashes := r.splashes[:0]
	for _, sp := range r.splashes {
		sp.age += AnimationSpeed
		if sp.age < 50 {
			splashes = append(splashes, sp)
		}
	}
	r.splashes = splashes
	shots := r.shots[:0]
	for _, shot := range r.shots {
		shot.age += AnimationSpeed
		if shot.age < 24 {
			shots = append(shots, shot)
		}
	}
	r.shots = shots
	alive := r.blasts[:0]
	for _, b := range r.blasts {
		b.age += AnimationSpeed
		if b.age < 42 {
			alive = append(alive, b)
		}
	}
	r.blasts = alive
	var dirty image.Rectangle
	burstIndex := 0
	for _, e := range effects {
		if e.Jumped || e.Fired || e.Pickup {
			continue
		}
		if e.HazardDeath {
			if len(r.splashes) < 48 {
				r.splashes = append(r.splashes, splash{x: float64(e.Pos.X) / 65536, y: float64(e.Pos.Y) / 65536, seat: e.Seat})
			}
			continue
		}
		if e.Shot {
			if len(r.shots) < 128 {
				r.shots = append(r.shots, shot{from: Point{float64(e.From.X) / 65536, float64(e.From.Y) / 65536}, to: Point{float64(e.Pos.X) / 65536, float64(e.Pos.Y) / 65536}, weapon: e.Weapon, age: func() int {
					if e.Weapon == sim.Uzi {
						return -2 * burstIndex
					}
					return 0
				}()})
				burstIndex++
			}
			continue
		}
		if e.Radius > 0 && len(r.blasts) < 128 {
			r.blasts = append(r.blasts, blast{x: float64(e.Pos.X) / 65536, y: float64(e.Pos.Y) / 65536, radius: float64(e.Radius), weapon: e.Weapon})
		}
		box := image.Rect(e.Dirty.X0, e.Dirty.Y0, e.Dirty.X1, e.Dirty.Y1).Inset(-5)
		dirty = dirty.Union(box)
	}
	if !dirty.Empty() {
		r.update(s, dirty.Intersect(r.Terrain.Bounds()))
	}
}
func (r *Scene) update(s *sim.State, box image.Rectangle) {
	r.dirty = r.dirty.Union(box)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			c := color.RGBA{}
			if s.Terrain.Solid(x, y) {
				c = materials.Color(s.Config.Seed, x, y)
				rim := hazardFor(s.Config.Seed).light
				if !s.Terrain.Solid(x, y-1) {
					c = rim
				} else if !s.Terrain.Solid(x, y-4) {
					c = color.RGBA{uint8((int(c.R) + int(rim.R)) / 2), uint8((int(c.G) + int(rim.G)) / 2), uint8((int(c.B) + int(rim.B)) / 2), 255}
				} else if !s.Terrain.Solid(x-2, y) || !s.Terrain.Solid(x+2, y) || !s.Terrain.Solid(x, y+3) {
					c = color.RGBA{c.R / 2, c.G / 2, c.B / 2, 255}
				}

			}
			r.Terrain.SetRGBA(x, y, c)
		}
	}
	r.revision++
}

type View struct {
	ImpactReview        bool
	Network             bool
	Settlement          string
	TableLobby          TableLobbyView
	Precision           bool
	Settings            SettingsView
	BridgeStatus        string
	BridgeLobby         BridgeLobbyView
	Muted               bool
	WeaponPage          int
	WeaponMenu          bool
	MenuOrigin          image.Point
	KeyLabels           map[string]string
	ControlsOpen        bool
	ControlRows         [][2]string
	BindingIndex        int
	ControlsMessage     string
	HideTop, HideBottom bool
	Camera              *Camera
	Arena               bool
	Dev                 bool
	MouseX, MouseY      int
	Paused              bool
	Frame               uint64
	Power               int
	Error               string
}

func (r *Scene) Draw(c Canvas, s *sim.State, v View) {
	c.Rect(0, 0, Width, Height, Navy)
	cameraX := 0.0
	if v.Arena {
		if v.Camera == nil {
			cam := Camera{Zoom: 1.25, Bounds: Stage(v.HideTop, v.HideBottom)}
			cam.Focus(s)
			v.Camera = &cam
		}
		cameraX = v.Camera.X * v.Camera.Zoom
	}
	background(c, v.Frame, cameraX)
	if v.Arena {
		r.arena(c, s, v)
		if v.Network && v.Error != "" && s.Phase != sim.Ended {
			y := 86.0
			if v.HideTop {
				y = 18
			}
			c.Rect(320, y, 800, 42, color.RGBA{7, 18, 32, 225})
			c.Text(shortTableText(v.Error, 94), 338, y+13, 13, Mint)
		}
		if s.Phase == sim.Ended && !v.ImpactReview {
			drawVictory(c, s, v)
		} else if v.HideBottom {
			drawCompactPower(c, s, v)
		}
	} else if v.TableLobby.Open {
		drawTableLobby(c, v)
	} else {
		lobby(c, v)
	}
	if v.Settings.Open {
		drawSettings(c, v)
	}
}

func background(c Canvas, frame uint64, cameraX float64) {
	for y := 0; y < Height; y += 6 {
		t := float64(y) / Height
		c.Rect(0, float64(y), Width, 6, color.RGBA{uint8(9 + 5*t), uint8(16 + 14*t), uint8(35 + 17*t), 255})
	}
	for n := 0; n < 115; n++ {
		x := repeat(float64(n*193+71)-cameraX*0.04, Width)
		y := float64((n*97 + 29) % 620)
		r := 0.8
		if n%9 == 0 {
			r = 1.5
		}
		c.Circle(x, y, r, color.RGBA{95, 144, 173, uint8(80 + n%100)})
	}
	moonX := 1120 - cameraX*0.08
	// Distant moon, orbital ring and a soft atmosphere, all presentational.
	c.Circle(moonX, 210, 151, color.RGBA{34, 85, 97, 15})
	c.Circle(moonX, 210, 137, color.RGBA{38, 93, 108, 22})
	c.Circle(moonX, 210, 122, color.RGBA{30, 66, 90, 255})
	c.Circle(moonX+28, 182, 119, color.RGBA{13, 28, 49, 255})
	for n := 0; n < 100; n++ {
		a := float64(n) * math.Pi * 2 / 100
		b := float64(n+1) * math.Pi * 2 / 100
		c.Line(moonX+190*math.Cos(a), 210+32*math.Sin(a)+28*math.Cos(a), moonX+190*math.Cos(b), 210+32*math.Sin(b)+28*math.Cos(b), 1, color.RGBA{88, 156, 172, 100})
	}
	for layer := 0; layer < 3; layer++ {
		pts := []Point{{0, 900}}
		for x := 0; x <= Width+50; x += 55 {
			y := 570 + layer*70 + int(60*math.Sin((float64(x)+cameraX*(0.14+float64(layer)*0.12))*0.012+float64(layer)*2))
			pts = append(pts, Point{float64(x), float64(y)})
		}
		pts = append(pts, Point{Width, 900})
		c.Poly(pts, color.RGBA{uint8(11 + layer*2), uint8(30 + layer*6), uint8(50 + layer*5), 255})
	}
	// Fine scan lines give the menus a restrained technical texture.
	for y := 0; y < Height; y += 4 {
		c.Rect(0, float64(y), Width, 1, color.RGBA{2, 7, 19, 12})
	}
}

func brand(c Canvas) {
	im := cover.Logo()
	c.Image(im, 40, 18, 310, 310*float64(im.Bounds().Dy())/float64(im.Bounds().Dx()))
}

func lobby(c Canvas, v View) {
	brand(c)
	c.Text("DEVELOPMENT BUILD", 1120, 43, 12, Muted)
	settingsCog(c)
	if v.BridgeStatus != "" {
		c.Text(v.BridgeStatus, 65, 773, 13, Muted)
	}
	c.Line(40, 106, 1400, 106, 1, color.RGBA{53, 80, 99, 180})
	c.Text("THE NEXT TURN IS YOURS.", 65, 171, 13, Mint)
	c.Text("SMALL TICKETS.", 60, 209, 63, White)
	c.Text("BIG CONSEQUENCES.", 60, 283, 63, White)
	c.Text("Assemble your Stakey squad. Shape the battlefield.", 65, 373, 20, Muted)
	c.Text("Every turn counts. Every player verifies.", 65, 405, 20, Muted)
	c.Rect(65, 474, 391, 66, Panel)
	c.Rect(65, 474, 3, 66, Mint)
	c.Text("PAID MATCHES", 87, 488, 14, White)
	c.Text("Create or join a table through dcrpulse", 87, 513, 12, Muted)
	c.Text("2—6 SQUADS", 65, 588, 14, White)
	c.Text("DESTRUCTIBLE WORLDS", 232, 588, 14, White)
	c.Text("ASYNC TURNS", 508, 588, 14, White)
	c.Text("Built on Decred", 65, 623, 13, Mint)
	if v.Dev {
		c.Rect(65, 675, 391, 56, Panel)
		c.Text("RETURN TO INTERNAL ARENA", 87, 688, 16, Mint)
		c.Text("ENTER to resume", 87, 710, 11, Muted)
	}
	tableButton(c, LobbyTablesRect, "TABLE LOBBY  >", true, v)
	// Hero Stakey, drawn from reusable geometry rather than a screenshot of
	// the comic. The renderer controls all expressions and equipment.
	c.Circle(1058, 590, 203, color.RGBA{30, 102, 111, 18})
	c.Circle(1058, 590, 160, color.RGBA{40, 140, 140, 13})
	c.Poly([]Point{{787, 744}, {960, 701}, {1228, 725}, {1320, 811}, {763, 811}}, color.RGBA{20, 54, 72, 255})
	c.Line(809, 746, 979, 714, 3, Mint)
	stakey(c, 1045, 702, 7, Mint, 1, float64(v.Frame)/15, true)
	stakey(c, 1290, 770, 2.5, Blue, -1, float64(v.Frame)/17, false)
	for i := 0; i < 7; i++ {
		x := 795 + float64(i)*65
		c.Poly([]Point{{x, 795}, {x + 12, 750 - float64(i%3)*13}, {x + 23, 784}, {x + 17, 807}}, color.RGBA{24, 130, 136, 200})
	}
	drawBridgeLobby(c, v.BridgeLobby)
	c.Rect(40, 817, 1360, 53, Panel)
	c.Text("STAKEY / SQUAD 01", 65, 834, 12, Mint)
	c.Text("Original characters inspired by pLabarta's Decred comic", 480, 835, 12, Muted)
	c.Text("DECRED  •  P2P", 1220, 835, 12, White)
	if v.Error != "" {
		c.Text(v.Error, 65, 746, 13, color.RGBA{255, 158, 110, 255})
	}
}

func (r *Scene) arena(c Canvas, s *sim.State, v View) {
	cam := Camera{Zoom: 1.25}
	cam.Focus(s)
	if v.Camera != nil {
		cam = *v.Camera
		cam.Clamp(s)
	}
	cam.Bounds = Stage(v.HideTop, v.HideBottom)
	cam.Clamp(s)
	v.Camera = &cam
	margin, top := cam.Origin()
	scale := cam.Zoom
	bounds := cam.Viewport()
	restore := c.Clip(bounds)
	c.Rect(float64(bounds.Min.X), float64(bounds.Min.Y), float64(bounds.Dx()), float64(bounds.Dy()), color.RGBA{23, 50, 70, 150})

	drawScenery(c, s, cam, false)
	r.drawWeather(c, s, cam, false)
	c.Image(r.Terrain, margin, top, float64(s.Config.Width)*scale, float64(s.Config.Height)*scale)
	fx := func(x int32) float64 { return margin + float64(x)/65536*scale }
	fy := func(y int32) float64 { return top + float64(y)/65536*scale }
	for _, o := range s.Objects {
		drawObject(c, o, fx(int32(o.Pos.X)), fy(int32(o.Pos.Y)), scale, s.Tick)
	}
	for i, w := range s.Worms {
		if w.HP <= 0 {
			r.drawGrave(c, s, i, margin, top, scale)
			continue
		}
		x, y := fx(int32(w.Pos.X)), fy(int32(w.Pos.Y))
		col := teams[w.Seat]
		actor := r.presentedPose(i, s, v.Power)
		stakey(c, x, y, math.Max(scale, 1.0), col, int(w.Facing), float64(s.Tick)*AnimationSpeed/7, i == int(s.Active), actor)
		if i == int(s.Active) && s.Phase != sim.Ended {
			// A readable marker survives team-color similarity and busy terrain.
			pulse := 2 * math.Sin(float64(s.Tick)*math.Pi/30)
			c.Line(x-17, y+4, x+17, y+4, 3, col)
			c.Rect(x-43, y-90+pulse, 86, 18, Panel)
			c.Text(fmt.Sprintf("ACTIVE · S%d", w.Seat+1), x-39, y-89+pulse, 11, White)
			sprite := weapons.Sprite(int(s.Weapon))
			a := float64(s.Aim) * 2 * math.Pi / 65536
			recoil := actor.recoil
			width, height := 28*scale, 18*scale
			if math.Cos(a) < 0 {
				height = -height
			}
			wc := c
			angle := actor.angle
			if angle != 0 {
				wc = &actorCanvas{Canvas: c, x: x, y: y - 15*scale, angle: angle}
			}
			if !actor.jetpack {
				wc.RotatedImage(sprite, x+math.Cos(a)*(13-recoil)*scale, y+(actor.bob-16*actor.sy)*scale, width, height, a)
			}
		}
		if w.Chute && !w.Grounded {
			sprite := weapons.Sprite(int(sim.Parachute))
			c.Image(sprite, x-30*scale, y-70*scale, 60*scale, 60*scale)
		}
		c.Rect(x-29, y-52, 58, 20, Navy)
		c.Text(fmt.Sprintf("%d · %d", w.Seat+1, w.HP), x-24, y-50, 13, White)
		if i == int(s.Active) {
			c.Poly([]Point{{x - 5, y - 66}, {x + 5, y - 66}, {x, y - 59}}, col)
		}
	}
	if len(s.Rope.Pivots) > 0 {
		w := s.Worms[s.Active]
		prev := Point{fx(int32(w.Pos.X)), fy(int32(w.Pos.Y))}
		for n := len(s.Rope.Pivots) - 1; n >= 0; n-- {
			p := s.Rope.Pivots[n].Pos
			next := Point{fx(int32(p.X)), fy(int32(p.Y))}
			c.Line(prev.X, prev.Y, next.X, next.Y, 2, Mint)
			prev = next
		}
	}
	for _, p := range s.Projectiles {
		x, y := fx(int32(p.Pos.X)), fy(int32(p.Pos.Y))
		sprite := weapons.Sprite(int(p.Weapon))
		size := math.Max(20*scale, 16)
		angle := float64(s.Tick) * AnimationSpeed * 0.10
		if p.Weapon == sim.Bazooka {
			sprite = weapons.Projectile(0)
			size = math.Max(16*scale, 12)
			angle = math.Atan2(float64(p.Vel.Y), float64(p.Vel.X))
		}
		switch p.Weapon {
		case sim.Mortar:
			sprite = weapons.ExpansionProjectile(0)
		case sim.BouncingBomb:
			sprite = weapons.ExpansionProjectile(1)
		case sim.DrillRocket:
			sprite = weapons.ExpansionProjectile(2)
		case sim.HomingRocket:
			sprite = weapons.ExpansionProjectile(3)
		case sim.Airstrike:
			sprite = weapons.ExpansionProjectile(4)
		case sim.Flamethrower:
			sprite = weapons.ExpansionProjectile(5)
		case sim.RemoteCharge:
			sprite = weapons.ExpansionProjectile(6)
			angle = 0
		case sim.HeavyCluster:
			sprite = weapons.ExpansionProjectile(7)
		case sim.BisonBomb:
			angle = 0
			size = 32 * scale
		}
		if p.Weapon == sim.Mortar || p.Weapon == sim.DrillRocket || p.Weapon == sim.HomingRocket || p.Weapon == sim.Airstrike || p.Weapon == sim.Flamethrower {
			angle = math.Atan2(float64(p.Vel.Y), float64(p.Vel.X))
		}
		if p.Burning {
			angle = -math.Pi / 2
			size = 16 * scale * (1 + 0.12*math.Sin(float64(s.Tick)*AnimationSpeed*0.4))
		}
		if p.Weapon == sim.Grenade && p.Radius == 24 {
			sprite = weapons.Projectile(3)
			size = math.Max(14*scale, 12)
		}
		if p.Weapon == sim.Mine || p.Weapon == sim.Dynamite {
			angle = 0
		}
		bounds := sprite.Bounds()
		sw := size * float64(bounds.Dx()) / float64(bounds.Dy())
		if p.Weapon == sim.BisonBomb && p.Vel.X < 0 {
			sw = -sw
		}
		c.RotatedImage(sprite, x, y, sw, size, angle)
	}
	for _, shot := range r.shots {
		if shot.age < 0 {
			continue
		}
		x0, y0 := margin+shot.from.X*scale, top+shot.from.Y*scale
		x1, y1 := margin+shot.to.X*scale, top+shot.to.Y*scale
		distance := math.Hypot(shot.to.X-shot.from.X, shot.to.Y-shot.from.Y)
		t := math.Min(1, float64(shot.age+1)*64/math.Max(1, distance))
		x, y := x0+(x1-x0)*t, y0+(y1-y0)*t
		tail := math.Max(0, t-40/math.Max(1, distance))
		c.Line(x0+(x1-x0)*tail, y0+(y1-y0)*tail, x, y, math.Max(1, scale), color.RGBA{255, 209, 119, uint8(140 * (24 - shot.age) / 24)})
		sprite := weapons.Projectile(1)
		size := 12.0
		if shot.weapon == sim.Uzi {
			sprite = weapons.Projectile(2)
			size = 8
		}
		if shot.weapon == sim.FirePunch || shot.weapon == sim.BaseballBat {
			sprite = weapons.Sprite(int(shot.weapon))
			size = 24
		}
		angle := math.Atan2(y1-y0, x1-x0)
		melee := shot.weapon == sim.BaseballBat || shot.weapon == sim.FirePunch
		if melee {
			angle += math.Sin(float64(shot.age)/12*math.Pi) * 0.7
			size = 32 * scale
		}
		if (shot.age < 22 && t < 1) || (melee && shot.age < 12) {
			bounds := sprite.Bounds()
			c.RotatedImage(sprite, x, y, size*float64(bounds.Dx())/float64(bounds.Dy()), size, angle)
		}
	}

	for _, b := range r.blasts {
		x, y := margin+b.x*scale, top+b.y*scale
		t := float64(b.age) / 42
		radius := b.radius * scale
		hot := color.RGBA{255, 208, 123, 255}
		spark := color.RGBA{255, 185, 92, 255}
		switch b.weapon {
		case sim.Grenade, sim.BouncingBomb:
			hot = color.RGBA{166, 255, 165, 255}
			spark = Mint
		case sim.Mortar, sim.Dynamite:
			hot = color.RGBA{255, 231, 190, 255}
			spark = color.RGBA{255, 111, 53, 255}
		case sim.DrillRocket:
			hot = color.RGBA{144, 231, 255, 255}
			spark = Blue
		case sim.HomingRocket, sim.Airstrike:
			hot = color.RGBA{193, 204, 255, 255}
			spark = color.RGBA{113, 150, 255, 255}
		case sim.ClusterBomb, sim.HeavyCluster:
			hot = color.RGBA{255, 239, 122, 255}
			spark = color.RGBA{255, 143, 78, 255}
		case sim.Mine, sim.RemoteCharge:
			hot = color.RGBA{229, 172, 255, 255}
			spark = color.RGBA{191, 120, 255, 255}
		case sim.BisonBomb:
			hot = color.RGBA{255, 189, 137, 255}
			spark = color.RGBA{184, 146, 93, 255}
		}
		if b.weapon == sim.Mortar || b.weapon == sim.HeavyCluster {
			c.Circle(x, y, radius*(.6+t), color.RGBA{spark.R, spark.G, spark.B, uint8(70 * (1 - t))})
		}
		c.Circle(x, y-15*t, radius*(0.5+t), color.RGBA{43, 74, 92, uint8(95 * (1 - t))})
		if t < 0.35 {
			c.Circle(x, y, radius*(0.2+t), color.RGBA{hot.R, hot.G, hot.B, uint8(240 * (1 - t/0.35))})
		}
		for n := 0; n < 12; n++ {
			a := float64(n)*math.Pi/6 + b.x*0.013
			d := radius * (0.3 + 1.8*t)
			px, py := x+math.Cos(a)*d, y+math.Sin(a)*d+25*t*t
			c.Line(px, py, px-math.Cos(a)*7*(1-t), py-math.Sin(a)*7*(1-t), 2*scale, color.RGBA{spark.R, spark.G, spark.B, uint8(255 * (1 - t))})
		}
	}
	r.drawFloaters(c, margin, top, scale)
	r.hazard(c, s, v, cam)

	if s.Phase == sim.Playing {
		w := s.Worms[s.Active]
		a := float64(s.Aim) * math.Pi * 2 / 65536
		x, y := fx(int32(w.Pos.X)), fy(int32(w.Pos.Y))-12*scale
		for k := 22; k < 70; k += 9 {
			c.Circle(x+math.Cos(a)*float64(k), y+math.Sin(a)*float64(k), 1.4, Mint)
		}
	}
	if s.HasTarget && sim.UsesTarget(s.Weapon) {
		x, y := fx(int32(s.Target.X)), fy(int32(s.Target.Y))
		c.Circle(x, y, 9, Mint)
		c.Circle(x, y, 6, Navy)
		c.Line(x-16, y, x+16, y, 2, Mint)
		c.Line(x, y-16, x, y+16, 2, Mint)
		if s.Weapon == sim.Girder {
			col := color.RGBA{39, 211, 162, 90}
			if !s.CanPlaceGirder() {
				col = color.RGBA{240, 104, 120, 120}
			}
			c.Rect(x-40*scale, y-6*scale, 80*scale, 12*scale, col)
		}
	}
	drawScenery(c, s, cam, true)
	r.drawWeather(c, s, cam, true)
	restore()
	r.hud(c, s, v)
}

func WeaponName(w sim.Weapon) string {
	names := []string{"BAZOOKA", "GRENADE", "CLUSTER BOMB", "SHOTGUN", "UZI", "FIRE PUNCH", "DYNAMITE", "MINE", "TELEPORT", "NINJA ROPE", "JETPACK", "MORTAR", "BAT", "BOUNCING BOMB", "DRILL ROCKET", "PARACHUTE", "HOMING ROCKET", "AIRSTRIKE", "FLAMETHROWER", "REMOTE CHARGE", "HEAVY CLUSTER", "BISON BOMB", "GIRDER", "MINING DRILL"}
	if int(w) >= len(names) {
		return "UNKNOWN"
	}
	return names[w]
}

// Stakey uses a notched ticket body, a blue folded edge and expressive eyes.
func stakey(c Canvas, x, y, k float64, col color.RGBA, facing int, frame float64, active bool, poses ...pose) {
	if k < 0.7 {
		k = 0.7
	}
	if len(poses) > 0 && poses[0].angle != 0 {
		c = &actorCanvas{Canvas: c, x: x, y: y - 15*k, angle: poses[0].angle}
	}
	y += math.Sin(frame*0.35+x*0.01) * k * 0.22
	p := pose{sx: 1, sy: 1, lookX: float64(facing) * .8}
	if len(poses) > 0 {
		p = poses[0]
	}
	// Separate axis warping keeps feet planted during squash and stretch.
	px := func(v float64) float64 { return x + (v*p.sx-float64(facing)*p.recoil)*k }
	py := func(v float64) float64 { return y + v*p.sy*k }
	if p.hurt {
		col = color.RGBA{255, 158, 138, 255}
	}

	boot := color.RGBA{12, 24, 44, 255}
	// Grounded feet retain their contact height while the torso bounces.
	leftX, rightX := -5+p.stride+p.trail, 6-p.stride+p.trail
	leftY, rightY := -1+math.Min(0, p.stride)-p.tuck, -1+math.Min(0, -p.stride)-p.tuck
	c.Circle(px(leftX), py(leftY), 3*k, boot)
	c.Circle(px(rightX), py(rightY), 3*k, boot)
	c.Line(px(-4), py(-7+p.bob), px(-4+p.trail*.4), py(-4-p.tuck), 2*k, col)
	c.Line(px(-4+p.trail*.4), py(-4-p.tuck), px(leftX), py(leftY), 2*k, col)
	c.Line(px(5), py(-7+p.bob), px(5+p.trail*.4), py(-4-p.tuck), 2*k, col)
	c.Line(px(5+p.trail*.4), py(-4-p.tuck), px(rightX), py(rightY), 2*k, col)
	y += p.bob * k
	if p.jetpack {
		back := -float64(facing) * 11
		if p.thrust {
			flame := 8 + 3*math.Sin(frame*2.7)
			c.Poly([]Point{{px(back - 3), py(-10)}, {px(back + 3), py(-10)}, {px(back + p.trail), py(flame)}}, color.RGBA{255, 151, 68, 230})
			c.Poly([]Point{{px(back - 1.5), py(-10)}, {px(back + 1.5), py(-10)}, {px(back), py(flame * .55)}}, White)
		}
		c.Image(weapons.Sprite(int(sim.Jetpack)), px(back)-10*k, py(-25), 20*k, 23*k)
	}
	fold := float64(facing)
	c.Poly([]Point{{px(-11 * fold), py(-29)}, {px(-14 * fold), py(-9)}, {px(-8 * fold), py(-9)}, {px(-7 * fold), py(-29)}}, Blue)
	pts := []Point{{px(-10), py(-8)}, {px(-10), py(-29)}}
	for n := 0; n < 4; n++ {
		for j := 0; j <= 8; j++ {
			a := float64(j) * math.Pi / 8
			pts = append(pts, Point{px(-10 + float64(n)*5 + float64(j)*5/8), py(-29 + math.Sin(a)*1.7)})
		}
	}
	pts = append(pts, Point{px(10), py(-8)})
	c.Poly(pts, col)
	c.Line(px(-10), py(-25), px(-10), py(-9), 0.7*k, color.RGBA{117, 248, 213, 200})
	drawStakeyFace(c, px, py, k, p, col)
	c.Line(px(-10), py(-19), px(-14), py(-12-p.handLift), 1.8*k, col)
	c.Circle(px(-14), py(-12-p.handLift), 1.7*k, col)
	c.Line(px(10), py(-19), px(14), py(-15-p.handLift), 1.8*k, col)
	if active {
		c.Rect(px(-11), py(-28), 22*p.sx*k, 3*p.sy*k, boot)
		c.Rect(px(-1), py(-29), 5*p.sx*k, 4*p.sy*k, Blue)
		if len(poses) == 0 {
			c.Rect(px(8), py(-19), 13*k, 4*k, color.RGBA{35, 52, 72, 255})
			c.Circle(px(21), py(-17), 2.7*k, color.RGBA{61, 84, 104, 255})
			c.Circle(px(21), py(-17), 1.4*k, Mint)
			c.Line(px(9), py(-18), px(16), py(-18), 0.7*k, White)
		}
	}
}

func repeat(x, period float64) float64 { return x - math.Floor(x/period)*period }
