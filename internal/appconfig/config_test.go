package appconfig

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrutil/v4"
	flags "github.com/jessevdk/go-flags"
)

func TestProfileCreationAndCLIOverrides(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	c, e := Load([]string{"-datadir", one})
	if e != nil {
		t.Fatal(e)
	}
	if !c.Created || c.ConfigFile != filepath.Join(one, ConfigName) || c.Controls != filepath.Join(one, "controls.json") || c.BridgeConfig != filepath.Join(one, "bridge.json") || c.LogDir != filepath.Join(one, "logs") {
		t.Fatalf("paths: %+v", c)
	}
	raw, e := os.ReadFile(c.ConfigFile)
	if e != nil || !strings.Contains(strings.ToLower(string(raw)), "maxlogfiles") {
		t.Fatalf("template: %v", e)
	}
	st, e := os.Stat(c.ConfigFile)
	if e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0600 {
		t.Fatalf("config mode: %v", st.Mode())
	}
	ini := "; preserve this comment\n[Application Options]\ndebuglevel=warn\nmaxlogfiles=4\nconnect=true\nappdata=" + two + "\n"
	if e = os.WriteFile(c.ConfigFile, []byte(ini), 0600); e != nil {
		t.Fatal(e)
	}
	c, e = Load([]string{"--datadir", one, "--debuglevel", "SDK=debug,SWAR=info"})
	if e != nil {
		t.Fatal(e)
	}
	if c.Created || c.AppData != one || c.DebugLevel != "SDK=debug,SWAR=info" || c.MaxLogFiles != 4 || !c.Connect {
		t.Fatalf("precedence: %+v", c)
	}
	kept, _ := os.ReadFile(c.ConfigFile)
	if string(kept) != ini {
		t.Fatal("overwrote existing configuration")
	}
	d, e := Load([]string{"--appdata", two})
	if e != nil {
		t.Fatal(e)
	}
	if d.BridgeConfig == c.BridgeConfig || d.LogDir == c.LogDir || d.Controls == c.Controls || d.AppData == c.AppData {
		t.Fatal("profiles share state")
	}
}
func TestAlternateConfigAndErrors(t *testing.T) {
	root := t.TempDir()
	conf := filepath.Join(root, "custom.conf")
	c, e := Load([]string{"-A", root, "-C", conf, "-d", "debug"})
	if e != nil || c.ConfigFile != conf {
		t.Fatalf("custom config: %v", e)
	}
	for _, args := range [][]string{{"--appdata", root, "--datadir", filepath.Join(root, "other")}, {"--datadir", root, "--logsize", "0"}, {"--datadir", root, "--maxlogfiles", "0"}, {"--datadir", root, "--settings", "unknown"}, {"--datadir", root, "unexpected"}} {
		if _, e := Load(args); e == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	before, _ := os.ReadDir(root)
	_, e = Load([]string{"--datadir", filepath.Join(root, "help-only"), "--help"})
	var fe *flags.Error
	if !errors.As(e, &fe) || fe.Type != flags.ErrHelp {
		t.Fatalf("help: %v", e)
	}
	if _, e = Load([]string{"--datadir", filepath.Join(root, "show-only"), "--debuglevel", "show"}); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadDir(root)
	if len(before) != len(after) {
		t.Fatal("help created a profile")
	}
	if DefaultDir() != dcrutil.AppDataDir("dcrstakewars", false) {
		t.Fatal("not the Decred OS directory convention")
	}
}

func TestExpandedPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STAKEWARS_TEST_PROFILE", root)
	c, e := Load([]string{"--appdata", "$STAKEWARS_TEST_PROFILE"})
	if e != nil || c.AppData != root {
		t.Fatalf("environment path: %+v %v", c, e)
	}
	if _, e = Load([]string{"--datadir="}); e == nil {
		t.Fatal("accepted empty profile directory")
	}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Skip(e)
	}
	got, e := expand("~/stakewars-path-test")
	if e != nil || got != filepath.Join(home, "stakewars-path-test") {
		t.Fatalf("home expansion: %s %v", got, e)
	}
}
