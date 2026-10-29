package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fightPair is socialPair with the pair fully fed and rested.
func fightPair(t testing.TB, seed uint64) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 20, Height: 12})
	a, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.ensurePerson(a)
	g.ensurePerson(b)
	return g, a, b
}

// memOf reports whether from holds memory k about subject.
func memOf(g *Game, from, subject entity.EntityID, k social.MemKind) bool {
	e := g.edgeOf(from, subject)
	if e == nil {
		return false
	}
	now := socNow(g)
	for _, m := range e.mem[e.dir(from)] {
		if m.Kind == k && m.Subject == social.ID(subject) && m.Expires > now {
			return true
		}
	}
	return false
}

// fightOutcome is from's fight memory of subject (ok false: none).
func fightOutcome(g *Game, from, subject entity.EntityID) (social.FightOutcome, bool) {
	switch {
	case memOf(g, from, subject, social.MemAngeringFight):
		return social.FightAngering, true
	case memOf(g, from, subject, social.MemCatharticFight):
		return social.FightCathartic, true
	}
	return 0, false
}

// runFight steps until the pair's fight is over (at most limit ticks).
func runFight(t testing.TB, g *Game, a, b entity.EntityID, limit int) int {
	t.Helper()
	for i := 0; i < limit; i++ {
		if !g.mind.InFight(a) && !g.mind.InFight(b) {
			return i
		}
		g.Step(1)
	}
	t.Fatalf("fight still on after %d ticks", limit)
	return limit
}

// §M6 chance table: insult 4%, slight 0.5%, nothing else; the recipient's
// Bloodthirsty, hunger band and drink scale it.
func TestFightChanceTableAndMultipliers(t *testing.T) {
	g, a, b := fightPair(t, 1)
	_ = a
	if c := g.fightChanceFor(b, social.Insult); c != 40 {
		t.Fatalf("insult %d‰, want 40", c)
	}
	if c := g.fightChanceFor(b, social.Slight); c != 5 {
		t.Fatalf("slight %d‰, want 5", c)
	}
	for _, k := range []social.Interaction{social.Chitchat, social.DeepTalk, social.KindWords} {
		if c := g.fightChanceFor(b, k); c != 0 {
			t.Fatalf("%v %d‰, want 0", k, c)
		}
	}
	// hunger bands: fed, hungry, very hungry, ravenous, starving
	for _, c := range []struct{ hunger, want int }{
		{needs.Full, 40}, {needs.CriticalHunger, 60}, {needs.CriticalHunger / 2, 80},
		{needs.CriticalHunger / 4, 100}, {0, 120},
	} {
		g.SetNeeds(b, c.hunger, needs.Full)
		if got := g.fightChanceFor(b, social.Insult); got != int32(c.want) {
			t.Errorf("hunger %d: %d‰, want %d", c.hunger, got, c.want)
		}
	}
	g.SetNeeds(b, needs.Full, needs.Full)
	// drink: the drug state's multiplier (alcohol band x withdrawal)
	s := g.drugState(b)
	s.Alcohol = 450
	if m := s.FightMultiplier(); m <= 1000 {
		t.Fatalf("drunk multiplier %d should raise the chance", m)
	}
	if got, want := g.fightChanceFor(b, social.Insult), social.FightChance(social.Insult, s.FightMultiplier()); got != want || got <= 40 {
		t.Fatalf("drunk %d‰, want %d", got, want)
	}
	s.Alcohol = 0
	withTraits(g, b, true, trait.Bloodthirsty)
	if c := g.fightChanceFor(b, social.Insult); c != 160 {
		t.Fatalf("bloodthirsty insult %d‰, want 160", c)
	}
	if c := g.fightChanceFor(b, social.Slight); c != 20 {
		t.Fatalf("bloodthirsty slight %d‰, want 20", c)
	}
}

// Only a slight or an insult can start a fight, and when one does both
// are in the brawl, with a danger message and a history line.
func TestFightTriggersOnlyFromSlightOrInsult(t *testing.T) {
	g, a, b := fightPair(t, 1)
	for range 3000 {
		for _, k := range []social.Interaction{social.Chitchat, social.DeepTalk, social.KindWords} {
			g.maybeSocialFight(a, b, k)
		}
	}
	if g.mind.InFight(a) || g.mind.InFight(b) {
		t.Fatal("a friendly interaction started a fight")
	}
	started := false
	for i := 0; i < 3000 && !started; i++ {
		g.maybeSocialFight(a, b, social.Insult)
		started = g.mind.InFight(b)
	}
	if !started {
		t.Fatal("3000 insults never started a fight")
	}
	for _, id := range []entity.EntityID{a, b} {
		br := g.breaks[id]
		if br.kind != mentalbreak.SocialFight {
			t.Fatalf("%d: break %v", id, br.kind)
		}
		j, _ := g.jobs.Get(id)
		other := a
		if id == a {
			other = b
		}
		if j.Kind != job.Brawl || j.Target != other {
			t.Fatalf("%d: job %v target %d, want a brawl with %d", id, j.Kind, j.Target, other)
		}
		if !socPawnView(g, id).SocialFight {
			t.Fatalf("%d: PawnView.SocialFight unset", id)
		}
	}
	var msg *MessageView
	for i := range g.messages {
		if strings.Contains(g.messages[i].Text, "are fighting!") {
			msg = &g.messages[i]
		}
	}
	if msg == nil || !msg.Danger || msg.Focus == 0 || !strings.Contains(msg.Text, "Ada") || !strings.Contains(msg.Text, "Bo") {
		t.Fatalf("fight message %+v", msg)
	}
	found := false
	for _, h := range g.history {
		if h.Kind == "fight" && len(h.Pawns) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("no fight history line")
	}
}

// A pawn that can't do violence never starts a fight, and dragged into
// one it never strikes back; the fighter it faces does.
func TestFightNonViolentNeverInitiatesNorStrikes(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	dove := spawnGen(t, g, 5, 5, genPawn("Dove", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Gentle} }))
	hawk, _ := g.SpawnNamedPawn(6, 5, "Hawk")
	g.ensurePerson(dove)
	g.ensurePerson(hawk)
	if g.canViolence(dove) {
		t.Fatal("set-up: Dove should be incapable of violence")
	}
	for range 5000 {
		g.maybeSocialFight(hawk, dove, social.Insult) // dove is the one who would start it
	}
	if g.mind.InFight(dove) {
		t.Fatal("a pacifist started a fight")
	}
	g.mind.StartFight(hawk, dove)
	hawkStruck := false
	for i := 0; i < 2000 && g.mind.InFight(hawk); i++ {
		g.Step(1)
		if _, ok := g.lastStrike[dove]; ok {
			t.Fatal("a pacifist struck back in a brawl")
		}
		if _, ok := g.lastStrike[hawk]; ok {
			hawkStruck = true
		}
	}
	if !hawkStruck {
		t.Fatal("the violent fighter never struck")
	}
}

// Brawls are fists only, whatever the fighters carry; every hit is
// "harmed me" for the victim.
func TestBrawlFistsOnlyAndHarmedMe(t *testing.T) {
	g, a, b := fightPair(t, 2)
	g.equip[a] = item.Rifle
	g.equip[b] = item.Club
	g.mind.StartFight(b, a)
	hits := map[entity.EntityID]bool{}
	strikes := 0
	for i := 0; i < 3000 && g.mind.InFight(a); i++ {
		g.Step(1)
		for _, tr := range g.traces {
			if tr.Attacker != a && tr.Attacker != b {
				continue
			}
			if tr.Weapon != weapon.DefOf(weapon.Fists).Name {
				t.Fatalf("brawl strike with %q", tr.Weapon)
			}
			if tr.Tick == socNow(g)-1 || tr.Tick == socNow(g) {
				strikes++
			}
			if tr.Hit {
				hits[tr.Target] = true
			}
		}
	}
	if strikes == 0 {
		t.Fatal("no brawl strikes")
	}
	for victim := range hits {
		other := a
		if victim == a {
			other = b
		}
		if !memOf(g, victim, other, social.MemHarmedMe) {
			t.Fatalf("%d was hit but holds no harmed-me memory of %d", victim, other)
		}
	}
	if len(hits) == 0 {
		t.Fatal("no brawl hits landed")
	}
}

// The fight lasts at least 420 ticks unless someone goes down, then ends
// on a recovery roll; ending gives no catharsis.
func TestFightMinimumDurationThenEnds(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		g, a, b := fightPair(t, seed)
		g.mind.StartFight(b, a)
		for i := 0; i < social.FightMinTicks-1; i++ {
			g.Step(1)
			if !g.standingPawn(a) || !g.standingPawn(b) {
				break
			}
			if !g.mind.InFight(a) || !g.mind.InFight(b) {
				t.Fatalf("seed %d: fight ended after %d ticks, before the minimum", seed, i+1)
			}
		}
		runFight(t, g, a, b, 20000)
		for _, id := range []entity.EntityID{a, b} {
			if _, ok := g.breaks[id]; ok {
				t.Fatalf("seed %d: break left on %d", seed, id)
			}
			for _, m := range g.memories[id] {
				if m.Kind == mood.Catharsis {
					t.Fatalf("seed %d: a social fight gave catharsis", seed)
				}
			}
			if j, _ := g.jobs.Get(id); j.Kind == job.Brawl && j.Active() {
				t.Fatalf("seed %d: brawl job left on %d", seed, id)
			}
		}
	}
}

// Each fighter rolls its own outcome: both outcomes occur across seeds,
// and the two fighters sometimes disagree.
func TestFightOutcomesBothOccurIndependently(t *testing.T) {
	seen := map[social.FightOutcome]int{}
	differ, same := 0, 0
	for seed := uint64(1); seed <= 30; seed++ {
		g, a, b := fightPair(t, seed)
		g.mind.StartFight(b, a)
		runFight(t, g, a, b, 20000)
		oa, okA := fightOutcome(g, a, b)
		ob, okB := fightOutcome(g, b, a)
		if !okA || !okB {
			t.Fatalf("seed %d: outcome memories %v %v", seed, okA, okB)
		}
		seen[oa]++
		seen[ob]++
		if oa == ob {
			same++
		} else {
			differ++
		}
	}
	if seen[social.FightAngering] == 0 || seen[social.FightCathartic] == 0 {
		t.Fatalf("outcomes %v: both should occur", seen)
	}
	if differ == 0 || same == 0 {
		t.Fatalf("outcomes not independent: %d differ, %d same", differ, same)
	}
}

// A brawler takes no orders: drafting (and moving) is rejected mid-fight.
func TestFightRejectsDrafting(t *testing.T) {
	g, a, b := fightPair(t, 1)
	g.mind.StartFight(b, a)
	g.Step(10)
	g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(a), Enabled: true})
	g.Apply(command.Command{Kind: command.MovePawn, Pawn: uint64(b), X: 1, Y: 1})
	g.Step(1)
	if g.drafted(a) {
		t.Fatal("a brawler was drafted")
	}
	if !g.mind.InFight(a) || !g.mind.InFight(b) {
		t.Fatal("an order ended the fight")
	}
	if j, _ := g.jobs.Get(b); j.Kind != job.Brawl {
		t.Fatalf("a move order replaced the brawl: %v", j.Kind)
	}
}

// No-flap: once a pair has fought, insults can't start another fight
// between them for a day; after it they can.
func TestFightPairCooldownNoFlap(t *testing.T) {
	g, a, b := fightPair(t, 4)
	withTraits(g, a, true, trait.Bloodthirsty)
	withTraits(g, b, true, trait.Bloodthirsty)
	g.mind.StartFight(b, a)
	runFight(t, g, a, b, 20000)
	for _, id := range []entity.EntityID{a, b} {
		v, _ := g.vital.Get(id)
		v.Status, v.Injury = health.Alive, 0
		g.vital.Set(id, v)
	}
	fights := 0
	end := socNow(g) + social.FightPairCooldown - 1
	for socNow(g) < end {
		g.maybeSocialFight(a, b, social.Insult)
		g.maybeSocialFight(b, a, social.Insult)
		if g.mind.InFight(a) {
			fights++
			break
		}
		g.clock.Advance(97)
	}
	if fights != 0 {
		t.Fatal("the pair fought again inside the cooldown")
	}
	g.clock.Advance(int(end-socNow(g)) + 2)
	for range 2000 {
		g.maybeSocialFight(a, b, social.Insult)
		if g.mind.InFight(a) {
			fights++
			break
		}
	}
	if fights != 1 {
		t.Fatal("past the cooldown the pair never fought again")
	}
}

// Bystanders don't treat a brawler as a threat: an armed colonist next
// to the fight neither engages nor flees, and an unarmed one keeps calm.
func TestFightBystandersStayOut(t *testing.T) {
	g, a, b := fightPair(t, 5)
	armed, _ := g.SpawnNamedPawn(7, 5, "Cy")
	unarmed, _ := g.SpawnNamedPawn(5, 6, "Di")
	g.equip[armed] = item.Club
	g.mind.StartFight(b, a)
	for i := 0; i < 600 && g.mind.InFight(a); i++ {
		g.Step(1)
		for _, id := range []entity.EntityID{armed, unarmed} {
			if j, _ := g.jobs.Get(id); j.Kind == job.Attack || j.Kind == job.Brawl {
				t.Fatalf("bystander %d joined in: %v on %d", id, j.Kind, j.Target)
			}
			if _, ok := g.calm[id]; ok {
				t.Fatalf("bystander %d fled from a brawl", id)
			}
			if _, ok := g.lastStrike[id]; ok {
				t.Fatalf("bystander %d struck", id)
			}
		}
	}
	for _, f := range []entity.EntityID{a, b} {
		if _, _, _, ok := g.combat.Guard.ThreatTo(armed, f, 7, 5, combat.AnimalRelease, combat.RaiderRelease); ok {
			t.Fatal("a brawler counts as a threat to a bystander")
		}
	}
}

// Going down ends the fight at once, for both.
func TestFightEndsWhenOneIsDowned(t *testing.T) {
	g, a, b := fightPair(t, 1)
	g.mind.StartFight(b, a)
	g.Step(5)
	g.hurt(a, downDamage(g, a), b)
	if g.mind.InFight(a) || g.mind.InFight(b) {
		t.Fatal("the fight went on with a fighter down")
	}
	if _, ok := fightOutcome(g, b, a); !ok {
		t.Fatal("the standing fighter got no outcome")
	}
}

// Save/load mid-fight continues exactly as the unbroken game.
func TestFightSaveLoadMidFightSameHash(t *testing.T) {
	mk := func() (*Game, entity.EntityID, entity.EntityID) {
		g, a, b := fightPair(t, 6)
		g.positions.Set(b, Position{11, 5}) // they walk to each other first
		g.mind.StartFight(b, a)
		return g, a, b
	}
	straight, _, _ := mk()
	straight.Step(800)
	g, a, _ := mk()
	g.Step(150)
	if !g.mind.InFight(a) {
		t.Fatal("set-up: fight already over")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "fight"); err != nil {
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
	if !l.mind.InFight(a) {
		t.Fatal("the fight did not survive the load")
	}
	l.Step(650)
	if l.StateHash() != straight.StateHash() {
		t.Fatal("save/load mid-fight diverged from the unbroken run")
	}
}

// Brawl hit rolls never move the main stream.
func TestFightRNGIsolation(t *testing.T) {
	g, a, b := fightPair(t, 1)
	g.mind.StartFight(b, a)
	before, _ := g.rng.MarshalBinary()
	for i := 0; i < 200 && g.mind.InFight(a); i++ {
		g.phaseAI()
		g.phaseJobs()
	}
	after, _ := g.rng.MarshalBinary()
	if !bytes.Equal(before, after) {
		t.Fatal("a brawl moved g.rng")
	}
}
