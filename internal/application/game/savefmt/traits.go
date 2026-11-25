// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// TraitIDs maps a saved trait ID onto book (PLG-044, PLG-026): the ID of
// the same key now, trait.None when the run lacks the key (its plugin is
// no longer loaded). A save without trait keys keeps its IDs.
func (k Keys) TraitIDs(book trait.Book) func(trait.ID) trait.ID {
	to := k.Remap(trait.Class, func(key string) (int, bool) {
		if key == "core.brawler" {
			key = "core.scrapper"
		}
		id, ok := book.IDOf(key)
		return int(id), ok
	})
	return func(id trait.ID) trait.ID { return trait.ID(to(int(id))) } // #nosec G115 -- Remap yields 0, the input, or a current book ID (keys.go:23-40)
}

// RemapTraits maps the saved trait IDs of bios, colonists and any other
// pawns, in place, onto book: a plugin's traits move when the plugin set or
// its order changes, and one whose plugin is gone is taken off the pawn.
func (k Keys) RemapTraits(book trait.Book, bios map[entity.EntityID]pawngen.Pawn, colonists []pawngen.Pawn, others ...*pawngen.Pawn) {
	to := k.TraitIDs(book)
	for id, p := range bios {
		bios[id] = p.RemappedTraits(to)
	}
	for i, p := range colonists {
		colonists[i] = p.RemappedTraits(to)
	}
	for _, p := range others {
		if p != nil {
			*p = p.RemappedTraits(to)
		}
	}
}
