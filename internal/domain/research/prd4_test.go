package research

import (
	"testing"

	"rimworld-go/internal/domain/economy/recipe"
)

// TestPRD4Gates: the wave E recipes and the shelf sit behind the wiki's
// research (original project names); the free ones need none; every
// recipe key a project names is a real recipe.
func TestPRD4Gates(t *testing.T) {
	t.Parallel()
	var pr Progress
	gated := map[string]Project{
		"build:shelf": Upholstery, "recipe:cook_pemmican": TrailRations, "recipe:cook_survival_meal": SealedRations,
		"recipe:make_ash_flake": LeafRefining, "recipe:make_silverdust": LeafRefining, "recipe:make_battle_tonic": TonicSynthesis,
		"recipe:make_reagent": TonicSynthesis, "recipe:fabricate_advanced": PrecisionFabrication,
		"recipe:make_flak_vest": FlakArmor, "recipe:make_flak_jacket": FlakArmor, "recipe:make_plate_armor": Armoursmithing,
		"recipe:make_marine_armor": PoweredArmor, "recipe:make_marine_helmet": PoweredArmor,
		"recipe:make_simple_helmet": Metalworking, "recipe:make_advanced_helmet": RifledBarrels,
		"recipe:make_he_shells": Artillery, "recipe:make_emp_shells": Microcircuits,
	}
	for key, want := range gated {
		if p, ok := pr.Locks(key); !ok || p != want {
			t.Errorf("%s: locked by %v %v, want %v", key, p, ok, want)
		}
	}
	for _, id := range []recipe.ID{recipe.CookFineMeal, recipe.CookLavishMeal, recipe.MakeKibble, recipe.BrewWort,
		recipe.FillBarrel, recipe.RefineChemfuelWood, recipe.MakeFragGrenades, recipe.SmeltApparel} {
		if p, ok := pr.Locks("recipe:" + recipe.DefOf(id).Name); ok {
			t.Errorf("%s needs no research beyond its bench, locked by %v", recipe.DefOf(id).Name, p)
		}
	}
	names := map[string]bool{}
	for _, d := range recipe.All() {
		names["recipe:"+d.Name] = true
	}
	for _, d := range All() {
		for _, u := range d.Unlocks {
			if len(u) > 7 && u[:7] == "recipe:" && !names[u] {
				t.Errorf("%s unlocks unknown %s", d.Name, u)
			}
		}
		for _, r := range d.Requires {
			if r >= d.Project {
				t.Errorf("%s requires a later project %v", d.Name, r)
			}
		}
	}
}
