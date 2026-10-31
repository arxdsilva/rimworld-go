package incident

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// TestClimateEventNumbers (TEMP-2, wiki: Events): offsets, ramps,
// lengths, cooldowns, the volcanic winter's first day; weights original.
func TestClimateEventNumbers(t *testing.T) {
	t.Parallel()
	if HeatWaveOffset != 170 || ColdSnapOffset != -200 || VolcanicOffset != -70 || SnapRamp != 12000 || VolcanicRamp != 50000 {
		t.Fatal("wiki offsets and ramps")
	}
	for _, c := range []struct {
		id     ID
		lo, hi uint64
	}{{HeatWave, 90000, 210000}, {ColdSnap, 90000, 210000}, {VolcanicWinter, 450000, 2400000}} {
		if ClimateTicks(c.id, 0) != c.lo || ClimateTicks(c.id, ClimateSpan(c.id)) != c.hi {
			t.Errorf("%v lasts %d..%d, want %d..%d", c.id, ClimateTicks(c.id, 0), ClimateTicks(c.id, ClimateSpan(c.id)), c.lo, c.hi)
		}
	}
	if ClimateTicks(Eclipse, 5) != 0 || IsClimate(ToxicFallout) {
		t.Error("only the three climate events")
	}
	if DefOf(HeatWave).Cooldown != 30 || DefOf(ColdSnap).Cooldown != 30 || DefOf(VolcanicWinter).Cooldown != 140 ||
		DefOf(VolcanicWinter).MinDay != 60 || DefOf(HeatWave).MinDay != 0 {
		t.Error("wiki cooldowns and first day")
	}
	if DefOf(HeatWave).Weight != 5 || DefOf(ColdSnap).Weight != 5 || DefOf(VolcanicWinter).Weight != 1 {
		t.Error("original weights")
	}
	if DefOf(VolcanicWinter).Allowed(State{}, 59*clock.TicksPerDay, biome.TemperateForest) ||
		!DefOf(VolcanicWinter).Allowed(State{}, 60*clock.TicksPerDay, biome.TemperateForest) {
		t.Error("volcanic winter from day 60")
	}
}

// TestClimateConditions: a heat wave needs the day's mean at 20 °C or
// more; a cold snap above 0 and below 15 °C; a volcanic winter any day.
func TestClimateConditions(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		id   ID
		temp int
		ok   bool
	}{{HeatWave, 199, false}, {HeatWave, 200, true}, {ColdSnap, 0, false}, {ColdSnap, 1, true}, {ColdSnap, 149, true},
		{ColdSnap, 150, false}, {VolcanicWinter, -300, true}, {Eclipse, 100, false}} {
		if ClimateFits(c.id, c.temp) != c.ok {
			t.Errorf("%v at %d: %v", c.id, c.temp, !c.ok)
		}
	}
}

// TestClimateRampAndExclusion: the heat wave ramps +17 °C over 12,000
// ticks in and out of its length; a volcanic winter ramps -7 °C, the sky
// to 70 % and the wildlife to half over 50,000; one at a time.
func TestClimateRampAndExclusion(t *testing.T) {
	t.Parallel()
	var st State
	const now, ticks = 1000, 100000
	if !st.StartClimate(HeatWave, now, ticks) || st.StartClimate(ColdSnap, now+10, ticks) || st.StartClimate(VolcanicWinter, now+10, ticks) {
		t.Fatal("climate events must exclude one another")
	}
	for _, c := range []struct {
		at  uint64
		off int
	}{{now, 0}, {now + 6000, 85}, {now + 12000, 170}, {now + 50000, 170}, {now + ticks - 12000, 170}, {now + ticks - 3000, 42}, {now + ticks, 0}} {
		if got := st.ClimateOffset(c.at); got != c.off {
			t.Errorf("offset at +%d: %d, want %d", c.at-now, got, c.off)
		}
	}
	if k, end := st.ClimateView(now + 5); k != "heat_wave" || end != now+ticks || st.SkyPermil(now+50000) != 1000 {
		t.Errorf("view %q %d", k, end)
	}
	if _, ended := st.EndClimate(now + ticks - 1); ended {
		t.Fatal("ended early")
	}
	if id, ended := st.EndClimate(now + ticks); !ended || id != HeatWave || st.ClimateOn(now+ticks) {
		t.Fatal("the heat wave did not end")
	}
	if k, end := st.ClimateView(now + ticks); k != "" || end != 0 {
		t.Fatalf("view after the end %q %d", k, end)
	}
	v := State{}
	start := uint64(5 * clock.TicksPerDay)
	if !v.StartClimate(VolcanicWinter, start, 10*clock.TicksPerDay) {
		t.Fatal("volcanic winter after the heat wave")
	}
	full := start + 50000
	if v.ClimateOffset(full) != -70 || v.SkyPermil(full) != 700 || v.WildlifePermil(full) != 500 ||
		v.ClimateOffset(start+25000) != -35 || v.SkyPermil(start+25000) != 850 || v.WildlifePermil(start+25000) != 750 {
		t.Fatalf("volcanic winter: %d %d %d", v.ClimateOffset(full), v.SkyPermil(full), v.WildlifePermil(full))
	}
	if title, text := ClimateLetter(VolcanicWinter, 10*clock.TicksPerDay); title != "Volcanic winter" || text == "" || ClimateEndText(ColdSnap) == "" {
		t.Fatal("letters")
	}
}

// TestExtremeWeatherRaisesClimateWeights: the wizard's Extreme weather
// triples the climate events' weights; diseases keep their own odds.
func TestExtremeWeatherRaisesClimateWeights(t *testing.T) {
	t.Parallel()
	ext := Odds{Disease: FullFreq, Climate: ExtremeClimatePct}
	if DefOf(HeatWave).WeightOdds(ext) != 15 || DefOf(VolcanicWinter).WeightOdds(ext) != 3 ||
		DefOf(Flu).WeightOdds(ext) != DefOf(Flu).Weight || DefOf(Party).WeightOdds(ext) != DefOf(Party).Weight {
		t.Fatal("extreme weather weights")
	}
	if DefOf(HeatWave).WeightAt(0) != 5 || DefOf(ColdSnap).WeightOdds(Odds{Disease: FullFreq}) != 0 {
		t.Fatal("disease frequency leaves climate weights alone; climate 0 disables them")
	}
	now := uint64(20 * clock.TicksPerDay)
	if d := TotalOdds(Nature, State{}, now, biome.TemperateForest, ext, nil) - Total(Nature, State{}, now, biome.TemperateForest, nil); d != 20 {
		t.Fatalf("extreme weather adds %d weight, want 20 (two events at 5 -> 15)", d)
	}
}
