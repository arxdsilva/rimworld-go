// Package difficulty is how hard the colony's world is: six original
// presets plus custom settings, threat points with adaptation, the save
// mode, and the scaling of yields and chances (NG-1 §3.3, §4.5).
package difficulty

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Preset is a named difficulty.
type Preset uint8

const (
	Wanderer Preset = iota
	Gentle
	Settled
	Seasoned
	Grim
	Merciless
	Custom
	NumPresets
)

var presetKeys = [NumPresets]string{"wanderer", "gentle", "settled", "seasoned", "grim", "merciless", "custom"}

// Key is the preset's stable name (commands and saves).
func (p Preset) Key() string {
	if p >= NumPresets {
		return ""
	}
	return presetKeys[p]
}

// String is the display label ("Wanderer").
func (p Preset) String() string {
	k := p.Key()
	if k == "" {
		return ""
	}
	return strings.ToUpper(k[:1]) + k[1:]
}

// SaveMode is how saving works for a colony (fixed at start).
type SaveMode uint8

const (
	ReloadAnytime SaveMode = iota
	Commitment
)

func (m SaveMode) String() string {
	if m == Commitment {
		return "commitment"
	}
	return "reload_anytime"
}

// Settings are every difficulty parameter (percents unless noted).
type Settings struct {
	ThreatScale            int
	BigThreats             bool
	PeacefulDays           int
	Adaptation             bool
	AdaptStrength          int
	TimeEscalation         bool
	EscPerYear             int
	IgnoreWealth           bool
	CropYield              int
	MineYield              int
	ButcherYield           int
	ResearchSpeed          int
	MoodOffset             int // points
	Infection              int
	DiseaseFreq            int
	AnimalRevenge          int
	PredatorsHuntColonists bool
	ExtremeWeather         bool
	PainTolerance          int // percent offset
}

// Adapter is the adaptation state (saved).
type Adapter struct {
	Days  int16
	Value int8
}

// Named is a preset with its display text.
type Named struct {
	Preset   Preset
	Key      string
	Label    string
	Desc     string
	Settings Settings
}

// presets is the §3.3 table. AdaptStrength is kept at 100 where adaptation
// is off so that switching it on in the custom editor starts sensibly;
// EscPerYear likewise holds the editor's default.
var presets = [Custom]Named{
	Wanderer: {Desc: "Build in peace — no raids or packs.", Settings: Settings{
		ThreatScale: 5, BigThreats: false, PeacefulDays: 0, Adaptation: false, AdaptStrength: 100, EscPerYear: 25,
		CropYield: 130, MineYield: 130, ButcherYield: 120, ResearchSpeed: 130, MoodOffset: 12,
		Infection: 40, DiseaseFreq: 0, AnimalRevenge: 50, PredatorsHuntColonists: false, ExtremeWeather: false, PainTolerance: 10}},
	Gentle: {Desc: "Rare, small threats; room to learn.", Settings: Settings{
		ThreatScale: 35, BigThreats: true, PeacefulDays: 12, Adaptation: true, AdaptStrength: 60, EscPerYear: 25,
		CropYield: 115, MineYield: 115, ButcherYield: 110, ResearchSpeed: 115, MoodOffset: 8,
		Infection: 60, DiseaseFreq: 50, AnimalRevenge: 70, PredatorsHuntColonists: false, ExtremeWeather: true, PainTolerance: 5}},
	Settled: {Desc: "Fair challenge with time to recover.", Settings: Settings{
		ThreatScale: 65, BigThreats: true, PeacefulDays: 8, Adaptation: true, AdaptStrength: 80, EscPerYear: 25,
		CropYield: 105, MineYield: 105, ButcherYield: 100, ResearchSpeed: 105, MoodOffset: 4,
		Infection: 80, DiseaseFreq: 75, AnimalRevenge: 85, PredatorsHuntColonists: true, ExtremeWeather: true, PainTolerance: 0}},
	Seasoned: {Desc: "The intended challenge.", Settings: Settings{
		ThreatScale: 100, BigThreats: true, PeacefulDays: 6, Adaptation: true, AdaptStrength: 100, EscPerYear: 25,
		CropYield: 100, MineYield: 100, ButcherYield: 100, ResearchSpeed: 100, MoodOffset: 0,
		Infection: 100, DiseaseFreq: 100, AnimalRevenge: 100, PredatorsHuntColonists: true, ExtremeWeather: true, PainTolerance: 0}},
	Grim: {Desc: "Harsh threats, thin margins.", Settings: Settings{
		ThreatScale: 145, BigThreats: true, PeacefulDays: 4, Adaptation: true, AdaptStrength: 100, EscPerYear: 25,
		CropYield: 90, MineYield: 90, ButcherYield: 90, ResearchSpeed: 95, MoodOffset: -4,
		Infection: 115, DiseaseFreq: 120, AnimalRevenge: 115, PredatorsHuntColonists: true, ExtremeWeather: true, PainTolerance: 0}},
	Merciless: {Desc: "Built to break you. Mostly.", Settings: Settings{
		ThreatScale: 200, BigThreats: true, PeacefulDays: 3, Adaptation: true, AdaptStrength: 100, EscPerYear: 25,
		CropYield: 80, MineYield: 80, ButcherYield: 80, ResearchSpeed: 90, MoodOffset: -8,
		Infection: 130, DiseaseFreq: 150, AnimalRevenge: 130, PredatorsHuntColonists: true, ExtremeWeather: true, PainTolerance: -5}},
}

func init() {
	for i := range presets {
		presets[i].Preset = Preset(i)
		presets[i].Key = presetKeys[i]
		presets[i].Label = Preset(i).String()
	}
}

// Presets lists the six presets (not Custom); Of returns one's settings.
func Presets() []Named { return append([]Named(nil), presets[:]...) }

// Of is p's settings; Custom (and anything unknown) is Seasoned, the
// neutral starting point of the custom editor.
func Of(p Preset) Settings {
	if p >= Custom {
		return presets[Seasoned].Settings
	}
	return presets[p].Settings
}

// Parse maps a preset key (case-insensitive) to a Preset, "custom" included.
func Parse(key string) (Preset, bool) {
	key = strings.ToLower(key)
	for i, k := range presetKeys {
		if k == key {
			return Preset(i), true
		}
	}
	return Settled, false
}

// Match is the preset whose settings equal s, or Custom.
func Match(s Settings) Preset {
	for _, n := range presets {
		if n.Settings == s {
			return n.Preset
		}
	}
	return Custom
}

// Limits are the custom editor's inclusive ranges (§3.3).
type Limits struct{ Min, Max int }

// Limit returns the range of an Encode key ("ts", "cy", ...); ok is false
// for keys without a numeric range.
func Limit(key string) (Limits, bool) {
	f, ok := fieldByKey[key]
	if !ok || f.bool {
		return Limits{}, false
	}
	return Limits{f.min, f.max}, true
}

// field describes one Settings member for the compact codec and the range
// check. ptr returns the member of a Settings.
type field struct {
	key      string
	bool     bool
	min, max int
	ptr      func(*Settings) *int
	bptr     func(*Settings) *bool
}

var fields = []field{
	{key: "ts", min: 0, max: 400, ptr: func(s *Settings) *int { return &s.ThreatScale }},
	{key: "bt", bool: true, bptr: func(s *Settings) *bool { return &s.BigThreats }},
	{key: "pd", min: 0, max: 30, ptr: func(s *Settings) *int { return &s.PeacefulDays }},
	{key: "ad", bool: true, bptr: func(s *Settings) *bool { return &s.Adaptation }},
	{key: "as", min: 0, max: 200, ptr: func(s *Settings) *int { return &s.AdaptStrength }},
	{key: "te", bool: true, bptr: func(s *Settings) *bool { return &s.TimeEscalation }},
	{key: "ep", min: 0, max: 200, ptr: func(s *Settings) *int { return &s.EscPerYear }},
	{key: "iw", bool: true, bptr: func(s *Settings) *bool { return &s.IgnoreWealth }},
	{key: "cy", min: 20, max: 300, ptr: func(s *Settings) *int { return &s.CropYield }},
	{key: "my", min: 20, max: 300, ptr: func(s *Settings) *int { return &s.MineYield }},
	{key: "by", min: 20, max: 300, ptr: func(s *Settings) *int { return &s.ButcherYield }},
	{key: "rs", min: 25, max: 300, ptr: func(s *Settings) *int { return &s.ResearchSpeed }},
	{key: "mo", min: -20, max: 20, ptr: func(s *Settings) *int { return &s.MoodOffset }},
	{key: "if", min: 0, max: 300, ptr: func(s *Settings) *int { return &s.Infection }},
	{key: "df", min: 0, max: 300, ptr: func(s *Settings) *int { return &s.DiseaseFreq }},
	{key: "ar", min: 0, max: 300, ptr: func(s *Settings) *int { return &s.AnimalRevenge }},
	{key: "ph", bool: true, bptr: func(s *Settings) *bool { return &s.PredatorsHuntColonists }},
	{key: "ew", bool: true, bptr: func(s *Settings) *bool { return &s.ExtremeWeather }},
	{key: "pt", min: -20, max: 20, ptr: func(s *Settings) *int { return &s.PainTolerance }},
}

var fieldByKey = func() map[string]field {
	m := make(map[string]field, len(fields))
	for _, f := range fields {
		m[f.key] = f
	}
	return m
}()

// Clamp forces every numeric member of s into its custom range.
func Clamp(s Settings) Settings {
	for _, f := range fields {
		if f.bool {
			continue
		}
		p := f.ptr(&s)
		*p = min(max(*p, f.min), f.max)
	}
	return s
}

// Valid lists the members of s outside their custom range (empty: valid).
func Valid(s Settings) []string {
	var bad []string
	for _, f := range fields {
		if f.bool {
			continue
		}
		if v := *f.ptr(&s); v < f.min || v > f.max {
			bad = append(bad, f.key)
		}
	}
	return bad
}

// Threat points constants (§3.3).
const (
	pointsFloor   = 35
	colonistPts   = 25
	timePtsPerYr  = 80
	wealthKnee1   = 20_000
	wealthKnee2   = 100_000
	adaptMin      = -30
	adaptMax      = 40
	adaptDayCap   = 120
	adaptDayFloor = -40
	adaptDeathHit = 25
	adaptStep     = 2
	adaptDeadBand = 3
)

// wealthCurve is the original wealth-to-points curve of §3.3.
func wealthCurve(w int) float64 {
	switch {
	case w <= 0:
		return 0
	case w <= wealthKnee1:
		return float64(w) / 50
	case w <= wealthKnee2:
		return 400 + float64(w-wealthKnee1)/80
	}
	return 1400 + float64(w-wealthKnee2)/150
}

// ThreatPoints is a threat's points (NG-1 §3.3). adapt is the current
// Adapter.Value (ignored while adaptation is off).
func ThreatPoints(s Settings, wealth, colonists int, years float64, adapt int) int {
	if years < 0 {
		years = 0
	}
	base := float64(colonistPts * max(colonists, 0))
	if s.IgnoreWealth {
		base += timePtsPerYr * years
	} else {
		base += wealthCurve(wealth)
	}
	if !s.Adaptation {
		adapt = 0
	}
	pts := base * float64(s.ThreatScale) / 100 * float64(max(100+adapt, 0)) / 100
	if s.TimeEscalation {
		pts *= math.Max(0, (100+float64(s.EscPerYear)*years)/100)
	}
	return max(int(math.Floor(pts+0.5)), pointsFloor)
}

// Scale applies a percent setting to base (rounding half up, never below 0).
func Scale(pct, base int) int {
	if pct <= 0 || base <= 0 {
		return 0
	}
	return (base*pct + 50) / 100
}

// Day advances adaptation one day; lost reports a colonist death that day.
// Value drifts toward a target set by the quiet-day count, by at most two
// points a day and only once it is three or more away: the dead band and
// the rate limit keep it from flapping (AGENTS.md hysteresis rule).
func (a *Adapter) Day(s Settings, lost bool) {
	if lost {
		a.Days = int16(max(int(a.Days)-adaptDeathHit, adaptDayFloor))
	} else {
		a.Days = int16(min(int(a.Days)+1, adaptDayCap))
	}
	if !s.Adaptation {
		return
	}
	t := min(max(int(a.Days)/2-10, adaptMin), adaptMax) * s.AdaptStrength / 100
	t = min(max(t, adaptMin), adaptMax)
	d := t - int(a.Value)
	if d > -adaptDeadBand && d < adaptDeadBand {
		return
	}
	step := min(max(d, -adaptStep), adaptStep)
	a.Value = int8(int(a.Value) + step)
}

// Encode and Decode carry custom settings in a command string: sorted-free
// "key=value,..." in the stable order of fields, booleans as 0/1.
func Encode(s Settings) string {
	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(f.key)
		b.WriteByte('=')
		if f.bool {
			if *f.bptr(&s) {
				b.WriteByte('1')
			} else {
				b.WriteByte('0')
			}
		} else {
			b.WriteString(strconv.Itoa(*f.ptr(&s)))
		}
	}
	return b.String()
}

// Decode parses Encode's output. Missing keys keep the Seasoned value;
// unknown keys, malformed pairs and out-of-range numbers are errors.
func Decode(v string) (Settings, error) {
	s := Of(Seasoned)
	if strings.TrimSpace(v) == "" {
		return s, nil
	}
	seen := map[string]bool{}
	var unknown []string
	for _, pair := range strings.Split(v, ",") {
		k, val, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok {
			return Settings{}, fmt.Errorf("difficulty: malformed pair %q", pair)
		}
		f, known := fieldByKey[k]
		if !known {
			unknown = append(unknown, k)
			continue
		}
		if seen[k] {
			return Settings{}, fmt.Errorf("difficulty: duplicate key %q", k)
		}
		seen[k] = true
		if f.bool {
			switch val {
			case "0":
				*f.bptr(&s) = false
			case "1":
				*f.bptr(&s) = true
			default:
				return Settings{}, fmt.Errorf("difficulty: %s must be 0 or 1, got %q", k, val)
			}
			continue
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return Settings{}, fmt.Errorf("difficulty: %s: %w", k, err)
		}
		if n < f.min || n > f.max {
			return Settings{}, fmt.Errorf("difficulty: %s=%d outside %d..%d", k, n, f.min, f.max)
		}
		*f.ptr(&s) = n
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Settings{}, fmt.Errorf("difficulty: unknown keys %s", strings.Join(unknown, ", "))
	}
	return s, nil
}
