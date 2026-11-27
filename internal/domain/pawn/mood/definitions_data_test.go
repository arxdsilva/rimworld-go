// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import (
	_ "embed"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/expectations"
	"rimworld-go/internal/domain/sim/clock"
)

//go:embed definitions.json
var reviewedDefinitions []byte

func TestDefinitionsMatchReviewedData(t *testing.T) {
	var rows []struct {
		ID          int    `json:"id"`
		Kind        string `json:"kind"`
		Label       string `json:"label"`
		Value       int    `json:"value"`
		Duration    string `json:"duration"`
		Group       string `json:"group"`
		StackLimit  uint8  `json:"stack_limit"`
		StackFactor int32  `json:"stack_factor"`
		Source      string `json:"source"`
	}
	if err := json.Unmarshal(reviewedDefinitions, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != int(numKinds) {
		t.Fatalf("definitions: %d rows for %d kinds", len(rows), numKinds)
	}
	groups := []string{
		"groupNone", "groupSleepPlace", "groupSleepRoom", "groupMealKind",
		"groupMealTable", "groupLaidToRest", "groupSoaked", "groupSnowDay",
		"groupLovesWork", "groupDiningRoom", "groupRecRoom", "groupSharedBed",
		"groupComfort", "groupBeauty", "groupOutdoors", "groupSleptTemp",
		"groupTempCold", "groupTempHot",
	}
	for id, row := range rows {
		if row.ID != id || row.Kind == "" {
			t.Fatalf("saved thought id at %d: %+v", id, row)
		}
		if row.Source != "original tuning" && !strings.HasPrefix(row.Source, "wiki: ") {
			t.Fatalf("thought %s has no clean-room provenance", row.Kind)
		}
		group := slices.Index(groups, row.Group)
		if group < 0 {
			t.Fatalf("unknown group %q for %s", row.Group, row.Kind)
		}
		var ticks uint64
		if row.Duration != "0" {
			unit := row.Duration[len(row.Duration)-1]
			count, err := strconv.Atoi(row.Duration[:len(row.Duration)-1])
			if err != nil || count <= 0 || (unit != 'd' && unit != 'h' && unit != 't') {
				t.Fatalf("invalid duration %q for %s", row.Duration, row.Kind)
			}
			ticks = uint64(count) * uint64(clock.TicksPerHour)
			if unit == 't' {
				ticks = uint64(count)
			}
			if unit == 'd' {
				ticks = uint64(count) * uint64(clock.TicksPerDay)
			}
		}
		want := Def{
			Label: row.Label, Value: row.Value, Duration: ticks,
			Group: uint8(group), StackLimit: row.StackLimit, StackFactor: row.StackFactor,
		}
		if got := defs[Kind(id)]; got != want {
			t.Errorf("id %d (%s): generated %+v, data %+v", id, row.Kind, got, want)
		}
	}
	// #13: the sky song is a positive remembered thought (its real expiry
	// comes from AddLabeledUntil).
	if sky := DefOf(SkySong); SkySong != 114 || sky.Value != 6 || sky.Duration == 0 {
		t.Fatalf("sky song %d: %+v, want id 114, value 6, remembered", SkySong, sky)
	}
	for band := expectations.ExtremelyLow; band <= expectations.SkyHigh; band++ {
		kind := ExpectationsExtremelyLow + Kind(band)
		if got, want := DefOf(kind).Value, band.Effects().MoodBonus; got != want {
			t.Fatalf("wealth-band mood %d changed: %d, want %d", band, got, want)
		}
	}
}
