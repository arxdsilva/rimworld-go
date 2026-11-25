// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import "rimworld-go/internal/domain/sim/entity"

// PrunePawn drops one pawn's expired event memories and releases empty
// entries. Read-model queries use AppendActiveAt and never call this.
func PrunePawn(all map[entity.EntityID]Memories, id entity.EntityID, now uint64) {
	if active := all[id].Prune(now); len(active) > 0 {
		all[id] = active
	} else {
		delete(all, id)
	}
}

// PruneAll reclaims expired event memories in the daily social phase even
// when a map has no standing colonists or social edges.
func PruneAll(all map[entity.EntityID]Memories, now uint64) {
	for id := range all {
		PrunePawn(all, id, now)
	}
}
