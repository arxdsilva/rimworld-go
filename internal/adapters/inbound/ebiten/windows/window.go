package windows

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Main-tab windows (PLAY-018: WINDOWS owns them): Work, Schedule,
// Research, Animals, Wildlife, Assign, History, Factions and World open
// bottom-left above the tab strip, in the layout's Window rect.
//
// One window is open at a time, and Open + Mode say which:
//   - "work", "schedule", "research", "animals" (Animals, or Wildlife while
//     Wild is set) are drawn by hand inside the Window rect (work.go,
//     schedule.go, research.go, animals.go);
//   - any other Mode is the id of a modeless kit modal (assign, history,
//     factions, world): its definition is Set.modal and the kit declares
//     it on the screen layer (Declare: Kit.RunWindow).
//
// Because they share Open, a tab click, a key or Escape closes or replaces
// any window alike, the inspectors and the Architect give way while one
// is up (they all test Open), and the layout's Window rect exists
// whenever Want is non-zero.

// Draw draws the open main-tab window when it is a hand-drawn one (the
// kit windows are painted with the kit, after the HUD).
func (w *Set) Draw(screen *ebiten.Image, sw, sh int) {
	if w.Open && w.legacy() {
		w.drawLegacy(screen, sw, sh)
	}
}

// legacy reports the open window is one of the four drawn by hand.
func (w *Set) legacy() bool {
	switch w.Mode {
	case "", "work", "schedule", "research", "animals":
		return true
	}
	return false
}

// drawLegacy draws the open hand-drawn window.
func (w *Set) drawLegacy(screen *ebiten.Image, sw, sh int) {
	if !w.live() {
		return
	}
	switch w.Mode {
	case "schedule":
		w.Schedule().draw(screen, sw, sh)
	case "research":
		w.Research().draw(screen, sw, sh)
	case "animals":
		w.Animals().Draw(screen, sw, sh)
	default:
		w.Work().draw(screen, sw, sh)
	}
}

// Want is the open window's wanted size (0, 0: none). It feeds the
// layout (Host.WindowRect), so it never calls WindowRect.
func (w *Set) Want() (float32, float32) {
	if !w.Open || !w.live() {
		return 0, 0
	}
	switch w.Mode {
	case "", "work":
		return w.Work().want()
	case "schedule":
		return w.Schedule().want()
	case "research":
		return w.Research().want()
	case "animals":
		return w.Animals().want()
	}
	if w.KitOpen() {
		return w.modal.W, w.modal.H
	}
	return 0, 0
}

// Rect is the open window's rectangle: the layout's Window rect (zero
// while no window is open).
func (w *Set) Rect() uikit.Button {
	r := w.sh.WindowRect()
	return uikit.Button{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

// Anchor is the anchor kit windows give their modal: the Window rect.
func (w *Set) Anchor() gfx.Rect { return w.sh.WindowRect() }

// Buttons are the open hand-drawn window's buttons (Work, Schedule,
// Research, Animals); a kit window (History, Assign, World, Factions)
// declares its own widgets and has none here.
func (w *Set) Buttons() []uikit.Button {
	if !w.live() || !w.Open || !w.legacy() {
		return nil
	}
	switch w.Mode {
	case "schedule":
		return w.Schedule().buttons()
	case "research":
		return w.Research().Buttons()
	case "animals":
		return w.Animals().Buttons()
	}
	return w.Work().buttons()
}

// Click handles a "win." click.
func (w *Set) Click(id string) {
	if id == "win.close" {
		w.close()
	}
}

// Show opens the window of mode ("work", "schedule", "research",
// "animals" or a kit window's id), replacing any other; the Architect and
// the order menu give way.
func (w *Set) Show(mode string) {
	w.Open, w.Mode = true, mode
	w.sh.Do(ports.CloseArchitect{})
	w.sh.Float(0, 0, nil)
}

// close closes whichever main-tab window is open.
func (w *Set) close() {
	if w.KitOpen() {
		w.CloseKit()
		return
	}
	w.Open = false
}

// ToggleWildlife opens or closes the Wildlife window (tab.wildlife): the
// Animals tab's wild species list (the Animals key does the same).
func (w *Set) ToggleWildlife() {
	if w.Open && w.Mode == "animals" && w.Wild {
		w.Open = false
		return
	}
	w.Show("animals")
	w.Wild = true
}

// ToggleAnimals opens or closes the Animals window (tab.animals) on the
// colony's tame animals. Wildlife open: the click shows the colony list
// (switching tabs replaces the window; it never just closes it).
func (w *Set) ToggleAnimals() {
	if w.Open && w.Mode == "animals" && !w.Wild {
		w.Open = false
		return
	}
	w.Show("animals")
	w.Wild = false
}

// KitOpen reports a kit window is the open main-tab window.
func (w *Set) KitOpen() bool {
	return w.modal != nil && w.Open && w.Mode == w.modal.ID
}

// KitID is the open kit window's id ("" when none is open).
func (w *Set) KitID() string {
	if !w.KitOpen() {
		return ""
	}
	return w.modal.ID
}

// OpenKit opens the modeless modal m as the main-tab window, replacing
// whichever is open (the same one already open: nothing).
func (w *Set) OpenKit(m uikit.Modal) {
	if w.KitID() == m.ID && m.ID != "" {
		return
	}
	w.modal = &m
	w.Show(m.ID)
}

// CloseKit closes the kit window, running its OnClose.
func (w *Set) CloseKit() {
	if !w.KitOpen() {
		return
	}
	k := w.modal
	w.CloseQuietly()
	if k.OnClose != nil {
		k.OnClose()
	}
}

// CloseQuietly closes the kit window without running its OnClose.
func (w *Set) CloseQuietly() {
	w.Open = false
	w.modal = nil
}

// CloseModal removes the modal id without running its OnClose: the kit
// window when it is the open one, else wherever it is in the kit's stack.
func (w *Set) CloseModal(id string) {
	if w.KitID() == id && id != "" {
		w.CloseQuietly()
		return
	}
	w.kit().CloseModal(id)
}

// toggleKit closes the kit window id when it is the open one, else runs
// open.
func (w *Set) toggleKit(id string, open func()) {
	if w.KitID() == id && id != "" {
		w.CloseKit()
		return
	}
	open()
}

// Declare declares the open window's widget-kit parts: the wheel catch
// over the Window rect, and a kit window's frame, body and footer.
func (w *Set) Declare() {
	if !w.live() {
		return
	}
	if w.modal != nil && !w.KitOpen() {
		w.modal = nil // Escape or another tab closed it
	}
	if w.Open {
		// the wheel over any window scrolls it (or does nothing), never the map
		if r := w.sh.WindowRect(); !r.Empty() {
			w.kit().Add(uikit.Widget{ID: "window.area", R: r, Wheel: true})
		}
	}
	if w.modal != nil {
		m := *w.modal
		w.kit().RunWindow(m, w.CloseKit, func() { w.CloseModal(m.ID) })
	}
}
