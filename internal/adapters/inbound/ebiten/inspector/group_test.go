package inspector

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// groupShell is a fake shell whose selection model reads the fake world,
// and that records the menus it is asked to open.
type groupShell struct {
	*fakeShell
	menus [][]game.OrderOption
	pick  func(int)
}

func (g *groupShell) Pick(_, _ float32, _ bool, entries []game.OrderOption, pick func(int)) {
	g.menus = append(g.menus, entries)
	g.pick = pick
}
func (g *groupShell) Cursor() (int, int) { return 0, 0 }

// selShell is the part of the shell the selection model reads.
type selShell struct {
	selection.Shell
	f *fakeShell
}

func (s selShell) World() game.WorldView             { return s.f.world }
func (s selShell) Pawns() []game.PawnView            { return s.f.pawns }
func (s selShell) Selected() (entity.EntityID, bool) { return s.f.Selected() }
func (s selShell) Enqueue(c command.Command)         { s.f.Enqueue(c) }
func (s selShell) ClearSelect()                      {}

// selHost is the host the model reports zone changes to.
type selHost struct{ selection.Host }

func (selHost) SetZone(uint16, bool) {}

func newGroupShell() *groupShell {
	f := newFake()
	f.sel = selection.New(selShell{f: f}, selHost{})
	f.env.Updates = 1
	return &groupShell{fakeShell: f}
}

func doors(ids ...entity.EntityID) []game.BuildingView {
	var out []game.BuildingView
	for _, id := range ids {
		out = append(out, game.BuildingView{ID: id, Kind: doorKind, X: int(id), Y: 1, W: 1, H: 1})
	}
	return out
}

func sets(cmds []command.Command, k command.Kind) (n int, on bool) {
	for _, c := range cmds {
		if c.Kind == k {
			n++
			on = c.Enabled
		}
	}
	return n, on
}

func groupButton(t *testing.T, p *Pane, label string) Button {
	t.Helper()
	for _, c := range p.Commands().SelectionCommands() {
		if c.Label == label {
			return c
		}
	}
	t.Fatalf("no %q button in %v", label, labels(p.Commands().SelectionCommands()))
	return Button{}
}

func labels(bs []Button) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Label)
	}
	return out
}

func TestDoorGroupForbidsAllAndOffersNoMove(t *testing.T) {
	g := newGroupShell()
	g.world.Buildings = doors(1, 2, 3)
	g.sel.SetGroup(selection.Building, []entity.EntityID{1, 2, 3})
	p := New(g)

	if spec := p.Commands().GroupPane(); spec.Title != "3 doors" || len(spec.Rows) != 3 {
		t.Fatalf("pane = %q with %d rows", spec.Title, len(spec.Rows))
	}
	for _, l := range labels(p.Commands().SelectionCommands()) {
		if l == "Move" || l == "Copy" {
			t.Fatalf("%s must not appear in a group", l)
		}
	}
	p.Commands().ApplyCommandClick(groupButton(t, p, "Forbid").ID)
	if n, on := sets(g.cmds, command.SetForbidden); n != 3 || !on {
		t.Fatalf("want 3 SetForbidden on, got %+v", g.cmds)
	}
}

func TestDoorGroupForbidFollowsTheStackRule(t *testing.T) {
	g := newGroupShell()
	g.world.Buildings = doors(1, 2, 3)
	g.world.Buildings[0].Forbidden = true
	g.sel.SetGroup(selection.Building, []entity.EntityID{1, 2, 3})
	p := New(g)

	b := groupButton(t, p, "Forbid")
	if !b.Mixed {
		t.Fatalf("one of three forbidden is mixed: %+v", b)
	}
	p.Commands().ApplyCommandClick(b.ID)
	if n, _ := sets(g.cmds, command.SetForbidden); n != 2 {
		t.Fatalf("a mixed group forbids the two that are not: %+v", g.cmds)
	}
	g.cmds = nil
	for i := range g.world.Buildings {
		g.world.Buildings[i].Forbidden = true
	}
	p.Building().ForbidSelection()
	if n, on := sets(g.cmds, command.SetForbidden); n != 3 || on {
		t.Fatalf("F on an all-forbidden group allows all: %+v", g.cmds)
	}
	g.cmds = nil
	g.world.Buildings[1].Forbidden = false
	p.Building().ForbidSelection()
	if n, on := sets(g.cmds, command.SetForbidden); n != 1 || !on {
		t.Fatalf("F forbids the one still allowed: %+v", g.cmds)
	}
}

func TestDoorGroupDeconstructMarksAll(t *testing.T) {
	g := newGroupShell()
	g.world.Buildings = doors(1, 2, 3)
	g.sel.SetGroup(selection.Building, []entity.EntityID{1, 2, 3})
	p := New(g)

	b := groupButton(t, p, "Deconstruct")
	p.Commands().ApplyCommandClick(b.ID)
	if !reflect.DeepEqual(g.clicks, []string{"bldg.deconstruct", "bldg.deconstruct", "bldg.deconstruct"}) {
		t.Fatalf("clicks = %v", g.clicks)
	}
	if !g.sel.Grouped() {
		t.Fatal("the group survives the click")
	}
}

func TestMixedGroupTitleAndOwnCommands(t *testing.T) {
	g := newGroupShell()
	g.world.Buildings = doors(1)
	g.world.Items = []game.ItemView{{ID: 5, Kind: "wood", Qty: 3}}
	g.pawns = []game.PawnView{{ID: 9, Name: "Ada", Status: "alive"}}
	g.sel.SetMembers([]selection.Cand{{Kind: selection.Building, ID: 1}, {Kind: selection.Item, ID: 5}, {Kind: selection.Pawn, ID: 9}})
	p := New(g)
	none := Producer{Commands: func(entity.EntityID) []Button { return nil }}
	p.Producers[selection.Item], p.Producers[selection.Pawn] = none, none // the group's own commands only

	if spec := p.Commands().GroupPane(); spec.Title != "Various (3)" || len(spec.Rows) != 3 {
		t.Fatalf("pane = %q with %d rows", spec.Title, len(spec.Rows))
	}
	got := labels(p.Commands().SelectionCommands())
	count := map[string]int{}
	for _, l := range got {
		count[l]++
	}
	if count["Draft"] != 1 || count["Forbid"] != 1 || count["Allow"] != 1 {
		t.Fatalf("one Draft, one Forbid, one Allow: %v", got)
	}
	p.Commands().ApplyCommandClick(groupButton(t, p, "Forbid").ID)
	if n, on := sets(g.cmds, command.SetForbidden); n != 2 || !on {
		t.Fatalf("Forbid covers the stack and the door: %+v", g.cmds)
	}
}

func TestGroupOfDoorsAndBedsKeepsForbid(t *testing.T) {
	// No stack and no colonist: the group offers nothing of its own, so the
	// buildings' Forbid is not filtered away.
	g := newGroupShell()
	g.world.Buildings = doors(1, 2)
	g.world.Frames = []game.FrameView{{ID: 8, Kind: "wall", X: 4, Y: 4}}
	g.sel.SetMembers([]selection.Cand{{Kind: selection.Building, ID: 1}, {Kind: selection.Building, ID: 2}, {Kind: selection.Frame, ID: 8}})
	p := New(g)
	if l := labels(p.Commands().SelectionCommands()); !slicesHas(l, "Forbid") {
		t.Fatalf("buttons = %v", l)
	}
}

func slicesHas(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestPruneGroupDropsGoneMembersOfAMixedGroup(t *testing.T) {
	g := newGroupShell()
	g.world.Buildings = doors(1)
	g.world.Items = []game.ItemView{{ID: 5, Kind: "wood", Qty: 3}}
	g.sel.SetMembers([]selection.Cand{{Kind: selection.Building, ID: 1}, {Kind: selection.Item, ID: 5}, {Kind: selection.Building, ID: 99}})
	p := New(g)
	p.Commands().PruneGroup()
	if !g.sel.Grouped() || len(g.sel.Group.Members) != 2 {
		t.Fatalf("members = %+v", g.sel.Group.Members)
	}
	g.world.Items = nil
	p.Commands().PruneGroup()
	if g.sel.Grouped() || !g.sel.HasBuilding || g.sel.SelBuilding != 1 {
		t.Fatalf("a group of one is the single selection: %+v", g.sel.Group)
	}
}

// zoneCrop stands for the root's growing-zone command: its crop list is
// Options with Menu, the current crop marked.
func zoneCrop(z *[]string, id uint16, cur string, sent *[]command.Command) Button {
	return Button{ID: "cmd.zone.crop", Menu: true, Label: "Plant: " + cur,
		Options: func() []Option {
			var out []Option
			for _, c := range []string{"rice", "corn"} {
				label := c
				if c == cur {
					label += " (current)"
				}
				out = append(out, Option{Label: label, Run: func() {
					if c != cur {
						*sent = append(*sent, command.Command{Kind: command.SetZoneCrop, Target: uint64(id), Name: c})
					}
				}})
			}
			return out
		}}
}

func TestZoneGroupCropIsOneMenuForBoth(t *testing.T) {
	g := newGroupShell()
	g.world.Zones = []game.ZoneView{{ID: 1, Kind: game.ZoneGrow, Crop: "rice"}, {ID: 2, Kind: game.ZoneGrow, Crop: "corn"}}
	g.sel.SetGroup(selection.Zone, []entity.EntityID{1, 2})
	p := New(g)
	var sent []command.Command
	p.Producers[selection.Zone] = Producer{Commands: func(id entity.EntityID) []Button {
		cur := map[entity.EntityID]string{1: "rice", 2: "corn"}[id]
		return []Button{zoneCrop(nil, uint16(id), cur, &sent), {ID: "cmd.zone.rename", Label: "Rename", Single: true, Run: func() { t.Fatal("rename is single") }}}
	}}

	if spec := p.Commands().GroupPane(); spec.Title != "2 zones" {
		t.Fatalf("title = %q", spec.Title)
	}
	cmds := p.Commands().SelectionCommands()
	if len(cmds) != 1 || cmds[0].Options == nil || !cmds[0].Menu {
		t.Fatalf("one merged crop button with a menu, Rename gone: %v", labels(cmds))
	}
	p.Commands().ApplyCommandClick(cmds[0].ID)
	if len(g.menus) != 1 || len(g.menus[0]) != 2 {
		t.Fatalf("one menu of two crops: %+v", g.menus)
	}
	if g.menus[0][0].Label != "rice" || g.menus[0][1].Label != "corn" {
		t.Fatalf("a crop current in one zone is not current in the group: %+v", g.menus[0])
	}
	g.pick(0) // rice: zone 1 has it already
	if len(sent) != 1 || sent[0].Target != 2 || sent[0].Name != "rice" {
		t.Fatalf("rice goes to the zone that lacks it: %+v", sent)
	}
	sent = nil
	g.pick(1)
	if len(sent) != 1 || sent[0].Target != 1 || sent[0].Name != "corn" {
		t.Fatalf("corn goes to zone 1: %+v", sent)
	}
}

func TestSingleZoneCropOpensItsMenuOnClick(t *testing.T) {
	g := newGroupShell()
	g.world.Zones = []game.ZoneView{{ID: 1, Kind: game.ZoneGrow, Crop: "rice"}}
	g.sel.SetGroup(selection.Zone, []entity.EntityID{1})
	p := New(g)
	var sent []command.Command
	p.Producers[selection.Zone] = Producer{Commands: func(id entity.EntityID) []Button {
		return []Button{zoneCrop(nil, 1, "rice", &sent)}
	}}
	p.Commands().ApplyCommandClick("cmd.zone.crop")
	if len(g.menus) != 1 || len(g.menus[0]) != 2 || !strings.HasSuffix(g.menus[0][0].Label, "(current)") {
		t.Fatalf("a left click lists the crops: %+v", g.menus)
	}
}

func TestMergeCommandsMergesMenusAndDropsSingles(t *testing.T) {
	var ran []string
	opt := func(l string, dis bool) Option {
		return Option{Label: l, Disabled: dis, Reason: "no", Run: func() { ran = append(ran, l) }}
	}
	mk := func(os ...Option) Button { return Button{Key: "k", Options: func() []Option { return os }} }
	got := MergeCommands([][]Button{
		{mk(opt("a", false), opt("b", false)), {Key: "m", Single: true}},
		{mk(opt("b", false), opt("a", true)), {Key: "m", Single: true}},
	}, func(_ int, c Button) { c.Options()[0].Run() })
	if len(got) != 1 {
		t.Fatalf("Single is dropped: %+v", got)
	}
	opts := got[0].Options()
	if len(opts) != 2 || opts[0].Label != "a" || opts[0].Disabled || opts[1].Label != "b" {
		t.Fatalf("shared options: %+v", opts)
	}
	opts[0].Run() // disabled for member 1: runs for member 0 only
	opts[1].Run()
	if !reflect.DeepEqual(ran, []string{"a", "b", "b"}) {
		t.Fatalf("ran %v", ran)
	}
}
