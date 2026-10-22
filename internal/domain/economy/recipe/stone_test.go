package recipe

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/geology"
)

func TestCutRecipes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id    ID
		stone geology.Stone
		name  string
		label string
	}{
		{CutSandstone, geology.Sandstone, "cut_sandstone", "Cut sandstone blocks"},
		{CutGranite, geology.Granite, "cut_granite", "Cut granite blocks"},
		{CutLimestone, geology.Limestone, "cut_limestone", "Cut limestone blocks"},
		{CutSlate, geology.Slate, "cut_slate", "Cut slate blocks"},
		{CutMarble, geology.Marble, "cut_marble", "Cut marble blocks"},
	}
	for _, tc := range tests {
		d := DefOf(tc.id)
		if d.ID != tc.id || d.Name != tc.name || d.Label != tc.label {
			t.Errorf("%v: id/name/label = %v/%q/%q", tc.stone, d.ID, d.Name, d.Label)
		}
		if d.Bench != construction.StonecutterTable || d.Skill != work.Craft || d.Work != 300 || d.Legacy {
			t.Errorf("%v: bench/skill/work = %v/%v/%d", tc.stone, d.Bench, d.Skill, d.Work)
		}
		if d.Input != (Input{Kind: item.ChunkOf(tc.stone), Qty: 1}) {
			t.Errorf("%v: input %+v", tc.stone, d.Input)
		}
		if d.Output != item.BlocksOf(tc.stone) || d.OutQty != CutYield || item.StoneOf(d.Output) != tc.stone {
			t.Errorf("%v: output %d× %v", tc.stone, d.OutQty, d.Output)
		}
		if !d.Input.Accepts(item.ChunkOf(tc.stone)) || d.Input.Accepts(item.BlocksOf(tc.stone)) || d.Input.Take(40) != 1 {
			t.Errorf("%v: input accepts the wrong stacks", tc.stone)
		}
	}
	if CutYield != 10 {
		t.Fatalf("CutYield = %d", CutYield)
	}
	if got := For(construction.StonecutterTable); len(got) != len(tests) {
		t.Fatalf("stonecutter runs %d recipes, want %d", len(got), len(tests))
	}
}

func TestKnifeAcceptsAnyBlocks(t *testing.T) {
	t.Parallel()
	in := DefOf(Knife).Input
	for _, k := range item.StoneBlocks() {
		if !in.Accepts(k) || !in.AcceptsStack(item.Stack{Kind: k, Qty: 5}) {
			t.Errorf("knife rejects %v", k)
		}
	}
	for _, k := range []item.Kind{item.Stone, item.GraniteChunk, item.Steel, item.Wood} {
		if in.Accepts(k) {
			t.Errorf("knife accepts %v", k)
		}
	}
	if in.Take(20) != 3 {
		t.Fatalf("knife takes %d", in.Take(20))
	}
}
