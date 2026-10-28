package options

import (
	"fmt"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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

type OptionRow struct {
	Key, Label string
	kind       rowKind
	value      func(config.Settings) string
	frac       func(config.Settings) float64   // stepper bar fill, 0..1
	apply      func(s *config.Settings, d int) // d: -1/+1 for steppers, 0 to toggle/cycle
	live       func(s *Screen) string          // value that needs the running app (Auto UI scale); overrides value
	Tip        string                          // hover line
	extra      *optionAction                   // a second button beside the control
	// applyApp, when set, replaces apply for a change that needs the running
	// app (the screen's device scale, the hardware).
	applyApp func(sc *Screen, s *config.Settings, d int)
	// Col is 0 for the first column and 1 for the second (Graphics only).
	Col int
}

// optionAction is a one-shot button on a row (e.g. Reset lessons): it
// changes the settings once and holds no value of its own.
type optionAction struct {
	key, label, tip string
	apply           func(s *config.Settings)
	// applyApp replaces apply when the action needs the running app, and
	// off returns the reason the button is disabled ("" when it works).
	applyApp func(sc *Screen, s *config.Settings)
	off      func(sc *Screen) string
}

var optionTabLabels = map[string]string{"audio": "Audio", "graphics": "Graphics", "gameplay": "Gameplay", "controls": "Controls", "access": "Accessibility", "story": "Story"}

func OptionTabs() []string { return []string{"audio", "graphics", "gameplay", "controls", "access"} }

// TabList is the tabs on offer now: Story (story_tab.go) joins them
// while a colony is running.
func (o *Screen) TabList() []string {
	tabs := OptionTabs()
	if !o.sh.Env().Running {
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
	OptCtrlX   = 260
	OptBtnH    = 32
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

func volumeRow(key, label string, get func(*config.Audio) *int) OptionRow {
	return OptionRow{Key: key, Label: label, kind: rowStep,
		value: func(s config.Settings) string { return fmt.Sprintf("%d%%", *get(&s.Audio)) },
		frac:  func(s config.Settings) float64 { return float64(*get(&s.Audio)) / 100 },
		apply: func(s *config.Settings, d int) { *get(&s.Audio) += d * config.VolumeStep }}
}

func toggleRow(key, label string, get func(*config.Settings) *bool) OptionRow {
	return OptionRow{Key: key, Label: label, kind: rowToggle,
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

func withTip(r OptionRow, tip string) OptionRow { r.Tip = tip; return r }

// chosenRow makes a graphics row record the player's hand-made change
// (config.Graphics.Choose), so a quality preset never overwrites it.
func chosenRow(r OptionRow) OptionRow {
	inner, key := r.apply, r.Key
	r.apply = func(s *config.Settings, d int) {
		inner(s, d)
		s.Graphics.Choose(key)
	}
	return r
}

// choiceRow is a graphics row that cycles through values (one button).
func choiceRow(key, label, tip string, get func(*config.Settings) *string, values []string) OptionRow {
	return chosenRow(OptionRow{Key: key, Label: label, kind: rowChoice, Tip: tip,
		value: func(s config.Settings) string { return titleCase(*get(&s)) },
		apply: func(s *config.Settings, _ int) { p := get(s); *p = cycle(values, *p) }})
}

// qualityRow is the quality preset: High, Medium or Low, applied to every
// detail row at once (the hand-picked marks are dropped), and Custom once a
// single row is changed by hand. "Detect again" reads the hardware again.
func qualityRow() OptionRow {
	r := OptionRow{Key: "quality", Label: "Quality", kind: rowChoice,
		Tip:   "A preset for every detail row. Changing a single row by hand makes it Custom.",
		value: func(s config.Settings) string { return titleCase(s.Graphics.Quality) },
		applyApp: func(o *Screen, s *config.Settings, _ int) {
			s.Graphics.Chosen = nil // a preset replaces every row, including the hand-picked ones
			s.Graphics.ApplyTier(cycle(config.QualityLevels, s.Graphics.Quality), o.sh.Env().DeviceScale())
		}}
	r.extra = &optionAction{key: "detect", label: "Detect again",
		tip: "Read this computer's memory and cores again and apply the preset that fits. Your hand-picked rows are cleared.",
		applyApp: func(o *Screen, s *config.Settings) {
			s.Graphics.Chosen = nil
			env := o.sh.Env()
			ApplyDetectedTier(s, env.Hardware, env.DeviceScale)
		},
		off: func(o *Screen) string {
			if o.sh.Env().Hardware == nil {
				return "This computer's hardware can't be read"
			}
			return ""
		}}
	return r
}

// graphicsRows are the Graphics tab's rows: the window's (Fullscreen),
// then the quality preset and the detail rows it sets, each of which
// records a hand-made change and says what it costs.
func graphicsRows() []OptionRow {
	flag := func(key, label, tip string, get func(*config.Settings) *bool) OptionRow {
		return chosenRow(withTip(toggleRow(key, label, get), tip))
	}
	detail := func(r OptionRow) OptionRow { r.Col = 1; return r }
	return []OptionRow{
		qualityRow(),
		toggleRow("fullscreen", "Fullscreen", func(s *config.Settings) *bool { return &s.Graphics.Fullscreen }),
		flag("vsync", "VSync", "Wait for the monitor's refresh before showing a frame. Stops tearing; off (the default) lets the frame cap pace the game instead of the screen.",
			func(s *config.Settings) *bool { return &s.Graphics.VSync }),
		frameCapRow(),
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

// frameCapRow is Frame cap (PERF-002): the most frames a second the game
// draws with VSync off (background.go frameCapGap), cycling config.FrameCaps;
// 120 on every display by default.
func frameCapRow() OptionRow {
	return chosenRow(OptionRow{Key: "framecap", Label: "Frame cap", kind: rowChoice,
		Tip: "The most frames a second the game draws with VSync off: 120 on every display by default. " +
			"Off draws as fast as the computer can; on macOS the screen's own refresh still bounds it.",
		value: func(s config.Settings) string { return FrameCapLabel(s.Graphics.FrameCap) },
		apply: func(s *config.Settings, _ int) {
			s.Graphics.FrameCap = cycle(config.FrameCaps, s.Graphics.FrameCap)
		}})
}

// FrameCapLabel names a frame cap: "120" or "Off".
func FrameCapLabel(v int) string {
	if v <= 0 {
		return "Off"
	}
	return fmt.Sprintf("%d", v)
}

// renderScaleRow is World detail (PERF-002): the share of the physical
// pixels the map draws at (world_scale.go), cycling config.RenderScales.
// Applied live, like HiDPI (applySettings -> updateScale).
func renderScaleRow() OptionRow {
	return chosenRow(OptionRow{Key: "renderscale", Label: "World detail", kind: rowChoice,
		Tip: "How many pixels the map draws at on a high-density screen: Full, 3/4 or Half. " +
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
func uiScaleRow() OptionRow {
	return OptionRow{Key: "uiscale", Label: "UI scale", kind: rowChoice,
		Tip:   "Scales text and every panel. Auto fits the window.",
		value: uiScaleLabel,
		live: func(o *Screen) string {
			if o.sh.Settings().Accessibility.UIScaleAuto {
				return fmt.Sprintf("Auto (%d%%)", int(math.Round(o.sh.Env().UIScale*100)))
			}
			return uiScaleLabel(o.sh.Settings())
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
func learningHelperRow() OptionRow {
	r := withTip(toggleRow("learnhelper", "Learning helper", func(s *config.Settings) *bool { return &s.Gameplay.ShowLearningHelper }),
		"Short lessons in the top-right corner, done as you play. Turning it off keeps your progress.")
	r.extra = &optionAction{key: "reset", label: "Reset lessons", tip: "Mark every lesson as not learned yet",
		apply: func(s *config.Settings) { s.Gameplay.LearnedLessons = nil }}
	return r
}

// rowValue is the text of a row's control.
func (o *Screen) rowValue(r OptionRow) string {
	if r.live != nil {
		return r.live(o)
	}
	return r.value(o.sh.Settings())
}

// OptionRows are the rows of a tab.
func OptionRows(tab string) []OptionRow {
	switch tab {
	case "graphics":
		return graphicsRows()
	case "gameplay":
		return []OptionRow{
			toggleRow("pausethreat", "Pause on threat", func(s *config.Settings) *bool { return &s.Gameplay.PauseOnThreat }),
			withTip(toggleRow("runinbackground", "Run in background", func(s *config.Settings) *bool { return &s.Gameplay.RunInBackground }),
				"Keep the colony running while the window is in the background or minimised. Off: the game pauses until you come back."),
			{Key: "autosave", Label: "Autosave", kind: rowChoice,
				value: func(s config.Settings) string { return autosaveLabel(s.Gameplay.AutosaveDays) },
				apply: func(s *config.Settings, _ int) {
					s.Gameplay.AutosaveDays = cycle(config.AutosaveChoices, s.Gameplay.AutosaveDays)
				}},
			toggleRow("edgescroll", "Edge scrolling", func(s *config.Settings) *bool { return &s.Gameplay.EdgeScroll }),
			learningHelperRow(),
			{Key: "panspeed", Label: "Camera pan speed", kind: rowStep,
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
		return []OptionRow{
			uiScaleRow(),
			toggleRow("reducedmotion", "Reduced motion", func(s *config.Settings) *bool { return &s.Accessibility.ReducedMotion }),
			toggleRow("reducedflash", "Reduced flashing", func(s *config.Settings) *bool { return &s.Accessibility.ReducedFlashing }),
		}
	}
	return []OptionRow{
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

func (o *Screen) CurrentTab() string {
	if _, ok := optionTabLabels[o.Tab]; ok && (o.Tab != "story" || o.sh.Env().Running) {
		return o.Tab
	}
	return "audio"
}

func rowY(i int) float32 { return float32(optRowY + i*optRowStep) }

// OptPos is row i's label x, control x and y: rows stack down their column.
func OptPos(rows []OptionRow, i int) (labelX, ctrlX, y float32) {
	n := 0
	for _, r := range rows[:i] {
		if r.Col == rows[i].Col {
			n++
		}
	}
	if rows[i].Col == 1 {
		return optCol2LabelX, optCol2CtrlX, rowY(n)
	}
	return 56, OptCtrlX, rowY(n)
}

func (o *Screen) Buttons(sw, sh int) []uikit.Button {
	tab := o.CurrentTab()
	var out []uikit.Button
	for i, t := range o.TabList() {
		out = append(out, uikit.Button{ID: "options.tab." + t, Label: optionTabLabels[t],
			X: float32(56 + i*(optTabW+8)), Y: optTabY, W: optTabW, H: OptBtnH, Selected: t == tab})
	}
	if tab == "controls" {
		out = append(out, o.controlsButtons(sw, sh)...)
	}
	rows := OptionRows(tab)
	for i, r := range rows {
		_, cx, y := OptPos(rows, i)
		if r.kind == rowStep {
			out = append(out,
				uikit.Button{ID: ids.PrefixOptions + r.Key + ".dec", Label: "-", X: cx, Y: y, W: OptBtnH, H: OptBtnH},
				uikit.Button{ID: ids.PrefixOptions + r.Key + ".inc", Label: "+", X: cx + OptBtnH + 8 + optBarW + 8, Y: y, W: OptBtnH, H: OptBtnH})
			continue
		}
		lock := o.optionLock(r.Key) // saves.go: developer mode is locked in commitment mode
		out = append(out, uikit.Button{ID: ids.PrefixOptions + r.Key, Label: o.rowValue(r), X: cx, Y: y, W: 160, H: OptBtnH,
			Disabled: lock != "", Reason: lock, Tip: r.Tip})
		if x := r.extra; x != nil {
			off := ""
			if x.off != nil {
				off = x.off(o)
			}
			out = append(out, uikit.Button{ID: ids.PrefixOptions + r.Key + "." + x.key, Label: x.label, X: cx + 160 + 8, Y: y, W: 150, H: OptBtnH,
				Tip: x.tip, Disabled: off != "", Reason: off})
		}
	}
	return append(out, uikit.Button{ID: ids.OptionsBack, Label: "Back", X: 40, Y: footerY(sh), W: 140, H: 36})
}

// ApplyOption handles an options.* click: a tab, or a row's control.
func (o *Screen) ApplyOption(id string) {
	key := strings.TrimPrefix(id, ids.PrefixOptions)
	if t, ok := strings.CutPrefix(key, "tab."); ok {
		o.Tab = t
		*o.sh.Rebind() = RebindState{}
		o.sh.Cue("ui.toggle")
		return
	}
	if strings.HasPrefix(key, "ctl.") {
		o.applyControlsClick(key) // controls_ui.go
		o.sh.Cue("ui.toggle")
		return
	}
	for _, r := range OptionRows(o.CurrentTab()) {
		if x := r.extra; x != nil && key == r.Key+"."+x.key {
			if x.off != nil && x.off(o) != "" {
				return
			}
			s := o.sh.Settings()
			if x.applyApp != nil {
				x.applyApp(o, &s)
			} else {
				x.apply(&s)
			}
			o.sh.SetSettings(s)
			o.sh.Cue("ui.toggle")
			return
		}
	}
	d := 0
	if k, ok := strings.CutSuffix(key, ".dec"); ok {
		key, d = k, -1
	} else if k, ok := strings.CutSuffix(key, ".inc"); ok {
		key, d = k, 1
	}
	for _, r := range OptionRows(o.CurrentTab()) {
		if r.Key == key {
			s := o.sh.Settings()
			if r.applyApp != nil {
				r.applyApp(o, &s, d)
			} else {
				r.apply(&s, d)
			}
			o.sh.SetSettings(s)
			o.sh.Cue("ui.toggle")
			return
		}
	}
}

func (o *Screen) DrawOptions(screen *ebiten.Image, sw, sh int) {
	o.sh.PaintMenu(screen, sw, sh, "Options", nil, nil)
	uikit.DrawPanel(screen, 40, 96, float32(sw-80), float32(sh-180))
	tab := o.CurrentTab()
	if tab == "story" { // the kit paints the tab's content over the panel
		mx, my := o.sh.Cursor()
		o.drawButtons(screen, o.Buttons(sw, sh), mx, my)
		return
	}
	if tab == "controls" {
		o.drawControls(screen, sw, sh)
		mx, my := o.sh.Cursor()
		o.drawButtons(screen, o.Buttons(sw, sh)[:len(o.TabList())], mx, my)
		o.drawButtons(screen, []uikit.Button{{ID: ids.OptionsBack, Label: "Back", X: 40, Y: footerY(sh), W: 140, H: 36}}, mx, my)
		return
	}
	rows := OptionRows(tab)
	for i, r := range rows {
		lx, cx, y := OptPos(rows, i)
		ty := float64(y) + (OptBtnH-gfx.LineH(gfx.Body))/2
		gfx.Text(screen, r.Label, float64(lx), ty, gfx.Body)
		if r.kind != rowStep {
			continue
		}
		bx := cx + OptBtnH + 8
		by := y + OptBtnH/2 - 6
		gfx.FillRect(screen, bx, by, optBarW, 12, uikit.BtnDisabled, false)
		gfx.FillRect(screen, bx, by, optBarW*float32(gfx.Clamp01(r.frac(o.sh.Settings()))), 12, uikit.Accent, false)
		gfx.StrokeRect(screen, bx, by, optBarW, 12, 1, uikit.PanelBorder, false)
		gfx.Text(screen, r.value(o.sh.Settings()), OptCtrlX+2*OptBtnH+optBarW+28, ty, gfx.Body)
	}
	if note := optionNotes[tab]; note != "" {
		ny := float64(rowY(len(rows))) + optNoteY - 20
		for i, ln := range gfx.WrapPx(note, gfx.Small, float64(sw)-2*56-16) {
			gfx.Text(screen, ln, 56, ny+float64(i)*gfx.LineH(gfx.Small), gfx.WithColor(gfx.Small, uikit.TextDim))
		}
	}
	mx, my := o.sh.Cursor()
	o.drawButtons(screen, o.Buttons(sw, sh), mx, my)
}

// footerYOff is how far above the screen's bottom the footer buttons sit
// (it mirrors the root's; a root test keeps them equal).
const footerYOff = 64

func footerY(sh int) float32 { return float32(sh - footerYOff) }

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
