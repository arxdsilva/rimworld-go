package ebiten

// journey_food3_test.go: the FOOD-6 journeys: sky events (eclipse, toxic
// fallout), the rot bar, food binges and food poisoning.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// f3Growth is the growth percent of plant id (-1: gone).
func f3Growth(j *journey, id entity.EntityID) int {
	for _, p := range j.world().Plants {
		if p.ID == id {
			return p.Growth
		}
	}
	return -1
}

// TestJourneyFoodEclipse (FOOD-6): during an eclipse the HUD shows the sky
// chip, a crop in the open stops growing while one in a sun-lamp basin
// keeps growing, and the chip clears when the event ends.
func TestJourneyFoodEclipse(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	for _, gx := range []int{15, 17, 19, 21} { // the sun lamp draws 2900 W: four 800 W generators
		wdBuild(j, "wood-fired generator", gx, 2)
	}
	fuel := func() {
		for _, gx := range []int{15, 17, 19, 21} {
			j.g.FuelCampfire(gx, 2)
		}
	}
	fuel()
	for x := 8; x <= 21; x++ {
		j.g.BuildNow("power conduit", "steel", "south", x, 4)
	}
	wdBuild(j, "hydroponics basin", 10, 5)
	wdBuild(j, "sun lamp", 14, 5)
	j.tick(240) // the generator lights
	basin := wdBuilding(j, 10, 5)
	var bv game.BuildingView
	for _, b := range j.world().Buildings {
		if b.ID == basin {
			bv = b
		}
	}
	indoor, err := j.g.SpawnCrop("rice", bv.X, bv.Y, 10)
	if err != nil {
		t.Fatal(err)
	}
	outdoor, err := j.g.SpawnCrop("rice", 4, 16, 10)
	if err != nil {
		t.Fatal(err)
	}
	j.g.SkipTo(2, 9)
	fuel()
	j.tick(240) // the lamp relights after the jump
	// the control: the basin crop grows in an ordinary morning
	c0 := f3Growth(j, indoor)
	fdSteps(j, 2*2500, func() bool { careFeed(j); fuel(); return false })
	if d := f3Growth(j, indoor) - c0; d <= 0 {
		t.Fatalf("the powered basin crop grew %d points in two ordinary morning hours (growth %d, basin %+v)", d, f3Growth(j, indoor), bv)
	}
	if !j.g.FireIncident("eclipse") {
		t.Fatal(`FireIncident("eclipse") did not fire`)
	}
	j.update(1)
	w := j.world()
	chip, ok := hud.SkyChip(w.SkyEvent, w.SkyEventEnds, uint64(j.g.Tick()))
	if !ok || !strings.HasPrefix(chip.Label, "Eclipse") || !strings.Contains(chip.Label, "h left") {
		t.Fatalf("sky chip %+v (ok %v) for event %q ending at %d", chip, ok, w.SkyEvent, w.SkyEventEnds)
	}
	j.draw(2) // the Global stack draws with the chip
	in0, out0 := f3Growth(j, indoor), f3Growth(j, outdoor)
	fdSteps(j, 2*2500, func() bool { careFeed(j); fuel(); return false })
	if j.world().SkyEvent != "eclipse" {
		t.Fatalf("the eclipse ended within two hours (%q)", j.world().SkyEvent)
	}
	if d := f3Growth(j, outdoor) - out0; d != 0 {
		t.Fatalf("the crop in the open grew %d points during the eclipse", d)
	}
	if d := f3Growth(j, indoor) - in0; d <= 0 {
		var lamp game.BuildingView
		var pv game.PlantView
		for _, b := range j.world().Buildings {
			if b.X == 14 && b.Y == 5 {
				lamp = b
			}
		}
		for _, p := range j.world().Plants {
			if p.ID == indoor {
				pv = p
			}
		}
		t.Fatalf("the sun-lamp basin crop grew %d points during the eclipse, want growth; plant %+v; lamp glow %v powered %v on %v radius %d", d, pv, lamp.Glow, lamp.Powered, lamp.PowerOn, lamp.GlowRadius)
	}
	if !fdSteps(j, 3*foodDay, func() bool { careFeed(j); return j.world().SkyEvent == "" }) {
		t.Fatal("the eclipse never ended")
	}
	w = j.world()
	if _, ok := hud.SkyChip(w.SkyEvent, w.SkyEventEnds, uint64(j.g.Tick())); ok {
		t.Fatalf("the sky chip is still up after the event: %q", w.SkyEvent)
	}
}

// TestJourneyFoodToxicFallout (FOOD-6): under toxic fallout a colonist in
// the open builds up toxic poisoning, shown on its pane, while one in a
// roofed room stays clean.
func TestJourneyFoodToxicFallout(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}, [2]int{12, 12}))
	j.g.ClearPlants(0, 0, 23, 23)
	for x := 10; x <= 14; x++ {
		for y := 10; y <= 14; y++ {
			if x == 10 || x == 14 || y == 10 || y == 14 {
				wdBuild(j, "wall", x, y)
			}
		}
	}
	cs := ordColonists(j)
	open, shut := cs[0], cs[1]
	// the closed room is roofed by the colonists' own work
	fdSteps(j, foodDay, func() bool { careFeed(j); return false })
	if p := j.pawn(shut); p.X < 11 || p.X > 13 || p.Y < 11 || p.Y > 13 {
		t.Fatalf("the colonist left the sealed room: (%d, %d)", p.X, p.Y)
	}
	if !j.g.FireIncident("toxic_fallout") {
		t.Fatal(`FireIncident("toxic_fallout") did not fire`)
	}
	j.update(1)
	if j.world().SkyEvent != "toxic fallout" {
		t.Fatalf("sky event %q", j.world().SkyEvent)
	}
	fdSteps(j, 10*2500, func() bool { careFeed(j); return j.pawn(open).ToxicBuildup > 0 })
	fdSteps(j, 2500, func() bool { careFeed(j); return false })
	p := j.pawn(open)
	if p.ToxicBuildup <= 0 {
		t.Fatalf("the colonist in the open has toxic buildup %d", p.ToxicBuildup)
	}
	lines := strings.Join(inspector.PawnStatusLines(p, func(entity.EntityID) string { return "" }), "\n")
	if !strings.Contains(lines, "Toxic buildup") {
		t.Fatalf("pane lines %q lack \"Toxic buildup\"", lines)
	}
	if b := j.pawn(shut).ToxicBuildup; b != 0 {
		t.Fatalf("the colonist in the roofed room has toxic buildup %d", b)
	}
}

// TestJourneyFoodRotBar (FOOD-6): a spoiling food stack's pane shows the
// Freshness bar.
func TestJourneyFoodRotBar(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	meal, err := j.g.SpawnStack(item.Meal, 3, 12, 6)
	if err != nil {
		t.Fatal(err)
	}
	j.lookAt(12, 6)
	j.update(1)
	j.settle()
	j.clickTile(12, 6, 0, 0)
	if !j.a.sel.HasItem || j.a.sel.SelItem != meal {
		t.Fatalf("the click did not select the meal (%v/%d)", j.a.sel.HasItem, j.a.sel.SelItem)
	}
	bars := j.a.insp.Thing().ItemPane(meal).Bars
	for _, b := range bars {
		if b.Label == "Freshness" {
			if b.Pct <= 0 || b.Pct > 100 {
				t.Fatalf("Freshness bar at %d%%", b.Pct)
			}
			return
		}
	}
	t.Fatalf("the item pane has no Freshness bar: %+v", bars)
}

// TestJourneyFoodBinge (FOOD-6): a colonist in a food binge eats one meal
// a sitting, a bounded number over the break.
func TestJourneyFoodBinge(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	j.g.ClearPlants(0, 0, 23, 23)
	id := ordColonists(j)[0]
	if _, err := j.g.SpawnStack(item.Meal, 200, 10, 4); err != nil {
		t.Fatal(err)
	}
	j.g.SetNeeds(entity.EntityID(id), needs.Full, needs.Full)
	j.g.ForceBreak(entity.EntityID(id), mentalbreak.FoodBinge)
	j.update(1)
	if mb := j.pawn(id).MentalBreak; !strings.Contains(strings.ToLower(mb), "binge") {
		t.Fatalf("PawnView.MentalBreak is %q after the forced binge", mb)
	}
	prev, sittings := f2Qty(j, item.Meal), 0
	for i := 0; i < 60000 && strings.Contains(strings.ToLower(j.pawn(id).MentalBreak), "binge"); i++ {
		j.g.Step(1)
		if q := f2Qty(j, item.Meal); q < prev {
			if prev-q > 1 {
				t.Fatalf("one sitting ate %d meals, want 1", prev-q)
			}
			sittings++
			prev = q
		}
		j.g.SetNeeds(entity.EntityID(id), needs.Full, needs.Full)
	}
	if mb := j.pawn(id).MentalBreak; mb != "" {
		t.Fatalf("still %q after 60000 ticks", mb)
	}
	most := 45000/mind.BingeGap + 2
	if sittings < 3 || sittings > most {
		t.Fatalf("the binge ate %d meals, want 3 to %d", sittings, most)
	}
}
