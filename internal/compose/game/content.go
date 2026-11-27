// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"log"
	"slices"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/inbound/ebiten/plugins"
	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/adapters/outbound/starlarkhost"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/contentid"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/platform/assetroot"
	"rimworld-go/internal/platform/config"
)

// runContent is the run's frozen content (PLG-*): the catalog built once
// at start from the installed plugins and the enabled set, what the game
// plays with, what saves record, and what the Plugins screen shows.
type runContent struct {
	catalog  modding.Catalog
	found    []modding.Plugin             // every installed plugin, enabled or not
	problems []modding.Problem            // what Build reported for the enabled ones
	enabled  []string                     // the enabled set the run started with
	roots    []string                     // the plugin folders read (none for captures)
	text     map[string]map[string]string // the loaded plugins' translations by locale (PLG-055)
}

// game is the books the simulation reads.
func (c runContent) game() appgame.Content {
	cat := c.catalog
	return appgame.Content{Backstories: cat.Backstories, Recipes: cat.Recipes, Traits: cat.Traits,
		Research: cat.Research, Incidents: cat.Incidents, Items: cat.Items, Sources: sources(cat.Defs), StatMods: cat.StatMods}
}

// sources is who added or patched each def a plugin touched (PLG-072).
func sources(defs []modding.Def) map[string]appgame.Source {
	var out map[string]appgame.Source
	for _, d := range defs {
		var s appgame.Source
		if d.Plugin != modding.Core {
			s.AddedBy = d.Plugin
		}
		for _, t := range d.Touched {
			if !slices.Contains(s.PatchedBy, t.Plugin) {
				s.PatchedBy = append(s.PatchedBy, t.Plugin)
			}
		}
		if s.AddedBy == "" && len(s.PatchedBy) == 0 {
			continue
		}
		if out == nil {
			out = map[string]appgame.Source{}
		}
		out[appgame.SourceKey(d.Class, d.Key)] = s
	}
	return out
}

// save is the content a save header records: fingerprint and load order.
func (c runContent) save() savefmt.Content {
	return contentid.Of(c.catalog.IDs(), contentid.Books{Backstories: c.catalog.Backstories, Recipes: c.catalog.Recipes,
		Traits: c.catalog.Traits, Research: c.catalog.Research, Incidents: c.catalog.Incidents, Items: c.catalog.Items})
}

// loadContent reads the plugin folders and builds the catalog for the
// enabled ids. Captures load no plugin, so their frames never depend on
// what a player installed. Problems are logged here; the Content Errors
// window shows them in the game.
func loadContent(capturing bool, enabled []string, version string) runContent {
	if capturing {
		return buildContent(nil, nil, version)
	}
	roots := modloader.DefaultRoots()
	c := buildContent(modloader.New(roots...), enabled, version)
	c.roots = roots
	return c
}

// pluginCaptures are the -capture-ui states of the Plugins screens.
var pluginCaptures = []string{"plugins", "plugins-problems"}

// captureContent is a capture's content: no plugin, except for the
// Plugins captures, which read only the shipped mods folder (never the
// player's) with its example enabled beside a fixture plugin that cannot
// load, so the screens are reviewed with rows, a reason and a problem.
func captureContent(ui, version string) runContent {
	if !slices.Contains(pluginCaptures, ui) {
		return buildContent(nil, nil, version)
	}
	root, err := assetroot.Find("mods", true)
	if err != nil {
		return buildContent(nil, nil, version)
	}
	c := buildContent(fixtureSource{modloader.New(root)}, []string{"example_frontier", "needs_missing"}, version)
	c.roots = []string{root}
	return c
}

// captureSettings are a Plugins capture's settings: the defaults with the
// fixture's enabled set, in memory only (no Save), so the screen shows the
// plugins as on. Other captures keep h.
func captureSettings(ui string, h uiadapter.SettingsHooks, c runContent) uiadapter.SettingsHooks {
	if !slices.Contains(pluginCaptures, ui) {
		return h
	}
	return uiadapter.SettingsHooks{Load: func() (config.Settings, error) {
		s := config.Defaults()
		s.Plugins.Enabled = c.enabled
		return s.Normalize(), nil
	}}
}

// fixtureSource adds a capture-only plugin whose requirement is missing.
type fixtureSource struct{ modding.Source }

func (f fixtureSource) Plugins() ([]modding.Plugin, error) {
	ps, err := f.Source.Plugins()
	ps = append(ps, modding.Plugin{Dir: "(capture fixture)", Manifest: modding.Manifest{
		ID: "needs_missing", Name: "Needs a missing plugin", Authors: []string{"rimworld-go"}, Version: "0.1.0",
		Game: []string{"0.1"}, Requires: []string{"absent_library"},
		Description: "A capture fixture: it requires a plugin that is not installed, so it cannot load."}})
	return ps, err
}

// buildContent is loadContent over src (nil: no plugin folders).
func buildContent(src modding.Source, enabled []string, version string) runContent {
	var found []modding.Plugin
	if src != nil {
		var err error
		if found, err = src.Plugins(); err != nil {
			log.Printf("plugins: %v (continuing without plugins)", err)
			found = nil
		}
	}
	cat, probs := modding.BuildWith(found, enabled, version, starlarkhost.NewDefs())
	text, lp := pluginText(found, cat)
	probs = append(probs, lp...)
	for _, p := range probs {
		log.Printf("plugins: %s", p)
	}
	if len(cat.Plugins) > 0 {
		log.Printf("plugins: loaded %v", cat.IDs()[1:])
	}
	return runContent{catalog: cat, found: found, problems: probs, enabled: enabled, text: text}
}

// view is the run's plugins for the Plugins screens (PLG-070): every
// installed plugin, the loaded ones in load order with the reason for
// their place, then the rest by id; every problem; the enabled set the run
// started with; and the folders read. A capture's is empty.
func (c runContent) view() plugins.View {
	v := plugins.View{Enabled: config.Plugins{Enabled: c.enabled}.Normalize().Enabled, Folders: c.roots}
	placed := map[string]int{}
	for i, p := range c.catalog.Order.Loaded {
		placed[p.ID] = i
	}
	rows := make([]plugins.Row, 0, len(c.found))
	for _, p := range c.found {
		m := p.Manifest
		r := plugins.Row{ID: m.ID, Name: m.Name, Version: m.Version, Description: m.Description, URL: m.URL,
			Authors: m.Authors, Requires: m.Requires, After: m.After, Before: m.Before, Conflicts: m.Conflicts,
			Locales: modding.PluginLocales(p), HasWasm: len(p.Wasm) > 0}
		r.API = max(m.API, 1)
		caps, legacy := modding.CapsOf(m)
		r.Caps, r.LegacyCaps = caps.Names(), legacy
		if i, ok := placed[m.ID]; ok && m.ID != modding.Core {
			r.Loaded, r.Position, r.Reason = true, i, c.catalog.Order.Loaded[i].Reason
		}
		rows = append(rows, r)
	}
	slices.SortStableFunc(rows, func(a, b plugins.Row) int { return loadRank(a) - loadRank(b) })
	v.Rows = rows
	for _, p := range c.problems {
		v.Problems = append(v.Problems, plugins.Problem{Plugin: p.Plugin, File: p.File, Def: p.Def, Field: p.Field, Msg: p.Msg, Warn: p.Warn})
	}
	return v
}

// loadRank sorts a loaded row by its place, before every row that is not
// loaded (those keep the source's id order).
func loadRank(r plugins.Row) int {
	if r.Loaded {
		return r.Position
	}
	return 1 << 30
}

// enabledPlugins is the enabled set from the stored settings (none when
// there is no settings file or it can't be read).
func enabledPlugins(load func() (config.Settings, error)) []string {
	if load == nil {
		return nil
	}
	s, err := load()
	if err != nil {
		return nil
	}
	return s.Normalize().Plugins.Enabled
}
