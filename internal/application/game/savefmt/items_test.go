// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
)

// imBook is core plus plugin kinds keyed keys.
func imBook(keys ...string) item.Book {
	core := item.Core()
	defs := make([]item.Def, 0, item.CoreKinds+len(keys))
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	for range keys {
		defs = append(defs, item.Def{Label: "x", Path: "Items/Parts"})
	}
	return item.NewBook(defs, append(core.Keys(), keys...))
}

// imState stands in for a save's state: every shape Walk handles.
type imState struct {
	Stacks  []struct{ V item.Stack }
	Carried map[uint64]item.Stack
	Last    map[uint64]item.Kind
	ByKind  map[item.Kind]int
	Forbid  []item.Kind
	Worn    [2]item.Kind
	Filter  zone.Filter
	Ptr     *item.Kind
	Bytes   []uint8 // never a kind
	hidden  item.Kind
}

// Walk moves plugin kinds by key and removes a gone plugin's stacks,
// entries and filter bits; core kinds, None and plain bytes stay.
func TestItemKindsWalk(t *testing.T) {
	t.Parallel()
	saved := imBook("a.cup", "b.bell")
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	keys := Keys{item.Class: saved.Keys()}
	if !keys.Items(saved).Same() || !(Keys{}).Items(imBook("b.bell")).Same() || !keys.Items(imBook("a.cup", "b.bell", "c.pot")).Same() {
		t.Fatal("the same keys, none, or more appended move kinds")
	}
	m := keys.Items(imBook("b.bell"))
	ptr := bell
	s := imState{
		Stacks:  []struct{ V item.Stack }{{item.Stack{Kind: cup, Qty: 1}}, {item.Stack{Kind: bell, Qty: 2}}, {item.Stack{Kind: item.Wood, Qty: 3}}},
		Carried: map[uint64]item.Stack{1: {Kind: cup}, 2: {Kind: bell}},
		Last:    map[uint64]item.Kind{1: cup, 2: item.Meal},
		ByKind:  map[item.Kind]int{cup: 1, bell: 2, item.Steel: 3},
		Forbid:  []item.Kind{item.Wood, cup, bell},
		Worn:    [2]item.Kind{cup, item.None},
		Filter:  zone.Filter{}.With(item.Wood, true).With(cup, true).With(bell, true),
		Ptr:     &ptr,
		Bytes:   []uint8{uint8(cup)},
		hidden:  cup,
	}
	m.Walk(&s)
	nb := item.Kind(item.CoreKinds) // the bell's kind now
	if len(s.Stacks) != 2 || s.Stacks[0].V.Kind != nb || s.Stacks[1].V.Kind != item.Wood {
		t.Fatalf("stacks %+v", s.Stacks)
	}
	if len(s.Carried) != 1 || s.Carried[2].Kind != nb || len(s.Last) != 1 || s.Last[2] != item.Meal {
		t.Fatalf("maps %+v %+v", s.Carried, s.Last)
	}
	if len(s.ByKind) != 2 || s.ByKind[nb] != 2 || s.ByKind[item.Steel] != 3 {
		t.Fatalf("keys %+v", s.ByKind)
	}
	if !slices.Equal(s.Forbid, []item.Kind{item.Wood, nb}) || s.Worn != [2]item.Kind{item.None, item.None} || *s.Ptr != nb {
		t.Fatalf("forbid %v worn %v ptr %v", s.Forbid, s.Worn, *s.Ptr)
	}
	if want := (zone.Filter{}).With(item.Wood, true).With(nb, true); s.Filter != want {
		t.Fatalf("filter %v, want %v", s.Filter, want)
	}
	if s.Bytes[0] != uint8(cup) || s.hidden != cup {
		t.Fatal("a plain byte or an unexported field was touched")
	}
	cmds := m.Commands([]command.Command{{Kind: command.SetBillIngredient, Stuff: uint8(cup)},
		{Kind: command.SetBillIngredient, Stuff: uint8(bell)}, {Kind: command.DesignateBuild, Stuff: uint8(cup)}})
	if len(cmds) != 2 || cmds[0].Stuff != uint8(nb) || cmds[1].Stuff != uint8(cup) {
		t.Fatalf("commands %+v", cmds)
	}
}

func TestItemKindsTradeCommandsAndRecursiveState(t *testing.T) {
	old := imBook("a.cup", "b.bell")
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	m := Keys{item.Class: old.Keys()}.Items(imBook("b.bell", "a.cup"))
	type order struct{ Cmd command.Command }
	type state struct {
		Extra   []state
		Pending []command.Command
		Orders  map[uint64][]order
	}
	local := command.Command{Kind: command.ProposeTrade, Name: trade.EncodeOffer([]trade.Delta{{Kind: cup, Qty: 1, StackID: 17}}), ExactLots: true}
	world := command.Command{Kind: command.WorldCaravanTrade, Name: trade.EncodeOffer([]trade.Delta{{Kind: cup, Qty: -1, StackID: 21}}), Quote: 9}
	ordinary := command.Command{Kind: command.WorldCaravanTrade, Name: trade.EncodeOffer([]trade.Delta{{Kind: cup, Qty: -1}, {Kind: bell, Qty: 1}}), Quote: 12}
	s := state{Extra: []state{{Pending: []command.Command{ordinary}}}, Pending: []command.Command{local, world, ordinary}, Orders: map[uint64][]order{1: {{Cmd: local}, {Cmd: world}}}}
	m.Walk(&s)
	if len(s.Pending) != 2 || len(s.Orders[1]) != 1 || len(s.Extra[0].Pending) != 1 {
		t.Fatalf("wrong saved intent removal: %+v", s)
	}
	wantLocal := trade.EncodeOffer([]trade.Delta{{Kind: bell, Qty: 1, StackID: 17}})
	wantOrdinary := trade.EncodeOffer([]trade.Delta{{Kind: bell, Qty: -1}, {Kind: cup, Qty: 1}})
	if s.Pending[0].Name != wantLocal || s.Orders[1][0].Cmd.Name != wantLocal || s.Pending[1].Name != wantOrdinary || s.Extra[0].Pending[0].Name != wantOrdinary || s.Pending[1].Quote != 12 {
		t.Fatalf("wrong restored intent: %+v", s)
	}
}

// The typed inventory owners need canonical Goods order after a remap.
// A reward sequence that happened to have that order must retain its order.
func TestItemKindsSortOnlyTradeLedgers(t *testing.T) {
	old := imBook("a.cup", "b.bell")
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	m := Keys{item.Class: old.Keys()}.Items(imBook("b.bell", "a.cup"))
	lines := func() []trade.Line {
		return []trade.Line{{Kind: cup, Qty: 2}, {Kind: bell, Qty: 3}}
	}
	s := struct {
		Stock   trade.Stock
		Carrier visit.Cargo
		Rewards []trade.Line
	}{Stock: trade.Stock{Lines: lines()}, Carrier: visit.Cargo{Lines: lines()}, Rewards: lines()}
	m.Walk(&s)
	for _, ls := range [][]trade.Line{s.Stock.Lines, s.Carrier.Lines} {
		if len(ls) != 2 || ls[0].Kind != cup || ls[0].Qty != 3 || ls[1].Kind != bell || ls[1].Qty != 2 {
			t.Fatalf("inventory order/quantity changed incorrectly: %+v", ls)
		}
	}
	if len(s.Rewards) != 2 || s.Rewards[0].Kind != bell || s.Rewards[0].Qty != 2 || s.Rewards[1].Kind != cup || s.Rewards[1].Qty != 3 {
		t.Fatalf("reward sequence was reordered: %+v", s.Rewards)
	}
}

func TestHistoricalFilterNamespace(t *testing.T) {
	old := append(slices.Clone(item.Core().Keys()[:165]), "a.cup", "b.bell", "gone")
	book := imBook("b.bell", "a.cup")
	f := (zone.Filter{}).With(item.Wood, true).With(165, true).With(166, true).With(167, true).With(item.None, true)
	s := struct{ Filter zone.Filter }{f}
	(Keys{}).Items(book).Walk(&s)
	if s.Filter != f {
		t.Fatal("no keys must preserve numeric identity")
	}
	MigrateConstructionStacks(&s, 20)
	if s.Filter != f {
		t.Fatal("construction migration changed filter")
	}
	(Keys{item.Class: old}).Items(book).Walk(&s)
	want := (zone.Filter{}).With(item.Wood, true).With(item.Kind(item.CoreKinds), true).With(item.Kind(item.CoreKinds+1), true).With(item.None, true)
	if s.Filter != want {
		t.Fatalf("got %v want %v", s.Filter, want)
	}
}
