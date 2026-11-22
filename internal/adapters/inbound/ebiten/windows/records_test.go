// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestRecordRows(t *testing.T) {
	t.Parallel()
	ps := []game.PawnView{
		{Name: "B", Kills: 1, DamageDealt: 9},
		{Name: "A", Kills: 3},
		{Name: "Idle"},
		{Name: "Raider", Hostile: true, Kills: 9, DamageTaken: 50},
		{Name: "C", Kills: 1, DamageDealt: 20},
		{Name: "Taken", DamageTaken: 30},
		{Name: "D", Kills: 1, DamageDealt: 9, DamageTaken: 4},
		{Name: "Bruised", DamageTaken: 70},
	}
	rows := RecordRows(ps, func(p game.PawnView) string { return p.Name })
	want := []string{"A", "C", "D", "B", "Bruised", "Taken"}
	if len(rows) != len(want) {
		t.Fatalf("rows %+v", rows)
	}
	for i, n := range want {
		if rows[i].Name != n {
			t.Fatalf("row %d %q, want %q (rows %+v)", i, rows[i].Name, n, rows)
		}
	}
	if rows[2].Taken != 4 || rows[4].Taken != 70 || rows[5].Taken != 30 {
		t.Fatalf("Taken column wrong: %+v", rows)
	}
}

func TestRecordCellsDrawFourColumns(t *testing.T) {
	t.Parallel()
	cells := recordCells(10, 700, [4]string{"Ada", "3", "40", "12"})
	if cells[3].Text != "12" || cells[2].Text != "40" {
		t.Fatalf("cells = %+v", cells)
	}
	for i := 1; i < 4; i++ {
		if cells[i].X <= cells[i-1].X {
			t.Fatalf("column %d not right of column %d: %+v", i, i-1, cells)
		}
	}
	if cells[0].X+cells[0].Fit > cells[1].X {
		t.Fatalf("name column overlaps Kills: %+v", cells)
	}
	if cells[3].X+60 > 10+700 {
		t.Fatalf("Damage taken column runs off the list: %+v", cells)
	}
}
