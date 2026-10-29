package inspector

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	PrisonerTabID = "inspector.prisoner"
	// Capture and Arrest never had an app.go click handler (they were
	// widget buttons), so they are new-style commands: "cmd." ids that carry
	// their command in Cmds.
	PrisonerCapture = "cmd.pawn.capture"
	PrisonerArrest  = "cmd.pawn.arrest"
	PrisonerRelease = "prisoner.release"
	// prisonerResMax is the resistance a full bar shows (the most a new
	// prisoner starts with).
	prisonerResMax = prisoner.MaxInitialResistance / prisoner.ResistanceUnit
	prisonerIcon   = 16 // icon size in the tab
)

// prisonerModes are the interaction modes in the order the tab offers them.
var PrisonerModes = []struct {
	Mode  prisoner.Mode
	Label string
	Tip   string
}{
	{prisoner.None, "No interaction", "Wardens only bring food."},
	{prisoner.Reduce, "Reduce", "Wardens talk the prisoner's resistance down."},
	{prisoner.Recruit, "Recruit", "Wardens wear resistance down, then try to win the prisoner over."},
	{prisoner.Release, "Release", "A warden lets the prisoner go home; its faction is grateful."},
	{prisoner.Execute, "Execute", "A warden kills the prisoner. Its relatives will not forget."},
}

func PrisonerModeID(m prisoner.Mode) string { return fmt.Sprintf("prisoner.mode.%d", m) }

// selectedPrisoner is the selected pawn and its Prisoner view when it is
// the colony's prisoner.
func (a Tabs) selectedPrisoner() (game.PawnView, game.PrisonerView, bool) {
	p, ok := a.SelectedPawn()
	if !ok || a.sim() == nil {
		return game.PawnView{}, game.PrisonerView{}, false
	}
	v, ok := a.sim().Prisoner(p.ID)
	return p, v, ok
}

// prisonerTabOn reports the Prisoner tab is the one showing.
func (a Tabs) PrisonerTabOn() bool {
	if a.Tab != "prisoner" {
		return false
	}
	_, _, ok := a.selectedPrisoner()
	return ok
}

// isPrisoner reports colonist id is the colony's prisoner.
func (a Tabs) isPrisoner(id entity.EntityID) bool {
	if a.sim() == nil {
		return false
	}
	_, ok := a.sim().Prisoner(id)
	return ok
}

// prisonerCommands are colonist p's Capture and Arrest buttons (nil for a
// prisoner, a raider, an embodied player or the dead). Capture sends p to
// take the downed enemy nearest to it; Arrest has the nearest other
// colonist take p to a prisoner bed. Raiders are not selectable, so the
// captor is the one selected.
func (a Tabs) prisonerCommands(p game.PawnView) []Button {
	if p.Prisoner || p.Hostile || p.Embodied || p.Status == "dead" {
		return nil
	}
	capture := Button{ID: PrisonerCapture, Label: "Capture", Icon: "capture"}
	arrest := Button{ID: PrisonerArrest, Label: "Arrest", Icon: "arrest",
		Desc: "Arrest " + p.Name + ": another colonist takes them to a prisoner bed.",
		Cmds: []command.Command{{Kind: command.Arrest, Target: uint64(p.ID)}}}
	var target game.PawnView
	found := false
	for _, h := range a.shell.Hostiles() {
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
	return []Button{capture, arrest}
}

// prisonerTabBody declares the Prisoner tab of colonist id inside r, the
// expanded panel's inner rectangle.
func (a Tabs) prisonerTabBody(id entity.EntityID, r gfx.Rect) {
	if p, ok := a.pawnView(id); ok && a.sim() != nil {
		a.prisonerTab(p, r)
	}
}

// queueIcon paints ui icon key at (x, y), prisonerIcon square.
func (a Tabs) queueIcon(key string, x, y float32) {
	img := a.env().Art.Image(key, color.NRGBA{})
	if img == nil {
		return
	}
	a.kit().Queue(func(dst *ebiten.Image) { gfx.Icon(dst, img, float64(x), float64(y), prisonerIcon) })
}

// prisonerTab declares and paints prisoner p's tab in the content rect in:
// resistance, the mode selector, Release now, the warden and what is
// missing for the wardens to do their work.
func (a Tabs) prisonerTab(p game.PawnView, in gfx.Rect) {
	v, ok := a.sim().Prisoner(p.ID)
	if !ok {
		return // stale selection: the tab goes with the prisoner
	}
	a.kit().Label(in.X, in.Y, gfx.Ellipsize(a.shell.PawnLabel(p), gfx.Label, float64(in.W)), gfx.WithColor(gfx.Label, uikit.Heading))
	y := in.Y + float32(gfx.LineStep(gfx.Label)) + 8
	a.queueIcon("ui_icon/prisoner", in.X, y+2)
	a.kit().Label(in.X+prisonerIcon+6, y, "Prisoner of the colony", gfx.Body)
	y += float32(gfx.LineStep(gfx.Body)) + 8

	a.kit().Label(in.X, y, fmt.Sprintf("Resistance %d", v.Resistance), gfx.Body)
	bar := gfx.Rect{X: in.X + 120, Y: y + 4, W: in.W - 120, H: 10}
	frac := min(float32(v.Resistance)/float32(prisonerResMax), 1)
	a.kit().Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, bar, uikit.SliderTrack)
		uikit.FillRect(dst, gfx.Rect{X: bar.X, Y: bar.Y, W: bar.W * frac, H: bar.H}, uikit.SliderFill)
	})
	y += float32(gfx.LineStep(gfx.Body)) + 10

	a.kit().Label(in.X, y, "Interaction", gfx.Label)
	y += float32(gfx.LineStep(gfx.Label)) + 4
	const bw, bh, gap = 120, 24, 8
	for i, m := range PrisonerModes {
		br := gfx.Rect{X: in.X + float32(i%3)*(bw+gap), Y: y + float32(i/3)*(bh+6), W: bw, H: bh}
		o := uikit.BtnOpts{Selected: uint8(m.Mode) == v.Mode, Tip: m.Label + "\n" + m.Tip}
		if m.Mode == prisoner.Execute {
			o.Style = uikit.BtnDanger
		}
		if a.kit().Button(PrisonerModeID(m.Mode), br, m.Label, o) {
			a.shell.Enqueue(command.Command{Kind: command.SetPrisonerMode, Target: uint64(p.ID), Mode: uint8(m.Mode)})
		}
	}
	rel := gfx.Rect{X: in.X + 2*(bw+gap), Y: y + bh + 6, W: bw, H: bh}
	ro := uikit.BtnOpts{Tip: "Release now\nThe prisoner gets up and walks home."}
	if p.Status != "alive" {
		ro.Disabled, ro.Reason = true, "too hurt to walk out"
	}
	if a.kit().Button(PrisonerRelease, rel, "Release now", ro) {
		a.shell.Enqueue(command.Command{Kind: command.ReleasePrisoner, Target: uint64(p.ID)})
	}
	y += 2*(bh+6) + 8

	warden := v.Warden
	if warden == "" {
		warden = "none yet"
	}
	a.kit().Label(in.X, y, "Warden: "+warden, gfx.Body)
	y += float32(gfx.LineStep(gfx.Body)) + 6
	for _, w := range a.PrisonerWarnings(p, v) {
		a.kit().Label(in.X, y, gfx.Ellipsize(w, gfx.Small, float64(in.W)), gfx.WithColor(gfx.Small, uikit.Warning))
		y += float32(gfx.LineStep(gfx.Small)) + 2
	}
}

// prisonerWarnings say what keeps the wardens from their work, each with
// the fix.
func (a Tabs) PrisonerWarnings(p game.PawnView, v game.PrisonerView) []string {
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
func (a Tabs) anyWarden() bool {
	for _, c := range a.shell.Pawns() {
		if !c.Hostile && !c.Prisoner && int(work.Warden) < len(c.Priorities) && c.Priorities[work.Warden] > 0 {
			return true
		}
	}
	return false
}
