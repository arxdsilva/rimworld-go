// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"fmt"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func panelFixture() []game.PluginPanelView {
	return []game.PluginPanelView{
		{Plugin: "alpha", Title: "Alpha panel", Lines: []string{"one", "two"},
			Actions: []game.PluginActionView{{ID: "go", Label: "Go"}, {ID: "stop", Label: "Stop"}}},
		{Plugin: "beta", Title: "Beta panel", Lines: []string{"x"}},
	}
}

// TestPluginPanelsWindow: the window declares one button per action and a
// press sends ScriptAction "plugin\naction"; it changes nothing else.
func TestPluginPanelsWindow(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.world.PluginPanels = panelFixture()
	w.PluginPanels().Toggle()
	if w.KitID() != PluginPanelsModalID {
		t.Fatalf("window %q open", w.KitID())
	}
	sh.kit.Begin()
	w.Declare()
	if _, ok := sh.kit.Find(ids.PluginAction + "beta.go"); ok {
		t.Fatal("beta has no actions")
	}
	for _, id := range []string{"alpha.go", "alpha.stop"} {
		if _, ok := sh.kit.Find(ids.PluginAction + id); !ok {
			t.Fatalf("no button %q", id)
		}
	}
	if !sh.kit.Click(ids.PluginAction + "alpha.stop") {
		t.Fatal("no click handler for alpha.stop")
	}
	sh.kit.Begin()
	w.Declare()
	want := command.Command{Kind: command.ScriptAction, Name: "alpha\nstop"}
	if len(sh.cmds) != 1 || sh.cmds[0] != want {
		t.Fatalf("press sent %+v, want %+v", sh.cmds, want)
	}
	w.PluginPanels().Toggle()
	if w.KitID() != "" {
		t.Fatal("a second Toggle did not close the window")
	}
}

// TestPluginPanelsEmpty: no panels is an empty state with no buttons.
func TestPluginPanelsEmpty(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	w.PluginPanels().Launch()
	sh.kit.Begin()
	w.Declare()
	if w.KitID() != PluginPanelsModalID {
		t.Fatal("window did not open")
	}
	if _, ok := sh.kit.Find(ppScrollID); ok {
		t.Fatal("an empty window has a scroll view")
	}
	if len(sh.cmds) != 0 {
		t.Fatalf("commands %+v", sh.cmds)
	}
}

// TestPluginPanelsOffColony: off the colony the window does not open.
func TestPluginPanelsOffColony(t *testing.T) {
	t.Parallel()
	_, w := newFake()
	w.PluginPanels().Launch()
	if w.KitID() != "" {
		t.Fatal("opened without a colony")
	}
}

// TestPluginPanelsVirtualised: with many panels only those in view declare
// buttons, and scrolling brings the later ones in.
func TestPluginPanelsVirtualised(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	var ps []game.PluginPanelView
	for i := range 40 {
		ps = append(ps, game.PluginPanelView{Plugin: fmt.Sprintf("p%d", i), Title: "T", Lines: []string{"l"},
			Actions: []game.PluginActionView{{ID: "a", Label: "A"}}})
	}
	sh.world.PluginPanels = ps
	w.PluginPanels().Launch()
	sh.kit.Begin()
	w.Declare()
	if _, ok := sh.kit.Find(ids.PluginAction + "p0.a"); !ok {
		t.Fatal("first panel's button missing")
	}
	if _, ok := sh.kit.Find(ids.PluginAction + "p39.a"); ok {
		t.Fatal("last panel declared while out of view")
	}
	sh.kit.Scroll[ppScrollID] = 1e6
	sh.kit.Begin()
	w.Declare()
	if _, ok := sh.kit.Find(ids.PluginAction + "p39.a"); !ok {
		t.Fatal("last panel's button missing after scrolling to the end")
	}
	if _, ok := sh.kit.Find(ids.PluginAction + "p0.a"); ok {
		t.Fatal("first panel still declared after scrolling away")
	}
}

func TestPanelHeight(t *testing.T) {
	t.Parallel()
	a := game.PluginPanelView{Lines: []string{"a", "b"}}
	b := a
	b.Actions = []game.PluginActionView{{ID: "x"}}
	if panelHeight(b) <= panelHeight(a) || panelHeight(a) <= panelHeight(game.PluginPanelView{}) {
		t.Fatalf("heights %v %v", panelHeight(a), panelHeight(b))
	}
}
