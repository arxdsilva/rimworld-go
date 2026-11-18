// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

import (
	"reflect"
	"slices"
	"testing"
)

// Every Kind method gives a plugin kind (and None) a neutral answer, never
// a panic (PLG-026): a plugin kind is past every per-kind table.
func TestKindMethodsNeutralPastCore(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[Kind]()
	for i := range typ.NumMethod() {
		m := typ.Method(i)
		for k := CoreKinds; k <= int(None); k++ {
			args := []reflect.Value{reflect.ValueOf(Kind(k))}
			for a := 1; a < m.Type.NumIn(); a++ {
				args = append(args, reflect.Zero(m.Type.In(a)))
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Kind(%d).%s panics: %v", k, m.Name, r)
					}
				}()
				m.Func.Call(args)
			}()
		}
		k := Kind(CoreKinds)
		if k.IsFood() || k.IsWeapon() || k.IsApparel() || k.IsStuff() || k.IsMedicine() || k.IsCorpse() || k.Perishable() ||
			k.String() != "unknown" || k.StackLimit() != StackLimit {
			t.Fatalf("a plugin kind is a plain good by its Kind methods: %+v", CoreDef(k))
		}
	}
}

// The zero Book is core; a plugin kind answers from its def; core keys
// and names round-trip.
func TestBookCoreAndPlugin(t *testing.T) {
	t.Parallel()
	var zero Book
	core := Core()
	if zero.Len() != CoreKinds || !slices.Equal(zero.Keys(), core.Keys()) || zero.Value(Steel) != Steel.Value() ||
		zero.Def(Steel) != core.Def(Steel) || zero.Plugin(Kind(CoreKinds)) {
		t.Fatal("the zero book is not core")
	}
	if k, ok := core.KindOf("core.steel"); !ok || k != Steel || core.Key(Steel) != "core.steel" {
		t.Fatalf("core.steel = %v %v", k, ok)
	}
	defs, keys := append([]Def(nil), core.defs...), core.Keys()
	defs[Steel].Value = 3
	defs = append(defs, Def{Label: "copper bell", Path: "Manufactured/Components", Mass: 700, Value: 40, Stack: 25,
		Texture: "textures/bell.png"})
	keys = append(keys, "frontier.bell")
	b := NewBook(defs, keys)
	bell := Kind(CoreKinds)
	if !b.Plugin(bell) || b.Name(bell) != "frontier.bell" || b.Label(bell) != "copper bell" || b.Value(bell) != 40 ||
		b.MassGrams(bell) != 700 || b.StackLimit(bell) != 25 || b.Category(bell) != "Manufactured" || b.Value(Steel) != 3 {
		t.Fatalf("plugin kind reads %+v", b.Def(bell))
	}
	if k, ok := b.Parse("frontier.bell"); !ok || k != bell {
		t.Fatalf("Parse(frontier.bell) = %v %v", k, ok)
	}
	if k, ok := b.Parse("steel"); !ok || k != Steel {
		t.Fatal("Parse(steel)")
	}
	if _, ok := core.Parse("frontier.bell"); ok || b.Plugins()[0] != bell || len(core.Plugins()) != 0 {
		t.Fatal("core parses a plugin key, or Plugins is wrong")
	}
	if b.Name(None) != "unknown" || b.Label(None) != "unknown" || b.Def(None) != (Def{}) {
		t.Fatal("None is not neutral")
	}
}

// The zero book, Core() and the Kind methods agree on mass, stack limit and
// value for every core kind and for None (W0: the read paths move to the book
// without moving a core number).
func TestBookMatchesKindMethods(t *testing.T) {
	t.Parallel()
	books := map[string]Book{"zero": {}, "core": Core()}
	kinds := []Kind{None}
	for k := range CoreKinds {
		kinds = append(kinds, Kind(k))
	}
	for name, b := range books {
		for _, k := range kinds {
			if b.MassGrams(k) != k.MassGrams() || b.StackLimit(k) != k.StackLimit() || b.Value(k) != k.Value() {
				t.Fatalf("%s book, kind %d: mass %d/%d stack %d/%d value %d/%d", name, k, b.MassGrams(k), k.MassGrams(),
					b.StackLimit(k), k.StackLimit(), b.Value(k), k.Value())
			}
		}
	}
	if CoreDef(Steel).TraderStock != 0 {
		t.Fatal("a core def never carries trader stock")
	}
}
