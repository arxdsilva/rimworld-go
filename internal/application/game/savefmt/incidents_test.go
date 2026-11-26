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
	// a save from before 25.2 (#13), whose core ended at rival_raid, keeps
	// its plugin incidents through their keys when core grows by four
	ks := incident.Core().Keys()
	if !slices.Equal(ks[len(ks)-4:], []string{"core.escape_pod_crash", "core.ore_meteor", "core.supply_drop", "core.sky_song"}) {
		t.Fatalf("fixture: the windfalls are not core's last four: %q", ks[len(ks)-4:])
	}
	old := Keys{incident.Class: append(slices.Clone(ks[:len(ks)-4]), "p.a", "p.b")}
	st = &incident.State{Last: map[incident.ID]uint64{incident.RivalRaid: 4, n - 4: 7, n - 3: 9}}
	st.Enqueue(incident.Queued{ID: n - 3, At: 11})
	old.RemapIncidents(incidentBook("p.a", "p.b"), st)
	if want := map[incident.ID]uint64{incident.RivalRaid: 4, n: 7, n + 1: 9}; !reflect.DeepEqual(st.Last, want) {
		t.Fatalf("pre-#13 Last %v, want %v", st.Last, want)
	}
	if len(st.Queue) != 1 || st.Queue[0].ID != n+1 {
		t.Fatalf("pre-#13 Queue %+v", st.Queue)
	}
	st = state() // a save without keys keeps its IDs
	Keys{}.RemapIncidents(incident.Core(), st)
	if len(st.Last) != 3 || len(st.Queue) != 3 {
		t.Fatalf("keyless save changed: %v %v", st.Last, st.Queue)
	}
}
