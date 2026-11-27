// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// panelHost is the (idle) script host that turns the game's script clock on.
type panelHost struct{}

func (panelHost) Fire(scripting.Event, scripting.World, scripting.PawnReader) scripting.Result {
	return scripting.Result{}
}
func (panelHost) Uses(scripting.Hook) bool { return false }

// panelExt is a plugin host with one panel and one action; it records the
// actions the game ran.
type panelExt struct{ acted []string }

func (*panelExt) Subscribed(scripting.EventKind) bool { return false }
func (*panelExt) Event(scripting.Event, scripting.World, scripting.PawnReader) scripting.Result {
	return scripting.Result{}
}
func (*panelExt) RunsIncidents() bool { return false }
func (*panelExt) RunIncident(scripting.Event, scripting.World, scripting.PawnReader) (scripting.Result, bool) {
	return scripting.Result{}, false
}
func (*panelExt) Validates() bool { return false }
func (*panelExt) ValidateCommand(scripting.CommandView, scripting.Event, scripting.World, scripting.PawnReader) (string, string, scripting.Result) {
	return "", "", scripting.Result{}
}
func (*panelExt) HasPanels() bool { return true }
func (*panelExt) Panels(scripting.Event, scripting.World, scripting.PawnReader) ([]scripting.Panel, scripting.Result) {
	return []scripting.Panel{{Plugin: "plugin", Title: "Stats", Lines: []string{"all well"},
		Actions: []scripting.PanelAction{{ID: "action", Label: "Cheer"}}}}, scripting.Result{}
}
func (p *panelExt) Act(ev scripting.Event, _ scripting.World, _ scripting.PawnReader) scripting.Result {
	p.acted = append(p.acted, ev.Plugin+"\n"+ev.Action)
	return scripting.Result{}
}

// TestJourneyPluginPanelAction: after the hourly refresh the Plugins tab
// opens the window with the plugin's panel; its button leaves the client as
// ScriptAction "plugin\naction", and the game hands it to the plugin.
func TestJourneyPluginPanelAction(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ext := &panelExt{}
	game.UseScripts(j.g, panelHost{})
	game.UseExtensions(j.g, ext)
	j.tick(clock.TicksPerHour + 1)
	if ps := j.world().PluginPanels; len(ps) != 1 || ps[0].Plugin != "plugin" {
		t.Fatalf("panels after an hour: %+v", ps)
	}
	if !j.clickID(ids.TabPlugins) || j.a.win.KitID() != windows.PluginPanelsModalID {
		t.Fatalf("Plugins tab did not open the window (open %q)", j.a.win.KitID())
	}
	j.update(1)
	j.reset()
	if !j.clickID(ids.PluginAction + "plugin.action") {
		t.Fatal("the panel button is disabled")
	}
	got := j.cmdsOf(command.ScriptAction)
	if len(got) != 1 || got[0].Name != "plugin\naction" {
		t.Fatalf("ScriptAction sent: %+v", got)
	}
	j.tick(1)
	if len(ext.acted) != 1 || ext.acted[0] != "plugin\naction" {
		t.Fatalf("plugin ran %v", ext.acted)
	}
}
