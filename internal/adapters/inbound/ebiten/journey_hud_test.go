package ebiten

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// journey_hud_test.go: HUD journeys (QA-001): the colonist bar, the global
// toggles, the overlay keys, the thing keys, search, letters and alerts.
// The debt below is what this file still owes the guard
// (TestEveryInputActionHasAJourney); delete a name here once a journey in
// this file covers it.
//
// view.screenshot (F10) is armed here but never completed: writing the
// file needs ReadPixels on a Draw, which a headless journey cannot do.
// TestJourneyHudScreenshotKeyArms stops at the armed request.
var _ = owe("journey_hud_test.go",
	[]string{},
	[]string{})

// clickWith clicks a HUD button by id with modifiers held (clickID has none).
func (j *journey) clickWith(id string, mods input.Mod) {
	j.t.Helper()
	j.update(1)
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.ID != id {
			continue
		}
		s := float32(j.a.scale.S)
		j.f.Cur.Mx, j.f.Cur.My = int((b.X+b.W/2)*s), int((b.Y+b.H/2)*s)
		j.f.Click(j.t, j.a, mbLeft, mods)
		j.f.Cur.Mx, j.f.Cur.My = -50, -50
		return
	}
	j.t.Fatalf("no button %q", id)
}

// TestJourneyHudBarSelectAndShift: a bar portrait selects its colonist, a
// Shift click on another adds it to a group.
func TestJourneyHudBarSelectAndShift(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 4}))
	ids := selPawnIDs(j)
	bar := func(i int) string { return fmt.Sprintf("bar.pawn.%d", ids[i]) }
	if !j.clickID(bar(0)) {
		t.Fatal("bar slot disabled")
	}
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != ids[0] {
		t.Fatalf("bar click selected %v", j.a.renderer.Selected())
	}
	j.settle()
	j.clickWith(bar(1), input.ModShift)
	if !j.a.grouped() || len(j.a.sel.Group.Ids) != 2 {
		t.Fatalf("Shift on the bar gave group %v, want both", j.a.sel.Group.Ids)
	}
}

// TestJourneyHudToggles: every lower-right toggle flips its flag.
func TestJourneyHudToggles(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	for _, c := range []struct {
		id  string
		get func() bool
	}{
		{"hud.roof", r.roofOverlayOn},
		{"hud.home", r.homeShown},
		{"hud.plan", r.planShown},
		{"hud.rooms", func() bool { return j.a.rooms }},
		{"hud.learn", func() bool { return j.a.settings.Gameplay.ShowLearningHelper }},
		{"hud.bar", func() bool { return j.a.settings.Gameplay.ShowColonistBar }},
		{"hud.rescat", func() bool { return j.a.settings.Gameplay.CategorizedResources }},
		{"hud.power", r.powerShown},
	} {
		before := c.get()
		if !j.clickID(c.id) {
			t.Fatalf("%s is disabled", c.id)
		}
		if c.get() == before {
			t.Errorf("%s did not flip (still %v)", c.id, before)
		}
		j.settle()
		if !j.clickID(c.id) || c.get() != before {
			t.Errorf("%s did not flip back", c.id)
		}
	}
}

// TestJourneyHudHomeAuto: the Auto toggle asks the game to flip home
// auto-expand.
func TestJourneyHudHomeAuto(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	was := j.world().HomeAuto
	if !j.clickID("hud.homeauto") {
		t.Fatal("hud.homeauto is disabled")
	}
	c, ok := j.lastCmd(command.SetHomeAuto)
	if !ok || c.Enabled == was {
		t.Fatalf("SetHomeAuto %+v (sent %v), want Enabled=%v", c, ok, !was)
	}
	if j.world().HomeAuto == was {
		t.Fatal("home auto-expand did not change")
	}
}

// TestJourneyHudOverlayKeys: Shift+R, Shift+P and G toggle their overlays;
// holding Alt over a tile adds the detailed cell lines.
func TestJourneyHudOverlayKeys(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	j.key("R", input.ModShift)
	if !r.roofOverlayOn() {
		t.Fatal("Shift+R did not show the roof overlay")
	}
	j.key("R", input.ModShift)
	if r.roofOverlayOn() {
		t.Fatal("Shift+R again did not hide it")
	}
	j.key("P", input.ModShift)
	if !r.powerShown() {
		t.Fatal("Shift+P did not show the power overlay")
	}
	j.key("P", input.ModShift)
	if r.powerShown() {
		t.Fatal("Shift+P again did not hide it")
	}
	j.key("G", 0)
	if !j.a.rooms {
		t.Fatal("G did not show room stats")
	}
	j.key("G", 0)
	if j.a.rooms {
		t.Fatal("G again did not hide them")
	}

	j.f.MoveTo(j.a, 15, 7)
	j.update(1)
	plain := len(j.a.cellInfoLines())
	if plain == 0 {
		t.Fatal("hovering a tile shows no cell info")
	}
	j.f.Cur.Mods = input.ModAlt
	j.update(1)
	if alt := len(j.a.cellInfoLines()); alt <= plain {
		t.Fatalf("Alt added no detail lines (%d plain, %d with Alt)", plain, alt)
	}
	j.release()
	j.update(1)
	if len(j.a.cellInfoLines()) != plain {
		t.Fatal("releasing Alt kept the detail lines")
	}
}

// TestJourneyHudForbidKey: F forbids the selected stack and again allows it.
func TestJourneyHudForbidKey(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id, _ := j.g.SpawnStack(item.Wood, 10, 12, 2)
	j.clickTile(12, 2, mbLeft, 0)
	j.key("F", 0)
	c, ok := j.lastCmd(command.SetForbidden)
	if !ok || c.Target != uint64(id) || !c.Enabled {
		t.Fatalf("F sent %+v (%v), want a forbid of %d", c, ok, id)
	}
	j.reset()
	j.key("F", 0)
	if c, ok := j.lastCmd(command.SetForbidden); !ok || c.Enabled {
		t.Fatalf("the second F sent %+v (%v), want an allow", c, ok)
	}
}

// TestJourneyHudPowerKeyAndThermostat: V flicks a selected device's power
// switch; the heater pane's steppers set the thermostat.
func TestJourneyHudPowerKeyAndThermostat(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("heater", "steel", "south", 12, 2) {
		t.Fatal("no heater")
	}
	j.clickTile(12, 2, mbLeft, 0)
	if !j.a.sel.HasBuilding {
		t.Fatal("the heater did not select")
	}
	j.key("V", 0)
	if _, ok := j.lastCmd(command.SetPowerWanted); !ok {
		t.Fatal("V sent no SetPowerWanted")
	}
	j.reset()
	if !j.clickID("cmd.power.temp_up") {
		t.Fatal("the +1 button is disabled")
	}
	up, ok := j.lastCmd(command.SetThermostat)
	if !ok {
		t.Fatal("+1 sent no SetThermostat")
	}
	j.reset()
	if !j.clickID("cmd.power.temp_down") {
		t.Fatal("the -1 button is disabled")
	}
	down, ok := j.lastCmd(command.SetThermostat)
	if !ok || down.Count >= up.Count {
		t.Fatalf("-1 sent %+v after +1 sent %+v", down, up)
	}
}

// TestJourneyHudResourcesCollapse: a category header folds its rows.
func TestJourneyHudResourcesCollapse(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.a.settings.Gameplay.CategorizedResources = true
	j.update(1)
	var id string
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if strings.HasPrefix(b.ID, "res.cat.") {
			id = b.ID
			break
		}
	}
	if id == "" {
		t.Fatal("no resource category header")
	}
	cat := strings.TrimPrefix(id, "res.cat.")
	was := j.a.hst.res.collapsed[cat]
	j.clickID(id)
	if j.a.hst.res.collapsed[cat] == was {
		t.Fatalf("clicking %s did not change its collapsed state", id)
	}
}

// TestJourneyHudSearchTypeEnter: Z opens search, typing filters, Enter
// selects the top hit and centres the camera on it.
func TestJourneyHudSearchTypeEnter(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns())
	j.g.SpawnNamedPawn(8, 4, "Ada")
	bo, _ := j.g.SpawnNamedPawn(18, 7, "Bo")
	j.update(1)
	j.key("Z", 0)
	if !j.a.ord.Search.Open {
		t.Fatal("Z did not open search")
	}
	j.typeText("bo")
	j.key("Enter", 0)
	if j.a.ord.Search.Open {
		t.Fatal("Enter left the search open")
	}
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != bo {
		t.Fatalf("Enter selected %v, want Bo (%v)", j.a.renderer.Selected(), bo)
	}
	// The camera was on column 0: Bo's tile is now where the view looks.
	j.draw(2)
	if tx, ty := j.a.renderer.TileAt(j.a.sw/2, j.a.sh/2); tx < 14 {
		t.Fatalf("the camera looks at tile (%d, %d), want it moved to Bo's side", tx, ty)
	}
}

// TestJourneyHudSearchClickResult: a result row can be clicked.
func TestJourneyHudSearchClickResult(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns())
	j.g.SpawnNamedPawn(8, 4, "Ada")
	bo, _ := j.g.SpawnNamedPawn(12, 6, "Bo")
	j.update(1)
	j.key("Z", 0)
	j.typeText("bo")
	if !j.clickID("search.pick.0") {
		t.Fatal("result row disabled")
	}
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != bo {
		t.Fatalf("clicking the row selected %v, want Bo", j.a.renderer.Selected())
	}
}

// TestJourneyHudLetterClickAndDismiss: a letter row opens its window, and a
// right click on the row dismisses it.
func TestJourneyHudLetterClickAndDismiss(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.FireIncident("flu") || len(j.world().Letters) == 0 {
		t.Fatal("the flu outbreak posted no letter")
	}
	j.update(2)
	l := j.world().Letters[0]
	if !j.clickID(fmt.Sprintf("letter.%d", l.ID)) {
		t.Fatal("letter row disabled")
	}
	if j.a.uiModalTop() == "" {
		t.Fatal("clicking the letter opened no window")
	}
	j.key("Escape", 0)
	if j.a.uiModalTop() != "" {
		t.Fatal("Escape left the letter window open")
	}
	rows := j.a.letterRows(j.a.hud())
	if len(rows) == 0 {
		t.Fatal("no letter rows")
	}
	s := float32(j.a.scale.S)
	cx, cy := rows[len(rows)-1].r.Center()
	j.f.Cur.Mx, j.f.Cur.My = int(cx*s), int(cy*s)
	j.f.Click(t, j.a, mbRight, 0)
	c, ok := j.lastCmd(command.DismissLetter)
	if !ok || c.Target != l.ID {
		t.Fatalf("right click sent %+v (%v), want a dismissal of %d", c, ok, l.ID)
	}
	if len(j.world().Letters) != 0 {
		t.Fatal("the letter is still in the stack")
	}
}

// TestJourneyHudAlertJump: an alert row takes the camera to its target.
func TestJourneyHudAlertJump(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.g.Raid(1)
	j.update(2)
	id := ""
	for _, b := range j.a.alertButtons(j.a.sw) {
		id = b.ID
		break
	}
	if id == "" {
		t.Fatal("the raid raised no alert")
	}
	j.lookAt(0, 0)
	r := j.a.renderer
	x0, y0 := r.cam.OffsetX, r.cam.OffsetY
	if !j.clickID(id) {
		t.Fatalf("%s disabled", id)
	}
	j.draw(3)
	if r.cam.OffsetX == x0 && r.cam.OffsetY == y0 {
		t.Fatalf("clicking %s did not move the camera", id)
	}
}

// TestJourneyHudScreenshotKeyArms: F10 requests a screenshot. The file is
// written after a Draw through ReadPixels, which a headless journey cannot
// run, so the test clears the request before any Draw.
func TestJourneyHudScreenshotKeyArms(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.key("F10", 0)
	if !j.a.meter.Shot.Pending {
		t.Fatal("F10 did not arm a screenshot")
	}
	j.a.meter.Shot.Pending = false
}

// TestJourneyHudHaulMark: the item pane's Mark for hauling button marks a found stack.
func TestJourneyHudHaulMark(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	an, err := j.g.SpawnAnimal("dustbuck", 12, 2)
	if err != nil {
		t.Fatal(err)
	}
	j.g.KillAnimal(an)
	var tx, ty int
	found := false
	for _, it := range j.world().Items {
		if it.Unclaimed {
			tx, ty, found = it.X, it.Y, true
		}
	}
	if !found {
		t.Fatal("no found stack to mark")
	}
	j.lookAt(tx, ty)
	j.clickTile(tx, ty, mbLeft, 0)
	if !j.a.sel.HasItem {
		t.Fatal("the stack did not select")
	}
	if !j.clickID("item.haulmark") {
		t.Fatal("the Mark for hauling button is disabled")
	}
	if c, ok := j.lastCmd(command.SetHaulMark); !ok || !c.Enabled {
		t.Fatalf("Mark for hauling sent %+v (%v)", c, ok)
	}
}
