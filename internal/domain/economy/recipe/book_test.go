package recipe

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
)

// The zero Book is core's, and For is the bench index of At minus the
// legacy recipes, in ID order (PLG-025).
func TestBookZeroIsCore(t *testing.T) {
	t.Parallel()
	var zero Book
	core := Core()
	if zero.Len() != int(numRecipes) || core.Len() != zero.Len() || !reflect.DeepEqual(zero.All(), defs[:]) {
		t.Fatalf("zero %d, core %d recipes", zero.Len(), core.Len())
	}
	for k := construction.Kind(0); k.Valid(); k++ {
		var want []Def
		for _, d := range defs {
			if d.At(k) && !d.Legacy {
				want = append(want, d)
			}
		}
		if got := zero.For(k); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: For %d recipes, want %d", k, len(got), len(want))
		}
	}
	if zero.Key(SimpleMeal) != "core.simple_meal" || zero.Key(numRecipes) != "" || zero.DefOf(numRecipes).Name != "" {
		t.Fatal("keys")
	}
	if id, ok := zero.IDOf("core.make_foam_shells"); !ok || id != MakeFoamShells {
		t.Fatalf("IDOf = %d, %v", id, ok)
	}
	if _, ok := zero.IDOf("p.none"); ok {
		t.Fatal("IDOf found a missing key")
	}
	if zero.Upgrade(ButcherDustbuck) != ButcherAnimal || zero.Upgrade(Club) != Club {
		t.Fatal("upgrade")
	}
}

// NewBook gives each recipe its position as ID, indexes its benches, and
// stops at MaxBook; NameOf keeps plugin names apart from core's.
func TestNewBook(t *testing.T) {
	t.Parallel()
	d := Core().DefOf(Club)
	d.ID, d.Name = 99, NameOf("p.club")
	b := NewBook(append(slices.Clone(Core().All()), d), append(Core().Keys(), "p.club"))
	id := ID(Core().Len())
	if got := b.DefOf(id); got.ID != id || got.Name != "p.club" || b.Key(id) != "p.club" || b.Len() != Core().Len()+1 {
		t.Fatalf("plugin recipe %+v key %q", got, b.Key(id))
	}
	if l := b.For(construction.Workbench); l[len(l)-1].ID != id || len(Core().For(construction.Workbench))+1 != len(l) {
		t.Fatal("the workbench index lacks the plugin recipe")
	}
	if NameOf("core.club") != "club" || NameOf("p.club") == "club" {
		t.Fatal("NameOf")
	}
	many := make([]Def, MaxBook+5)
	keys := make([]string, len(many))
	if NewBook(many, keys).Len() != MaxBook {
		t.Fatal("a book holds at most MaxBook recipes")
	}
}

// Work discovery reads For on every assignment: it allocates nothing.
func TestBookForAllocatesNothing(t *testing.T) {
	b := NewBook(Core().All(), Core().Keys())
	var zero Book
	if n := testing.AllocsPerRun(100, func() {
		_ = b.For(construction.Stove)
		_ = zero.For(construction.ButcherTable)
		_ = zero.DefOf(Club)
	}); n != 0 {
		t.Fatalf("%v allocations per call", n)
	}
}

func BenchmarkBookFor(b *testing.B) {
	book := Core()
	b.ReportAllocs()
	for b.Loop() {
		_ = book.For(construction.Workbench)
	}
}
