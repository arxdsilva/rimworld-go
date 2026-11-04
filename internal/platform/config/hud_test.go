package config

import (
	"encoding/json"
	"testing"
)

func TestHUDOldFileGetsDefaults(t *testing.T) {
	t.Parallel()
	s := Defaults()
	if err := json.Unmarshal([]byte(`{"version":7,"audio":{"master":50}}`), &s); err != nil {
		t.Fatal(err)
	}
	s = s.Normalize()
	if s.HUD.RefreshTenths != Defaults().HUD.RefreshTenths || s.HUD.Text != Defaults().HUD.Text {
		t.Errorf("a version 7 file lost the card's defaults: %+v", s.HUD)
	}
	if s.Version != SettingsVersion {
		t.Errorf("version %d", s.Version)
	}
}

func TestHUDNormalizeClamps(t *testing.T) {
	t.Parallel()
	s := Defaults()
	s.HUD.RefreshTenths = 99
	s.HUD.Pane.Height = 1
	s.HUD.Card.Anchor = 40
	s.HUD.Text.SmallDelta = 9
	s.HUD.Colors = map[string]string{"ok": "#01020304", "bad": "red", "": "#01020304"}
	got := s.Normalize().HUD
	if got.RefreshTenths != 30 || got.Pane.Height != HUDPaneMinH || got.Card.Anchor != 8 || got.Text.SmallDelta != 0 {
		t.Errorf("not clamped: %+v", got)
	}
	if len(got.Colors) != 1 || got.Colors["ok"] == "" {
		t.Errorf("colours %v", got.Colors)
	}
}

func TestHUDNormalizeModes(t *testing.T) {
	t.Parallel()
	s := Defaults()
	s.HUD.Pane.Restyle = false
	s.HUD.Docked = true
	if s.Normalize().HUD.Docked {
		t.Error("an unstyled pane can't dock the card")
	}
	s = Defaults()
	s.HUD.Docked = true
	s.HUD.Pane.LogBody = true
	if s.Normalize().HUD.Pane.LogBody {
		t.Error("the log body is floating only")
	}
}

func TestHUDTreeBounds(t *testing.T) {
	t.Parallel()
	deep := HUDNode{Kind: "gap"}
	for range HUDMaxDepth + 1 {
		deep = HUDNode{Kind: "block", Kids: []HUDNode{deep}}
	}
	wide := HUDNode{Kind: "block", Kids: make([]HUDNode, HUDMaxNodes+1)}
	okTree := HUDNode{Kind: "block", Kids: []HUDNode{{Kind: "gap"}}}
	s := Defaults()
	s.HUD.Layouts = HUDLayouts{Docked: &deep, Floating: &wide}
	if l := s.Normalize().HUD.Layouts; l.Docked != nil || l.Floating != nil {
		t.Errorf("an oversized tree survived: %+v", l)
	}
	s.HUD.Layouts = HUDLayouts{Docked: &okTree}
	if s.Normalize().HUD.Layouts.Docked == nil {
		t.Error("a small tree was dropped")
	}
}

// A file from before the compact pane moves what still has the old default.
func TestCompactPaneMigration(t *testing.T) {
	old := func(f func(h *HUD)) Settings {
		s := Defaults()
		s.Version = 8
		s.HUD.Pane.Height, s.HUD.Pane.TabW = 360, 72
		s.HUD.Text.BodyLinePct, s.HUD.Text.ValueW = 100, 60
		f(&s.HUD)
		return s.Normalize()
	}
	d := Defaults().HUD
	got := old(func(*HUD) {}).HUD
	if got.Pane.Height != d.Pane.Height || got.Pane.TabW != d.Pane.TabW ||
		got.Text.BodyLinePct != d.Text.BodyLinePct || got.Text.ValueW != d.Text.ValueW {
		t.Fatalf("old defaults not moved: %+v %+v", got.Pane, got.Text)
	}
	got = old(func(h *HUD) { h.Pane.Height, h.Pane.TabW, h.Text.ValueW = 500, 80, 44 }).HUD
	if got.Pane.Height != HUDPaneMaxH || got.Pane.TabW != 80 || got.Text.ValueW != 44 {
		t.Fatalf("a player's own sizes: %+v %+v", got.Pane, got.Text)
	}
	got = old(func(h *HUD) { h.Layouts.Docked = &HUDNode{Kind: "column"} }).HUD
	if got.Pane.Height != HUDPaneMaxH || got.Pane.TabW != 72 || got.Text.BodyLinePct != 100 || got.Text.ValueW != 60 {
		t.Fatalf("a player's own layout keeps its widths and gets the most height: %+v %+v", got.Pane, got.Text)
	}
	if got = old(func(h *HUD) { h.Pane.Height = 150 }).HUD; got.Pane.Height != 150 {
		t.Fatalf("a height in range stays: %d", got.Pane.Height)
	}
}
