package recipe_test

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/world/plant"
)

func TestFabricateAndRefineRecipes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id       recipe.ID
		name     string
		in       recipe.Input
		out      item.Kind
		work     int
		wt       work.Type
		learn    skill.ID
		minSkill uint8
	}{
		{recipe.FabricateComponent, "fabricate_component", recipe.Input{Kind: item.Steel, Qty: 12}, item.Components,
			1500, work.Craft, skill.Crafting, 6},
		{recipe.RefineMedicine, "refine_medicine", recipe.Input{Kind: item.HerbalMedicine, Qty: 2}, item.IndustrialMedicine,
			800, work.Doctor, skill.Medicine, 4},
	}
	bench := recipe.For(construction.Workbench)
	for _, tc := range tests {
		d := recipe.DefOf(tc.id)
		if d.ID != tc.id || d.Name != tc.name || d.Bench != construction.Workbench || d.Input != tc.in ||
			d.Output != tc.out || d.OutQty != 1 || d.Work != tc.work || d.Skill != tc.wt || d.Learn != tc.learn ||
			d.MinSkill != tc.minSkill || d.Legacy {
			t.Errorf("%s = %+v", tc.name, d)
		}
		found := false
		for _, b := range bench {
			found = found || b.ID == tc.id
		}
		if !found {
			t.Errorf("the workbench does not offer %s", tc.name)
		}
	}
}

// TestEveryRecipeLearnsItsSkill: a recipe trains the skill of the work
// type that does it (cremating is crafting); only the two advanced
// recipes ask for a minimum level.
func TestEveryRecipeLearnsItsSkill(t *testing.T) {
	t.Parallel()
	for _, d := range recipe.All() {
		want, ok := skill.ForWork(d.Skill)
		if !ok {
			t.Fatalf("%s: work type %v trains no skill", d.Name, d.Skill)
		}
		if d.Learn != want {
			t.Errorf("%s learns %s, want %s", d.Name, d.Learn.Key(), want.Key())
		}
		if d.ID != recipe.FabricateComponent && d.ID != recipe.RefineMedicine && d.MinSkill != 0 {
			t.Errorf("%s: min skill %d", d.Name, d.MinSkill)
		}
	}
	if recipe.DefOf(recipe.Cremate).Learn != skill.Crafting || recipe.DefOf(recipe.ButcherAnimal).Learn != skill.Cooking {
		t.Fatal("cremate is crafting, butchering is cooking")
	}
}

// TestFabricateNeedsSkillAndResearch (domain part): fabricating is locked
// behind Fabrication (after Gunsmithing) and asks for crafting 6;
// refining behind Pharmacology (after Herbalism) and medicine 4.
func TestFabricateNeedsSkillAndResearch(t *testing.T) {
	t.Parallel()
	var pr research.Progress
	key := "recipe:" + recipe.DefOf(recipe.FabricateComponent).Name
	if p, ok := pr.Locks(key); !ok || p != research.Fabrication {
		t.Fatalf("fabricate locked by %v %v", p, ok)
	}
	if pr.Available(research.Fabrication) {
		t.Fatal("fabrication needs gunsmithing first")
	}
	pr.Add(research.Traps, research.DefOf(research.Traps).Cost)
	pr.Add(research.Gunsmithing, research.DefOf(research.Gunsmithing).Cost)
	if !pr.Available(research.Fabrication) || !pr.Add(research.Fabrication, research.DefOf(research.Fabrication).Cost) {
		t.Fatal("fabrication researchable after gunsmithing")
	}
	if _, ok := pr.Locks(key); ok {
		t.Fatal("fabrication researched: components unlocked")
	}
	for _, b := range []construction.Kind{construction.ToolCabinet, construction.HospitalBed} {
		if _, ok := (research.Progress{}).Locks("build:" + b.String()); !ok {
			t.Errorf("%s is not behind research", b)
		}
		if _, ok := pr.Locks("build:" + b.String()); ok {
			t.Errorf("%s still locked after fabrication", b)
		}
	}
	refine := "recipe:" + recipe.DefOf(recipe.RefineMedicine).Name
	if p, ok := pr.Locks(refine); !ok || p != research.Pharmacology {
		t.Fatalf("refine locked by %v %v", p, ok)
	}
	if recipe.DefOf(recipe.FabricateComponent).MinSkill != 6 || recipe.DefOf(recipe.RefineMedicine).MinSkill != 4 {
		t.Fatal("minimum skills")
	}
}

// TestResearchUnlockKeysResolve: every unlock names a real building, crop
// or recipe.
func TestResearchUnlockKeysResolve(t *testing.T) {
	t.Parallel()
	for _, d := range research.All() {
		for _, u := range d.Unlocks {
			kind, name, _ := strings.Cut(u, ":")
			ok := false
			switch kind {
			case "build":
				_, ok = construction.Parse(name)
			case "crop":
				_, ok = plant.Parse(name)
			case "recipe":
				for _, r := range recipe.All() {
					ok = ok || r.Name == name && !r.Legacy
				}
			}
			if !ok {
				t.Errorf("%s unlocks unknown %q", d.Name, u)
			}
		}
	}
}

func TestInputTakeModesAndUnknown(t *testing.T) {
	t.Parallel()
	in := recipe.Input{Kind: item.Steel, Qty: 12}
	if in.Take(40) != 12 || (recipe.Input{AnyDryable: true, Qty: 1, Max: 25}).Take(40) != 25 ||
		(recipe.Input{AnyDryable: true, Qty: 1, Max: 25}).Take(7) != 7 {
		t.Fatal("take")
	}
	if !in.AcceptsStack(item.Stack{Kind: item.Steel}) || in.AcceptsStack(item.Stack{Kind: item.Wood}) {
		t.Fatal("steel input takes steel stacks")
	}
	dry := recipe.DefOf(recipe.DryFood).Input
	if !dry.AcceptsStack(item.Stack{Kind: item.Meat, Qty: 3}) || dry.AcceptsStack(item.Stack{Kind: item.Meat, Qty: 3, Dried: true}) {
		t.Fatal("dry recipes skip dried stacks")
	}
	if recipe.DefOf(recipe.ID(250)).Name != "" {
		t.Fatal("unknown recipe")
	}
	for m, want := range map[recipe.Mode]string{recipe.DoCount: "do", recipe.Forever: "forever", recipe.UntilHave: "until you have"} {
		if m.String() != want {
			t.Errorf("mode %d = %q", m, m.String())
		}
	}
}
