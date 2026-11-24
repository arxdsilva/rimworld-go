// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wizard

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// TestFactionsTableKeepsItsAnchors pins the split of the Factions dialog's
// body: the table above, a 24 px hint line under it.
func TestFactionsTableKeepsItsAnchors(t *testing.T) {
	tb, hint := factionsSplit(gfx.Rect{X: 10, Y: 20, W: 1000, H: 400})
	if tb != (gfx.Rect{X: 10, Y: 20, W: 1000, H: 376}) || hint != (gfx.Rect{X: 10, Y: 396, W: 1000, H: 24}) {
		t.Fatalf("split = %+v / %+v", tb, hint)
	}
}

// TestFactionColumnsDrawn lays the Factions dialog out as production does
// (the pushed modal through ModalLayout) and wants every column placed,
// the titleless emblem column included.
func TestFactionColumnsDrawn(t *testing.T) {
	for _, sc := range uikit.ColumnScreens() {
		sh, w := toWorld(t)
		generate(t, sh, w)
		ng := w.Draft()
		landingPage{w}.pushFactions(ng, nil)
		var m *uikit.Modal
		for i := range sh.kit.Modals {
			if sh.kit.Modals[i].ID == FacModalID {
				m = &sh.kit.Modals[i]
			}
		}
		if m == nil {
			t.Fatal("the dialog did not open")
		}
		_, _, body, _ := uikit.ModalLayout(sc.W, sc.H, *m)
		tb, _ := factionsSplit(body)
		cols := w.FactionColumns(FactionRows(ng.Planet.Factions))
		if cols[0].ID != "emblem" || cols[0].Title != "" {
			t.Fatalf("first column = %q %q", cols[0].ID, cols[0].Title)
		}
		if gaps := uikit.TableColumnGaps(body, tb, cols, FacRowH); len(gaps) != 0 {
			t.Errorf("%dx%d: %v", sc.W, sc.H, gaps)
		}
	}
}

// TestScenarioFileColumnsDrawn opens both scenario file loaders and wants
// every column placed in the body of the modal each pushed.
func TestScenarioFileColumnsDrawn(t *testing.T) {
	open := map[string]func(*Flow){
		"scen.load":     func(w *Flow) { scenarioPage{w}.openScenarioFiles() },
		"sced.loadfile": func(w *Flow) { editorPage{w}.editorLoad() },
	}
	for id, f := range open {
		for _, sc := range uikit.ColumnScreens() {
			sh, w := newFakeShell(t)
			f(w)
			var m *uikit.Modal
			for i := range sh.kit.Modals {
				if sh.kit.Modals[i].ID == id {
					m = &sh.kit.Modals[i]
				}
			}
			if m == nil {
				t.Fatalf("%s: the modal did not open", id)
			}
			_, _, body, _ := uikit.ModalLayout(sc.W, sc.H, *m)
			cols := scenarioFileColumns(sh, []ScenarioFile{{Name: "a", Modified: "b"}})
			if gaps := uikit.TableColumnGaps(body, body, cols, 28); len(gaps) != 0 {
				t.Errorf("%s %dx%d: %v", id, sc.W, sc.H, gaps)
			}
		}
	}
}

// TestScenarioFileColumnsSort checks the shared builder's columns: the
// names sort without case, the modified column by its text.
func TestScenarioFileColumnsSort(t *testing.T) {
	sh, _ := newFakeShell(t)
	files := []ScenarioFile{{Name: "beta", Modified: "2"}, {Name: "Alpha", Modified: "3"}}
	cols := scenarioFileColumns(sh, files)
	if len(cols) != 2 || cols[0].ID != "name" || cols[1].ID != "mod" || cols[1].W != 150 {
		t.Fatalf("columns = %+v", cols)
	}
	if !cols[0].Less(1, 0) || cols[0].Less(0, 1) || !cols[1].Less(0, 1) {
		t.Error("sort order wrong")
	}
	if cols[0].Cell(1) != "Alpha" || cols[1].Cell(0) != "2" {
		t.Error("cell text wrong")
	}
}
