// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/world/biome"
)

func icSame(t *testing.T, got, want incident.Book) {
	t.Helper()
	if got.Len() != want.Len() {
		t.Fatalf("%d incidents, want %d", got.Len(), want.Len())
	}
	for i := range want.Len() {
		id := incident.ID(i)
		if g, w := got.DefOf(id), want.DefOf(id); !reflect.DeepEqual(g, w) {
			t.Fatalf("id %d: %+v\nwant %+v", id, g, w)
		}
		if got.Key(id) != want.Key(id) {
			t.Fatalf("id %d: key %q, want %q", id, got.Key(id), want.Key(id))
		}
	}
}

// TestIncidentCodecRoundTripsCore: decoding core's encoded defs (straight
// and through JSON) gives core back.
func TestIncidentCodecRoundTripsCore(t *testing.T) {
	t.Parallel()
	core := incident.Core()
	defs := EncodeIncidents(core)
	if len(defs) != core.Len() {
		t.Fatalf("%d defs for %d incidents", len(defs), core.Len())
	}
	for name, in := range map[string][]Def{"trees": defs, "json": bsJSON(t, defs)} {
		got, probs := DecodeIncidents(in)
		if len(probs) > 0 {
			t.Fatalf("%s: %v", name, probs)
		}
		icSame(t, got, core)
	}
}

func icPlugin() map[string]any {
	return map[string]any{"like": "core.flu", "label": "Marsh fever", "weight": 5.0, "cooldown": 30.0, "min_day": 20.0,
		"disease": true, "biomes": []any{"tropical_swamp"}}
}

// TestIncidentPluginDefFollowsCore: a plugin incident takes the next ID
// with its own numbers and runs its like's behaviour.
func TestIncidentPluginDefFollowsCore(t *testing.T) {
	t.Parallel()
	defs := append(EncodeIncidents(incident.Core()), Def{Class: IncidentClass, Key: "moremods.marsh_fever", Plugin: "moremods", Body: icPlugin()})
	got, probs := DecodeIncidents(bsJSON(t, defs))
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	n := incident.Core().Len()
	id := incident.ID(n)
	d := got.DefOf(id)
	flu, _ := incident.Core().IDOf("core.flu")
	if got.Len() != n+1 || got.Key(id) != "moremods.marsh_fever" || d.ID != id || d.Like != flu || d.Name != "Marsh fever" ||
		d.Weight != 5 || d.Cooldown != 30 || d.MinDay != 20 || !d.Disease || d.Category != incident.Nature || !d.Biomes.Has(biome.TropicalSwamp) {
		t.Fatalf("plugin def %+v", d)
	}
	if got.Like(id) != flu || got.Like(flu) != flu {
		t.Fatal("Like")
	}
	icSame(t, incident.NewBook(got.All()[:n], got.Keys()[:n]), incident.Core())
}

// TestIncidentDecodeProblems: each rule names its field and drops the def.
func TestIncidentDecodeProblems(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, field, msg string
		edit             func(map[string]any)
	}{
		{"unknown field", "colour", "unknown field", func(b map[string]any) { b["colour"] = "red" }},
		{"like missing", "like", "required", func(b map[string]any) { delete(b, "like") }},
		{"like unknown", "like", "unknown core incident", func(b map[string]any) { b["like"] = "core.nope" }},
		{"like plugin", "like", "unknown core incident", func(b map[string]any) { b["like"] = "moremods.other" }},
		{"like rival raid", "like", "queued by the story", func(b map[string]any) { b["like"] = "core.rival_raid" }},
		{"like type", "like", "want a string", func(b map[string]any) { b["like"] = 3.0 }},
		{"label missing", "label", "required", func(b map[string]any) { delete(b, "label") }},
		{"label long", "label", "want 1 to 60", func(b map[string]any) { b["label"] = strings.Repeat("a", 61) }},
		{"weight missing", "weight", "required", func(b map[string]any) { delete(b, "weight") }},
		{"weight fraction", "weight", "not a whole number", func(b map[string]any) { b["weight"] = 1.5 }},
		{"weight big", "weight", "out of range", func(b map[string]any) { b["weight"] = 99999.0 }},
		{"threat", "category", "nature incident", func(b map[string]any) { b["category"] = "threat" }},
		{"category unknown", "category", "unknown category", func(b map[string]any) { b["category"] = "joy" }},
		{"biome unknown", "biomes.0", "unknown key", func(b map[string]any) { b["biomes"] = []any{"moon"} }},
		{"helpful type", "helpful", "want a boolean", func(b map[string]any) { b["helpful"] = "yes" }},
	}
	for _, c := range cases {
		body := icPlugin()
		c.edit(body)
		good := Def{Class: IncidentClass, Key: "moremods.good", Plugin: "moremods", Body: icPlugin()}
		bad := Def{Class: IncidentClass, Key: "moremods.bad", Plugin: "moremods", File: "defs/i.json", Body: body}
		got, probs := DecodeIncidents(bsJSON(t, append(EncodeIncidents(incident.Core()), bad, good)))
		if len(probs) != 1 || probs[0].Field != c.field || !strings.Contains(probs[0].Msg, c.msg) || probs[0].Def != "incident/moremods.bad" {
			t.Errorf("%s: problems %+v", c.name, probs)
		}
		if _, ok := got.IDOf("moremods.bad"); ok {
			t.Errorf("%s: bad def kept", c.name)
		}
		if _, ok := got.IDOf("moremods.good"); !ok {
			t.Errorf("%s: good def dropped", c.name)
		}
	}
}

// TestIncidentCorePatchRules: a patch changes core numbers but not the
// category, disease flag or behaviour, and a plugin key needs its prefix.
func TestIncidentCorePatchRules(t *testing.T) {
	t.Parallel()
	patched := func(k string, v any) ([]Problem, incident.Book) {
		defs := EncodeIncidents(incident.Core())
		for _, d := range defs {
			if d.Key == "core.flu" {
				d.Body[k] = v
			}
		}
		b, p := DecodeIncidents(bsJSON(t, defs))
		return p, b
	}
	p, b := patched("weight", 9.0)
	flu, _ := b.IDOf("core.flu")
	if len(p) > 0 || b.DefOf(flu).Weight != 9 || b.DefOf(flu).Like != flu {
		t.Fatalf("weight patch: %v %+v", p, b.DefOf(flu))
	}
	for k, v := range map[string]any{"category": "threat", "disease": false, "like": "core.party"} {
		if p, _ := patched(k, v); len(p) != 1 || p[0].Field != k {
			t.Errorf("%s: problems %v", k, p)
		}
	}
	_, probs := DecodeIncidents(append(EncodeIncidents(incident.Core()), Def{Class: IncidentClass, Key: "nodot", Plugin: "moremods", Body: icPlugin()}))
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "<plugin>.<name>") {
		t.Fatalf("key: %v", probs)
	}
}
