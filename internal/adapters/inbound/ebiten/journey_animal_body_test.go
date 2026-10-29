package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/economy/item"
)

// journey_animal_body_test.go: picking a big animal by its body (PLAY-022
// C7). The art is xlArt's elephant (animal_species_test.go): its body
// covers two tiles west of its cell to two east, and the row above.

// TestJourneyAnimalBodyClickSelectsBigAnimal: a click on an XL animal's
// flank, two tiles off its cell, selects it and opens its pane; a stack or
// a colonist on another body tile still wins its own tile; a box dragged
// over the flank alone takes the animal.
func TestJourneyAnimalBodyClickSelectsBigAnimal(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id, err := j.g.SpawnAnimal("elephant", 12, 6)
	if err != nil {
		t.Fatal(err)
	}
	ax, ay := -1, -1
	for _, an := range j.world().Animals {
		if an.ID == id {
			ax, ay = an.X, an.Y
		}
	}
	if ax < 0 {
		t.Fatal("no elephant in the view")
	}
	j.a.renderer.art = xlArt()
	j.update(1)

	// the flank: two tiles east of the cell, nothing else on it but grass
	j.clickTile(ax+2, ay, inputframe.Left, 0)
	if !j.a.sel.HasAnimal || j.a.sel.SelAnimal != id {
		t.Fatalf("a click on the flank selected animal %v (has %v), want the elephant %v", j.a.sel.SelAnimal, j.a.sel.HasAnimal, id)
	}
	if !j.a.insp.PaneShown() {
		t.Fatal("no pane for the elephant")
	}

	// a stack on another body tile is picked on its own tile
	j.settle()
	sid, err := j.g.SpawnStack(item.Wood, 5, ax-1, ay-1)
	if err != nil {
		t.Fatal(err)
	}
	j.clickTile(ax-1, ay-1, inputframe.Left, 0)
	if !j.a.sel.HasItem || j.a.sel.SelItem != sid || j.a.sel.HasAnimal {
		t.Fatalf("a stack on the body: item %v/%v, animal %v", j.a.sel.HasItem, j.a.sel.SelItem, j.a.sel.HasAnimal)
	}
	// clicking it again cycles on to the body over it
	j.settle()
	j.clickTile(ax-1, ay-1, inputframe.Left, 0)
	if !j.a.sel.HasAnimal || j.a.sel.SelAnimal != id {
		t.Fatal("a second click on the stack's tile does not reach the body over it")
	}

	// a colonist on a body tile wins it
	j.settle()
	pid, err := j.g.SpawnPawn(ax+1, ay-1)
	if err != nil {
		t.Fatal(err)
	}
	j.update(1)
	j.clickTile(ax+1, ay-1, inputframe.Left, 0)
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != pid || j.a.sel.HasAnimal {
		t.Fatalf("a colonist on the body: selected %v, animal %v", j.a.renderer.Selected(), j.a.sel.HasAnimal)
	}

	// a box over the flank and the empty tile under it takes the animal
	j.settle()
	j.dragTiles(inputframe.Left, ax+2, ay+1, ax+2, ay, 0)
	if !j.a.sel.HasAnimal || j.a.sel.SelAnimal != id {
		t.Fatalf("a box over the flank selected animal %v (has %v)", j.a.sel.SelAnimal, j.a.sel.HasAnimal)
	}
}
