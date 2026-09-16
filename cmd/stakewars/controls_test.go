//go:build desktop

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

func TestControlsLoadDefaultsAndCustomKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controls.json")
	g := &game{}
	if err := g.loadControls(path); err != nil {
		t.Fatal(err)
	}
	if g.keys["fire"] != ebiten.KeySpace || g.keys["jump"] != ebiten.KeyJ {
		t.Fatal("default bindings changed")
	}
	if err := os.WriteFile(path, []byte(`{"jump":"Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := g.loadControls(path); err != nil {
		t.Fatal(err)
	}
	if g.keys["jump"] != ebiten.KeyZ || g.controlLabels()["jump"] != "Z jump" {
		t.Fatal("custom binding not applied to HUD")
	}
	for _, bad := range []string{`{"jump":"Space"}`, `{"jump":"K"}`, `{"unknown":"Z"}`, `{"jump":"notakey"}`} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if err := g.loadControls(path); err == nil {
			t.Fatal("invalid control settings accepted", bad)
		}
	}
}
func TestWeaponCycleSkipsEmptyInventory(t *testing.T) {
	s, err := sim.New(sim.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	g := &game{s: s}
	s.Weapon = sim.RemoteCharge
	s.Ammo[0][sim.HeavyCluster] = 0
	if g.cycleWeapon(1) != sim.BisonBomb {
		t.Fatal("cycle stuck on empty weapon")
	}
	s.Shots = 1
	if g.cycleWeapon(1) != sim.RemoteCharge {
		t.Fatal("cycle left active remote charge")
	}
}

func TestLegacyRopeControlsUseReboundAimKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controls.json")
	if err := os.WriteFile(path, []byte(`{"aimUp":"I","aimDown":"O","lower":"ArrowDown","rise":"ArrowUp"}`), 0600); err != nil {
		t.Fatal(err)
	}
	g := &game{}
	if err := g.loadControls(path); err != nil {
		t.Fatal(err)
	}
	if g.keys["aimUp"] != ebiten.KeyI || g.keys["aimDown"] != ebiten.KeyO || g.controlLabels()["ropeLength"] != "I / O" {
		t.Fatal("rope controls must follow rebound aim keys")
	}
	if _, exists := g.keys["lower"]; exists {
		t.Fatal("obsolete rope binding is still active")
	}
	if _, exists := g.keys["rise"]; exists {
		t.Fatal("obsolete jetpack binding is still active")
	}
	if g.controlLabels()["liftHint"] != "Hold I" {
		t.Fatal("jetpack hint does not follow rebound aim-up")
	}
}

func TestBridgeSettingsNeverExposePrivateKey(t *testing.T) {
	g := &game{}
	g.initSettings(filepath.Join(t.TempDir(), "bridge.json"))
	g.bridgeConfig.ClientKey = "SUPER-SECRET-PRIVATE-KEY"
	g.settingsOpen = true
	g.settingsTab = 1
	view := g.settingsView()
	for _, field := range view.Fields {
		if strings.Contains(field[1], g.bridgeConfig.ClientKey) {
			t.Fatal("private key exposed to renderer")
		}
	}
	g.settingsFocus = 0
	g.settingsSelectAll = true
	g.editSetting("192.168.1.2", false)
	if g.bridgeConfig.Host != "192.168.1.2" {
		t.Fatal("select-all typing did not replace IP")
	}
	g.settingsFocus = 2
	g.editSetting("a\r\nb\r\n", true)
	if g.bridgeConfig.ClientCert != "a\nb\n" {
		t.Fatal("PEM line endings changed")
	}
	g.bridgeGeneration = 2
	g.bridgeUpdates <- connectionUpdate{generation: 1, connected: true, status: "stale"}
	g.pollSettings()
	if g.bridgeConnected || g.bridgeStatus == "stale" {
		t.Fatal("stale connection result applied")
	}
}

func TestBridgeNetworkSelectionDisconnectsAndPersistsInView(t *testing.T) {
	g := &game{}
	g.initSettings(filepath.Join(t.TempDir(), "bridge.json"))
	g.bridgeConnected = true
	g.settingsFocus = 1
	g.selectBridgeNetwork("simnet")
	if g.bridgeConfig.Network != "simnet" || g.bridgeConnected || g.settingsFocus != -1 {
		t.Fatal("network change did not invalidate the active bridge connection")
	}
	if view := g.settingsView(); view.Network != "simnet" {
		t.Fatal("selected network is not visible in settings")
	}
	g.selectBridgeNetwork("unknown")
	if g.bridgeConfig.Network != "simnet" {
		t.Fatal("unknown network changed settings")
	}
}
