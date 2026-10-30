package kit

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

// Raider armor (CBT-3). A raider's armor follows its pawn kind, which the
// raid's points already chose through its weapon (Cost is the kind's
// combat power): the wiki's raider kinds wear flak and helmets from the
// pools their pages list. Tribes go bare but for their chief's plate
// armor (berserker chief); industrial raiders wear flak by power (pirate
// gunner 65: vest and helmet maybe; mercenary gunner 85: vest or jacket,
// a helmet likely; sniper and heavy mercenary 110+: the jacket and a
// helmet), and an industrial blade of 75 points or more is a slasher,
// whose page requires a shield belt. Spacer factions dress as the
// Empire's trooper (flak jacket and pants, a helmet) and marine armor at
// 110 points (original: the wiki names no marine-armored raider). A body
// slot holds one piece here, so the vest and the jacket never stack, and
// the flak helmet is the simple helmet. Armor costs no extra points.
// wiki: Raider (pawn kinds and apparel pools)
const (
	gunnerPower = 65  // wiki: Raider (pirate gunner)
	mercPower   = 85  // wiki: Raider (mercenary gunner)
	heavyPower  = 110 // wiki: Raider (mercenary sniper)
	// slasherPower is an industrial blade's cost that makes its raider a
	// slasher (the gladius and the longsword; original mapping).
	slasherPower = 75
	// chiefPower is a tribal weapon's cost that makes its raider the
	// chief (the greatbow; original mapping of the berserker chief's 85).
	chiefPower = 85
)

// Armor is the apparel a raider armed with w wears, for a faction of
// tech. seed picks the optional pieces (the raider's id, mixed: no stream
// draw, so a raid's other rolls stay as they were).
func Armor(w item.Kind, tech faction.Tech, seed uint64) []item.Kind {
	cost, r := Cost(w), mix(seed)
	maybe := func(pct uint64) bool { // one draw of 0..99 per call, from r's bits
		v := r % 100
		r /= 100
		return v < pct
	}
	switch {
	case tech == faction.Neolithic:
		if cost >= chiefPower {
			return []item.Kind{item.PlateArmor}
		}
		return nil
	case tech >= faction.Spacer:
		if cost >= heavyPower {
			return []item.Kind{item.MarineArmor, item.MarineHelmet, item.FlakPants}
		}
		return []item.Kind{item.FlakJacket, item.FlakPants, item.SimpleHelmet}
	case isMelee(w) && cost >= slasherPower:
		return []item.Kind{item.ShieldBelt, item.FlakVest, item.SimpleHelmet}
	case cost >= heavyPower:
		return []item.Kind{item.FlakJacket, item.SimpleHelmet}
	case cost >= mercPower:
		out := []item.Kind{item.FlakVest}
		if maybe(50) {
			out[0] = item.FlakJacket
		}
		if maybe(75) {
			out = append(out, item.SimpleHelmet)
		}
		return out
	case cost >= gunnerPower:
		var out []item.Kind
		if maybe(50) {
			out = append(out, item.FlakVest)
		}
		if maybe(50) {
			out = append(out, item.SimpleHelmet)
		}
		return out
	}
	return nil
}

// isMelee reports a melee weapon of the raid catalogue.
func isMelee(k item.Kind) bool {
	for _, e := range melee {
		if e.k == k {
			return true
		}
	}
	return false
}

// mix is splitmix64's finaliser: seed's bits spread evenly.
func mix(seed uint64) uint64 {
	z := seed + 0x9E3779B97F4A7C15
	z = (z ^ z>>30) * 0xBF58476D1CE4E5B9
	z = (z ^ z>>27) * 0x94D049BB133111EB
	return z ^ z>>31
}

// Casket is the old blade the n-th casket's remains lie with (moved from
// the game). original
func Casket(n int) item.Kind {
	return [...]item.Kind{item.Club, item.Knife, item.Spear}[(n%3+3)%3]
}

// TechOf is faction f's technology (industrial for none: a raid the dev
// hooks send).
func TechOf(f *faction.Faction) faction.Tech {
	if f == nil {
		return faction.Industrial
	}
	return faction.DefOf(f.Type).Tech
}

// Ally is the k-th allied fighter's weapon (moved from the game): neolithic
// peoples bring spears and clubs, the others rifles with one knife among
// every three. original tuning
func Ally(tech faction.Tech, k int) item.Kind {
	if tech == faction.Neolithic {
		if k%2 == 0 {
			return item.Spear
		}
		return item.Club
	}
	if k%3 == 2 {
		return item.Knife
	}
	return item.Rifle
}
