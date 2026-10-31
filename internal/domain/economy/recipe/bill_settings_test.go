package recipe

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestDefInputs(t *testing.T) {
	t.Parallel()
	club := DefOf(Club)
	if in := club.AppendInputs(nil); len(in) != 1 || in[0] != club.Input {
		t.Fatalf("a single-input recipe: %v", in)
	}
	// PRD-2: a stuffed recipe lists its material families.
	if club.InputsLabel() != "4 wood, metal or stone blocks" || DefOf(SimpleMeal).InputsLabel() != "10 raw food" || DefOf(MakeParka).InputsLabel() != "14 fabric or leather" {
		t.Fatalf("labels %q %q %q", club.InputsLabel(), DefOf(SimpleMeal).InputsLabel(), DefOf(MakeParka).InputsLabel())
	}
	if ks := club.InputKinds(); !slices.Equal(ks, item.Stuffs(item.Woody|item.Metallic|item.Stony)) || ks[0] != item.Wood {
		t.Fatalf("club kinds %v", ks)
	}
	if ks := DefOf(Cremate).InputKinds(); len(ks) != 1 || ks[0] != item.CorpseHuman {
		t.Fatalf("cremate kinds %v", ks)
	}
	meal := DefOf(SimpleMeal).InputKinds()
	if !slices.Contains(meal, item.Berries) || !slices.Contains(meal, item.Meat) || slices.Contains(meal, item.Meal) || len(meal) > 32 {
		t.Fatalf("meal kinds %v", meal)
	}
	multi := club
	multi.Inputs = []Input{{Kind: item.Wood, Qty: 4}, {Kind: item.Steel, Qty: 2}}
	if in := multi.AppendInputs(nil); len(in) != 2 || in[1].Kind != item.Steel {
		t.Fatalf("inputs %v", in)
	}
	if !multi.Accepts(item.Steel) || !multi.Accepts(item.Wood) || multi.Accepts(item.Cloth) {
		t.Fatal("accepts every input's kind")
	}
	if multi.InputsLabel() != "4 wood + 2 steel" || len(multi.InputKinds()) != 2 {
		t.Fatalf("label %q kinds %v", multi.InputsLabel(), multi.InputKinds())
	}
	for _, d := range All()[:MakeMace] { // every older recipe takes what it always took (PRD-3's guns take two)
		if len(d.Inputs) != 0 || !d.Accepts(d.Input.Kind) && !d.Input.AnyRaw && !d.Input.AnyDryable && !d.Input.AnyStoneBlocks {
			t.Fatalf("%s: inputs %v", d.Name, d.Inputs)
		}
	}
}

func TestBillSettings(t *testing.T) {
	t.Parallel()
	if (Bill{Target: 10}).ResumeAt() != 8 || (Bill{Target: 10, UnpauseAt: 3}).ResumeAt() != 3 ||
		(Bill{Target: 10, UnpauseAt: 10}).ResumeAt() != 8 {
		t.Fatal("ResumeAt honours an UnpauseAt below the target")
	}
	b := Bill{Mode: UntilHave, Target: 10, UnpauseAt: 3}.Update(10)
	if !b.Waiting || !b.Update(4).Waiting || b.Update(3).Waiting {
		t.Fatal("waits from the target down to UnpauseAt")
	}
	if (Bill{}).MaxSkill() != 20 || (Bill{SkillMax: 6}).MaxSkill() != 6 {
		t.Fatal("max skill")
	}
	if !(Bill{}).SkillOK(0) || (Bill{SkillMin: 5}).SkillOK(4) || (Bill{SkillMax: 5}).SkillOK(6) || !(Bill{SkillMin: 5, SkillMax: 5}).SkillOK(5) {
		t.Fatal("skill range")
	}
	if BestStockpile.String() != "best stockpile" || DropOnFloor.String() != "drop on floor" ||
		StockpileZone.String() != "specific stockpile" || !StockpileZone.Valid() || StoreMode(3).Valid() {
		t.Fatal("store modes")
	}
}

func TestBillForbidList(t *testing.T) {
	t.Parallel()
	b := Bill{}.SetAllowed(item.Meat, false).SetAllowed(item.Berries, false).SetAllowed(item.Meat, false)
	if !slices.Equal(b.Forbid, []item.Kind{item.Berries, item.Meat}) || b.Allows(item.Meat) || !b.Allows(item.Corn) {
		t.Fatalf("forbid %v", b.Forbid)
	}
	c := b // a copy shares the slice: changing one must not change the other
	c = c.SetAllowed(item.Berries, true)
	if !slices.Equal(b.Forbid, []item.Kind{item.Berries, item.Meat}) || !slices.Equal(c.Forbid, []item.Kind{item.Meat}) {
		t.Fatalf("b %v c %v", b.Forbid, c.Forbid)
	}
	if c = c.SetAllowed(item.Meat, true).SetAllowed(item.Corn, true); c.Forbid != nil {
		t.Fatalf("all allowed: %v", c.Forbid)
	}
}
