package ebiten

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	windowspkg "rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// journey_mouse_test.go: the mouse's buttons beyond the left click
// (CTL-010..020) through the real loop: right clicks and drags on the HUD,
// painting, modifier clicks, the middle tap and Escape.

// mouseCenter is where (physical pixels) the HUD button or kit widget with
// this id sits, after one Update to lay the screen out.
func (j *journey) mouseCenter(id string) (int, int) {
	j.t.Helper()
	j.update(1)
	s := float32(j.a.scale.S)
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.ID == id {
			return int((b.X + b.W/2) * s), int((b.Y + b.H/2) * s)
		}
	}
	w, ok := j.a.uiSt().Find(id)
	if !ok {
		j.t.Fatalf("no button or widget %q on %s", id, j.a.nav.screen())
	}
	x, y := w.R.Center()
	return int(x * s), int(y * s)
}

// mouseClick presses and releases btn at a physical pixel and parks the
// cursor off the window.
func (j *journey) mouseClick(btn inputframe.MouseBtn, x, y int, mods input.Mod) {
	j.t.Helper()
	j.f.Cur.Mx, j.f.Cur.My = x, y
	j.f.Click(j.t, j.a, btn, mods)
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
}

// mouseOn clicks btn on the button with this id.
func (j *journey) mouseOn(btn inputframe.MouseBtn, id string, mods input.Mod) {
	j.t.Helper()
	x, y := j.mouseCenter(id)
	j.mouseClick(btn, x, y, mods)
}

// mouseDragOver holds btn down on the first pixel, runs a frame on each
// next one and lets go on the last.
func (j *journey) mouseDragOver(btn inputframe.MouseBtn, mods input.Mod, pts ...[2]int) {
	j.t.Helper()
	f := j.f
	f.Cur.Mods = mods
	f.Cur.Mx, f.Cur.My = pts[0][0], pts[0][1]
	f.Cur.Press[btn], f.Cur.Down[btn] = true, true
	f.Frame(j.t, j.a)
	for _, p := range pts[1:] {
		f.Cur.Mx, f.Cur.My = p[0], p[1]
		f.Frame(j.t, j.a)
	}
	f.Cur.Down[btn], f.Cur.Release[btn] = false, true
	f.Frame(j.t, j.a)
	f.Cur.Mods = 0
	f.Cur.Mx, f.Cur.My = -50, -50
}

// mouseDragIDs drags btn across the buttons with these ids, in order.
func (j *journey) mouseDragIDs(btn inputframe.MouseBtn, ids ...string) {
	j.t.Helper()
	pts := make([][2]int, len(ids))
	for i, id := range ids {
		x, y := j.mouseCenter(id)
		pts[i] = [2]int{x, y}
	}
	j.mouseDragOver(btn, 0, pts...)
}

// barIDs are the bar portraits' button ids in colonist order.
func barIDs(j *journey) []string {
	var out []string
	for _, id := range selPawnIDs(j) {
		out = append(out, fmt.Sprintf("bar.pawn.%d", id))
	}
	return out
}

// CTL-010: a right click on a message dismisses it, and it stays gone.
func TestJourneyMouseRightClickDismissesMessage(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.FireIncident("stray_joins") {
		t.Fatal("the stray incident refused the colony")
	}
	j.update(2)
	rows := j.a.chrome.Right().MessageRows(j.a.hud())
	if len(rows) == 0 {
		t.Fatalf("no message row for %+v", j.world().Messages)
	}
	before := len(rows)
	r := rows[0].R
	s := float32(j.a.scale.S)
	j.mouseClick(inputframe.Right, int((r.X+r.W/2)*s), int((r.Y+r.H/2)*s), 0)
	for i := 0; i < 5; i++ {
		j.draw(1)
		j.update(1)
		if got := len(j.a.chrome.Right().MessageRows(j.a.hud())); got != before-1 {
			t.Fatalf("frame %d: %d message rows, want %d (the notice stays gone)", i, got, before-1)
		}
	}
	if len(j.world().Messages) == 0 {
		t.Fatal("the game forgot the message: the dismissal is the client's")
	}
}

// CTL-011: a right drag from one bar portrait onto another reorders the bar.
func TestJourneyMouseRightDragReordersBar(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{10, 4}, [2]int{12, 4}))
	ids := selPawnIDs(j)
	order := func() []uint64 {
		var out []uint64
		for _, p := range j.g.Snapshot() {
			out = append(out, uint64(p.BarOrder))
		}
		return out
	}
	before := order()
	bar := barIDs(j)
	j.mouseDragIDs(inputframe.Right, bar[0], bar[2])
	c, ok := j.lastCmd(command.SetColonistOrder)
	if !ok || c.Pawn != uint64(ids[0]) {
		t.Fatalf("SetColonistOrder %+v (sent %v), want colonist %d moved", c, ok, ids[0])
	}
	if len(j.cmds) != 1 || j.a.ord.Float.Open {
		t.Fatalf("the bar drag also did something on the map: commands %+v, order menu open %v", j.cmds, j.a.ord.Float.Open)
	}
	j.tick(1)
	after := order()
	if fmt.Sprint(after) == fmt.Sprint(before) {
		t.Fatalf("the bar order read model stayed %v", before)
	}
	if j.pawn(uint64(ids[0])).BarOrder <= j.pawn(uint64(ids[1])).BarOrder {
		t.Fatalf("colonist %d should stand after %d now: orders %v", ids[0], ids[1], after)
	}
}

// CTL-012: a right click on a portrait opens the world view, and the press
// is the bar's alone: no order, no menu.
func TestJourneyMouseRightClickBarOpensWorld(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{10, 4}))
	if j.a.world.Open() {
		t.Fatal("the world view is open before the click")
	}
	j.mouseOn(inputframe.Right, barIDs(j)[1], 0)
	if !j.a.world.Open() {
		t.Fatal("a right click on a bar portrait did not open the world view")
	}
	if len(j.cmds) != 0 || j.a.ord.Float.Open {
		t.Fatalf("the bar press also acted on the map: commands %+v, order menu %v", j.cmds, j.a.ord.Float.Open)
	}
}

// workJourney is three colonists with the Work tab open.
func workJourney(t *testing.T) (*journey, []uint64) {
	t.Helper()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{10, 4}, [2]int{12, 4}))
	var ids []uint64
	for _, id := range selPawnIDs(j) {
		ids = append(ids, uint64(id))
	}
	j.key("F1", 0)
	if !j.a.win.Open || j.a.win.Mode != "work" {
		t.Fatalf("F1 left the Work tab closed (open %v, mode %q)", j.a.win.Open, j.a.win.Mode)
	}
	return j, ids
}

func workCell(pawn uint64, col int) string { return fmt.Sprintf("work.cell.%d.%d", pawn, col) }

// CTL-013: a right click on a Work cell lowers the priority.
func TestJourneyMouseRightClickLowersWorkPriority(t *testing.T) {
	t.Parallel()
	j, ids := workJourney(t)
	const col = 1
	start := windowspkg.WorkPriority(j.pawn(ids[0]), col)
	j.mouseOn(inputframe.Right, workCell(ids[0], col), 0)
	c, ok := j.lastCmd(command.SetWorkPriority)
	want := windowspkg.PrevPriority(start)
	if !ok || c.Pawn != ids[0] || c.Count != want {
		t.Fatalf("SetWorkPriority %+v (sent %v), want priority %d for colonist %d (was %d)", c, ok, want, ids[0], start)
	}
	if len(j.cmds) != 1 {
		t.Fatalf("a right click on a cell sent %+v", j.cmds)
	}
	if got := j.pawn(ids[0]).Priorities[col]; got != want {
		t.Fatalf("read model priority %d, want %d", got, want)
	}
}

// CTL-014: Shift+left on a Work header sets every capable colonist; a plain
// click changes nothing; Shift+right goes the other way.
func TestJourneyMouseWorkHeaderShift(t *testing.T) {
	t.Parallel()
	j, ids := workJourney(t)
	const col = 1
	head := fmt.Sprintf("work.head.%d", col)
	j.mouseOn(inputframe.Left, workCell(ids[2], col), 0) // make the colonists differ
	j.reset()
	j.mouseOn(inputframe.Left, head, 0)
	if len(j.cmds) != 0 {
		t.Fatalf("a plain click on the header sent %+v", j.cmds)
	}
	first := windowspkg.WorkPriority(j.pawn(ids[0]), col)
	want := windowspkg.NextPriority(first)
	j.mouseOn(inputframe.Left, head, input.ModShift)
	for _, id := range ids {
		if got := windowspkg.WorkPriority(j.pawn(id), col); got != want {
			t.Fatalf("Shift+click: colonist %d has priority %d, want everyone at %d", id, got, want)
		}
	}
	prev := windowspkg.PrevPriority(want)
	j.mouseOn(inputframe.Right, head, input.ModShift)
	for _, id := range ids {
		if got := windowspkg.WorkPriority(j.pawn(id), col); got != prev {
			t.Fatalf("Shift+right: colonist %d has priority %d, want everyone at %d", id, got, prev)
		}
	}
}

// CTL-015: a left drag across three Work cells paints the first click's value.
func TestJourneyMouseDragPaintsWorkCells(t *testing.T) {
	t.Parallel()
	j, ids := workJourney(t)
	col := -1
	for c := 0; c < 32 && col < 0; c++ {
		if _, ok := windowspkg.WorkTypeForCol(c); !ok {
			continue
		}
		same := true
		for _, id := range ids {
			p := j.pawn(id)
			same = same && !p.Incapable.Enabled(work.Type(c)) && windowspkg.WorkPriority(p, c) == windowspkg.WorkPriority(j.pawn(ids[0]), c)
		}
		if same {
			col = c
		}
	}
	if col < 0 {
		t.Fatal("no work column where all three colonists are capable and start alike")
	}
	want := windowspkg.NextPriority(windowspkg.WorkPriority(j.pawn(ids[0]), col))
	j.mouseDragIDs(inputframe.Left, workCell(ids[0], col), workCell(ids[1], col), workCell(ids[2], col))
	cs := j.cmdsOf(command.SetWorkPriority)
	if len(cs) != 3 {
		t.Fatalf("a drag across three cells sent %d commands: %+v", len(cs), cs)
	}
	for i, c := range cs {
		if c.Pawn != ids[i] || c.Count != want {
			t.Fatalf("command %d: %+v, want colonist %d at the first click's priority %d", i, c, ids[i], want)
		}
	}
}

// CTL-015: the same in Assign, across three rows of a column.
func TestJourneyMouseDragPaintsAssignRows(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{10, 4}, [2]int{12, 4}))
	ids := selPawnIDs(j)
	j.key("F3", 0)
	if j.a.uiModalTop() != windowspkg.AssignModalID {
		t.Fatalf("F3 opened %q", j.a.uiModalTop())
	}
	first := j.pawn(uint64(ids[0])).CareLevel
	for _, id := range ids {
		if j.pawn(uint64(id)).CareLevel != first {
			t.Fatal("the colonists start with different care levels")
		}
	}
	j.mouseDragIDs(inputframe.Left, windowspkg.AssignCareID(ids[0]), windowspkg.AssignCareID(ids[1]), windowspkg.AssignCareID(ids[2]))
	cs := j.cmdsOf(command.SetMedicalCare)
	if len(cs) != 3 {
		t.Fatalf("a drag across three rows sent %d commands: %+v", len(cs), cs)
	}
	for i, c := range cs {
		if c.Pawn != uint64(ids[i]) || c.Mode == first || c.Mode != cs[0].Mode {
			t.Fatalf("command %d: %+v, want colonist %d at the first click's level %d", i, c, ids[i], cs[0].Mode)
		}
	}
}

// ordersTool opens the Architect's Orders category and right clicks a tool
// card, which lists the whole-map option.
func ordersTool(j *journey, tool string) {
	j.t.Helper()
	j.key("Tab", 0)
	if !j.clickID("arch.cat.orders") {
		j.t.Fatal("Orders is disabled")
	}
	j.mouseOn(inputframe.Right, "arch.tool."+tool, 0)
	if !j.a.ord.Float.Open {
		j.t.Fatalf("a right click on the %s card opened no menu", tool)
	}
	if j.a.arch.ToolID != "" {
		j.t.Fatalf("the right click armed %q", j.a.arch.ToolID)
	}
}

// CTL-016: the menu is the right click's. A left click on an order arms the
// tool and touches nothing on the map.
func TestJourneyMouseLeftClickOnAnOrderArmsIt(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.key("Tab", 0)
	if !j.clickID("arch.cat.orders") {
		t.Fatal("Orders is disabled")
	}
	if !j.clickID("arch.tool.forbid") {
		t.Fatal("Forbid is disabled")
	}
	if j.a.arch.ToolID != "forbid" {
		t.Fatalf("a left click armed %q, want forbid", j.a.arch.ToolID)
	}
	if j.a.ord.Float.Open {
		t.Fatal("a left click opened the menu")
	}
	if c, ok := j.lastCmd(command.DesignateForbid); ok {
		t.Fatalf("a left click sent %+v", c)
	}
}

// CTL-016: right click on Forbid offers "Forbid all on the map".
func TestJourneyMouseForbidAllOnTheMap(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ordersTool(j, "forbid")
	j.pick("Forbid all on the map")
	c, ok := j.lastCmd(command.DesignateForbid)
	w, h := j.g.Width(), j.g.Height()
	if !ok || !c.Enabled || c.X != 0 || c.Y != 0 || c.X2 != w-1 || c.Y2 != h-1 {
		t.Fatalf("DesignateForbid %+v (sent %v), want the whole %dx%d map forbidden", c, ok, w, h)
	}
}

// CTL-016: right click on Hunt offers "Cancel all hunting".
func TestJourneyMouseCancelAllHunting(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ordersTool(j, "hunt")
	j.pick("Cancel all hunting")
	c, ok := j.lastCmd(command.Undesignate)
	w, h := j.g.Width(), j.g.Height()
	if !ok || c.Order != command.OrderHunt || c.X != 0 || c.Y != 0 || c.X2 != w-1 || c.Y2 != h-1 {
		t.Fatalf("Undesignate %+v (sent %v), want every hunt order on the %dx%d map cleared", c, ok, w, h)
	}
}

// middleDrag drags the middle button 40 px right from the map's middle and
// returns how far the camera moved.
func middleDrag(j *journey) float64 {
	j.t.Helper()
	r := j.a.renderer
	x0 := r.cam.OffsetX
	mx, my := j.a.sw/2, j.a.sh/2
	j.mouseDragOver(inputframe.Middle, 0, [2]int{mx, my}, [2]int{mx + 40, my})
	return r.cam.OffsetX - x0
}

// CTL-017: with a build tool held a middle tap turns it a quarter; a middle
// drag pans the map and does not turn it.
func TestJourneyMouseMiddleTapRotatesTool(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("furniture", "bed")
	before := j.a.arch.Facing
	j.clickTile(10, 3, inputframe.Middle, 0)
	if want := architect.RotateFacing(before, 1); j.a.arch.Facing != want {
		t.Fatalf("a middle tap turned the bed tool to %d, want %d", j.a.arch.Facing, want)
	}
	if len(j.cmds) != 0 {
		t.Fatalf("the middle tap sent %+v", j.cmds)
	}
	turned := j.a.arch.Facing
	if moved := middleDrag(j); moved > -39 || moved < -41 {
		t.Fatalf("the middle drag moved the camera by %v, want -40", moved)
	}
	if j.a.arch.Facing != turned {
		t.Fatalf("a middle drag turned the tool from %d to %d", turned, j.a.arch.Facing)
	}
}

// CTL-017: the same for a building in Move mode.
func TestJourneyMouseMiddleTapRotatesMovedBuilding(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("bed", "wood", "south", 9, 5) {
		t.Fatal("no bed")
	}
	j.clickTile(9, 5, inputframe.Left, 0)
	if !j.clickID("bldg.move") || !j.a.arch.Mover.State().On {
		t.Fatal("Move mode did not start")
	}
	before := j.a.arch.Mover.State().Facing
	j.clickTile(13, 3, inputframe.Middle, 0)
	if want := architect.RotateFacing(before, 1); j.a.arch.Mover.State().Facing != want {
		t.Fatalf("a middle tap turned the moved bed to %d, want %d", j.a.arch.Mover.State().Facing, want)
	}
	if len(j.cmds) != 0 {
		t.Fatalf("the middle tap sent %+v", j.cmds)
	}
	turned := j.a.arch.Mover.State().Facing
	middleDrag(j)
	if j.a.arch.Mover.State().Facing != turned {
		t.Fatalf("a middle drag turned the moved bed from %d to %d", turned, j.a.arch.Mover.State().Facing)
	}
}

// CTL-019: hovering a wild animal's row with the real shell (which has to
// satisfy the window's camera interface) gives an arrow to the animal while
// it is on screen, and none once it is not.
func TestJourneyMouseWildRowArrow(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(100, 24))
	if _, err := j.g.SpawnAnimal("deer", 14, 6); err != nil {
		t.Fatal(err)
	}
	j.key("F5", 0)
	if !j.a.win.Open || !j.a.win.Wild {
		t.Fatalf("F5 left mode %q wild %v", j.a.win.Mode, j.a.win.Wild)
	}
	if _, _, _, _, ok := j.a.win.Animals().WildArrow(); ok {
		t.Fatal("an arrow with the cursor off the list")
	}
	x, y := j.mouseCenter("animals.focus.deer")
	j.f.Cur.Mx, j.f.Cur.My = x, y
	j.update(1)
	x0, y0, x1, y1, ok := j.a.win.Animals().WildArrow()
	if !ok {
		t.Fatal("hovering the deer row gave no arrow to the visible deer")
	}
	if x1 < 0 || y1 < 0 || x1 >= float32(j.a.sw) || y1 >= float32(j.a.sh) || x0 == x1 && y0 == y1 {
		t.Fatalf("arrow (%v, %v) to (%v, %v) is not from the row to a spot on screen", x0, y0, x1, y1)
	}
	j.draw(2) // the arrow draws without a panic
	j.lookAt(80, 6)
	j.update(1)
	if _, _, _, _, ok := j.a.win.Animals().WildArrow(); ok {
		t.Fatal("an arrow to a deer that is off screen")
	}
}

// CTL-020: Escape clears a lone selected animal, item and plant (each
// takes one Escape and none opens the pause menu); the next one does.
func TestJourneyMouseEscapeClearsLoneSelections(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if _, err := j.g.SpawnAnimal("deer", 10, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnStack(item.Wood, 10, 12, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnPlant(14, 2); err != nil {
		t.Fatal(err)
	}
	j.update(1)
	for _, c := range []struct {
		name string
		x    int
		has  func() bool
	}{
		{"animal", 10, func() bool { return j.a.sel.HasAnimal }},
		{"item", 12, func() bool { return j.a.sel.HasItem }},
		{"plant", 14, func() bool { return j.a.sel.HasPlant }},
	} {
		j.settle()
		j.clickTile(c.x, 2, inputframe.Left, 0)
		if !c.has() {
			t.Fatalf("the click did not select the %s", c.name)
		}
		j.key("Escape", 0)
		if c.has() || j.a.sel.Grouped() || j.a.renderer.HasSelect() {
			t.Fatalf("Escape left the %s selected", c.name)
		}
		if j.a.nav.overlay() == overlayPause {
			t.Fatalf("the Escape that cleared the %s also opened the pause menu", c.name)
		}
	}
	j.key("Escape", 0)
	if j.a.nav.overlay() != overlayPause {
		t.Fatal("an Escape with nothing selected did not open the pause menu")
	}
}

// CTL-018: Ctrl-click on a trade counter moves ten, and no more than the
// trader has.
func TestJourneyMouseCtrlClickTradeCounter(t *testing.T) {
	t.Parallel()
	j := radioColony(t)
	j.g.FireIncident("orbital_trader")
	useRadio(t, j)
	j.clickID("comms.call")
	j.update(2)
	if j.a.uiModalTop() != dialogs.TradeModalID {
		t.Fatalf("top modal %q, want the trade dialog", j.a.uiModalTop())
	}
	rows := func() []game.TradeRowView {
		v, ok := j.g.TradeView()
		if !ok {
			t.Fatal("the trade session ended")
		}
		return v.Rows
	}
	var big, small game.TradeRowView
	for _, r := range rows() {
		if r.TraderQty >= 11 && big.Kind == "" {
			big = r
		}
		if r.TraderQty >= 2 && r.TraderQty < 10 && small.Kind == "" {
			small = r
		}
	}
	if big.Kind == "" || small.Kind == "" {
		t.Fatalf("the trader's stock %v has no row over 10 and row under 10", rows())
	}
	qty := func(kind string) int {
		for _, r := range rows() {
			if r.Kind == kind {
				return r.TraderQty
			}
		}
		return 0 // a row nobody holds is gone
	}
	j.clickWith("trade."+big.Kind+".take.1", input.ModCtrl)
	j.clickWith("trade."+small.Kind+".take.1", input.ModCtrl)
	if !j.clickID("modal." + dialogs.TradeModalID + ".accept") {
		t.Fatal("Accept is disabled with goods on the table")
	}
	if got := qty(big.Kind); got != big.TraderQty-10 {
		t.Fatalf("a Ctrl-click bought %d %s, want 10", big.TraderQty-got, big.Kind)
	}
	if got := qty(small.Kind); got != 0 {
		t.Fatalf("a Ctrl-click bought %d of the trader's %d %s, want all of it (the bound)", small.TraderQty-got, small.TraderQty, small.Kind)
	}
}

// CTL-018: Ctrl-click on a caravan's goods counter loads ten, and no more
// than the colony has.
func TestJourneyMouseCtrlClickCaravanCounter(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0)
	}
	j.key("F8", 0)
	j.update(2)
	if !j.clickID("world.act.form") {
		t.Fatal("Form caravan is disabled on a colony with a world")
	}
	j.update(1)
	if j.a.uiModalTop() != dialogs.FormCaravanID {
		t.Fatalf("top modal %q, want Form caravan", j.a.uiModalTop())
	}
	j.clickID("form.member." + strconv.FormatUint(uint64(j.g.Snapshot()[0].ID), 10))
	for _, s := range j.g.WorldSites() {
		if s.Visitable {
			j.clickID("form.dest." + strconv.Itoa(s.Tile))
			break
		}
	}
	kinds := map[uint64]string{}
	have := map[string]int{}
	for _, it := range j.world().Items {
		kinds[uint64(it.ID)] = it.Kind
		have[it.Kind] += it.Qty
	}
	if have["silver"] < 20 || have["rifle"] < 1 || have["rifle"] >= 10 {
		t.Fatalf("the colony holds %d silver and %d rifles: need a plenty of one and fewer than 10 of the other", have["silver"], have["rifle"])
	}
	j.clickWith("form.goods.silver.inc", input.ModCtrl)
	j.clickWith("form.goods.rifle.inc", input.ModCtrl)
	if !j.clickID("modal." + dialogs.FormCaravanID + ".form.send") {
		t.Fatal("Send is disabled with a colonist, a destination and goods")
	}
	c, ok := j.lastCmd(command.FormCaravan)
	if !ok {
		t.Fatal("Send sent no FormCaravan")
	}
	loaded := map[string]int{}
	for _, part := range strings.Split(c.Name, ",") {
		id, qty, found := strings.Cut(part, ":")
		if !found {
			continue
		}
		sid, _ := strconv.ParseUint(id, 10, 64)
		n, _ := strconv.Atoi(qty)
		loaded[kinds[sid]] += n
	}
	if loaded["silver"] != 10 || loaded["rifle"] != have["rifle"] {
		t.Fatalf("loaded %v from %v, want 10 silver and all %d rifles (the bound)", loaded, have, have["rifle"])
	}
}
