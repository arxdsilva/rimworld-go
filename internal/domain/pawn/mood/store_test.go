// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestPruneAllReclaimsExpiredMemoriesAndLeavesActiveOnes(t *testing.T) {
	t.Parallel()
	const expired, living entity.EntityID = 3, 8
	all := map[entity.EntityID]Memories{
		expired: {{Kind: Grief, Expires: 10, Label: "old loss", Subject: 22}},
		living: {
			{Kind: Grief, Expires: 10, Label: "old loss", Subject: 23},
			{Kind: GotMarried, Expires: 900, Label: "new marriage"},
		},
	}
	PruneAll(all, 10)
	if _, ok := all[expired]; ok {
		t.Fatal("expired pawn memory entry was retained")
	}
	if len(all[living]) != 1 || all[living][0].Kind != GotMarried {
		t.Fatalf("active memories were changed: %+v", all[living])
	}
	PrunePawn(all, living, 900)
	if len(all) != 0 {
		t.Fatalf("expired memories retained: %+v", all)
	}
}
