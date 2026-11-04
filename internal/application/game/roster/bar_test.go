package roster

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestBarOrderDefaultIsTheColonyOrder(t *testing.T) {
	t.Parallel()
	var b BarOrder
	on := []entity.EntityID{3, 5, 9}
	if got := b.Order(on); !slices.Equal(got, on) {
		t.Fatalf("order %v", got)
	}
	if got := b.Places(on, on[1:]); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("places %v", got)
	}
	if b.Export() != nil {
		t.Fatal("an unused order exports something")
	}
}

func TestBarOrderMove(t *testing.T) {
	t.Parallel()
	on := []entity.EntityID{1, 2, 3, 4}
	for _, c := range []struct {
		name  string
		id    entity.EntityID
		place int
		want  []entity.EntityID
	}{
		{"to the front", 4, 0, []entity.EntityID{4, 1, 2, 3}},
		{"forward", 1, 2, []entity.EntityID{2, 3, 1, 4}},
		{"below the bar clamps", 3, -5, []entity.EntityID{3, 1, 2, 4}},
		{"past the bar clamps", 2, 99, []entity.EntityID{1, 3, 4, 2}},
	} {
		var b BarOrder
		b.Move(on, c.id, c.place)
		if got := b.Order(on); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// A move that changes nothing leaves nothing to save: no order, no hash.
func TestBarOrderNoOpsKeepItEmpty(t *testing.T) {
	t.Parallel()
	on := []entity.EntityID{1, 2}
	var b BarOrder
	b.Move(on, 7, 0) // not on the bar
	b.Move(on, 1, 0) // already there
	b.Move(on, 2, 9) // clamps to where it stands
	if b.Export() != nil {
		t.Fatalf("exported %v", b.Export())
	}
}

func TestBarOrderNewColonistEntersAtTheEnd(t *testing.T) {
	t.Parallel()
	var b BarOrder
	b.Move([]entity.EntityID{1, 2, 3}, 3, 0)
	got := b.Order([]entity.EntityID{1, 2, 3, 8})
	if want := []entity.EntityID{3, 1, 2, 8}; !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
	b.Move([]entity.EntityID{1, 2, 3, 8}, 1, 3) // and can move like anyone
	got = b.Order([]entity.EntityID{1, 2, 3, 8})
	if want := []entity.EntityID{3, 2, 8, 1}; !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
}

// A colonist away is skipped, counts for no place, and comes back to the slot
// it held even when the others moved meanwhile.
func TestBarOrderGoneColonistIsIgnoredAndKeepsItsSlot(t *testing.T) {
	t.Parallel()
	var b BarOrder
	all := []entity.EntityID{1, 2, 3, 4}
	b.Move(all, 4, 3)
	b.Move(all, 1, 1) // 2 1 3 4
	home := []entity.EntityID{1, 2, 4}
	if got, want := b.Order(home), []entity.EntityID{2, 1, 4}; !slices.Equal(got, want) {
		t.Fatalf("away: %v, want %v", got, want)
	}
	if got, want := b.Places(home, home), []int{1, 0, 2}; !slices.Equal(got, want) {
		t.Fatalf("places: %v, want %v", got, want)
	}
	b.Move(home, 4, 0) // the bar's places: 4 2 1, with 3 still after 2 and 1's slots
	if got, want := b.Order(home), []entity.EntityID{4, 2, 1}; !slices.Equal(got, want) {
		t.Fatalf("moved away: %v, want %v", got, want)
	}
	if got, want := b.Order(all), []entity.EntityID{4, 2, 3, 1}; !slices.Equal(got, want) {
		t.Fatalf("back: %v, want %v", got, want)
	}
}

func TestBarOrderPlacesOfOneMapAmongAll(t *testing.T) {
	t.Parallel()
	var b BarOrder
	all := []entity.EntityID{1, 2, 3, 4}
	b.Move(all, 4, 0) // 4 1 2 3
	if got, want := b.Places(all, all[2:]), []int{3, 0}; !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
	if got := b.Places(all, nil); len(got) != 0 {
		t.Fatalf("no ids: %v", got)
	}
}

func TestBarOrderExportImport(t *testing.T) {
	t.Parallel()
	var b BarOrder
	on := []entity.EntityID{1, 2, 3}
	b.Move(on, 3, 0)
	saved := b.Export()
	saved2 := b.Export()
	saved2[0] = 99 // the export is a copy
	var c BarOrder
	c.Import(saved)
	saved[0] = 98 // and so is the import
	if got, want := c.Order(on), []entity.EntityID{3, 1, 2}; !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
	c.Import(nil) // an older save
	if c.Export() != nil {
		t.Fatal("an older save left an order")
	}
}
