package recipe_test

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/research"
)

// TestServoRecipesNeedResearch (MED-05): the servo tier is made at the
// workbench from components at crafting 8, behind Servo replacements,
// which needs Prosthetics and Fabrication; the valve heart is steel at
// crafting 5 behind Prosthetics. Nothing makes a nanomed kit or an organ.
func TestServoRecipesNeedResearch(t *testing.T) {
	t.Parallel()
	servo := map[recipe.ID]item.Kind{recipe.MakeServoArm: item.ServoArm, recipe.MakeServoLeg: item.ServoLeg,
		recipe.MakeOpticImplant: item.OpticImplant, recipe.MakeCochlearImplant: item.CochlearImplant,
		recipe.MakeSpinalBrace: item.SpinalBrace, recipe.MakePumpHeart: item.PumpHeart, recipe.MakeSynthStomach: item.SynthStomach}
	var pr research.Progress
	for id, out := range servo {
		d := recipe.DefOf(id)
		if d.Output != out || d.OutQty != 1 || d.Bench != construction.Workbench || d.Input.Kind != item.Components ||
			d.Input.Qty <= 0 || d.MinSkill != 8 {
			t.Errorf("%s = %+v", d.Name, d)
		}
		if p, ok := pr.Locks("recipe:" + d.Name); !ok || p != research.ServoReplacements {
			t.Errorf("%s locked by %v %v", d.Name, p, ok)
		}
	}
	v := recipe.DefOf(recipe.MakeValveHeart)
	if v.Output != item.ValveHeart || v.Input != (recipe.Input{Kind: item.Steel, Qty: 25}) || v.MinSkill != 5 {
		t.Errorf("valve heart = %+v", v)
	}
	if p, ok := pr.Locks("recipe:" + v.Name); !ok || p != research.Prosthetics {
		t.Errorf("valve heart locked by %v %v", p, ok)
	}

	d := research.DefOf(research.ServoReplacements)
	if len(d.Requires) != 2 || d.Requires[0] != research.Prosthetics || d.Requires[1] != research.Fabrication {
		t.Fatalf("servo replacements requires %v", d.Requires)
	}
	if pr.Available(research.ServoReplacements) {
		t.Fatal("servo replacements needs its prerequisites")
	}
	for _, p := range []research.Project{research.Herbalism, research.Prosthetics, research.Traps, research.Gunsmithing} {
		pr.Add(p, research.DefOf(p).Cost)
	}
	if pr.Available(research.ServoReplacements) {
		t.Fatal("servo replacements needs fabrication too")
	}
	pr.Add(research.Fabrication, research.DefOf(research.Fabrication).Cost)
	if !pr.Available(research.ServoReplacements) || !pr.Add(research.ServoReplacements, d.Cost) {
		t.Fatal("servo replacements researchable once both are done")
	}
	if _, ok := pr.Locks("recipe:" + recipe.DefOf(recipe.MakePumpHeart).Name); ok {
		t.Fatal("researched: the pump heart is unlocked")
	}
	for _, r := range recipe.All() {
		if r.Output == item.NanomedKit || r.Output.IsOrgan() {
			t.Errorf("%s makes %v", r.Name, r.Output)
		}
	}
}
