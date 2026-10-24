package game

import (
	"testing"

	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// couple is a sandbox with Ada (female, 30) and Bo (male, 31) side by
// side, a few days into the game.
func couple(t testing.TB) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g, a, b := socialPair(t)
	jumpTo(g, 3, 8)
	withTraits(g, a, false)
	withTraits(g, b, true)
	setAge(g, a, 30)
	setAge(g, b, 31)
	return g, a, b
}

func setAge(g *Game, id entity.EntityID, years int) {
	p := g.sk.bios[id]
	p.AgeDays = int32(years * pawngen.DaysPerYear)
	g.sk.bios[id] = p
}

// adore makes from think the world of to (deep talks, kept fresh).
func adore(g *Game, from, to entity.EntityID) {
	for range 5 {
		g.addOpinionMemory(from, to, social.MemDeepTalk, socNow(g))
	}
}

// moodOfKind sums id's active thoughts of kind k.
func moodOfKind(g *Game, id entity.EntityID, k mood.Kind) (int, bool) {
	v, found := 0, false
	for _, th := range g.thoughts(id) {
		if th.Kind == k {
			v += th.Value
			found = true
		}
	}
	return v, found
}

func hasMem(g *Game, from, about entity.EntityID, k social.MemKind) bool {
	e := g.edgeOf(from, about)
	return e != nil && social.HasMemory(e.mem[e.dir(from)], k, social.ID(about), socNow(g))
}

func advanceDays(g *Game, d int) { g.clock.Advance(d * clock.TicksPerDay) }

// woo repeats a's romance attempt on b (waiting out each rebuff) until
// they are lovers.
func woo(t testing.TB, g *Game, a, b entity.EntityID) {
	t.Helper()
	for range 20 {
		if g.tryRomance(a, b, socNow(g)) && g.pairOf(a, b).Kind == social.PairLover {
			return
		}
		advanceDays(g, 4)
		adore(g, b, a)
	}
	t.Fatalf("no romance after 20 tries (attraction %d)", g.attractionOf(b, a))
}

// Every pair transition along the life of a couple, and the invalid ones
// refused.
func TestRomancePairTransitions(t *testing.T) {
	g, a, b := couple(t)
	adore(g, b, a)
	if g.breakUp(a, b, socNow(g)) || g.propose(a, b, socNow(g)) || g.marry(a, b, socNow(g)) {
		t.Fatal("strangers broke up, proposed or married")
	}
	woo(t, g, a, b) // None -> Lover
	if g.marry(a, b, socNow(g)) {
		t.Fatal("lovers married without an engagement")
	}
	adore(g, b, a)
	for range 10 { // Lover -> Fiance (high opinion: accepted)
		if g.pairOf(a, b).Kind == social.PairFiance {
			break
		}
		g.propose(a, b, socNow(g))
		advanceDays(g, 11)
		adore(g, b, a)
	}
	if k := g.pairOf(a, b).Kind; k != social.PairFiance {
		t.Fatalf("pair %d after proposals, want fiance", k)
	}
	if g.propose(a, b, socNow(g)) {
		t.Fatal("fiances proposed again")
	}
	if !g.marry(a, b, socNow(g)) || g.pairOf(a, b).Kind != social.PairSpouse {
		t.Fatal("fiances did not marry")
	}
	if g.marry(a, b, socNow(g)) {
		t.Fatal("married twice")
	}
	if g.breakUp(a, b, socNow(g)) {
		t.Fatal("divorced within the grace after the wedding")
	}
	advanceDays(g, 6)
	if !g.breakUp(a, b, socNow(g)) || g.pairOf(a, b).Kind != social.PairExSpouse {
		t.Fatal("Spouse -> ExSpouse failed")
	}
	if g.romanceAttraction(a, b, socNow(g)) != 0 {
		t.Fatal("ex-spouses may reconcile right after the divorce")
	}
	advanceDays(g, 31) // the divorced one forgives once the memory fades
	adore(g, b, a)
	adore(g, a, b)
	woo(t, g, a, b) // ExSpouse -> Lover
	if hs := g.History(); len(hs) == 0 {
		t.Fatal("no history lines")
	}
}

// Orientation, age and kin decide who may court whom; the Social tab
// says why not.
func TestRomanceEligibility(t *testing.T) {
	g, a, b := couple(t)
	if g.attractionOf(a, b) <= 0 || g.attractionOf(b, a) <= 0 {
		t.Fatal("a straight man and woman are not attracted")
	}
	if n := g.romanceNote(a, b); n != "" {
		t.Fatalf("note %q for an eligible pair", n)
	}
	withTraits(g, a, true) // two men
	if g.attractionOf(a, b) != 0 || g.romanceNote(a, b) != "not attracted" {
		t.Fatalf("straight men attracted: %d %q", g.attractionOf(a, b), g.romanceNote(a, b))
	}
	withTraits(g, a, true, trait.Gay)
	if g.attractionOf(a, b) <= 0 {
		t.Fatal("a gay man is not attracted to a man")
	}
	withTraits(g, a, false, trait.Bisexual)
	if g.attractionOf(a, b) <= 0 {
		t.Fatal("bisexual not attracted")
	}
	withTraits(g, a, false, trait.Asexual)
	if g.attractionOf(a, b) != 0 {
		t.Fatal("asexual attracted")
	}
	withTraits(g, a, false)
	setAge(g, b, 14)
	g.socState().relVer++
	if g.attractionOf(a, b) != 0 || g.romanceNote(a, b) != "too young" {
		t.Fatalf("a 14-year-old is courted: %q", g.romanceNote(a, b))
	}
	setAge(g, b, 31)
	reg := g.peopleReg()
	mom, dad := g.entities.Next(), g.entities.Next()
	reg.add(&person{ID: mom, Bio: g.worldBio(mom, false, 60), HasBio: true, Where: whereOffMap})
	reg.add(&person{ID: dad, Bio: g.worldBio(dad, true, 62), HasBio: true, Where: whereOffMap})
	g.setParents(a, [2]entity.EntityID{mom, dad})
	g.setParents(b, [2]entity.EntityID{mom, dad})
	if g.attractionOf(a, b) != 0 || g.romanceNote(a, b) != "family" {
		t.Fatalf("siblings attracted: %q", g.romanceNote(a, b))
	}
	g2, c, d := couple(t)
	e, _ := g2.SpawnNamedPawn(7, 5, "Cy")
	g2.ensurePerson(e)
	withTraits(g2, e, false)
	setAge(g2, e, 29)
	g2.setPair(d, e, social.Pair{Kind: social.PairLover})
	if n := g2.romanceNote(d, c); n != "already partnered" {
		t.Fatalf("note %q for a partnered pawn", n)
	}
}

// A rebuff: -10 / -15 opinion, -5 / -3 mood, and no new attempt on that
// target while it lasts.
func TestRomanceRebuffMemories(t *testing.T) {
	g, a, b := couple(t)
	withTraits(g, b, true, trait.Asexual) // always says no
	if !g.tryRomance(a, b, socNow(g)) {
		t.Fatal("attempt not resolved")
	}
	if k := g.pairOf(a, b).Kind; k != social.PairNone {
		t.Fatalf("an asexual pawn accepted: %d", k)
	}
	if o, back := socPoints(g, a, b), socPoints(g, b, a); o != -10 || back != -15 {
		t.Fatalf("opinions after a rebuff %d / %d, want -10 / -15", o, back)
	}
	if v, _ := moodOfKind(g, a, mood.RebuffedMood); v != -5 {
		t.Fatalf("rebuffed mood %d", v)
	}
	if v, _ := moodOfKind(g, b, mood.HadToRebuff); v != -3 {
		t.Fatalf("had-to-rebuff mood %d", v)
	}
	withTraits(g, b, true)
	if g.romanceAttraction(a, b, socNow(g)) != 0 || g.tryRomance(a, b, socNow(g)) {
		t.Fatal("re-attempt during the rebuff cooldown")
	}
	advanceDays(g, 3)
	if g.romanceAttraction(a, b, socNow(g)) <= 0 {
		t.Fatal("cooldown never ends")
	}
}

// An affair: the cheated partner holds it against the cheater and is
// miserable.
func TestRomanceAffair(t *testing.T) {
	g, a, b := couple(t)
	c, _ := g.SpawnNamedPawn(7, 5, "Cy")
	g.ensurePerson(c)
	withTraits(g, c, true)
	setAge(g, c, 32)
	g.setPair(a, c, social.Pair{Kind: social.PairSpouse, Since: socNow(g), LastChange: socNow(g)})
	adore(g, b, a)
	woo(t, g, a, b)
	if !hasMem(g, c, a, social.MemCheatedOnMe) {
		t.Fatal("no cheated-on memory")
	}
	if v, _ := moodOfKind(g, c, mood.CheatedOn); v != -20 {
		t.Fatalf("cheated-on mood %d, want -20", v)
	}
	if g.pairOf(a, c).Kind != social.PairSpouse {
		t.Fatal("the affair ended the marriage by itself")
	}
	if g.partnerOf(a) != c {
		t.Fatal("main partner should stay the spouse")
	}
}

// Breakups and divorces: who is dumped remembers it; the grace keeps a
// fresh couple together.
func TestRomanceBreakupAndDivorce(t *testing.T) {
	g, a, b := couple(t)
	now := socNow(g)
	g.setPair(a, b, social.Pair{Kind: social.PairLover, Since: now, LastChange: now})
	if g.breakUp(a, b, now) {
		t.Fatal("broke up inside the grace")
	}
	advanceDays(g, 5)
	if !g.breakUp(a, b, socNow(g)) || g.pairOf(a, b).Kind != social.PairExLover {
		t.Fatal("breakup failed")
	}
	if !hasMem(g, b, a, social.MemBrokeUpWithMe) {
		t.Fatal("no broke-up-with-me memory")
	}
	if v, _ := moodOfKind(g, b, mood.BrokenUpWith); v != -15 {
		t.Fatalf("dumped mood %d", v)
	}
	g2, c, d := couple(t)
	g2.setPair(c, d, social.Pair{Kind: social.PairSpouse})
	advanceDays(g2, 5)
	if !g2.breakUp(c, d, socNow(g2)) || g2.pairOf(c, d).Kind != social.PairExSpouse {
		t.Fatal("divorce failed")
	}
	if !hasMem(g2, d, c, social.MemDivorcedMe) {
		t.Fatal("no divorced-me memory")
	}
	if v, _ := moodOfKind(g2, d, mood.Divorced); v != -20 {
		t.Fatalf("divorced mood %d", v)
	}
	if got := g2.History(); len(got) == 0 || got[len(got)-1].Kind != "divorce" {
		t.Fatalf("history %+v", got)
	}
}

// No-flap: a couple that keeps trying to break up and reconcile changes
// state at most once per grace.
func TestRomanceBreakupReconcileNoFlap(t *testing.T) {
	g, a, b := couple(t)
	now := socNow(g)
	g.setPair(a, b, social.Pair{Kind: social.PairLover, Since: now, LastChange: now})
	changes, last := 0, g.pairOf(a, b).Kind
	var lastAt uint64 = now
	for h := 0; h < 30*24; h++ {
		g.clock.Advance(clock.TicksPerHour)
		adore(g, a, b)
		adore(g, b, a)
		g.breakUp(a, b, socNow(g))
		g.tryRomance(a, b, socNow(g))
		if k := g.pairOf(a, b).Kind; k != last {
			if socNow(g)-lastAt < social.BreakupGrace {
				t.Fatalf("pair flipped %d -> %d after %d ticks", last, k, socNow(g)-lastAt)
			}
			changes, last, lastAt = changes+1, k, socNow(g)
		}
	}
	if changes > 30/5+1 {
		t.Fatalf("%d pair changes in 30 days", changes)
	}
}

// A proposal: turned down (opinion below zero) with the memories and the
// ten-day cooldown; accepted by an adoring lover.
func TestRomanceProposalRejectAndCooldown(t *testing.T) {
	g, a, b := couple(t)
	g.setPair(a, b, social.Pair{Kind: social.PairLover})
	for range 4 {
		g.addOpinionMemory(b, a, social.MemInsulted, socNow(g))
	}
	if !g.propose(a, b, socNow(g)) || g.pairOf(a, b).Kind != social.PairLover {
		t.Fatal("proposal not turned down")
	}
	if !hasMem(g, a, b, social.MemRejectedMyProposal) || !hasMem(g, b, a, social.MemIRejectedProposal) {
		t.Fatal("rejection memories missing")
	}
	if v, _ := moodOfKind(g, a, mood.ProposalRejected); v != -18 {
		t.Fatalf("rejected mood %d", v)
	}
	proposals := 0
	for h := 0; h < 25*24; h++ { // no-flap: one proposal per cooldown at most
		g.clock.Advance(clock.TicksPerHour)
		if g.propose(a, b, socNow(g)) {
			proposals++
		}
	}
	if proposals > 25/10 {
		t.Fatalf("%d proposals in 25 days", proposals)
	}
	g2, c, d := couple(t)
	g2.setPair(c, d, social.Pair{Kind: social.PairLover})
	withTraits(g2, c, false, trait.Striking)
	adore(g2, d, c)
	if !g2.propose(c, d, socNow(g2)) || g2.pairOf(c, d).Kind != social.PairFiance {
		t.Fatalf("adoring lover (opinion %d) said no", socPoints(g2, d, c))
	}
}

// The interaction context carries the attraction and the partnered flag,
// so romance gets chosen; the social loop resolves it.
func TestRomanceChosenInSocialLoop(t *testing.T) {
	g, a, b := couple(t)
	withTraits(g, a, false, trait.Striking)
	withTraits(g, b, true, trait.Striking)
	adore(g, a, b)
	adore(g, b, a)
	for range 400 {
		g.interact(a, b, socNow(g))
		g.clock.Advance(100)
		if g.pairOf(a, b).Kind == social.PairLover {
			return
		}
	}
	t.Fatal("no romance in 400 interactions between adoring pawns")
}
