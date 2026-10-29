package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// raceOutcome runs disease k on the race engine from its onset, an hour
// at a time, with a tend of quality tendQ held the whole way (-1:
// untended) and normal immunity: the hour it killed (-1: it did not) and
// the hour it was beaten (-1: not within limit).
func raceOutcome(k body.CondKind, tendQ int32, limit int) (died, cured int) {
	dz, _ := body.DiseaseOf(k)
	d := body.DefOfCond(k)
	c := body.Condition{Kind: k, Sev: dz.Onset, TendQ: max(tendQ, 0)}
	for h := 1; h <= limit; h++ {
		dead, beaten := infection.StepRace(&c, d, tendQ >= 0, 1000, 1)
		switch {
		case dead:
			return h, -1
		case beaten:
			return -1, h
		}
	}
	return -1, -1
}

// TestDiseaseOutcomeCurves: each lethal disease kills when left untended
// and is beaten when well tended (herbal-level quality, 70 %), at the
// wiki's rates; untreated flu kills in about four days.
func TestDiseaseOutcomeCurves(t *testing.T) {
	for _, c := range []struct {
		kind               body.CondKind
		dieMin, dieMax     int // untended death, hours
		tendQ, cureWithinH int32
	}{
		{body.CondFlu, 90, 110, 700, 10 * 24},
		{body.CondPlague, 30, 42, 700, 10 * 24},
		{body.CondMalaria, 60, 75, 700, 10 * 24},
		{body.CondSleepingSickness, 180, 215, 700, 20 * 24},
	} {
		name := body.DefOfCond(c.kind).Label
		died, _ := raceOutcome(c.kind, -1, 30*24)
		if died < c.dieMin || died > c.dieMax {
			t.Errorf("untended %s killed after %d h, want %d-%d", name, died, c.dieMin, c.dieMax)
		}
		t.Logf("%s: untended death at %d h", name, died)
		died, cured := raceOutcome(c.kind, c.tendQ, 30*24)
		t.Logf("%s: tended at %d‰ beaten at %d h", name, c.tendQ, cured)
		if died >= 0 || cured < 0 || cured > int(c.cureWithinH) {
			t.Errorf("%s tended at %d‰: died at %d h, beaten at %d h", name, c.tendQ, died, cured)
		}
	}
}

// TestUntreatedFluKillsColonist: in the game, a colonist with the flu and
// no one to tend it dies of disease in about four days; the death clock
// and the headline show the race.
func TestUntreatedFluKillsColonist(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	g.setPriority(id, work.Doctor, 0) // no one to tend it
	if !g.GiveDisease(id, uint8(body.CondFlu)) {
		t.Fatal("give flu")
	}
	if !g.indexedPatient(id) || g.diseaseLabel(id) != "flu" {
		t.Fatalf("a sick colonist is a patient: indexed %v, label %q", g.indexedPatient(id), g.diseaseLabel(id))
	}
	hours := 0
	for ; hours < 6*24 && g.VitalOf(id).Status != health.Dead; hours++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(id, 1<<20, 1<<20)
		if hours == 48 {
			hv, _ := g.Health(id)
			if hv.State != "Flu: losing" || hv.DeathInHours <= 0 || len(hv.Conditions) != 1 || !strings.HasPrefix(hv.Conditions[0].Label, "Flu") {
				t.Fatalf("health view at 48 h: %q, death in %d h, conditions %+v", hv.State, hv.DeathInHours, hv.Conditions)
			}
		}
	}
	if v := g.VitalOf(id); v.Status != health.Dead || v.Cause != health.CauseDisease {
		t.Fatalf("untreated flu runs its course: %+v", v)
	}
	if hours < 90 || hours > 110 {
		t.Fatalf("died after %d h, want about 4 days", hours)
	}
}

// TestTendedFluIsBeaten: a doctor with medicine tends the flu once per
// 12-hour window and the patient recovers; the disease leaves the body.
func TestTendedFluIsBeaten(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(14, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	scheduleAll(g, schedule.Work)
	placeBuilt(t, g, construction.Bed, construction.South, 15, 2)
	if _, err := g.SpawnStack(item.IndustrialMedicine, 20, 6, 9); err != nil {
		t.Fatal(err)
	}
	g.GiveDisease(patient, uint8(body.CondFlu))
	for h := 0; h < 9*24 && g.diseaseLabel(patient) != ""; h++ {
		g.Step(clock.TicksPerHour)
		g.SetNeeds(patient, 1<<20, 1<<20)
		g.SetNeeds(doc, 1<<20, 1<<20)
	}
	if g.VitalOf(patient).Status == health.Dead || g.diseaseLabel(patient) != "" {
		t.Fatalf("a tended flu is beaten: %+v, %q", g.VitalOf(patient), g.diseaseLabel(patient))
	}
	if used := 20 - g.countItem(item.IndustrialMedicine); used < 3 {
		t.Fatalf("tended each window: %d doses", used)
	}
}

// TestParasitesCureByTendSum: gut worms build no immunity and never kill;
// they hurt and double hunger until the tends add up to 300 % quality,
// and the severity shown falls with each tend. Muscle parasites slow
// moving and manipulation and double the awake rest fall.
func TestParasitesCureByTendSum(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	g.setPriority(id, work.Doctor, 0)
	g.GiveDisease(id, uint8(body.CondGutWorms))
	b := g.bodies[id]
	if b.Pain() < 200 {
		t.Fatalf("gut worms hurt: pain %d", b.Pain())
	}
	if hr, rr := g.diseaseNeedRates(id, 1000, 1000); hr != 2000 || rr != 1000 {
		t.Fatalf("gut worms double hunger only: %d %d", hr, rr)
	}
	if !g.feelsSick(id) || g.sick(id) || g.infectionLosing(id) {
		t.Fatal("parasites give the sick thought but race nothing")
	}
	for i := 0; i < 10*24; i++ {
		medHour(g)
	}
	c, ok := b.Cond(body.CondGutWorms)
	if !ok || g.VitalOf(id).Status == health.Dead || c.Immunity != 0 || c.Sev != 1000 {
		t.Fatalf("untended worms neither kill, heal nor immunise: %+v %v", c, ok)
	}
	tend := func(q int32, want int) {
		t.Helper()
		if n := b.TendAll(q, uint64(g.clock.Now())); n != want {
			t.Fatalf("tend of %d changed %d, want %d", q, n, want)
		}
		medHour(g)
	}
	tend(1000, 1)
	tend(1000, 0) // within the 48-hour window: no second tend
	if c, _ := b.Cond(body.CondGutWorms); c.TendSum != 1000 || c.Sev >= 1000 {
		t.Fatalf("a full tend counts once and lowers the shown severity: %+v", c)
	}
	g.clock.Advance(48 * clock.TicksPerHour)
	tend(1000, 1)
	g.clock.Advance(48 * clock.TicksPerHour)
	tend(999, 1)
	if _, ok := b.Cond(body.CondGutWorms); !ok {
		t.Fatal("2999 is short of the cure")
	}
	g.clock.Advance(48 * clock.TicksPerHour)
	tend(1, 1)
	if _, ok := b.Cond(body.CondGutWorms); ok || g.diseaseLabel(id) != "" {
		t.Fatal("cured once the tends reach 300 %")
	}

	g.GiveDisease(id, uint8(body.CondMuscleParasites))
	if hr, rr := g.diseaseNeedRates(id, 1000, 1000); hr != 1000 || rr != 2000 {
		t.Fatalf("muscle parasites double the rest fall: %d %d", hr, rr)
	}
	if mv, mp := b.Capacity(body.Moving), b.Capacity(body.Manipulation); mv > 700 || mp > 700 {
		t.Fatalf("muscle parasites slow moving %d and manipulation %d", mv, mp)
	}
}

// diseaseGame is a wild map of biome b at day 20 with three colonists
// and disease frequency freq.
func diseaseGame(t *testing.T, b biome.ID, freq int) *Game {
	t.Helper()
	g := eventMap(t, 5, 40, 30, b)
	g.SpawnNamedPawn(18, 15, "Bo")
	g.SpawnNamedPawn(22, 15, "Cy")
	p := difficulty.Wanderer
	g.ng.setup = &Setup{Preset: p, Difficulty: difficulty.Of(p), Storyteller: "steady"}
	g.ng.setup.Difficulty.DiseaseFreq = freq
	return g
}

// pickedDiseases rolls n nature picks (forgetting the cooldowns) and
// counts each disease picked.
func pickedDiseases(g *Game, n int) map[incident.ID]int {
	now := uint64(g.clock.Now())
	out := map[incident.ID]int{}
	for i := 0; i < n; i++ {
		g.incidents.Last = nil
		if id, ok := g.pickNature(now); ok && incident.DefOf(id).Disease {
			out[id]++
		}
	}
	return out
}

// TestDiseaseFreqZeroNoIncidents: at 0 % disease frequency no disease is
// ever rolled or fired; at 100 % they are.
func TestDiseaseFreqZeroNoIncidents(t *testing.T) {
	g := diseaseGame(t, biome.TemperateForest, 0)
	if got := pickedDiseases(g, 400); len(got) != 0 {
		t.Fatalf("frequency 0 rolled %v", got)
	}
	if g.FireIncident("flu") {
		t.Fatal("frequency 0: no flu even from the dev hook")
	}
	g.ng.setup.Difficulty.DiseaseFreq = 100
	if got := pickedDiseases(g, 400); len(got) == 0 {
		t.Fatal("frequency 100 rolls diseases")
	}
}

// TestSleepingSicknessTropicalOnly: sleeping sickness is rolled in the
// tropics and never in a temperate forest.
func TestSleepingSicknessTropicalOnly(t *testing.T) {
	temperate := diseaseGame(t, biome.TemperateForest, 300)
	if got := pickedDiseases(temperate, 600); got[incident.SleepingSickness] != 0 || got[incident.Malaria] != 0 {
		t.Fatalf("temperate forest rolled %v", got)
	}
	tropic := diseaseGame(t, biome.TropicalRainforest, 300)
	if got := pickedDiseases(tropic, 600); got[incident.SleepingSickness] == 0 {
		t.Fatalf("the tropics roll sleeping sickness: %v", got)
	}
}

// TestSynthStomachBlocksGutWorms: a colonist with a synth stomach never
// catches gut worms.
func TestSynthStomachBlocksGutWorms(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 4, "Ada")
	v, _ := g.vital.Get(id)
	g.ensureBody(id, &v).SetImplant(body.Stomach, item.SynthStomach)
	if len(g.diseaseCandidates(body.CondGutWorms)) != 0 || g.FireIncident("gut_worms") || g.GiveDisease(id, uint8(body.CondGutWorms)) {
		t.Fatal("a synth stomach keeps gut worms out")
	}
	if len(g.diseaseCandidates(body.CondMuscleParasites)) != 1 {
		t.Fatal("but not muscle parasites")
	}
}

// TestDiseaseLetter: an outbreak strikes 1..n colonists on the medical
// stream and posts one caution letter naming them at the first.
func TestDiseaseLetter(t *testing.T) {
	g := diseaseGame(t, biome.TemperateForest, 100)
	before := len(g.letters.list)
	if !g.FireIncident("plague") {
		t.Fatal("no plague")
	}
	var sick []string
	for _, id := range g.pawnIDs {
		if g.diseaseLabel(id) == "plague" {
			sick = append(sick, g.personName(id))
		}
	}
	if len(sick) < 1 || len(sick) > 3 || len(g.letters.list) != before+1 {
		t.Fatalf("%d sick, %d letters", len(sick), len(g.letters.list)-before)
	}
	l := g.letters.list[len(g.letters.list)-1]
	if l.Kind != letterCaution || l.Title != "Plague" || !strings.Contains(l.Text, sick[0]) || !strings.Contains(l.Text, "15 hours") {
		t.Fatalf("letter %+v", l)
	}
	for _, id := range g.diseaseCandidates(body.CondPlague) {
		if g.diseaseLabel(id) == "plague" {
			t.Fatal("only the healthy can catch it")
		}
	}

	g2 := diseaseGame(t, biome.TemperateForest, 100)
	g2.FireIncident("gut_worms")
	n := 0
	for _, id := range g2.pawnIDs {
		if g2.diseaseLabel(id) != "" {
			n++
		}
	}
	if l := g2.letters.list[len(g2.letters.list)-1]; n != 1 || !strings.Contains(l.Text, "300%") {
		t.Fatalf("gut worms strike one (%d) and the letter tells the cure: %q", n, l.Text)
	}
}

// TestDiseaseBedRestDoesNotFlap (no-flap): a colonist goes to bed when
// its flu reaches the major stage and stays there until immune, even as
// the severity hovers at the stage line; once immune and up, it does not
// go back for the disease alone.
func TestDiseaseBedRestDoesNotFlap(t *testing.T) {
	setup := func(t *testing.T, sev, imm int32) (*Game, *body.Condition) {
		t.Helper()
		g := New(Config{Seed: 1, Width: 20, Height: 10})
		id, _ := g.SpawnNamedPawn(4, 4, "Pat")
		g.SpawnNamedPawn(12, 6, "Doc")
		buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
		g.GiveDisease(id, uint8(body.CondFlu))
		c, _ := g.bodies[id].Cond(body.CondFlu)
		c.Sev, c.Immunity = sev, imm
		c.Stage = body.DefOfCond(body.CondFlu).StageAt(sev)
		c.TendQ, c.TendUntil = 500, uint64(g.clock.Now())+1000*clock.TicksPerHour // tended: no tend due
		return g, c
	}
	t.Run("minor: up and about", func(t *testing.T) {
		g, _ := setup(t, 600, 0)
		id := g.pawnIDs[0]
		if g.care.Rest.NeedsBedRest(id) {
			t.Fatal("a tended minor flu needs no bed")
		}
	})
	t.Run("at the major line", func(t *testing.T) {
		g, c := setup(t, 666, 0)
		id := g.pawnIDs[0]
		if !g.care.Rest.NeedsBedRest(id) {
			t.Fatal("major flu sends to bed")
		}
		if n := kindChanges(g, id, 300); n > 3 {
			t.Fatalf("job kind changed %d times in 300 ticks", n)
		}
		// severity falls back under the line (stage band and all): still
		// in bed while not immune
		c.Sev, c.Stage = 500, 0
		g.care.Rest.Start(id, uint64(g.clock.Now())-care.RestMin)
		g.care.Rest.Update(id)
		if !g.care.Rest.On(id) {
			t.Fatal("a minor flu not yet beaten keeps the patient in bed")
		}
		c.Immunity = infection.Full
		g.care.Rest.Update(id)
		if g.care.Rest.On(id) {
			t.Fatal("immune: up once care.RestMin has passed")
		}
		g.care.Rest.Update(id)
		if g.care.Rest.On(id) || g.care.Rest.NeedsBedRest(id) {
			t.Fatal("an immune flu does not send it back to bed")
		}
	})
	t.Run("immune at major, receding", func(t *testing.T) {
		g, _ := setup(t, 700, infection.Full)
		id := g.pawnIDs[0]
		if g.care.Rest.NeedsBedRest(id) {
			t.Fatal("an immune flu needs no bed")
		}
		if n := kindChanges(g, id, 300); n > 3 {
			t.Fatalf("job kind changed %d times in 300 ticks", n)
		}
	})
}
