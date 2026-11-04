package ebiten

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/learn"
	"rimworld-go/internal/domain/sim/command"
)

// journey_dossier_lessons_test.go (RHUD-043…047): the card's lessons finish on
// the real pointer, the real gear, the real tabs and the real Close, with the
// modal up.

func TestJourneyDossierLessons(t *testing.T) {
	j := dossierSetup(t) // rests on the card only for a moment, then the gear
	learned := func(id string) bool { return slices.Contains(j.a.settings.Gameplay.LearnedLessons, id) }
	want := func(id string, done bool) {
		t.Helper()
		if learned(id) != done {
			t.Fatalf("lesson %s done = %v, want %v (learned %v)", id, !done, done, j.a.settings.Gameplay.LearnedLessons)
		}
	}
	want("dossier_gear", true)
	want("dossier_hover", false)
	want("dossier_look", false)

	j.update(learn.LookRestFrames + 2)
	want("dossier_look", true)
	want("dossier_sections", false)
	want("dossier_close", false)

	j.clickID("dossier.setup.tabs.1")
	j.update(2)
	want("dossier_sections", true)

	j.clickID("modal.dossier.setup.close")
	j.update(2)
	want("dossier_close", true)

	// the window is down: a rest on the card teaches
	pane := j.a.hud().Pane
	sc := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int((pane.X+pane.W/2)*sc), int((pane.Y+pane.H/2)*sc)
	j.update(learn.CardRestFrames + 2)
	want("dossier_hover", true)
}

func TestJourneyDossierLessonsNotOfferedWithTheCardOff(t *testing.T) {
	j := newJourney(t)
	dossierOff(j)
	p := j.a.renderer.snapshot()[0]
	dossierClick(j, p.X, p.Y)
	for _, b := range j.a.lrn.Buttons() {
		if strings.HasPrefix(b.ID, "learn.lesson.dossier") {
			t.Fatalf("a card lesson is offered with the card off: %s", b.ID)
		}
	}
	pane := j.a.hud().Pane
	sc := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int((pane.X+pane.W/2)*sc), int((pane.Y+pane.H/2)*sc)
	j.update(learn.CardRestFrames + 2)
	if slices.Contains(j.a.settings.Gameplay.LearnedLessons, "dossier_hover") {
		t.Fatal("the hover lesson finished with no card")
	}
}

// With the card up and a colonist selected, Tab still opens the Architect: the
// card's header buttons are not in the Tab order of the map.
func TestJourneyDossierLeavesTabToTheArchitect(t *testing.T) {
	j, _ := dossierPawn(t, true)
	j.draw(1)
	j.key("Tab", 0)
	j.update(1)
	if !j.a.arch.Open {
		t.Fatal("Tab did not open the Architect with the card up")
	}
	if !j.clickID("arch.cat.structure") {
		t.Fatal("the Architect's categories are not live")
	}
}

// RHUD-100: the hour selector reaches the Schedule window.
func TestJourneyDossierHourManageOpensSchedule(t *testing.T) {
	j, _ := dossierPawn(t, true)
	if !clickCardItem(j, "dossier.d.", "pick.dossier.hour") {
		t.Fatalf("no item opens the hour menu; items %v", cardIDs(j, "dossier.d."))
	}
	j.draw(1)
	pickRow(j, "dossier.hour", 4) // Manage...
	j.update(2)
	if !j.a.win.Open || j.a.win.Mode != "schedule" {
		t.Fatalf("Manage opened window %v mode %q, want Schedule", j.a.win.Open, j.a.win.Mode)
	}
}

// RHUD-092: the card's area selector is the game's list, and hovering a row
// draws that area on the map.
func TestJourneyDossierAreaRowPreviewsOnTheMap(t *testing.T) {
	j := newJourney(t)
	j.g.Enqueue(command.Command{Kind: command.CreateArea})
	j.tick(1)
	areas := j.g.Areas()
	if len(areas) == 0 {
		t.Fatal("no area to preview")
	}
	p := j.a.renderer.snapshot()[0]
	dossierClick(j, p.X, p.Y)
	if !clickCardItem(j, "dossier.d.", architect.AreaPickID) {
		t.Fatalf("no item opens the area list; items %v", cardIDs(j, "dossier.d."))
	}
	j.draw(1)
	w, ok := j.a.uiSt().Find("areapick.row." + strconv.Itoa(int(areas[0].ID)))
	if !ok {
		t.Fatalf("no row for area %d", areas[0].ID)
	}
	cx, cy := w.R.Center()
	sc := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int(cx*sc), int(cy*sc)
	j.update(2)
	if got := j.a.renderer.overlays().Area.Hover; got != int(areas[0].ID) {
		t.Fatalf("hover preview %d, want area %d", got, areas[0].ID)
	}
}
