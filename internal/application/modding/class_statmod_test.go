// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/stat/statmod"
)

// C4: the stat_mod decode table, capability checks included.
func TestStatModClass(t *testing.T) {
	t.Parallel()
	stats := pluginapi.Set(1 << pluginapi.Stats)
	health := stats | 1<<pluginapi.Health
	cases := []struct {
		name   string
		plugin string
		caps   pluginapi.Set
		body   map[string]any
		field  string // "" for a valid def
		msg    string
		want   statmod.Def
	}{
		{name: "add", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": 3.0},
			want: statmod.Def{Key: "m.x", Stat: stat.WorkSpeed, Add: 3}},
		{name: "mul and trait", plugin: "m", caps: stats, body: map[string]any{"stat": "move_speed", "mul": 1200.0, "trait": "core.kind"},
			want: statmod.Def{Key: "m.x", Stat: stat.MoveSpeed, Mul: 1200, When: statmod.Cond{Trait: "core.kind"}}},
		{name: "negative add", plugin: "m", caps: stats, body: map[string]any{"stat": "beauty", "add": -4.0},
			want: statmod.Def{Key: "m.x", Stat: stat.Beauty, Add: -4}},
		{name: "health conditions", plugin: "m", caps: health, body: map[string]any{"stat": "work_speed", "mul": 900.0, "injured": true, "condition": "m.flu"},
			want: statmod.Def{Key: "m.x", Stat: stat.WorkSpeed, Mul: 900, When: statmod.Cond{Condition: "m.flu", Injured: true}}},
		{name: "no stats capability", plugin: "m", caps: pluginapi.Legacy(), body: map[string]any{"stat": "work_speed", "add": 1.0}, msg: "capability"},
		{name: "unknown plugin has none", plugin: "ghost", caps: 0, body: map[string]any{"stat": "work_speed", "add": 1.0}, msg: "capability"},
		{name: "condition needs health", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": 1.0, "condition": "m.flu"}, field: "condition", msg: "health"},
		{name: "injured needs health", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": 1.0, "injured": true}, field: "injured", msg: "health"},
		{name: "threshold rejected", plugin: "m", caps: health, body: map[string]any{"stat": "work_speed", "add": 1.0, "below": 50.0}, field: "below", msg: "unknown field"},
		{name: "needs stat", plugin: "m", caps: stats, body: map[string]any{"add": 1.0}, field: "stat", msg: "required"},
		{name: "unknown stat", plugin: "m", caps: stats, body: map[string]any{"stat": "luck"}, field: "stat", msg: "unknown stat"},
		{name: "stat type", plugin: "m", caps: stats, body: map[string]any{"stat": 3.0}, field: "stat", msg: "want a string"},
		{name: "fractional add", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": 1.5}, field: "add", msg: "whole number"},
		{name: "string mul", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "mul": "12"}, field: "mul", msg: "want a number"},
		{name: "mul range", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "mul": -1.0}, field: "mul", msg: "out of range"},
		{name: "add at max", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": 100000.0},
			want: statmod.Def{Key: "m.x", Stat: stat.WorkSpeed, Add: 100000}},
		{name: "mul at max", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "mul": 10000.0},
			want: statmod.Def{Key: "m.x", Stat: stat.WorkSpeed, Mul: 10000}},
		{name: "add too big", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": 100001.0}, field: "add", msg: "out of range"},
		{name: "add too small", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": -100001.0}, field: "add", msg: "out of range"},
		{name: "add int32 overflow", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "add": 3e9}, field: "add", msg: "out of range"},
		{name: "mul too big", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "mul": 10001.0}, field: "mul", msg: "out of range"},
		{name: "mul int32 overflow", plugin: "m", caps: stats, body: map[string]any{"stat": "work_speed", "mul": 3e9}, field: "mul", msg: "out of range"},
		{name: "injured type", plugin: "m", caps: health, body: map[string]any{"stat": "work_speed", "injured": "yes"}, field: "injured", msg: "want a boolean"},
		{name: "core has none", plugin: Core, caps: stats, body: map[string]any{"stat": "work_speed"}, msg: "no stat_mod defs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := Def{Class: StatModClass, Key: tc.plugin + ".x", Plugin: tc.plugin, File: "defs/a.json", Body: tc.body}
			b, probs := DecodeStatMods([]Def{d}, capsByPlugin{tc.plugin: tc.caps})
			if tc.msg == "" {
				if len(probs) != 0 || len(b.Defs) != 1 || b.Defs[0] != tc.want {
					t.Fatalf("got %+v, %v; want %+v", b.Defs, probs, tc.want)
				}
				return
			}
			if len(b.Defs) != 0 || !Errors(probs) {
				t.Fatalf("def kept: %+v %v", b.Defs, probs)
			}
			found := false
			for _, p := range probs {
				if p.Field == tc.field && strings.Contains(p.Msg, tc.msg) && p.Def == d.Ref() && p.Plugin == tc.plugin {
					found = true
				}
			}
			if !found {
				t.Fatalf("want field %q msg %q, got %v", tc.field, tc.msg, probs)
			}
		})
	}
}

// Wrong class and order: defs keep load order and a bad one drops alone.
func TestStatModClassOrderAndWrongClass(t *testing.T) {
	t.Parallel()
	caps := capsByPlugin{"a": 1 << pluginapi.Stats, "b": 1 << pluginapi.Stats}
	mk := func(p, n string, body map[string]any) Def {
		return Def{Class: StatModClass, Key: p + "." + n, Plugin: p, Body: body}
	}
	b, probs := DecodeStatMods([]Def{
		mk("b", "one", map[string]any{"stat": "work_speed"}),
		mk("a", "bad", map[string]any{"stat": "nope"}),
		{Class: "trait", Key: "a.t", Plugin: "a"},
		mk("a", "two", map[string]any{"stat": "beauty"}),
	}, caps)
	if len(b.Defs) != 2 || b.Defs[0].Key != "b.one" || b.Defs[1].Key != "a.two" || len(probs) != 2 {
		t.Fatalf("book %+v, problems %v", b.Defs, probs)
	}
}

// Core has no stat_mod defs; the class is registered after the others.
func TestStatModClassIsRegistered(t *testing.T) {
	t.Parallel()
	cs := Classes()
	if cs[len(cs)-1] != StatModClass {
		t.Fatalf("classes %v", cs)
	}
	if got := statModClass(nil).core(); len(got) != 0 {
		t.Fatalf("core stat_mod defs: %v", got)
	}
	cat, probs := Build(nil, nil, "0.1.0")
	if len(probs) != 0 || len(cat.StatMods.Defs) != 0 {
		t.Fatalf("core catalog: %+v %v", cat.StatMods, probs)
	}
}
