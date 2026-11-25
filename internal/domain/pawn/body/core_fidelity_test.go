// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"rimworld-go/internal/domain/sim/clock"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// Published Core capacities are permille, including improvements above 100%.
func TestCoreOrganCapacityAndRestFactors(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Body)
		digestion int32
		moving    int32
		cons      int32
		rest      int32
		fatal     bool
	}{
		{"healthy", func(*Body) {}, 1000, 1000, 1000, 1000, false},
		{"missing stomach", func(b *Body) { b.destroy(Stomach) }, 500, 1000, 1000, 850, false},
		{"synthetic stomach", func(b *Body) {
			b.destroy(Stomach)
			b.SetReplacement(Stomach, item.SynthStomach)
		}, 1125, 1000, 1000, 1037, false},
		{"synthetic heart", func(b *Body) {
			b.destroy(Heart)
			b.SetReplacement(Heart, item.PumpHeart)
		}, 1000, 1050, 1000, 1075, false},
		{"prosthetic heart", func(b *Body) {
			b.destroy(Heart)
			b.SetReplacement(Heart, item.ValveHeart)
		}, 1000, 921, 960, 940, false},
		{"one lung", func(b *Body) { b.destroy(LungL) }, 1000, 810, 900, 850, false},
		{"two lungs", func(b *Body) { b.destroy(LungL); b.destroy(LungR) }, 1000, 640, 800, 700, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b Body
			tc.setup(&b)
			if got := b.Capacity(Digestion); got != tc.digestion {
				t.Errorf("digestion = %d, want %d", got, tc.digestion)
			}
			if got := b.Capacity(Moving); got != tc.moving {
				t.Errorf("moving = %d, want %d", got, tc.moving)
			}
			if got := b.Capacity(Consciousness); got != tc.cons {
				t.Errorf("consciousness = %d, want %d", got, tc.cons)
			}
			if got := RestRateMultiplier(&b); got != tc.rest {
				t.Errorf("rest multiplier = %d, want %d", got, tc.rest)
			}
			if b.VitalLost() != tc.fatal {
				t.Errorf("fatal = %v, want %v", b.VitalLost(), tc.fatal)
			}
		})
	}
}

func TestCorePainThresholdAndJoywire(t *testing.T) {
	var b Body
	b.Wounds = []Wound{{Part: ArmL, Dmg: 8, Kind: Cut}}
	if got := b.Capacity(Consciousness); got != 1000 {
		t.Fatalf("10%% pain should retain full consciousness: %d", got)
	}
	b.Wounds[0].Dmg = 12
	if got := b.Capacity(Consciousness); got != 978 {
		t.Fatalf("15%% pain should reduce consciousness about 2.2%%: %d", got)
	}
	if !AddImplant(&b, Brain, item.BlissCircuit) {
		t.Fatal("could not attach bliss circuit")
	}
	if got := b.Capacity(Consciousness); got != 782 {
		t.Fatalf("joywire lowers brain efficiency before pain factor: %d", got)
	}
}

func TestCoreNaturalOrganSurgeryRequirements(t *testing.T) {
	for _, tc := range []struct {
		part  Part
		organ item.Kind
		skill int
	}{
		{KidneyL, item.Kidney, 5},
		{LungR, item.Lung, 5},
		{Liver, item.Liver, 5},
		{Heart, item.Heart, 8},
	} {
		op := Operation{Kind: InstallOrgan, Part: tc.part, Item: tc.organ}
		if got := OperationMinSkill(op); got != tc.skill {
			t.Errorf("%s Medical requirement %d, want %d", tc.part.Label(), got, tc.skill)
		}
		if got := OperationWork(op); got != 2500 {
			t.Errorf("%s installation work %d, want 2500", tc.part.Label(), got)
		}
		harvest := Operation{Kind: HarvestOrgan, Part: tc.part}
		if got := OperationWork(harvest); got != 2000 {
			t.Errorf("%s harvest work %d, want 2000", tc.part.Label(), got)
		}
	}
	if qty, _ := MedicineNeed(HarvestOrgan); qty != 1 {
		t.Fatalf("harvest medicine = %d, want one", qty)
	}
}

func TestSyntheticStomachCuresExistingGutWorms(t *testing.T) {
	var b Body
	if !b.AddCond(Condition{Kind: CondGutWorms, Part: Torso, Sev: 1000}) {
		t.Fatal("failed to infect patient")
	}
	op := Operation{Kind: InstallImplant, Part: Stomach, Item: item.SynthStomach}
	if !b.Apply(op, Success, nil, 0) || !b.GutWormImmune() {
		t.Fatal("failed to install synthetic stomach")
	}
	if _, infected := b.Cond(CondGutWorms); infected {
		t.Fatal("existing gut worms survived synthetic stomach installation")
	}
}

// Cataracts and hearing loss change one natural part before the 75/25
// better-side weighting, including when the other side is bionic.
func TestCoreChronicPartEfficiency(t *testing.T) {
	var b Body
	b.Develop(CondCataract, EyeL, 0)
	if got := b.Capacity(Sight); got != 875 {
		t.Fatalf("one cataract sight = %d", got)
	}
	b.destroy(EyeR)
	b.SetReplacement(EyeR, item.OpticImplant)
	if got := b.Capacity(Sight); got != 1062 {
		t.Fatalf("bionic eye and cataract sight = %d", got)
	}
	b = Body{}
	b.Develop(CondHearingLoss, EarR, 0)
	if got := b.Capacity(Hearing); got != 875 {
		t.Fatalf("one damaged ear hearing = %d", got)
	}
	b = Body{}
	b.Develop(CondCarcinoma, Liver, 0)
	if got := b.Capacity(Digestion); got != 875 {
		t.Fatalf("early liver carcinoma digestion = %d", got)
	}
	c, _ := b.Cond(CondCarcinoma)
	c.Sev, c.Stage = 500, 2
	if got := b.Capacity(Digestion); got != 750 {
		t.Fatalf("major liver carcinoma digestion = %d", got)
	}
	if got := b.Capacity(BloodFiltration); got != 500 {
		t.Fatalf("major liver carcinoma filtration = %d", got)
	}
	if got := OperationWork(Operation{Kind: ExciseCarcinoma, Part: Liver}); got != 4500 {
		t.Fatalf("excision work = %d", got)
	}
}

func TestCoreCarcinomaCareAndPainlessness(t *testing.T) {
	const hour = uint64(clock.TicksPerHour)
	var untreated Body
	untreated.Develop(CondCarcinoma, Liver, 0)
	var tended Body
	tended.Develop(CondCarcinoma, Liver, 0)
	if n := tended.TendAll(1000, 0); n != 1 {
		t.Fatalf("carcinoma not tendable: %d", n)
	}
	var salt uint64
	for CarcinomaGrowth(untreated.Conds[0], salt) != Growing {
		salt++
	}
	for h := uint64(1); h <= 72; h++ {
		untreated.StepAging(h*hour, salt, nil)
		tended.StepAging(h*hour, salt, nil)
	}
	c1, _ := untreated.Cond(CondCarcinoma)
	c2, _ := tended.Cond(CondCarcinoma)
	if c1 == nil || c2 == nil || c2.Sev >= c1.Sev {
		t.Fatalf("treatment failed to reduce progression: untreated=%+v tended=%+v", c1, c2)
	}
	var painless Body
	painless.Develop(CondCarcinoma, ArmL, 0)
	c, _ := painless.Cond(CondCarcinoma)
	c.Rate = -1
	if painless.Pain() != 0 || painless.Capacity(Manipulation) >= 1000 {
		t.Fatal("painless cancer must still impair the affected part")
	}
}

func TestCoreAsthmaAndAlzheimersStages(t *testing.T) {
	var asthma Body
	if !asthma.Develop(CondAsthma, LungL, 0) {
		t.Fatal("asthma onset")
	}
	if got := asthma.Capacity(Breathing); got != 950 {
		t.Fatalf("minor asthma breathing = %d", got)
	}
	for h := uint64(1); h <= 48; h++ {
		asthma.StepAging(h*clock.TicksPerHour, 1, nil)
	}
	c, _ := asthma.Cond(CondAsthma)
	if c.Sev != 500 || asthma.Capacity(Breathing) != 750 {
		t.Fatalf("untreated two-day asthma = %+v, breathing=%d", c, asthma.Capacity(Breathing))
	}
	asthma.TendAll(1000, 48*clock.TicksPerHour)
	for h := uint64(49); h <= 73; h++ {
		asthma.StepAging(h*clock.TicksPerHour, 1, nil)
	}
	if asthma.Capacity(Breathing) <= 750 {
		t.Fatal("quality asthma treatment did not restore some lung efficiency")
	}
	var alz Body
	if !alz.Develop(CondAlzheimers, Brain, 0) {
		t.Fatal("Alzheimer's onset")
	}
	if alz.Capacity(Consciousness) != 950 {
		t.Fatalf("early Alzheimer's consciousness = %d", alz.Capacity(Consciousness))
	}
	alz.StepAging(200*clock.TicksPerDay, 1, nil)
	if alz.Capacity(Consciousness) != 850 {
		t.Fatalf("advanced Alzheimer's consciousness = %d", alz.Capacity(Consciousness))
	}
}

func TestCoreHeartAttackIntervalTimer(t *testing.T) {
	var b Body
	b.Develop(CondHeartAttack, Heart, 0)
	maxRoll := func(n int) int { return n - 1 }
	for h := 1; h <= 3; h++ {
		dead, cured, _ := b.StepHeartAttack(maxRoll)
		if dead || cured {
			t.Fatal("heart attack progressed before 10000-tick interval")
		}
		c, _ := b.Cond(CondHeartAttack)
		if c.Sev != HeartAttackOnset {
			t.Fatalf("severity changed before the interval: %d", c.Sev)
		}
	}
	dead, _, _ := b.StepHeartAttack(maxRoll)
	if !dead {
		t.Fatal("10000-tick interval did not trigger the next heart attack severity")
	}
}

func TestCoreAnestheticRecoverySymptoms(t *testing.T) {
	const hour = uint64(clock.TicksPerHour)
	var b Body
	b.Wounds = []Wound{{Part: ArmL, Dmg: 8, Kind: Cut}}
	AnesthetizeFor(&b, 10, 75000)
	if b.Pain() != 0 || VomitEvery(&b) != 6 {
		t.Fatalf("sedated pain=%d vomit=%d", b.Pain(), VomitEvery(&b))
	}
	b.StepAnesthetic(10 + 7*hour)
	if b.Capacity(Sight) != 850 || b.Capacity(Digestion) != 800 || b.Capacity(Talking) != 500 {
		t.Fatalf("woozy symptoms sight=%d digestion=%d talking=%d", b.Capacity(Sight), b.Capacity(Digestion), b.Capacity(Talking))
	}
	if b.Pain() != 80 || !AnestheticWoozy(&b) {
		t.Fatalf("woozy pain=%d, not in happy recovery stage", b.Pain())
	}
	b.StepAnesthetic(10 + 13*hour)
	if b.Capacity(Moving) != 850 || b.Capacity(Manipulation) != 785 || VomitEvery(&b) != 96 {
		t.Fatalf("wearing-off symptoms moving=%d manipulation=%d nausea=%d",
			b.Capacity(Moving), b.Capacity(Manipulation), VomitEvery(&b))
	}
	if b.Pain() != 95 {
		t.Fatalf("wearing-off pain=%d", b.Pain())
	}
	b.StepAnesthetic(10 + 20*hour)
	if !b.Anesthetized() {
		t.Fatal("75000-tick anesthesia ended early")
	}
	if !b.StepAnesthetic(10+75000) || b.Anesthetized() {
		t.Fatal("anesthesia remained after its rolled duration")
	}
}

func TestCoreJawAffectsEatingSpeed(t *testing.T) {
	var b Body
	if EatingSpeed(&b) != 1000 {
		t.Fatalf("healthy eating speed = %d", EatingSpeed(&b))
	}
	b.destroy(Jaw)
	b.SetReplacement(Jaw, item.ServoJaw)
	if got := EatingSpeed(&b); got < 1237 || got > 1238 {
		t.Fatalf("bionic jaw eating speed = %d, want 1237-1238", got)
	}
	b = Body{}
	b.destroy(Jaw)
	b.SetReplacement(Jaw, item.Denture)
	if got := EatingSpeed(&b); got != 810 {
		t.Fatalf("denture eating speed = %d", got)
	}
}

func TestCoreAsthmaCanReceiveHealthyLung(t *testing.T) {
	var b Body
	b.Develop(CondAsthma, LungR, 0)
	op := Operation{Kind: InstallOrgan, Part: LungR, Item: item.Lung}
	if !b.CanOperate(op) {
		t.Fatal("asthmatic lung cannot be replaced")
	}
	if !b.Apply(op, Success, nil, 1) {
		t.Fatal("lung transplant failed")
	}
	if b.condOn(CondAsthma, LungR) || b.Capacity(Breathing) != 1000 {
		t.Fatalf("new lung still has asthma: %+v", b.Conds)
	}
}

// Mood suppression thresholds are small per-thought rates, expressed as
// fractions of ten thousand. Mood owns the deterministic selection.
func TestAlzheimerThoughtNullificationOdds(t *testing.T) {
	var b Body
	if AlzheimerThoughtNullification(&b) != 0 || AlzheimerThoughtNullification(nil) != 0 {
		t.Fatal("healthy pawn suppresses conditional thoughts")
	}
	b.Develop(CondAlzheimers, Brain, 0)
	c, _ := b.Cond(CondAlzheimers)
	for stage, want := range []int{0, 1500, 3300, 5000} {
		c.Stage = uint8(stage)
		if got := AlzheimerThoughtNullification(&b); got != want {
			t.Errorf("stage %d suppression=%d/10000, want=%d", stage, got, want)
		}
	}
}

// Part-efficiency modifiers are additive offsets on top of injury,
// then clamped; they are not percentages of already-injured efficiency.
func TestCoreChronicOffsetsOnDamagedParts(t *testing.T) {
	var cataract Body
	eyeHP := cataract.maxHP(EyeL)
	cataract.Wounds = []Wound{{Part: EyeL, Dmg: eyeHP / 2, Kind: Cut}}
	before := cataract.eff(EyeL)
	cataract.Develop(CondCataract, EyeL, 0)
	if got, want := cataract.eff(EyeL), max(int32(0), before-500); got != want {
		t.Fatalf("injured cataract eye efficiency=%d, want %d", got, want)
	}
	if cataract.Capacity(Sight) != 750 {
		t.Fatalf("healthy right eye + blind injured left eye: sight %d", cataract.Capacity(Sight))
	}

	var cancer Body
	armHP := cancer.maxHP(ArmL)
	cancer.Wounds = []Wound{{Part: ArmL, Dmg: armHP / 2, Kind: Cut}}
	before = cancer.eff(ArmL)
	cancer.Develop(CondCarcinoma, ArmL, 0)
	c, _ := cancer.Cond(CondCarcinoma)
	c.Sev, c.Stage = 500, 2
	if got, want := cancer.eff(ArmL), max(int32(0), before-500); got != want {
		t.Fatalf("major carcinoma offset on damaged arm=%d, want %d", got, want)
	}

	var lungs Body
	lungHP := lungs.maxHP(LungL)
	lungs.Wounds = []Wound{{Part: LungL, Dmg: lungHP / 2, Kind: Cut}}
	before = lungs.eff(LungL)
	lungs.Develop(CondAsthma, LungL, 0)
	if got, want := lungs.eff(LungL), max(int32(0), before-100); got != want {
		t.Fatalf("minor asthma on damaged lung=%d, want %d", got, want)
	}
	c, _ = lungs.Cond(CondAsthma)
	c.Sev, c.Stage = 500, 2
	if got, want := lungs.eff(LungL), max(int32(0), before-500); got != want {
		t.Fatalf("major asthma on damaged lung=%d, want %d", got, want)
	}

	var brain Body
	brainHP := brain.maxHP(Brain)
	brain.Wounds = []Wound{{Part: Brain, Dmg: brainHP / 2, Kind: Cut}}
	before = brain.eff(Brain)
	brain.Develop(CondAlzheimers, Brain, 0)
	if got, want := brain.eff(Brain), max(int32(0), before-50); got != want {
		t.Fatalf("Alzheimer's offset on damaged brain=%d, want %d", got, want)
	}
}

// A birthday incident from the published Ailments graph selects both natural human lungs.
func TestCoreHumanAsthmaBirthday(t *testing.T) {
	ageDays := 40 * clock.DaysPerYear
	lifeDays := 80 * clock.DaysPerYear
	draws := 0
	if !AsthmaBirthday(ageDays, lifeDays, func(n int) int {
		draws++
		if n != OnsetScale {
			t.Fatalf("roll scale = %d", n)
		}
		return 0
	}) || draws != 1 {
		t.Fatal("human birthday did not roll a single asthma incident")
	}
	var b Body
	if !DevelopAsthma(&b, 100) || !b.condOn(CondAsthma, LungL) || !b.condOn(CondAsthma, LungR) {
		t.Fatalf("human asthma did not affect both lungs: %+v", b.Conds)
	}
	if DevelopAsthma(&b, 101) {
		t.Fatal("already asthmatic human received another copy")
	}
	draws = 0
	if AsthmaBirthday(ageDays-1, lifeDays, func(int) int { draws++; return 0 }) || draws != 0 {
		t.Fatal("asthma was rolled outside the birthday")
	}
}

func TestTemporaryHealthFractionIgnoresPermanentScars(t *testing.T) {
	b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 25, Kind: Cut, Permanent: true}, {Part: ArmR, Dmg: 20, Kind: Cut}}}
	if got := TemporaryHealthFraction(&b, 45, 100); got != .8 {
		t.Fatalf("temporary wound only: %v, want .8", got)
	}
	b.Wounds = b.Wounds[:1]
	if got := TemporaryHealthFraction(&b, 25, 100); got != 1 {
		t.Fatalf("permanent scar only: %v, want 1", got)
	}
	if got := TemporaryHealthFraction(nil, 25, 100); got != .75 {
		t.Fatalf("legacy injury: %v, want .75", got)
	}
}
