// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

import "testing"

// TestEveryImplantHasValue: every implant is worth something (MED-00 gave
// the prosthetic foot, glass eye and wooden nose the values they lacked).
func TestEveryImplantHasValue(t *testing.T) {
	// wiki: Power claw, Cochlear implant, Joywire, Painstopper, Death acidifier (market value).
	// original tuning: the three project-specific legacy parts.
	want := map[Kind]int{
		ProstheticFoot: 15, GlassEye: 40, WoodenNose: 5,
		ServoClaw: 385, AcousticImplant: 220,
		BlissCircuit: 220, NerveDampener: 220, AcidFailsafe: 400,
	}
	for k := Kind(1); k < numKinds; k++ {
		if !k.IsImplant() {
			continue
		}
		if k.Value() <= 0 {
			t.Fatalf("%v has no value", k)
		}
		if v, ok := want[k]; ok && k.Value() != v {
			t.Fatalf("%v value %d, want %d", k, k.Value(), v)
		}
	}
}
