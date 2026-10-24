package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// weddingColony is an engaged couple, a guest and a marriage spot; the
// couple has been engaged for two days.
func weddingColony(t *testing.T) (g *Game, a, b, c entity.EntityID) {
	t.Helper()
	g, a, b = couple(t)
	c, _ = g.SpawnNamedPawn(4, 8, "Cy")
	g.ensurePerson(c)
	withTraits(g, c, true)
	setAge(g, c, 40)
	scheduleAll(g, schedule.Anything)
	buildNow(t, g, construction.MarriageSpot, construction.DefaultStuff(construction.MarriageSpot), construction.South, 12, 6)
	now := socNow(g)
	g.setPair(a, b, social.Pair{Kind: social.PairFiance, Since: now - 2*social.Day, LastChange: now - 2*social.Day})
	return g, a, b, c
}

// keepFit tops up everyone's needs (so nobody leaves to eat or sleep).
func keepFit(g *Game) {
	for _, id := range g.pawnIDs {
		g.SetNeeds(id, needs.Full, needs.Full)
	}
}

func weddings(g *Game) int {
	n := 0
	for _, h := range g.History() {
		if h.Kind == "wedding" {
			n++
		}
	}
	return n
}

// weddingTicks covers the hourly check, the walk, two hours of guests, the
// ceremony and the reception, with room to spare.
const weddingTicks = 20000

// runWedding runs the wedding colony for weddingTicks; with save set it
// saves and loads once, on the first tick inside phase at.
func runWedding(t *testing.T, save bool, at social.GatherPhase) (*Game, bool) {
	t.Helper()
	g, _, _, _ := weddingColony(t)
	saved := false
	for i := range weddingTicks {
		if i%100 == 0 {
			keepFit(g)
		}
		g.Step(1)
		if save && !saved {
			if ga := g.running(); ga != nil && ga.Phase == at {
				g, saved = saveLoad(t, g), true
			}
		}
	}
	return g, saved
}

// A wedding end to end: the couple gathers at the spot, marries at the
// end of the ceremony, everyone gets the thought, then it is over.
func TestWeddingEndToEnd(t *testing.T) {
	g, a, b, c := weddingColony(t)
	sawCeremony, sawGuest := false, false
	for i := range weddingTicks {
		if i%100 == 0 {
			keepFit(g)
		}
		g.Step(1)
		if ga := g.running(); ga != nil {
			if ga.Phase == social.PhaseCeremony {
				sawCeremony = true
				if v := g.Gatherings(); len(v) != 1 || v[0].Kind != "wedding" || v[0].Phase != "ceremony" || len(v[0].Hosts) != 2 {
					t.Fatalf("gathering view %+v", v)
				}
			}
			if j, _ := g.jobs.Get(c); j.Kind == job.Gather && j.State == job.Do {
				sawGuest = true
			}
		}
		if weddings(g) == 1 && g.running() == nil {
			break
		}
	}
	if !sawCeremony || !sawGuest {
		t.Fatalf("ceremony %v, guest gathered %v", sawCeremony, sawGuest)
	}
	if weddings(g) != 1 || g.pairOf(a, b).Kind != social.PairSpouse {
		t.Fatalf("weddings %d, pair %d", weddings(g), g.pairOf(a, b).Kind)
	}
	for _, id := range []entity.EntityID{a, b} {
		if v, _ := moodOfKind(g, id, mood.GotMarried); v != 40 {
			t.Fatalf("got-married mood %d", v)
		}
	}
	if v, _ := moodOfKind(g, c, mood.AttendedWedding); v != 20 {
		t.Fatalf("attended-wedding mood %d", v)
	}
	if g.running() != nil || len(g.Gatherings()) != 0 {
		t.Fatal("the wedding never ended")
	}
	if j, _ := g.jobs.Get(c); j.Kind == job.Gather && j.Active() {
		t.Fatal("guest still gathering after the end")
	}
}

// Save/load inside each wedding phase: exactly one marriage and the same
// state as the uninterrupted run.
func TestWeddingSaveLoadEachPhase(t *testing.T) {
	straight, _ := runWedding(t, false, 0)
	want := straight.StateHash()
	if weddings(straight) != 1 {
		t.Fatalf("uninterrupted run married %d times", weddings(straight))
	}
	for _, ph := range []social.GatherPhase{social.PhaseCalling, social.PhaseUnderway, social.PhaseCeremony, social.PhaseReception} {
		g, saved := runWedding(t, true, ph)
		if !saved {
			t.Fatalf("phase %d never reached", ph)
		}
		if n := weddings(g); n != 1 {
			t.Fatalf("save in phase %d: %d marriages", ph, n)
		}
		if g.StateHash() != want {
			t.Fatalf("save in phase %d diverged from the uninterrupted run", ph)
		}
	}
}

// A drafted host calls the wedding off; the couple waits a day before the
// next try.
func TestWeddingAbortAndRetry(t *testing.T) {
	g, a, b, _ := weddingColony(t)
	lo := min(a, b)
	g.weddingCheck(lo, socNow(g))
	if g.running() == nil {
		t.Fatal("no wedding scheduled")
	}
	g.draft[a] = true
	for range gatherPeriod + 1 {
		g.Step(1)
	}
	if g.running() != nil {
		t.Fatal("drafted host: still running")
	}
	g.draft[a] = false
	g.weddingCheck(lo, socNow(g))
	if g.running() != nil {
		t.Fatal("rescheduled inside the retry day")
	}
	advanceDays(g, 1)
	g.weddingCheck(lo, socNow(g))
	if g.running() == nil {
		t.Fatal("not rescheduled after the retry day")
	}
	g.breakUp(a, b, socNow(g)+6*social.Day) // a breakup calls it off too
	g.clock.Advance(gatherPeriod - int(socNow(g)%gatherPeriod))
	g.gatherTick(socNow(g))
	if g.running() != nil {
		t.Fatal("wedding of a split couple still running")
	}
}

// marry is a no-op for spouses (a reloaded reception can't pay twice).
func TestMarryOnlyOnce(t *testing.T) {
	g, a, b, _ := weddingColony(t)
	if !g.marry(a, b, socNow(g)) || g.marry(a, b, socNow(g)) {
		t.Fatal("marry should succeed once")
	}
	if weddings(g) != 1 {
		t.Fatalf("%d wedding lines", weddings(g))
	}
	n := 0
	for _, m := range g.memories[a] {
		if m.Kind == mood.GotMarried {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d got-married memories", n)
	}
}

// A party: fit colonists in a decent mood and a party spot; attendees
// get the thought, then it ends. Two colonists are not a party.
func TestPartyThrownAndAttended(t *testing.T) {
	g, a, b, c := weddingColony(t)
	g.setPair(a, b, social.Pair{})
	if g.partyFits(socNow(g)) {
		t.Fatal("party without a party spot or campfire")
	}
	buildNow(t, g, construction.PartySpot, construction.DefaultStuff(construction.PartySpot), construction.South, 8, 8)
	keepFit(g)
	if !g.partyFits(socNow(g)) || !g.throwParty(socNow(g)) {
		t.Fatal("party not thrown")
	}
	if v := g.Gatherings(); len(v) != 1 || v[0].Kind != "party" {
		t.Fatalf("view %+v", v)
	}
	for i := range 4 * social.Hour {
		if i%100 == 0 {
			keepFit(g)
		}
		g.Step(1)
		if g.running() == nil {
			break
		}
	}
	if g.running() != nil {
		t.Fatal("party never ended")
	}
	got := 0
	for _, id := range []entity.EntityID{a, b, c} {
		if v, _ := moodOfKind(g, id, mood.AttendedParty); v == 8 {
			got++
		}
	}
	if got < 2 {
		t.Fatalf("%d of 3 got the party thought", got)
	}
	g2, _, _ := couple(t)
	buildNow(t, g2, construction.PartySpot, construction.DefaultStuff(construction.PartySpot), construction.South, 8, 8)
	if g2.partyFits(socNow(g2)) {
		t.Fatal("a party of two")
	}
}

// Surname adoption changes only Bio.Last: the wife takes the husband's,
// a same-sex couple the lower id's.
func TestMarrySurnameAdoption(t *testing.T) {
	g, a, b, c := weddingColony(t) // a female, b and c male
	setLast := func(id entity.EntityID, s string) {
		p := g.sk.bios[id]
		p.Last = s
		g.sk.bios[id] = p
	}
	setLast(a, "Reed")
	setLast(b, "Stone")
	first := g.sk.bios[a].First
	g.shareSurname(a, b)
	if g.sk.bios[a].Last != "Stone" || g.sk.bios[b].Last != "Stone" || g.sk.bios[a].First != first {
		t.Fatalf("surnames %q / %q", g.sk.bios[a].Last, g.sk.bios[b].Last)
	}
	setLast(c, "Vale")
	g.shareSurname(c, b)
	lo, hi := min(b, c), max(b, c)
	if g.sk.bios[hi].Last != g.sk.bios[lo].Last {
		t.Fatal("same-sex couple kept two names")
	}
	if g.BedForPrisoners(a) {
		t.Fatal("a pawn is not a prisoner bed")
	}
}
