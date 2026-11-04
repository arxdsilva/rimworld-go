package config

import (
	"regexp"
	"strings"
)

// HUD is the colonist card's look and layout (RHUD, the dossier): player-side
// settings, never in a save. Colours store only differences from the theme, as
// Controls.Bindings does.
type HUD struct {
	Enabled       bool              `json:"enabled"`
	Docked        bool              `json:"docked"`
	RefreshTenths int               `json:"refresh_tenths"` // 1..30
	Pane          HUDPane           `json:"pane"`
	Card          HUDCard           `json:"card"`
	Text          HUDText           `json:"text"`
	Colors        map[string]string `json:"colors,omitempty"` // key -> "#RRGGBBAA"
	Layouts       HUDLayouts        `json:"layouts,omitempty"`
	Preset        string            `json:"preset,omitempty"` // "" once edited
}

// HUDPane is the docked card's pane.
type HUDPane struct {
	Restyle     bool `json:"restyle"`
	Height      int  `json:"height"`   // 120..192, on a 720-tall screen (dossier/sheet.DockedFor)
	MinTabs     int  `json:"min_tabs"` // 6..12
	TabW        int  `json:"tab_w"`    // 72..150
	FactionIcon bool `json:"faction_icon"`
	LogBody     bool `json:"log_body"` // floating only
}

// HUDCard is the floating card and the letter stack beside it.
type HUDCard struct {
	Anchor          int  `json:"anchor"` // 0..8, row by row from the top left
	OffsetX         int  `json:"offset_x"`
	OffsetY         int  `json:"offset_y"`
	W               int  `json:"w"` // 100..1000
	H               int  `json:"h"` // 100..1000
	CompressLetters bool `json:"compress_letters"`
	LetterGap       int  `json:"letter_gap"` // 0..12
}

// HUDText is the card's text sizes and columns.
type HUDText struct {
	BodySize     int  `json:"body_size"`      // 7..20
	BodyLinePct  int  `json:"body_line_pct"`  // 100..250
	LargeDelta   int  `json:"large_delta"`    // 0..24
	LargeLinePct int  `json:"large_line_pct"` // 100..250
	SmallDelta   int  `json:"small_delta"`    // -5..0
	SmallLinePct int  `json:"small_line_pct"` // 100..250
	LabelW       int  `json:"label_w"`        // 30..150
	ValueW       int  `json:"value_w"`        // 30..150
	Decimals     bool `json:"decimals"`
}

// HUDLayouts are the player's edited trees; nil is the built-in default.
type HUDLayouts struct {
	Docked   *HUDNode `json:"docked,omitempty"`
	Floating *HUDNode `json:"floating,omitempty"`
	// Dropped says Normalize threw a stored tree away for being too big, so the
	// card can say so. Never written.
	Dropped bool `json:"-"`
}

// HUDNode is one node of a card layout: a container (column strip block line)
// or a widget key.
type HUDNode struct {
	Kind     string    `json:"k"`
	Kids     []HUDNode `json:"c,omitempty"`
	Fill     bool      `json:"fill,omitempty"`
	Who      string    `json:"who,omitempty"` // letters c s p w; "" all
	Def      string    `json:"def,omitempty"`
	BarStyle string    `json:"bar,omitempty"` // fade_up low_only fade_down main_only
}

// Limits of a stored layout: past them the tree is dropped for the built-in.
const (
	HUDMaxNodes = 256
	HUDMaxDepth = 8
)

func defaultHUD() HUD {
	return HUD{
		Enabled: true,
		Docked:  true, RefreshTenths: 5,
		Pane: HUDPane{Restyle: true, Height: 168, MinTabs: 6, TabW: 96, FactionIcon: true},
		Card: HUDCard{Anchor: 6, W: 400, H: 460, LetterGap: 4},
		Text: HUDText{BodySize: 12, BodyLinePct: 89, LargeDelta: 4, LargeLinePct: 100, SmallDelta: -2, SmallLinePct: 100,
			LabelW: 100, ValueW: 36},
	}
}

var hudColor = regexp.MustCompile(`^#[0-9A-Fa-f]{8}$`)

func clampInt(v, lo, hi int) int { return max(lo, min(hi, v)) }

// normalize clamps every range, drops malformed colours and oversized trees.
func (h HUD) normalize() HUD {
	h.RefreshTenths = clampInt(h.RefreshTenths, 1, 30)
	h.Pane.Height = clampInt(h.Pane.Height, HUDPaneMinH, HUDPaneMaxH)
	h.Pane.MinTabs = clampInt(h.Pane.MinTabs, 6, 12)
	h.Pane.TabW = clampInt(h.Pane.TabW, 72, 150)
	h.Card.Anchor = clampInt(h.Card.Anchor, 0, 8)
	h.Card.W = clampInt(h.Card.W, 100, 1000)
	h.Card.H = clampInt(h.Card.H, 100, 1000)
	h.Card.LetterGap = clampInt(h.Card.LetterGap, 0, 12)
	t := &h.Text
	t.BodySize = clampInt(t.BodySize, 7, 20)
	t.LargeDelta = clampInt(t.LargeDelta, 0, 24)
	t.SmallDelta = clampInt(t.SmallDelta, -5, 0)
	for _, p := range []*int{&t.BodyLinePct, &t.LargeLinePct, &t.SmallLinePct} {
		*p = clampInt(*p, HUDLineMinPct, 250)
	}
	t.LabelW = clampInt(t.LabelW, 30, 150)
	t.ValueW = clampInt(t.ValueW, 30, 150)
	if !h.Pane.Restyle {
		h.Docked = false // the plain pane sits beside a floating card
	}
	if h.Docked {
		h.Pane.LogBody = false // the log body is for the floating card's pane
	}
	for k, v := range h.Colors {
		if strings.TrimSpace(k) == "" || !hudColor.MatchString(v) {
			delete(h.Colors, k)
		}
	}
	docked, floating := h.Layouts.Docked, h.Layouts.Floating
	h.Layouts.Docked = boundedTree(docked)
	h.Layouts.Floating = boundedTree(floating)
	if h.Layouts.Docked != docked || h.Layouts.Floating != floating {
		h.Layouts.Dropped = true
	}
	return h
}

func boundedTree(n *HUDNode) *HUDNode {
	if n == nil {
		return nil
	}
	if c, ok := treeCount(*n, 1); !ok || c > HUDMaxNodes {
		return nil
	}
	return n
}

func treeCount(n HUDNode, depth int) (int, bool) {
	if depth > HUDMaxDepth {
		return 0, false
	}
	c := 1
	for _, k := range n.Kids {
		sub, ok := treeCount(k, depth+1)
		if !ok {
			return 0, false
		}
		c += sub
		if c > HUDMaxNodes {
			return c, true // a wide tree need not be walked to the end
		}
	}
	return c, true
}

// Limits of the docked pane's height, on a 720-tall screen: 192 and its tabs
// are 30 % of the screen, the most the card may take. Line heights go below
// 100 % so a row needs less than the font's default leading.
const (
	HUDPaneMinH   = 120
	HUDPaneMaxH   = 192
	HUDLineMinPct = 85
)

// compactDocked moves a card from before the compact pane (settings version 9)
// to the new sizes. Only what still has the old default moves, and a layout of
// the player's own keeps the widths and line height it was made for; normalize
// then limits a taller pane.
func (h HUD) compactDocked() HUD {
	if h.Pane.Height == 360 {
		h.Pane.Height = defaultHUD().Pane.Height
		if h.Layouts.Docked != nil {
			h.Pane.Height = HUDPaneMaxH // the most room a tall layout can have
		}
	}
	if h.Layouts.Docked != nil {
		return h
	}
	d := defaultHUD()
	if h.Pane.TabW == 72 {
		h.Pane.TabW = d.Pane.TabW
	}
	if h.Text.BodySize == 12 && h.Text.BodyLinePct == 100 {
		h.Text.BodyLinePct = d.Text.BodyLinePct
	}
	if h.Text.ValueW == 60 {
		h.Text.ValueW = d.Text.ValueW
	}
	return h
}
