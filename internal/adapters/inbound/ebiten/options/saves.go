package options

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Saved games. The screen only sees saves through the Saver, so it never
// touches files or game internals.

// AutosaveHour is when each in-game day's autosave happens.
const AutosaveHour = 6

// Commitment mode (NG-1 §2.13): the colony has one save, its slot, which
// the game writes itself: every morning at AutosaveHour, and whenever the
// player leaves (Save & Quit to Menu, Return to Main Menu, Quit, closing
// the window). There is no -autosave copy, Load is off in the pause menu
// and developer mode is locked. Reload-anytime colonies keep manual saves
// and the Options autosave interval to <slot>-autosave.
const (
	CommitLoadReason = "Commitment: one timeline"
	commitDevReason  = "Commitment mode"
)

// NewSlotName names a new colony's save slot.
func NewSlotName() string { return "colony-" + time.Now().Format("2006-01-02-1504") }

// SaveNow writes the current colony to slot and reports success (the hint
// says what happened).
func (s *Screen) SaveNow(slot string) bool {
	if !s.sh.Env().CanSave {
		return false
	}
	if err := s.sh.Save(slot); err != nil {
		s.sh.Hint("save failed: " + err.Error())
		return false
	}
	s.sh.Hint("saved " + slot)
	return true
}

// TickAutosave saves at AutosaveHour every Options-chosen number of
// in-game days (0: never) to <slot>-autosave; a commitment colony saves
// its own slot every day, whatever Options say.
func (s *Screen) TickAutosave() {
	env := s.sh.Env()
	if !env.CanSave || env.Capture {
		return
	}
	n, slot := s.sh.Settings().Gameplay.AutosaveDays, s.SaveSlot+"-autosave"
	if env.Commitment {
		n, slot = 1, s.SaveSlot // min(Options, daily), never off
	}
	if n <= 0 {
		return
	}
	d := s.sh.Date()
	day := int(s.sh.Tick()) / 60000
	if d.Hour == AutosaveHour && day-s.AutosaveDay >= n {
		s.AutosaveDay = day
		s.SaveNow(slot)
	}
}

// SaveBeforeLeaving saves a commitment colony as the player leaves it
// (Save & Quit to Menu, Return to Main Menu, Quit). It reports whether
// leaving may go ahead: false only when that save failed, so the player
// stays (the hint says why) instead of losing the day since the last save.
// A reload-anytime colony leaves without saving, as before.
func (s *Screen) SaveBeforeLeaving() bool {
	env := s.sh.Env()
	if !env.Commitment || env.Capture {
		return true
	}
	return s.SaveNow(s.SaveSlot)
}

// optionLock is why an Options row cannot change now ("" when it can): a
// running commitment colony locks developer mode.
func (s *Screen) optionLock(key string) string {
	if key == "devmode" && s.sh.Env().Commitment {
		return commitDevReason
	}
	return ""
}

// RefreshSlots re-reads the save list for the load screen.
func (s *Screen) RefreshSlots() {
	s.Slots = nil
	if !s.sh.Env().CanLoad {
		return
	}
	slots, err := s.sh.List()
	if err != nil {
		s.sh.Hint("cannot read saves: " + err.Error())
		return
	}
	s.Slots = slots
}

// LoadSlot replaces the session with saved game i.
func (s *Screen) LoadSlot(i int) {
	if i < 0 || i >= len(s.Slots) || !s.sh.Env().CanLoad {
		return
	}
	sl := s.Slots[i]
	if sl.Err != "" {
		s.sh.Hint("cannot load: " + sl.Err)
		return
	}
	slot, tick, err := s.sh.Load(sl.Path)
	if err != nil {
		s.sh.Hint("load failed: " + err.Error())
		return
	}
	s.SaveSlot = strings.TrimSuffix(slot, "-autosave")
	s.AutosaveDay = int(tick) / 60000
	s.sh.Hint("loaded " + sl.Name)
}

const (
	slotRowH = 40
	slotRows = 9
)

// commitTag marks a commitment save in the load list.
const commitTag = "Commitment"

// LoadButtons are the Load screen's buttons: Back, then one per slot.
func (s *Screen) LoadButtons(sw, sh int) []uikit.Button {
	out := []uikit.Button{{ID: "load.back", Label: "Back", X: 40, Y: footerY(sh), W: 140, H: 36}}
	x := float32(sw)/2 - 300
	for i, sl := range s.Slots {
		if i >= slotRows {
			break
		}
		// the row's text is drawn by DrawLoad (proportional columns)
		out = append(out, uikit.Button{ID: ids.PrefixLoadSlot + strconv.Itoa(i), Quiet: true,
			X: x, Y: float32(120 + i*(slotRowH+6)), W: 600, H: slotRowH,
			Disabled: sl.Err != "", Reason: sl.Err})
	}
	return out
}

// DrawLoad paints the Load screen.
func (s *Screen) DrawLoad(screen *ebiten.Image, sw, sh int) {
	body := []string{}
	if len(s.Slots) == 0 {
		body = append(body, "No saves yet.", "", "Save from the pause menu (Esc) while playing; the game also", "autosaves every morning at 06:00.")
	}
	buttons := s.LoadButtons(sw, sh)
	s.sh.PaintMenu(screen, sw, sh, "Load Game", body, buttons)
	for i, sl := range s.Slots {
		if i >= slotRows {
			break
		}
		b := buttons[i+1]
		bx, by := float64(b.X), float64(b.Y)
		right := float64(b.X+b.W) - 12
		if !sl.Modified.IsZero() {
			when := sl.Modified.Format("Jan 2 15:04")
			gfx.Text(screen, when, right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.TextDim, Align: gfx.AlignRight})
			right -= float64(gfx.TextW(when, gfx.Small)) + 16
		}
		if sl.Commitment {
			gfx.Text(screen, commitTag, right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.Accent, Align: gfx.AlignRight})
			right -= float64(gfx.TextW(commitTag, gfx.Small)) + 16
		}
		gfx.Text(screen, gfx.Ellipsize(sl.Name, gfx.Label, right-bx-12), bx+12, by+3, gfx.Label)
		sub, c := fmt.Sprintf("%s  %d colonists", sl.Date, sl.Colonists), color.Color(uikit.TextDim)
		if sl.Err != "" {
			sub, c = "cannot load: "+sl.Err, uikit.Danger
		}
		gfx.Text(screen, gfx.Ellipsize(sub, gfx.Small, float64(b.W)-24), bx+12, by+float64(b.H)-gfx.LineH(gfx.Small)-2, gfx.WithColor(gfx.Small, c))
	}
}
