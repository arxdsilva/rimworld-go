package store

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// TestUrgentMarks: marks by id and by rectangle, Keep drops refused ones,
// and the marks round-trip sorted and unique.
func TestUrgentMarks(t *testing.T) {
	t.Parallel()
	w := newWorld()
	a, b, c := w.put(item.Wood, 5, 2, 2), w.put(item.Steel, 5, 4, 4), w.put(item.Meal, 1, 9, 9)
	var u Urgent
	if u.Any() || u.Export() != nil {
		t.Fatal("empty")
	}
	u.MarkRect(w, 5, 5, 1, 1, true) // corners in any order
	if !u.Has(a) || !u.Has(b) || u.Has(c) || len(u.IDs()) != 2 {
		t.Fatalf("rect marks %v", u.IDs())
	}
	u.Mark(c, true)
	u.Mark(c, true)
	u.MarkRect(w, 2, 2, 2, 2, false)
	if u.Has(a) || !u.Has(c) || len(u.IDs()) != 2 {
		t.Fatalf("after unmark %v", u.IDs())
	}
	u.Keep(func(id entity.EntityID) bool { return id != b })
	var v Urgent
	v.Import([]entity.EntityID{c, c, 3})
	if len(v.IDs()) != 2 || !v.Has(3) || !v.Has(c) || len(u.Export()) != 1 {
		t.Fatalf("import %v export %v", v.IDs(), u.Export())
	}
	s := Store{Urgent: v}
	var r Store
	r.Import(s.Export())
	if !r.Urgent.Has(c) {
		t.Fatal("store round trip")
	}
	v.Import(nil)
	if v.Any() {
		t.Fatal("import none")
	}
}

// dispenserWorld is a powered dispenser at (10, 10) (3x4, spot (11, 14))
// with hoppers on its west and east sides.
func dispenserWorld(t *testing.T) (*world, entity.EntityID) {
	t.Helper()
	w := newWorld()
	d := w.build(construction.NutrientPasteDispenser, 10, 10)
	return w, d
}

// TestHopperOrder: southwest first, then counterclockwise, never the
// spot, at most MaxHoppers.
func TestHopperOrder(t *testing.T) {
	t.Parallel()
	w, d := dispenserWorld(t)
	for _, c := range [][2]int{{9, 10}, {13, 13}, {10, 14}, {11, 9}, {12, 14}} {
		w.build(construction.Hopper, c[0], c[1])
	}
	w.build(construction.Hopper, 11, 14) // on the spot: ignored
	hs, n := Paste{}.Hoppers(w, w.bldgs[d])
	want := [][2]int{{10, 14}, {12, 14}, {13, 13}, {11, 9}, {9, 10}}
	if n != len(want) {
		t.Fatalf("%d hoppers %v", n, hs[:n])
	}
	for i, c := range want {
		if hs[i] != c {
			t.Fatalf("hopper %d at %v, want %v (%v)", i, hs[i], c, hs[:n])
		}
	}
	w2, d2 := dispenserWorld(t)
	for x := 9; x <= 13; x++ {
		for y := 9; y <= 14; y++ {
			if x == 9 || x == 13 || y == 9 || y == 14 {
				w2.build(construction.Hopper, x, y)
			}
		}
	}
	if _, n := (Paste{}).Hoppers(w2, w2.bldgs[d2]); n != MaxHoppers {
		t.Fatalf("a ring of hoppers feeds %d, want %d (corners never touch a side)", n, MaxHoppers)
	}
}

// TestDispense: a meal takes PasteCost from the hoppers in draw order, a
// short hopper is emptied and the next tried; too little in all of them
// takes nothing; an unpowered dispenser makes nothing.
func TestDispense(t *testing.T) {
	t.Parallel()
	w, d := dispenserWorld(t)
	w.build(construction.Hopper, 10, 14)
	w.build(construction.Hopper, 9, 10)
	first := w.put(item.Meat, 1, 10, 14) // 2200: one meat makes a meal alone
	second := w.put(item.Potatoes, 75, 9, 10)
	p := Paste{}
	if food := p.Food(w, w, w.bldgs[d]); food != item.Meat.Nutrition()+75*item.Potatoes.Nutrition() {
		t.Fatalf("food %d", food)
	}
	if !p.Dispense(w, w, d) {
		t.Fatal("dispense")
	}
	if _, ok := w.stacks[first]; ok || w.stacks[second].Qty != 75 {
		t.Fatalf("the first hopper pays first: %+v %+v", w.stacks[first], w.stacks[second])
	}
	before := w.stacks[second].Qty
	p.Dispense(w, w, d)
	per := (PasteCost + item.Potatoes.Nutrition() - 1) / item.Potatoes.Nutrition()
	if w.stacks[second].Qty != before-per {
		t.Fatalf("potatoes %d, want %d", w.stacks[second].Qty, before-per)
	}
	w.dark[d] = true
	if p.Dispense(w, w, d) || p.Ready(w, w) {
		t.Fatal("no power, no paste")
	}
	w.dark[d] = false
	w.SetStack(second, item.Stack{Kind: item.Potatoes, Qty: per - 1, X: 9, Y: 10})
	if p.Dispense(w, w, d) || w.stacks[second].Qty != per-1 {
		t.Fatal("too little food takes nothing")
	}
	w.SetStack(second, item.Stack{Kind: item.Meal, Qty: 9, X: 9, Y: 10}) // not raw food
	if p.Ready(w, w) {
		t.Fatal("a meal in a hopper is not paste food")
	}
}

// TestDispensePasteJob: a hungry colonist finds the nearest usable
// dispenser, works DispenseWork ticks at the spot, and then eats the meal
// it made (claimed, dispenser released).
func TestDispensePasteJob(t *testing.T) {
	t.Parallel()
	w, d := dispenserWorld(t)
	w.build(construction.Hopper, 10, 14)
	w.put(item.Meat, 3, 10, 14)
	pawn := entity.EntityID(7)
	w.pos[pawn] = [2]int{11, 20}
	p := Paste{}
	bid, sx, sy, ok := p.Find(w, w, w, pawn)
	if !ok || bid != d || sx != 11 || sy != 14 {
		t.Fatalf("find %d (%d,%d) %v", bid, sx, sy, ok)
	}
	j := p.Job(bid, sx, sy)
	j.State = job.Do
	w.Claim(pawn, d)
	w.pos[pawn] = [2]int{sx, sy}
	for range DispenseWork {
		p.Do(w, w, w, pawn, j)
		j = w.jobs[pawn]
	}
	if j.Kind != job.Eat || j.State != job.Do || w.stacks[j.Target].Kind != item.NutrientPasteMeal || w.claims[j.Target] != pawn {
		t.Fatalf("after dispensing: %+v %+v", j, w.stacks[j.Target])
	}
	if _, held := w.claims[d]; held || w.stacks[w.ItemAt(10, 14)].Qty != 2 {
		t.Fatal("the dispenser is free again and one meat was used")
	}
	w.walls = true
	if _, _, _, ok := p.Find(w, w, w, pawn); ok {
		t.Fatal("out of reach")
	}
	w.walls = false
	w.SetStack(w.ItemAt(10, 14), item.Stack{Kind: item.Meat, Qty: 0, X: 10, Y: 14})
	j = p.Job(d, sx, sy)
	j.WorkLeft = 1
	p.Do(w, w, w, pawn, j)
	if w.jobs[pawn].State != job.Fail {
		t.Fatal("a dry dispenser fails the job")
	}
}

// TestStorageBuildings: each storage building lays its zone with its
// defaults; a moved one keeps its settings; links share settings; the
// readouts count stacks against capacity; lifting takes it out of its
// group; graves pick by their settings; LayMissing fills an old save's.
func TestStorageBuildings(t *testing.T) {
	t.Parallel()
	w := newWorld()
	var s Buildings
	sh1, sh2 := w.build(construction.Shelf, 1, 1), w.build(construction.Shelf, 4, 1)
	hop := w.build(construction.Hopper, 8, 1)
	gr := w.build(construction.Grave, 1, 5)
	sar := w.build(construction.Sarcophagus, 4, 5)
	if _, _, ok := ZoneKind(construction.Wall); ok {
		t.Fatal("a wall stores nothing")
	}
	z1, ok := s.ZoneOf(w.zs, w, sh1)
	hz, _ := s.ZoneOf(w.zs, w, hop)
	gz, _ := s.ZoneOf(w.zs, w, gr)
	if !ok || z1.Kind != zone.KindShelf || hz.Kind != zone.KindHopper || gz.Kind != zone.KindGrave || s.Of(w.zs, w, z1) != sh1 {
		t.Fatalf("zones %+v %+v %+v", z1, hz, gz)
	}
	w.zs.Update(z1.ID, func(z *zone.Zone) { z.Priority = zone.PriorityCritical })
	if !s.Link(w.zs, w, sh2, sh1) || s.Link(w.zs, w, hop, sh1) {
		t.Fatal("link rules")
	}
	z2, _ := s.ZoneOf(w.zs, w, sh2)
	if z2.Priority != zone.PriorityCritical || z2.Group == 0 {
		t.Fatalf("linked shelf %+v", z2)
	}
	w.put(item.Steel, 10, 1, 1)
	w.put(item.Wood, 10, 1, 1)
	z1, _ = s.ZoneOf(w.zs, w, sh1)
	if g, n, c := s.Fill(w.zs, w, z1, false); g != uint64(z2.Group) || n != 2 || c != 2*zone.ShelfStacks {
		t.Fatalf("shelf readout %d %d/%d", g, n, c)
	}
	if _, n, c := s.Fill(w.zs, w, gz, true); n != 1 || c != 1 {
		t.Fatal("a filled grave holds its one body")
	}
	if g, n, c := s.Fill(w.zs, w, zone.Zone{Kind: zone.KindGrow}, false); g+uint64(n+c) != 0 {
		t.Fatal("a grow zone has no readout")
	}
	colonist := item.Stack{Kind: item.CorpseHuman, Qty: 1, Of: 3}
	if _, ok := s.Grave(w.zs, w, gr, colonist, false); !ok {
		t.Fatal("a grave takes a stranger")
	}
	if _, ok := s.Grave(w.zs, w, sar, colonist, false); ok {
		t.Fatal("a sarcophagus takes colonists only")
	}
	if _, ok := s.Grave(w.zs, w, sh1, colonist, true); ok {
		t.Fatal("a shelf is no grave")
	}
	carcass := item.Stack{Kind: item.CorpseAnimal, Qty: 1}
	none := func(entity.EntityID) bool { return false }
	if s.AnyGrave(w.zs, w, carcass, false, none) || !s.AnyGrave(w.zs, w, colonist, true, none) ||
		s.AnyGrave(w.zs, w, colonist, true, func(entity.EntityID) bool { return true }) {
		t.Fatal("any grave")
	}
	// move the first shelf: its zone (and group) go along
	b := w.bldgs[sh1]
	b.X = 1
	b.Y = 10
	w.bldgs[sh1] = b
	cells, _, _ := construction.Footprint(construction.Shelf, construction.South, 1, 10)
	s.Lay(w.zs, w, construction.Shelf, cells)
	if mz, ok := s.ZoneOf(w.zs, w, sh1); !ok || mz.Priority != zone.PriorityCritical || mz.Group != z2.Group {
		t.Fatalf("a moved shelf keeps its settings: %+v", mz)
	}
	if !s.Unlink(w.zs, w, sh2) || s.Unlink(w.zs, w, sh2) {
		t.Fatal("unlink")
	}
	s.Lift(w.zs, w.bldgs[sh1])
	s.Lift(w.zs, construction.Building{Kind: construction.Wall})
	if _, ok := s.ZoneOf(w.zs, w, sh1); ok {
		t.Fatal("lifted")
	}
	// an older save's grave without a zone
	w.zs.RemoveTiles(w.zs.Cells(gz.ID))
	if _, ok := s.Grave(w.zs, w, gr, colonist, true); !ok {
		t.Fatal("a grave without a zone uses the defaults")
	}
	s.LayMissing(w.zs, w)
	if _, ok := s.ZoneOf(w.zs, w, gr); !ok {
		t.Fatal("LayMissing lays the grave's zone")
	}
}

// TestSearch: the search fills the highest priority first, tops up a
// stack before a free cell, honours reservations, blockers and keep, and
// refills a hopper only once it is down to half.
func TestSearch(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.zs.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(1, 1, 3, 1), nil)
	hi := w.zs.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(10, 10, 11, 10), nil)
	w.zs.Update(hi, func(z *zone.Zone) { z.Priority = zone.PriorityPreferred })
	wood := item.Stack{Kind: item.Wood, Qty: 10}
	if x, y, ok := Search(w, wood, 1, 1, nil, zone.PriorityLow-1, false); !ok || x != 10 || y != 10 {
		t.Fatalf("highest priority first: %d,%d", x, y)
	}
	w.reserve[[2]int{10, 10}] = true
	w.blocked[[2]int{11, 10}] = true
	if x, _, ok := Search(w, wood, 1, 1, nil, zone.PriorityLow-1, false); !ok || x != 1 {
		t.Fatalf("reserved and blocked cells are skipped: %d", x)
	}
	w.put(item.Wood, 5, 3, 1)
	if x, _, ok := Search(w, wood, 1, 1, nil, zone.PriorityNormal, false); ok {
		t.Fatalf("above Normal there is no room: %d", x)
	}
	if x, _, ok := Search(w, wood, 1, 1, nil, zone.PriorityLow-1, false); !ok || x != 3 {
		t.Fatalf("a top-up beats a nearer free cell: %d", x)
	}
	if x, _, ok := Search(w, wood, 1, 1, func(x, y int) bool { return x == 2 }, zone.PriorityLow-1, true); !ok || x != 2 {
		t.Fatalf("keep: %d", x)
	}
	if HasHopper(w) {
		t.Fatal("no hopper yet")
	}
	w.build(construction.Hopper, 20, 20)
	food := w.put(item.Potatoes, 40, 20, 20)
	potato := item.Stack{Kind: item.Potatoes, Qty: 5}
	if !HasHopper(w) {
		t.Fatal("a hopper")
	}
	if x, _, ok := Search(w, potato, 20, 20, nil, zone.PriorityPreferred, false); ok {
		t.Fatalf("a hopper over half full takes no top-up: %d", x)
	}
	w.SetStack(food, item.Stack{Kind: item.Potatoes, Qty: 37, X: 20, Y: 20})
	if x, y, ok := Search(w, potato, 1, 1, nil, zone.PriorityPreferred, false); !ok || x != 20 || y != 20 {
		t.Fatal("a hopper at half is refilled")
	}
	w.build(construction.Shelf, 15, 15)
	steel := item.Stack{Kind: item.Steel, Qty: 5}
	for range zone.ShelfStacks {
		w.put(item.Steel, 75, 15, 15)
	}
	if x, y, ok := Search(w, steel, 15, 15, func(x, y int) bool { return y == 15 }, zone.PriorityNormal, false); !ok || x != 16 || y != 15 {
		t.Fatalf("a full shelf cell is skipped: %d,%d", x, y)
	}
}
