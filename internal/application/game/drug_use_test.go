package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// drugColony is a small map in recreation hours with one bored colonist.
func drugColony(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	scheduleAll(g, schedule.Recreation)
	id, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.joy[id] = 1000
	return g, id
}

// drugStepFed steps n ticks in chunks, keeping hunger and rest full so only
// the drugs decide what the colonist does.
func drugStepFed(g *Game, id entity.EntityID, n int) {
	for n > 0 {
		k := min(n, 100)
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(k)
		n -= k
	}
}

// hasDrugThought reports a current thought of kind k.
func hasDrugThought(g *Game, id entity.EntityID, k mood.Kind) bool {
	for _, th := range g.thoughts(id) {
		if th.Kind == k {
			return true
		}
	}
	return false
}

// addict doses id with k until hooked (deterministic: the drug stream).
func drugAddict(t *testing.T, g *Game, id entity.EntityID, k item.Kind) {
	t.Helper()
	for i := 0; i < 500 && !g.drugState(id).Addicted(k); i++ {
		g.ingestDose(id, k)
	}
	if !g.drugState(id).Addicted(k) {
		t.Fatalf("never got hooked on %v", k)
	}
}

// addiction is id's addiction to k.
func drugAddiction(g *Game, id entity.EntityID, k item.Kind) *drug.Addiction {
	s := g.drugs[id]
	for i := range s.Addictions {
		if s.Addictions[i].Drug == k {
			return &s.Addictions[i]
		}
	}
	return nil
}

func TestIngestBeerWhenBored(t *testing.T) {
	g, id := drugColony(t)
	g.SpawnStack(item.Beer, 3, 9, 2)
	drugStepFed(g, id, 2000)
	if n := g.stockOf(item.Beer); n != 2 {
		t.Fatalf("beer = %d, want 2 (one drink, then the dose gap)", n)
	}
	s := g.drugs[id]
	if s == nil || s.Alcohol <= 0 || s.Policy != drug.SocialDrinking {
		t.Fatalf("drug state %+v", s)
	}
	if g.joy[id] <= 1000 {
		t.Fatalf("joy %d: a beer is recreation", g.joy[id])
	}
	if !hasLog(g, id, "Took beer") {
		t.Fatalf("log %v", logTexts(g, id))
	}
}

// Not bored: no drink (social drinking is for low recreation).
func TestIngestNotWhenEntertained(t *testing.T) {
	g, id := drugColony(t)
	g.SpawnStack(item.Beer, 3, 9, 2)
	for i := 0; i < 20; i++ {
		g.joy[id] = recreation.Full
		drugStepFed(g, id, 100)
	}
	if n := g.stockOf(item.Beer); n != 3 || g.drugs[id] != nil {
		t.Fatalf("beer = %d, state %v: an entertained colonist doesn't drink", n, g.drugs[id])
	}
}

func TestDrugTipsyThoughtDecaysThenHangover(t *testing.T) {
	g, id := drugColony(t)
	for i := 0; i < 3; i++ {
		g.ingestDose(id, item.Beer)
	}
	if !hasDrugThought(g, id, mood.Drunk) {
		t.Fatalf("thoughts %+v: three beers make it tipsy", g.thoughts(id))
	}
	found := false
	for _, th := range g.Snapshot()[0].Thoughts {
		found = found || th.Label == mood.DefOf(mood.Drunk).Label
	}
	if !found {
		t.Fatalf("Needs tab thoughts %+v lack %q", g.Snapshot()[0].Thoughts, mood.DefOf(mood.Drunk).Label)
	}
	if len(g.drugLines(id)) == 0 {
		t.Fatal("the Health tab shows the alcohol line")
	}
	drugStepFed(g, id, 3*clock.TicksPerHour)
	if hasDrugThought(g, id, mood.Drunk) {
		t.Fatalf("alcohol %d after 3 h: no longer tipsy", g.drugs[id].Alcohol)
	}
	drugStepFed(g, id, 4*clock.TicksPerHour)
	if g.drugs[id].Alcohol != 0 || !hasDrugThought(g, id, mood.Hungover) {
		t.Fatalf("alcohol %d, thoughts %+v: sober and hungover", g.drugs[id].Alcohol, g.thoughts(id))
	}
	drugStepFed(g, id, clock.TicksPerDay)
	if hasDrugThought(g, id, mood.Hungover) {
		t.Fatal("the hangover wears off in half a day")
	}
}

func TestDrugAddictionLifecycle(t *testing.T) {
	g, id := drugColony(t)
	g.cmdSetDrugPolicy(id, uint8(drug.Unrestricted))
	drugAddict(t, g, id, item.AshleafTea)
	a := drugAddiction(g, id, item.AshleafTea)
	a.Need = 40 // two hours from withdrawal
	drugStepFed(g, id, 3*clock.TicksPerHour)
	a = drugAddiction(g, id, item.AshleafTea)
	if !a.Withdrawal || !a.Seeking || !hasDrugThought(g, id, mood.Withdrawal) {
		t.Fatalf("addiction %+v, thoughts %+v: in withdrawal", a, g.thoughts(id))
	}
	if m := g.drugs[id].FightMultiplier(); m != 3000 {
		t.Fatalf("fight multiplier %d in tea withdrawal, want 3000", m)
	}
	// The craving finds the tea at once.
	g.drugs[id].Retry = 0
	g.SpawnStack(item.AshleafTea, 2, 9, 2)
	drugStepFed(g, id, 600)
	a = drugAddiction(g, id, item.AshleafTea)
	if g.stockOf(item.AshleafTea) != 1 || a.Withdrawal || a.Seeking || hasDrugThought(g, id, mood.Withdrawal) {
		t.Fatalf("tea %d, addiction %+v: the dose relieves the craving", g.stockOf(item.AshleafTea), a)
	}
	// Ten clean days end it.
	g.deleteItem(g.itemIDs[0])
	now := int(g.clock.Now())
	jumpTo(g, now/clock.TicksPerDay+11, 0)
	drugStepFed(g, id, clock.TicksPerHour+1)
	if g.drugs[id].Addicted(item.AshleafTea) {
		t.Fatal("ten days clean: recovered")
	}
}

// A craving with its drug out of reach doesn't retry every idle recheck.
func TestDrugCravingSeekNoFlap(t *testing.T) {
	g, id := drugColony(t)
	drugAddict(t, g, id, item.AshleafTea)
	drugAddiction(g, id, item.AshleafTea).Need, drugAddiction(g, id, item.AshleafTea).Seeking = 100, true
	g.SpawnStack(item.AshleafTea, 2, 12, 4)
	walledIn(t, g, 12, 4, 12, 4)
	g.joy[id] = recreation.Full
	searches, last := 0, g.drugs[id].Retry
	for i := 0; i < 30; i++ {
		if c := kindChanges(g, id, 100); c > 2 {
			t.Fatalf("job kind changed %d times in 100 ticks: the craving flaps", c)
		}
		if r := g.drugs[id].Retry; r != last {
			searches, last = searches+1, r
		}
	}
	if searches == 0 || searches > 2 {
		t.Fatalf("%d failed searches in 3000 ticks: want one, then the retry cooldown", searches)
	}
}

// A sated addict, with the drug in reach, takes it once and stops.
func TestDrugCravingStopsWhenSated(t *testing.T) {
	g, id := drugColony(t)
	drugAddict(t, g, id, item.AshleafTea)
	a := drugAddiction(g, id, item.AshleafTea)
	a.Need, a.Seeking = 100, true
	g.joy[id] = recreation.Full
	g.SpawnStack(item.AshleafTea, 5, 9, 2)
	drugStepFed(g, id, 3000)
	if n := g.stockOf(item.AshleafTea); n != 4 {
		t.Fatalf("tea = %d, want 4: one dose sates the craving", n)
	}
}

func TestDrugPolicyNoDrugsBlocks(t *testing.T) {
	g, id := drugColony(t)
	g.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(id), Mode: uint8(drug.NoDrugs)})
	g.Step(1)
	if p := drug.Preset(g.DrugPolicy(id)); p != drug.NoDrugs {
		t.Fatalf("policy %v", p)
	}
	drugAddict(t, g, id, item.Beer)
	a := drugAddiction(g, id, item.Beer)
	a.Need, a.Seeking = 100, true
	g.SpawnStack(item.Beer, 3, 9, 2)
	for i := 0; i < 30; i++ {
		g.joy[id] = 1000
		drugStepFed(g, id, 100)
		if j, _ := g.jobs.Get(id); j.Kind == job.Ingest {
			t.Fatal("no drugs: no ingest job, craving or not")
		}
	}
	if n := g.stockOf(item.Beer); n != 3 {
		t.Fatalf("beer = %d", n)
	}
}

func TestDrugPolicyCommandValidates(t *testing.T) {
	g, id := drugColony(t)
	if p := drug.Preset(g.DrugPolicy(id)); p != drug.SocialDrinking {
		t.Fatalf("default policy %v", p)
	}
	g.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(id), Mode: 99})
	g.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: 9999, Mode: uint8(drug.Unrestricted)})
	g.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(id), Mode: uint8(drug.SocialDrinking)})
	g.Step(1)
	if g.drugs != nil && len(g.drugs) != 0 {
		t.Fatalf("drugs %v: bad or no-op policy commands leave no state", g.drugs)
	}
}

func TestDrugAbstainerNeverDrinks(t *testing.T) {
	g, _ := drugColony(t)
	id := spawnGen(t, g, 3, 3, genPawn("Prue", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Abstainer} }))
	g.cmdSetDrugPolicy(id, uint8(drug.Unrestricted))
	g.SpawnStack(item.Beer, 3, 9, 2)
	g.SpawnStack(item.AshleafTea, 3, 10, 2)
	for i := 0; i < 30; i++ {
		g.joy[id] = 1000
		drugStepFed(g, id, 100)
		if j, _ := g.jobs.Get(id); j.Kind == job.Ingest {
			t.Fatalf("the abstainer went for a drink: %+v", j)
		}
	}
	if s := g.drugs[id]; s != nil && s.LastDose != [3]uint64{} {
		t.Fatalf("abstainer state %+v", s)
	}
}

func TestDrugSaveRoundTrip(t *testing.T) {
	g, id := drugColony(t)
	g.cmdSetDrugPolicy(id, uint8(drug.Unrestricted))
	drugAddict(t, g, id, item.AshleafTea)
	g.ingestDose(id, item.Beer)
	g.ingestDose(id, item.SparkTablet)
	g.SpawnStack(item.Beer, 4, 9, 2)
	drugStepFed(g, id, 500)
	h := saveLoad(t, g)
	if !reflect.DeepEqual(g.drugs, h.drugs) {
		t.Fatalf("drug states differ\n%+v\n%+v", g.drugs[id], h.drugs[id])
	}
	if drug.Preset(h.DrugPolicy(id)) != drug.Unrestricted {
		t.Fatal("policy lost")
	}
	sameState(t, "after load", g, h)
	for i := 0; i < 4; i++ {
		drugStepFed(g, id, clock.TicksPerHour)
		drugStepFed(h, id, clock.TicksPerHour)
		if g.StateHash() != h.StateHash() {
			t.Fatalf("hour %d: loaded game diverged", i)
		}
	}
}

// Drug use is deterministic: two runs from one seed agree.
func TestDrugDeterminismSameSeed(t *testing.T) {
	run := func() uint64 {
		g, id := drugColony(t)
		g.cmdSetDrugPolicy(id, uint8(drug.Unrestricted))
		g.SpawnStack(item.Beer, 4, 9, 2)
		g.SpawnStack(item.AshleafTea, 4, 10, 5)
		g.SpawnStack(item.SparkTablet, 4, 4, 6)
		for i := 0; i < 20; i++ {
			g.joy[id] = 1000
			drugStepFed(g, id, clock.TicksPerHour/2)
		}
		if g.drugs[id] == nil {
			t.Fatal("no drug taken")
		}
		return g.StateHash()
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("hashes %x != %x", a, b)
	}
}
