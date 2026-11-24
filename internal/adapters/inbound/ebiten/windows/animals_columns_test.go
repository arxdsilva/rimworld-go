// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"slices"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

type colonyEnglish struct{}

func (colonyEnglish) T(s string) string { return s }

// colonyWant is a cell's expected x (from the window's left, after the
// dropped columns) and width, worked out by hand from the column consts.
type colonyWant struct {
	id   string
	x, w float32
}

// colonyCellsAt checks colonyCells of a window at (10, 20) against want.
func colonyCellsAt(t *testing.T, width float32, want []colonyWant) {
	t.Helper()
	win := gfx.Rect{X: 10, Y: 20, W: width, H: 400}
	cells := colonyCells(nil, win, 1) // the second row: 30 px lower
	if len(cells) != len(want) {
		t.Fatalf("%v px: %d cells, want %d: %+v", width, len(cells), len(want), cells)
	}
	for i, wc := range want {
		got := cells[i]
		h := float32(24)
		if wc.id == cIcon {
			h = 26
		}
		exp := gfx.Rect{X: 10 + wc.x, Y: 20 + 60 + 30, W: wc.w, H: h}
		if got.Col != wc.id || got.R != exp {
			t.Errorf("%v px cell %d: %s %+v, want %s %+v", width, i, got.Col, got.R, wc.id, exp)
		}
	}
}

// The Colony cells sit where the columns were drawn before they were
// declared: at full width, and at 1226 px (Doing and Training dropped, the
// columns after them 212 px left).
func TestColonyCellsKeepTheirAnchors(t *testing.T) {
	t.Parallel()
	common := []colonyWant{{cIcon, 16, 26}, {cName, 52, 76}, {cSpecies, 134, 92}, {cSex, 232, 22}, {cAge, 258, 58}, {cFood, 322, 70},
		{cProduct, 396, 66}, {cPreg, 468, 38}, {cHealth, 512, 40}, {cMaster, 556, 88}, {cFollow, 648, 48}, {cArea, 704, 118}}
	full := append(slices.Clone(common), colonyWant{cTrain, 828, 98}, colonyWant{cDoing, 932, 100}, colonyWant{cActions, 1040, 196}, colonyWant{cDiet, 1242, 120})
	colonyCellsAt(t, 1374, full)
	narrow := append(slices.Clone(common), colonyWant{cActions, 828, 196}, colonyWant{cDiet, 1030, 120})
	colonyCellsAt(t, 1226, narrow)
}

// Every declared Colony column is drawn at both screens; only the columns
// colonyDrops names may be missing, and only at 1280x768, exactly the ones
// animalColsFor drops there.
func TestColonyColumnsDrawn(t *testing.T) {
	t.Parallel()
	cols := colonyColumns(colonyEnglish{})
	declared := uikit.TableColIDs(cols[:])
	for _, s := range uikit.ColumnScreens() {
		_, win := windowsMainTabRect(t, func(w *Set) { w.ToggleAnimals() }, s.W, s.H)
		var buf [colonyColN]uikit.TableCell
		cells := colonyCells(buf[:0], win, 0)
		var droppable []string
		if s.Small {
			droppable = colonyDropIDs()
		}
		if gaps := uikit.ColumnGaps(win, declared, droppable, cells); gaps != nil {
			t.Errorf("%dx%d: %v", s.W, s.H, gaps)
		}
		var dropped []string
		for _, d := range animalColsFor(win.W, false) {
			dropped = append(dropped, d.id)
		}
		for _, id := range declared {
			if missing := colonyCell(cells, id).Empty(); missing != slices.Contains(dropped, id) {
				t.Errorf("%dx%d: column %s missing=%v but animalColsFor drops %v", s.W, s.H, id, missing, dropped)
			}
		}
		if !s.Small && len(cells) != colonyColN {
			t.Errorf("%dx%d: %d of %d cells placed", s.W, s.H, len(cells), colonyColN)
		}
	}
}

// The header's titles come from the declared list and the droppable ids are
// declared ones.
func TestColonyColumnsDeclareTheDrops(t *testing.T) {
	t.Parallel()
	cols := colonyColumns(colonyEnglish{})
	ids := uikit.TableColIDs(cols[:])
	for _, id := range colonyDropIDs() {
		if !slices.Contains(ids, id) {
			t.Errorf("drop %q is not a declared column", id)
		}
	}
	if len(colonyDropIDs()) != len(colonyDrops) || colonyHeadInset(cArea) != 2 || colonyHeadInset(cName) != 0 {
		t.Error("drop ids or head insets wrong")
	}
}

// Buttons, the area dropdown and the master menu read the cell rects.
func TestColonyWidgetsUseCells(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.world.Animals = []game.AnimalView{{ID: 50, Species: "dustbuck", Tame: true, CanFollow: true, FoodPolicy: "x"}, {ID: 51, Species: "dustbuck", Tame: true}}
	w.ToggleAnimals()
	r := w.Rect()
	win := gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
	by := map[string]uikit.Button{}
	for _, b := range w.Animals().Buttons() {
		by[b.ID] = b
	}
	for i, id := range []string{"50", "51"} {
		cells := colonyCells(nil, win, i)
		eq := func(b uikit.Button, c gfx.Rect, wide float32) {
			t.Helper()
			if b.ID == "" || b.X != c.X || b.Y != c.Y || b.W != wide || b.H != c.H {
				t.Errorf("row %d: button %s at %v,%v %vx%v, cell %+v", i, b.ID, b.X, b.Y, b.W, b.H, c)
			}
		}
		act := colonyCell(cells, cActions)
		eq(by["animals.master."+id], colonyCell(cells, cMaster), colonyCell(cells, cMaster).W)
		eq(by["animals.slaughter."+id], act, 100)
		rel := by["animals.release."+id]
		if rel.X+rel.W != act.Right() || rel.Y != act.Y {
			t.Errorf("row %d: release %+v does not end the actions cell %+v", i, rel, act)
		}
		if diet := colonyCell(cells, cDiet); diet.Empty() {
			if _, ok := by["animals.diet."+id]; ok {
				t.Errorf("row %d: a Diet button with the column dropped", i)
			}
		} else {
			eq(by["animals.diet."+id], diet, diet.W)
		}
		if i == 0 {
			eq(by["animals.follow."+id], colonyCell(cells, cFollow), colonyCell(cells, cFollow).W)
		} else if _, ok := by["animals.follow."+id]; ok {
			t.Errorf("row %d has a follow button without CanFollow", i)
		}
	}
	before := sh.floats
	w.Animals().masterMenu(sh.world.Animals[1])
	if sh.floats != before+1 {
		t.Errorf("master menu floats %d, want %d", sh.floats, before+1)
	}
}

// colonyShell records the Area dropdowns the Declare pass makes.
type colonyShell struct {
	*fakeShell
	areas map[string]gfx.Rect
}

func (c colonyShell) AreaDropdown(id string, r gfx.Rect, _ int, _ func(int)) { c.areas[id] = r }

// Declare puts the Area dropdown in the Area cell of every unpenned row.
func TestColonyDeclareUsesAreaCells(t *testing.T) {
	t.Parallel()
	fs, _ := newFake()
	cs := colonyShell{fs, map[string]gfx.Rect{}}
	w := New(cs)
	fs.env.Game = fakeReader{}
	fs.world.Animals = []game.AnimalView{{ID: 50, Tame: true}, {ID: 51, Tame: true, Penned: true}, {ID: 52, Tame: true}}
	w.ToggleAnimals()
	w.Animals().Declare(1280, 720)
	r := w.Rect()
	win := gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
	if len(cs.areas) != 2 {
		t.Fatalf("dropdowns %v, want rows 0 and 2 only", cs.areas)
	}
	for id, row := range map[string]int{"animals.area.50": 0, "animals.area.52": 2} {
		if got, want := cs.areas[id], colonyCell(colonyCells(nil, win, row), cArea); got != want {
			t.Errorf("%s at %+v, want the Area cell %+v", id, got, want)
		}
	}
}

// colonyDrawShell answers the labels asked of the shell and records where
// each row's icon is drawn.
type colonyDrawShell struct {
	*fakeShell
	icons *[]gfx.Rect
}

func (colonyDrawShell) AnimalLabel(an game.AnimalView) string { return an.Species }
func (colonyDrawShell) AnimalHealth(game.AnimalView) string   { return "100%" }
func (c colonyDrawShell) DrawAnimalIcon(_ *ebiten.Image, _ game.AnimalView, x, y, size float64) {
	*c.icons = append(*c.icons, gfx.Rect{X: float32(x), Y: float32(y), W: float32(size), H: float32(size)})
}

// anchorsOf runs draw with the canvas deferring into a world pass whose fog
// callback records, and drops, the logical anchor of every draw (a text's
// own x and y, a fill's centre), so the test sees where each element would
// land without reading pixels (not possible headless). It must not run in
// parallel: the pass is canvas-global.
func anchorsOf(draw func(img *ebiten.Image)) [][2]float32 {
	var out [][2]float32
	img := ebiten.NewImage(1500, 720)
	p := &gfx.WorldPass{}
	p.Begin(img, func(lx, ly float32) bool { out = append(out, [2]float32{lx, ly}); return true })
	gfx.BeginWorldPass(p, 1)
	defer gfx.EndWorldPass()
	draw(img)
	return out
}

// drawnAt reports an anchor with x == x and y within [y0, y1).
func drawnAt(a [][2]float32, x, y0, y1 float32) bool {
	for _, p := range a {
		if p[0] == x && p[1] >= y0 && p[1] < y1 {
			return true
		}
	}
	return false
}

// drawColony draws every column of every row at its cell: the icon at the
// Icon cell (numeric literals from the pre-cell draw: x+16, row y, 26) and
// the Name, Species, Health and Food texts and the titles at their cells'
// x, at full width and with columns dropped. Dropping a draw call or
// moving it off its cell turns this red.
func TestDrawColonyOnTheCells(t *testing.T) { //nolint:paralleltest // canvas-global world pass
	for _, width := range []float32{1374, 900} {
		var icons []gfx.Rect
		fs, _ := newFake()
		w := New(colonyDrawShell{fs, &icons})
		fs.env.Game = fakeReader{}
		fs.world.Animals = []game.AnimalView{{ID: 1, Species: "dustbuck", Name: "Rex", Tame: true, Food: 10, State: "idle"}, {ID: 2, Species: "ox", Name: "Bo", Tame: true, Food: 90, State: "idle"}}
		win := gfx.Rect{X: 8, Y: 40, W: width, H: 380}
		a := anchorsOf(func(img *ebiten.Image) {
			w.Animals().drawColony(img, uikit.Button{X: win.X, Y: win.Y, W: win.W, H: win.H})
		})
		if len(icons) != 2 {
			t.Fatalf("%v px: %d icons drawn, want 2", width, len(icons))
		}
		for row := 0; row < 2; row++ {
			if want := (gfx.Rect{X: 24, Y: 100 + float32(row)*30, W: 26, H: 26}); icons[row] != want {
				t.Errorf("%v px row %d: icon %+v, want %+v", width, row, icons[row], want)
			}
			cells := colonyCells(nil, win, row)
			y0 := win.Y + animalsRowTop + float32(row*animalRowH)
			for _, id := range []string{cName, cSpecies, cHealth} {
				if c := colonyCell(cells, id); !drawnAt(a, c.X, y0, y0+animalRowH) {
					t.Errorf("%v px row %d: no text at the %s cell x %v", width, row, id, c.X)
				}
			}
			if c := colonyCell(cells, cFood); !drawnAt(a, c.X+c.W/2, y0, y0+animalRowH) {
				t.Errorf("%v px row %d: no food bar at the Food cell %+v", width, row, c)
			}
		}
		heads := colonyCells(nil, win, 0)
		for _, h := range colonyColumns(colonyEnglish{}) {
			c := colonyCell(heads, h.ID)
			if h.Title == "" || c.Empty() {
				continue
			}
			if !drawnAt(a, c.X+float32(colonyHeadInset(h.ID)), win.Y+animalsRowTop-20, win.Y+animalsRowTop) {
				t.Errorf("%v px: header %s not drawn at its cell x %v", width, h.ID, c.X)
			}
		}
	}
}

// The icon keeps the whole-pixel row y it had before the cells (the window's
// Y truncated, then the row offset), and the x of the window plus 16, on a
// window at a fractional position. Literals worked out from the pre-cell
// draw: y = int(50.75)+60+30*row.
func TestColonyIconKeepsWholePixelRow(t *testing.T) { //nolint:paralleltest // canvas-global world pass
	var icons []gfx.Rect
	fs, _ := newFake()
	w := New(colonyDrawShell{fs, &icons})
	fs.env.Game = fakeReader{}
	fs.world.Animals = []game.AnimalView{{ID: 1, Tame: true}, {ID: 2, Tame: true}}
	anchorsOf(func(img *ebiten.Image) {
		w.Animals().drawColony(img, uikit.Button{X: 8.5, Y: 50.75, W: 1374, H: 380})
	})
	want := []gfx.Rect{{X: 24.5, Y: 110, W: 26, H: 26}, {X: 24.5, Y: 140, W: 26, H: 26}}
	if len(icons) != 2 || icons[0] != want[0] || icons[1] != want[1] {
		t.Errorf("icons %+v, want %+v", icons, want)
	}
}
