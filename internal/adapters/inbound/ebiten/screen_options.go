package ebiten

import (
	"fmt"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/config"
)

// Options (MAIN UI §42): tabs of rows built from one table, so the
// buttons, their drawing, the clicks and the tests never drift apart.
// Every change goes through setSettings (settings.go): applied at once
// and saved.

type rowKind int

const (
	rowToggle rowKind = iota // one On/Off button
	rowStep                  // − bar +
	rowChoice                // one button that cycles
)

type optionRow struct {
	key, label string
	kind       rowKind
	value      func(config.Settings) string
	frac       func(config.Settings) float64   // stepper bar fill, 0..1
	apply      func(s *config.Settings, d int) // d: -1/+1 for steppers, 0 to toggle/cycle
	live       func(a *App) string             // value that needs the running app (Auto UI scale); overrides value
	tip        string                          // hover line
	extra      *optionAction                   // a second button beside the control
	// applyApp, when set, replaces apply for a change that needs the running
	// app (the screen's device scale, the hardware).
	applyApp func(a *App, s *config.Settings, d int)
	// col is 0 for the first column and 1 for the second (Graphics only).
	col int
}

// optionAction is a one-shot button on a row (e.g. Reset lessons): it
// changes the settings once and holds no value of its own.
type optionAction struct {
	key, label, tip string
	apply           func(s *config.Settings)
	// applyApp replaces apply when the action needs the running app, and
	// off returns the reason the button is disabled ("" when it works).
	applyApp func(a *App, s *config.Settings)
	off      func(a *App) string
}

var optionTabLabels = map[string]string{"audio": "Audio", "graphics": "Graphics", "gameplay": "Gameplay", "controls": "Controls", "access": "Accessibility", "story": "Story"}

func optionTabs() []string { return []string{"audio", "graphics", "gameplay", "controls", "access"} }

// optionTabList is the tabs on offer now: Story (story_tab.go) joins them
// while a colony is running.
func (a *App) optionTabList() []string {
	tabs := optionTabs()
	if a.renderer == nil {
		return tabs
	}
	return append(tabs[:3:3], "story", "controls", "access")
}

// Layout (px).
const (
	optTabY    = 108
	optTabW    = 150
	optRowY    = 164
	optRowStep = 44
	optCtrlX   = 260
	optBtnH    = 32
	optBarW    = 160
	optNoteY   = 24 // below the last row
	// the second column (Graphics): its labels and controls
	optCol2LabelX = 600
	optCol2CtrlX  = 720
)

func onOff(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}

func volumeRow(key, label string, get func(*config.Audio) *int) optionRow {
	return optionRow{key: key, label: label, kind: rowStep,
		value: func(s config.Settings) string { return fmt.Sprintf("%d%%", *get(&s.Audio)) },
		frac:  func(s config.Settings) float64 { return float64(*get(&s.Audio)) / 100 },
		apply: func(s *config.Settings, d int) { *get(&s.Audio) += d * config.VolumeStep }}
}

func toggleRow(key, label string, get func(*config.Settings) *bool) optionRow {
	return optionRow{key: key, label: label, kind: rowToggle,
		value: func(s config.Settings) string { return onOff(*get(&s)) },
		apply: func(s *config.Settings, _ int) { p := get(s); *p = !*p }}
}

func cycle[T comparable](xs []T, v T) T {
	for i, x := range xs {
		if x == v {
			return xs[(i+1)%len(xs)]
		}
	}
	return xs[0]
}

func autosaveLabel(days int) string {
	switch days {
	case 0:
		return "Off"
	case 1:
		return "Daily"
	case 7:
		return "Weekly"
	}
	return fmt.Sprintf("Every %d days", days)
}

func withTip(r optionRow, tip string) optionRow { r.tip = tip; return r }

// chosenRow makes a graphics row record the player's hand-made change
// (config.Graphics.Choose), so a quality preset never overwrites it.
func chosenRow(r optionRow) optionRow {
	inner, key := r.apply, r.key
	r.apply = func(s *config.Settings, d int) {
		inner(s, d)
		s.Graphics.Choose(key)
	}
	return r
}

// choiceRow is a graphics row that cycles through values (one button).
func choiceRow(key, label, tip string, get func(*config.Settings) *string, values []string) optionRow {
	return chosenRow(optionRow{key: key, label: label, kind: rowChoice, tip: tip,
		value: func(s config.Settings) string { return titleCase(*get(&s)) },
		apply: func(s *config.Settings, _ int) { p := get(s); *p = cycle(values, *p) }})
}

// qualityRow is the quality preset: High, Medium or Low, applied to every
// detail row at once (the hand-picked marks are dropped), and Custom once a
// single row is changed by hand. "Detect again" reads the hardware again.
func qualityRow() optionRow {
	r := optionRow{key: "quality", label: "Quality", kind: rowChoice,
		tip:   "A preset for every detail row. Changing a single row by hand makes it Custom.",
		value: func(s config.Settings) string { return titleCase(s.Graphics.Quality) },
		applyApp: func(a *App, s *config.Settings, _ int) {
			s.Graphics.Chosen = nil // a preset replaces every row, including the hand-picked ones
			s.Graphics.ApplyTier(cycle(config.QualityLevels, s.Graphics.Quality), a.deviceScale())
		}}
	r.extra = &optionAction{key: "detect", label: "Detect again",
		tip: "Read this computer's memory and cores again and apply the preset that fits. Your hand-picked rows are cleared.",
		applyApp: func(a *App, s *config.Settings) {
			s.Graphics.Chosen = nil
			a.applyDetectedTier(s)
		},
		off: func(a *App) string {
			if a.cfg.Hardware == nil {
				return "This computer's hardware can't be read"
			}
			return ""
		}}
	return r
}

// graphicsRows are the Graphics tab's rows: the window's (Fullscreen),
// then the quality preset and the detail rows it sets, each of which
// records a hand-made change and says what it costs.
func graphicsRows() []optionRow {
	flag := func(key, label, tip string, get func(*config.Settings) *bool) optionRow {
		return chosenRow(withTip(toggleRow(key, label, get), tip))
	}
	detail := func(r optionRow) optionRow { r.col = 1; return r }
	return []optionRow{
		qualityRow(),
		toggleRow("fullscreen", "Fullscreen", func(s *config.Settings) *bool { return &s.Graphics.Fullscreen }),
		flag("vsync", "VSync", "Wait for the monitor's refresh before showing a frame. Stops tearing; off can raise the frame rate.",
			func(s *config.Settings) *bool { return &s.Graphics.VSync }),
		flag("hidpi", "Sharp on high-density screens", "Render at the screen's true resolution. Costs GPU time on very large maps.",
			func(s *config.Settings) *bool { return &s.Graphics.HiDPI }),
		withTip(toggleRow("perfoverlay", "Performance overlay", func(s *config.Settings) *bool { return &s.Graphics.PerfOverlay }),
			"Shows fps, 1 % low, frame, update and draw time, sim rate and memory in a small box by the resources. Costs nothing measurable."),
		detail(choiceRow("effects", "Effects", "How many weather, smoke and spark particles draw. Reduced and Minimal cost less CPU and GPU time.",
			func(s *config.Settings) *string { return &s.Graphics.Effects }, config.EffectLevels)),
		detail(flag("shadows", "Shadows", "Soft cast shadows under everything; costs GPU time.",
			func(s *config.Settings) *bool { return &s.Graphics.Shadows })),
		detail(flag("lighting", "Lighting", "Sun, moon and lamp light shaded by each sprite's relief. Off keeps a plain day and night tint; the relief costs GPU time and memory.",
			func(s *config.Settings) *bool { return &s.Graphics.Lighting })),
		detail(choiceRow("water", "Water", "Animated water ripples and glints; simple water is still. Animated costs GPU time.",
			func(s *config.Settings) *string { return &s.Graphics.Water }, []string{config.WaterAnimated, config.WaterSimple})),
		detail(choiceRow("grass", "Grass", "How many tufts grow on each grassy tile. Fewer tufts cost less GPU time.",
			func(s *config.Settings) *string { return &s.Graphics.Grass }, config.GrassLevels)),
		detail(renderScaleRow()),
	}
}

// renderScaleRow is World detail (PERF-002): the share of the physical
// pixels the map draws at (world_scale.go), cycling config.RenderScales.
// Applied live, like HiDPI (applySettings -> updateScale).
func renderScaleRow() optionRow {
	return chosenRow(optionRow{key: "renderscale", label: "World detail", kind: rowChoice,
		tip: "How many pixels the map draws at on a high-density screen: Full, 3/4 or Half. " +
			"Half costs a quarter of the GPU fill; the interface stays sharp.",
		value: func(s config.Settings) string { return renderScaleLabel(s.Graphics.RenderScale) },
		apply: func(s *config.Settings, _ int) {
			s.Graphics.RenderScale = cycle(config.RenderScales, s.Graphics.RenderScale)
		}})
}

// renderScaleLabel names a render scale: Full, 3/4 or Half.
func renderScaleLabel(v float64) string {
	switch v {
	case 0.75:
		return "3/4"
	case 0.5:
		return "Half"
	}
	return "Full"
}

// uiScaleLabel names a UI scale choice; Auto is shown with its live value.
func uiScaleLabel(s config.Settings) string {
	if s.Accessibility.UIScaleAuto {
		return "Auto"
	}
	return fmt.Sprintf("%d%%", s.Accessibility.UIScale)
}

// uiScaleRow cycles Auto, then every preset, then Auto again. The text
// size follows it; there is no separate slider.
func uiScaleRow() optionRow {
	return optionRow{key: "uiscale", label: "UI scale", kind: rowChoice,
		tip:   "Scales text and every panel. Auto fits the window.",
		value: uiScaleLabel,
		live: func(a *App) string {
			if a.settings.Accessibility.UIScaleAuto {
				return fmt.Sprintf("Auto (%d%%)", int(math.Round(a.scale.ui*100)))
			}
			return uiScaleLabel(a.settings)
		},
		apply: func(s *config.Settings, _ int) {
			ac := &s.Accessibility
			if ac.UIScaleAuto {
				ac.UIScaleAuto, ac.UIScale = false, config.UIScalePresets[0]
				return
			}
			for i, p := range config.UIScalePresets {
				if p == ac.UIScale && i+1 < len(config.UIScalePresets) {
					ac.UIScale = config.UIScalePresets[i+1]
					return
				}
			}
			ac.UIScaleAuto = true
		}}
}

// learningHelperRow shows or hides the learning helper (PLAY-018) and, beside
// it, starts its lessons again.
func learningHelperRow() optionRow {
	r := withTip(toggleRow("learnhelper", "Learning helper", func(s *config.Settings) *bool { return &s.Gameplay.ShowLearningHelper }),
		"Short lessons in the top-right corner, done as you play. Turning it off keeps your progress.")
	r.extra = &optionAction{key: "reset", label: "Reset lessons", tip: "Mark every lesson as not learned yet",
		apply: func(s *config.Settings) { s.Gameplay.LearnedLessons = nil }}
	return r
}

// rowValue is the text of a row's control.
func (a *App) rowValue(r optionRow) string {
	if r.live != nil {
		return r.live(a)
	}
	return r.value(a.settings)
}

// optionRows are the rows of a tab.
func optionRows(tab string) []optionRow {
	switch tab {
	case "graphics":
		return graphicsRows()
	case "gameplay":
		return []optionRow{
			toggleRow("pausethreat", "Pause on threat", func(s *config.Settings) *bool { return &s.Gameplay.PauseOnThreat }),
			withTip(toggleRow("runinbackground", "Run in background", func(s *config.Settings) *bool { return &s.Gameplay.RunInBackground }),
				"Keep the colony running while the window is in the background or minimised. Off: the game pauses until you come back."),
			{key: "autosave", label: "Autosave", kind: rowChoice,
				value: func(s config.Settings) string { return autosaveLabel(s.Gameplay.AutosaveDays) },
				apply: func(s *config.Settings, _ int) {
					s.Gameplay.AutosaveDays = cycle(config.AutosaveChoices, s.Gameplay.AutosaveDays)
				}},
			toggleRow("edgescroll", "Edge scrolling", func(s *config.Settings) *bool { return &s.Gameplay.EdgeScroll }),
			learningHelperRow(),
			{key: "panspeed", label: "Camera pan speed", kind: rowStep,
				value: func(s config.Settings) string { return fmt.Sprintf("%d%%", s.Gameplay.PanSpeed) },
				frac: func(s config.Settings) float64 {
					return float64(s.Gameplay.PanSpeed-config.PanSpeedMin) / (config.PanSpeedMax - config.PanSpeedMin)
				},
				apply: func(s *config.Settings, d int) { s.Gameplay.PanSpeed += d * config.PanSpeedStep }},
			toggleRow("devmode", "Developer mode", func(s *config.Settings) *bool { return &s.Gameplay.DevMode }),
		}
	case "controls":
		return nil // controls_ui.go draws its own rows
	case "story":
		return nil // story_tab.go declares its own widgets
	case "access":
		return []optionRow{
			uiScaleRow(),
			toggleRow("reducedmotion", "Reduced motion", func(s *config.Settings) *bool { return &s.Accessibility.ReducedMotion }),
			toggleRow("reducedflash", "Reduced flashing", func(s *config.Settings) *bool { return &s.Accessibility.ReducedFlashing }),
		}
	}
	return []optionRow{
		volumeRow("master", "Master volume", func(a *config.Audio) *int { return &a.Master }),
		volumeRow("music", "Music", func(a *config.Audio) *int { return &a.Music }),
		volumeRow("effects", "Effects", func(a *config.Audio) *int { return &a.Effects }),
		volumeRow("ambience", "Ambience", func(a *config.Audio) *int { return &a.Ambience }),
		volumeRow("ui", "Interface", func(a *config.Audio) *int { return &a.UI }),
		toggleRow("muteunfocused", "Mute when unfocused", func(s *config.Settings) *bool { return &s.Audio.MuteUnfocused }),
	}
}

// optionNotes is the dim line under a tab's rows.
var optionNotes = map[string]string{
	"gameplay": "Developer mode: 4 ultrafast, Numpad + one tick (paused), / debug actions, ` log, ' inspector.",
}

func (a *App) currentOptionsTab() string {
	if _, ok := optionTabLabels[a.optionsTab]; ok && (a.optionsTab != "story" || a.renderer != nil) {
		return a.optionsTab
	}
	return "audio"
}

func rowY(i int) float32 { return float32(optRowY + i*optRowStep) }

// optPos is row i's label x, control x and y: rows stack down their column.
func optPos(rows []optionRow, i int) (labelX, ctrlX, y float32) {
	n := 0
	for _, r := range rows[:i] {
		if r.col == rows[i].col {
			n++
		}
	}
	if rows[i].col == 1 {
		return optCol2LabelX, optCol2CtrlX, rowY(n)
	}
	return 56, optCtrlX, rowY(n)
}

func (a *App) optionsButtons(sw, sh int) []button {
	tab := a.currentOptionsTab()
	var out []button
	for i, t := range a.optionTabList() {
		out = append(out, button{id: "options.tab." + t, label: optionTabLabels[t],
			x: float32(56 + i*(optTabW+8)), y: optTabY, w: optTabW, h: optBtnH, selected: t == tab})
	}
	if tab == "controls" {
		out = append(out, a.controlsButtons(sw, sh)...)
	}
	rows := optionRows(tab)
	for i, r := range rows {
		_, cx, y := optPos(rows, i)
		if r.kind == rowStep {
			out = append(out,
				button{id: "options." + r.key + ".dec", label: "-", x: cx, y: y, w: optBtnH, h: optBtnH},
				button{id: "options." + r.key + ".inc", label: "+", x: cx + optBtnH + 8 + optBarW + 8, y: y, w: optBtnH, h: optBtnH})
			continue
		}
		lock := a.optionLock(r.key) // saves.go: developer mode is locked in commitment mode
		out = append(out, button{id: "options." + r.key, label: a.rowValue(r), x: cx, y: y, w: 160, h: optBtnH,
			disabled: lock != "", reason: lock, tip: r.tip})
		if x := r.extra; x != nil {
			off := ""
			if x.off != nil {
				off = x.off(a)
			}
			out = append(out, button{id: "options." + r.key + "." + x.key, label: x.label, x: cx + 160 + 8, y: y, w: 150, h: optBtnH,
				tip: x.tip, disabled: off != "", reason: off})
		}
	}
	return append(out, button{id: "options.back", label: "Back", x: 40, y: footerY(sh), w: 140, h: 36})
}

// applyOption handles an options.* click: a tab, or a row's control.
func (a *App) applyOption(id string) {
	key := strings.TrimPrefix(id, "options.")
	if t, ok := strings.CutPrefix(key, "tab."); ok {
		a.optionsTab = t
		a.rebind = rebindState{}
		a.cfg.Sound.Play("ui.toggle", 1, 0)
		return
	}
	if strings.HasPrefix(key, "ctl.") {
		a.applyControlsClick(key) // controls_ui.go
		a.cfg.Sound.Play("ui.toggle", 1, 0)
		return
	}
	for _, r := range optionRows(a.currentOptionsTab()) {
		if x := r.extra; x != nil && key == r.key+"."+x.key {
			if x.off != nil && x.off(a) != "" {
				return
			}
			s := a.settings
			if x.applyApp != nil {
				x.applyApp(a, &s)
			} else {
				x.apply(&s)
			}
			a.setSettings(s)
			a.cfg.Sound.Play("ui.toggle", 1, 0)
			return
		}
	}
	d := 0
	if k, ok := strings.CutSuffix(key, ".dec"); ok {
		key, d = k, -1
	} else if k, ok := strings.CutSuffix(key, ".inc"); ok {
		key, d = k, 1
	}
	for _, r := range optionRows(a.currentOptionsTab()) {
		if r.key == key {
			s := a.settings
			if r.applyApp != nil {
				r.applyApp(a, &s, d)
			} else {
				r.apply(&s, d)
			}
			a.setSettings(s)
			a.cfg.Sound.Play("ui.toggle", 1, 0)
			return
		}
	}
}

func (a *App) drawOptions(screen *ebiten.Image, sw, sh int) {
	a.fillMenu(screen, sw, sh, "Options", nil, 0)
	drawPanel(screen, 40, 96, float32(sw-80), float32(sh-180))
	tab := a.currentOptionsTab()
	if tab == "story" { // the kit paints the tab's content over the panel
		mx, my := a.currentCursor()
		a.drawButtons(screen, a.optionsButtons(sw, sh), mx, my)
		return
	}
	if tab == "controls" {
		a.drawControls(screen, sw, sh)
		mx, my := a.currentCursor()
		a.drawButtons(screen, a.optionsButtons(sw, sh)[:len(a.optionTabList())], mx, my)
		a.drawButtons(screen, []button{{id: "options.back", label: "Back", x: 40, y: footerY(sh), w: 140, h: 36}}, mx, my)
		return
	}
	rows := optionRows(tab)
	for i, r := range rows {
		lx, cx, y := optPos(rows, i)
		ty := float64(y) + (optBtnH-lineH(tsBody))/2
		drawText(screen, r.label, float64(lx), ty, tsBody)
		if r.kind != rowStep {
			continue
		}
		bx := cx + optBtnH + 8
		by := y + optBtnH/2 - 6
		uiFillRect(screen, bx, by, optBarW, 12, uiBtnDisabled, false)
		uiFillRect(screen, bx, by, optBarW*float32(clamp01(r.frac(a.settings))), 12, uiAccent, false)
		uiStrokeRect(screen, bx, by, optBarW, 12, 1, uiPanelBorder, false)
		drawText(screen, r.value(a.settings), optCtrlX+2*optBtnH+optBarW+28, ty, tsBody)
	}
	if note := optionNotes[tab]; note != "" {
		ny := float64(rowY(len(rows))) + optNoteY - 20
		for i, ln := range wrapPx(note, tsSmall, float64(sw)-2*56-16) {
			drawText(screen, ln, 56, ny+float64(i)*lineH(tsSmall), withColor(tsSmall, uiTextDim))
		}
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.optionsButtons(sw, sh), mx, my)
}
