// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saves

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// TestSaveListKeepsItsAnchors pins the list's rect to numbers worked out
// from the dialog's constants: the name label (22) and field (28), the note
// line (3 + LineH + 8) and the delete column on the right.
func TestSaveListKeepsItsAnchors(t *testing.T) {
	body := gfx.Rect{X: 10, Y: 20, W: 600, H: 400}
	// 20 + 22 (label) + 28 (field) + 3 + LineH(Small) 15 + 8 = 96; 600 - 84 = 516.
	want := gfx.Rect{X: 10, Y: 96, W: 516, H: 324}
	if got := dialogList(body); got != want {
		t.Fatalf("list = %+v, want %+v", got, want)
	}
}

// TestSaveListColumnsDrawn lays the Save dialog out as production does (the
// pushed modal through ModalLayout at each screen) and wants every declared
// column of the saves table placed.
func TestSaveListColumnsDrawn(t *testing.T) {
	for _, sc := range uikit.ColumnScreens() {
		f, b := newFake()
		openDialog(b, func() {})
		k := f.Kit()
		if len(k.Modals) == 0 {
			t.Fatal("the dialog did not open")
		}
		_, _, body, _ := uikit.ModalLayout(sc.W, sc.H, k.Modals[len(k.Modals)-1])
		list := dialogList(body)
		if gaps := uikit.TableColumnGaps(body, list, dialogCols(b), listRowH); len(gaps) != 0 {
			t.Errorf("%dx%d: %v", sc.W, sc.H, gaps)
		}
		if got := len(uikit.TableColIDs(dialogCols(b))); got != 4 {
			t.Errorf("columns = %d, want 4", got)
		}
	}
}

// TestSaveDialogKeepsItsAnchors pins the name field and the first delete
// cell to numbers from the layout before the helpers existed.
func TestSaveDialogKeepsItsAnchors(t *testing.T) {
	body := gfx.Rect{X: 10, Y: 20, W: 600, H: 400}
	if got, want := dialogField(body), (gfx.Rect{X: 10, Y: 42, W: 600, H: 28}); got != want {
		t.Fatalf("field = %+v, want %+v", got, want)
	}
	list := gfx.Rect{X: 10, Y: 96, W: 516, H: 324}
	// X = 526 + 6; Y = 96 + TableHeaderH + row*26 - scroll + 1; W = 84 - 6; H = 26 - 2.
	if got, want := deleteCell(list, 0, 0), (gfx.Rect{X: 532, Y: 96 + uikit.TableHeaderH + 1, W: 78, H: 24}); got != want {
		t.Fatalf("delete cell 0 = %+v, want %+v", got, want)
	}
	if got, want := deleteCell(list, 2, 10), (gfx.Rect{X: 532, Y: 96 + uikit.TableHeaderH + 52 - 10 + 1, W: 78, H: 24}); got != want {
		t.Fatalf("delete cell 2 = %+v, want %+v", got, want)
	}
}

// TestSaveDeleteColumnDrawn adds the delete column to the table's cells
// and wants it placed inside the dialog's body at both screens.
func TestSaveDeleteColumnDrawn(t *testing.T) {
	for _, sc := range uikit.ColumnScreens() {
		f, b := newFake()
		openDialog(b, func() {})
		k := f.Kit()
		_, _, body, _ := uikit.ModalLayout(sc.W, sc.H, k.Modals[len(k.Modals)-1])
		list := dialogList(body)
		cols := dialogCols(b)
		cells := uikit.TableCells(nil, uikit.TableBodyRow(list, listRowH), cols)
		cells = append(cells, uikit.TableCell{Col: "delete", R: deleteCell(list, 0, 0)})
		declared := append(uikit.TableColIDs(cols), "delete")
		if gaps := uikit.ColumnGaps(body, declared, nil, cells); len(gaps) != 0 {
			t.Errorf("%dx%d: %v", sc.W, sc.H, gaps)
		}
	}
}
