// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// The Auto-slaughter cells keep the x and w the labels and step buttons
// had: Species 150-8 wide, then three 110 wide fields.
func TestAutoCellsKeepTheirAnchors(t *testing.T) {
	t.Parallel()
	list := autoListRect(gfx.Rect{X: 12, Y: 40, W: 480, H: 300})
	if list != (gfx.Rect{X: 12, Y: 68, W: 480, H: 272}) {
		t.Fatalf("list %+v", list)
	}
	want := []colonyWant{{autoSpecies, 12, 142}, {autoMales, 162, 110}, {autoFemales, 272, 110}, {autoYoung, 382, 110}}
	cells := autoCells(nil, list)
	if len(cells) != len(want) {
		t.Fatalf("%d cells", len(cells))
	}
	for i, wc := range want {
		if c := cells[i]; c.Col != wc.id || c.R.X != wc.x || c.R.W != wc.w || c.R.Y != 68 {
			t.Errorf("cell %d: %s %+v, want %+v", i, c.Col, c.R, wc)
		}
	}
	if !autoCell(cells, "none").Empty() {
		t.Error("an unknown column has a cell")
	}
}

// Every Auto-slaughter column is drawn on the Animals window's anchor at
// both screens.
func TestAutoSlaughterColumnsDrawn(t *testing.T) {
	t.Parallel()
	cols := autoColumns(colonyEnglish{})
	declared := uikit.TableColIDs(cols[:])
	for _, s := range uikit.ColumnScreens() {
		win, body := windowsKitBody(t, func(w *Set) { w.Animals().OpenAutoSlaughter() }, s.W, s.H)
		var buf [autoColN]uikit.TableCell
		cells := autoCells(buf[:0], autoListRect(body))
		if gaps := uikit.ColumnGaps(win, declared, nil, cells); gaps != nil {
			t.Errorf("%dx%d: %v", s.W, s.H, gaps)
		}
	}
}

// autoBody draws its header and species label at the cells and puts each
// field's step buttons inside its cell.
func TestAutoBodyRunsOnTheCells(t *testing.T) { //nolint:paralleltest // canvas-global world pass
	fs, w := newFake()
	fs.env.Game = fakeReader{}
	fs.world.Animals = []game.AnimalView{{ID: 1, Species: "dustbuck", Label: "Dustbuck", Tame: true}}
	w.Animals().OpenAutoSlaughter()
	body := gfx.Rect{X: 20, Y: 40, W: 480, H: 300}
	w.Animals().autoBody(body)
	cells := autoCells(nil, autoListRect(body))
	a := anchorsOf(func(img *ebiten.Image) { fs.kit.Paint(img) })
	for _, c := range cells {
		if !drawnAt(a, c.R.X, body.Y, body.Y+1) {
			t.Errorf("header of %s not drawn at its cell x %v", c.Col, c.R.X)
		}
	}
	if sp := cells[0].R; !drawnAt(a, sp.X, sp.Y, sp.Y+autoRowH) {
		t.Error("the species label is not drawn at its cell")
	}
	for f, id := range []string{autoMales, autoFemales, autoYoung} {
		c := autoCell(cells, id)
		for _, dir := range []string{"dec", "inc"} {
			b, ok := fs.kit.Find("autoslaughter.dustbuck." + string(rune('0'+f)) + "." + dir)
			if !ok || b.R.X < c.X || b.R.Right() > c.Right() || b.R.Y != c.Y {
				t.Errorf("%s %s step button %+v (found %v) is not in its cell %+v", id, dir, b.R, ok, c)
			}
		}
	}
}
