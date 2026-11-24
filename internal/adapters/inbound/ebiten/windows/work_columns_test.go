// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// TestWorkCellsMatchCellRect: every column's cell is WorkCellRect, and the
// name cell sits at the window padding, as drawn.
func TestWorkCellsMatchCellRect(t *testing.T) {
	t.Parallel()
	r := uikit.Button{X: 100, Y: 50, W: 900, H: 400}
	var buf [workMaxCells]uikit.TableCell
	cells := workCells(buf[:0], r, 2)
	if len(cells) != len(WorkColumns)+1 {
		t.Fatalf("%d cells, want %d", len(cells), len(WorkColumns)+1)
	}
	name := cells[0]
	if want := (gfx.Rect{X: 100 + hudlayout.WindowPad, Y: 50 + 97 + 60, W: 232, H: hudlayout.WorkBox}); name.Col != "name" || name.R != want {
		t.Errorf("name cell = %+v, want %+v", name, want)
	}
	for _, c := range cells[1:] {
		col := workColIndex(c.Col)
		if col < 0 || WorkColumns[col] != c.Col {
			t.Fatalf("cell %q is not a work column", c.Col)
		}
		if want := WorkCellRect(r, 2, col); c.R != want {
			t.Errorf("%s: %+v, want %+v", c.Col, c.R, want)
		}
	}
	if workColIndex("name") != -1 {
		t.Error("the name cell is not a work column")
	}
}

// TestWorkColumnsDrawn: the name and all 14 live columns are placed inside the
// Work window, for every shown row, at both column screens.
func TestWorkColumnsDrawn(t *testing.T) {
	t.Parallel()
	if len(WorkLive) != len(WorkColumns) {
		t.Fatalf("%d live of %d columns", len(WorkLive), len(WorkColumns))
	}
	declared := workDeclared()
	for _, s := range uikit.ColumnScreens() {
		_, win := windowsMainTabRect(t, func(w *Set) { w.Show("work") }, s.W, s.H)
		r := uikit.Button{X: win.X, Y: win.Y, W: win.W, H: win.H}
		for row := 0; row < 3; row++ {
			var buf [workMaxCells]uikit.TableCell
			cells := workCells(buf[:0], r, row)
			if gaps := uikit.ColumnGaps(win, declared, nil, cells); len(gaps) > 0 {
				t.Errorf("%dx%d row %d: %v (window %+v)", s.W, s.H, row, gaps, win)
			}
		}
	}
}

// TestWorkNameCellKeepsWholePixelRow: the name text sat on int(r.Y)+97+row*30
// before the cells existed; a fractional window Y still truncates.
func TestWorkNameCellKeepsWholePixelRow(t *testing.T) {
	t.Parallel()
	var buf [workMaxCells]uikit.TableCell
	name := workCells(buf[:0], uikit.Button{X: 10, Y: 50.75, W: 900, H: 400}, 1)[0]
	if name.R.Y != 177 { // 50 + 97 + 30
		t.Fatalf("name Y = %v, want 177", name.R.Y)
	}
}

// workDrawShell lays the Work window in a chosen rect instead of the fake's.
type workDrawShell struct {
	*fakeShell
	win gfx.Rect
}

func (s workDrawShell) WindowRect() gfx.Rect { return s.win }

// TestWorkDrawOnTheCells: at both column screens every column's header, each
// row's name and each row's cell box are drawn at their workCells x. Dropping
// a column from the draw loops, or shifting a draw off its cell, turns it red.
func TestWorkDrawOnTheCells(t *testing.T) { //nolint:paralleltest // canvas-global world pass
	for _, s := range uikit.ColumnScreens() {
		_, win := windowsMainTabRect(t, func(w *Set) { w.Show("work") }, s.W, s.H)
		fs, _ := newFake()
		fs.env.Game = fakeReader{}
		for i := 1; i <= 3; i++ {
			fs.pawns = append(fs.pawns, game.PawnView{ID: entity.EntityID(i), Name: "P", Status: "alive", Priorities: []int{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}})
		}
		w := New(workDrawShell{fs, win})
		a := anchorsOf(func(img *ebiten.Image) { w.Work().draw(img, s.W, s.H) })
		r := uikit.Button{X: win.X, Y: win.Y, W: win.W, H: win.H}
		for _, c := range workCells(nil, r, 0) {
			if col := workColIndex(c.Col); col >= 0 {
				x := c.R.X + float32(hudlayout.WorkBox)/2
				if !drawnAt(a, x, win.Y+workHeadY, win.Y+workHeadY+workHeadH) {
					t.Errorf("%dx%d: header %s not drawn at its cell x %v", s.W, s.H, c.Col, x)
				}
			}
		}
		for row := 0; row < 3; row++ {
			for _, c := range workCells(nil, r, row) {
				x, y0, y1 := c.R.X, c.R.Y, c.R.Y+c.R.H
				if c.Col != workNameCol {
					x += float32(hudlayout.WorkBox) / 2 // the box centre
				}
				if !drawnAt(a, x, y0, y1) {
					t.Errorf("%dx%d row %d: %s not drawn at its cell (x %v, y %v..%v)", s.W, s.H, row, c.Col, x, y0, y1)
				}
			}
		}
	}
}

// TestRecordsDrawOnTheCells: the Records header and a data row put each
// column's text at its recordCells x at both column screens (the draw clips to
// a sub-image, so the texts are read from the gfx draw trace).
func TestRecordsDrawOnTheCells(t *testing.T) { //nolint:paralleltest // canvas-global draw trace
	for _, s := range uikit.ColumnScreens() {
		fs, w := newFake()
		fs.env.Game = fakeReader{}
		fs.pawns = []game.PawnView{{ID: 1, Name: "Ada", Status: "alive", Kills: 3, DamageDealt: 40, DamageTaken: 12}}
		_, body := windowsKitBody(t, func(w *Set) { w.History().Launch() }, s.W, s.H)
		list := historyListRect(body)
		w.History().recordsList(list)
		var got []gfx.TraceEvent
		gfx.SetDrawTrace(func(e gfx.TraceEvent) {
			if e.Kind == "text" {
				got = append(got, e)
			}
		})
		fs.kit.Paint(ebiten.NewImage(s.W, s.H))
		gfx.SetDrawTrace(nil)
		cells := recordCells(nil, recordsView(body))
		want := [][4]string{{"Colonist", "Kills", "Damage dealt", "Damage taken"}, {"Ada", "3", "40", "12"}}
		sc := float32(gfx.UIScale())
		for row, texts := range want {
			for k, c := range cells {
				found := false
				for _, e := range got {
					if e.Text == texts[k] && e.X == c.R.X*sc {
						found = true
					}
				}
				if !found {
					t.Errorf("%dx%d row %d: %q not drawn at the %s cell x %v (trace %+v)", s.W, s.H, row, texts[k], c.Col, c.R.X, got)
				}
			}
		}
	}
}
