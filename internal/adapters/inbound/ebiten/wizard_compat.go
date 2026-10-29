package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	portspkg "rimworld-go/internal/adapters/inbound/ebiten/ports"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The New Colony wizard lives in wizard (ARCH-005). These aliases and
// one-line wrappers keep the root's call sites (the screen table, the
// click routes, the colony's Bio tab and Factions window, the tests)
// unchanged while the features move out one package at a time; the last
// mover deletes this file (SOLID_GO.md §11).

// The wizard's lists and the backdrop's one shared cache (drawn under its
// mutex), as the root's tests read them.
var (
	setupBackdrop   = &wizard.Backdrop
	setupBackdropMu = &wizard.BackdropMu
)

// ---- the App's wizard methods

func (a *App) applyWizardClick(id string) bool { return a.wiz.Click(id) }

func (a *App) scenarioUI(sw, sh int)    { a.wiz.Declare(portspkg.ScreenScenario, sw, sh) }
func (a *App) editorUI(sw, sh int)      { a.wiz.Declare(portspkg.ScreenScenarioEditor, sw, sh) }
func (a *App) storytellerUI(sw, sh int) { a.wiz.Declare(portspkg.ScreenStoryteller, sw, sh) }
func (a *App) worldGenUI(sw, sh int)    { a.wiz.Declare(portspkg.ScreenWorldGen, sw, sh) }
func (a *App) landingUI(sw, sh int)     { a.wiz.Declare(portspkg.ScreenLanding, sw, sh) }
func (a *App) pawnsUI(sw, sh int)       { a.wiz.Declare(portspkg.ScreenPawns, sw, sh) }

func (a *App) scenarioButtons(sw, sh int) []uikit.Button {
	return a.wiz.Buttons(portspkg.ScreenScenario, sw, sh)
}
func (a *App) storytellerButtons(sw, sh int) []uikit.Button {
	return a.wiz.Buttons(portspkg.ScreenStoryteller, sw, sh)
}
func (a *App) worldGenButtons(sw, sh int) []uikit.Button {
	return a.wiz.Buttons(portspkg.ScreenWorldGen, sw, sh)
}
func (a *App) landingButtons(sw, sh int) []uikit.Button {
	return a.wiz.Buttons(portspkg.ScreenLanding, sw, sh)
}
func (a *App) pawnButtons(sw, sh int) []uikit.Button {
	return a.wiz.Buttons(portspkg.ScreenPawns, sw, sh)
}

func (a *App) drawScenario(s *ebiten.Image, sw, sh int) {
	a.wiz.Draw(portspkg.ScreenScenario, s, sw, sh)
}
func (a *App) drawScenarioEditor(s *ebiten.Image, sw, sh int) {
	a.wiz.Draw(portspkg.ScreenScenarioEditor, s, sw, sh)
}
func (a *App) drawStoryteller(s *ebiten.Image, sw, sh int) {
	a.wiz.Draw(portspkg.ScreenStoryteller, s, sw, sh)
}
func (a *App) drawWorldGen(s *ebiten.Image, sw, sh int) {
	a.wiz.Draw(portspkg.ScreenWorldGen, s, sw, sh)
}
func (a *App) drawLanding(s *ebiten.Image, sw, sh int) { a.wiz.Draw(portspkg.ScreenLanding, s, sw, sh) }
func (a *App) drawPawns(s *ebiten.Image, sw, sh int)   { a.wiz.Draw(portspkg.ScreenPawns, s, sw, sh) }

// goBack is the shell's Back on a setup screen: the wizard may keep the
// screen (asking first, or while the colony is built).
func (a *App) goBack() {
	if a.wiz.GoBack() {
		a.popScreen()
	}
}

// popScreen pops one screen, closing its dialogs, and lets the wizard
// tidy its draft.
func (a *App) popScreen() {
	a.uiSt().Modals = nil
	was := a.nav.screen()
	a.nav.pop()
	a.wiz.Popped(was, a.nav.screen())
}

// wizHost is what the wizard sees of the App: the ports roles it calls
// and wizard.Host.
type wizHost struct{ a *App }

var _ wizard.Shell = wizHost{}

func (h wizHost) Hint(text string)              { shellHint{h.a}.Hint(text) }
func (h wizHost) Settings() config.Settings     { return shellSettings{h.a}.Settings() }
func (h wizHost) KeyName(x input.Action) string { return shellSettings{h.a}.KeyName(x) }
func (h wizHost) KeyHint(x input.Action) string { return shellSettings{h.a}.KeyHint(x) }
func (h wizHost) Kit() *uikit.Kit               { return shellWidgets{h.a}.Kit() }
func (h wizHost) Do(it ports.Intent)            { shellDispatch{h.a}.Do(it) }

func (h wizHost) Env() wizard.Env {
	a := h.a
	return wizard.Env{
		Screen:      a.nav.screen(),
		Scenarios:   a.cfg.Scenarios,
		Art:         a.art,
		DefaultSeed: a.cfg.DefaultSeed,
		Capture:     a.cfg.Capture != nil,
		DevMode:     a.tools.Mode(),
		MenuTick:    a.menuTick,
		SessionErr:  a.sessionErr,
		Now:         a.now(),
	}
}

func (h wizHost) KeepDraft(d *wizard.Draft)                       { h.a.ng = d }
func (h wizHost) SaveSettings(s config.Settings)                  { h.a.setSettings(s) }
func (h wizHost) SliderTick(sp uikit.SliderSpec) uikit.SliderSpec { return h.a.withTick(sp) }
func (h wizHost) Load(s *game.Setup) (wizard.Loader, error)       { return h.a.loadColony(s) }
func (h wizHost) MapSizeNote(side int) string                     { return options.MapSizeNote(side, h.a.hardwareTotal()) }
func (h wizHost) DrawDiorama(dst *ebiten.Image, sw, sh int, dim float64) bool {
	return h.a.menuScreen.DrawDiorama(dst, sw, sh, dim)
}
func (h wizHost) DrawPortrait(dst *ebiten.Image, l look.Look, x, y, w, ht float64) bool {
	return drawLookPortrait(dst, h.a.art, l, x, y, w, ht)
}
