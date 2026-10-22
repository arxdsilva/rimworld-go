package difficulty

import (
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
		{Wanderer, 5, false, 0, false, 100, 130, 130, 120, 130, 12, 40, 0, 50, false, false, 10},
		{Gentle, 35, true, 12, true, 60, 115, 115, 110, 115, 8, 60, 50, 70, false, true, 5},
		{Settled, 65, true, 8, true, 80, 105, 105, 100, 105, 4, 80, 75, 85, true, true, 0},
		{Seasoned, 100, true, 6, true, 100, 100, 100, 100, 100, 0, 100, 100, 100, true, true, 0},
		{Grim, 145, true, 4, true, 100, 90, 90, 90, 95, -4, 115, 120, 115, true, true, 0},
		{Merciless, 200, true, 3, true, 100, 80, 80, 80, 90, -8, 130, 150, 130, true, true, -5},
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
		{"low wealth", s, 5000, 3, 0, 0, 100 + 75},       // 5000/50 + 25*3
		{"knee one", s, 20000, 0, 0, 0, 400},             // 20000/50
		{"mid curve", s, 60000, 2, 0, 0, 400 + 500 + 50}, // 400 + 40000/80 + 50
		{"knee two", s, 100000, 0, 0, 0, 1400},
		{"top curve", s, 250000, 0, 0, 0, 1400 + 1000},
		{"scale", Of(Merciless), 5000, 3, 0, 0, 350},              // (100+75) * 2
		{"adapt up", s, 5000, 3, 0, 40, 245},                      // 175 * 1.4
		{"adapt down", s, 5000, 3, 0, -30, 123},                   // 175 * 0.7 = 122.5 -> 123
		{"adapt off ignored", Of(Wanderer), 100000, 0, 0, 40, 70}, // 1400 * 5% (no adapt)
		{"time replaces wealth", func() Settings { x := s; x.IgnoreWealth = true; return x }(), 999999, 2, 1.5, 0, 120 + 50},
		{"escalation", func() Settings { x := s; x.TimeEscalation = true; x.EscPerYear = 50; return x }(), 5000, 3, 2, 0, 350},
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

func TestAdapterRisesAndFallsAndIsBounded(t *testing.T) {
	t.Parallel()
	s := Of(Seasoned)
	var a Adapter
	for i := 0; i < 400; i++ {
		a.Day(s, false)
	}
	// Target is capped at 40; the dead band stops Value within 2 of it.
	if a.Days != 120 || a.Value < 38 || a.Value > 40 {
		t.Fatalf("after a long quiet spell: %+v, want days 120 value 38..40", a)
	}
	a.Day(s, true)
	if a.Days != 95 {
		t.Errorf("a death takes 25 days: %d", a.Days)
	}
	for i := 0; i < 10; i++ {
		a.Day(s, true)
	}
	if a.Days != -40 {
		t.Errorf("days floor at -40: %d", a.Days)
	}
	for i := 0; i < 60; i++ {
		a.Day(s, true)
	}
	if a.Value > -28 {
		t.Errorf("value falls toward -30: %d", a.Value)
	}
	if a.Value < -30 {
		t.Errorf("value never below -30: %d", a.Value)
	}
	// Strength scales the target; off stops movement.
	weak := Of(Gentle) // 60%
	var b Adapter
	for i := 0; i < 400; i++ {
		b.Day(weak, false)
	}
	if b.Value < 22 || b.Value > 24 {
		t.Errorf("60%% strength settles near 24, got %d", b.Value)
	}
	off := Of(Wanderer)
	var c Adapter
	for i := 0; i < 100; i++ {
		c.Day(off, false)
	}
	if c.Value != 0 {
		t.Errorf("adaptation off keeps value at 0, got %d", c.Value)
	}
}

// directionChanges counts reversals of Value's movement inside every
// sliding window of w days and returns the worst window.
func worstReversals(vals []int8, w int) int {
	var dirs []int // sign of each day's change, 0 for no move
	for i := 1; i < len(vals); i++ {
		switch d := int(vals[i]) - int(vals[i-1]); {
		case d > 0:
			dirs = append(dirs, 1)
		case d < 0:
			dirs = append(dirs, -1)
		default:
			dirs = append(dirs, 0)
		}
	}
	worst := 0
	for i := 0; i+w <= len(dirs); i++ {
		changes, last := 0, 0
		for _, d := range dirs[i : i+w] {
			if d == 0 {
				continue
			}
			if last != 0 && d != last {
				changes++
			}
			last = d
		}
		worst = max(worst, changes)
	}
	return worst
}

func TestAdaptationNoFlap(t *testing.T) {
	t.Parallel()
	s := Of(Seasoned)
	patterns := map[string]func(day int) bool{
		"alternate":      func(d int) bool { return d%2 == 1 },
		"every third":    func(d int) bool { return d%3 == 0 },
		"every 10":       func(d int) bool { return d%10 == 0 },
		"every 25":       func(d int) bool { return d%25 == 24 },
		"every 50":       func(d int) bool { return d%50 == 49 },
		"two on two off": func(d int) bool { return d%4 < 2 },
	}
	for name, lost := range patterns {
		for _, days := range []int16{-40, 0, 20, 40, 60, 90, 120} {
			for _, val := range []int8{-30, -10, 0, 10, 25, 40} {
				a := Adapter{Days: days, Value: val}
				vals := []int8{a.Value}
				for d := 0; d < 60; d++ {
					a.Day(s, lost(d))
					vals = append(vals, a.Value)
				}
				if w := worstReversals(vals, 6); w > 1 {
					t.Errorf("%s from days=%d value=%d: %d reversals inside 6 days: %v", name, days, val, w, vals)
				}
			}
		}
	}
	// The boundary itself: Value already within the dead band of its target
	// never moves however the daily result alternates around it.
	a := Adapter{Days: 40, Value: 10} // target = 10
	for d := 0; d < 60; d++ {
		a.Day(s, d%2 == 0)
	}
	if a.Value < -30 || a.Value > 40 {
		t.Errorf("value escaped its bounds: %d", a.Value)
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
	bad := []string{"zz=1", "ts", "ts=abc", "ts=401", "bt=2", "mo=-21", "ts=1,ts=2", "ts=1,zz=2,yy=3"}
	for _, v := range bad {
		if _, err := Decode(v); err == nil {
			t.Errorf("Decode(%q) should fail", v)
		}
	}
	if _, err := Decode("zz=1,yy=2"); err == nil || !strings.Contains(err.Error(), "yy, zz") {
		t.Errorf("unknown keys are named: %v", err)
	}
}

func TestClampValidLimit(t *testing.T) {
	t.Parallel()
	wild := Settings{ThreatScale: 9999, PeacefulDays: -4, CropYield: 1, MoodOffset: 99, ResearchSpeed: 0}
	c := Clamp(wild)
	if bad := Valid(c); len(bad) != 0 {
		t.Errorf("clamped settings are valid: %v", bad)
	}
	if c.ThreatScale != 400 || c.PeacefulDays != 0 || c.CropYield != 20 || c.MoodOffset != 20 || c.ResearchSpeed != 25 {
		t.Errorf("clamp: %+v", c)
	}
	if len(Valid(wild)) == 0 {
		t.Error("wild settings are invalid")
	}
	if l, ok := Limit("ts"); !ok || l.Min != 0 || l.Max != 400 {
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
