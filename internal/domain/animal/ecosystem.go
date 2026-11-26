// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"rimworld-go/internal/domain/world/biome"
	"sort"
)

// EcosystemWeight is an original wildlife-budget cost in hundredths of
// adult body size. Species without a known size cost one body unit.
func EcosystemWeight(s Species) int {
	if size := BodySize(s); size > 0 {
		return size
	}
	return 100
}

// EcosystemCapacity converts published biome density into our area-based
// budget, in the same units as EcosystemWeight.
func EcosystemCapacity(density100, area int) int {
	if density100 <= 0 || area <= 0 {
		return 0
	}
	return density100 * area / 10000
}

// WildWeight combines biome frequency and the original seasonal preference.
func WildWeight(s Species, b biome.ID, season string) int {
	return Weight(s, b) * SeasonWeight(s, season)
}

// WildEligible excludes the domestic and pet roster from setup wildlife.
func WildEligible(s Species) bool {
	b := RosterOf(s).Behaviour
	return b != Domestic && b != Pet
}

// PickWildSpecies draws from the native roster in stable species order.
func PickWildSpecies(b biome.ID, season string, setup bool, roll func(int) int, keep func(Species) bool) (Species, bool) {
	weight := func(s Species) int {
		if setup && !WildEligible(s) {
			return 0
		}
		return WildWeight(s, b, season)
	}
	total := 0
	for _, s := range All() {
		if keep(s) {
			total += weight(s)
		}
	}
	if total <= 0 {
		return 0, false
	}
	r := roll(total)
	for _, s := range All() {
		if keep(s) {
			r -= weight(s)
			if r < 0 {
				return s, true
			}
		}
	}
	return 0, false
}

func prey(s Species) bool     { return !DefOf(s).Predator }
func predator(s Species) bool { return DefOf(s).Predator }

// PickEcosystemGroup prefers prey; a low predator share permits predator
// arrivals. A negative limit leaves species size unconstrained.
func PickEcosystemGroup(n, preds, limit int, roll func(int) int, pick func(func(Species) bool) (Species, bool)) (Species, bool) {
	first, second := prey, predator
	if roll(7) < 1 && preds*7 < n {
		first, second = predator, prey
	}
	fits := func(role func(Species) bool) func(Species) bool {
		return func(s Species) bool { return role(s) && (limit < 0 || EcosystemWeight(s) <= limit) }
	}
	if s, ok := pick(fits(first)); ok {
		return s, true
	}
	return pick(fits(second))
}

// StartEcosystem fills the original starting budget. The map owner chooses
// habitat and actually spawns herds; returned weight is what really spawned.
const (
	StartHerdsMin = 4
	StartHerdsMax = 6
	StartCapPct   = 80
)

func StartEcosystem(capacity int, roll func(int) int, pick func(func(Species) bool) (Species, bool), place func(Species, int) int) {
	if capacity <= 0 {
		return
	}
	target := capacity * StartCapPct / 100
	n := StartHerdsMin + roll(StartHerdsMax-StartHerdsMin+1)
	picks := make([]Species, 0, n)
	for i := 0; i < n; i++ {
		if s, ok := pick(func(Species) bool { return true }); ok {
			picks = append(picks, s)
		}
	}
	has := func(f func(Species) bool) bool {
		for _, s := range picks {
			if f(s) {
				return true
			}
		}
		return false
	}
	if len(picks) > 0 && !has(prey) {
		if s, ok := pick(prey); ok {
			picks[0] = s
		}
	}
	if len(picks) > 1 && !has(predator) {
		if s, ok := pick(predator); ok {
			picks[len(picks)-1] = s
		}
	}
	if len(picks) > 1 {
		sort.SliceStable(picks[1:], func(i, j int) bool { return predator(picks[i+1]) && !predator(picks[j+1]) })
	}
	count, preds := 0, 0
	add := func(s Species, room int) {
		if EcosystemWeight(s) > room {
			if fit, ok := pick(func(sp Species) bool { return EcosystemWeight(sp) <= room && predator(sp) == predator(s) }); ok {
				s = fit
			} else if fit, ok := pick(func(sp Species) bool { return EcosystemWeight(sp) <= room }); ok {
				s = fit
			} else if count > 0 {
				return
			}
		}
		k := place(s, room)
		count += k
		if predator(s) {
			preds += k
		}
	}
	for i, s := range picks {
		room := target - count
		if i == 0 {
			room = max(room, min(capacity, RosterOf(s).HerdMin*EcosystemWeight(s)))
		}
		if room > 0 {
			add(s, room)
		}
	}
	for failed := 0; count < target && failed < 32; {
		room := target - count
		limit := room
		if count == 0 {
			limit = -1
		} // first indivisible fallback
		s, ok := PickEcosystemGroup(count, preds, limit, roll, pick)
		if !ok {
			break
		}
		before := count
		add(s, room)
		if count == before {
			failed++
		} else {
			failed = 0
		}
	}
}
