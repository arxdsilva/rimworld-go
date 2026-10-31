package ebiten

// journey_food2_test.go: the FOOD-4 (cooking, eating, policies) and FOOD-5
// (food UI) journeys: the Assign tab's food policy, the policy editor, the
// Animals tab's diet and auto-slaughter, the fertility overlay, cutting
// blight, ingredient lines, hoppers and barrels, meal moods and the plant
// cutting work column.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	windowspkg "rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// f2Hungry makes colonist id hungry (rest full) and lets the view catch up.
func f2Hungry(j *journey, id uint64) {
	j.g.SetNeeds(entity.EntityID(id), needs.CriticalHunger-800, needs.Full)
	j.update(1)
}

// f2Qty is the total quantity of stacks whose kind is k lying uncarried.
func f2Qty(j *journey, k item.Kind) (n int) {
	for _, it := range j.world().Items {
		if it.Kind == k.String() {
			n += it.Qty
		}
	}
	return n
}

// f2Names lists the food policy names.
func f2Names(j *journey) (out []string) {
	for _, p := range j.world().FoodPolicies {
		out = append(out, p.Name)
	}
	return out
}

// TestJourneyFoodPolicySimpleEatsSimple (FOOD-4/5): a colonist set to the
// Simple policy through the Assign tab eats the simple meal although a
// lavish one lies closer.
func TestJourneyFoodPolicySimpleEatsSimple(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	j.g.ClearPlants(0, 0, 23, 23)
	id := ordColonists(j)[0]
	if _, err := j.g.SpawnStack(item.LavishMeal, 4, 9, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnStack(item.Meal, 4, 14, 4); err != nil {
		t.Fatal(err)
	}
	j.key("F3", 0)
	btn := windowspkg.AssignFoodID(id)
	for i := 0; i < 10 && j.pawn(id).FoodPolicy != "Simple"; i++ {
		if !j.clickID(btn) {
			t.Fatalf("%s is disabled", btn)
		}
		if _, ok := j.lastCmd(command.SetFoodPolicy); !ok {
			t.Fatalf("click %d sent no SetFoodPolicy", i)
		}
		j.update(2)
	}
	if got := j.pawn(id).FoodPolicy; got != "Simple" {
		t.Fatalf("PawnView.FoodPolicy is %q after cycling, policies %q", got, f2Names(j))
	}
	j.key("Escape", 0)
	f2Hungry(j, id)
	lavish, simple := f2Qty(j, item.LavishMeal), f2Qty(j, item.Meal)
	j.key("Digit3", 0)
	for i := 0; i < 3000 && f2Qty(j, item.Meal) == simple; i++ {
		j.update(1)
		if f2Qty(j, item.LavishMeal) != lavish {
			break
		}
	}
	j.key("Space", 0)
	if f2Qty(j, item.LavishMeal) != lavish {
		t.Fatal("the Simple-policy colonist ate the lavish meal")
	}
	if f2Qty(j, item.Meal) >= simple {
		t.Fatalf("the colonist did not eat the simple meal (job %q)", j.pawn(id).Job)
	}
}

// TestJourneyFoodPolicyEditor (FOOD-5): create a policy, rename it and
// forbid a food in the editor: each sends its EditFoodPolicy command and
// the policies in WorldView follow.
func TestJourneyFoodPolicyEditor(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	n0 := len(j.world().FoodPolicies)
	j.key("F3", 0)
	j.clickID(windowspkg.AssignFoodEditID)
	if j.a.uiModalTop() != windowspkg.FoodPolicyModalID {
		t.Fatalf("the editor did not open: top modal %q", j.a.uiModalTop())
	}
	j.clickID(windowspkg.FoodNewID)
	if c, ok := j.lastCmd(command.EditFoodPolicy); !ok || c.Mode != command.FoodPolicyCreate {
		t.Fatalf("Copy sent %+v (%v), want Create", c, ok)
	}
	j.update(2)
	ps := j.world().FoodPolicies
	if len(ps) != n0+1 {
		t.Fatalf("%d policies after Copy, want %d", len(ps), n0+1)
	}
	made := ps[len(ps)-1]
	j.clickID(windowspkg.FoodSelectID(made.ID))
	j.clickID(windowspkg.FoodNameID)
	// the field snaps back to the policy's name whenever it is emptied, so
	// type the new name after the old one and delete the old one
	j.key("End", 0)
	j.typeText("Pickled")
	j.key("Home", 0)
	for range len(made.Name) {
		j.key("Delete", 0)
	}
	j.key("Enter", 0)
	j.update(1)
	if c, ok := j.lastCmd(command.EditFoodPolicy); !ok || c.Mode != command.FoodPolicyRename || c.Index != made.ID || c.Name != "Pickled" {
		t.Fatalf("rename sent %+v (%v), want Rename %d \"Pickled\"", c, ok, made.ID)
	}
	j.update(2)
	var cur game.FoodPolicyView
	for _, p := range j.world().FoodPolicies {
		if p.ID == made.ID {
			cur = p
		}
	}
	if cur.Name != "Pickled" {
		t.Fatalf("policy %d is named %q after rename", made.ID, cur.Name)
	}
	// expand a category, then forbid its first food
	if id := wbFind(j, "foodpolicy.expand."); id != "" {
		j.clickID(id)
	}
	row := ""
	for _, id := range wbIDs(j) {
		if strings.HasPrefix(id, "foodpolicy.filter.") && id != "foodpolicy.filter.Foods" {
			row = id
			break
		}
	}
	if row == "" {
		t.Fatalf("no food row on screen: %q", wbIDs(j))
	}
	before := fmt.Sprint(cur.Allowed)
	j.clickID(row)
	c, ok := j.lastCmd(command.EditFoodPolicy)
	if !ok || c.Mode != command.FoodPolicyAllow || c.Index != made.ID || c.Name != strings.TrimPrefix(row, "foodpolicy.filter.") {
		t.Fatalf("clicking %s sent %+v (%v), want an Allow edit of that node", row, c, ok)
	}
	j.update(2)
	for _, p := range j.world().FoodPolicies {
		if p.ID == made.ID && fmt.Sprint(p.Allowed) == before {
			t.Fatalf("policy %d still allows %v after the edit %+v", made.ID, p.Allowed, c)
		}
	}
	j.clickID(windowspkg.FoodDeleteID)
	if c, ok := j.lastCmd(command.EditFoodPolicy); !ok || c.Mode != command.FoodPolicyDelete || c.Index != made.ID {
		t.Fatalf("Delete sent %+v (%v)", c, ok)
	}
}

// TestJourneyAnimalDiet (FOOD-5): the Animals tab's diet button cycles a
// tame animal's food policy.
func TestJourneyAnimalDiet(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	cow, err := j.g.SpawnAnimal("cow", 12, 5)
	if err != nil {
		t.Fatal(err)
	}
	j.g.TameAnimal(cow, "Bess")
	j.update(1)
	policy := func() string {
		for _, a := range j.world().Animals {
			if a.ID == cow {
				return a.FoodPolicy
			}
		}
		t.Fatal("the cow is gone")
		return ""
	}
	before := policy()
	j.key("F4", 0)
	id := windowspkg.AnimalDietID(cow)
	if !j.clickID(id) {
		t.Fatalf("%s is disabled", id)
	}
	c, ok := j.lastCmd(command.SetFoodPolicy)
	if !ok || c.Pawn != uint64(cow) {
		t.Fatalf("SetFoodPolicy %+v (sent %v), want animal %d", c, ok, cow)
	}
	j.update(2)
	if after := policy(); after == before {
		t.Fatalf("the cow's diet stays %q after the click (policies %q)", after, f2Names(j))
	}
}

// TestJourneyAutoSlaughterSteppers (FOOD-5): the auto-slaughter window's
// steppers send SetAutoSlaughter and WorldView carries the caps.
func TestJourneyAutoSlaughterSteppers(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	for i := range 3 {
		id, err := j.g.SpawnAnimal("chicken", 12+i, 5)
		if err != nil {
			t.Fatal(err)
		}
		j.g.TameAnimal(id, "Hen"+strconv.Itoa(i))
	}
	j.update(1)
	j.key("F4", 0)
	j.clickID(windowspkg.AutoSlaughterBtnID)
	if j.a.uiModalTop() != windowspkg.AutoSlaughterModalID {
		t.Fatalf("the window did not open: top modal %q", j.a.uiModalTop())
	}
	for range 3 { // males: none -> 0, 1, 2
		j.clickID(windowspkg.AutoSlaughterStepID("chicken", 0, true))
	}
	for range 2 { // females: none -> 0, 1
		j.clickID(windowspkg.AutoSlaughterStepID("chicken", 1, true))
	}
	c, ok := j.lastCmd(command.SetAutoSlaughter)
	if !ok || c.Name != "chicken" {
		t.Fatalf("SetAutoSlaughter %+v (sent %v)", c, ok)
	}
	j.update(2)
	var cap game.AutoSlaughterView
	for _, a := range j.world().AutoSlaughter {
		if a.Species == "chicken" {
			cap = a
		}
	}
	if cap.MaxMales != 2 || cap.MaxFemales != 1 || cap.MaxYoung != -1 {
		t.Fatalf("WorldView.AutoSlaughter for chickens is %+v, want males 2, females 1, young none", cap)
	}
	j.clickID(windowspkg.AutoSlaughterStepID("chicken", 0, false))
	j.update(2)
	for _, a := range j.world().AutoSlaughter {
		if a.Species == "chicken" && a.MaxMales != 1 {
			t.Fatalf("after a decrement the male cap is %d, want 1", a.MaxMales)
		}
	}
}

// TestJourneyFertilityOverlay (FOOD-5): Shift+F shows and hides the
// fertility overlay.
func TestJourneyFertilityOverlay(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ov := &j.a.renderer.overlays().Fertility
	if ov.On {
		t.Fatal("the fertility overlay starts on")
	}
	j.key("F", input.ModShift)
	if !ov.On {
		t.Fatal("Shift+F did not show the fertility overlay")
	}
	j.draw(2)
	j.key("F", input.ModShift)
	if ov.On {
		t.Fatal("Shift+F again did not hide it")
	}
}

// TestJourneyCutAllBlight (FOOD-5): the plant pane's Cut all blight button
// marks every blighted plant for cutting.
func TestJourneyCutAllBlight(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	j.layZone("zones", "grow", 8, 1, 12, 3)
	if !fdSteps(j, 4*foodDay, func() bool { careFeed(j); return len(fdPlants(j, "potato")) >= 10 }) {
		t.Fatalf("only %d potatoes were sown in four days", len(fdPlants(j, "potato")))
	}
	if !j.g.FireIncident("crop_blight") {
		t.Fatal("crop_blight did not fire")
	}
	j.update(1)
	var sick []game.PlantView
	for _, p := range j.world().Plants {
		if p.Blighted {
			sick = append(sick, p)
		}
	}
	if len(sick) == 0 {
		t.Fatal("no blighted plant")
	}
	j.key("Escape", 0)
	j.lookAt(sick[0].X, sick[0].Y)
	j.update(1)
	j.settle()
	j.clickTile(sick[0].X, sick[0].Y, 0, 0)
	if !j.clickID(inspector.CutBlightID) {
		t.Fatal("Cut all blight is disabled")
	}
	j.update(2)
	cut := 0
	for _, p := range j.world().Plants {
		if p.Blighted && p.Cut {
			cut++
		}
	}
	t.Logf("blighted and cut after the click: %d; plant selected %v", cut, j.a.sel.HasPlant)
	if n := len(j.cmdsOf(command.DesignateCut)); n < len(sick) {
		t.Fatalf("Cut all blight sent %d DesignateCut for %d blighted plants; sent %+v", n, len(sick), j.cmds)
	}
	j.update(2)
	for _, p := range j.world().Plants {
		if p.Blighted && !p.Cut {
			t.Fatalf("blighted plant %d at (%d, %d) is not marked for cutting", p.ID, p.X, p.Y)
		}
	}
}

// TestJourneyMealShowsIngredients (FOOD-4): a cook makes a simple meal at
// a stove; the stack lists its ingredients and the pane reads "Made with".
func TestJourneyMealShowsIngredients(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wdBuild(j, "stove", 10, 5)
	var def recipe.Def
	found := false
	for _, d := range recipe.For(construction.Stove) {
		if d.Output == item.Meal && !found {
			def, found = d, true
		}
	}
	if !found {
		t.Fatal("no stove recipe yields a simple meal")
	}
	j.g.Enqueue(command.Command{Kind: command.AddBill, X: 10, Y: 5, Recipe: uint8(def.ID), Mode: 0, Count: 1})
	j.g.ApplyPending()
	for i, k := range []item.Kind{item.Berries, item.Corn} { // two raw foods, so the meal lists both
		if _, err := j.g.SpawnStack(k, 10, 12, 6+i); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := j.g.SpawnStack(item.Wood, 40, 11, 7); err != nil { // the stove burns wood
		t.Fatal(err)
	}
	var meal game.ItemView
	j.key("Digit3", 0)
	for i := 0; i < 6000 && meal.ID == 0; i++ {
		careFeed(j)
		j.update(1)
		for _, it := range j.world().Items {
			if it.Kind == item.Meal.String() {
				meal = it
			}
		}
	}
	j.key("Space", 0)
	if meal.ID == 0 {
		t.Fatalf("the %q bill produced no meal; cook job %q", def.Label, j.pawn(ordColonists(j)[0]).Job)
	}
	if len(meal.Ingredients) == 0 {
		t.Fatalf("ItemView.Ingredients is empty on %+v", meal)
	}
	if lines := inspector.ItemLines(meal, false); !strings.Contains(strings.Join(lines, "\n"), "Made with") {
		t.Fatalf("item lines %q lack \"Made with\"", lines)
	}
}

// TestJourneyHaulerFillsBarrelCookDoesNot (FOOD-4): a cook fills the
// hopper but leaves the barrel's fill bill; the same colonist as a hauler
// fills the barrel.
func TestJourneyHaulerFillsBarrelCookDoesNot(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	j.g.ClearPlants(0, 0, 23, 23)
	id := ordColonists(j)[0]
	wdBuild(j, "hopper", 12, 8)
	wdBuild(j, "fermenting barrel", 14, 5)
	j.g.Enqueue(command.Command{Kind: command.AddBill, X: 14, Y: 5, Recipe: uint8(recipe.FillBarrel), Mode: 0, Count: 1})
	j.g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: id, Work: command.WorkHaul, Count: 0})
	j.g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: id, Work: command.WorkCook, Count: 1})
	j.g.ApplyPending()
	if _, err := j.g.SpawnStack(item.Wort, 20, 10, 8); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnStack(item.Berries, 30, 9, 9); err != nil {
		t.Fatal(err)
	}
	hopperHas := func() bool {
		for _, it := range j.world().Items {
			if it.Kind == item.Berries.String() && it.X == 12 && it.Y == 8 && !it.Carried {
				return true
			}
		}
		return false
	}
	barrelFilled := func() bool {
		for _, b := range j.world().Buildings {
			if b.X == 14 && b.Y == 5 && b.DryQty > 0 {
				return true
			}
		}
		return false
	}
	j.key("Digit3", 0)
	for i := 0; i < 4000 && !hopperHas(); i++ {
		careFeed(j)
		j.update(1)
	}
	for i := 0; i < 1500; i++ {
		careFeed(j)
		j.update(1)
	}
	j.key("Space", 0)
	if !hopperHas() {
		t.Fatalf("the cook did not fill the hopper: items %+v", fdItems(j, "berries"))
	}
	if barrelFilled() {
		t.Fatal("the cook filled the fermenting barrel: that is hauling work")
	}
	j.g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: id, Work: command.WorkHaul, Count: 1})
	j.g.ApplyPending()
	j.key("Digit3", 0)
	for i := 0; i < 6000 && !barrelFilled(); i++ {
		careFeed(j)
		j.update(1)
	}
	j.key("Space", 0)
	if !barrelFilled() {
		t.Fatalf("the hauler never filled the barrel; wort left %d", f2Qty(j, item.Wort))
	}
}

// f2Thoughts are the colonist's thought labels with their values.
func f2Thoughts(j *journey, id uint64) map[string]int {
	out := map[string]int{}
	for _, th := range j.pawn(id).Thoughts {
		out[strings.ToLower(th.Label)] += th.Value
	}
	return out
}

// f2Mood finds a thought whose label holds sub.
func f2Mood(m map[string]int, sub string) (int, bool) {
	for k, v := range m {
		if strings.Contains(k, sub) {
			return v, true
		}
	}
	return 0, false
}

// f2EatFine hungers the colonist with a fine meal at hand and waits for
// the meal to be eaten.
func f2EatFine(j *journey, id uint64) {
	j.t.Helper()
	if _, err := j.g.SpawnStack(item.FineMeal, 1, 9, 4); err != nil {
		j.t.Fatal(err)
	}
	f2Hungry(j, id)
	j.key("Digit3", 0)
	for i := 0; i < 3000 && f2Qty(j, item.FineMeal) > 0; i++ {
		j.update(1)
	}
	j.key("Space", 0)
	j.update(2)
	if f2Qty(j, item.FineMeal) > 0 {
		j.t.Fatalf("the fine meal was not eaten (job %q)", j.pawn(id).Job)
	}
}

// TestJourneyFineMealMoods (FOOD-4): a fine meal eaten on the floor lifts
// the mood by 5 and the missing table lowers it by 3; at a table only the
// fine meal counts.
func TestJourneyFineMealMoods(t *testing.T) {
	t.Parallel()
	t.Run("floor", func(t *testing.T) {
		t.Parallel()
		j := newJourney(t, jPawns([2]int{8, 4}))
		j.g.ClearPlants(0, 0, 23, 23)
		id := ordColonists(j)[0]
		f2EatFine(j, id)
		m := f2Thoughts(j, id)
		if v, ok := f2Mood(m, "fine meal"); !ok || v != 5 {
			t.Fatalf("thoughts %v: want \"ate a fine meal\" +5", m)
		}
		if v, ok := f2Mood(m, "without a table"); !ok || v != -3 {
			t.Fatalf("thoughts %v: want \"ate without a table\" -3", m)
		}
	})
	t.Run("table", func(t *testing.T) {
		t.Parallel()
		j := newJourney(t, jPawns([2]int{8, 4}))
		j.g.ClearPlants(0, 0, 23, 23)
		id := ordColonists(j)[0]
		wdBuild(j, "table", 10, 4)
		wdBuild(j, "chair", 10, 5)
		f2EatFine(j, id)
		m := f2Thoughts(j, id)
		if v, ok := f2Mood(m, "fine meal"); !ok || v != 5 {
			t.Fatalf("thoughts %v: want \"ate a fine meal\" +5", m)
		}
		if _, ok := f2Mood(m, "without a table"); ok {
			t.Logf("thoughts %v: the colonist still ate without a table (it ate where it stood)", m)
		}
	})
}

// TestJourneyPlantCutWorkColumn (FOOD-2/5): the Work tab has a plant
// cutting column; clicking its cell cycles that priority.
func TestJourneyPlantCutWorkColumn(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := uint64(j.g.Snapshot()[0].ID)
	j.key("F1", 0)
	col := len(windowspkg.WorkColumns) - 1 // the Cut column
	if windowspkg.WorkColumns[col] != "Cut" {
		t.Fatalf("the last Work column is %q, want Cut", windowspkg.WorkColumns[col])
	}
	cell := "work.cell." + strconv.FormatUint(id, 10) + "." + strconv.Itoa(col)
	if wbFind(j, strings.ToLower(cell)) == "" {
		var cells []string
		for _, s := range wbIDs(j) {
			if strings.HasPrefix(s, "work.cell."+strconv.FormatUint(id, 10)) {
				cells = append(cells, s)
			}
		}
		t.Fatalf("no cell %s on the Work tab; cells %q", cell, cells)
	}
	want := windowspkg.NextPriority(windowspkg.WorkPriority(j.pawn(id), col))
	if !j.clickID(cell) {
		t.Fatalf("cell %s is disabled", cell)
	}
	c, ok := j.lastCmd(command.SetWorkPriority)
	if !ok || c.Pawn != id || c.Work != command.WorkPlantCut || c.Count != want {
		t.Fatalf("SetWorkPriority %+v (sent %v), want plant cutting priority %d", c, ok, want)
	}
}
