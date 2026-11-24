// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"math/bits"
	"slices"
	"testing"
)

// stabShares lists a plan's shares in dealing order.
func stabShares(sh shares) []share { return slices.Clone(sh.s[:sh.n]) }

// stabSum is the plan's total damage.
func stabSum(sh shares) int {
	t := 0
	for _, x := range sh.s[:sh.n] {
		t += x.dmg
	}
	return t
}

// TestStabHumanlikeGateHit: a torso stab whose gate roll is 59 (< 60)
// picks an inside child; Heart takes 40 %, the torso 75 %. The gate is
// rolled over 100, then the pick over the children's coverage.
func TestStabHumanlikeGateHit(t *testing.T) {
	t.Parallel()
	b := Body{}
	for _, c := range []struct {
		dmg  int
		want []share
	}{
		{100, []share{{Heart, 40}, {Torso, 75}}},
		{200, []share{{Heart, 80}, {Torso, 150}}},
	} {
		r := &splitRec{vals: []int{59, 0}}
		sh := planStab(&b, Torso, c.dmg, stabRules[SplitStab], r.roll)
		if got := stabShares(sh); !slices.Equal(got, c.want) {
			t.Errorf("dmg %d: %v, want %v", c.dmg, got, c.want)
		}
		if len(r.ns) != 2 || r.ns[0] != 100 || r.ns[1] <= 0 {
			t.Errorf("dmg %d: draw bounds %v", c.dmg, r.ns)
		}
	}
	if got := stabSum(planStab(&b, Torso, 100, stabRules[SplitStab], (&splitRec{vals: []int{59, 0}}).roll)); got != 115 {
		t.Errorf("total %d, want 115", got)
	}
}

// TestStabGateMiss: a gate roll of 60 is a miss: one share of dmg on the
// torso and no pick.
func TestStabGateMiss(t *testing.T) {
	t.Parallel()
	b := Body{}
	r := &splitRec{vals: []int{60}}
	got := stabShares(planStab(&b, Torso, 100, stabRules[SplitStab], r.roll))
	if !slices.Equal(got, []share{{Torso, 100}}) || len(r.ns) != 1 || r.ns[0] != 100 {
		t.Errorf("got %v, draws %v", got, r.ns)
	}
}

// TestStabPickReachesEveryChild: sweeping the pick over the torso's
// coverage reaches each inside child (and only them).
func TestStabPickReachesEveryChild(t *testing.T) {
	t.Parallel()
	b := Body{}
	seen := map[Part]bool{}
	for r := 0; r < 400; r++ {
		sh := planStab(&b, Torso, 100, stabRules[SplitStab], (&splitRec{vals: []int{0, r}}).roll)
		if sh.n != 2 || sh.s[1].part != Torso || !inside(&b, sh.s[0].part) {
			t.Fatalf("pick %d: %v", r, stabShares(sh))
		}
		seen[sh.s[0].part] = true
	}
	if want := bits.OnesCount32(insideChildren(&b, Torso)); len(seen) != want {
		t.Errorf("reached %d children, want %d", len(seen), want)
	}
}

// TestStabNoEligibleChildDrawsNothing: an outer part with no inside child
// (an arm, a hand, the humanlike brain, which is outer) is one share of
// dmg with no draw.
func TestStabNoEligibleChildDrawsNothing(t *testing.T) {
	t.Parallel()
	b := Body{}
	for _, p := range []Part{ArmL, HandL, Brain} {
		got := stabShares(planStab(&b, p, 100, stabRules[SplitStab], splitNever))
		if !slices.Equal(got, []share{{p, 100}}) {
			t.Errorf("%v: %v", p, got)
		}
	}
	// A torso whose organs are all missing has no child either.
	m := Body{Missing: insideChildren(&b, Torso)}
	got := stabShares(planStab(&m, Torso, 100, stabRules[SplitStab], splitNever))
	if !slices.Equal(got, []share{{Torso, 100}}) {
		t.Errorf("organs missing: %v", got)
	}
}

// TestStabInsideFirstPart: a stab first landing on the spine (a bone) takes
// the chain with no draw; bones are not exempt.
func TestStabInsideFirstPart(t *testing.T) {
	t.Parallel()
	b := Body{}
	got := stabShares(planStab(&b, Spine, 100, stabRules[SplitStab], splitNever))
	if !slices.Equal(got, []share{{Spine, 40}, {Torso, 75}}) {
		t.Errorf("got %v", got)
	}
}

// TestStabNeverWoundsBrain: on every animal plan a head stab, over every
// gate roll and every pick roll, never wounds the brain (the brain is a
// candidate inside the head, so dropping the exclusion fails this), and a
// stab first landing on the brain wounds the head only.
func TestStabNeverWoundsBrain(t *testing.T) {
	t.Parallel()
	for _, a := range []Anatomy{Quadruped, Bird, Snake, Lizard, Insect} {
		b := Body{Anatomy: a}
		brain := planOf(a).brain
		head := b.table()[brain].Parent
		if !inside(&b, brain) || insideChildren(&b, head)&bit(brain) == 0 {
			t.Fatalf("%v: brain %v is not an inside child of %v", a, brain, head)
		}
		for gate := 0; gate < 100; gate++ {
			for pick := 0; pick < 100; pick++ {
				sh := planStab(&b, head, 100, stabRules[SplitStab], (&splitRec{vals: []int{gate, pick}}).roll)
				for _, x := range sh.s[:sh.n] {
					if x.part == brain {
						t.Fatalf("%v: head stab gate %d pick %d wounds the brain: %v", a, gate, pick, stabShares(sh))
					}
				}
			}
		}
		got := stabShares(planStab(&b, brain, 100, stabRules[SplitStab], splitNever))
		if len(got) != 1 || got[0].part != head || got[0].dmg != 100 {
			t.Errorf("%v: brain stab %v, want one share on the head", a, got)
		}
		if outerOf(&b, brain) != head {
			t.Errorf("%v: outer of the brain is %v, not %v", a, outerOf(&b, brain), head)
		}
	}
}

// TestStabStopRule: a retargeted Heart destroyed by its own share ends a
// humanlike blow (a lost vital part kills) before the torso share; a
// quadruped's destroyed heart does not stop the blow, and the torso share
// is dealt.
func TestStabStopRule(t *testing.T) {
	t.Parallel()
	h := Body{}
	r := &splitRec{vals: []int{59, 0}}
	StrikeBlow(&h, Torso, 100, Bruise, SplitStab, 0, 100, r.roll, 1)
	if !h.isMissing(Heart) || splitDmgs(&h)[Torso] != 0 {
		t.Errorf("humanlike: heart missing %v, torso wounded %d", h.isMissing(Heart), splitDmgs(&h)[Torso])
	}
	q := Body{Anatomy: Quadruped}
	r = &splitRec{vals: []int{59, 0}}
	StrikeBlow(&q, QTorso, 100, Bruise, SplitStab, 0, 100, r.roll, 1)
	if !q.isMissing(QHeart) || splitDmgs(&q)[QTorso] == 0 {
		t.Errorf("quadruped: heart missing %v, torso wounded %d", q.isMissing(QHeart), splitDmgs(&q)[QTorso])
	}
}

// TestStabSpreadBlowTotal: on a body no share can overflow, SpreadBlow
// returns the plan's total (and a retargeted stab shows a total above dmg).
func TestStabSpreadBlowTotal(t *testing.T) {
	t.Parallel()
	over := false
	for seed := uint64(1); seed <= 200; seed++ {
		b, probe := splitBigQuad(), splitBigQuad()
		replay := splitPCG(seed)
		p, ok := probe.PickPart(replay)
		if !ok {
			t.Fatal("no part")
		}
		want := stabSum(planStab(&probe, p, 100, stabRules[SplitStab], replay))
		got := SpreadBlow(&b, 100, Stab, SplitStab, splitPCG(seed), 1)
		if got != want {
			t.Fatalf("seed %d: SpreadBlow %d, plan %d", seed, got, want)
		}
		over = over || got > 100
	}
	if !over {
		t.Error("no seed retargeted inside")
	}
}

// TestStabChain: the pure chain rule on synthetic chains.
func TestStabChain(t *testing.T) {
	t.Parallel()
	ps := [maxShares]Part{1, 2, 3, 4, 5, 6, 7, 8}
	for _, c := range []struct {
		n, dmg int
		want   []share
	}{
		{3, 100, []share{{1, 40}, {2, 40}, {3, 75}}},
		{8, 100, []share{{1, 40}, {2, 40}, {3, 40}, {4, 40}, {5, 40}, {6, 40}, {7, 40}, {8, 75}}},
		{2, 1, []share{{1, 1}, {2, 1}}},
		{1, 77, []share{{1, 77}}},
		{0, 100, nil},
	} {
		got := stabShares(stabChain(&ps, c.n, c.dmg))
		if !slices.Equal(got, c.want) && (len(got) != 0 || len(c.want) != 0) {
			t.Errorf("n=%d dmg=%d: %v, want %v", c.n, c.dmg, got, c.want)
		}
	}
	if got := stabSum(stabChain(&ps, 8, 100)); got != 355 {
		t.Errorf("n=8 total %d, want 355", got)
	}
}

// TestStabAllocs: planning a stab allocates nothing.
func TestStabAllocs(t *testing.T) { // not parallel: AllocsPerRun
	b := Body{}
	for _, vals := range [][]int{{59, 0}, {60}} {
		r := &splitRec{}
		n := testing.AllocsPerRun(200, func() {
			r.ns = r.ns[:0]
			for _, rule := range stabRules[SplitStab : SplitRangedStab+1] {
				r.vals = vals
				planStab(&b, Torso, 100, rule, r.roll)
			}
		})
		if n != 0 {
			t.Errorf("%v: %v allocs", vals, n)
		}
	}
}

// TestPokeGateBoundary: Poke's gate is 40 %: roll 39 hits, 40 misses, and a
// Stab hit at 59 is a Poke miss.
func TestPokeGateBoundary(t *testing.T) {
	t.Parallel()
	b := Body{}
	r := &splitRec{vals: []int{39, 0}}
	sh := planStab(&b, Torso, 100, stabRules[SplitPoke], r.roll)
	if got := stabShares(sh); !slices.Equal(got, []share{{Heart, 40}, {Torso, 75}}) || stabSum(sh) != 115 {
		t.Errorf("gate 39: %v", got)
	}
	if len(r.ns) != 2 || r.ns[0] != 100 || r.ns[1] <= 0 {
		t.Errorf("gate 39 draws %v", r.ns)
	}
	r = &splitRec{vals: []int{40}}
	sh = planStab(&b, Torso, 100, stabRules[SplitPoke], r.roll)
	if got := stabShares(sh); !slices.Equal(got, []share{{Torso, 100}}) || len(r.ns) != 1 || r.ns[0] != 100 {
		t.Errorf("gate 40: %v draws %v", got, r.ns)
	}
	r = &splitRec{vals: []int{59, 0}}
	if stabSum(planStab(&b, Torso, 100, stabRules[SplitStab], r.roll)) != 115 {
		t.Error("stab at 59 misses")
	}
	r = &splitRec{vals: []int{59, 0}}
	got := stabShares(planStab(&b, Torso, 100, stabRules[SplitPoke], r.roll))
	if !slices.Equal(got, []share{{Torso, 100}}) {
		t.Errorf("poke at 59: %v", got)
	}
}

// TestPokeNeverWoundsBrain: Poke bars the brain as Stab does, on every
// animal plan, and a poke landing first on a brain takes the head, no draw.
func TestPokeNeverWoundsBrain(t *testing.T) {
	t.Parallel()
	for _, a := range []Anatomy{Quadruped, Bird, Snake, Lizard, Insect} {
		b := Body{Anatomy: a}
		brain := planOf(a).brain
		head := b.table()[brain].Parent
		for gate := 0; gate < 100; gate++ {
			for pick := 0; pick < 100; pick++ {
				sh := planStab(&b, head, 100, stabRules[SplitPoke], (&splitRec{vals: []int{gate, pick}}).roll)
				for _, x := range sh.s[:sh.n] {
					if x.part == brain {
						t.Fatalf("%v: poke gate %d pick %d wounds the brain", a, gate, pick)
					}
				}
			}
		}
		got := stabShares(planStab(&b, brain, 100, stabRules[SplitPoke], splitNever))
		if !slices.Equal(got, []share{{head, 100}}) {
			t.Errorf("%v: brain poke %v", a, got)
		}
	}
}

// TestRangedStabReachesBrain: on an animal head a ranged stab can reach the
// brain, which is the head's only inside child; Stab cannot.
func TestRangedStabReachesBrain(t *testing.T) {
	t.Parallel()
	for _, a := range []Anatomy{Quadruped, Bird, Snake, Lizard, Insect} {
		b := Body{Anatomy: a}
		brain := planOf(a).brain
		head := b.table()[brain].Parent
		if insideChildren(&b, head) != bit(brain) {
			t.Fatalf("%v: the brain is not the head's only inside child", a)
		}
		rule := stabRules[SplitRangedStab]
		r := &splitRec{vals: []int{59, 0}}
		sh := planStab(&b, head, 100, rule, r.roll)
		if got := stabShares(sh); !slices.Equal(got, []share{{brain, 40}, {head, 75}}) || stabSum(sh) != 115 {
			t.Errorf("%v: %v", a, got)
		}
		if len(r.ns) != 2 || r.ns[0] != 100 || r.ns[1] != b.table()[brain].Coverage {
			t.Errorf("%v: draws %v, coverage %d", a, r.ns, b.table()[brain].Coverage)
		}
		r = &splitRec{vals: []int{60}}
		got := stabShares(planStab(&b, head, 100, rule, r.roll))
		if !slices.Equal(got, []share{{head, 100}}) || len(r.ns) != 1 {
			t.Errorf("%v: gate 60 %v draws %v", a, got, r.ns)
		}
		r = &splitRec{vals: []int{59, 0}}
		got = stabShares(planStab(&b, head, 100, stabRules[SplitStab], r.roll))
		if !slices.Equal(got, []share{{head, 100}}) || len(r.ns) != 0 {
			t.Errorf("%v: stab %v draws %v", a, got, r.ns)
		}
		got = stabShares(planStab(&b, brain, 100, rule, splitNever))
		if !slices.Equal(got, []share{{brain, 40}, {head, 75}}) {
			t.Errorf("%v: first on brain %v", a, got)
		}
		m := Body{Anatomy: a}
		m.Wounds = nil
		m.Missing = bit(brain)
		if got := stabShares(planStab(&m, head, 100, rule, splitNever)); !slices.Equal(got, []share{{head, 100}}) {
			t.Errorf("%v: missing brain %v", a, got)
		}
	}
}

// TestRangedStabHumanlikeIsStab: on the humanlike plan the two rules give
// the same shares and the same draws.
func TestRangedStabHumanlikeIsStab(t *testing.T) {
	t.Parallel()
	b := Body{}
	for _, p := range []Part{Torso, planOf(Humanlike).brain, Spine, ArmL} {
		for gate := 0; gate < 100; gate++ {
			for pick := 0; pick < 400; pick++ {
				ra, rb := &splitRec{vals: []int{gate, pick}}, &splitRec{vals: []int{gate, pick}}
				a := planStab(&b, p, 100, stabRules[SplitStab], ra.roll)
				c := planStab(&b, p, 100, stabRules[SplitRangedStab], rb.roll)
				if !slices.Equal(stabShares(a), stabShares(c)) || !slices.Equal(ra.ns, rb.ns) {
					t.Fatalf("%v gate %d pick %d: %v vs %v", p, gate, pick, stabShares(a), stabShares(c))
				}
			}
		}
	}
}

// TestStabVariantTotals: end to end with dmg 10 on fresh bodies.
func TestStabVariantTotals(t *testing.T) {
	t.Parallel()
	h := Body{}
	before := h.Damage()
	StrikeBlow(&h, Torso, 10, Bruise, SplitPoke, 0, 100, (&splitRec{vals: []int{39, 0}}).roll, 1)
	if d := h.Damage() - before; d != 11 {
		t.Errorf("poke damage rose %d, want 11", d)
	}
	q := Body{Anatomy: Quadruped}
	head := q.table()[QBrain].Parent
	before = q.Damage()
	StrikeBlow(&q, head, 10, Stab, SplitRangedStab, 0, 100, (&splitRec{vals: []int{59, 0}}).roll, 1)
	if d := q.Damage() - before; d != 11 {
		t.Errorf("ranged stab damage rose %d, want 11", d)
	}
	for _, c := range []struct {
		s    Split
		want int
	}{{SplitRangedStab, 11}, {SplitStab, 10}} {
		b := Body{Anatomy: Quadruped}
		off := 0
		for p := Part(0); p < QBrain; p++ {
			off += b.table()[p].Coverage
		}
		first := true
		roll := func(int) int {
			if first {
				first = false
				return off
			}
			return 0
		}
		before := b.Damage()
		got := SpreadBlow(&b, 10, Stab, c.s, roll, 1)
		if d := b.Damage() - before; got != c.want || d != c.want {
			t.Errorf("split %d: returned %d, Damage rose %d, want %d", c.s, got, d, c.want)
		}
	}
}
