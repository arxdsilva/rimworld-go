package ebiten

// journey_orders_test.go: orders, draft and formation journeys (QA-001).

import (
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

var _ = owe("journey_orders_test.go", []string{}, []string{})

// TestJourneyOrdersEmergencyTend: a drafted colonist right-clicking a
// wounded colonist gets "Tend <name>" instead of walking onto them. The
// suite found this dead: the formation drag claimed every drafted
// right-click on a tile without a raider or animal (formation.go).
func TestJourneyOrdersEmergencyTend(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	ps := j.g.Snapshot()
	doc, hurt := ps[0], ps[1]
	j.g.InjureCut(hurt.ID, 6)
	j.update(1)
	d := j.pawn(uint64(doc.ID))
	j.clickTile(d.X, d.Y, inputframe.Left, 0)
	j.key("R", 0)
	if !j.pawn(uint64(doc.ID)).Drafted {
		t.Fatal("R did not draft the doctor")
	}
	h := j.pawn(uint64(hurt.ID))
	j.clickTile(h.X, h.Y, inputframe.Right, 0)
	if a := j.a; a.ord.Form.On {
		t.Fatal("the right-click on the wounded colonist started a formation drag")
	}
	j.pick("Tend")
	c, ok := j.lastCmd(command.EmergencyTend)
	if !ok || c.Pawn != uint64(doc.ID) {
		t.Fatalf("Tend sent %+v, want EmergencyTend from the doctor", c)
	}
	if _, moved := j.lastCmd(command.MovePawn); moved {
		t.Fatal("the right-click also moved the doctor")
	}
}

// ordPick selects the i-th colonist with a left click on its tile (the game
// settles a spawn on the nearest free tile, so the tile is read back).
func ordPick(j *journey, i int) {
	j.t.Helper()
	var at [][2]int
	for _, p := range j.g.Snapshot() {
		if !p.Hostile {
			at = append(at, [2]int{p.X, p.Y})
		}
	}
	x, y := at[i][0], at[i][1]
	j.clickTile(x, y, inputframe.Left, 0)
	j.draw(2)
	if !j.a.renderer.HasSelect() {
		j.t.Fatalf("clicking (%d, %d) selected no colonist", x, y)
	}
}

// ordColonists are the colonists' ids in spawn order.
func ordColonists(j *journey) []uint64 {
	var ids []uint64
	for _, p := range j.g.Snapshot() {
		if !p.Hostile {
			ids = append(ids, uint64(p.ID))
		}
	}
	return ids
}

// TestJourneyOrdersDraftKeyAndButton: R drafts the selected colonist, the
// pane's Draft button undrafts.
func TestJourneyOrdersDraftKeyAndButton(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := ordColonists(j)[0]
	ordPick(j, 0)
	j.key("R", 0)
	if c, ok := j.lastCmd(command.DraftPawn); !ok || !c.Enabled || c.Pawn != id {
		t.Fatalf("R sent %+v (%v), want DraftPawn enabled for %d", c, ok, id)
	}
	if !j.pawn(id).Drafted {
		t.Fatal("the pawn is not drafted after R")
	}
	j.reset()
	if !j.clickID("pawn.draft") {
		t.Fatal("the Draft button is disabled")
	}
	if c, ok := j.lastCmd(command.DraftPawn); !ok || c.Enabled || c.Pawn != id {
		t.Fatalf("the Draft button sent %+v (%v), want DraftPawn disabled", c, ok)
	}
	if j.pawn(id).Drafted {
		t.Fatal("the pawn is still drafted after the button")
	}
}

// TestJourneyOrdersDraftedRightClickMovesAndArrives: a drafted colonist
// walks to the right-clicked tile and gets there.
func TestJourneyOrdersDraftedRightClickMovesAndArrives(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := ordColonists(j)[0]
	ordPick(j, 0)
	j.key("R", 0)
	j.clickTile(14, 5, inputframe.Right, 0)
	c, ok := j.lastCmd(command.MovePawn)
	if !ok || c.Pawn != id || c.X != 14 || c.Y != 5 || c.Queue {
		t.Fatalf("right click sent %+v (%v), want MovePawn to (14, 5)", c, ok)
	}
	j.key("Digit1", 0)
	j.until(func() bool { p := j.pawn(id); return p.X == 14 && p.Y == 5 }, 900)
	j.key("Space", 0)
}

// TestJourneyOrdersShiftQueuesMove: Shift on the right click queues the move.
func TestJourneyOrdersShiftQueuesMove(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ordPick(j, 0)
	j.key("R", 0)
	j.clickTile(12, 4, inputframe.Right, 0)
	j.clickTile(14, 6, inputframe.Right, input.ModShift)
	moves := j.cmdsOf(command.MovePawn)
	if len(moves) != 2 || moves[0].Queue || !moves[1].Queue || moves[1].X != 14 || moves[1].Y != 6 {
		t.Fatalf("moves %+v, want a plain move then a queued one to (14, 6)", moves)
	}
}

// TestJourneyOrdersFormationDrag: a right-drag with three drafted
// colonists selected lines them up along the drag.
func TestJourneyOrdersFormationDrag(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 3}, [2]int{9, 3}, [2]int{10, 3}))
	j.dragTiles(inputframe.Left, 7, 2, 11, 6, 0)
	if !j.a.sel.Grouped() {
		t.Fatal("the box select did not group the three colonists")
	}
	j.key("R", 0)
	if n := len(j.cmdsOf(command.DraftPawn)); n != 3 {
		t.Fatalf("R drafted %d colonists, want 3", n)
	}
	j.dragTiles(inputframe.Right, 8, 7, 14, 7, 0)
	moves := j.cmdsOf(command.MovePawn)
	if len(moves) != 3 {
		t.Fatalf("the drag sent %d moves, want 3", len(moves))
	}
	seen := map[[2]int]bool{}
	pawns := map[uint64]bool{}
	for _, m := range moves {
		if m.Y != 7 || m.X < 8 || m.X > 14 {
			t.Fatalf("move %+v is off the drag line", m)
		}
		seen[[2]int{m.X, m.Y}] = true
		pawns[m.Pawn] = true
	}
	if len(seen) != 3 || len(pawns) != 3 {
		t.Fatalf("moves %+v: %d cells for %d pawns, want 3 and 3", moves, len(seen), len(pawns))
	}
}

// TestJourneyOrdersUndraftedMenuGoHere: an undrafted colonist gets the
// order menu on a right click, and Go here moves it and closes the menu.
func TestJourneyOrdersUndraftedMenuGoHere(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ordPick(j, 0)
	j.clickTile(13, 6, inputframe.Right, 0)
	if len(j.a.ord.FloatButtons()) == 0 {
		t.Fatal("no order menu opened")
	}
	if len(j.cmds) != 0 {
		t.Fatalf("opening the menu sent %+v", j.cmds)
	}
	j.pick("Go here")
	c, ok := j.lastCmd(command.MovePawn)
	if !ok || c.X != 13 || c.Y != 6 {
		t.Fatalf("Go here sent %+v (%v), want MovePawn to (13, 6)", c, ok)
	}
	if n := len(j.a.ord.FloatButtons()); n != 0 {
		t.Fatalf("%d menu rows still open after the pick", n)
	}
}

// TestJourneyOrdersEquipFromMenu: a weapon on the ground offers Equip.
func TestJourneyOrdersEquipFromMenu(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if _, err := j.g.SpawnStack(item.Knife, 1, 12, 4); err != nil {
		t.Fatal(err)
	}
	ordPick(j, 0)
	j.clickTile(12, 4, inputframe.Right, 0)
	j.pick("Equip")
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderEquip || c.X != 12 || c.Y != 4 {
		t.Fatalf("Equip sent %+v (%v), want Prioritize OrderEquip at (12, 4)", c, ok)
	}
}

// TestJourneyOrdersHaulHuntMenu: a stack offers Haul, a wild animal Hunt.
func TestJourneyOrdersHaulHuntMenu(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if _, err := j.g.SpawnStack(item.Wood, 20, 12, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnAnimal("deer", 14, 6); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnStack(item.Rifle, 1, 9, 6); err != nil { // hunting needs a ranged weapon
		t.Fatal(err)
	}
	j.key("Tab", 0) // a stockpile for the wood to go to
	j.clickID("arch.cat.zones")
	j.clickID("arch.tool.stockpile")
	j.dragTiles(inputframe.Left, 16, 6, 17, 7, 0)
	j.key("Escape", 0)
	j.key("Escape", 0)
	j.settle()
	ordPick(j, 0)
	j.clickTile(12, 5, inputframe.Right, 0)
	j.pick("Haul")
	if c, ok := j.lastCmd(command.Prioritize); !ok || c.Order != command.OrderHaul || c.X != 12 || c.Y != 5 {
		t.Fatalf("Haul sent %+v (%v), want Prioritize OrderHaul at (12, 5)", c, ok)
	}
	j.reset()
	j.clickTile(9, 6, inputframe.Right, 0)
	j.pick("Equip")
	j.key("Digit1", 0)
	j.until(func() bool { return j.pawn(ordColonists(j)[0]).Weapon == "rifle" }, 900)
	j.key("Space", 0)
	j.reset()
	j.clickTile(14, 6, inputframe.Right, 0)
	j.pick("Hunt")
	if c, ok := j.lastCmd(command.Prioritize); !ok || c.Order != command.OrderHunt || c.X != 14 || c.Y != 6 {
		t.Fatalf("Hunt sent %+v (%v), want Prioritize OrderHunt at (14, 6)", c, ok)
	}
}

// TestJourneyOrdersDisabledEntryHintOnly: a disabled menu row sends
// nothing, says why and keeps the menu open.
func TestJourneyOrdersDisabledEntryHintOnly(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("wall", "wood", "", 13, 5) {
		t.Fatal("no wall built")
	}
	ordPick(j, 0)
	j.clickTile(13, 5, inputframe.Right, 0)
	rows := len(j.a.ord.FloatButtons())
	if rows == 0 {
		t.Fatal("no menu opened on the wall")
	}
	j.pick("Go here")
	if len(j.cmds) != 0 {
		t.Fatalf("a disabled row sent %+v", j.cmds)
	}
	if j.a.hint == "" {
		t.Fatal("a disabled row gave no reason")
	}
	if n := len(j.a.ord.FloatButtons()); n != rows {
		t.Fatalf("menu has %d rows after the pick, want it still open with %d", n, rows)
	}
}

// TestJourneyOrdersClearOrders: the pane's clear-orders button.
func TestJourneyOrdersClearOrders(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := ordColonists(j)[0]
	ordPick(j, 0)
	j.key("R", 0)
	j.clickTile(13, 6, inputframe.Right, 0)
	j.clickTile(15, 6, inputframe.Right, input.ModShift)
	j.tick(1)
	j.reset()
	if !j.clickID("pawn.clearorders") {
		t.Fatal("the clear-orders button is disabled with a queued order")
	}
	if c, ok := j.lastCmd(command.ClearOrders); !ok || c.Pawn != id {
		t.Fatalf("sent %+v (%v), want ClearOrders for %d", c, ok, id)
	}
}

// TestJourneyOrdersRightClickCancelsToolNoOrder: with an Architect tool in
// hand a right click drops the tool and orders nothing.
func TestJourneyOrdersRightClickCancelsToolNoOrder(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	ordPick(j, 0)
	j.key("Tab", 0)
	j.clickID("arch.cat.structure")
	j.clickID("arch.tool.wall")
	if j.a.arch.ToolID == "" {
		t.Fatal("the wall tool did not arm")
	}
	j.clickTile(14, 5, inputframe.Right, 0)
	if j.a.arch.ToolID != "" {
		t.Fatalf("tool %q still armed after the right click", j.a.arch.ToolID)
	}
	if n := len(j.cmdsOf(command.MovePawn)); n != 0 || len(j.a.ord.FloatButtons()) != 0 {
		t.Fatalf("the right click ordered %d moves / opened a menu", n)
	}
}

// TestJourneyOrdersGizmoKeys: B H Y N J U M K I L O P press the selection's
// command cells in order, a disabled cell says why, and a key past the
// last cell does nothing.
func TestJourneyOrdersGizmoKeys(t *testing.T) {
	t.Parallel()
	keys := []string{"B", "H", "Y", "N", "J", "U", "M", "K", "I", "L", "O", "P"}
	t.Run("twelve cells", func(t *testing.T) {
		t.Parallel()
		j := newJourney(t)
		id := ordColonists(j)[0]
		ordPick(j, 0)
		j.a.insp.TestCommands = func(k selectionpkg.Kind, _ entity.EntityID) []inspector.Button {
			cells := make([]inspector.Button, len(keys))
			for i := range cells {
				cells[i] = inspector.Button{ID: "cmd.test", Label: keys[i],
					Cmds: []command.Command{{Kind: command.ClearOrders, Pawn: id, X: i}}}
			}
			cells[4].Disabled, cells[4].Reason, cells[4].Cmds = true, "not now", nil
			return cells
		}
		for i, k := range keys {
			j.reset()
			j.a.hint = ""
			j.key(k, 0)
			c, ok := j.lastCmd(command.ClearOrders)
			if i == 4 {
				if ok || j.a.hint != "not now" {
					t.Fatalf("%s on a disabled cell: sent %v, hint %q", k, ok, j.a.hint)
				}
				continue
			}
			if !ok || c.X != i || len(j.cmds) != 1 {
				t.Fatalf("%s sent %+v (%v), want cell %d only", k, j.cmds, ok, i)
			}
		}
	})
	t.Run("a building's own cells", func(t *testing.T) {
		t.Parallel()
		j := newJourney(t)
		if !j.g.BuildNow("bed", "wood", "n", 10, 4) {
			t.Fatal("no bed built")
		}
		j.clickTile(10, 4, inputframe.Left, 0)
		j.draw(2)
		n := len(j.a.insp.Commands().SelectionCommands())
		if n < 2 || n >= len(keys) {
			t.Fatalf("a bed has %d cells, want a few", n)
		}
		for _, k := range keys[n:] {
			j.reset()
			j.key(k, 0)
			if len(j.cmds) != 0 {
				t.Fatalf("%s past the last cell sent %+v", k, j.cmds)
			}
		}
	})
}

// TestJourneyOrdersCaptureAndArrest: the pane's Capture sends the colonist
// after the nearest downed raider, Arrest has another colonist take them.
func TestJourneyOrdersCaptureAndArrest(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}))
	j.g.Raid(1)
	j.tick(2)
	var raider uint64
	for _, p := range j.g.Snapshot() {
		if p.Hostile {
			raider = uint64(p.ID)
		}
	}
	if raider == 0 {
		t.Fatal("the raid brought no raider")
	}
	for dmg := 20; dmg <= 400 && j.pawn(raider).Status != "downed"; dmg += 20 {
		j.g.Injure(entity.EntityID(raider), dmg)
	}
	if st := j.pawn(raider).Status; st != "downed" {
		t.Skipf("could not down the raider (status %q)", st)
	}
	ordPick(j, 0)
	id := uint64(j.a.renderer.Selected())
	if !j.clickID("cmd.pawn.capture") {
		t.Fatal("Capture is disabled with a downed raider on the map")
	}
	if c, ok := j.lastCmd(command.Capture); !ok || c.Pawn != id || c.Target != raider {
		t.Fatalf("Capture sent %+v (%v), want captor %d target %d", c, ok, id, raider)
	}
	if !j.clickID("cmd.pawn.arrest") {
		t.Fatal("Arrest is disabled")
	}
	if c, ok := j.lastCmd(command.Arrest); !ok || c.Target != id {
		t.Fatalf("Arrest sent %+v (%v), want target %d", c, ok, id)
	}
}

// careDoerAndPatient puts two colonists on the map, downs the second and
// returns (doer, patient) ids and the patient's tile.
func careDoerAndPatient(j *journey) (doer, patient uint64, px, py int) {
	j.t.Helper()
	ids := ordColonists(j)
	doer, patient = ids[0], ids[1]
	careDown(j, patient)
	p := j.pawn(patient)
	return doer, patient, p.X, p.Y
}

// TestJourneyOrdersRescueDowned (SIM-062): select a colonist, right-click a
// downed one, choose "Rescue <name>": the rescuer carries the patient to the
// medical bed and the read model shows the carry.
func TestJourneyOrdersRescueDowned(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	bed := careBed(j, "hospital bed", 16, 2)
	doer, patient, px, py := careDoerAndPatient(j)
	name := j.pawn(patient).Name
	ordPick(j, 0)
	if got := uint64(j.a.renderer.Selected()); got != doer {
		t.Fatalf("selected %d, want the first colonist %d", got, doer)
	}
	labels := careRightClick(j, px, py)
	j.pick("Rescue " + name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderRescue || c.Pawn != doer || c.Target != patient {
		t.Fatalf("Rescue sent %+v (%v), want Prioritize/OrderRescue doer %d target %d; menu %q", c, ok, doer, patient, labels)
	}
	sawCarry := false
	careRun(j, "Digit2", 6000, func() bool {
		careFeed(j)
		if j.pawn(patient).CarriedBy == entity.EntityID(doer) && j.pawn(doer).CarryingPawn == entity.EntityID(patient) {
			sawCarry = true
		}
		return j.pawn(patient).InBed
	})
	if !sawCarry {
		t.Fatal("the read model never showed CarriedBy/CarryingPawn during the carry")
	}
	for _, b := range j.world().Buildings {
		if b.ID == bed && b.Claimant != entity.EntityID(patient) {
			t.Fatalf("the patient is in a bed, but not the medical one (claimant %d)", b.Claimant)
		}
	}
	if j.pawn(patient).CarriedBy != 0 {
		t.Fatal("still carried after reaching the bed")
	}
}

// TestJourneyOrdersCaptureFromMenu: "Capture <name>" on a downed raider
// carries them to the prison bed.
func TestJourneyOrdersCaptureFromMenu(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}))
	carePrisonBed(j, careBed(j, "sleeping spot", 14, 2))
	j.g.Raid(1)
	j.tick(2)
	var raider uint64
	for _, p := range j.g.Snapshot() {
		if p.Hostile {
			raider = uint64(p.ID)
		}
	}
	if raider == 0 {
		t.Fatal("the raid brought no raider")
	}
	careDown(j, raider)
	ordPick(j, 0)
	doer := uint64(j.a.renderer.Selected())
	r := j.pawn(raider)
	name := r.Name
	labels := careRightClick(j, r.X, r.Y)
	j.pick("Capture " + name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderCapture || c.Pawn != doer || c.Target != raider {
		t.Fatalf("Capture sent %+v (%v), want Prioritize/OrderCapture doer %d target %d; menu %q", c, ok, doer, raider, labels)
	}
	careRun(j, "Digit3", 8000, func() bool {
		careFeed(j)
		return j.pawn(raider).Prisoner
	})
}

// TestJourneyOrdersArrestFromMenu: "Arrest <name>" on a standing colonist
// sends OrderArrest and the arrester starts the arrest job (it may succeed
// or be resisted).
func TestJourneyOrdersArrestFromMenu(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	carePrisonBed(j, careBed(j, "sleeping spot", 16, 2))
	doer, target := ordColonists(j)[0], ordColonists(j)[1]
	tp := j.pawn(target)
	ordPick(j, 0)
	labels := careRightClick(j, tp.X, tp.Y)
	j.pick("Arrest " + tp.Name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderArrest || c.Pawn != doer || c.Target != target {
		t.Fatalf("Arrest sent %+v (%v), want Prioritize/OrderArrest doer %d target %d; menu %q", c, ok, doer, target, labels)
	}
	started := false
	careRun(j, "Digit2", 3000, func() bool {
		careFeed(j)
		if strings.Contains(strings.ToLower(j.pawn(doer).Job), "arrest") {
			started = true
		}
		return started
	})
}

// TestJourneyOrdersPrioritizeTend: an undrafted colonist offered
// "Prioritize tending <name>" goes and tends the wounded one.
func TestJourneyOrdersPrioritizeTend(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	ids := ordColonists(j)
	doer, hurt := ids[0], ids[1]
	j.g.InjureCut(entity.EntityID(hurt), 6)
	j.update(1)
	hp := j.pawn(hurt)
	ordPick(j, 0)
	labels := careRightClick(j, hp.X, hp.Y)
	j.pick("Prioritize tending " + hp.Name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderTendPatient || c.Pawn != doer || c.Target != hurt {
		t.Fatalf("Prioritize tending sent %+v (%v), want OrderTendPatient doer %d target %d; menu %q", c, ok, doer, hurt, labels)
	}
	careRun(j, "Digit2", 3000, func() bool {
		careFeed(j)
		return j.pawn(hurt).Tended
	})
}

// TestJourneyOrdersRescueQueued: Shift-choosing "Rescue <name>" behind a
// walk order queues it under that label, and it runs after the walk.
func TestJourneyOrdersRescueQueued(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	careBed(j, "hospital bed", 16, 2)
	doer, patient, px, py := careDoerAndPatient(j)
	name := j.pawn(patient).Name
	ordPick(j, 0)
	j.clickTile(6, 7, inputframe.Right, 0)
	j.pick("Go here")
	if !j.pawn(doer).HasOrder {
		t.Fatal("the walk order did not start")
	}
	careRightClick(j, px, py)
	carePickShift(j, "Rescue "+name)
	c, ok := j.lastCmd(command.Prioritize)
	if !ok || c.Order != command.OrderRescue || !c.Queue || c.Target != patient {
		t.Fatalf("Shift-Rescue sent %+v (%v), want a queued OrderRescue", c, ok)
	}
	j.update(1)
	q := j.pawn(doer).QueuedOrders
	if len(q) != 1 || q[0].Label != "Rescue "+name {
		t.Fatalf("queued orders %+v, want one labelled %q", q, "Rescue "+name)
	}
	careRun(j, "Digit3", 8000, func() bool {
		careFeed(j)
		return j.pawn(patient).InBed
	})
	if len(j.pawn(doer).QueuedOrders) != 0 {
		t.Fatal("the queue still holds the rescue after it ran")
	}
}
