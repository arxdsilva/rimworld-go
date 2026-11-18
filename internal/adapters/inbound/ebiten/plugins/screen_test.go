// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package plugins

import (
	"github.com/hajimehoshi/ebiten/v2"

	"fmt"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// fakeShell is the screens' shell: a view, the saved settings, the intents
// sent and a widget kit driven one frame at a time.
type fakeShell struct {
	view     View
	settings config.Settings
	saves    int
	did      []ports.Intent
	nav      func(ports.Intent) // the navigator, as the root applies intents
	kit      *uikit.Kit
	frame    inputframe.Frame
}

type kitHost struct{ f *fakeShell }

func (h kitHost) Frame() *inputframe.Frame { return &h.f.frame }
func (h kitHost) Cursor() (int, int)       { return h.f.frame.Raw.Mx, h.f.frame.Raw.My }
func (h kitHost) Size() (int, int)         { return 1280, 720 }
func (h kitHost) Updates() uint64          { return 0 }
func (h kitHost) KeyRepeat(k string) bool  { return h.f.frame.Key(k) }
func (h kitHost) Cue(string)               {}
func (h kitHost) Hint(string)              {}
func (h kitHost) ReducedMotion() bool      { return true }

func (f *fakeShell) Kit() *uikit.Kit {
	if f.kit == nil {
		f.frame = inputframe.NewFrame(inputframe.Raw{Mx: -50, My: -50})
		f.kit = uikit.New(kitHost{f})
	}
	return f.kit
}
func (f *fakeShell) Settings() config.Settings      { return f.settings }
func (f *fakeShell) KeyName(input.Action) string    { return "" }
func (f *fakeShell) KeyHint(input.Action) string    { return "" }
func (f *fakeShell) T(src string) string            { return src }
func (f *fakeShell) Tf(src string, a ...any) string { return fmt.Sprintf(src, a...) }
func (f *fakeShell) Do(it ports.Intent) {
	f.did = append(f.did, it)
	if f.nav != nil {
		f.nav(it)
	}
}
func (f *fakeShell) Plugins() View                                                       { return f.view }
func (f *fakeShell) SetSettings(s config.Settings)                                       { f.settings = s.Normalize(); f.saves++ }
func (f *fakeShell) PaintMenu(*ebiten.Image, int, int, string, []string, []uikit.Button) {}

// harness is one screen over a fake shell, on screen scr; Push and Back
// move scr as the root's navigator would (the main menu below).
type harness struct {
	f   *fakeShell
	s   *Screen
	scr ports.Screen
	was []ports.Screen
}

func newHarness(v View, enabled ...string) *harness {
	f := &fakeShell{view: v}
	f.settings.Plugins.Enabled = enabled
	h := &harness{f: f, s: New(f), scr: ports.ScreenPlugins}
	f.nav = func(it ports.Intent) {
		switch it := it.(type) {
		case ports.Push:
			h.was, h.scr = append(h.was, h.scr), it.Screen
		case ports.Back:
			h.scr = ports.ScreenMainMenu
			if n := len(h.was); n > 0 {
				h.was, h.scr = h.was[:n-1], h.was[n-1]
			}
		}
	}
	return h
}

// step runs one Update over raw: the kit's frame, then the declarations.
func (h *harness) step(raw inputframe.Raw) {
	k := h.f.Kit()
	h.f.frame = inputframe.NewFrame(raw)
	k.Begin()
	h.s.Declare(h.scr, 1280, 720)
}

func (h *harness) idle() { h.step(inputframe.Raw{Mx: -50, My: -50}) }

func (h *harness) widget(t *testing.T, id string) (uikit.Widget, bool) {
	t.Helper()
	h.idle()
	return h.f.Kit().Find(id)
}

// click presses and releases the left button on widget id.
func (h *harness) click(t *testing.T, id string) {
	t.Helper()
	w, ok := h.widget(t, id)
	if !ok {
		t.Fatalf("no widget %q on %s", id, h.scr)
	}
	x, y := w.R.Center()
	raw := inputframe.Raw{Mx: int(x), My: int(y)}
	raw.Press[inputframe.Left], raw.Down[inputframe.Left] = true, true
	h.step(raw)
	raw.Press[inputframe.Left], raw.Down[inputframe.Left], raw.Release[inputframe.Left] = false, false, true
	h.step(raw)
	h.idle()
}

// twoPlugins: beta loaded at 1 (after core), alpha installed and off.
func twoPlugins() View {
	return View{
		Rows: []Row{
			{ID: "beta", Name: "Beta", Version: "1.0.0", Authors: []string{"B"}, Loaded: true, Position: 1, Reason: "alphabetical"},
			{ID: "alpha", Name: "Alpha", Version: "0.2.0", Requires: []string{"beta"}},
		},
		Problems: []Problem{
			{Plugin: "beta", File: "defs/x.json", Def: "backstory/beta.x", Field: "title", Msg: "is empty"},
			{Plugin: "beta", Field: "game", Msg: "declares no supported game version", Warn: true},
			{Plugin: "gone", Msg: "enabled but not installed"},
		},
		Enabled: []string{"beta", "gone"},
		Folders: []string{"/mods", "/home/me/mods"},
	}
}

// Ticking a row's box turns its plugin on in the saved settings, as a
// sorted set, and the restart banner is due; unticking it again clears it.
func TestToggleSavesTheEnabledSet(t *testing.T) {
	t.Parallel()
	h := newHarness(twoPlugins(), "beta", "gone")
	h.click(t, uikit.OptID(OnID, 1)) // alpha
	if got := h.f.settings.Plugins.Enabled; !slices.Equal(got, []string{"alpha", "beta", "gone"}) || h.f.saves != 1 {
		t.Fatalf("after ticking alpha: enabled %v after %d saves", got, h.f.saves)
	}
	if !Pending(h.f.settings.Plugins.Enabled, h.f.view.Enabled) {
		t.Fatal("a changed set must wait for the next start")
	}
	h.click(t, uikit.OptID(OnID, 1))
	if got := h.f.settings.Plugins.Enabled; !slices.Equal(got, []string{"beta", "gone"}) {
		t.Fatalf("after unticking alpha: %v", got)
	}
	if Pending(h.f.settings.Plugins.Enabled, h.f.view.Enabled) {
		t.Fatal("the set the run started with needs no restart")
	}
	if len(h.f.did) != 0 {
		t.Fatalf("toggling sent %v; it must only save settings", h.f.did)
	}
}

// The list's states: loaded, failed with its first error, off, and the
// two that wait for a restart.
func TestStateOf(t *testing.T) {
	t.Parallel()
	loaded := Row{ID: "a", Loaded: true}
	idle := Row{ID: "b"}
	for _, c := range []struct {
		r           Row
		on, started bool
		want        State
	}{
		{loaded, true, true, StateLoaded},
		{idle, true, true, StateFailed},
		{idle, true, false, StateOnAtRestart},
		{loaded, false, true, StateOffAtRestart},
		{idle, false, true, StateOff},
		{idle, false, false, StateOff},
	} {
		if got := StateOf(c.r, c.on, c.started); got != c.want {
			t.Errorf("StateOf(%+v, on %v, started %v) = %d, want %d", c.r, c.on, c.started, got, c.want)
		}
	}
	s := New(&fakeShell{})
	v := View{Problems: []Problem{{Plugin: "b", Msg: "needs x", Warn: true}, {Plugin: "b", Msg: "conflicts with c"}}, Enabled: []string{"b"}}
	if got, _ := s.stateText(v, idle, map[string]bool{"b": true}); got != "not loaded: conflicts with c" {
		t.Errorf("failed row says %q", got)
	}
}

// Toggle never shares the caller's array, keeps ids no longer installed
// and leaves the set sorted.
func TestToggle(t *testing.T) {
	t.Parallel()
	set := make([]string, 2, 8)
	copy(set, []string{"gone", "zeta"})
	on := Toggle(set, "alpha", true)
	if !slices.Equal(on, []string{"alpha", "gone", "zeta"}) {
		t.Fatalf("on: %v", on)
	}
	if spare := set[:3]; spare[0] != "gone" || spare[2] != "" {
		t.Fatalf("Toggle wrote into the caller's array: %v", spare)
	}
	if off := Toggle([]string{"alpha", "gone"}, "alpha", false); !slices.Equal(off, []string{"gone"}) {
		t.Fatalf("off: %v", off)
	}
	if none := Toggle([]string{"alpha"}, "alpha", false); none != nil {
		t.Fatalf("an empty set must be nil, as settings keep it: %v", none)
	}
}

// Problem counts: errors raise the menu notice, warnings don't.
func TestViewCounts(t *testing.T) {
	t.Parallel()
	v := twoPlugins()
	if v.Errors() != 2 {
		t.Errorf("errors %d, want 2", v.Errors())
	}
	if e, w := v.Counts("beta"); e != 1 || w != 1 {
		t.Errorf("beta: %d errors, %d warnings", e, w)
	}
	if got := New(&fakeShell{}).countsText(v, "beta"); got != "1 error, 1 warning" {
		t.Errorf("beta's counts read %q", got)
	}
	if got := v.Problems[0].Where(); got != "beta › defs/x.json › backstory/beta.x › title" {
		t.Errorf("where %q", got)
	}
	if (View{Problems: []Problem{{Warn: true}}}).Errors() != 0 {
		t.Error("a warning counted as an error")
	}
}

// No plugin installed: no list, the folders are named, Problems is
// disabled while there are none, and Back goes back.
func TestEmptyState(t *testing.T) {
	t.Parallel()
	h := newHarness(View{Folders: []string{"/mods"}})
	if _, ok := h.widget(t, ListID); ok {
		t.Fatal("an empty install shows a list")
	}
	if w, ok := h.f.Kit().Find(ProblemsID); !ok || !w.Disabled {
		t.Fatalf("Problems with nothing to show: %+v", w)
	}
	h.click(t, BackID)
	if len(h.f.did) != 1 || h.f.did[0] != (ports.Back{}) {
		t.Fatalf("Back sent %v", h.f.did)
	}
}

// The problems view turns the selected problem's plugin off; it can't
// for a problem with no plugin, for core, or for a plugin already off.
func TestProblemsDisable(t *testing.T) {
	t.Parallel()
	h := newHarness(twoPlugins(), "beta", "gone")
	h.scr = ports.ScreenPluginProblems
	h.click(t, DisableID)
	if got := h.f.settings.Plugins.Enabled; !slices.Equal(got, []string{"gone"}) {
		t.Fatalf("Disable beta left %v", got)
	}
	if w, _ := h.widget(t, DisableID); !w.Disabled || w.Reason != "This plugin is already off." {
		t.Fatalf("Disable on a plugin already off: %+v", w)
	}
	for _, p := range []Problem{{Msg: "x"}, {Plugin: Core, Msg: "x"}} {
		h := newHarness(View{Problems: []Problem{p}}, "beta")
		h.scr = ports.ScreenPluginProblems
		if w, ok := h.widget(t, DisableID); !ok || !w.Disabled || w.Reason == "" {
			t.Errorf("Disable for %+v: %+v", p, w)
		}
	}
}

// Show plugin selects the problem's plugin: back to the list when the
// view sits on it, else the list is pushed (from the main menu's notice).
func TestShowPlugin(t *testing.T) {
	t.Parallel()
	h := newHarness(twoPlugins(), "beta")
	h.click(t, ProblemsID)
	if len(h.f.did) != 1 || h.f.did[0] != (ports.Push{Screen: ports.ScreenPluginProblems}) {
		t.Fatalf("Problems sent %v", h.f.did)
	}
	h.s.sel = 1
	h.click(t, InspectID)
	if h.f.did[1] != (ports.Back{}) || h.s.sel != 0 || h.scr != ports.ScreenPlugins {
		t.Fatalf("over the list: sent %v, selected %d", h.f.did, h.s.sel)
	}
	h2 := newHarness(twoPlugins(), "beta")
	h2.scr = ports.ScreenPluginProblems
	h2.click(t, InspectID)
	if len(h2.f.did) != 1 || h2.f.did[0] != (ports.Push{Screen: ports.ScreenPlugins}) {
		t.Fatalf("from the menu: sent %v", h2.f.did)
	}
	h2.scr, h2.s.probSel = ports.ScreenPluginProblems, 2 // "gone" is not installed
	if w, ok := h2.widget(t, InspectID); !ok || !w.Disabled {
		t.Fatal("Show plugin for a plugin not installed")
	}
}

// The detail explains the plugin's place in the load order (§46) and
// lists its problems.
func TestDetailExplainsThePosition(t *testing.T) {
	t.Parallel()
	s := New(&fakeShell{})
	v := twoPlugins()
	var texts []string
	for _, l := range s.detailLines(v, v.Rows[0], map[string]bool{"beta": true}) {
		texts = append(texts, l.text)
	}
	for _, want := range []string{"Position 1: alphabetical", "Error: defs/x.json › backstory/beta.x › title: is empty",
		"Warning: game: declares no supported game version", "Requires: none"} {
		if !slices.Contains(texts, want) {
			t.Errorf("detail lacks %q: %q", want, texts)
		}
	}
}

// The detail names the languages a plugin translates (PLG-055).
func TestDetailListsTranslatedLocales(t *testing.T) {
	t.Parallel()
	s := New(&fakeShell{})
	v := twoPlugins()
	for loc, want := range map[string][]string{"": {"Translates: none"}, "pt-BR, de": {"Translates: pt-BR, de"}} {
		r := v.Rows[0]
		if loc != "" {
			r.Locales = []string{"pt-BR", "de"}
		}
		var texts []string
		for _, l := range s.detailLines(v, r, nil) {
			texts = append(texts, l.text)
		}
		if !slices.Contains(texts, want[0]) {
			t.Errorf("detail lacks %q: %q", want[0], texts)
		}
	}
}
