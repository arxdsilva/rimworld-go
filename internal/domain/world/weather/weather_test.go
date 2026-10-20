package weather

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

const testBlock = clock.TicksPerHour * 6

func TestNextDeterministic(t *testing.T) {
	prev := State{Kind: Cloudy, Target: Cloudy, Intensity: 0.15}
	for b := uint64(0); b < 50; b++ {
		a := Next(7, b, prev, 8, "Spring")
		c := Next(7, b, prev, 8, "Spring")
		if a != c {
			t.Fatalf("block %d: %+v != %+v", b, a, c)
		}
	}
	differs := false
	for b := uint64(0); b < 50; b++ {
		if Next(1, b, prev, 8, "Spring") != Next(2, b, prev, 8, "Spring") {
			differs = true
		}
	}
	if !differs {
		t.Error("different seeds should produce different weather")
	}
}

func TestMarkovRowsSumToOne(t *testing.T) {
	for _, season := range []string{"Spring", "Summer", "Fall", "Winter"} {
		for _, k := range []Kind{Clear, Cloudy, Rain, Storm, Snow, Blizzard} {
			sum := 0.0
			for _, p := range Row(k, season) {
				if p < 0 {
					t.Errorf("%v %s: negative weight", k, season)
				}
				sum += p
			}
			if math.Abs(sum-1) > 1e-9 {
				t.Errorf("%v %s: row sums to %v", k, season, sum)
			}
		}
	}
	if r := Row(Clear, "Winter"); r[0] != 0.62 || r[3] != 0 {
		t.Errorf("clear row %v", r)
	}
	if Row(Snow, "Winter") != Row(Rain, "Winter") || Row(Blizzard, "Winter") != Row(Storm, "Winter") {
		t.Error("snow/blizzard should use rain/storm rows")
	}
	if Row(Rain, "Spring")[2] <= Row(Rain, "Summer")[2] {
		t.Error("spring should be wetter than summer")
	}
}

func TestNoSnowAboveTwoC(t *testing.T) {
	prev := State{}
	for b := uint64(0); b < 2000; b++ {
		prev = Next(3, b, prev, 2.5, "Winter")
		if prev.Target.Frozen() {
			t.Fatalf("block %d: %v chosen at +2.5C", b, prev.Target)
		}
	}
}

func TestColdTurnsRainToSnow(t *testing.T) {
	prev := State{}
	seen := map[Kind]bool{}
	for b := uint64(0); b < 2000; b++ {
		prev = Next(3, b, prev, -5, "Winter")
		seen[prev.Target] = true
	}
	if seen[Rain] || seen[Storm] {
		t.Errorf("liquid precipitation at -5C: %v", seen)
	}
	if !seen[Snow] || !seen[Blizzard] {
		t.Errorf("expected snow and blizzard in the cold: %v", seen)
	}
}

func TestIntensityEasesOverAnHour(t *testing.T) {
	s := State{Kind: Clear, Target: Storm, Goal: 1}
	s = s.Step(250, 10, "Spring")
	if s.Kind != Storm {
		t.Errorf("easing up from clear should switch kind at once, got %v", s.Kind)
	}
	if s.Intensity <= 0 || s.Intensity >= 0.2 {
		t.Errorf("first step intensity %v", s.Intensity)
	}
	prev := s.Intensity
	for i := 0; i < 5; i++ {
		s = s.Step(250, 10, "Spring")
		if s.Intensity <= prev {
			t.Fatalf("intensity not rising: %v -> %v", prev, s.Intensity)
		}
		prev = s.Intensity
	}
	// 2500 ticks (an hour) after the start it is fully there.
	for i := 0; i < 4; i++ {
		s = s.Step(250, 10, "Spring")
	}
	if s.Intensity != 1 {
		t.Errorf("after 2500 ticks intensity %v, want 1", s.Intensity)
	}
	// easing down keeps the old kind until near the target
	s = State{Kind: Rain, Target: Clear, Intensity: 0.6}
	s = s.Step(250, 10, "Spring")
	if s.Kind != Rain || s.Intensity >= 0.6 {
		t.Errorf("easing down: %+v", s)
	}
	for i := 0; i < 10; i++ {
		s = s.Step(250, 10, "Spring")
	}
	if s.Kind != Clear || s.Intensity != 0 {
		t.Errorf("rain should have cleared: %+v", s)
	}
}

func TestSnowDepthAccumulatesMeltsAndRainHalves(t *testing.T) {
	s := State{Kind: Snow, Target: Snow, Goal: 0.5, Intensity: 0.5}
	s = s.Step(250, -5, "Winter")
	if math.Abs(s.SnowDepth-0.006) > 1e-9 {
		t.Errorf("depth after one snowing step %v, want 0.006", s.SnowDepth)
	}
	for i := 0; i < 500; i++ {
		s = s.Step(250, -5, "Winter")
	}
	if s.SnowDepth != 1 {
		t.Errorf("depth should clamp at 1, got %v", s.SnowDepth)
	}
	m := State{Kind: Clear, Target: Clear, SnowDepth: 0.5}
	m = m.Step(250, 10, "Spring")
	if math.Abs(m.SnowDepth-0.46) > 1e-9 {
		t.Errorf("melt at 10C: %v, want 0.46", m.SnowDepth)
	}
	for i := 0; i < 50; i++ {
		m = m.Step(250, 10, "Spring")
	}
	if m.SnowDepth != 0 {
		t.Errorf("snow should melt away fully, got %v", m.SnowDepth)
	}
	h := Next(1, 5, State{Kind: Clear, SnowDepth: 0.8}, 10, "Spring")
	for b := uint64(0); h.Target != Rain && h.Target != Storm && b < 500; b++ {
		h = Next(1, b, State{Kind: Clear, SnowDepth: 0.8}, 10, "Spring")
		if h.Target == Rain || h.Target == Storm {
			if math.Abs(h.SnowDepth-0.4) > 1e-9 {
				t.Fatalf("rain block should halve snow, got %v", h.SnowDepth)
			}
		} else if h.SnowDepth != 0.8 {
			t.Fatalf("dry block changed snow: %v", h.SnowDepth)
		}
	}
}

func TestWetnessRisesAndDries(t *testing.T) {
	s := State{Kind: Rain, Target: Rain, Goal: 0.6, Intensity: 0.6}
	s = s.Step(250, 10, "Spring")
	if math.Abs(s.Wetness-0.012) > 1e-9 {
		t.Errorf("wetness %v, want 0.012", s.Wetness)
	}
	for i := 0; i < 200; i++ {
		s = s.Step(250, 10, "Spring")
	}
	if s.Wetness != 1 {
		t.Errorf("wetness should clamp at 1, got %v", s.Wetness)
	}
	d := State{Kind: Clear, Target: Clear, Wetness: 0.5}
	if d = d.Step(250, 10, "Spring"); math.Abs(d.Wetness-0.494) > 1e-9 {
		t.Errorf("dry %v, want 0.494", d.Wetness)
	}
	d = State{Kind: Clear, Target: Clear, Wetness: 0.5}
	if d = d.Step(250, 10, "Summer"); math.Abs(d.Wetness-0.488) > 1e-9 {
		t.Errorf("summer dry %v, want 0.488", d.Wetness)
	}
}

func TestWindSpeedInRangePerKind(t *testing.T) {
	ranges := map[Kind][2]float64{
		Clear: {0.4, 1.2}, Cloudy: {0.8, 1.6}, Rain: {1.2, 2.2},
		Storm: {2.5, 4.0}, Snow: {0.6, 1.4}, Blizzard: {3.0, 4.5},
	}
	for k, r := range ranges {
		for _, in := range []float64{0, 0.3, 1} {
			s := State{Kind: k, Target: k, Goal: in, Intensity: in}.Step(250, 0, "Winter")
			if s.WindSpeed < r[0]-1e-9 || s.WindSpeed > r[1]+1e-9 {
				t.Errorf("%v at %v: wind %v outside %v", k, in, s.WindSpeed, r)
			}
		}
		lo := State{Kind: k, Target: k}.Step(250, 0, "Winter").WindSpeed
		hi := State{Kind: k, Target: k, Goal: 1, Intensity: 1}.Step(250, 0, "Winter").WindSpeed
		if math.Abs(lo-r[0]) > 1e-9 || math.Abs(hi-r[1]) > 1e-9 {
			t.Errorf("%v: wind ends %v..%v, want %v", k, lo, hi, r)
		}
	}
}

func TestWindDirectionVariesSlowly(t *testing.T) {
	prev := State{}
	dirs := map[float64]bool{}
	for b := uint64(0); b < 40; b++ {
		prev = Next(9, b, prev, 10, "Spring")
		dirs[prev.WindDir] = true
	}
	if len(dirs) < 10 {
		t.Errorf("wind direction barely changes: %d values", len(dirs))
	}
}

func TestWeatherYearHasVariety(t *testing.T) {
	prev := State{}
	seen := map[Kind]int{}
	for b := uint64(0); b < 60*4; b++ {
		tick := clock.Tick(b * testBlock)
		mean := float64(temperature.DailyMean(tick)) / 10
		season := temperature.Season(tick)
		prev = Next(1, b, prev, mean, season)
		seen[prev.Target]++
		if prev.Target.Frozen() && season != "Winter" {
			t.Fatalf("block %d: %v in %s", b, prev.Target, season)
		}
	}
	for _, k := range []Kind{Clear, Cloudy, Rain, Storm, Snow, Blizzard} {
		if seen[k] == 0 {
			t.Errorf("%v never appeared in 60 days: %v", k, seen)
		}
	}
}
