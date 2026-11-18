package savefmt

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// pluginBook is core's book with p.a then p.b after it (or only those
// keys given).
func pluginBook(keys ...string) recipe.Book {
	core := recipe.Core()
	list, ks := append([]recipe.Def(nil), core.All()...), core.Keys()
	for _, k := range keys {
		d := core.DefOf(recipe.Club)
		d.Name = k
		list, ks = append(list, d), append(ks, k)
	}
	return recipe.NewBook(list, ks)
}

// Saved recipe IDs follow their keys: recipe 0 stays (it is a recipe, not
// "none"), a moved plugin recipe moves, a gone one is reported gone.
func TestRecipeIDsFollowKeys(t *testing.T) {
	t.Parallel()
	saved := pluginBook("p.a", "p.b")
	n := recipe.ID(recipe.Core().Len())
	keys := Keys{recipe.Class: saved.Keys()}
	swapped := keys.Recipes(pluginBook("p.b", "p.a"))
	onlyB := keys.Recipes(pluginBook("p.b"))
	core := keys.Recipes(recipe.Core())
	for _, c := range []struct {
		m      RecipeIDs
		in     recipe.ID
		want   recipe.ID
		wantOK bool
	}{
		{swapped, recipe.SimpleMeal, recipe.SimpleMeal, true}, {swapped, n, n + 1, true}, {swapped, n + 1, n, true},
		{onlyB, n, 0, false}, {onlyB, n + 1, n, true}, {core, n, 0, false}, {core, n + 1, 0, false},
		{core, recipe.MakeFoamShells, recipe.MakeFoamShells, true}, {core, n + 2, 0, false},
	} {
		if got, ok := c.m(c.in); got != c.want || ok != c.wantOK {
			t.Errorf("map(%d) = %d, %v; want %d, %v", c.in, got, ok, c.want, c.wantOK)
		}
	}
	old := Keys{}.Recipes(pluginBook("p.a")) // a save from before PLG-025
	if id, ok := old(n + 7); id != n+7 || !ok {
		t.Fatal("a save without recipe keys must keep its IDs")
	}
}

// Bills and AddBill commands of a gone recipe are dropped, the rest
// remapped; a pack gathering for a gone recipe matches no bill.
func TestRecipeIDsRemapSavedState(t *testing.T) {
	t.Parallel()
	n := recipe.ID(recipe.Core().Len())
	m := Keys{recipe.Class: pluginBook("p.a", "p.b").Keys()}.Recipes(pluginBook("p.b"))
	bills := map[entity.EntityID][]recipe.Bill{
		1: {{Recipe: recipe.SimpleMeal, Target: 1}, {Recipe: n, Target: 2}, {Recipe: n + 1, Target: 3}},
		2: {{Recipe: recipe.Club}},
	}
	keep := bills[2]
	m.Bills(bills)
	if got := bills[1]; len(got) != 2 || got[0].Recipe != recipe.SimpleMeal || got[1].Recipe != n || got[1].Target != 3 {
		t.Fatalf("bench 1 bills %+v", got)
	}
	if &bills[2][0] != &keep[0] {
		t.Fatal("an untouched bench's bills were rebuilt")
	}
	cmds := []command.Command{{Kind: command.AddBill, Recipe: uint8(n)}, {Kind: command.Prioritize, Recipe: uint8(n)},
		{Kind: command.AddBill, Recipe: uint8(n + 1)}}
	got := m.Commands(cmds)
	if len(got) != 2 || got[0].Kind != command.Prioritize || got[0].Recipe != uint8(n) || got[1].Recipe != uint8(n) {
		t.Fatalf("commands %+v", got)
	}
	same := []command.Command{{Kind: command.AddBill, Recipe: uint8(n + 1)}}
	if out := m.Commands(same); &out[0] != &same[0] || out[0].Recipe != uint8(n) {
		t.Fatalf("no drop: %+v", out)
	}
	for _, c := range []struct{ in, want int }{{int(n), -1}, {int(n + 1), int(n)}, {0, 0}} {
		if j := m.Job(job.Record{Kind: job.PackIngredients, SpareX: c.in}); j.SpareX != c.want {
			t.Errorf("pack job recipe %d -> %d, want %d", c.in, j.SpareX, c.want)
		}
	}
	jobs := []job.Record{{Kind: job.PackIngredients, SpareX: int(n + 1)}}
	if RemapJobs(m, jobs, func(j *job.Record) *job.Record { return j }); jobs[0].SpareX != int(n) {
		t.Fatalf("RemapJobs: %+v", jobs)
	}
	if j := m.Job(job.Record{Kind: job.Haul, SpareX: int(n)}); j.SpareX != int(n) {
		t.Fatal("another job's SpareX is not a recipe")
	}
	if !slices.Equal(Keys(nil).Recipes(recipe.Core()).Commands(cmds[:1]), cmds[:1]) {
		t.Fatal("an older save's commands change")
	}
}
