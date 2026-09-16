package render

import "strings"

// BridgeLobbyView carries display text only; invitation data is never proof of a seat.
type BridgeLobbyView struct {
	Visible bool
	Notice  string
	Terms   []string
}

func drawBridgeLobby(c Canvas, v BridgeLobbyView) {
	if !v.Visible {
		return
	}
	c.Rect(825, 145, 535, 285, Panel)
	c.Rect(825, 145, 3, 285, Mint)
	c.Text("BRIDGE / INVITATION PREVIEW", 845, 164, 17, Mint)
	c.Text("NOT SEATED · PAYMENTS DISABLED", 845, 195, 13, White)
	if len(v.Terms) == 0 {
		c.Text("Waiting for a StakeWars table invitation.", 845, 237, 14, Muted)
		c.Text("Invitations do not reserve or fund a seat.", 845, 266, 14, Muted)
	} else {
		for i, line := range v.Terms {
			c.Text(line, 845, 232+float64(i)*26, 14, White)
		}
	}
	// Notices come only from fixed application messages, never peer text.
	words := strings.Fields(v.Notice)
	line, row := "", 0
	for _, word := range words {
		if len(line)+len(word)+1 > 65 {
			c.Text(line, 845, 363+float64(row)*20, 12, Muted)
			line = ""
			row++
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		c.Text(line, 845, 363+float64(row)*20, 12, Muted)
	}
}
