// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package caravan

import (
	"math"
	"sort"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
)

// Caravan food (#14): a caravan's people eat the real food packed in its
// cargo. Each road hour the reserve (FoodDays) drains first, then the
// eaters (colonists and prisoners; animals graze) eat whole units off the
// cargo, then, when the cargo cannot cover the hour, they starve.

// Edible reports k is people's caravan food. Hay is fodder for animals; it
// is the diet's last-resort rule too. wiki: Caravan
func Edible(k item.Kind) bool {
	return k.IsFood() && k != item.Hay
}

// Nutrition is the edible nutrition in goods (needs units). Empty and
// negative stacks count 0.
func Nutrition(goods []item.Stack) int {
	total := 0
	for _, s := range goods {
		if s.Qty > 0 && Edible(s.Kind) {
			total += s.Qty * s.Kind.Nutrition()
		}
	}
	return total
}

// HourNeed is the nutrition eaters eat in an hour (needs units). wiki: Food
func HourNeed(eaters int) int {
	if eaters <= 0 {
		return 0
	}
	return eaters * needs.HungerPerHour
}

// Days is how long nutrition feeds eaters, in milli-days (the FoodDays
// unit), clamped to MaxInt32.
func Days(nutrition, eaters int) int32 {
	if eaters <= 0 || nutrition <= 0 {
		return 0
	}
	if int64(nutrition) > math.MaxInt64/1000 {
		return math.MaxInt32
	}
	d := int64(nutrition) * 1000 / (int64(eaters) * needs.HungerPerHour * 24)
	if d > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(d)
}

// Ration is the whole units of goods that cover need, spoiling first: eat
// the units taken, fed the nutrition they give beyond need, ok false when
// goods cannot cover need (then eat is everything edible and fed 0). A need
// of zero or less eats nothing and credits -need. The result does not
// depend on the order of goods, and goods is not changed. wiki: Caravan
func Ration(goods []item.Stack, need int) (eat []item.Stack, fed int, ok bool) {
	if need <= 0 {
		return nil, -need, true
	}
	have := map[item.Kind]int{}
	for _, s := range goods {
		if s.Qty > 0 && Edible(s.Kind) {
			have[s.Kind] += s.Qty
		}
	}
	kinds := make([]item.Kind, 0, len(have))
	for k := range have {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return eatsBefore(kinds[i], kinds[j]) })
	rem := need
	for _, k := range kinds {
		if rem <= 0 {
			break
		}
		unit := k.Nutrition()
		n := (rem + unit - 1) / unit // original tuning: whole units, round up
		if n > have[k] {
			n = have[k]
		}
		eat = append(eat, item.Stack{Kind: k, Qty: n})
		rem -= n * unit
	}
	if rem > 0 {
		return eat, 0, false
	}
	return eat, -rem, true
}

// eatsBefore orders kinds: perishable by shelf life ascending, then those
// that never spoil, then by kind. wiki: Caravan (spoiling food first)
func eatsBefore(a, b item.Kind) bool {
	sa, sb := a.ShelfTicks(), b.ShelfTicks()
	if sa != sb {
		if sa == 0 || sb == 0 {
			return sb == 0
		}
		return sa < sb
	}
	return a < b
}
