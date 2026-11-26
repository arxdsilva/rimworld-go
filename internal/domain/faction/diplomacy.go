// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import "math"

// Costs and gift bands follow Core diplomacy measurements.
const (
	GiftSilverPerGoodwill = 40 // minimum cost below neutral
	AidCost               = 31
	AidCooldownDays       = 1
	CaravanCost           = 19
	CaravanArriveMinHours = 48
	CaravanArriveMaxHours = 48
	CaravanCooldownDays   = 4
)

// Player-facing refusal reasons.
const (
	ReasonGone       = "that faction is gone"
	ReasonBesieged   = "they refuse anything from us while we attack them"
	ReasonHostile    = "they are hostile"
	ReasonNotFriend  = "they are not friendly enough"
	ReasonGoodwill   = "they don't trust you enough"
	ReasonCoolingOff = "they answered recently"
	ReasonNoFighters = "they have no fighters to send"
)

// Cooldowns is when a faction answers requests again: the tick each
// cooldown ends (0: none). Saved by the game, per faction.
type Cooldowns struct {
	AidUntil     uint64
	CaravanUntil uint64
}

// giftPositiveGain interpolates measured Core gifts above zero goodwill.
// 2041 silver -> +40, 3661 -> +60, 5539 -> +75 at neutral.
func giftPositiveGain(current, silver int) int {
	if silver <= 0 || current >= goodwillMax {
		return 0
	}
	return int(float64(100-current) * (1 - math.Exp(-float64(silver)/3995)))
}

// GiftCostAt estimates the next goodwill point's silver cost.
func GiftCostAt(goodwill int) int {
	switch {
	case goodwill < 0:
		return GiftSilverPerGoodwill
	case goodwill >= 75:
		return 160
	default:
		return int(math.Ceil(3995 * math.Log(float64(100-goodwill)/float64(99-goodwill))))
	}
}

func baseGiftGoodwill(t Type) int {
	if def(t).GoodwillMax < 0 {
		return def(t).GoodwillMax
	}
	return 0
}

// GiftGoodwill measures one gift in its recipient's current relation.
// Gifts toward a faction's base goodwill include the 25% restoration bonus.
func GiftGoodwill(f Faction, value int) int {
	if value <= 0 || !f.Type.HasWorldSettlements() || def(f.Type).Fixed || f.Defeated {
		return 0
	}
	start, gw := f.Goodwill, f.Goodwill
	if gw < 0 {
		paid := min(-gw, value/GiftSilverPerGoodwill)
		gw += paid
		value -= paid * GiftSilverPerGoodwill
	}
	if gw >= 0 && gw < 75 && value > 0 {
		lo, hi := 0, 6000
		for lo < hi {
			mid := lo + (hi-lo)/2
			if giftPositiveGain(gw, mid) >= 75-gw {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		if value >= lo {
			gw = 75
			value -= lo
		} else {
			gw += giftPositiveGain(gw, value)
			value = 0
		}
	}
	if gw >= 75 {
		gw += value / 160
	}
	if base := baseGiftGoodwill(f.Type); start < base {
		if towardBase := min(gw, base) - start; towardBase > 0 {
			gw += towardBase / 4
		}
	}
	return min(gw, goodwillMax) - start
}

// GiftSilverFor finds the minimum silver that reaches target goodwill.
func GiftSilverFor(f Faction, target int) int {
	if target <= f.Goodwill || !f.Type.HasWorldSettlements() || def(f.Type).Fixed || f.Defeated {
		return 0
	}
	target = min(target, goodwillMax)
	delta := target - f.Goodwill
	lo, hi := 0, (goodwillMax-goodwillMin)*160
	for lo < hi {
		mid := lo + (hi-lo)/2
		if GiftGoodwill(f, mid) >= delta {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// TradeGoodwill awards +1 per 500 silver of paid merchandise, carrying
// fractional value between trades on the recipient's saved faction.
func TradeGoodwill(f *Faction, value int, event string) int {
	if f == nil || value <= 0 || f.Defeated || f.Besieged ||
		!f.Type.HasWorldSettlements() || def(f.Type).Fixed {
		return 0
	}
	const maxInt = int(^uint(0) >> 1)
	if value > maxInt-int(f.TradeRemainder) {
		return 0
	}
	total := int(f.TradeRemainder) + value
	delta := min(total/500, goodwillMax-f.Goodwill)
	f.TradeRemainder = uint16(total % 500)
	if delta > 0 {
		Adjust(f, delta, event)
	}
	return delta
}

// CanRequestAid reports whether f answers a military aid request at tick
// now, or the player-facing reason it won't.
func CanRequestAid(f Faction, cd Cooldowns, now uint64) (bool, string) {
	switch {
	case f.Defeated:
		return false, ReasonGone
	case f.Relation == Hostile:
		return false, ReasonHostile
	case f.Relation != Friendly:
		return false, ReasonNotFriend
	case !f.Type.HasWorldSettlements() || def(f.Type).Tech == Neolithic:
		return false, ReasonNoFighters
	case f.Goodwill < AidCost:
		return false, ReasonGoodwill
	case now < cd.AidUntil:
		return false, ReasonCoolingOff
	}
	return true, ""
}

// CanRequestCaravan reports whether f sends a trade caravan on request at
// tick now, or the player-facing reason it won't.
func CanRequestCaravan(f Faction, cd Cooldowns, now uint64) (bool, string) {
	switch {
	case f.Defeated:
		return false, ReasonGone
	case f.Relation == Hostile:
		return false, ReasonHostile
	case !f.Type.HasWorldSettlements() || f.Relation != Friendly:
		return false, ReasonNotFriend
	case f.Goodwill < CaravanCost:
		return false, ReasonGoodwill
	case now < cd.CaravanUntil:
		return false, ReasonCoolingOff
	}
	return true, ""
}
