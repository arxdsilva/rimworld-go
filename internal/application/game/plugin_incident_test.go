// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
)

// incidentBookWith is core's incidents plus one nature incident per key
// that runs the eclipse's behaviour under its own label; with onlyPlugins,
// core's nature events weigh nothing (the plugin's are the only draw).
func incidentBookWith(onlyPlugins bool, keys ...string) incident.Book {
	core := incident.Core()
	list, all := core.All(), core.Keys()
	if onlyPlugins {
		for i := range list {
			list[i].Weight = 0
		}
	}
	for _, k := range keys {
		list = append(list, incident.Def{Key: k, Name: "Dark noon", Category: incident.Nature, Weight: 5, Cooldown: 7, Like: incident.Eclipse})
		all = append(all, k)
	}
	return incident.NewBook(list, all)
}

// A plugin incident fires through the dev hook, runs its like's behaviour
// (an eclipse), keeps its own cooldown state and announces itself under its
// own label (PLG-026).
func TestPluginIncidentRunsItsLike(t *testing.T) {
	t.Parallel()
	book := incidentBookWith(false, "frontier.dark_noon")
	id, _ := book.IDOf("frontier.dark_noon")
	g := New(Config{Seed: 3, Width: 20, Height: 20, Content: Content{Incidents: book}})
	if got := g.PluginIncidents(); len(got) != 1 || got[0] != (PluginIncident{Key: "frontier.dark_noon", Label: "Dark noon", Like: "Eclipse"}) {
		t.Fatalf("PluginIncidents %+v", got)
	}
	if !g.FireIncident("frontier.dark_noon") {
		t.Fatal("the plugin incident did not fire")
	}
	if g.incidents.EclipseUntil == 0 {
		t.Fatal("it did not run the eclipse's behaviour")
	}
	if _, ok := g.incidents.Last[id]; !ok {
		t.Fatalf("its own cooldown not recorded: %v", g.incidents.Last)
	}
	if _, ok := g.incidents.Last[incident.Eclipse]; ok {
		t.Fatal("the like's cooldown was used")
	}
	if eventMessages(g, "Dark noon") == 0 {
		t.Fatal("no message under the plugin incident's own label")
	}
	if len(New(Config{Seed: 3, Width: 20, Height: 20}).PluginIncidents()) != 0 {
		t.Fatal("core lists no plugin incidents")
	}
}

// The storyteller draws a plugin incident by weight.
func TestStorytellerDrawsPluginIncident(t *testing.T) {
	t.Parallel()
	book := incidentBookWith(true, "frontier.dark_noon")
	id, _ := book.IDOf("frontier.dark_noon")
	g := New(Config{Seed: 5, Width: 20, Height: 20, Content: Content{Incidents: book}})
	now := uint64(30 * clock.TicksPerDay)
	if got, ok := g.pickNature(now); !ok || got != id {
		t.Fatalf("picked %d, %v; want the plugin incident %d", got, ok, id)
	}
	g.rollNature(now)
	if _, ok := g.incidents.Last[id]; !ok || g.incidents.EclipseUntil == 0 {
		t.Fatalf("rollNature did not fire it: %v", g.incidents.Last)
	}
}

// A save keeps a plugin incident's cooldown by key; a missing plugin's is
// dropped on load (PLG-026, PLG-044).
func TestSavedIncidentsFollowTheirKeys(t *testing.T) {
	t.Parallel()
	saved := incidentBookWith(false, "a.first", "b.second")
	g := New(Config{Seed: 3, Width: 20, Height: 20, Content: Content{Incidents: saved}})
	first, _ := saved.IDOf("a.first")
	second, _ := saved.IDOf("b.second")
	g.incidents.Fired(first, 10)
	g.incidents.Fired(second, 20)
	g.incidents.Fired(incident.HerdCrossing, 30)
	g.QueueIncident(second, 99999, 0)
	var buf bytes.Buffer
	if err := g.Save(&buf, "incidents"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		book incident.Book
		want map[string]uint64
	}{
		{"same", saved, map[string]uint64{"a.first": 10, "b.second": 20}},
		{"reordered", incidentBookWith(false, "b.second", "a.first"), map[string]uint64{"a.first": 10, "b.second": 20}},
		{"first gone", incidentBookWith(false, "b.second"), map[string]uint64{"b.second": 20}},
		{"core only", incident.Core(), map[string]uint64{}},
	} {
		l, _, err := LoadContent(bytes.NewReader(buf.Bytes()), Content{Incidents: tc.book})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := map[string]uint64{}
		for id, at := range l.incidents.Last {
			if id != incident.HerdCrossing {
				got[tc.book.Key(id)] = at
			}
		}
		if len(got) != len(tc.want) || l.incidents.Last[incident.HerdCrossing] != 30 {
			t.Errorf("%s: Last %v", tc.name, l.incidents.Last)
		}
		for k, at := range tc.want {
			if got[k] != at {
				t.Errorf("%s: %s cooldown %d, want %d (%v)", tc.name, k, got[k], at, got)
			}
		}
		wantQueue := tc.want["b.second"] != 0
		if has := len(l.incidents.Queue) == 1 && tc.book.Key(l.incidents.Queue[0].ID) == "b.second"; has != wantQueue || !wantQueue && len(l.incidents.Queue) != 0 {
			t.Errorf("%s: queue %+v", tc.name, l.incidents.Queue)
		}
	}
}
