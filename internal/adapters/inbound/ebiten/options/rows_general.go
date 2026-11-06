package options

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/openfolder"
)

// generalRows are the General tab's rows (OPT-002…006, 028, 032). The
// Language row (OPT-001) joins the first column with the translation layer.
func generalRows(t lang.Tr) []OptionRow {
	return []OptionRow{
		languageRow(t),
		withTip(toggleRow("pauseonload", "Pause on load", func(s *config.Settings) *bool { return &s.General.PauseOnLoad }),
			"A loaded game starts paused, so you can look around before time moves."),
		listRow("autopause", "Automatic pause", "What pauses the game by itself: nothing, major threats, any threat, or any letter.",
			func(s *config.Settings) *string { return &s.General.AutoPause }, config.AutoPauseModes, autoPauseLabel),
		stepRow("autosaveskept", "Autosaves kept", "How many autosaves a colony keeps. The oldest is written over once they are all in use. They are listed on the Load screen.",
			func(s *config.Settings) *int { return &s.General.AutosavesKept }, 1, config.AutosavesKeptMax, 1,
			func(n int) string { return fmt.Sprintf("%d", n) }),
		withTip(toggleRow("customcursor", "Custom cursor", func(s *config.Settings) *bool { return &s.Camera.CustomCursor }),
			"Draws the game's own pointer instead of the system one."),
		{Key: "names", Label: t.T("Preferred names"), kind: rowChoice, run: editNames,
			Tip:   t.T("Names the new colony screen gives your colonists first, in order. Open to edit the list."),
			value: func(s config.Settings) string { return namesLabel(s.General.PreferredNames) }},
		col2(actionRow("savesfolder", "Saves folder", "Opens the folder that holds your saved games in the file browser.", "Open",
			func(o *Screen) { openFolder(o, "saves") })),
		col2(actionRow("logfolder", "Log folder", "Opens the folder for the game's logs in the file browser.", "Open",
			func(o *Screen) { openFolder(o, "logs") })),
		col2(actionRow("restore", "Restore defaults", "Puts every option back to how it was at first start, after you confirm. Key bindings stay as they are.", "Restore",
			askRestore)),
	}
}

func autoPauseLabel(v string) string {
	switch v {
	case config.AutoPauseMajor:
		return "Major threats"
	case config.AutoPauseThreat:
		return "Any threat"
	case config.AutoPauseLetter:
		return "Any letter"
	}
	return "Never"
}

// Folders (OPT-003).

// folder is where the saves or the logs are: the Env's choice, else the
// per-user config folder the save store uses.
func folder(o *Screen, kind string) string {
	env := o.sh.Env()
	if kind == "logs" && env.LogDir != "" {
		return env.LogDir
	}
	if kind == "saves" && env.SavesDir != "" {
		return env.SavesDir
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "rimworld-go", kind)
}

// openFolder shows the saves or the logs folder in the file browser and
// says in a hint when that did not work.
func openFolder(o *Screen, kind string) {
	path := folder(o, kind)
	open := o.sh.Env().OpenFolder
	if open == nil {
		open = openfolder.Open
	}
	if err := open(path); err != nil {
		o.sh.Hint(o.sh.Tf("cannot open the %s folder: %s", kind, err.Error()))
		return
	}
	o.sh.Hint(o.sh.Tf("opened %s", path))
}

// Restore defaults (OPT-002).

// askRestore asks before putting every option back.
func askRestore(o *Screen) {
	o.sh.Kit().Confirm("options.restore", o.sh.T("Restore all defaults?"),
		o.sh.T("Every option goes back to how it was at first start, graphics included (they are detected again). Key bindings, lessons you finished and your other progress stay."),
		o.sh.T("Restore"), true, func() { restoreDefaults(o) })
}

// restoreDefaults replaces the settings with the defaults, keeping what is
// not a preference: the key bindings (the Controls tab resets those), the
// install id, finished lessons, notices already seen, the last choices of
// the new colony screen, and developer mode in a commitment colony (it is
// locked there). The graphics tier is detected again.
func restoreDefaults(o *Screen) {
	cur, env := o.sh.Settings(), o.sh.Env()
	d := config.Defaults()
	d.Controls = cur.Controls
	d.Telemetry.InstallID = cur.Telemetry.InstallID
	g, k := &d.Gameplay, cur.Gameplay
	g.LearnedLessons, g.RAMNoticeSeen = k.LearnedLessons, k.RAMNoticeSeen
	g.LastStoryteller, g.LastDifficulty, g.LastSaveMode = k.LastStoryteller, k.LastDifficulty, k.LastSaveMode
	if env.Commitment {
		g.DevMode = k.DevMode
	}
	if env.Hardware != nil && env.DeviceScale != nil {
		ApplyDetectedTier(&d, env.Hardware, env.DeviceScale)
	}
	o.sh.SetSettings(d)
}

// Preferred names (OPT-032).

const (
	maxPreferredNames = 12
	maxNameRunes      = 20
	namesModalID      = "options.names"
)

func namesLabel(names []string) string {
	switch len(names) {
	case 0:
		return "None"
	case 1:
		return names[0]
	}
	return fmt.Sprintf("%d names", len(names))
}

// ParseNames reads a comma separated list: names are trimmed, cut to
// maxNameRunes, and a blank or repeated one is dropped (capital letters do
// not make a name different); the first maxPreferredNames stay.
func ParseNames(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == '\n' }) {
		n := strings.Join(strings.Fields(part), " ")
		if r := []rune(n); len(r) > maxNameRunes {
			n = strings.TrimSpace(string(r[:maxNameRunes]))
		}
		if k := strings.ToLower(n); n != "" && !seen[k] {
			seen[k] = true
			out = append(out, n)
		}
	}
	if len(out) > maxPreferredNames {
		out = out[:maxPreferredNames]
	}
	return out
}

func nameRune(r rune) bool {
	return unicode.IsLetter(r) || r == ' ' || r == ',' || r == '-' || r == '\''
}

// editNames opens the list of preferred names in a dialog; OK stores it.
func editNames(o *Screen) {
	o.namesDraft = strings.Join(o.sh.Settings().General.PreferredNames, ", ")
	k := o.sh.Kit()
	const w = 520
	k.Push(uikit.Modal{ID: namesModalID, Title: o.sh.T("Preferred names"), W: w, H: uikit.ModalTitleH + uikit.ModalFooterH + 2*uikit.ModalPad + 96,
		Draw: func(body gfx.Rect) {
			uikit.TextLines(k, body.X, body.Y, body.W,
				o.sh.Tf("Names for new colonists, separated by commas (up to %d). Names already in use are skipped.", maxPreferredNames), uikit.DimText())
			k.TextField(namesModalID+".field", gfx.Rect{X: body.X, Y: body.Y + 52, W: body.W, H: 34}, &o.namesDraft,
				uikit.TextFieldSpec{MaxLen: maxPreferredNames * (maxNameRunes + 2), Valid: nameRune, Placeholder: "Ada, Bo, Chidi"})
		},
		Buttons: []uikit.ModalButton{
			{ID: "ok", Label: o.sh.T("Save"), Style: uikit.BtnPrimary, Closes: true, OnClick: func() { saveNames(o) }},
			{ID: "cancel", Label: o.sh.T("Cancel"), Closes: true},
		}})
}

func saveNames(o *Screen) {
	s := o.sh.Settings()
	s.General.PreferredNames = ParseNames(o.namesDraft)
	o.sh.SetSettings(s)
}

// languageRow cycles the catalogs found in Env.LangDir (OPT-001), showing
// each one's own name; the root loads the catalog when the setting changes.
func languageRow(t lang.Tr) OptionRow {
	return OptionRow{Key: "language", Label: t.T("Language"), kind: rowChoice,
		Tip:   t.T("The language of the interface. Only the catalogs found next to the game are listed."),
		value: func(s config.Settings) string { return langName(nil, s.General.Language) },
		live: func(o *Screen) string {
			return langName(lang.Codes(o.sh.Env().LangDir), o.sh.Settings().General.Language)
		},
		applyApp: func(o *Screen, s *config.Settings, _ int) {
			cs := lang.Codes(o.sh.Env().LangDir)
			for i, c := range cs {
				if c.Code == s.General.Language {
					s.General.Language = cs[(i+1)%len(cs)].Code
					return
				}
			}
			s.General.Language = cs[0].Code
		}}
}

func langName(cs []lang.Choice, code string) string {
	for _, c := range cs {
		if c.Code == code {
			return c.Name
		}
	}
	return code
}
