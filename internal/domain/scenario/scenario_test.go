package scenario

import (
	"encoding/json"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

func words(s string) int { return len(strings.Fields(s)) }

func builtin(t *testing.T, key string) Def {
	t.Helper()
	d, ok := Builtin(key)
	if !ok {
		t.Fatalf("no builtin %q", key)
	}
	return d
}

func TestBuiltinsValid(t *testing.T) {
	t.Parallel()
	bs := Builtins()
	table := []struct {
		key, name    string
		fac          faction.Type
		people, from int
		arrival      Arrival
		tech         faction.Tech
		hardness     uint8
	}{
		{"broken_hull", "Broken Hull", faction.Settlers, 3, 8, DropPods, faction.Industrial, 2},
		{"ember_clan", "Ember Clan", faction.Clans, 5, 10, Standing, faction.Neolithic, 3},
		{"gilded_expedition", "Gilded Expedition", faction.Settlers, 1, 6, DropPods, faction.Industrial, 3},
		{"bare_hands", "Bare Hands", faction.Settlers, 1, 6, DropPods, faction.Industrial, 4},
	}
	if len(bs) != len(table) {
		t.Fatalf("%d builtins, want %d", len(bs), len(table))
	}
	seeds := map[uint64]bool{}
	for i, w := range table {
		d := bs[i]
		if d.Key != w.key || d.Name != w.name || d.Faction != w.fac || d.People != w.people || d.ChooseFrom != w.from ||
			d.Arrival != w.arrival || d.Tech != w.tech || d.Hardness != w.hardness || !d.Builtin {
			t.Errorf("%s: %+v", w.key, d)
		}
		if seeds[d.Seed] || d.Seed == 0 {
			t.Errorf("%s: seed %d must be unique and set", d.Key, d.Seed)
		}
		seeds[d.Seed] = true
		for _, p := range d.Validate() {
			if p.Severity == Error {
				t.Errorf("%s: %s", d.Key, p.Msg)
			}
		}
		if d.Summary == "" || len([]rune(d.Summary)) > MaxSummary || d.Desc == "" || len([]rune(d.Desc)) > MaxDesc {
			t.Errorf("%s: summary/desc length", d.Key)
		}
		if words(d.Desc) < 40 {
			t.Errorf("%s: description is too thin", d.Key)
		}
		e := d.Apply()
		if words(e.Intro) < 40 || words(e.Intro) > 120 {
			t.Errorf("%s: intro is %d words, want 40..120", d.Key, words(e.Intro))
		}
		if d.HardnessPips() != int(w.hardness) {
			t.Errorf("%s pips", d.Key)
		}
	}
	// Only Bare Hands (no food, pods) draws a warning.
	for _, d := range bs {
		var warns []Problem
		for _, p := range d.Validate() {
			if p.Severity == Warning {
				warns = append(warns, p)
			}
		}
		if (d.Key == "bare_hands") != (len(warns) == 1) || len(warns) > 1 {
			t.Errorf("%s warnings: %v", d.Key, warns)
		}
	}
	// Builtins are fresh copies.
	bs[0].Parts[0].Qty = 1
	if Builtins()[0].Parts[0].Qty != 40 {
		t.Error("Builtins returns copies")
	}
	if _, ok := Builtin("nope"); ok {
		t.Error("unknown builtin")
	}
}

// TestKitsMatchTheSpec spot-checks the §3.1 kits through Apply.
func TestKitsMatchTheSpec(t *testing.T) {
	t.Parallel()
	qty := func(e Effects, k item.Kind) int {
		n := 0
		for _, s := range e.StartItems {
			if s.Kind == k {
				n += s.Qty
			}
		}
		return n
	}
	hull := builtin(t, "broken_hull").Apply()
	for k, want := range map[item.Kind]int{item.SurvivalMeal: 40, item.Meal: 10, // FOOD-4: survival meals, not the legacy ration
		item.IndustrialMedicine: 12, item.HerbalMedicine: 6,
		item.Components: 20, item.Steel: 300, item.Wood: 200, item.Silver: 600, item.Rifle: 2, item.Knife: 1,
		item.Parka: 1, item.Duster: 1, item.Tuque: 1} {
		if got := qty(hull, k); got != want {
			t.Errorf("broken hull %s: %d, want %d", k, got, want)
		}
	}
	if len(hull.Scatter) != 3 || hull.Scatter[0].Kind != item.Steel || hull.Scatter[0].Qty != 400 || hull.Scatter[0].Piles != 4 || !hull.Scatter[0].Near {
		t.Errorf("hull scatter: %+v", hull.Scatter)
	}
	if len(hull.Research) != 2 || hull.Research[0] != research.FoodPreservation || hull.Research[1] != research.Traps {
		t.Errorf("hull research: %v", hull.Research)
	}
	if len(hull.Animals) != 1 || !containsSpecies(PetSet(), hull.Animals[0]) {
		t.Errorf("hull pet: %v", hull.Animals)
	}
	ember := builtin(t, "ember_clan").Apply()
	for k, want := range map[item.Kind]int{item.Meat: 40, item.Berries: 60, item.HerbalMedicine: 10, item.Wood: 120,
		item.Leather: 30, item.Spear: 3, item.Club: 2, item.Knife: 2, item.Tuque: 5} {
		if got := qty(ember, k); got != want {
			t.Errorf("ember %s: %d, want %d", k, got, want)
		}
	}
	if ember.ResearchCostPct != 150 || len(ember.Research) != 0 || len(ember.Animals) != 2 {
		t.Errorf("ember: cost %d research %v animals %v", ember.ResearchCostPct, ember.Research, ember.Animals)
	}
	if ember.Rules.Tech != backstory.TagNeolithic {
		t.Error("clan backstories are neolithic")
	}
	gilded := builtin(t, "gilded_expedition").Apply()
	if qty(gilded, item.Silver) != 3000 || qty(gilded, item.Gold) != 30 || qty(gilded, item.Components) != 30 || len(gilded.Research) != 5 {
		t.Errorf("gilded: %+v", gilded)
	}
	bare := builtin(t, "bare_hands").Apply()
	if len(bare.StartItems) != 0 || !bare.Rules.NoClothes || bare.Rules.ForcedCond != pawngen.Exhausted || bare.Rules.CondChance != 100 || len(bare.Research) != 0 {
		t.Errorf("bare hands: %+v", bare)
	}
}

func containsSpecies(l []animal.Species, s animal.Species) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func TestRandomPetUsesTheScenarioSeed(t *testing.T) {
	t.Parallel()
	d := builtin(t, "broken_hull")
	a, b := d.Apply(), d.Apply()
	if a.Animals[0] != b.Animals[0] {
		t.Error("same seed, same pet")
	}
	seen := map[animal.Species]bool{}
	for seed := uint64(0); seed < 200; seed++ {
		d.Seed = seed
		pet := d.Apply().Animals[0]
		if !containsSpecies(PetSet(), pet) {
			t.Fatalf("pet %v is not in the pet set", pet)
		}
		seen[pet] = true
	}
	if len(seen) != 4 {
		t.Errorf("the seed should reach all four pets, got %v", seen)
	}
	pets := PetSet()
	pets[0] = animal.Deer
	if PetSet()[0] != animal.Cat {
		t.Error("PetSet returns a copy")
	}
}

// valid is a minimal valid scenario for the validation cases.
func valid() Def {
	return Def{Name: "Test", Faction: faction.Settlers, People: 3, ChooseFrom: 8, Arrival: DropPods, Tech: faction.Industrial,
		Parts: []Part{{Kind: PartStartItem, Item: item.Food, Qty: 10}}}
}

func TestValidateCases(t *testing.T) {
	t.Parallel()
	with := func(mod func(*Def)) Def { d := valid(); mod(&d); return d }
	addPart := func(p Part) Def { return with(func(d *Def) { d.Parts = append(d.Parts, p) }) }
	cases := []struct {
		name   string
		def    Def
		part   int
		sev    Severity
		substr string
	}{
		{"empty name", with(func(d *Def) { d.Name = "  " }), -1, Error, "needs a name"},
		{"long name", with(func(d *Def) { d.Name = strings.Repeat("n", 41) }), -1, Error, "name is 41"},
		{"long summary", with(func(d *Def) { d.Summary = strings.Repeat("s", 91) }), -1, Error, "summary"},
		{"long desc", with(func(d *Def) { d.Desc = strings.Repeat("d", 1201) }), -1, Error, "description"},
		{"people zero", with(func(d *Def) { d.People = 0 }), -1, Error, "Starting people"},
		{"people eleven", with(func(d *Def) { d.People, d.ChooseFrom = 11, 11 }), -1, Error, "Starting people"},
		{"choose from fewer", with(func(d *Def) { d.ChooseFrom = 2 }), -1, Error, "Choose from"},
		{"choose from too many", with(func(d *Def) { d.ChooseFrom = 12 }), -1, Error, "at most"},
		{"faction", with(func(d *Def) { d.Faction = faction.Freebooters }), -1, Error, "Settlers or Clan"},
		{"arrival", with(func(d *Def) { d.Arrival = 9 }), -1, Error, "arrival"},
		{"tech", with(func(d *Def) { d.Tech = 9 }), -1, Error, "tech"},
		{"hardness", with(func(d *Def) { d.Hardness = 9 }), -1, Error, "Hardness"},
		{"unknown kind", addPart(Part{Kind: NumPartKinds}), 1, Error, "Unknown part kind"},
		{"none kind", addPart(Part{}), 1, Error, "Unknown part kind"},
		{"no food", with(func(d *Def) { d.Parts = nil }), -1, Warning, "no food"},
		{"trait unknown", addPart(Part{Kind: PartForcedTrait, Trait: trait.NumTraits, Chance: 50}), 1, Error, "trait does not exist"},
		{"trait none", addPart(Part{Kind: PartForcedTrait, Chance: 50}), 1, Error, "trait does not exist"},
		{"trait chance low", addPart(Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 4}), 1, Error, "chance"},
		{"trait chance high", addPart(Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 101}), 1, Error, "chance"},
		{"trait who", addPart(Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 50, Who: 7}), 1, Error, "who"},
		{"trait n zero", addPart(Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 50, Who: WhoNRandom}), 1, Error, "N must"},
		{"trait n over people", addPart(Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 50, Who: WhoNRandom, N: 4}), 1, Error, "N must"},
		{"age min over max", addPart(Part{Kind: PartAgeRange, MinAge: 40, MaxAge: 30}), 1, Error, "above the maximum"},
		{"age too young", addPart(Part{Kind: PartAgeRange, MinAge: 10, MaxAge: 30}), 1, Error, "between 14 and 80"},
		{"age too old", addPart(Part{Kind: PartAgeRange, MinAge: 20, MaxAge: 90}), 1, Error, "between 14 and 80"},
		{"condition unknown", addPart(Part{Kind: PartForcedCondition, PawnCond: 99, Chance: 50}), 1, Error, "unknown condition"},
		{"condition none", addPart(Part{Kind: PartForcedCondition, Chance: 50}), 1, Error, "unknown condition"},
		{"condition chance", addPart(Part{Kind: PartForcedCondition, PawnCond: pawngen.Nearsighted, Chance: 1}), 1, Error, "chance"},
		{"item unknown", addPart(Part{Kind: PartStartItem, Item: 250, Qty: 5}), 1, Error, "unknown item"},
		{"item corpse", addPart(Part{Kind: PartStartItem, Item: item.CorpseHuman, Qty: 5}), 1, Error, "unknown item"},
		{"item qty zero", addPart(Part{Kind: PartStartItem, Item: item.Wood, Qty: 0}), 1, Error, "quantity"},
		{"item qty high", addPart(Part{Kind: PartStartItem, Item: item.Wood, Qty: 5001}), 1, Error, "quantity"},
		{"weapon qty", addPart(Part{Kind: PartStartItem, Item: item.Rifle, Qty: 11}), 1, Error, "quantity"},
		{"scatter unknown", addPart(Part{Kind: PartScatterItem, Item: 250, Qty: 5}), 1, Error, "unknown item"},
		{"scatter piles", addPart(Part{Kind: PartScatterItem, Item: item.Wood, Qty: 5, N: 21}), 1, Error, "piles"},
		{"animal unknown", addPart(Part{Kind: PartStartAnimal, Species: 250, N: 1}), 1, Error, "domestic or pet"},
		{"animal wild", addPart(Part{Kind: PartStartAnimal, Species: animal.TimberWolf, N: 1}), 1, Error, "domestic or pet"},
		{"animal count", addPart(Part{Kind: PartStartAnimal, Species: animal.Goat, N: 6}), 1, Error, "count"},
		{"animal count zero", addPart(Part{Kind: PartStartAnimal, Species: animal.Goat}), 1, Error, "count"},
		{"condition permanent unknown", addPart(Part{Kind: PartCondition, Cond: 99}), 1, Error, "unknown condition"},
		{"condition permanent twice", with(func(d *Def) {
			d.Parts = append(d.Parts, Part{Kind: PartCondition, Cond: StillAir}, Part{Kind: PartCondition, Cond: StillAir})
		}), 2, Error, "one of each"},
		{"incident unknown", addPart(Part{Kind: PartScheduledIncident, Incident: "meteor", FirstDay: 5}), 1, Error, "unknown incident"},
		{"incident day zero", addPart(Part{Kind: PartScheduledIncident, Incident: "raid", FirstDay: 0}), 1, Error, "first day"},
		{"incident day high", addPart(Part{Kind: PartScheduledIncident, Incident: "raid", FirstDay: 121}), 1, Error, "first day"},
		{"incident repeat", addPart(Part{Kind: PartScheduledIncident, Incident: "raid", FirstDay: 5, EveryDays: 61}), 1, Error, "every 61"},
		{"disabled unknown", addPart(Part{Kind: PartDisabledIncident, Incident: "meteor"}), 1, Error, "unknown incident"},
		{"disabled and scheduled", with(func(d *Def) {
			d.Parts = append(d.Parts, Part{Kind: PartScheduledIncident, Incident: "raid", FirstDay: 5}, Part{Kind: PartDisabledIncident, Incident: "raid"})
		}), 2, Error, "both disabled and scheduled"},
		{"building unknown", addPart(Part{Kind: PartDisallowBuilding, Building: 250}), 1, Error, "unknown building"},
		{"building wall", addPart(Part{Kind: PartDisallowBuilding, Building: construction.Wall}), 1, Warning, "harder"},
		{"building bed", addPart(Part{Kind: PartDisallowBuilding, Building: construction.Bed}), 1, Warning, "harder"},
		{"building campfire", addPart(Part{Kind: PartDisallowBuilding, Building: construction.Campfire}), 1, Warning, "harder"},
		{"research unknown", addPart(Part{Kind: PartResearch, Project: research.NumProjects}), 1, Error, "unknown research"},
		{"cost low", addPart(Part{Kind: PartResearchCost, Percent: 49}), 1, Error, "percent"},
		{"cost high", addPart(Part{Kind: PartResearchCost, Percent: 301}), 1, Error, "percent"},
		{"intro long", addPart(Part{Kind: PartIntro, Text: strings.Repeat("x", 1501)}), -1, Error, "intro text"},
		{"second forced condition", with(func(d *Def) {
			d.Parts = append(d.Parts, Part{Kind: PartForcedCondition, PawnCond: pawngen.OldFracture, Chance: 50},
				Part{Kind: PartForcedCondition, PawnCond: pawngen.HandTremor, Chance: 50})
		}), 2, Warning, "only the first"},
		{"conflicting traits", with(func(d *Def) {
			d.Parts = append(d.Parts, Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 50},
				Part{Kind: PartForcedTrait, Trait: trait.Brooding, Chance: 50})
		}), 2, Error, "conflicts with"},
		{"same trait twice", with(func(d *Def) {
			d.Parts = append(d.Parts, Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 50},
				Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 50})
		}), 2, Error, "conflicts with"},
	}
	for _, c := range cases {
		found := false
		for _, p := range c.def.Validate() {
			if p.Part == c.part && p.Severity == c.sev && strings.Contains(strings.ToLower(p.Msg), strings.ToLower(c.substr)) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: wanted part %d %v containing %q, got %+v", c.name, c.part, c.sev, c.substr, c.def.Validate())
		}
	}
	// A valid scenario has no problems; walk-in needs no food; compatible traits pass.
	if ps := valid().Validate(); len(ps) != 0 {
		t.Errorf("valid scenario: %+v", ps)
	}
	walk := valid()
	walk.Arrival, walk.Parts = WalkIn, nil
	if ps := walk.Validate(); len(ps) != 0 {
		t.Errorf("walk-in without food: %+v", ps)
	}
	ok := addPart(Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 50})
	ok.Parts = append(ok.Parts, Part{Kind: PartForcedTrait, Trait: trait.Tireless, Chance: 50, Who: WhoNRandom, N: 2})
	if HasErrors(ok.Validate()) {
		t.Errorf("compatible traits: %+v", ok.Validate())
	}
	// Problems are ordered by part and HasErrors tells error from warning.
	mixed := with(func(d *Def) {
		d.Name = ""
		d.Parts = append(d.Parts, Part{Kind: PartDisallowBuilding, Building: construction.Wall}, Part{Kind: PartResearchCost, Percent: 1})
	})
	ps := mixed.Validate()
	for i := 1; i < len(ps); i++ {
		if ps[i].Part < ps[i-1].Part {
			t.Errorf("problems out of order: %+v", ps)
		}
	}
	if !HasErrors(ps) || HasErrors([]Problem{{Severity: Warning}}) {
		t.Error("HasErrors")
	}
	if Error.String() != "error" || Warning.String() != "warning" {
		t.Error("severity strings")
	}
}

func TestApplyFlattens(t *testing.T) {
	t.Parallel()
	d := Def{
		Name: "Everything", Faction: faction.Clans, People: 4, ChooseFrom: 9, Arrival: WalkIn, Tech: faction.Spacer, Seed: 5,
		Parts: []Part{
			{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 80, Who: WhoAll},
			{Kind: PartForcedTrait, Trait: trait.Tireless, Chance: 100, Who: WhoOneRandom},
			{Kind: PartForcedTrait, Trait: trait.Steady, Chance: 60, Who: WhoNRandom, N: 2},
			{Kind: PartForcedTrait, Trait: trait.Bookish, Chance: 5, Who: WhoOneRandom},
			{Kind: PartAgeRange, MinAge: 20, MaxAge: 60},
			{Kind: PartAgeRange, MinAge: 25, MaxAge: 70},
			{Kind: PartForcedCondition, PawnCond: pawngen.Nearsighted, Chance: 30},
			{Kind: PartForcedCondition, PawnCond: pawngen.HandTremor, Chance: 90},
			{Kind: PartNoClothes},
			{Kind: PartStartItem, Item: item.Wood, Qty: 50},
			{Kind: PartScatterItem, Item: item.Steel, Qty: 90, N: 3, Near: false},
			{Kind: PartScatterItem, Item: item.Gold, Qty: 9, N: 0, Near: true},
			{Kind: PartStartAnimal, Species: animal.Cow, N: 2},
			{Kind: PartStartAnimal, Species: animal.Goat},
			{Kind: PartCondition, Cond: EndlessChill},
			{Kind: PartCondition, Cond: EndlessChill},
			{Kind: PartCondition, Cond: StillAir},
			{Kind: PartScheduledIncident, Incident: "raid", FirstDay: 8, EveryDays: 12},
			{Kind: PartDisabledIncident, Incident: "hive_swarm"},
			{Kind: PartDisallowBuilding, Building: construction.Lamp},
			{Kind: PartResearch, Project: research.Herbalism},
			{Kind: PartResearch, Project: research.Herbalism},
			{Kind: PartResearch, Project: research.Traps},
			{Kind: PartResearchCost, Percent: 150},
			{Kind: PartResearchCost, Percent: 50},
			{Kind: PartIntro, Text: "first"},
			{Kind: PartIntro, Text: "second"},
		},
	}
	e := d.Apply()
	if e.People != 4 || e.ChooseFrom != 9 || e.Arrival != WalkIn || e.Tech != faction.Spacer || e.Faction != faction.Clans {
		t.Errorf("mandatory fields: %+v", e)
	}
	wantForced := []pawngen.ForcedTrait{{Trait: trait.Sunny, Chance: 80}, {Trait: trait.Tireless, Chance: 25},
		{Trait: trait.Steady, Chance: 30}, {Trait: trait.Bookish, Chance: 1}}
	if len(e.Rules.Forced) != len(wantForced) {
		t.Fatalf("forced traits: %+v", e.Rules.Forced)
	}
	for i, w := range wantForced {
		if e.Rules.Forced[i] != w {
			t.Errorf("forced %d: %+v, want %+v", i, e.Rules.Forced[i], w)
		}
	}
	if e.Rules.MinAge != 25 || e.Rules.MaxAge != 60 {
		t.Errorf("age ranges intersect: %d..%d", e.Rules.MinAge, e.Rules.MaxAge)
	}
	if e.Rules.ForcedCond != pawngen.Nearsighted || e.Rules.CondChance != 30 || !e.Rules.NoClothes || e.Rules.Tech != backstory.TagSpacer {
		t.Errorf("rules: %+v", e.Rules)
	}
	if len(e.StartItems) != 1 || e.StartItems[0] != (Stack{Kind: item.Wood, Qty: 50, Near: true, Piles: 1}) {
		t.Errorf("start items: %+v", e.StartItems)
	}
	if len(e.Scatter) != 2 || e.Scatter[0] != (Stack{Kind: item.Steel, Qty: 90, Near: false, Piles: 3}) || e.Scatter[1].Piles != 1 {
		t.Errorf("scatter: %+v", e.Scatter)
	}
	if len(e.Animals) != 3 || e.Animals[0] != animal.Cow || e.Animals[1] != animal.Cow || e.Animals[2] != animal.Goat {
		t.Errorf("animals: %v", e.Animals)
	}
	if len(e.Conditions) != 2 || e.Conditions[0] != EndlessChill || e.Conditions[1] != StillAir {
		t.Errorf("conditions: %v", e.Conditions)
	}
	if len(e.Scheduled) != 1 || e.Scheduled[0] != (Sched{Incident: "raid", FirstDay: 8, EveryDays: 12}) {
		t.Errorf("scheduled: %v", e.Scheduled)
	}
	if !e.Disabled["hive_swarm"] || len(e.Disabled) != 1 || !e.Disallowed[construction.Lamp] || len(e.Disallowed) != 1 {
		t.Errorf("disabled %v disallowed %v", e.Disabled, e.Disallowed)
	}
	if len(e.Research) != 2 || e.Research[0] != research.Herbalism || e.Research[1] != research.Traps {
		t.Errorf("research: %v", e.Research)
	}
	// Spacer 80% x 150% x 50%.
	if e.ResearchCostPct != 60 {
		t.Errorf("cost pct %d, want 60", e.ResearchCostPct)
	}
	if e.Intro != "first" {
		t.Errorf("intro %q", e.Intro)
	}
	// The empty scenario still has usable maps and the tech cost.
	z := (Def{}).Apply()
	if z.Disabled == nil || z.Disallowed == nil || z.ResearchCostPct != 150 {
		t.Errorf("zero def: %+v", z)
	}
	for tech, want := range map[faction.Tech]int{faction.Neolithic: 150, faction.Industrial: 100, faction.Spacer: 80} {
		if TechCostPct(tech) != want {
			t.Errorf("tech %d cost %d", tech, TechCostPct(tech))
		}
	}
	// A forced trait never exceeds what its chance allows.
	if forcedChance(Part{Chance: 100, Who: WhoNRandom, N: 9}, 3) != 100 {
		t.Error("N is capped at the number of people")
	}
	if forcedChance(Part{Chance: 70}, 0) != 70 {
		t.Error("zero people leaves the chance alone")
	}
}

func TestHardnessEstimate(t *testing.T) {
	t.Parallel()
	d := Def{People: 5, Tech: faction.Industrial}
	if d.HardnessPips() != 1 {
		t.Errorf("%d", d.HardnessPips())
	}
	d.People, d.Tech = 1, faction.Neolithic
	d.Parts = []Part{{Kind: PartNoClothes}}
	if d.HardnessPips() != 4 {
		t.Errorf("%d", d.HardnessPips())
	}
}

func TestCatalog(t *testing.T) {
	t.Parallel()
	cat := Catalog()
	names := []string{"Starting people", "Items", "Conditions", "Incidents", "Restrictions", "Presentation"}
	if len(cat) != len(names) {
		t.Fatalf("%d categories", len(cat))
	}
	seen := map[PartKind]bool{}
	for i, c := range cat {
		if c.Name != names[i] || len(c.Kinds) == 0 {
			t.Errorf("category %d: %+v", i, c)
		}
		for _, k := range c.Kinds {
			if k.Label == "" || k.Desc == "" || k.Label != k.Kind.Label() || seen[k.Kind] {
				t.Errorf("kind %+v", k)
			}
			seen[k.Kind] = true
		}
	}
	if len(seen) != int(NumPartKinds)-1 {
		t.Errorf("catalog lists %d kinds, want %d", len(seen), NumPartKinds-1)
	}
	for k := PartKind(1); k < NumPartKinds; k++ {
		if !seen[k] || k.Key() == "" || k.Label() == "" || k.Desc() == "" {
			t.Errorf("kind %d is missing from the catalog or unnamed", k)
		}
		if got, ok := ParsePartKind(k.Key()); !ok || got != k {
			t.Errorf("ParsePartKind(%q)", k.Key())
		}
		// NewPart makes a valid, summarisable part of every kind.
		d := valid()
		d.Parts = append(d.Parts, NewPart(k))
		if HasErrors(d.Validate()) {
			t.Errorf("NewPart(%s) is invalid: %+v", k.Label(), d.Validate())
		}
		if k != PartNoClothes && k != PartIntro && NewPart(k).Summary() == "" {
			t.Errorf("NewPart(%s) has no summary", k.Label())
		}
	}
	if PartNone.Label() != "" || PartNone.Key() != "" || PartNone.Desc() != "" || NumPartKinds.Label() != "" {
		t.Error("none and out-of-range kinds are unnamed")
	}
	if _, ok := ParsePartKind("zap"); ok {
		t.Error("unknown kind key")
	}
	if len(Conditions()) != 4 || StillAir.Label() != "Still air" || StillAir.Desc() == "" || NoCondition.Label() != "" || NoCondition.Desc() != "" || NoCondition.Key() != "" {
		t.Error("conditions")
	}
	if len(Incidents()) != 7 || len(PawnConditions()) != 4 || PawnConditionLabel(pawngen.Exhausted) != "Exhausted" || PawnConditionLabel(0) != "" {
		t.Error("incident and pawn condition lists")
	}
	if len(ItemChoices()) < 20 || len(AnimalChoices()) < 10 {
		t.Errorf("choices: %d items, %d animals", len(ItemChoices()), len(AnimalChoices()))
	}
	for _, k := range ItemChoices() {
		if k.IsCorpse() || k == item.Stone {
			t.Errorf("item choice %s", k)
		}
	}
	for _, s := range AnimalChoices() {
		if b := animal.RosterOf(s).Behaviour; b != animal.Domestic && b != animal.Pet {
			t.Errorf("animal choice %s is %v", s, b)
		}
	}
	// Summaries read well.
	for _, c := range []struct {
		p    Part
		want string
	}{
		{Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 40, Who: WhoOneRandom}, "Sunny for one random colonist, 40%"},
		{Part{Kind: PartForcedTrait, Trait: trait.Sunny, Chance: 40, Who: WhoNRandom, N: 2}, "Sunny for 2 random colonists, 40%"},
		{Part{Kind: PartScheduledIncident, Incident: "raid", FirstDay: 9, EveryDays: 7}, "Raid on day 9, every 7 days"},
		{Part{Kind: PartStartAnimal, Random: true, N: 1}, "1 random pet"},
		{Part{Kind: PartScatterItem, Item: item.Steel, Qty: 4, N: 2}, "4 x steel in 2 piles, anywhere"},
		{Part{Kind: PartIntro, Text: strings.Repeat("word ", 30)}, strings.TrimSpace(strings.Repeat("word ", 30))[:40] + "..."},
		{Part{Kind: PartNoClothes}, "Colonists start unclothed"},
	} {
		if got := c.p.Summary(); got != c.want {
			t.Errorf("summary %q, want %q", got, c.want)
		}
	}
	if (Part{}).Summary() != "" {
		t.Error("none part has no summary")
	}
}

func TestCodecRoundTrip(t *testing.T) {
	t.Parallel()
	custom := Def{
		Key: "my_start", Name: "My start", Summary: "s", Desc: "d", Seed: 99,
		Faction: faction.Clans, People: 4, ChooseFrom: 7, Arrival: Standing, Tech: faction.Spacer, Hardness: 2,
	}
	for k := PartKind(1); k < NumPartKinds; k++ {
		custom.Parts = append(custom.Parts, NewPart(k))
	}
	custom.Parts = append(custom.Parts,
		Part{Kind: PartForcedTrait, Trait: trait.Steady, Chance: 60, Who: WhoNRandom, N: 2},
		Part{Kind: PartStartAnimal, Random: true, N: 2},
		Part{Kind: PartScatterItem, Item: item.Gold, Qty: 3, N: 2, Near: false},
		Part{Kind: PartIntro, Text: "line one\n\"quoted\" <tag> & é"},
	)
	all := append(Builtins(), custom)
	for _, d := range all {
		b, err := Encode(d)
		if err != nil {
			t.Fatalf("%s: encode: %v", d.Name, err)
		}
		got, err := Decode(b)
		if err != nil {
			t.Fatalf("%s: decode: %v\n%s", d.Name, err, b)
		}
		d.Builtin = false // files never carry the built-in flag
		if !sameDef(got, d) {
			t.Errorf("%s round trip:\n got %+v\nwant %+v", d.Name, got, d)
		}
		again, _ := Encode(got)
		if string(again) != string(b) {
			t.Errorf("%s: re-encoding differs", d.Name)
		}
	}
	b, _ := Encode(custom)
	text := string(b)
	for _, want := range []string{`"version": 1`, `"trait": "sunny"`, `"item": "packaged survival meal"`, `"species": "goat"`,
		`"building": "horseshoe pin"`, `"project": "traps"`, `"condition": "ashfall_sky"`, `"faction": "clans"`,
		`"arrival": "standing"`, `"tech": "spacer"`, `"pawn_condition": "old_fracture"`, `"who": "n_random"`} {
		if !strings.Contains(text, want) {
			t.Errorf("file lacks %s", want)
		}
	}
	if strings.Contains(text, `"trait": 1`) || strings.Contains(text, `"item": 0`) {
		t.Error("enums are stored by key, not number")
	}
	// An envelope with exactly the documented shape.
	var env struct {
		Version int            `json:"version"`
		Def     map[string]any `json:"def"`
	}
	if err := json.Unmarshal(b, &env); err != nil || env.Version != 1 || env.Def["name"] != "My start" {
		t.Errorf("envelope: %+v %v", env, err)
	}
	// Encode refuses what it cannot name.
	bad := []Def{
		with(custom, func(d *Def) { d.Arrival = 9 }),
		with(custom, func(d *Def) { d.Tech = 9 }),
		with(custom, func(d *Def) { d.Faction = faction.NumTypes }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: NumPartKinds}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartForcedTrait, Trait: trait.NumTraits}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartForcedTrait, Trait: trait.Sunny, Who: 9}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartForcedCondition}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartStartItem, Item: 250}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartStartAnimal, Species: 250}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartCondition}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartDisallowBuilding, Building: 250}} }),
		with(custom, func(d *Def) { d.Parts = []Part{{Kind: PartResearch, Project: research.NumProjects}} }),
	}
	for i, d := range bad {
		if _, err := Encode(d); err == nil {
			t.Errorf("bad def %d should not encode", i)
		}
	}
}

func with(d Def, mod func(*Def)) Def {
	d.Parts = append([]Part(nil), d.Parts...)
	mod(&d)
	return d
}

func sameDef(a, b Def) bool {
	if a.Key != b.Key || a.Name != b.Name || a.Summary != b.Summary || a.Desc != b.Desc || a.Seed != b.Seed ||
		a.Faction != b.Faction || a.People != b.People || a.ChooseFrom != b.ChooseFrom || a.Arrival != b.Arrival ||
		a.Tech != b.Tech || a.Hardness != b.Hardness || a.Builtin != b.Builtin || len(a.Parts) != len(b.Parts) {
		return false
	}
	for i := range a.Parts {
		if a.Parts[i] != b.Parts[i] {
			return false
		}
	}
	return true
}

func TestDecodeUnknownPart(t *testing.T) {
	t.Parallel()
	file := func(parts string) string {
		return `{"version":1,"def":{"name":"X","faction":"settlers","people":2,"choose_from":4,"arrival":"drop_pods","tech":"industrial","parts":[` + parts + `]}}`
	}
	if _, err := Decode([]byte(file(`{"kind":"start_item","item":"wood","qty":5}`))); err != nil {
		t.Fatalf("baseline file should decode: %v", err)
	}
	cases := []struct {
		name, in, want string
	}{
		{"unknown part kind", file(`{"kind":"meteor_shower"}`), `"meteor_shower"`},
		{"unknown item", file(`{"kind":"start_item","item":"unobtainium","qty":5}`), `"unobtainium"`},
		{"unknown trait", file(`{"kind":"forced_trait","trait":"grumpy","chance":50}`), `"grumpy"`},
		{"unknown who", file(`{"kind":"forced_trait","trait":"sunny","chance":50,"who":"some"}`), `"some"`},
		{"unknown species", file(`{"kind":"start_animal","species":"unicorn","n":1}`), `"unicorn"`},
		{"unknown building", file(`{"kind":"disallow_building","building":"castle"}`), `"castle"`},
		{"unknown project", file(`{"kind":"research","project":"alchemy"}`), `"alchemy"`},
		{"unknown condition", file(`{"kind":"condition","condition":"eclipse"}`), `"eclipse"`},
		{"unknown pawn condition", file(`{"kind":"forced_condition","pawn_condition":"gout","chance":50}`), `"gout"`},
		{"unknown incident", file(`{"kind":"scheduled_incident","incident":"flood","first_day":4}`), `"flood"`},
		{"unknown incident disabled", file(`{"kind":"disabled_incident","incident":"flood"}`), `"flood"`},
		{"field of another kind", file(`{"kind":"start_item","item":"wood","qty":5,"trait":"sunny"}`), `[trait]`},
		{"typo in a part", file(`{"kind":"start_item","item":"wood","quantity":5}`), `[quantity]`},
		{"two extras are named", file(`{"kind":"no_clothes","b":1,"a":2}`), `[a b]`},
		{"missing kind", file(`{"item":"wood"}`), `kind`},
		{"unknown def field", `{"version":1,"def":{"name":"X","faction":"settlers","people":2,"choose_from":4,"arrival":"drop_pods","tech":"industrial","parts":[],"flavour":"x"}}`, `flavour`},
		{"unknown envelope field", `{"version":1,"def":{},"extra":1}`, `extra`},
		{"unknown version", `{"version":2,"def":{}}`, `version 2`},
		{"no def", `{"version":1}`, `def`},
		{"unknown faction", strings.Replace(file(``), `"settlers"`, `"pirates"`, 1), `"pirates"`},
		{"unknown arrival", strings.Replace(file(``), `"drop_pods"`, `"teleport"`, 1), `"teleport"`},
		{"unknown tech", strings.Replace(file(``), `"industrial"`, `"quantum"`, 1), `"quantum"`},
		{"not json", `{`, `scenario`},
		{"trailing data", file(``) + `{}`, `trailing`},
		{"part not an object", file(`7`), `part 1`},
		{"kind not a string", file(`{"kind":5}`), `kind`},
		{"number out of range", file(`{"kind":"forced_trait","trait":"sunny","chance":300}`), `chance`},
	}
	for _, c := range cases {
		_, err := Decode([]byte(c.in))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want it to mention %s", c.name, err, c.want)
		}
	}
	// The part number is reported.
	_, err := Decode([]byte(file(`{"kind":"no_clothes"},{"kind":"zzz"}`)))
	if err == nil || !strings.Contains(err.Error(), "part 2") {
		t.Errorf("part number: %v", err)
	}
}

// TestNamesAvoidBlocklist is the originality lint over everything a player
// reads in the built-in scenarios (NG-1 §0.4).
func TestNamesAvoidBlocklist(t *testing.T) {
	t.Parallel()
	check := func(what, text string) {
		if p, bad := faction.Blocked(text); bad {
			t.Errorf("%s %q contains blocklisted %q", what, text, p)
		}
	}
	for _, d := range Builtins() {
		check("name", d.Name)
		check("summary", d.Summary)
		check("desc", d.Desc)
		check("intro", d.Apply().Intro)
	}
	for _, c := range Catalog() {
		check("category", c.Name)
		for _, k := range c.Kinds {
			check("part", k.Label+" "+k.Desc)
		}
	}
	for _, c := range Conditions() {
		check("condition", c.Label())
	}
}
