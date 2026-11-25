// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package work

// keys are the stable names of the work types in definition files
// (PLG-025: a recipe's "skill"), indexed by Type.
var keys = [NumTypes]string{
	Firefight: "firefight", Doctor: "doctor", Grow: "grow", Cook: "cook", Construct: "construct",
	Hunt: "hunt", Haul: "haul", Clean: "clean", Research: "research", Mine: "mine", Craft: "craft",
	Handle: "handle", Warden: "warden", PlantCut: "plant_cut",
	Patient: "patient", BedRest: "bed_rest", Basic: "basic",
	Smith: "smith", Tailor: "tailor", Art: "art",
}

// Key is t's stable name ("cook"; "" for an unknown type).
func (t Type) Key() string {
	if t >= NumTypes {
		return ""
	}
	return keys[t]
}

// ParseType finds a work type by its key.
func ParseType(key string) (Type, bool) {
	for i, k := range keys {
		if k == key {
			return Type(i), true
		}
	}
	return 0, false
}
