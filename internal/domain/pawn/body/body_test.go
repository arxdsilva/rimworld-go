package body

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func fixed(v int) func(int) int { return func(n int) int { return v % n } }

func TestCoverageSumsTo1000(t *testing.T) {
	s := 0
	for p := Part(0); p < NumParts; p++ {
		s += DefOf(p).Coverage
	}
	if s != 1000 {
		t.Fatalf("coverage %d", s)
	}
	if DefOf(NumParts).Label != "" || Torso.Label() != "torso" {
		t.Fatal("defs")
	}
}

func TestHitDistribution(t *testing.T) {
	var b Body
	r := rand.New(rand.NewSource(1))
	hits := [NumParts]int{}
	for i := 0; i < 100000; i++ {
		p, _ := b.PickPart(r.Intn)
		hits[p]++
	}
	if hits[Torso] < hits[LegL] || hits[LegL] < hits[EarL] || hits[EarL] == 0 {
		t.Fatalf("distribution %v", hits)
	}
	if hits[Torso] < 36000 || hits[Torso] > 42000 {
		t.Fatalf("torso %d", hits[Torso])
	}
	b.Missing = ^uint32(0)
	if _, ok := b.PickPart(r.Intn); ok {
		t.Fatal("pick from nothing")
	}
	b.Missing = 0
	if p, _ := b.PickPart(func(int) int { return -5 }); p != Brain {
		t.Fatal("negative roll")
	}
	if p, _ := b.PickPart(func(n int) int { return n + 50 }); p != Torso {
		t.Fatal("overflow roll falls back")
	}
	b.destroy(LegL)
	for i := 0; i < 2000; i++ {
		if p, _ := b.PickPart(r.Intn); p == LegL || p == FootL {
			t.Fatal("hit missing part")
		}
	}
}

func TestHurtWoundAndNoop(t *testing.T) {
	var b Body
	p := b.Hurt(fixed(0), 5, Cut, 7) // brain
	if p != Brain || len(b.Wounds) != 1 || b.Wounds[0].At != 7 || b.Damage() != 5 {
		t.Fatalf("%v %+v", p, b.Wounds)
	}
	b.Hurt(fixed(0), 0, Cut, 1)
	if len(b.Wounds) != 1 {
		t.Fatal("zero damage wounded")
	}
	b.hurtPart(Brain, -1, Cut, 0)
	b.Missing = ^uint32(0)
	b.Hurt(fixed(0), 5, Cut, 1)
}

func TestDestroyCascade(t *testing.T) {
	var b Body
	b.Hurt(fixed(rollFor(HandL)), 5, Cut, 1)
	b.Implants[HandL] = item.WoodenHand
	b.Hurt(fixed(rollFor(ArmL)), 35, Cut, 2)
	if !b.isMissing(ArmL) || !b.isMissing(HandL) || len(b.Wounds) != 0 || b.Implants[HandL] != 0 {
		t.Fatalf("cascade %b %v", b.Missing, b.Wounds)
	}
	if b.Damage() != 35 {
		t.Fatalf("damage %d (hand counted once under arm)", b.Damage())
	}
	if b.VitalLost() {
		t.Fatal("arm is not vital")
	}
}

// rollFor returns a roll that picks part p on a healthy body.
func rollFor(p Part) int {
	n := 0
	for q := Part(0); q < p; q++ {
		n += DefOf(q).Coverage
	}
	return n
}

func TestAccumulatedDestroy(t *testing.T) {
	var b Body
	for i := 0; i < 3; i++ {
		b.Hurt(fixed(rollFor(FootL)), 8, Bite, uint64(i))
	}
	if !b.isMissing(FootL) {
		t.Fatal("foot should be gone at 20 accumulated")
	}
}

func TestVitalDestroyedKills(t *testing.T) {
	for _, p := range []Part{Brain, Heart, Torso} {
		var b Body
		b.Hurt(fixed(rollFor(p)), 100, Cut, 1)
		if !b.VitalLost() || b.Damage() < KillDamage {
			t.Fatalf("%v damage %d", p, b.Damage())
		}
	}
}

func TestDamageMapping(t *testing.T) {
	var b Body
	b.Hurt(fixed(rollFor(Torso)), 30, Bruise, 1)
	b.Hurt(fixed(rollFor(LegL)), 40, Cut, 1) // leg lost -> 40
	if b.Damage() != 70 {
		t.Fatalf("damage %d", b.Damage())
	}
	b.Implants[LegL] = item.PegLeg
	if b.Damage() != 30 {
		t.Fatalf("replaced leg should not count: %d", b.Damage())
	}
}

func TestHealSmallestFirstAndScar(t *testing.T) {
	var b Body
	b.Wounds = []Wound{{Part: Torso, Dmg: 10, Kind: Bruise}, {Part: ArmL, Dmg: 3, Kind: Cut}}
	if b.Heal(fixed(500), 2) || b.Wounds[1].Dmg != 1 || b.Wounds[0].Dmg != 10 {
		t.Fatalf("%+v", b.Wounds)
	}
	if !b.Heal(fixed(50), 1) { // cut closes, roll 50 < 100: scar
		t.Fatal("expected scar")
	}
	if !b.Wounds[1].Permanent || b.Wounds[1].Dmg != scarDmg {
		t.Fatalf("%+v", b.Wounds)
	}
	if b.Heal(fixed(0), 100) || len(b.Wounds) != 1 || !b.Wounds[0].Permanent {
		t.Fatalf("bruise heals clean, scar stays: %+v", b.Wounds)
	}
	if b.Heal(fixed(0), 5) {
		t.Fatal("nothing left")
	}
	b.Wounds = []Wound{{Part: ArmL, Dmg: 3, Kind: Burn, Tended: true}}
	if b.Heal(fixed(500), 3) || len(b.Wounds) != 0 {
		t.Fatal("no scar on high roll")
	}
}

func TestScarRate(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	scars := 0
	const n = 20000
	for i := 0; i < n; i++ {
		b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 1, Kind: Cut}}}
		if b.Heal(r.Intn, 1) {
			scars++
		}
	}
	if scars < n*8/100 || scars > n*12/100 {
		t.Fatalf("scar rate %d/%d", scars, n)
	}
}

func TestTendAndPain(t *testing.T) {
	b := Body{Wounds: []Wound{{Part: Torso, Dmg: 10, Kind: Cut}, {Part: Torso, Dmg: 5, Kind: Cut, Permanent: true}}}
	if b.Pain() != 85 {
		t.Fatalf("pain %d", b.Pain())
	}
	if b.Tend() != 1 || b.Tend() != 0 || b.Pain() != 45 {
		t.Fatalf("tended pain %d", b.Pain())
	}
	b.Wounds = []Wound{{Part: Torso, Dmg: 500, Kind: Cut}}
	if b.Pain() != 1000 || b.Capacity(Consciousness) != 500 {
		t.Fatal("pain cap")
	}
}

func TestCapacities(t *testing.T) {
	var b Body
	for c := Capacity(0); c <= NumCaps; c++ {
		if b.Capacity(c) != 1000 {
			t.Fatalf("healthy %d", c)
		}
	}
	b.destroy(EyeL)
	if b.Capacity(Sight) != 500 {
		t.Fatal("one eye")
	}
	b.destroy(LegL)
	if b.Capacity(Moving) != 500 {
		t.Fatalf("one leg %d", b.Capacity(Moving))
	}
	b.Implants[LegL] = item.ProstheticLeg
	if b.Capacity(Moving) != (850+1000)/2 {
		t.Fatalf("prosthetic leg %d", b.Capacity(Moving))
	}
	b.destroy(ArmR)
	b.Implants[HandL] = 0
	if b.Capacity(Manipulation) != 500 {
		t.Fatal("one arm")
	}
	b.destroy(ArmL)
	if b.Capacity(Manipulation) != 0 {
		t.Fatal("no arms")
	}
	b.destroy(HandL)
	b.Implants[HandL] = item.WoodenHand
	b.destroy(LungL)
	if b.Capacity(Breathing) != 500 {
		t.Fatal("lung")
	}
	b.destroy(EarL)
	b.Implants[EarL] = item.HearingTrumpet
	if b.Capacity(Hearing) != 800 {
		t.Fatalf("trumpet %d", b.Capacity(Hearing))
	}
}

func TestWoodenHandOnlyHand(t *testing.T) {
	var b Body
	b.destroy(HandL)
	b.Implants[HandL] = item.WoodenHand
	if b.Capacity(Manipulation) != (500+1000)/2 {
		t.Fatalf("%d", b.Capacity(Manipulation))
	}
}

func TestJawAndDenture(t *testing.T) {
	var b Body
	b.destroy(Jaw)
	if b.Capacity(Talking) != 0 || !b.Disfigured() {
		t.Fatal("no jaw")
	}
	if !b.Install(Jaw, item.Denture) || b.Capacity(Talking) != 800 {
		t.Fatalf("denture %d", b.Capacity(Talking))
	}
	b.Wounds = []Wound{{Part: Brain, Dmg: 24, Kind: Cut}}
	if b.Capacity(Talking) > 100 {
		t.Fatal("talking needs consciousness")
	}
}

func TestConsciousnessPain(t *testing.T) {
	b := Body{Wounds: []Wound{{Part: Torso, Dmg: 25, Kind: Cut}}} // pain 200
	if b.Capacity(Consciousness) != 900 || b.Capacity(Moving) != 900 || b.Capacity(Manipulation) != 900 {
		t.Fatalf("%d", b.Capacity(Consciousness))
	}
}

func TestDisfigured(t *testing.T) {
	var b Body
	if b.Disfigured() {
		t.Fatal("healthy")
	}
	b.Wounds = []Wound{{Part: ArmL, Dmg: 1, Permanent: true}}
	if b.Disfigured() {
		t.Fatal("arm scar")
	}
	b.Wounds = append(b.Wounds, Wound{Part: Nose, Dmg: 1, Permanent: true})
	if !b.Disfigured() {
		t.Fatal("nose scar")
	}
	b = Body{}
	b.destroy(EarR)
	if !b.Disfigured() {
		t.Fatal("missing ear")
	}
}

func TestImplants(t *testing.T) {
	var b Body
	if b.Install(LegL, item.PegLeg) {
		t.Fatal("Install needs a missing part")
	}
	b.destroy(LegL)
	if b.Install(LegL, item.WoodenHand) || b.Install(NumParts, item.PegLeg) {
		t.Fatal("wrong implant")
	}
	if !b.Install(LegL, item.PegLeg) || b.Install(LegL, item.ProstheticLeg) || b.Install(FootL, item.PegLeg) {
		t.Fatal("install rules")
	}
	if b.ArtificialParts() != 1 || b.Capacity(Moving) != (600+1000)/2 {
		t.Fatalf("%d %d", b.ArtificialParts(), b.Capacity(Moving))
	}
	if k, ok := b.Remove(LegL); !ok || k != item.PegLeg || b.ArtificialParts() != 0 {
		t.Fatal("remove")
	}
	if _, ok := b.Remove(LegL); ok {
		t.Fatal("double remove")
	}
	if _, ok := b.Remove(NumParts); ok {
		t.Fatal("bad part")
	}
	if ImplantEfficiency(item.Food) != 0 || ImplantEfficiency(item.ProstheticArm) != 850 {
		t.Fatal("eff")
	}
}

func TestSurgeryChanceCurve(t *testing.T) {
	h := item.HerbalMedicine
	if c := SurgeryChance(0, false, h, 1000, 1000); c != 400 {
		t.Fatalf("0: %d", c)
	}
	if c := SurgeryChance(8, false, h, 1000, 1000); c != 850 {
		t.Fatalf("8: %d", c)
	}
	if c := SurgeryChance(20, false, h, 1000, 1000); c != 980 {
		t.Fatalf("20: %d", c)
	}
	if c := SurgeryChance(99, false, h, 1000, 1000); c != 980 {
		t.Fatal("clamp skill")
	}
	if c := SurgeryChance(-3, false, h, 1000, 1000); c != 400 {
		t.Fatal("neg skill")
	}
	if c := SurgeryChance(0, false, item.Food, 1000, 1000); c != 280 {
		t.Fatalf("no medicine %d", c)
	}
	if c := SurgeryChance(0, true, item.IndustrialMedicine, 1000, 1000); c != 484 {
		t.Fatalf("bed+industrial %d", c)
	}
	if c := SurgeryChance(8, true, item.IndustrialMedicine, 1000, 1000); c != MaxSurgeryChance {
		t.Fatalf("cap %d", c)
	}
	if c := SurgeryChance(0, false, h, 500, 800); c != 200 {
		t.Fatalf("min(manip,sight) %d", c)
	}
	if c := SurgeryChance(0, false, h, -5, 2000); c != 0 {
		t.Fatal("clamped negative")
	}
}

func TestRollOutcome(t *testing.T) {
	if RollOutcome(500, fixed(499)) != Success {
		t.Fatal("success")
	}
	seq := func(vs ...int) func(int) int {
		i := 0
		return func(int) int { v := vs[i]; i++; return v }
	}
	if RollOutcome(500, seq(900, 79)) != MinorFail || RollOutcome(500, seq(900, 80)) != Catastrophic {
		t.Fatal("split")
	}
	r := rand.New(rand.NewSource(3))
	var cnt [3]int
	for i := 0; i < 50000; i++ {
		cnt[RollOutcome(0, r.Intn)]++
	}
	if cnt[Success] != 0 || cnt[MinorFail] < 38000 || cnt[MinorFail] > 42000 {
		t.Fatalf("%v", cnt)
	}
}

func TestApply(t *testing.T) {
	var b Body
	install := Operation{InstallImplant, LegL, item.PegLeg}
	if b.Apply(Operation{InstallImplant, LegL, item.WoodenHand}, Success, fixed(0), 1) {
		t.Fatal("wrong kind")
	}
	if !b.Apply(Operation{Kind: Amputate, Part: LegL}, Success, fixed(0), 1) || !b.isMissing(FootL) {
		t.Fatal("amputate")
	}
	if b.Apply(Operation{Kind: Amputate, Part: Heart}, Success, fixed(0), 1) || b.Apply(Operation{Kind: 9, Part: Nose}, Success, fixed(0), 1) || b.Apply(Operation{Part: NumParts}, Success, fixed(0), 1) {
		t.Fatal("invalid ops")
	}
	// minor failure wounds the nearest present ancestor, 5..15
	b.Apply(install, MinorFail, fixed(10), 5)
	if len(b.Wounds) != 1 || b.Wounds[0].Part != Torso || b.Wounds[0].Dmg != 15 || b.Wounds[0].Kind != Surgical {
		t.Fatalf("%+v", b.Wounds)
	}
	if !b.Apply(install, Success, fixed(0), 6) || b.Implants[LegL] != item.PegLeg {
		t.Fatal("install")
	}
	if !b.Apply(Operation{Kind: RemoveImplant, Part: LegL}, Success, fixed(0), 7) || b.ArtificialParts() != 0 {
		t.Fatal("remove")
	}
	// catastrophic on a small part destroys it
	n := Operation{Kind: Amputate, Part: EarL}
	b.Apply(n, Catastrophic, fixed(20), 8) // 40 dmg >= 10
	if !b.isMissing(EarL) {
		t.Fatal("catastrophic destroys ear")
	}
	// minor failure never destroys
	c := Body{}
	for i := 0; i < 5; i++ {
		c.Apply(Operation{Kind: Amputate, Part: EyeL}, MinorFail, fixed(10), 1)
	}
	if c.isMissing(EyeL) || c.partDmg(EyeL) > DefOf(EyeL).MaxHP-1 {
		t.Fatal("minor must not destroy")
	}
	c.Apply(Operation{Kind: Amputate, Part: EyeL}, MinorFail, fixed(10), 1)
}

func TestPartsAndGob(t *testing.T) {
	b := Body{Wounds: []Wound{{Part: Torso, Dmg: 10, Kind: Cut, Tended: true}, {Part: Torso, Dmg: 1, Permanent: true}, {Part: ArmL, Dmg: 99}}}
	b.destroy(LegL)
	b.Implants[LegL] = item.PegLeg
	ps := b.Parts()
	if len(ps) != int(NumParts) || ps[Torso].HP != 49 || ps[Torso].Wounds != 1 || ps[Torso].Scars != 1 || !ps[Torso].Tended {
		t.Fatalf("%+v", ps[Torso])
	}
	if !ps[LegL].Missing || !ps[LegL].Fitted || ps[LegL].Implant != item.PegLeg || ps[ArmL].HP != 0 {
		t.Fatal("status")
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&b); err != nil {
		t.Fatal(err)
	}
	var got Body
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil || got.Missing != b.Missing || got.Implants != b.Implants || len(got.Wounds) != 3 {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestReplaceHealthyLeg(t *testing.T) {
	var b Body
	b.Wounds = append(b.Wounds, Wound{Part: FootL, Dmg: 3}, Wound{Part: LegL, Dmg: 1, Permanent: true}, Wound{Part: LegR, Dmg: 2})
	op := Operation{InstallImplant, LegL, item.ProstheticLeg}
	if !b.CanOperate(op) {
		t.Fatal("healthy leg replaceable")
	}
	if !b.Apply(op, Success, fixed(0), 1) {
		t.Fatal("apply")
	}
	if !b.isMissing(LegL) || !b.isMissing(FootL) || b.Implants[LegL] != item.ProstheticLeg || b.ArtificialParts() != 1 {
		t.Fatalf("%b %v", b.Missing, b.Implants)
	}
	if len(b.Wounds) != 1 || b.Wounds[0].Part != LegR {
		t.Fatalf("wounds %+v", b.Wounds)
	}
	if got := b.Capacity(Moving); got != (850+950)/2 {
		t.Fatalf("moving %d", got)
	}
	if b.CanOperate(Operation{InstallImplant, FootL, item.ProstheticFoot}) || b.CanOperate(op) {
		t.Fatal("covered/occupied")
	}
	if b.MissingUncovered() != 0 {
		t.Fatal("prosthetic leg covers leg and foot")
	}
}

func TestReplaceFailureKeepsPart(t *testing.T) {
	for _, o := range []Outcome{MinorFail, Catastrophic} {
		var b Body
		op := Operation{InstallImplant, LegL, item.PegLeg}
		if b.Apply(op, o, fixed(0), 1) {
			t.Fatal("failure must not install")
		}
		if b.Implants[LegL] != 0 || len(b.Wounds) != 1 || b.Wounds[0].Kind != Surgical {
			t.Fatalf("%v %+v", o, b.Wounds)
		}
		if o == MinorFail && b.isMissing(LegL) {
			t.Fatal("leg kept")
		}
	}
}

func TestInstallBlockedByMissingAncestor(t *testing.T) {
	var b Body
	b.destroy(ArmL)
	if b.CanOperate(Operation{InstallImplant, HandL, item.WoodenHand}) {
		t.Fatal("hand under missing arm")
	}
	if !b.CanOperate(Operation{InstallImplant, ArmL, item.ProstheticArm}) {
		t.Fatal("arm ok")
	}
	if b.CanOperate(Operation{InstallImplant, HandR, item.ProstheticArm}) || b.CanReplace(NumParts) || b.CanReplace(Heart) {
		t.Fatal("bad targets")
	}
	if got := b.InstallOptions(item.WoodenHand); len(got) != 1 || got[0] != HandR {
		t.Fatalf("%v", got)
	}
	if got := b.InstallOptions(item.ProstheticArm); len(got) != 2 || got[0] != ArmL || got[1] != ArmR {
		t.Fatalf("%v", got)
	}
}

func TestProstheticFoot(t *testing.T) {
	var b Body
	if !b.Apply(Operation{InstallImplant, FootL, item.ProstheticFoot}, Success, fixed(0), 1) {
		t.Fatal("foot")
	}
	if got := b.Capacity(Moving); got != (750+1000)/2 {
		t.Fatalf("moving %d", got)
	}
	if Fits(item.ProstheticFoot, LegL) || ImplantEfficiency(item.ProstheticFoot) != 750 {
		t.Fatal("fit/eff")
	}
}

func TestCosmeticImplants(t *testing.T) {
	var b Body
	if !b.Apply(Operation{InstallImplant, EyeL, item.GlassEye}, Success, fixed(0), 1) {
		t.Fatal("eye")
	}
	if b.ArtificialParts() != 1 || b.Capacity(Sight) != 500 || b.MissingUncovered() != 0 {
		t.Fatalf("%d %d %d", b.ArtificialParts(), b.Capacity(Sight), b.MissingUncovered())
	}
	if !b.Disfigured() {
		t.Fatal("missing face part still disfigures")
	}
	if !b.Apply(Operation{InstallImplant, Nose, item.WoodenNose}, Success, fixed(0), 1) || b.ArtificialParts() != 2 {
		t.Fatal("nose")
	}
	if Fits(item.WoodenNose, EyeL) || Fits(item.GlassEye, Nose) {
		t.Fatal("fits")
	}
	b.destroy(EarL)
	if b.MissingUncovered() != 1 {
		t.Fatal("bare missing ear")
	}
}
