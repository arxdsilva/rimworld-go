// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// GiftMethod distinguishes goods given to a visiting trader, sent by pod,
// or delivered to a faction settlement by a visiting caravan.
type GiftMethod uint8

const (
	GiftVisitor GiftMethod = iota
	GiftPod
	GiftSettlement
)

// giftRate is the published selling rate in basis points, before item
// modifiers. Unaccompanied pods keep a fixed 61.2% regardless of Social.
func giftRate(method GiftMethod, social int) int {
	if method == GiftPod {
		return 6120
	}
	rate := 6000 + 90*min(max(social, 0), 20)
	if method == GiftSettlement {
		rate += 120
	}
	return rate
}

// GiftBaseCenti is an ordinary item's eligible gift value in centi-silver.
// -1 means a custom-content market value cannot be represented.
func GiftBaseCenti(market int, method GiftMethod, social int) int {
	if market <= 0 {
		return 0
	}
	rate := giftRate(method, social)
	const maxInt = int(^uint(0) >> 1)
	if market > (maxInt-50)/rate {
		return -1
	}
	return (market*rate + 50) / 100
}

// GiftUnitCenti includes silver's face value and the weapon sell factor.
func GiftUnitCenti(k item.Kind, market int, method GiftMethod, social int) int {
	if market <= 0 {
		return 0
	}
	if k == item.Silver {
		const maxInt = int(^uint(0) >> 1)
		if market > maxInt/100 {
			return -1
		}
		return market * 100
	}
	if k.IsWeapon() {
		v := GiftBaseCenti(market, method, social)
		if v < 0 {
			return -1
		}
		return (v + 2) / 5
	}
	return GiftBaseCenti(market, method, social)
}

// GiftGoodsCenti includes minified furniture and sculpture multipliers.
func GiftGoodsCenti(b item.Book, g Goods, method GiftMethod, social int) int {
	v := GiftUnitCenti(g.Kind, g.Value(b), method, social)
	if v <= 0 || g.Kind != item.MinifiedBuilding || method == GiftPod {
		return v
	}
	multiplier := 70
	switch construction.Kind(g.Building) {
	case construction.SmallSculpture, construction.LargeSculpture, construction.GrandSculpture:
		multiplier = 110
	}
	const maxInt = int(^uint(0) >> 1)
	if v > (maxInt-50)/multiplier {
		return -1
	}
	return (v*multiplier + 50) / 100
}
