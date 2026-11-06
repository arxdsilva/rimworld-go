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
// in-game days (0: never) to <slot>-autosave, or with General.AutosavesKept
// above one to the next of <slot>-autosave-1…N (autosaveSlot); a
// commitment colony saves its own slot every day, whatever Options say.
func (s *Screen) TickAutosave() {
	env := s.sh.Env()
	if !env.CanSave || env.Capture {
		return
	}
	n := s.sh.Settings().Gameplay.AutosaveDays
	if env.Commitment {
		n = 1 // min(Options, daily), never off
	}
	if n <= 0 {
		return
	}
	d := s.sh.Date()
	day := int(s.sh.Tick()) / 60000
	if d.Hour == AutosaveHour && day-s.AutosaveDay >= n {
		s.AutosaveDay = day
		slot := s.SaveSlot
		if !env.Commitment {
			slot = autosaveSlot(s, day)
		}
		s.SaveNow(slot)
	}
}

// autosaveSlot is the slot the next autosave of this colony is written
// to. One kept autosave is <slot>-autosave, as it always was. With more it
// is the first of <slot>-autosave-1…N that does not exist, else the one
// written longest ago, so the oldest is overwritten; when the save list
// cannot be read it goes round by the in-game day.
func autosaveSlot(s *Screen, day int) string {
	base := s.SaveSlot + "-autosave"
	kept := s.sh.Settings().General.AutosavesKept
	if kept <= 1 {
		return base
	}
	slots, err := s.sh.List()
	if err != nil || !s.sh.Env().CanLoad {
		return fmt.Sprintf("%s-%d", base, day%kept+1)
	}
	var oldest time.Time
	pick := ""
	for i := 1; i <= kept; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		at, found := time.Time{}, false
		for _, sl := range slots {
			if sl.Name == name {
				at, found = sl.Modified, true
				break
			}
		}
		if !found {
			return name
		}
		if pick == "" || at.Before(oldest) {
			pick, oldest = name, at
		}
	}
	return pick
}

// baseSlot is a colony's slot name from the name of one of its autosaves
// (<slot>-autosave or <slot>-autosave-N).
func baseSlot(name string) string {
	if i := strings.LastIndex(name, "-autosave-"); i >= 0 {
		if _, err := strconv.Atoi(name[i+len("-autosave-"):]); err == nil {
			return name[:i]
		}
	}
	return strings.TrimSuffix(name, "-autosave")
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
	return cardLock(key, s.sh.Settings().HUD)
}

// RefreshSlots re-reads the save list for the load screen.
func (s *Screen) RefreshSlots() {
	s.Slots, s.SlotPage = nil, 0
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
	if i == slotNextPage { // the "more saves" row of a long list
		_, _, pages := s.slotWindow()
		s.SlotPage = (s.SlotPage + 1) % max(1, pages)
		return
	}
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
	s.SaveSlot = baseSlot(slot)
	s.AutosaveDay = int(tick) / 60000
	s.sh.Hint("loaded " + sl.Name)
}

const (
	slotRowH = 40
	slotRows = 9
)

// commitTag marks a commitment save in the load list; EndingTag a save
// whose ship has left (AX-8: the story is complete).
const (
	commitTag = "Commitment"
	EndingTag = "Complete: escaped by ship"
)

// slotNextPage is the index the "more saves" row sends (load.slot.-1): the
// root hands every load.slot.N click to LoadSlot(N).
const slotNextPage = -1

// slotWindow is the saves the Load screen shows now: slots start..start+n
// (all of them up to slotRows; a longer list gives slotRows-1 per page and
// a row to turn the page), and the number of pages.
func (s *Screen) slotWindow() (start, n, pages int) {
	if len(s.Slots) <= slotRows {
		return 0, len(s.Slots), 1
	}
	per := slotRows - 1
	pages = (len(s.Slots) + per - 1) / per
	page := max(0, min(s.SlotPage, pages-1))
	start = page * per
	return start, min(per, len(s.Slots)-start), pages
}

// LoadButtons are the Load screen's buttons: Back, then one per slot shown,
// then, when the list is longer than a page, the row that turns the page.
func (s *Screen) LoadButtons(sw, sh int) []uikit.Button {
	out := []uikit.Button{{ID: "load.back", Label: "Back", X: 40, Y: footerY(sh), W: 140, H: 36}}
	x := float32(sw)/2 - 300
	start, n, pages := s.slotWindow()
	for j := 0; j < n; j++ {
		i, sl := start+j, s.Slots[start+j]
		// the row's text is drawn by DrawLoad (proportional columns)
		out = append(out, uikit.Button{ID: ids.PrefixLoadSlot + strconv.Itoa(i), Quiet: true,
			X: x, Y: float32(120 + j*(slotRowH+6)), W: 600, H: slotRowH,
			Disabled: sl.Err != "", Reason: sl.Err})
	}
	if pages > 1 {
		page := start/(slotRows-1) + 1
		out = append(out, uikit.Button{ID: ids.PrefixLoadSlot + strconv.Itoa(slotNextPage),
			Label: fmt.Sprintf("More saves (page %d of %d)", page, pages),
			X:     x, Y: float32(120 + n*(slotRowH+6)), W: 600, H: slotRowH})
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
	start, n, _ := s.slotWindow()
	for j := 0; j < n; j++ {
		sl := s.Slots[start+j]
		b := buttons[j+1]
		bx, by := float64(b.X), float64(b.Y)
		right := float64(b.X+b.W) - 12
		if !sl.Modified.IsZero() {
			when := sl.Modified.Format("Jan 2 15:04")
			gfx.Text(screen, when, right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.TextDim, Align: gfx.AlignRight})
			right -= float64(gfx.TextW(when, gfx.Small)) + 16
		}
		if sl.Ending != "" {
			gfx.Text(screen, EndingTag, right, by+4, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.Good, Align: gfx.AlignRight})
			right -= float64(gfx.TextW(EndingTag, gfx.Small)) + 16
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
