// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"os"
	"path/filepath"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	appgame "rimworld-go/internal/application/game"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/plugins"
	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/application/game/contentid"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/mood"
)

// The shipped example plugin, enabled, reaches the game's books and the
// save header (PLG-024, PLG-043, PLG-050); disabled, nothing changes.
func TestContentFromEnabledPlugin(t *testing.T) {
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "0.1.35")
	if len(on.problems) != 0 {
		t.Fatalf("problems: %v", on.problems)
	}
	if got := on.game().Backstories.Len(); got != backstory.Core().Len()+3 {
		t.Fatalf("game books hold %d backstories", got)
	}
	h := on.save()
	if !slices.Equal(h.Plugins, []string{"core", "example_frontier"}) || h.Hash == contentid.Fingerprint() {
		t.Fatalf("save content = %+v", h)
	}
	off := buildContent(src, nil, "0.1.35")
	if h := off.save(); !slices.Equal(h.Plugins, []string{"core"}) || h.Hash != contentid.Current().Hash {
		t.Fatalf("disabled: save content = %+v", h)
	}
}

// Captures never load plugins, whatever is installed or enabled.
func TestCapturesLoadNoPlugin(t *testing.T) {
	c := loadContent(true, []string{"example_frontier"}, "0.1.35")
	if len(c.found) != 0 || c.save().Hash != contentid.Current().Hash || c.game().Backstories.Len() != backstory.Core().Len() {
		t.Fatalf("capture content = %+v", c.save())
	}
}

// The Plugins screens' view (PLG-070): the example plugin enabled is a
// loaded row at position 1 with its reason and the set the run started
// with; off, the same row is not loaded; a broken enabled set lists its
// problems; a capture's view is empty.
func TestContentView(t *testing.T) {
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "0.1.35").view()
	if len(on.Rows) != 1 || !slices.Equal(on.Enabled, []string{"example_frontier"}) {
		t.Fatalf("on: %+v", on)
	}
	if r := on.Rows[0]; r.ID != "example_frontier" || !r.Loaded || r.Position != 1 || r.Reason == "" || r.Name == "" {
		t.Fatalf("on: row %+v", r)
	}
	off := buildContent(src, nil, "0.1.35").view()
	if len(off.Rows) != 1 || off.Rows[0].Loaded || off.Enabled != nil || len(off.Problems) != 0 {
		t.Fatalf("off: %+v", off)
	}
	bad := buildContent(src, []string{"example_frontier", "missing"}, "0.1.35").view()
	if bad.Errors() != 1 || bad.Problems[0].Plugin != "missing" {
		t.Fatalf("missing plugin: problems %+v", bad.Problems)
	}
	if v := loadContent(true, []string{"example_frontier"}, "0.1.35").view(); len(v.Rows)+len(v.Problems)+len(v.Enabled)+len(v.Folders) != 0 {
		t.Fatalf("capture view %+v", v)
	}
}

// The missing-plugin notice works on a real save (PLG-050): saved with
// the example plugin on, the slot listed by a run without it names it;
// listed by a run with it, there is no notice; and the save still loads.
func TestSaveFromPluginRunNamesTheMissingPlugin(t *testing.T) {
	dir := t.TempDir()
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "v1.2.3")
	h := saveHooks(dir, "v1.2.3", on)
	meta := ports.SaveMeta{Name: "Frontier", Colony: "col-9", ColonyName: "Frontier", Base: "Frontier"}
	if err := h.Save(appgame.NewColony(appgame.Config{Seed: 4, Width: 30, Height: 30, Content: on.game()}), meta); err != nil {
		t.Fatal(err)
	}
	closeSaves(h)

	off := saveHooks(dir, "v1.2.3", buildContent(src, nil, "v1.2.3"))
	slots, err := off.List()
	if err != nil || len(slots) != 1 {
		t.Fatalf("slots %+v, %v", slots, err)
	}
	if !strings.Contains(slots[0].Notice, "example_frontier") {
		t.Fatalf("notice %q does not name the missing plugin", slots[0].Notice)
	}
	if _, _, err := off.Load(slots[0].Path); err != nil {
		t.Fatalf("a run without the plugin cannot load the save: %v", err)
	}
	same, _ := saveHooks(dir, "v1.2.3", on).List()
	if len(same) != 1 || same[0].Notice != "" {
		t.Fatalf("the same plugin set gives a notice: %+v", same)
	}
}

type memSource []modding.Plugin

func (s memSource) Plugins() ([]modding.Plugin, error) { return s, nil }

const registerTrait = `
def register_defs():
    return {"trait": [{"name": "scripted_calm", "label": "Scripted calm", "desc": "{name} is calm.",
                       "group": "calm", "mods": [{"stat": "move_speed", "mul": 1100}], "rarity": 2}]}
`

func scriptedPlugin(id string, caps []string, src string) modding.Plugin {
	m := modding.Manifest{ID: id, Name: id, Version: "1.0.0", Authors: []string{"t"}, Game: []string{"0.1"}, Description: "d", Capabilities: caps}
	if caps != nil {
		m.API = 1
	}
	return modding.Plugin{Dir: id, Manifest: m, Scripts: []modding.Blob{{Path: "scripts/main.star", Data: []byte(src)}}}
}

// C2: a plugin's register_defs reaches the catalog through buildContent
// (and so the game's books); without the defs capability it does not.
func TestBuildContentScriptDefs(t *testing.T) {
	base := buildContent(nil, nil, "0.1.35").game().Traits.Len()
	on := buildContent(memSource{scriptedPlugin("sd", []string{"defs"}, registerTrait)}, []string{"sd"}, "0.1.35")
	if len(on.problems) != 0 {
		t.Fatalf("problems: %v", on.problems)
	}
	if got := on.game().Traits.Len(); got != base+1 {
		t.Fatalf("traits %d, want %d", got, base+1)
	}
	off := buildContent(memSource{scriptedPlugin("sd", []string{"query"}, registerTrait)}, []string{"sd"}, "0.1.35")
	if got := off.game().Traits.Len(); got != base {
		t.Fatalf("without defs: traits %d, want %d", got, base)
	}
}

// C3: rows carry the API and the capabilities; a manifest without them is
// the legacy set at API 1.
func TestViewRowsAPICaps(t *testing.T) {
	src := memSource{
		scriptedPlugin("legacy", nil, "x = 1\n"),
		scriptedPlugin("decl", []string{"commands", "query"}, "x = 1\n"),
		wasmOnly("wasm"),
	}
	v := buildContent(src, []string{"legacy", "decl", "wasm"}, "0.1.35").view()
	rows := map[string]plugins.Row{}
	for _, r := range v.Rows {
		rows[r.ID] = r
	}
	if r := rows["legacy"]; r.API != 1 || !r.LegacyCaps || !slices.Equal(r.Caps, []string{"query", "commands", "events"}) || r.HasWasm {
		t.Fatalf("legacy row %+v", r)
	}
	if r := rows["decl"]; r.API != 1 || r.LegacyCaps || !slices.Equal(r.Caps, []string{"query", "commands"}) {
		t.Fatalf("declared row %+v", r)
	}
	if !rows["wasm"].HasWasm {
		t.Fatalf("wasm row %+v", rows["wasm"])
	}
}

func wasmOnly(id string) modding.Plugin {
	p := scriptedPlugin(id, nil, "")
	p.Scripts = nil
	p.Wasm = []modding.Blob{{Path: "scripts/a.wasm", Data: []byte{0}}}
	return p
}

// C9: the composition root hands the catalog's thought book to the game.
// Disabled, the book is core's and the save hash is core's; the example
// plugin enabled softens sleeping outside, names itself as the patcher and
// moves the hash.
func TestGameWiresThoughtBook(t *testing.T) {
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	off := buildContent(src, nil, "0.1.35")
	if off.save().Hash != contentid.Current().Hash {
		t.Fatalf("disabled hash = %d", off.save().Hash)
	}
	core := mood.Core()
	for i := range core.Len() {
		if got, want := off.game().Thoughts.DefOf(mood.Kind(i)), core.DefOf(mood.Kind(i)); got != want {
			t.Fatalf("disabled: %s = %+v, want %+v", core.Key(mood.Kind(i)), got, want)
		}
	}
	on := buildContent(src, []string{"example_frontier"}, "0.1.35")
	g := on.game()
	if v := g.Thoughts.DefOf(mood.SleptOutside).Value; v != -2 {
		t.Fatalf("enabled: slept outside = %d, want -2", v)
	}
	if s := g.Sources["thought/core.slept_outside"]; !slices.Equal(s.PatchedBy, []string{"example_frontier"}) {
		t.Fatalf("sources = %+v", s)
	}
	if on.save().Hash == contentid.Current().Hash {
		t.Fatal("the patched thought left the hash at core's")
	}
}

// A plugin's own thought is refused, yet its def is still found in Sources.
func TestFixtureThoughtIsListed(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "fixture")
	if err := os.MkdirAll(filepath.Join(root, "defs"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"plugin.json":        `{"id":"fixture","name":"Fixture","authors":["t"],"version":"1.0.0","game":["0.1"],"description":"d"}`,
		"defs/thoughts.json": `{"thought":[{"name":"x","label":"hi","value":1,"duration":100}]}`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := buildContent(modloader.New(dir), []string{"fixture"}, "0.1.35")
	if _, ok := c.game().Sources["thought/fixture.x"]; !ok {
		t.Fatalf("thought/fixture.x not in Sources: %v (problems %v)", c.game().Sources, c.problems)
	}
	if c.game().Thoughts.Len() != mood.Core().Len() {
		t.Fatalf("book grew to %d", c.game().Thoughts.Len())
	}
}
