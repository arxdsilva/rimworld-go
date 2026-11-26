// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package social

// Graph is the family edges KinOf walks. Parents is [mother, father];
// 0 means unknown.
type Graph interface {
	Parents(ID) [2]ID
	Spouses(ID) []ID
}

type ancestor struct {
	id  ID
	gen int
}

// maxAncestors is how many ancestors MaxFamilyDepth generations can hold
// (2 + 4 + ... parents): the walk's buffers are this big, so the usual
// family never allocates.
const maxAncestors = 2<<MaxFamilyDepth - 2

// appendAncestors appends id's ancestors up to MaxFamilyDepth generations
// to dst, nearest first, in a deterministic order.
func appendAncestors(dst []ancestor, id ID, g Graph) []ancestor {
	var bufA, bufB [1 << MaxFamilyDepth]ID
	level := append(bufA[:0], id)
	next := bufB[:0]
	for gen := 1; gen <= MaxFamilyDepth; gen++ {
		next = next[:0]
		for _, x := range level {
			for _, p := range g.Parents(x) {
				if p != 0 {
					dst = append(dst, ancestor{p, gen})
					next = append(next, p)
				}
			}
		}
		level, next = next, level
	}
	return dst
}

func genOf(list []ancestor, id ID) int {
	for _, a := range list {
		if a.id == id {
			return a.gen
		}
	}
	return 0
}

func parentIs(p [2]ID, id ID) bool { return p[0] == id || p[1] == id }

func hasID(ids []ID, id ID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// Ancestry is one person's family edges read once from a Graph: the walk
// KinBetween needs for every pair the person belongs to. A reader that asks
// about many pairs of one fixed graph builds it once per person.
type Ancestry struct {
	id      ID
	parents [2]ID
	spouses []ID
	anc     []ancestor
}

// AncestryOf reads id's parents, spouses and bounded ancestors from g.
func AncestryOf(id ID, g Graph) Ancestry {
	return Ancestry{id: id, parents: g.Parents(id), spouses: g.Spouses(id), anc: appendAncestors(nil, id, g)}
}

// Related reports whether the person has any family edge: a pair of people
// who are both unrelated can have no kin.
func (a *Ancestry) Related() bool {
	return a.parents != [2]ID{} || len(a.spouses) != 0 || len(a.anc) != 0
}

// KinOf lists every relation b has to a (b is a's <kind>): blood relations
// from a bounded walk, then step and in-law relations through Spouses.
func KinOf(a, b ID, g Graph) []Kind {
	if a == 0 || b == 0 || a == b || g == nil {
		return nil
	}
	var bufA, bufB [maxAncestors]ancestor
	return kinFrom(nil,
		Ancestry{id: a, parents: g.Parents(a), spouses: g.Spouses(a), anc: appendAncestors(bufA[:0], a, g)},
		Ancestry{id: b, parents: g.Parents(b), spouses: g.Spouses(b), anc: appendAncestors(bufB[:0], b, g)}, g)
}

// AppendKinBetween appends KinOf's relations to dst for two people whose
// Ancestry g already gave.
func AppendKinBetween(dst []Kind, a, b *Ancestry, g Graph) []Kind {
	if a.id == 0 || b.id == 0 || a.id == b.id || g == nil {
		return dst
	}
	return kinFrom(dst, *a, *b, g)
}

func kinFrom(out []Kind, a, b Ancestry, g Graph) []Kind {
	if !a.Related() && !b.Related() {
		return out
	}
	if k, ok := bloodKin(a.id, b.id, a.anc, b.anc); ok {
		out = append(out, k)
	}
	return stepKin(out, a, b, g)
}

func bloodKin(a, b ID, ancA, ancB []ancestor) (Kind, bool) {
	if gen := genOf(ancA, b); gen > 0 {
		return [...]Kind{0, Parent, Grandparent, GreatGrandparent, Kin}[gen], true
	}
	if gen := genOf(ancB, a); gen > 0 {
		return [...]Kind{0, Child, Grandchild, GreatGrandchild, Kin}[gen], true
	}
	bestA, bestB := 0, 0
	shared := 0
	for _, x := range ancA {
		gb := genOf(ancB, x.id)
		if gb == 0 {
			continue
		}
		if bestA == 0 || x.gen+gb < bestA+bestB {
			bestA, bestB = x.gen, gb
		}
		if x.gen == 1 && gb == 1 {
			shared++
		}
	}
	switch {
	case bestA == 0:
		return 0, false
	case bestA == 1 && bestB == 1:
		if shared >= 2 {
			return Sibling, true
		}
		return HalfSibling, true
	case bestA == 2 && bestB == 1:
		return AuntUncle, true
	case bestA == 1 && bestB == 2:
		return NieceNephew, true
	case bestA == 3 && bestB == 1:
		return GrandAuntUncle, true
	case bestA == 1 && bestB == 3:
		return GrandNieceNephew, true
	case bestA == 2 && bestB == 2:
		return Cousin, true
	case bestA == 3 && bestB == 3:
		return SecondCousin, true
	}
	return Kin, true
}

func stepKin(out []Kind, a, b Ancestry, g Graph) []Kind {
	pa, pb := a.parents, b.parents
	// b is the spouse of one of a's parents but not a parent.
	for _, p := range pa {
		if p != 0 && b.id != p && !hasID(pa[:], b.id) && hasID(g.Spouses(p), b.id) {
			out = append(out, StepParent)
			break
		}
	}
	// a is the spouse of one of b's parents but not a parent.
	for _, p := range pb {
		if p != 0 && a.id != p && !hasID(pb[:], a.id) && hasID(g.Spouses(p), a.id) {
			out = append(out, StepChild)
			break
		}
	}
	// b is a parent of one of a's spouses.
	for _, s := range a.spouses {
		if parentIs(g.Parents(s), b.id) {
			out = append(out, ParentInLaw)
			break
		}
	}
	// b is the spouse of one of a's children.
	for _, s := range b.spouses {
		if parentIs(g.Parents(s), a.id) {
			out = append(out, ChildInLaw)
			break
		}
	}
	return out
}

// StrongestKin picks the relation with the greatest opinion bonus (ties:
// the lowest kind); ok is false for an empty list.
func StrongestKin(ks []Kind) (Kind, bool) {
	best, ok := Kind(0), false
	for _, k := range ks {
		if !ok || Bonus(k) > Bonus(best) || (Bonus(k) == Bonus(best) && k < best) {
			best, ok = k, true
		}
	}
	return best, ok
}
