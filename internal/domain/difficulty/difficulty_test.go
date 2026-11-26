// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import (
	"bytes"
	"encoding/gob"
	"math"
	"strings"
	"testing"
)

func TestPresetValues(t *testing.T) {
	t.Parallel()
	// One row per §3.3 column: scale, big, peaceful, adapt on/strength,
	// crop, mine, butcher, research, mood, infection, disease, revenge,
	// predators, weather, pain.
	type row struct {
		p                      Preset
		scale                  int
		big                    bool
		peaceful               int
		adapt                  bool
		strength               int
		crop, mine, butch, res int
		mood, inf, dis, rev    int
		predators, weather     bool
		pain                   int
	}
	rows := []row{
		{Wanderer, 10, false, 0, false, 100, 120, 120, 100, 120, 10, 30, 33, 25, false, false, 10},
		{Gentle, 30, true, 12, true, 100, 120, 120, 100, 120, 10, 50, 40, 100, false, false, 5},
		{Settled, 60, true, 8, true, 100, 100, 100, 100, 100, 5, 75, 67, 100, true, true, 0},
		{Seasoned, 100, true, 6, true, 90, 100, 100, 100, 100, 0, 100, 100, 100, true, true, 0},
		{Grim, 155, true, 4, true, 70, 95, 95, 90, 95, -5, 100, 105, 100, true, true, 0},
		{Merciless, 220, true, 3, true, 40, 80, 80, 80, 90, -10, 110, 111, 100, true, true, -5},
	}
	if len(Presets()) != len(rows) {
		t.Fatalf("Presets() = %d rows, want %d", len(Presets()), len(rows))
	}
	for _, r := range rows {
		s := Of(r.p)
		got := row{r.p, s.ThreatScale, s.BigThreats, s.PeacefulDays, s.Adaptation, s.AdaptStrength,
			s.CropYield, s.MineYield, s.ButcherYield, s.ResearchSpeed, s.MoodOffset, s.Infection,
			s.DiseaseFreq, s.AnimalRevenge, s.PredatorsHuntColonists, s.ExtremeWeather, s.PainTolerance}
		if got != r {
			t.Errorf("%s:\n got %+v\nwant %+v", r.p, got, r)
		}
		if s.TimeEscalation || s.IgnoreWealth {
			t.Errorf("%s: time escalation and ignore-wealth are off in every preset", r.p)
		}
		if bad := Valid(s); len(bad) != 0 {
			t.Errorf("%s outside custom ranges: %v", r.p, bad)
		}
		if Match(s) != r.p {
			t.Errorf("Match(%s) = %s", r.p, Match(s))
		}
	}
	for _, n := range Presets() {
		if n.Key == "" || n.Label == "" || n.Desc == "" {
			t.Errorf("preset %v lacks text: %+v", n.Preset, n)
		}
		if p, ok := Parse(n.Key); !ok || p != n.Preset {
			t.Errorf("Parse(%q) = %v %v", n.Key, p, ok)
		}
		if p, ok := Parse(strings.ToUpper(n.Key)); !ok || p != n.Preset {
			t.Errorf("Parse is case-insensitive: %q", n.Key)
		}
	}
	if p, ok := Parse("custom"); !ok || p != Custom {
		t.Error("custom parses")
	}
	if _, ok := Parse("easy"); ok {
		t.Error("unknown keys do not parse")
	}
	if Of(Custom) != Of(Seasoned) || Of(NumPresets) != Of(Seasoned) {
		t.Error("custom starts from seasoned")
	}
	if Match(Settings{ThreatScale: 123}) != Custom {
		t.Error("odd settings are custom")
	}
	if Preset(99).Key() != "" || Preset(99).String() != "" {
		t.Error("out-of-range preset has no key")
	}
}

func TestThreatPointsMonotonicInWealth(t *testing.T) {
	t.Parallel()
	for _, p := range []Preset{Gentle, Seasoned, Merciless} {
		s := Of(p)
		prev := 0
		for w := 0; w <= 400_000; w += 500 {
			got := ThreatPoints(s, w, 4, 0, 0)
			if got < prev {
				t.Fatalf("%s: points fell from %d to %d at wealth %d", p, prev, got, w)
			}
			prev = got
		}
	}
}

func TestThreatPointsFormula(t *testing.T) {
	t.Parallel()
	s := Of(Seasoned)
	cases := []struct {
		name         string
		s            Settings
		wealth, cols int
		years        float64
		adapt        int
		want         int
	}{
		{"floor", Of(Wanderer), 0, 1, 0, 0, 35},
		{"low wealth", s, 4000, 3, 0, 0, 80 + 63},        // 4000/50 + 3*(20+1); EAI-013: colonists by wealth
		{"knee one", s, 20000, 0, 0, 0, 400},             // 20000/50
		{"mid curve", s, 60000, 2, 0, 0, 400 + 500 + 65}, // 400 + 40000/80 + 2*32.5
		{"knee two", s, 100000, 0, 0, 0, 1400},
		{"top curve", s, 250000, 0, 0, 0, 1400 + 1000},
		{"rich colonists", s, 400000, 2, 0, 0, 1400 + 2000 + 120},  // past the last knee: 60 a colonist
		{"scale", Of(Merciless), 4000, 3, 0, 0, 315},               // 143 * 2.2
		{"adapt up", s, 4000, 3, 0, 40, 200},                       // 143 * 1.4 = 200.2
		{"adapt down", s, 4000, 3, 0, -30, 100},                    // 143 * 0.7 = 100.1
		{"adapt off ignored", Of(Wanderer), 100000, 0, 0, 40, 140}, // 1400 * 10% (no adapt)
		{"time replaces wealth", func() Settings { x := s; x.IgnoreWealth = true; return x }(), 999999, 2, 1.5, 0, 120 + 50},
		{"escalation", func() Settings { x := s; x.TimeEscalation = true; x.EscPerYear = 50; return x }(), 4000, 3, 2, 0, 286},
	}
	for _, c := range cases {
		if got := ThreatPoints(c.s, c.wealth, c.cols, c.years, c.adapt); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	if ThreatPoints(s, -5, -2, -1, 0) != 35 {
		t.Error("negative inputs hit the floor")
	}
}

func TestScale(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ pct, base, want int }{
		{100, 7, 7}, {130, 10, 13}, {50, 5, 3}, {50, 3, 2}, {80, 1, 1}, {20, 1, 0}, {0, 9, 0}, {-5, 9, 0}, {120, -3, 0},
	} {
		if got := Scale(c.pct, c.base); got != c.want {
			t.Errorf("Scale(%d, %d) = %d, want %d", c.pct, c.base, got, c.want)
		}
	}
}

func TestAdaptationGrowthBoundsAndGrace(t *testing.T) {
	t.Parallel()
	s := Of(Seasoned)
	a := Adapter{}
	for i := 0; i < 60; i++ {
		a.HalfDay(s, i/2)
	}
	if a.AdaptDays() != 0 {
		t.Fatal(a)
	}
	a.Lose(1, false, 29)
	if a.AdaptDays() != -8 {
		t.Fatal(a)
	}
	a.HalfDay(s, 29)
	if a.AdaptDays() != -6.5 {
		t.Fatal(a)
	}
	for i := 0; i < 1000; i++ {
		before := a.AdaptDays()
		a.HalfDay(s, 30+i/2)
		if a.AdaptDays() < before {
			t.Fatal("growth reversed")
		}
	}
	if a.AdaptDays() != 100 {
		t.Fatal(a)
	}
	for i := 0; i < 10; i++ {
		a.Lose(3, true, 600)
	}
	if a.AdaptDays() != -60 {
		t.Fatal(a)
	}
}

func TestAdaptationFactorKnots(t *testing.T) {
	t.Parallel()
	s := Of(Seasoned)
	s.AdaptStrength = 100
	for _, c := range []struct {
		days   int32
		factor float64
	}{
		{-60, .4}, {-30, .4}, {-15, .6}, {0, .8}, {30, 1}, {60, 1.2}, {100, 1.4666666666666666},
	} {
		a := Adapter{Units: c.days * 10000, Precise: true}
		if got := a.Factor(s); math.Abs(got-c.factor) > 1e-9 {
			t.Fatalf("%+v got %g", c, got)
		}
	}
	s.AdaptStrength = 40
	a := Adapter{Units: 600000, Precise: true}
	if math.Abs(a.Factor(s)-1.08) > 1e-9 {
		t.Fatal(a.Factor(s))
	}
	s.Adaptation = false
	if a.Factor(s) != 1 {
		t.Fatal(a.Factor(s))
	}
}

func TestCodecRoundTrip(t *testing.T) {
	t.Parallel()
	for _, n := range Presets() {
		got, err := Decode(Encode(n.Settings))
		if err != nil || got != n.Settings {
			t.Errorf("%s round trip: %+v %v", n.Key, got, err)
		}
	}
	custom := Of(Grim)
	custom.TimeEscalation, custom.IgnoreWealth, custom.EscPerYear, custom.MoodOffset = true, true, 150, -17
	if got, err := Decode(Encode(custom)); err != nil || got != custom {
		t.Errorf("custom round trip: %+v %v", got, err)
	}
	if got, err := Decode(""); err != nil || got != Of(Seasoned) {
		t.Errorf("empty decodes to seasoned: %+v %v", got, err)
	}
	if got, err := Decode("ts=77"); err != nil || got.ThreatScale != 77 || got.CropYield != 100 {
		t.Errorf("partial string keeps defaults: %+v %v", got, err)
	}
	bad := []string{"zz=1", "ts", "ts=abc", "ts=501", "bt=2", "mo=-21", "ts=1,ts=2", "ts=1,zz=2,yy=3"}
	for _, v := range bad {
		if _, err := Decode(v); err == nil {
			t.Errorf("Decode(%q) should fail", v)
		}
	}
	if _, err := Decode("zz=1,yy=2"); err == nil || !strings.Contains(err.Error(), "yy, zz") {
		t.Errorf("unknown keys are named: %v", err)
	}
}

func TestCustomDifficultyFiveHundredPercentLimits(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"ts", "cy", "my", "by", "rs", "if", "df", "ar"} {
		limit, ok := Limit(key)
		if !ok || limit.Max != 500 {
			t.Fatalf("%s max = %+v, ok %v; want 500", key, limit, ok)
		}
		if got, err := Decode(key + "=500"); err != nil || len(Valid(got)) != 0 {
			t.Fatalf("%s=500 was rejected: %+v, %v", key, got, err)
		}
		if _, err := Decode(key + "=501"); err == nil {
			t.Fatalf("%s=501 was accepted", key)
		}
	}
}

func TestClampValidLimit(t *testing.T) {
	t.Parallel()
	wild := Settings{ThreatScale: 9999, PeacefulDays: -4, CropYield: 1, MoodOffset: 99, ResearchSpeed: 0}
	c := Clamp(wild)
	if bad := Valid(c); len(bad) != 0 {
		t.Errorf("clamped settings are valid: %v", bad)
	}
	if c.ThreatScale != 500 || c.PeacefulDays != 0 || c.CropYield != 20 || c.MoodOffset != 20 || c.ResearchSpeed != 25 {
		t.Errorf("clamp: %+v", c)
	}
	if len(Valid(wild)) == 0 {
		t.Error("wild settings are invalid")
	}
	if l, ok := Limit("ts"); !ok || l.Min != 0 || l.Max != 500 {
		t.Errorf("ts limit: %+v %v", l, ok)
	}
	if _, ok := Limit("bt"); ok {
		t.Error("toggles have no range")
	}
	if _, ok := Limit("nope"); ok {
		t.Error("unknown key has no range")
	}
}

func TestSaveModeString(t *testing.T) {
	t.Parallel()
	if ReloadAnytime.String() != "reload_anytime" || Commitment.String() != "commitment" {
		t.Error("save mode strings are stable keys")
	}
}

// TestThreatPointsCapped (EAI-013): however rich, crowded, adapted and
// escalated a colony is, a threat never passes PointsCap, and the cap
// leaves an ordinary colony's points alone.
func TestThreatPointsCapped(t *testing.T) {
	t.Parallel()
	s := Of(Merciless)
	s.Adaptation, s.TimeEscalation, s.EscPerYear = true, true, 100
	if got := ThreatPoints(s, 50_000_000, 60, 10, 40); got != PointsCap {
		t.Fatalf("a huge colony gets %d points, want the cap %d", got, PointsCap)
	}
	prev := 0
	for w := 0; w <= 5_000_000; w += 50_000 {
		got := ThreatPoints(s, w, 20, 3, 40)
		if got > PointsCap || got < prev {
			t.Fatalf("wealth %d: %d points (after %d), want rising and at most %d", w, got, prev, PointsCap)
		}
		prev = got
	}
	if got := ThreatPoints(Of(Seasoned), 60000, 2, 0, 0); got >= PointsCap {
		t.Fatalf("an ordinary colony is capped at %d", got)
	}
}

// TestColonistPointsByWealth (EAI-013): the same people draw a bigger
// threat in a richer colony, beyond the wealth's own share, and never a
// smaller one; with wealth ignored a colonist counts flat.
func TestColonistPointsByWealth(t *testing.T) {
	t.Parallel()
	s := Of(Seasoned)
	share := func(w int) int { return ThreatPoints(s, w, 5, 0, 0) - ThreatPoints(s, w, 0, 0, 0) }
	poor, mid, rich := ThreatPoints(s, 0, 5, 0, 0), share(100_000), share(300_000) // no wealth: below the floor alone
	if poor != 100 || mid != 200 || rich != 300 {
		t.Fatalf("five colonists add %d, %d, %d at wealth 0, 100k, 300k; want 100, 200, 300", poor, mid, rich)
	}
	prev := 0
	for w := 20_000; w <= 600_000; w += 1000 {
		if got := share(w); got < prev {
			t.Fatalf("five colonists add %d at wealth %d, less than %d before", got, w, prev)
		}
		prev = share(w)
	}
	flat := s
	flat.IgnoreWealth = true
	if a, b := ThreatPoints(flat, 0, 4, 1, 0), ThreatPoints(flat, 900_000, 4, 1, 0); a != b || a != 4*25+80 {
		t.Fatalf("wealth ignored: %d and %d, want both %d", a, b, 4*25+80)
	}
}

func TestFriendlyFirePresetsAndCodec(t *testing.T) {
	for _, n := range Presets() {
		if n.Settings.FriendlyFire != 40 {
			t.Errorf("%s: FriendlyFire=%d, want 40", n.Preset, n.Settings.FriendlyFire)
		}
	}
	for _, v := range []int{0, 37, 100} {
		s := Of(Seasoned)
		s.FriendlyFire = v
		got, err := Decode(Encode(s))
		if err != nil || got.FriendlyFire != v {
			t.Errorf("round trip %d: got %d, err %v", v, got.FriendlyFire, err)
		}
	}
	old := strings.Join(strings.Split(Encode(Of(Seasoned)), ",ff=")[:1], "")
	if strings.Contains(old, "ff=") {
		t.Fatalf("ff must be the last key: %q", old)
	}
	got, err := Decode(old)
	if err != nil || got.FriendlyFire != 100 {
		t.Errorf("pre-change string: got %d, err %v", got.FriendlyFire, err)
	}
	for _, bad := range []string{"ff=101", "ff=-1"} {
		if _, err := Decode(bad); err == nil {
			t.Errorf("Decode(%q) want error", bad)
		}
	}
}

func TestLegacyAndExplicitZeroDifficultyControls(t *testing.T) {
	t.Parallel()
	legacy := Settings{Adaptation: true, AdaptStrength: 100}
	if legacy.AdaptGrowthPercent() != 100 || legacy.FoodPoisonPercent() != 100 {
		t.Fatal("legacy settings must preserve the original neutral multipliers")
	}
	upgraded := Clamp(legacy)
	if upgraded.BalanceVersion != 3 || upgraded.AdaptGrowth != 100 || upgraded.FoodPoisonChance != 100 {
		t.Fatalf("legacy editor upgrade %+v", upgraded)
	}
	custom := Of(Seasoned)
	custom.AdaptGrowth, custom.FoodPoisonChance = 0, 0
	if custom.AdaptGrowthPercent() != 0 || custom.FoodPoisonPercent() != 0 {
		t.Fatal("explicit zero settings were treated as legacy")
	}
	got, err := Decode(Encode(custom))
	if err != nil || got != custom {
		t.Fatalf("custom zero setting round trip %+v: %v", got, err)
	}
	for _, v := range []string{"ag=101", "fp=501"} {
		if _, err := Decode(v); err == nil {
			t.Fatalf("%q should fail", v)
		}
	}
}

func TestAdaptationGrowthAndImpactIndependent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		growth   int
		wantDays float64
	}{{0, 0}, {25, 1}, {75, 3}, {100, 4}, {50, 2}} {
		s := Of(Seasoned)
		s.AdaptGrowth = tc.growth
		a := Adapter{}
		for i := 0; i < 8; i++ {
			a.HalfDay(s, 30+i/2)
		}
		if a.AdaptDays() != tc.wantDays {
			t.Fatalf("%+v got %g", tc, a.AdaptDays())
		}
	}
	s := Of(Seasoned)
	s.AdaptGrowth = 25
	a := Adapter{}
	a.HalfDay(s, 30)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(a); err != nil {
		t.Fatal(err)
	}
	var saved Adapter
	if err := gob.NewDecoder(&buf).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		a.HalfDay(s, 31+i/2)
		saved.HalfDay(s, 31+i/2)
	}
	if a.AdaptDays() != 1 || a != saved {
		t.Fatalf("%+v / %+v", a, saved)
	}
	a = Adapter{Units: 600000, Precise: true}
	s.AdaptGrowth = 100
	a.HalfDay(s, 40)
	if a.AdaptDays() != 60.25 {
		t.Fatal(a)
	}
}

func BenchmarkAdaptationGrowthFraction(b *testing.B) {
	s := Of(Gentle)
	a := Adapter{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.HalfDay(s, 100)
	}
	benchAdapt = a
}

func TestDifficultySaveVersionPreservesZeroAndLegacyDefaults(t *testing.T) {
	t.Parallel()
	type legacySettings struct {
		ThreatScale   int
		Adaptation    bool
		AdaptStrength int
	}
	encodeDecode := func(source any) Settings {
		t.Helper()
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(source); err != nil {
			t.Fatal(err)
		}
		var restored Settings
		if err := gob.NewDecoder(&buf).Decode(&restored); err != nil {
			t.Fatal(err)
		}
		return restored
	}
	legacy := encodeDecode(legacySettings{ThreatScale: 100, Adaptation: true, AdaptStrength: 100})
	if legacy.BalanceVersion != 0 || legacy.AdaptGrowthPercent() != 100 || legacy.FoodPoisonPercent() != 100 {
		t.Fatalf("legacy save %+v", legacy)
	}
	explicitZero := Of(Seasoned)
	explicitZero.AdaptGrowth = 0
	explicitZero.FoodPoisonChance = 0
	restored := encodeDecode(explicitZero)
	if restored != explicitZero || restored.AdaptGrowthPercent() != 0 || restored.FoodPoisonPercent() != 0 {
		t.Fatalf("explicit zero save changed: %+v", restored)
	}
	a := Adapter{Days: 4, Value: 2, GrowthCarry: 75}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(a); err != nil {
		t.Fatal(err)
	}
	var decoded Adapter
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	decoded.Migrate(40)
	if decoded.AdaptDays() != 4.75 || !decoded.Precise {
		t.Fatalf("adaptation credit lost: %+v", decoded)
	}
}

func TestFieldKeysStayInSyncWithCodec(t *testing.T) {
	t.Parallel()
	for _, f := range fields {
		got, ok := findField(f.key)
		if !ok || got.key != f.key || got.min != f.min || got.max != f.max {
			t.Fatalf("field lookup out of sync for %s", f.key)
		}
	}
	if _, ok := findField("unknown"); ok {
		t.Fatal("unknown field accepted")
	}
}

func TestAdaptationLossPopulationKnots(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		population int
		death      bool
		loss       float64
	}{
		{1, false, 8}, {2, false, 6}, {3, false, 6}, {7, false, 5.5}, {11, false, 5}, {20, false, 3.5}, {40, false, 3.5},
		{1, true, 30}, {11, true, 30}, {20, true, 20}, {40, true, 20},
	} {
		a := Adapter{Units: 400000, Precise: true}
		a.Lose(c.population, c.death, 40)
		if a.AdaptDays() != 40-c.loss {
			t.Fatalf("%+v got %+v", c, a)
		}
	}
}

func TestClampFriendlyFire(t *testing.T) {
	s := Of(Seasoned)
	s.FriendlyFire = 250
	if got := Clamp(s).FriendlyFire; got != 100 {
		t.Errorf("250 -> %d", got)
	}
	s.FriendlyFire = -5
	if got := Clamp(s).FriendlyFire; got != 0 {
		t.Errorf("-5 -> %d", got)
	}
}

func TestMatchWithFriendlyFire(t *testing.T) {
	for _, n := range Presets() {
		if got := Match(Of(n.Preset)); got != n.Preset {
			t.Errorf("Match(Of(%v)) = %v", n.Preset, got)
		}
	}
	s := Of(Seasoned)
	s.FriendlyFire = 50
	if got := Match(s); got != Custom {
		t.Errorf("FF 50 -> %v, want Custom", got)
	}
}

func TestAdaptationRecoveryNoFlap(t *testing.T) {
	t.Parallel()
	s := Of(Seasoned)
	s.AdaptStrength = 100
	for _, start := range []int32{-600000, -10000, 0, 590000, 990000} {
		a := Adapter{Units: start, Precise: true}
		a.Lose(5, false, 31)
		last := a.Factor(s)
		for i := 0; i < 220; i++ {
			a.HalfDay(s, 31+i/2)
			next := a.Factor(s)
			if next < last || next < .4 || next > 1.4666666666666668 {
				t.Fatalf("recovery reversed or escaped bounds: %g -> %g", last, next)
			}
			last = next
		}
	}
}

func TestLegacyCustomDifficultyCommand(t *testing.T) {
	got, err := Decode("as=150,ff=37")
	if err != nil || got.AdaptStrength != 150 || got.FriendlyFirePercent() != 37 ||
		got.AdaptGrowthPercent() != 100 || got.FoodPoisonPercent() != 100 {
		t.Fatalf("legacy custom command: %+v, %v", got, err)
	}
	a := Adapter{Days: 90}
	got.Adaptation = true
	if f := a.Factor(got); f <= a.Factor(Settings{Adaptation: true, AdaptStrength: 100}) {
		t.Fatalf("legacy extended impact has no effect: %g", f)
	}
	got, err = Decode(Encode(got))
	if err != nil || got.AdaptStrength != 150 {
		t.Fatalf("legacy impact round trip: %+v, %v", got, err)
	}
}
