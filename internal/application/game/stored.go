// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/economy/item"
)

// storedRows is the resource list's rows (PLAY-018): qty[k] is how much of
// item kind k lies in stockpiles, one row per kind present, ordered by
// category (item.Categories) and then label; a plugin kind (PLG-026) by
// its key, label and category in the run's items.
func storedRows(qty *[256]int, items item.Book) []StoredResourceView {
	var out []StoredResourceView
	for k := range qty {
		if qty[k] == 0 {
			continue
		}
		kind := item.Kind(k)
		label := itemLabel(kind)
		if items.Plugin(kind) {
			label = items.Label(kind)
		}
		out = append(out, StoredResourceView{Kind: items.Name(kind), Label: label, Category: items.Category(kind), Qty: qty[k]})
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
	if _, ok := g.storageAt(st.X, st.Y); ok { // stockpiles and dumps (PLAY-023)
		qty[st.Kind] += st.Qty
	}
}
