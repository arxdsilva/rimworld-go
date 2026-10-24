package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// recSim records enqueued commands and answers CanMoveTo / Locked from
// test fields, delegating everything else to the real game.
type recSim struct {
	Simulation
	cmds   []command.Command
	moveOK bool
	locked map[string]string
}

func (s *recSim) Enqueue(c command.Command) { s.cmds = append(s.cmds, c) }
func (s *recSim) CanMoveTo(entity.EntityID, string, int, int) (bool, string) {
	if s.moveOK {
		return true, ""
	}
	return false, "blocked"
}
func (s *recSim) World() game.WorldView {
	w := s.Simulation.World()
	if s.locked != nil {
		w.Locked = s.locked
	}
	return w
}

// bldgApp builds a colony with one building of kind and selects it.
func bldgApp(t *testing.T, kind, stuff, facing string) (*App, *recSim, game.BuildingView) {
	t.Helper()
	a, g, _ := liveColony(t, 0)
	if !g.BuildNow(kind, stuff, facing, 8, 8) {
		t.Fatalf("BuildNow %s failed", kind)
	}
	rs := &recSim{Simulation: g, moveOK: true}
	a.renderer.game = rs
	b, ok := buildingAt(a.renderer.world(), 8, 8)
	if !ok {
		t.Fatalf("no %s built", kind)
	}
	a.selectCand(selCand{selBuilding, b.ID})
	return a, rs, b
}

func findBtn(a *App, id string) (button, bool) {
	for _, b := range a.buildingButtons() {
		if b.id == id {
			return b, true
		}
	}
	return button{}, false
}

func TestMoveButtonDisabledForWalls(t *testing.T) {
	t.Parallel()
	a, _, _ := bldgApp(t, "wall", "wood", "south")
	mv, ok := findBtn(a, "bldg.move")
	if !ok || !mv.disabled || mv.reason != "can't be moved" {
		t.Fatalf("move button = %+v ok=%v", mv, ok)
	}
	if cp, ok := findBtn(a, "bldg.copy"); !ok || cp.disabled {
		t.Fatalf("copy must work on walls: %+v", cp)
	}
	for _, k := range []string{"door", "grave", "fence", "fence gate"} {
		if movableKind(k, false) {
			t.Fatalf("%s must not be movable", k)
		}
	}
	if movableKind("rug", true) || !movableKind("bed", false) {
		t.Fatal("floors are fixed, beds move")
	}
}

func TestMoveModeEnqueuesMoveBuilding(t *testing.T) {
	t.Parallel()
	a, rs, b := bldgApp(t, "butcher table", "wood", "east")
	a.applyBuildingClick("bldg.move")
	if !a.moveSt().on {
		t.Fatal("Move should enter move mode")
	}
	a.moveStep(5, 5, true, 0, true, false) // the opening click is swallowed
	if len(rs.cmds) != 0 {
		t.Fatalf("opening frame enqueued %v", rs.cmds)
	}
	rs.moveOK = false
	a.moveStep(5, 5, true, 0, true, false)
	if len(rs.cmds) != 0 || !a.moveSt().on || a.hint != "blocked" {
		t.Fatalf("invalid spot: cmds=%v on=%v hint=%q", rs.cmds, a.moveSt().on, a.hint)
	}
	rs.moveOK = true
	a.moveStep(5, 5, true, 0, true, false)
	if len(rs.cmds) != 1 {
		t.Fatalf("cmds = %v", rs.cmds)
	}
	c := rs.cmds[0]
	if c.Kind != command.MoveBuilding || c.Target != uint64(b.ID) || c.X != 5 || c.Y != 5 || c.Facing != facingIndex("east") {
		t.Fatalf("command = %+v", c)
	}
	if a.moveSt().on {
		t.Fatal("placing should leave move mode")
	}
}

func TestMoveModeRotates(t *testing.T) {
	t.Parallel()
	a, rs, _ := bldgApp(t, "butcher table", "wood", "south")
	if bv, _ := buildable("butcher table"); !bv.Rotatable {
		t.Skip("butcher table not rotatable")
	}
	a.applyBuildingClick("bldg.move")
	a.moveStep(5, 5, true, 0, false, false) // swallow fresh frame
	a.moveStep(5, 5, true, 1, false, false)
	if a.moveSt().facing != facingIndex("east") {
		t.Fatalf("facing = %d", a.moveSt().facing)
	}
	a.moveStep(5, 5, true, -1, false, false)
	a.moveStep(5, 5, true, -1, true, false)
	if len(rs.cmds) != 1 || rs.cmds[0].Facing != facingIndex("west") {
		t.Fatalf("cmds = %+v", rs.cmds)
	}
}

func TestMoveModeCancel(t *testing.T) {
	t.Parallel()
	a, rs, _ := bldgApp(t, "butcher table", "wood", "south")
	a.applyBuildingClick("bldg.move")
	a.moveStep(5, 5, true, 0, false, false)
	a.moveStep(5, 5, true, 0, false, true)
	if a.moveSt().on || len(rs.cmds) != 0 {
		t.Fatalf("cancel: on=%v cmds=%v", a.moveSt().on, rs.cmds)
	}
	if a.moveStep(5, 5, true, 0, true, false) {
		t.Fatal("inactive move mode must not consume input")
	}
}

func TestCopyOpensArchitectPreset(t *testing.T) {
	t.Parallel()
	// A butcher table cannot be stone, so kind and facing are checked on
	// it and the material presets on stone walls and a steel door.
	a, _, _ := bldgApp(t, "butcher table", "wood", "east")
	a.applyBuildingClick("bldg.copy")
	tool, ok := toolByID(a.archTool)
	if !a.archOpen || !ok || tool.build != "butcher table" {
		t.Fatalf("architect open=%v tool=%q", a.archOpen, a.archTool)
	}
	if a.archFacing != facingIndex("east") {
		t.Fatalf("facing = %d", a.archFacing)
	}
	for _, c := range []struct{ kind, stuff, want string }{
		{"wall", "marble blocks", "marble blocks"},
		{"wall", "stone", "granite blocks"}, // a pre-migration read model
		{"door", "steel", "steel"},
	} {
		w, _, _ := bldgApp(t, c.kind, c.stuff, "south")
		w.applyBuildingClick("bldg.copy")
		wt, _ := toolByID(w.archTool)
		if wt.build != c.kind || w.toolStuff(wt) != c.want {
			t.Fatalf("%s %s copy: tool=%q stuff=%q", c.stuff, c.kind, w.archTool, w.archStuff)
		}
	}
}

func TestCopyDisabledWhenLocked(t *testing.T) {
	t.Parallel()
	a, rs, _ := bldgApp(t, "butcher table", "wood", "south")
	rs.locked = map[string]string{"build:butcher table": "Smithing"}
	cp, ok := findBtn(a, "bldg.copy")
	if !ok || !cp.disabled || cp.reason != "needs research: Smithing" {
		t.Fatalf("copy = %+v", cp)
	}
	if hasBuildTool("no such kind") {
		t.Fatal("unknown kind has no tool")
	}
	if a.presetArchTool("no such kind", "", "") {
		t.Fatal("preset of unknown kind should fail")
	}
}

func TestPendingMoveShowsCancel(t *testing.T) {
	t.Parallel()
	a, rs, b := bldgApp(t, "butcher table", "wood", "south")
	if _, ok := findBtn(a, "bldg.cancelmove"); ok {
		t.Fatal("no cancel button without a pending move")
	}
	b.MovingTo = &[2]int{4, 5}
	if got := movingLine(b); got != "Moving to (4, 5)" {
		t.Fatalf("line = %q", got)
	}
	r := buildingRect()
	if out := a.moveCopyButtons(b, r); len(out) != 3 || out[2].id != "bldg.cancelmove" {
		t.Fatalf("buttons = %+v", out)
	}
	f := game.FrameView{X: 4, Y: 5, Moving: true, MovingFrom: b.ID}
	c := moveCancelRect(f)
	if c.Kind != command.DesignateCancel || c.X != 4 || c.X2 != 4 || c.Y != 5 || c.Y2 != 5 {
		t.Fatalf("cancel = %+v", c)
	}
	if got, ok := pendingMoveFrame(game.WorldView{Frames: []game.FrameView{f}}, b.ID); !ok || got.X != 4 {
		t.Fatal("pendingMoveFrame")
	}
	_ = rs
}
