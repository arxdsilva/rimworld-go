// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mapview

import (
	"rimworld-go/internal/domain/economy/item"
	"slices"
	"strings"
)

// StoredResourceView is one row of the resource list (PLAY-018).
type StoredResourceView struct {
	Kind     string // item kind key ("wood")
	Label    string // "Wood"
	Category string // "Food", "Materials", "Medicine", "Apparel", "Weapons", "Manufactured", "Corpses"
	Qty      int
}

// StoredRows lists stockpiled quantities in category/name order.
func StoredRows(qty *[256]int, items item.Book, itemLabel func(item.Kind) string) []StoredResourceView {
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
		out = append(out, StoredResourceView{
			Kind: items.Name(kind), Label: label, Category: items.Category(kind), Qty: qty[k],
		})
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
