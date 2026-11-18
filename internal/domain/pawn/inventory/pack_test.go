// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inventory

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestPackAddJoinsASameLoadEntry(t *testing.T) {
	t.Parallel()
	var p Pack
	a := p.Add(item.Stack{Kind: item.Wood, Qty: 5, X: 9, Y: 9, Carrier: 7, Forbidden: true, Unclaimed: true, HaulMarked: true})
	b := p.Add(item.Stack{Kind: item.Wood, Qty: 3})
	if a == 0 || a != b || len(p.Items) != 1 || p.Items[0].Stack.Qty != 8 {
		t.Fatalf("wood did not join: ids %d, %d, items %+v", a, b, p.Items)
	}
	s := p.Items[0].Stack
	if s.X != 0 || s.Y != 0 || s.Carrier != 0 || s.Forbidden || s.Unclaimed || s.HaulMarked {
		t.Fatalf("a packed stack keeps its place or marks: %+v", s)
	}
	if c := p.Add(item.Stack{Kind: item.Stone, Qty: 1}); c == a || len(p.Items) != 2 {
		t.Fatalf("another kind joined: id %d, items %+v", c, p.Items)
	}
	if p.Add(item.Stack{Kind: item.Wood}) != 0 || len(p.Items) != 2 {
		t.Fatal("adding nothing made an entry")
	}
}

func TestPackAddAveragesRot(t *testing.T) {
	t.Parallel()
	var p Pack
	p.Add(item.Stack{Kind: item.Meal, Qty: 1, Rot: 100})
	p.Add(item.Stack{Kind: item.Meal, Qty: 1, Rot: 300})
	if len(p.Items) != 1 || p.Items[0].Stack.Rot != 200 {
		t.Fatalf("rot not averaged: %+v", p.Items)
	}
}

func TestPackTakeKeepsIDs(t *testing.T) {
	t.Parallel()
	var p Pack
	a, b, c := p.Add(item.Stack{Kind: item.Wood, Qty: 5}), p.Add(item.Stack{Kind: item.Stone, Qty: 2}), p.Add(item.Stack{Kind: item.Steel, Qty: 1})

	if st, ok := p.Take(a, 2); !ok || st.Kind != item.Wood || st.Qty != 2 || p.Items[0].Stack.Qty != 3 {
		t.Fatalf("part of an entry: %+v %v, left %+v", st, ok, p.Items)
	}
	if st, ok := p.Take(b, 99); !ok || st.Qty != 2 {
		t.Fatalf("more than there is gives what there is: %+v %v", st, ok)
	}
	if len(p.Items) != 2 || p.Items[0].ID != a || p.Items[1].ID != c {
		t.Fatalf("the others moved or changed ID: %+v", p.Items)
	}
	for _, bad := range []struct {
		id uint32
		n  int
	}{{b, 1}, {a, 0}, {a, -1}, {0, 1}} {
		if _, ok := p.Take(bad.id, bad.n); ok {
			t.Fatalf("Take(%d, %d) gave something", bad.id, bad.n)
		}
	}
	if d := p.Add(item.Stack{Kind: item.Stone, Qty: 1}); d == a || d == b || d == c {
		t.Fatalf("an ID was used again: %d", d)
	}
}

func TestPackGramsAndFind(t *testing.T) {
	t.Parallel()
	var p Pack
	if p.Grams(item.Book{}) != 0 {
		t.Fatal("an empty pack weighs something")
	}
	a := p.Add(item.Stack{Kind: item.Wood, Qty: 3})
	p.Add(item.Stack{Kind: item.Steel, Qty: 2})
	if want := 3*item.Wood.MassGrams() + 2*item.Steel.MassGrams(); p.Grams(item.Book{}) != want {
		t.Fatalf("grams %d, want %d", p.Grams(item.Book{}), want)
	}
	if id, ok := p.Find(item.Wood); !ok || id != a {
		t.Fatalf("find wood: %d %v", id, ok)
	}
	if _, ok := p.Find(item.Stone); ok {
		t.Fatal("found what is not there")
	}
}

// Pack.Grams reads each kind's mass from the book, so a plugin kind weighs
// what its def says.
func TestPackGramsReadsTheBook(t *testing.T) {
	t.Parallel()
	var defs []item.Def
	for k := range item.CoreKinds {
		defs = append(defs, item.CoreDef(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "bell", Mass: 700, Stack: 25})
	b := item.NewBook(defs, append(item.Core().Keys(), "frontier.bell"))
	bell := item.Kind(item.CoreKinds)
	var p Pack
	p.Add(item.Stack{Kind: bell, Qty: 3})
	p.Add(item.Stack{Kind: item.Wood, Qty: 2})
	if got, want := p.Grams(b), 700*3+item.Wood.MassGrams()*2; got != want {
		t.Fatalf("Grams = %d, want %d", got, want)
	}
}
