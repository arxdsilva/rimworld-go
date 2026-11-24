// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Column-test rects (QA-002). A window's column test lays it out on each
// uikit.ColumnScreens screen through the production path. The fake shell's
// own WindowRect and Size stay fixed (8,300 900x380 on 1280x720): a draw
// path under test reads the rect it is given, not the shell.

// windowsMainTabRect opens a main-tab window on a 3-colonist colony (the
// fit_test.go recipe) and returns it with the hudlayout Window rect it gets
// on an sw x sh logical screen; Research spans the screen (UIS-065, as
// colony_compose.go sets FullWidthWindow).
func windowsMainTabRect(t testing.TB, open func(*Set), sw, sh int) (*Set, gfx.Rect) {
	t.Helper()
	fs, w := newFake()
	fs.env.Game = fakeReader{}
	for i := 1; i <= 3; i++ {
		fs.pawns = append(fs.pawns, game.PawnView{ID: entity.EntityID(i), Name: "P", Status: "alive"})
	}
	open(w)
	ww, wh := w.Want()
	if ww <= 0 || wh <= 0 {
		t.Fatalf("the window %q wants nothing: %vx%v", w.Mode, ww, wh)
	}
	l := hudlayout.Compute(sw, sh, hudlayout.Inputs{Colonists: 3, WindowW: ww, WindowH: wh, FullWidthWindow: w.Open && w.Mode == "research"})
	return w, l.Window
}

// windowsKitBody opens a kit window (History, Assign, Factions…) and
// returns its hudlayout Window rect and the body its Draw is given there:
// the open kit Modal (Set.modal, stored by OpenKit) anchored on win, as
// Set.Anchor -> WindowRect -> hud().Window anchors it in production,
// through uikit.ModalLayout, the path RunWindow takes.
func windowsKitBody(t testing.TB, open func(*Set), sw, sh int) (win, body gfx.Rect) {
	t.Helper()
	w, win := windowsMainTabRect(t, open, sw, sh)
	if !w.KitOpen() {
		t.Fatalf("the window %q is not a kit window", w.Mode)
	}
	m := *w.modal
	m.Anchor = func() gfx.Rect { return win }
	_, _, body, _ = uikit.ModalLayout(sw, sh, m)
	return win, body
}

// TestWindowsKitBodyIsTheAnchoredInset: Assign (frameless, no footer) gets
// its Window rect inset by the window margin, at both column screens.
func TestWindowsKitBodyIsTheAnchoredInset(t *testing.T) {
	t.Parallel()
	for _, s := range uikit.ColumnScreens() {
		win, body := windowsKitBody(t, func(w *Set) { w.Assign().Launch() }, s.W, s.H)
		if win.Empty() {
			t.Fatalf("%dx%d: no Window rect", s.W, s.H)
		}
		want := gfx.Rect{X: win.X + hudlayout.WindowPad, Y: win.Y + hudlayout.WindowPad, W: win.W - 2*hudlayout.WindowPad, H: win.H - 2*hudlayout.WindowPad}
		if body != want {
			t.Errorf("%dx%d: body %+v, want %+v (Window %+v)", s.W, s.H, body, want, win)
		}
	}
}

// windowsMainTabRect gives Research the full screen width (UIS-065).
func TestWindowsMainTabRectResearchSpansTheScreen(t *testing.T) {
	t.Parallel()
	for _, s := range uikit.ColumnScreens() {
		_, win := windowsMainTabRect(t, func(w *Set) { w.Show("research") }, s.W, s.H)
		_, work := windowsMainTabRect(t, func(w *Set) { w.Show("work") }, s.W, s.H)
		if win.W <= work.W {
			t.Errorf("%dx%d: Research %v wide, Work %v", s.W, s.H, win.W, work.W)
		}
	}
}
