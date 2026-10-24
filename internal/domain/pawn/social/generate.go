package social

// Chances (permille) for colonists' starting family ties.
const (
	chanceSiblings    = 60
	chanceParentChild = 40
	chanceSpouses     = 50
	chanceOffMapKin   = 700 // a colonist has living or dead parents off-map
	chanceOffMapSib   = 300
	chanceGrandparent = 500
	minParentGap      = 16
	maxSpouseGap      = 15
	spouseMinAge      = 18
	wifeTakesName     = 500
)

// PersonSpec is an off-map relative to create. IDs are allocated by the
// caller's firstID and increase in order of appearance.
type PersonSpec struct {
	ID       ID
	Male     bool
	AgeYears int
	Dead     bool
	Spouse   ID
}

// ParentLink says who a person's parents are ([mother, father]).
type ParentLink struct {
	Child   ID
	Parents [2]ID
}

// Marriage is a starting spouse pair.
type Marriage struct{ A, B ID }

// SurnameHint says Person takes the surname of From.
type SurnameHint struct{ Person, From ID }

// Plan is the starting family web of a colony.
type Plan struct {
	Specs     []PersonSpec
	Parents   []ParentLink // colonists and specs
	Marriages []Marriage   // colonist-colonist
	Surnames  []SurnameHint
}

type famGen struct {
	roll  func(int) int
	next  ID
	plan  Plan
	par   map[ID][2]ID // lookup only, never iterated
	spec  map[ID]int   // index into plan.Specs, lookup only
	wed   map[ID]bool
	ages  map[ID]int // colonists
	order []ID
}

// Families builds the starting family ties among colonists (chance-based
// siblings, parent-child and spouses) and the off-map relatives behind
// them. roll(n) is in [0,n); when nil a stream derived from seed is used.
// Spec IDs start at firstID.
func Families(seed uint64, colonists []Person, roll func(int) int, firstID ID) Plan {
	if roll == nil {
		state := seed
		roll = func(n int) int {
			state = splitmix64(state)
			return int(state % uint64(n))
		}
	}
	f := &famGen{roll: roll, next: firstID, par: map[ID][2]ID{}, spec: map[ID]int{}, wed: map[ID]bool{}, ages: map[ID]int{}}
	byID := map[ID]Person{}
	for _, c := range colonists {
		byID[c.ID] = c
		f.ages[c.ID] = c.AgeYears
	}
	for i := 0; i < len(colonists); i++ {
		for j := i + 1; j < len(colonists); j++ {
			f.pair(colonists[i], colonists[j])
		}
	}
	for _, c := range colonists {
		f.backstory(c)
	}
	for _, c := range colonists {
		f.surname(c, byID)
	}
	return f.plan
}

func (f *famGen) hit(permille int) bool { return f.roll(1000) < permille }

func (f *famGen) setParents(child ID, p [2]ID) {
	f.par[child] = p
	for i := range f.plan.Parents {
		if f.plan.Parents[i].Child == child {
			f.plan.Parents[i].Parents = p
			return
		}
	}
	f.plan.Parents = append(f.plan.Parents, ParentLink{child, p})
}

func (f *famGen) newSpec(male bool, age int, dead bool) *PersonSpec {
	f.plan.Specs = append(f.plan.Specs, PersonSpec{ID: f.next, Male: male, AgeYears: age, Dead: dead})
	f.spec[f.next] = len(f.plan.Specs) - 1
	f.next++
	return &f.plan.Specs[len(f.plan.Specs)-1]
}

func (f *famGen) deadAt(age int) bool { return f.roll(1000) < age*8-100 }

// makeParents creates a mother and father for a child of the given age.
func (f *famGen) makeParents(childAge int) [2]ID {
	mAge := childAge + 18 + f.roll(15)
	fAge := childAge + 18 + f.roll(20)
	mDead, fDead := f.deadAt(mAge), f.deadAt(fAge)
	m := f.newSpec(false, mAge, mDead).ID
	fa := f.newSpec(true, fAge, fDead).ID
	f.plan.Specs[f.spec[m]].Spouse = fa
	f.plan.Specs[f.spec[fa]].Spouse = m
	return [2]ID{m, fa}
}

// canParent reports whether every known parent is old enough for child c.
func (f *famGen) canParent(ps [2]ID, c Person) bool {
	for _, id := range ps {
		if i, ok := f.spec[id]; ok && f.plan.Specs[i].AgeYears-c.AgeYears < minParentGap {
			return false
		}
		if age, ok := f.ages[id]; ok && age-c.AgeYears < minParentGap {
			return false
		}
	}
	return true
}

func (f *famGen) pair(a, b Person) {
	if f.hit(chanceSiblings) {
		pa, pb := f.par[a.ID], f.par[b.ID]
		switch {
		case pa == [2]ID{} && pb == [2]ID{}:
			age := a.AgeYears
			if b.AgeYears > age {
				age = b.AgeYears
			}
			p := f.makeParents(age)
			f.setParents(a.ID, p)
			f.setParents(b.ID, p)
		case pa == [2]ID{} && pb != [2]ID{}:
			if f.canParent(pb, a) {
				f.setParents(a.ID, pb)
			}
		case pb == [2]ID{} && pa != [2]ID{}:
			if f.canParent(pa, b) {
				f.setParents(b.ID, pa)
			}
		}
		return
	}
	if f.hit(chanceParentChild) {
		old, young := a, b
		if b.AgeYears > a.AgeYears {
			old, young = b, a
		}
		slot := 0
		if old.Male {
			slot = 1
		}
		cur := f.par[young.ID]
		if old.AgeYears-young.AgeYears >= minParentGap && cur[slot] == 0 && !f.wed[old.ID] {
			cur[slot] = old.ID
			f.setParents(young.ID, cur)
		}
		return
	}
	if f.hit(chanceSpouses) && !f.wed[a.ID] && !f.wed[b.ID] &&
		a.AgeYears >= spouseMinAge && b.AgeYears >= spouseMinAge && Attraction(a, b, f.related(a, b), 2000) > 0 &&
		Attraction(b, a, f.related(a, b), 2000) > 0 {
		gap := a.AgeYears - b.AgeYears
		if gap < 0 {
			gap = -gap
		}
		if gap <= maxSpouseGap && !f.related(a, b) {
			f.wed[a.ID], f.wed[b.ID] = true, true
			f.plan.Marriages = append(f.plan.Marriages, Marriage{a.ID, b.ID})
		}
	}
}

// related is true when a and b already share a parent or are parent/child.
func (f *famGen) related(a, b Person) bool {
	pa, pb := f.par[a.ID], f.par[b.ID]
	for _, x := range pa {
		if x == 0 {
			continue
		}
		if x == b.ID || x == pb[0] || x == pb[1] {
			return true
		}
	}
	for _, x := range pb {
		if x != 0 && x == a.ID {
			return true
		}
	}
	return false
}

// backstory gives a colonist off-map parents, siblings and grandparents.
func (f *famGen) backstory(c Person) {
	p := f.par[c.ID]
	if p == [2]ID{} && f.hit(chanceOffMapKin) {
		p = f.makeParents(c.AgeYears)
		f.setParents(c.ID, p)
		if f.hit(chanceOffMapSib) {
			age := c.AgeYears + f.roll(11) - 5
			for _, id := range p {
				if cap := f.plan.Specs[f.spec[id]].AgeYears - 18; age > cap {
					age = cap
				}
			}
			if age < 1 {
				age = 1
			}
			s := f.newSpec(f.roll(2) == 0, age, f.deadAt(age))
			f.setParents(s.ID, p)
		}
	}
	for _, id := range p {
		if i, ok := f.spec[id]; ok && f.hit(chanceGrandparent) {
			if f.par[id] != [2]ID{} {
				continue
			}
			age := f.plan.Specs[i].AgeYears
			gp := f.makeParents(age)
			f.setParents(id, gp)
		}
	}
}

func (f *famGen) surname(c Person, byID map[ID]Person) {
	p := f.par[c.ID]
	if p[1] != 0 {
		f.plan.Surnames = append(f.plan.Surnames, SurnameHint{c.ID, p[1]})
	}
	for _, m := range f.plan.Marriages {
		if m.A != c.ID && m.B != c.ID {
			continue
		}
		other := m.A
		if other == c.ID {
			other = m.B
		}
		o := byID[other]
		if !c.Male && o.Male && f.hit(wifeTakesName) {
			f.plan.Surnames = append(f.plan.Surnames, SurnameHint{c.ID, other})
		}
	}
}
