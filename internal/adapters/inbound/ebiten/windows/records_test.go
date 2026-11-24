// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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

// TestRecordCellsDrawFourColumns is the DEF 3.1 guard: all four columns are
// routed, ordered, apart and inside the list.
func TestRecordCellsDrawFourColumns(t *testing.T) {
	t.Parallel()
	list := gfx.Rect{X: 10, Y: 0, W: 700, H: 22}
	texts := recordTexts(recordRow{Name: "Ada", Kills: 3, Damage: 40, Taken: 12})
	var buf [4]uikit.TableCell
	cells := recordCells(buf[:0], list)
	if len(cells) != 4 {
		t.Fatalf("cells = %+v", cells)
	}
	// routing: Damage dealt is value index 2, Damage taken index 3
	if texts[3] != "12" || texts[2] != "40" || texts[1] != "3" || texts[0] != "Ada" {
		t.Fatalf("texts = %+v", texts)
	}
	if cells[2].Col != "dealt" || cells[3].Col != "taken" {
		t.Fatalf("cells = %+v", cells)
	}
	// order: each cell is right of the previous one
	for i := 1; i < 4; i++ {
		if cells[i].R.X <= cells[i-1].R.X {
			t.Fatalf("column %d not right of column %d: %+v", i, i-1, cells)
		}
	}
	// no overlap: the name column stops before Kills
	if cells[0].R.Right() > cells[1].R.X {
		t.Fatalf("name column overlaps Kills: %+v", cells)
	}
	// inside the list: Damage taken does not run off it
	if cells[3].R.Right() > list.Right() {
		t.Fatalf("Damage taken column runs off the list: %+v", cells)
	}
}
