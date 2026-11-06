package windows

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func apparelFixture(t *testing.T, n int) (*fakeShell, *Set) {
	t.Helper()
	sh, w := openAssignWindow(t, n)
	sh.world.Gear.Policies = []gearview.Policy{
		{ID: 1, Name: "Anything", Default: true, Allowed: []string{"Tee shirt"}, Stuffs: []string{"cloth"}, HPMax: 100, QualityMax: 6, NoTainted: true},
		{ID: 2, Name: "Worker", HPMin: 50, HPMax: 100, QualityMax: 6},
		{ID: 5, Name: "Nudist", HPMax: 100, QualityMax: 6},
	}
	for i := range sh.pawns {
		sh.pawns[i].Gear.ApparelPolicy, sh.pawns[i].Gear.ApparelPolicyName = 1, "Anything"
	}
	return sh, w
}

func TestNextApparelPolicyWraps(t *testing.T) {
	t.Parallel()
	ps := []gearview.Policy{{ID: 3}, {ID: 5}, {ID: 9}}
	for cur, want := range map[uint32]int{3: 5, 5: 9, 9: 3, 77: 3} {
		if got, ok := NextApparelPolicy(ps, cur); !ok || got != want {
			t.Errorf("after %d: got %d (%v), want %d", cur, got, ok, want)
		}
	}
	if _, ok := NextApparelPolicy(nil, 1); ok {
		t.Error("no policies: ok")
	}
}

func TestAssignApparelClickCyclesAndSends(t *testing.T) {
	t.Parallel()
	sh, w := apparelFixture(t, 2)
	assignPress(t, sh, w, AssignApparelID(1))
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.SetApparelPolicy || sh.cmds[0].Pawn != 1 || sh.cmds[0].Index != 2 {
		t.Fatalf("click: %+v", sh.cmds)
	}
}

func TestAssignDragPaintsApparelPolicy(t *testing.T) {
	t.Parallel()
	sh, w := apparelFixture(t, 4)
	sh.pawns[2].Gear.ApparelPolicy = 2 // already there
	assignPress(t, sh, w, AssignApparelID(1))
	assignHold(sh, w, AssignApparelID(1), true)
	assignHold(sh, w, AssignApparelID(2), true)
	assignHold(sh, w, AssignApparelID(3), true)
	assignHold(sh, w, AssignApparelID(4), true)
	if len(sh.cmds) != 3 {
		t.Fatalf("1 click + 2 painted = 3 commands: %+v", sh.cmds)
	}
	for i, pawn := range []uint64{1, 2, 4} {
		if c := sh.cmds[i]; c.Kind != command.SetApparelPolicy || c.Pawn != pawn || c.Index != 2 {
			t.Fatalf("command %d: %+v", i, c)
		}
	}
	assignHold(sh, w, AssignDrugID(1), true) // another column paints nothing here
}

func TestClearForcedNeedsAForcedGarment(t *testing.T) {
	t.Parallel()
	sh, w := apparelFixture(t, 2)
	sh.pawns[1].ApparelForced = []bool{false, true}
	assignFrame(sh, w)
	if wd, ok := sh.kit.Find(AssignClearID(1)); !ok || !wd.Disabled || wd.Reason == "" {
		t.Fatalf("pawn 1 has nothing forced: %+v (%v)", wd, ok)
	}
	assignPress(t, sh, w, AssignClearID(1))
	if len(sh.cmds) != 0 {
		t.Fatalf("a disabled button sent %+v", sh.cmds)
	}
	assignPress(t, sh, w, AssignClearID(2))
	if len(sh.cmds) != 1 || sh.cmds[0].Kind != command.ClearForcedApparel || sh.cmds[0].Pawn != 2 {
		t.Fatalf("clear forced: %+v", sh.cmds)
	}
}

func TestApparelHeaderOpensTheEditor(t *testing.T) {
	t.Parallel()
	sh, w := apparelFixture(t, 1)
	assignPress(t, sh, w, AssignApparelEditID)
	if w.KitID() != ApparelPolicyModalID {
		t.Fatalf("open window %q", w.KitID())
	}
	assignFrame(sh, w)
	assignPress(t, sh, w, ApparelBackID)
	if w.KitID() != AssignModalID {
		t.Fatalf("Back left %q", w.KitID())
	}
}

// openApparelEditor opens the editor in a window as tall as the real one.
func openApparelEditor(t *testing.T, n int) (*fakeShell, *Set) {
	t.Helper()
	sh, _ := apparelFixture(t, n)
	w := New(sizeShell{sh, 1920, 1080})
	w.Assign().OpenApparelPolicies()
	assignFrame(sh, w)
	return sh, w
}

func lastEdit(t *testing.T, sh *fakeShell) command.Command {
	t.Helper()
	if len(sh.cmds) == 0 {
		t.Fatal("no command sent")
	}
	c := sh.cmds[len(sh.cmds)-1]
	if c.Kind != command.EditApparelPolicy {
		t.Fatalf("sent %+v", c)
	}
	return c
}

func TestApparelEditorCreateCopyDeleteDefault(t *testing.T) {
	t.Parallel()
	sh, w := openApparelEditor(t, 1)
	assignPress(t, sh, w, ApparelNewID)
	if c := lastEdit(t, sh); c.Mode != command.ApparelPolicyCreate || c.Index != -1 || c.Name == "" {
		t.Fatalf("New: %+v", c)
	}
	assignPress(t, sh, w, ApparelCopyID)
	if c := lastEdit(t, sh); c.Mode != command.ApparelPolicyCreate || c.Index != 1 {
		t.Fatalf("Copy: %+v", c)
	}
	// the default policy cannot be deleted, and says why
	if wd, _ := sh.kit.Find(ApparelDeleteID); !wd.Disabled || wd.Reason == "" {
		t.Fatalf("Delete on the default: %+v", wd)
	}
	if wd, _ := sh.kit.Find(ApparelDefaultID); !wd.Disabled {
		t.Fatalf("the default toggle on the default: %+v", wd)
	}
	assignPress(t, sh, w, ApparelSelectID(2))
	assignFrame(sh, w)
	if wd, _ := sh.kit.Find(ApparelDeleteID); wd.Disabled {
		t.Fatalf("Delete on a spare policy is disabled: %+v", wd)
	}
	n := len(sh.cmds)
	assignPress(t, sh, w, ApparelDefaultID)
	if c := lastEdit(t, sh); len(sh.cmds) != n+1 || c.Mode != command.ApparelPolicyDefault || c.Index != 2 {
		t.Fatalf("Default: %+v", c)
	}
	assignPress(t, sh, w, ApparelDeleteID)
	if c := lastEdit(t, sh); c.Mode != command.ApparelPolicyDelete || c.Index != 2 {
		t.Fatalf("Delete: %+v", c)
	}
}

func TestApparelEditorLastPolicyStays(t *testing.T) {
	t.Parallel()
	sh, w := apparelFixture(t, 1)
	sh.world.Gear.Policies = []gearview.Policy{{ID: 4, Name: "Only"}}
	w.Assign().OpenApparelPolicies()
	assignFrame(sh, w)
	if wd, _ := sh.kit.Find(ApparelDeleteID); !wd.Disabled || wd.Reason == "" {
		t.Fatalf("Delete the last policy: %+v", wd)
	}
}

func TestApparelEditorToggles(t *testing.T) {
	t.Parallel()
	sh, w := openApparelEditor(t, 1)
	assignPress(t, sh, w, ApparelTaintedID) // Anything refuses tainted: allow it
	if c := lastEdit(t, sh); c.Mode != command.ApparelPolicyTainted || !c.Enabled || c.Index != 1 {
		t.Fatalf("tainted: %+v", c)
	}
	assignPress(t, sh, w, apparelStuffPrefix+"cloth") // allowed: forbid
	if c := lastEdit(t, sh); c.Mode != command.ApparelPolicyStuff || c.Enabled || c.Name != "cloth" {
		t.Fatalf("material: %+v", c)
	}
	assignPress(t, sh, w, ApparelClearAllID)
	if c := lastEdit(t, sh); c.Mode != command.ApparelPolicyAllow || c.Enabled || c.Name != apparelRoot {
		t.Fatalf("clear all: %+v", c)
	}
	assignPress(t, sh, w, ApparelAllowAllID)
	if c := lastEdit(t, sh); c.Mode != command.ApparelPolicyAllow || !c.Enabled {
		t.Fatalf("allow all: %+v", c)
	}
}

func TestApparelRowsKeepOnlyApparelThenMaterials(t *testing.T) {
	t.Parallel()
	rows := apparelRows(gearview.Policy{}, map[string]bool{})
	sawHead, mats := false, 0
	for _, r := range rows {
		switch {
		case r.head != "":
			sawHead = true
		case r.stuff != "":
			mats++
		case r.node.Path != apparelRoot && !strings.HasPrefix(r.node.Path, apparelRoot+"/"):
			t.Errorf("row %q outside Apparel", r.node.Path)
		}
	}
	if !sawHead || mats == 0 {
		t.Fatalf("heading %v, %d materials", sawHead, mats)
	}
	open := apparelRows(gearview.Policy{}, map[string]bool{apparelRoot: true})
	if len(open) <= len(rows) {
		t.Errorf("expanding Apparel shows %d rows, collapsed %d", len(open), len(rows))
	}
}

// sizeShell reports a screen of another size, as another UI scale does.
type sizeShell struct {
	*fakeShell
	sw, sh int
}

func (s sizeShell) Size() (int, int) { return s.sw, s.sh }
func (s sizeShell) WindowRect() gfx.Rect {
	return gfx.Rect{X: 0, Y: 40, W: min(880, float32(s.sw)-250), H: min(480, float32(s.sh)-120)}
}

// At every UI scale the screen of 1920x1080 maps to, the Assign window and the
// editor keep every widget inside the window.
func TestApparelWindowsFitAtEveryUIScale(t *testing.T) {
	t.Parallel()
	for _, pct := range []int{75, 100, 125, 150, 175, 200} {
		for _, editor := range []bool{false, true} {
			f := &fakeShell{}
			sh := sizeShell{f, 1920 * 100 / pct, 1080 * 100 / pct}
			f.kit = uikit.New(fakeHost{f})
			f.env.Game = assignDrugReader{}
			f.world.Gear.Policies = []gearview.Policy{{ID: 1, Name: "Anything", Default: true}, {ID: 2, Name: "Worker"}}
			f.world.FoodPolicies = []game.FoodPolicyView{{ID: 1, Name: "Anything"}}
			for i := 1; i <= 12; i++ {
				f.pawns = append(f.pawns, game.PawnView{ID: entity.EntityID(i), Name: "Colonist", ApparelForced: []bool{true}})
				f.pawns[i-1].Gear.ApparelPolicy, f.pawns[i-1].Gear.ApparelPolicyName = 1, "Anything"
			}
			w := New(sh)
			if editor {
				w.Assign().OpenApparelPolicies()
			} else {
				w.Assign().Launch()
			}
			f.kit.Begin()
			w.Declare()
			win := sh.WindowRect()
			seen := 0
			for _, wd := range f.kit.Cur {
				if !strings.HasPrefix(wd.ID, "assign.") && !strings.HasPrefix(wd.ID, "apparelpolicy.") {
					continue
				}
				seen++
				if r := wd.R; r.X < win.X-0.5 || r.Y < win.Y-0.5 || r.Right() > win.Right()+0.5 || r.Bottom() > win.Bottom()+0.5 {
					t.Errorf("%d%% editor=%v: %s at %+v leaves the window %+v", pct, editor, wd.ID, r, win)
				}
			}
			if seen < 8 {
				t.Errorf("%d%% editor=%v: only %d widgets declared", pct, editor, seen)
			}
		}
	}
}
