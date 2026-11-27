// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/incident"
)

func incidentBook(keys ...string) incident.Book {
	core := incident.Core()
	list, ks := core.All(), core.Keys()
	for _, k := range keys {
		d := core.DefOf(incident.Flu)
		d.Key = k
		list, ks = append(list, d), append(ks, k)
	}
	return incident.NewBook(list, ks)
}

// Saved incident state follows keys: a moved plugin incident's cooldown and
// queue entry move with it, a gone one's are dropped, core's stay.
func TestRemapIncidentsFollowsKeys(t *testing.T) {
	t.Parallel()
	n := incident.ID(incident.Core().Len())
	keys := Keys{incident.Class: incidentBook("p.a", "p.b").Keys()}
	state := func() *incident.State {
		st := &incident.State{Last: map[incident.ID]uint64{incident.HerdCrossing: 5, n: 7, n + 1: 9}}
		st.Enqueue(incident.Queued{ID: n, At: 10})
		st.Enqueue(incident.Queued{ID: n + 1, At: 11, Points: 3})
		st.Enqueue(incident.Queued{ID: incident.RivalRaid, At: 12})
		return st
	}
	st := state()
	keys.RemapIncidents(incidentBook("p.b", "p.a"), st)
	if want := map[incident.ID]uint64{incident.HerdCrossing: 5, n + 1: 7, n: 9}; !reflect.DeepEqual(st.Last, want) {
		t.Fatalf("swapped Last %v, want %v", st.Last, want)
	}
	if q := st.Queue; len(q) != 3 || q[0].ID != n+1 || q[1].ID != n || q[1].Points != 3 || q[2].ID != incident.RivalRaid {
		t.Fatalf("swapped Queue %+v", q)
	}
	st = state()
	keys.RemapIncidents(incident.Core(), st) // the plugin is gone
	if want := map[incident.ID]uint64{incident.HerdCrossing: 5}; !reflect.DeepEqual(st.Last, want) {
		t.Fatalf("gone Last %v", st.Last)
	}
	if len(st.Queue) != 1 || st.Queue[0].ID != incident.RivalRaid {
		t.Fatalf("gone Queue %+v", st.Queue)
	}
	st = state() // a save without keys keeps its IDs
	Keys{}.RemapIncidents(incident.Core(), st)
	if len(st.Last) != 3 || len(st.Queue) != 3 {
		t.Fatalf("keyless save changed: %v %v", st.Last, st.Queue)
	}
}

// A plugin incident saved before the #13 windfalls were appended to core
// sat at the ID OreMeteor now has; by its key it moves past the new core
// IDs, while the core windfalls and the song's end tick are untouched.
func TestRemapIncidentsAfterWindfallAppends(t *testing.T) {
	t.Parallel()
	if incident.OreMeteor != incident.TransportPodCrash+1 || incident.SkySong != incident.OreMeteor+2 {
		t.Fatal("the windfalls are not the three IDs after TransportPodCrash")
	}
	old := incidentBook("p.a").Keys()
	old = append(slices.Clone(old[:incident.OreMeteor]), old[incident.SkySong+1:]...) // the key list before the appends
	if old[incident.OreMeteor] != "p.a" {
		t.Fatalf("old keys %v", old[incident.OreMeteor-1:])
	}
	n := incident.ID(incident.Core().Len())
	st := &incident.State{SongUntil: 900, Last: map[incident.ID]uint64{incident.OreMeteor: 7, incident.TransportPodCrash: 5}}
	st.Enqueue(incident.Queued{ID: incident.OreMeteor, At: 10})
	Keys{incident.Class: old}.RemapIncidents(incidentBook("p.a"), st)
	if want := map[incident.ID]uint64{n: 7, incident.TransportPodCrash: 5}; !reflect.DeepEqual(st.Last, want) {
		t.Fatalf("remapped Last %v, want %v", st.Last, want)
	}
	if len(st.Queue) != 1 || st.Queue[0].ID != n || st.SongUntil != 900 {
		t.Fatalf("remapped Queue %+v, SongUntil %d", st.Queue, st.SongUntil)
	}
	// Today's saves (core windfalls in the key list) keep them.
	st = &incident.State{Last: map[incident.ID]uint64{incident.SkySong: 3}}
	Keys{incident.Class: incidentBook("p.a").Keys()}.RemapIncidents(incidentBook("p.a"), st)
	if st.Last[incident.SkySong] != 3 {
		t.Fatalf("core windfall moved: %v", st.Last)
	}
}
