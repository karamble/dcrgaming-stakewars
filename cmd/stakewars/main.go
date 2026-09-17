//go:build desktop

// Command stakewars is the SDK-backed desktop game. Cooperative matches use
// durable signed turns; isolated engineering fixtures require the dev tag.
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	flags "github.com/jessevdk/go-flags"
	"github.com/karamble/dcrstakewars/internal/appconfig"
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"github.com/karamble/dcrstakewars/internal/logging"
	"github.com/karamble/dcrstakewars/internal/session"
	"github.com/karamble/dcrstakewars/internal/tablelobby"
	"github.com/karamble/dcrstakewars/internal/turnbatch"
	"image"
	"image/png"

	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/karamble/dcrstakewars/pkg/render"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

type game struct {
	session                             *session.Controller
	sessionView                         session.View
	networkMatch                        string
	turnInputs                          []sim.Input
	remoteBatch                         *turnbatch.Batch
	submitted                           bool
	startup, startupDrawn, startupReady bool
	tableOpen                           bool
	tableSelected                       int
	tableState                          *tablelobby.Table
	tableSerial                         uint64
	tableDemoScenario                   int
	tableHelp                           bool
	settingsOpen                        bool
	settingsTab, settingsFocus          int
	settingsSelectAll, clipboardBusy    bool
	settingsRevision                    uint64
	bridgeConfig                        bridgeconn.Config
	bridgePath, bridgeStatus            string
	dataDir                             string
	bridgeConnected, bridgeBusy         bool
	bridgeGeneration                    uint64
	bridgeCancel                        context.CancelFunc
	bridgeDone                          <-chan struct{}
	bridgeUpdates                       chan connectionUpdate
	bridgeNotice                        string
	bridgeInvite                        *bridgeconn.Invitation
	clipboardUpdates                    chan clipboardResult

	screenshotAfter uint64
	speaker         speaker
	weaponPage      int
	weaponMenu      bool
	menuOrigin      image.Point
	controlsOpen    bool
	keys            map[string]ebiten.Key
	controlsPath    string
	controlsMessage string
	bindingIndex    int

	hideTop, hideBottom bool
	seed                uint64
	follow              bool
	camera              render.Camera
	dragX, dragY        int
	s                   *sim.State
	scene               *render.Scene
	gpu                 *render.GPU
	arena               bool
	frame               uint64
	power               int
	charging            bool
	fireReleaseRequired bool
	paused              bool
	message             string
	screenshot          string
	shotDone            bool
	quitRequested       bool
}

func (g *game) reset() error {
	config := sim.DefaultConfig()
	if devEnabled {
		config.Seed = g.seed
		if config.Seed == 0 {
			var seedBytes [8]byte
			if _, err := rand.Read(seedBytes[:]); err != nil {
				return err
			}
			config.Seed = binary.LittleEndian.Uint64(seedBytes[:])
		}
	}
	s, err := sim.New(config)
	if err != nil {
		return err
	}
	g.s = s
	g.follow = true
	g.camera = render.Camera{Zoom: 1.25, Bounds: render.Stage(g.hideTop, g.hideBottom)}
	g.camera.Focus(s)
	g.scene = render.New(s)
	g.gpu = render.NewGPU(g.scene)
	g.charging = false
	g.power = 0
	g.paused = false
	g.weaponMenu = false
	g.weaponPage = 0
	g.controlsOpen = false
	return nil
}
func (g *game) Update() error {
	previousScene := g.scene
	previousTick := uint32(0)
	if g.s != nil {
		previousTick = g.s.Tick
	}
	defer func() {
		if g.s != nil && g.arena && !g.paused {
			if g.scene == previousScene && g.s.Tick == previousTick {
				// Keep damage and death animation alive while async turn delivery waits.
				g.scene.Update(g.s, nil)
			}
			if g.camera.Advance(g.s, g.follow) {
				g.follow = true
			}
		}
	}()
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) || ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight)
	if g.shotDone || g.quitRequested || ebiten.IsWindowBeingClosed() || (ctrl && inpututil.IsKeyJustPressed(ebiten.KeyQ)) {
		return ebiten.Termination
	}
	g.frame++
	if g.startup {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			return ebiten.Termination
		}
		if g.startupDrawn && !g.startupReady {
			if err := g.reset(); err != nil {
				return err
			}
			render.PreloadVisuals()
			g.startupReady = true
		}
		if g.startupReady && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)) {
			g.startup = false
		}
		return nil
	}
	if !g.pressed("fire") {
		g.fireReleaseRequired = false
	}
	g.pollSettings()
	if g.settingsOpen {
		g.updateSettings()
		return g.advanceNetworkIdle()
	}
	if !g.arena && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if image.Pt(x, y).In(render.LobbySettingsRect) {
			g.settingsOpen = true
			g.bindingIndex = -1
			return nil
		}
	}
	if !g.controlsOpen && inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.speaker.muted = !g.speaker.muted
		g.speaker.stop()
	}
	if !g.arena && g.tableOpen {
		g.updateTableLobby()
		return g.advanceNetworkIdle()
	}
	if !g.arena {
		x, y := ebiten.CursorPosition()
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && image.Pt(x, y).In(render.LobbyQuitRect)) {
			return ebiten.Termination
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && image.Pt(x, y).In(render.LobbyTablesRect) {
			g.tableOpen = true
			return nil
		}
		if (devEnabled || g.networkMatch != "") && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && image.Pt(x, y).In(render.LobbyResumeRect))) {
			g.arena = true
			g.paused = false
		}
		return g.advanceNetworkIdle()
	}
	if g.s.Phase == sim.Ended {
		if g.camera.ReviewingImpact() {
			return nil
		}
		x, y := ebiten.CursorPosition()
		click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
		if devEnabled && g.networkMatch == "" && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || (click && image.Pt(x, y).In(render.VictoryAgainRect))) {
			return g.reset()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (click && image.Pt(x, y).In(render.VictoryLobbyRect)) {
			g.arena = false
			g.paused = false
		}
		return nil
	}
	if !g.paused && inpututil.IsKeyJustPressed(ebiten.KeyK) && g.bindingIndex < 0 {
		g.controlsOpen = !g.controlsOpen
		g.weaponMenu = false
		g.charging = false
		g.power = 0
	}
	if g.controlsOpen {
		g.updateControls()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && !g.controlsOpen {
		if g.weaponMenu {
			g.weaponMenu = false
		} else {
			g.paused = !g.paused
		}
		g.charging = false
		g.power = 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) && !g.controlsOpen {
		g.paused = !g.paused
	}
	if g.paused {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			if image.Pt(x, y).In(render.ResumeRect) {
				g.paused = false
			}
			if image.Pt(x, y).In(render.LeaveRect) {
				g.arena = false
				g.paused = false
			}
		}
		g.charging = false
		g.power = 0
		return g.advanceNetworkIdle()
	}
	uiClick := false
	if !g.paused && !g.controlsOpen {
		x, y := ebiten.CursorPosition()
		click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
		if inpututil.IsKeyJustPressed(ebiten.KeyF1) || (click && image.Pt(x, y).In(render.TopToggleRect)) {
			g.hideTop = !g.hideTop
			uiClick = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF2) || (click && image.Pt(x, y).In(render.BottomToggleRect)) {
			g.hideBottom = !g.hideBottom
			uiClick = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyH) {
			hide := !(g.hideTop && g.hideBottom)
			g.hideTop = hide
			g.hideBottom = hide
		}
		g.camera.SetViewport(render.Stage(g.hideTop, g.hideBottom), g.s)
	}

	if g.paused {
		return nil
	}
	mx, my := ebiten.CursorPosition()
	if !g.controlsOpen && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.weaponMenu = !g.weaponMenu
		g.menuOrigin = render.MenuOrigin(mx, my)
		g.charging = false
		g.power = 0
	}
	selected := -1
	menuWasOpen := g.weaponMenu
	if g.weaponMenu && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if w, ok := render.MenuWeaponAt(g.menuOrigin, mx, my); ok {
			selected = int(w)
		}
		g.weaponMenu = false
		uiClick = true
	}
	if g.controlsOpen || menuWasOpen {
		uiClick = true
	}
	if !uiClick && !g.hideBottom && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if image.Pt(mx, my).In(render.TrayPrev) {
			g.weaponPage = (g.weaponPage + render.WeaponPages - 1) % render.WeaponPages
			uiClick = true
		}
		if image.Pt(mx, my).In(render.TrayNext) {
			g.weaponPage = (g.weaponPage + 1) % render.WeaponPages
			uiClick = true
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !uiClick && !g.hideTop && image.Pt(mx, my).In(render.CameraBar) {
		g.follow = false
		g.camera.X = float64(mx-render.CameraBar.Min.X)/float64(render.CameraBar.Dx())*float64(g.s.Config.Width) - float64(g.camera.Viewport().Dx())/g.camera.Zoom/2
	}
	if ebiten.IsKeyPressed(ebiten.KeyPageUp) {
		g.follow = false
		g.camera.Y -= 14 / g.camera.Zoom
	}
	if ebiten.IsKeyPressed(ebiten.KeyPageDown) {
		g.follow = false
		g.camera.Y += 14 / g.camera.Zoom
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.follow = false
		g.camera.X -= 14 / g.camera.Zoom
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		g.follow = false
		g.camera.X += 14 / g.camera.Zoom
	}
	if ebiten.IsFocused() && !uiClick && !render.HUDContains(mx, my, g.hideTop, g.hideBottom) {
		dx, dy := render.EdgePan(g.camera.Viewport(), mx, my)
		if dx != 0 || dy != 0 {
			g.follow = false
			g.camera.X += dx / g.camera.Zoom
			g.camera.Y += dy / g.camera.Zoom
		}
	}
	if !uiClick && !render.HUDContains(mx, my, g.hideTop, g.hideBottom) && image.Pt(mx, my).In(g.camera.Viewport()) {

		dx, dy := ebiten.Wheel()
		if dx != 0 {
			g.follow = false
		}
		g.camera.X += dx * 50 / g.camera.Zoom
		if dy != 0 {
			g.follow = false
			wx, wy := g.camera.World(float64(mx), float64(my))
			g.camera.Zoom *= 1 + dy*0.1
			g.camera.Clamp(g.s)
			nx, ny := g.camera.World(float64(mx), float64(my))
			g.camera.X += wx - nx
			g.camera.Y += wy - ny
		}
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) && !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
			g.follow = false
			g.camera.X += float64(g.dragX-mx) / g.camera.Zoom
			g.camera.Y += float64(g.dragY-my) / g.camera.Zoom
		}
	}
	g.dragX, g.dragY = mx, my
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.follow = true
		g.camera.Focus(g.s)
	}
	g.camera.Clamp(g.s)
	if g.networkMatch != "" && !g.localTurn() {
		return g.updateRemoteTurn()
	}
	var in []sim.Input
	add := func(kind sim.EventKind, param int32) {
		in = append(in, sim.Input{Tick: g.s.Tick, Seat: g.s.ActiveSeat(), Sequence: uint16(len(in)), Kind: kind, Param: param})
	}
	move := int32(0)
	if !g.controlsOpen && g.pressed("left") {
		move--
	}
	if !g.controlsOpen && g.pressed("right") {
		move++
	}
	facing := g.s.Worms[g.s.Active].Facing
	if move != 0 {
		facing = int8(move)
	}
	intendedAim := g.s.Aim
	if move != 0 {
		intendedAim = sim.FacingAim(intendedAim, facing)
	}
	precise := !g.controlsOpen && g.pressed("precision")
	if precise && move != 0 {
		add(sim.Face, move)
		move = 0
	}
	if move != int32(g.s.Walk) {
		add(sim.Move, move)
	}
	aim := int32(intendedAim)
	aimStep := int32(480)
	if precise {
		aimStep = 24
	}
	if facing < 0 {
		aimStep = -aimStep
	}
	ropeAttached := len(g.s.Rope.Pivots) > 0
	if !g.controlsOpen && g.pressed("aimUp") {
		if ropeAttached {
			add(sim.RopeAdjust, -1)
		} else if g.s.Weapon != sim.Jetpack {
			aim -= aimStep
		}
	}
	if !g.controlsOpen && g.pressed("aimDown") {
		if ropeAttached {
			add(sim.RopeAdjust, 1)
		} else {
			aim += aimStep
		}
	}
	aim &= 65535
	if aim != int32(g.s.Aim) {
		add(sim.Aim, aim)
	}
	if !g.controlsOpen && g.justPressed("jump") {
		kind := int32(0)
		if precise {
			kind = 2
		}
		add(sim.Jump, kind)
	}
	if !g.controlsOpen && g.justPressed("highJump") {
		add(sim.Jump, 1)
	}
	if !g.controlsOpen && g.justPressed("skip") {
		add(sim.Skip, 0)
	}
	if !g.controlsOpen && g.s.Phase == sim.Retreating && g.s.Weapon == sim.Parachute && g.justPressed("fire") {
		add(sim.Fire, 1)
	}
	if g.s.Phase == sim.Playing || g.s.Phase == sim.Ready {
		if selected >= 0 {
			add(sim.SelectWeapon, int32(selected))
			g.weaponPage = selected / render.TraySlots
			g.charging = false
			g.power = 0
		}
		if !g.controlsOpen {
			for i, key := range []ebiten.Key{ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5} {
				if inpututil.IsKeyJustPressed(key) {
					add(sim.SetFuse, int32(i+1))
				}
			}
		}
		if !g.hideBottom && !uiClick && g.s.Shots == 0 && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			if weapon, ok := render.WeaponAt(x, y, g.weaponPage); ok {
				add(sim.SelectWeapon, int32(weapon))
				g.charging = false
				g.power = 0
			}
		}
		if !g.controlsOpen && g.justPressed("previous") {
			weapon := g.cycleWeapon(-1)
			add(sim.SelectWeapon, int32(weapon))
			g.weaponPage = int(weapon) / render.TraySlots
		}
		if !g.controlsOpen && g.justPressed("next") {
			weapon := g.cycleWeapon(1)
			add(sim.SelectWeapon, int32(weapon))
			g.weaponPage = int(weapon) / render.TraySlots
		}
		if (!g.controlsOpen && g.justPressed("rope")) && g.s.Shots == 0 {
			g.weaponPage = int(sim.NinjaRope) / render.TraySlots
			add(sim.SelectWeapon, int32(sim.NinjaRope))
			add(sim.Fire, 1)
		}
		if !g.controlsOpen && g.justPressed("teleport") {
			g.weaponPage = int(sim.Teleport) / render.TraySlots
			add(sim.SelectWeapon, int32(sim.Teleport))
		}
		if g.controlsOpen || menuWasOpen || uiClick {
			g.charging = false
			g.power = 0
		} else if !sim.Charged(g.s.Weapon) {
			g.charging = false
			g.power = 0
			if g.justPressed("fire") || (g.s.Weapon == sim.MiningDrill && g.pressed("fire")) {
				add(sim.Fire, 1000)
			}
		} else if power := g.chargeShot(g.pressed("fire")); power > 0 {
			add(sim.Fire, int32(power))
		}
		if sim.UsesTarget(g.s.Weapon) && !uiClick && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && image.Pt(mx, my).In(g.camera.Viewport()) {
			x, y := g.camera.World(float64(mx), float64(my))
			if x >= 0 && x < float64(g.s.Config.Width) && y >= 0 && y < float64(g.s.Config.Height) {
				add(sim.SetTarget, int32(uint32(int(y))<<16|uint32(int(x))))
			}
		}
		if g.s.Weapon == sim.Teleport && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			worldX, worldY := g.camera.World(float64(x), float64(y))
			wx, wy := int(worldX), int(worldY)
			if !uiClick && !render.HUDContains(x, y, g.hideTop, g.hideBottom) && image.Pt(x, y).In(g.camera.Viewport()) && wx >= 6 && wx < int(g.s.Config.Width)-6 && wy >= 24 && wy < int(g.s.Config.Height) {
				add(sim.TeleportTo, int32(uint32(wy)<<16|uint32(wx)))
			}
		}
	} else {
		g.charging = false
		g.power = 0
	}
	if !g.controlsOpen && g.pressed("aimUp") && g.s.Weapon == sim.Jetpack && !ropeAttached {
		add(sim.Thrust, 1)
	} else if g.s.Thrust != 0 {
		add(sim.Thrust, 0)
	}
	turn := g.s.Turn
	if g.networkMatch != "" {
		g.turnInputs = append(g.turnInputs, in...)
	}
	effects, err := sim.Step(g.s, in)
	if err != nil {
		return err
	}
	if g.networkMatch != "" && (g.s.Turn != turn || g.s.Phase == sim.Ended) {
		if err := g.session.Submit(g.networkMatch, g.s, g.turnInputs); err != nil {
			g.message = err.Error()
		} else {
			g.submitted = true
			g.turnInputs = nil
		}
	}
	g.scene.Update(g.s, effects)
	g.camera.ObserveImpact(effects)
	g.speaker.play(g.scene.DrainSounds(), g.camera.X+float64(g.camera.Viewport().Dx())/g.camera.Zoom/2)
	if g.s.Turn != turn {
		g.weaponPage = 0
		g.weaponMenu = false
		g.charging = false
		g.power = 0
		g.follow = true
	}
	return nil
}
func (g *game) Draw(screen *ebiten.Image) {
	g.gpu.Target = screen
	mx, my := ebiten.CursorPosition()
	if g.startup {
		render.DrawCover(g.gpu, g.frame, g.startupReady)
		g.startupDrawn = true
	} else {
		g.scene.Draw(g.gpu, g.s, render.View{ImpactReview: g.camera.ReviewingImpact(), Network: g.networkMatch != "", Settlement: g.sessionView.Status, TableLobby: g.tableLobbyView(), Settings: g.settingsView(), BridgeStatus: g.bridgeStatus, BridgeConnected: g.bridgeConnected, BridgeBusy: g.bridgeBusy, BridgeLobby: g.bridgeLobbyView(), WeaponPage: g.weaponPage, WeaponMenu: g.weaponMenu, MenuOrigin: g.menuOrigin, KeyLabels: g.controlLabels(), ControlsOpen: g.controlsOpen, ControlRows: g.controlRows(), BindingIndex: g.bindingIndex, ControlsMessage: g.controlsMessage, Muted: g.speaker.muted, HideTop: g.hideTop, HideBottom: g.hideBottom, Camera: &g.camera, Arena: g.arena, Dev: devEnabled && g.networkMatch == "", MouseX: mx, MouseY: my, Paused: g.paused, Frame: g.frame, Power: g.power, Precision: g.pressed("precision"), Error: g.message})
	}
	if g.screenshot != "" && g.frame >= g.screenshotAfter && !g.shotDone && (!g.startup || g.startupReady) {
		img := image.NewRGBA(image.Rect(0, 0, render.Width, render.Height))
		screen.ReadPixels(img.Pix)
		f, err := os.Create(g.screenshot)
		if err != nil {
			g.message = err.Error()
			return
		}
		err = png.Encode(f, img)
		closeErr := f.Close()
		if err != nil {
			g.message = err.Error()
			return
		}
		if closeErr != nil {
			g.message = closeErr.Error()
			return
		}
		g.shotDone = true
	}
}
func (*game) Layout(int, int) (int, int) { return render.Width, render.Height }
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	cfg, err := appconfig.Load(os.Args[1:])
	if err != nil {
		var flagError *flags.Error
		if errors.As(err, &flagError) && flagError.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stdout, err)
			return nil
		}
		return err
	}
	if cfg.DebugLevel == "show" {
		fmt.Println("Supported subsystems:", logging.Subsystems())
		return nil
	}
	logs, err := logging.Open(cfg.LogDir, cfg.DebugLevel, cfg.LogSize, cfg.MaxLogFiles, os.Stdout)
	if err != nil {
		return err
	}
	defer logs.Close()
	log := logs.Logger("SWAR")
	log.Infof("StakeWars starting; application directory: %s", cfg.AppData)
	log.Infof("Config: %s; log: %s", cfg.ConfigFile, logs.Path)
	if cfg.Created {
		log.Infof("Created default configuration file")
	}
	session.UseLoggers(logs.Logger("SESS"), logs.Logger("SDK"))
	bridgeconn.UseLogger(logs.Logger("BRDG"))
	err = runDesktop(cfg)
	if err != nil {
		log.Errorf("Desktop stopped: %v", err)
	} else {
		log.Info("StakeWars stopped")
	}
	return err
}
func runDesktop(cfg *appconfig.Config) error {
	showCover, skipCover, tableDemo := &cfg.Cover, &cfg.SkipCover, &cfg.TableDemo
	settings, arena, shotAfter, shot := &cfg.Settings, &cfg.Arena, &cfg.ScreenshotAfter, &cfg.Screenshot
	mute, seed, dataDir := &cfg.Mute, &cfg.Seed, &cfg.AppData
	bridgePath, demoNetwork, controls := &cfg.BridgeConfig, &cfg.DemoNetwork, &cfg.Controls
	if (*arena || *demoNetwork) && !devEnabled {
		return fmt.Errorf("interactive fixtures require -tags desktop,dev")
	}
	releaseProfile, err := session.AcquireProfileInstance(*dataDir)
	if err != nil {
		return err
	}
	defer releaseProfile()
	g := &game{dataDir: *dataDir, screenshotAfter: *shotAfter, speaker: speaker{muted: *mute}, seed: *seed, arena: *arena, screenshot: *shot}
	if err = g.loadControls(*controls); err != nil {
		return err
	}
	g.startup = !*skipCover && (*showCover || (*settings == "" && !*tableDemo && *shot == ""))
	if g.startup {
		g.gpu = render.NewGPU(&render.Scene{})
	} else if err := g.reset(); err != nil {
		return err
	}
	g.initSettings(*bridgePath)
	// Saved, valid credentials are enough intent to connect. Requiring a trip
	// through Settings hid both a healthy bridge and startup failures.
	if err := g.bridgeConfig.Validate(); err == nil {
		g.connectBridge()
	} else {
		g.bridgeStatus = "Bridge offline · open Settings to configure the connection"
	}
	if *settings != "" {
		if *settings != "controls" && *settings != "bridge" {
			return fmt.Errorf("settings must be controls or bridge")
		}
		g.settingsOpen = true
		g.arena = false
		if *settings == "bridge" {
			g.settingsTab = 1
		}
	}
	if *tableDemo {
		if !devEnabled {
			return fmt.Errorf("table demo requires -tags desktop,dev")
		}
		t := tablelobby.Demo(4, 0)
		g.tableState = &t
		g.tableOpen = true
		g.tableSelected = -1
		g.arena = false
	}
	defer func() {
		g.stopBridge()
		if g.bridgeDone != nil {
			<-g.bridgeDone
		}
	}()
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowSize(1280, 800)
	title := "StakeWars"
	if cfg.CustomDir {
		title += " · " + filepath.Base(*dataDir)
	}
	if *demoNetwork {
		title += " · SIMULATED FUNDS"
	}
	ebiten.SetWindowTitle(title)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowClosingHandled(true)
	return ebiten.RunGame(g)
}
