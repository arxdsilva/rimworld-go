// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package plugins

import (
	"slices"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Widget ids. A row's checkbox is OnID.N and its list row ListID.N, N
// being the row index (manifest ids may be blank or repeated).
const (
	ListID     = "plugins.list"
	OnID       = "plugins.on"
	BackID     = "plugins.back"
	ProblemsID = "plugins.problems"
)

// Layout, in logical pixels.
const (
	margin   = 40
	top      = 112 // the content's top, under the title and the banner
	checkW   = 28  // the checkbox column left of the list
	gap      = 16
	footerH  = 36
	footerUp = 56 // the footer buttons' top, from the bottom edge
	pad      = 10
)

// Screen is the Plugin Manager and the problems view over one shell.
type Screen struct {
	sh       Shell
	sel      int  // the selected plugin row
	probSel  int  // the selected problem
	overList bool // the problems view sits on the list (Show plugin goes back)
}

// New is the Plugins screens over their shell.
func New(sh Shell) *Screen { return &Screen{sh: sh} }

// Declare declares the screen's widgets (once per Update).
func (s *Screen) Declare(scr ports.Screen, sw, sh int) {
	if scr == ports.ScreenPluginProblems {
		s.declareProblems(sw, sh)
		return
	}
	s.declareList(sw, sh)
}

// Draw paints the backdrop and the title; the kit paints the rest.
func (s *Screen) Draw(dst *ebiten.Image, scr ports.Screen, sw, sh int) {
	title := s.sh.T("Plugins")
	if scr == ports.ScreenPluginProblems {
		title = s.sh.T("Plugin problems")
	}
	s.sh.PaintMenu(dst, sw, sh, title, nil, nil)
}

// enabledNow is the saved enabled set as a lookup.
func (s *Screen) enabledNow() map[string]bool {
	on := map[string]bool{}
	for _, id := range s.sh.Settings().Plugins.Enabled {
		on[id] = true
	}
	return on
}

// setEnabled turns plugin id on or off in the saved settings.
func (s *Screen) setEnabled(id string, on bool) {
	st := s.sh.Settings()
	st.Plugins.Enabled = Toggle(st.Plugins.Enabled, id, on)
	s.sh.SetSettings(st)
}

// banner declares the restart notice while the saved set differs from the
// one the run started with.
func (s *Screen) banner(v View) {
	if !Pending(s.sh.Settings().Plugins.Enabled, v.Enabled) {
		return
	}
	s.sh.Kit().Label(margin+16, 86, s.sh.T("Changes take effect the next time the game starts."),
		gfx.WithColor(gfx.Body, uikit.Warning))
}

// footer declares Back and the screen's other footer buttons; it returns
// the next free x.
func (s *Screen) footer(sh int, backID string) float32 {
	k := s.sh.Kit()
	y := float32(sh - footerUp)
	if k.Button(backID, gfx.Rect{X: margin, Y: y, W: 140, H: footerH}, s.sh.T("Back"), uikit.BtnOpts{}) {
		s.sh.Do(ports.Back{})
	}
	return margin + 140 + gap
}

// rowH is a list row: two lines of body text.
func rowH() float32 { return float32(2*gfx.LineH(gfx.Body)) + 10 }

// declareList is the Plugin Manager: the list on the left with its
// checkboxes, the selected plugin's detail on the right.
func (s *Screen) declareList(sw, sh int) {
	v := s.sh.Plugins()
	k := s.sh.Kit()
	s.overList = false
	s.banner(v)
	x := s.footer(sh, BackID)
	n := len(v.Problems)
	label := s.sh.Tf("Problems (%d)", n)
	if k.Button(ProblemsID, gfx.Rect{X: x, Y: float32(sh - footerUp), W: 160, H: footerH}, label,
		uikit.BtnOpts{Disabled: n == 0, Reason: s.sh.T("Nothing went wrong while loading plugins.")}) {
		s.overList = true
		s.sh.Do(ports.Push{Screen: ports.ScreenPluginProblems})
	}
	body := gfx.Rect{X: margin, Y: top, W: float32(sw - 2*margin), H: float32(sh-footerUp-gap) - top}
	if len(v.Rows) == 0 {
		s.declareEmpty(body, v)
		return
	}
	s.sel = max(0, min(s.sel, len(v.Rows)-1))
	listW := min(520, body.W*0.45)
	list := gfx.Rect{X: body.X + checkW, Y: body.Y, W: listW, H: body.H}
	k.Panel(gfx.Rect{X: body.X, Y: body.Y, W: checkW + listW, H: body.H}) // under the boxes and the rows
	s.declareChecks(v, list)
	on := s.enabledNow()
	k.List(ListID, list, len(v.Rows), &s.sel, uikit.ListSpec{RowH: rowH(), Draw: func(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool) {
		s.drawRow(dst, v, i, row, hover, selected, on)
	}})
	detail := gfx.Rect{X: list.Right() + gap, Y: body.Y, W: body.Right() - list.Right() - gap, H: body.H}
	s.declareDetail(v, v.Rows[s.sel], detail, on)
}

// declareChecks declares the checkbox of every list row fully in view,
// beside the list so the two never share a press.
func (s *Screen) declareChecks(v View, list gfx.Rect) {
	k := s.sh.Kit()
	h := rowH()
	off := k.Scroll[ListID]
	first, last := uikit.ListVisible(off, list.H, h, len(v.Rows))
	on := s.enabledNow()
	for i := first; i <= last; i++ {
		y := list.Y + float32(i)*h - off
		if y < list.Y || y+h > list.Bottom() {
			continue
		}
		r := v.Rows[i]
		was := on[r.ID]
		val := was
		opts := uikit.CheckOpts{Tip: s.sh.T("On\nLoad this plugin the next time the game starts.")}
		if r.ID == "" || r.ID == Core {
			opts.Disabled, opts.Reason = true, s.sh.T("This plugin has no usable id, so it can't be turned on.")
		}
		box := gfx.Rect{X: list.X - checkW + 6, Y: y, W: checkW - 6, H: h}
		if k.Check(uikit.OptID(OnID, i), box, "", &val, opts) {
			s.setEnabled(r.ID, !was)
		}
	}
}

// stateText is a row's state as the list says it.
func (s *Screen) stateText(v View, r Row, on map[string]bool) (string, gfx.TextStyle) {
	started := slices.Contains(v.Enabled, r.ID)
	switch StateOf(r, on[r.ID], started) {
	case StateLoaded:
		return s.sh.T("loaded"), gfx.WithColor(gfx.Small, uikit.Good)
	case StateFailed:
		if msg := v.FirstError(r.ID); msg != "" {
			return s.sh.Tf("not loaded: %s", msg), gfx.WithColor(gfx.Small, uikit.Danger)
		}
		return s.sh.T("not loaded"), gfx.WithColor(gfx.Small, uikit.Danger)
	case StateOnAtRestart:
		return s.sh.T("on after restart"), gfx.WithColor(gfx.Small, uikit.Warning)
	case StateOffAtRestart:
		return s.sh.T("off after restart"), gfx.WithColor(gfx.Small, uikit.Warning)
	}
	return s.sh.T("off"), gfx.WithColor(gfx.Small, uikit.TextDim)
}

// countsText is a plugin's problem counts ("" when it has none).
func (s *Screen) countsText(v View, id string) string {
	errs, warns := v.Counts(id)
	var parts []string
	switch {
	case errs == 1:
		parts = append(parts, s.sh.T("1 error"))
	case errs > 1:
		parts = append(parts, s.sh.Tf("%d errors", errs))
	}
	switch {
	case warns == 1:
		parts = append(parts, s.sh.T("1 warning"))
	case warns > 1:
		parts = append(parts, s.sh.Tf("%d warnings", warns))
	}
	return strings.Join(parts, ", ")
}

// drawRow paints one list row: name and version, then author and id,
// with the state and problem counts on the right.
func (s *Screen) drawRow(dst *ebiten.Image, v View, i int, row gfx.Rect, hover, selected bool, on map[string]bool) {
	uikit.DrawListRow(dst, row, "", hover, selected, false)
	r := v.Rows[i]
	lh := gfx.LineH(gfx.Body)
	x, y := float64(row.X+8), float64(row.Y+5)
	state, st := s.stateText(v, r, on)
	natural, _ := gfx.Measure(state, st)
	stW := min(float64(row.W)*0.45, natural)
	right := float64(row.Right() - 8)
	name := r.Name
	if name == "" {
		name = r.ID
	}
	head := s.sh.Tf("%s  %s", name, r.Version)
	if r.Loaded {
		head = s.sh.Tf("#%d  %s  %s", r.Position, name, r.Version)
	}
	gfx.Text(dst, gfx.Ellipsize(head, gfx.Label, right-x-stW-12), x, y, gfx.Label)
	gfx.Text(dst, gfx.Ellipsize(state, st, stW), right-stW, y+2, st)
	sub := s.sh.Tf("%s · %s", strings.Join(r.Authors, ", "), r.ID)
	if len(r.Authors) == 0 {
		sub = r.ID
	}
	counts := s.countsText(v, r.ID)
	cw, _ := gfx.Measure(counts, gfx.Small)
	dim := gfx.WithColor(gfx.Body, uikit.TextDim)
	gfx.Text(dst, gfx.Ellipsize(sub, dim, right-x-cw-12), x, y+lh, dim)
	if counts != "" {
		gfx.Text(dst, counts, right-cw, y+lh+2, gfx.WithColor(gfx.Small, uikit.Warning))
	}
}

// listOr is the list joined, or none.
func (s *Screen) listOr(list []string) string {
	if len(list) == 0 {
		return s.sh.T("none")
	}
	return strings.Join(list, ", ")
}

// detailLines are the selected plugin's facts: what it is, what it needs,
// where it loads and why (§46), and its problems.
func (s *Screen) detailLines(v View, r Row, on map[string]bool) []line {
	dim := gfx.WithColor(gfx.Body, uikit.TextDim)
	out := []line{{text: r.Name, st: gfx.WithColor(gfx.Large, uikit.Heading)}}
	out = append(out, line{text: s.sh.Tf("%s · version %s · by %s", r.ID, r.Version, s.listOr(r.Authors)), st: dim})
	if r.Description != "" {
		out = append(out, line{text: r.Description, st: gfx.Body})
	}
	out = append(out, line{},
		line{text: s.sh.Tf("Requires: %s", s.listOr(r.Requires)), st: gfx.Body},
		line{text: s.sh.Tf("Loads after: %s", s.listOr(r.After)), st: gfx.Body},
		line{text: s.sh.Tf("Loads before: %s", s.listOr(r.Before)), st: gfx.Body},
		line{text: s.sh.Tf("Conflicts with: %s", s.listOr(r.Conflicts)), st: gfx.Body},
		line{text: s.sh.Tf("Translates: %s", s.listOr(r.Locales)), st: gfx.Body})
	if r.URL != "" {
		out = append(out, line{text: s.sh.Tf("Homepage: %s", r.URL), st: dim})
	}
	out = append(out, line{})
	state, st := s.stateText(v, r, on)
	if r.Loaded {
		out = append(out, line{text: s.sh.Tf("Position %d: %s", r.Position, r.Reason), st: gfx.Body})
	} else {
		out = append(out, line{text: s.sh.T("Not in this run's load order."), st: dim})
	}
	out = append(out, line{text: s.sh.Tf("State: %s", state), st: gfx.WithColor(gfx.Body, st.Color)})
	for _, p := range v.Problems {
		if p.Plugin != r.ID {
			continue
		}
		out = append(out, problemLine(s.sh, p))
	}
	return out
}

// line is one line of wrapped text in a style.
type line struct {
	text string
	st   gfx.TextStyle
}

// problemLine is one of a plugin's problems as a line: where in the
// plugin, then the message.
func problemLine(sh Shell, p Problem) line {
	msg := p.Msg
	if where := (Problem{File: p.File, Def: p.Def, Field: p.Field}).Where(); where != "" {
		msg = sh.Tf("%s: %s", where, p.Msg)
	}
	if p.Warn {
		return line{text: sh.Tf("Warning: %s", msg), st: gfx.WithColor(gfx.Body, uikit.Warning)}
	}
	return line{text: sh.Tf("Error: %s", msg), st: gfx.WithColor(gfx.Body, uikit.Danger)}
}

// declareDetail paints the selected plugin's detail, wrapped, cut at the
// panel's foot.
func (s *Screen) declareDetail(v View, r Row, box gfx.Rect, on map[string]bool) {
	if r.Name == "" {
		r.Name = r.ID
	}
	lines := s.detailLines(v, r, on)
	k := s.sh.Kit()
	k.Panel(box)
	paintLines(k, box, lines)
}

// paintLines queues wrapped lines inside box, stopping at its foot.
func paintLines(k *uikit.Kit, box gfx.Rect, lines []line) {
	k.Queue(func(dst *ebiten.Image) {
		x, y := float64(box.X+pad), float64(box.Y+pad)
		w := float64(box.W - 2*pad)
		for _, l := range lines {
			st := l.st
			if st.Size == 0 {
				st = gfx.Body
			}
			lh := gfx.LineH(st)
			if l.text == "" {
				y += lh / 2
				continue
			}
			for _, part := range gfx.WrapPx(l.text, st, w) {
				if y+lh > float64(box.Bottom()-pad) {
					return
				}
				gfx.Text(dst, part, x, y, st)
				y += lh
			}
		}
	})
}

// declareEmpty is the empty state: no plugin installed, and where to put
// one.
func (s *Screen) declareEmpty(box gfx.Rect, v View) {
	lines := []line{
		{text: s.sh.T("No plugins are installed."), st: gfx.WithColor(gfx.Large, uikit.Heading)},
		{},
		{text: s.sh.T("To add one, put its folder in one of these folders, then restart the game:"), st: gfx.Body},
	}
	for _, f := range v.Folders {
		lines = append(lines, line{text: f, st: gfx.WithColor(gfx.Body, uikit.TextDim)})
	}
	k := s.sh.Kit()
	k.Panel(box)
	paintLines(k, box, lines)
}
