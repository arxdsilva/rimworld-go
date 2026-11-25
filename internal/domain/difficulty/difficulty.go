// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package difficulty defines native difficulty presets, custom limits,
// adaptation and deterministic threat scaling.
package difficulty

//go:generate python3 ../../../scripts/gen_difficulty_presets.py

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
	ThreatScale      int
	BigThreats       bool
	PeacefulDays     int
	Adaptation       bool
	AdaptStrength    int // adaptation impact
	AdaptGrowth      int // percentage of an adaptation day earned per calm day
	FoodPoisonChance int // food contamination multiplier
	TimeEscalation   bool
	EscPerYear       int
	IgnoreWealth     bool
	CropYield        int
	MineYield        int
	ButcherYield     int
	ResearchSpeed    int
	MoodOffset       int // points
	Infection        int
	DiseaseFreq      int
	AnimalRevenge    int
	// FriendlyFire is the permitted share of a colonist-on-colonist shot interception.
	FriendlyFire int
	// BalanceVersion 0 lacked friendly-fire, 1 lacked growth/poison,
	// 2 and later retain explicit zero for all controls.
	BalanceVersion         uint8 `json:"-"`
	PredatorsHuntColonists bool
	ExtremeWeather         bool
	PainTolerance          int // percent offset
	// QuestRewardPct scales quest rewards (QST-034, percent, 0..500).
	QuestRewardPct int
	// QuestThreats: violent quests are offered (QST-033; off only on the
	// most peaceful preset).
	QuestThreats bool
}

// DefaultFriendlyFire is the vanilla interception scale for player allies.
const DefaultFriendlyFire = 40

// FriendlyFirePercent reads the explicit setting, or the legacy unstored default.
func (s Settings) FriendlyFirePercent() int { return min(max(s.upgraded().FriendlyFire, 0), 100) }

func (s Settings) upgraded() Settings {
	if s.BalanceVersion == 0 && s.FriendlyFire == 0 {
		s.FriendlyFire = DefaultFriendlyFire
	}
	if s.BalanceVersion < 2 {
		s.AdaptGrowth, s.FoodPoisonChance = 100, 100
	}
	s.BalanceVersion = 3
	return s
}

// Upgraded normalizes an old saved setting, retaining explicit zero controls.
func (s Settings) Upgraded() Settings { return s.upgraded() }

// FoodPoisonPercent is the chance multiplier applied at consumption.
func (s Settings) FoodPoisonPercent() int { return s.upgraded().FoodPoisonChance }

// AdaptGrowthPercent controls quiet-day adaptation increase.
func (s Settings) AdaptGrowthPercent() int { return s.upgraded().AdaptGrowth }

// Setback is the historical saved harm accumulator. New adaptation
// accounts for events when they occur; these fields only decode old saves.
type Setback struct {
	Wounds int
	Downed int
	Deaths int
}

// Named is a preset with its display text.
type Named struct {
	Preset   Preset
	Key      string
	Label    string
	Desc     string
	Settings Settings
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
	s = s.upgraded()
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
	f, ok := findField(key)
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
	{key: "ts", min: 0, max: 500, ptr: func(s *Settings) *int { return &s.ThreatScale }},
	{key: "bt", bool: true, bptr: func(s *Settings) *bool { return &s.BigThreats }},
	{key: "pd", min: 0, max: 30, ptr: func(s *Settings) *int { return &s.PeacefulDays }},
	{key: "ad", bool: true, bptr: func(s *Settings) *bool { return &s.Adaptation }},
	{key: "as", min: 0, max: 200, ptr: func(s *Settings) *int { return &s.AdaptStrength }},
	{key: "ag", min: 0, max: 100, ptr: func(s *Settings) *int { return &s.AdaptGrowth }},
	{key: "fp", min: 0, max: 500, ptr: func(s *Settings) *int { return &s.FoodPoisonChance }},
	{key: "te", bool: true, bptr: func(s *Settings) *bool { return &s.TimeEscalation }},
	{key: "ep", min: 0, max: 200, ptr: func(s *Settings) *int { return &s.EscPerYear }},
	{key: "iw", bool: true, bptr: func(s *Settings) *bool { return &s.IgnoreWealth }},
	{key: "cy", min: 20, max: 500, ptr: func(s *Settings) *int { return &s.CropYield }},
	{key: "my", min: 20, max: 500, ptr: func(s *Settings) *int { return &s.MineYield }},
	{key: "by", min: 20, max: 500, ptr: func(s *Settings) *int { return &s.ButcherYield }},
	{key: "rs", min: 25, max: 500, ptr: func(s *Settings) *int { return &s.ResearchSpeed }},
	{key: "mo", min: -20, max: 20, ptr: func(s *Settings) *int { return &s.MoodOffset }},
	{key: "if", min: 0, max: 500, ptr: func(s *Settings) *int { return &s.Infection }},
	{key: "df", min: 0, max: 500, ptr: func(s *Settings) *int { return &s.DiseaseFreq }},
	{key: "ar", min: 0, max: 500, ptr: func(s *Settings) *int { return &s.AnimalRevenge }},
	{key: "ph", bool: true, bptr: func(s *Settings) *bool { return &s.PredatorsHuntColonists }},
	{key: "ew", bool: true, bptr: func(s *Settings) *bool { return &s.ExtremeWeather }},
	{key: "pt", min: -20, max: 20, ptr: func(s *Settings) *int { return &s.PainTolerance }},
	{key: "qr", min: 0, max: 500, ptr: func(s *Settings) *int { return &s.QuestRewardPct }},
	{key: "qt", bool: true, bptr: func(s *Settings) *bool { return &s.QuestThreats }},
	{key: "ff", min: 0, max: 100, ptr: func(s *Settings) *int { return &s.FriendlyFire }},
}

func findField(key string) (field, bool) {
	for _, f := range fields {
		if f.key == key {
			return f, true
		}
	}
	return field{}, false
}

// Clamp forces every numeric member of s into its custom range.
func Clamp(s Settings) Settings {
	s = s.upgraded()
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
	s = s.upgraded()
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
	pointsFloor = 35
	colonistPts = 25
	// PointsCap is the most a threat may ever be given (EAI-013, original
	// tuning): wealth, colonists, adaptation and escalation never push a
	// raid past it.
	PointsCap    = 10_000
	timePtsPerYr = 80
	wealthKnee1  = 20_000
	wealthKnee2  = 100_000
)

// colonistCurve is the points one standing colonist adds to a threat at
// colony wealth w (EAI-013, original tuning). The knees are wealth and
// points per colonist, straight lines between them and flat past the
// last: a poor colony's people draw small raids, a rich one's bigger ones.
func colonistCurve(w int) float64 {
	k := [...][2]float64{{0, 20}, {20_000, 25}, {100_000, 40}, {300_000, 60}}
	x := float64(max(w, 0))
	for i := 1; i < len(k); i++ {
		if x <= k[i][0] {
			return k[i-1][1] + (x-k[i-1][0])*(k[i][1]-k[i-1][1])/(k[i][0]-k[i-1][0])
		}
	}
	return k[len(k)-1][1]
}

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
// Adapter.Value (ignored while adaptation is off). Each standing colonist
// counts by the colony's wealth (colonistCurve; a flat colonistPts when
// the difficulty ignores wealth), and the result stays within
// pointsFloor..PointsCap (EAI-013).
func ThreatPoints(s Settings, wealth, colonists int, years float64, adapt int) int {
	if years < 0 {
		years = 0
	}
	base := float64(colonistPts * max(colonists, 0))
	if s.IgnoreWealth {
		base += float64(timePtsPerYr * years)
	} else {
		base = colonistCurve(wealth)*float64(max(colonists, 0)) + wealthCurve(wealth)
	}
	if !s.Adaptation {
		adapt = 0
	}
	pts := base * float64(s.ThreatScale) / 100 * float64(max(100+adapt, 0)) / 100
	if s.TimeEscalation {
		pts *= math.Max(0, (100+float64(float64(s.EscPerYear)*years))/100)
	}
	return min(max(int(math.Floor(pts+0.5)), pointsFloor), PointsCap)
}

// Scale applies a percent setting to base (rounding half up, never below 0).
func Scale(pct, base int) int {
	if pct <= 0 || base <= 0 {
		return 0
	}
	return (base*pct + 50) / 100
}

// Yield fields share the original numeric identifiers with the game.
const (
	YieldCrop = iota
	YieldMine
	YieldButcher
)

// ScaleYield applies the difficulty's yield percentage. Positive ore
// stays at least one unit; other yields use the original half-up Scale.
func ScaleYield(s Settings, field, base int) int {
	pct := 100
	switch field {
	case YieldCrop:
		pct = s.CropYield
	case YieldMine:
		pct = s.MineYield
	case YieldButcher:
		pct = s.ButcherYield
	}
	v := Scale(pct, base)
	if field == YieldMine && base > 0 {
		return max(v, 1)
	}
	return v
}

// Encode and Decode carry custom settings in a command string: sorted-free
// "key=value,..." in the stable order of fields, booleans as 0/1.
func Encode(s Settings) string {
	s = s.upgraded()
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
		f, known := findField(k)
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
	if !seen["ff"] {
		s.FriendlyFire = 100
	}
	if !seen["ag"] {
		s.AdaptGrowth = 100
	}
	if !seen["fp"] {
		s.FoodPoisonChance = 100
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Settings{}, fmt.Errorf("difficulty: unknown keys %s", strings.Join(unknown, ", "))
	}
	return s, nil
}
