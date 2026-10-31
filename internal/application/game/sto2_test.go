package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// STO-2: stockpile simulation (vanilla defaults, storage buildings,
// groups, haul urgently, the nutrient paste dispenser).

// allowAll sets the storage zone on (x, y) to take everything storable,
// rotten stacks too (STO-2: new stockpiles refuse corpses, bones, chunks
// and rotten stacks).
func allowAll(g *Game, x, y int) {
	id, _ := g.zones.At(x, y)
	g.Enqueue(command.Command{Kind: command.SetStockpileFilter, Target: uint64(id), Name: FilterRoot, Enabled: true})
	g.Enqueue(command.Command{Kind: command.SetStockpileSpecial, Target: uint64(id), Index: zone.SpecialRotten, Enabled: true})
	g.ApplyPending()
}

// pasteKitchen is a powered dispenser at (10, 4) (spot (11, 8)) with a
// hopper on its west side holding qty potatoes, and a colonist.
func pasteKitchen(t *testing.T, qty int) (g *Game, disp, hop, ada entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 1, Width: 30, Height: 16})
	disp = buildNow(t, g, construction.NutrientPasteDispenser, item.Steel, construction.South, 10, 4)
	hop = buildNow(t, g, construction.Hopper, item.Steel, construction.South, 9, 4)
	onPower(t, g, 10, 4, 13, 4)
	if !g.powered(disp) {
		t.Fatal("the dispenser has no power")
	}
	if qty > 0 {
		spawn(t, g, item.Potatoes, qty, 9, 4)
	}
	ada, err := g.SpawnNamedPawn(11, 12, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	onlyWork(g, ada)
	return g, disp, hop, ada
}

// TestDispenserFeedsAHungryColonist: with no other food, a hungry
// colonist takes a paste meal at the spot (the wiki's 0.3 nutrition of
// raw food from the hopper), eats it and remembers it; the hopper's
// storage reads in the views.
func TestDispenserFeedsAHungryColonist(t *testing.T) {
	t.Parallel()
	g, disp, hop, ada := pasteKitchen(t, 60)
	if bv := buildingView(g, hop); bv.Stored != 1 || bv.Capacity != 1 {
		t.Fatalf("hopper readout %d/%d", bv.Stored, bv.Capacity)
	}
	if z := zoneViewAt(t, g, 9, 4); z.Kind != ZoneHopper || z.Priority != "Important" || z.Shelf != uint64(hop) {
		t.Fatalf("hopper zone %+v", z)
	}
	g.SetNeeds(ada, needs.CriticalHunger, needs.Full)
	sawDispense := false
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.DispensePaste {
			sawDispense = j.Target == disp
		}
		if k, ok := g.lastMeal[ada]; ok {
			if k != item.NutrientPasteMeal {
				t.Fatalf("ate %s", k)
			}
			break
		}
	}
	if !sawDispense || g.lastMeal[ada] != item.NutrientPasteMeal {
		t.Fatalf("no paste meal eaten (dispense job seen %v)", sawDispense)
	}
	per := (store.PasteCost + item.Potatoes.Nutrition() - 1) / item.Potatoes.Nutrition()
	if st, _ := g.stacks.Get(g.itemAt(9, 4)); st.Qty != 60-per {
		t.Fatalf("potatoes left %d, want %d", st.Qty, 60-per)
	}
	if n, _ := g.need.Get(ada); n.Hunger <= needs.CriticalHunger {
		t.Fatalf("still hungry: %d", n.Hunger)
	}
	if !slices.ContainsFunc(g.memories[ada], func(m mood.Memory) bool { return m.Kind == mood.AtePaste }) {
		t.Fatal("no paste thought")
	}
}

// TestDispenserNeedsPowerAndFood: a dark dispenser or empty hoppers make
// no meal, so a hungry colonist with no food finds none.
func TestDispenserNeedsPowerAndFood(t *testing.T) {
	t.Parallel()
	g, _, _, ada := pasteKitchen(t, 0)
	g.SetNeeds(ada, needs.CriticalHunger, needs.Full)
	if g.seekFood(ada) {
		t.Fatal("empty hoppers feed nobody")
	}
	spawn(t, g, item.Potatoes, 75, 9, 4)
	if !g.seekFood(ada) {
		t.Fatal("a stocked dispenser is food")
	}
	var fresh research.Progress
	if p, locked := fresh.Locks("build:hopper"); !locked || p != research.NutrientPaste {
		t.Fatal("the hopper waits for Nutrient paste")
	}
}

// TestHopperRefillNoFlap: haulers fill an empty hopper with raw food
// (priority Important beats the stockpile), never top up a hopper still
// over half full, and refill it once it is down to half (hysteresis: no
// trickle of one trip per meal).
func TestHopperRefillNoFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	hop := buildNow(t, g, construction.Hopper, item.Steel, construction.South, 4, 4)
	zoneCmd(g, command.DesignateStockpile, 15, 2, 20, 8, 0)
	spawn(t, g, item.Potatoes, 75, 16, 3)
	spawn(t, g, item.Potatoes, 75, 17, 3)
	spawn(t, g, item.Steel, 20, 18, 3) // never into a hopper
	ada, _ := g.SpawnNamedPawn(10, 10, "Ada")
	onlyWork(g, ada, work.Haul)
	hopper := func() int { st, _ := g.stacks.Get(g.itemAt(4, 4)); return st.Qty }
	for i := 0; i < 200 && hopper() < 75; i++ {
		runFed(g, 1, ada)
	}
	if hopper() != 75 {
		t.Fatalf("hopper filled to %d", hopper())
	}
	trips := 0
	take := func(n int) { st, _ := g.stacks.Get(g.itemAt(4, 4)); st.Qty -= n; g.putStack(g.itemAt(4, 4), st) }
	take(30) // 45 left: over half
	for range 60 {
		runFed(g, 1, ada)
		if j, _ := g.jobs.Get(ada); j.Kind.Hauls() && j.DestX == 4 && j.DestY == 4 { // FOOD-4: a FillHopper job too
			trips++
		}
	}
	if trips != 0 || hopper() != 45 {
		t.Fatalf("a hopper over half full was topped up (%d trips, %d)", trips, hopper())
	}
	take(10) // 35: under half
	for i := 0; i < 200 && hopper() < 75; i++ {
		runFed(g, 1, ada)
	}
	if hopper() != 75 {
		t.Fatalf("refilled to %d", hopper())
	}
	if st, _ := g.stacks.Get(g.itemAt(4, 4)); st.Kind != item.Potatoes || buildingView(g, hop).Stored != 1 {
		t.Fatal("the hopper holds one stack of potatoes")
	}
}

// TestCookFillsHopper: hopper filling is cook work too (wiki).
func TestCookFillsHopper(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	buildNow(t, g, construction.Hopper, item.Steel, construction.South, 4, 4)
	spawn(t, g, item.Meat, 40, 12, 4)
	cook, _ := g.SpawnNamedPawn(10, 10, "Cook")
	onlyWork(g, cook, work.Cook)
	for i := 0; i < 200 && g.itemAt(4, 4) == 0; i++ {
		runFed(g, 1, cook)
	}
	if st, _ := g.stacks.Get(g.itemAt(4, 4)); st.Kind != item.Meat || st.Qty != 40 {
		t.Fatalf("the cook filled the hopper with %+v", st)
	}
}

// TestHaulUrgentlyFirst: a stack marked to haul urgently is hauled before
// a nearer plain one; the mark shows in the view, survives a save, and is
// dropped once the stack is stored, where it stays (no flapping).
func TestHaulUrgentlyFirst(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	zoneCmd(g, command.DesignateStockpile, 25, 2, 28, 8, 0)
	spawn(t, g, item.Steel, 10, 5, 5)                  // near
	urgent, _ := g.SpawnStack(item.Silver, 10, 18, 12) // far
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	onlyWork(g, ada, work.Haul)
	applyNow(g, command.Command{Kind: command.HaulUrgently, X: 17, Y: 11, X2: 19, Y2: 13, Enabled: true})
	if iv, _ := itemView(g, urgent); !iv.Urgent {
		t.Fatal("the view shows the mark")
	}
	if h := saveLoad(t, g); !h.haul.store.Urgent.Has(urgent) || h.StateHash() != g.StateHash() {
		t.Fatal("the mark survives a save")
	}
	g.Step(2)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Haul || j.Target != urgent {
		t.Fatalf("the urgent stack goes first: %+v", j)
	}
	for i := 0; i < 300 && !g.inMatchingStockpile(mustStack(g, urgent)); i++ {
		runFed(g, 1, ada)
	}
	runFed(g, 5, ada)
	if g.haul.store.Urgent.Any() {
		t.Fatal("a stored stack loses its mark")
	}
	at := mustStack(g, urgent)
	runFed(g, 100, ada)
	if now := mustStack(g, urgent); now.X != at.X || now.Y != at.Y {
		t.Fatal("a stored urgent stack never moves again")
	}
	applyNow(g, command.Command{Kind: command.HaulUrgently, X: 0, Y: 0, X2: 29, Y2: 15, Enabled: false})
	if g.haul.store.Urgent.Any() {
		t.Fatal("unmarked")
	}
}

func mustStack(g *Game, id entity.EntityID) item.Stack { st, _ := g.stacks.Get(id); return st }

// TestLinkedShelvesShareSettings: LinkStorage links a shelf into
// another's group, which then shares its settings both ways; the views
// carry the group and readouts; the group survives a save; unlinking
// keeps the settings; a bill naming one member stores into the group.
func TestLinkedShelvesShareSettings(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	a := buildNow(t, g, construction.Shelf, item.Wood, construction.South, 2, 2)
	b := buildNow(t, g, construction.Shelf, item.Wood, construction.South, 6, 2)
	hop := buildNow(t, g, construction.Hopper, item.Steel, construction.South, 10, 2)
	za, zb := zoneViewAt(t, g, 2, 2), zoneViewAt(t, g, 6, 2)
	applyNow(g, command.Command{Kind: command.SetZonePriority, Target: uint64(za.ID), Index: 0})
	applyNow(g, command.Command{Kind: command.LinkStorage, Target: uint64(b), Zone: uint64(a)})
	applyNow(g, command.Command{Kind: command.LinkStorage, Target: uint64(hop), Zone: uint64(a)}) // kinds differ
	zb = zoneViewAt(t, g, 6, 2)
	if zb.Group == 0 || zb.Group != zoneViewAt(t, g, 2, 2).Group || zb.Priority != "Critical" || zoneViewAt(t, g, 10, 2).Group != 0 {
		t.Fatalf("linked %+v", zb)
	}
	applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(zb.ID), Name: "steel", Enabled: false})
	if slices.Contains(zoneViewAt(t, g, 2, 2).Filter, "Raw resources/Metals") {
		t.Fatal("an edit to one member reaches the other")
	}
	spawn(t, g, item.Wood, 30, 2, 2)
	if bv := buildingView(g, a); bv.Group != zb.Group || bv.Stored != 1 || bv.Capacity != 2*zone.ShelfStacks {
		t.Fatalf("shelf view %+v", bv)
	}
	h := saveLoad(t, g)
	if zoneViewAt(t, h, 6, 2).Group != zb.Group {
		t.Fatal("the group survives a save")
	}
	// a bill product stores into any member of the group it names
	sid, _ := g.SpawnStack(item.Club, 1, 15, 10)
	cook, _ := g.SpawnNamedPawn(15, 11, "Smith")
	if !(craftOutput{g}).Store(cook, item.Club, 15, 10, uint64(za.ID)) {
		t.Fatal("the club finds a cell in the group")
	}
	if j, _ := g.jobs.Get(cook); j.Target != sid || !g.zones.SameStore(zoneIDAt(g, j.DestX, j.DestY), zone.ID(za.ID)) {
		t.Fatalf("store job %+v", j)
	}
	applyNow(g, command.Command{Kind: command.UnlinkStorage, Target: uint64(b)})
	if z := zoneViewAt(t, g, 6, 2); z.Group != 0 || z.Priority != "Critical" || zoneViewAt(t, g, 2, 2).Group != 0 {
		t.Fatalf("unlinked %+v", z)
	}
}

func zoneIDAt(g *Game, x, y int) zone.ID { id, _ := g.zones.At(x, y); return id }

// TestSpecialFiltersOnEveryStorePath (STO-2 bug fixes): a forced haul, a
// delivery's spare cell and a bill's product honour the special filters,
// not just the kind.
func TestSpecialFiltersOnEveryStorePath(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	zoneCmd(g, command.DesignateStockpile, 20, 2, 24, 6, 0)
	sp := zoneViewAt(t, g, 20, 2)
	applyNow(g, command.Command{Kind: command.SetStockpileSpecial, Target: uint64(sp.ID), Index: zone.SpecialTainted, Enabled: false})
	tainted, _ := g.SpawnStack(item.Parka, 1, 5, 5)
	st := mustStack(g, tainted)
	st.Of = 77
	g.putStack(tainted, st)
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	if _, r := g.forcedJob(ada, command.OrderHaul, 5, 5, construction.LayerObject); r != ReasonNoStockpile {
		t.Fatalf("a forced haul of tainted apparel: %q", r)
	}
	if _, _, ok := g.stockpileCell(st, 5, 5); ok {
		t.Fatal("a spare or leftover cell for tainted apparel")
	}
	if g.spareTakes(st, 20, 2) {
		t.Fatal("the spare cell takes tainted apparel")
	}
	if (craftOutput{g}).Store(ada, item.Parka, 5, 5, 0) {
		t.Fatal("a bill product refused by the special filters found a cell")
	}
}

// TestGravesAndSarcophagiAsStorage: a grave set to take animals buries a
// carcass; a sarcophagus refuses a stranger; the higher priority grave is
// filled first; an older save's grave gets its settings on load.
func TestGravesAndSarcophagiAsStorage(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	gr := buildNow(t, g, construction.Grave, item.Wood, construction.South, 4, 4)
	sar := buildNow(t, g, construction.Sarcophagus, item.Wood, construction.South, 10, 4)
	gz, sz := zoneViewAt(t, g, 4, 4), zoneViewAt(t, g, 10, 4)
	if gz.Kind != ZoneGrave || gz.Shelf != uint64(gr) || sz.Shelf != uint64(sar) || slices.Contains(sz.Special, "allow stranger corpses") {
		t.Fatalf("grave %+v sarcophagus %+v", gz, sz)
	}
	stranger := item.Stack{Kind: item.CorpseHuman, Qty: 1, Of: 999, X: 20, Y: 10}
	if _, ok := g.graveSettings(sar, stranger); ok {
		t.Fatal("a sarcophagus takes colonists only")
	}
	if _, ok := g.graveSettings(gr, stranger); !ok {
		t.Fatal("a grave takes a stranger")
	}
	carcass, _ := g.SpawnStack(item.CorpseAnimal, 1, 20, 10)
	digger, _ := g.SpawnNamedPawn(18, 10, "Digger")
	onlyWork(g, digger, work.Haul)
	if g.assignBury(digger) {
		t.Fatal("graves take no animals by default")
	}
	applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(gz.ID), Name: "Corpses/Animal", Enabled: true})
	applyNow(g, command.Command{Kind: command.SetZonePriority, Target: uint64(gz.ID), Index: 0})
	for i := 0; i < 400 && !g.graveFilled(gr); i++ {
		runFed(g, 1, digger)
	}
	if _, ok := g.stacks.Get(carcass); ok || !g.graveFilled(gr) || buildingView(g, gr).Stored != 1 {
		t.Fatal("the carcass was buried in the grave")
	}
	// an older save's grave: no zone until the load lays one
	g.zones.RemoveTiles(g.zones.Cells(zoneIDAt(g, 10, 4)))
	h := saveLoad(t, g)
	if z, ok := h.haul.store.Buildings.ZoneOf(h.zones, haulHost{h}, sar); !ok || !z.Special.NoStranger {
		t.Fatal("a loaded sarcophagus gets its settings")
	}
}

// TestNewStockpileDefaults: a painted stockpile takes no corpses, chunks
// or rotten food; existing zones keep their settings through a save.
func TestNewStockpileDefaults(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	zoneCmd(g, command.DesignateStockpile, 2, 2, 6, 6, 0)
	z, _ := g.zones.Zone(zoneIDAt(g, 2, 2))
	if z.Allows(item.CorpseAnimal) || z.Allows(item.GraniteChunk) || !z.Allows(item.Meal) ||
		z.AllowsStack(item.Stack{Kind: item.Meat, Qty: 1, Rot: 1 << 30}) {
		t.Fatalf("new stockpile %+v", z)
	}
	old := g.zones.Paint(zone.KindStockpile, zone.Zone{Food: true, Wood: true, Stone: true}, zone.RectOf(10, 2, 12, 4), nil)
	h := saveLoad(t, g)
	if oz, _ := h.zones.Zone(old); oz.Custom || !oz.Allows(item.GraniteChunk) || !oz.Allows(item.CorpseAnimal) {
		t.Fatalf("an older zone keeps its switches: %+v", oz)
	}
	if v := zoneViewAt(t, g, 2, 2); v.Capacity != 25 || v.Stored != 0 {
		t.Fatalf("readout %d/%d", v.Stored, v.Capacity)
	}
	if len(StorageFilterRows(ZoneHopper, nil)) >= len(StockpileFilterRows(nil)) || StorageFilterRows("nonsense", nil) != nil {
		t.Fatal("hopper rows are the raw food subtree")
	}
	if len(StorageSpecials()) <= len(StockpileSpecials()) {
		t.Fatal("every toggle")
	}
}
