package zone

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// allKinds is every known item kind.
func allKinds() []item.Kind {
	var out []item.Kind
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		out = append(out, k)
	}
	return out
}

// TestFilterTreeFilesEveryStorableKind: every kind sits under exactly
// one category (STO-2: human corpses and bones too), categories precede their
// children, and each category's Kinds is the union of what lies under it.
func TestFilterTreeFilesEveryStorableKind(t *testing.T) {
	t.Parallel()
	seen := map[item.Kind]int{}
	for _, c := range categories() {
		for _, k := range c.kinds {
			seen[k]++
		}
	}
	for _, k := range allKinds() {
		want := 1
		if seen[k] != want || Storable(k) != (want == 1) {
			t.Errorf("%s filed %d times (storable %v), want %d", k, seen[k], Storable(k), want)
		}
	}
	tree := Tree()
	paths := map[string]bool{}
	for i, n := range tree {
		if n.Leaf {
			if n.Kinds != filterOf(n.Kind) || n.Path != n.Kind.String() {
				t.Errorf("leaf %+v", n)
			}
			continue
		}
		if parent, _, ok := strings.Cut(n.Path, "/"); ok && !paths[parent] {
			t.Errorf("%s comes before its parent", n.Path)
		}
		paths[n.Path] = true
		var under Filter
		for _, m := range tree[i+1:] {
			if m.Depth <= n.Depth {
				break
			}
			if m.Leaf {
				under = under.With(m.Kind, true)
			}
		}
		if under != n.Kinds || n.Kinds.Len() == 0 {
			t.Errorf("%s holds %d kinds, its subtree %d", n.Path, n.Kinds.Len(), under.Len())
		}
	}
	if set, ok := Resolve("Foods"); !ok || !set.Has(item.FineMeal) || !set.Has(item.Kibble) || set.Has(item.Wood) {
		t.Fatal("Food resolves to the food kinds")
	}
	if set, ok := Resolve("wood"); !ok || set != filterOf(item.Wood) {
		t.Fatal("a kind name resolves to that kind")
	}
	if _, ok := Resolve("Nonsense"); ok {
		t.Fatal("an unknown node resolves")
	}
	if set, ok := Resolve(Root); !ok || set != sets.storable {
		t.Fatal("the root resolves to every storable kind")
	}
	if depth := slices.MaxFunc(tree, func(a, b Node) int { return a.Depth - b.Depth }).Depth; depth != 3 {
		t.Fatalf("the tree is %d deep, want 3 (Foods/Raw food/Meat/meat)", depth)
	}
}

// legacyCat is the PLAY-023 rule the older switches followed (the game's
// former stockCatOf), kept here to pin Effective to it: 0 never stored,
// 1 food, 2 wood, 3 stone, 4 goods.
func legacyCat(k item.Kind) int {
	switch {
	case k == item.CorpseHuman, k == item.Bones:
		return 0
	case k.IsFood(), k.IsCorpse(), k == item.FertileEgg:
		return 1
	case k == item.Wood:
		return 2
	case k == item.Stone, item.IsChunk(k), item.IsStoneBlocks(k):
		return 3
	}
	return 4 // PRD-4: every other storable kind is a good (shells and the wave E materials now too)
}

// TestLegacySwitchesUnchanged: before any edit a zone takes what its
// Food, Wood, Stone, Corpses and Chunks switches did.
func TestLegacySwitchesUnchanged(t *testing.T) {
	t.Parallel()
	for _, z := range []Zone{
		{Kind: KindStockpile, Food: true, Wood: true, Stone: true}, {Kind: KindStockpile, Wood: true},
		{Kind: KindDump, Corpses: true, Chunks: true}, {Kind: KindDump, Food: true, Stone: true}, {Kind: KindDump},
	} {
		for _, k := range allKinds() {
			cat := legacyCat(k)
			want := false
			if z.Kind == KindDump {
				switch {
				case k.IsCorpse() && k != item.CorpseHuman:
					want = z.Corpses
				case item.IsChunk(k):
					want = z.Chunks
				case cat == 4:
					want = false
				default:
					want = cat == 1 && z.Food || cat == 2 && z.Wood || cat == 3 && z.Stone
				}
			} else {
				want = cat == 1 && z.Food || cat == 2 && z.Wood || cat == 3 && z.Stone || cat == 4
			}
			if got := z.Allows(k); got != want || z.Effective().Has(k) != want {
				t.Errorf("%+v takes %s: %v, want %v", z, k, got, want)
			}
		}
	}
	if (Zone{Kind: KindGrow, Food: true}).Allows(item.Meal) || (Zone{Kind: KindPen}).Effective() != (Filter{}) {
		t.Fatal("non-storage zones take nothing")
	}
}

// TestSetAllowedAndNames: the first edit freezes the switches into a
// filter; Names lists whole categories, and single kinds only where their
// category is partly allowed.
func TestSetAllowedAndNames(t *testing.T) {
	t.Parallel()
	z := Zone{Kind: KindStockpile, Food: true, Wood: true, Stone: true}
	before := z.Effective().Names()
	if !slices.Contains(before, "Raw resources/Wood") || !slices.Contains(before, "Foods") || slices.Contains(before, "wood") {
		t.Fatalf("a new stockpile lists %v", before)
	}
	wood, _ := Resolve("Raw resources/Wood")
	z.SetAllowed(wood, false)
	if !z.Custom || z.Allows(item.Wood) || !z.Allows(item.Steel) || !z.Allows(item.Meal) {
		t.Fatalf("after forbidding wood: %+v", z)
	}
	for _, n := range z.Effective().Names() {
		if strings.Contains(strings.ToLower(n), "wood") {
			t.Fatalf("names after forbidding wood: %v", z.Effective().Names())
		}
	}
	steel, _ := Resolve("steel")
	metals, _ := Resolve("Raw resources/Metals")
	z.SetAllowed(metals, false)
	z.SetAllowed(steel, true)
	names := z.Effective().Names()
	if !slices.Contains(names, "steel") || slices.Contains(names, "Raw resources/Metals") || slices.Contains(names, "gold") {
		t.Fatalf("one metal allowed: %v", names)
	}
	none, _ := Resolve("Foods")
	z.SetAllowed(none, true)
	if z.Allows(item.CorpseHuman) {
		t.Fatal("a legacy stockpile's switches never take human corpses")
	}
	z.SetAllowed(filterOf(item.CorpseHuman), true) // STO-2: storable now
	if !z.Allows(item.CorpseHuman) {
		t.Fatal("human corpses can be allowed")
	}
	// paste copies filter, specials and priority
	var dst Zone
	dst.Kind, dst.Priority = KindStockpile, PriorityLow
	z.Special.NoRotten, z.Priority = true, PriorityCritical
	dst.PasteFrom(z)
	if dst.Filter != z.Filter || !dst.Custom || dst.Special != z.Special || dst.Priority != PriorityCritical {
		t.Fatalf("pasted %+v from %+v", dst, z)
	}
	src := Zone{Kind: KindStockpile, Food: true}
	dst.PasteFrom(src) // an unedited source pastes its switches' filter
	if !dst.Allows(item.Meal) || dst.Allows(item.Wood) || !dst.Allows(item.Steel) {
		t.Fatal("paste from an unedited zone")
	}
}

// TestSpecialFilters: rotten and tainted toggles, hit-point and quality
// ranges, their names, and stacks without hit points or quality pass the
// ranges.
func TestSpecialFilters(t *testing.T) {
	t.Parallel()
	z := Zone{Kind: KindStockpile, Food: true, Wood: true, Stone: true}
	var all []string
	for _, tg := range Toggles() {
		all = append(all, tg.Name)
	}
	if got := z.Special.Names(); len(got) != len(Toggles()) || !slices.Equal(got[:2], SpecialNames()) {
		t.Fatalf("default specials %v, want every toggle %v", got, all)
	}
	rotten := item.Stack{Kind: item.Meat, Qty: 1, Rot: item.Meat.ShelfTicks()}
	tainted := item.Stack{Kind: item.Parka, Qty: 1, Of: 9}
	worn := item.Stack{Kind: item.Parka, Qty: 1, Wear: 100} // 40 of 140 hp
	good := item.Stack{Kind: item.Parka, Qty: 1, Quality: quality.Stored(quality.Good)}
	for _, st := range []item.Stack{rotten, tainted, worn, good} {
		if !z.AllowsStack(st) {
			t.Fatalf("the defaults refuse %+v", st)
		}
	}
	z.Special = z.Special.Set(SpecialRotten, false, 0, 0).Set(SpecialTainted, false, 0, 0)
	z.Special = z.Special.Set(SpecialHP, false, 50, 100).Set(SpecialQuality, false, 6, 3) // swapped: 3..6
	if z.AllowsStack(rotten) || z.AllowsStack(tainted) || z.AllowsStack(worn) || !z.AllowsStack(good) {
		t.Fatal("special filters")
	}
	if z.AllowsStack(item.Stack{Kind: item.Parka, Qty: 1, Quality: quality.Stored(quality.Normal)}) {
		t.Fatal("normal quality is under 3..6")
	}
	if !z.AllowsStack(item.Stack{Kind: item.Wood, Qty: 5}) || !z.AllowsStack(item.Stack{Kind: item.Parka, Qty: 1}) {
		t.Fatal("no hit points or no quality: the ranges don't apply")
	}
	if got := z.Special.Names(); !slices.Equal(got[len(got)-2:], []string{"hp 50-100%", "quality 3-6"}) || slices.Contains(got, "allow rotten") {
		t.Fatalf("names %v", got)
	}
	z.Special = z.Special.Set(SpecialHP, true, -5, 200).Set(SpecialQuality, true, 0, 9).Set(numSpecials, true, 1, 1)
	if lo, hi := z.Special.HPRange(); lo != 0 || hi != 100 || z.Special.HPMax != 0 || z.Special.QualityMax != 0 {
		t.Fatalf("full ranges clamp to the zero values: %+v", z.Special)
	}
	z.Special = z.Special.Set(SpecialHP, true, 0, 0)
	if _, hi := z.Special.HPRange(); hi != 1 {
		t.Fatalf("a range ending at 0 keeps 1 %%: %d", hi)
	}
}

// TestShelfZones: a shelf's storage never merges with a neighbour and the
// zone tools leave it alone; only RemoveTiles lifts it.
func TestShelfZones(t *testing.T) {
	t.Parallel()
	m := NewMap(10, 10)
	a := m.Paint(KindShelf, Zone{}, RectOf(1, 1, 2, 1), nil)
	b := m.Paint(KindShelf, Zone{}, RectOf(3, 1, 4, 1), nil)
	sp := m.Paint(KindStockpile, Zone{}, RectOf(1, 2, 4, 3), nil)
	if a == 0 || b == 0 || a == b || len(m.Cells(a)) != 2 || len(m.Cells(sp)) != 8 {
		t.Fatalf("shelves %d %d stockpile %d", a, b, sp)
	}
	m.Clear(RectOf(0, 0, 9, 9))
	m.Remove(a)
	m.RemoveCells(b, RectOf(0, 0, 9, 9))
	m.AddCells(a, [][2]int{{1, 0}})
	if len(m.Cells(a)) != 2 || len(m.Cells(b)) != 2 {
		t.Fatalf("the zone tools touched a shelf: %v %v", m.Cells(a), m.Cells(b))
	}
	if _, ok := m.Zone(sp); ok {
		t.Fatal("clear removed the stockpile")
	}
	m.RemoveTiles(m.Cells(a))
	if _, ok := m.Zone(a); ok {
		t.Fatal("RemoveTiles lifts a shelf's storage")
	}
	if !KindShelf.Storage() || KindShelf.Stacks() != ShelfStacks || KindStockpile.Stacks() != 1 {
		t.Fatal("shelf storage holds ShelfStacks per cell")
	}
}

// TestRowsAndShelfDefaults: the rows read a Names list back (whole, part
// or none of each node); a new shelf takes general items at Preferred.
func TestRowsAndShelfDefaults(t *testing.T) {
	t.Parallel()
	z := Zone{Kind: KindStockpile, Food: true, Wood: true, Stone: true}
	metals, _ := Resolve("Raw resources/Metals")
	z.SetAllowed(metals, false)
	z.SetAllowed(filterOf(item.Gold), true)
	rows := Rows(append(z.Effective().Names(), "Nonsense"))
	if len(rows) != len(Tree()) {
		t.Fatal("one row per tree node")
	}
	state := map[string]Row{}
	for _, r := range rows {
		state[r.Path] = r
	}
	if !state["gold"].On || state["steel"].On || !state["Raw resources/Metals"].Partial || !state["Raw resources"].Partial ||
		!state["Foods"].On || state["Foods"].Partial || state["Foods/Meals"].Depth != 1 || !state["meal"].Leaf ||
		state["Foods/Raw food/Meat"].Depth != 2 || state["meat"].Depth != 3 {
		t.Fatalf("rows: %+v %+v %+v", state["gold"], state["Raw resources/Metals"], state["Foods"])
	}
	sh := NewShelf()
	if sh.Kind != KindShelf || !sh.Hidden || sh.Priority != PriorityPreferred || !sh.Custom ||
		sh.Allows(item.GraniteChunk) || sh.Allows(item.CorpseAnimal) || !sh.Allows(item.Meal) || !sh.Allows(item.Rifle) {
		t.Fatalf("new shelf %+v", sh)
	}
}
