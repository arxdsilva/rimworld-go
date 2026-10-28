package ebiten

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Group selection (PLAY-018, CORE-PANE): merged commands and the group
// pane. The merge is a pure function of the members' command lists.

func keysOf(cs []cmdButton) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Key
	}
	return out
}

func TestMergeKeepsTheFirstDeclaredOrder(t *testing.T) {
	t.Parallel()
	sets := [][]cmdButton{
		{{Key: "a", Label: "A"}, {Key: "b", Label: "B"}},
		{{Key: "b", Label: "B"}, {ID: "cmd.pawn.c", Label: "C"}, {Key: "a", Label: "A"}},
	}
	got := mergeCommands(sets, nil)
	var ids []string
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	want := []string{"cmd.group.a", "cmd.group.b", "cmd.group.pawn.c"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("merged ids = %v, want %v (first declaration wins; a missing Key falls back to the ID)", ids, want)
	}
	if got[0].Label != "A" || got[2].Label != "C" {
		t.Fatalf("labels = %q %q", got[0].Label, got[2].Label)
	}
}

func TestMergeDisabledOnlyWhenEveryMemberIs(t *testing.T) {
	t.Parallel()
	off := func(reason string) cmdButton {
		return cmdButton{Key: "bed", Label: "Bed", Disabled: true, Reason: reason}
	}
	on := cmdButton{Key: "bed", Label: "Bed"}

	if got := mergeCommands([][]cmdButton{{off("no bed")}, {off("no bed")}, {on}}, nil)[0]; got.Disabled {
		t.Fatalf("one capable member keeps the merged command enabled: %+v", got)
	}
	got := mergeCommands([][]cmdButton{{off("no bed")}, {off("no bed")}, {off("asleep")}}, nil)[0]
	if !got.Disabled || got.Reason != "no bed (2 of 3)" {
		t.Fatalf("all disabled: Disabled=%v Reason=%q, want %q", got.Disabled, got.Reason, "no bed (2 of 3)")
	}
}

func TestMergeToggleOnAndMixed(t *testing.T) {
	t.Parallel()
	tog := func(on bool) cmdButton { return cmdButton{Key: "draft", Label: "Draft", Toggle: true, On: on} }
	cases := []struct {
		name         string
		members      []bool
		on, mixed    bool
		wantRunsOnly []int // members a click runs
	}{
		{"all on", []bool{true, true, true}, true, false, []int{0, 1, 2}},
		{"all off", []bool{false, false}, false, false, []int{0, 1}},
		{"mixed", []bool{true, false, false}, false, true, []int{1, 2}},
	}
	for _, c := range cases {
		var sets [][]cmdButton
		for _, on := range c.members {
			sets = append(sets, []cmdButton{tog(on)})
		}
		var ran []int
		got := mergeCommands(sets, func(_ *App, member int, _ cmdButton) { ran = append(ran, member) })[0]
		if !got.Toggle || got.On != c.on || got.Mixed != c.mixed {
			t.Errorf("%s: Toggle=%v On=%v Mixed=%v, want On=%v Mixed=%v", c.name, got.Toggle, got.On, got.Mixed, c.on, c.mixed)
		}
		got.Run(nil)
		if !reflect.DeepEqual(ran, c.wantRunsOnly) {
			t.Errorf("%s: a click ran members %v, want those whose On equals the merged On: %v", c.name, ran, c.wantRunsOnly)
		}
	}
}

func TestMergedClickSkipsDisabledMembersAndDropsOptions(t *testing.T) {
	t.Parallel()
	opt := func(*App) []cmdOption { return []cmdOption{{Label: "x"}} }
	sets := [][]cmdButton{
		{{Key: "k", Label: "K", Options: opt}},
		{{Key: "k", Label: "K", Disabled: true, Reason: "no"}},
		{{Key: "k", Label: "K"}},
	}
	var ran []int
	got := mergeCommands(sets, func(_ *App, member int, _ cmdButton) { ran = append(ran, member) })[0]
	if got.Options != nil {
		t.Fatal("options are per member: a merged command has none")
	}
	got.Run(nil)
	if !reflect.DeepEqual(ran, []int{0, 2}) {
		t.Fatalf("ran members %v, want the enabled ones [0 2]", ran)
	}
}

func TestMergeKeepsAKeyFromOnlySomeMembers(t *testing.T) {
	t.Parallel()
	got := mergeCommands([][]cmdButton{{{Key: "only"}}, {{Key: "other"}}}, nil)
	if !reflect.DeepEqual(keysOf(got), []string{"only", "other"}) {
		t.Fatalf("merged keys = %v", keysOf(got))
	}
}

// Members whose commands carry a legacy id (no Run, no Cmds) act on the
// single selection: the merge selects each member for its call and puts the
// group back.
func TestGroupCommandRunsLegacyMembersOneByOne(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{3, 3}, [2]int{4, 3}, [2]int{5, 3})
	a.setGroup(selPawn, []entity.EntityID{1, 2, 3})
	a.setTestCommands(func(*App, selKind, entity.EntityID) []cmdButton {
		return []cmdButton{{ID: "pawn.clearorders", Key: "clear", Label: "Clear orders"}}
	})
	f.Frame(t, a)
	var merged *cmdButton
	for _, c := range a.selectionCommands() {
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
	if !a.grouped() || len(a.group.ids) != 3 {
		t.Fatalf("the group survives the click: %+v", a.group)
	}
}

func TestPawnGroupDraftAndUndraft(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{3, 3}, [2]int{4, 3}, [2]int{5, 3})
	a.setGroup(selPawn, []entity.EntityID{1, 2, 3})
	a.setTestCommands(noCommands) // only the group's own commands
	f.Frame(t, a)
	draft := func() cmdButton { return a.selectionCommands()[0] }
	var ids []string
	for _, c := range a.selectionCommands() {
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
	a.setGroup(selItem, []entity.EntityID{21, 22, 23})
	a.setTestCommands(noCommands) // only the group's own commands
	var ids []string
	for _, c := range a.selectionCommands() {
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
	if c := a.selectionCommands()[0]; c.On || !c.Mixed {
		t.Fatalf("one forbidden is mixed: %+v", c)
	}
	s.world.Items[1].Forbidden = true
	s.world.Items[2].Forbidden = true
	if c := a.selectionCommands()[0]; !c.On || c.Mixed {
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
	a.setGroup(selPawn, []entity.EntityID{1, 2, 3})
	f.Frame(t, a)
	spec := a.selectionPane()
	if spec.Title != "3 colonists" || spec.Close != "group.close" {
		t.Fatalf("pane = %+v", spec)
	}
	if len(spec.Rows) != 3 || spec.Rows[0].ID != "group.pick.1" || !strings.Contains(spec.Rows[0].Label, "Ada") {
		t.Fatalf("rows = %+v", spec.Rows)
	}
	if !a.paneShown() || a.hud().Pane.Empty() {
		t.Fatal("a group has a pane")
	}
	for _, r := range a.chromeRects(a.sw, a.sh) {
		if r.Empty() {
			t.Fatalf("empty chrome rect %+v", r)
		}
	}
	for _, b := range a.paneButtons() {
		if b.id == "group.pick.2" {
			click(t, a, "group.pick.2")
			if a.grouped() || !a.renderer.HasSelect() || a.renderer.Selected() != 2 {
				t.Fatalf("a row selects only that colonist: group=%+v", a.group)
			}
			return
		}
	}
	t.Fatal("no group.pick.2 button in the pane")
}

func TestGroupHasNoLegacyPanel(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{3, 3}, [2]int{4, 3})
	a.setGroup(selPawn, []entity.EntityID{1, 2})
	f.Frame(t, a)
	pane := a.hud().Pane
	for _, r := range a.chromeRects(a.sw, a.sh) {
		if r.X == 8 && r.Y == 40 { // the legacy group panel sat at 8,40
			t.Fatalf("the legacy group rect %+v is still in the chrome (pane is %+v)", r, pane)
		}
	}
	assertChromeIsTheLayout(t, a)
}
