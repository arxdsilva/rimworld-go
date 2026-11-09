package ebiten

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savefmt"
)

// The Load screen's list (SAV-011, SAV-012, SAV-021, SAV-022): what a row
// says, sorting and filtering, and the saves this build reads differently.

// listedJourney is a colony whose save list is slots and whose loads go to
// load (nil: a fresh colony).
func listedJourney(t *testing.T, slots []ports.SaveSlot, load func(path string) error) (*journey, *[]string) {
	t.Helper()
	j, _ := savesJourney(t)
	var loaded []string
	hooks := j.a.cfg.Saves
	hooks.List = func() ([]ports.SaveSlot, error) { return slots, nil }
	hooks.Load = func(path string) (Simulation, ports.SaveMeta, error) {
		loaded = append(loaded, path)
		if load != nil {
			if err := load(path); err != nil {
				return nil, ports.SaveMeta{}, err
			}
		}
		return game.New(game.Config{Seed: 3, Width: 24, Height: 24}), ports.SaveMeta{Name: path}, nil
	}
	j.a.cfg.Saves = hooks
	return j, &loaded
}

// toMenu leaves the colony without saving (the unsaved-progress question's
// Leave).
func toMenu(j *journey) {
	j.t.Helper()
	j.key("Escape", 0)
	j.clickID("pause.menu")
	j.clickID(unsavedLeave)
	if j.a.nav.screen() != ports.ScreenMainMenu {
		j.t.Fatalf("not on the main menu: %s", j.a.nav.screen())
	}
}

// loadRow is the Load screen's button for save i.
func loadRow(j *journey, i int) uikit.Button {
	j.t.Helper()
	j.update(1)
	id := fmt.Sprintf("%s%d", ids.PrefixLoadSlot, i)
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.ID == id {
			return b
		}
	}
	j.t.Fatalf("no row %s", id)
	return uikit.Button{}
}

// rowOrder is the Load screen's save rows by name, top to bottom.
func rowOrder(j *journey) string {
	j.update(1)
	var out []string
	for _, b := range j.a.opt.Saves.LoadButtons(j.a.sw, j.a.sh) {
		var i int
		if _, err := fmt.Sscanf(b.ID, ids.PrefixLoadSlot+"%d", &i); err == nil && i >= 0 {
			out = append(out, j.a.opt.Saves.Slots[i].Name)
		}
	}
	return strings.Join(out, ",")
}

// TestJourneySavesLoadRowShowsColonyAndBuild: a row names the colony, the
// day and season, the colonists and the build, and its hover card has
// everything.
func TestJourneySavesLoadRowShowsColonyAndBuild(t *testing.T) {
	t.Parallel()
	j, _ := listedJourney(t, []ports.SaveSlot{{Name: "ridge", Path: "ridge", Title: "Ridge before winter",
		ColonyName: "Ridge", Day: 14, Season: "Fall", Colonists: 5, Storyteller: "Calm", Difficulty: "Rough",
		Biome: "Boreal forest", Build: "0.9.3", MapSize: 250, Version: 16, Modified: time.Unix(1_700_000_000, 0)}}, nil)
	j.key("Escape", 0)
	j.clickID("pause.load")
	if j.a.nav.screen() != ports.ScreenLoad {
		t.Fatalf("Load opened %s", j.a.nav.screen())
	}
	row := loadRow(j, 0)
	for _, s := range []string{"Ridge before winter", "Ridge · Day 14, Fall · 5 colonists · Calm/Rough · Boreal forest · build 0.9.3", "map 250"} {
		if !strings.Contains(row.Tip, s) {
			t.Fatalf("row card %q lacks %q", row.Tip, s)
		}
	}
	j.draw(1) // the two lines paint, cut to the row
}

// TestJourneySavesSortAndFilter: Sort turns the list to name order and
// back; Colony shows one colony's saves at a time, then all again.
func TestJourneySavesSortAndFilter(t *testing.T) {
	t.Parallel()
	j, _ := listedJourney(t, []ports.SaveSlot{
		{Name: "rB", Path: "rB", ColonyName: "Ridge"}, {Name: "ma", Path: "ma", ColonyName: "Marsh"},
		{Name: "Ra", Path: "Ra", ColonyName: "Ridge"},
	}, nil)
	j.key("Escape", 0)
	j.clickID("pause.load")
	if got := rowOrder(j); got != "rB,ma,Ra" {
		t.Fatalf("default order %s, want the store's", got)
	}
	j.clickID(ids.LoadSort)
	if got := rowOrder(j); got != "ma,Ra,rB" {
		t.Fatalf("by name %s", got)
	}
	j.clickID(ids.LoadColony)
	if got := rowOrder(j); got != "Ra,rB" {
		t.Fatalf("Ridge only %s", got)
	}
	j.clickID(ids.LoadColony)
	j.clickID(ids.LoadSort)
	if got := rowOrder(j); got != "ma" {
		t.Fatalf("Marsh, store order: %s", got)
	}
	j.clickID(ids.LoadColony)
	if got := rowOrder(j); got != "rB,ma,Ra" {
		t.Fatalf("All again: %s", got)
	}
	for _, id := range []string{ids.LoadSort, ids.LoadColony} {
		for _, b := range j.a.opt.Saves.LoadButtons(j.a.sw, j.a.sh) {
			if b.ID == id && !strings.HasPrefix(b.Label, map[string]string{ids.LoadSort: "Sort: Newest", ids.LoadColony: "Colony: All"}[id]) {
				t.Fatalf("%s says %q", id, b.Label)
			}
		}
	}
}

// TestJourneySavesOlderVersionAsks: Continue on an older save asks before
// it upgrades it; Cancel stays on the menu, Load loads.
func TestJourneySavesOlderVersionAsks(t *testing.T) {
	t.Parallel()
	j, loaded := listedJourney(t, []ports.SaveSlot{{Name: "old", Path: "old", Compat: int(savefmt.CompatOlder), Modified: time.Unix(1_700_000_000, 0)}}, nil)
	toMenu(j)
	j.clickID(ids.MenuContinue)
	if j.a.uiSt().Top() != "saves.compat" || len(*loaded) != 0 {
		t.Fatalf("Continue on an older save: modal %q, loaded %v", j.a.uiSt().Top(), *loaded)
	}
	j.clickID("modal.saves.compat.cancel")
	if len(*loaded) != 0 || j.a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("Cancel loaded %v", *loaded)
	}
	j.clickID(ids.MenuContinue)
	j.clickID("modal.saves.compat.ok")
	j.update(1)
	if len(*loaded) != 1 || j.a.nav.screen() != ports.ScreenColony || j.a.renderer == nil {
		t.Fatalf("Load: loaded %v, screen %s", *loaded, j.a.nav.screen())
	}
}

// TestJourneySavesNewerRefused: a save from a newer build is listed off,
// with the build and what to do.
func TestJourneySavesNewerRefused(t *testing.T) {
	t.Parallel()
	j, loaded := listedJourney(t, []ports.SaveSlot{{Name: "new", Path: "new", Compat: int(savefmt.CompatNewer), Build: "2.0.1"}}, nil)
	toMenu(j)
	j.clickID(ids.MenuLoad)
	row := loadRow(j, 0)
	if !row.Disabled || row.Reason != "Made with a newer version (build 2.0.1). Update the game to load it." {
		t.Fatalf("row %+v", row)
	}
	if j.clickID(ids.PrefixLoadSlot+"0") || len(*loaded) != 0 {
		t.Fatalf("a newer save loaded: %v", *loaded)
	}
}

// TestJourneySavesCorruptDisabled: a save the list cannot read is off with
// its reason; one that fails to load is marked damaged and cannot be tried
// again until the list is read anew.
func TestJourneySavesCorruptDisabled(t *testing.T) {
	t.Parallel()
	j, loaded := listedJourney(t, []ports.SaveSlot{{Name: "bad", Path: "bad", Err: "corrupt"}, {Name: "torn", Path: "torn"}},
		func(string) error { return errors.New("unexpected end of file") })
	toMenu(j)
	j.clickID(ids.MenuLoad)
	if row := loadRow(j, 0); !row.Disabled || !strings.Contains(row.Reason, "corrupt") {
		t.Fatalf("unreadable row %+v", row)
	}
	j.clickID(ids.PrefixLoadSlot + "1")
	if len(*loaded) != 1 || !strings.Contains(j.a.hint, "unexpected end of file") {
		t.Fatalf("loaded %v, hint %q", *loaded, j.a.hint)
	}
	if row := loadRow(j, 1); !row.Disabled || !strings.Contains(row.Reason, "damaged file") {
		t.Fatalf("after the failed load %+v", row)
	}
	if j.clickID(ids.PrefixLoadSlot+"1") || len(*loaded) != 1 {
		t.Fatalf("a damaged save was tried again: %v", *loaded)
	}
}
