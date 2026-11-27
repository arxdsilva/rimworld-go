// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package statmod

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/pawn/stat"
)

func book() Book {
	return Book{Defs: []Def{
		{Key: "a:all", Stat: stat.WorkSpeed, Mul: 1100},
		{Key: "a:trait", Stat: stat.MoveSpeed, Add: 5, When: Cond{Trait: "tough"}},
		{Key: "a:cond", Stat: stat.RestRate, Mul: 900, When: Cond{Condition: "asthma"}},
		{Key: "a:inj", Stat: stat.PainTolerance, Add: -3, When: Cond{Injured: true}},
		{Key: "a:both", Stat: stat.Beauty, Add: 1, When: Cond{Trait: "tough", Injured: true}},
	}}
}

func TestLayerMatches(t *testing.T) {
	m := func(s stat.ID, add, mul int32) stat.Mod { return stat.Mod{Stat: s, Add: add, Mul: mul} }
	all := m(stat.WorkSpeed, 0, 1100)
	cases := []struct {
		name string
		f    Facts
		want []stat.Mod
	}{
		{"none", Facts{}, []stat.Mod{all}},
		{"trait", Facts{Traits: []string{"x", "tough"}}, []stat.Mod{all, m(stat.MoveSpeed, 5, 0)}},
		{"other trait", Facts{Traits: []string{"x"}}, []stat.Mod{all}},
		{"condition", Facts{Conditions: []string{"asthma"}}, []stat.Mod{all, m(stat.RestRate, 0, 900)}},
		{"injured", Facts{Injured: true}, []stat.Mod{all, m(stat.PainTolerance, -3, 0)}},
		{"trait and injured", Facts{Traits: []string{"tough"}, Injured: true},
			[]stat.Mod{all, m(stat.MoveSpeed, 5, 0), m(stat.PainTolerance, -3, 0), m(stat.Beauty, 1, 0)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := book().Layer(c.f); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestLayerNoMatchNil(t *testing.T) {
	b := Book{Defs: []Def{{Stat: stat.Beauty, When: Cond{Injured: true}}}}
	if got := b.Layer(Facts{}); got != nil {
		t.Fatalf("got %v", got)
	}
	if n := testing.AllocsPerRun(50, func() { _ = b.Layer(Facts{}) }); n != 0 {
		t.Fatalf("allocs %v", n)
	}
	if (Book{}).Layer(Facts{Injured: true}) != nil {
		t.Fatal("empty book")
	}
}

func TestLayerOrderStable(t *testing.T) {
	f := Facts{Traits: []string{"tough"}, Conditions: []string{"asthma"}, Injured: true}
	first := book().Layer(f)
	want := []stat.ID{stat.WorkSpeed, stat.MoveSpeed, stat.RestRate, stat.PainTolerance, stat.Beauty}
	for i := 0; i < 20; i++ {
		got := book().Layer(f)
		if !reflect.DeepEqual(got, first) {
			t.Fatal("not repeatable")
		}
		for j, s := range want {
			if got[j].Stat != s {
				t.Fatalf("pos %d: %v", j, got[j].Stat)
			}
		}
	}
}

func TestLayerNoFlap(t *testing.T) {
	b := book()
	const n = 10
	f := Facts{}
	prev := b.Layer(f)
	changes := 0
	for i := 0; i < n; i++ {
		f.Injured = !f.Injured
		cur := b.Layer(f)
		if !reflect.DeepEqual(cur, prev) {
			changes++
		}
		for k := 0; k < 3; k++ {
			if !reflect.DeepEqual(b.Layer(f), cur) {
				t.Fatal("same facts differ")
			}
		}
		prev = cur
	}
	if changes != n {
		t.Fatalf("changes %d want %d", changes, n)
	}
}

func TestStatByKey(t *testing.T) {
	for id := stat.ID(0); id < stat.NumStats; id++ {
		got, ok := StatByKey(id.Key())
		if !ok || got != id {
			t.Fatalf("%s: %v %v", id.Key(), got, ok)
		}
	}
	if _, ok := StatByKey("nope"); ok {
		t.Fatal("unknown")
	}
	if _, ok := StatByKey(""); ok {
		t.Fatal("empty")
	}
	if id, ok := StatByKey("work_speed"); !ok || id != stat.WorkSpeed {
		t.Fatal("work_speed")
	}
	if id, ok := StatByKey("beauty"); !ok || id != stat.Beauty {
		t.Fatal("beauty")
	}
}
