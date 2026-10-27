package body

import (
	"fmt"
	"hash/fnv"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// humanFixtures are humanlike bodies in a spread of states: whole,
// wounded on many parts, with parts lost, fresh stumps, implants, blood
// loss and conditions.
func humanFixtures() []Body {
	var out []Body
	out = append(out, Body{})
	for p := Part(0); p < NumParts; p++ {
		out = append(out, Body{Wounds: []Wound{{Part: p, Dmg: 7, Kind: Cut}, {Part: p, Dmg: 3, Kind: Bite, Tended: true, TendQ: 400}}})
		var m Body
		m.destroy(p)
		out = append(out, m)
	}
	out = append(out,
		Body{Missing: bit(LegL) | bit(FootL), Fresh: bit(LegL)},
		Body{Missing: bit(LegR) | bit(FootR), Fitted: FittedFrom([]item.Kind{LegR: item.PegLeg})},
		Body{Missing: bit(Heart), Fitted: FittedFrom([]item.Kind{Heart: item.PumpHeart}), BloodLoss: 350, BloodStage: 2},
		Body{Wounds: []Wound{{Part: Torso, Dmg: 40, Kind: Bruise}, {Part: Spine, Dmg: 20, Kind: Cut, Permanent: true}}, Missing: bit(KidneyL)},
		Body{Wounds: []Wound{{Part: Heart, Dmg: 12, Kind: Cut}}, Conds: []Condition{{Kind: CondInfection, Part: Heart, Sev: 500}}},
	)
	return out
}

// humanFingerprint is everything the humanlike body reads and reports,
// as text: the part table, then for each fixture its capacities, pain,
// damage, bleeding, part rows, capacity factors, which losses kill and
// where a spread of rolls land.
func humanFingerprint() string {
	var sb strings.Builder
	for p := Part(0); p < NumParts; p++ {
		fmt.Fprintf(&sb, "%d %+v\n", p, DefOf(p))
	}
	for i, b := range humanFixtures() {
		fmt.Fprintf(&sb, "#%d", i)
		for c := Capacity(0); c < NumCaps; c++ {
			fmt.Fprintf(&sb, " %d", b.Capacity(c))
		}
		fmt.Fprintf(&sb, " pain %d dmg %d bleed %d vital %v disfig %v miss %d imm %d\n",
			b.Pain(), b.Damage(), b.BleedPerDay(), b.VitalLost(), b.Disfigured(), b.MissingUncovered(), b.ImmunityGain())
		fmt.Fprintf(&sb, "%+v\n", b.Parts())
		for c := Capacity(0); c < NumCaps; c++ {
			fmt.Fprintf(&sb, "%+v", b.CapacityFactors(c))
		}
		for p := Part(0); p < NumParts; p++ {
			fmt.Fprintf(&sb, " %v/%d", b.LossKills(p), b.PartBleedPerDay(p))
		}
		for r := 0; r < 1000; r += 37 {
			p, ok := b.PickPart(func(int) int { return r })
			fmt.Fprintf(&sb, " %d%v", p, ok)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// humanFingerprintHash was pinned before MED-13 split the anatomy into
// per-plan tables: the humanlike body must read exactly as it did.
const humanFingerprintHash = 0x3bdbc554b1763654

// MED-13: the humanlike tables and capacities are byte-identical to
// before the per-anatomy split.
func TestHumanlikeUnchanged(t *testing.T) {
	h := fnv.New64a()
	h.Write([]byte(humanFingerprint()))
	if got := h.Sum64(); got != humanFingerprintHash {
		t.Fatalf("humanlike fingerprint %#x, want %#x", got, humanFingerprintHash)
	}
}

// MED-13: every plan fits the bit sets, covers exactly 1000, and is one
// tree under its root; its brain and heart are vital, its labels unique,
// and its capacity rules name its own parts.
func TestEachAnatomyCoverageSums(t *testing.T) {
	for a := Anatomy(0); a < NumAnatomies; a++ {
		t.Run(a.String(), func(t *testing.T) {
			ds := DefsOf(a)
			if len(ds) == 0 || len(ds) > 32 {
				t.Fatalf("%d parts: Missing is a uint32", len(ds))
			}
			n := Part(len(ds))
			root := RootOf(a)
			sum := 0
			seen := map[string]bool{}
			for p, d := range ds {
				sum += d.Coverage
				if d.Label == "" || seen[d.Label] || d.MaxHP <= 0 || d.Coverage <= 0 {
					t.Fatalf("part %d: %+v", p, d)
				}
				seen[d.Label] = true
				if d.Parent >= n || (d.Parent == Part(p)) != (Part(p) == root) {
					t.Fatalf("part %d %q: parent %d, root %d", p, d.Label, d.Parent, root)
				}
				b := Body{Anatomy: a}
				if !b.under(Part(p), root) {
					t.Fatalf("part %d %q is not under the root", p, d.Label)
				}
			}
			if sum != 1000 {
				t.Fatalf("coverage sums to %d", sum)
			}
			if !ds[root].Vital {
				t.Fatal("the root is vital")
			}
			if a == Humanlike {
				return
			}
			pl := planOf(a)
			if !ds[pl.brain].Vital || !ds[pl.heart].Vital || ds[pl.brain].Cap != Consciousness || ds[pl.heart].Cap != BloodPumping {
				t.Fatalf("brain %+v heart %+v", ds[pl.brain], ds[pl.heart])
			}
			if pl.caps[Moving].empty() {
				t.Fatal("every animal moves on something")
			}
			for c, r := range pl.caps { // chains serve the capacity; a multiplier (the liver) may serve another
				for _, ch := range r.chains {
					for _, p := range ch {
						if p >= n || !ds[p].HasCap || ds[p].Cap != Capacity(c) {
							t.Fatalf("capacity %d reads part %d (%+v)", c, p, ds[p])
						}
					}
				}
				for _, p := range r.mul {
					if p >= n || !ds[p].HasCap {
						t.Fatalf("capacity %d multiplies by part %d (%+v)", c, p, ds[p])
					}
				}
			}
			// a whole body is at full capacity everywhere
			b := Body{Anatomy: a, Scale: 550}
			for c := Capacity(0); c < NumCaps; c++ {
				if v := b.Capacity(c); v != 1000 {
					t.Fatalf("whole %v: capacity %d = %d", a, c, v)
				}
			}
			if b.OrganFailure() || b.VitalLost() || b.Damage() != 0 || b.Pain() != 0 {
				t.Fatal("a whole body is well")
			}
		})
	}
}

// pickRoll is the roll that lands PickPart on part p of b (b whole).
func pickRoll(t *testing.T, b *Body, p Part) func(int) int {
	t.Helper()
	r := 0
	for q := Part(0); q < p; q++ {
		r += b.table()[q].Coverage
	}
	roll := func(int) int { return r }
	if got, _ := b.PickPart(roll); got != p {
		t.Fatalf("roll %d picks %d, want %d", r, got, p)
	}
	return roll
}

// MED-13: a hit can land on a bird's wing; the wing carries nothing a
// bird walks with, so losing it costs no moving (a leg costs half).
func TestBirdWingHit(t *testing.T) {
	b := Body{Anatomy: Bird, Scale: 280}
	wingHP := b.PartDef(BWingL).MaxHP
	if wingHP != 7 { // 25 at a man's scale, ×0.28
		t.Fatalf("wing hit points %d", wingHP)
	}
	if p := b.Hurt(pickRoll(t, &b, BWingL), 3, Bite, 1); p != BWingL || len(b.Wounds) != 1 || b.Wounds[0].Part != BWingL {
		t.Fatalf("the bite lands on the wing: %v %+v", p, b.Wounds)
	}
	torso := Body{Anatomy: Bird, Scale: 280, Wounds: []Wound{{Part: BTorso, Dmg: 3, Kind: Bite}}}
	if b.Capacity(Moving) != torso.Capacity(Moving) || b.Pain() != torso.Pain() {
		t.Fatalf("a wing wound weighs on moving only through its pain: %d vs %d", b.Capacity(Moving), torso.Capacity(Moving))
	}
	if rows := b.Parts(); rows[BWingL].Label != "left wing" || rows[BWingL].HP != wingHP-3 || rows[BWingL].Wounds != 1 {
		t.Fatalf("wing row %+v", rows[BWingL])
	}
	b.Hurt(pickRoll(t, &Body{Anatomy: Bird}, BWingL), wingHP, Bite, 2) // takes it off
	if !b.PartMissing(BWingL) || b.FreshStump(BWingL) == false || b.PartBleedPerDay(BWingL) <= 0 {
		t.Fatalf("the wing is torn off and its stump bleeds: %+v", b)
	}
	b.TendAll(500, 3)
	if b.Capacity(Moving) != 1000 {
		t.Fatalf("a bird with one wing still walks at %d", b.Capacity(Moving))
	}
	b.destroy(BLegL)
	if !b.PartMissing(BFootL) || b.Capacity(Moving) != 500 {
		t.Fatalf("one leg: foot lost %v, moving %d", b.PartMissing(BFootL), b.Capacity(Moving))
	}
}

// MED-13: part hit points scale with the body; pain reads damage
// against that scale, so the same share of a body hurts the same.
func TestAnatomyScale(t *testing.T) {
	small := Body{Anatomy: Quadruped, Scale: 250}
	if hp := small.PartDef(QTorso).MaxHP; hp != 13 { // 50 × 0.25 = 12.5, rounded
		t.Fatalf("torso %d", hp)
	}
	if hp := small.PartDef(QEyeL).MaxHP; hp != 3 {
		t.Fatalf("eye %d", hp)
	}
	if hp := (&Body{Anatomy: Quadruped, Scale: 10}).PartDef(QEyeL).MaxHP; hp != 1 {
		t.Fatalf("a part keeps at least 1 hit point: %d", hp)
	}
	small.Wounds = []Wound{{Part: QTorso, Dmg: 5, Kind: Bruise}}
	man := Body{Wounds: []Wound{{Part: Torso, Dmg: 20, Kind: Bruise}}}
	if small.Pain() != man.Pain() {
		t.Fatalf("a quarter-size body hurts from 5 as a man from 20: %d vs %d", small.Pain(), man.Pain())
	}
	if rows := small.Parts(); len(rows) != int(numQuadruped) || rows[QTorso].MaxHP != 13 || rows[QTorso].HP != 8 {
		t.Fatalf("rows %+v", rows[QTorso])
	}
}

// MED-13: a part with a vital part under it counts as vital (the head
// over the brain); a snake's only lung and its spine are never taken off.
func TestAnimalLossKills(t *testing.T) {
	q := Body{Anatomy: Quadruped, Scale: 1000}
	if !q.LossKills(QHead) || !q.LossKills(QBrain) || !q.LossKills(QTorso) || q.LossKills(QLegFL) || q.LossKills(QLungL) {
		t.Fatal("quadruped: head, brain and torso kill; a leg or one lung does not")
	}
	q.destroy(QLungL)
	if !q.LossKills(QLungR) {
		t.Fatal("the last lung kills")
	}
	s := Body{Anatomy: Snake, Scale: 300}
	if !s.LossKills(SLung) || !s.LossKills(SSpine) || s.LossKills(STail) {
		t.Fatal("snake: the lung and spine kill, the tail does not")
	}
	s.Wounds = []Wound{{Part: SSpine, Dmg: s.PartDef(SSpine).MaxHP / 2, Kind: Cut}}
	if v := s.Capacity(Moving); v >= 1000 || v <= 0 {
		t.Fatalf("a hurt spine slows the snake: %d", v)
	}
	i := Body{Anatomy: Insect}
	i.destroy(ILegFL)
	if v := i.Capacity(Moving); v != 833 {
		t.Fatalf("an insect on five legs: %d", v)
	}
	if i.Capacity(Breathing) != 1000 || i.Capacity(BloodFiltration) != 1000 {
		t.Fatal("no lungs or kidneys: those stay full")
	}
}

// MED-13: implants, organs, surgery and aging are the humanlike body's:
// nothing can be fitted on or done to an animal plan.
func TestAnimalBodyRefusesSurgery(t *testing.T) {
	for a := Quadruped; a < NumAnatomies; a++ {
		b := Body{Anatomy: a, Scale: 500}
		b.destroy(planOf(a).caps[Moving].parts()[0])
		for p := Part(0); p < b.numParts(); p++ {
			if b.CanReplace(p) || b.Install(p, item.PegLeg) {
				t.Fatalf("%v: part %d takes an implant", a, p)
			}
			for k := OpKind(0); k < NumOps; k++ {
				if b.CanOperate(Operation{Kind: k, Part: p, Item: item.PegLeg}) {
					t.Fatalf("%v: operation %d on part %d", a, k, p)
				}
			}
		}
		if len(b.InstallOptions(item.PegLeg)) != 0 || b.CanDevelop(CondCataract, EyeL) {
			t.Fatalf("%v: install options or aging", a)
		}
	}
}
