// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wizard

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

type pawnsEnT struct{}

func (pawnsEnT) T(s string) string { return s }

func pawnSkillIDs() []string {
	c := pawnSkillColumns(pawnsEnT{})
	return uikit.TableColIDs(c[:])
}

// TestPawnSkillCellsKeepTheirAnchors pins the table to the offsets it drew
// at before the cells existed (QA-002): Skill at x+8, Passion at x+160,
// Level at x+240, whose bar is W-296 wide from x+280.
func TestPawnSkillCellsKeepTheirAnchors(t *testing.T) {
	row := gfx.Rect{X: 50, Y: 7, W: 600, H: 20}
	got := pawnSkillCells(nil, row)
	want := []uikit.TableCell{
		{Col: "skill", R: gfx.Rect{X: 58, Y: 7, W: 152, H: 20}},
		{Col: "passion", R: gfx.Rect{X: 210, Y: 7, W: 80, H: 20}},
		{Col: "level", R: gfx.Rect{X: 290, Y: 7, W: 360, H: 20}},
	}
	if len(got) != len(want) {
		t.Fatalf("%d cells, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if bx, bw := pawnBarSpan(got[2].R); bx != 330 || bw != 304 {
		t.Errorf("bar x %v w %v, want 330 and 304", bx, bw)
	}
	if g, l := pawnGlyphX(got[1].R), pawnLevelTextX(got[2].R); g != 216 || l != 300 {
		t.Errorf("glyph x %v level text x %v, want 216 and 300", g, l)
	}
	if ids := pawnSkillIDs(); len(ids) != 3 || ids[0] != "skill" || ids[2] != "level" {
		t.Errorf("declared ids = %v", ids)
	}
	// a row narrower than the Level start leaves no negative width
	for _, c := range pawnSkillCells(nil, gfx.Rect{W: 100, H: 10}) {
		if c.R.W < 0 {
			t.Errorf("%s: negative width %v", c.Col, c.R.W)
		}
	}
}

// TestPawnSkillColumnsDrawn lays the table out in the detail panel the Pawns
// page builds from uikit.DialogFrame at each screen and checks all three
// columns have a cell in the header and every row.
func TestPawnSkillColumnsDrawn(t *testing.T) {
	declared := pawnSkillIDs()
	for _, sc := range uikit.ColumnScreens() {
		_, _, content, _ := uikit.DialogFrame(sc.W, sc.H)
		detail, _ := pawnDetailRect(content)
		in := detail.Inset(12)
		if in.Empty() {
			t.Fatalf("%dx%d: no detail panel", sc.W, sc.H)
		}
		y := in.Y
		for i := -1; i < 11; i++ {
			h := float32(20)
			if i < 0 {
				h = pawnSkillHdrH
			}
			row := gfx.Rect{X: in.X, Y: y, W: in.W, H: h}
			y += h
			if g := uikit.ColumnGaps(detail, declared, nil, pawnSkillCells(nil, row)); g != nil {
				t.Errorf("%dx%d row %d: %v", sc.W, sc.H, i, g)
			}
		}
	}
}
