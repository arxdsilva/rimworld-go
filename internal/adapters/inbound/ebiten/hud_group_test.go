package ebiten

import (
	"reflect"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Group selection (PLAY-018, CORE-PANE): merged commands and the group
// pane. The merge is a pure function of the members' command lists.

// Members whose commands carry a legacy id (no Run, no Cmds) act on the
// single selection: the merge selects each member for its call and puts the
// group back.
func TestGroupCommandRunsLegacyMembersOneByOne(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{3, 3}, [2]int{4, 3}, [2]int{5, 3})
	a.sel.SetGroup(selectionpkg.Pawn, []entity.EntityID{1, 2, 3})
	a.setTestCommands(func(*App, selectionpkg.Kind, entity.EntityID) []inspector.Button {
		return []inspector.Button{{ID: "pawn.clearorders", Key: "clear", Label: "Clear orders"}}
	})
	f.Frame(t, a)
	var merged *inspector.Button
	for _, c := range a.insp.Commands().SelectionCommands() {
		if c.Key == "clear" {
			merged = &c
		}
	}
	if merged == nil || merged.ID != "cmd.group.clear" {
		t.Fatalf("merged = %+v", merged)
	}
	click(t, a, merged.ID)
	got := cmdsOf(s, command.ClearOrders)
	if len(got) != 3 || got[0].Pawn != 1 || got[1].Pawn != 2 || got[2].Pawn != 3 {
		t.Fatalf("ClearOrders for each member: %+v", s.cmds)
	}
	if !a.sel.Grouped() || len(a.sel.Group.Ids) != 3 {
		t.Fatalf("the group survives the click: %+v", a.sel.Group)
	}
}

func TestPawnGroupDraftAndUndraft(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{3, 3}, [2]int{4, 3}, [2]int{5, 3})
	a.sel.SetGroup(selectionpkg.Pawn, []entity.EntityID{1, 2, 3})
	a.setTestCommands(noCommands) // only the group's own commands
	f.Frame(t, a)
	draft := func() inspector.Button { return a.insp.Commands().SelectionCommands()[0] }
	var ids []string
	for _, c := range a.insp.Commands().SelectionCommands() {
		ids = append(ids, c.ID)
	}
	if !reflect.DeepEqual(ids, []string{"group.draft", "group.undraft"}) {
		t.Fatalf("pawn group commands = %v", ids)
	}
	if c := draft(); c.On || c.Mixed || c.Hotkey != input.Draft {
		t.Fatalf("nobody drafted: %+v", c)
	}
	s.pawns[0].Drafted = true
	if c := draft(); c.On || !c.Mixed {
		t.Fatalf("one drafted is mixed: %+v", c)
	}
	click(t, a, "group.draft")
	if got := cmdsOf(s, command.DraftPawn); len(got) != 2 || !got[0].Enabled {
		t.Fatalf("drafts the two that are not: %+v", s.cmds)
	}
	for i := range s.pawns {
		s.pawns[i].Drafted = true
	}
	if c := draft(); !c.On || c.Mixed {
		t.Fatalf("all drafted: %+v", c)
	}
	s.cmds = nil
	click(t, a, "group.undraft")
	if got := cmdsOf(s, command.DraftPawn); len(got) != 3 || got[0].Enabled {
		t.Fatalf("undrafts all three: %+v", s.cmds)
	}
}

func TestItemGroupForbidAllowAndHaul(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: foundItems()} // 21 found, 22 found and marked, 23 colony
	a.sel.SetGroup(selectionpkg.Item, []entity.EntityID{21, 22, 23})
	a.setTestCommands(noCommands) // only the group's own commands
	var ids []string
	for _, c := range a.insp.Commands().SelectionCommands() {
		ids = append(ids, c.ID)
	}
	if !reflect.DeepEqual(ids, []string{"group.forbid", "group.allow", "group.haulmark"}) {
		t.Fatalf("item group commands = %v", ids)
	}
	click(t, a, "group.haulmark")
	if got := cmdsOf(s, command.SetHaulMark); len(got) != 1 || got[0].Target != 21 {
		t.Fatalf("marks the found, unmarked stack only: %+v", s.cmds)
	}
	s.world.Items[0].Forbidden = true
	if c := a.insp.Commands().SelectionCommands()[0]; c.On || !c.Mixed {
		t.Fatalf("one forbidden is mixed: %+v", c)
	}
	s.world.Items[1].Forbidden = true
	s.world.Items[2].Forbidden = true
	if c := a.insp.Commands().SelectionCommands()[0]; !c.On || c.Mixed {
		t.Fatalf("all forbidden: %+v", c)
	}
	s.cmds = nil
	click(t, a, "group.allow")
	if got := cmdsOf(s, command.SetForbidden); len(got) != 3 || got[0].Enabled {
		t.Fatalf("allows all three: %+v", s.cmds)
	}
}

func TestGroupPaneListsTheMembers(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{3, 3}, [2]int{4, 3}, [2]int{5, 3})
	a.sel.SetGroup(selectionpkg.Pawn, []entity.EntityID{1, 2, 3})
	f.Frame(t, a)
	spec := a.insp.SelectionPane()
	if spec.Title != "3 colonists" || spec.Close != "group.close" {
		t.Fatalf("pane = %+v", spec)
	}
	if len(spec.Rows) != 3 || spec.Rows[0].ID != "group.pick.1" || !strings.Contains(spec.Rows[0].Label, "Ada") {
		t.Fatalf("rows = %+v", spec.Rows)
	}
	if !a.insp.PaneShown() || a.hud().Pane.Empty() {
		t.Fatal("a group has a pane")
	}
	for _, r := range a.chromeRects(a.sw, a.sh) {
		if r.Empty() {
			t.Fatalf("empty chrome rect %+v", r)
		}
	}
	for _, b := range a.insp.PaneButtons() {
		if b.ID == "group.pick.2" {
			click(t, a, "group.pick.2")
			if a.sel.Grouped() || !a.renderer.HasSelect() || a.renderer.Selected() != 2 {
				t.Fatalf("a row selects only that colonist: group=%+v", a.sel.Group)
			}
			return
		}
	}
	t.Fatal("no group.pick.2 button in the pane")
}

func TestGroupHasNoLegacyPanel(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{3, 3}, [2]int{4, 3})
	a.sel.SetGroup(selectionpkg.Pawn, []entity.EntityID{1, 2})
	f.Frame(t, a)
	pane := a.hud().Pane
	for _, r := range a.chromeRects(a.sw, a.sh) {
		if r.X == 8 && r.Y == 40 { // the legacy group panel sat at 8,40
			t.Fatalf("the legacy group rect %+v is still in the chrome (pane is %+v)", r, pane)
		}
	}
	assertChromeIsTheLayout(t, a)
}
