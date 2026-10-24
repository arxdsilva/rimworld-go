package ebiten

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// The pawn inspector's Health and Social tabs (ENG-003 C4,
// MAIN_UI_SCREENS §14 and §15). Like the Bio tab they read only their read
// models (Simulation.Health, Simulation.Social) and are declared with the
// widget kit every Update (colony_ui.go calls socialTabsUI); inspector.go
// draws the panel and the tab row around them. Until the body and social
// milestones fill the read models, ok is false and the tab says so.

// noDataYet is what a tab shows while its read model has nothing.
const noDataYet = "No data yet."

// kitTabOn reports a widget-kit tab (Health or Social) is the one showing
// for the selected colonist.
func (a *App) kitTabOn() bool {
	_, ok := a.selectedPawn()
	return ok && (a.inspectTab == "health" || a.inspectTab == "social")
}

// socialTabsUI declares the Health or Social tab's content while it is open.
func (a *App) socialTabsUI() {
	a.historyTabUI(a.sw, a.sh) // history_ui.go: the HUD's History tab
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen || !a.kitTabOn() {
		return
	}
	p, _ := a.selectedPawn()
	r := inspectorRect()
	in := rect{r.x + 12, r.y + 10, r.w - 24, r.h - 10 - bioActionH}
	a.uiLabel(in.X, in.Y, ellipsize(pawnLabel(p), tsLabel, float64(in.W)-bioCloseGap), withColor(tsLabel, uiHeading))
	if a.inspectTab == "social" {
		a.socialTab(p, in) // social_ui.go
		return
	}
	a.healthTab(p, in) // health_ui.go
}

// healthLines is the Health tab's plain text: capacities, pain, parts,
// queued operations and drug lines (nil: no data); the tab itself is
// health_ui.go.
func (a *App) healthLines(p game.PawnView) []string {
	h, ok := a.renderer.game.Health(p.ID)
	if !ok {
		return nil
	}
	var out []string
	for _, c := range h.Capacities {
		out = append(out, fmt.Sprintf("%s %d%%", c.Label, c.Pct))
	}
	if h.Pain > 0 {
		out = append(out, fmt.Sprintf("Pain %d%%", h.Pain))
	}
	for _, pt := range h.Parts {
		out = append(out, pt.Label+": "+pt.Status)
	}
	for _, op := range h.Operations {
		out = append(out, "Operation: "+op.Label)
	}
	for _, d := range h.Drugs {
		out = append(out, fmt.Sprintf("%s %d%%", d.Label, d.Pct))
	}
	return out
}

// socialLines is the Social tab's plain text: each relation with both
// opinions, then the social log (nil: no data).
func (a *App) socialLines(p game.PawnView) []string {
	s, ok := a.renderer.game.Social(p.ID)
	if !ok {
		return nil
	}
	var out []string
	for _, r := range s.Relations {
		ln := fmt.Sprintf("%s  %+d / %+d", r.Name, r.OpinionTo, r.OpinionFrom)
		if r.Label != "" {
			ln = r.Label + " " + ln
		}
		out = append(out, ln)
	}
	for _, l := range s.Log {
		out = append(out, l.Text)
	}
	return out
}
