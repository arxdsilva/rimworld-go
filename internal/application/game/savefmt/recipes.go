// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"math"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// RecipeIDs maps a saved recipe ID onto the running content (PLG-044,
// PLG-025): the ID of the same key now, ok false when the run lacks the
// key (its plugin is no longer loaded). A recipe ID starts at 0, a real
// recipe, so it is shifted by one through Remap, whose 0 is "gone".
type RecipeIDs func(recipe.ID) (recipe.ID, bool)

// Recipes is the recipe map onto book. A save without recipe keys
// (before PLG-025: core content only) keeps its IDs.
func (k Keys) Recipes(book recipe.Book) RecipeIDs {
	to := k.Remap(recipe.Class, func(key string) (int, bool) {
		id, ok := book.IDOf(key)
		return int(id) + 1, ok
	})
	return func(id recipe.ID) (recipe.ID, bool) {
		j := to(int(id) + 1)
		if j == 0 {
			return 0, false
		}
		return recipe.ID(j - 1), true
	}
}

// Moved is, per bench whose list lost a bill, each saved bill index's new
// index (-1: dropped), for the jobs that point at a bill by index.
type Moved map[entity.EntityID][]int

// Bills remaps every saved bill's recipe in place. A bill whose recipe is
// gone is dropped, never pointed at another recipe; a bench's list is
// rebuilt only then, and its index moves are returned for Job. Each bench
// is done on its own, so the map's order never shows.
func (m RecipeIDs) Bills(bills map[entity.EntityID][]recipe.Bill) Moved {
	var moved Moved
	for bid, list := range bills {
		kept := list[:0:0]
		to := make([]int, len(list))
		for i, bl := range list {
			id, ok := m(bl.Recipe)
			if !ok {
				to[i] = -1
				continue
			}
			list[i].Recipe, bl.Recipe = id, id
			to[i] = len(kept)
			kept = append(kept, bl)
		}
		if len(kept) < len(list) {
			bills[bid] = kept
			if moved == nil {
				moved = Moved{}
			}
			moved[bid] = to
		}
	}
	return moved
}

// Commands remaps the recipe of every AddBill command in cmds, in place,
// dropping one whose recipe is gone (cmds itself when none is).
func (m RecipeIDs) Commands(cmds []command.Command) []command.Command {
	var out []command.Command // nil until a command is dropped
	for i, c := range cmds {
		if c.Kind == command.AddBill {
			id, ok := m(recipe.ID(c.Recipe))
			if !ok {
				if out == nil {
					out = append(make([]command.Command, 0, len(cmds)), cmds[:i]...)
				}
				continue
			}
			c.Recipe = uint8(id)
			cmds[i] = c
		}
		if out != nil {
			out = append(out, c)
		}
	}
	if out == nil {
		return cmds
	}
	return out
}

// Job remaps a bill job onto the loaded content: its bill index follows
// moved (a job whose bill was dropped gets an index past every list, so
// it fails as when its bill is removed, never working another bill), and
// a pack gathering's recipe (job.PackIngredients keeps it in SpareX)
// follows its key, a gone one becoming -1, which no bill matches.
func (m RecipeIDs) Job(j job.Record, moved Moved) job.Record {
	if j.Kind == job.Bill || j.Kind == job.HaulIngredients || j.Kind == job.PackIngredients {
		if to, ok := moved[j.Dest]; ok && j.BillIndex >= 0 && j.BillIndex < len(to) {
			j.BillIndex = to[j.BillIndex]
			if j.BillIndex < 0 {
				j.BillIndex = math.MaxInt32
			}
		}
	}
	if j.Kind != job.PackIngredients || j.SpareX < 0 || j.SpareX >= recipe.MaxBook {
		return j
	}
	id, ok := m(recipe.ID(j.SpareX))
	j.SpareX = int(id)
	if !ok {
		j.SpareX = -1
	}
	return j
}

// RemapJobs remaps the bill jobs among a job store's saved entries, in
// place (Job); rec reaches an entry's job.
func RemapJobs[E any](m RecipeIDs, moved Moved, entries []E, rec func(*E) *job.Record) {
	for i := range entries {
		r := rec(&entries[i])
		*r = m.Job(*r, moved)
	}
}
