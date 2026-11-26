// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"math"
	"rimworld-go/internal/domain/pawn/body"
	"testing"
)

func TestThreatPowerNativeTrainingEligibility(t *testing.T) {
	a := State{Species: TimberWolf, Tame: true}
	if got := ThreatPower(a); got != 75 {
		t.Fatalf("untrained wolf power %g", got)
	}
	a.Injury = DefOf(a.Species).MaxHP / 2
	if got := ThreatPower(a); math.Abs(got-75*(.35+.65*(1-float64(a.Injury)/float64(DefOf(a.Species).MaxHP)))) > 1e-9 {
		t.Fatalf("injured wolf power %g", got)
	}
	// A healed scar alone cannot make an otherwise eligible animal weaker for raid sizing.
	a.Body = NewBody(a.Species)
	a.Body.Wounds = []body.Wound{{Part: a.Body.Root(), Dmg: a.Injury, Permanent: true}}
	if got := ThreatPower(a); got != 75 {
		t.Fatalf("permanent scar reduced power: %v", got)
	}
	a.Downed = true
	if ThreatPower(a) != 0 {
		t.Fatal("downed animal contributes")
	}
	for _, s := range []Species{RedFox, Deer, Cow, Chicken} {
		if got := ThreatPower(State{Species: s, Tame: true}); got != 0 {
			t.Fatalf("ineligible %v power %g", s, got)
		}
	}
}
