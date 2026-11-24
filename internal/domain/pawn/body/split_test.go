// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"math/bits"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/combat/damage"
)

// splitRec answers draws from vals in order (0 once they run out) and
// records each draw's bound.
type splitRec struct {
	vals []int
	ns   []int
}

func (r *splitRec) roll(n int) int {
	r.ns = append(r.ns, n)
	if len(r.vals) == 0 {
		return 0
	}
	v := r.vals[0]
	r.vals = r.vals[1:]
	return v
}

// splitTap records each bound in ns and answers from src.
func splitTap(src func(int) int, ns *[]int) func(int) int {
	return func(n int) int {
		*ns = append(*ns, n)
		return src(n)
	}
}

func splitNever(int) int { panic("no draw expected") }

// splitPCG is a seeded draw source.
func splitPCG(seed uint64) func(int) int {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)).IntN
}

// splitWounded is the set of parts holding a wound.
func splitWounded(b *Body) uint32 {
	var m uint32
	for _, w := range b.Wounds {
		m |= bit(w.Part)
	}
	return m
}

// splitDmgs is each part's wound damage, for the parts that hold one.
func splitDmgs(b *Body) map[Part]int {
	out := map[Part]int{}
	for _, w := range b.Wounds {
		out[w.Part] += w.Dmg
	}
	return out
}

// splitBigQuad is a quadruped at 10x a human's scale: every part's hit
// points and killAt are x10, so no share of a W1 test blow is capped.
func splitBigQuad() Body { return Body{Anatomy: Quadruped, Scale: 10000} }

// TestSplitOf: the cut, blunt, scratch, stab, bullet, poke and ranged stab forms each split
// by their own rule; bite and arrow (the wiki gives them no multi-part
// rule), every other form and an unknown one do not split.
func TestSplitOf(t *testing.T) {
	t.Parallel()
	for f := damage.Form(0); f <= damage.NumForms+1; f++ {
		want := SplitNone
		switch f {
		case damage.FormCut:
			want = SplitCut
		case damage.FormBlunt:
			want = SplitBlunt
		case damage.FormScratch:
			want = SplitScratch
		case damage.FormStab:
			want = SplitStab
		case damage.FormBullet:
			want = SplitBullet
		case damage.FormPoke:
			want = SplitPoke
		case damage.FormRangedStab:
			want = SplitRangedStab
		}
		if got := SplitOf(f); got != want {
			t.Errorf("SplitOf(%v) = %d, want %d", f, got, want)
		}
	}
	if SplitOf(damage.FormBite) != SplitNone || SplitOf(damage.FormArrow) != SplitNone {
		t.Error("bite and arrow wound one part")
	}
}

// TestSplitsPredicate: splits holds for exactly the seven rules, not for no
// split, an unknown value or the sentinel.
func TestSplitsPredicate(t *testing.T) {
	t.Parallel()
	for s := Split(0); s < 16; s++ {
		want := s == SplitCut || s == SplitBlunt || s == SplitScratch || s == SplitStab || s == SplitBullet ||
			s == SplitPoke || s == SplitRangedStab
		if got := splits(s); got != want {
			t.Errorf("splits(%d) = %v, want %v", s, got, want)
		}
	}
	if splits(SplitNone) || splits(Split(9)) || splits(numSplits) {
		t.Error("none, Split(9) or numSplits splits")
	}
}

// TestSplitPctOf: pct % of dmg rounded down, at least 1 for any damage,
// nothing for none.
func TestSplitPctOf(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ dmg, pct, want int }{
		{0, 67, 0}, {-4, 67, 0}, {1, 67, 1}, {53, 67, 35}, {100, 40, 40}, {100, 75, 75},
		{2, 40, 1}, {10, 67, 6}, {100, 67, 67},
	} {
		if got := pctOf(c.dmg, c.pct); got != c.want {
			t.Errorf("pctOf(%d, %d) = %d, want %d", c.dmg, c.pct, got, c.want)
		}
	}
}

// TestSplitBigQuad: the fixture's part hit points and killAt are 10x a
// quadruped's.
func TestSplitBigQuad(t *testing.T) {
	t.Parallel()
	b, q := splitBigQuad(), Body{Anatomy: Quadruped}
	if killAt(&b) != 10*KillDamage {
		t.Fatalf("killAt %d", killAt(&b))
	}
	for p := Part(0); p < b.numParts(); p++ {
		if b.maxHP(p) != 10*q.maxHP(p) {
			t.Fatalf("%v: maxHP %d, a quadruped's %d", p, b.maxHP(p), q.maxHP(p))
		}
	}
}

// TestSplitPlansInBounds: every split's plan on every part of every
// anatomy deals 1..maxShares shares, each on a part of the plan and none
// negative, with at least 1 damage in all (a cut of 1 may leave a share
// at 0).
func TestSplitPlansInBounds(t *testing.T) {
	t.Parallel()
	roll := splitPCG(11)
	for a := Anatomy(0); a < NumAnatomies; a++ {
		b := Body{Anatomy: a}
		for p := Part(0); p < b.numParts(); p++ {
			for s := SplitCut; s < numSplits; s++ {
				for _, dmg := range []int{1, 2, 53, 100} {
					sh := planSplit(&b, p, dmg, s, roll)
					if sh.n < 1 || sh.n > maxShares {
						t.Fatalf("%v %v split %d dmg %d: %d shares", a, p, s, dmg, sh.n)
					}
					total := 0
					for _, x := range sh.s[:sh.n] {
						if x.part >= b.numParts() || x.dmg < 0 {
							t.Fatalf("%v %v split %d dmg %d: share %+v", a, p, s, dmg, x)
						}
						total += x.dmg
					}
					if total < 1 {
						t.Fatalf("%v %v split %d dmg %d: deals nothing", a, p, s, dmg)
					}
				}
			}
		}
	}
}

// TestBombWoundsOnePart: a bomb's shrapnel cuts (WoundOf) but does not
// split: StrikeBlow wounds one part and draws as Strike, SpreadBlow draws
// as Spread.
func TestBombWoundsOnePart(t *testing.T) {
	t.Parallel()
	kind, s := WoundOf(damage.FormBomb), SplitOf(damage.FormBomb)
	if kind != Cut || s != SplitNone {
		t.Fatalf("bomb: kind %v split %d", kind, s)
	}
	lo, hi := damage.FormBomb.Overkill()
	for _, p := range []Part{Torso, ArmL, Heart, LegL} {
		var a, b Body
		var na, nb []int
		ga := Strike(&a, p, 50, kind, lo, hi, splitTap(splitPCG(7), &na), 1)
		gb := StrikeBlow(&b, p, 50, kind, s, lo, hi, splitTap(splitPCG(7), &nb), 1)
		if ga != gb || !reflect.DeepEqual(a, b) || !slices.Equal(na, nb) {
			t.Fatalf("%v: StrikeBlow %v %v %v, Strike %v %v %v", p, gb, b.Wounds, nb, ga, a.Wounds, na)
		}
		if n := len(b.Wounds); n > 1 {
			t.Fatalf("%v: %d wounds", p, n)
		}
	}
	var a, b Body
	var na, nb []int
	Spread(&a, 30, kind, splitTap(splitPCG(9), &na), 1)
	if got := SpreadBlow(&b, 30, kind, s, splitTap(splitPCG(9), &nb), 1); got != 30 {
		t.Fatalf("SpreadBlow returned %d", got)
	}
	if !reflect.DeepEqual(a, b) || !slices.Equal(na, nb) {
		t.Fatalf("SpreadBlow %v %v, Spread %v %v", b.Wounds, nb, a.Wounds, na)
	}
}

// TestStrikeBlowDrawFree: no split is Strike exactly; a blunt hit with no
// inner part under it, an invalid or missing part and no damage draw
// nothing.
func TestStrikeBlowDrawFree(t *testing.T) {
	t.Parallel()
	for _, kind := range []WoundKind{Cut, Bruise, Gunshot, Burn} {
		for _, p := range []Part{Torso, Brain, Heart, ArmL, FootR} {
			for _, dmg := range []int{5, 30, 70} {
				var a, b Body
				var na, nb []int
				ga := Strike(&a, p, dmg, kind, 0, 700, splitTap(splitPCG(3), &na), 1)
				gb := StrikeBlow(&b, p, dmg, kind, SplitNone, 0, 700, splitTap(splitPCG(3), &nb), 1)
				if ga != gb || !reflect.DeepEqual(a, b) || !slices.Equal(na, nb) {
					t.Fatalf("%v %v %d: differs from Strike", kind, p, dmg)
				}
			}
		}
	}
	for _, p := range []Part{ArmL, HandL, Brain} {
		var b Body
		StrikeBlow(&b, p, 10, Bruise, SplitBlunt, 400, 1000, splitNever, 1)
		if got := splitDmgs(&b); len(got) != 1 || got[p] != 10 {
			t.Fatalf("blunt on %v: %v", p, got)
		}
	}
	b := Body{Missing: bit(HandL)}
	for _, s := range []Split{SplitNone, SplitCut, SplitBlunt} {
		if StrikeBlow(&b, HandL, 10, Cut, s, 0, 100, splitNever, 1) ||
			StrikeBlow(&b, Part(200), 10, Cut, s, 0, 100, splitNever, 1) ||
			StrikeBlow(&b, Torso, 0, Cut, s, 0, 100, splitNever, 1) ||
			StrikeBlow(&b, Torso, -3, Cut, s, 0, 100, splitNever, 1) {
			t.Fatal("reported destroyed")
		}
	}
	if len(b.Wounds) != 0 {
		t.Fatalf("wounded: %v", b.Wounds)
	}
	if sh := planSplit(&b, ArmL, 5, SplitNone, splitNever); sh.n != 1 || sh.s[0] != (share{ArmL, 5}) {
		t.Fatalf("no split plans %v", sh.s[:sh.n])
	}
}

// TestSplitDrawOrder: a cut draws its count then one pick per extra; a
// blunt hit draws its gate, the inner part, keep and extra; then only
// overkill draws come.
func TestSplitDrawOrder(t *testing.T) {
	t.Parallel()
	var b Body
	r := &splitRec{vals: []int{75, 0, 0}} // k = 3: Brain (40), then Heart
	StrikeBlow(&b, Torso, 10, Cut, SplitCut, 0, 100, r.roll, 1)
	if want := []int{100, 514, 514 - 40}; !slices.Equal(r.ns, want) {
		t.Fatalf("cut draws %v, want %v", r.ns, want)
	}
	if got := splitWounded(&b); got != bit(Torso)|bit(Brain)|bit(Heart) {
		t.Fatalf("cut wounded %b", got)
	}
	var c Body
	r = &splitRec{}
	StrikeBlow(&c, Torso, 10, Bruise, SplitBlunt, 400, 1000, r.roll, 1)
	if want := []int{100, 169, 11, 16}; !slices.Equal(r.ns, want) {
		t.Fatalf("blunt draws %v, want %v", r.ns, want)
	}
	// a cut whose first share overkills its part draws the window last
	var d Body
	r = &splitRec{vals: []int{30, 0}}
	d.Wounds = append(d.Wounds, Wound{Part: ArmL, Dmg: 30, Kind: Cut})
	StrikeBlow(&d, ArmL, 10, Cut, SplitCut, 0, 700, r.roll, 1) // 7 on 5 left: overkill 57
	if want := []int{100, 321, 1000}; !slices.Equal(r.ns, want) {
		t.Fatalf("cut then overkill draws %v, want %v", r.ns, want)
	}
}

// TestSplitShowsPerPart: the HUD read model shows a split blow's wounds on
// each part it struck.
func TestSplitShowsPerPart(t *testing.T) {
	t.Parallel()
	var b Body
	r := &splitRec{vals: []int{30, 0}} // k = 2: ArmL, then Torso
	StrikeBlow(&b, ArmL, 10, Cut, SplitCut, 0, 100, r.roll, 1)
	for _, s := range b.Parts() {
		want := 0
		if s.Part == ArmL || s.Part == Torso {
			want = 1
		}
		if s.Wounds != want {
			t.Errorf("%v: %d wounds, want %d", s.Part, s.Wounds, want)
		}
	}
}

// splitDeadOracle is the game's death rule on a body: a pawn's
// (humanlike) at Damage 100 or a vital part lost; an animal's when its
// wounds and lost parts reach its hit points (100 scaled by Scale), a lost
// vital part or an organ failure aside.
func splitDeadOracle(b *Body) bool {
	if b.Anatomy == Humanlike {
		return b.Damage() >= KillDamage || b.VitalLost()
	}
	hp := KillDamage
	if b.Scale > 0 {
		hp = KillDamage * int(b.Scale) / 1000
	}
	n := 0
	for _, w := range b.Wounds {
		n += w.Dmg
	}
	for p := Part(0); p < b.numParts(); p++ {
		if par := b.table()[p].Parent; b.isMissing(p) && (par == p || !b.isMissing(par)) {
			n += b.maxHP(p)
		}
	}
	return n >= hp
}

// splitReplayBlow deals a placed split blow share by share as StrikeBlow
// does (the caller checks the bodies match), failing when an extra share
// dealt while the body lives destroys a part whose loss would kill, or
// any part but its own and what hangs under it.
func splitReplayBlow(t *testing.T, b *Body, p Part, dmg int, kind WoundKind, s Split, lo, hi int, roll func(int) int, now uint64) {
	t.Helper()
	sh := planSplit(b, p, dmg, s, roll)
	Strike(b, sh.s[0].part, sh.s[0].dmg, kind, lo, hi, roll, now)
	for _, x := range sh.s[1:sh.n] {
		if splitDeadOracle(b) {
			return // dead: no further share is dealt
		}
		kills, before := b.LossKills(x.part), b.Missing
		dealPlaced(b, x.part, x.dmg, kind, lo, hi, roll, now)
		for lost := b.Missing &^ before; lost != 0; lost &= lost - 1 {
			q := Part(bits.TrailingZeros32(lost))
			if kills || !b.under(q, x.part) {
				t.Fatalf("an extra share on %v (LossKills %v) destroyed %v", x.part, kills, q)
			}
		}
	}
}

// TestStrikeBlowVitalAndDeath: on the placed path no extra share dealt
// while the body lives destroys a part whose loss would kill (a part lost
// under a destroyed part that was not such a part aside); once a share
// kills (at its Scale's hit points), no further share is dealt; and death
// comes at Damage. Each blow is replayed share by share on a copy and must
// leave the same body.
func TestStrikeBlowVitalAndDeath(t *testing.T) {
	t.Parallel()
	plans := []Body{{}, {}, {Anatomy: Quadruped}, {Anatomy: Quadruped, Scale: 1700}, {Anatomy: Bird},
		{Anatomy: Snake}, {Anatomy: Lizard, Scale: 350}, {Anatomy: Insect}}
	for seed := uint64(1); seed <= 5000; seed++ {
		pcg := rand.NewPCG(seed, seed*31+7)
		roll := rand.New(pcg).IntN
		b := plans[seed%uint64(len(plans))]
		for blow := 0; !splitDeadOracle(&b); blow++ {
			if blow == 100 {
				t.Fatalf("seed %d: alive after %d blows (Damage %d)", seed, blow, b.Damage())
			}
			s, kind, form := SplitCut, Cut, damage.FormCut
			if blow%2 == 1 {
				s, kind, form = SplitBlunt, Bruise, damage.FormBlunt
			}
			lo, hi := form.Overkill()
			p, ok := b.PickPart(roll)
			if !ok {
				// an animal plan whose root a first share destroyed: by the
				// animal rule only its root's hit points count, so it lives
				// with nothing left to strike (the game never strikes an
				// animal placed: SpreadBlow keeps the root)
				if b.Anatomy == Humanlike || !b.isMissing(b.Root()) {
					t.Fatalf("seed %d blow %d: nothing to pick on a living body", seed, blow)
				}
				break
			}
			probe := *pcg // the same draws for the replay
			r := Body{Wounds: slices.Clone(b.Wounds), Missing: b.Missing, Fresh: b.Fresh, Anatomy: b.Anatomy, Scale: b.Scale}
			splitReplayBlow(t, &r, p, 40, kind, s, lo, hi, rand.New(&probe).IntN, uint64(blow))
			StrikeBlow(&b, p, 40, kind, s, lo, hi, roll, uint64(blow))
			if !slices.Equal(b.Wounds, r.Wounds) || b.Missing != r.Missing || b.Fresh != r.Fresh {
				t.Fatalf("seed %d blow %d: StrikeBlow %v %b, replay %v %b", seed, blow, b.Wounds, b.Missing, r.Wounds, r.Missing)
			}
		}
	}
	// a first share that kills stops the blow: the brain is lost, the
	// torso's share is not dealt
	dead := Body{Wounds: []Wound{{Part: Brain, Dmg: 20, Kind: Cut}}}
	if !StrikeBlow(&dead, Brain, 40, Cut, SplitCut, 0, 100, (&splitRec{vals: []int{30, 80}}).roll, 1) {
		t.Fatal("brain not destroyed")
	}
	if got := splitDmgs(&dead); len(got) != 0 || dead.Damage() < KillDamage {
		t.Fatalf("dealt after death: %v", got)
	}
	// scripted: a 40 cut on Torso with k = 2 is 56, 28 each; a vital
	// extra is left at MaxHP - 1
	for _, c := range []struct {
		pick  int
		extra Part
	}{{40, Heart}, {0, Brain}} {
		var b Body
		r := &splitRec{vals: []int{30, c.pick}}
		StrikeBlow(&b, Torso, 40, Cut, SplitCut, 0, 100, r.roll, 1)
		got := splitDmgs(&b)
		if got[Torso] != 28 || got[c.extra] != 24 || b.isMissing(c.extra) || len(got) != 2 {
			t.Fatalf("extra %v: %v", c.extra, got)
		}
	}
}

// TestStrikeBlowEdges: a vital part at MaxHP - 1 takes no further share;
// a part lost to the first share takes nothing; dmg 1 at k = 4 wounds
// only the first part.
func TestStrikeBlowEdges(t *testing.T) {
	t.Parallel()
	b := Body{Wounds: []Wound{{Part: Heart, Dmg: 24, Kind: Cut}}}
	StrikeBlow(&b, Torso, 20, Cut, SplitCut, 0, 100, (&splitRec{vals: []int{30, 40}}).roll, 1)
	if got := splitDmgs(&b); got[Heart] != 24 || got[Torso] != 14 || b.isMissing(Heart) {
		t.Fatalf("heart at MaxHP-1: %v", got)
	}
	// ArmL at 30 of 35 takes 14: destroyed with HandL, so HandL's share is lost
	c := Body{Wounds: []Wound{{Part: ArmL, Dmg: 30, Kind: Cut}}}
	if !StrikeBlow(&c, ArmL, 20, Cut, SplitCut, 0, 100, (&splitRec{vals: []int{30, 296}}).roll, 1) {
		t.Fatal("arm not reported destroyed")
	}
	if !c.isMissing(ArmL) || !c.isMissing(HandL) || len(c.Wounds) != 0 {
		t.Fatalf("lost arm: %v missing %b", c.Wounds, c.Missing)
	}
	var d Body
	StrikeBlow(&d, Torso, 1, Cut, SplitCut, 0, 100, (&splitRec{vals: []int{95, 0, 0, 0}}).roll, 1)
	if got := splitDmgs(&d); len(got) != 1 || got[Torso] != 2 {
		t.Fatalf("dmg 1 at k = 4: %v", got)
	}
}

// TestStrikeBlowAllocs: a split blow on a body without implants allocates
// nothing beyond its wound appends (measured 0 with room for them).
func TestStrikeBlowAllocs(t *testing.T) { // not parallel: AllocsPerRun
	buf := make([]Wound, 0, 64)
	var b Body
	var vals []int
	roll := func(int) int {
		if len(vals) == 0 {
			return 0
		}
		v := vals[0]
		vals = vals[1:]
		return v
	}
	cutVals, bluntVals := []int{95, 0, 0, 0}, []int{0, 0, 0, 0}
	for _, c := range []struct {
		s      Split
		vals   []int
		dmg    int
		lo, hi int
	}{{SplitCut, cutVals, 60, 0, 100}, {SplitBlunt, bluntVals, 100, 400, 1000}} {
		n := testing.AllocsPerRun(200, func() {
			b = Body{Wounds: buf[:0]}
			vals = c.vals
			StrikeBlow(&b, Torso, c.dmg, Cut, c.s, c.lo, c.hi, roll, 1)
		})
		if n != 0 || len(b.Wounds) < 2 {
			t.Fatalf("split %d: %v allocs, %d wounds", c.s, n, len(b.Wounds))
		}
	}
}

// TestStrikeBlowCutDistribution: a cut wounds 1, 2, 3 or 4 parts 30, 45,
// 20 and 5 % of the time (wiki: Damage Types).
func TestStrikeBlowCutDistribution(t *testing.T) {
	t.Parallel()
	roll := splitPCG(2026)
	var count [5]int
	const n = 10000
	for range n {
		var b Body
		StrikeBlow(&b, Torso, 10, Cut, SplitCut, 0, 100, roll, 1)
		m := splitWounded(&b)
		k := 0
		for ; m != 0; m &= m - 1 {
			k++
		}
		count[k]++
	}
	for k, want := range []int{0, 30, 45, 20, 5} {
		if got := count[k] * 100; got < (want-2)*n || got > (want+2)*n {
			t.Errorf("%d parts: %d of %d, want %d %%", k, count[k], n, want)
		}
	}
}

// TestStrikeBlowCutDeterministic: the same seed gives the same body.
func TestStrikeBlowCutDeterministic(t *testing.T) {
	t.Parallel()
	run := func() Body {
		roll := splitPCG(77)
		var b Body
		for i := range 10 {
			s := SplitCut
			if i%3 == 0 {
				s = SplitBlunt
			}
			p, _ := b.PickPart(roll)
			StrikeBlow(&b, p, 6, Cut, s, 0, 100, roll, uint64(i))
		}
		return b
	}
	a, b := run(), run()
	if len(a.Wounds) < 10 || !reflect.DeepEqual(a, b) {
		t.Fatalf("not repeatable: %v vs %v", a.Wounds, b.Wounds)
	}
}

// TestSplitScaledBody: a body scaled past a human's hit points lives past
// Damage 100 (MED-13: an elephant's 170), so every share of a split blow
// on it lands; it dies at its own scaled Damage or a vital part lost.
func TestSplitScaledBody(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		scale int32
		want  int
	}{{0, 100}, {1000, 100}, {1700, 170}, {350, 35}, {100, 10}} {
		if got := killAt(&Body{Anatomy: Quadruped, Scale: c.scale}); got != c.want {
			t.Errorf("killAt at Scale %d: %d, want %d", c.scale, got, c.want)
		}
	}
	if bodyDead(&Body{Anatomy: Quadruped, Scale: 1700, Wounds: []Wound{{Part: QLegFL, Dmg: 40}, {Part: QLegFR, Dmg: 40}, {Part: QTail, Dmg: 16}, {Part: QTorso, Dmg: 10}}}) {
		t.Error("an elephant dead at 106")
	}
	// the game kills a pawn on a lost vital part, never an animal: an
	// animal dies only when its wounds and lost parts reach its hit points
	if bodyDead(&Body{Anatomy: Quadruped, Scale: 1700, Missing: bit(QBrain)}) {
		t.Error("an elephant dead at 26 without its brain")
	}
	if !bodyDead(&Body{Missing: bit(Heart)}) {
		t.Error("a pawn alive without its heart")
	}
	small := Body{Anatomy: Quadruped, Scale: 350, Missing: bit(QStomach) | bit(QLiver)} // 11 + 14 lost
	if !small.OrganFailure() || small.Damage() < KillDamage || bodyDead(&small) {
		t.Errorf("a small animal in organ failure at 25 of 35: failure %v, Damage %d, dead %v", small.OrganFailure(), small.Damage(), bodyDead(&small))
	}
	// an elephant in organ failure (stomach and liver at MaxHP - 1, wounds
	// 117 of 170) takes a cut's whole planned 28: 14 on the torso (PickPart
	// 155), 14 on the head (k = 2, pick 0)
	ill := Body{Anatomy: Quadruped, Scale: 1700, Wounds: []Wound{{Part: QStomach, Dmg: 50}, {Part: QLiver, Dmg: 67}}}
	if !ill.OrganFailure() {
		t.Fatal("no organ failure")
	}
	before := ill.Damage()
	if got := SpreadBlow(&ill, 20, Cut, SplitCut, (&splitRec{vals: []int{155, 30, 0}}).roll, 1); got != 28 ||
		splitDmgs(&ill)[QTorso] != 14 || splitDmgs(&ill)[QHead] != 14 || ill.Damage()-before != 28 {
		t.Fatalf("organ failure: returned %d, wounds %v, Damage %d -> %d", got, splitDmgs(&ill), before, ill.Damage())
	}
	// placed: 110 on the legs and tail, then a cut on the torso with k = 2,
	// the head the extra: 9 and 8 (17) both land
	b := Body{Anatomy: Quadruped, Scale: 1700, Wounds: []Wound{
		{Part: QLegFL, Dmg: 25}, {Part: QLegFR, Dmg: 25}, {Part: QLegHL, Dmg: 25}, {Part: QLegHR, Dmg: 25}, {Part: QTail, Dmg: 10}}}
	StrikeBlow(&b, QTorso, 12, Cut, SplitCut, 0, 100, (&splitRec{vals: []int{30, 0}}).roll, 1)
	if got := splitDmgs(&b); got[QTorso] != 9 || got[QHead] != 8 || b.Damage() != 127 {
		t.Fatalf("placed on a scaled body: %v (Damage %d)", got, b.Damage())
	}
	// unplaced: every seeded split blow of 12 on an elephant at 110 deals
	// its whole planned total
	for seed := uint64(1); seed <= 300; seed++ {
		e := Body{Anatomy: Quadruped, Scale: 1700}
		Spread(&e, 110, Cut, splitPCG(seed), 0)
		pcg := rand.NewPCG(seed, 99)
		probe := *pcg
		s := SplitCut
		if seed%2 == 0 {
			s = SplitBlunt
		}
		pr := rand.New(&probe).IntN
		p, _ := e.PickPart(pr)
		plan := planSplit(&e, p, 12, s, pr)
		want := 0
		for _, x := range plan.s[:plan.n] {
			want += x.dmg
		}
		before := e.Damage()
		got := SpreadBlow(&e, 12, Cut, s, rand.New(pcg).IntN, 1)
		if before < KillDamage || got != want || e.Damage()-before != want {
			t.Fatalf("seed %d split %d: Damage %d, returned %d, rose %d, want %d", seed, s, before, got, e.Damage()-before, want)
		}
	}
}
