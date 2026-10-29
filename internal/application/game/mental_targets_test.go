package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Mental breaks aimed at others (SIM-066), through the whole game.

func hasCatharsis(g *Game, id entity.EntityID) bool {
	for _, th := range g.thoughts(id) {
		if th.Kind == mood.Catharsis {
			return true
		}
	}
	return false
}

// TestInsultingSpreeInsultsAndCanStartAFight: a colonist on an insulting
// spree insults the others; over a few seeds an insult turns into a
// social fight, which ends the spree.
func TestInsultingSpreeInsultsAndCanStartAFight(t *testing.T) {
	fought := false
	for seed := uint64(1); seed <= 8 && !fought; seed++ {
		g := New(Config{Seed: seed, Width: 20, Height: 12})
		ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
		bo, _ := g.SpawnNamedPawn(9, 5, "Bo")
		cy, _ := g.SpawnNamedPawn(9, 8, "Cy")
		g.ForceBreak(ada, mentalbreak.InsultingSpree)
		insulted := false
		for i := 0; i < 250 && !fought; i++ {
			g.Step(100)
			fed(g, ada, bo, cy)
			now := uint64(g.clock.Now())
			if g.hasOpinionMemory(bo, ada, social.MemInsulted, now) || g.hasOpinionMemory(cy, ada, social.MemInsulted, now) {
				insulted = true
			}
			fought = g.mind.InFight(ada)
		}
		if !insulted && !fought {
			t.Fatalf("seed %d: the spree insulted nobody", seed)
		}
	}
	if !fought {
		t.Fatal("no insult of the spree ever started a social fight")
	}
}

// TestBerserkAttacksAndEndsWhenDowned: a berserker attacks the colonist
// next to it; colonists may attack it; downed, it calms with catharsis.
func TestBerserkAttacksAndEndsWhenDowned(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	bo, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.ForceBreak(ada, mentalbreak.Berserk)
	struck := false
	for i := 0; i < 600 && !struck; i++ {
		g.Step(1)
		fed(g, ada, bo)
		struck = g.lastStrike[ada].Target == bo
	}
	if !struck {
		t.Fatal("the berserker never attacked the colonist beside it")
	}
	if pv := snapOf(g, ada); pv.MentalBreak != "berserk" || pv.MentalTarget != bo {
		t.Fatalf("view: break %q target %d", pv.MentalBreak, pv.MentalTarget)
	}
	pos, _ := g.positions.Get(ada)
	if g.raiderAt(pos.X, pos.Y) != ada || !g.standingRaider(ada) {
		t.Fatal("a berserk colonist is an attack target")
	}
	g.hurt(ada, health.DownedInjury+10, bo)
	g.Step(1)
	if g.Breaking(ada) != "" || !hasCatharsis(g, ada) {
		t.Fatalf("downed, the berserker calms: %q", g.Breaking(ada))
	}
	if g.standingRaider(ada) || g.raiderAt(pos.X, pos.Y) == ada {
		t.Fatal("calm again, it is no attack target")
	}
}

// TestDraftedColonistAttacksBerserker: "Melee attack" a berserk colonist
// is a valid forced attack.
func TestDraftedColonistAttacksBerserker(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	bo, _ := g.SpawnNamedPawn(12, 5, "Bo")
	g.ForceBreak(ada, mentalbreak.Berserk)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(bo), Enabled: true})
	g.Step(1)
	pos, _ := g.positions.Get(ada)
	g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: uint64(bo), X: pos.X, Y: pos.Y, Order: command.OrderAttack})
	g.Step(1)
	if j, _ := g.jobs.Get(bo); j.Kind != job.Attack || j.Target != ada {
		t.Fatalf("drafted attack on the berserker: %+v", j)
	}
}

// TestMurderousRageEndsWhenTargetDies: the rage names its one target and
// ends once it is dead.
func TestMurderousRageEndsWhenTargetDies(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	bo, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.ForceBreak(ada, mentalbreak.MurderousRage)
	if g.mind.Target(ada) != bo {
		t.Fatalf("target %d, want %d", g.mind.Target(ada), bo)
	}
	struck := false
	for i := 0; i < 600 && !struck; i++ {
		g.Step(1)
		fed(g, ada, bo)
		struck = g.lastStrike[ada].Target == bo
	}
	if !struck || g.Breaking(ada) != "murderous rage" {
		t.Fatalf("the rager attacks its target: struck %v, break %q", struck, g.Breaking(ada))
	}
	g.die(bo, health.CauseInjury)
	g.Step(2)
	if g.Breaking(ada) != "" || !hasCatharsis(g, ada) {
		t.Fatalf("its target dead, the rage ends: %q", g.Breaking(ada))
	}
}

// TestMurderousRageFinishesADownedTarget: a downed target is struck
// where it lies.
func TestMurderousRageFinishesADownedTarget(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	bo, _ := g.SpawnNamedPawn(8, 5, "Bo")
	g.down(bo, health.CauseInjury)
	g.ForceBreak(ada, mentalbreak.MurderousRage)
	before := g.lastHit[bo].Tick
	hit := false
	for i := 0; i < 2000 && !hit; i++ {
		g.Step(1)
		fed(g, ada)
		hit = g.lastStrike[ada].Target == bo && g.lastHit[bo].Tick != before
	}
	if !hit {
		t.Fatal("the rager never struck its downed target")
	}
}

// TestCatatonicGoesDownIsRescuedAndFed: a catatonic colonist lies down
// (a cause rescue admits), a doctor carries it to bed and feeds it
// (SIM-063 rescue, SIM-064 feeding), and it gets up when it recovers.
func TestCatatonicGoesDownIsRescuedAndFed(t *testing.T) {
	g := New(Config{Seed: 66, Width: 24, Height: 10})
	ann, _ := g.SpawnNamedPawn(4, 5, "Ann")
	doc, _ := g.SpawnNamedPawn(12, 5, "Doc")
	onlyWork(g, doc, work.Doctor)
	scheduleAll(g, schedule.Work)
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 18, 3)
	if _, err := g.SpawnStack(item.Meal, 5, 16, 7); err != nil {
		t.Fatal(err)
	}
	g.ForceBreak(ann, mentalbreak.Catatonic)
	if v, _ := g.vital.Get(ann); v.Status != health.Downed || v.Cause != health.CauseCatatonic {
		t.Fatalf("catatonic: %+v", v)
	}
	// SIM-063: the doctor rescues it to the free bed
	if !runCare(g, 600, func() bool { return g.bedUnder(ann) == bed }, doc) {
		t.Fatal("the catatonic colonist was not rescued to a bed")
	}
	// SIM-064: hungry, it is fed where it lies
	g.SetNeeds(ann, care.FeedHungry-10, needs.Full)
	for i := 0; i < 3000 && !hasLog(g, ann, "Fed by"); i++ {
		g.Step(1)
		fed(g, doc)
	}
	if n, _ := g.need.Get(ann); n.Hunger < needs.Full*8/10 {
		t.Fatalf("the catatonic colonist was not fed: hunger %d", n.Hunger)
	}
	b := g.breaks[ann]
	b.until = uint64(g.clock.Now()) + 5
	g.breaks[ann] = b
	g.Step(10)
	if v, _ := g.vital.Get(ann); v.Status != health.Alive || g.Breaking(ann) != "" || !hasCatharsis(g, ann) {
		t.Fatalf("recovered, it gets up: %+v break %q", v, g.Breaking(ann))
	}
}

// TestBreakThresholdsFollowTraits: the minor threshold moves with the
// break-threshold stat (Steady -6, Brittle +6) and the other tiers follow
// (wiki ratios, mentalbreak.ShiftThresholds).
func TestBreakThresholdsFollowTraits(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	plain, _ := g.SpawnNamedPawn(2, 2, "Ada")
	steady := spawnGen(t, g, 3, 2, genPawn("Rain", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Steady} }))
	brittle := spawnGen(t, g, 4, 2, genPawn("Sun", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Brittle} }))
	for _, c := range []struct {
		id   entity.EntityID
		want mentalbreak.Thresholds
	}{
		{plain, mentalbreak.Thresholds{Minor: 35, Major: 20, Extreme: 5}},
		{steady, mentalbreak.Thresholds{Minor: 29, Major: 16, Extreme: 4}},
		{brittle, mentalbreak.Thresholds{Minor: 41, Major: 23, Extreme: 5}},
	} {
		if got := g.mind.Thresholds(c.id); got != c.want {
			t.Errorf("%s: thresholds %+v, want %+v", g.name(c.id), got, c.want)
		}
	}
}

// TestMentalBreakDoesNotFlap: a miserable colonist breaks, recovers, and
// does not break again while it feels catharsis.
func TestMentalBreakDoesNotFlap(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.ForceBreak(id, mentalbreak.SadWander)
	b := g.breaks[id]
	g.Step(int(b.until-uint64(g.clock.Now())) + 1)
	if g.Breaking(id) != "" || !hasCatharsis(g, id) {
		t.Fatal("the break ends with catharsis")
	}
	starts := 0
	for i := 0; i < 20*4; i++ { // most of the catharsis day, hour by hour
		g.Step(250)
		g.SetNeeds(id, needs.CriticalHunger, needs.Full)
		g.joy[id] = 0
		for _, k := range []mood.Kind{mood.SleptOutside, mood.AteRaw, mood.SleptInAwfulRoom} {
			g.remember(id, k)
		}
		if g.Breaking(id) != "" {
			starts++
		}
	}
	if starts != 0 {
		t.Fatal("broke again while still feeling catharsis")
	}
}

// snapOf is id's read model.
func snapOf(g *Game, id entity.EntityID) PawnView {
	for _, pv := range g.Snapshot() {
		if pv.ID == id {
			return pv
		}
	}
	return PawnView{}
}
