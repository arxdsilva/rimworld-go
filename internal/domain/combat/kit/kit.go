// Package kit arms a raid (DEF-2): it spends a raid's points on the
// raiders' weapons from the vanilla catalogue, by the faction's
// technology. It is pure: the caller passes the roll (the game's main
// stream), so a raid is the same on every run.
package kit

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

// entry is one weapon a raider may carry: the raid points it costs and
// the oldest technology that fields it. Melee weapons are listed apart.
type entry struct {
	k    item.Kind
	cost int
	tech faction.Tech
}

// The raid points a raider costs with each weapon. original tuning: the
// four colony weapons keep their pre-DEF-2 prices (knife 50, club 60,
// spear 70, rifle 90); the rest follow the wiki's raider combat powers
// (pirate gunner 65, mercenary gunner 85, sniper 110, grenadier 70,
// slasher 140; wiki: Raid) scaled onto that ladder by how deadly each
// weapon is. Tribes field the knife, club, spear, bows and pila;
// industrial factions every firearm, frag grenades, molotovs and the
// incendiary launcher. EMP weapons stay out until machines fight (DEF-4).
var (
	melee = [...]entry{
		{item.Knife, 50, faction.Neolithic}, {item.Club, 60, faction.Neolithic}, {item.Spear, 70, faction.Neolithic},
		{item.Mace, 70, faction.Industrial}, {item.Gladius, 75, faction.Industrial}, {item.Longsword, 90, faction.Industrial},
	}
	ranged = [...]entry{
		{item.ShortBow, 60, faction.Neolithic}, {item.RecurveBow, 70, faction.Neolithic},
		{item.Pila, 75, faction.Neolithic}, {item.Greatbow, 85, faction.Neolithic},
		{item.Revolver, 75, faction.Industrial}, {item.Autopistol, 75, faction.Industrial},
		{item.MachinePistol, 85, faction.Industrial}, {item.Rifle, 90, faction.Industrial},
		{item.PumpShotgun, 90, faction.Industrial}, {item.MolotovCocktails, 90, faction.Industrial},
		{item.FragGrenades, 95, faction.Industrial}, {item.HeavySMG, 100, faction.Industrial},
		{item.ChainShotgun, 105, faction.Industrial}, {item.AssaultRifle, 110, faction.Industrial},
		{item.LMG, 110, faction.Industrial}, {item.IncendiaryLauncher, 110, faction.Industrial},
		{item.SniperRifle, 115, faction.Industrial}, {item.Minigun, 160, faction.Industrial},
	}
)

// Cheapest is the least a raider costs (a knife).
const Cheapest = 50

// RangedPct is the chance (percent) a raider who can afford a ranged
// weapon carries one. original tuning (as before DEF-2)
const RangedPct = 55

// fits reports whether a faction of tech fields e: tribes only their own
// weapons, industrial factions everything but the tribal bows and pila
// (they carry guns), spacers everything industrial.
func fits(e entry, tech faction.Tech) bool {
	if tech == faction.Neolithic {
		return e.tech == faction.Neolithic
	}
	return e.tech != faction.Neolithic || e.k == item.Knife || e.k == item.Club || e.k == item.Spear
}

// Cost is the raid points a raider with weapon k costs (0: k is not in
// the raid catalogue).
func Cost(k item.Kind) int {
	for _, l := range [...][]entry{melee[:], ranged[:]} {
		for _, e := range l {
			if e.k == k {
				return e.cost
			}
		}
	}
	return 0
}

// Pick spends points on raiders' weapons for a faction of tech, at most
// limit raiders and at least one. Each raider who can afford a ranged
// weapon of its faction carries one RangedPct of the time (one of those
// it can afford, evenly); otherwise a melee weapon, and when that is more
// than what is left (after the first raider) a knife.
func Pick(points, limit int, tech faction.Tech, roll func(int) int) []item.Kind {
	var kit []item.Kind
	var can [len(ranged)]item.Kind
	var costs [len(ranged)]int
	for len(kit) < limit && (points >= Cheapest || len(kit) == 0) {
		n := 0
		for _, e := range ranged {
			if e.cost <= points && fits(e, tech) {
				can[n], costs[n] = e.k, e.cost
				n++
			}
		}
		if n > 0 && roll(100) < RangedPct {
			i := roll(n)
			kit = append(kit, can[i])
			points -= costs[i]
			continue
		}
		m := 0
		for _, e := range melee {
			if fits(e, tech) {
				can[m], costs[m] = e.k, e.cost
				m++
			}
		}
		i := roll(m)
		k, c := can[i], costs[i]
		if c > points && len(kit) > 0 {
			k, c = item.Knife, Cheapest // the cheapest blade fits what is left
		}
		kit = append(kit, k)
		points -= c
	}
	return kit
}

// Legacy is the dev hooks' kit of n raiders (sieges, sappers): half
// rifles, a quarter knives, a quarter spears, one roll each.
func Legacy(n int, roll func(int) int) []item.Kind {
	kit := make([]item.Kind, n)
	for i := range kit {
		switch roll(4) {
		case 0, 1:
			kit[i] = item.Rifle
		case 2:
			kit[i] = item.Knife
		case 3:
			kit[i] = item.Spear
		}
	}
	return kit
}
