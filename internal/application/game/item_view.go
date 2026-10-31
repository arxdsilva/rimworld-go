package game

import (
	"rimworld-go/internal/application/game/craft"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/filth"
)

// itemCounts are the raw counts behind WorldView's item totals: loose
// (uncarried) quantities by category and by food kind, and the stockpiled
// quantity per kind (stored.go).
type itemCounts struct {
	food, wood, stone, steel, silver, gold int
	foodBy                                 [256]int
	foodHas                                [256]bool // a loose stack of that food kind was seen (FoodByKind lists it)
	stored                                 [256]int
}

// itemMemo is the view cache's last item list and totals (PERF-002): the
// list is rebuilt into scratch and handed out as the old slice while it
// reads the same, and the FoodByKind map and Stored rows (shared, read
// only) are rebuilt only when their counts move.
type itemMemo struct {
	views, scratch []ItemView
	totals         itemCounts
	foodByKind     map[string]int
	stored         []StoredResourceView
	ok             bool
}

// fillItemViews sets w.Items and the item totals (Food .. Gold,
// FoodByKind, Stored). Hidden (fogged) stacks neither show nor count.
func (g *Game) fillItemViews(w *WorldView) {
	var t itemCounts
	if !g.views.on {
		w.Items = g.appendItemViews(make([]ItemView, 0, len(g.itemIDs)), &t)
		t.apply(w, t.foodByKind(), storedRows(&t.stored))
		return
	}
	m := &g.views.items
	m.scratch = g.appendItemViews(m.scratch[:0], &t)
	if !m.ok || !slices.EqualFunc(m.scratch, m.views, sameItemView) {
		m.views = make([]ItemView, len(m.scratch))
		copy(m.views, m.scratch)
	}
	if !m.ok || t.foodBy != m.totals.foodBy || t.foodHas != m.totals.foodHas {
		m.foodByKind = t.foodByKind()
	}
	if !m.ok || t.stored != m.totals.stored {
		m.stored = storedRows(&t.stored)
	}
	m.totals, m.ok = t, true
	w.Items = m.views
	t.apply(w, m.foodByKind, m.stored)
}

// sameItemView is a == b field by field (Ingredients keeps ItemView from
// ==); TestSameItemViewSeesEveryField fails on a field left out.
func sameItemView(a, b ItemView) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.Qty == b.Qty && a.X == b.X && a.Y == b.Y && a.Carried == b.Carried &&
		a.FreshDays == b.FreshDays && a.Rotten == b.Rotten && a.RotRate == b.RotRate && a.Dried == b.Dried && a.Of == b.Of &&
		a.OfName == b.OfName && a.Animal == b.Animal && a.Wreck == b.Wreck && a.Catch == b.Catch && a.Forbidden == b.Forbidden &&
		a.Unclaimed == b.Unclaimed && a.HaulMarked == b.HaulMarked && a.PoisonPermille == b.PoisonPermille &&
		a.Quality == b.Quality && a.Stuff == b.Stuff && a.HP == b.HP && a.MaxHP == b.MaxHP && a.Tainted == b.Tainted &&
		a.Urgent == b.Urgent && slices.Equal(a.Ingredients, b.Ingredients) && a.Meat == b.Meat
}

// appendItemViews appends every visible stack's view to out and counts it
// into t.
func (g *Game) appendItemViews(out []ItemView, t *itemCounts) []ItemView {
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok || g.fogged(st.X, st.Y) { // fog.go: hidden items neither show nor count
			continue
		}
		g.countStored(&t.stored, st)
		out = append(out, ItemView{
			ID: id, Kind: st.Kind.String(), Qty: st.Qty, X: st.X, Y: st.Y, Carried: st.Carrier != 0,
			FreshDays: craft.FreshDays(st), Rotten: st.Rotten(), Dried: st.Dried,
			RotRate: item.RotRate(int(g.temperatureAt(st.X, st.Y))),
			Catch:   filth.Catch(st.Kind) / 10, Forbidden: st.Forbidden,
			Unclaimed: st.Unclaimed, HaulMarked: st.HaulMarked, PoisonPermille: int(st.Poison),
			Quality: craft.QualityLabel(st.Quality), Stuff: craft.StuffLabel(st.Kind, st.Stuff), // PRD-2
			HP: st.HP(), MaxHP: st.MaxHP(), Tainted: st.Tainted(), Urgent: g.haul.store.Urgent.Has(id), // STO-2
		})
		g.fillCorpseView(&out[len(out)-1], st) // burial.go
		if st.Carrier != 0 {
			continue
		}
		switch {
		case st.Kind.IsFood():
			t.food += st.Qty
			t.foodBy[st.Kind] += st.Qty
			t.foodHas[st.Kind] = true
		case st.Kind == item.Wood:
			t.wood += st.Qty
		case item.IsStoneBlocks(st.Kind) || st.Kind == item.Stone:
			t.stone += st.Qty
		case st.Kind == item.Steel:
			t.steel += st.Qty
		case st.Kind == item.Silver:
			t.silver += st.Qty
		case st.Kind == item.Gold:
			t.gold += st.Qty
		}
	}
	return out
}

// foodByKind is WorldView.FoodByKind (nil: no loose food at all).
func (t *itemCounts) foodByKind() map[string]int {
	var out map[string]int
	for k, has := range t.foodHas {
		if !has {
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[item.Kind(k).String()] += t.foodBy[k]
	}
	return out
}

// apply copies the totals into w.
func (t *itemCounts) apply(w *WorldView, foodByKind map[string]int, stored []StoredResourceView) {
	w.Food, w.Wood, w.Stone, w.Steel, w.Silver, w.Gold = t.food, t.wood, t.stone, t.steel, t.silver, t.gold
	w.FoodByKind, w.Stored = foodByKind, stored
}
