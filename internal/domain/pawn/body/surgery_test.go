package body

import (
	"math/rand"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
)

// MED-06: surgery v2 (wiki: Doctoring, Vitals monitor, Anesthetic).

// plainOp is a level-8 surgeon with whole hands and eyes, herbal
// medicine, an implant install in a clean lit room, no bed or monitor.
func plainOp() SurgeryInput {
	return SurgeryInput{Skill: 8, Manip: 1000, Sight: 1000, LightPct: 100, Op: InstallImplant, Medicine: item.HerbalMedicine}
}

func TestSurgeryChanceTable(t *testing.T) {
	t.Parallel()
	for lvl, want := range []int32{100, 200, 300, 400, 500, 600, 700, 750, 800, 850, 900,
		920, 940, 960, 980, 980, 980, 980, 980, 980, 980} { // wiki base, capped at 98 %
		in := plainOp()
		in.Skill = lvl
		if c, _ := SurgeryChance(in); c != want {
			t.Errorf("Medicine %d: %d, want %d", lvl, c, want)
		}
	}
	mod := func(f func(*SurgeryInput)) SurgeryInput { in := plainOp(); f(&in); return in }
	cases := []struct {
		name string
		in   SurgeryInput
		want int32
	}{
		{"plain", plainOp(), 800},
		{"skill below 0", mod(func(in *SurgeryInput) { in.Skill = -3 }), 100},
		{"skill above 20", mod(func(in *SurgeryInput) { in.Skill = 99 }), 980},
		{"hospital bed ×1.15", mod(func(in *SurgeryInput) { in.HospitalBed = true }), 920},
		{"hospital bed and monitor", mod(func(in *SurgeryInput) { in.HospitalBed, in.Monitor = true, true }), 966},
		{"monitor ×1.05", mod(func(in *SurgeryInput) { in.Monitor = true }), 840},
		{"outdoors ×0.85", mod(func(in *SurgeryInput) { in.Outdoors = true }), 680},
		{"filthy ×0.6", mod(func(in *SurgeryInput) { in.CleanPct = -500 }), 480},
		{"sterile ×1.018", mod(func(in *SurgeryInput) { in.CleanPct = 100 }), 814},
		{"dark ×0.75", mod(func(in *SurgeryInput) { in.LightPct = 0 }), 600},
		{"half lit is enough", mod(func(in *SurgeryInput) { in.LightPct = 50 }), 800},
		{"quarter lit", mod(func(in *SurgeryInput) { in.LightPct = 25 }), 700},
		{"industrial", mod(func(in *SurgeryInput) { in.Medicine = item.IndustrialMedicine }), 880},
		{"nanomed", mod(func(in *SurgeryInput) { in.Medicine = item.NanomedKit }), 960},
		{"no medicine", mod(func(in *SurgeryInput) { in.Medicine = 0 }), 560},
		{"half manipulation", mod(func(in *SurgeryInput) { in.Manip = 500 }), 400},
		{"servo hands help", mod(func(in *SurgeryInput) { in.Manip = 1100 }), 880},
		{"sight caps at 100 %", mod(func(in *SurgeryInput) { in.Sight = 1250 }), 800},
		{"blind", mod(func(in *SurgeryInput) { in.Sight = 0 }), 0},
		{"amputate ×1.2", mod(func(in *SurgeryInput) { in.Op = Amputate }), 960},
		{"remove implant ×1.2", mod(func(in *SurgeryInput) { in.Op = RemoveImplant }), 960},
		{"harvest ×1.2", mod(func(in *SurgeryInput) { in.Op = HarvestOrgan }), 960},
		{"install organ ×1", mod(func(in *SurgeryInput) { in.Op = InstallOrgan }), 800},
		{"excise ×1", mod(func(in *SurgeryInput) { in.Op = ExciseCarcinoma }), 800},
		{"anesthetize ×2 capped", mod(func(in *SurgeryInput) { in.Op = Anesthetize }), 980},
		{"euthanize ×5 capped", mod(func(in *SurgeryInput) { in.Skill, in.Op = 0, Euthanize }), 500},
	}
	for _, c := range cases {
		if got, _ := SurgeryChance(c.in); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	_, fs := SurgeryChance(mod(func(in *SurgeryInput) { in.Skill, in.HospitalBed, in.Monitor = 12, true, true }))
	if fs[0] != (Factor{"Medicine skill 12", 94}) {
		t.Fatalf("the base comes first: %+v", fs)
	}
	seen := map[string]int32{}
	for _, f := range fs {
		seen[f.Label] = f.Pct
	}
	if seen["Hospital bed"] != 115 || seen["Vitals monitor"] != 105 || seen["Capped"] != 98 || seen["Light"] != 100 {
		t.Fatalf("factors %+v", fs)
	}
}

func TestCleanlinessFactor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		clean int32
		out   bool
		want  int32
	}{
		{-900, false, 600}, {-500, false, 600}, {-250, false, 800}, {-100, false, 920}, {0, false, 1000},
		{50, false, 1009}, {100, false, 1018}, {400, false, 1018}, {-500, true, 850}, {100, true, 850},
	} {
		if got := CleanFactor(c.clean, c.out); got != c.want {
			t.Errorf("CleanFactor(%d, outdoors %v) = %d, want %d", c.clean, c.out, got, c.want)
		}
	}
	for _, c := range []struct{ lit, want int32 }{{-5, 750}, {0, 750}, {10, 800}, {49, 995}, {50, 1000}, {100, 1000}} {
		if got := LightFactor(c.lit); got != c.want {
			t.Errorf("LightFactor(%d) = %d, want %d", c.lit, got, c.want)
		}
	}
}

func TestRollOutcome(t *testing.T) {
	t.Parallel()
	seq := func(vs ...int) func(int) int {
		i := 0
		return func(int) int { v := vs[i]; i++; return v }
	}
	if RollOutcome(500, 10, seq(499)) != Success {
		t.Fatal("success under the chance")
	}
	if RollOutcome(500, 10, seq(900, 19)) != Ridiculous || RollOutcome(500, 10, seq(900, 20, 79)) != MinorFail ||
		RollOutcome(500, 10, seq(900, 20, 80)) != Catastrophic {
		t.Fatal("the failure split at skill 10")
	}
	if RollOutcome(500, 5, seq(900, 99)) != Ridiculous || RollOutcome(500, 5, seq(900, 100, 0)) != MinorFail {
		t.Fatal("a weak surgeon fails ridiculously 10 % of the time")
	}
	r := rand.New(rand.NewSource(3))
	var cnt [4]int
	for i := 0; i < 50000; i++ {
		cnt[RollOutcome(0, 12, r.Intn)]++
	}
	// 2 % ridiculous, the rest minor 80 : 20 catastrophic
	if cnt[Success] != 0 || cnt[Ridiculous] < 800 || cnt[Ridiculous] > 1200 || cnt[MinorFail] < 37200 || cnt[MinorFail] > 41200 {
		t.Fatalf("%v", cnt)
	}
}

func TestFailDeathAndMedicineNeeds(t *testing.T) {
	t.Parallel()
	if FailDeathChance(Operation{InstallImplant, Heart, item.PumpHeart}) != 250 ||
		FailDeathChance(Operation{InstallOrgan, Heart, item.Heart}) != 250 ||
		FailDeathChance(Operation{Kind: ExciseCarcinoma, Part: Stomach}) != 250 ||
		FailDeathChance(Operation{InstallOrgan, KidneyL, item.Kidney}) != 0 ||
		FailDeathChance(Operation{Kind: Amputate, Part: ArmL}) != 0 {
		t.Fatal("heart and carcinoma operations kill 25 % of failures")
	}
	for _, c := range []struct {
		k     OpKind
		qty   int
		least item.Kind
	}{
		{InstallImplant, 2, item.HerbalMedicine}, {RemoveImplant, 2, item.HerbalMedicine}, {HarvestOrgan, 2, item.HerbalMedicine},
		{Amputate, 2, item.HerbalMedicine}, {Anesthetize, 1, item.HerbalMedicine}, {Euthanize, 1, item.HerbalMedicine},
		{ExciseCarcinoma, 4, item.IndustrialMedicine},
	} {
		if q, l := MedicineNeed(c.k); q != c.qty || l != c.least {
			t.Errorf("MedicineNeed(%d) = %d %s", c.k, q, l)
		}
	}
	if MinSkill(ExciseCarcinoma) != 10 || MinSkill(HarvestOrgan) != 0 {
		t.Fatal("excision needs Medicine 10")
	}
}

// TestNewOperationsCanOperate: the surgery v2 kinds keep their saved
// numbers and check the body.
func TestNewOperationsCanOperate(t *testing.T) {
	t.Parallel()
	if NumOps != ExciseCarcinoma+1 || HarvestOrgan != Amputate+1 {
		t.Fatal("operation kinds are append only")
	}
	var b Body
	for p := Part(0); p < NumParts; p++ {
		harvest := b.CanOperate(Operation{Kind: HarvestOrgan, Part: p})
		if harvest != (OrganOf(p) != 0) {
			t.Errorf("harvest %s: %v", p.Label(), harvest)
		}
		if b.CanOperate(Operation{InstallOrgan, p, OrganOf(p)}) {
			t.Errorf("no transplant into a whole, healthy %s", p.Label())
		}
		if b.CanOperate(Operation{Kind: ExciseCarcinoma, Part: p}) {
			t.Errorf("no carcinoma to excise on the %s", p.Label())
		}
		if (p == Torso) != b.CanOperate(Operation{Kind: Euthanize, Part: p}) || (p == Torso) != b.CanOperate(Operation{Kind: Anesthetize, Part: p}) {
			t.Errorf("whole-body operations go on the torso, not the %s", p.Label())
		}
	}
	if b.CanOperate(Operation{InstallOrgan, KidneyL, item.Lung}) {
		t.Fatal("a lung does not fit a kidney")
	}
	b.Wounds = append(b.Wounds, Wound{Part: KidneyL, Dmg: 10, Kind: Cut})
	if !b.CanOperate(Operation{InstallOrgan, KidneyL, item.Kidney}) {
		t.Fatal("a damaged kidney can be replaced")
	}
	b.SetImplant(Heart, item.PumpHeart)
	b.Missing |= bit(Heart)
	if b.CanOperate(Operation{Kind: HarvestOrgan, Part: Heart}) || !b.CanOperate(Operation{InstallOrgan, Heart, item.Heart}) {
		t.Fatal("an artificial heart is not harvested but can give way to a natural one")
	}
}

func TestApplyHarvestAndTransplant(t *testing.T) {
	t.Parallel()
	var b Body
	if !b.Apply(Operation{Kind: HarvestOrgan, Part: KidneyL}, Success, fixed(0), 1) || !b.isMissing(KidneyL) {
		t.Fatal("a harvest takes the kidney")
	}
	if b.OrganFailure() || b.ImmunityGain() != 750 {
		t.Fatal("one kidney left: alive, immunity slowed")
	}
	if !b.Apply(Operation{InstallOrgan, KidneyL, item.Kidney}, Success, fixed(0), 2) || b.isMissing(KidneyL) || b.ImmunityGain() != 1000 {
		t.Fatalf("the transplant restores the kidney: %+v", b)
	}
	b.SetImplant(Heart, item.ValveHeart)
	b.Missing |= bit(Heart)
	if !b.Apply(Operation{InstallOrgan, Heart, item.Heart}, Success, fixed(0), 3) || b.isMissing(Heart) || b.Implant(Heart) != 0 {
		t.Fatal("a natural heart replaces the valve heart")
	}
	if !b.Apply(Operation{Kind: HarvestOrgan, Part: Heart}, Success, fixed(0), 4) || !b.VitalLost() {
		t.Fatal("harvesting the heart kills")
	}
	var c Body
	if !c.Apply(Operation{Kind: Euthanize, Part: Torso}, Success, fixed(0), 1) || !c.Apply(Operation{Kind: Anesthetize, Part: Torso}, Success, fixed(0), 1) ||
		len(c.Wounds) != 0 || c.Missing != 0 {
		t.Fatal("euthanasia and anaesthesia leave the body to the game")
	}
}

func TestFailedOperations(t *testing.T) {
	t.Parallel()
	var b Body
	brace := Operation{InstallImplant, Spine, item.SpinalBrace}
	if b.Apply(brace, MinorFail, fixed(3), 1) || !b.isMissing(Spine) || b.fitted(Spine) {
		t.Fatal("a failed spinal brace destroys the spine")
	}
	var c Body
	c.Apply(Operation{Kind: Amputate, Part: ArmL}, Ridiculous, fixed(1), 1)
	if len(c.Wounds) < 2 {
		t.Fatalf("a ridiculous failure wounds several parts: %+v", c.Wounds)
	}
	for _, w := range c.Wounds {
		if w.Kind != Surgical || w.Size != int32(w.Dmg) {
			t.Fatalf("surgical wounds: %+v", w)
		}
	}
}

func TestAnestheticStages(t *testing.T) {
	t.Parallel()
	const h = uint64(clock.TicksPerHour)
	var b Body
	b.Anesthetize(100)
	if !b.Sedated() || b.Capacity(Consciousness) != 10 {
		t.Fatalf("sedated: consciousness %d", b.Capacity(Consciousness))
	}
	baseMove := (&Body{}).Capacity(Moving)
	b.StepAnesthetic(100 + 6*h - 1)
	if !b.Sedated() {
		t.Fatal("sedated for 6 hours")
	}
	b.StepAnesthetic(100 + 6*h)
	if b.Sedated() || b.Capacity(Consciousness) != 700 || b.Capacity(Moving) >= baseMove || b.Capacity(Manipulation) > 800 {
		t.Fatalf("woozy: consciousness %d, moving %d, manipulation %d", b.Capacity(Consciousness), b.Capacity(Moving), b.Capacity(Manipulation))
	}
	woozyManip := b.Capacity(Manipulation)
	b.StepAnesthetic(100 + 12*h)
	if b.Capacity(Consciousness) != 900 || b.Capacity(Manipulation) <= woozyManip {
		t.Fatalf("wearing off: consciousness %d", b.Capacity(Consciousness))
	}
	if !b.StepAnesthetic(100+20*h) || b.Anesthetized() || b.Capacity(Consciousness) != 1000 || b.Conds != nil {
		t.Fatalf("gone after 20 hours: %+v", b.Conds)
	}
	// no flap: stepped every 100 ticks the stage only ever goes down
	var c Body
	c.Anesthetize(0)
	changes, prev := 0, uint8(anesthStageSedated)
	for now := uint64(0); now <= 21*h; now += 100 {
		c.StepAnesthetic(now)
		st := uint8(255)
		if x, ok := c.Cond(CondAnesthetic); ok {
			st = x.Stage
			if DefOfCond(CondAnesthetic).StageAt(x.Sev) != st {
				t.Fatalf("severity %d out of its stage %d band", x.Sev, st)
			}
		}
		if st != prev {
			if st != 255 && st > prev {
				t.Fatalf("the stage went back up at %d", now)
			}
			changes++
			prev = st
		}
	}
	if changes != 3 {
		t.Fatalf("sedated, woozy, wearing off, gone: %d changes", changes)
	}
	c.Anesthetize(5)
	c.Anesthetize(9) // a second operation starts it over
	if x, _ := c.Cond(CondAnesthetic); len(c.Conds) != 1 || x.Since != 9 {
		t.Fatalf("one anaesthetic at a time: %+v", c.Conds)
	}
}
