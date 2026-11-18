// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
)

// A world person's saved backstories follow their keys like a colonist's:
// a save made with a plugin before core's last backstory moved, and one
// whose plugin is gone, both land on the right def (or none).
func TestRemapBackstoriesCoversWorldPeople(t *testing.T) {
	t.Parallel()
	core := backstory.Core()
	last := backstory.ID(core.Len()) // IDs run 1..Len; 0 is none
	saved := core.Keys()
	saved = append(saved, "gone.drifter") // the saving run had a plugin backstory
	k := Keys{backstory.Class: saved}
	plugin := backstory.ID(len(saved))
	person := &pawngen.Pawn{Childhood: last, Adulthood: plugin}
	k.RemapBackstories(core, nil, nil, person, nil)
	if person.Childhood != last || person.Adulthood != 0 {
		t.Fatalf("person backstories %d/%d, want %d/0 (core kept, the missing plugin's dropped)", person.Childhood, person.Adulthood, last)
	}
}
