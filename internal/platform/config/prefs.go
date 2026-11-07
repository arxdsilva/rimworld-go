package config

// The Options rows added by OPTIONS_GAP_ANALYSIS.md (OPT-001…032), settings
// version 10. Every field is presentation: none reaches the simulation.
// Each group's zero values are not its defaults; Defaults and Normalize
// set them.

// General is the Options screen's General tab.
type General struct {
	// Language is the catalog code ("en"; assets/lang/<code>.json, OPT-001).
	Language string `json:"language"`
	// PauseOnLoad: a loaded game starts paused (OPT-004).
	PauseOnLoad bool `json:"pause_on_load"`
	// AutoPause is what pauses the game by itself, one of AutoPauseModes
	// (OPT-005). It replaces Gameplay.PauseOnThreat.
	AutoPause string `json:"auto_pause"`
	// AutosavesKept is how many rotating autosaves a colony keeps,
	// 1..AutosavesKeptMax (OPT-006).
	AutosavesKept int `json:"autosaves_kept"`
	// PreferredNames are names the new-game wizard gives rolled colonists,
	// in order, skipping any already used (OPT-032).
	PreferredNames []string `json:"preferred_names,omitempty"`
}

// AutoPause modes (General.AutoPause).
const (
	AutoPauseNever  = "never"
	AutoPauseMajor  = "major"  // threat letters only
	AutoPauseThreat = "threat" // any message or letter marked a danger
	AutoPauseLetter = "letter" // any letter
)

// AutoPauseModes is the cycle order of General.AutoPause.
var AutoPauseModes = []string{AutoPauseNever, AutoPauseMajor, AutoPauseThreat, AutoPauseLetter}

// AutosavesKeptMax is the most autosaves kept per colony.
const AutosavesKeptMax = 25

// TempKelvin is Gameplay.TempUnit for Kelvin (OPT-007), beside "" and "F".
const TempKelvin = "K"

// Interface is the Options screen's Interface tab.
type Interface struct {
	ClockRealtime bool `json:"clock_realtime"` // OPT-008: a wall clock beside the date
	Clock12h      bool `json:"clock_12h"`      // OPT-008: 12-hour clocks
	LargerSmall   bool `json:"larger_small"`   // OPT-009: small text at the Body size (14) instead of 11
	// AnimalNames is which animals carry a name label, one of
	// AnimalNameModes (OPT-010).
	AnimalNames string `json:"animal_names"`
	// BarWeapons is when the colonist bar shows a weapon under a portrait,
	// one of BarWeaponModes (OPT-011).
	BarWeapons    string `json:"bar_weapons"`
	BarMood       bool   `json:"bar_mood"`         // OPT-012: the mood fill behind portraits
	HatsOnlyOnMap bool   `json:"hats_only_on_map"` // OPT-014
	// MenuBackground is one of MenuBackgrounds (OPT-015).
	MenuBackground string `json:"menu_background"`
	// FarPawns is which pawns stand out at the map tier, one of FarPawnModes,
	// drawn as FarStyle, one of FarStyles (OPT-016).
	FarPawns string `json:"far_pawns"`
	FarStyle string `json:"far_style"`
	// TooltipDelayMS is the hover time before a tooltip shows,
	// TooltipDelayMin..TooltipDelayMax in TooltipDelayStep (OPT-017).
	TooltipDelayMS int `json:"tooltip_delay_ms"`
	// ColorVision is the UI palette variant, one of ColorVisions (OPT-018).
	ColorVision string `json:"color_vision"`
	// Confirm asks before deconstructing and releasing an animal (OPT-019).
	Confirm bool `json:"confirm"`
	// UnpauseSpeed is the speed Space resumes at, one of UnpauseSpeeds
	// (OPT-020).
	UnpauseSpeed string `json:"unpause_speed"`
}

// Interface choices.
const (
	AnimalNamesNone  = "none"
	AnimalNamesNamed = "named" // named tame animals (today's look)
	AnimalNamesTame  = "tame"  // every tame animal

	BarWeaponsNever   = "never"
	BarWeaponsDrafted = "drafted"
	BarWeaponsAlways  = "always"

	MenuDiorama = "diorama"
	MenuSplash  = "splash"
	MenuRandom  = "random"

	FarPawnsNone    = "none"
	FarPawnsHostile = "hostile"
	FarPawnsAll     = "all"
	FarDots         = "dots"
	FarSilhouettes  = "silhouettes"
	FarBoth         = "both"

	ColorVisionNormal = "normal"
	ColorVisionDeuter = "deuteranopia"
	ColorVisionProtan = "protanopia"
	ColorVisionTritan = "tritanopia"

	UnpauseNormal = "normal"
	UnpauseLast   = "last"

	TooltipDelayMin  = 0
	TooltipDelayMax  = 2000
	TooltipDelayStep = 100
)

// Cycle orders of the Interface choices.
var (
	AnimalNameModes = []string{AnimalNamesNone, AnimalNamesNamed, AnimalNamesTame}
	BarWeaponModes  = []string{BarWeaponsNever, BarWeaponsDrafted, BarWeaponsAlways}
	MenuBackgrounds = []string{MenuDiorama, MenuSplash, MenuRandom}
	FarPawnModes    = []string{FarPawnsNone, FarPawnsHostile, FarPawnsAll}
	FarStyles       = []string{FarDots, FarSilhouettes, FarBoth}
	ColorVisions    = []string{ColorVisionNormal, ColorVisionDeuter, ColorVisionProtan, ColorVisionTritan}
	UnpauseSpeeds   = []string{UnpauseNormal, UnpauseLast}
)

// Camera is the camera and window rows (Controls and Graphics tabs).
type Camera struct {
	ZoomToMouse bool `json:"zoom_to_mouse"` // OPT-021
	SmoothJumps bool `json:"smooth_jumps"`  // OPT-022
	// DragPct scales map dragging, DragMin..DragMax in DragStep (OPT-023).
	DragPct        int  `json:"drag_pct"`
	ZoomOutToWorld bool `json:"zoom_out_to_world"` // OPT-024
	RememberStyles bool `json:"remember_styles"`   // OPT-025: the last draw style per tool
	PlantSway      bool `json:"plant_sway"`        // OPT-026
	// ShakePct is the screen shake strength, 0..ShakeMax in ShakeStep
	// (OPT-027); reduced motion draws none whatever it says.
	ShakePct     int  `json:"shake_pct"`
	CustomCursor bool `json:"custom_cursor"` // OPT-028
	// Window is a windowed size preset, one of WindowSizes ("" = free size),
	// and Borderless a borderless fullscreen window (OPT-029).
	Window     string `json:"window,omitempty"`
	Borderless bool   `json:"borderless"`
}

// Camera ranges and choices.
const (
	DragMin   = 50
	DragMax   = 300
	DragStep  = 25
	ShakeMax  = 200
	ShakeStep = 25
)

// WindowSizes are the windowed size presets ("" = free size).
var WindowSizes = []string{"", "1280x720", "1600x900", "1920x1080", "2560x1440"}

// Developer is the Developer tab, shown with developer mode (OPT-030, 031).
type Developer struct {
	VerboseLog    bool `json:"verbose_log"`
	OpenLogOnWarn bool `json:"open_log_on_warn"`
	LogEscCloses  bool `json:"log_esc_closes"`
	PauseOnError  bool `json:"pause_on_error"`
	TestMapSizes  bool `json:"test_map_sizes"`
}

func defaultGeneral() General {
	return General{Language: "en", AutoPause: AutoPauseMajor, AutosavesKept: 1}
}

func defaultInterface() Interface {
	return Interface{AnimalNames: AnimalNamesNamed, BarWeapons: BarWeaponsNever, BarMood: true, MenuBackground: MenuDiorama,
		FarPawns: FarPawnsNone, FarStyle: FarDots, TooltipDelayMS: 400, ColorVision: ColorVisionNormal, UnpauseSpeed: UnpauseNormal}
}

func defaultCamera() Camera {
	return Camera{ZoomToMouse: false, SmoothJumps: true, DragPct: 100, PlantSway: true, ShakePct: 100}
}

// normalizePrefs clamps the version 10 groups; a file older than version 10
// gets the defaults, with AutoPause from PauseOnThreat and SmoothJumps from
// ReducedMotion, which used to decide them.
func (s Settings) normalizePrefs(old bool) Settings {
	if old {
		s.General, s.Interface, s.Camera = defaultGeneral(), defaultInterface(), defaultCamera()
		if !s.Gameplay.PauseOnThreat {
			s.General.AutoPause = AutoPauseNever
		}
		s.Camera.SmoothJumps = !s.Accessibility.ReducedMotion
	}
	g, i, c := defaultGeneral(), defaultInterface(), defaultCamera()
	if s.General.Language == "" {
		s.General.Language = g.Language
	}
	pick(&s.General.AutoPause, AutoPauseModes, g.AutoPause)
	s.General.AutosavesKept = max(1, min(AutosavesKeptMax, s.General.AutosavesKept))
	pick(&s.Interface.AnimalNames, AnimalNameModes, i.AnimalNames)
	pick(&s.Interface.BarWeapons, BarWeaponModes, i.BarWeapons)
	pick(&s.Interface.MenuBackground, MenuBackgrounds, i.MenuBackground)
	pick(&s.Interface.FarPawns, FarPawnModes, i.FarPawns)
	pick(&s.Interface.FarStyle, FarStyles, i.FarStyle)
	pick(&s.Interface.ColorVision, ColorVisions, i.ColorVision)
	pick(&s.Interface.UnpauseSpeed, UnpauseSpeeds, i.UnpauseSpeed)
	s.Interface.TooltipDelayMS = snap(s.Interface.TooltipDelayMS, TooltipDelayMin, TooltipDelayMax, TooltipDelayStep)
	s.Camera.DragPct = snap(s.Camera.DragPct, DragMin, DragMax, DragStep)
	s.Camera.ShakePct = snap(s.Camera.ShakePct, 0, ShakeMax, ShakeStep)
	pick(&s.Camera.Window, WindowSizes, c.Window)
	if s.Gameplay.TempUnit != "" && s.Gameplay.TempUnit != "F" && s.Gameplay.TempUnit != TempKelvin {
		s.Gameplay.TempUnit = ""
	}
	return s
}

// pick replaces *v with def unless it is one of choices.
func pick(v *string, choices []string, def string) {
	if !contains(choices, *v) {
		*v = def
	}
}
