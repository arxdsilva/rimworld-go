package recipe

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestCarpetRecipes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		id    ID
		bench construction.Kind
		in    item.Kind
		qty   int
		out   item.Kind
	}{
		{MakeStrawCarpet, construction.Workbench, item.Straw, 10, item.CarpetStraw},
		{MakeWoodCarpet, construction.Workbench, item.Wood, 6, item.CarpetWood},
		{SewClothCarpet, construction.TailoringBench, item.Cloth, 8, item.CarpetCloth},
		{SewLeatherCarpet, construction.TailoringBench, item.Leather, 6, item.CarpetLeather},
	} {
		d := DefOf(tc.id)
		if d.Bench != tc.bench || d.Input != (Input{Kind: tc.in, Qty: tc.qty}) || d.Output != tc.out || d.OutQty != 1 || d.Skill != work.Craft || d.Work <= 0 {
			t.Fatalf("%s = %+v", d.Name, d)
		}
		found := false
		for _, r := range For(tc.bench) {
			found = found || r.ID == tc.id
		}
		if !found {
			t.Fatalf("%s not offered at %s", d.Name, tc.bench)
		}
		if !d.Input.Accepts(tc.in) || d.Input.Accepts(item.Stone) {
			t.Fatalf("%s input accepts wrong kinds", d.Name)
		}
	}
}
