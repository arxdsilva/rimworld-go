// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

// Trainability is the highest ordinary training order an animal can learn.
// The zero value is intentionally invalid: all declared species need an
// explicit row. Pen livestock, other untrainable species, hive monsters
// and original hostile mechs have no normal training.
// wiki: https://rimworldwiki.com/wiki/List_of_animals (SIM-047)
// wiki: https://rimworldwiki.com/wiki/Trainability (SIM-047)
type Trainability uint8

const (
	TrainUnspecified Trainability = iota
	TrainNone
	TrainSimple
	TrainIntermediate
	TrainAdvanced
)

// trainability gives each species its training class. Original creatures
// retain their own tuning, with the corresponding published animal as
// reference where applicable (see husbandry.go). Ordinary skill access
// follows class thresholds, avoiding size/behavior heuristics.
var trainability = [numSpecies]Trainability{
	Deer:             TrainNone,
	TimberWolf:       TrainAdvanced,
	Alpaca:           TrainNone,
	Bison:            TrainNone,
	Emberbuck:        TrainNone, // original tuning
	Capybara:         TrainNone,
	Caribou:          TrainNone,
	Cow:              TrainNone,
	Donkey:           TrainNone,
	Dromedary:        TrainNone,
	Elephant:         TrainAdvanced,
	Elk:              TrainNone,
	Gazelle:          TrainNone,
	Goat:             TrainNone,
	Horse:            TrainNone,
	Ibex:             TrainNone,
	GroundSloth:      TrainAdvanced, // original tuning
	Woolhorn:         TrainNone,     // original tuning
	Pig:              TrainNone,
	Rhinoceros:       TrainIntermediate,
	Sheep:            TrainNone,
	Yak:              TrainNone,
	Timbergnaw:       TrainNone, // original tuning
	ArcticFox:        TrainIntermediate,
	ArcticWolf:       TrainAdvanced,
	CinderRat:        TrainNone, // original tuning
	Cat:              TrainNone,
	Chinchilla:       TrainNone,
	Cougar:           TrainAdvanced,
	FennecFox:        TrainIntermediate,
	GrizzlyBear:      TrainAdvanced,
	GuineaPig:        TrainNone,
	Hare:             TrainNone,
	Husky:            TrainAdvanced,
	Labrador:         TrainAdvanced,
	Lynx:             TrainIntermediate,
	Monkey:           TrainIntermediate,
	Panther:          TrainAdvanced,
	PolarBear:        TrainAdvanced,
	Raccoon:          TrainNone,
	Rat:              TrainNone,
	RedFox:           TrainIntermediate,
	Snowhare:         TrainNone,
	Squirrel:         TrainNone,
	DireWolf:         TrainIntermediate, // original tuning
	WildBoar:         TrainNone,
	YorkshireTerrier: TrainIntermediate,
	Cassowary:        TrainNone,
	Chicken:          TrainNone,
	Duck:             TrainNone,
	Emu:              TrainNone,
	Goose:            TrainNone,
	Ostrich:          TrainNone,
	Turkey:           TrainNone,
	Cobra:            TrainNone,
	Iguana:           TrainNone,
	Tortoise:         TrainNone,
	HiveBeetle:       TrainNone,     // original tuning
	HiveBrute:        TrainNone,     // original tuning
	HiveCrawler:      TrainNone,     // original tuning
	Glimmerstag:      TrainAdvanced, // original tuning
	Harrow:           TrainNone,     // original tuning
	Spindle:          TrainNone,     // original tuning
	Bulwark:          TrainNone,     // original tuning
	Quill:            TrainNone,     // original tuning
}

// TrainabilityOf returns the explicitly defined class. Invalid IDs
// cannot be trained; callers do not index beyond the saved species enum.
func TrainabilityOf(s Species) Trainability {
	if s >= numSpecies {
		return TrainNone
	}
	return trainability[s]
}
