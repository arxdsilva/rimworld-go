package body

import "testing"

func TestBleedRate(t *testing.T) {
	for _, c := range []struct {
		w    Wound
		want int32
	}{
		{Wound{Part: ArmL, Dmg: 10, Kind: Cut}, 600}, // 60 % a day
		{Wound{Part: ArmL, Dmg: 10, Kind: Bite}, 600},
		{Wound{Part: ArmL, Dmg: 10, Kind: Scratch}, 600},
		{Wound{Part: Torso, Dmg: 10, Kind: Surgical}, 600},
		{Wound{Part: Heart, Dmg: 10, Kind: Cut}, 3000}, // heart ×5
		{Wound{Part: ArmL, Dmg: 10, Kind: Bruise}, 0},
		{Wound{Part: ArmL, Dmg: 10, Kind: Burn}, 0},
		{Wound{Part: ArmL, Dmg: 10, Kind: Cut, Tended: true}, 0},
		{Wound{Part: ArmL, Dmg: 1, Kind: Cut, Permanent: true}, 0},
		{Wound{Part: ArmL, Kind: Cut}, 0},
	} {
		if got := BleedRate(c.w); got != c.want {
			t.Errorf("BleedRate(%+v) = %d, want %d", c.w, got, c.want)
		}
	}
	b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 10, Kind: Cut}, {Part: LegL, Dmg: 5, Kind: Bite}, {Part: LegL, Dmg: 9, Kind: Bruise}}}
	if b.BleedPerDay() != 900 || b.PartBleedPerDay(LegL) != 300 || b.PartBleedPerDay(Brain) != 0 {
		t.Fatalf("bleed %d, leg %d", b.BleedPerDay(), b.PartBleedPerDay(LegL))
	}
}

func TestBloodStageTable(t *testing.T) {
	for _, c := range []struct {
		loss int32
		prev uint8
		want uint8
	}{
		{0, 0, 0}, {149, 0, 0}, {150, 0, 1}, {299, 0, 1}, {300, 0, 2}, {450, 0, 3}, {600, 0, 4}, {1000, 0, 4},
		// entering jumps straight through every threshold passed
		{620, 1, 4},
		// leaving needs 50 below the threshold
		{560, 4, 4}, {550, 4, 4}, {549, 4, 3}, {400, 3, 3}, {399, 3, 2}, {100, 1, 1}, {99, 1, 0},
		// falling far drops every stage at once
		{0, 4, 0}, {260, 4, 2},
		// an out-of-range previous stage is clamped
		{700, 9, 4},
	} {
		if got := BloodStage(c.loss, c.prev); got != c.want {
			t.Errorf("BloodStage(%d, %d) = %d, want %d", c.loss, c.prev, got, c.want)
		}
	}
	if BloodStageLabel(4) != "extreme" || BloodStageLabel(0) != "" || BloodStageLabel(9) != "" {
		t.Error("labels")
	}
}

func TestBloodLossConsciousness(t *testing.T) {
	for _, c := range []struct {
		loss int32
		want int32
	}{{0, 1000}, {150, 900}, {300, 800}, {450, 600}, {600, 100}, {999, 100}} {
		b := Body{BloodLoss: c.loss}
		if got := b.Capacity(Consciousness); got != c.want {
			t.Errorf("loss %d: consciousness %d, want %d", c.loss, got, c.want)
		}
	}
	// the stored stage holds a body at the extreme cap just under 600
	b := Body{BloodLoss: 580, BloodStage: 4}
	if b.Capacity(Consciousness) != 100 {
		t.Fatalf("hysteresis: consciousness %d", b.Capacity(Consciousness))
	}
}

func TestStepBlood(t *testing.T) {
	b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 10, Kind: Cut}}} // 600/day = 25/h
	b.StepBlood(24)
	if b.BloodLoss != 600 || b.BloodStage != 4 {
		t.Fatalf("a day of a 10 cut: loss %d stage %d", b.BloodLoss, b.BloodStage)
	}
	if h := b.BleedDeathHours(); h != 16 {
		t.Fatalf("death in %d h", h)
	}
	b.TendAll(0, 0)
	if b.BleedPerDay() != 0 || b.BleedDeathHours() != -1 {
		t.Fatal("tending stops the bleeding")
	}
	b.StepBlood(24) // regen 14/h
	if b.BloodLoss != 600-24*14 || b.BloodStage != 2 {
		t.Fatalf("a day of regen: loss %d stage %d", b.BloodLoss, b.BloodStage)
	}
	b.StepBlood(100)
	if b.BloodLoss != 0 || b.BloodStage != 0 {
		t.Fatalf("regen floors at 0: %d", b.BloodLoss)
	}
	// a bleed too slow to round to an hour still loses blood
	b = Body{Wounds: []Wound{{Part: ArmL, Dmg: 1, Kind: Cut}}}
	b.StepBlood(1)
	if b.BloodLoss != 3 {
		t.Fatalf("slow bleed %d", b.BloodLoss)
	}
	// loss caps at death
	b = Body{Wounds: []Wound{{Part: Heart, Dmg: 20, Kind: Cut}}, BloodLoss: 990}
	b.StepBlood(5)
	if b.BloodLoss != BloodLossDeath {
		t.Fatalf("cap %d", b.BloodLoss)
	}
}

func TestPainPerHP(t *testing.T) {
	open := Body{Wounds: []Wound{{Part: Torso, Dmg: 8, Kind: Cut}}}
	if open.Pain() != 100 { // 12.5 per HP
		t.Fatalf("open pain %d", open.Pain())
	}
	open.TendAll(0, 0)
	if open.Pain() != 100 {
		t.Fatalf("tending keeps pain: %d", open.Pain())
	}
	bruise := Body{Wounds: []Wound{{Part: Torso, Dmg: 64, Kind: Bruise}}}
	if bruise.Pain() != 800 { // pain shock
		t.Fatalf("64 HP pain %d", bruise.Pain())
	}
	scar := Body{Wounds: []Wound{{Part: Torso, Dmg: 3, Kind: Cut, Permanent: true}}}
	if scar.Pain() != 3 {
		t.Fatalf("scar pain %d", scar.Pain())
	}
}

func TestFreshStumpHurtsAndBleedsUntilTended(t *testing.T) {
	var b Body
	b.hurtPart(HandL, 20, Cut, 0) // torn off
	if !b.isMissing(HandL) || !b.FreshStump(HandL) {
		t.Fatal("the hand is gone and its stump fresh")
	}
	if b.BleedPerDay() != 20*2*60 || b.PartBleedPerDay(HandL) != 2400 || b.Pain() != 250 {
		t.Fatalf("stump bleed %d pain %d", b.BleedPerDay(), b.Pain())
	}
	b.hurtPart(ArmL, 35, Cut, 0) // the arm goes too: one stump, the arm's
	if b.FreshStump(HandL) || !b.FreshStump(ArmL) || b.BleedPerDay() != 35*2*60 {
		t.Fatalf("stumps %b bleed %d", b.Fresh, b.BleedPerDay())
	}
	if b.TendAll(0, 0) != 1 || b.Fresh != 0 || b.BleedPerDay() != 0 || b.Pain() != 0 {
		t.Fatalf("tended stump: bleed %d pain %d", b.BleedPerDay(), b.Pain())
	}
	b.hurtPart(LegL, 40, Burn, 0) // a burn sears the stump shut
	if b.FreshStump(LegL) {
		t.Fatal("a burned-off leg does not bleed")
	}
	b.hurtPart(EarL, 10, Bruise, 0)
	if !b.FreshStump(EarL) || b.FreshStump(NumParts) {
		t.Fatal("ear stump")
	}
	b.SetImplant(EarL, 1)
	if b.FreshStump(EarL) || b.PartBleedPerDay(NumParts) != 0 {
		t.Fatal("an implant closes the stump")
	}
}
