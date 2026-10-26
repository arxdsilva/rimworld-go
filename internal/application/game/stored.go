package game

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
)

// storedRows is the resource list's rows (PLAY-018): qty[k] is how much of
// item kind k lies in stockpiles, one row per kind present, ordered by
// category (item.Categories) and then label.
func storedRows(qty *[256]int) []StoredResourceView {
	var out []StoredResourceView
	for k := range qty {
		if qty[k] == 0 {
			continue
		}
		kind := item.Kind(k)
		out = append(out, StoredResourceView{Kind: kind.String(), Label: itemLabel(kind), Category: kind.Category(), Qty: qty[k]})
	}
	slices.SortFunc(out, func(a, b StoredResourceView) int {
		if r := item.CategoryRank(a.Category) - item.CategoryRank(b.Category); r != 0 {
			return r
		}
		if c := strings.Compare(a.Label, b.Label); c != 0 {
			return c
		}
		return strings.Compare(a.Kind, b.Kind)
	})
	return out
}

// countStored adds stack st to qty when it lies on a stockpile cell and
// nobody carries it. One zone lookup per stack (zones.At is a cell index),
// never a scan of the cells.
func (g *Game) countStored(qty *[256]int, st item.Stack) {
	if st.Carrier != 0 {
		return
	}
	if _, ok := g.zoneOf(zone.KindStockpile, st.X, st.Y); ok {
		qty[st.Kind] += st.Qty
	}
}
