package render

import (
	"image"
	"image/color"
	"math"
)

var LobbySettingsRect = image.Rect(1338, 28, 1394, 84)
var SettingsCloseRect = image.Rect(1138, 108, 1186, 148)
var SettingsControlsTab = image.Rect(250, 166, 690, 206)
var SettingsBridgeTab = image.Rect(706, 166, 1166, 206)

// SettingsView contains presentation values only. Secrets must be masked by the
// caller before they enter this view or a screenshot/preview renderer.
type SettingsView struct {
	Open            bool
	Tab             int
	Network         string
	Fields          [][2]string
	Focus           int
	Status          string
	Connected, Busy bool
}

func SettingsControlRect(index int) image.Rectangle {
	x, y := 250+(index/7)*470, 246+(index%7)*48
	return image.Rect(x, y, x+446, y+42)
}
func SettingsFieldRect(index int) image.Rectangle {
	y := 300 + index*58
	return image.Rect(440, y, 1156, y+42)
}

func SettingsNetworkRect(index int) image.Rectangle {
	x := 440 + index*238
	return image.Rect(x, 242, x+222, 282)
}

var SettingsConnectRect = image.Rect(250, 680, 500, 728)
var SettingsDisconnectRect = image.Rect(520, 680, 770, 728)
var SettingsSaveRect = image.Rect(790, 680, 1040, 728)

func settingsCog(c Canvas) {
	x, y := 1366.0, 56.0
	c.Rect(1338, 28, 56, 56, Panel)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		c.Line(x+13*math.Cos(a), y+13*math.Sin(a), x+21*math.Cos(a), y+21*math.Sin(a), 5, Mint)
	}
	c.Circle(x, y, 16, Mint)
	c.Circle(x, y, 10, Panel)
	c.Circle(x, y, 4, Mint)
}
func drawSettings(c Canvas, v View) {
	c.Rect(0, 0, Width, Height, color.RGBA{4, 10, 23, 225})
	c.Rect(225, 96, 990, 716, Panel)
	c.Text("SETTINGS", 250, 115, 28, Mint)
	c.Text("X", 1152, 116, 23, White)
	for i, b := range []image.Rectangle{SettingsControlsTab, SettingsBridgeTab} {
		col := Navy
		if v.Settings.Tab == i {
			col = color.RGBA{22, 71, 77, 255}
		}
		c.Rect(float64(b.Min.X), float64(b.Min.Y), float64(b.Dx()), float64(b.Dy()), col)
		c.Text([]string{"KEYBOARD & MOUSE", "BRIDGE CONNECTION"}[i], float64(b.Min.X+14), float64(b.Min.Y+10), 16, White)
	}
	if v.Settings.Tab == 0 {
		c.Text("Click an action to rebind it. Escape cancels a binding or closes settings.", 250, 220, 13, Muted)
		for i, row := range v.ControlRows {
			b := SettingsControlRect(i)
			col := Navy
			if i == v.BindingIndex {
				col = color.RGBA{22, 71, 77, 255}
			}
			c.Rect(float64(b.Min.X), float64(b.Min.Y), float64(b.Dx()), float64(b.Dy()), col)
			c.Text(row[0], float64(b.Min.X+10), float64(b.Min.Y+5), 13, White)
			key := row[1]
			if i == v.BindingIndex {
				key = "PRESS KEY"
			}
			c.Text(key, float64(b.Min.X+10), float64(b.Min.Y+23), 12, Mint)
		}
		notes := []string{"W / S aim; rope: reel in/out. Jetpack: Hold W to lift. SPACE fires / uses tools.", "SHIFT: precise aim / turn in place. SHIFT+J: backflip. 1–5: grenade fuse.", "Right-click: weapons. Mouse edges / middle-drag: pan. Wheel: zoom. C: follow.", "Left/right arrows and Page Up/Down: pan. H: hide bars. F1/F2: each bar. M: sound."}
		// Bound action labels in the rows above remain the authority after rebinding.
		for i, n := range notes {
			c.Text(controlText(n, v), 250, 604+float64(i)*25, 13, White)
		}
		c.Text(v.ControlsMessage, 250, 752, 13, Muted)
		return
	}
	c.Text("Choose the bridge network, then paste the three PEM credentials from dcrpulse.", 250, 220, 13, Muted)
	c.Text("Network", 250, 253, 15, White)
	for i, network := range []string{"mainnet", "testnet3", "simnet"} {
		b := SettingsNetworkRect(i)
		col := Navy
		if v.Settings.Network == network {
			col = color.RGBA{22, 71, 77, 255}
		}
		c.Rect(float64(b.Min.X), float64(b.Min.Y), float64(b.Dx()), float64(b.Dy()), col)
		c.Text(network, float64(b.Min.X+14), float64(b.Min.Y+10), 15, Mint)
	}
	for i, field := range v.Settings.Fields {
		b := SettingsFieldRect(i)
		c.Text(field[0], 250, float64(b.Min.Y+10), 15, White)
		col := Navy
		if i == v.Settings.Focus {
			col = color.RGBA{22, 71, 77, 255}
		}
		c.Rect(float64(b.Min.X), float64(b.Min.Y), float64(b.Dx()), float64(b.Dy()), col)
		value := []rune(field[1])
		if len(value) > 78 {
			value = append([]rune("…"), value[len(value)-77:]...)
		}
		c.Text(string(value), float64(b.Min.X+12), float64(b.Min.Y+11), 15, Mint)
	}
	c.Text("Register this game as stakewars in dcrpulse. Use its gaming port (default 8443).", 250, 610, 13, Muted)
	c.Text("Tab: next field · Ctrl+V: paste · Ctrl+A: replace field", 250, 650, 13, Muted)
	for i, b := range []image.Rectangle{SettingsConnectRect, SettingsDisconnectRect, SettingsSaveRect} {
		c.Rect(float64(b.Min.X), float64(b.Min.Y), float64(b.Dx()), float64(b.Dy()), Navy)
		label := []string{"CONNECT", "DISCONNECT", "SAVE SETTINGS"}[i]
		if i == 0 && v.Settings.Busy {
			label = "CONNECTING…"
		}
		c.Text(label, float64(b.Min.X+16), float64(b.Min.Y+13), 16, Mint)
	}
	status := []rune(v.Settings.Status)
	if len(status) > 110 {
		c.Text(string(status[:110]), 250, 752, 13, White)
		status = status[110:]
		c.Text(string(status), 250, 774, 13, White)
	} else {
		c.Text(string(status), 250, 752, 13, White)
	}
}
