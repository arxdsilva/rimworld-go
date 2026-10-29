package recipe

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestStuffedRecipes (PRD-2): the clubs, knives, spears and leather
// garments take any material of their families; the rest none.
func TestStuffedRecipes(t *testing.T) {
	t.Parallel()
	for _, id := range []ID{Club, Knife, Spear, MakeParka, MakeTuque, MakeDuster} {
		d := DefOf(id)
		if !d.Stuffed() || d.Input.Stuff != item.MadeFrom(d.Output) || !d.Input.Accepts(item.DefaultStuff(d.Output)) {
			t.Errorf("%s: %+v", d.Name, d.Input)
		}
	}
	if DefOf(MakeRifle).Stuffed() || DefOf(SimpleMeal).Stuffed() || DefOf(MakeRifle).Materials() != nil {
		t.Fatal("unstuffed recipes")
	}
	if got := DefOf(MakeParka).Materials(); !slices.Equal(got, []item.Kind{item.Leather, item.Cloth, item.Wool}) {
		t.Fatalf("parka materials %v", got)
	}
	multi := DefOf(Club)
	multi.Inputs = []Input{multi.Input, {Kind: item.Steel, Qty: 1}}
	if multi.Stuffed() {
		t.Fatal("a multi-input recipe is not stuffed")
	}
}

func TestPinStuff(t *testing.T) {
	t.Parallel()
	club := DefOf(Club)
	b := Bill{Recipe: Club}.PinStuff(club, item.Steel)
	if b.Stuff != item.Steel || !b.Allows(item.Steel) || b.Allows(item.Wood) || !b.Allows(item.Meal) {
		t.Fatalf("pinned steel: %+v", b)
	}
	if b.PinStuff(club, item.Cloth).Stuff != 0 || b.PinStuff(club, 0).Stuff != 0 {
		t.Fatal("a kind the recipe doesn't take, or 0, unpins")
	}
	if (Bill{}).PinStuff(DefOf(MakeRifle), item.Wood).Stuff != 0 {
		t.Fatal("an unstuffed recipe never pins")
	}
	if forbid := (Bill{}).SetAllowed(item.Steel, false); forbid.Allows(item.Steel) || !forbid.Allows(item.Wood) {
		t.Fatal("the allowed-stuff list is Forbid")
	}
}
