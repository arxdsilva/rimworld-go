// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
)

func TestCoreArtificialCatalogueAndMedicalSkills(t *testing.T) {
	t.Parallel()
	variants := []struct {
		k          item.Kind
		p          Part
		efficiency int32
		medical    int
	}{
		{item.ProstheticFoot, FootL, 800, 3},
		{item.ServoArm, ArmL, 1250, 5},
		{item.ServoLeg, LegR, 1250, 5},
		{item.OpticImplant, EyeR, 1250, 5},
		{item.CochlearImplant, EarL, 1250, 5},
		{item.ServoJaw, Jaw, 1250, 5},
		{item.SynthTongue, Tongue, 1000, 5},
		{item.SpinalBrace, Spine, 1000, 5},
		{item.PumpHeart, Heart, 1250, 5},
		{item.SynthStomach, Stomach, 1250, 5},
		{item.ApexArm, ArmR, 1500, 5},
		{item.ApexLeg, LegL, 1500, 5},
		{item.ApexEye, EyeL, 1500, 5},
		{item.ServoClaw, HandR, 1000, 4},
		{item.AcousticImplant, EarR, 650, 4},
	}
	for _, tc := range variants {
		if !FitsReplacement(tc.k, tc.p) || ImplantEfficiency(tc.k) != tc.efficiency || MedicalSkill(tc.k) != tc.medical {
			t.Errorf("%s: fit/eff/skill = %v/%d/%d", tc.k, FitsReplacement(tc.k, tc.p), ImplantEfficiency(tc.k), MedicalSkill(tc.k))
		}
		if OperationMinSkill(Operation{Kind: InstallImplant, Part: tc.p, Item: tc.k}) != tc.medical {
			t.Errorf("%s: surgery gate incorrect", tc.k)
		}
	}
	if FitsReplacement(item.BlissCircuit, Brain) || FitsImplant(item.ServoArm, Brain) {
		t.Fatal("replacement and additive definitions overlap")
	}
	for _, k := range item.ImplantKinds() {
		found := false
		for p := Part(0); p < NumParts; p++ {
			found = found || FitsReplacement(k, p) || FitsImplant(k, p)
		}
		if !found {
			t.Errorf("%s has no possible surgical target", k)
		}
	}
}

func TestCoreEarAndEyeWeightsAndTongue(t *testing.T) {
	var b Body
	b.destroy(EyeL)
	b.destroy(EarL)
	if b.Capacity(Sight) != 750 || b.Capacity(Hearing) != 750 {
		t.Fatalf("missing left eye and ear %d %d", b.Capacity(Sight), b.Capacity(Hearing))
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: EarL, Item: item.AcousticImplant}, Success, nil, 0) {
		t.Fatal("acoustic implant on absent ear")
	}
	if got := b.Capacity(Hearing); got != (3*1000+650)/4 {
		t.Fatalf("cochlear hearing %d", got)
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: EyeL, Item: item.ApexEye}, Success, nil, 0) {
		t.Fatal("apex eye surgery")
	}
	if got := b.Capacity(Sight); got != (3*1500+1000)/4 {
		t.Fatalf("apex sight %d", got)
	}
	b.destroy(Tongue)
	if b.Capacity(Talking) != 0 {
		t.Fatalf("missing tongue: %d", b.Capacity(Talking))
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: Tongue, Item: item.SynthTongue}, Success, nil, 0) {
		t.Fatal("synth tongue surgery")
	}
	if b.Capacity(Talking) != 1000 {
		t.Fatal("synth tongue restored talking")
	}
	b.destroy(Jaw)
	if b.CanOperate(Operation{Kind: InstallImplant, Part: Tongue, Item: item.SynthTongue}) {
		t.Fatal("tongue cannot be installed without its natural jaw")
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: Jaw, Item: item.ServoJaw}, Success, nil, 0) {
		t.Fatal("servo jaw surgery")
	}
	if b.Capacity(Talking) == 0 {
		t.Fatal("servo jaw should restore jaw and tongue function")
	}
}

func TestProstheticInjuryRecoversWithoutScars(t *testing.T) {
	var b Body
	b.destroy(LegL)
	if !b.InstallReplacement(LegL, item.ServoLeg) {
		t.Fatal("install leg")
	}
	b.hurtPart(LegL, 12, Stab, 0)
	before := b.Capacity(Moving)
	if !HasDamagedReplacement(&b) || b.Pain() != 0 || b.BleedPerDay() != 0 || b.Damage() != 12 {
		t.Fatalf("artificial damage state: %+v", b)
	}
	for h := 0; h < 50 && HasDamagedReplacement(&b); h++ {
		b.HealHour(func(int) int { return 0 }, 0)
	}
	if HasDamagedReplacement(&b) || b.Capacity(Moving) <= before || b.Damage() != 0 || len(b.Wounds) != 0 {
		t.Fatalf("artificial part never recovered: %+v", b)
	}
}

func TestCoreSurgeryRecoveryAndUpgrades(t *testing.T) {
	var b Body
	if !b.Apply(Operation{Kind: InstallImplant, Part: HandL, Item: item.WoodenHand}, Success, nil, 0) {
		t.Fatal("wooden hand")
	}
	op := Operation{Kind: InstallImplant, Part: ArmL, Item: item.ApexArm}
	if !b.CanOperate(op) {
		t.Fatal("upgrade parent arm")
	}
	drops := SurgeryYields(&b, op)
	if len(drops) != 1 || drops[0] != (SurgeryDrop{item.Wood, 1}) {
		t.Fatalf("hand should refund one wood: %+v", drops)
	}
	if !b.Apply(op, Success, nil, 0) || b.ReplacementAt(HandL) != 0 || b.ReplacementAt(ArmL) != item.ApexArm {
		t.Fatal("arm upgrade didn't remove child")
	}
	op = Operation{Kind: InstallImplant, Part: ArmL, Item: item.ServoArm}
	drops = SurgeryYields(&b, op)
	if len(drops) != 1 || drops[0] != (SurgeryDrop{item.ApexArm, 1}) || !b.Apply(op, Success, nil, 0) {
		t.Fatalf("replacing an apex arm: %+v", drops)
	}
	op = Operation{Kind: RemoveImplant, Part: ArmL}
	if drops = SurgeryYields(&b, op); len(drops) != 1 || drops[0].Kind != item.ServoArm ||
		!b.Apply(op, Success, nil, 0) {
		t.Fatalf("removing servo arm: %+v", drops)
	}
	if !AddImplant(&b, Brain, item.NerveDampener) {
		t.Fatal("nerve dampener")
	}
	op = Operation{Kind: RemoveImplant, Part: Brain, Item: item.NerveDampener}
	if drops = SurgeryYields(&b, op); len(drops) != 1 || drops[0].Kind != item.NerveDampener ||
		!b.Apply(op, Success, nil, 0) {
		t.Fatalf("recover nerve dampener: %+v", drops)
	}
	var natural Body
	op = Operation{Kind: InstallImplant, Part: Heart, Item: item.PumpHeart}
	if drops = SurgeryYields(&natural, op); len(drops) != 1 || drops[0].Kind != item.Heart {
		t.Fatalf("natural heart salvage: %+v", drops)
	}
}

func TestCoreImplantedMeleeTools(t *testing.T) {
	var b Body
	if !b.Apply(Operation{Kind: InstallImplant, Part: HandL, Item: item.ServoClaw}, Success, nil, 0) {
		t.Fatal("servo claw installation")
	}
	ts, n := MeleeTools(&b)
	if n != 1 || ts[0].Damage != 22 || ts[0].Pen != 330 || ts[0].Form != damage.FormScratch ||
		b.Capacity(Moving) != 920 {
		t.Fatalf("servo claw melee %+v, moving %d", ts, b.Capacity(Moving))
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: ArmR, Item: item.ApexArm}, Success, nil, 0) {
		t.Fatal("apex arm")
	}
	ts, n = MeleeTools(&b)
	if n != 2 || ts[1].Name != "apex strike" || ts[1].Damage <= ts[0].Damage/2 {
		t.Fatalf("apex limb tool %+v", ts)
	}
}

func TestLegacyArtificialPartsStayRemovableButOutOfCorePicker(t *testing.T) {
	var b Body
	for _, k := range []item.Kind{item.GlassEye, item.WoodenNose, item.HearingTrumpet} {
		if ListedArtificial(k) {
			t.Fatalf("legacy item %s exposed as Core", k)
		}
	}
	for _, op := range AvailableOperations(&b) {
		if !ListedArtificial(op.Item) && op.Item != 0 {
			t.Fatalf("legacy non-Core part offered: %+v", op)
		}
	}
	b.destroy(EyeL)
	b.SetReplacement(EyeL, item.GlassEye)
	if !b.CanOperate(Operation{Kind: RemoveImplant, Part: EyeL}) {
		t.Fatal("old saved eye cannot be removed")
	}
	found := false
	for _, op := range AvailableOperations(&b) {
		found = found || op.Kind == RemoveImplant && op.Part == EyeL && op.Item == 0
	}
	if !found {
		t.Fatal("no removal choice for installed legacy eye")
	}
}

// Core improvised prostheses are installed directly from stock materials.
// The saved item kind still denotes the installed health state.
func TestCoreImprovisedSurgeryMaterialsAndWork(t *testing.T) {
	for _, tc := range []struct {
		kind     item.Kind
		resource item.Kind
		work     int
	}{
		{item.PegLeg, item.Wood, 1500},
		{item.WoodenHand, item.Wood, 1500},
		{item.ProstheticFoot, item.Wood, 1000},
		{item.Denture, 0, 1500},
		{item.ServoArm, item.ServoArm, 2500},
		{item.BlissCircuit, item.BlissCircuit, 2500},
	} {
		op := Operation{Kind: InstallImplant, Part: Torso, Item: tc.kind}
		if got := OperationMaterial(op); got != tc.resource {
			t.Errorf("%s material %s want %s", tc.kind, got, tc.resource)
		}
		if got := OperationWork(op); got != tc.work {
			t.Errorf("%s work %d want %d", tc.kind, got, tc.work)
		}
	}
	if OperationWork(Operation{Kind: RemoveImplant, Part: LegL}) != 2000 ||
		OperationWork(Operation{Kind: RemoveImplant, Part: Brain, Item: item.NerveDampener}) != 2500 {
		t.Fatal("Core removal time")
	}
}

// wiki: Death acidifier: the corpse retains burns from dissolved gear.
// The existing native caller selects a count of three to six.
func TestCoreCorrosiveBurnsAffectOnlyExposedTissue(t *testing.T) {
	var b Body
	b.Missing |= bit(ArmL) | bit(HandL)
	b.SetReplacement(ArmL, item.ServoArm)
	n := CorrodeOnDeath(&b, 6, func(max int) int { return max / 2 }, 57)
	if n != 6 || len(b.Wounds) != 6 {
		t.Fatalf("corpse burns %d / %d, expected six", n, len(b.Wounds))
	}
	for _, w := range b.Wounds {
		if w.Kind != CorrosiveBurn || w.Dmg != 1 || w.Part == ArmL || w.Part == HandL ||
			b.bleedRate(w) != 0 || WoundKindLabel(w.Kind) != "corrosive burn" {
			t.Fatalf("invalid chemical corpse injury: %+v", w)
		}
	}
	if b.ReplacementAt(ArmL) != item.ServoArm {
		t.Fatal("corrosion destroyed the artificial prosthesis")
	}
}

// Core natural punches require a real hand and the arm it is attached to.
func TestCoreNaturalFists(t *testing.T) {
	var b Body
	if NaturalFists(&b) != 2 || NaturalFists(nil) != 2 {
		t.Fatal("healthy humanlike has two natural fists")
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: HandL, Item: item.WoodenHand}, Success, nil, 0) {
		t.Fatal("wooden hand")
	}
	if got := NaturalFists(&b); got != 1 {
		t.Fatalf("wooden hand retains its fist: %d", got)
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: ArmR, Item: item.ProstheticArm}, Success, nil, 0) {
		t.Fatal("prosthetic arm")
	}
	if got := NaturalFists(&b); got != 0 {
		t.Fatalf("prosthetic arm retains its original fist: %d", got)
	}
	tools, n := MeleeTools(&b)
	if n != 1 || tools[0].Name != "prosthetic fist" || tools[0].Damage != 8 {
		t.Fatalf("prosthetic arm has no melee strike: %+v (n=%d)", tools, n)
	}
}
