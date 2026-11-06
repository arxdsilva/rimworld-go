package options

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/platform/config"
)

// interfaceRows are the Interface tab's rows (OPT-007…013, 015, 016, 019,
// 020, 026). Two columns of seven fit a 960x540 window.
func interfaceRows() []OptionRow {
	i := func(s *config.Settings) *config.Interface { return &s.Interface }
	return []OptionRow{
		withTip(toggleRow("clockrealtime", "Real-time clock", func(s *config.Settings) *bool { return &i(s).ClockRealtime }),
			"Shows the time of day on your computer beside the in-game date."),
		withTip(toggleRow("clock12h", "12-hour clocks", func(s *config.Settings) *bool { return &i(s).Clock12h }),
			"Reads every clock as 1-12 with AM or PM instead of 0-23."),
		listRow("animalnames", "Animal names", "Which animals carry a name label on the map: none, tame animals that have a name, or every tame animal.",
			func(s *config.Settings) *string { return &i(s).AnimalNames }, config.AnimalNameModes, animalNamesLabel),
		listRow("barweapons", "Weapons on the bar", "When the colonist bar shows a colonist's weapon under the portrait.",
			func(s *config.Settings) *string { return &i(s).BarWeapons }, config.BarWeaponModes, nil),
		withTip(toggleRow("barmood", "Mood on the bar", func(s *config.Settings) *bool { return &i(s).BarMood }),
			"The fill behind a portrait that rises with the colonist's mood."),
		withTip(toggleRow("colonistbar", "Colonist bar", func(s *config.Settings) *bool { return &s.Gameplay.ShowColonistBar }),
			"The row of colonist portraits along the top of the map."),
		withTip(toggleRow("categorized", "Group resources", func(s *config.Settings) *bool { return &s.Gameplay.CategorizedResources }),
			"Group the resource list by category."),
		col2(tempUnitRow()),
		col2(withTip(toggleRow("confirm", "Confirm removal", func(s *config.Settings) *bool { return &i(s).Confirm }),
			"Ask first when you deconstruct a building or release an animal.")),
		col2(listRow("unpausespeed", "Speed after pause", "The speed Space resumes at: normal speed, or the speed you paused from.",
			func(s *config.Settings) *string { return &i(s).UnpauseSpeed }, config.UnpauseSpeeds, unpauseLabel)),
		col2(listRow("farpawns", "Far zoom pawns", "Which pawns stand out when the map is zoomed far out.",
			func(s *config.Settings) *string { return &i(s).FarPawns }, config.FarPawnModes, farPawnsLabel)),
		col2(listRow("farstyle", "Far zoom look", "How those pawns are marked: dots, silhouettes or both.",
			func(s *config.Settings) *string { return &i(s).FarStyle }, config.FarStyles, nil)),
		col2(listRow("menubg", "Menu backdrop", "What the main menu shows behind the buttons.",
			func(s *config.Settings) *string { return &i(s).MenuBackground }, config.MenuBackgrounds, nil)),
		col2(withTip(toggleRow("plantsway", "Plants sway", func(s *config.Settings) *bool { return &s.Camera.PlantSway }),
			"Trees and bushes move in the wind. Reduced motion still stops them.")),
	}
}

// tempUnitRow cycles Celsius, Fahrenheit and Kelvin (OPT-007).
func tempUnitRow() OptionRow {
	return OptionRow{Key: "tempunit", Label: "Temperature unit", kind: rowChoice,
		Tip:   "How temperatures read everywhere: degrees Celsius, degrees Fahrenheit or kelvin.",
		value: func(s config.Settings) string { return units.FromUnit(s.Gameplay.TempUnit).Name() },
		apply: func(s *config.Settings, _ int) {
			s.Gameplay.TempUnit = map[string]string{"": units.UnitF, units.UnitF: units.UnitK}[s.Gameplay.TempUnit]
		}}
}

func animalNamesLabel(v string) string {
	switch v {
	case config.AnimalNamesTame:
		return "All tame"
	case config.AnimalNamesNamed:
		return "Named"
	}
	return "None"
}

func unpauseLabel(v string) string {
	if v == config.UnpauseLast {
		return "Last speed"
	}
	return "Normal"
}

func farPawnsLabel(v string) string {
	if v == config.FarPawnsHostile {
		return "Hostiles"
	}
	return titleCase(v)
}

// accessRows are the Accessibility tab's rows beyond the scale and motion
// ones (OPT-009, 014, 017, 018).
func accessRows() []OptionRow {
	return []OptionRow{
		stepRow("tooltipdelay", "Tooltip delay", "How long the pointer rests on something before its tooltip shows.",
			func(s *config.Settings) *int { return &s.Interface.TooltipDelayMS },
			config.TooltipDelayMin, config.TooltipDelayMax, config.TooltipDelayStep, delayLabel),
		listRow("colorvision", "Colour vision", "Swaps the interface colours that are hard to tell apart for a type of colour blindness.",
			func(s *config.Settings) *string { return &s.Interface.ColorVision }, config.ColorVisions, nil),
		withTip(toggleRow("largersmall", "Larger small text", func(s *config.Settings) *bool { return &s.Interface.LargerSmall }),
			"The smallest text is drawn a little bigger."),
		withTip(toggleRow("hatsonmap", "Hats only on the map", func(s *config.Settings) *bool { return &s.Interface.HatsOnlyOnMap }),
			"Portraits show the face and hair even when the colonist wears a hat."),
	}
}

func delayLabel(ms int) string {
	if ms <= 0 {
		return "Instant"
	}
	return fmt.Sprintf("%d ms", ms)
}

// cameraRows are the camera rows on the Gameplay tab's second column
// (OPT-021…025, 027).
func cameraRows() []OptionRow {
	c := func(s *config.Settings) *config.Camera { return &s.Camera }
	return []OptionRow{
		col2(withTip(toggleRow("zoomtomouse", "Zoom to mouse", func(s *config.Settings) *bool { return &c(s).ZoomToMouse }),
			"The wheel zooms toward the pointer instead of the middle of the view.")),
		col2(withTip(toggleRow("smoothjumps", "Smooth jumps", func(s *config.Settings) *bool { return &c(s).SmoothJumps }),
			"Jumping to a colonist or an alert glides there. Reduced motion jumps at once.")),
		col2(numberRow("dragpct", "Drag speed", "How far the map moves for each pixel you drag it.",
			func(s *config.Settings) *int { return &c(s).DragPct }, ladder(config.DragMin, config.DragMax, config.DragStep), percent)),
		col2(withTip(toggleRow("zoomoutworld", "Zoom to globe", func(s *config.Settings) *bool { return &c(s).ZoomOutToWorld }),
			"Zooming out past the farthest zoom opens the globe.")),
		col2(withTip(toggleRow("rememberstyles", "Keep styles", func(s *config.Settings) *bool { return &c(s).RememberStyles }),
			"Each Architect tool keeps the draw style you last used with it.")),
		col2(numberRow("shake", "Screen shake", "How hard the screen shakes for blasts and lightning. 0 turns it off; reduced motion shakes nothing.",
			func(s *config.Settings) *int { return &c(s).ShakePct }, ladder(0, config.ShakeMax, config.ShakeStep), percent)),
	}
}

// windowRows are the window rows at the end of the Graphics tab (OPT-029).
func windowRows() []OptionRow {
	return []OptionRow{
		listRow("windowsize", "Window size", "A size for the window when it is not fullscreen. Free lets you drag its edges.",
			func(s *config.Settings) *string { return &s.Camera.Window }, config.WindowSizes, windowLabel),
		col2(withTip(toggleRow("borderless", "Borderless", func(s *config.Settings) *bool { return &s.Camera.Borderless }),
			"Fullscreen is a window without borders the size of the screen, so switching away is quick.")),
	}
}

func windowLabel(v string) string {
	if v == "" {
		return "Free"
	}
	return v
}

// developerRows are the Developer tab's rows (OPT-030, 031).
func developerRows() []OptionRow {
	d := func(s *config.Settings) *config.Developer { return &s.Developer }
	return []OptionRow{
		withTip(toggleRow("verboselog", "Verbose log", func(s *config.Settings) *bool { return &d(s).VerboseLog }),
			"The log keeps every line, not only warnings and errors."),
		withTip(toggleRow("logonwarn", "Open log on warnings", func(s *config.Settings) *bool { return &d(s).OpenLogOnWarn }),
			"The log opens by itself when a warning or an error comes in."),
		withTip(toggleRow("logesc", "Escape closes the log", func(s *config.Settings) *bool { return &d(s).LogEscCloses }),
			"Escape closes the log window before anything else."),
		withTip(toggleRow("pauseonerror", "Pause on error", func(s *config.Settings) *bool { return &d(s).PauseOnError }),
			"The game pauses when an error is logged."),
		withTip(toggleRow("testmaps", "Test map sizes", func(s *config.Settings) *bool { return &d(s).TestMapSizes }),
			"The new colony screen offers very small maps for testing."),
	}
}
