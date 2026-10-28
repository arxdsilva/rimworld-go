package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// Main-tab windows (PLAY-018: WINDOWS owns them): Work, Schedule,
// Research, Animals, Wildlife, Assign, History, Factions and World open
// bottom-left above the tab strip, in the layout's Window rect.
//
// One window is open at a time, and `workOpen` + `panelMode` say which:
//   - "work", "schedule", "research", "animals" (Animals, or Wildlife while
//     animalsWild is set) are drawn the legacy way inside the Window rect
//     (work.go, schedule_ui.go, research_ui.go, animals_ui.go);
//   - any other panelMode is the id of a modeless kit modal (assign,
//     history, factions, world): the modal's definition is in
//     windowState.kit and the kit declares it on the screen layer
//     (ui_modal.go: uiRunWindow).
//
// Because they share workOpen, a tab click, a key or Escape (app.go) closes
// or replaces any window alike, the inspectors and the Architect give way
// while one is up (they all test workOpen), and the layout's Window rect
// exists whenever windowWant is non-zero.

// windowState is this region's presentation state (App.hst).
type windowState struct {
	// kit is the open kit window's definition. It counts as open only while
	// workOpen is set and panelMode is its id (windowKitOpen): Escape and a
	// replacing tab click change those, never this field.
	kit *modal
}

// drawWindows draws the open main-tab window when it is a legacy one (the
// kit windows are painted with the kit, after the HUD).
func (a *App) drawWindows(screen *ebiten.Image, sw, sh int) {
	if a.workOpen && a.legacyWindow() {
		a.drawWork(screen, sw, sh)
	}
}

// legacyWindow reports the open window is one of the four drawn by hand.
func (a *App) legacyWindow() bool {
	switch a.panelMode {
	case "", "work", "schedule", "research", "animals":
		return true
	}
	return false
}

// windowWant is the open window's wanted size (0, 0: none). It feeds the
// layout (hud()), so it never calls a.hud().
func (a *App) windowWant() (float32, float32) {
	if !a.workOpen || a.renderer == nil {
		return 0, 0
	}
	switch a.panelMode {
	case "", "work":
		return a.workWant()
	case "schedule":
		return a.scheduleWant()
	case "research":
		return a.researchWant()
	case "animals":
		return a.animalsWant()
	}
	if a.windowKitOpen() {
		k := a.hst.win.kit
		return k.w, k.h
	}
	return 0, 0
}

// windowRect is the open window's rectangle: the layout's Window rect
// (zero while no window is open).
func (a *App) windowRect() button {
	w := a.hud().Window
	return button{x: w.X, y: w.Y, w: w.W, h: w.H}
}

// windowAnchor is the anchor kit windows give their modal: the Window rect.
func (a *App) windowAnchor() rect { return a.hud().Window }

// applyWindowClick handles a "win." click.
func (a *App) applyWindowClick(id string) {
	if id == "win.close" {
		a.closeWindow()
	}
}

// showWindow opens the legacy window of mode ("work", "schedule",
// "research" or "animals"), replacing any other.
func (a *App) showWindow(mode string) {
	a.workOpen, a.panelMode = true, mode
	a.shell.Do(ports.CloseArchitect{})
	a.closeFloat()
}

// closeWindow closes whichever main-tab window is open.
func (a *App) closeWindow() {
	if a.windowKitOpen() {
		a.uiCloseWindow()
		return
	}
	a.workOpen = false
}

// toggleWildlifeTab opens or closes the Wildlife window (tab.wildlife): the
// Animals tab's wild species list (hotkeys.go openAnimals does the same).
func (a *App) toggleWildlifeTab() {
	if a.workOpen && a.panelMode == "animals" && a.animalsWild {
		a.workOpen = false
		return
	}
	a.showWindow("animals")
	a.animalsWild = true
}

// toggleAnimalsTab opens or closes the Animals window (tab.animals) on the
// colony's tame animals. Wildlife open: the click shows the colony list
// (switching tabs replaces the window; it never just closes it).
func (a *App) toggleAnimalsTab() {
	if a.workOpen && a.panelMode == "animals" && !a.animalsWild {
		a.workOpen = false
		return
	}
	a.showWindow("animals")
	a.animalsWild = false
}

// windowKitOpen reports a kit window is the open main-tab window.
func (a *App) windowKitOpen() bool {
	k := a.hst.win.kit
	return k != nil && a.workOpen && a.panelMode == k.id
}

// uiOpenWindow opens the modeless modal m as the main-tab window,
// replacing whichever is open (the same one already open: nothing).
func (a *App) uiOpenWindow(m modal) {
	if a.windowKitOpen() && a.hst.win.kit.id == m.id {
		return
	}
	a.hst.win.kit = &m
	a.showWindow(m.id)
}

// uiCloseWindow closes the kit window, running its onClose.
func (a *App) uiCloseWindow() {
	if !a.windowKitOpen() {
		return
	}
	k := a.hst.win.kit
	a.closeWindowQuietly()
	if k.onClose != nil {
		k.onClose()
	}
}

// closeWindowQuietly closes the kit window without running its onClose.
func (a *App) closeWindowQuietly() {
	a.workOpen = false
	a.hst.win.kit = nil
}

// toggleKitWindow closes the kit window id when it is the open one, else
// runs open.
func (a *App) toggleKitWindow(id string, open func()) {
	if a.windowKitOpen() && a.hst.win.kit.id == id {
		a.uiCloseWindow()
		return
	}
	open()
}

// mainWindowUI declares the open window's widget-kit parts: the Animals tab
// button's toggle, and a kit window's frame, body and footer.
func (a *App) mainWindowUI(sw, sh int) {
	if a.renderer == nil {
		return
	}
	w := &a.hst.win
	if w.kit != nil && !a.windowKitOpen() {
		w.kit = nil // Escape or another tab closed it
	}
	for _, b := range a.hudTabButtons(sw, sh) {
		if b.id == ids.TabAnimals && !b.disabled { // app.go's toggle does not know Wildlife
			a.uiSt().add(uiWidget{id: b.id, label: b.label, r: rect{X: b.x, Y: b.y, W: b.w, H: b.h}, tip: b.tip, click: a.toggleAnimalsTab})
		}
	}
	if a.workOpen {
		// the wheel over any window scrolls it (or does nothing), never the map
		if r := a.hud().Window; !r.Empty() {
			a.uiSt().add(uiWidget{id: "window.area", r: r, wheel: true})
		}
	}
	if w.kit != nil {
		a.uiRunWindow(*w.kit)
	}
}
