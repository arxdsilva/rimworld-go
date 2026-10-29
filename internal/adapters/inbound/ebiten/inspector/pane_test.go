package inspector

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestNewFillsItsOwnKinds(t *testing.T) {
	p := New(newFake())
	for _, k := range []selection.Kind{selection.Pawn, selection.Animal, selection.Item, selection.Plant, selection.Building, selection.Frame} {
		if pr := p.Producers[k]; pr.Pane == nil || pr.Commands == nil {
			t.Errorf("kind %v has no producer", k)
		}
	}
	if _, ok := p.Producers[selection.Zone]; ok {
		t.Error("the zone is the root's to plug in")
	}
	f := newFake()
	f.env.Sim = nil
	if New(f).Commands().SelectionCommands() != nil || New(f).SelectionPane().Title != "" {
		t.Error("off the colony the inspector shows nothing")
	}
}

func TestPluggedProducerAndCommandClick(t *testing.T) {
	f := newFake()
	f.pawn = 7
	p := New(f)
	ran, opt := 0, ""
	p.Producers[selection.Pawn] = Producer{Commands: func(id entity.EntityID) []Button {
		return []Button{
			{ID: "cmd.t.run", Run: func() { ran++ }},
			{ID: "cmd.t.cmds", Cmds: []command.Command{{Kind: command.ClearOrders, Pawn: uint64(id)}}},
			{ID: "cmd.t.opts", Options: func() []Option {
				return []Option{{Label: "a", Disabled: true}, {Label: "b", Run: func() { opt = "b" }}}
			}},
			{ID: "cmd.t.off", Disabled: true, Reason: "not now", Run: func() { ran += 100 }},
			{ID: "pawn.legacy"},
		}
	}}
	c := p.Commands()
	for _, id := range []string{"cmd.t.run", "cmd.t.cmds", "cmd.t.opts", "cmd.t.off"} {
		c.ApplyCommandClick(id)
	}
	if ran != 1 || opt != "b" || len(f.cmds) != 1 || f.cmds[0].Pawn != 7 || f.hint != "not now" {
		t.Fatalf("ran %d, opt %q, cmds %+v, hint %q", ran, opt, f.cmds, f.hint)
	}
	c.PressCommandKey(4) // the fifth command key: a legacy id goes through the click routes
	if len(f.clicks) != 1 || f.clicks[0] != "pawn.legacy" {
		t.Fatalf("clicks %v", f.clicks)
	}
}

func TestTabsOpenCollapseAndRefuse(t *testing.T) {
	f := newFake()
	f.pawn = 7
	p := New(f)
	p.TestPane = func(selection.Kind, entity.EntityID) Spec {
		return Spec{Title: "Ada", Tabs: []Tab{{ID: "inspector.needs", Label: "Needs"}, {ID: "inspector.bio", Label: "Bio", Disabled: true, Reason: "no bio"}}}
	}
	if !p.OpenPaneTab("needs") || p.Tab != "needs" || p.PaneOpenTab() != "inspector.needs" {
		t.Fatalf("needs not open: %q %q", p.Tab, p.PaneOpenTab())
	}
	if p.OpenPaneTab("bio") || f.hint != "no bio" {
		t.Fatalf("a disabled tab opened (hint %q)", f.hint)
	}
	p.SelectPaneTab("inspector.needs") // the open tab collapses
	if p.PaneOpenTab() != "" || p.Tab != "character" || p.ClosePaneTab() {
		t.Fatalf("not collapsed: %q %q", p.PaneOpenTab(), p.Tab)
	}
}
