package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// socialPair is a small sandbox with two registered colonists side by side.
func socialPair(t testing.TB) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	a, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.ensurePerson(a)
	g.ensurePerson(b)
	return g, a, b
}

// withTraits gives id the traits (and sex) for opinion tests.
func withTraits(g *Game, id entity.EntityID, male bool, ts ...trait.ID) {
	p := g.sk.bios[id]
	p.Traits = ts
	p.Look.Sex = look.Female
	if male {
		p.Look.Sex = look.Male
	}
	g.sk.bios[id] = p
	g.socState().relVer++ // traits changed: tests only
}

func socNow(g *Game) uint64 { return uint64(g.clock.Now()) }

func socPoints(g *Game, from, to entity.EntityID) int { return centiPoints(g.opinion(from, to)) }

// §21: opinions are directional; an insult is -15 opinion and -5 mood,
// and both end with their memories.
func TestSocialOpinionDirectionalAndInsultExpires(t *testing.T) {
	g, a, b := socialPair(t)
	insulted := func() int {
		v := 0
		for _, th := range g.thoughts(b) {
			if th.Kind == mood.Insulted {
				v += th.Value
			}
		}
		return v
	}
	g.applyInteraction(a, b, social.Insult, socNow(g))
	if got := socPoints(g, b, a); got != -15 {
		t.Fatalf("B of A after an insult = %d, want -15", got)
	}
	if got := socPoints(g, a, b); got != 0 {
		t.Fatalf("A of B = %d, want 0 (opinions are one way)", got)
	}
	if got := insulted(); got != -5 {
		t.Fatalf("insulted mood = %d, want -5", got)
	}
	g.clock.Advance(int(20 * clock.TicksPerDay))
	if got := socPoints(g, b, a); got != 0 {
		t.Fatalf("insult outlived its memory: %d", got)
	}
	if got := insulted(); got != 0 {
		t.Fatalf("insult mood outlived its memory: %d", got)
	}
}

// §21: stacking diminishes by 0.9 per copy; deep talk is +15 both ways.
func TestSocialStackingAndDeepTalk(t *testing.T) {
	g, a, b := socialPair(t)
	g.applyInteraction(a, b, social.Insult, socNow(g))
	g.applyInteraction(a, b, social.Insult, socNow(g))
	if got := g.opinion(b, a); got != -1500-1350 {
		t.Fatalf("two insults = %d centi, want %d", got, -1500-1350)
	}
	g2, c, d := socialPair(t)
	g2.applyInteraction(c, d, social.DeepTalk, socNow(g2))
	if got, back := socPoints(g2, d, c), socPoints(g2, c, d); got != 15 || back != 15 {
		t.Fatalf("deep talk = %d / %d, want 15 both ways", got, back)
	}
}

// §21: a Fair pawn earns +20 from an eligible observer; Gracious ignores
// looks.
func TestSocialBeautyAndGracious(t *testing.T) {
	g, a, b := socialPair(t)
	withTraits(g, b, false, trait.Fair)
	if got := socPoints(g, a, b); got != 20 {
		t.Fatalf("opinion of a Fair pawn = %d, want 20", got)
	}
	withTraits(g, a, true, trait.Gracious)
	if got := socPoints(g, a, b); got != 0 {
		t.Fatalf("Gracious opinion of a Fair pawn = %d, want 0", got)
	}
}

// §21: siblings start at +20 and can still end up negative.
func TestSocialSiblingsCanFallOut(t *testing.T) {
	g, a, b := socialPair(t)
	reg := g.peopleReg()
	mom, dad := g.entities.Next(), g.entities.Next()
	reg.add(&person{ID: mom, Bio: g.worldBio(mom, false, 60), HasBio: true, Where: whereOffMap})
	reg.add(&person{ID: dad, Bio: g.worldBio(dad, true, 62), HasBio: true, Where: whereOffMap})
	g.setParents(a, [2]entity.EntityID{mom, dad})
	g.setParents(b, [2]entity.EntityID{mom, dad})
	if got := socPoints(g, a, b); got != 20 {
		t.Fatalf("sibling opinion = %d, want 20", got)
	}
	for range 3 {
		g.applyInteraction(b, a, social.Insult, socNow(g))
	}
	if got := socPoints(g, a, b); got >= 0 {
		t.Fatalf("sibling insulted three times still at %d", got)
	}
	v, ok := g.socialView(a)
	if !ok || len(v.Relations) != 3 || v.Relations[2].Other != uint64(b) || v.Relations[2].OpinionTo >= 0 {
		t.Fatalf("social view rows (parents +30 first, then the sibling) = %+v", v.Relations)
	}
	var parents int
	for _, r := range v.Relations {
		if r.Status == "off-map" && (r.Label == "mother" || r.Label == "father") {
			parents++
		}
	}
	if parents != 2 {
		t.Fatalf("view shows %d off-map parents, want 2: %+v", parents, v.Relations)
	}
}

// §21 no-flap: the friend and rival flags enter at +-20 and leave only
// past +-15, so an opinion wobbling around the threshold flips them once.
func TestSocialFriendRivalHysteresisNoFlap(t *testing.T) {
	var friend, rival bool
	fChanges, rChanges := 0, 0
	for i := 0; i < 400; i++ {
		o := int32(1700 + (i%7)*100) // 17..23 points
		if f := friendBand(friend, o); f != friend {
			friend, fChanges = f, fChanges+1
		}
		if r := rivalBand(rival, -o); r != rival {
			rival, rChanges = r, rChanges+1
		}
	}
	if fChanges != 1 || rChanges != 1 || !friend || !rival {
		t.Fatalf("flags flapped: friend %d changes (%v), rival %d (%v)", fChanges, friend, rChanges, rival)
	}
	if friendBand(true, 1499) || !friendBand(true, 1500) || friendBand(false, 1999) {
		t.Fatal("friend band edges wrong")
	}
	if rivalBand(true, -1499) || !rivalBand(true, -1500) || rivalBand(false, -1999) {
		t.Fatal("rival band edges wrong")
	}
	// on an edge: deep talk +15 and a Fair look +20 make a friend; slights
	// down into the 15..20 band keep it, an insult below 15 drops it.
	g, a, b := socialPair(t)
	withTraits(g, b, false, trait.Fair)
	g.applyInteraction(b, a, social.DeepTalk, socNow(g))
	if !g.friendOf(a, b) {
		t.Fatalf("no friend flag at %d", socPoints(g, a, b))
	}
	for range 4 { // 30, 25.5, 21.45, 17.8: inside the band, still a friend
		g.applyInteraction(b, a, social.Slight, socNow(g))
	}
	if o := g.opinion(a, b); o >= social.FriendEnter || o < social.FriendExit || !g.friendOf(a, b) {
		t.Fatalf("friend flag dropped inside the band at %d", o)
	}
	g.applyInteraction(b, a, social.Insult, socNow(g)) // 2.8
	if g.friendOf(a, b) {
		t.Fatalf("friend flag kept at %d", socPoints(g, a, b))
	}
}

// §20.18: pawns a wall separates never interact; the same pair without the
// wall do.
func TestSocialSeparatedPawnsNeverInteract(t *testing.T) {
	run := func(wall bool) int {
		g := New(Config{Seed: 3, Width: 20, Height: 12})
		if wall {
			for y := 0; y < 12; y++ {
				buildNow(t, g, construction.Wall, item.Wood, construction.South, 10, y)
			}
		}
		a, _ := g.SpawnNamedPawn(8, 5, "Ada")
		b, _ := g.SpawnNamedPawn(12, 5, "Bo")
		for range 40 {
			g.SetNeeds(a, needs.Full, needs.Full)
			g.SetNeeds(b, needs.Full, needs.Full)
			g.Step(100)
		}
		if g.soc == nil {
			return 0
		}
		return len(g.soc.logs[a]) + len(g.soc.logs[b])
	}
	if n := run(true); n != 0 {
		t.Fatalf("walled-off pawns logged %d interactions", n)
	}
	if n := run(false); n == 0 {
		t.Fatal("neighbours never interacted")
	}
}

// Interactions come no faster than the cooldown, and log prose.
func TestSocialInteractionCadence(t *testing.T) {
	g, a, b := socialPair(t)
	for range 50 {
		g.SetNeeds(a, needs.Full, needs.Full)
		g.SetNeeds(b, needs.Full, needs.Full)
		g.Step(100)
	}
	l := g.soc.logs[a]
	if len(l) == 0 {
		t.Fatal("no interactions in 5000 ticks")
	}
	var last [2]uint64
	for _, e := range l {
		i := 0
		if e.A == b {
			i = 1
		}
		if last[i] != 0 && e.Tick-last[i] < interactCooldown-chatBonus {
			t.Fatalf("pawn %d interacted again after %d ticks", e.A, e.Tick-last[i])
		}
		last[i] = e.Tick
	}
	v, _ := g.socialView(a)
	if len(v.Log) == 0 || v.Log[0].Text == "" || v.Log[0].Icon == "" {
		t.Fatalf("log view = %+v", v.Log)
	}
	if pv := socPawnView(g, a); pv.Interaction == "" || pv.InteractTick == 0 {
		t.Fatalf("pawn view bubble = %q at %d", pv.Interaction, pv.InteractTick)
	}
}

func socPawnView(g *Game, id entity.EntityID) PawnView {
	for _, p := range g.Snapshot() {
		if p.ID == id {
			return p
		}
	}
	return PawnView{}
}

// Social rolls come from the social stream: the main stream is untouched.
func TestSocialRNGIsolation(t *testing.T) {
	g, a, b := socialPair(t)
	before, _ := g.rng.MarshalBinary()
	for range 20 {
		g.interact(a, b, socNow(g))
		g.interact(b, a, socNow(g))
	}
	after, _ := g.rng.MarshalBinary()
	if !bytes.Equal(before, after) {
		t.Fatal("social rolls moved g.rng")
	}
}

// grief walks ties, not proximity: a sibling across the map (and one in a
// caravan) grieves, a stranger beside the body does not; the killer is
// hated; the cold-hearted feel nothing.
func TestSocialGriefFollowsTies(t *testing.T) {
	g := New(Config{Seed: 2, Width: 40, Height: 12})
	dead, _ := g.SpawnNamedPawn(5, 5, "Dee")
	sib, _ := g.SpawnNamedPawn(35, 5, "Sib")
	away, _ := g.SpawnNamedPawn(35, 8, "Away")
	cold, _ := g.SpawnNamedPawn(34, 8, "Cold")
	stranger, _ := g.SpawnNamedPawn(6, 5, "Stan")
	killer, _ := g.SpawnNamedPawn(7, 5, "Kil")
	for _, id := range []entity.EntityID{dead, sib, away, cold, stranger, killer} {
		g.ensurePerson(id)
	}
	withTraits(g, cold, true, trait.ColdHearted)
	mom, dad := g.entities.Next(), g.entities.Next()
	g.peopleReg().add(&person{ID: mom, Bio: g.worldBio(mom, false, 60), HasBio: true, Where: whereOffMap})
	g.peopleReg().add(&person{ID: dad, Bio: g.worldBio(dad, true, 60), HasBio: true, Where: whereDead})
	for _, id := range []entity.EntityID{dead, sib, away, cold} {
		g.setParents(id, [2]entity.EntityID{mom, dad})
	}
	g.setWhere(away, whereCaravan)
	g.dieBy(dead, health.CauseInjury, killer)

	grief := func(id entity.EntityID) int {
		for _, m := range g.memories[id] {
			if m.Kind == mood.Grief && m.Subject == uint64(dead) && m.Expires > socNow(g) {
				return m.Value
			}
		}
		return 0
	}
	if grief(sib) != -14 || grief(away) != -14 {
		t.Fatalf("sibling grief %d, caravan sibling %d; want -14", grief(sib), grief(away))
	}
	if grief(stranger) != 0 || grief(cold) != 0 {
		t.Fatalf("stranger grief %d, cold-hearted %d; want none", grief(stranger), grief(cold))
	}
	if got := socPoints(g, sib, killer); got != -80 {
		t.Fatalf("sibling's opinion of the killer = %d, want -80", got)
	}
	if got := socPoints(g, stranger, killer); got != 0 {
		t.Fatalf("stranger's opinion of the killer = %d", got)
	}
	g.clock.Advance(int(31 * clock.TicksPerDay))
	if grief(sib) != 0 {
		t.Fatal("grief outlived 30 days")
	}
	v, _ := g.socialView(sib)
	found := false
	for _, r := range v.Relations {
		if r.Other == uint64(dead) {
			found = r.Status == "dead" && r.Label == "sister" || r.Status == "dead" && r.Label == "brother"
		}
	}
	if !found {
		t.Fatalf("dead sibling not shown as dead: %+v", v.Relations)
	}
	if h := g.historyViews(); len(h) == 0 || h[len(h)-1].Kind != "death" {
		t.Fatalf("history = %+v", h)
	}
}

// A missing colonist is grieved by the lost table; its return ends it.
func TestSocialLostThenReturned(t *testing.T) {
	g, a, b := socialPair(t)
	mom := g.entities.Next()
	g.peopleReg().add(&person{ID: mom, Bio: g.worldBio(mom, false, 60), HasBio: true, Where: whereOffMap})
	g.setParents(b, [2]entity.EntityID{mom, 0})
	g.setParents(a, [2]entity.EntityID{mom, 0})
	g.setWhere(b, whereKidnapped)
	g.socialOnLost(b)
	has := func() bool {
		for _, m := range g.memories[a] {
			if m.Kind == mood.Grief && m.Subject == uint64(b) {
				return true
			}
		}
		return false
	}
	if !has() {
		t.Fatal("no grief for a missing half-sibling")
	}
	g.setWhere(b, whereColony)
	g.socialOnReturned(b)
	if has() {
		t.Fatal("grief kept after the return")
	}
}

// harm, rescue and botched surgery leave their memories.
func TestSocialHarmRescueSurgeryMemories(t *testing.T) {
	g, a, b := socialPair(t)
	g.onRescued(a, b)
	if got := socPoints(g, b, a); got != 25 {
		t.Fatalf("stranger rescue = %d, want 25", got)
	}
	g2, c, d := socialPair(t)
	g2.applyInteraction(c, d, social.Chitchat, socNow(g2))
	g2.onRescued(c, d)
	if got := g2.opinion(d, c); got != 66+1500 {
		t.Fatalf("rescue by an acquaintance = %d centi, want %d", got, 66+1500)
	}
	g.onHarmed(a, b)
	if got := socPoints(g, a, b); got != -15 {
		t.Fatalf("harmed me = %d, want -15", got)
	}
	g.socialBotchedSurgery(a, b)
	if got := socPoints(g, b, a); got != 25-20 {
		t.Fatalf("botched surgery = %d, want 5", got)
	}
}

// A raider is a registered person with a full bio.
func TestSocialRaiderPerson(t *testing.T) {
	g := New(Config{Seed: 4, Width: 30, Height: 30})
	g.SpawnNamedPawn(15, 15, "Ada")
	g.Raid(2)
	if len(g.raiderIDs) == 0 {
		t.Skip("no raider spawned")
	}
	for _, id := range g.raiderIDs {
		p := g.personOf(id)
		if p == nil || p.Where != whereRaider || !p.HasBio || p.Bio.AgeDays == 0 {
			t.Fatalf("raider %d person = %+v", id, p)
		}
		if g.personStatus(id) != "raider" {
			t.Fatalf("raider status %q", g.personStatus(id))
		}
	}
}

// Idle chat stands both pawns together: the target waits for the
// initiator.
func TestSocialChatMirrorsTarget(t *testing.T) {
	g, a, b := socialPair(t)
	g.positions.Set(b, Position{X: 10, Y: 5})
	if !g.idleChat(a, 300) {
		t.Fatal("no chat started")
	}
	j, _ := g.jobs.Get(b)
	if j.Kind != job.Chat || j.Target != a {
		t.Fatalf("target job = %+v", j)
	}
	g.Step(5)
	if j, _ := g.jobs.Get(b); j.Target != a || j.WorkLeft <= 0 {
		t.Fatalf("target stopped waiting: %+v", j)
	}
}

// Smoothed mood climbs at most 12 points an hour toward the target and is
// what the pawn view shows, beside the target.
func TestSocialMoodSmoothing(t *testing.T) {
	g, a, _ := socialPair(t)
	g.stepMood(a)
	start := g.moodOf(a)
	for range 5 {
		g.memories[a] = g.memories[a].Add(mood.Catharsis, socNow(g))
	}
	target := mood.Level(g.thoughts(a))
	if target <= start+5 {
		t.Fatalf("setup: target %d vs %d", target, start)
	}
	g.stepMood(a)
	if got := g.moodOf(a); got > start+2 || got < start {
		t.Fatalf("one 250-tick step moved mood %d -> %d", start, got)
	}
	for range 100 {
		g.stepMood(a)
	}
	if got := g.moodOf(a); got != target {
		t.Fatalf("smoothed mood %d never reached %d", got, target)
	}
	pv := socPawnView(g, a)
	if pv.Mood != target || pv.MoodTarget != target {
		t.Fatalf("view mood %d target %d", pv.Mood, pv.MoodTarget)
	}
}

// The founders get families: off-map relatives with bios, from the social
// stream, the same for the same seed.
func TestSocialFamilySeedDeterministic(t *testing.T) {
	sig := func() (int, int, uint64) {
		g := NewColony(Config{Seed: 9, Width: 48, Height: 40})
		off := 0
		for _, id := range g.people.ids {
			if p := g.people.byID[id]; p.Where == whereOffMap || p.Where == whereDead {
				if !p.HasBio {
					t.Fatalf("off-map person %d without a bio", id)
				}
				off++
			}
		}
		return off, len(g.people.ids), g.StateHash()
	}
	o1, n1, h1 := sig()
	o2, n2, h2 := sig()
	if o1 != o2 || n1 != n2 || h1 != h2 {
		t.Fatalf("family seeding differs: %d/%d/%x vs %d/%d/%x", o1, n1, h1, o2, n2, h2)
	}
	if o1 == 0 {
		t.Fatal("no off-map relatives seeded")
	}
}

// History keeps the newest 500 lines.
func TestSocialHistoryCap(t *testing.T) {
	g, a, _ := socialPair(t)
	for range historyCap + 20 {
		g.addHistory("test", "x", a)
	}
	if h := g.historyViews(); len(h) != historyCap || h[0].Pawns[0] != uint64(a) {
		t.Fatalf("history len %d", len(h))
	}
}

// Save and load mid-run reproduce the uninterrupted run, social state and
// all (edges, flags, logs, smoothed moods, the stream).
func TestSocialSaveLoadSameHash(t *testing.T) {
	straight := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	straight.Step(6000)
	g := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	g.Step(3000)
	if g.soc == nil || len(g.soc.edges) == 0 {
		t.Fatal("no social edges after 3000 ticks")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "mid"); err != nil {
		t.Fatal(err)
	}
	h := g.StateHash()
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if l.StateHash() != h {
		t.Fatal("load changed the state hash")
	}
	for _, k := range sortedEdgeKeys(g) {
		if c1, c2 := social.Compatibility(g.streams.seed, social.ID(k.lo), social.ID(k.hi), 30, 30),
			social.Compatibility(l.streams.seed, social.ID(k.lo), social.ID(k.hi), 30, 30); c1 != c2 {
			t.Fatal("compatibility changed across save/load")
		}
	}
	l.Step(3000)
	if l.StateHash() != straight.StateHash() {
		t.Fatal("save/load mid-run diverged from the uninterrupted run")
	}
}

func sortedEdgeKeys(g *Game) []pairKey {
	var ks []pairKey
	for k := range g.soc.edges {
		ks = append(ks, k)
	}
	return ks
}

// An old save without social blocks loads; its colonists register on the
// first social pass.
func TestSocialOldSaveRegistersColonists(t *testing.T) {
	g := NewColony(Config{Seed: 6, Width: 48, Height: 40})
	g.people, g.soc = nil, nil
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if l.people != nil {
		t.Fatal("registry invented on load")
	}
	l.Step(socialPeriod)
	for _, id := range l.pawnIDs {
		if l.personOf(id) == nil {
			t.Fatalf("colonist %d unregistered after a social pass", id)
		}
	}
}
