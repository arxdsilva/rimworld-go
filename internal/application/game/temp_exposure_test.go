package game

import (
	"testing"

	"rimworld-go/internal/application/game/bodytemp"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// coldGame is a colony with Ada at (5, 5) under a fixed outdoor
// temperature of air tenths of a degree C.
func coldGame(t *testing.T, air int) (*Game, entity.EntityID) {
	t.Helper()
	g, ada := def4Game(t, 30, 20, 5, 5)
	g.climate = temperature.Climate{YearMean: air}
	return g, ada
}

// TestPrisonersGuestsAndRaidersTakeExposure (TEMP-3): a prisoner and a guest out
// in −30 °C build hypothermia, feel a harsh rung and show it in their
// views; a hostile raider freezes as well.
func TestPrisonersGuestsAndRaidersTakeExposure(t *testing.T) {
	t.Parallel()
	g, ada := coldGame(t, -300)
	rid := turretRaider(t, g, 20, 10)
	g.makePrisoner(rid, ada)
	vis := g.spawnVisitor(0, nil, 15, 8, uint64(g.clock.Now()))
	foe := turretRaider(t, g, 25, 15)
	for i := 0; i < 20; i++ {
		g.expose()
	}
	for _, id := range []entity.EntityID{ada, rid, vis} {
		if pv := g.prisonerPawnView(id); id != ada && pv.Hypothermia == 0 {
			t.Errorf("%d: hypothermia %d", id, pv.Hypothermia)
		}
		if g.exposure[id].Hypo < exposure.Trivial || body.ExposureSev(g.bodies[id], body.CondHypothermia) == 0 {
			t.Errorf("%d: ledger %+v, no condition", id, g.exposure[id])
		}
	}
	if g.exposure[foe].Hypo == 0 || body.ExposureSev(g.bodies[foe], body.CondHypothermia) == 0 {
		t.Error("a hostile raider freezes too (a raid in a blizzard)")
	}
	found := false
	for _, th := range g.prisonerThoughtList(rid) {
		found = found || th.Kind == mood.NumbingCold || th.Kind == mood.AbsolutelyFreezing
	}
	if !found {
		t.Error("the prisoner feels a harsh rung")
	}
}

// TestExposureDownsThroughConsciousness: past 62 % the extreme stage caps
// consciousness and the colonist goes down from hypothermia; back in
// warm air it recovers and stands, once each (no flapping), and full
// severity kills.
func TestExposureDownsThroughConsciousness(t *testing.T) {
	t.Parallel()
	g, ada := coldGame(t, -300)
	g.exposure[ada] = bodytemp.Sev{Hypo: exposure.Extreme - exposure.Full/200}
	downs, stands := 0, 0
	was := health.Alive
	step := func(n int) {
		for i := 0; i < n; i++ {
			g.expose()
			if v, _ := g.vital.Get(ada); v.Status != was {
				if v.Status == health.Downed {
					downs++
					if v.Cause != health.CauseHypothermia {
						t.Fatalf("downed by %v", v.Cause)
					}
				} else {
					stands++
				}
				was = v.Status
			}
		}
	}
	step(10)
	if downs != 1 || body.ExposureSev(g.bodies[ada], body.CondHypothermia) < 620 {
		t.Fatalf("downs %d, condition %d", downs, body.ExposureSev(g.bodies[ada], body.CondHypothermia))
	}
	g.climate = temperature.Climate{YearMean: 200}
	step(60) // 6 h: recovery takes about 3.6 h from near death
	if downs != 1 || stands != 1 || was != health.Alive || g.exposure[ada] != (bodytemp.Sev{}) || g.bodies[ada].Conds != nil {
		t.Fatalf("downs %d stands %d status %v ledger %+v conds %v", downs, stands, was, g.exposure[ada], g.bodies[ada].Conds)
	}
	g.climate = temperature.Climate{YearMean: -300}
	g.exposure[ada] = bodytemp.Sev{Hypo: exposure.Full - 1}
	g.expose()
	if v, _ := g.vital.Get(ada); v.Status != health.Dead || v.Cause != health.CauseHypothermia {
		t.Fatalf("full hypothermia kills: %v %v", v.Status, v.Cause)
	}
}

// TestFrostbiteStrikesExtremities: a colonist at 90 % hypothermia in
// −30 °C air takes frostbite on an extremity in the end; it shows on the
// view, gets an infection roll and hurts.
func TestFrostbiteStrikesExtremities(t *testing.T) {
	t.Parallel()
	g, ada := coldGame(t, -300)
	for i := 0; i < 3000 && body.FrostbiteParts(g.bodies[ada]) == 0; i++ {
		g.exposure[ada] = bodytemp.Sev{Hypo: exposure.Full / 10 * 9}
		g.expose()
	}
	b := g.bodies[ada]
	if body.FrostbiteParts(b) == 0 {
		t.Fatal("no frostbite in 3000 tries")
	}
	for _, w := range b.Wounds {
		if w.Kind == body.Frostbite && (w.InfectAt == 0 || body.Inner(w.Part) || w.Part == body.Torso) {
			t.Errorf("frostbite on %s, infect roll %d", w.Part.Label(), w.InfectAt)
		}
	}
	for _, pv := range g.Snapshot() {
		if pv.ID == ada && pv.Frostbite == 0 {
			t.Error("PawnView.Frostbite")
		}
	}
	h, bo := coldGame(t, 50) // above 0 °C: never frostbite
	for i := 0; i < 300; i++ {
		h.exposure[bo] = bodytemp.Sev{Hypo: exposure.Full / 10 * 9}
		h.expose()
	}
	if body.FrostbiteParts(h.bodies[bo]) != 0 {
		t.Error("frostbite above 0 C")
	}
}

// TestAnimalsTakeExposure: a cow (wiki −10..40 °C) in −40 °C air builds
// hypothermia on its decision ticks, shows its range and severity, and
// dies of full severity; a hive insect only slows.
func TestAnimalsTakeExposure(t *testing.T) {
	t.Parallel()
	g, _ := coldGame(t, -400)
	cow, err := g.SpawnAnimal("cow", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	g.Step(clock.TicksPerHour * 2)
	var av AnimalView
	for _, a := range g.World().Animals {
		if a.ID == cow {
			av = a
		}
	}
	if av.ComfortMin != -100 || av.ComfortMax != 400 || av.Hypothermia == 0 || av.Heatstroke != 0 {
		t.Fatalf("cow view %d..%d hypo %d heat %d", av.ComfortMin, av.ComfortMax, av.Hypothermia, av.Heatstroke)
	}
	g.exposure[cow] = bodytemp.Sev{Hypo: exposure.Full - 1}
	g.Step(animal.DecideEvery)
	if g.animals.Has(cow) || g.exposure[cow] != (bodytemp.Sev{}) {
		t.Fatal("full hypothermia kills the cow")
	}
}

// TestExposureMigratesFromPermille: a v7 save's permille exposure keeps
// its meaning on the wiki scale (600 collapsed: now the extreme stage).
func TestExposureMigratesFromPermille(t *testing.T) {
	t.Parallel()
	g, ada := coldGame(t, 200)
	g.exposure[ada] = bodytemp.Sev{Hypo: 600, Heat: 300}
	g.migrateSave(7)
	if e := g.exposure[ada]; e.Hypo != exposure.Extreme || e.Heat != exposure.FromPermille(310) {
		t.Fatalf("migrated %+v", e)
	}
	g.exposure[ada] = bodytemp.Sev{Hypo: exposure.Minor, Heat: 7}
	h := saveLoad(t, g)
	if h.exposure[ada] != g.exposure[ada] {
		t.Fatalf("round trip %+v -> %+v", g.exposure[ada], h.exposure[ada])
	}
}

// TestSleptColdJudgedBeforeApparel: the slept-in-the-cold/heat range is
// the pawn's own before apparel (wiki: Thoughts): its clothes widen the
// comfort band it lives in, never the one it sleeps by.
func TestSleptColdJudgedBeforeApparel(t *testing.T) {
	t.Parallel()
	g, ada := coldGame(t, 50)
	c, h := g.comfortShift(ada)
	lo, hi := restRoutine{g}.Band(ada)
	if want := shifted(exposure.Everyday, c, h); lo != want.Min || hi != want.Max {
		t.Fatalf("sleep band %d..%d, want %+v", lo, hi, want)
	}
	if worn := g.bandsOf(ada); worn.Min >= lo && len(g.Wearing(ada)) > 0 {
		t.Errorf("clothes %v should widen the waking band (%+v) past the sleep band", g.Wearing(ada), worn)
	}
}

// TestFreezingPawnLiesDownedThroughTheExtremeStage: a colonist freezing
// in −40 °C goes down as it passes 62 % and stays down until it dies at
// 100 %, no sooner than the wiki rate allows for that stretch.
func TestFreezingPawnLiesDownedThroughTheExtremeStage(t *testing.T) {
	t.Parallel()
	g, ada := coldGame(t, -400)
	var downAt, deadAt int
	for tick := 0; tick < 40*clock.TicksPerHour && deadAt == 0; tick += climateInterval {
		g.Step(climateInterval)
		g.SetNeeds(ada, needs.Full, needs.Full)
		v, _ := g.vital.Get(ada)
		switch {
		case v.Status == health.Dead:
			deadAt = tick
		case downAt == 0 && v.Status == health.Downed:
			downAt = tick
			if v.Cause != health.CauseHypothermia || g.exposure[ada].Hypo < exposure.Extreme-exposure.Full/50 {
				t.Fatalf("down by %v at %d", v.Cause, g.exposure[ada].Hypo)
			}
		case downAt != 0 && v.Status != health.Downed:
			t.Fatalf("stood up at %d while freezing", g.exposure[ada].Hypo)
		}
	}
	if downAt == 0 || deadAt == 0 {
		t.Fatalf("down at %d, dead at %d", downAt, deadAt)
	}
	pos, _ := g.positions.Get(ada)
	gain := exposure.HypoGain(int(g.temperatureAt(pos.X, pos.Y)), g.bandsOf(ada))
	if wiki := int((exposure.Full - exposure.Extreme) / gain * 60); deadAt-downAt < wiki*9/10 {
		t.Errorf("downed %d ticks before dying, the wiki needs about %d", deadAt-downAt, wiki)
	}
}
