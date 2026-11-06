package windows

import (
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// The research window's info panel (RSCH-2, wiki: Research, the left
// pane): the selected project's name, description, "X / Y" progress,
// prerequisites (each a link that jumps to it), hidden prerequisites as
// text, the bench and facility it needs (red while missing), what it
// unlocks, Start research and, in developer mode, Finish now. Its lines are
// made once per selection or research-list change and drawn from the cache.

// researchPanel is the panel's cache and scroll.
type researchPanel struct {
	dirty bool
	w     float32 // the body width the rows were made for
	bench bool    // hasBench as the rows were made
	rows  []panelRow
	h     float32 // the rows' height
	off   float32 // the body's scroll

	startLabel, startReason string
	startOff, startShown    bool
	finishReason            string
	finishOff               bool
}

// panelRow is one line of the panel; a link row jumps to a prerequisite.
type panelRow struct {
	text  string
	st    gfx.TextStyle
	y, h  float32
	link  int // the prerequisite's node (-1: plain text)
	id    string
	click func()
}

// Panel colours: missing bench or facility, a finished prerequisite.
func panelRed() color.RGBA  { return uikit.Danger }
func panelDone() color.RGBA { return uikit.Good }

// researchETA is the time left at today's pace in words.
func researchETA(tr lang.Tr, hours int) string {
	if hours < 48 {
		return lang.Tf(tr, "About %d hours to go at today's pace", hours)
	}
	return lang.Tf(tr, "About %d days to go at today's pace", (hours+12)/24)
}

// benchParts splits the bench tier in words into the bench and the
// facility ("hi-tech research bench + multi-analyzer").
func benchParts(v game.ResearchView) (bench, facility string) {
	s := v.Bench
	if s == "" {
		s = research.SimpleBench.String()
	}
	bench, facility, _ = strings.Cut(s, " + ")
	return bench, facility
}

// upperFirst is s with its first letter upper-cased ("needs a ..." reads
// as a sentence).
func upperFirst(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}

// needsFirst is why a locked project cannot start.
func needsFirst(tr lang.Tr, v game.ResearchView) string {
	switch {
	case len(v.Requires) > 0:
		return lang.Tf(tr, "Needs %s first", strings.Join(v.Requires, ", "))
	case len(v.Hidden) > 0:
		return lang.Tf(tr, "Needs %s first", strings.Join(v.Hidden, ", "))
	}
	return tr.T("Not available yet")
}

// buildPanel remakes the panel's lines for the selected project at body
// width w.
func (st *researchState) buildPanel(w float32) {
	tr := st.tr
	p := &st.panel
	p.dirty, p.w, p.bench = false, w, st.hasBench
	p.rows, p.h = p.rows[:0], 0
	add := func(s string, ts gfx.TextStyle, c color.Color) {
		ts.Color = c
		for _, line := range gfx.WrapPx(s, ts, float64(w)) {
			h := gfx.LineStep(ts)
			p.rows = append(p.rows, panelRow{text: line, st: ts, y: p.h, h: h, link: -1})
			p.h += h
		}
	}
	gap := func(h float32) { p.h += h }
	p.startShown = st.sel >= 0 && st.sel < len(st.nodes)
	if !p.startShown {
		add(tr.T("Pick a project in the tree to see what it needs and unlocks."), gfx.Small, uikit.TextDim)
		return
	}
	n := &st.nodes[st.sel]
	v := n.v
	add(v.Label, gfx.Large, uikit.Heading)
	add(research.TechLevel(n.level).String(), gfx.Small, uikit.TextDim)
	gap(4)
	add(v.Desc, gfx.Small, uikit.Text)
	gap(8)
	prog := lang.Tf(tr, "Progress: %d / %d", researchDone(v), researchCost(v))
	if !v.Done {
		prog = lang.Tf(tr, "%s (%d%%)", prog, researchPct(v))
	}
	add(prog, gfx.Body, uikit.Text)
	if v.ETAHours > 0 && !v.Done {
		add(researchETA(tr, v.ETAHours), gfx.Small, uikit.TextDim)
	}
	gap(8)
	add(tr.T("Prerequisites"), gfx.Label, uikit.Heading)
	if len(n.pre) == 0 {
		add(tr.T("None"), gfx.Small, uikit.TextDim)
	}
	for _, j := range n.pre {
		pv := st.nodes[j].v
		ts, word := gfx.WithColor(gfx.Body, uikit.Text), ""
		if pv.Done {
			ts.Color, word = panelDone(), tr.T(" (finished)")
		}
		h := gfx.LineStep(ts) + 2
		jj := j
		p.rows = append(p.rows, panelRow{text: gfx.Ellipsize("• "+pv.Label+word, ts, float64(w)), st: ts, y: p.h, h: h, link: j,
			id: researchLinkPfx + strconv.Itoa(int(pv.ID)), click: func() { st.jump(jj) }})
		p.h += h
	}
	if len(v.Hidden) > 0 {
		add(lang.Tf(tr, "Also needs: %s", strings.Join(v.Hidden, ", ")), gfx.Small, uikit.TextDim)
	}
	gap(8)
	bench, facility := benchParts(v)
	analyzer := v.BenchMissing == rlab.MissingAnalyzer
	c := color.Color(uikit.Text)
	if v.BenchMissing == rlab.MissingBench || v.BenchMissing == rlab.MissingHiTech {
		c = panelRed()
	}
	add(lang.Tf(tr, "Bench: %s", bench), gfx.Body, c)
	if facility != "" {
		c = uikit.Text
		if analyzer {
			c = panelRed()
		}
		add(lang.Tf(tr, "Facility: %s", facility), gfx.Body, c)
	}
	if v.BenchMissing != "" && !v.Done {
		add(upperFirst(v.BenchMissing)+".", gfx.Small, panelRed())
	}
	gap(8)
	add(tr.T("Unlocks"), gfx.Label, uikit.Heading)
	if len(v.Unlocks) == 0 {
		add(tr.T("Nothing new to build"), gfx.Small, uikit.TextDim)
	} else {
		add(strings.Join(v.Unlocks, ", "), gfx.Small, uikit.Text)
	}
	p.startLabel, p.startReason, p.startOff = tr.T("Start research"), "", false
	p.finishReason, p.finishOff = "", false
	switch {
	case v.Done:
		p.startLabel, p.startReason, p.startOff = tr.T("Finished"), lang.Tf(tr, "%s is already researched", v.Label), true
		p.finishReason, p.finishOff = p.startReason, true
	case v.Current:
		p.startLabel, p.startReason, p.startOff = tr.T("Researching"), lang.Tf(tr, "%s is the project under way", v.Label), true
	case !v.Available:
		p.startReason, p.startOff = needsFirst(tr, v), true
		p.finishReason, p.finishOff = p.startReason, true
	}
}

// start starts the selected project (Start research).
func (w Research) start() {
	st := w.rs()
	if st.sel < 0 || st.sel >= len(st.nodes) {
		return
	}
	if v := st.nodes[st.sel].v; v.Available && !v.Done && !v.Current {
		w.sh.Enqueue(command.Command{Kind: command.SetResearch, Build: v.ID})
	}
}

// finish finishes the selected project at once (developer mode): it is
// made the current project first when it is not.
func (w Research) finish() {
	st := w.rs()
	if st.sel < 0 || st.sel >= len(st.nodes) {
		return
	}
	n := st.nodes[st.sel]
	if n.v.Done || !n.v.Available && !n.v.Current {
		return
	}
	if !n.v.Current {
		w.sh.Enqueue(command.Command{Kind: command.SetResearch, Build: n.id})
	}
	w.sh.Enqueue(command.Command{Kind: command.DevFinishResearch, Build: n.id})
}

// declarePanel declares the panel's widgets: the body's wheel, the
// visible prerequisite links, Start research and Finish now.
func (w Research) declarePanel(st *researchState, g researchGeom, dev bool, fr *inputframe.Frame, mx, my int) {
	k := w.kit()
	p := &st.panel
	if p.dirty || p.w != g.body.W || p.bench != st.hasBench {
		st.buildPanel(g.body.W)
	}
	k.Add(uikit.Widget{ID: researchInfoID, R: g.body, Wheel: true})
	if len(k.Modals) == 0 && g.body.Hit(mx, my) && fr.Raw.WheelY != 0 {
		p.off -= float32(fr.Raw.WheelY) * rtWheel / 2
		fr.Raw.WheelY = 0
	}
	p.off = max(0, min(p.off, p.h-g.body.H))
	for i := range p.rows {
		row := &p.rows[i]
		if r, ok := panelRowRect(g.body, p.off, row); ok && row.link >= 0 {
			k.Add(uikit.Widget{ID: row.id, R: r, Label: st.nodes[row.link].v.Label, Focusable: true, Click: row.click})
		}
	}
	if st.startFn == nil {
		st.startFn, st.finishFn = w.start, w.finish
	}
	if p.startShown && !g.start.Empty() {
		k.Add(uikit.Widget{ID: researchStartID, R: g.start, Label: p.startLabel, Focusable: true, Disabled: p.startOff, Reason: p.startReason,
			Click: st.startFn})
		if k.KeyAct(researchStartID) {
			st.startFn()
		}
	}
	if dev && p.startShown && !g.finish.Empty() {
		k.Add(uikit.Widget{ID: researchFinishID, R: g.finish, Label: "Debug: finish now", Focusable: true, Disabled: p.finishOff,
			Reason: p.finishReason, Click: st.finishFn})
	}
}

// panelRowRect is the row's rectangle in the body at scroll off, and
// whether it is wholly inside (only whole rows are drawn or clickable).
func panelRowRect(body gfx.Rect, off float32, row *panelRow) (gfx.Rect, bool) {
	r := gfx.Rect{X: body.X, Y: body.Y - off + row.y, W: body.W, H: row.h}
	return r, r.Y >= body.Y-0.5 && r.Bottom() <= body.Bottom()+0.5
}

// drawPanel draws the info panel from its cache.
func (w Research) drawPanel(dst *ebiten.Image, st *researchState, g researchGeom, dev bool, mx, my int) {
	pr := g.panel
	if pr.Empty() {
		return
	}
	gfx.FillRect(dst, pr.X, pr.Y, pr.W, pr.H, uikit.LerpRGBA(uikit.Field, uikit.Panel, 0.5), false)
	gfx.StrokeRect(dst, pr.X, pr.Y, pr.W, pr.H, 1, uikit.PanelBorder, false)
	p := &st.panel
	for i := range p.rows {
		row := &p.rows[i]
		r, ok := panelRowRect(g.body, p.off, row)
		if !ok {
			continue
		}
		ts := row.st
		if row.link >= 0 && r.Hit(mx, my) {
			gfx.FillRect(dst, r.X, r.Y, r.W, r.H, uikit.Fade(uikit.Accent, 0x30), false)
			ts.Color = uikit.Heading
		}
		gfx.Text(dst, row.text, float64(r.X), float64(r.Y), ts)
	}
	if p.h > g.body.H && g.body.H > 0 { // the body scrolls: a thin bar on its right
		h := max(12, g.body.H*g.body.H/p.h)
		y := g.body.Y + (g.body.H-h)*p.off/(p.h-g.body.H)
		gfx.FillRect(dst, pr.Right()-4, y, 2, h, uikit.ScrollThumb, false)
	}
	if !p.startShown {
		return
	}
	b := uikit.Button{ID: researchStartID, X: g.start.X, Y: g.start.Y, W: g.start.W, H: g.start.H, Label: p.startLabel,
		Disabled: p.startOff, Selected: !p.startOff, Quiet: true}
	uikit.DrawButton(dst, b, b.Contains(mx, my) && !b.Disabled)
	if dev && !g.finish.Empty() {
		f := uikit.Button{ID: researchFinishID, X: g.finish.X, Y: g.finish.Y, W: g.finish.W, H: g.finish.H, Label: "Debug: finish now",
			Disabled: p.finishOff, Quiet: true}
		uikit.DrawButton(dst, f, f.Contains(mx, my) && !f.Disabled)
	}
}
