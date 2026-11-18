// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import "rimworld-go/internal/domain/incident"

// IncidentIDs maps a saved incident ID onto the running content (PLG-044,
// PLG-026): the ID of the same key now, ok false when the run lacks the key
// (its plugin is no longer loaded). An incident ID starts at 0, a real
// incident, so it is shifted by one through Remap, whose 0 is "gone".
type IncidentIDs func(incident.ID) (incident.ID, bool)

// Incidents is the incident map onto book. A save without incident keys
// (core content only) keeps its IDs.
func (k Keys) Incidents(book incident.Book) IncidentIDs {
	to := k.Remap(incident.Class, func(key string) (int, bool) {
		id, ok := book.IDOf(key)
		return int(id) + 1, ok
	})
	return func(id incident.ID) (incident.ID, bool) {
		j := to(int(id) + 1)
		if j == 0 {
			return 0, false
		}
		return incident.ID(j - 1), true
	}
}

// RemapIncidents maps the storyteller's saved state onto book in place:
// each event's last-fired tick and each queued event moves to its new ID,
// and an entry whose incident is gone (its plugin is not loaded) is
// dropped. The running climate event is always a core one (a plugin
// incident runs its like's behaviour), so it keeps its ID.
func (k Keys) RemapIncidents(book incident.Book, st *incident.State) {
	to := k.Incidents(book)
	if len(st.Last) > 0 {
		last := make(map[incident.ID]uint64, len(st.Last))
		for id, at := range st.Last {
			if j, ok := to(id); ok {
				last[j] = at
			}
		}
		st.Last = last
	}
	if len(st.Queue) > 0 {
		q := st.Queue[:0:0]
		for _, e := range st.Queue {
			if j, ok := to(e.ID); ok {
				e.ID = j
				q = append(q, e)
			}
		}
		st.Queue = q
	}
}
