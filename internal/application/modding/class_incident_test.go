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
		{"season bad", "season", "unknown season", func(b map[string]any) { b["season"] = "spring" }},
		{"season type", "season", "want a string", func(b map[string]any) { b["season"] = 1.0 }},
		{"target bad", "target", "unknown target", func(b map[string]any) { b["target"] = "moon" }},
		{"target type", "target", "want a string", func(b map[string]any) { b["target"] = true }},
		{"min points negative", "min_points", "out of range", func(b map[string]any) { b["min_points"] = -1.0 }},
		{"min points type", "min_points", "want a number", func(b map[string]any) { b["min_points"] = "9" }},
		{"points pct negative", "points_pct", "out of range", func(b map[string]any) { b["points_pct"] = -5.0 }},
		{"min points too high", "min_points", "out of range", func(b map[string]any) { b["min_points"] = float64(icMaxPoints + 1) }},
		{"points pct too high", "points_pct", "out of range", func(b map[string]any) { b["points_pct"] = float64(icMaxPct + 1) }},
		{"fill", "fill", "unknown field", func(b map[string]any) { b["fill"] = 8.0 }},
		{"under raid", "under_raid", "unknown field", func(b map[string]any) { b["under_raid"] = true }},
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

// TestPluginIncidentBudgetFields: a plugin def may set the point minimum,
// the spend, the season and the target and stays a nature incident; one
// without them decodes zeroes.
func TestPluginIncidentBudgetFields(t *testing.T) {
	t.Parallel()
	full := icPlugin()
	full["min_points"], full["points_pct"], full["season"], full["target"] = 250.0, 40.0, "winter", "world"
	defs := append(EncodeIncidents(incident.Core()),
		Def{Class: IncidentClass, Key: "moremods.full", Plugin: "moremods", Body: full},
		Def{Class: IncidentClass, Key: "moremods.plain", Plugin: "moremods", Body: icPlugin()})
	got, probs := DecodeIncidents(bsJSON(t, defs))
	if len(probs) > 0 {
		t.Fatal(probs)
	}
	fid, _ := got.IDOf("moremods.full")
	pid, _ := got.IDOf("moremods.plain")
	f, p := got.DefOf(fid), got.DefOf(pid)
	if f.MinPoints != 250 || f.PointsPct != 40 || f.Season != incident.WinterOnly || f.Target != incident.WorldObject || f.Category != incident.Nature {
		t.Fatalf("full def %+v", f)
	}
	if p.MinPoints != 0 || p.PointsPct != 0 || p.Season != incident.AnySeason || p.Target != incident.HomeMap {
		t.Fatalf("plain def %+v", p)
	}
	full["season"] = "not_winter"
	if got, _ := DecodeIncidents(bsJSON(t, defs)); got.DefOf(fid).Season != incident.NotWinter {
		t.Fatal("not_winter")
	}
	full["season"], full["target"] = "any", "map"
	if got, _ := DecodeIncidents(bsJSON(t, defs)); got.DefOf(fid).Season != incident.AnySeason || got.DefOf(fid).Target != incident.HomeMap {
		t.Fatal("any/map")
	}
}

// TestIncidentLikeRefusesThreatFaces: a plugin incident cannot run a threat
// face's behaviour; the nature likes still work.
func TestIncidentLikeRefusesThreatFaces(t *testing.T) {
	t.Parallel()
	try := func(like string) []Problem {
		body := icPlugin()
		body["like"] = like
		_, probs := DecodeIncidents(bsJSON(t, append(EncodeIncidents(incident.Core()),
			Def{Class: IncidentClass, Key: "moremods.x", Plugin: "moremods", Body: body})))
		return probs
	}
	for _, k := range []string{"core.raid", "core.predator_pack", "core.manhunter_pack", "core.hive_swarm", "core.mech_raid"} {
		p := try(k)
		if len(p) != 1 || p[0].Field != "like" || !strings.Contains(p[0].Msg, "storyteller threat face") {
			t.Errorf("%s: problems %+v", k, p)
		}
	}
	for _, k := range []string{"core.mad_animal", "core.crashed_ship_part"} {
		if p := try(k); len(p) != 0 {
			t.Errorf("%s: %+v", k, p)
		}
	}
}

// TestIncidentCorePatchBudgetFields: a core patch may set the four fields.
func TestIncidentCorePatchBudgetFields(t *testing.T) {
	t.Parallel()
	defs := EncodeIncidents(incident.Core())
	for _, d := range defs {
		if d.Key == "core.flu" {
			d.Body["min_points"], d.Body["points_pct"], d.Body["season"], d.Body["target"] = 10.0, 20.0, "not_winter", "world"
		}
	}
	b, probs := DecodeIncidents(bsJSON(t, defs))
	flu, _ := b.IDOf("core.flu")
	d := b.DefOf(flu)
	if len(probs) > 0 || d.MinPoints != 10 || d.PointsPct != 20 || d.Season != incident.NotWinter || d.Target != incident.WorldObject {
		t.Fatalf("%v %+v", probs, d)
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
