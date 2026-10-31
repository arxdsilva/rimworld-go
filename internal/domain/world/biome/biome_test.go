package biome

import (
	"testing"

	"rimworld-go/internal/domain/world/temperature"
)

func TestEveryBiomeDefined(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range All() {
		if d.Key == "" || d.Name == "" || d.Blurb == "" || seen[d.Key] {
			t.Fatalf("biome %d: incomplete or duplicate %+v", d.ID, d)
		}
		seen[d.Key] = true
		if id, ok := Parse(d.Key); !ok || id != d.ID {
			t.Fatalf("Parse(%q) = %v, %v", d.Key, id, ok)
		}
		if g := d.Ground; !d.Legacy && g.Soil+g.Rich+g.Gravel+g.Sand+g.Marsh+g.Ice != 1000 {
			t.Fatalf("%s ground mix sums to %d", d.Key, g.Soil+g.Rich+g.Gravel+g.Sand+g.Marsh+g.Ice)
		}
	}
	if len(seen) != int(NumBiomes) {
		t.Fatalf("%d biomes", len(seen))
	}
}

func TestTemperateIsTheOriginalClimate(t *testing.T) {
	if d := DefOf(TemperateForest); d.Climate != temperature.Temperate || !d.Legacy {
		t.Fatalf("temperate forest %+v", d)
	}
	if DefOf(NumBiomes+3).ID != TemperateForest {
		t.Fatal("unknown biome should read as temperate forest")
	}
}

func TestClimatesOrderedByCold(t *testing.T) {
	cold, _ := DefOf(IceSheet).Climate.Range()
	_, hot := DefOf(ExtremeDesert).Climate.Range()
	for _, d := range All() {
		lo, hi := d.Climate.Range()
		if lo < cold || hi > hot {
			t.Fatalf("%s range %v..%v outside ice sheet..extreme desert", d.Key, lo, hi)
		}
	}
}

func TestSet(t *testing.T) {
	s := Of(Tundra, Desert)
	if !s.Has(Tundra) || !s.Has(Desert) || s.Has(TemperateForest) {
		t.Fatalf("set %b", s)
	}
}

// TestClimatesMatchWiki (TEMP-2, wiki: Biomes): every biome's warmest
// summer afternoon and coldest winter night are the wiki's "temperature
// variation", and its annual mean lies inside the wiki's "average
// temperature" span.
func TestClimatesMatchWiki(t *testing.T) {
	t.Parallel()
	for b, w := range map[ID]struct{ summer, winter, avgHi, avgLo int }{
		TemperateForest: {35, -25, 25, 0}, TemperateSwamp: {35, -25, 25, 0},
		TropicalRainforest: {35, 0, 30, 15}, TropicalSwamp: {35, 0, 30, 15}, AridShrubland: {35, 0, 30, 15},
		Desert: {35, -25, 30, 0}, ExtremeDesert: {35, -5, 30, 10},
		BorealForest: {25, -35, 10, -10}, ColdBog: {25, -35, 10, -10}, Tundra: {25, -50, 0, -20},
		IceSheet: {10, -70, -20, -40}, SeaIce: {10, -60, -20, -30},
	} {
		cl := DefOf(b).Climate
		lo, hi := cl.Range()
		if hi != temperature.C(w.summer) || lo != temperature.C(w.winter) {
			t.Errorf("%s: %v..%v, want %d..%d C", b, lo, hi, w.winter, w.summer)
		}
		if cl.YearMean < w.avgLo*10 || cl.YearMean > w.avgHi*10 || cl.Amplitude <= 0 || cl.DaySwing <= 0 {
			t.Errorf("%s: mean %d outside %d..%d C (%+v)", b, cl.YearMean, w.avgLo, w.avgHi, cl)
		}
	}
	if DefOf(Desert).Warmth != 300 || DefOf(TemperateForest).Warmth != 90 || DefOf(SeaIce).Warmth != -280 {
		t.Error("habitat warmth keeps the pre-TEMP-2 means")
	}
	if temperature.Temperate != (temperature.Climate{YearMean: 50, Amplitude: 240, PeakDay: 15, DaySwing: 60}) {
		t.Errorf("temperate %+v", temperature.Temperate)
	}
}

// TestFitKeepsSitesInsideTheBiome: a planet site's climate is brought
// inside its biome's extremes; one inside already is unchanged.
func TestFitKeepsSitesInsideTheBiome(t *testing.T) {
	t.Parallel()
	in := temperature.Climate{YearMean: 100, Amplitude: 150, PeakDay: 45, DaySwing: 50}
	if Fit(TemperateForest, in) != in || Fit(Desert, temperature.Climate{}) != (temperature.Climate{}) {
		t.Fatal("a fitting climate changed")
	}
	hot := Fit(Desert, temperature.Climate{YearMean: 300, Amplitude: 100, PeakDay: 15, DaySwing: 120})
	if lo, hi := hot.Range(); hi != 350 || lo < -250 || hot.Amplitude != 100 || hot.DaySwing != 120 || hot.YearMean != 130 {
		t.Fatalf("a hot desert site: %+v (%v..%v)", hot, lo, hi)
	}
	wide := Fit(TropicalRainforest, temperature.Climate{YearMean: 260, Amplitude: 200, PeakDay: 15, DaySwing: 40})
	if lo, hi := wide.Range(); lo != 0 || hi != 350 || wide.Amplitude != 135 || wide.DaySwing != 40 {
		t.Fatalf("a wide tropical site: %+v (%v..%v)", wide, lo, hi)
	}
	tight := Fit(ExtremeDesert, temperature.Climate{YearMean: 150, Amplitude: 30, PeakDay: 15, DaySwing: 300})
	if lo, hi := tight.Range(); lo < -50 || hi > 350 || tight.Amplitude != 0 || tight.DaySwing != 200 {
		t.Fatalf("an all-swing site: %+v (%v..%v)", tight, lo, hi)
	}
}
