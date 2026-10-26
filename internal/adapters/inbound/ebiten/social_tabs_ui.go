package ebiten

import (
	"fmt"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The colonist pane's Health and Social tabs (ENG-003 C4,
// MAIN_UI_SCREENS §14 and §15). Like the Bio tab they read only their read
// models (Simulation.Health, Simulation.Social) and are declared with the
// widget kit every Update, into the expanded panel the pane opens above
// itself (cmds_pawn.go's Health and Social paneTabs call kitTabBody with
// that rectangle). Until the body and social milestones fill the read
// models, ok is false and the tab says so.

// noDataYet is what a tab shows while its read model has nothing.
const noDataYet = "No data yet."

// kitTabOn reports a widget-kit tab (Health or Social) is the one showing
// for the selected colonist.
func (a *App) kitTabOn() bool {
	_, ok := a.selectedPawn()
	return ok && (a.inspectTab == "health" || a.inspectTab == "social")
}

// socialTabsUI declares the History tab's widgets (history_ui.go); the
// Health and Social tabs' own bodies come from the pane (kitTabBody).
func (a *App) socialTabsUI() {
	a.historyTabUI(a.sw, a.sh) // history_ui.go: the HUD's History tab
}

// kitTabBody declares the Health or Social tab ("health" or "social") of
// colonist id inside r, the expanded panel's inner rectangle: a heading,
// then the tab.
func (a *App) kitTabBody(id entity.EntityID, tab string, r rect) {
	p, ok := a.pawnView(id)
	if !ok || a.renderer == nil {
		return
	}
	in := r
	if tab == "social" {
		a.uiLabel(in.X, in.Y, "Social", withColor(tsLabel, uiHeading))
		a.socialTab(p, in) // social_ui.go
		return
	}
	a.uiLabel(in.X, in.Y, "Health", withColor(tsLabel, uiHeading))
	a.healthTab(p, in) // health_ui.go
}

// healthLines is the Health tab's plain text: capacities, pain, parts,
// queued operations with their success chance (MED-06) and drug lines
// (nil: no data); the tab itself is health_ui.go. Prisoners and guests
// get the same tab (pawnTabs), so their operations queue there too.
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
		ln := "Operation: " + op.Label
		if op.ChanceTip != "" { // MED-06: its success chance
			ln += " (" + healthChanceText(op) + ")"
		}
		out = append(out, ln)
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
