package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/command"
)

// The pawn inspector's Prisoner tab and the Capture / Arrest actions
// (ENG-003 M8, MAIN_UI_SCREENS §27). The tab reads only
// Simulation.Prisoner and the snapshot and emits SetPrisonerMode and
// ReleasePrisoner; the actions emit Capture (the selected colonist is the
// captor) and Arrest (of the selected colonist, by whoever the game picks). prisonerUI declares all of it
// with the widget kit every Update; inspector.go lists the tab and the
// action buttons and draws the panel around them.

const (
	prisonerTabID   = "inspector.prisoner"
	prisonerCapture = "pawn.capture"
	prisonerArrest  = "pawn.arrest"
	prisonerRelease = "prisoner.release"
	// prisonerResMax is the resistance a full bar shows (the most a new
	// prisoner starts with).
	prisonerResMax = prisoner.MaxInitialResistance / prisoner.ResistanceUnit
	prisonerIcon   = 16 // icon size on the action buttons and the tab
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

// prisonerTabSpec is the Prisoner tab's entry in the inspector's tab row
// (ok false: the selected pawn is no prisoner).
func (a *App) prisonerTabSpec() (buttonSpec, bool) {
	if _, _, ok := a.selectedPrisoner(); !ok {
		return buttonSpec{}, false
	}
	return buttonSpec{id: prisonerTabID, label: "Prisoner", selected: a.inspectTab == "prisoner", w: tabW("Prisoner"), h: 22}, true
}

// prisonerAction is one action-bar button with its command and icon.
type prisonerAction struct {
	b    button
	cmd  command.Command
	icon string
}

// prisonerActionList is the action bar's two icon buttons for a selected
// colonist p, right of Clear orders: Capture (p takes the downed enemy
// nearest to it) and Arrest (the nearest other colonist arrests p).
// Raiders are not selectable, so the captor is the one selected.
func (a *App) prisonerActionList(p game.PawnView, r button) []prisonerAction {
	if p.Prisoner || p.Hostile || p.Embodied || p.Status == "dead" {
		return nil
	}
	y := r.y + r.h - 62
	capture := button{id: prisonerCapture, x: r.x + 268, y: y, w: 58, h: 24, quiet: true}
	arrest := button{id: prisonerArrest, x: r.x + 330, y: y, w: 58, h: 24, quiet: true,
		tip: "Arrest " + p.Name + ": another colonist takes them to a prisoner bed."}
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
		capture.disabled, capture.reason = true, p.Status
	case !found:
		capture.disabled, capture.reason = true, "no downed enemy to capture"
	default:
		capture.tip = "Capture " + target.Name + ": carry them to a bed marked For prisoners."
	}
	return []prisonerAction{
		{capture, command.Command{Kind: command.Capture, Pawn: uint64(p.ID), Target: uint64(target.ID)}, "ui_icon/capture"},
		{arrest, command.Command{Kind: command.Arrest, Target: uint64(p.ID)}, "ui_icon/arrest"},
	}
}

// prisonerActions are prisonerActionList's buttons (inspector.go's bar).
func (a *App) prisonerActions(p game.PawnView, r button) []button {
	var out []button
	for _, pa := range a.prisonerActionList(p, r) {
		out = append(out, pa.b)
	}
	return out
}

// prisonerUI declares the Prisoner tab button, the Capture / Arrest
// actions (with their icons) and, while open, the tab's content.
func (a *App) prisonerUI() {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen {
		return
	}
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	u := a.uiSt()
	r := inspectorRect()
	for _, b := range a.inspectorButtons() {
		if b.id == prisonerTabID {
			u.add(uiWidget{id: b.id, label: b.label, r: rect{b.x, b.y, b.w, b.h}, click: func() { a.inspectTab = "prisoner" }})
		}
	}
	for _, pa := range a.prisonerActionList(p, r) {
		b, cmd := pa.b, pa.cmd
		w := uiWidget{id: b.id, label: b.label, r: rect{b.x, b.y, b.w, b.h}, disabled: b.disabled, reason: b.reason, tip: b.tip}
		if !b.disabled {
			w.click = func() { a.renderer.enqueue(cmd) }
		}
		u.add(w)
		a.queueIcon(pa.icon, b.x+(b.w-prisonerIcon)/2, b.y+(b.h-prisonerIcon)/2)
	}
	if a.prisonerTabOn() {
		a.prisonerTab(p, rect{r.x + 12, r.y + 10, r.w - 24, r.h - 10 - bioActionH})
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
	a.uiLabel(in.X, in.Y, ellipsize(pawnLabel(p), tsLabel, float64(in.W)-bioCloseGap), withColor(tsLabel, uiHeading))
	y := in.Y + float32(lineStep(tsLabel)) + 8
	a.queueIcon("ui_icon/prisoner", in.X, y+2)
	a.uiLabel(in.X+prisonerIcon+6, y, "Prisoner of the colony", tsBody)
	y += float32(lineStep(tsBody)) + 8

	a.uiLabel(in.X, y, fmt.Sprintf("Resistance %d", v.Resistance), tsBody)
	bar := rect{in.X + 120, y + 4, in.W - 120, 10}
	frac := min(float32(v.Resistance)/float32(prisonerResMax), 1)
	a.uiSt().queue(func(dst *ebiten.Image) {
		fillRect(dst, bar, uiSliderTrack)
		fillRect(dst, rect{bar.X, bar.Y, bar.W * frac, bar.H}, uiSliderFill)
	})
	y += float32(lineStep(tsBody)) + 10

	a.uiLabel(in.X, y, "Interaction", tsLabel)
	y += float32(lineStep(tsLabel)) + 4
	const bw, bh, gap = 120, 24, 8
	for i, m := range prisonerModes {
		br := rect{in.X + float32(i%3)*(bw+gap), y + float32(i/3)*(bh+6), bw, bh}
		o := btnOpts{Selected: uint8(m.mode) == v.Mode, Tip: m.label + "\n" + m.tip}
		if m.mode == prisoner.Execute {
			o.Style = btnDanger
		}
		if a.uiButton(prisonerModeID(m.mode), br, m.label, o) {
			a.renderer.enqueue(command.Command{Kind: command.SetPrisonerMode, Target: uint64(p.ID), Mode: uint8(m.mode)})
		}
	}
	rel := rect{in.X + 2*(bw+gap), y + bh + 6, bw, bh}
	ro := btnOpts{Tip: "Release now\nThe prisoner gets up and walks home."}
	if p.Status != "alive" {
		ro.Disabled, ro.Reason = true, "too hurt to walk out"
	}
	if a.uiButton(prisonerRelease, rel, "Release now", ro) {
		a.renderer.enqueue(command.Command{Kind: command.ReleasePrisoner, Target: uint64(p.ID)})
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
	for _, c := range a.renderer.snapshot() {
		if !c.Hostile && !c.Prisoner && int(work.Warden) < len(c.Priorities) && c.Priorities[work.Warden] > 0 {
			return true
		}
	}
	return false
}
