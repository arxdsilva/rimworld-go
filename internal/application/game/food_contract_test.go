package game

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/crops"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
)

// TestFoodContractCommandsAreInert (FOOD-0): the food policy commands
// change nothing until FOOD-4 handles them: a colony that got them hashes
// as its twin that didn't (FOOD-3 handles SetAutoSlaughter:
// herd_food3_test.go).
func TestFoodContractCommandsAreInert(t *testing.T) {
	t.Parallel()
	a, b := NewColony(Config{Seed: 7, Width: 40, Height: 40}), NewColony(Config{Seed: 7, Width: 40, Height: 40})
	a.Step(30)
	b.Step(30)
	pawn := uint64(a.Snapshot()[0].ID)
	for _, c := range []command.Command{
		{Kind: command.SetFoodPolicy, Pawn: pawn, Index: 2},
		{Kind: command.EditFoodPolicy, Index: 0, Mode: command.FoodPolicyAllow, Name: "Foods/Raw food", Enabled: false},
		{Kind: command.EditFoodPolicy, Index: -1, Mode: command.FoodPolicyCreate, Name: "vegetarian"},
		{Kind: command.EditFoodPolicy, Index: 1, Mode: command.FoodPolicyRename, Name: "meat only"},
		{Kind: command.EditFoodPolicy, Index: 1, Mode: command.FoodPolicyDelete},
	} {
		a.Enqueue(c)
	}
	a.ApplyPending()
	a.Step(60)
	b.Step(60)
	if a.StateHash() != b.StateHash() {
		t.Fatal("a FOOD-0 contract command changed the state")
	}
	w := a.World()
	if w.FoodDays != 0 || w.FoodPolicies != nil {
		t.Fatalf("food views filled before FOOD-4/5: %d %v", w.FoodDays, w.FoodPolicies)
	}
	if p := a.Snapshot()[0]; p.FoodPolicy != "" || p.Malnutrition != 0 {
		t.Fatalf("pawn food views filled before FOOD-1/4: %q %d", p.FoodPolicy, p.Malnutrition)
	}
}

// TestFoodContractSpeciesStayOut (FOOD-0, flipped by FOOD-2): the new
// crops are offered in the crop menu and the zone picker now, still never
// spawn wild, and the agave fruits (wiki: Agave, 10 fruit).
func TestFoodContractSpeciesStayOut(t *testing.T) {
	t.Parallel()
	added := map[plant.Species]item.Kind{plant.Rice: item.Rice, plant.Strawberry: item.Strawberries,
		plant.Haygrass: item.Hay, plant.Hops: item.Hops}
	for s, k := range added {
		d := plant.DefOf(s)
		if !d.Crop || d.Yields != k || d.Yield <= 0 || d.GrowTicks <= 0 {
			t.Errorf("%s: crop %v yields %s x%d, grow %d", s, d.Crop, d.Yields, d.Yield, d.GrowTicks)
		}
		if got, ok := plant.Parse(d.Name); !ok || got != s {
			t.Errorf("%s does not parse", d.Name)
		}
		if !slices.Contains(plant.Crops(), s) {
			t.Errorf("%s is not offered (FOOD-2)", s)
		}
		inPicker := false
		for _, v := range crops.ZoneSpecies() {
			inPicker = inPicker || v.Name == d.Name
		}
		if !inPicker {
			t.Errorf("%s is not in the zone crop picker (FOOD-2)", s)
		}
		for _, bd := range biome.All() {
			for _, p := range bd.Plants {
				if p.Species == s {
					t.Errorf("%s spawns wild in %s", s, bd.Key)
				}
			}
		}
	}
	if d := plant.DefOf(plant.Agave); d.Yields != item.AgaveFruit || d.Yield != 10 {
		t.Errorf("agave: yields %s x%d, want agave fruit x10 (FOOD-2)", d.Yields, d.Yield)
	}
	g := NewColony(Config{Seed: 3, Width: 20, Height: 20})
	if p := g.lockedBy("crop:hops"); p != "Brewing" {
		t.Errorf("hops locked by %q, want Brewing", p)
	}
	if g.cropLocked(plant.Rice) || g.cropLocked(plant.Strawberry) || g.cropLocked(plant.Haygrass) {
		t.Error("rice, strawberries and haygrass need no research")
	}
}

// TestFoodContractBuildingsAndKinds (FOOD-0): the hydroponics basin and
// the sun lamp are research-gated steel buildings; the slaughter job and
// the food thoughts exist with their wiki values.
func TestFoodContractBuildingsAndKinds(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 3, Width: 20, Height: 20})
	if p := g.lockedBy("build:hydroponics basin"); p != "Hydroponics" {
		t.Errorf("hydroponics basin locked by %q", p)
	}
	if p := g.lockedBy("build:sun lamp"); p != "Electricity" {
		t.Errorf("sun lamp locked by %q", p)
	}
	basin := construction.DefOf(construction.HydroponicsBasin)
	if basin.Size != [2]int{1, 4} || !basin.MetalOnly || basin.Components != 1 || basin.Work != 280 || basin.Wood != 11 {
		t.Errorf("hydroponics basin %+v", basin)
	}
	if lamp := construction.DefOf(construction.SunLamp); !lamp.MetalOnly || lamp.Wood != 4 || lamp.Work != 33 {
		t.Errorf("sun lamp %+v", lamp)
	}
	if job.Slaughter.String() != "slaughter" {
		t.Errorf("slaughter job named %q", job.Slaughter)
	}
	for k, v := range map[mood.Kind]int{mood.AteFineMeal: 5, mood.AteLavishMeal: 12, mood.AteKibble: -12,
		mood.AteHumanMeat: -20, mood.ButcheredHuman: -6, mood.ObservedButchery: -6, mood.Ravenous: -12,
		mood.Malnourished: -20, mood.AteInsectMeat: -6} {
		if d := mood.DefOf(k); d.Value != v || d.Label == "" {
			t.Errorf("mood %d: %q %d, want %d", k, d.Label, d.Value, v)
		}
	}
}

// TestSameItemViewSeesEveryField: the item list's identity memo
// (item_view.go) compares every ItemView field, so a field added later
// and left out of sameItemView fails here.
func TestSameItemViewSeesEveryField(t *testing.T) {
	t.Parallel()
	var a ItemView
	if !sameItemView(a, a) {
		t.Fatal("a view differs from itself")
	}
	typ := reflect.TypeOf(a)
	for i := 0; i < typ.NumField(); i++ {
		b := a
		f := reflect.ValueOf(&b).Elem().Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString("x")
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Int, reflect.Int64:
			f.SetInt(1)
		case reflect.Uint64:
			f.SetUint(1)
		case reflect.Float64:
			f.SetFloat(1)
		case reflect.Slice:
			f.Set(reflect.Append(f, reflect.Zero(f.Type().Elem())))
		default:
			t.Fatalf("field %s: kind %s not covered by this test", typ.Field(i).Name, f.Kind())
		}
		if sameItemView(a, b) {
			t.Errorf("sameItemView misses field %s", typ.Field(i).Name)
		}
	}
}
