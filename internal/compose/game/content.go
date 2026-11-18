package game

import (
	"log"
	"slices"

	"rimworld-go/internal/adapters/inbound/ebiten/plugins"
	"rimworld-go/internal/adapters/outbound/modloader"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/contentid"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/platform/config"
)

// runContent is the run's frozen content (PLG-*): the catalog built once
// at start from the installed plugins and the enabled set, what the game
// plays with, what saves record, and what the Plugins screen shows.
type runContent struct {
	catalog  modding.Catalog
	found    []modding.Plugin  // every installed plugin, enabled or not
	problems []modding.Problem // what Build reported for the enabled ones
	enabled  []string          // the enabled set the run started with
	roots    []string          // the plugin folders read (none for captures)
}

// game is the books the simulation reads.
func (c runContent) game() appgame.Content {
	return appgame.Content{Backstories: c.catalog.Backstories}
}

// save is the content a save header records: fingerprint and load order.
func (c runContent) save() savefmt.Content {
	return contentid.Of(c.catalog.IDs(), c.catalog.Backstories)
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
	cat, probs := modding.Build(found, enabled, version)
	for _, p := range probs {
		log.Printf("plugins: %s", p)
	}
	if len(cat.Plugins) > 0 {
		log.Printf("plugins: loaded %v", cat.IDs()[1:])
	}
	return runContent{catalog: cat, found: found, problems: probs, enabled: enabled}
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
			Authors: m.Authors, Requires: m.Requires, After: m.After, Before: m.Before, Conflicts: m.Conflicts}
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
