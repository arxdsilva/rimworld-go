package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The colonist pane's Prisoner tab and the Capture / Arrest commands
// (ENG-003 M8, MAIN_UI_SCREENS §27). The tab reads only
// Simulation.Prisoner and the snapshot and emits SetPrisonerMode and
// ReleasePrisoner; the commands emit Capture (the selected colonist is the
// captor) and Arrest (of the selected colonist, by whoever the game
// picks). The tab is declared with the widget kit every Update into the
// expanded panel (cmds_pawn.go's Prisoner paneTab calls prisonerTabBody);
// the commands are cmds_pawn.go's pawnCommands entries.

const (
	prisonerTabID = "inspector.prisoner"
	// Capture and Arrest never had an app.go click handler (they were
	// widget buttons), so they are new-style commands: "cmd." ids that carry
	// their command in Cmds.
	prisonerCapture = "cmd.pawn.capture"
	prisonerArrest  = "cmd.pawn.arrest"
	prisonerRelease = "prisoner.release"
	// prisonerResMax is the resistance a full bar shows (the most a new
	// prisoner starts with).
	prisonerResMax = prisoner.MaxInitialResistance / prisoner.ResistanceUnit
	prisonerIcon   = 16 // icon size in the tab
)

// prisonerModes are the interaction modes in the order the tab offers them.
var prisonerModes = []struct {
	mode  prisoner.Mode
	label string
	tip   string
}{
	{prisoner.None, "No interaction", "Wardens only bring food."},
	{prisoner.Reduce, "Reduce", "Wardens talk the prisoner's resistance down."},
	{prisoner.Recruit, "Recruit", "Wardens wear resistance down, then try to win the prisoner over."},
	{prisoner.Release, "Release", "A warden lets the prisoner go home; its faction is grateful."},
	{prisoner.Execute, "Execute", "A warden kills the prisoner. Its relatives will not forget."},
}

func prisonerModeID(m prisoner.Mode) string { return fmt.Sprintf("prisoner.mode.%d", m) }

// selectedPrisoner is the selected pawn and its Prisoner view when it is
// the colony's prisoner.
func (a *App) selectedPrisoner() (game.PawnView, game.PrisonerView, bool) {
	p, ok := a.selectedPawn()
	if !ok || a.renderer == nil {
		return game.PawnView{}, game.PrisonerView{}, false
	}
	v, ok := a.renderer.game.Prisoner(p.ID)
	return p, v, ok
}

// prisonerTabOn reports the Prisoner tab is the one showing.
func (a *App) prisonerTabOn() bool {
	if a.inspectTab != "prisoner" {
		return false
	}
	_, _, ok := a.selectedPrisoner()
	return ok
}

// isPrisoner reports colonist id is the colony's prisoner.
func (a *App) isPrisoner(id entity.EntityID) bool {
	if a.renderer == nil {
		return false
	}
	_, ok := a.renderer.game.Prisoner(id)
	return ok
}

// prisonerCommands are colonist p's Capture and Arrest buttons (nil for a
// prisoner, a raider, an embodied player or the dead). Capture sends p to
// take the downed enemy nearest to it; Arrest has the nearest other
// colonist take p to a prisoner bed. Raiders are not selectable, so the
// captor is the one selected.
func (a *App) prisonerCommands(p game.PawnView) []cmdButton {
	if p.Prisoner || p.Hostile || p.Embodied || p.Status == "dead" {
		return nil
	}
	capture := cmdButton{ID: prisonerCapture, Label: "Capture", Icon: "capture"}
	arrest := cmdButton{ID: prisonerArrest, Label: "Arrest", Icon: "arrest",
		Desc: "Arrest " + p.Name + ": another colonist takes them to a prisoner bed.",
		Cmds: []command.Command{{Kind: command.Arrest, Target: uint64(p.ID)}}}
	var target game.PawnView
	found := false
	for _, h := range a.renderer.hostiles() {
		if h.Status != "downed" || h.Prisoner {
			continue
		}
		if d, best := abs(h.X-p.X)+abs(h.Y-p.Y), abs(target.X-p.X)+abs(target.Y-p.Y); !found || d < best || d == best && h.ID < target.ID {
			target, found = h, true
		}
	}
	switch {
	case p.Status != "alive":
		capture.Disabled, capture.Reason = true, p.Status
		capture.Desc = "Capture a downed enemy and carry them to a bed marked For prisoners."
	case !found:
		capture.Disabled, capture.Reason = true, "no downed enemy to capture"
		capture.Desc = "Capture a downed enemy and carry them to a bed marked For prisoners."
	default:
		capture.Desc = "Capture " + target.Name + ": carry them to a bed marked For prisoners."
		capture.Cmds = []command.Command{{Kind: command.Capture, Pawn: uint64(p.ID), Target: uint64(target.ID)}}
	}
	return []cmdButton{capture, arrest}
}

// prisonerTabBody declares the Prisoner tab of colonist id inside r, the
// expanded panel's inner rectangle.
func (a *App) prisonerTabBody(id entity.EntityID, r rect) {
	if p, ok := a.pawnView(id); ok && a.renderer != nil {
		a.prisonerTab(p, r)
	}
}

// queueIcon paints ui icon key at (x, y), prisonerIcon square.
func (a *App) queueIcon(key string, x, y float32) {
	img := a.art.image(key, noTint)
	if img == nil {
		return
	}
	a.uiSt().queue(func(dst *ebiten.Image) { drawIcon(dst, img, float64(x), float64(y), prisonerIcon) })
}

// prisonerTab declares and paints prisoner p's tab in the content rect in:
// resistance, the mode selector, Release now, the warden and what is
// missing for the wardens to do their work.
func (a *App) prisonerTab(p game.PawnView, in rect) {
	v, ok := a.renderer.game.Prisoner(p.ID)
	if !ok {
		return // stale selection: the tab goes with the prisoner
	}
	a.uiLabel(in.X, in.Y, ellipsize(pawnLabel(p), tsLabel, float64(in.W)), withColor(tsLabel, uiHeading))
	y := in.Y + float32(lineStep(tsLabel)) + 8
	a.queueIcon("ui_icon/prisoner", in.X, y+2)
	a.uiLabel(in.X+prisonerIcon+6, y, "Prisoner of the colony", tsBody)
	y += float32(lineStep(tsBody)) + 8

	a.uiLabel(in.X, y, fmt.Sprintf("Resistance %d", v.Resistance), tsBody)
	bar := rect{X: in.X + 120, Y: y + 4, W: in.W - 120, H: 10}
	frac := min(float32(v.Resistance)/float32(prisonerResMax), 1)
	a.uiSt().queue(func(dst *ebiten.Image) {
		fillRect(dst, bar, uiSliderTrack)
		fillRect(dst, rect{X: bar.X, Y: bar.Y, W: bar.W * frac, H: bar.H}, uiSliderFill)
	})
	y += float32(lineStep(tsBody)) + 10

	a.uiLabel(in.X, y, "Interaction", tsLabel)
	y += float32(lineStep(tsLabel)) + 4
	const bw, bh, gap = 120, 24, 8
	for i, m := range prisonerModes {
		br := rect{X: in.X + float32(i%3)*(bw+gap), Y: y + float32(i/3)*(bh+6), W: bw, H: bh}
		o := btnOpts{Selected: uint8(m.mode) == v.Mode, Tip: m.label + "\n" + m.tip}
		if m.mode == prisoner.Execute {
			o.Style = btnDanger
		}
		if a.uiButton(prisonerModeID(m.mode), br, m.label, o) {
			a.shell.Enqueue(command.Command{Kind: command.SetPrisonerMode, Target: uint64(p.ID), Mode: uint8(m.mode)})
		}
	}
	rel := rect{X: in.X + 2*(bw+gap), Y: y + bh + 6, W: bw, H: bh}
	ro := btnOpts{Tip: "Release now\nThe prisoner gets up and walks home."}
	if p.Status != "alive" {
		ro.Disabled, ro.Reason = true, "too hurt to walk out"
	}
	if a.uiButton(prisonerRelease, rel, "Release now", ro) {
		a.shell.Enqueue(command.Command{Kind: command.ReleasePrisoner, Target: uint64(p.ID)})
	}
	y += 2*(bh+6) + 8

	warden := v.Warden
	if warden == "" {
		warden = "none yet"
	}
	a.uiLabel(in.X, y, "Warden: "+warden, tsBody)
	y += float32(lineStep(tsBody)) + 6
	for _, w := range a.prisonerWarnings(p, v) {
		a.uiLabel(in.X, y, ellipsize(w, tsSmall, float64(in.W)), withColor(tsSmall, uiWarning))
		y += float32(lineStep(tsSmall)) + 2
	}
}

// prisonerWarnings say what keeps the wardens from their work, each with
// the fix.
func (a *App) prisonerWarnings(p game.PawnView, v game.PrisonerView) []string {
	var out []string
	if !a.anyWarden() {
		out = append(out, "No warden: give a colonist Warden work in the Work tab.")
	}
	if !v.ForPrisonersBed {
		out = append(out, "Not in a prisoner bed: mark a bed For prisoners.")
	}
	if p.Hunger < 30 {
		out = append(out, "Hungry: wardens bring meals, keep food in stock.")
	}
	return out
}

// anyWarden reports a colonist has Warden work on.
func (a *App) anyWarden() bool {
	for _, c := range a.shell.Pawns() {
		if !c.Hostile && !c.Prisoner && int(work.Warden) < len(c.Priorities) && c.Priorities[work.Warden] > 0 {
			return true
		}
	}
	return false
}
