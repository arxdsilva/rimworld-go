package devtools

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/input"
)

// debugLogLines is the last messages (with their tick) and every
// colonist's postponed work.
func debugLogLines(w game.WorldView, pawns []game.PawnView, n int) []string {
	var out []string
	msgs := w.Messages
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	for _, m := range msgs {
		out = append(out, fmt.Sprintf("t%-7d %s", m.Tick, m.Text))
	}
	for _, p := range pawns {
		for _, l := range p.Postponed {
			out = append(out, pawnLabel(p)+" put off: "+l)
		}
	}
	if len(out) == 0 {
		out = append(out, "no messages yet")
	}
	return out
}

// logShown is the debug log's lines, at most n, each cut to the log's
// width (at most devLogMaxW, and maxW when the column is narrower).
func (p *Panel) logShown(n int, maxW float32) []string {
	lines := debugLogLines(p.sh.World(), p.sh.Pawns(), 12)
	if len(lines) > 18 {
		lines = lines[:18]
	}
	if n < len(lines) {
		lines = lines[:max(n, 0)]
	}
	for i, l := range lines {
		lines[i] = gfx.Ellipsize(l, gfx.Small, min(devLogMaxW, float64(maxW)))
	}
	return lines
}

// drawLog paints the debug log in r.
func (p *Panel) drawLog(screen *ebiten.Image, r gfx.Rect) {
	lines := p.logShown(int((r.H-devLogChrome)/devStep), r.W-20)
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	gfx.Text(screen, "Debug log ("+p.sh.KeyName(input.DevLog)+")", float64(r.X)+8, float64(r.Y)+3, gfx.Label)
	for i, l := range lines {
		gfx.Text(screen, l, float64(r.X)+8, float64(r.Y)+24+float64(i*devStep), gfx.Small)
	}
}

// inspectLines is the raw record of the selection.
func (p *Panel) inspectLines() []string {
	s := p.sh.Selected()
	switch {
	case s.Pawn != nil:
		pw := s.Pawn
		return []string{
			fmt.Sprintf("pawn #%d %q at (%d, %d)", pw.ID, pw.Name, pw.X, pw.Y),
			fmt.Sprintf("status=%s drafted=%v hostile=%v", pw.Status, pw.Drafted, pw.Hostile),
			fmt.Sprintf("job=%q acting=%q", pw.Job, pw.Acting),
			fmt.Sprintf("queued=%d postponed=%d", len(pw.QueuedOrders), len(pw.Postponed)),
		}
	case s.Animal != nil:
		an := s.Animal
		return []string{fmt.Sprintf("animal #%d %s at (%d, %d)", an.ID, an.Species, an.X, an.Y),
			fmt.Sprintf("mode=%q health=%d tame=%v hostile=%v", an.Mode, an.Health, an.Tame, an.Hostile)}
	case s.Item != nil:
		it := s.Item
		return []string{fmt.Sprintf("item #%d %s x%d at (%d, %d)", it.ID, it.Kind, it.Qty, it.X, it.Y),
			fmt.Sprintf("forbidden=%v carried=%v fresh=%.1fd", it.Forbidden, it.Carried, it.FreshDays)}
	case s.Building != nil:
		b := s.Building
		return []string{fmt.Sprintf("building #%d %s (%s) at (%d, %d) %dx%d", b.ID, b.Kind, b.Stuff, b.X, b.Y, b.W, b.H),
			fmt.Sprintf("facing=%s hp=%d%% claimant=%d", b.Facing, b.HPPct, b.Claimant)}
	case s.Plant != nil:
		pl := s.Plant
		return []string{fmt.Sprintf("plant #%d %s at (%d, %d) growth=%d%% stage=%d", pl.ID, pl.Species, pl.X, pl.Y, pl.Growth, pl.Stage)}
	case s.Frame != nil:
		f := s.Frame
		return []string{fmt.Sprintf("frame #%d %s at (%d, %d)", f.ID, f.Kind, f.X, f.Y)}
	case len(s.Group) >= 2:
		return []string{fmt.Sprintf("group of %d: %v", len(s.Group), s.Group)}
	}
	return []string{"nothing selected"}
}

// inspectorLines are the inspector panel's lines, title first.
func (p *Panel) inspectorLines() []string {
	e := p.sh.Env()
	return append([]string{fmt.Sprintf("Debug inspector  tick %d", p.sh.Tick()),
		fmt.Sprintf("update budget: owed %d held %d", e.Owed, e.Held)}, p.inspectLines()...)
}

// inspectorSize is the inspector panel's size.
func (p *Panel) inspectorSize() (w, h float32) {
	lines := p.inspectorLines()
	return gfx.MaxTextW(lines, gfx.Small) + 20, float32(len(lines)*devStep + 12)
}

// drawInspector paints the debug inspector in r.
func (p *Panel) drawInspector(screen *ebiten.Image, r gfx.Rect) {
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	for i, l := range p.inspectorLines() {
		gfx.Text(screen, l, float64(r.X)+8, float64(r.Y)+4+float64(i*devStep), gfx.Small)
	}
}
