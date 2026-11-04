package inspector

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestPackRows(t *testing.T) {
	t.Parallel()
	if rows := PackRows(game.PawnView{PackLimitGrams: 40000}); rows != nil {
		t.Fatalf("an empty pack shows %v", rows)
	}
	p := game.PawnView{PackGrams: 1500, PackLimitGrams: 20000, Pack: []game.PackItemView{{ID: 1, Name: "wood", Qty: 3}, {ID: 2, Name: "steel", Qty: 1}}}
	want := []string{"Pack  1.5 / 20.0 kg", "  wood x3", "  steel x1"}
	if got := PackRows(p); !slices.Equal(got, want) {
		t.Fatalf("rows %q, want %q", got, want)
	}
}
