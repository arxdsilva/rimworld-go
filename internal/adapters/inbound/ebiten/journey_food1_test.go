package ebiten

// journey_food1_test.go: the FOOD-1/2/3 journeys (malnutrition, rice and
// hydroponics, growth at night, slaughter and butchering, auto-slaughter,
// hunting, blight) through the real UI loop. Long waits step the colony in
// 250-tick batches (Game.Step) between UI actions. Fields and hooks that
// land with the parallel agents are read by reflection or by name lookups,
// so a missing one fails the journey with a message.

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

const foodDay = 60000 // ticks in a day

// fdSteps runs the colony in 250-tick batches until cond holds or max
// ticks pass, keeping everyone rested (not fed) when rest is set.
func fdSteps(j *journey, max int, cond func() bool) bool {
	for n := 0; n < max; n += 250 {
		if cond() {
			return true
		}
		j.g.Step(250)
	}
	return cond()
}

// fdPlants are the plant views of species (case blind) in the world.
func fdPlants(j *journey, species string) (out []game.PlantView) {
	for _, p := range j.world().Plants {
		if strings.EqualFold(p.Species, species) {
			out = append(out, p)
		}
	}
	return out
}

// fdItems are the item views whose kind holds sub (lower case).
func fdItems(j *journey, sub string) (out []game.ItemView) {
	for _, it := range j.world().Items {
		if strings.Contains(strings.ToLower(it.Kind), sub) {
			out = append(out, it)
		}
	}
	return out
}

// fdCrop picks crop in the selected grow zone's crop picker.
func fdCrop(j *journey, zoneID uint16, crop string) {
	j.t.Helper()
	known := false
	for _, c := range architect.ZoneSpecies() {
		known = known || c.Name == crop
	}
	if !known {
		j.t.Fatalf("%q is not in the zone crop picker yet (FOOD-2 G)", crop)
	}
	j.clickID("cmd.zone.crop")
	j.pick(architect.CropLabel(crop))
	want, _ := game.ZoneSpeciesID(crop)
	if c, ok := j.lastCmd(command.SetZoneCrop); !ok || c.Target != uint64(zoneID) || c.Build != want {
		j.t.Fatalf("SetZoneCrop %+v (sent %v), want %s (%d) on zone %d", c, ok, crop, want, zoneID)
	}
}

// TestJourneyMalnutritionStages (FOOD-1): a colonist with no food goes
// hungry, then ravenous, then malnourished; the malnutrition rises and the
// colonist lives through the first day of it.
func TestJourneyMalnutritionStages(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	id := ordColonists(j)[0]
	for _, it := range j.world().Items {
		t.Logf("starting item: %s x%d", it.Kind, it.Qty)
	}
	zeroAt, onset := uint64(0), uint64(0)
	last, rises := 0, 0
	start := uint64(j.g.Tick())
	for n := 0; n < 40*foodDay/250; n++ {
		j.g.Step(250)
		p := j.pawn(id)
		now := uint64(j.g.Tick())
		if p.Status != "alive" {
			t.Fatalf("the colonist died (%s) after %.2f days, malnutrition %d, hunger %d%%", p.Reason, float64(now-start)/foodDay, p.Malnutrition, p.Hunger)
		}
		if p.Hunger == 0 && zeroAt == 0 {
			zeroAt = now
		}
		if p.Malnutrition > 0 && onset == 0 {
			onset = now
		}
		if p.Malnutrition > last {
			rises++
		}
		last = max(last, p.Malnutrition)
		if onset != 0 && now-onset >= foodDay {
			break
		}
	}
	switch {
	case zeroAt == 0:
		t.Fatalf("hunger never reached empty: %d%%", j.pawn(id).Hunger)
	case onset == 0:
		t.Fatalf("no malnutrition after hunger emptied at tick %d (PawnView.Malnutrition %d)", zeroAt, j.pawn(id).Malnutrition)
	case onset < zeroAt:
		t.Fatalf("malnutrition began (tick %d) before hunger emptied (tick %d)", onset, zeroAt)
	case rises < 3:
		t.Fatalf("malnutrition rose on only %d samples, ending at %d%%", rises, last)
	}
}

// TestJourneyRiceGrowsAndHarvests (FOOD-2): a grow zone switched to rice
// through the zone pane is sown, ripens in about 5 to 6 days on soil and
// yields rice.
func TestJourneyRiceGrowsAndHarvests(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	grow := j.layZone("zones", "grow", 8, 1, 10, 2)
	fdCrop(j, grow.ID, "rice")
	sown, ripe := uint64(0), uint64(0)
	id := ordColonists(j)[0]
	fdSteps(j, 12*foodDay, func() bool {
		careFeed(j)
		now := uint64(j.g.Tick())
		for _, p := range fdPlants(j, "rice") {
			if sown == 0 {
				sown = now
			}
			if ripe == 0 && (p.Harvestable || p.Growth >= 100) {
				ripe = now
			}
		}
		_ = id
		return len(fdItems(j, "rice")) > 0
	})
	if sown == 0 {
		t.Fatal("nothing was sown with rice in 12 days")
	}
	if ripe == 0 {
		t.Fatal("the rice never ripened")
	}
	if d := float64(ripe-sown) / foodDay; d < 4.0 || d > 7.0 {
		t.Fatalf("rice ripened %.2f days after sowing, want about 5 to 6", d)
	}
	if len(fdItems(j, "rice")) == 0 {
		t.Fatal("the ripe rice was never harvested")
	}
}

// fdGrowth is the mean growth percent of the rice plants standing in the
// rectangle, and how many there are.
func fdGrowth(j *journey, x0, y0, x1, y1 int) (float64, int) {
	sum, n := 0, 0
	for _, p := range fdPlants(j, "rice") {
		if p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
			sum += p.Growth
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return float64(sum) / float64(n), n
}

// TestJourneyHydroponicsUnderSunLamp (FOOD-2): rice in a powered basin
// under a sun lamp grows faster than on soil, and dies when the power is
// cut.
func TestJourneyHydroponicsUnderSunLamp(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	wdBuild(j, "wood-fired generator", 15, 2)
	j.g.FuelCampfire(15, 2)
	for x := 8; x <= 15; x++ {
		j.g.BuildNow("power conduit", "steel", "south", x, 4)
	}
	wdBuild(j, "hydroponics basin", 10, 5)
	wdBuild(j, "sun lamp", 14, 5)
	b := wdBuilding(j, 10, 5)
	var basin game.BuildingView
	for _, v := range j.world().Buildings {
		if v.ID == b {
			basin = v
		}
	}
	rice, _ := game.ZoneSpeciesID("rice")
	if rice == 0 {
		t.Fatal("rice has no zone species id yet (FOOD-2 G)")
	}
	j.g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: basin.X, Y: basin.Y, X2: basin.X + basin.W - 1, Y2: basin.Y + basin.H - 1, Build: rice})
	j.g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: 8, Y: 12, X2: 8 + basin.W - 1, Y2: 12, Build: rice})
	j.g.ApplyPending()
	j.tick(240)
	bx0, by0, bx1, by1 := basin.X, basin.Y, basin.X+basin.W-1, basin.Y+basin.H-1
	both := func() bool {
		careFeed(j)
		_, a := fdGrowth(j, bx0, by0, bx1, by1)
		_, s := fdGrowth(j, 8, 12, 8+basin.W-1, 12)
		return a > 0 && s > 0
	}
	if !fdSteps(j, 4*foodDay, both) {
		t.Fatalf("rice was not sown in both the basin and the soil strip: basin %d, soil %d", len(fdPlants(j, "rice")), 0)
	}
	b0, _ := fdGrowth(j, bx0, by0, bx1, by1)
	s0, _ := fdGrowth(j, 8, 12, 8+basin.W-1, 12)
	fdSteps(j, foodDay, func() bool { careFeed(j); return false })
	b1, _ := fdGrowth(j, bx0, by0, bx1, by1)
	s1, _ := fdGrowth(j, 8, 12, 8+basin.W-1, 12)
	if db, ds := b1-b0, s1-s0; ds <= 0 || db < ds*1.8 {
		t.Fatalf("one day's growth: basin %+.1f points, soil %+.1f points, want the basin about 2.8 times the soil", db, ds)
	}
	// cut the power: take the generator down
	j.g.Enqueue(command.Command{Kind: command.DesignateDeconstruct, X: 15, Y: 2, X2: 16, Y2: 3})
	j.g.ApplyPending()
	dead := fdSteps(j, 4*foodDay, func() bool {
		careFeed(j)
		_, n := fdGrowth(j, bx0, by0, bx1, by1)
		return n == 0
	})
	if !dead {
		t.Fatal("the basin's rice survived four days without power")
	}
}

// TestJourneyNoGrowthAtNight (FOOD-2): a crop under a lamp stands still
// through the night but grows by day.
func TestJourneyNoGrowthAtNight(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	if !j.g.BuildNow("lamp", "", "south", 9, 8) {
		wdBuild(j, "torch lamp", 9, 8)
	}
	id, err := j.g.SpawnCrop("potato", 10, 8, 10)
	if err != nil {
		t.Fatal(err)
	}
	growth := func() int {
		for _, p := range j.world().Plants {
			if p.ID == id {
				return p.Growth
			}
		}
		t.Fatal("the crop is gone")
		return 0
	}
	run := func(day, hour int) int {
		j.g.SkipTo(day, hour)
		g0 := growth()
		fdSteps(j, 6*2500, func() bool { careFeed(j); return false })
		return growth() - g0
	}
	if day := run(2, 8); day <= 0 {
		t.Fatalf("the control crop gained %d growth over six daytime hours", day)
	}
	if night := run(3, 21); night != 0 {
		t.Fatalf("the crop under the lamp gained %d growth from 21:00 to 03:00; plants rest 19:00 to 05:00", night)
	}
}

// fdTameCow spawns a tame cow at (x, y).
func fdTameCow(j *journey, species string, x, y int) entity.EntityID {
	j.t.Helper()
	id, err := j.g.SpawnAnimal(species, x, y)
	if err != nil {
		j.t.Fatal(err)
	}
	j.g.TameAnimal(id, "Bess")
	return id
}

// fdAnimal is the animal view with that id.
func fdAnimal(j *journey, id entity.EntityID) (game.AnimalView, bool) {
	for _, a := range j.world().Animals {
		if a.ID == id {
			return a, true
		}
	}
	return game.AnimalView{}, false
}

// TestJourneySlaughterWithoutWeapon (FOOD-3): the Slaughter button on a
// tame cow sends the mark; an unarmed colonist kills it and a corpse
// remains.
func TestJourneySlaughterWithoutWeapon(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	cow := fdTameCow(j, "cow", 12, 5)
	pid := ordColonists(j)[0]
	if w := j.pawn(pid).Weapon; w != "fists" {
		t.Fatalf("the handler is armed with %q", w)
	}
	j.lookAt(12, 5)
	j.update(1)
	j.settle()
	j.clickTile(12, 5, inputframe.Left, 0)
	if !j.clickID("animal.hunt") {
		t.Fatal("the Slaughter button is disabled")
	}
	if c, ok := j.lastCmd(command.DesignateHunt); !ok {
		t.Fatalf("Slaughter sent no DesignateHunt (%+v)", c)
	}
	dead := false
	j.key("Digit3", 0)
	for i := 0; i < 6000 && !dead; i++ {
		careFeed(j)
		j.update(1)
		_, alive := fdAnimal(j, cow)
		dead = !alive
	}
	j.key("Space", 0)
	if !dead {
		t.Fatalf("the cow was not slaughtered; handler job %q", j.pawn(pid).Job)
	}
	if w := j.pawn(pid).Weapon; w != "fists" {
		t.Fatalf("the handler picked up %q to do it", w)
	}
	if len(fdItems(j, "cow")) == 0 && len(fdItems(j, "meat")) == 0 {
		t.Fatalf("no corpse or meat after the slaughter: %+v", j.world().Items)
	}
}

// TestJourneyButcherYield (FOOD-3): butchering a cow gives meat named for
// the species in a plausible quantity.
func TestJourneyButcherYield(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	for _, k := range []string{"butchering spot", "butcher table"} {
		if j.g.BuildNow(k, "wood", "south", 10, 5) {
			break
		}
	}
	j.g.Enqueue(command.Command{Kind: command.AddBill, X: 10, Y: 5, Recipe: uint8(recipe.ButcherAnimal), Mode: 0, Count: 1})
	j.g.ApplyPending()
	cow := fdTameCow(j, "cow", 12, 5)
	j.g.KillAnimal(cow)
	j.update(2)
	meat := 0
	named := false
	j.key("Digit3", 0)
	for i := 0; i < 8000 && meat == 0; i++ {
		careFeed(j)
		j.update(1)
		for _, it := range j.world().Items {
			if strings.Contains(strings.ToLower(it.Kind), "meat") {
				meat += it.Qty
				named = it.Meat == "cow"
			}
		}
	}
	j.key("Space", 0)
	switch {
	case meat == 0:
		t.Fatalf("no meat after butchering; items %+v", j.world().Items)
	case !named:
		t.Fatalf("the meat stack does not carry the cow (ItemView.Meat): %+v", fdItems(j, "meat"))
	case meat < 150 || meat > 340:
		t.Fatalf("a cow gave %d meat, want the wiki 336 scaled by butcher efficiency (75%% + 2.5%% a level) and age: about 150 to 340", meat)
	}
	t.Logf("cow meat yield %d", meat)
}

// TestJourneyAutoSlaughter (FOOD-3): with the chicken cap set, surplus
// tame chickens are marked for slaughter.
func TestJourneyAutoSlaughter(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	var ids []entity.EntityID
	for i := range 8 {
		ids = append(ids, fdTameCow(j, "chicken", 12+i%4, 6+i/4))
	}
	// the cap: through the animals window when it has a control, else the command
	if id := w2Find(j, []string{"autoslaughter"}); id != "" {
		j.clickID(id)
	} else {
		t.Log("no auto-slaughter control on screen yet: sending SetAutoSlaughter directly")
		j.g.Enqueue(command.Command{Kind: command.SetAutoSlaughter, Name: "chicken", X: 1, Y: 2, X2: -1})
		j.g.ApplyPending()
	}
	fdSteps(j, 250, func() bool { return false }) // the command applies and the view refreshes
	var cap game.AutoSlaughterView
	for _, a := range j.world().AutoSlaughter {
		if a.Species == "chicken" {
			cap = a
		}
	}
	if cap.Species == "" || cap.MaxMales != 1 || cap.MaxFemales != 2 {
		t.Fatalf("WorldView.AutoSlaughter for chickens is %+v, want caps 1 and 2", cap)
	}
	fdSteps(j, 2*foodDay, func() bool { return false }) // the check runs daily at 07:00
	marked := 0
	for _, id := range ids {
		if a, ok := fdAnimal(j, id); ok && a.Designated {
			marked++
		}
	}
	gone := len(ids) - len(j.world().Animals)
	if marked+gone < 3 {
		t.Fatalf("only %d chickens marked and %d gone of 8 with caps 1 male and 2 females", marked, gone)
	}
}

// TestJourneyHunterHaulsKill (FOOD-3): a hunter shoots a marked deer and
// carries the kill to a stockpile.
func TestJourneyHunterHaulsKill(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	id := ordColonists(j)[0]
	j.g.Arm(weID(id), item.Rifle)
	sp := j.layZone("zones", "stockpile", 16, 4, 17, 5)
	// new stockpiles refuse corpses (STO-2): allow animal corpses
	j.g.Enqueue(command.Command{Kind: command.SetStockpileFilter, Target: uint64(sp.ID), Name: "Corpses/Animal", Enabled: true})
	j.g.ApplyPending()
	deer, err := j.g.SpawnAnimal("deer", 12, 12)
	if err != nil {
		t.Fatal(err)
	}
	j.openArch("orders", "hunt")
	j.lookAt(12, 12)
	j.update(1)
	j.settle()
	j.dragTiles(inputframe.Left, 12, 12, 12, 12, 0)
	if _, ok := j.lastCmd(command.DesignateHunt); !ok {
		t.Fatal("the hunt tool sent no DesignateHunt")
	}
	j.leaveArch()
	stored, killed := false, false
	in := func(x, y int) bool { return x >= sp.MinX && x <= sp.MaxX && y >= sp.MinY && y <= sp.MaxY }
	j.key("Digit3", 0)
	for i := 0; i < 12000 && !stored; i++ {
		careFeed(j)
		j.update(1)
		if _, alive := fdAnimal(j, deer); !alive {
			killed = true
		}
		for _, it := range j.world().Items {
			k := strings.ToLower(it.Kind)
			if (strings.Contains(k, "corpse") || strings.Contains(k, "meat")) && !it.Carried && in(it.X, it.Y) {
				stored = true
			}
		}
	}
	j.key("Space", 0)
	if !killed {
		t.Fatalf("the deer was never killed; hunter job %q", j.pawn(id).Job)
	}
	if !stored {
		t.Fatalf("the kill never reached the stockpile %d..%d,%d..%d: %+v", sp.MinX, sp.MaxX, sp.MinY, sp.MaxY, fdItems(j, "corpse"))
	}
}

// TestJourneyBlight (FOOD-2): blight strikes a field, spreads to
// neighbouring plants and stops once the blighted ones are cut.
func TestJourneyBlight(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	// blight strikes zone-sown crops: lay a potato field and let the colonist sow it
	j.layZone("zones", "grow", 8, 1, 12, 3)
	if !fdSteps(j, 4*foodDay, func() bool { careFeed(j); return len(fdPlants(j, "potato")) >= 10 }) {
		t.Fatalf("only %d potatoes were sown in four days", len(fdPlants(j, "potato")))
	}
	if !j.g.FireIncident("crop_blight") {
		t.Fatal(`FireIncident("crop_blight") did not fire: it needs a healthy domesticated zone crop under 15 growing days (FOOD-2 G, incidents.go incidentFits)`)
	}
	j.update(1)
	count := func() (n int) {
		for _, p := range j.world().Plants {
			if p.Blighted {
				n++
			}
		}
		return n
	}
	first := count()
	if first == 0 {
		t.Fatal("blight struck but no PlantView.Blighted is set")
	}
	fdSteps(j, 3*foodDay, func() bool { careFeed(j); return false })
	spread := count()
	if spread <= first {
		t.Fatalf("blight did not spread: %d blighted plants at first, %d after three days", first, spread)
	}
	// cut every blighted plant with the cut tool
	j.openArch("orders", "cut")
	j.lookAt(10, 2)
	j.update(1)
	j.settle()
	j.dragTiles(inputframe.Left, 8, 1, 12, 3, 0)
	j.leaveArch()
	cleared := fdSteps(j, 4*foodDay, func() bool { careFeed(j); return count() == 0 })
	if !cleared {
		t.Fatalf("%d blighted plants remain four days after cutting", count())
	}
	left := fmt.Sprint(count())
	fdSteps(j, 2*foodDay, func() bool { careFeed(j); return false })
	if got := fmt.Sprint(count()); got != left {
		t.Fatalf("blight returned after cutting: %s -> %s", left, got)
	}
}
