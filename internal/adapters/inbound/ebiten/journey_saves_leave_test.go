package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// Leaving a reload-anytime colony (SAV-005, SAV-013): Save & Quit saves the
// colony's slot and leaves without a question; Return to Main Menu with
// unsaved progress asks first.

// unsavedLeave, unsavedSave and unsavedCancel are the unsaved-progress
// question's buttons (options/saves leave.go).
const (
	unsavedLeave  = "modal.saves.unsaved.leave"
	unsavedSave   = "modal.saves.unsaved.save"
	unsavedCancel = "modal.saves.unsaved.cancel"
)

// clickToQuit is j.clickID for a button that may end the game: the same
// press and release through the mouse, where ebiten.Termination is the
// expected result (quitFrame), not a failure. It reports whether the
// button was enabled.
func clickToQuit(j *journey, id string) bool {
	j.t.Helper()
	j.update(1)
	x, y, disabled, _, ok := elementAt(j.a, id)
	if !ok {
		j.t.Fatalf("no button %q on %s", id, j.a.nav.screen())
	}
	if disabled {
		return false
	}
	s := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int(x*s), int(y*s)
	j.f.Cur.Press[inputframe.Left], j.f.Cur.Down[inputframe.Left] = true, true
	quitFrame(j.t, j.a)
	j.f.Cur.Down[inputframe.Left], j.f.Cur.Release[inputframe.Left] = false, true
	quitFrame(j.t, j.a)
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	return true
}

// TestJourneySavesSaveAndQuitToMenuReloadAnytime: the pause menu's Save &
// Quit to Menu writes the colony's slot and returns to the main menu, with
// no unsaved-progress question.
func TestJourneySavesSaveAndQuitToMenuReloadAnytime(t *testing.T) {
	t.Parallel()
	j, f := savesJourney(t)
	j.tick(30)
	j.key("Escape", 0)
	if !j.clickID("pause.savequit") {
		t.Fatal("Save & Quit to Menu is disabled")
	}
	if j.a.nav.screen() != ports.ScreenMainMenu || j.a.renderer != nil {
		t.Fatalf("after Save & Quit: %s", j.a.nav.screen())
	}
	if len(f.order) != 1 || f.order[0] != "col" {
		t.Fatalf("saves %v, want [col]", f.order)
	}
	if top := j.a.uiSt().Top(); top != "" {
		t.Fatalf("a question opened: %s", top)
	}
}

// TestJourneySavesSaveAndQuitToDesktopReloadAnytime: Save & Quit to
// Desktop writes the slot and quits.
func TestJourneySavesSaveAndQuitToDesktopReloadAnytime(t *testing.T) {
	t.Parallel()
	j, f := savesJourney(t)
	j.tick(30)
	j.key("Escape", 0)
	if !clickToQuit(j, "pause.savequitos") {
		t.Fatal("Save & Quit to Desktop is disabled")
	}
	if !j.a.shouldQuit || len(f.order) != 1 || f.order[0] != "col" {
		t.Fatalf("quit %v, saves %v", j.a.shouldQuit, f.order)
	}
}

// TestJourneySavesUnsavedProgressConfirm: Return to Main Menu with unsaved
// progress asks; Cancel stays, Leave goes without saving, Save & leave
// saves and goes.
func TestJourneySavesUnsavedProgressConfirm(t *testing.T) {
	t.Parallel()
	j, f := savesJourney(t)
	j.tick(30)
	j.key("Escape", 0)
	j.clickID("pause.menu")
	if j.a.nav.screen() != ports.ScreenColony || j.a.uiSt().Top() != "saves.unsaved" {
		t.Fatalf("Return to Main Menu: %s, modal %q; want the question over the colony", j.a.nav.screen(), j.a.uiSt().Top())
	}
	j.clickID(unsavedCancel)
	if j.a.nav.screen() != ports.ScreenColony || j.a.renderer == nil || j.a.uiSt().Top() != "" || len(f.order) != 0 {
		t.Fatalf("Cancel: %s, modal %q, saves %v", j.a.nav.screen(), j.a.uiSt().Top(), f.order)
	}
	j.clickID("pause.menu")
	j.clickID(unsavedLeave)
	if j.a.nav.screen() != ports.ScreenMainMenu || len(f.order) != 0 {
		t.Fatalf("Leave: %s, saves %v; want the menu and no save", j.a.nav.screen(), f.order)
	}

	j, f = savesJourney(t)
	j.tick(30)
	j.key("Escape", 0)
	j.clickID("pause.menu")
	j.clickID(unsavedSave)
	if j.a.nav.screen() != ports.ScreenMainMenu || len(f.order) != 1 || f.order[0] != "col" {
		t.Fatalf("Save & leave: %s, saves %v", j.a.nav.screen(), f.order)
	}
}
