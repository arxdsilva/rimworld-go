package game

import (
	"testing"

	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// letterAbout reports a letter titled title whose jump link follows focus.
func letterAbout(g *Game, title string, focus entity.EntityID) bool {
	for _, m := range g.World().Messages {
		if m.Letter && m.Title == title && m.Focus == focus {
			return true
		}
	}
	return false
}

// §13.2 Social impact: a Social-20 speaker's deep talk (and insult) lands
// harder than a Social-0 speaker's; the memory carries the scale.
func TestSocialImpactScalesInteractions(t *testing.T) {
	landed := func(level int, k social.Interaction) (int32, int16) {
		g, a, b := socialPair(t)
		g.sk.skills[a][skill.Social].Level = uint8(level)
		g.applyInteraction(a, b, k, socNow(g))
		e := g.edgeOf(b, a)
		return g.opinion(b, a), e.mem[e.dir(b)][0].Scale
	}
	hi, hs := landed(20, social.DeepTalk)
	lo, ls := landed(0, social.DeepTalk)
	if hi <= lo || hs != social.ImpactScale(social.SocialImpact(20, 1000)) || ls != social.ImpactScale(social.SocialImpact(0, 1000)) {
		t.Fatalf("deep talk: Social 20 lands %d (scale %d), Social 0 %d (scale %d)", hi, hs, lo, ls)
	}
	if want := int32(1500) * int32(hs) / 1000; hi != want {
		t.Fatalf("Social-20 deep talk %d, want %d", hi, want)
	}
	if ins20, _ := landed(20, social.Insult); ins20 >= func() int32 { v, _ := landed(0, social.Insult); return v }() {
		t.Fatalf("a skilled speaker's insult (%d) stings less than an unskilled one's", ins20)
	}
}

// Grief lines name who died: Dee's brother grieves "my sister died".
func TestGriefThoughtNamesTheRelation(t *testing.T) {
	g := New(Config{Seed: 2, Width: 40, Height: 12})
	dee, _ := g.SpawnNamedPawn(5, 5, "Dee")
	sib, _ := g.SpawnNamedPawn(30, 5, "Sib")
	g.ensurePerson(dee)
	g.ensurePerson(sib)
	withTraits(g, dee, false)
	withTraits(g, sib, true)
	mom := g.entities.Next()
	g.peopleReg().add(&person{ID: mom, Bio: g.worldBio(mom, false, 60), HasBio: true, Where: whereOffMap})
	g.setParents(dee, [2]entity.EntityID{mom, 0})
	g.setParents(sib, [2]entity.EntityID{mom, 0})
	g.dieBy(dee, health.CauseInjury, 0)
	var line string
	n := 0
	for _, th := range g.thoughts(sib) {
		if th.Kind == mood.Grief {
			line, n = th.Label, n+1
		}
	}
	if n != 1 || line != "my half-sister died" && line != "my sister died" {
		t.Fatalf("grief lines %d, last %q", n, line)
	}
}

// Breakup pressure reads a known affair and proposals the time together:
// the interaction context the game builds carries both.
func TestInteractCtxAffairAndTogether(t *testing.T) {
	g, a, b := couple(t)
	adore(g, b, a)
	woo(t, g, a, b)
	if c := g.interactCtx(a, b, socNow(g)); c.Affair || c.Together != 0 {
		t.Fatalf("new lovers: %+v", c)
	}
	advanceDays(g, 12)
	c := g.interactCtx(a, b, socNow(g))
	if c.Together != uint64(12*social.Day) {
		t.Fatalf("together %d ticks, want 12 days", c.Together)
	}
	cara, _ := g.SpawnNamedPawn(8, 5, "Cara")
	g.ensurePerson(cara)
	g.cheatedOn(a, b, cara, socNow(g))
	if c := g.interactCtx(a, b, socNow(g)); !c.Affair {
		t.Fatal("the cheated partner's context has no affair")
	}
	if c := g.interactCtx(b, a, socNow(g)); c.Affair {
		t.Fatal("the cheater's context has the affair")
	}
	ca := g.interactCtx(a, b, socNow(g))
	w := social.Weights(ca)[social.Breakup]
	ca.Affair = false
	if plain := social.Weights(ca)[social.Breakup]; w <= plain {
		t.Fatalf("affair breakup weight %d, without %d", w, plain)
	}
}

// Letters: a turned-down proposal, a social fight, a bonded animal's
// death, a recruit, each about the pawn.
func TestSocialEventsPostLetters(t *testing.T) {
	g, a, b := couple(t)
	adore(g, b, a)
	woo(t, g, a, b)
	advanceDays(g, 6)
	for range 5 {
		g.addOpinionMemory(b, a, social.MemInsulted, socNow(g)) // b has gone off a
	}
	if !g.propose(a, b, socNow(g)) || g.pairOf(a, b).Kind != social.PairLover {
		t.Fatal("set-up: the proposal was not turned down")
	}
	if !letterAbout(g, "Proposal rejected", a) {
		t.Fatal("no letter for the rejected proposal")
	}

	f, x, y := fightPair(t, 6)
	f.mind.StartFight(x, y)
	if !letterAbout(f, "Social fight", x) {
		t.Fatal("no letter for the social fight")
	}

	h, ada, dog := bondYard(t, 9)
	d, _ := h.animals.Get(dog)
	d.Bond = uint64(ada)
	h.animals.Set(dog, d)
	h.killAnimal(dog, 0)
	if !letterAbout(h, "Bonded animal died", ada) || !hasLetter(h, "has lost a bonded companion") {
		t.Fatal("no letter for the bonded animal's death")
	}
	// an unbonded pet's death stays a plain message
	k, _, cat := bondYard(t, 10)
	k.killAnimal(cat, 0)
	for _, m := range k.World().Messages {
		if m.Letter {
			t.Fatalf("an unbonded pet's death is a letter: %+v", m)
		}
	}
}
