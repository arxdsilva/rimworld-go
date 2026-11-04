package game

import (
	"os"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// neatKinds is the 27 storage pieces, in profile order.
func neatKinds(t testing.TB) []construction.Kind {
	t.Helper()
	var out []construction.Kind
	for k := construction.DisplayCase; k.IsNeatStorage(); k++ {
		out = append(out, k)
	}
	if len(out) != 27 {
		t.Fatalf("%d pieces", len(out))
	}
	return out
}

// firstTaken is the first item kind z takes, in filter-tree order.
func firstTaken(t testing.TB, z zone.Zone) item.Kind {
	t.Helper()
	for _, n := range zone.Tree() {
		if n.Leaf && z.Allows(n.Kind) {
			return n.Kind
		}
	}
	t.Fatalf("zone %+v takes nothing", z)
	return 0
}

// setDown spawns a full stack of k on (x, y), as a hauler sets one down.
func setDown(t testing.TB, g *Game, k item.Kind, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnStack(k, k.StackLimit(), x, y)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(id)
	g.mergeOrSet(id, st)
	return id
}

// TestNeatStorageHauling: each of the 27 pieces accepts its set and refuses the
// rest, fills to its slots, shelters what it holds and shows every stack's slot;
// the next stack goes elsewhere, and one dropped on a full cell spills.
func TestNeatStorageHauling(t *testing.T) {
	t.Parallel()
	for _, k := range neatKinds(t) {
		t.Run(k.String(), func(t *testing.T) {
			t.Parallel()
			g := New(Config{Seed: 1, Width: 24, Height: 12})
			bid := buildNow(t, g, k, construction.DefaultStuff(k), construction.South, 10, 6)
			v := zoneViewAt(t, g, 10, 6)
			z, _ := g.storageAt(10, 6)
			name := construction.DefOf(k).Name
			if v.Kind != ZoneShelf || v.Profile != name || v.Shelf != uint64(bid) || !v.Hidden || !z.Kind.Building() {
				t.Fatalf("zone %+v", v)
			}
			if p, ok := construction.NeatProfile(k); !ok || int(z.Profile) != p {
				t.Fatalf("profile %d, want %d", z.Profile, p)
			}
			ceiling := zone.UniverseOf(z.Kind, z.Profile)
			for _, n := range zone.Tree() {
				if !n.Leaf {
					continue
				}
				_, _, ok := g.stockpileCell(item.Stack{Kind: n.Kind, Qty: 1}, 10, 6)
				if ok != ceiling.Has(n.Kind) {
					t.Errorf("%s: stockpileCell says %v for %s", name, ok, n.Kind)
				}
			}
			rows := ProfileFilterRows(v.Profile, v.Filter)
			if len(rows) == 0 || len(rows) >= len(StorageFilterRows(ZoneStockpile, nil)) {
				t.Errorf("%s: %d window rows", name, len(rows))
			}

			fill := firstTaken(t, z)
			cells, _, _ := construction.Footprint(k, construction.South, 10, 6)
			room := z.Slots() * len(cells)
			if b := buildingView(g, bid); b.Capacity != room || b.Stored != 0 {
				t.Fatalf("%s: readout %d/%d, want 0/%d", name, b.Stored, b.Capacity, room)
			}
			for i := range room {
				x, y, ok := g.stockpileCell(item.Stack{Kind: fill, Qty: 1}, 10, 6)
				if !ok || !slices.Contains(cells, [2]int{x, y}) {
					t.Fatalf("%s: stack %d of %d goes to (%d, %d, %v)", name, i, room, x, y, ok)
				}
				setDown(t, g, fill, x, y)
			}
			if b := buildingView(g, bid); b.Stored != room {
				t.Fatalf("%s: holds %d of %d", name, b.Stored, room)
			}
			seen := map[[3]int]bool{}
			for _, it := range g.World().Items {
				slot := [3]int{it.X, it.Y, int(it.Slot)}
				if it.Store != uint8(k) || int(it.Slot) >= z.Slots() || seen[slot] {
					t.Fatalf("%s: item view %+v", name, it)
				}
				seen[slot] = true
			}
			if len(seen) != room {
				t.Fatalf("%s: %d slots shown of %d", name, len(seen), room)
			}
			for _, c := range cells {
				if (craftWeather{g}).Exposed(c[0], c[1]) || !g.onShelf(c[0], c[1]) {
					t.Fatalf("%s: (%d, %d) doesn't shelter", name, c[0], c[1])
				}
			}
			if x, y, ok := g.stockpileCell(item.Stack{Kind: fill, Qty: 1}, 10, 6); ok {
				t.Fatalf("%s: a full piece takes more: (%d, %d)", name, x, y)
			}
			// the next goes elsewhere
			set, _ := zone.Resolve(zone.Root)
			pile := g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(1, 1, 3, 3), nil)
			g.zones.Update(pile, func(z *zone.Zone) { z.SetAllowed(set, true) })
			if x, y, ok := g.stockpileCell(item.Stack{Kind: fill, Qty: 1}, 10, 6); !ok || x > 3 || y > 3 {
				t.Fatalf("%s: the next stack goes to (%d, %d, %v), want the stockpile", name, x, y, ok)
			}
			// one dropped on a full cell spills
			id := setDown(t, g, fill, cells[0][0], cells[0][1])
			if st, _ := g.stacks.Get(id); slices.Contains(cells, [2]int{st.X, st.Y}) {
				t.Fatalf("%s: a dropped stack stayed on the full piece", name)
			}
			if b := buildingView(g, bid); b.Stored != room {
				t.Fatalf("%s: holds %d after the spill, want %d", name, b.Stored, room)
			}
		})
	}
}

// TestNeatStorageHaulersFill: a hauler carries twelve bodies to a corpse dump
// spot (a rotten one too), and the thirteenth stays out; a crate takes a stack
// from a stockpile only once nothing above it has room.
func TestNeatStorageHaulersFill(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	spot := buildNow(t, g, construction.CorpseDumpSpot, item.Wood, construction.South, 5, 5)
	var bodies []entity.EntityID
	for i := range 13 {
		id, err := g.SpawnStack(item.CorpseAnimal, 1, 10+i, 10)
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, id)
	}
	rotten, _ := g.stacks.Get(bodies[3])
	rotten.Rot = rotten.Shelf() + 1
	g.putStack(bodies[3], rotten)
	ada, _ := g.SpawnNamedPawn(9, 9, "Ada")
	onlyWork(g, ada, work.Haul)
	for i := 0; i < 900 && countAt(g, 5, 5) < 12; i++ {
		runFed(g, 1, ada)
	}
	runFed(g, 30, ada)
	if n := countAt(g, 5, 5); n != 12 {
		t.Fatalf("the dump spot holds %d bodies, want 12", n)
	}
	if st, ok := g.stacks.Get(bodies[3]); !ok || st.X != 5 || st.Y != 5 {
		t.Fatalf("the rotten body lies at (%d, %d) (%v)", st.X, st.Y, ok)
	}
	out := 0
	for _, id := range bodies {
		if st, ok := g.stacks.Get(id); !ok {
			t.Fatalf("body %d went away", id)
		} else if st.X != 5 || st.Y != 5 {
			out++
		}
	}
	if out != 1 || buildingView(g, spot).Stored != 12 {
		t.Fatalf("%d bodies left out, want 1; stored %d", out, buildingView(g, spot).Stored)
	}
	// a thirteenth dropped on the spot spills to a tile beside it
	body := setDown(t, g, item.CorpseAnimal, 5, 5)
	if st, _ := g.stacks.Get(body); st.X == 5 && st.Y == 5 {
		t.Fatal("a thirteenth stack lies on the spot")
	}
	if countAt(g, 5, 5) != 12 {
		t.Fatal("the spill moved the spot's count")
	}
}

// TestNoFlapStorage: a stack set down in a crate because the better piece was
// full stays its minimum time after room opens, then moves up once and never
// comes back.
func TestNoFlapStorage(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	buildNow(t, g, construction.RoundPedestal, item.Wood, construction.South, 4, 4) // Preferred, one slot
	buildNow(t, g, construction.SmallCrate, item.Wood, construction.South, 20, 4)   // Normal
	gold := setDown(t, g, item.Gold, 4, 4)
	spawn(t, g, item.Steel, 50, 12, 8)
	ada, _ := g.SpawnNamedPawn(12, 10, "Ada")
	onlyWork(g, ada, work.Haul)
	where := func() (x, y int, carried bool) {
		for _, id := range g.itemIDs {
			if st := g.stacks.Ref(id); st != nil && st.Kind == item.Steel {
				return st.X, st.Y, st.Carrier != 0
			}
		}
		t.Fatal("the steel went away")
		return 0, 0, false
	}
	for i := 0; ; i++ {
		x, y, carried := where()
		if x == 20 && y == 4 && !carried {
			break
		}
		if i > 400 {
			t.Fatalf("the steel never reached the crate: (%d, %d)", x, y)
		}
		runFed(g, 1, ada)
	}
	set := uint64(g.clock.Now())
	g.deleteItem(gold) // room opens above at once
	for uint64(g.clock.Now()) < set+rehaulMinStay-TicksPerCell*8 {
		runFed(g, 1, ada)
		if x, y, carried := where(); x != 20 || y != 4 || carried {
			t.Fatalf("moved up at tick %d of the %d minimum stay", uint64(g.clock.Now())-set, rehaulMinStay)
		}
	}
	moves, last := 0, [2]int{20, 4}
	for range 900 {
		runFed(g, 1, ada)
		x, y, carried := where()
		if carried {
			continue
		}
		if at := [2]int{x, y}; at != last {
			moves++
			last = at
		}
	}
	if moves != 1 || last != [2]int{4, 4} {
		t.Fatalf("%d moves, ending at %v; want one move, to the pedestal", moves, last)
	}
}

// TestNeatStorageSettings: the filter commands stay inside a piece's ceiling,
// unequal pieces don't link, and a paste from a shelf is clamped.
func TestNeatStorageSettings(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 16})
	rack := buildNow(t, g, construction.WeaponRack, item.Wood, construction.South, 2, 2)
	rack2 := buildNow(t, g, construction.WeaponRack, item.Wood, construction.South, 2, 4)
	shelf := buildNow(t, g, construction.Shelf, item.Wood, construction.South, 2, 6)
	rv := zoneViewAt(t, g, 2, 2)
	ceiling := zone.UniverseOf(zone.KindShelf, zone.ProfileWeaponRack)
	applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(rv.ID), Name: FilterRoot, Enabled: true})
	if z, _ := g.storageAt(2, 2); z.Effective() != ceiling {
		t.Fatalf("allow all on a rack: %v", z.Effective().Names())
	}
	applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(rv.ID), Name: "wood", Enabled: true})
	if z, _ := g.storageAt(2, 2); z.Allows(item.Wood) {
		t.Fatal("a rack took wood by command")
	}
	sv := zoneViewAt(t, g, 2, 6)
	applyNow(g, command.Command{Kind: command.PasteStockpile, Target: uint64(rv.ID), Zone: uint64(sv.ID)})
	if z, _ := g.storageAt(2, 2); z.Effective() != ceiling {
		t.Fatalf("a shelf's filter pasted on a rack: %v", z.Effective().Names())
	}
	applyNow(g, command.Command{Kind: command.LinkStorage, Target: uint64(shelf), Zone: uint64(rack)})
	applyNow(g, command.Command{Kind: command.LinkStorage, Target: uint64(rack), Zone: uint64(shelf)})
	if z, _ := g.storageAt(2, 2); z.Group != 0 {
		t.Fatalf("a rack linked to a shelf: %+v", z)
	}
	applyNow(g, command.Command{Kind: command.LinkStorage, Target: uint64(rack2), Zone: uint64(rack)})
	if z, _ := g.storageAt(2, 4); z.Group == 0 {
		t.Fatal("two racks refuse to link")
	}
}

// TestShelterStaysAsItWas: hoppers and graves still leave what lies on them to
// the weather; shelves, small shelves and bookcases still shelter it.
func TestShelterStaysAsItWas(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		k       construction.Kind
		stuff   item.Kind
		shelter bool
	}{
		{construction.Shelf, item.Wood, true},
		{construction.SmallShelf, item.Wood, true},
		{construction.EggBox, item.Wood, true},
		{construction.Bookcase, item.Wood, true},
		{construction.SmallBookcase, item.Wood, true},
		{construction.Hopper, item.Steel, false},
		{construction.Grave, item.Wood, false},
		{construction.Sarcophagus, item.Wood, false},
	} {
		g := New(Config{Seed: 1, Width: 24, Height: 12})
		buildNow(t, g, c.k, c.stuff, construction.South, 10, 6)
		if g.onShelf(10, 6) != c.shelter || (craftWeather{g}).Exposed(10, 6) == c.shelter {
			t.Errorf("%s: shelters %v, want %v", c.k, g.onShelf(10, 6), c.shelter)
		}
		if v := zoneViewAt(t, g, 10, 6); v.Profile != "" {
			t.Errorf("%s: profile %q", c.k, v.Profile)
		}
	}
}

// TestNeatItemViews: a stack's Store and Slot follow its piece and cell; a
// plain shelf's, a loose and a carried stack read zero, and the memoised list
// sees them change.
func TestNeatItemViews(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	buildNow(t, g, construction.Crate, item.Wood, construction.South, 10, 6)
	buildNow(t, g, construction.Shelf, item.Wood, construction.South, 4, 6)
	a, b := setDown(t, g, item.Wood, 10, 6), setDown(t, g, item.Steel, 10, 6)
	c := setDown(t, g, item.Cloth, 11, 6)
	shelved := setDown(t, g, item.Wood, 4, 6)
	loose := setDown(t, g, item.Wood, 20, 2)
	view := func(id entity.EntityID) ItemView {
		for _, it := range g.World().Items {
			if it.ID == id {
				return it
			}
		}
		t.Fatalf("no view of stack %d", id)
		return ItemView{}
	}
	crate := uint8(construction.Crate)
	for _, w := range []struct {
		id          entity.EntityID
		store, slot uint8
	}{{a, crate, 0}, {b, crate, 1}, {c, crate, 0}, {shelved, 0, 0}, {loose, 0, 0}} {
		if v := view(w.id); v.Store != w.store || v.Slot != w.slot {
			t.Errorf("stack %d: store %d slot %d, want %d %d", w.id, v.Store, v.Slot, w.store, w.slot)
		}
	}
	if v := view(a); sameItemView(v, ItemView{ID: v.ID}) || sameItemView(ItemView{Store: 1}, ItemView{}) || sameItemView(ItemView{Slot: 1}, ItemView{}) {
		t.Fatal("the memoised list misses Store or Slot")
	}
	g.deleteItem(a) // the next stack moves down a slot
	if v := view(b); v.Slot != 0 {
		t.Fatalf("after the first stack went: slot %d", v.Slot)
	}
	st, _ := g.stacks.Get(b)
	st.Carrier = 99
	g.putStack(b, st)
	if v := view(b); v.Store != 0 || v.Slot != 0 {
		t.Fatalf("a carried stack: %+v", v)
	}
}

// TestNeatStorageRoundTrip: 27 filled pieces save and load with the same stacks,
// profiles, settings and hash; an older save loads with no profile anywhere.
func TestNeatStorageRoundTrip(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 60, Height: 40})
	type piece struct {
		k    construction.Kind
		x, y int
		bid  entity.EntityID
	}
	var pieces []piece
	for i, k := range neatKinds(t) {
		x, y := 2+(i%9)*6, 2+(i/9)*6
		pieces = append(pieces, piece{k, x, y, buildNow(t, g, k, construction.DefaultStuff(k), construction.South, x, y)})
		z, _ := g.storageAt(x, y)
		for range z.Slots() {
			setDown(t, g, firstTaken(t, z), x, y)
		}
		if i%2 == 0 {
			g.zones.Update(z.ID, func(z *zone.Zone) { z.Priority = zone.PriorityCritical })
		}
	}
	zoneCrate, _ := g.storageAt(pieces[23].x, pieces[23].y)
	applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(zoneCrate.ID), Name: "wood", Enabled: false})
	g.Step(1) // the room and roof grids exist once it has ticked
	h := saveLoad(t, g)
	if gh, hh := g.StateHash(), h.StateHash(); gh != hh {
		t.Fatalf("state hash %#x before the save, %#x after the load", gh, hh)
	}
	for _, p := range pieces {
		a, _ := g.storageAt(p.x, p.y)
		b, ok := h.storageAt(p.x, p.y)
		if want, _ := construction.NeatProfile(p.k); !ok || a != b || int(b.Profile) != want {
			t.Errorf("%s: zone %+v, was %+v", p.k, b, a)
		}
		if ga, gb := buildingView(g, p.bid), buildingView(h, p.bid); ga.Stored != gb.Stored || ga.Capacity != gb.Capacity || gb.Stored != b.Slots() {
			t.Errorf("%s: readout %d/%d, was %d/%d", p.k, gb.Stored, gb.Capacity, ga.Stored, ga.Capacity)
		}
		if name := zoneViewAt(t, h, p.x, p.y).Profile; name != construction.DefOf(p.k).Name {
			t.Errorf("%s: profile %q after a load", p.k, name)
		}
	}
	if !slices.EqualFunc(g.World().Items, h.World().Items, sameItemView) {
		t.Fatal("the item views changed over a save")
	}
	if z, _ := h.storageAt(pieces[23].x, pieces[23].y); z.Allows(item.Wood) {
		t.Fatal("a piece's edited filter was lost")
	}

	f, err := os.Open(saveV12Fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old, _, err := Load(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range old.zones.Zones() {
		if z.Profile != zone.ProfileNone {
			t.Errorf("zone %d of a v12 save has profile %d", z.ID, z.Profile)
		}
	}
}

// TestNeatProfileMatchesConstruction: the zone package's constants and the
// construction kinds name the same 27 pieces in the same order.
func TestNeatProfileMatchesConstruction(t *testing.T) {
	t.Parallel()
	for i, k := range neatKinds(t) {
		p := zone.Profile(i + 1)
		got, ok := store.ProfileKind(p)
		if !ok || got != k {
			t.Errorf("profile %d: kind %v, want %s", p, got, k)
		}
		if zk, z, ok := store.ZoneKind(k); !ok || zk != zone.KindShelf || z.Profile != p {
			t.Errorf("%s: %v %+v %v", k, zk, z, ok)
		}
	}
}
