package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

// TestJourneyTameAnimalEatsStockpiledKibble (FEED-001): a tame husky, and
// a stockpile painted with the Architect tool in a walled room past a
// door, holding kibble, the only food on the map. Once hungry the husky
// walks through the door to the kibble: its food rises, its state reads
// "eating kibble", and its pane says which prepared food it eats.
func TestJourneyTameAnimalEatsStockpiledKibble(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	for y := 8; y <= 12; y++ {
		for x := 6; x <= 10; x++ {
			kind := "wall"
			switch {
			case x != 6 && x != 10 && y != 8 && y != 12:
				continue
			case x == 10 && y == 10:
				kind = "door"
			}
			if !j.g.BuildNow(kind, "wood", "south", x, y) {
				t.Fatalf("%s at %d,%d", kind, x, y)
			}
		}
	}
	j.openArch("zones", "stockpile")
	j.dragTiles(inputframe.Left, 7, 9, 9, 11, 0)
	j.leaveArch()
	if _, err := j.g.SpawnStack(item.Kibble, 60, 8, 10); err != nil {
		t.Fatal(err)
	}
	id, err := j.g.SpawnAnimal("husky", 15, 10)
	if err != nil {
		t.Fatal(err)
	}
	j.g.TameAnimal(id, "Rex")
	husky := func() game.AnimalView {
		for _, an := range j.world().Animals {
			if an.ID == id {
				return an
			}
		}
		t.Fatal("the husky is gone")
		return game.AnimalView{}
	}
	if ls := j.a.insp.Animal().AnimalPaneLines(husky()); !slices.Contains(ls, "Also eats kibble, pemmican and meals") {
		t.Fatalf("pane lines %q", ls)
	}
	// it gets hungry (food drains in under two days)
	for i := 0; i < 400 && husky().State != "eating kibble"; i++ {
		careFeedHostileFree(j)
		j.g.Step(250)
	}
	hungry := husky()
	if hungry.State != "eating kibble" {
		t.Fatalf("the hungry husky heads for the kibble: state %q, food %d%%", hungry.State, hungry.Food)
	}
	j.key("Digit3", 0)
	for i := 0; i < 3000 && husky().Food <= hungry.Food; i++ {
		careFeedHostileFree(j)
		j.update(1)
	}
	j.key("Space", 0)
	if got := husky(); got.Food <= hungry.Food {
		t.Fatalf("its food rises: %d%% then %d%% (state %q)", hungry.Food, got.Food, got.State)
	}
	left := 0
	for _, it := range j.world().Items {
		if it.Kind == item.Kibble.String() {
			left += it.Qty
		}
	}
	if left >= 60 {
		t.Fatalf("kibble eaten: %d left", left)
	}
}
