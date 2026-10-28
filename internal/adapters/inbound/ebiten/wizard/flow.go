package wizard

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/storyteller"
)

// Flow is the New Colony wizard: the draft being set up and the screens
// that edit it. Each screen's widgets live on a page type over the Flow
// (scenarioPage, editorPage, storyPage, worldPage, landingPage,
// pawnsPage); the Flow dispatches the shell's screen calls to them.
type Flow struct {
	sh Shell
	d  *Draft // nil outside the wizard
	// Unsaved are this session's custom scenarios that were never saved
	// to a file; they stay in the list until the game quits.
	Unsaved []scenario.Def
	// confirmed lets the next Back pop without asking (pop).
	confirmed bool
}

// New is the wizard over its shell.
func New(sh Shell) *Flow { return &Flow{sh: sh} }

// The screens' pages: each holds the Flow it edits.
type (
	scenarioPage struct{ *Flow }
	editorPage   struct{ *Flow }
	storyPage    struct{ *Flow }
	worldPage    struct{ *Flow }
	landingPage  struct{ *Flow }
	pawnsPage    struct{ *Flow }
)

// Draft is the wizard's draft, created on first use.
func (w *Flow) Draft() *Draft { return w.draft() }

// Drop forgets the draft (the colony started).
func (w *Flow) Drop() { w.setDraft(nil) }

func (w *Flow) setDraft(d *Draft) {
	w.d = d
	w.sh.KeepDraft(d)
}

// Declare declares the widgets of the wizard screen s (the shell's screen
// table, once per Update).
func (w *Flow) Declare(s ports.Screen, sw, sh int) {
	switch s {
	case ports.ScreenScenario:
		scenarioPage{w}.scenarioUI(sw, sh)
	case ports.ScreenScenarioEditor:
		editorPage{w}.editorUI(sw, sh)
	case ports.ScreenStoryteller:
		storyPage{w}.storytellerUI(sw, sh)
	case ports.ScreenWorldGen:
		worldPage{w}.worldGenUI(sw, sh)
	case ports.ScreenLanding:
		landingPage{w}.landingUI(sw, sh)
	case ports.ScreenPawns:
		pawnsPage{w}.pawnsUI(sw, sh)
	}
}

// Buttons are the legacy button list of screen s: none, the screens'
// controls are kit widgets.
func (w *Flow) Buttons(ports.Screen, int, int) []uikit.Button { return nil }

// Draw paints the wizard screen s under its widgets.
func (w *Flow) Draw(s ports.Screen, dst *ebiten.Image, sw, sh int) {
	switch s {
	case ports.ScreenScenario:
		scenarioPage{w}.drawScenario(dst, sw, sh)
	case ports.ScreenScenarioEditor:
		editorPage{w}.drawScenarioEditor(dst, sw, sh)
	case ports.ScreenStoryteller:
		storyPage{w}.drawStoryteller(dst, sw, sh)
	case ports.ScreenWorldGen:
		w.drawNGFrame(dst, sw, sh, "Create World")
	case ports.ScreenLanding:
		landingPage{w}.drawLanding(dst, sw, sh)
	case ports.ScreenPawns:
		if w.d != nil && w.d.Load != nil {
			w.drawLoading(dst, sw, sh)
			return
		}
		w.drawNGFrame(dst, sw, sh, "Create Characters")
	}
}

// WorldGenerate starts (or skips) generating the planet of the draft's
// parameters.
func (w *Flow) WorldGenerate() { worldPage{w}.worldGenerate() }

// WorldStep advances the planet generation one frame.
func (w *Flow) WorldStep() { worldPage{w}.worldStep() }

// CoverageOptions are the globe coverages the Create World slider offers.
func (w *Flow) CoverageOptions() []int { return worldPage{w}.coverageOptions() }

// PoolKeyFor is what the candidate pool of ng is made from.
func (w *Flow) PoolKeyFor(ng *Draft) string { return pawnsPage{w}.poolKeyFor(ng) }

// PaintStoryCard draws a storyteller's card (also the Options story tab's).
func (w *Flow) PaintStoryCard(dst *ebiten.Image, r gfx.Rect, p storyteller.Profile, selected, hot bool) {
	storyPage{w}.paintStoryCard(dst, r, p, selected, hot)
}

// FactionColumns are the Factions table's columns for rows.
func (w *Flow) FactionColumns(rows []FacRow) []uikit.TableCol {
	return landingPage{w}.factionColumns(rows)
}

// FactionsTable declares the Factions table (also the colony's Factions
// window); pick, when set, runs for a row picked.
func (w *Flow) FactionsTable(r gfx.Rect, rows []FacRow, sel *int, pick func(FacRow)) {
	landingPage{w}.factionsTable(r, rows, sel, pick)
}

// BackstoryLine is "Childhood: Title" with the description as a tooltip
// (also the colony's Bio tab).
func (w *Flow) BackstoryLine(x, y, width float32, label, title, desc, id string) {
	pawnsPage{w}.backstoryLine(x, y, width, label, title, desc, id)
}

// TraitChips lays the traits out as chips and returns the next y (also
// the colony's Bio tab).
func (w *Flow) TraitChips(x, y, width float32, v game.CandidateView) float32 {
	return pawnsPage{w}.traitChips(x, y, width, v)
}

// kit is the shell's widget kit.
func (w *Flow) kit() *uikit.Kit { return w.sh.Kit() }

// in is this Update's input.
func (w *Flow) in() *inputframe.Frame { return w.sh.Kit().Host().Frame() }

// art is the sprite catalog (nil while the art is missing).
func (w *Flow) art() *art.Catalog { return w.sh.Env().Art }

// store is the custom scenario files (nil: unavailable).
func (w *Flow) store() ScenarioStore { return w.sh.Env().Scenarios }

// screen is the screen on top.
func (w *Flow) screen() ports.Screen { return w.sh.Env().Screen }

// sliderInt is a kit slider with the shell's step sound.
func (w *Flow) sliderInt(id string, r gfx.Rect, v *int, sp uikit.SliderSpec) bool {
	return w.kit().SliderInt(id, r, v, w.sh.SliderTick(sp))
}

// Next is the wizard screen after s (s itself past the last).
func Next(s ports.Screen) ports.Screen {
	switch s {
	case ports.ScreenScenario:
		return ports.ScreenStoryteller
	case ports.ScreenStoryteller:
		return ports.ScreenWorldGen
	case ports.ScreenWorldGen:
		return ports.ScreenLanding
	case ports.ScreenLanding:
		return ports.ScreenPawns
	default:
		return s
	}
}

// globeInputFrom adapts this Update's frame for the globe: w x h is the
// logical screen, the pointer the raw (physical) one the globe picks and
// drags with. blocked is whether the pointer is over screen chrome.
func globeInputFrom(in *inputframe.Frame, w, h int, blocked bool) globe.Input {
	return globe.Input{
		W: w, H: h, MX: in.Raw.Mx, MY: in.Raw.My, Wheel: in.Raw.WheelY,
		Down: in.Raw.Down, Press: in.Raw.Press, Release: in.Raw.Release,
		Held: in.Raw.Held, Keys: in.Raw.Pressed, Shift: in.Shift(), Blocked: blocked,
	}
}
