package ebiten

import (
	architectpkg "rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
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
	b, ok := inspector.BuildingAt(a.renderer.world(), 8, 8)
	if !ok {
		t.Fatalf("no %s built", kind)
	}
	a.sel.Select(selection.Cand{Kind: selection.Building, ID: b.ID})
	return a, rs, b
}

func findBtn(a *App, id string) (inspector.Button, bool) {
	return bldCmd(a.insp.Building().BuildingCommands(a.sel.SelBuilding), id)
}

func TestMoveButtonDisabledForWalls(t *testing.T) {
	t.Parallel()
	a, _, _ := bldgApp(t, "wall", "wood", "south")
	mv, ok := findBtn(a, "bldg.move")
	if !ok || !mv.Disabled || mv.Reason != "can't be moved" {
		t.Fatalf("move button = %+v ok=%v", mv, ok)
	}
	if cp, ok := findBtn(a, "bldg.copy"); !ok || cp.Disabled {
		t.Fatalf("copy must work on walls: %+v", cp)
	}
	for _, k := range []string{"door", "grave", "fence", "fence gate"} {
		if architectpkg.MovableKind(k, false) {
			t.Fatalf("%s must not be movable", k)
		}
	}
	if architectpkg.MovableKind("rug", true) || !architectpkg.MovableKind("bed", false) {
		t.Fatal("floors are fixed, beds move")
	}
}

func TestMoveModeEnqueuesMoveBuilding(t *testing.T) {
	t.Parallel()
	a, rs, b := bldgApp(t, "butcher table", "wood", "east")
	a.insp.Building().ApplyBuildingClick("bldg.move")
	if !a.arch.Mover.State().On {
		t.Fatal("Move should enter move mode")
	}
	a.arch.Mover.Step(5, 5, true, 0, true, false) // the opening click is swallowed
	if len(rs.cmds) != 0 {
		t.Fatalf("opening frame enqueued %v", rs.cmds)
	}
	rs.moveOK = false
	a.arch.Mover.Step(5, 5, true, 0, true, false)
	if len(rs.cmds) != 0 || !a.arch.Mover.State().On || a.hint != "blocked" {
		t.Fatalf("invalid spot: cmds=%v on=%v hint=%q", rs.cmds, a.arch.Mover.State().On, a.hint)
	}
	rs.moveOK = true
	a.arch.Mover.Step(5, 5, true, 0, true, false)
	if len(rs.cmds) != 1 {
		t.Fatalf("cmds = %v", rs.cmds)
	}
	c := rs.cmds[0]
	if c.Kind != command.MoveBuilding || c.Target != uint64(b.ID) || c.X != 5 || c.Y != 5 || c.Facing != architectpkg.FacingIndex("east") {
		t.Fatalf("command = %+v", c)
	}
	if a.arch.Mover.State().On {
		t.Fatal("placing should leave move mode")
	}
}

func TestMoveModeRotates(t *testing.T) {
	t.Parallel()
	a, rs, _ := bldgApp(t, "butcher table", "wood", "south")
	if bv, _ := architectpkg.Buildable("butcher table"); !bv.Rotatable {
		t.Skip("butcher table not rotatable")
	}
	a.insp.Building().ApplyBuildingClick("bldg.move")
	a.arch.Mover.Step(5, 5, true, 0, false, false) // swallow fresh frame
	a.arch.Mover.Step(5, 5, true, 1, false, false)
	if a.arch.Mover.State().Facing != architectpkg.FacingIndex("east") {
		t.Fatalf("facing = %d", a.arch.Mover.State().Facing)
	}
	a.arch.Mover.Step(5, 5, true, -1, false, false)
	a.arch.Mover.Step(5, 5, true, -1, true, false)
	if len(rs.cmds) != 1 || rs.cmds[0].Facing != architectpkg.FacingIndex("west") {
		t.Fatalf("cmds = %+v", rs.cmds)
	}
}

func TestMoveModeCancel(t *testing.T) {
	t.Parallel()
	a, rs, _ := bldgApp(t, "butcher table", "wood", "south")
	a.insp.Building().ApplyBuildingClick("bldg.move")
	a.arch.Mover.Step(5, 5, true, 0, false, false)
	a.arch.Mover.Step(5, 5, true, 0, false, true)
	if a.arch.Mover.State().On || len(rs.cmds) != 0 {
		t.Fatalf("cancel: on=%v cmds=%v", a.arch.Mover.State().On, rs.cmds)
	}
	if a.arch.Mover.Step(5, 5, true, 0, true, false) {
		t.Fatal("inactive move mode must not consume input")
	}
}

func TestCopyOpensArchitectPreset(t *testing.T) {
	t.Parallel()
	// A butcher table cannot be stone, so kind and facing are checked on
	// it and the material presets on stone walls and a steel door.
	a, _, _ := bldgApp(t, "butcher table", "wood", "east")
	a.insp.Building().ApplyBuildingClick("bldg.copy")
	tool, ok := toolByID(a.arch.ToolID)
	if !a.arch.Open || !ok || tool.Build != "butcher table" {
		t.Fatalf("architect open=%v tool=%q", a.arch.Open, a.arch.ToolID)
	}
	if a.arch.Facing != architectpkg.FacingIndex("east") {
		t.Fatalf("facing = %d", a.arch.Facing)
	}
	for _, c := range []struct{ kind, stuff, want string }{
		{"wall", "marble blocks", "marble blocks"},
		{"wall", "stone", "granite blocks"}, // a pre-migration read model
		{"door", "steel", "steel"},
	} {
		w, _, _ := bldgApp(t, c.kind, c.stuff, "south")
		w.insp.Building().ApplyBuildingClick("bldg.copy")
		wt, _ := toolByID(w.arch.ToolID)
		if wt.Build != c.kind || w.toolStuff(wt) != c.want {
			t.Fatalf("%s %s copy: tool=%q stuff=%q", c.stuff, c.kind, w.arch.ToolID, w.arch.Stuff)
		}
	}
}

func TestCopyDisabledWhenLocked(t *testing.T) {
	t.Parallel()
	a, rs, _ := bldgApp(t, "butcher table", "wood", "south")
	rs.locked = map[string]string{"build:butcher table": "Smithing"}
	cp, ok := findBtn(a, "bldg.copy")
	if !ok || !cp.Disabled || cp.Reason != "needs research: Smithing" {
		t.Fatalf("copy = %+v", cp)
	}
	if hasBuildTool("no such kind") {
		t.Fatal("unknown kind has no tool")
	}
	if a.arch.Preset("no such kind", "", "") {
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
	if got := architectpkg.MovingLine(b); got != "Moving to (4, 5)" {
		t.Fatalf("line = %q", got)
	}
	if out := a.insp.Building().MoveCommands(b); len(out) != 3 || out[2].ID != "bldg.cancelmove" {
		t.Fatalf("buttons = %+v", out)
	}
	f := game.FrameView{X: 4, Y: 5, Moving: true, MovingFrom: b.ID}
	c := architectpkg.MoveCancelRect(f)
	if c.Kind != command.DesignateCancel || c.X != 4 || c.X2 != 4 || c.Y != 5 || c.Y2 != 5 {
		t.Fatalf("cancel = %+v", c)
	}
	if got, ok := architectpkg.PendingMoveFrame(game.WorldView{Frames: []game.FrameView{f}}, b.ID); !ok || got.X != 4 {
		t.Fatal("pendingMoveFrame")
	}
	_ = rs
}
