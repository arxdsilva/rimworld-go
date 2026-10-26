package body

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// MED-05: kidneys, liver and spine; blood pumping, filtration and
// digestion; the servo tier and the efficiency fixes.

func TestNewOrgansAreWholeAndNotVital(t *testing.T) {
	t.Parallel()
	want := map[Part]struct{ hp, cov int }{KidneyL: {50, 17}, KidneyR: {50, 17}, Liver: {50, 25}, Spine: {40, 25}}
	for p, w := range want {
		d := DefOf(p)
		if d.MaxHP != w.hp || d.Coverage != w.cov || d.Vital || d.Parent != Torso || d.Label == "" {
			t.Errorf("%v: %+v", p, d)
		}
	}
	if NumParts > 32 {
		t.Fatalf("%d parts overflow the Missing bitmask", NumParts)
	}
	var b Body
	for _, c := range []Capacity{BloodPumping, BloodFiltration, Digestion} {
		if b.Capacity(c) != 1000 {
			t.Errorf("healthy capacity %d = %d", c, b.Capacity(c))
		}
	}
	if b.OrganFailure() || b.VitalLost() || b.ImmunityGain() != 1000 {
		t.Fatal("a healthy body is failing")
	}
}

func TestKidneyLossKillsWhenBoth(t *testing.T) {
	t.Parallel()
	var b Body
	b.destroy(KidneyL)
	if b.OrganFailure() || b.VitalLost() || b.Capacity(BloodFiltration) != 500 {
		t.Fatalf("one kidney: filtration %d", b.Capacity(BloodFiltration))
	}
	b.destroy(KidneyR)
	if !b.OrganFailure() || !b.VitalLost() || b.Damage() < KillDamage {
		t.Fatal("no kidneys must kill")
	}
}

func TestOneKidneyLowersImmunity(t *testing.T) {
	t.Parallel()
	var b Body
	b.destroy(KidneyR)
	if got := b.ImmunityGain(); got != 750 {
		t.Fatalf("immunity gain %d, want 750", got)
	}
	if got := b.Capacity(Consciousness); got != 950 {
		t.Fatalf("consciousness %d, want 950", got)
	}
}

func TestLiverLungsAndStomachAreLifeOrgans(t *testing.T) {
	t.Parallel()
	for _, parts := range [][]Part{{Liver}, {LungL, LungR}, {Stomach}} {
		var b Body
		for _, p := range parts {
			b.destroy(p)
		}
		if !b.OrganFailure() {
			t.Errorf("losing %v must kill", parts)
		}
	}
	var b Body
	b.destroy(LungL)
	if b.OrganFailure() {
		t.Fatal("one lung is enough")
	}
	b = Body{}
	b.destroy(Stomach)
	if !b.Install(Stomach, item.SynthStomach) || b.OrganFailure() || !b.GutWormImmune() {
		t.Fatal("a synth stomach stands in and keeps gut worms out")
	}
	if b.Capacity(Digestion) != ServoEff {
		t.Fatalf("digestion %d", b.Capacity(Digestion))
	}
}

func TestLiverDamageLowersDigestionAndFiltration(t *testing.T) {
	t.Parallel()
	var b Body
	b.Wounds = []Wound{{Part: Liver, Dmg: 25, Kind: Bruise}}
	if b.Capacity(Digestion) != 500 || b.Capacity(BloodFiltration) != 500 {
		t.Fatalf("half a liver: digestion %d filtration %d", b.Capacity(Digestion), b.Capacity(BloodFiltration))
	}
}

func TestSpineLossStopsMoving(t *testing.T) {
	t.Parallel()
	var b Body
	b.Wounds = []Wound{{Part: Spine, Dmg: 20, Kind: Bruise}}
	if got, want := b.Capacity(Moving), b.Capacity(Consciousness)*500/1000; got != want {
		t.Fatalf("half a spine: moving %d, want %d", got, want)
	}
	b.Wounds = nil
	b.destroy(Spine)
	if b.Capacity(Moving) != 0 || b.OrganFailure() {
		t.Fatalf("no spine: moving %d (and alive)", b.Capacity(Moving))
	}
	if !b.Install(Spine, item.SpinalBrace) || b.Capacity(Moving) != 1000 || !b.SpineBraced() {
		t.Fatalf("spinal brace: moving %d", b.Capacity(Moving))
	}
}

func TestBloodPumpingWeighsOnConsciousnessMovingAndBleed(t *testing.T) {
	t.Parallel()
	var b Body
	b.Wounds = []Wound{{Part: Heart, Dmg: 10, Kind: Bruise}} // 600 pumping, pain 125
	if b.Capacity(BloodPumping) != 600 {
		t.Fatalf("pumping %d", b.Capacity(BloodPumping))
	}
	// consciousness: 1000 - 125/2 = 938, × (1 - 0.2 × 0.4) = 862
	if got := b.Capacity(Consciousness); got != 862 {
		t.Fatalf("consciousness %d", got)
	}
	// moving: min(1000, 862) × 0.92 = 793
	if got := b.Capacity(Moving); got != 793 {
		t.Fatalf("moving %d", got)
	}
	b.Wounds = []Wound{{Part: Heart, Dmg: 10, Kind: Cut}}
	full := BleedRate(b.Wounds[0])
	if got := b.BleedPerDay(); got != full*600/1000 || b.PartBleedPerDay(Heart) != got {
		t.Fatalf("bleed %d, want %d", got, full*600/1000)
	}
}

func TestArtificialHeartKeepsPatientAlive(t *testing.T) {
	t.Parallel()
	for _, k := range []item.Kind{item.PumpHeart, item.ValveHeart} {
		var b Body
		if !b.CanOperate(Operation{InstallImplant, Heart, k}) {
			t.Fatalf("%v cannot replace the heart", k)
		}
		if !b.Apply(Operation{InstallImplant, Heart, k}, Success, func(int) int { return 0 }, 1) {
			t.Fatalf("%v install failed", k)
		}
		if b.VitalLost() || b.Damage() != 0 || !b.HeartAttackImmune() {
			t.Fatalf("%v: vital lost %v, damage %d", k, b.VitalLost(), b.Damage())
		}
		if b.Capacity(BloodPumping) != ImplantEfficiency(k) {
			t.Fatalf("%v pumping %d", k, b.Capacity(BloodPumping))
		}
	}
	var b Body
	if b.HeartAttackImmune() || b.GutWormImmune() || b.SpineBraced() {
		t.Fatal("natural organs give no immunity")
	}
	if b.CanOperate(Operation{Amputate, Heart, 0}) {
		t.Fatal("the heart is never amputated")
	}
}

func TestServoArmAbove100(t *testing.T) {
	t.Parallel()
	var b Body
	b.destroy(ArmL)
	if !b.Install(ArmL, item.ServoArm) {
		t.Fatal("servo arm")
	}
	if got := b.Capacity(Manipulation); got != (1250+1000)/2 {
		t.Fatalf("manipulation %d, want 1125", got)
	}
	b.destroy(ArmR)
	b.Install(ArmR, item.ServoArm)
	if got := b.Capacity(Manipulation); got != 1250 {
		t.Fatalf("two servo arms %d", got)
	}
	// Above 1000 the bonus scales with consciousness; at or under it the
	// old min rule holds.
	if limbs(1250, 500) != 625 || limbs(800, 500) != 500 || limbs(800, 1000) != 800 {
		t.Fatal("limbs rule")
	}
	var c Body
	c.destroy(LegL)
	c.destroy(LegR)
	c.Install(LegL, item.ServoLeg)
	c.Install(LegR, item.ServoLeg)
	if c.Capacity(Moving) != 1250 {
		t.Fatalf("servo legs moving %d", c.Capacity(Moving))
	}
	c.destroy(EyeL)
	c.Install(EyeL, item.OpticImplant)
	c.destroy(EarL)
	c.Install(EarL, item.CochlearImplant)
	if c.Capacity(Sight) != 1125 || c.Capacity(Hearing) != 1125 {
		t.Fatalf("optic %d cochlear %d", c.Capacity(Sight), c.Capacity(Hearing))
	}
}

func TestProstheticArmEfficiencyFixed(t *testing.T) {
	t.Parallel()
	want := map[item.Kind]int32{item.ProstheticArm: 500, item.WoodenHand: 600, item.PegLeg: 600, item.ProstheticLeg: 850,
		item.ServoArm: 1250, item.ServoLeg: 1250, item.OpticImplant: 1250, item.CochlearImplant: 1250,
		item.SpinalBrace: 1000, item.PumpHeart: 1250, item.SynthStomach: 1250, item.ValveHeart: 800}
	for k, w := range want {
		if got := ImplantEfficiency(k); got != w {
			t.Errorf("%v efficiency %d, want %d", k, got, w)
		}
	}
	var b Body
	b.destroy(ArmL)
	b.Install(ArmL, item.ProstheticArm)
	if got := b.Capacity(Manipulation); got != (500+1000)/2 {
		t.Fatalf("prosthetic arm manipulation %d", got)
	}
}

func TestEveryImplantFitsSomewhere(t *testing.T) {
	t.Parallel()
	fits := map[item.Kind][]Part{item.ServoArm: {ArmL, ArmR}, item.ServoLeg: {LegL, LegR}, item.OpticImplant: {EyeL, EyeR},
		item.CochlearImplant: {EarL, EarR}, item.SpinalBrace: {Spine}, item.PumpHeart: {Heart}, item.ValveHeart: {Heart},
		item.SynthStomach: {Stomach}}
	for k := item.Kind(1); k != 0; k++ {
		if !k.IsImplant() {
			continue
		}
		var on []Part
		for p := Part(0); p < NumParts; p++ {
			if Fits(k, p) {
				on = append(on, p)
			}
		}
		if len(on) == 0 || ImplantEfficiency(k) == 0 && k != item.GlassEye && k != item.WoodenNose {
			t.Errorf("%v fits %v with efficiency %d", k, on, ImplantEfficiency(k))
		}
		if w, ok := fits[k]; ok && (len(w) != len(on) || w[0] != on[0] || w[len(w)-1] != on[len(on)-1]) {
			t.Errorf("%v fits %v, want %v", k, on, w)
		}
	}
}

func TestOrgansMatchTheirParts(t *testing.T) {
	t.Parallel()
	want := map[Part]item.Kind{KidneyL: item.Kidney, KidneyR: item.Kidney, LungL: item.Lung, LungR: item.Lung,
		Heart: item.Heart, Liver: item.Liver}
	for p := Part(0); p < NumParts; p++ {
		if OrganOf(p) != want[p] {
			t.Errorf("%v organ %v, want %v", p, OrganOf(p), want[p])
		}
		for _, k := range []item.Kind{item.Kidney, item.Lung, item.Heart, item.Liver} {
			if Fits(k, p) {
				t.Errorf("organ %v must not fit as an implant", k)
			}
			if FitsOrgan(k, p) != (want[p] == k) {
				t.Errorf("FitsOrgan(%v, %v)", k, p)
			}
		}
	}
	if FitsOrgan(item.PumpHeart, Heart) {
		t.Fatal("an implant is not an organ")
	}
}

func TestNanomedPotency(t *testing.T) {
	t.Parallel()
	if pot, maxQ := Potency(item.NanomedKit); pot != 1600 || maxQ != 1300 {
		t.Fatalf("nanomed %d/%d", pot, maxQ)
	}
	in := TendInput{Skill: 20, Manip: 1000, Sight: 1000, Roll: func(n int) int { return n - 1 }}
	in.Medicine = item.IndustrialMedicine
	ind := TendQuality(in)
	in.Medicine = item.NanomedKit
	if nano := TendQuality(in); nano != 1300 || nano <= ind {
		t.Fatalf("nanomed tend %d vs industrial %d", nano, ind)
	}
}

func TestLossKillsGuardsLastOrgans(t *testing.T) {
	t.Parallel()
	var b Body
	for _, p := range []Part{Brain, Torso, Heart, Liver, Stomach} {
		if !b.LossKills(p) {
			t.Errorf("losing %v kills", p)
		}
	}
	for _, p := range []Part{KidneyL, LungR, Spine, ArmL, EyeL} {
		if b.LossKills(p) {
			t.Errorf("losing %v alone does not kill", p)
		}
	}
	b.destroy(KidneyL)
	b.destroy(LungR)
	if !b.LossKills(KidneyR) || !b.LossKills(LungL) || b.LossKills(NumParts) {
		t.Fatal("the last kidney and lung are guarded")
	}
	b.destroy(Stomach)
	b.Install(Stomach, item.SynthStomach)
	if b.LossKills(LegL) || b.Implant(Stomach) != item.SynthStomach {
		t.Fatal("the check must not touch the body")
	}
}
