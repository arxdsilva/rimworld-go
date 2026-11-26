// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"bytes"
	"encoding/gob"
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestImplantAccessor: Fitted grows only as far as the last fitted part
// and goes back to nil once nothing is fitted, so a body without implants
// saves (and hashes) as before.
func TestImplantAccessor(t *testing.T) {
	var b Body
	if b.ReplacementAt(LegL) != 0 || b.Replacements != nil {
		t.Fatal("a fresh body has no implants")
	}
	b.SetReplacement(EyeR, 0)
	if b.Replacements != nil {
		t.Fatal("clearing an empty slot must not grow Fitted")
	}
	b.SetReplacement(EyeL, item.GlassEye)
	if len(b.Replacements) != int(EyeL)+1 || b.ReplacementAt(EyeL) != item.GlassEye || b.ReplacementAt(LegL) != 0 {
		t.Fatalf("fitted %v", b.Replacements)
	}
	b.SetReplacement(LegL, item.PegLeg)
	if len(b.Replacements) != int(LegL)+1 || b.ReplacementAt(LegL) != item.PegLeg || b.ArtificialParts() != 2 {
		t.Fatalf("fitted %v", b.Replacements)
	}
	b.SetReplacement(LegL, 0)
	if len(b.Replacements) != int(EyeL)+1 {
		t.Fatalf("trailing empty slots are trimmed: %v", b.Replacements)
	}
	b.SetReplacement(EyeL, 0)
	if b.Replacements != nil {
		t.Fatalf("no implants left: %v", b.Replacements)
	}
	b.SetReplacement(NumParts, item.PegLeg)
	if b.Replacements != nil || b.ReplacementAt(NumParts) != 0 {
		t.Fatal("an unknown part takes nothing")
	}
}

// TestInstallRemoveUseFitted: the checked install and removal keep Fitted
// as small as SetImplant does.
func TestInstallRemoveUseFitted(t *testing.T) {
	var b Body
	b.destroy(FootR)
	if !b.InstallReplacement(FootR, item.ProstheticFoot) || b.ReplacementAt(FootR) != item.ProstheticFoot || len(b.Replacements) != int(FootR)+1 {
		t.Fatalf("install: %v", b.Replacements)
	}
	if b.InstallReplacement(FootR, item.ProstheticFoot) {
		t.Fatal("a fitted part takes no second implant")
	}
	if k, ok := b.RemoveReplacement(FootR); !ok || k != item.ProstheticFoot || b.Replacements != nil {
		t.Fatalf("remove: %v %v %v", k, ok, b.Replacements)
	}
	if _, ok := b.RemoveReplacement(FootR); ok {
		t.Fatal("nothing left to remove")
	}
}

// TestFittedFromLegacyArray: an older save's fixed-size per-part array
// becomes the same implants in Fitted.
func TestFittedFromLegacyArray(t *testing.T) {
	var legacy [20]item.Kind
	if ReplacementsFrom(legacy[:]) != nil {
		t.Fatal("an empty array has no implants")
	}
	legacy[LegL], legacy[Nose] = item.PegLeg, item.WoodenNose
	f := ReplacementsFrom(legacy[:])
	b := Body{Replacements: f}
	if len(f) != int(LegL)+1 || b.ReplacementAt(LegL) != item.PegLeg || b.ReplacementAt(Nose) != item.WoodenNose || b.ReplacementAt(LegR) != 0 {
		t.Fatalf("fitted %v", f)
	}
}

// TestBodyGobKeepsMedicalFields: the appended wound and body fields
// survive a gob round trip.
func TestBodyGobKeepsMedicalFields(t *testing.T) {
	b := Body{
		Wounds:    []Wound{{Part: ArmL, Dmg: 4, Kind: Cut, TendQ: 650, TendAt: 7, InfectAt: 99, Bleed: 120}},
		BloodLoss: 210,
		Conds:     []Condition{{Kind: 3, Part: Torso, Sev: 400, Immunity: 200, TendQ: 500, TendSum: 900, TendUntil: 5, Since: 2, Stage: 1}},
	}
	b.SetReplacement(EarL, item.HearingTrumpet)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&b); err != nil {
		t.Fatal(err)
	}
	var got Body
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Wounds, b.Wounds) || got.BloodLoss != b.BloodLoss || !slices.Equal(got.Conds, b.Conds) || !slices.Equal(got.Replacements, b.Replacements) {
		t.Fatalf("%+v", got)
	}
}
