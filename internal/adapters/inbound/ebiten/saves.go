package ebiten

import (
	"fmt"
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

func (a *App) canSave() bool { return a.cfg.Saves.Save != nil && a.renderer != nil }
func (a *App) canLoad() bool { return a.cfg.Saves.List != nil && a.cfg.Saves.Load != nil }

// newSlotName names a new colony's save slot.
func newSlotName() string { return "colony-" + time.Now().Format("2006-01-02-1504") }

// saveNow writes the current colony to its slot.
func (a *App) saveNow(slot string) {
	if !a.canSave() {
		return
	}
	if err := a.cfg.Saves.Save(a.renderer.game, slot); err != nil {
		a.hint = "save failed: " + err.Error()
		return
	}
	a.hint = "saved " + slot
}

// tickAutosave saves at autosaveHour every Options-chosen number of
// in-game days (0: never).
func (a *App) tickAutosave() {
	n := a.settings.Gameplay.AutosaveDays
	if !a.canSave() || a.cfg.Capture != nil || n <= 0 {
		return
	}
	d := a.renderer.game.Date()
	day := int(a.renderer.game.Tick()) / 60000
	if d.Hour == autosaveHour && day-a.autosaveDay >= n {
		a.autosaveDay = day
		a.saveNow(a.saveSlot + "-autosave")
	}
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

func (a *App) loadButtons(sw, sh int) []button {
	out := []button{{id: "load.back", label: "Back", x: 40, y: footerY(sh), w: 140, h: 36}}
	x := float32(sw)/2 - 300
	for i, sl := range a.slots {
		if i >= slotRows {
			break
		}
		label := fmt.Sprintf("%-28s  %s  %d colonists", sl.Name, sl.Date, sl.Colonists)
		if sl.Err != "" {
			label = fmt.Sprintf("%-28s  (cannot load: %s)", sl.Name, sl.Err)
		}
		out = append(out, button{id: "load.slot." + strconv.Itoa(i), label: label,
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
		if sl.Modified.IsZero() {
			continue
		}
		b := buttons[i+1]
		when := sl.Modified.Format("Jan 2 15:04")
		drawLabel(screen, int(b.x+b.w)-len(when)*6-10, int(b.y)+int(b.h)/2-7, when)
	}
}
