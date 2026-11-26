// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package tradeview

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/trade"
)

func itemLabel(b item.Book, k item.Kind, label func(item.Kind) string, g trade.Goods) string {
	if named := g.Label(b); named != "" {
		return named
	}
	if label != nil && !b.Plugin(k) {
		return label(k)
	}
	return b.Label(k)
}

func stackMarket(b item.Book, st item.Stack) int {
	if b.Plugin(st.Kind) {
		return b.Value(st.Kind)
	}
	return item.StackValue(st)
}

// giftRows selects actual physical item valuations while retaining ordinary
// resources as one kind row. The IDs are the game's live stock IDs.
func giftRows(b item.Book, stacks []trade.GiftStack, social int, label func(item.Kind) string, method trade.GiftMethod) []Row {
	grouped := make(map[item.Kind]int)
	out := make([]Row, 0)
	for _, v := range stacks {
		st := v.Stack
		c := trade.GiftStackCenti(b, st, method, social)
		if st.Qty <= 0 || c <= 0 {
			continue
		}
		if !trade.GiftIndividual(st) {
			grouped[st.Kind] += st.Qty
			continue
		}
		g := trade.Goods{Kind: st.Kind, Building: st.Building}
		out = append(out, Row{
			Kind:  g.Name(b) + "@" + strconv.FormatUint(v.ID, 10),
			Label: giftLabel(b, st, label), Category: b.Category(st.Kind),
			ColonyQty: st.Qty, StackID: v.ID, GiftC: c, WillSell: true,
		})
	}
	for kind, qty := range grouped {
		out = append(out, Row{
			Kind: b.Name(kind), Label: itemLabel(b, kind, label, trade.Goods{Kind: kind}),
			Category: b.Category(kind), ColonyQty: qty,
			GiftC: trade.GiftUnitCenti(kind, b.Value(kind), method, social), WillSell: true,
		})
	}
	slices.SortStableFunc(out, func(a, b Row) int {
		if x, y := item.CategoryRank(a.Category), item.CategoryRank(b.Category); x != y {
			return cmp.Compare(x, y)
		}
		if x := cmp.Compare(a.Label, b.Label); x != 0 {
			return x
		}
		return cmp.Compare(a.StackID, b.StackID)
	})
	return out
}

func giftLabel(b item.Book, st item.Stack, label func(item.Kind) string) string {
	result := itemLabel(b, st.Kind, label, trade.Goods{Kind: st.Kind, Building: st.Building})
	if st.Kind == item.MinifiedBuilding {
		return result
	}
	if st.Kind.IsApparel() || st.Kind.IsWeapon() {
		level := quality.Default
		if q, ok := st.QualityLevel(); ok {
			level = q
		}
		result += " (" + level.String()
		if item.MadeFrom(st.Kind) != 0 {
			result += ", " + st.StuffOfStack().String()
		}
		if maxHP := st.MaxHP(); maxHP > 0 {
			result += fmt.Sprintf(", %d%% HP", st.HP()*100/maxHP)
		}
		return result + ")"
	}
	if maxHP := st.MaxHP(); maxHP > 0 && st.Wear > 0 {
		result += fmt.Sprintf(" (%d%% HP)", st.HP()*100/maxHP)
	}
	return result
}
