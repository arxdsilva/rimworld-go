package ebiten

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The colonist card in the game (RHUD): the pane body, the header, the
// floating card and its anchors, the toggle and the rename dialog. Every
// journey turns the card on through the settings first; it ships off.

// dossierOn turns the card on, docked or floating, through the Options rows
// (Accessibility tab) as a player would, and returns to the colony.
func dossierOn(j *journey, docked bool) {
	j.t.Helper()
	dossierSet(j, true, docked)
}

// dossierOff turns the card off through the Options rows: the plain pane.
func dossierOff(j *journey) {
	j.t.Helper()
	dossierSet(j, false, j.a.settings.HUD.Docked)
}

// dossierSet walks to Options and back. The Escapes on the way may drop the
// selection: a journey that needs it clicks the thing again.
func dossierSet(j *journey, on, docked bool) {
	j.t.Helper()
	for i := 0; i < 4 && !hasWidget(j.a, "pause.options") && !j.clickable("pause.options"); i++ {
		j.key("Escape", 0) // the first one may only drop the selection
		j.update(1)
	}
	j.clickID("pause.options")
	if j.a.nav.screen() != ports.ScreenOptions {
		j.t.Fatalf("Options opened %s", j.a.nav.screen())
	}
	j.clickID("options.tab.access")
	if j.a.settings.HUD.Docked != docked {
		if !j.a.settings.HUD.Enabled {
			j.clickID("options.card") // the placement row greys out with the card off
		}
		j.clickID("options.carddock")
	}
	if j.a.settings.HUD.Enabled != on {
		j.clickID("options.card")
	}
	h := j.a.settings.HUD
	if h.Enabled != on || h.Docked != docked {
		j.t.Fatalf("the Options rows left the card enabled=%v docked=%v, want %v and docked=%v", h.Enabled, h.Docked, on, docked)
	}
	for i := 0; i < 3 && j.a.nav.screen() != ports.ScreenColony; i++ {
		j.key("Escape", 0)
		j.update(1)
	}
	for i := 0; i < 2 && j.a.nav.overlay() != overlayNone; i++ {
		j.key("Escape", 0)
	}
	j.update(1)
}

// clickable reports a HUD button or kit widget of the current screen has the id.
func (j *journey) clickable(id string) bool {
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.ID == id {
			return true
		}
	}
	return hasWidget(j.a, id)
}

// dossierClick clicks what stands on a cell.
func dossierClick(j *journey, x, y int) {
	j.t.Helper()
	j.settle()
	j.clickTile(x, y, inputframe.Left, 0)
	j.update(2)
}

// dossierPawn is a journey with the card on and the first colonist selected.
func dossierPawn(t *testing.T, docked bool, opts ...jOpt) (*journey, game.PawnView) {
	t.Helper()
	j := newJourney(t, opts...)
	dossierOn(j, docked)
	p := j.a.renderer.snapshot()[0]
	dossierClick(j, p.X, p.Y)
	if j.a.renderer.Selected() != p.ID {
		t.Fatalf("selected %d, want %d", j.a.renderer.Selected(), p.ID)
	}
	return j, p
}

// rectsMeet reports two rects overlap.
func rectsMeet(a, b gfx.Rect) bool {
	return !a.Empty() && !b.Empty() && a.X < b.Right() && b.X < a.Right() && a.Y < b.Bottom() && b.Y < a.Bottom()
}

// cardIDs are the declared widgets with the prefix.
func cardIDs(j *journey, prefix string) []string {
	var out []string
	for _, w := range j.a.uiSt().Cur {
		if strings.HasPrefix(w.ID, prefix) {
			out = append(out, w.ID)
		}
	}
	return out
}

// pickRow clicks row i of the open picker.
func pickRow(j *journey, picker string, i int) {
	j.t.Helper()
	j.update(1)
	w, ok := j.a.uiSt().Find("modal.pick." + picker + ".list")
	if !ok {
		j.t.Fatalf("picker %q has no list (top %q)", picker, j.a.uiSt().Top())
	}
	s := float32(j.a.scale.S)
	y := w.R.Y + uikit.ListRowH*(float32(i)+0.5)
	j.f.clickAt(j.t, j.a, int((w.R.X+20)*s), int(y*s))
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
}

// clickCardItem clicks the card's items in turn until a click opens modal;
// other clicks (a tab, another selector) are put back.
func clickCardItem(j *journey, prefix, modal string) bool {
	j.t.Helper()
	for i := 0; i < 80; i++ {
		id := fmt.Sprintf("%s%d", prefix, i)
		j.update(1)
		if _, ok := j.a.uiSt().Find(id); !ok {
			continue
		}
		j.clickID(id)
		j.update(1)
		if j.a.uiSt().Top() == modal {
			return true
		}
		for j.a.uiSt().CloseTop() {
		}
		j.a.insp.SelectPaneTab("")
		j.update(1)
	}
	return false
}

func TestJourneyDossierDocked(t *testing.T) {
	j, p := dossierPawn(t, true)
	l := j.a.hud()
	if want := sheet.DockedFor(j.a.settings.HUD, j.a.sh).Pane; l.Pane.H != float32(want.Height) || l.Pane.W != float32(want.TabW*want.MinTabs) {
		t.Fatalf("pane %v, want %d x %d", l.Pane, want.TabW*want.MinTabs, want.Height)
	}
	if !l.Card.Empty() {
		t.Fatalf("a floating card while docked: %v", l.Card)
	}
	if len(cardIDs(j, "dossier.d.")) == 0 {
		t.Fatal("the docked card declared nothing")
	}
	if _, ok := j.a.uiSt().Find(inspectorpkg.PaneBodyID); ok {
		t.Fatal("the plain body's scroll view is still declared under the card")
	}
	j.draw(1)
	// a click on a card row with a tab behind it goes to that tab
	if !hasWidget(j.a, inspectorpkg.HeadTitleID) {
		t.Fatal("the name has no widget")
	}
	j.clickID(inspectorpkg.HeadTitleID)
	j.update(1)
	if got := j.a.insp.PaneOpenTab(); got != "inspector.social" {
		t.Fatalf("clicking the name opened %q", got)
	}
	j.draw(1)
	j.clickID(inspectorpkg.HeadTitleID) // and closes it
	j.update(1)
	if got := j.a.insp.PaneOpenTab(); got != "" {
		t.Fatalf("clicking the name again left tab %q", got)
	}

	// a group, or anything but one pawn or animal, keeps the plain pane
	other, err := j.g.SpawnPawn(10, 4)
	if err != nil {
		t.Fatal(err)
	}
	j.update(1)
	j.a.sel.SetGroup(selection.Pawn, []entity.EntityID{p.ID, other})
	j.update(2)
	if !j.a.sel.Grouped() {
		t.Fatal("no group")
	}
	if hasWidget(j.a, inspectorpkg.HeadNextID) || len(cardIDs(j, "dossier.")) != 0 {
		t.Fatal("a group shows the card")
	}
	if l := j.a.hud(); l.Pane.H != hudlayout.PaneH || l.Pane.W != hudlayout.PaneW {
		t.Fatalf("the group's pane moved: %+v", l.Pane)
	}
	j.draw(1)
	j.a.sel.Clear(selection.None)
	j.update(2)
	if hasWidget(j.a, inspectorpkg.HeadTitleID) || len(cardIDs(j, "dossier.")) != 0 {
		t.Fatal("no selection still shows the card")
	}
	dossierClick(j, p.X, p.Y)
	if !hasWidget(j.a, inspectorpkg.HeadNextID) {
		t.Fatal("the header did not come back")
	}
}

func TestJourneyDossierFloating(t *testing.T) {
	j, p := dossierPawn(t, false)
	l := j.a.hud()
	if l.Card.Empty() {
		t.Fatal("no floating card")
	}
	if rectsMeet(l.Card, l.Pane) || rectsMeet(l.Card, l.PaneTabs) {
		t.Fatalf("the card %v covers the pane %v or its tabs %v", l.Card, l.Pane, l.PaneTabs)
	}
	if len(cardIDs(j, "dossier.f.")) == 0 || len(cardIDs(j, "dossier.d.")) != 0 {
		t.Fatalf("declared: %v", cardIDs(j, "dossier."))
	}
	if !hasWidget(j.a, inspectorpkg.PaneBodyID) {
		t.Fatal("the plain pane lost its body beside the floating card")
	}
	j.draw(1)
	j.draw(1)

	// a press on the card stays there; the same press off it deselects
	s := float32(j.a.scale.S)
	cx, cy := l.Card.Right()-3, l.Card.Bottom()-3
	j.f.clickAt(t, j.a, int(cx*s), int(cy*s))
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	j.update(1)
	if j.a.renderer.Selected() != p.ID {
		t.Fatal("a press on the floating card reached the map")
	}

	// a tab keeps clear of the card
	j.a.insp.OpenPaneTab("bio")
	j.update(2)
	l = j.a.hud()
	if l.Expanded.Empty() || rectsMeet(l.Expanded, l.Card) {
		t.Fatalf("the open tab %v against the card %v", l.Expanded, l.Card)
	}
	j.draw(1)

	// the log in the pane's body
	j.a.settings.HUD.Pane.LogBody = true
	j.a.insp.SelectPaneTab("")
	j.update(2)
	if hasWidget(j.a, inspectorpkg.PaneBodyID) {
		t.Fatal("the log body still declares the plain scroll view")
	}
	j.draw(1)
}

func TestJourneyDossierHeaderCare(t *testing.T) {
	j, p := dossierPawn(t, true)
	for _, id := range []string{inspectorpkg.HeadNextID, inspectorpkg.HeadInfoID, inspectorpkg.HeadRenameID,
		inspectorpkg.HeadThreatID, inspectorpkg.HeadCareID, inspectorpkg.HeadSelfID} {
		if !hasWidget(j.a, id) {
			t.Fatalf("no %s on a colonist", id)
		}
	}
	// care: the picker sends the level
	j.clickID(inspectorpkg.HeadCareID)
	j.update(1)
	if j.a.uiSt().Top() != "pick."+inspectorpkg.HeadCareID {
		t.Fatalf("top modal %q", j.a.uiSt().Top())
	}
	j.draw(1)
	pickRow(j, inspectorpkg.HeadCareID, int(medcare.NumPolicies)-1)
	c, ok := j.lastCmd(command.SetMedicalCare)
	if !ok || c.Pawn != uint64(p.ID) || c.Mode != uint8(medcare.NumPolicies)-1 {
		t.Fatalf("care command %+v, sent %v", c, ok)
	}
	// self-tend flips what the game has
	j.reset()
	h, _ := j.g.Health(p.ID)
	j.clickID(inspectorpkg.HeadSelfID)
	c, ok = j.lastCmd(command.SetSelfTend)
	if !ok || c.Pawn != uint64(p.ID) || c.Enabled == h.SelfTend {
		t.Fatalf("self-tend command %+v, sent %v, was %v", c, ok, h.SelfTend)
	}
	// threat moves to the next response
	j.reset()
	j.clickID(inspectorpkg.HeadThreatID)
	if _, ok := j.lastCmd(command.SetThreatResponse); !ok {
		t.Fatal("the threat button sent nothing")
	}
	// the info button opens the stat sheet
	j.clickID(inspectorpkg.HeadInfoID)
	j.update(1)
	if j.a.uiSt().Top() == "" {
		t.Fatal("the info button opened nothing")
	}
	j.draw(1)
	for j.a.uiSt().CloseTop() {
	}
	j.update(1)
	// next: alone on its cell, a hint; with company, the next thing
	j.a.hint = ""
	j.clickID(inspectorpkg.HeadNextID)
	if j.a.hint == "" || j.a.renderer.Selected() != p.ID {
		t.Fatalf("alone on the cell: hint %q, selected %d", j.a.hint, j.a.renderer.Selected())
	}

	// two on one cell: next goes to the other, and round again
	j2 := newJourney(t, jPawns([2]int{8, 4}, [2]int{8, 4}))
	dossierOn(j2, true)
	dossierClick(j2, 8, 4)
	first := j2.a.renderer.Selected()
	j2.clickID(inspectorpkg.HeadNextID)
	j2.update(1)
	if got := j2.a.renderer.Selected(); got == first || got == 0 {
		t.Fatalf("next selected %d after %d", got, first)
	}
	j2.clickID(inspectorpkg.HeadNextID)
	j2.update(1)
	if got := j2.a.renderer.Selected(); got != first {
		t.Fatalf("next round selected %d, want %d", got, first)
	}
}

// incapableSim is the game with a colonist who cannot doctor.
type incapableSim struct {
	Simulation
	why string
}

func (s incapableSim) IncapableReason(entity.EntityID, work.Type) string { return s.why }

// Self-tend: the reason stops the toggle in the header and on the Health tab
// (RHUD-022); with doctoring off it warns (RHUD-023).
func TestJourneyDossierSelfTendRefused(t *testing.T) {
	j, p := dossierPawn(t, true)
	j.a.renderer.game = incapableSim{j.a.renderer.game, "Cannot do medical work."}
	j.update(1)
	if w, ok := j.a.uiSt().Find(inspectorpkg.HeadSelfID); !ok || !w.Disabled || w.Reason != "Cannot do medical work." {
		t.Fatalf("self-tend widget %+v, found %v", w, ok)
	}
	if j.clickID(inspectorpkg.HeadSelfID) {
		t.Fatal("clicked a disabled toggle")
	}
	if _, ok := j.lastCmd(command.SetSelfTend); ok {
		t.Fatal("self-tend went through for a colonist who cannot doctor")
	}
	// the Health tab's checkbox says the same
	j.a.insp.OpenPaneTab("health")
	j.update(2)
	if w, ok := j.a.uiSt().Find(inspectorpkg.HealthSelfID); !ok || !w.Disabled || w.Reason != "Cannot do medical work." {
		t.Fatalf("Health tab's checkbox %+v, found %v", w, ok)
	}
	j.a.insp.SelectPaneTab("")
	j.a.renderer.game = j.g

	j.a.hint = ""
	j.g.Enqueue(command.Command{Kind: command.SetSelfTend, Pawn: uint64(p.ID), Enabled: false}) // colonists start with it on
	j.g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: uint64(p.ID), Work: command.WorkDoctor, Count: 0})
	j.g.Step(1)
	j.update(2)
	if j.pawn(uint64(p.ID)).WorkDoctor {
		t.Fatal("doctoring is still on")
	}
	j.clickID(inspectorpkg.HeadSelfID)
	if !strings.Contains(j.a.hint, "doctoring switched off") {
		t.Fatalf("hint %q", j.a.hint)
	}
}

// The log body scrolls with the wheel, and takes it from the map.
func TestJourneyDossierLogWheel(t *testing.T) {
	j, p := dossierPawn(t, false)
	j.a.settings.HUD.Pane.LogBody = true
	j.update(2)
	l := j.a.hud()
	g := inspectorpkg.PaneLayout(l.Pane, j.a.insp.SelectionPane())
	cx, cy := g.Body.Center()
	s := float32(j.a.scale.S)
	j.f.Cur.Mx, j.f.Cur.My = int(cx*s), int(cy*s)
	j.a.insp.Pawn().SyncLogFor(p.ID)
	j.f.Cur.WheelY = -1
	j.update(1)
	j.f.Cur.WheelY = 0
	if !j.a.renderer.wheelTaken {
		t.Fatal("the wheel over the log body reached the map")
	}
	if got := j.a.insp.Pawn().LogScroll; got != 3 {
		t.Fatalf("log scroll %d", got)
	}
}

// A tame animal is named and cared for from the header; a mech would not be.
func TestJourneyDossierTameAnimal(t *testing.T) {
	j := newJourney(t)
	id, err := j.g.SpawnAnimal("dustbuck", 12, 6)
	if err != nil {
		t.Fatal(err)
	}
	j.g.TameAnimal(id, "Clover")
	dossierOn(j, true)
	dossierClick(j, 12, 6)
	for _, w := range []string{inspectorpkg.HeadRenameID, inspectorpkg.HeadCareID} {
		if !hasWidget(j.a, w) {
			t.Fatalf("a tame animal has no %s", w)
		}
	}
	if hasWidget(j.a, inspectorpkg.HeadSelfID) || hasWidget(j.a, inspectorpkg.HeadThreatID) {
		t.Fatal("a tame animal has a colonist's button")
	}
	j.clickID(inspectorpkg.HeadRenameID)
	j.update(1)
	j.typeText("Bran") // over the selected name
	j.update(1)
	j.clickID(inspectorpkg.HeadRenameOK)
	j.update(2)
	if c, ok := j.lastCmd(command.SetName); !ok || c.Pawn != uint64(id) || c.Name != "Bran" {
		t.Fatalf("rename %+v, sent %v", c, ok)
	}
	if an := j.world().Animals[0]; an.Name != "Bran" {
		t.Fatalf("name %q", an.Name)
	}
}

// Header buttons by kind: an animal gets no self-tend or threat; a wild one
// neither name nor care.
func TestJourneyDossierHeaderAnimal(t *testing.T) {
	j := newJourney(t)
	if _, err := j.g.SpawnAnimal("dustbuck", 12, 6); err != nil {
		t.Fatal(err)
	}
	dossierOn(j, true)
	dossierClick(j, 12, 6)
	if !hasWidget(j.a, inspectorpkg.HeadInfoID) || len(cardIDs(j, "dossier.d.")) == 0 {
		t.Fatalf("a wild animal has no card: %v", cardIDs(j, "dossier."))
	}
	for _, id := range []string{inspectorpkg.HeadSelfID, inspectorpkg.HeadThreatID, inspectorpkg.HeadCareID, inspectorpkg.HeadRenameID} {
		if hasWidget(j.a, id) {
			t.Fatalf("a wild animal has %s", id)
		}
	}
	j.draw(1)
}

func TestJourneyDossierHourPick(t *testing.T) {
	j, p := dossierPawn(t, true)
	if !clickCardItem(j, "dossier.d.", "pick.dossier.hour") {
		t.Fatalf("no item opens the hour menu; items %v", cardIDs(j, "dossier.d."))
	}
	j.draw(1)
	pickRow(j, "dossier.hour", 3) // Sleep
	c, ok := j.lastCmd(command.SetSchedule)
	if !ok || c.Pawn != uint64(p.ID) || c.Index != int(j.world().Hour) || c.Mode != 3 {
		t.Fatalf("schedule command %+v, sent %v", c, ok)
	}
}

func TestJourneyDossierRename(t *testing.T) {
	j, p := dossierPawn(t, true)
	before := p.Name
	j.clickID(inspectorpkg.HeadRenameID)
	j.update(1)
	if j.a.uiSt().Top() == "" {
		t.Fatal("no rename dialog")
	}
	j.draw(1)
	if !j.a.uiSt().Editing() {
		t.Fatal("the name field is not being edited")
	}
	for range len([]rune(before)) + 2 {
		j.key("Backspace", 0)
	}
	j.typeText("Wren")
	j.update(1)
	j.draw(1)
	j.clickID(inspectorpkg.HeadRenameOK)
	j.update(1)
	c, ok := j.lastCmd(command.SetName)
	if !ok || c.Name != "Wren" || c.Pawn != uint64(p.ID) {
		t.Fatalf("rename command %+v, sent %v", c, ok)
	}
	j.update(2)
	if got := j.pawn(uint64(p.ID)); got.Name != "Wren" {
		t.Fatalf("name after rename: %q", got.Name)
	}
	if j.a.uiSt().Top() != "" {
		t.Fatalf("the dialog stayed open: %q", j.a.uiSt().Top())
	}

	// what the handler would refuse stays home, and the dialog says so
	j.reset()
	j.clickID(inspectorpkg.HeadRenameID)
	j.update(1)
	j.typeText("<#>")
	j.update(1)
	j.draw(1)
	j.clickID(inspectorpkg.HeadRenameOK)
	j.update(1)
	if j.a.uiSt().Top() == "" {
		t.Fatal("OK closed the dialog over a refused name")
	}
	if _, ok := j.lastCmd(command.SetName); ok {
		t.Fatal("a refused name was sent")
	}
}

func TestJourneyDossierToggle(t *testing.T) {
	j := newJourney(t)
	dossierOff(j)
	p := j.a.renderer.snapshot()[0]
	dossierClick(j, p.X, p.Y)
	if hasWidget(j.a, hud.DossierButtonID) {
		t.Fatal("the toggle shows with the card off")
	}
	dossierOn(j, true)
	dossierClick(j, p.X, p.Y)
	j.update(1)
	j.clickID(hud.DossierButtonID)
	j.update(2)
	if j.a.settings.HUD.Docked || j.a.hud().Card.Empty() {
		t.Fatalf("toggle: docked %v, card %v", j.a.settings.HUD.Docked, j.a.hud().Card)
	}
	if len(cardIDs(j, "dossier.f.")) == 0 {
		t.Fatal("the card did not float")
	}
	j.draw(1)
	j.clickID(hud.DossierButtonID)
	j.update(2)
	if !j.a.settings.HUD.Docked || !j.a.hud().Card.Empty() || len(cardIDs(j, "dossier.d.")) == 0 {
		t.Fatalf("toggled back: docked %v, card %v", j.a.settings.HUD.Docked, j.a.hud().Card)
	}
	j.draw(1)
}

// Nine anchors on every screen size: the corner it names, on screen, with the
// offsets clamped to what keeps it there.
func TestJourneyDossierNineAnchors(t *testing.T) {
	for _, sz := range [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}} {
		sw, sh := sz[0], sz[1]
		screen := gfx.Rect{W: float32(sw), H: float32(sh)}
		seen := map[[2]float32]bool{}
		for anchor := range 9 {
			r := hudlayout.CardRect(sw, sh, anchor, 0, 0, 400, 460)
			if r.Empty() || r.X < 0 || r.Y < 0 || r.Right() > screen.W || r.Bottom() > screen.H {
				t.Fatalf("%v anchor %d: %v off the screen", sz, anchor, r)
			}
			seen[[2]float32{r.X, r.Y}] = true
			switch col := anchor % 3; col {
			case 0:
				if r.X > screen.W/3 {
					t.Errorf("%v anchor %d: %v is not on the left", sz, anchor, r)
				}
			case 2:
				if r.Right() < 2*screen.W/3 {
					t.Errorf("%v anchor %d: %v is not on the right", sz, anchor, r)
				}
			}
			minX, maxX, minY, maxY := hudlayout.CardOffsetBounds(sw, sh, anchor, 400, 460)
			if minX > 0 || maxX < 0 || minY > 0 || maxY < 0 {
				t.Errorf("%v anchor %d: bounds %d %d %d %d leave out the rest position", sz, anchor, minX, maxX, minY, maxY)
			}
			for _, o := range [][2]int{{minX, minY}, {maxX, maxY}, {minX - 5000, maxY + 5000}, {maxX + 5000, minY - 5000}} {
				q := hudlayout.CardRect(sw, sh, anchor, o[0], o[1], 400, 460)
				if q.X < 0 || q.Y < 0 || q.Right() > screen.W || q.Bottom() > screen.H {
					t.Errorf("%v anchor %d offset %v: %v off the screen", sz, anchor, o, q)
				}
			}
		}
		if len(seen) != 9 {
			t.Errorf("%v: the nine anchors make %d places", sz, len(seen))
		}
	}
	// and in the game: wherever it sits, the card stays clear of the pane
	j, _ := dossierPawn(t, false)
	for anchor := range 9 {
		j.a.settings.HUD.Card.Anchor = anchor
		j.update(2)
		l := j.a.hud()
		if l.Card.Empty() || rectsMeet(l.Card, l.Pane) || rectsMeet(l.Card, l.PaneTabs) || rectsMeet(l.Card, l.Tabs) {
			t.Errorf("anchor %d: card %v, pane %v, its tabs %v, bottom tabs %v", anchor, l.Card, l.Pane, l.PaneTabs, l.Tabs)
		}
		j.draw(1)
	}
}

// One that dies while selected leaves no panic and no card; dossier_compat_test.go
// has the ids nothing ever had.
func TestJourneyDossierStaleID(t *testing.T) {
	for _, docked := range []bool{true, false} {
		j := newJourney(t)
		dossierOn(j, docked)
		id, err := j.g.SpawnAnimal("dustbuck", 12, 6)
		if err != nil {
			t.Fatal(err)
		}
		j.update(1)
		dossierClick(j, 12, 6)
		if len(cardIDs(j, "dossier.")) == 0 {
			t.Fatalf("docked %v: no card on the animal", docked)
		}
		j.draw(1)
		j.g.KillAnimal(id)
		j.update(3)
		j.draw(1)
	}
}

// With the card off the pane is the pane it was: no spec field, no geometry,
// no widget, no toggle of the card's, the same after it has been on.
func TestJourneyDossierOffIsIdentical(t *testing.T) {
	j := newJourney(t)
	if !j.a.settings.HUD.Enabled {
		t.Fatal("the card ships on")
	}
	dossierOff(j)
	p := j.a.renderer.snapshot()[0]
	dossierClick(j, p.X, p.Y)
	capture := func() (hudlayout.Layout, []string, inspectorpkg.Spec) {
		var hits []string
		for _, w := range j.a.uiSt().Cur {
			hits = append(hits, fmt.Sprintf("%s %v", w.ID, w.R))
		}
		slices.Sort(hits)
		return j.a.hud(), hits, j.a.insp.SelectionPane()
	}
	offL, offHits, offSpec := capture()
	j.draw(1)

	if w, h := j.a.insp.Card().Size(); w != 0 || h != 0 {
		t.Fatalf("the card sizes the pane while off: %v x %v", w, h)
	}
	if offSpec.Restyled || offSpec.Card || offSpec.LogBody || offSpec.TabW != 0 || offSpec.TitleColor.A != 0 ||
		offSpec.TitleTip != "" || offSpec.TitleTab != "" || offSpec.Emblem != "" || offSpec.EmblemTip != "" {
		t.Fatalf("the spec is dressed while off: %+v", offSpec)
	}
	if offL.Pane.W != hudlayout.PaneW || offL.Pane.H != hudlayout.PaneH || !offL.Card.Empty() {
		t.Fatalf("the layout moved: pane %v, card %v", offL.Pane, offL.Card)
	}
	for _, h := range offHits {
		if strings.HasPrefix(h, "pane.head.") || strings.HasPrefix(h, "dossier.") || strings.HasPrefix(h, hud.DossierButtonID) {
			t.Fatalf("a card widget while off: %s", h)
		}
	}

	// a run with the card on, then off again, ends where it began
	dossierOn(j, true)
	dossierClick(j, p.X, p.Y)
	j.update(2)
	j.draw(1)
	j.a.settings.HUD.Enabled = false
	j.update(3)
	backL, backHits, backSpec := capture()
	if !reflect.DeepEqual(backL, offL) {
		t.Fatalf("layout after a round trip:\n%+v\nwas\n%+v", backL, offL)
	}
	if !slices.Equal(backHits, offHits) {
		t.Fatalf("widgets after a round trip:\n%v\nwas\n%v", backHits, offHits)
	}
	if backSpec.Title != offSpec.Title || backSpec.Subtitle != offSpec.Subtitle || len(backSpec.Lines) != len(offSpec.Lines) || len(backSpec.Tabs) != len(offSpec.Tabs) {
		t.Fatal("the spec differs after a round trip")
	}
	// and a shell with no card at all draws the same frame's widgets
	j.a.dos = dossierState{}
	j.update(2)
	_, noneHits, _ := capture()
	if !slices.Equal(noneHits, offHits) {
		t.Fatalf("widgets without a panel:\n%v\nwas\n%v", noneHits, offHits)
	}
}

// The pane's numbers at the smallest supported window (RHUD-003): pinned so a
// change has to be looked at. 626 tall: 30 % less the PaneTabsH (30) tabs is
// 157; 473 wide leaves CmdReserveCols x CmdStride (5 x 80) and HudMargin
// before the right column; the text 10 (12 x 626/720), the tabs 80 wide
// (96 x 10/12).
func TestJourneyDossierGolden1113(t *testing.T) {
	j, _ := dossierPawn(t, true)
	j.a.sw, j.a.sh = 1113, 626
	j.update(3)
	j.img = nil
	l := j.a.hud()
	want := gfx.Rect{X: 0, Y: 435, W: 473, H: 157}
	if l.Pane != want {
		t.Errorf("pane %+v, want %+v", l.Pane, want)
	}
	if wantTabs := (gfx.Rect{X: 0, Y: want.Y - hudlayout.PaneTabsH, W: 473, H: hudlayout.PaneTabsH}); l.PaneTabs != wantTabs {
		t.Errorf("tabs %+v, want %+v", l.PaneTabs, wantTabs)
	}
	if !l.Card.Empty() {
		t.Errorf("card %+v while docked", l.Card)
	}
	if l.Pane.Right() >= l.Global.X {
		t.Errorf("pane %+v under the right column %+v", l.Pane, l.Global)
	}
	spec := j.a.insp.SelectionPane()
	g := inspectorpkg.PaneLayout(l.Pane, spec)
	if h := j.a.dos.panel.Height(true, g.Card.W); h > g.Card.H {
		t.Errorf("the default tree is %v tall in a %v body", h, g.Card.H)
	}
	if l.Pane.H-g.Card.H != sheet.DockMargin {
		t.Errorf("the pane takes %v around the card, sheet.DockedFor counts %d", l.Pane.H-g.Card.H, sheet.DockMargin)
	}
	j.draw(1)
}

// Nothing the card declares shares an id with anything else this frame.
func TestJourneyDossierUniqueIDs(t *testing.T) {
	for _, docked := range []bool{true, false} {
		j, _ := dossierPawn(t, docked)
		seen := map[string]bool{}
		for _, w := range j.a.uiSt().Cur {
			if w.ID == "" {
				continue
			}
			if seen[w.ID] && (strings.HasPrefix(w.ID, "dossier") || strings.HasPrefix(w.ID, "pane.head.")) {
				t.Errorf("docked %v: %q declared twice", docked, w.ID)
			}
			seen[w.ID] = true
		}
		for id := range seen {
			if strings.HasPrefix(id, "dossier") || strings.HasPrefix(id, "pane.head.") {
				continue
			}
			if strings.Contains(id, "pane.head") {
				t.Errorf("a stray id near the header's: %q", id)
			}
		}
	}
}

// A map switch and the return to the menu drop the card's caches (RHUD-503,
// 504), with no panic.
func TestJourneyDossierResets(t *testing.T) {
	j, _ := dossierPawn(t, true)
	if j.a.dos.panel == nil {
		t.Fatal("no panel in a session")
	}
	viewed := j.g.ViewedMap()
	j.a.dos.viewed = viewed + 1 // the map changed under the card
	j.update(2)
	if j.a.dos.viewed != viewed {
		t.Fatalf("the card follows map %d, viewing %d", j.a.dos.viewed, viewed)
	}
	j.draw(1)
	j.a.dropSession()
	if j.a.dos.panel != nil {
		t.Fatal("the panel outlived the session")
	}
}
