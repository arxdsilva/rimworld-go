package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Saved games. The app only sees saves through these hooks (wired in
// main to the file store), so it never touches files or game internals.

// SaveSlot is one save as the load screen lists it.
type SaveSlot struct {
	Name      string
	Path      string
	Date      string // in-game date
	Colonists int
	Modified  time.Time
	Err       string // not loadable, and why
	// Commitment marks a commitment colony's one save (tagged in the list;
	// it loads like any other).
	Commitment bool
}

// SaveHooks connects the UI to saved-game storage. Any nil hook disables
// that feature.
type SaveHooks struct {
	Save func(sim Simulation, slot string) error
	List func() ([]SaveSlot, error)
	Load func(path string) (Simulation, string, error) // sim, slot name
	// SaveCommit saves a commitment-mode colony to its one slot (NG-1 §2.13);
	// nil falls back to Save.
	SaveCommit func(sim Simulation, slot string) error
}

// autosaveHour is when each in-game day's autosave happens.
const autosaveHour = 6

// Commitment mode (NG-1 §2.13): the colony has one save, its slot, which
// the game writes itself: every morning at autosaveHour, and whenever the
// player leaves (Save & Quit to Menu, Return to Main Menu, Quit, closing
// the window). There is no -autosave copy, Load is off in the pause menu
// and developer mode is locked. Reload-anytime colonies keep manual saves
// and the Options autosave interval to <slot>-autosave.
const (
	commitLoadReason = "Commitment: one timeline"
	commitDevReason  = "Commitment mode"
)

// commitment reports a commitment colony is running (read through the
// simulation's story read model).
func (a *App) commitment() bool { return a.renderer != nil && a.renderer.game.Story().Commitment }

// saveHook is how the running colony saves: SaveCommit for a commitment
// colony when wired, else Save.
func (a *App) saveHook() func(Simulation, string) error {
	if a.cfg.Saves.SaveCommit != nil && a.commitment() {
		return a.cfg.Saves.SaveCommit
	}
	return a.cfg.Saves.Save
}

func (a *App) canSave() bool { return a.renderer != nil && a.saveHook() != nil }
func (a *App) canLoad() bool { return a.cfg.Saves.List != nil && a.cfg.Saves.Load != nil }

// newSlotName names a new colony's save slot.
func newSlotName() string { return "colony-" + time.Now().Format("2006-01-02-1504") }

// saveNow writes the current colony to slot and reports success (the hint
// says what happened).
func (a *App) saveNow(slot string) bool {
	if !a.canSave() {
		return false
	}
	if err := a.saveHook()(a.renderer.game, slot); err != nil {
		a.hint = "save failed: " + err.Error()
		return false
	}
	a.hint = "saved " + slot
	return true
}

// tickAutosave saves at autosaveHour every Options-chosen number of
// in-game days (0: never) to <slot>-autosave; a commitment colony saves
// its own slot every day, whatever Options say.
func (a *App) tickAutosave() {
	if !a.canSave() || a.cfg.Capture != nil {
		return
	}
	n, slot := a.settings.Gameplay.AutosaveDays, a.saveSlot+"-autosave"
	if a.commitment() {
		n, slot = 1, a.saveSlot // min(Options, daily), never off
	}
	if n <= 0 {
		return
	}
	d := a.renderer.game.Date()
	day := int(a.renderer.game.Tick()) / 60000
	if d.Hour == autosaveHour && day-a.autosaveDay >= n {
		a.autosaveDay = day
		a.saveNow(slot)
	}
}

// saveBeforeLeaving saves a commitment colony as the player leaves it
// (Save & Quit to Menu, Return to Main Menu, Quit). It reports whether
// leaving may go ahead: false only when that save failed, so the player
// stays (the hint says why) instead of losing the day since the last save.
// A reload-anytime colony leaves without saving, as before.
func (a *App) saveBeforeLeaving() bool {
	if !a.commitment() || a.cfg.Capture != nil {
		return true
	}
	return a.saveNow(a.saveSlot)
}

// windowHooks is the OS window's close handling (faked in tests).
type windowHooks interface {
	SetClosingHandled(bool)
	BeingClosed() bool
}

type ebitenWindow struct{}

func (ebitenWindow) SetClosingHandled(v bool) { ebiten.SetWindowClosingHandled(v) }
func (ebitenWindow) BeingClosed() bool        { return ebiten.IsWindowBeingClosed() }

// appWindow is the window; closeHandled what it was last told. Both are
// package-level because the window is process-wide (one App per window).
var (
	appWindow    windowHooks = ebitenWindow{}
	closeHandled bool
)

// handleWindowClose takes over the window's close button while a
// commitment colony runs, and reports when the game should end: the
// window is being closed, the colony has been saved. It quits even when
// that save fails, so a broken disk never traps the player in the window.
func (a *App) handleWindowClose() bool {
	want := a.commitment() && a.cfg.Capture == nil
	if want != closeHandled {
		appWindow.SetClosingHandled(want)
		closeHandled = want
	}
	if !closeHandled || !appWindow.BeingClosed() {
		return false
	}
	a.saveBeforeLeaving()
	return true
}

// optionLock is why an Options row cannot change now ("" when it can): a
// running commitment colony locks developer mode.
func (a *App) optionLock(key string) string {
	if key == "devmode" && a.commitment() {
		return commitDevReason
	}
	return ""
}

// refreshSlots re-reads the save list for the load screen.
func (a *App) refreshSlots() {
	a.slots = nil
	if !a.canLoad() {
		return
	}
	slots, err := a.cfg.Saves.List()
	if err != nil {
		a.hint = "cannot read saves: " + err.Error()
		return
	}
	a.slots = slots
}

// loadSlot replaces the session with saved game i.
func (a *App) loadSlot(i int) {
	if i < 0 || i >= len(a.slots) || !a.canLoad() {
		return
	}
	sl := a.slots[i]
	if sl.Err != "" {
		a.hint = "cannot load: " + sl.Err
		return
	}
	sim, slot, err := a.cfg.Saves.Load(sl.Path)
	if err != nil {
		a.hint = "load failed: " + err.Error()
		return
	}
	a.dropSession()
	a.startSession(sim)
	a.saveSlot = strings.TrimSuffix(slot, "-autosave")
	a.autosaveDay = int(sim.Tick()) / 60000
	a.hint = "loaded " + sl.Name
}

const (
	slotRowH = 40
	slotRows = 9
)

// commitTag marks a commitment save in the load list.
const commitTag = "Commitment"

func (a *App) loadButtons(sw, sh int) []button {
	out := []button{{id: "load.back", label: "Back", x: 40, y: footerY(sh), w: 140, h: 36}}
	x := float32(sw)/2 - 300
	for i, sl := range a.slots {
		if i >= slotRows {
			break
		}
		// the row's text is drawn by drawLoad (proportional columns)
		out = append(out, button{id: "load.slot." + strconv.Itoa(i), quiet: true,
			x: x, y: float32(120 + i*(slotRowH+6)), w: 600, h: slotRowH,
			disabled: sl.Err != "", reason: sl.Err})
	}
	return out
}

func (a *App) drawLoad(screen *ebiten.Image, sw, sh int) {
	body := []string{}
	if len(a.slots) == 0 {
		body = append(body, "No saves yet.", "", "Save from the pause menu (Esc) while playing; the game also", "autosaves every morning at 06:00.")
	}
	a.paintMenu(screen, sw, sh, "Load Game", body, a.loadButtons(sw, sh))
	buttons := a.loadButtons(sw, sh)
	for i, sl := range a.slots {
		if i >= slotRows {
			break
		}
		b := buttons[i+1]
		bx, by := float64(b.x), float64(b.y)
		right := float64(b.x+b.w) - 12
		if !sl.Modified.IsZero() {
			when := sl.Modified.Format("Jan 2 15:04")
			drawText(screen, when, right, by+4, textStyle{Size: tsSmall.Size, Color: uiTextDim, Align: alignRight})
			right -= float64(textW(when, tsSmall)) + 16
		}
		if sl.Commitment {
			drawText(screen, commitTag, right, by+4, textStyle{Size: tsSmall.Size, Color: uiAccent, Align: alignRight})
			right -= float64(textW(commitTag, tsSmall)) + 16
		}
		drawText(screen, ellipsize(sl.Name, tsLabel, right-bx-12), bx+12, by+3, tsLabel)
		sub, c := fmt.Sprintf("%s  %d colonists", sl.Date, sl.Colonists), color.Color(uiTextDim)
		if sl.Err != "" {
			sub, c = "cannot load: "+sl.Err, uiDanger
		}
		drawText(screen, ellipsize(sub, tsSmall, float64(b.w)-24), bx+12, by+float64(b.h)-lineH(tsSmall)-2, withColor(tsSmall, c))
	}
}
