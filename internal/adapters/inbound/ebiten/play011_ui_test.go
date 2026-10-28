package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// swapSim scripts the placement read models for the PLAY-011 ghost.
type swapSim struct {
	*stubSim
	same    map[[2]int]bool
	facings map[[2]int]string
	label   string
}

func (s *swapSim) CanPlaceAt(kind, stuff, facing string, x, y int) (bool, string) {
	if s.same[[2]int{x, y}] {
		return false, game.ReasonSame
	}
	return true, ""
}
func (s *swapSim) ReplacesAt(kind, stuff string, x, y int) string { return s.label }
func (s *swapSim) AutoFacingAt(kind, facing string, x, y int) string {
	if f, ok := s.facings[[2]int{x, y}]; ok {
		return f
	}
	return facing
}

func TestDoorAxisFollowsFacing(t *testing.T) {
	t.Parallel()
	for f, ew := range map[string]bool{"south": true, "north": true, "": true, "east": false, "west": false} {
		if got := doorAxisEW(f); got != ew {
			t.Fatalf("doorAxisEW(%q) = %v, want %v", f, got, ew)
		}
	}
	if got := buildSpriteStuff("door", "wood", "east", 0, doorAxisEW("east"), false); got != "structure/door_wood_ns_closed" {
		t.Fatal(got)
	}
	if got := buildSpriteStuff("door", "steel", "south", 0, doorAxisEW("south"), true); got != "structure/door_steel_ew_open" {
		t.Fatal(got)
	}
}

func TestGhostCellsCarryAutoFacingAndSame(t *testing.T) {
	t.Parallel()
	a, stub := stubColony(t)
	sim := &swapSim{stubSim: stub, same: map[[2]int]bool{{5, 4}: true},
		facings: map[[2]int]string{{4, 4}: "east"}, label: "wood wall → steel"}
	a.renderer.game = sim
	a.openArchitect()
	a.arch.ToolID = "wall"
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 4, 4, 6, 4
	_, cells := a.previewCells()
	if len(cells) != 3 {
		t.Fatalf("cells = %d", len(cells))
	}
	if cells[0].facing != "east" || cells[1].facing != "south" {
		t.Fatalf("facings = %q %q", cells[0].facing, cells[1].facing)
	}
	if !cells[1].same || cells[1].ok || cells[1].reason != "" {
		t.Fatalf("same cell = %+v", cells[1])
	}
	if cells[0].same || !cells[0].ok || cells[0].replaces != "wood wall → steel" {
		t.Fatalf("swap cell = %+v", cells[0])
	}
	a.submitArchitect()
	if len(stub.cmds) != 2 {
		t.Fatalf("submitted %d commands, want 2 (same cell skipped)", len(stub.cmds))
	}
	for _, c := range stub.cmds {
		if c.X == 5 {
			t.Fatalf("same cell submitted: %+v", c)
		}
	}
}

func TestGhostDrawsEachCellWithItsOwnFacing(t *testing.T) {
	t.Parallel()
	cells := []ghostCell{{facing: "east"}, {facing: "south"}, {}}
	want := []string{"structure/door_wood_ns_closed", "structure/door_wood_ew_closed", "structure/door_wood_ew_closed"}
	for i, c := range cells {
		f := ghostFacing(c, "south")
		if got := buildSpriteStuff("door", "wood", f, 0, doorAxisEW(f), false); got != want[i] {
			t.Fatalf("cell %d key = %s, want %s", i, got, want[i])
		}
	}
}

func TestSameCellsAreNeutralAndFreeOfRed(t *testing.T) {
	t.Parallel()
	same := ghostFill(ghostCell{same: true})
	bad := ghostFill(ghostCell{})
	ok := ghostFill(ghostCell{ok: true})
	if same == bad || same == ok || same.R != same.G || same.G != same.B {
		t.Fatalf("same %v bad %v ok %v", same, bad, ok)
	}
}

func TestDoorGateHintsDropRotate(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"door", "fence_gate"} {
		tool, ok := toolByID(id)
		if !ok || strings.Contains(tool.desc, "Q/E") {
			t.Fatalf("%s desc = %q", id, tool.desc)
		}
	}
}

func TestSwapLine(t *testing.T) {
	t.Parallel()
	if swapLine(game.BuildingView{}) != "" {
		t.Fatal("no swap, no line")
	}
	if got := swapLine(game.BuildingView{SwapTo: "steel", SwapPct: 40}); got != "Changing to steel (40%)" {
		t.Fatal(got)
	}
}

func TestFencesAndGatesNotMovable(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"fence", "fence gate"} {
		if movableKind(k, false) {
			t.Fatalf("%s must not be movable", k)
		}
	}
}
