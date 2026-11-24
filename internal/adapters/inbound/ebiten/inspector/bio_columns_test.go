// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inspector

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/skill"
)

const nSkills = int(skill.NumSkills)

type enT struct{}

func (enT) T(s string) string { return s }

// bioCols is the declared list in English.
func bioCols() []uikit.TableCol { c := bioColumns(enT{}); return c[:] }

// TestBioSkillCellsKeepTheirAnchors pins the skill table's x anchors to the
// numbers drawn before the cells existed (QA-002): name at x+22, passion at
// x+108, level right edge at x+160, bar at x+164 of width W-216, This week
// ending 6 px short of the row.
func TestBioSkillCellsKeepTheirAnchors(t *testing.T) {
	row := gfx.Rect{X: 100, Y: 10, W: 490, H: 16}
	got := bioSkillCells(nil, row)
	want := []uikit.TableCell{
		{Col: "skill", R: gfx.Rect{X: 122, Y: 10, W: 72, H: 16}},
		{Col: "passion", R: gfx.Rect{X: 208, Y: 10, W: 12, H: 16}},
		{Col: "level", R: gfx.Rect{X: 236, Y: 10, W: 24, H: 16}},
		{Col: "xp", R: gfx.Rect{X: 264, Y: 10, W: 274, H: 16}},
		{Col: "week", R: gfx.Rect{X: 538, Y: 10, W: 46, H: 16}},
	}
	if len(got) != len(want) {
		t.Fatalf("%d cells, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if ids := uikit.TableColIDs(bioCols()); len(ids) != 5 || ids[0] != "skill" || ids[4] != "week" {
		t.Errorf("declared ids = %v", ids)
	}
}

// TestBioSkillsRectKeepsTheCut pins the table rect and row height formula.
func TestBioSkillsRectKeepsTheCut(t *testing.T) {
	in := gfx.Rect{X: 10, Y: 20, W: 400, H: 460}
	r, rowH := bioSkillsRect(in, 120, 18, 35, nSkills)
	// avail = 480-18-35-120-20 = 287; /12 = 23.9 -> clamped to bioRowMax 18
	if rowH != 18 || r != (gfx.Rect{X: 10, Y: 120, W: 400, H: 20 + 18*float32(nSkills)}) {
		t.Errorf("rect %+v rowH %v", r, rowH)
	}
	r, rowH = bioSkillsRect(in, 300, 18, 35, nSkills)
	// room is 127: the table scrolls, rows at bioRowMin
	if rowH != 14 || r.H != 127 {
		t.Errorf("tight: rect %+v rowH %v", r, rowH)
	}
	if _, rowH = bioSkillsRect(in, 120, 0, 0, 0); rowH != 18 {
		t.Errorf("no skills: rowH %v", rowH)
	}
}

// TestBioSkillColumnsDrawn lays the skills table out where production puts
// the Bio tab, at both screens, and checks every declared column has a cell
// in the header and in every visible row, an incapable row's included (its
// cells are geometry only: "incapable" is drawn in the Experience cell).
func TestBioSkillColumnsDrawn(t *testing.T) {
	declared := uikit.TableColIDs(bioCols())
	for _, sc := range uikit.ColumnScreens() {
		win := bioTabRect(t, sc)
		if win.Empty() {
			t.Fatalf("%dx%d: no tab panel", sc.W, sc.H)
		}
		r, rowH := bioSkillsRect(win, win.Y+60, 18, bioRelationsH(2), nSkills)
		hdr := gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: bioHdrH}
		if g := uikit.ColumnGaps(win, declared, nil, bioSkillCells(nil, hdr)); g != nil {
			t.Errorf("%dx%d header: %v", sc.W, sc.H, g)
		}
		rows := 0
		for i := 0; i < nSkills; i++ {
			row := gfx.Rect{X: r.X, Y: r.Y + bioHdrH + float32(i)*rowH, W: r.W, H: rowH}
			if row.Bottom() > r.Bottom()+0.01 {
				break // scrolled out; the scroll area clips it
			}
			rows++
			if g := uikit.ColumnGaps(win, declared, nil, bioSkillCells(nil, row)); g != nil {
				t.Errorf("%dx%d row %d: %v", sc.W, sc.H, i, g)
			}
		}
		if rows == 0 {
			t.Errorf("%dx%d: no row fits the table %+v", sc.W, sc.H, r)
		}
	}
}

// bioTabRect is the Bio tab's inner rect as production lays it out: the
// hud() inputs of a selected colonist with the default docked card pane
// (config HUD Pane: Height 165, TabW 96 x 6 tabs -> PaneW 576).
func bioTabRect(t testing.TB, sc uikit.ColumnScreen) gfx.Rect {
	t.Helper()
	bw, bh, ok := hudlayout.PaneTabSize("bio")
	if !ok {
		t.Fatal("no bio tab size")
	}
	l := hudlayout.Compute(sc.W, sc.H, hudlayout.Inputs{Colonists: 3, PaneShown: true, Commands: 12,
		PaneW: 576, PaneH: 165, ExpandedW: bw, ExpandedH: bh})
	return l.Expanded.Inset(hudlayout.PaneInset)
}

// TestBioTabRectIsProduction pins the test rect to the Expanded panel of
// production at both screens (probed through hud(): {580,245,460x489} at
// 1280x768, {0,1442,514x489} at 3840x2160), inset by 12.
func TestBioTabRectIsProduction(t *testing.T) {
	want := map[bool]gfx.Rect{
		true:  {X: 592, Y: 257, W: 436, H: 465},
		false: {X: 12, Y: 1454, W: 490, H: 465},
	}
	for _, sc := range uikit.ColumnScreens() {
		if got := bioTabRect(t, sc); got != want[sc.Small] {
			t.Errorf("%dx%d: %+v, want %+v", sc.W, sc.H, got, want[sc.Small])
		}
	}
}

// TestBioHeaderRowFollowsTheScrolledView pins the header to the rows' view:
// with a scrollbar (12 px) the This week edge ends 6 px short of the view,
// as before the cells existed.
func TestBioHeaderRowFollowsTheScrolledView(t *testing.T) {
	r := gfx.Rect{X: 100, Y: 10, W: 490, H: 120}
	view := gfx.Rect{X: 100, Y: 30, W: 478, H: 100}
	hr := bioHeaderRow(r, view)
	if hr != (gfx.Rect{X: 100, Y: 10, W: 478, H: 20}) {
		t.Fatalf("header row %+v", hr)
	}
	cells := bioSkillCells(nil, hr)
	if got := cells[4].R.Right(); got != 100+478-6 {
		t.Errorf("This week right edge %v, want 572", got)
	}
	if got := cells[3].R; got.X != 264 || got.W != 262 {
		t.Errorf("Experience cell %+v, want X 264 W 262", got)
	}
}

// bioDrawShell adds the one shell call the skills table draw makes.
type bioDrawShell struct{ *fakeShell }

func (bioDrawShell) Skill(s game.SkillView) (string, string) { return "L" + s.Label, "tip" }

// anchorsOf runs draw with the canvas deferring into a world pass whose fog
// callback records, and drops, the logical anchor of every draw (a text's
// own x and y, a fill's centre), so the test sees where each element lands
// without reading pixels. The pass is canvas-global: callers do not run in
// parallel.
func anchorsOf(draw func(img *ebiten.Image)) [][2]float32 {
	var out [][2]float32
	img := ebiten.NewImage(4000, 2200)
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

// TestBioSkillsDrawOnTheCells draws the real table at both screens, with a
// scrolling table too, and checks the header titles and each row's name,
// passion, level, bar and This week land at their cell's x (name, passion,
// bar start or centre, right edges), and an incapable row writes
// "incapable" at the Experience cell. A draw shifted off its cell turns
// this red.
func TestBioSkillsDrawOnTheCells(t *testing.T) { //nolint:paralleltest // canvas-global world pass
	skills := []game.SkillView{
		{Key: "shooting", Label: "Shooting", Passion: 1, XPPct: 40},
		{Key: "melee", Label: "Melee", Passion: 2, XPPct: 90},
		{Key: "cooking", Label: "Cooking", Incapable: true},
	}
	var week [skill.NumSkills]int
	week[0], week[1] = 5, 12
	for _, scroll := range []bool{false, true} {
		for _, sc := range uikit.ColumnScreens() {
			win := bioTabRect(t, sc)
			r, rowH := bioSkillsRect(win, win.Y+60, 18, bioRelationsH(2), len(skills))
			if scroll {
				r.H = bioHdrH + 2*rowH // two rows of three fit: the table scrolls
			}
			f := newFake()
			f.pawns = []game.PawnView{{ID: 1}}
			tabs := New(bioDrawShell{f}).Tabs()
			f.frameOnce()
			b := game.BioView{Skills: skills}
			view := gfx.Rect{X: r.X, Y: r.Y + bioHdrH, W: r.W, H: r.H - bioHdrH}
			if scroll {
				view.W -= uikit.ScrollBarW
			}
			a := anchorsOf(func(img *ebiten.Image) {
				tabs.bioSkills(r, b, week, rowH) // the header (the rows draw into a clipped copy)
				f.Kit().Paint(img)
				for i, s := range skills {
					tabs.bioRow(img, gfx.Rect{X: view.X, Y: view.Y + float32(i)*rowH, W: view.W, H: rowH}, i, s, week[i])
				}
			})
			name := func(what string, x, y0, y1 float32) {
				t.Helper()
				if !drawnAt(a, x, y0, y1) {
					t.Errorf("%dx%d scroll=%v: %s not drawn at x %v in y %v..%v", sc.W, sc.H, scroll, what, x, y0, y1)
				}
			}
			hc := bioSkillCells(nil, bioHeaderRow(r, view))
			name("Skill title", hc[0].R.X-14, r.Y, r.Y+bioHdrH)
			name("Passion title", hc[1].R.X-14, r.Y, r.Y+bioHdrH)
			name("Lvl title", hc[2].R.Right(), r.Y, r.Y+bioHdrH)
			name("Experience title", hc[3].R.X, r.Y, r.Y+bioHdrH)
			name("This week title", hc[4].R.Right(), r.Y, r.Y+bioHdrH)
			for i, s := range skills {
				row := gfx.Rect{X: view.X, Y: view.Y + float32(i)*rowH, W: view.W, H: rowH}
				if row.Y >= view.Bottom() {
					continue // scrolled out of view
				}
				c := bioSkillCells(nil, row)
				y0, y1 := row.Y, row.Bottom()
				name(s.Label+" name", c[0].R.X, y0, y1)
				name(s.Label+" level", c[2].R.Right(), y0, y1)
				if s.Incapable {
					name("incapable", c[3].R.X, y0, y1)
					continue
				}
				name(s.Label+" passion", c[1].R.X, y0, y1)
				name(s.Label+" bar", c[3].R.X+c[3].R.W/2, y0, y1)
				name(s.Label+" week", c[4].R.Right(), y0, y1)
			}
		}
	}
}
