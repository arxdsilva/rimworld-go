package game

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/health"
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
	if s := g.drugs[id]; s != nil && len(s.LastDose) != 0 {
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

// MED-07: medical drugs and overdose.

// drugPain gives id one tended torso bruise of dmg HP (pain 12.5 per HP;
// a tended bruise neither bleeds nor needs a doctor).
func drugPain(g *Game, id entity.EntityID, dmg int) *body.Body {
	v, _ := g.vital.Get(id)
	b := g.ensureBody(id, &v)
	b.Wounds = []body.Wound{{Part: body.Torso, Dmg: dmg, Kind: body.Bruise, Tended: true, TendQ: 500}}
	g.bodyChanged(id, b, &v)
	return b
}

// TestOverdoseStagesKill: spark tablets pile up an overdose through the
// stages (major costs consciousness, extreme downs the colonist) until a
// dose takes it to 1000 and kills it, of an overdose.
func TestOverdoseStagesKill(t *testing.T) {
	g, id := drugColony(t)
	seen := map[int]bool{}
	for i := 0; i < 10; i++ {
		if g.ingestDose(id, item.SparkTablet) {
			break
		}
		b := g.bodies[id]
		sev := b.Overdose()
		if sev < 180*int32(i+1) || sev > 350*int32(i+1) {
			t.Fatalf("dose %d: overdose %d outside 180..350 a dose", i+1, sev)
		}
		st := int(b.OverdoseStage())
		seen[st] = true
		v, _ := g.vital.Get(id)
		switch st {
		case body.OverdoseStageMajor:
			if b.Capacity(body.Consciousness) != 500 || v.Status != health.Alive {
				t.Fatalf("major: consciousness %d status %v", b.Capacity(body.Consciousness), v.Status)
			}
		case body.OverdoseStageExtreme:
			if b.Capacity(body.Consciousness) != 100 || v.Status != health.Downed {
				t.Fatalf("extreme: consciousness %d status %v", b.Capacity(body.Consciousness), v.Status)
			}
		}
	}
	v, _ := g.vital.Get(id)
	if v.Status != health.Dead || v.Cause != health.CauseOverdose || v.Cause.String() != "overdose" {
		t.Fatalf("status %v cause %v: the last dose kills", v.Status, v.Cause)
	}
	if !seen[body.OverdoseStageMajor] && !seen[body.OverdoseStageExtreme] {
		t.Fatalf("stages seen %v", seen)
	}
}

// TestOverdoseExtremeMayKill: left at the extreme stage, the hourly roll
// (mean three days) kills some colonists before it wears off, not all.
func TestOverdoseExtremeMayKill(t *testing.T) {
	deaths := 0
	for seed := uint64(1); seed <= 40; seed++ {
		g := New(Config{Seed: seed, Width: 16, Height: 8})
		id, _ := g.SpawnNamedPawn(2, 2, "Ada")
		g.drugState(id)
		v, _ := g.vital.Get(id)
		b := g.ensureBody(id, &v)
		b.AddOverdose(999, uint64(g.clock.Now()))
		for h := 0; h < 30; h++ {
			g.drugTick(id, uint64(h+1)*clock.TicksPerHour-uint64(id)*drugTickStagger%clock.TicksPerHour)
		}
		if v, _ := g.vital.Get(id); v.Status == health.Dead {
			if v.Cause != health.CauseOverdose {
				t.Fatalf("cause %v", v.Cause)
			}
			deaths++
		}
	}
	// six hours at the extreme stage at 1/72 an hour: about 8 %
	if deaths == 0 || deaths > 12 {
		t.Fatalf("%d of 40 died at the extreme stage", deaths)
	}
}

// TestOverdoseDecays: an overdose falls 1000 a day; the colonist stands up
// once it leaves the extreme stage and is clear of it within a day.
func TestOverdoseDecays(t *testing.T) {
	g, id := drugColony(t)
	g.ingestDose(id, item.ProphylTablet)
	sev := g.bodies[id].Overdose()
	if sev < 80 || sev > 140 {
		t.Fatalf("prophyl overdose %d, want 80..140", sev)
	}
	if pv := g.Snapshot()[0]; pv.OverdosePct != int(sev)/10 || pv.OverdoseStage != 0 {
		t.Fatalf("view %d %d", pv.OverdosePct, pv.OverdoseStage)
	}
	drugStepFed(g, id, 4*clock.TicksPerHour)
	if got := g.bodies[id].Overdose(); got >= sev || got < sev-5*42 {
		t.Fatalf("after 4 h: %d from %d (about 42 an hour)", got, sev)
	}
	drugStepFed(g, id, 4*clock.TicksPerHour)
	if b := g.bodies[id]; b != nil && b.Overdose() != 0 {
		t.Fatalf("after 8 h: %d left", b.Overdose())
	}

	// extreme: downed, then up again as it wears off
	g2, id2 := drugColony(t)
	v, _ := g2.vital.Get(id2)
	b := g2.ensureBody(id2, &v)
	b.AddOverdose(800, uint64(g2.clock.Now()))
	g2.bodyChanged(id2, b, &v)
	g2.updateDowned(id2, b, &v)
	g2.drugState(id2)
	if v, _ := g2.vital.Get(id2); v.Status != health.Downed {
		t.Fatalf("extreme overdose downs: %v", v.Status)
	}
	drugStepFed(g2, id2, 4*clock.TicksPerHour)
	v, _ = g2.vital.Get(id2)
	if v.Status == health.Dead {
		t.Skip("the extreme roll killed (seed-dependent)")
	}
	drugStepFed(g2, id2, 2*clock.TicksPerHour)
	if v, _ := g2.vital.Get(id2); v.Status != health.Alive || g2.bodies[id2].OverdoseStage() != body.OverdoseStageMajor {
		t.Fatalf("after 6 h: status %v stage %d", v.Status, g2.bodies[id2].OverdoseStage())
	}
}

// TestPainkillerHalvesPain: a numbroot dose halves the Health tab's pain
// for 12 hours and shows as a drug line and a condition row.
func TestPainkillerHalvesPain(t *testing.T) {
	g, id := drugColony(t)
	drugPain(g, id, 36) // pain 450
	hv, _ := g.Health(id)
	if hv.Pain != 45 {
		t.Fatalf("pain %d%%", hv.Pain)
	}
	g.ingestDose(id, item.NumbrootTincture)
	hv, _ = g.Health(id)
	if hv.Pain != 22 {
		t.Fatalf("numbed pain %d%%, want 22", hv.Pain)
	}
	line, row := false, false
	for _, l := range hv.Drugs {
		line = line || l.Label == "numbroot: pain halved" && l.Pct == 100
	}
	for _, c := range hv.Conditions {
		row = row || c.Label == "Numbroot (pain halved)"
	}
	if !line || !row {
		t.Fatalf("drug lines %+v conditions %+v", hv.Drugs, hv.Conditions)
	}
	for h := 0; h < 13; h++ {
		drugPain(g, id, 36) // hold the wound against healing
		drugStepFed(g, id, clock.TicksPerHour)
	}
	if hv, _ = g.Health(id); hv.Pain != 45 || g.bodies[id].Numbed() {
		t.Fatalf("after 13 h: pain %d%%, numbed %v", hv.Pain, g.bodies[id].Numbed())
	}
}

// TestPainkillerUseDoesNotFlap: held at pain 450 with numbroot in reach,
// a colonist takes a dose, then none while it acts, then one again once
// it wore off: about one per 12 hours, and few job changes around the
// moment it wears off.
func TestPainkillerUseDoesNotFlap(t *testing.T) {
	g, id := drugColony(t)
	if _, err := g.SpawnStack(item.NumbrootTincture, 10, 9, 2); err != nil {
		t.Fatal(err)
	}
	drugPain(g, id, 36)
	for h := 0; h < 48; h++ {
		drugPain(g, id, 36)
		drugStepFed(g, id, clock.TicksPerHour)
	}
	took := 10 - g.stockOf(item.NumbrootTincture)
	if took < 3 || took > 4 {
		t.Fatalf("%d doses in 48 h, want one per 12 h", took)
	}
	// pain just under the line once the dose wears off: none taken
	g2, id2 := drugColony(t)
	g2.SpawnStack(item.NumbrootTincture, 10, 9, 2)
	drugPain(g2, id2, 31) // pain 387
	g2.SetNeeds(id2, needs.Full, needs.Full)
	if n := kindChanges(g2, id2, 600); n > 4 {
		t.Fatalf("job kind changed %d times under the pain line", n)
	}
	if g2.stockOf(item.NumbrootTincture) != 10 {
		t.Fatal("no dose under the pain line")
	}
	// around the wear-off moment
	drugPain(g, id, 36)
	b := g.bodies[id]
	if c, ok := b.Cond(body.CondPainkiller); ok {
		c.Since = uint64(g.clock.Now()) - body.PainkillerHours*clock.TicksPerHour + 150
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	if n := kindChanges(g, id, 600); n > 6 {
		t.Fatalf("job kind changed %d times around the wear-off", n)
	}
}

// TestProphylBlocksMalaria: a colonist on prophyl is left out of a
// malaria, plague or sleeping sickness outbreak (not flu) until five days
// after the dose.
func TestProphylBlocksMalaria(t *testing.T) {
	g, id := drugColony(t)
	other, _ := g.SpawnNamedPawn(4, 2, "Bo")
	g.ingestDose(id, item.ProphylTablet)
	for _, k := range []body.CondKind{body.CondMalaria, body.CondPlague, body.CondSleepingSickness} {
		if c := g.diseaseCandidates(k); len(c) != 1 || c[0] != other {
			t.Fatalf("%v candidates %v: the protected colonist is left out", k, c)
		}
	}
	if c := g.diseaseCandidates(body.CondFlu); len(c) != 2 {
		t.Fatalf("flu candidates %v: prophyl does not stop flu", c)
	}
	if !g.diseaseOutbreak(incident.Malaria, uint64(g.clock.Now())) {
		t.Fatal("outbreak")
	}
	if _, has := g.bodies[id].Cond(body.CondMalaria); has {
		t.Fatal("the protected colonist caught malaria")
	}
	if _, has := g.bodies[other].Cond(body.CondMalaria); !has {
		t.Fatal("the other colonist caught it")
	}
	lines := g.drugLines(id)
	if len(lines) != 1 || lines[0].Label != "protected by prophyl" {
		t.Fatalf("drug lines %+v", lines)
	}
	g.clock.Advance(5 * clock.TicksPerDay)
	if c := g.diseaseCandidates(body.CondMalaria); len(c) != 1 || c[0] != id {
		t.Fatalf("after five days: candidates %v", c)
	}
}

// TestProphylRetakenEveryFiveDays: under the default policy a colonist
// takes a prophyl tablet at once, none again before five days, then one.
func TestProphylRetakenEveryFiveDays(t *testing.T) {
	g, id := drugColony(t)
	if _, err := g.SpawnStack(item.ProphylTablet, 5, 9, 2); err != nil {
		t.Fatal(err)
	}
	drugStepFed(g, id, 2*clock.TicksPerHour)
	if n := g.stockOf(item.ProphylTablet); n != 4 {
		t.Fatalf("stock %d after 2 h: one taken at once", n)
	}
	first := g.drugs[id].LastDoseOf(item.ProphylTablet)
	if !g.drugs[id].Protected(uint64(g.clock.Now())) {
		t.Fatal("protected")
	}
	g.clock.Advance(int(first + 5*clock.TicksPerDay - 4*clock.TicksPerHour - uint64(g.clock.Now())))
	drugStepFed(g, id, 3*clock.TicksPerHour)
	if n := g.stockOf(item.ProphylTablet); n != 4 {
		t.Fatalf("stock %d before five days: none retaken", n)
	}
	drugStepFed(g, id, 4*clock.TicksPerHour)
	if n := g.stockOf(item.ProphylTablet); n != 3 {
		t.Fatalf("stock %d after five days: retaken", n)
	}
	if last := g.drugs[id].LastDoseOf(item.ProphylTablet); last < first+5*clock.TicksPerDay {
		t.Fatalf("second dose at %d, first %d", last, first)
	}
	// "no drugs": none, ever
	g.cmdSetDrugPolicy(id, uint8(drug.NoDrugs))
	g.clock.Advance(6 * clock.TicksPerDay)
	drugStepFed(g, id, 3*clock.TicksPerHour)
	if n := g.stockOf(item.ProphylTablet); n != 3 {
		t.Fatalf("stock %d: no drugs means none", n)
	}
}

// legacyDrugState is a drug state as saves up to version 5 wrote it: the
// per-drug arrays sized for the three original drugs.
type legacyDrugState struct {
	Alcohol   int32
	Tolerance [drug.LegacyDrugs]int32
	LastDose  [drug.LegacyDrugs]uint64
	Policy    drug.Preset
}

// TestLegacyDrugSaveLoadsWithMedicalDrugs: an older save's tolerance and
// last doses load into the slices, the new drugs read zero, and prophyl
// is due at once.
func TestLegacyDrugSaveLoadsWithMedicalDrugs(t *testing.T) {
	var buf bytes.Buffer
	old := legacyDrugState{Alcohol: 100, Tolerance: [3]int32{0, 300, 50}, LastDose: [3]uint64{7, 0, 9}, Policy: drug.Unrestricted}
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	var sd savedDrug
	if err := gob.NewDecoder(&buf).Decode(&sd); err != nil {
		t.Fatal(err)
	}
	g, id := drugColony(t)
	g.importDrugs([]entry[savedDrug]{{id, sd}})
	s := g.drugs[id]
	if s.ToleranceOf(item.AshleafTea) != 300 || s.ToleranceOf(item.SparkTablet) != 50 || s.LastDoseOf(item.Beer) != 7 ||
		s.LastDoseOf(item.SparkTablet) != 9 || s.Policy != drug.Unrestricted {
		t.Fatalf("legacy state %+v", s)
	}
	if s.ToleranceOf(item.ProphylTablet) != 0 || s.LastDoseOf(item.NumbrootTincture) != 0 || s.Protected(0) {
		t.Fatal("the new drugs start untouched")
	}
	if !s.Policy.AllowsMedical(item.ProphylTablet, 0, false, s, uint64(g.clock.Now())) {
		t.Fatal("prophyl due at once")
	}
	g.ingestDose(id, item.ProphylTablet)
	if out := g.exportDrugs(); len(out) != 1 || out[0].V.LastDoses[4] == 0 {
		t.Fatalf("the new drug's slot saves: %+v", out)
	}
}
