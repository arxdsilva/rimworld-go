package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PRD-4: storage filters, special filters, copy and paste, shelves.

// zoneViewAt is the zone view holding tile (x, y).
func zoneViewAt(t *testing.T, g *Game, x, y int) ZoneView {
	t.Helper()
	for _, z := range g.World().Zones {
		for _, c := range z.Cells {
			if c[0] == x && c[1] == y {
				return z
			}
		}
	}
	t.Fatalf("no zone at (%d, %d)", x, y)
	return ZoneView{}
}

func hasWood(names []string) bool {
	return slices.ContainsFunc(names, func(s string) bool { return strings.Contains(strings.ToLower(s), "wood") })
}

// stockCmd applies one storage command at once.
func stockCmd(g *Game, c command.Command) {
	g.Enqueue(c)
	g.ApplyPending()
}

// TestStockpileFilterHauling: forbidding wood takes it out of the
// stockpile's Filter and haulers leave wood where it lies while they
// store the stone; the settings paste onto a second stockpile.
func TestStockpileFilterHauling(t *testing.T) {
	t.Parallel()
	g := storedYard(t)
	sp := zoneViewAt(t, g, 16, 1)
	if !hasWood(sp.Filter) || slices.Contains(sp.Special, "allow rotten") || !slices.Contains(sp.Special, "allow tainted apparel") ||
		slices.Contains(sp.Filter, "Chunks") || slices.Contains(sp.Filter, "Corpses") {
		t.Fatalf("a new stockpile (STO-2: no corpses, chunks or rotten): filter %v special %v", sp.Filter, sp.Special)
	}
	stockCmd(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(sp.ID), Name: "wood", Enabled: false})
	stockCmd(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(sp.ID), Name: "Nonsense", Enabled: false})
	sp = zoneViewAt(t, g, 16, 1)
	if hasWood(sp.Filter) || len(sp.Filter) == 0 {
		t.Fatalf("filter after forbidding wood: %v", sp.Filter)
	}
	if _, err := g.SpawnNamedPawn(4, 4, "Ada"); err != nil {
		t.Fatal(err)
	}
	spawn(t, g, item.Wood, 20, 6, 6)
	spawn(t, g, item.Steel, 20, 6, 7)
	g.Step(1500)
	for _, it := range g.World().Items {
		_, zoned := g.storageAt(it.X, it.Y)
		switch it.Kind {
		case "wood":
			if zoned {
				t.Fatal("wood reached a stockpile that forbids it")
			}
		case "steel":
			if !zoned && !it.Carried {
				t.Fatalf("steel left at (%d, %d)", it.X, it.Y)
			}
		}
	}
	// paste onto a second stockpile (and onto itself: ignored)
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 9, X2: 4, Y2: 10})
	g.Step(1)
	sp2 := zoneViewAt(t, g, 2, 9)
	if !hasWood(sp2.Filter) {
		t.Fatal("the second stockpile starts allowing wood")
	}
	stockCmd(g, command.Command{Kind: command.SetZonePriority, Target: uint64(sp.ID), Index: 0})
	stockCmd(g, command.Command{Kind: command.PasteStockpile, Target: uint64(sp2.ID), Zone: uint64(sp.ID)})
	stockCmd(g, command.Command{Kind: command.PasteStockpile, Target: uint64(sp2.ID), Zone: uint64(sp2.ID)})
	stockCmd(g, command.Command{Kind: command.PasteStockpile, Target: 999, Zone: uint64(sp.ID)})
	if sp2 = zoneViewAt(t, g, 2, 9); hasWood(sp2.Filter) || sp2.Priority != "Critical" {
		t.Fatalf("pasted: filter %v priority %s", sp2.Filter, sp2.Priority)
	}
	// a special filter toggle and a range
	stockCmd(g, command.Command{Kind: command.SetStockpileSpecial, Target: uint64(sp2.ID), Index: zone.SpecialRotten, Enabled: false})
	stockCmd(g, command.Command{Kind: command.SetStockpileSpecial, Target: uint64(sp2.ID), Index: zone.SpecialTainted, Enabled: false})
	stockCmd(g, command.Command{Kind: command.SetStockpileSpecial, Target: uint64(sp2.ID), Index: zone.SpecialHP, X: 50, X2: 100})
	got := zoneViewAt(t, g, 2, 9).Special
	if slices.Contains(got, "allow rotten") || slices.Contains(got, "allow tainted apparel") || got[len(got)-1] != "hp 50-100%" {
		t.Fatalf("specials %v", got)
	}
	// the filter rows read the view back
	rows := StockpileFilterRows(zoneViewAt(t, g, 2, 9).Filter)
	state := map[string]FilterRow{}
	for _, r := range rows {
		state[r.Path] = r
	}
	if state["wood"].On || !state["steel"].On || !state["Raw resources"].Partial || !state["Foods"].On || state["Foods"].Partial {
		t.Fatalf("rows: wood %+v steel %+v raw %+v food %+v", state["wood"], state["steel"], state["Raw resources"], state["Foods"])
	}
	// saved and loaded
	g2 := saveLoad(t, g)
	if z := zoneViewAt(t, g2, 2, 9); hasWood(z.Filter) || !slices.Equal(z.Special, got) {
		t.Fatalf("after a load: %v %v", z.Filter, z.Special)
	}
}

// TestSpecialFilterRefusesTainted: a stockpile that refuses tainted
// apparel is not where a tainted parka goes, nor where it stays.
func TestSpecialFilterRefusesTainted(t *testing.T) {
	t.Parallel()
	g := storedYard(t)
	sp := zoneViewAt(t, g, 16, 1)
	stockCmd(g, command.Command{Kind: command.SetStockpileSpecial, Target: uint64(sp.ID), Index: zone.SpecialTainted, Enabled: false})
	tainted := item.Stack{Kind: item.Parka, Qty: 1, Of: 77, X: 17, Y: 1}
	if g.inMatchingStockpile(tainted) {
		t.Fatal("a tainted parka counts as stored where tainted apparel is refused")
	}
	if _, _, ok := g.haulDest(0, item.Stack{Kind: item.Parka, Qty: 1, Of: 77, X: 3, Y: 3}, 0); ok {
		t.Fatal("a destination was found for a tainted parka")
	}
	if _, _, ok := g.haulDest(0, item.Stack{Kind: item.Parka, Qty: 1, X: 3, Y: 3}, 0); !ok {
		t.Fatal("a clean parka finds a cell")
	}
}

// TestShelfStoresThreeStacksPerCell: a built shelf lays its own storage
// (Preferred, hidden, no chunks), holds up to three stacks on one cell,
// shelters them from the weather and takes its storage away when it goes.
func TestShelfStoresThreeStacksPerCell(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	if !g.BuildNow("shelf", "wood", "south", 10, 6) {
		t.Fatal("no shelf")
	}
	sh := zoneViewAt(t, g, 10, 6)
	bid := g.layerBldgAt(10, 6, construction.LayerObject)
	if sh.Kind != ZoneShelf || sh.Shelf != uint64(bid) || bid == 0 || len(sh.Cells) != 2 || sh.Priority != "Preferred" || !sh.Hidden {
		t.Fatalf("shelf zone %+v (building %d)", sh, bid)
	}
	z, _ := g.storageAt(10, 6)
	if z.Allows(item.GraniteChunk) || !z.Allows(item.Wood) || !z.Allows(item.Meal) {
		t.Fatal("a shelf takes general items, not chunks")
	}
	// three kinds go on one cell, the fourth to the other
	for i, k := range []item.Kind{item.Wood, item.Steel, item.Cloth, item.Leather} {
		x, y, ok := g.stockpileCell(item.Stack{Kind: k}, 10, 6)
		if !ok || !g.onShelf(x, y) {
			t.Fatalf("%s: no shelf cell (%d, %d, %v)", k, x, y, ok)
		}
		if want := 10; i == 3 {
			if x == want {
				t.Fatalf("a fourth stack on a full shelf cell")
			}
		} else if x != want || y != 6 {
			t.Fatalf("%s went to (%d, %d), want the nearest cell", k, x, y)
		}
		id, err := g.SpawnStack(k, 10, x, y)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := g.stacks.Get(id)
		g.mergeOrSet(id, st)
	}
	if n := countAt(g, 10, 6); n != 3 {
		t.Fatalf("shelf cell holds %d stacks, want 3", n)
	}
	// a fifth dropped on the full cell spills
	id, _ := g.SpawnStack(item.Wool, 5, 10, 6)
	st, _ := g.stacks.Get(id)
	g.mergeOrSet(id, st)
	if countAt(g, 10, 6) != 3 {
		t.Fatal("a full shelf cell took a fourth stack")
	}
	// topping up a stack already there is fine
	if x, y, ok := g.stockpileCell(item.Stack{Kind: item.Steel}, 10, 6); !ok || x != 10 || y != 6 {
		t.Fatalf("topping up steel: (%d, %d, %v)", x, y, ok)
	}
	if (craftWeather{g}).Exposed(10, 6) || !(craftWeather{g}).Exposed(3, 3) {
		t.Fatal("a shelf shelters what lies on it")
	}
	// the zone tools leave it alone
	stockCmd(g, command.Command{Kind: command.DeleteZone, Target: uint64(sh.ID)})
	if zoneViewAt(t, g, 10, 6).ID != sh.ID {
		t.Fatal("deleting a shelf's storage")
	}
	g2 := saveLoad(t, g)
	if z := zoneViewAt(t, g2, 11, 6); z.Kind != ZoneShelf {
		t.Fatalf("after a load: %+v", z)
	}
	g.removeBuilding(bid)
	if _, ok := g.storageAt(10, 6); ok {
		t.Fatal("the shelf's storage outlived it")
	}
}

// TestShelfOverStockpileAndMove: a shelf built on a stockpile takes its
// cells; moved, it keeps its settings and leaves nothing behind.
func TestShelfOverStockpileAndMove(t *testing.T) {
	t.Parallel()
	g := storedYard(t)
	if !g.BuildNow("shelf", "wood", "south", 17, 2) {
		t.Fatal("no shelf on the stockpile")
	}
	sh := zoneViewAt(t, g, 17, 2)
	if sh.Kind != ZoneShelf || zoneViewAt(t, g, 16, 2).Kind != ZoneStockpile {
		t.Fatalf("shelf %+v", sh)
	}
	stockCmd(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(sh.ID), Name: "Food", Enabled: false})
	bid := entity.EntityID(sh.Shelf)
	b, _ := g.bldgs.Get(bid)
	fr := construction.Frame{Kind: b.Kind, Stuff: b.Stuff, X: 4, Y: 4, Reinstall: uint64(bid)}
	g.completeMove(bid, fr)
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, 4, 4)
	g.zoneBuilt(b.Kind, cells)
	moved := zoneViewAt(t, g, 4, 4)
	if moved.Kind != ZoneShelf || moved.Shelf != uint64(bid) || slices.Contains(moved.Filter, "Food") {
		t.Fatalf("moved shelf zone %+v", moved)
	}
	if z, ok := g.storageAt(17, 2); ok && z.Kind == zone.KindShelf {
		t.Fatal("the old cells kept a shelf's storage")
	}
}

func countAt(g *Game, x, y int) int {
	n := 0
	for _, id := range g.itemIDs {
		if st := g.stacks.Ref(id); st != nil && st.Carrier == 0 && st.X == x && st.Y == y {
			n++
		}
	}
	return n
}

// TestWaveEStackLimit: a hauler carries no more of a wave E kind than a
// tile holds, and merging stops at the kind's limit.
func TestWaveEStackLimit(t *testing.T) {
	t.Parallel()
	g := storedYard(t)
	spawn(t, g, item.FineMeal, 8, 16, 1)
	id, err := g.SpawnStack(item.FineMeal, 6, 16, 1)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(id)
	g.mergeOrSet(id, st)
	total, most := 0, 0
	for _, it := range g.World().Items {
		if it.Kind == "fine meal" {
			total += it.Qty
			most = max(most, it.Qty)
		}
	}
	if total != 14 || most != item.FineMeal.StackLimit() {
		t.Fatalf("fine meals: %d in all, biggest stack %d", total, most)
	}
}
