// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// cutEdges are the upper edges of a cut's part-count buckets on one roll
// of 100: 1 part under 30, 2 under 75, 3 under 95, else 4. wiki: Damage
// Types (Cut: 30 %, 45 %, 20 % and 5 % for 1 to 4 parts).
var cutEdges = [4]int{30, 75, 95, 100}

// cutCount draws how many parts a cut wounds (1-4): one roll of 100.
func cutCount(roll func(int) int) int {
	r := min(max(roll(100), 0), 99)
	for i, e := range cutEdges {
		if r < e {
			return i + 1
		}
	}
	return len(cutEdges)
}

// cutTotal is a cut of dmg over k parts: dmg x (1+1.4)/(k+1.4) x k,
// rounded half up (x1.00, 1.41, 1.64, 1.78). wiki: Damage Types (Cut).
func cutTotal(dmg, k int) int {
	d := 10*k + 14
	return (2*dmg*24*k + d) / (2 * d)
}

// cutCandidates is the set of parts a cut first striking p may also
// wound: p's parent (unless p is the root) and children, plus, when p is
// inside, its inside siblings; present parts only, never p. original
// tuning: the wiki says "adjacent" without defining it. The humanlike
// table is flat, so a cut first striking the torso may reach any part
// hanging from it (an arm, a leg, the brain, an organ).
func cutCandidates(b *Body, p Part) uint32 {
	t := b.table()
	n := Part(len(t))
	if p >= n {
		return 0
	}
	par := t[p].Parent
	var m uint32
	if par != p && !b.isMissing(par) {
		m |= bit(par)
	}
	in := inside(b, p)
	for q := Part(0); q < n; q++ {
		if q == p || b.isMissing(q) {
			continue
		}
		if t[q].Parent == p || in && t[q].Parent == par && inside(b, q) {
			m |= bit(q)
		}
	}
	return m
}

// pickByCoverage draws one part of mask by coverage: one roll of the
// set's coverage, none when it has no coverage (false). original tuning:
// the wiki does not say how a cut's extra parts or a blunt blow's inner
// part are chosen; weighting them by coverage matches how a blow picks
// its first part (PickPart).
func pickByCoverage(b *Body, mask uint32, roll func(int) int) (Part, bool) {
	t := b.table()
	total := 0
	for q := range t {
		if mask&bit(Part(q)) != 0 {
			total += t[q].Coverage
		}
	}
	if total <= 0 {
		return 0, false
	}
	r := min(max(roll(total), 0), total-1)
	for q := range t {
		if mask&bit(Part(q)) == 0 {
			continue
		}
		if r < t[q].Coverage {
			return Part(q), true
		}
		r -= t[q].Coverage
	}
	return 0, false
}

// planCut is a cut of dmg first striking p (R1): it draws the part count,
// then one coverage pick per extra part among cutCandidates without
// replacement (original tuning: coverage-weighted, pickByCoverage); when
// they run out the count shrinks with them. A single cut never wounds an
// inside part: it lands on the nearest outer ancestor instead (original
// tuning: moved, not re-rolled). The total (cutTotal at the final count)
// is split evenly, the first target taking the remainder (original
// tuning: the wiki gives no rounding), then each inside, non-bone target
// whose parent is spared gives that parent a bonus (cutBonuses).
func planCut(b *Body, p Part, dmg int, roll func(int) int) shares {
	var sh shares
	k := cutCount(roll)
	if k == 1 {
		sh.add(outerOf(b, p), dmg)
		return sh
	}
	var picked [4]Part
	picked[0] = p
	n, cand := 1, cutCandidates(b, p)
	for n < k {
		q, ok := pickByCoverage(b, cand, roll)
		if !ok {
			break
		}
		cand &^= bit(q)
		picked[n] = q
		n++
	}
	total := cutTotal(dmg, n)
	each := total / n
	sh.add(picked[0], total-each*(n-1))
	for _, q := range picked[1:n] {
		sh.add(q, each)
	}
	cutBonuses(b, &sh)
	return sh
}

// cutBonuses appends the parent bonuses to sh's targets: a target that is
// inside and not bone, whose parent is present and not a target, gives
// that parent a wound of its own share; each parent takes one bonus, of
// the first such target in order. wiki: Damage Types (Cut: the parent of
// a struck inner part is also damaged; bones exempt). original tuning:
// one bonus a parent.
func cutBonuses(b *Body, sh *shares) {
	t := b.table()
	nt := sh.n
	var targets, given uint32
	for _, x := range sh.s[:nt] {
		targets |= bit(x.part)
	}
	for _, x := range sh.s[:nt] {
		if !inside(b, x.part) || solid(b, x.part) {
			continue
		}
		par := t[x.part].Parent
		if par == x.part || b.isMissing(par) || (targets|given)&bit(par) != 0 {
			continue
		}
		given |= bit(par)
		sh.add(par, x.dmg)
	}
}
