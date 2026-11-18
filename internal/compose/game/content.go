package game

import (
	"log"

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
	return buildContent(modloader.New(modloader.DefaultRoots()...), enabled, version)
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
	return runContent{catalog: cat, found: found, problems: probs}
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
