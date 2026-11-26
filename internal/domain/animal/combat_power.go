// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/pawn/body"
)

// CombatPower gives the public natural species combat power table.
// Original species are not assigned invented vanilla values.
// https://rimworldwiki.com/wiki/List_of_animals/Additional
func CombatPower(s Species) int {
	switch s {
	case Deer:
		return 50
	case TimberWolf, ArcticWolf, Husky, Cow, Yak:
		return 75
	case Alpaca, Cobra:
		return 65
	case Bison, Horse:
		return 100
	case Capybara, ArcticFox, FennecFox, RedFox, Goat, Pig:
		return 45
	case Caribou, Elk:
		return 110
	case Donkey, Lynx:
		return 80
	case Dromedary:
		return 90
	case Elephant:
		return 260
	case Gazelle, Iguana:
		return 40
	case Ibex, Turkey:
		return 50
	case Rhinoceros:
		return 270
	case Sheep, WildBoar:
		return 55
	case Cat, Monkey, Raccoon, Goose:
		return 35
	case Chinchilla, GuineaPig, Hare, Snowhare, Squirrel:
		return 33
	case Cougar, Panther:
		return 120
	case GrizzlyBear, PolarBear:
		return 200
	case Labrador, Emu, Tortoise:
		return 70
	case Rat, YorkshireTerrier, Chicken, Duck:
		return 30
	case Cassowary:
		return 60
	case Ostrich:
		return 85
	default:
		return 0
	}
}

// ThreatPower counts a living, standing, owned animal that can learn
// attack. Learned training and biological age do not change eligibility.
func ThreatPower(a State) float64 {
	if !a.Tame || a.Downed || a.Dead() || TrainabilityOf(a.Species) != TrainAdvanced {
		return 0
	}
	hp := body.TemporaryHealthFraction(a.Body, a.Injury, DefOf(a.Species).MaxHP)
	return float64(CombatPower(a.Species)) * difficulty.InjuryWeight(hp)
}
