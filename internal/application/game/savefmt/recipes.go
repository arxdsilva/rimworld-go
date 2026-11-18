package savefmt

import (
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

// Bills remaps every saved bill's recipe in place. A bill whose recipe is
// gone is dropped, never pointed at another recipe; a bench's list is
// rebuilt only then (the jobs on it fare as after a removed bill). Each
// bench is done on its own, so the map's order never shows.
func (m RecipeIDs) Bills(bills map[entity.EntityID][]recipe.Bill) {
	for bid, list := range bills {
		kept := list[:0:0]
		gone := false
		for i, bl := range list {
			id, ok := m(bl.Recipe)
			if !ok {
				gone = true
				continue
			}
			list[i].Recipe, bl.Recipe = id, id
			kept = append(kept, bl)
		}
		if gone {
			bills[bid] = kept
		}
	}
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

// Job remaps a pack gathering's recipe (job.PackIngredients keeps it in
// SpareX); a gone recipe becomes -1, which no bill matches, so the job
// ends as when its bill is removed.
func (m RecipeIDs) Job(j job.Record) job.Record {
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

// RemapJobs remaps the pack gatherings among a job store's saved entries,
// in place (Job); rec reaches an entry's job.
func RemapJobs[E any](m RecipeIDs, entries []E, rec func(*E) *job.Record) {
	for i := range entries {
		r := rec(&entries[i])
		*r = m.Job(*r)
	}
}
