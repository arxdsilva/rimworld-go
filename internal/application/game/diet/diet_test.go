package diet

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// eaters is a fake Eaters: people and animals by id, the rest refused.
type eaters struct{ people, animals map[entity.EntityID]bool }

func (e eaters) Eater(id entity.EntityID) (bool, bool) {
	if e.animals[id] {
		return true, true
	}
	return false, e.people[id]
}

var fakeEaters = eaters{people: map[entity.EntityID]bool{1: true, 2: true}, animals: map[entity.EntityID]bool{9: true}}

// TestDefaultPolicies (wiki: Assign): each default allows less than the
// one before, no default allows human or insect meat but the animals',
// and the animals' allows no meal.
func TestDefaultPolicies(t *testing.T) {
	t.Parallel()
	d := Defaults()
	names := []string{"Lavish", "Fine", "Simple", "Paste", "Raw", "Nothing", "Animal feed"}
	for i, p := range d {
		if p.ID != i || p.Name != names[i] {
			t.Fatalf("policy %d: %d %q", i, p.ID, p.Name)
		}
		if i < Nothing && (p.Allow.Has(item.HumanMeat) || p.Allow.Has(item.InsectMeat)) {
			t.Errorf("%s allows human or insect meat", p.Name)
		}
		if i > 0 && i < AnimalFeed && !d[i-1].Allow.Covers(p.Allow) {
			t.Errorf("%s allows more than %s", p.Name, d[i-1].Name)
		}
	}
	cases := []struct {
		pol            int
		allow, forbids []item.Kind
	}{
		{Lavish, []item.Kind{item.LavishMeal, item.FineMeal, item.Meal, item.NutrientPasteMeal, item.Berries, item.Kibble}, nil},
		{Fine, []item.Kind{item.FineMeal, item.SurvivalMeal}, []item.Kind{item.LavishMeal}},
		{Simple, []item.Kind{item.Meal, item.Pemmican}, []item.Kind{item.FineMeal, item.SurvivalMeal}},
		{Paste, []item.Kind{item.NutrientPasteMeal, item.Pemmican, item.Potatoes}, []item.Kind{item.Meal}},
		{Raw, []item.Kind{item.Potatoes, item.Meat, item.Milk, item.Kibble}, []item.Kind{item.NutrientPasteMeal, item.Pemmican, item.Meal}},
		{Nothing, nil, []item.Kind{item.Berries, item.Meal}},
		{AnimalFeed, []item.Kind{item.Kibble, item.Hay, item.Meat, item.HumanMeat, item.InsectMeat}, []item.Kind{item.Meal, item.NutrientPasteMeal}},
	}
	for _, c := range cases {
		for _, k := range c.allow {
			if !d[c.pol].Allow.Has(k) {
				t.Errorf("%s forbids %s", d[c.pol].Name, k)
			}
		}
		for _, k := range c.forbids {
			if d[c.pol].Allow.Has(k) {
				t.Errorf("%s allows %s", d[c.pol].Name, k)
			}
		}
	}
	var b Book
	if b.Of(1, false).ID != Lavish || b.Of(9, true).ID != AnimalFeed || !b.Menu(1, false).Has(item.CorpseAnimal) {
		t.Fatal("people hold Lavish, tame animals Animal feed; corpses are not a policy's")
	}
}

// TestBookCommands: assigning, copying, editing, renaming (names kept
// unique) and deleting (never the default; its users fall back).
func TestBookCommands(t *testing.T) {
	t.Parallel()
	var b Book
	apply := func(c command.Command) bool { return b.Apply(c, fakeEaters) }
	if !apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 1, Index: Simple}) || b.Of(1, false).Name != "Simple" {
		t.Fatal("assign Simple")
	}
	if apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 5, Index: Simple}) || apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 1, Index: 99}) {
		t.Fatal("no eater or no policy: nothing changes")
	}
	if !apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 9, Index: Raw}) || b.Of(9, true).ID != Raw {
		t.Fatal("an animal holds a policy")
	}
	if !apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyCreate, Index: Raw, Name: "Raw"}) {
		t.Fatal("create")
	}
	ps := b.Policies()
	made := ps[len(ps)-1]
	if made.ID != numDefaults || made.Name != "Raw 2" || made.Allow != ps[Raw].Allow {
		t.Fatalf("a copy of Raw, numbered: %+v", made)
	}
	if !apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyAllow, Index: made.ID, Name: "Foods/Meals", Enabled: true}) ||
		!b.Policies()[len(ps)-1].Allow.Has(item.LavishMeal) {
		t.Fatal("allow a category")
	}
	if !apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyAllow, Index: made.ID, Name: "lavish meal", Enabled: false}) ||
		b.Policies()[len(ps)-1].Allow.Has(item.LavishMeal) {
		t.Fatal("forbid a kind")
	}
	if apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyAllow, Index: made.ID, Name: "Weapons", Enabled: true}) {
		t.Fatal("only foods are a policy's")
	}
	if !apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyRename, Index: made.ID, Name: "  Fine "}) ||
		b.Policies()[len(ps)-1].Name != "Fine 2" {
		t.Fatalf("rename kept unique: %q", b.Policies()[len(ps)-1].Name)
	}
	apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 2, Index: made.ID})
	if apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyDelete, Index: Lavish}) {
		t.Fatal("the default stays")
	}
	if !apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyDelete, Index: made.ID}) || b.Of(2, false).ID != Lavish {
		t.Fatal("a deleted policy's users fall back to the default")
	}
	apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyDelete, Index: AnimalFeed})
	if b.Of(4, true).ID != Lavish || b.Of(9, true).ID != Raw {
		t.Fatal("Animal feed gone: unassigned animals take the default, assigned ones keep theirs")
	}
	if Defaults()[Lavish].Name != "Lavish" || len(Defaults()) != numDefaults {
		t.Fatal("editing the book left the defaults alone")
	}
}

// TestBookSaveRoundTrip: the defaults save as nil; an edited book comes
// back the same through gob.
func TestBookSaveRoundTrip(t *testing.T) {
	t.Parallel()
	var b Book
	if b.Export() != nil {
		t.Fatal("the defaults save nothing")
	}
	b.Apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 2, Index: Paste}, fakeEaters)
	b.Apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 1, Index: Raw}, fakeEaters)
	b.Apply(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyCreate, Index: -1, Name: "picky"}, fakeEaters)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(b.Export()); err != nil {
		t.Fatal(err)
	}
	var s Save
	if err := gob.NewDecoder(&buf).Decode(&s); err != nil {
		t.Fatal(err)
	}
	var c Book
	c.Import(&s)
	if !reflect.DeepEqual(b.Export(), c.Export()) || c.Of(2, false).ID != Paste || c.Policies()[numDefaults].Name != "picky" {
		t.Fatalf("round trip: %+v vs %+v", b.Export(), c.Export())
	}
	if s.Assigned[0].Eater != 1 {
		t.Fatal("assignments in id order")
	}
	c.Import(nil)
	if c.Export() != nil {
		t.Fatal("an older save: the defaults")
	}
}

// TestAteThoughts (wiki: Food, Meals, Kibble, Human meat, Insect meat):
// each meal's thoughts and values; a cannibal's flipped.
func TestAteThoughts(t *testing.T) {
	t.Parallel()
	meal := func(k item.Kind, ings ...item.Kind) item.Stack {
		st := item.Stack{Kind: k, Qty: 1}
		copy(st.Ingredients[:], ings)
		return st
	}
	cases := []struct {
		st       item.Stack
		cannibal bool
		want     int
		kinds    []mood.Kind
	}{
		{meal(item.LavishMeal), false, 12, []mood.Kind{mood.AteLavishMeal}},
		{meal(item.FineMeal), false, 5, []mood.Kind{mood.AteFineMeal}},
		{meal(item.Meal), false, 0, nil},
		{meal(item.NutrientPasteMeal), false, -4, []mood.Kind{mood.AtePaste}},
		{meal(item.Kibble), false, -12, []mood.Kind{mood.AteKibble}},
		{meal(item.Potatoes), false, -7, []mood.Kind{mood.AteRaw}},
		{meal(item.Berries), false, 0, nil},
		{meal(item.Milk), false, 0, nil},
		{meal(item.HumanMeat), false, -27, []mood.Kind{mood.AteRaw, mood.AteHumanMeat}},
		{meal(item.HumanMeat), true, 13, []mood.Kind{mood.AteRaw, mood.AteHumanMeat}},
		{meal(item.InsectMeat), false, -13, []mood.Kind{mood.AteRaw, mood.AteInsectMeat}},
		{meal(item.Meal, item.Potatoes, item.HumanMeat), false, -15, []mood.Kind{mood.AteHumanMeatCooked}},
		{meal(item.LavishMeal, item.HumanMeat, item.InsectMeat), true, 12 + 15 - 3, []mood.Kind{mood.AteLavishMeal, mood.AteHumanMeatCooked, mood.AteInsectMeatCooked}},
		{meal(item.NutrientPasteMeal, item.InsectMeat), false, -7, []mood.Kind{mood.AtePaste, mood.AteInsectMeatCooked}},
	}
	for i, c := range cases {
		ts := AppendAte(nil, c.st, c.cannibal)
		var kinds []mood.Kind
		for _, x := range ts {
			kinds = append(kinds, x.Kind)
		}
		if AteMood(c.st, c.cannibal) != c.want || !reflect.DeepEqual(kinds, c.kinds) {
			t.Errorf("case %d %s: mood %d %v, want %d %v", i, c.st.Kind, AteMood(c.st, c.cannibal), kinds, c.want, c.kinds)
		}
	}
	if ts := AppendAte(nil, meal(item.HumanMeat), true); ts[1].Label != cannibalRaw || ts[1].Value != 20 {
		t.Fatalf("a cannibal relishes it: %+v", ts)
	}
}

// TestScoreWeighsTasteAndWalk: optimality prefers the tastier food a
// fair walk further, but not any distance; kibble only in need; the
// policy and hay rule what a person takes.
func TestScoreWeighsTasteAndWalk(t *testing.T) {
	t.Parallel()
	var b Book
	p := b.Picker(1, false, false)
	st := func(k item.Kind) *item.Stack { return &item.Stack{Kind: k, Qty: 1} }
	if p.Score(st(item.LavishMeal), 60) <= p.Score(st(item.Meal), 0) {
		t.Fatal("a lavish meal is worth a walk")
	}
	if p.Score(st(item.LavishMeal), 120) >= p.Score(st(item.Meal), 0) {
		t.Fatal("but not any walk")
	}
	if p.Score(st(item.Meal), 50) <= p.Score(st(item.Potatoes), 0) || p.Score(st(item.Potatoes), 0) <= p.Score(st(item.Kibble), 0) {
		t.Fatal("a meal beats raw food nearby; raw food beats kibble")
	}
	if p.Score(st(item.NutrientPasteMeal), 0) <= p.Score(st(item.Potatoes), 0) || p.Score(st(item.Meal), 0) <= p.Score(st(item.NutrientPasteMeal), 0) {
		t.Fatal("paste between a simple meal and raw food")
	}
	rotting := &item.Stack{Kind: item.Meal, Qty: 1, Rot: item.Meal.ShelfTicks() - 1000}
	if p.Score(rotting, 0) <= p.Score(st(item.Meal), 0) {
		t.Fatal("food about to spoil goes first")
	}
	if p.Takes(st(item.Hay)) || !p.Takes(st(item.Kibble)) || p.Takes(&item.Stack{Kind: item.CorpseAnimal, Qty: 1}) {
		t.Fatal("a person never eats hay; a carcass only when desperate")
	}
	b.Apply(command.Command{Kind: command.SetFoodPolicy, Pawn: 1, Index: Nothing}, fakeEaters)
	if b.Picker(1, false, false).Takes(st(item.Meal)) || !b.Picker(1, true, false).Takes(st(item.Meal)) || !b.Picker(1, true, false).Takes(&item.Stack{Kind: item.CorpseAnimal, Qty: 1}) {
		t.Fatal("the policy holds unless malnourished")
	}
	if b.Picker(2, false, true).Score(st(item.HumanMeat), 0) <= b.Picker(2, false, false).Score(st(item.HumanMeat), 0) {
		t.Fatal("a cannibal rates human flesh higher")
	}
}

// TestPoisonChances (wiki: Ailments): the cook roll table, the kitchen
// roll's cleanliness formula and outdoor flat chance, both combined.
func TestPoisonChances(t *testing.T) {
	t.Parallel()
	want := []int{500, 400, 300, 200, 150, 100, 50, 25, 15, 10, 10, 10}
	for lvl, w := range want {
		if CookChance(lvl) != w {
			t.Errorf("cook %d: %d bp, want %d", lvl, CookChance(lvl), w)
		}
	}
	if CookChance(20) != 10 || CookChance(-1) != 500 {
		t.Error("cook ends")
	}
	for _, c := range []struct{ clean, want int }{{100, 0}, {-200, 0}, {-350, 250}, {-500, 500}, {-900, 500}} {
		if got := KitchenChance(c.clean, true); got != c.want {
			t.Errorf("cleanliness %d: %d bp, want %d", c.clean, got, c.want)
		}
	}
	if KitchenChance(-500, false) != OutdoorChance || OutdoorChance != 200 || RawChance != 200 || CorpseChance != 500 {
		t.Error("flat chances")
	}
	if MealChance(500, 500) != 975 || MealChance(10, 0) != 10 {
		t.Error("either roll poisons")
	}
	if Permille(0) != 0 || Permille(10) != 1 || Permille(3) != 1 || Permille(15) != 2 || Permille(975) != 98 {
		t.Error("permille rounding")
	}
}

// TestDays: tenths of a day of food at 1.6 nutrition a day each.
func TestDays(t *testing.T) {
	t.Parallel()
	if DailyNeed != 16000 || Days(16000*3, 2) != 15 || Days(5000, 0) != 0 || Days(0, 3) != 0 {
		t.Fatalf("days: need %d, %d", DailyNeed, Days(16000*3, 2))
	}
}
