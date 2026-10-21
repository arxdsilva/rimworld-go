package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

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
}

var optionTabLabels = map[string]string{"audio": "Audio", "graphics": "Graphics", "gameplay": "Gameplay", "access": "Accessibility"}

func optionTabs() []string { return []string{"audio", "graphics", "gameplay", "access"} }

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

// optionRows are the rows of a tab.
func optionRows(tab string) []optionRow {
	switch tab {
	case "graphics":
		return []optionRow{
			toggleRow("fullscreen", "Fullscreen", func(s *config.Settings) *bool { return &s.Graphics.Fullscreen }),
			toggleRow("vsync", "VSync", func(s *config.Settings) *bool { return &s.Graphics.VSync }),
			{key: "effects", label: "Effects", kind: rowChoice,
				value: func(s config.Settings) string {
					return strings.ToUpper(s.Graphics.Effects[:1]) + s.Graphics.Effects[1:]
				},
				apply: func(s *config.Settings, _ int) { s.Graphics.Effects = cycle(config.EffectLevels, s.Graphics.Effects) }},
		}
	case "gameplay":
		return []optionRow{
			toggleRow("pausethreat", "Pause on threat", func(s *config.Settings) *bool { return &s.Gameplay.PauseOnThreat }),
			{key: "autosave", label: "Autosave", kind: rowChoice,
				value: func(s config.Settings) string { return autosaveLabel(s.Gameplay.AutosaveDays) },
				apply: func(s *config.Settings, _ int) {
					s.Gameplay.AutosaveDays = cycle(config.AutosaveChoices, s.Gameplay.AutosaveDays)
				}},
			toggleRow("edgescroll", "Edge scrolling", func(s *config.Settings) *bool { return &s.Gameplay.EdgeScroll }),
			{key: "panspeed", label: "Camera pan speed", kind: rowStep,
				value: func(s config.Settings) string { return fmt.Sprintf("%d%%", s.Gameplay.PanSpeed) },
				frac: func(s config.Settings) float64 {
					return float64(s.Gameplay.PanSpeed-config.PanSpeedMin) / (config.PanSpeedMax - config.PanSpeedMin)
				},
				apply: func(s *config.Settings, d int) { s.Gameplay.PanSpeed += d * config.PanSpeedStep }},
		}
	case "access":
		return []optionRow{
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
	"access": "Text size and UI scale: planned (needs the new font and layout pass).",
}

func (a *App) currentOptionsTab() string {
	if _, ok := optionTabLabels[a.optionsTab]; ok {
		return a.optionsTab
	}
	return "audio"
}

func rowY(i int) float32 { return float32(optRowY + i*optRowStep) }

func (a *App) optionsButtons(sw, sh int) []button {
	tab := a.currentOptionsTab()
	var out []button
	for i, t := range optionTabs() {
		out = append(out, button{id: "options.tab." + t, label: optionTabLabels[t],
			x: float32(56 + i*(optTabW+8)), y: optTabY, w: optTabW, h: optBtnH, selected: t == tab})
	}
	for i, r := range optionRows(tab) {
		y := rowY(i)
		if r.kind == rowStep {
			out = append(out,
				button{id: "options." + r.key + ".dec", label: "-", x: optCtrlX, y: y, w: optBtnH, h: optBtnH},
				button{id: "options." + r.key + ".inc", label: "+", x: optCtrlX + optBtnH + 8 + optBarW + 8, y: y, w: optBtnH, h: optBtnH})
			continue
		}
		out = append(out, button{id: "options." + r.key, label: r.value(a.settings), x: optCtrlX, y: y, w: 160, h: optBtnH})
	}
	return append(out, button{id: "options.back", label: "Back", x: 40, y: footerY(sh), w: 140, h: 36})
}

// applyOption handles an options.* click: a tab, or a row's control.
func (a *App) applyOption(id string) {
	key := strings.TrimPrefix(id, "options.")
	if t, ok := strings.CutPrefix(key, "tab."); ok {
		a.optionsTab = t
		a.cfg.Sound.Play("ui.toggle", 1, 0)
		return
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
			r.apply(&s, d)
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
	rows := optionRows(tab)
	for i, r := range rows {
		y := rowY(i)
		drawLabel(screen, 56, int(y)+11, r.label)
		if r.kind != rowStep {
			continue
		}
		bx := float32(optCtrlX + optBtnH + 8)
		by := y + optBtnH/2 - 6
		vector.FillRect(screen, bx, by, optBarW, 12, uiBtnDisabled, false)
		vector.FillRect(screen, bx, by, optBarW*float32(clamp01(r.frac(a.settings))), 12, uiAccent, false)
		vector.StrokeRect(screen, bx, by, optBarW, 12, 1, uiPanelBorder, false)
		drawLabel(screen, optCtrlX+2*optBtnH+optBarW+28, int(y)+11, r.value(a.settings))
	}
	if note := optionNotes[tab]; note != "" {
		drawLabel(screen, 56, int(rowY(len(rows)))+optNoteY-20, note)
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.optionsButtons(sw, sh), mx, my)
}
