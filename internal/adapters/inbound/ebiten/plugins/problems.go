package plugins

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The problems view's widget ids (§48). Copy is not offered: the client
// has no system clipboard yet.
const (
	ProblemListID = "plugins.problems.list"
	ProblemBackID = "plugins.problems.back"
	InspectID     = "plugins.problems.inspect"
	DisableID     = "plugins.problems.disable"
)

// declareProblems is the content problems view: every problem loading
// reported, with its provenance, and what the player can do about the
// selected one: look at its plugin, or turn the plugin off.
func (s *Screen) declareProblems(sw, sh int) {
	v := s.sh.Plugins()
	k := s.sh.Kit()
	s.banner(v)
	x := s.footer(sh, ProblemBackID)
	body := gfx.Rect{X: margin, Y: top, W: float32(sw - 2*margin), H: float32(sh-footerUp-gap) - top}
	if len(v.Problems) == 0 {
		k.Panel(body)
		paintLines(k, body, []line{{text: s.sh.T("Nothing went wrong while loading plugins."), st: gfx.Body}})
		return
	}
	s.probSel = max(0, min(s.probSel, len(v.Problems)-1))
	p := v.Problems[s.probSel]
	y := float32(sh - footerUp)
	row := v.RowOf(p.Plugin)
	inspect := uikit.BtnOpts{Disabled: row < 0, Reason: s.sh.T("This problem names no installed plugin.")}
	if k.Button(InspectID, gfx.Rect{X: x, Y: y, W: 160, H: footerH}, s.sh.T("Show plugin"), inspect) {
		s.sel = row
		if s.overList {
			s.sh.Do(ports.Back{})
		} else {
			s.sh.Do(ports.Push{Screen: ports.ScreenPlugins})
		}
	}
	name := p.Plugin
	if row >= 0 && v.Rows[row].Name != "" {
		name = v.Rows[row].Name
	}
	reason := s.disableReason(p)
	if k.Button(DisableID, gfx.Rect{X: x + 160 + gap, Y: y, W: 260, H: footerH}, s.sh.Tf("Disable %s", name),
		uikit.BtnOpts{Style: uikit.BtnDanger, Disabled: reason != "", Reason: reason}) {
		s.setEnabled(p.Plugin, false)
	}
	k.Panel(body)
	list := gfx.Rect{X: body.X + pad, Y: body.Y + pad, W: body.W - 2*pad, H: body.H - 2*pad}
	k.List(ProblemListID, list, len(v.Problems), &s.probSel, uikit.ListSpec{RowH: rowH(),
		Draw: func(dst *ebiten.Image, i int, r gfx.Rect, hover, selected bool) {
			s.drawProblem(dst, v.Problems[i], r, hover, selected)
		}})
}

// disableReason is why the selected problem's plugin can't be turned off
// from here ("" when it can).
func (s *Screen) disableReason(p Problem) string {
	switch {
	case p.Plugin == "":
		return s.sh.T("This problem names no plugin.")
	case p.Plugin == Core:
		return s.sh.T("The game's own content can't be turned off.")
	case !s.enabledNow()[p.Plugin]:
		return s.sh.T("This plugin is already off.")
	}
	return ""
}

// drawProblem paints one problem: where it is, then what is wrong.
func (s *Screen) drawProblem(dst *ebiten.Image, p Problem, row gfx.Rect, hover, selected bool) {
	uikit.DrawListRow(dst, row, "", hover, selected, false)
	x, y := float64(row.X+8), float64(row.Y+5)
	w := float64(row.W - 16)
	where := p.Where()
	if where == "" {
		where = s.sh.T("plugins")
	}
	kind, c := s.sh.T("Error"), uikit.Danger
	if p.Warn {
		kind, c = s.sh.T("Warning"), uikit.Warning
	}
	head := s.sh.Tf("%s: %s", kind, where)
	gfx.Text(dst, gfx.Ellipsize(head, gfx.Label, w), x, y, gfx.WithColor(gfx.Label, c))
	gfx.Text(dst, gfx.Ellipsize(p.Msg, gfx.Body, w), x, y+gfx.LineH(gfx.Body), gfx.Body)
}
