package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// assignDrugReader is a Reader that also reports drug policies.
type assignDrugReader struct {
	fakeReader
	policy map[entity.EntityID]drug.Preset
}

func (r assignDrugReader) DrugPolicy(id entity.EntityID) uint8 { return uint8(r.policy[id]) }

func openAssignWindow(t *testing.T, n int) (*fakeShell, *Set) {
	t.Helper()
	sh, w := newFake()
	sh.env.Game = assignDrugReader{policy: map[entity.EntityID]drug.Preset{}}
	for i := 1; i <= n; i++ {
		sh.pawns = append(sh.pawns, game.PawnView{ID: entity.EntityID(i), Name: "P"})
	}
	sh.world.FoodPolicies = []game.FoodPolicyView{{ID: 1, Name: "Anything"}, {ID: 2, Name: "Meat"}}
	w.Assign().Launch()
	return sh, w
}

func assignFrame(sh *fakeShell, w *Set) {
	sh.kit.Begin()
	w.Declare()
}

// assignPress is a click on a widget through the kit's click path, seen on the
// next declare.
func assignPress(t *testing.T, sh *fakeShell, w *Set, id string) {
	t.Helper()
	assignFrame(sh, w)
	if !sh.kit.Click(id) {
		t.Fatalf("no widget %s", id)
	}
	assignFrame(sh, w)
}

// assignHold is the held left button over cell id for one Update.
func assignHold(sh *fakeShell, w *Set, id string, down bool) {
	assignFrame(sh, w)
	sh.f.Raw.Down[inputframe.Left] = down
	sh.kit.Hot = id
	w.Assign().Drag()
}

// CTL-015: a click on a Medical cell sets the next level, and dragging the
// held button over the cells below gives them that level: three rows, three
// commands, none for a row that already has it.
func TestAssignDragPaintsTheFirstClicksValue(t *testing.T) {
	t.Parallel()
	sh, w := openAssignWindow(t, 4)
	sh.pawns[3].CareLevel = 1 // already there
	assignPress(t, sh, w, AssignCareID(1))
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.SetMedicalCare || sh.cmds[0].Pawn != 1 || sh.cmds[0].Mode != 1 {
		t.Fatalf("the click: %+v", sh.cmds)
	}
	assignHold(sh, w, AssignCareID(1), true) // still on the first cell: nothing
	assignHold(sh, w, AssignCareID(2), true)
	assignHold(sh, w, AssignCareID(3), true)
	assignHold(sh, w, AssignCareID(4), true)
	if len(sh.cmds) != 3 {
		t.Fatalf("1 click + 2 painted rows = 3 commands, got %+v", sh.cmds)
	}
	for i, pawn := range []uint64{1, 2, 3} {
		if c := sh.cmds[i]; c.Kind != command.SetMedicalCare || c.Pawn != pawn || c.Mode != 1 {
			t.Fatalf("command %d: %+v", i, c)
		}
	}
	assignHold(sh, w, AssignCareID(2), false) // released
	assignHold(sh, w, AssignCareID(1), true)
	if len(sh.cmds) != 3 || w.paint.Armed {
		t.Fatalf("a released button paints nothing: %d commands, armed %v", len(sh.cmds), w.paint.Armed)
	}
}

// Another column's cells and the same cell entered twice paint nothing.
func TestAssignDragStaysInItsColumn(t *testing.T) {
	t.Parallel()
	sh, w := openAssignWindow(t, 3)
	assignPress(t, sh, w, AssignDrugID(1))
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.SetDrugPolicy {
		t.Fatalf("the click: %+v", sh.cmds)
	}
	want := sh.cmds[0].Mode
	assignHold(sh, w, AssignCareID(2), true) // another column
	assignHold(sh, w, AssignFoodID(2), true)
	if len(sh.cmds) != 1 {
		t.Fatalf("other columns are not painted: %+v", sh.cmds)
	}
	assignHold(sh, w, AssignDrugID(2), true)
	assignHold(sh, w, "", true) // off the cells: may enter again
	assignHold(sh, w, AssignDrugID(3), true)
	if len(sh.cmds) != 3 || sh.cmds[2].Pawn != 3 || sh.cmds[2].Mode != want {
		t.Fatalf("drug column: %+v", sh.cmds)
	}
}

// The Food column paints the policy the first click picked.
func TestAssignDragPaintsFoodPolicy(t *testing.T) {
	t.Parallel()
	sh, w := openAssignWindow(t, 3)
	sh.pawns[0].FoodPolicy, sh.pawns[1].FoodPolicy, sh.pawns[2].FoodPolicy = "Anything", "Anything", "Meat"
	assignPress(t, sh, w, AssignFoodID(1)) // Anything -> Meat
	assignHold(sh, w, AssignFoodID(2), true)
	assignHold(sh, w, AssignFoodID(3), true) // already Meat
	if len(sh.cmds) != 2 || sh.cmds[1].Kind != command.SetFoodPolicy || sh.cmds[1].Pawn != 2 || sh.cmds[1].Index != 2 {
		t.Fatalf("food: %+v", sh.cmds)
	}
}
