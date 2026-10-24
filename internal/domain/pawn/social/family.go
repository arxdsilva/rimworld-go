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

// ancestors lists id's ancestors up to MaxFamilyDepth generations, nearest
// first, in a deterministic order.
func ancestors(id ID, g Graph) []ancestor {
	out := []ancestor{}
	level := []ID{id}
	for gen := 1; gen <= MaxFamilyDepth; gen++ {
		var next []ID
		for _, x := range level {
			for _, p := range g.Parents(x) {
				if p != 0 {
					out = append(out, ancestor{p, gen})
					next = append(next, p)
				}
			}
		}
		level = next
	}
	return out
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

// KinOf lists every relation b has to a (b is a's <kind>): blood relations
// from a bounded walk, then step and in-law relations through Spouses.
func KinOf(a, b ID, g Graph) []Kind {
	if a == 0 || b == 0 || a == b || g == nil {
		return nil
	}
	var out []Kind
	if k, ok := bloodKin(a, b, g); ok {
		out = append(out, k)
	}
	out = append(out, stepKin(a, b, g)...)
	return out
}

func bloodKin(a, b ID, g Graph) (Kind, bool) {
	ancA, ancB := ancestors(a, g), ancestors(b, g)
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

func stepKin(a, b ID, g Graph) []Kind {
	var out []Kind
	pa, pb := g.Parents(a), g.Parents(b)
	// b is the spouse of one of a's parents but not a parent.
	for _, p := range pa {
		if p != 0 && b != p && !hasID(pa[:], b) && hasID(g.Spouses(p), b) {
			out = append(out, StepParent)
			break
		}
	}
	// a is the spouse of one of b's parents but not a parent.
	for _, p := range pb {
		if p != 0 && a != p && !hasID(pb[:], a) && hasID(g.Spouses(p), a) {
			out = append(out, StepChild)
			break
		}
	}
	// b is a parent of one of a's spouses.
	for _, s := range g.Spouses(a) {
		if parentIs(g.Parents(s), b) {
			out = append(out, ParentInLaw)
			break
		}
	}
	// b is the spouse of one of a's children.
	for _, s := range g.Spouses(b) {
		if parentIs(g.Parents(s), a) {
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
