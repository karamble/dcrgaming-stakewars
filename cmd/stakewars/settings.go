//go:build desktop

package main

import (
	"context"
	"fmt"
	"github.com/karamble/dcrstakewars/internal/session"
	"image"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"github.com/karamble/dcrstakewars/pkg/render"
)

type connectionUpdate struct {
	controller      *session.Controller
	generation      uint64
	status          string
	connected, busy bool
	notice          string
	invite          *bridgeconn.Invitation
	inviteSerial    uint64
	chainKnown      bool
	height          uint32
	gap             bool
}
type clipboardResult struct {
	field    int
	revision uint64
	text     string
	err      bool
}

func (g *game) initSettings(path string) {
	g.bridgePath = path
	g.settingsFocus = -1
	g.bridgeUpdates = make(chan connectionUpdate, 8)
	g.clipboardUpdates = make(chan clipboardResult, 1)
	var err error
	g.bridgeConfig, err = bridgeconn.Load(path)
	g.bridgeStatus = "Not connected. Paste the credentials supplied by dcrpulse."
	if err != nil {
		g.bridgeStatus = err.Error()
	}
}
func (g *game) stopBridge() {
	g.bridgeGeneration++
	if g.bridgeCancel != nil {
		g.bridgeCancel()
		g.bridgeCancel = nil
	}
	g.session = nil
	g.bridgeConnected = false
	g.bridgeBusy = false
	g.bridgeNotice = ""
	g.bridgeInvite = nil
	g.tableSerial = 0
	if g.tableState != nil && !g.tableState.Demo {
		g.tableState.Connected = false
		g.tableState.ChainKnown = false
		g.tableState.Stale = true
	}
}
func (g *game) connectBridge() {
	previous := g.bridgeDone
	g.stopBridge()
	cfg := g.bridgeConfig
	if err := cfg.Validate(); err != nil {
		g.bridgeStatus = err.Error()
		return
	}
	generation := g.bridgeGeneration
	ctx, cancel := context.WithCancel(context.Background())
	g.bridgeCancel = cancel
	g.bridgeBusy = true
	g.bridgeStatus = "Connecting securely…"
	updates := g.bridgeUpdates
	dir := filepath.Join(g.dataDir, "session")
	done := make(chan struct{})
	g.bridgeDone = done
	go func() {
		defer close(done)
		if previous != nil {
			select {
			case <-previous:
			case <-ctx.Done():
				return
			}
		}
		c, err := session.Open(ctx, cfg, dir)
		if err == nil {
			select {
			case updates <- connectionUpdate{generation: generation, controller: c, status: "Connected · ready for invitations", connected: true}:
			case <-ctx.Done():
			}
			err = c.Run(ctx)
		}
		status := "Disconnected"
		if err != nil {
			status = err.Error()
		}
		select {
		case updates <- connectionUpdate{generation: generation, status: status}:
		case <-ctx.Done():
		}
	}()
}
func (g *game) pollSettings() {
	defer g.pollSession()
	for {
		select {
		case u := <-g.bridgeUpdates:
			if u.generation == g.bridgeGeneration {
				g.session = u.controller
				g.bridgeStatus = u.status
				g.bridgeConnected = u.connected
				g.bridgeBusy = u.busy
				g.bridgeNotice = u.notice
				g.bridgeInvite = u.invite
				g.receiveTableUpdate(u)
			}
		case p := <-g.clipboardUpdates:
			g.clipboardBusy = false
			if !g.settingsOpen || g.settingsTab != 1 || p.field != g.settingsFocus || p.revision != g.settingsRevision {
				continue
			}
			if p.err {
				g.bridgeStatus = "Could not read clipboard. Check that a clipboard tool is available."
				continue
			}
			g.editSetting(p.text, true)
		default:
			return
		}
	}
}
func (g *game) field() *string {
	switch g.settingsFocus {
	case 0:
		return &g.bridgeConfig.Host
	case 1:
		return &g.bridgeConfig.Port
	case 2:
		return &g.bridgeConfig.ClientCert
	case 3:
		return &g.bridgeConfig.ClientKey
	case 4:
		return &g.bridgeConfig.BridgeCert
	}
	return nil
}
func (g *game) editSetting(text string, replace bool) {
	f := g.field()
	if f == nil {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if g.settingsFocus < 2 {
		text = strings.TrimSpace(text)
	}
	value := *f + text
	if replace || g.settingsSelectAll {
		value = text
	}
	limit := bridgeconn.MaxPEM
	if g.settingsFocus == 0 {
		limit = 64
	}
	if g.settingsFocus == 1 {
		limit = 5
	}
	if len(value) > limit {
		g.bridgeStatus = "That field is too long."
		return
	}
	*f = value
	g.settingsSelectAll = false
	g.settingsRevision++
	g.stopBridge()
	g.bridgeStatus = "Settings changed. Connect to verify; Save to remember them."
}
func (g *game) pasteSetting() {
	if g.field() == nil || g.clipboardBusy {
		return
	}
	g.clipboardBusy = true
	field, revision := g.settingsFocus, g.settingsRevision
	results := g.clipboardUpdates
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var name string
		var args []string
		switch runtime.GOOS {
		case "darwin":
			name = "pbpaste"
		case "windows":
			name = "powershell.exe"
			args = []string{"-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw"}
		default:
			if _, e := exec.LookPath("wl-paste"); e == nil {
				name = "wl-paste"
				args = []string{"--no-newline"}
			} else if _, e := exec.LookPath("xclip"); e == nil {
				name = "xclip"
				args = []string{"-selection", "clipboard", "-o"}
			} else {
				name = "xsel"
				args = []string{"--clipboard", "--output"}
			}
		}
		cmd := exec.CommandContext(ctx, name, args...)
		pipe, err := cmd.StdoutPipe()
		var data []byte
		if err == nil {
			err = cmd.Start()
		}
		if err == nil {
			data, err = io.ReadAll(io.LimitReader(pipe, bridgeconn.MaxPEM+1))
			pipe.Close()
			waitErr := cmd.Wait()
			if err == nil {
				err = waitErr
			}
		}
		results <- clipboardResult{field: field, revision: revision, text: string(data), err: err != nil}
	}()
}

func (g *game) selectBridgeNetwork(network string) {
	if network != "mainnet" && network != "testnet3" && network != "simnet" {
		return
	}
	if g.bridgeConfig.Network == network {
		return
	}
	g.bridgeConfig.Network = network
	g.settingsFocus = -1
	g.settingsRevision++
	g.stopBridge()
	g.bridgeStatus = "Network changed. Connect to verify; Save to remember it."
}

func (g *game) updateSettings() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.bindingIndex >= 0 {
			g.bindingIndex = -1
			return
		}
		g.settingsOpen = false
		g.settingsFocus = -1
		return
	}
	x, y := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		p := image.Pt(x, y)
		if p.In(render.SettingsCloseRect) {
			g.settingsOpen = false
			g.bindingIndex = -1
			g.settingsFocus = -1
			return
		}
		for i, b := range []image.Rectangle{render.SettingsControlsTab, render.SettingsBridgeTab} {
			if p.In(b) {
				g.settingsTab = i
				g.bindingIndex = -1
				g.settingsFocus = -1
				g.settingsRevision++
				return
			}
		}
	}
	if g.settingsTab == 0 {
		g.updateControls()
		return
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		p := image.Pt(x, y)
		for i, network := range []string{"mainnet", "testnet3", "simnet"} {
			if p.In(render.SettingsNetworkRect(i)) {
				g.selectBridgeNetwork(network)
				return
			}
		}
		for i := 0; i < 5; i++ {
			if p.In(render.SettingsFieldRect(i)) {
				g.settingsFocus = i
				g.settingsSelectAll = true
				g.settingsRevision++
			}
		}
		switch {
		case p.In(render.SettingsConnectRect):
			if !g.bridgeBusy {
				g.connectBridge()
			}
			return
		case p.In(render.SettingsDisconnectRect):
			g.stopBridge()
			g.bridgeStatus = "Disconnected."
			return
		case p.In(render.SettingsSaveRect):
			if err := bridgeconn.Save(g.bridgePath, g.bridgeConfig); err != nil {
				g.bridgeStatus = err.Error()
			} else {
				g.bridgeStatus = "Settings saved locally with owner-only permissions."
			}
			return
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.settingsFocus = (g.settingsFocus + 1) % 5
		g.settingsSelectAll = true
		g.settingsRevision++
	}
	if g.field() == nil {
		return
	}
	modifier := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) || ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight)
	if modifier {
		if inpututil.IsKeyJustPressed(ebiten.KeyA) {
			g.settingsSelectAll = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyV) {
			g.pasteSetting()
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) || (inpututil.KeyPressDuration(ebiten.KeyBackspace) > 25 && inpututil.KeyPressDuration(ebiten.KeyBackspace)%3 == 0) {
		value := []rune(*g.field())
		if g.settingsSelectAll {
			g.editSetting("", true)
		} else if len(value) > 0 {
			g.editSetting(string(value[:len(value)-1]), true)
		}
	}
	if chars := ebiten.AppendInputChars(nil); len(chars) > 0 {
		g.editSetting(string(chars), false)
	}
}
func (g *game) settingsView() render.SettingsView {
	summary := func(value string, secret bool) string {
		if value == "" {
			return "Click here and paste the complete PEM text"
		}
		if secret {
			return "••••••••  Private key entered"
		}
		return fmt.Sprintf("PEM entered · %d lines", strings.Count(strings.TrimSpace(value), "\n")+1)
	}
	return render.SettingsView{Open: g.settingsOpen, Tab: g.settingsTab, Network: g.bridgeConfig.Network, Focus: g.settingsFocus, Status: g.bridgeStatus, Connected: g.bridgeConnected, Busy: g.bridgeBusy, Fields: [][2]string{{"Bridge IP", g.bridgeConfig.Host}, {"Port", g.bridgeConfig.Port}, {"Client certificate", summary(g.bridgeConfig.ClientCert, false)}, {"Client private key", summary(g.bridgeConfig.ClientKey, true)}, {"Bridge certificate", summary(g.bridgeConfig.BridgeCert, false)}}}
}

func (g *game) bridgeLobbyView() render.BridgeLobbyView {
	v := render.BridgeLobbyView{Visible: g.bridgeConnected || g.bridgeNotice != "", Notice: g.bridgeNotice}
	if inv := g.bridgeInvite; inv != nil {
		v.Terms = []string{
			"SESSION " + inv.SID,
			fmt.Sprintf("%d seats · %d.%08d DCR per seat", inv.Seats, inv.BuyInAtoms/100000000, inv.BuyInAtoms%100000000),
			fmt.Sprintf("Refund delay: %d blocks", inv.CSVBlocks),
			fmt.Sprintf("Admission deadline: block %d", inv.Until),
		}
	}
	return v
}
