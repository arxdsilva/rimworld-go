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

func TestAdditiveBrainImplantsCoexistAndPersist(t *testing.T) {
	var b Body
	if !b.CanOperate(Operation{Kind: InstallImplant, Part: Brain, Item: item.BlissCircuit}) ||
		!b.Apply(Operation{Kind: InstallImplant, Part: Brain, Item: item.BlissCircuit}, Success, nil, 0) {
		t.Fatal("bliss circuit should install on an intact brain")
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: Brain, Item: item.NerveDampener}, Success, nil, 0) {
		t.Fatal("nerve dampener should coexist with bliss circuit")
	}
	if b.Missing != 0 || b.ReplacementAt(Brain) != 0 || b.ArtificialParts() != 2 {
		t.Fatalf("adding implants destroyed natural tissue: %+v", b)
	}
	if !slices.Equal(b.Implants, []Implant{{Brain, item.BlissCircuit}, {Brain, item.NerveDampener}}) {
		t.Fatalf("canonical implant order: %v", b.Implants)
	}
	if b.CanOperate(Operation{Kind: InstallImplant, Part: Brain, Item: item.BlissCircuit}) ||
		b.CanOperate(Operation{Kind: RemoveImplant, Part: Brain, Item: item.BlissCircuit}) {
		t.Fatal("duplicate or irreversible bliss circuit operation accepted")
	}
	if got := b.Capacity(Consciousness); got != 800 {
		t.Fatalf("bliss circuit consciousness = %d, want 800", got)
	}

	var save bytes.Buffer
	if err := gob.NewEncoder(&save).Encode(&b); err != nil {
		t.Fatal(err)
	}
	var restored Body
	if err := gob.NewDecoder(&save).Decode(&restored); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(restored.Implants, b.Implants) {
		t.Fatalf("round-trip implants: %+v", restored)
	}
	if !restored.Apply(Operation{Kind: RemoveImplant, Part: Brain, Item: item.NerveDampener}, Success, nil, 0) ||
		HasImplant(&restored, Brain, item.NerveDampener) || !HasImplant(&restored, Brain, item.BlissCircuit) {
		t.Fatal("removing nerve dampener affected the other brain implant")
	}
}

func TestNerveDampenerBlocksPainButNotInjuryOrBleeding(t *testing.T) {
	b := Body{Wounds: []Wound{{Part: ArmL, Kind: Cut, Dmg: 20}}}
	pain, bleeding, damage := b.Pain(), b.BleedPerDay(), b.Damage()
	if pain == 0 || bleeding == 0 || damage == 0 {
		t.Fatalf("invalid test setup: %d %d %d", pain, bleeding, damage)
	}
	if !AddImplant(&b, Brain, item.NerveDampener) || b.Pain() != 0 {
		t.Fatalf("nerve dampener pain = %d", b.Pain())
	}
	if b.BleedPerDay() != bleeding || b.Damage() != damage {
		t.Fatal("nerve dampener must not heal physical injuries")
	}
	if !DetachImplant(&b, Brain, item.NerveDampener) || b.Pain() != pain {
		t.Fatalf("removed nerve dampener pain = %d, want %d", b.Pain(), pain)
	}
}

func TestArtificialLegCanBeHitDamagedAndDestroyed(t *testing.T) {
	var b Body
	b.destroy(LegL)
	if !b.InstallReplacement(LegL, item.ServoLeg) {
		t.Fatal("could not replace missing leg")
	}
	hasLeg := false
	for roll := 0; roll < 1000; roll++ {
		if part, ok := b.PickPart(func(n int) int { return roll * n / 1000 }); ok && part == LegL {
			hasLeg = true
			break
		}
	}
	if !hasLeg {
		t.Fatal("replacement leg is absent from hit locations")
	}
	initial := b.Capacity(Moving)
	b.hurtPart(LegL, 10, Cut, 1)
	if b.ReplacementAt(LegL) != item.ServoLeg || b.Capacity(Moving) >= initial ||
		b.Pain() != 0 || b.BleedPerDay() != 0 {
		t.Fatalf("damaged artificial leg: %+v, moving %d", b, b.Capacity(Moving))
	}
	b.hurtPart(LegL, b.maxHP(LegL), Cut, 2)
	if b.ReplacementAt(LegL) != 0 || !b.isMissing(LegL) || b.Fresh&bit(LegL) != 0 {
		t.Fatalf("destroyed artificial leg: %+v", b)
	}
}

func TestDestroyingBrainRemovesAttachedDevices(t *testing.T) {
	var b Body
	AddImplant(&b, Brain, item.BlissCircuit)
	AddImplant(&b, Brain, item.NerveDampener)
	AddImplant(&b, Torso, item.AcidFailsafe)
	b.destroy(Brain)
	if len(b.Implants) != 1 || b.Implants[0].Kind != item.AcidFailsafe {
		t.Fatalf("brain loss should keep only the torso device: %v", b.Implants)
	}
	b.destroy(Torso)
	if b.Implants != nil {
		t.Fatalf("body loss should remove every device: %v", b.Implants)
	}
}

func TestRemovingArtificialHeartLeavesFatalOrganLoss(t *testing.T) {
	var b Body
	b.Missing |= bit(Heart)
	if !b.InstallReplacement(Heart, item.PumpHeart) || b.VitalLost() {
		t.Fatal("an artificial heart should sustain the pawn")
	}
	op := Operation{Kind: RemoveImplant, Part: Heart}
	if !b.CanOperate(op) || !b.Apply(op, Success, nil, 0) {
		t.Fatal("removing an artificial heart must be a valid operation")
	}
	if b.ReplacementAt(Heart) != 0 || !b.VitalLost() {
		t.Fatal("removing the only heart must be fatal")
	}
}
