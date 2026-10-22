// Package difficulty is how hard the colony's world is: six original
// presets plus custom settings, threat points with adaptation, the save
// mode, and the scaling of yields and chances (NG-1 §3.3, §4.5).
package difficulty

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

// SaveMode is how saving works for a colony (fixed at start).
type SaveMode uint8

const (
	ReloadAnytime SaveMode = iota
	Commitment
)

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

// Presets lists the six presets (not Custom); Of returns one's settings.
func Presets() []Named                { return nil }            // NG-1 stub (RULES)
func Of(p Preset) Settings            { return Settings{} }     // NG-1 stub (RULES)
func Parse(key string) (Preset, bool) { return Settled, false } // NG-1 stub (RULES)

// ThreatPoints is a threat's points (NG-1 §3.3).
func ThreatPoints(s Settings, wealth, colonists int, years float64, adapt int) int { return 35 } // NG-1 stub (RULES)

// Scale applies a percent setting to base (rounding half up, never below 0).
func Scale(pct, base int) int { return base * pct / 100 } // NG-1 stub (RULES)

// Day advances adaptation one day; lost reports a colonist death that day.
func (a *Adapter) Day(s Settings, lost bool) {} // NG-1 stub (RULES)

// Encode and Decode carry custom settings in a command string.
func Encode(s Settings) string          { return "" }              // NG-1 stub (RULES)
func Decode(v string) (Settings, error) { return Settings{}, nil } // NG-1 stub (RULES)
