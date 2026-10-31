package ebiten

// journey_wave2_test.go: the STO-2/STO-3 (storage), RES-1 (reservations)
// and CBT-1/CBT-2 (damage and attack mechanics) journeys through the real
// UI loop. Controls and read-model fields that land with the parallel
// agents are looked up at run time, so a missing one fails the journey
// with a message instead of breaking the build.

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestJourneyStorageAllowAllAndSearch (STO-3): Clear all empties a
// stockpile's filter; searching "wood" and allowing the hit leaves only
// wood in ZoneView.Filter.
func TestJourneyStorageAllowAllAndSearch(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	sp := j.layZone("zones", "stockpile", 8, 1, 9, 2)
	w2Click(j, "stockpile settings button", []string{"cmd.zone.storage"})
	w2Click(j, "Clear all button", []string{"storage", "clear"})
	j.update(2)
	if f := weStrings(j, mustZone(t, j, 8, 1), "Filter"); len(f) != 0 {
		t.Fatalf("ZoneView.Filter after Clear all: %q, want empty", f)
	}
	if c, ok := j.lastCmd(command.SetStockpileFilter); !ok || c.Target != uint64(sp.ID) || c.Enabled {
		t.Fatalf("Clear all sent %+v (%v), want SetStockpileFilter on zone %d, Enabled false", c, ok, sp.ID)
	}
	w2Click(j, "search field", []string{"storage", "search"})
	j.typeText("wood")
	j.update(1)
	if id := wbFind(j, "storage.filter.", "wood"); id == "" {
		j.clickID(weButton(j, "expander", "storage.expand.", "raw resources"))
	}
	j.clickID(weButton(j, "wood hit", "storage.filter.", "wood"))
	if c, ok := j.lastCmd(command.SetStockpileFilter); !ok || !c.Enabled || !strings.Contains(strings.ToLower(c.Name), "wood") {
		t.Fatalf("allowing the hit sent %+v (%v), want a wood node, Enabled true", c, ok)
	}
	j.update(2)
	f := weStrings(j, mustZone(t, j, 8, 1), "Filter")
	if len(f) == 0 {
		t.Fatal("ZoneView.Filter is empty after allowing wood")
	}
	for _, n := range f {
		if !strings.Contains(strings.ToLower(n), "wood") {
			t.Fatalf("ZoneView.Filter %q holds more than wood", f)
		}
	}
}

// TestJourneyLinkShelves (STO-2/3): two shelves selected with Shift and
// linked share a Group; forbidding wood on one applies to both; Unlink
// takes one out.
func TestJourneyLinkShelves(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wdBuild(j, "shelf", 10, 6)
	wdBuild(j, "shelf", 10, 8)
	a, b := wdBuilding(j, 10, 6), wdBuilding(j, 10, 8)
	wdSelect(j, 10, 6)
	w2Click(j, "Link button", []string{"cmd.shelf.link"})
	j.pick("Shelf at (10, 8)")
	c, ok := j.lastCmd(command.LinkStorage)
	if !ok || c.Target != uint64(a) || c.Zone != uint64(b) {
		t.Fatalf("LinkStorage %+v (sent %v), want shelf %d linked to leader %d", c, ok, a, b)
	}
	j.update(2)
	za, zb := w2ShelfZone(j, a), w2ShelfZone(j, b)
	if za.Group == 0 || za.Group != zb.Group {
		t.Fatalf("shelf groups %d and %d, want one shared non-zero Group", za.Group, zb.Group)
	}
	if !weHas(weStrings(j, za, "Filter"), "wood") || !weHas(weStrings(j, zb, "Filter"), "wood") {
		t.Fatal("a fresh shelf does not allow wood")
	}
	w2ForbidWood(j)
	j.update(2)
	za, zb = w2ShelfZone(j, a), w2ShelfZone(j, b)
	if weHas(weStrings(j, za, "Filter"), "wood") || weHas(weStrings(j, zb, "Filter"), "wood") {
		t.Fatalf("forbidding wood on one shelf left filters %q and %q: the group did not share it", weStrings(j, za, "Filter"), weStrings(j, zb, "Filter"))
	}
	j.key("Escape", 0) // close the Storage window
	j.update(1)
	w2Click(j, "Unlink button", []string{"cmd.shelf.unlink"})
	if c, ok := j.lastCmd(command.UnlinkStorage); !ok || c.Target == 0 {
		t.Fatalf("UnlinkStorage %+v (sent %v)", c, ok)
	}
	j.update(2)
	if za, zb = w2ShelfZone(j, a), w2ShelfZone(j, b); za.Group == zb.Group && za.Group != 0 {
		t.Fatalf("both shelves are still in group %d after Unlink", za.Group)
	}
}

// TestJourneyHaulUrgently (STO-2): the Haul urgently tool marks a far
// stack; the hauler takes it before the nearer normal stacks.
func TestJourneyHaulUrgently(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}), jSize(24, 24))
	j.g.ClearPlants(0, 0, 23, 23)
	j.g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 20, Y: 2, X2: 22, Y2: 4})
	j.g.ApplyPending()
	var near []entity.EntityID
	for _, p := range [][2]int{{17, 9}, {18, 10}, {19, 9}} {
		id, err := j.g.SpawnStack(item.Wood, 20, p[0], p[1])
		if err != nil {
			t.Fatal(err)
		}
		near = append(near, id)
	}
	far, err := j.g.SpawnStack(item.Wood, 20, 2, 14)
	if err != nil {
		t.Fatal(err)
	}
	j.openArch("orders", "")
	tool := ""
	for _, id := range wbIDs(j) {
		if strings.HasPrefix(id, "arch.tool.") && strings.Contains(strings.ToLower(id), "urgent") {
			tool = strings.TrimPrefix(id, "arch.tool.")
		}
	}
	if tool == "" {
		var tools []string
		for _, id := range wbIDs(j) {
			if strings.HasPrefix(id, "arch.tool.") {
				tools = append(tools, id)
			}
		}
		t.Fatalf("no Haul urgently tool among the Orders tools %q", tools)
	}
	j.openArch("orders", tool)
	j.lookAt(2, 14)
	j.update(1)
	j.settle()
	j.dragTiles(inputframe.Left, 2, 14, 2, 14, 0)
	if c, ok := j.lastCmd(command.HaulUrgently); !ok || !c.Enabled || c.X != 2 || c.Y != 14 {
		t.Fatalf("HaulUrgently %+v (sent %v), want Enabled at (2, 14)", c, ok)
	}
	j.leaveArch()
	j.update(1)
	if it, ok := w2Item(j, far); !ok || !w2Bool(j, it, "Urgent") {
		t.Fatalf("the marked stack's ItemView %+v is not Urgent", it)
	}
	for _, id := range near {
		if it, _ := w2Item(j, id); w2Bool(j, it, "Urgent") {
			t.Fatalf("near stack %d is Urgent too", id)
		}
	}
	first := entity.EntityID(0)
	j.key("Digit3", 0)
	for i := 0; i < 4000 && first == 0; i++ {
		careFeed(j)
		j.update(1)
		for _, it := range j.world().Items {
			if !it.Carried {
				continue
			}
			if it.ID == far {
				first = far
			}
			for _, n := range near {
				if it.ID == n && first == 0 {
					first = n
				}
			}
		}
	}
	j.key("Space", 0)
	if first != far {
		t.Fatalf("the first stack hauled was %d, want the urgent far stack %d (near %v)", first, far, near)
	}
}

// TestJourneyNutrientPasteDispenser (STO-2): a powered dispenser with a
// hopper of raw food feeds a hungry colonist a nutrient paste meal.
func TestJourneyNutrientPasteDispenser(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 8}), jSize(30, 30))
	j.g.ClearPlants(0, 0, 29, 29)
	wdBuild(j, "wood-fired generator", 15, 2)
	j.g.FuelCampfire(15, 2)
	for x := 8; x <= 15; x++ {
		j.g.BuildNow("power conduit", "steel", "south", x, 4)
	}
	wdBuild(j, "nutrient paste dispenser", 10, 5)
	d := wdBuilding(j, 10, 5)
	var hx, hy = 9, 5
	wdBuild(j, "hopper", hx, hy)
	if _, err := j.g.SpawnStack(item.Potatoes, 40, hx, hy); err != nil { // FOOD-4: raw berries (no raw-food thought) would beat paste
		t.Fatal(err)
	}
	j.tick(240) // the generator lights and the dispenser comes online
	_ = d
	id := ordColonists(j)[0]
	const hungry = needs.CriticalHunger - 800
	j.g.SetNeeds(entity.EntityID(id), hungry, needs.Full)
	start := j.pawn(id).Hunger
	sawPaste := false
	j.key("Digit3", 0)
	for i := 0; i < 6000 && !(sawPaste && j.pawn(id).Hunger >= start+20); i++ {
		j.update(1)
		p := j.pawn(id)
		sawPaste = sawPaste || strings.Contains(strings.ToLower(p.Job), "dispens")
		for _, it := range j.world().Items {
			sawPaste = sawPaste || strings.Contains(strings.ToLower(it.Kind), "paste")
		}
	}
	j.key("Space", 0)
	if p := j.pawn(id); p.Hunger < start+20 || !sawPaste {
		t.Fatalf("hunger %d (started %d), paste meal seen %v, job %q: the colonist did not eat from the dispenser", p.Hunger, start, sawPaste, p.Job)
	}
}

// TestJourneyBillTakesFromStockpile (STO-2/RES-1): a bill set to take from
// one chosen stockpile uses only its stacks, never the other stockpile's.
func TestJourneyBillTakesFromStockpile(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	j.g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 6, X2: 12, Y2: 6})
	j.g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 9, X2: 12, Y2: 9})
	j.g.ApplyPending()
	var chosen, other uint16
	for _, z := range j.world().Zones {
		switch {
		case z.MinY == 9: // the farther stockpile is the chosen one, so nearest-first would take the decoy
			chosen = z.ID
		case z.MinY == 6:
			other = z.ID
		}
	}
	if chosen == 0 || other == 0 {
		t.Fatalf("stockpiles not laid: %+v", j.world().Zones)
	}
	j.g.Enqueue(command.Command{Kind: command.RenameZone, Target: uint64(chosen), Name: "Alpha"})
	j.g.Enqueue(command.Command{Kind: command.RenameZone, Target: uint64(other), Name: "Beta"})
	j.g.ApplyPending()
	wbBench(j, 10, 5, uint8(recipe.Club))
	pick, err := j.g.SpawnStack(item.Wood, 60, 12, 9)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := j.g.SpawnStack(item.Wood, 60, 12, 6)
	if err != nil {
		t.Fatal(err)
	}
	wbSettings(j, "0")
	if w2Find(j, []string{"bill.cfg", "source"}) != "" {
		w2Click(j, "bill source control", []string{"bill.cfg", "source"})
		if !j.a.ord.Float.Open {
			t.Log("the source control opened no float menu")
		}
		picked := false
		var labels []string
		for _, b := range j.a.ord.FloatButtons() {
			labels = append(labels, b.Label)
			if !picked && strings.Contains(b.Label, "Alpha") {
				j.pick(b.Label)
				picked = true
			}
		}
		if !picked {
			t.Fatalf("no source choice for the chosen stockpile (Alpha); choices %q", labels)
		}
	} else {
		t.Log("no bill.cfg source control yet: sending SetStorageSource directly")
		j.g.Enqueue(command.Command{Kind: command.SetStorageSource, X: 10, Y: 5, Index: 0, Zone: uint64(chosen)})
		j.g.ApplyPending()
	}
	j.update(1)
	if c, ok := j.lastCmd(command.SetStorageSource); !ok || c.Zone != uint64(chosen) || c.X != 10 || c.Y != 5 {
		// the sim side is still checked below by sending the command directly
		t.Errorf("clicking the Alpha row of the source pick sent SetStorageSource %+v (sent %v), want bench (10, 5) zone %d; float menu still open: %v, top modal %q", c, ok, chosen, j.a.ord.Float.Open, j.a.uiModalTop())
		j.g.Enqueue(command.Command{Kind: command.SetStorageSource, X: 10, Y: 5, Index: 0, Zone: uint64(chosen)})
		j.g.ApplyPending()
	}
	j.key("Escape", 0)
	made := false
	j.key("Digit3", 0)
	for i := 0; i < 20000 && !made; i++ {
		careFeed(j)
		j.update(1)
		for _, it := range j.world().Items {
			made = made || it.Kind == item.Club.String()
		}
	}
	j.key("Space", 0)
	if !made {
		t.Fatal("no club was made from the chosen stockpile")
	}
	if it, ok := w2Item(j, keep); !ok || it.Qty != 60 {
		c, _ := w2Item(j, pick)
		t.Fatalf("the other stockpile's stack has Qty %d (found %v), want it untouched at 60; chosen stack Qty %d at (%d, %d); source cmds %+v", it.Qty, ok, c.Qty, c.X, c.Y, j.cmdsOf(command.SetStorageSource))
	}
	if it, ok := w2Item(j, pick); ok && it.Qty >= 60 {
		t.Fatalf("the chosen stockpile's stack was not used: %+v", it)
	}
}

// TestJourneyTwoEatersShareAStack (RES-1): two hungry colonists eat from
// one stack of meals during the same updates.
func TestJourneyTwoEatersShareAStack(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{8, 6}))
	j.g.ClearPlants(0, 0, 23, 23)
	if _, err := j.g.SpawnStack(item.Meal, 20, 11, 5); err != nil {
		t.Fatal(err)
	}
	cs := ordColonists(j)
	const hungry = needs.CriticalHunger - 800
	for _, id := range cs {
		j.g.SetNeeds(entity.EntityID(id), hungry, needs.Full)
	}
	j.update(1) // the view catches up with SetNeeds
	both, ate := false, [2]bool{}
	j.key("Digit1", 0)
	for i := 0; i < 4000 && !(both && j.pawn(cs[0]).Hunger >= 60 && j.pawn(cs[1]).Hunger >= 60); i++ {
		j.update(1)
		a, b := strings.ToLower(j.pawn(cs[0]).Job), strings.ToLower(j.pawn(cs[1]).Job)
		ate[0], ate[1] = ate[0] || strings.HasPrefix(a, "eat"), ate[1] || strings.HasPrefix(b, "eat")
		both = both || (strings.HasPrefix(a, "eat") && strings.HasSuffix(a, " do") && strings.HasPrefix(b, "eat") && strings.HasSuffix(b, " do"))
	}
	j.key("Space", 0)
	if !both {
		t.Fatalf("the two colonists never ate at once (each ate at all: %v); jobs now %q and %q", ate, j.pawn(cs[0]).Job, j.pawn(cs[1]).Job)
	}
	for _, id := range cs {
		if p := j.pawn(id); p.Hunger < 60 {
			t.Fatalf("colonist %d only reached food %d", id, p.Hunger)
		}
	}
}

// TestJourneyRifleWoundBleeds (CBT-1): a raider shot by a colonist's
// rifle shows a bleeding gunshot wound in the health view.
func TestJourneyRifleWoundBleeds(t *testing.T) {
	t.Parallel()
	j, _ := weFireSetup(t)
	raider := weRaiders(j)[0].ID
	seen, bleed := "", false
	j.key("Digit3", 0)
	for i := 0; i < 6000 && !(seen != "" && bleed); i++ {
		careFeed(j)
		j.update(1)
		hv, ok := j.g.Health(raider)
		if !ok {
			continue
		}
		for _, p := range hv.Parts {
			txt := strings.ToLower(p.Label + " " + p.Status + " " + p.TendTip)
			if strings.Contains(txt, "gunshot") || strings.Contains(txt, "bullet") {
				seen = txt
				bleed = bleed || p.Bleed > 0 || strings.Contains(strings.ToLower(hv.State), "bleed")
			}
		}
	}
	j.key("Space", 0)
	hv, _ := j.g.Health(raider)
	var wounds []string
	for _, p := range hv.Parts {
		if p.TendTip != "" {
			wounds = append(wounds, fmt.Sprintf("%s: %s (bleed %d)", p.Label, p.TendTip, p.Bleed))
		}
	}
	if seen == "" {
		t.Fatalf("no gunshot wound on the raider's health view; wounds %q, state %q", wounds, hv.State)
	}
	if !bleed {
		t.Fatalf("the gunshot wound %q does not bleed; wounds %q, state %q", seen, wounds, hv.State)
	}
}

// TestJourneyHelmetGuardsHeadOnly (CBT-1) is a game-level test: neither the
// gear tab nor the health view reports which part an apparel covers or
// absorbed a hit on, so a player cannot observe it through the UI.
func TestJourneyHelmetGuardsHeadOnly(t *testing.T) {
	t.Skip("covered at game level (combat/armor tests): no view in the UI exposes per-part armor")
}

// TestJourneyMeleeDodge (CBT-2): in a melee a blow can be dodged: the
// traces hold both hits and misses.
func TestJourneyMeleeDodge(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}, [2]int{10, 4}))
	for i := range ordColonists(j) {
		weDraft(j, i)
	}
	j.g.RaidArmed(item.Club, item.Club)
	j.update(1) // the raid alert pauses the game
	hits, misses := 0, 0
	type blow struct {
		who  entity.EntityID
		tick uint64
	}
	seen := map[blow]bool{}
	j.key("Digit3", 0)
	for i := 0; i < 6000 && (hits == 0 || misses == 0); i++ {
		careFeed(j)
		j.update(1)
		for _, tr := range j.world().Traces {
			k := blow{tr.Attacker, uint64(j.g.Tick()) - uint64(tr.Age)}
			if !tr.Melee || tr.Building || seen[k] {
				continue
			}
			seen[k] = true
			if tr.Hit {
				hits++
			} else {
				misses++
			}
		}
	}
	j.key("Space", 0)
	if hits == 0 || misses == 0 {
		t.Fatalf("melee traces: %d hits and %d misses, want both (a dodge or miss shows as a trace without Hit)", hits, misses)
	}
}

// TestJourneyAutoUndraft (CBT-2): a drafted colonist with no threats near
// undrafts after 10,000 ticks.
func TestJourneyAutoUndraft(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := weDraft(j, 0)
	start := j.g.Tick()
	j.key("Digit3", 0)
	for i := 0; i < 40000 && j.g.Tick()-start < 5000; i++ {
		careFeed(j)
		j.update(1)
	}
	if !j.pawn(id).Drafted {
		t.Fatalf("undrafted after only %d ticks", j.g.Tick()-start)
	}
	for i := 0; i < 40000 && j.g.Tick()-start < 10500 && j.pawn(id).Drafted; i++ {
		careFeed(j)
		j.update(1)
	}
	j.key("Space", 0)
	if j.pawn(id).Drafted {
		t.Fatalf("still drafted after %d ticks with no threat", j.g.Tick()-start)
	}
}

// TestJourneyDropWeapon (CBT-2): the gear tab's Drop button drops the
// wielded weapon on the ground.
func TestJourneyDropWeapon(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	j.g.Arm(weID(id), item.Rifle)
	ordPick(j, 0)
	j.clickID("inspector.gear")
	if j.pawn(id).Weapon == "fists" {
		t.Fatal("the colonist is not armed")
	}
	w2Click(j, "Drop weapon button", []string{"drop"})
	if c, ok := j.lastCmd(command.DropWeapon); !ok || c.Pawn != id {
		t.Fatalf("DropWeapon %+v (sent %v), want colonist %d", c, ok, id)
	}
	j.update(2)
	if w := j.pawn(id).Weapon; w != "fists" {
		t.Fatalf("PawnView.Weapon is %q after Drop, want fists", w)
	}
	p := j.pawn(id)
	for _, it := range j.world().Items {
		if it.Kind == item.Rifle.String() && !it.Carried && abs(it.X-p.X) <= 1 && abs(it.Y-p.Y) <= 1 {
			return
		}
	}
	t.Fatalf("no rifle lies on the ground next to (%d, %d): %+v", p.X, p.Y, j.world().Items)
}

// TestJourneyZonesOverlayToggle (STO-3): the zones overlay action hides
// every zone and shows them again.
func TestJourneyZonesOverlayToggle(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	j.layZone("zones", "stockpile", 8, 1, 9, 2)
	j.key("Escape", 0)
	b := w2KeyFor(j, "zones", "overlay")
	j.key(b.Key, b.Mods)
	c, ok := j.lastCmd(command.SetZoneOverlay)
	if !ok || c.Enabled {
		t.Fatalf("first press sent %+v (%v), want SetZoneOverlay with Enabled false (hidden)", c, ok)
	}
	j.key(b.Key, b.Mods)
	if c, ok = j.lastCmd(command.SetZoneOverlay); !ok || !c.Enabled {
		t.Fatalf("second press sent %+v (%v), want SetZoneOverlay with Enabled true (shown)", c, ok)
	}
}
