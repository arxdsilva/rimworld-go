// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package letters

import (
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/pawn/body"
)

func TestEventCodeKindsAndRaidFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		k    body.CondKind
		code string
	}{
		{body.CondFlu, "colony.disease.flu"},
		{body.CondPlague, "colony.disease.plague"},
		{body.CondMalaria, "colony.disease.malaria"},
		{body.CondSleepingSickness, "colony.disease.sleeping_sickness"},
		{body.CondGutWorms, "colony.disease.gut_worms"},
		{body.CondMuscleParasites, "colony.disease.muscle_parasites"},
	} {
		if got := DiseaseCode(tc.k); got != tc.code {
			t.Fatalf("condition %v has event code %q, want %q", tc.k, got, tc.code)
		}
	}
	if RaidCode(raid.Siege) != "colony.raid.siege" ||
		RaidCode(raid.Sappers) != "colony.raid.sappers" {
		t.Fatal("raid plans lost distinct codes")
	}
	fields := RaidFields(2, "", "faction", "")
	if len(fields) != 1 || fields[0].Name != "count" || fields[0].Value != "2" {
		t.Fatalf("drop-raid count-only fields %+v", fields)
	}
}
