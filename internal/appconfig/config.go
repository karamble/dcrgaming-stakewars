// Package appconfig follows Decred's defaults, INI file, CLI precedence.
package appconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/dcrutil/v4"
	flags "github.com/jessevdk/go-flags"
)

const AppName = "dcrstakewars"
const ConfigName = AppName + ".conf"

type Options struct {
	AppData         string `short:"A" long:"appdata" ini-name:"appdata" description:"Application directory for configuration, identity, saved matches and logs"`
	DataDir         string `long:"datadir" ini-name:"datadir" description:"Alias for --appdata; use a different directory for each identity"`
	ConfigFile      string `short:"C" long:"configfile" ini-name:"configfile" description:"Configuration file (default: APPDATA/dcrstakewars.conf)"`
	LogDir          string `long:"logdir" ini-name:"logdir" description:"Log directory (default: APPDATA/logs)"`
	DebugLevel      string `short:"d" long:"debuglevel" ini-name:"debuglevel" description:"trace, debug, info, warn, error, critical; or SWAR=debug,SESS=info,SDK=warn,BRDG=info; show lists subsystems"`
	MaxLogFiles     int    `long:"maxlogfiles" ini-name:"maxlogfiles" description:"Maximum number of rotated logs retained"`
	LogSize         int64  `long:"logsize" ini-name:"logsize" description:"Log rotation size in kilobytes"`
	BridgeConfig    string `long:"bridge-config" ini-name:"bridge-config" description:"Bridge credential file (default: APPDATA/bridge.json)"`
	Controls        string `long:"controls" ini-name:"controls" description:"Key bindings file (default: APPDATA/controls.json)"`
	Connect         bool   `long:"connect" ini-name:"connect" description:"Connect to the saved bridge at startup"`
	Settings        string `long:"settings" ini-name:"settings" description:"Open settings: controls or bridge"`
	Mute            bool   `long:"mute" ini-name:"mute" description:"Disable sound effects"`
	Cover           bool   `long:"cover" ini-name:"cover" description:"Show the opening cover even for a screenshot"`
	SkipCover       bool   `long:"skip-cover" ini-name:"skip-cover" description:"Skip the opening cover"`
	TableDemo       bool   `long:"table-demo" ini-name:"table-demo" description:"Fictional table preparation (dev build only)"`
	Arena           bool   `long:"dev-arena" ini-name:"dev-arena" description:"Open engineering gameplay fixture (dev build only)"`
	DemoNetwork     bool   `long:"dev-network" ini-name:"dev-network" description:"Label a mock bridge session as simulated funds (dev build only)"`
	Seed            uint64 `long:"dev-seed" ini-name:"dev-seed" description:"Engineering fixture seed; zero chooses a random seed"`
	Screenshot      string `long:"screenshot" ini-name:"screenshot" description:"Write desktop screenshot and exit"`
	ScreenshotAfter uint64 `long:"screenshot-after" ini-name:"screenshot-after" description:"Frames before screenshot capture"`
}
type Config struct {
	Options
	Created   bool
	CustomDir bool
}

func defaults() Options {
	return Options{DebugLevel: "info", MaxLogFiles: 10, LogSize: 1024, ScreenshotAfter: 3}
}
func DefaultDir() string { return dcrutil.AppDataDir(AppName, false) }
func expand(p string) (string, error) {
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		p = filepath.Join(home, strings.TrimLeft(strings.TrimPrefix(p, "~"), "/\\"))
	}
	if p == "" {
		return "", errors.New("empty path")
	}
	return filepath.Abs(filepath.Clean(p))
}
func parser(o *Options) *flags.Parser { return flags.NewParser(o, flags.HelpFlag|flags.PassDoubleDash) }

// Accept existing Go-style -long flags as well as Decred-style --long flags.
func arguments(args []string, p *flags.Parser) []string {
	out := append([]string(nil), args...)
	for i, a := range out {
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") {
			name := strings.SplitN(a[1:], "=", 2)[0]
			if len(name) > 1 && p.FindOptionByLongName(name) != nil {
				out[i] = "-" + a
			}
		}
	}
	return out
}
func home(o Options) (string, error) {
	if o.AppData != "" && o.DataDir != "" {
		a, e := expand(o.AppData)
		if e != nil {
			return "", e
		}
		b, e := expand(o.DataDir)
		if e != nil {
			return "", e
		}
		if a != b {
			return "", errors.New("--appdata and --datadir name different directories")
		}
		return a, nil
	}
	if o.DataDir != "" {
		return expand(o.DataDir)
	}
	if o.AppData != "" {
		return expand(o.AppData)
	}
	return expand(DefaultDir())
}
func Load(args []string) (*Config, error) {
	pre := defaults()
	p := parser(&pre)
	args = arguments(args, p)
	if rest, e := p.ParseArgs(args); e != nil {
		return nil, e
	} else if len(rest) != 0 {
		return nil, fmt.Errorf("unexpected positional arguments")
	}
	for _, name := range []string{"appdata", "datadir"} {
		o := p.FindOptionByLongName(name)
		if o.IsSet() && ((name == "appdata" && pre.AppData == "") || (name == "datadir" && pre.DataDir == "")) {
			return nil, fmt.Errorf("%s must not be empty", name)
		}
	}
	root, e := home(pre)
	if e != nil {
		return nil, e
	}
	path := pre.ConfigFile
	if path == "" {
		path = filepath.Join(root, ConfigName)
	}
	path, e = expand(path)
	if e != nil {
		return nil, e
	}
	// Help and subsystem listing do not create files or change any profile.
	if pre.DebugLevel == "show" {
		return &Config{Options: pre}, nil
	}
	created, e := create(path)
	if e != nil {
		return nil, e
	}
	o := defaults()
	p = parser(&o)
	if e = flags.NewIniParser(p).ParseFile(path); e != nil {
		return nil, fmt.Errorf("parse configuration: %w", e)
	}
	// CLI aliases both override an application directory named inside the INI.
	if pre.AppData != "" || pre.DataDir != "" {
		o.AppData = ""
		o.DataDir = ""
	}
	if _, e = p.ParseArgs(args); e != nil {
		return nil, e
	}
	root, e = home(o)
	if e != nil {
		return nil, e
	}
	o.AppData = root
	o.DataDir = root
	o.ConfigFile = path
	for _, v := range []struct {
		p    *string
		name string
	}{{&o.LogDir, "logs"}, {&o.BridgeConfig, "bridge.json"}, {&o.Controls, "controls.json"}} {
		if *v.p == "" {
			*v.p = filepath.Join(root, v.name)
		} else {
			*v.p, e = expand(*v.p)
			if e != nil {
				return nil, e
			}
		}
	}
	if o.MaxLogFiles < 1 || o.MaxLogFiles > 1000 {
		return nil, errors.New("maxlogfiles must be between 1 and 1000")
	}
	if o.LogSize < 1 || o.LogSize > 1048576 {
		return nil, errors.New("logsize must be between 1 and 1048576 KB")
	}
	if o.Settings != "" && o.Settings != "bridge" && o.Settings != "controls" {
		return nil, errors.New("settings must be controls or bridge")
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	return &Config{Options: o, Created: created, CustomDir: pre.AppData != "" || pre.DataDir != ""}, nil
}
func create(path string) (bool, error) {
	if _, e := os.Stat(path); e == nil {
		return false, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return false, e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return false, e
	}
	o := defaults()
	var b bytes.Buffer
	b.WriteString("; StakeWars configuration. Command-line options override this file.\n; Bridge PEM credentials are managed by Settings in bridge.json.\n; Each --appdata / --datadir directory is a separate game identity.\n\n")
	flags.NewIniParser(parser(&o)).Write(&b, flags.IniIncludeComments|flags.IniIncludeDefaults|flags.IniCommentDefaults)
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(e, os.ErrExist) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	defer f.Close()
	if _, e = f.Write(b.Bytes()); e != nil {
		return false, e
	}
	if e = f.Sync(); e != nil {
		return false, e
	}
	return true, f.Close()
}
