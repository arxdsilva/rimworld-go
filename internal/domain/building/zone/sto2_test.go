package zone

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestNewStorageDefaults: the vanilla defaults (STO-2). A stockpile takes
// everything but corpses, bones and chunks and refuses rotten stacks; a
// dumping stockpile takes corpses, bones and chunks only, rotten too; a
// hopper raw food at Important; a grave human bodies, a sarcophagus only
// colonists'.
func TestNewStorageDefaults(t *testing.T) {
	t.Parallel()
	sp := NewStockpile()
	rotten := item.Stack{Kind: item.Meat, Qty: 1, Rot: item.Meat.ShelfTicks()}
	if !sp.Custom || sp.Priority != PriorityNormal || !sp.Allows(item.Meal) || !sp.Allows(item.Steel) ||
		sp.Allows(item.CorpseAnimal) || sp.Allows(item.CorpseHuman) || sp.Allows(item.Bones) || sp.Allows(item.GraniteChunk) ||
		sp.AllowsStack(rotten) || !sp.AllowsStack(item.Stack{Kind: item.Meat, Qty: 1}) {
		t.Fatalf("new stockpile %+v", sp)
	}
	d := NewDump()
	if !d.Allows(item.CorpseHuman) || !d.Allows(item.CorpseAnimal) || !d.Allows(item.Bones) || !d.Allows(item.SlateChunk) ||
		d.Allows(item.Meal) || d.Allows(item.Wood) || !d.AllowsStack(item.Stack{Kind: item.CorpseAnimal, Qty: 1, Rot: 1 << 30}) {
		t.Fatalf("new dump %+v", d)
	}
	h := NewHopper()
	if !h.Kind.Storage() || h.Kind.Stacks() != 1 || h.Priority != PriorityImportant || !h.Hidden ||
		!h.Allows(item.Potatoes) || !h.Allows(item.Meat) || !h.Allows(item.Egg) || h.Allows(item.Meal) || h.Allows(item.Kibble) {
		t.Fatalf("new hopper %+v", h)
	}
	gr, sar := NewGrave(false), NewGrave(true)
	colonist := item.Stack{Kind: item.CorpseHuman, Qty: 1, Of: 4}
	if gr.Kind.Storage() || !gr.Kind.Settings() || !gr.Allows(item.CorpseHuman) || !gr.Allows(item.Bones) ||
		gr.Allows(item.CorpseAnimal) || !gr.Special.TakesBody(colonist, false) || !gr.Special.TakesBody(colonist, true) {
		t.Fatalf("new grave %+v", gr)
	}
	if !sar.Special.TakesBody(colonist, true) || sar.Special.TakesBody(colonist, false) || !sar.AllowsStack(colonist) {
		t.Fatal("a sarcophagus takes colonists only (stockpile checks ignore the corpse filters)")
	}
}

// TestUniverseClampsEdits: allow all on a hopper or a grave stays within
// what it may hold, and a paste is clamped the same way; RowsFor lists
// only the nodes holding such kinds.
func TestUniverseClampsEdits(t *testing.T) {
	t.Parallel()
	all, _ := Resolve(Root)
	h := NewHopper()
	h.SetAllowed(all, false)
	if h.Effective() != (Filter{}) {
		t.Fatal("clear all empties the filter")
	}
	h.SetAllowed(all, true)
	if h.Effective() != Universe(KindHopper) || h.Allows(item.Wood) {
		t.Fatal("allow all on a hopper is raw food")
	}
	sp := NewStockpile()
	sp.SetAllowed(all, true)
	if !sp.Allows(item.CorpseHuman) || sp.Effective() != sets.storable {
		t.Fatal("allow all on a stockpile is everything storable")
	}
	g := NewGrave(false)
	g.PasteFrom(sp)
	if g.Allows(item.Meal) || !g.Allows(item.CorpseAnimal) {
		t.Fatal("a paste onto a grave keeps only bodies")
	}
	rows := RowsFor(KindHopper, h.Effective().Names())
	for _, r := range rows {
		if !r.On || r.Path == "Raw resources" || r.Path == "Foods/Meals" {
			t.Fatalf("hopper row %+v", r)
		}
	}
	if len(rows) == 0 || len(rows) >= len(Rows(nil)) || len(RowsFor(KindPen, nil)) != 0 {
		t.Fatalf("hopper rows %d of %d", len(rows), len(Rows(nil)))
	}
}

// TestSTO2SpecialFilters: fresh, non-deadman's apparel, smeltable and
// burnable toggles, and their set/names round trip.
func TestSTO2SpecialFilters(t *testing.T) {
	t.Parallel()
	fresh := item.Stack{Kind: item.Meat, Qty: 1}
	clean := item.Stack{Kind: item.Parka, Qty: 1}
	tainted := item.Stack{Kind: item.Parka, Qty: 1, Of: 3}
	steel := item.Stack{Kind: item.Longsword, Qty: 1, Stuff: item.Steel}
	cases := []struct {
		i      int
		refuse []item.Stack
		take   []item.Stack
	}{
		{SpecialFresh, []item.Stack{fresh}, []item.Stack{{Kind: item.Wood, Qty: 1}, {Kind: item.Meat, Qty: 1, Rot: 1 << 30}}},
		{SpecialClean, []item.Stack{clean}, []item.Stack{tainted, fresh}},
		{SpecialSmeltable, []item.Stack{steel}, []item.Stack{clean, {Kind: item.Wood, Qty: 1}}},
		{SpecialNonSmeltable, []item.Stack{clean, {Kind: item.Wood, Qty: 1}}, []item.Stack{steel}},
		{SpecialBurnable, []item.Stack{{Kind: item.Wood, Qty: 1}, clean}, []item.Stack{{Kind: item.Steel, Qty: 1}}},
		{SpecialNonBurnable, []item.Stack{{Kind: item.Steel, Qty: 1}}, []item.Stack{{Kind: item.Wood, Qty: 1}}},
	}
	for _, c := range cases {
		s := Special{}.Set(c.i, false, 0, 0)
		for _, st := range c.refuse {
			if s.Takes(st) {
				t.Errorf("special %d takes %+v", c.i, st)
			}
		}
		for _, st := range c.take {
			if !s.Takes(st) {
				t.Errorf("special %d refuses %+v", c.i, st)
			}
		}
		if len(s.Names()) != len(Toggles())-1 || s.Set(c.i, true, 0, 0) != (Special{}) {
			t.Errorf("special %d names %v", c.i, s.Names())
		}
	}
	if (Special{NoColonist: true, NoStranger: true}).Takes(item.Stack{Kind: item.CorpseHuman, Qty: 1}) == false {
		t.Fatal("the corpse filters only apply through TakesBody")
	}
}

// TestStorageGroups: linking shares the first zone's settings, an edit
// to any member reaches all, unlinking keeps the settings, a group of
// one dissolves, and only building zones of one kind link.
func TestStorageGroups(t *testing.T) {
	t.Parallel()
	m := NewMap(12, 12)
	a := m.Paint(KindShelf, NewShelf(), RectOf(1, 1, 2, 1), nil)
	b := m.Paint(KindShelf, NewShelf(), RectOf(4, 1, 5, 1), nil)
	c := m.Paint(KindShelf, NewShelf(), RectOf(7, 1, 8, 1), nil)
	hop := m.Paint(KindHopper, NewHopper(), RectOf(1, 4, 1, 4), nil)
	sp := m.Paint(KindStockpile, NewStockpile(), RectOf(1, 7, 3, 8), nil)
	m.Update(a, func(z *Zone) { z.Priority, z.Name = PriorityCritical, "Armory" })
	if !m.Link(b, a) || !m.Link(c, a) || m.Link(hop, a) || m.Link(sp, a) || m.Link(a, a) || m.Link(b, a) {
		t.Fatal("link rules")
	}
	zb, _ := m.Zone(b)
	if zb.Group != a || zb.Priority != PriorityCritical || zb.Name != "Armory" {
		t.Fatalf("linked member %+v", zb)
	}
	wood, _ := Resolve("Raw resources/Wood")
	m.Update(c, func(z *Zone) { z.SetAllowed(wood, false) })
	for _, id := range []ID{a, b, c} {
		if z, _ := m.Zone(id); z.Allows(item.Wood) || z.Group != a {
			t.Fatalf("an edit to one member reaches %d: %+v", id, z)
		}
	}
	if !m.SameStore(c, a) || !m.SameStore(a, a) || m.SameStore(sp, a) || m.SameStore(0, 0) {
		t.Fatal("same store")
	}
	if len(m.Members(a)) != 3 || !m.Unlink(b) || m.Unlink(b) {
		t.Fatal("unlink")
	}
	if z, _ := m.Zone(b); z.Group != 0 || z.Priority != PriorityCritical {
		t.Fatalf("an unlinked zone keeps its settings: %+v", z)
	}
	m.Unlink(c)
	if z, _ := m.Zone(a); z.Group != 0 {
		t.Fatal("a group of one dissolves")
	}
	// building zones ignore the zone tools; the group survives a save
	m.Link(b, a)
	m.Clear(RectOf(0, 0, 11, 11))
	saved := Import(m.Export())
	if _, ok := saved.Zone(hop); !ok {
		t.Fatal("clear left the hopper alone")
	}
	if z, _ := saved.Zone(b); z.Group != a {
		t.Fatal("the group round-trips")
	}
}
