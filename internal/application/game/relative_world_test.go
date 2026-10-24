package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// hostileHuman is a hostile human faction of g (1 + index).
func hostileHuman(t *testing.T, g *Game) int {
	t.Helper()
	for i, f := range g.ng.factions {
		if f.Type.Human() && f.Relation == faction.Hostile {
			return i + 1
		}
	}
	t.Fatal("no hostile human faction")
	return 0
}

// hasLetter reports a letter whose text contains s.
func hasLetter(g *Game, s string) bool {
	for _, m := range g.World().Messages {
		if m.Letter && strings.Contains(m.Text, s) {
			return true
		}
	}
	return false
}

// relativeLine is the letter line naming id's tie to its closest colonist.
func relativeLine(t *testing.T, g *Game, id entity.EntityID, among string) string {
	t.Helper()
	c, k, ok := g.closestKin(id)
	if !ok {
		t.Fatalf("%d is no colonist's relative", id)
	}
	bio, _ := g.personBio(id)
	return g.personName(c) + "'s " + social.Label(k, bio.Look.Sex == look.Male) + " " + g.personName(id) + " is among the " + among
}

func TestRaidRelativeLinkedToAColonist(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	fac := hostileHuman(t, g)
	var kin entity.EntityID
	for i := 0; i < 80 && kin == 0; i++ {
		g.raidFrom([]item.Kind{item.Rifle, item.Rifle}, planAssault, fac)
		for _, id := range g.raiderIDs {
			if _, _, ok := g.closestKin(id); ok {
				kin = id
			}
		}
		if kin == 0 {
			for _, id := range slices.Clone(g.raiderIDs) {
				g.removeRaider(id)
			}
		}
	}
	if kin == 0 {
		t.Fatal("no raid brought a relative in 80 raids")
	}
	p := g.personOf(kin)
	if p.Where != whereRaider || p.Faction != fac || !g.isRaider(kin) || !g.onMap(kin) {
		t.Fatalf("relative raider %d: %+v", kin, p)
	}
	if name, _ := g.names.Get(kin); name != bioName(p.Bio) {
		t.Fatalf("map name %q, bio %q", name, bioName(p.Bio))
	}
	c, k, _ := g.closestKin(kin)
	if ks := social.KinOf(social.ID(c), social.ID(kin), famGraph{g}); !slices.Contains(ks, k) {
		t.Fatalf("KinOf(colonist, raider) = %v, want %v", ks, k)
	}
	if !hasLetter(g, relativeLine(t, g, kin, "raiders")) {
		t.Fatalf("no relative letter %q: %+v", relativeLine(t, g, kin, "raiders"), g.World().Messages)
	}
	// it gets away: an off-map person again, still family
	g.removeRaider(kin)
	if p.Where != whereOffMap || len(g.kinOf(c, kin)) == 0 {
		t.Fatalf("fled relative %+v, kin %v", p, g.kinOf(c, kin))
	}
	g.pruneStrangers()
	if g.personOf(kin) == nil {
		t.Fatal("the registry forgot a colonist's relative")
	}
}

func TestMadeRelativesFitTheFamily(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	fac := hostileHuman(t, g)
	kinds := map[social.Kind]int{}
	for i := 0; i < 30; i++ {
		id := g.makeRelative(fac, whereOffMap, backstory.TagIndustrial)
		if id == 0 {
			t.Fatal("no relative could be made")
		}
		c, k, ok := g.closestKin(id)
		if !ok {
			t.Fatalf("made relative %d is no one's kin", id)
		}
		kinds[k]++
		age := g.ageOf(id)
		if age < relativeMinAge || age > relativeMaxAge {
			t.Fatalf("relative %d aged %d", id, age)
		}
		p := g.personOf(id)
		switch k {
		case social.Parent:
			if age-g.ageOf(c) < relativeParentGap {
				t.Fatalf("parent %d years older than the child", age-g.ageOf(c))
			}
		case social.Sibling, social.HalfSibling, social.Cousin:
			for _, pa := range p.Parents {
				if pa != 0 && g.ageOf(pa)-age < relativeParentGap {
					t.Fatalf("%v: parent %d years older", k, g.ageOf(pa)-age)
				}
			}
		default:
			t.Fatalf("made relative is the colonist's %v", k)
		}
		if p.Where != whereOffMap || p.Faction != fac {
			t.Fatalf("made relative %+v", p)
		}
	}
	if len(kinds) < 2 {
		t.Fatalf("only %v relations were made", kinds)
	}
}

func TestRelativeForReusesOffMapKin(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	fac := hostileHuman(t, g)
	c := g.homeColonists()[0]
	// an off-map aunt with no faction (seeded relatives have none)
	cp := g.ensurePerson(c)
	au := g.entities.Next()
	g.peopleReg().add(&person{ID: au, Bio: g.worldBio(au, false, g.ageOf(c)+20), HasBio: true, Where: whereOffMap})
	if cp.Parents == [2]entity.EntityID{} {
		g.setParents(c, [2]entity.EntityID{au, 0})
	} else {
		g.setParents(au, g.personOf(cp.Parents[0]).Parents)
	}
	if !slices.Contains(g.offMapKin(fac), au) {
		t.Fatalf("off-map kin %v, want %d among them", g.offMapKin(fac), au)
	}
	for i := 0; i < 40; i++ {
		if id := g.relativeFor(fac, whereRaider, backstory.TagIndustrial); id == au {
			if p := g.personOf(au); p.Where != whereRaider || p.Faction != fac {
				t.Fatalf("recalled kin %+v", p)
			}
			return
		}
	}
	t.Fatal("relativeFor never brought the off-map relative")
}

func TestVisitorRelativeArrives(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	var kin entity.EntityID
	for i := 0; i < 80 && kin == 0; i++ {
		now := uint64(g.clock.Now())
		if !g.visitorsArrive(now) {
			t.Fatal("visitors did not arrive")
		}
		for _, id := range g.visitorIDs() {
			if _, _, ok := g.closestKin(id); ok {
				kin = id
			}
		}
		if kin == 0 {
			for _, id := range g.visitorIDs() {
				g.departVisitor(id)
			}
			g.worldSt.visitors = nil
		}
	}
	if kin == 0 {
		t.Fatal("no visiting group brought a relative in 80 visits")
	}
	if p := g.personOf(kin); p.Where != whereVisitor || !g.visitor(kin) || g.isRaider(kin) {
		t.Fatalf("relative visitor %+v", p)
	}
	f := g.factionOf(g.personOf(kin).Faction)
	if !hasLetter(g, relativeLine(t, g, kin, "visitors from "+f.Name)) {
		t.Fatalf("no relative letter %q", relativeLine(t, g, kin, "visitors from "+f.Name))
	}
}

func TestVisitorsStartConversations(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	if !g.visitorsArrive(uint64(g.clock.Now())) {
		t.Fatal("visitors did not arrive")
	}
	v := g.visitorIDs()[0]
	c := g.homeColonists()[0]
	g.socState()
	started := false
	for i := 0; i < 4*socialPeriod && !started; i++ {
		vp, _ := g.positions.Get(v)
		if x, y, ok := g.freeLandNear(vp.X, vp.Y, 2, [][2]int{{vp.X, vp.Y}}); ok {
			g.positions.Set(c, Position{x, y})
			g.invalidateCreatures()
		}
		g.Step(1)
		started = g.soc.next[v] != 0
	}
	if !started {
		t.Fatal("the visitor never started a conversation")
	}
	if g.edgeOf(v, c) == nil && len(g.soc.adj[v]) == 0 {
		t.Fatal("no relation from the visitor's conversation")
	}
}

func TestCaravanDeathWritesHistory(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 16, Height: 12})
	ada, _ := g.SpawnNamedPawn(3, 3, "Ada")
	g.ensurePerson(ada)
	g.setWhere(ada, whereCaravan)
	g.socialOnDeath(ada, 0)
	for _, h := range g.History() {
		if h.Text == "Ada died." {
			return
		}
	}
	t.Fatalf("no death line for a caravan member: %+v", g.History())
}

func TestPawnViewDisfigured(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 16, Height: 12})
	ada, _ := g.SpawnNamedPawn(3, 3, "Ada")
	bo, _ := g.SpawnNamedPawn(5, 3, "Bo")
	g.bodies = map[entity.EntityID]*body.Body{ada: {Wounds: []body.Wound{{Part: body.Nose, Dmg: 1, Permanent: true}}}}
	g.viewGen++
	for _, p := range g.Snapshot() {
		switch p.ID {
		case ada:
			if !p.Disfigured {
				t.Fatal("a scarred nose is not disfigured in the view")
			}
		case bo:
			if p.Disfigured {
				t.Fatal("an unscarred pawn is disfigured in the view")
			}
		}
	}
}
