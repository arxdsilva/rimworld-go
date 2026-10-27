package game

import (
	"slices"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// where is where a person is (saved, append only).
type where uint8

const (
	whereColony where = iota
	whereRaider
	whereVisitor
	wherePrisoner
	whereCaravan
	whereOffMap
	whereKidnapped
	whereDead
)

// worldPersonSalt seeds the bios of off-map people (a pure function of
// the person's id, so no stream moves).
const worldPersonSalt = 0x30E1D7E0

// person is someone the colony knows of. Colonists' bios live in the
// skill state (g.sk.bios); everyone else's here (HasBio).
type person struct {
	ID       entity.EntityID
	Bio      pawngen.Pawn
	HasBio   bool
	Faction  int // 1 + faction index; 0: the colony or none
	Where    where
	DiedAt   uint64
	KilledBy entity.EntityID
	Parents  [2]entity.EntityID // [mother, father]; 0 unknown
}

// peopleRegistry is every person the colony knows of (ENG-003, M3):
// colonists, raiders, visitors, prisoners and off-map relatives, with
// their bios and where they are. nil until the first person is
// registered; saved as People.
type peopleRegistry struct {
	byID   map[entity.EntityID]*person // lookup only, never iterated
	ids    []entity.EntityID           // sorted
	kids   map[entity.EntityID][]entity.EntityID
	seeded bool // seedFamilies ran
}

// peopleSave is peopleRegistry as saved.
type peopleSave struct {
	Version uint8 // format of this block; 0 = empty
	Seeded  bool
	Persons []personSave
}

type personSave struct {
	ID       entity.EntityID
	Bio      *pawngen.Pawn // nil: the bio is the colonist's (Bios)
	Faction  int
	Where    uint8
	DiedAt   uint64
	KilledBy entity.EntityID
	Parents  [2]entity.EntityID
}

func newPeople() *peopleRegistry {
	return &peopleRegistry{byID: map[entity.EntityID]*person{}, kids: map[entity.EntityID][]entity.EntityID{}}
}

// peopleReg is the registry, created on first use.
func (g *Game) peopleReg() *peopleRegistry {
	if g.people == nil {
		g.people = newPeople()
	}
	return g.people
}

// personOf is id's record (nil: unknown). It never registers anyone, so
// read models may call it.
func (g *Game) personOf(id entity.EntityID) *person {
	if g.people == nil {
		return nil
	}
	return g.people.byID[id]
}

func (r *peopleRegistry) add(p *person) {
	if _, ok := r.byID[p.ID]; ok {
		return
	}
	r.byID[p.ID] = p
	i, _ := slices.BinarySearch(r.ids, p.ID)
	r.ids = slices.Insert(r.ids, i, p.ID)
	r.linkKids(p.ID, p.Parents)
}

func (r *peopleRegistry) linkKids(child entity.EntityID, ps [2]entity.EntityID) {
	for _, pa := range ps {
		if pa == 0 {
			continue
		}
		ks := r.kids[pa]
		if i, found := slices.BinarySearch(ks, child); !found {
			r.kids[pa] = slices.Insert(ks, i, child)
		}
	}
}

func (r *peopleRegistry) unlinkKids(child entity.EntityID, ps [2]entity.EntityID) {
	for _, pa := range ps {
		if pa == 0 {
			continue
		}
		ks := r.kids[pa]
		if i, found := slices.BinarySearch(ks, child); found {
			ks = slices.Delete(ks, i, i+1)
			if len(ks) == 0 {
				delete(r.kids, pa)
			} else {
				r.kids[pa] = ks
			}
		}
	}
}

func (r *peopleRegistry) remove(id entity.EntityID) {
	p := r.byID[id]
	if p == nil {
		return
	}
	r.unlinkKids(id, p.Parents)
	delete(r.byID, id)
	if i, found := slices.BinarySearch(r.ids, id); found {
		r.ids = slices.Delete(r.ids, i, i+1)
	}
}

// setParents records child's parents ([mother, father]).
func (g *Game) setParents(child entity.EntityID, ps [2]entity.EntityID) {
	p := g.personOf(child)
	if p == nil {
		return
	}
	g.people.unlinkKids(child, p.Parents)
	p.Parents = ps
	g.people.linkKids(child, ps)
	g.socState().relVer++
}

// ensurePerson registers colonist id when the registry does not know it
// yet (spawned colonists, older saves). Simulation paths only: a read
// model never registers anyone.
func (g *Game) ensurePerson(id entity.EntityID) *person {
	if p := g.personOf(id); p != nil {
		return p
	}
	p := &person{ID: id, Where: whereColony}
	if g.isRaider(id) {
		p.Where = whereRaider
	}
	if v, ok := g.vital.Get(id); ok && v.Status == health.Dead {
		p.Where = whereDead
	}
	if _, ok := g.sk.bios[id]; !ok {
		p.Bio, p.HasBio = legacyBio(id, g.nameOf(id)), true
	}
	g.peopleReg().add(p)
	g.refreshSig(id)
	return p
}

// personBio is id's identity: the colonist bio, else the registry's.
func (g *Game) personBio(id entity.EntityID) (pawngen.Pawn, bool) {
	if p, ok := g.sk.bios[id]; ok {
		return p, true
	}
	if p := g.personOf(id); p != nil && p.HasBio {
		return p.Bio, true
	}
	return pawngen.Pawn{}, false
}

// personName is what the social log and tab call id.
func (g *Game) personName(id entity.EntityID) string {
	if n, ok := g.names.Get(id); ok && n != "" {
		return n
	}
	if p, ok := g.personBio(id); ok {
		if p.Nick != "" {
			return p.Nick
		}
		if p.First != "" {
			return p.First
		}
	}
	return "someone"
}

// setWhere moves person id (kidnapping, caravans, prisoners: M8/M9).
func (g *Game) setWhere(id entity.EntityID, w where) {
	if p := g.personOf(id); p != nil {
		p.Where = w
	}
}

// raiderPerson registers raider id of faction fac (0: a legacy raid) as a
// world person, right after it spawns, with a full bio (the same one its
// generated name came from). The bio stays in the registry: giving it to
// the skill state would change the raider's stats, look and violence.
func (g *Game) raiderPerson(id entity.EntityID, fac int) {
	tag := backstory.TagIndustrial
	seed := g.streams.seed
	if f := g.factionOf(fac); f != nil {
		if faction.DefOf(f.Type).Tech == faction.Neolithic {
			tag = backstory.TagNeolithic
		}
		seed = g.worldSeed()
	}
	p := pawngen.Generate(seed^raiderNameSalt, int(id), 0, pawngen.Rules{Tech: tag})
	if name, ok := g.names.Get(id); ok && name != "" && name != p.Nick && name != p.First {
		p.First, p.Nick = name, ""
	}
	g.peopleReg().add(&person{ID: id, Bio: p, HasBio: true, Faction: fac, Where: whereRaider})
	g.socState().sig[id] = personSig{}
}

// worldBio is a generated bio for off-map person id: sex and age as given.
func (g *Game) worldBio(id entity.EntityID, male bool, age int) pawngen.Pawn {
	want := look.Female
	if male {
		want = look.Male
	}
	var p pawngen.Pawn
	for roll := 0; roll < 32; roll++ {
		p = pawngen.Generate(g.streams.seed^worldPersonSalt, int(id), roll, pawngen.Rules{})
		if p.Look.Sex == want {
			break
		}
	}
	p.Look.Sex = want
	p.AgeDays = int32(max(age, 1) * pawngen.DaysPerYear)
	return p
}

// seedFamilies gives the founding colonists their starting relations,
// once, when the colony is founded: siblings, parents and spouses among
// them, off-map relatives behind them (entity ids from the allocator, no
// map components) and aligned surnames.
func (g *Game) seedFamilies() {
	reg := g.peopleReg()
	if reg.seeded {
		return
	}
	reg.seeded = true
	var cols []social.Person
	for _, id := range g.pawnIDs {
		if !g.alive(id) {
			continue
		}
		g.ensurePerson(id)
		cols = append(cols, g.socialPerson(id))
	}
	if len(cols) < 1 {
		return
	}
	now := uint64(g.clock.Now())
	r := g.stream(streamSocial)
	plan := social.Families(g.streams.seed, cols, r.IntN, social.ID(g.entities.Peek()))
	spec := map[social.ID]bool{}
	for _, sp := range plan.Specs {
		id := g.entities.Next()
		if id != entity.EntityID(sp.ID) {
			panic("game: family plan ids out of step with the allocator")
		}
		p := &person{ID: id, Bio: g.worldBio(id, sp.Male, sp.AgeYears), HasBio: true, Where: whereOffMap}
		if sp.Dead {
			p.Where = whereDead
		}
		reg.add(p)
		g.socState().sig[id] = personSig{}
		spec[sp.ID] = true
	}
	for _, pl := range plan.Parents {
		g.setParents(entity.EntityID(pl.Child), [2]entity.EntityID{entity.EntityID(pl.Parents[0]), entity.EntityID(pl.Parents[1])})
	}
	for _, m := range plan.Marriages {
		g.setPair(entity.EntityID(m.A), entity.EntityID(m.B), social.Pair{Kind: social.PairSpouse, Since: now, LastChange: now})
	}
	for _, sp := range plan.Specs {
		if sp.Spouse != 0 && sp.ID < sp.Spouse {
			g.setPair(entity.EntityID(sp.ID), entity.EntityID(sp.Spouse), social.Pair{Kind: social.PairSpouse})
		}
	}
	g.alignSurnames(plan, spec)
}

// alignSurnames gives families one surname (Bio.Last only): an off-map
// father takes his first colonist child's, his line and his wife follow,
// then each colonist takes the name its hint points at.
func (g *Game) alignSurnames(plan social.Plan, spec map[social.ID]bool) {
	named := map[social.ID]bool{}
	last := func(id social.ID) string { p, _ := g.personBio(entity.EntityID(id)); return p.Last }
	set := func(sid social.ID, s string) {
		id := entity.EntityID(sid)
		if p := g.personOf(id); p != nil && p.HasBio {
			p.Bio.Last = s
			return
		}
		if p, ok := g.sk.bios[id]; ok {
			p.Last = s
			g.sk.bios[id] = p
		}
	}
	for _, h := range plan.Surnames {
		if spec[h.From] && !named[h.From] {
			named[h.From] = true
			set(h.From, last(h.Person))
		}
	}
	for pass := 0; pass <= social.MaxFamilyDepth; pass++ {
		for _, pl := range plan.Parents {
			f := pl.Parents[1]
			if f == 0 || !spec[f] {
				continue
			}
			switch {
			case named[f] && spec[pl.Child] && !named[pl.Child]:
				named[pl.Child] = true
				set(pl.Child, last(f))
			case !named[f] && named[pl.Child]:
				named[f] = true
				set(f, last(pl.Child))
			}
		}
	}
	for _, sp := range plan.Specs {
		if !sp.Male && sp.Spouse != 0 && named[sp.Spouse] {
			set(sp.ID, last(sp.Spouse))
		}
	}
	for _, h := range plan.Surnames {
		if !spec[h.Person] {
			set(h.Person, last(h.From))
		}
	}
}

// famGraph is the family graph KinOf walks: registry parents, spouse pairs.
type famGraph struct{ g *Game }

func (f famGraph) Parents(id social.ID) [2]social.ID {
	if p := f.g.personOf(entity.EntityID(id)); p != nil {
		return [2]social.ID{social.ID(p.Parents[0]), social.ID(p.Parents[1])}
	}
	return [2]social.ID{}
}

func (f famGraph) Spouses(id social.ID) []social.ID {
	if f.g.soc == nil {
		return nil
	}
	return f.g.soc.spouses[entity.EntityID(id)]
}

// kinOf lists b's relations to a (b is a's <kind>): family, then the
// pair's romance relation.
func (g *Game) kinOf(a, b entity.EntityID) []social.Kind {
	ks := social.KinOf(social.ID(a), social.ID(b), famGraph{g})
	if e := g.edgeOf(a, b); e != nil {
		if k, ok := e.pair.Kind.Relation(); ok {
			ks = append(ks, k)
		}
	}
	return ks
}

// kinCandidates lists the people within family reach of id (parents,
// children and spouses, a bounded walk), ascending, without id.
func (g *Game) kinCandidates(id entity.EntityID) []entity.EntityID {
	if g.people == nil {
		return nil
	}
	seen := []entity.EntityID{id}
	level := []entity.EntityID{id}
	for d := 0; d < 2*social.MaxFamilyDepth && len(level) > 0; d++ {
		var next []entity.EntityID
		visit := func(x entity.EntityID) {
			if x == 0 || slices.Contains(seen, x) {
				return
			}
			seen = append(seen, x)
			next = append(next, x)
		}
		for _, x := range level {
			if p := g.personOf(x); p != nil {
				visit(p.Parents[0])
				visit(p.Parents[1])
			}
			for _, k := range g.people.kids[x] {
				visit(k)
			}
			for _, s := range (famGraph{g}).Spouses(social.ID(x)) {
				visit(entity.EntityID(s))
			}
		}
		level = next
	}
	out := seen[1:]
	slices.Sort(out)
	return out
}

// pruneStrangers drops, on the day rollover, people the colony has no
// tie to: off the map (or dead, not colonists), no social edge, no family.
func (g *Game) pruneStrangers() {
	reg := g.people
	if reg == nil {
		return
	}
	var drop []entity.EntityID
	for _, id := range reg.ids {
		p := reg.byID[id]
		if p.Where == whereColony || p.Where == wherePrisoner || p.Where == whereCaravan || p.Where == whereKidnapped {
			continue
		}
		if slices.Contains(g.pawnIDs, id) || p.Where != whereDead && g.onMap(id) {
			continue
		}
		if p.Parents != [2]entity.EntityID{} || len(reg.kids[id]) > 0 || g.soc != nil && len(g.soc.adj[id]) > 0 {
			continue
		}
		drop = append(drop, id)
	}
	for _, id := range drop {
		reg.remove(id)
		if g.soc != nil {
			g.soc.forget(id)
		}
	}
}

// onMap reports id stands on the map, alive or downed.
func (g *Game) onMap(id entity.EntityID) bool {
	if _, ok := g.positions.Get(id); !ok {
		return false
	}
	v, ok := g.vital.Get(id)
	return ok && v.Status != health.Dead
}

// exportPeople is the People save field (nil: nothing yet).
func (g *Game) exportPeople() *peopleSave {
	reg := g.people
	if reg == nil || len(reg.ids) == 0 && !reg.seeded {
		return nil
	}
	out := &peopleSave{Version: 1, Seeded: reg.seeded}
	for _, id := range reg.ids {
		p := reg.byID[id]
		ps := personSave{ID: id, Faction: p.Faction, Where: uint8(p.Where), DiedAt: p.DiedAt, KilledBy: p.KilledBy, Parents: p.Parents}
		if p.HasBio {
			b := p.Bio
			ps.Bio = &b
		}
		out.Persons = append(out.Persons, ps)
	}
	return out
}

// importPeople restores exportPeople's state (nil: fresh; an older save
// registers its colonists on the first social pass).
func (g *Game) importPeople(s *peopleSave) {
	g.people = nil
	if s == nil {
		return
	}
	reg := g.peopleReg()
	reg.seeded = s.Seeded
	for _, ps := range s.Persons {
		p := &person{ID: ps.ID, Faction: ps.Faction, Where: where(ps.Where), DiedAt: ps.DiedAt, KilledBy: ps.KilledBy, Parents: ps.Parents}
		if ps.Bio != nil {
			p.Bio, p.HasBio = *ps.Bio, true
		}
		reg.add(p)
	}
}
