package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Relatives among raiders and visitors (ENG-003, M9b). Now and then one
// of a raid's or a visiting group's people is family of a colonist: an
// off-map person the registry already knows (a seeded parent, a raider
// who got away) of that faction or of none, or a new person made for the
// purpose and linked into a colonist's family (parent, sibling,
// half-sibling or cousin, ages kept plausible). Either way the pawn on
// the map is that person: the same entity id as its registry record, so
// its relations, bio and history follow it; off the map again it is an
// OffMap person and keeps them. A letter names the tie. Every roll comes
// from the world stream.

const (
	// relativeRaidPermille / relativeVisitPermille: the chance a raid or a
	// visiting group brings a colonist's relative.
	relativeRaidPermille  = 150
	relativeVisitPermille = 150
	// relativeSalt seeds the bios of relatives made for a raid or visit.
	relativeSalt = 0x4E1A71
	// relativeMinAge / relativeMaxAge bound a relative who comes in person.
	relativeMinAge = 16
	relativeMaxAge = 80
	// relativeParentGap: a parent is at least this much older than a child.
	relativeParentGap = 16
)

// relativeRoll is the world stream's roll for a relative (permille).
func (g *Game) relativeRoll(permille int) bool {
	if len(g.pawnIDs) == 0 {
		return false
	}
	return g.stream(streamWorld).IntN(1000) < permille
}

// techTag is the backstory pool of faction f's people.
func techTag(f *faction.Faction) backstory.Tag {
	if f != nil && faction.DefOf(f.Type).Tech == faction.Neolithic {
		return backstory.TagNeolithic
	}
	return backstory.TagIndustrial
}

// bioName is what a bio is called on the map: the nickname, else the
// first name.
func bioName(p pawngen.Pawn) string {
	if p.Nick != "" {
		return p.Nick
	}
	return p.First
}

// homeColonists lists the living colonists at home, ascending.
func (g *Game) homeColonists() []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.pawnIDs {
		if p := g.personOf(id); g.onMap(id) && (p == nil || p.Where == whereColony) {
			out = append(out, id)
		}
	}
	return out
}

// relativeFor is the person who comes as a colonist's relative for
// faction fac (1 + index) to be w (raider or visitor): a free off-map
// kin, or a newly made one. Its registry record is ready (Where w,
// Faction fac); the caller puts it on the map. 0: nobody fits.
func (g *Game) relativeFor(fac int, w where, tag backstory.Tag) entity.EntityID {
	r := g.stream(streamWorld)
	kin := g.offMapKin(fac)
	if len(kin) > 0 && r.IntN(2) == 0 {
		return g.recallKin(kin[r.IntN(len(kin))], fac, w)
	}
	if id := g.makeRelative(fac, w, tag); id != 0 {
		return id
	}
	if len(kin) > 0 {
		return g.recallKin(kin[r.IntN(len(kin))], fac, w)
	}
	return 0
}

// recallKin makes off-map person id w of faction fac.
func (g *Game) recallKin(id entity.EntityID, fac int, w where) entity.EntityID {
	p := g.personOf(id)
	p.Where, p.Faction = w, fac
	g.socState().relVer++
	return id
}

// offMapKin lists the off-map people (ascending) who are family of a
// colonist at home and may come for faction fac: alive, grown, of that
// faction or of none, not on the map.
func (g *Game) offMapKin(fac int) []entity.EntityID {
	var out []entity.EntityID
	for _, c := range g.homeColonists() {
		for _, k := range g.kinCandidates(c) {
			p := g.personOf(k)
			if p == nil || p.Where != whereOffMap || !p.HasBio || p.Faction != fac && p.Faction != 0 {
				continue
			}
			if age := p.Bio.AgeYears(); age < relativeMinAge || age > relativeMaxAge {
				continue
			}
			if _, placed := g.positions.Get(k); placed || len(g.kinOf(c, k)) == 0 {
				continue
			}
			if i, found := slices.BinarySearch(out, k); !found {
				out = slices.Insert(out, i, k)
			}
		}
	}
	return out
}

// relPlan is how a new relative joins a colonist's family.
type relPlan struct {
	colonist entity.EntityID
	male     bool
	age      int
	parents  [2]entity.EntityID // the new person's parents
	slot     int                // >= 0: the new person is the colonist's parent in this slot
	via      entity.EntityID    // cousin: the colonist's parent whose parents an aunt or uncle shares
	auntMale bool
	auntAge  int
}

// ageOf is id's age in years (0: unknown).
func (g *Game) ageOf(id entity.EntityID) int {
	if p, ok := g.personBio(id); ok {
		return p.AgeYears()
	}
	return 0
}

// relPlans lists the ways a new relative can join colonist c's family.
func (g *Game) relPlans(c entity.EntityID) []relPlan {
	cp := g.ensurePerson(c)
	cAge := g.ageOf(c)
	if cAge <= 0 {
		return nil
	}
	r := g.stream(streamWorld)
	var out []relPlan
	ps := cp.Parents
	switch {
	case ps == [2]entity.EntityID{}:
		// a parent; only with no parent known, so no sibling of c turns half
		male := r.IntN(2) == 0
		age := cAge + 18 + r.IntN(15)
		if age <= relativeMaxAge {
			slot := 0
			if male {
				slot = 1
			}
			out = append(out, relPlan{colonist: c, male: male, age: age, slot: slot})
		}
	default:
		// a sibling (both parents known) or a half-sibling (one)
		capAge := relativeMaxAge
		for _, p := range ps {
			if p != 0 {
				if a := g.ageOf(p); a > 0 {
					capAge = min(capAge, a-relativeParentGap)
				}
			}
		}
		age := min(cAge+r.IntN(11)-5, capAge)
		if age >= relativeMinAge {
			out = append(out, relPlan{colonist: c, male: r.IntN(2) == 0, age: age, parents: ps, slot: -1})
		}
	}
	for _, p := range ps {
		pp := g.personOf(p)
		if p == 0 || pp == nil || pp.Parents == [2]entity.EntityID{} {
			continue
		}
		pAge := g.ageOf(p)
		capAunt := relativeMaxAge + relativeParentGap
		for _, gp := range pp.Parents {
			if a := g.ageOf(gp); gp != 0 && a > 0 {
				capAunt = min(capAunt, a-relativeParentGap)
			}
		}
		auntAge := min(pAge+r.IntN(11)-5, capAunt)
		age := min(cAge+r.IntN(11)-5, auntAge-relativeParentGap, relativeMaxAge)
		if pAge > 0 && age >= relativeMinAge {
			out = append(out, relPlan{colonist: c, male: r.IntN(2) == 0, age: age, slot: -1, via: p, auntMale: r.IntN(2) == 0, auntAge: auntAge})
		}
		break // one side of the family is enough
	}
	return out
}

// makeRelative registers a new person, a relative of a random colonist at
// home, as w of faction fac. 0: no colonist's family has room.
func (g *Game) makeRelative(fac int, w where, tag backstory.Tag) entity.EntityID {
	cols := g.homeColonists()
	if len(cols) == 0 {
		return 0
	}
	r := g.stream(streamWorld)
	start := r.IntN(len(cols))
	var plans []relPlan
	for i := range cols {
		if plans = g.relPlans(cols[(start+i)%len(cols)]); len(plans) > 0 {
			break
		}
	}
	if len(plans) == 0 {
		return 0
	}
	pl := plans[r.IntN(len(plans))]
	last := ""
	if b, ok := g.personBio(pl.colonist); ok {
		last = b.Last
	}
	parents := pl.parents
	if pl.via != 0 { // a cousin: first the aunt or uncle, off the map
		au := g.entities.Next()
		bio := g.relativeBio(au, pl.auntMale, pl.auntAge, backstory.TagIndustrial)
		if b, ok := g.personBio(pl.via); ok {
			bio.Last = b.Last
		}
		g.peopleReg().add(&person{ID: au, Bio: bio, HasBio: true, Where: whereOffMap})
		g.socState().sig[au] = personSig{}
		g.setParents(au, g.personOf(pl.via).Parents)
		parents = [2]entity.EntityID{au, 0}
		if pl.auntMale {
			parents = [2]entity.EntityID{0, au}
			last = bio.Last
		}
	}
	id := g.entities.Next()
	bio := g.relativeBio(id, pl.male, pl.age, tag)
	if last != "" && (pl.via == 0 || pl.auntMale) {
		bio.Last = last
	}
	g.peopleReg().add(&person{ID: id, Bio: bio, HasBio: true, Faction: fac, Where: w})
	g.socState().sig[id] = personSig{}
	if parents != [2]entity.EntityID{} {
		g.setParents(id, parents)
	}
	if pl.slot >= 0 {
		cp := g.personOf(pl.colonist).Parents
		cp[pl.slot] = id
		g.setParents(pl.colonist, cp)
	}
	return id
}

// relativeBio is a generated bio for relative id: sex and age as given,
// backstories from pool tag.
func (g *Game) relativeBio(id entity.EntityID, male bool, age int, tag backstory.Tag) pawngen.Pawn {
	want := look.Female
	if male {
		want = look.Male
	}
	seed := g.streams.seed
	if g.hasSetup() {
		seed = g.worldSeed()
	}
	var p pawngen.Pawn
	for roll := 0; roll < 32; roll++ {
		p = pawngen.Generate(seed^relativeSalt, int(id), roll, pawngen.Rules{Tech: tag})
		if p.Look.Sex == want {
			break
		}
	}
	p.Look.Sex = want
	p.AgeDays = int32(max(age, 1) * pawngen.DaysPerYear)
	return p
}

// closestKin is the colonist at home most closely related to id and how
// (id is c's <kind>).
func (g *Game) closestKin(id entity.EntityID) (entity.EntityID, social.Kind, bool) {
	var best entity.EntityID
	var bestK social.Kind
	for _, c := range g.homeColonists() {
		k, ok := social.StrongestKin(g.kinOf(c, id))
		if ok && (best == 0 || social.Bonus(k) > social.Bonus(bestK)) {
			best, bestK = c, k
		}
	}
	return best, bestK, best != 0
}

// relativeLetter posts title: "<colonist>'s <relation> <name> is among
// the <among>." with the relative in focus.
func (g *Game) relativeLetter(kind, title string, id entity.EntityID, among string) {
	c, k, ok := g.closestKin(id)
	if !ok {
		return
	}
	bio, _ := g.personBio(id)
	g.letter(kind, title, fmt.Sprintf("%s's %s %s is among the %s.", g.personName(c), social.Label(k, bio.Look.Sex == look.Male),
		g.personName(id), among), id)
}
