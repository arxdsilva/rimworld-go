package game

import (
	"testing"

	"rimworld-go/internal/application/game/farm"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
)

// climateAt moves g's clock to tick now and runs the climate tick, as the
// tick loop's incident phase does.
func climateAt(g *Game, now uint64) {
	g.clock = clock.NewAt(clock.Tick(now))
	g.climateTick(now)
}

// TestHeatWaveRampsTheOutdoorAir (TEMP-2, wiki: Events): a storyteller
// heat wave on a summer day adds 17 °C to the outdoor curve over 12,000
// ticks, blocks the other climate events, shows in the world view and
// ramps back out by its end, leaving the curve as it was.
func TestHeatWaveRampsTheOutdoorAir(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 20, Height: 20})
	base := g.climate
	start := uint64(base.PeakDay*clock.TicksPerDay + 12*clock.TicksPerHour) // midsummer: a 29 C mean
	climateAt(g, start)
	if !g.incidentFits(incident.HeatWave, start) || g.incidentFits(incident.ColdSnap, start) {
		t.Fatal("midsummer suits a heat wave, not a cold snap")
	}
	if !g.FireIncident("heat_wave") || g.FireIncident("cold_snap") || g.FireIncident("volcanic_winter") {
		t.Fatal("the heat wave must start and block the other climate events")
	}
	end := g.incidents.ClimateUntil
	if d := end - start; d < 90000 || d > 210000 {
		t.Fatalf("heat wave lasts %d ticks, want 1.5-3.5 days", d)
	}
	for _, c := range []struct {
		at  uint64
		off int
	}{{start + 6000, 85}, {start + 12000, 170}, {end - 12000, 170}, {end - 6000, 85}} {
		climateAt(g, c.at)
		if got := g.climate.YearMean - base.YearMean; got != c.off {
			t.Fatalf("offset at +%d: %d, want %d", c.at-start, got, c.off)
		}
		if got := g.climate.Outdoor(clock.Tick(c.at)) - base.Outdoor(clock.Tick(c.at)); got != temperature.Temp(c.off) {
			t.Fatalf("outdoor air +%d at +%d", got, c.at-start)
		}
	}
	w := g.World()
	if w.ClimateEvent != "heat_wave" || w.ClimateEventEnds != end || w.Extreme != w.ClimateEvent || w.SkyEvent != "" {
		t.Fatalf("view %q ends %d extreme %q", w.ClimateEvent, w.ClimateEventEnds, w.Extreme)
	}
	if l := letterByTitle(t, g, "Heat wave"); l.Kind != "caution" {
		t.Fatalf("heat wave letter %+v", l)
	}
	climateAt(g, end)
	if g.climate != base || g.World().ClimateEvent != "" || g.World().ClimateEventEnds != 0 || countText(g, incident.ClimateEndText(incident.HeatWave)) != 1 {
		t.Fatalf("after the heat wave: %+v %+v", g.climate, g.World().ClimateEvent)
	}
	if g.incidentFits(incident.HeatWave, end) && incident.DefOf(incident.HeatWave).Allowed(g.incidents, g.rel(end), g.biome) {
		t.Fatal("the heat wave cools down for 30 days")
	}
}

// TestClimateEventSeasons: a heat wave needs a 20 °C day; a cold snap a
// day between 0 and 15 °C; neither fits deep winter.
func TestClimateEventSeasons(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 20, Height: 20})
	day := func(d int) uint64 { return uint64(d * clock.TicksPerDay) }
	winter := day(g.climate.PeakDay + 30) // a -19 C mean
	if g.incidentFits(incident.HeatWave, winter) || g.incidentFits(incident.ColdSnap, winter) || !g.incidentFits(incident.VolcanicWinter, winter) {
		t.Fatal("deep winter: no heat wave, no cold snap; a volcanic winter any day")
	}
	if spring := day(0); !g.incidentFits(incident.ColdSnap, spring) || g.incidentFits(incident.HeatWave, spring) { // a 5 C mean
		t.Fatal("a 5 C spring day suits a cold snap only")
	}
}

// TestColdSnapSurvivesASave: the curve is saved without the event's
// offset and a load puts it back as at the last tick, so a loaded colony
// runs on exactly like the original.
func TestColdSnapSurvivesASave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 20, Height: 20})
	climateAt(g, uint64(2*clock.TicksPerDay))
	if !g.FireIncident("cold_snap") {
		t.Fatal("no cold snap on a 5 C day")
	}
	g.Step(7000)
	if off := g.climate.YearMean - temperature.Temperate.YearMean; off >= 0 || off != g.ng.climateOff {
		t.Fatalf("cold snap offset %d (%d)", off, g.ng.climateOff)
	}
	h := saveLoad(t, g)
	if h.climate != g.climate || h.StateHash() != g.StateHash() || h.World().ClimateEvent != "cold_snap" {
		t.Fatalf("loaded climate %+v, want %+v", h.climate, g.climate)
	}
	g.Step(300)
	h.Step(300)
	if h.climate != g.climate || h.StateHash() != g.StateHash() {
		t.Fatal("the loaded colony drifted")
	}
}

// TestOldExtremeEventMigrates: an older save's cold snap (its -14 C in
// the saved curve) becomes the climate event at the wiki's -20 C until
// its old end.
func TestOldExtremeEventMigrates(t *testing.T) {
	t.Parallel()
	g := bareSetupGame(difficulty.Seasoned)
	now := uint64(g.clock.Now())
	until := now + 2*clock.TicksPerDay
	h := loadAsVersionWith(t, g, 7, func(s *saveState) {
		s.NG1.Climate.YearMean -= 140
		s.NG1.Extreme = extremeEvent{Kind: 2, Delta: -140, Until: until}
	})
	if h.incidents.Climate != incident.ColdSnap || h.incidents.ClimateUntil != until || h.climate.YearMean != temperature.Temperate.YearMean-200 {
		t.Fatalf("migrated %+v, climate %+v", h.incidents, h.climate)
	}
	if exp := h.exportSetupState(); exp.Extreme != (extremeEvent{}) || exp.Climate != temperature.Temperate {
		t.Fatalf("saved again: %+v %+v", exp.Extreme, exp.Climate)
	}
}

// TestVolcanicWinterDimsAndThins (wiki: Events): at full strength a
// volcanic winter takes 7 °C off, the daylight (and so solar power and
// growing light) to 70 % and the wildlife capacity to half.
func TestVolcanicWinterDimsAndThins(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 40, Height: 40})
	noon := uint64(70*clock.TicksPerDay + 12*clock.TicksPerHour)
	climateAt(g, noon)
	sky, wild, base := skyLight(g), g.wildlifeCap(), g.climate.YearMean
	if _, _, ok := g.incidents.StartTimed(incident.VolcanicWinter, noon-60000, func(int) int { return 0 }); !ok {
		t.Fatal("no volcanic winter")
	}
	climateAt(g, noon)
	if got := skyLight(g); got < sky*0.699 || got > sky*0.701 {
		t.Fatalf("sky light %v, want 70 %% of %v", got, sky)
	}
	if got := g.wildlifeCap(); got != wild/2 {
		t.Fatalf("wildlife capacity %d, want half of %d", got, wild)
	}
	if st, _ := g.growState(plant.Plant{Species: plant.Potato, X: 5, Y: 5}, g.growNowOf()); g.climate.YearMean != base-70 || st != growStateGrowing ||
		g.World().ClimateEvent != "volcanic_winter" {
		t.Fatalf("mean %d, grow state %q", g.climate.YearMean-base, st)
	}
}

// TestExtremeWeatherTriplesClimateOdds: the wizard's Extreme weather
// raises the climate events' weights instead of switching them on.
func TestExtremeWeatherTriplesClimateOdds(t *testing.T) {
	t.Parallel()
	if o := natureOdds(bareSetupGame(difficulty.Seasoned)); o.Climate != 300 {
		t.Fatalf("extreme weather odds %+v", o)
	}
	if o := natureOdds(bareSetupGame(difficulty.Wanderer)); o.Climate != 100 {
		t.Fatalf("plain odds %+v", o)
	}
	if o := natureOdds(New(Config{Width: 10, Height: 10})); o.Climate != 100 {
		t.Fatalf("legacy odds %+v", o)
	}
}

// TestLeaflessPlantsStopGrowing (wiki: Plants): a tree whose cold
// threshold the air passes loses its leaves, reads "cold" and does not
// grow, warm or not, until its leaves are back a day later.
func TestLeaflessPlantsStopGrowing(t *testing.T) {
	t.Parallel()
	g := mildLight(t)
	openSky(g, 0, 0, 39, 39)
	jumpTo(g, int(g.clock.Now())/clock.TicksPerDay+1, 10)
	start := uint64(g.clock.Now())
	id, _ := g.spawnPlant(plant.Tree, 10, 10, 10)
	grow := func() plant.Plant {
		p, _ := g.plants.Get(id)
		var fp farm.Pass
		g.growPlant(id, p, uint64(g.clock.Now()), 100, nil, &fp)
		g.endFrostPass(&fp)
		q, _ := g.plants.Get(id)
		return q
	}
	setAir(g, 10, 10, plant.ColdThreshold(uint64(id))-1)
	p := grow()
	if !p.Leafless(uint64(g.clock.Now())) || p.Damage != 0 || p.Bare != start+clock.TicksPerHour+plant.LeafRegrow {
		t.Fatalf("tree in the cold: %+v", p)
	}
	if st, _ := g.growState(p, growNow{gp: 100}); st != growStateCold {
		t.Fatalf("leafless tree reads %q", st)
	}
	setAir(g, 10, 10, 200)
	for uint64(g.clock.Now())+climateInterval < p.Bare {
		g.clock.Advance(climateInterval)
		if q := grow(); q.Growth != p.Growth {
			t.Fatalf("a bare tree grew at %d: %+v", g.clock.Now(), q)
		}
	}
	g.clock.Advance(2 * climateInterval)
	if q := grow(); q.Leafless(uint64(g.clock.Now())) || q.Growth <= p.Growth {
		t.Fatalf("leaves back, warm noon: %+v", q)
	}
}
