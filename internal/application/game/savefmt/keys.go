// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/entity"
)

// Keys is, per moddable class, the namespaced key of every index a save
// holds: Keys[class][i-1] is index i's key (index 0 is "none"). A save
// records it so a load maps its indexes onto the running content's, which
// moves when the plugin set or its order changes (PLG-044).
type Keys map[string][]string

// Remap maps a saved index of class onto the running content: the index
// of the same key now, or 0 when the run lacks the key (a plugin that is
// no longer loaded). now looks a key up in the running content. A save
// without the class (written before PLG-044, core content only) keeps its
// indexes.
func (k Keys) Remap(class string, now func(key string) (int, bool)) func(int) int {
	saved, ok := k[class]
	if !ok {
		return func(i int) int { return i }
	}
	to := make([]int, len(saved)+1)
	for i, key := range saved {
		if j, ok := now(key); ok {
			to[i+1] = j
		}
	}
	return func(i int) int {
		if i <= 0 || i >= len(to) {
			return 0
		}
		return to[i]
	}
}

// RemapBackstories maps the saved backstory IDs of bios and colonists, in
// place, onto book: a plugin's backstories move when the plugin set or its
// order changes, and one whose plugin is gone becomes none.
func (k Keys) RemapBackstories(book backstory.Book, bios map[entity.EntityID]pawngen.Pawn, colonists []pawngen.Pawn, others ...*pawngen.Pawn) {
	to := k.Remap(backstory.Class, func(key string) (int, bool) {
		id, ok := book.IDOf(key)
		return int(id), ok
	})
	bs := func(id backstory.ID) backstory.ID { return backstory.ID(to(int(id))) }
	for id, p := range bios {
		bios[id] = p.Remapped(bs)
	}
	for i, p := range colonists {
		colonists[i] = p.Remapped(bs)
	}
	for _, p := range others { // the world's people: visitors, raiders, prisoners met before
		if p != nil {
			*p = p.Remapped(bs)
		}
	}
}
