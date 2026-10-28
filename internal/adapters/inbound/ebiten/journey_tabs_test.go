package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// journey_tabs_test.go: main tab journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_tabs_test.go",
	[]string{},
	[]string{})

func idStr(id uint64) string { return strconv.FormatUint(id, 10) }

// TestJourneyTabsWorkCellCycles: F1 opens Work; a cell cycles off, 1..4.
func TestJourneyTabsWorkCellCycles(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := uint64(j.g.Snapshot()[0].ID)
	j.key("F1", 0)
	if !j.a.workOpen || j.a.panelMode != "work" {
		t.Fatalf("F1 left the work tab closed (open %v, mode %q)", j.a.workOpen, j.a.panelMode)
	}
	cell := "work.cell." + idStr(id) + ".1"
	start := workPriority(j.pawn(id), 1)
	want := start
	for i := 0; i < 5; i++ {
		j.reset()
		want = nextPriority(want)
		if !j.clickID(cell) {
			t.Fatalf("cell %s is disabled", cell)
		}
		c, ok := j.lastCmd(command.SetWorkPriority)
		if !ok || c.Pawn != id || c.Count != want {
			t.Fatalf("click %d: %+v (sent %v), want priority %d for pawn %d", i, c, ok, want, id)
		}
		if got := j.pawn(id).Priorities[1]; got != want {
			t.Fatalf("click %d: read model priority %d, want %d", i, got, want)
		}
	}
	if want != start {
		t.Fatalf("five clicks ended at %d, started at %d: the cycle is not 5 long", want, start)
	}
	j.key("F1", 0)
	if j.a.workOpen {
		t.Fatal("F1 again left the work tab open")
	}
}

// TestJourneyTabsScheduleBrushAndCell: pick a brush, paint an hour.
func TestJourneyTabsScheduleBrushAndCell(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := uint64(j.g.Snapshot()[0].ID)
	j.key("F2", 0)
	if !j.a.workOpen || j.a.panelMode != "schedule" {
		t.Fatalf("F2 left the schedule closed (mode %q)", j.a.panelMode)
	}
	hour := 6
	brush := schedule.Recreation
	if j.pawn(id).Schedule[hour] == brush {
		brush = schedule.Sleep
	}
	if !j.clickID("sched.brush." + strconv.Itoa(int(brush))) {
		t.Fatal("brush is disabled")
	}
	cell := "sched.cell." + idStr(id) + "." + strconv.Itoa(hour)
	j.reset()
	j.clickID(cell)
	c, ok := j.lastCmd(command.SetSchedule)
	if !ok || c.Pawn != id || c.Index != hour || c.Mode != uint8(brush) {
		t.Fatalf("paint sent %+v (%v), want pawn %d hour %d mode %d", c, ok, id, hour, brush)
	}
	if got := j.pawn(id).Schedule[hour]; got != brush {
		t.Fatalf("schedule[%d] = %v, want %v", hour, got, brush)
	}
	j.reset()
	j.settle()
	j.clickID(cell)
	if n := len(j.cmdsOf(command.SetSchedule)); n != 0 {
		t.Fatalf("painting the same brush again sent %d commands", n)
	}
	// the everyone row paints every colonist
	j.reset()
	j.clickID("sched.brush." + strconv.Itoa(int(schedule.Work)))
	j.clickID("sched.cell.0." + strconv.Itoa(hour))
	if c, ok := j.lastCmd(command.SetSchedule); !ok || c.Pawn != 0 || c.Mode != uint8(schedule.Work) {
		t.Fatalf("everyone row sent %+v (%v)", c, ok)
	}
	if got := j.pawn(id).Schedule[hour]; got != schedule.Work {
		t.Fatalf("everyone row left schedule[%d] = %v", hour, got)
	}
}

// TestJourneyTabsAssignCareAndPolicies: F3 opens the Assign window.
func TestJourneyTabsAssignCareAndPolicies(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := uint64(j.g.Snapshot()[0].ID)
	j.key("F3", 0)
	if j.a.uiModalTop() != assignModalID {
		t.Fatalf("F3 opened %q", j.a.uiModalTop())
	}
	care0 := j.pawn(id).CareLevel
	j.clickID(assignCareID(j.g.Snapshot()[0].ID))
	c, ok := j.lastCmd(command.SetMedicalCare)
	if !ok || c.Pawn != id || c.Mode == care0 {
		t.Fatalf("care click sent %+v (%v) from level %d", c, ok, care0)
	}
	if got := j.pawn(id).CareLevel; got != c.Mode {
		t.Fatalf("care level %d after the click, want %d", got, c.Mode)
	}
	j.clickID(assignDrugID(j.g.Snapshot()[0].ID))
	if c, ok := j.lastCmd(command.SetDrugPolicy); !ok || c.Pawn != id {
		t.Fatalf("drug policy click sent %+v (%v)", c, ok)
	}
	j.clickID("assign.defcare.0")
	if c, ok := j.lastCmd(command.SetDefaultCare); !ok || c.Index != 0 {
		t.Fatalf("default care click sent %+v (%v)", c, ok)
	}
	j.key("F3", 0)
	if j.a.uiModalTop() != "" {
		t.Fatalf("F3 again left %q open", j.a.uiModalTop())
	}
}

// TestJourneyTabsResearchPick: F6 lists projects; the first available one is chosen.
func TestJourneyTabsResearchPick(t *testing.T) {
	t.Parallel()
	// game.New is a sandbox that knows everything; a new colony knows nothing.
	j := newJourney(t, jGame(game.NewColony(game.Config{Seed: 1, Width: 64, Height: 64})))
	j.key("F6", 0)
	if !j.a.workOpen || j.a.panelMode != "research" {
		t.Fatalf("F6 left mode %q", j.a.panelMode)
	}
	var pick string
	for _, b := range j.a.researchButtons() {
		if !b.disabled && !b.selected {
			pick = b.id
			break
		}
	}
	if pick == "" {
		t.Fatalf("no available research project: %+v", j.a.researchButtons())
	}
	j.clickID(pick)
	c, ok := j.lastCmd(command.SetResearch)
	if !ok || "research.pick."+strconv.Itoa(int(c.Build)) != pick {
		t.Fatalf("picking %s sent %+v (%v)", pick, c, ok)
	}
}

// TestJourneyTabsWorldAndHistory: F8 and F9 open their windows; Escape closes.
func TestJourneyTabsWorldAndHistory(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.key("F8", 0)
	if got := j.a.uiModalTop(); got != "colony.world" {
		t.Fatalf("F8 opened %q", got)
	}
	j.key("Escape", 0)
	if got := j.a.uiModalTop(); got != "" {
		t.Fatalf("Escape left %q open", got)
	}
	j.key("F9", 0)
	if got := j.a.uiModalTop(); got != "colony.history" {
		t.Fatalf("F9 opened %q", got)
	}
	j.key("F9", 0)
	if got := j.a.uiModalTop(); got != "" {
		t.Fatalf("F9 again left %q open", got)
	}
}

// TestJourneyTabsReservedKeysToast: F7 (Quests) and T (Beauty) only toast.
func TestJourneyTabsReservedKeysToast(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"F7", "T"} {
		t.Run(k, func(t *testing.T) {
			t.Parallel()
			j := newJourney(t)
			j.key(k, 0)
			toasts := j.a.liveToasts()
			if len(toasts) == 0 || !strings.Contains(toasts[len(toasts)-1].text, "not implemented") {
				t.Fatalf("%s toasted %+v, want a not-implemented notice", k, toasts)
			}
			if j.a.workOpen || j.a.uiModalTop() != "" {
				t.Fatalf("%s opened something (work %v, modal %q)", k, j.a.workOpen, j.a.uiModalTop())
			}
		})
	}
}

// TestJourneyTabsWildlife: F5 lists wild species; hunt, tame and cancel.
func TestJourneyTabsWildlife(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if _, err := j.g.SpawnAnimal("deer", 14, 6); err != nil {
		t.Fatal(err)
	}
	j.key("F5", 0)
	if !j.a.workOpen || j.a.panelMode != "animals" || !j.a.animalsWild {
		t.Fatalf("F5 left mode %q wild %v", j.a.panelMode, j.a.animalsWild)
	}
	j.clickID("animals.wild.hunt.deer")
	c, ok := j.lastCmd(command.DesignateHunt)
	if !ok || c.X != 14 || c.Y != 6 {
		t.Fatalf("Hunt all sent %+v (%v)", c, ok)
	}
	j.clickID("animals.wild.cancel.deer")
	if c, ok := j.lastCmd(command.Undesignate); !ok || c.Order != command.OrderHunt {
		t.Fatalf("Cancel sent %+v (%v)", c, ok)
	}
	j.clickID("animals.wild.tame.deer")
	if c, ok := j.lastCmd(command.DesignateTame); !ok || c.X != 14 || c.Y != 6 {
		t.Fatalf("Tame one sent %+v (%v)", c, ok)
	}
	j.clickID("animals.wild.cancel.deer")
	if c, ok := j.lastCmd(command.Undesignate); !ok || c.Order != command.OrderTame {
		t.Fatalf("Cancel sent %+v (%v)", c, ok)
	}
}

// clickIDWith clicks a HUD button or kit widget with modifiers held, the
// way clickID does without them.
func clickIDWith(j *journey, id string, mods input.Mod) {
	j.t.Helper()
	j.update(1)
	var x, y float32
	found := false
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.id == id {
			x, y, found = b.x+b.w/2, b.y+b.h/2, true
			break
		}
	}
	if !found {
		w, ok := j.a.uiSt().find(id)
		if !ok {
			j.t.Fatalf("no button or widget %q", id)
		}
		x, y = w.r.Center()
	}
	s := float32(j.a.scale.S)
	j.f.cur.mx, j.f.cur.my = int(x*s), int(y*s)
	j.f.click(j.t, j.a, mbLeft, mods)
	j.f.cur.mx, j.f.cur.my = -50, -50
}

// TestJourneyTabsQuantityModifiers: Ctrl multiplies a bill stepper by 10,
// Ctrl+Shift by 1000 (the amount modifiers), Shift by 100.
func TestJourneyTabsQuantityModifiers(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("workbench", "wood", "south", 10, 3) {
		t.Fatal("no bench")
	}
	j.lookAt(10, 3)
	j.update(1)
	j.clickTile(10, 3, mbLeft, 0)
	j.clickID("inspector.bills")
	j.clickID(billPickBtn)
	j.clickID(billPickList + ".0")
	j.clickID("modal.bill.picker.bill.picker.ok")
	bills := func() int {
		for _, b := range j.world().Buildings {
			if b.X == 10 && b.Y == 3 {
				return len(b.Bills)
			}
		}
		return 0
	}
	target := func() int {
		for _, b := range j.world().Buildings {
			if b.X == 10 && b.Y == 3 && len(b.Bills) > 0 {
				return b.Bills[0].Target
			}
		}
		return -1
	}
	if bills() != 1 {
		t.Fatalf("the bench has %d bills after adding one", bills())
	}
	for _, c := range []struct {
		name string
		mods input.Mod
		step int
	}{{"plain", 0, 1}, {"Ctrl", input.ModCtrl, 10}, {"Shift", input.ModShift, 100}, {"Ctrl+Shift", input.ModCtrl | input.ModShift, 1000}} {
		before := target()
		j.reset()
		clickIDWith(j, "bill.plus.0", c.mods)
		cmd, ok := j.lastCmd(command.SetBill)
		want := min(before+c.step, billMaxCount)
		if !ok || cmd.Count != want {
			t.Fatalf("%s: + from %d sent %+v (%v), want count %d", c.name, before, cmd, ok, want)
		}
		if got := target(); got != want {
			t.Fatalf("%s: the bill's target is %d, want %d", c.name, got, want)
		}
	}
	before := target()
	j.reset()
	clickIDWith(j, "bill.minus.0", input.ModCtrl)
	if cmd, ok := j.lastCmd(command.SetBill); !ok || cmd.Count != max(before-10, 0) {
		t.Fatalf("Ctrl -: sent %+v (%v) from %d", cmd, ok, before)
	}
}

// TestJourneyTabsTameAnimals: F4 lists the colony's animals: a master from
// the menu, training from the animal's pane, follow once it has learned
// obedience, and release.
func TestJourneyTabsTameAnimals(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{10, 6}))
	pawn := j.g.Snapshot()[0].ID
	id, err := j.g.SpawnAnimal("husky", 12, 6)
	if err != nil {
		t.Fatal(err)
	}
	j.g.TameAnimal(id, "Rex")
	n := idStr(uint64(id))
	animal := func() game.AnimalView {
		for _, an := range j.world().Animals {
			if an.ID == id {
				return an
			}
		}
		t.Fatal("the husky is gone")
		return game.AnimalView{}
	}

	j.key("F4", 0)
	if !j.a.workOpen || j.a.panelMode != "animals" || j.a.animalsWild {
		t.Fatalf("F4 left mode %q (wild %v)", j.a.panelMode, j.a.animalsWild)
	}
	j.clickID("animals.master." + n)
	j.pick("Colonist")
	if c, ok := j.lastCmd(command.SetAnimalMaster); !ok || c.Target != uint64(id) || c.Pawn != uint64(pawn) {
		t.Fatalf("master pick sent %+v (%v)", c, ok)
	}
	if animal().MasterID != pawn {
		t.Fatalf("master is %d, want %d", animal().MasterID, pawn)
	}
	j.clickID("animals.master." + n)
	j.pick("No master")
	if c, _ := j.lastCmd(command.SetAnimalMaster); c.Pawn != 0 || animal().MasterID != 0 {
		t.Fatalf("No master sent %+v, master %d", c, animal().MasterID)
	}
	j.clickID("animals.master." + n)
	j.pick("Colonist")

	// Training toggles live in the animal's pane (click the animal).
	j.key("F4", 0)
	j.lookAt(12, 6)
	j.update(1)
	j.clickTile(12, 6, mbLeft, 0)
	j.reset()
	j.clickID("animal.train.1")
	if c, ok := j.lastCmd(command.SetTraining); !ok || c.Target != uint64(id) || c.Index != 1 || !c.Enabled {
		t.Fatalf("Guard toggle sent %+v (%v)", c, ok)
	}
	j.clickID("animal.train.0")
	if c, ok := j.lastCmd(command.SetTraining); !ok || c.Index != 0 || c.Enabled {
		t.Fatalf("Obedience toggle sent %+v (%v)", c, ok)
	}
	j.clickID("animal.train.0") // back on: the handler trains it below
	if tr := animal().Training; !tr[0].On || !tr[1].On {
		t.Fatalf("training after the toggles: %+v", tr)
	}
	j.clickID("animal.train.1")

	// Follow needs a learned skill: a bonded handler trains it (hook-driven time).
	j.g.DevBond(id, pawn)
	for i := 0; i < 400 && !animal().CanFollow; i++ {
		j.g.Step(500)
	}
	if !animal().CanFollow {
		t.Fatalf("the husky never learned obedience: %+v", animal().Training)
	}
	j.key("F4", 0)
	j.reset()
	was := animal().Follows
	j.clickID("animals.follow." + n)
	if c, ok := j.lastCmd(command.SetAnimalFollow); !ok || c.Target != uint64(id) || c.Enabled == was {
		t.Fatalf("follow toggle sent %+v (%v) from %v", c, ok, was)
	}
	if animal().Follows == was {
		t.Fatal("follow did not change")
	}
	j.clickID("animals.follow." + n)
	if animal().Follows != was {
		t.Fatal("follow did not toggle back")
	}

	j.clickID("animals.release." + n)
	if c, ok := j.lastCmd(command.ReleaseAnimal); !ok || c.Pawn != uint64(id) {
		t.Fatalf("release sent %+v (%v)", c, ok)
	}
	if animal().Tame {
		t.Fatal("the husky is still tame")
	}
}
