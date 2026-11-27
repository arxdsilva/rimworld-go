// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/stat/statmod"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

// Content is the frozen definition books a run plays with: core's plus
// the enabled plugins' (application/modding builds it at start). Each
// moddable class reads its book from here, never a package registry.
type Content struct {
	Backstories backstory.Book
	Recipes     recipe.Book
	Traits      trait.Book
	Research    research.Book
	Incidents   incident.Book
	Items       item.Book
	// StatMods are the stat_mod defs, the last layer of a stat sheet.
	StatMods statmod.Book
	Thoughts mood.Book
	// Sources names the plugins behind each plugin-touched def (PLG-072),
	// keyed SourceKey(class, key); a core def no plugin touched is absent.
	Sources map[string]Source
}

// Source is who added or changed one definition (PLG-072).
type Source struct {
	AddedBy   string   // the plugin that added it; "" for the game's own
	PatchedBy []string // the plugins whose patches changed it, in load order
}

// SourceKey is the Sources key of the def keyed key in class.
func SourceKey(class, key string) string { return class + "/" + key }

// SourceOf is who added or changed key in class (zero: the game's own,
// unchanged).
func (c Content) SourceOf(class, key string) Source { return c.Sources[SourceKey(class, key)] }

// contentKeys is the key table a save records (PLG-044): each moddable
// class's keys, index i being saved ID i's.
func contentKeys(g *Game) map[string][]string {
	c := g.sk.content
	return map[string][]string{
		backstory.Class: backstoriesOf(g).Keys(),
		recipe.Class:    g.craft.Bills.Book().Keys(),
		trait.Class:     c.Traits.Keys(),
		research.Class:  c.Research.Keys(),
		incident.Class:  c.Incidents.Keys(),
		item.Class:      c.Items.Keys(),
	}
}
