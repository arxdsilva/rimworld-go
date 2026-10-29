package ebiten

import (
	"image"
	"math"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	optionspkg "rimworld-go/internal/adapters/inbound/ebiten/options"
	portspkg "rimworld-go/internal/adapters/inbound/ebiten/ports"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	windowspkg "rimworld-go/internal/adapters/inbound/ebiten/windows"
	wizardpkg "rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"strconv"
	"strings"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/devtools"
	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/learn"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/orders"
	"rimworld-go/internal/adapters/inbound/ebiten/perf"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
	"rimworld-go/internal/platform/sysinfo"
)

const (
	// DefaultWindowWidth is the initial interactive client width in pixels.
	DefaultWindowWidth = 1280
	// DefaultWindowHeight is the initial interactive client height in pixels.
	DefaultWindowHeight = 720

	bootFramesPerPhase = 8
	bootAssetPhase     = 4 // "asset manifests/atlases"

	bodyDefaultX = 56
	bodyDefaultY = 112
	bodyRailGap  = 16
	footerYOff   = 64
)

var bootPhases = []string{
	"configuration",
	"plugin discovery",
	"dependency resolution",
	"content definitions",
	"asset manifests/atlases",
	"localization",
	"renderer",
	"main-menu transition",
}

// SimFactory constructs a headless simulation when the player starts a
// game. The adapter never calls game.New itself; the composition root
// supplies the factory.
type SimFactory func(NewGame) (Simulation, error)

// NewGame is what the new-game wizard chose: the world seed and the
// biome the colony lands in.
type NewGame struct {
	Seed  uint64
	Biome biome.ID
	// Setup is the full wizard result (NG-1); nil = the legacy start from
	// Seed and Biome (captures, tests).
	Setup *game.Setup
}

// AppConfig is the composition-root wiring for the interactive client.
type AppConfig struct {
	Version string
	NewSim  SimFactory
	// DefaultSeed pre-fills the wizard's seed (0: a random one).
	DefaultSeed uint64
	// DefaultBiome is the biome the landing picker starts on.
	DefaultBiome biome.ID
	// LoadArt loads the original generated sprites (assets/metadata).
	// Nil: locate from cwd. When it fails the renderer draws plain
	// coloured fallbacks.
	LoadArt    func() (*artpkg.Catalog, error)
	LoadSplash func() (*ebiten.Image, error)
	// Scenarios stores custom scenarios (nil: custom scenarios unavailable).
	Scenarios wizardpkg.ScenarioStore
	// FPSProbe runs a dev frame-rate measurement and quits (fps_probe.go).
	FPSProbe *perf.FPSProbe
	// NewSimBuilder builds a wizard Setup colony step by step for the
	// loading screen (nil: NewSim in one step).
	NewSimBuilder func(NewGame) (SimBuilder, error)
	// LoadMenuScene loads the main-menu colony diorama (make art-menu).
	// Nil: locate from cwd. When it fails the gradient backdrop is drawn.
	LoadMenuScene func() (*mainmenu.Diorama, error)
	// Capture, when set, runs a scripted screenshot session and quits.
	Capture *Capture
	// Saves connects save/load to storage (zero value: disabled).
	Saves SaveHooks
	// Settings loads and stores the Options (zero value: defaults, not saved).
	Settings optionspkg.Hooks
	// Window applies fullscreen and VSync (nil: leave the window alone).
	Window func(fullscreen, vsync bool)
	// Sound plays the client's audio (nil: silent).
	Sound Sound
	// Hardware reads the machine's memory and cores (PLAY-019): the first
	// start picks a graphics tier from it and small machines get a notice
	// (nil: unknown, nothing detected).
	// Background is the window the run-in-background setting drives and
	// Window the close hooks (saves.go); nil means the real Ebitengine
	// window. Tests pass fakes here instead of swapping package state.
	Background  bgWindow
	WindowHooks windowHooks
	Hardware    func() sysinfo.Info
}

// App is the ebiten.Game shell: screen routing, widgets, and (once
// started) the colony renderer. Menus do not own or step a simulation.
type App struct {
	// hst is the colony HUD regions' own state (PLAY-018, hud.go): each
	// region's owner keeps its fields in its own struct type.
	hst hudStates
	// debt is the Update time budget's carried ticks (PERF-002).
	debt tickDebt
	// ui is the widget kit's state (NG-1 §1.4, ui_state.go).
	ui  uikit.Kit
	cfg AppConfig
	nav nav
	// sh is what the features see of the App: the ports roles and the
	// screen and click tables, filled by NewApp (shell.go), never by init().
	shell shell

	// sw, sh are the logical screen size every layout and hit test reads;
	// pw, ph the physical one (sw x scale.s). outW, outH are the window in
	// points and devRaw the screen's device factor from the last LayoutF;
	// devFn replaces the monitor in tests (ui_scale.go).
	sw, sh     int
	pw, ph     int
	scale      gfx.Scale
	wr         worldRender // world_scale.go: the render-scaled map (PERF-002)
	outW, outH float64
	devRaw     float64
	devFn      func() float64
	bootFrames int
	artTried   bool
	art        *artpkg.Catalog
	artWarn    string
	// closeHandled is what the window close hooks (cfg.WindowHooks,
	// saves.go) were last told.
	closeHandled bool
	// treeFocus / treeSel are reused buffers for the see-through canopy
	// focus (tree_focus.go).
	treeFocus, treeSel [][2]int
	menuTick           uint64           // drives menu motion; counts while no colony is shown
	menuScreen         *mainmenu.Screen // mainmenu.Screen owns the main menu (ARCH-005)
	wiz                *wizard.Flow     // wizard.Flow owns the New Colony flow (ARCH-005)
	// The colony's features, each its own package behind a Shell (ARCH-005
	// wave 5.3): the Options screen, the main-tab windows, the Architect,
	// the order flow, the dialogs, the learning helper, developer mode and
	// the frame-timing instruments. Built by NewApp from the <pkg>_compat.go
	// constructors.
	opt        *options.Screen
	win        *windows.Set
	arch       *architect.Tool
	ord        *orders.Controller
	deal       *dialogs.Dealings
	lrn        *learn.Helper
	tools      *devtools.Panel
	meter      *perf.Meter
	insp       *inspector.Pane // the selection's inspector (ARCH-005 wave 5.4)
	chrome     *hud.Chrome     // the colony HUD's regions (ARCH-005 wave 5.5)
	sessionErr string
	hint       string
	shouldQuit bool

	// ng is the New Colony wizard's draft as wizard.Flow hands it over
	// (KeepDraft, wizard_compat.go); nil outside the wizard.
	ng *wizardpkg.Draft
	// settings are the player's Options (settings.go).
	settings config.Settings
	snd      soundDirector // what is heard (sound_director.go)

	simsStarted int
	renderer    *Renderer
	cursor      *[2]int          // capture-only synthetic cursor
	sel         *selection.Model // selection.Model owns what is selected (ARCH-005)
	capture     captureState
	now         func() time.Time // wall clock for alpha (injectable for tests)
	lastTick    time.Time        // when the renderer last advanced
	lastDraw    time.Time        // previous colony Draw: the per-Draw camera step

	// Input (UI-016): the device source (a fake in tests), this Update's
	// resolved frame, the live bindings and an Update counter.
	src     inputframe.Source
	in      inputframe.Frame
	binds   input.Bindings
	updates uint64
	bg      bgState                // Run in background (background.go)
	rebind  optionspkg.RebindState // Options › Controls key capture (controls_ui.go)
	midDrag midDragState           // middle-mouse camera drag
}

// art_atlas.go: pages cut in the background, not mid-frame

// dev frame-rate probe (fps_probe.go)

// perf_stats.go: the overlay's timing (PERF-002)

// commitment: save before the window closes (saves.go)

// ui_scale.go: before any measure or hit test

// background.go: Run in background, from this frame's focus

// F10 screenshot, F11 hide UI (hotkeys.go)

// minimap (minimap.go): the camera moved

// hud.go: letters, alerts, command options

// hotkeys.go

// captures frame the camera themselves
// physical (camera.go)

// click outside a menu just closes it

// move_mode.go: placing a moved building
// zone_select.go: a selected zone's Expand or Shrink tool

// selection.go / multiselect.go / formation.go

func (a *App) pressEscape() {
	if a.handleColonyEscape() {
		return
	}
	a.apply(a.nav.escapeIntent())
}

func (a *App) handleColonyEscape() bool {
	if a.nav.screen() != portspkg.ScreenColony || a.nav.overlay() != overlayNone {
		return false
	}
	if a.chrome.Hidden {
		a.chrome.Hidden = false // Esc brings the interface back first
		return true
	}
	if a.ord.Float.Open {
		a.ord.CloseFloat()
		return true
	}
	if a.tools.State.Menu {
		a.tools.State.Menu = false
		return true
	}
	if a.ord.Form.On {
		a.ord.Form = orders.Formation{}
		return true
	}
	if a.arch.Mover.State().On {
		a.arch.Mover.State().On = false // Move mode takes Esc alone (UI-016)
		return true
	}
	if a.arch.Zones.Escape() { // zone_select.go: a zone's Expand or Shrink tool
		return true
	}
	if a.arch.Open && a.arch.ToolID != "" {
		a.arch.ToolID = ""
		a.arch.Drag = false
		return true
	}
	if a.arch.Open {
		a.arch.ClosePanel()
		return true
	}
	if a.win.Open {
		a.win.Open = false
		return true
	}
	if a.lrn.CloseCard() { // learn_ui.go: the lesson detail card goes first
		return true
	}
	if a.insp.ClosePaneTab() { // hud_inspect.go: an open inspector tab goes before the selection
		return true
	}
	if a.sel.HasBuilding {
		a.insp.Building().ClearBuilding()
		return true
	}
	if a.sel.HasFrame {
		a.insp.Thing().ClearFrame()
		return true
	}
	if a.sel.HasZone {
		a.arch.Zones.Clear() // zone_select.go
		return true
	}
	if a.sel.Grouped() {
		a.sel.Clear(selectionpkg.None)
		return true
	}
	if a.renderer != nil && a.renderer.HasSelect() {
		a.renderer.ClearSelect()
		return true
	}
	return false
}

func (a *App) size() (int, int) {
	sw, sh := a.sw, a.sh
	if sw < 1 {
		sw = DefaultWindowWidth
	}
	if sh < 1 {
		sh = DefaultWindowHeight
	}
	return sw, sh
}

func (a *App) tick() {
	switch a.nav.screen() {
	case portspkg.ScreenBoot:
		a.tickBoot()
	case portspkg.ScreenColony:
		if a.nav.overlay() == overlayNone && a.renderer != nil && !a.bg.paused { // background.go
			a.renderer.cueBus().Voice.View = a.mapViewport() // animal calls only on screen (physical, like worldToScreen)
			a.advanceColony()                                // update_budget.go: steps under the Update budget, stamps the Update
			a.opt.TickAutosave()
		} else {
			a.clearTickDebt() // a paused colony owes nothing on resume
		}
	}
}

func (a *App) tickBoot() {
	a.bootFrames++
	phase := a.bootPhase()
	if phase >= bootAssetPhase && !a.artTried {
		a.loadAssets()
	}
	if phase >= len(bootPhases) {
		a.finishBoot()
	}
}

func (a *App) bootPhase() int {
	return a.bootFrames / bootFramesPerPhase
}

func (a *App) loadAssets() {
	a.artTried = true
	if a.cfg.LoadArt != nil {
		art, err := a.cfg.LoadArt()
		if err != nil {
			a.artWarn = err.Error()
		} else {
			a.art = art
		}
	}
	a.menuScreen.LoadSplash()
	a.menuScreen.LoadScene()
}

func (a *App) finishBoot() {
	if !a.artTried {
		a.loadAssets()
	}
	a.nav.reset(frame{screen: portspkg.ScreenMainMenu})
}

// applyClick runs a clicked id: the widget kit first, then the first
// route whose id or prefix matches (click_routes.go), else the screen
// intent the id names.
func (a *App) applyClick(id string) {
	if a.uiSt().Click(id) { // widget kit ids (ui_state.go)
		return
	}
	for _, rt := range a.shell.routes {
		if rt.matches(id) && rt.run(a, id) {
			return
		}
	}
	a.apply(a.clickIntent(id))
}

func (a *App) clickIntent(id string) intent {
	switch id {
	case "menu.newgame":
		return intent{kind: intentPush, screen: portspkg.ScreenScenario}
	case "menu.load":
		return intent{kind: intentPush, screen: portspkg.ScreenLoad}
	case "menu.options":
		return intent{kind: intentPush, screen: portspkg.ScreenOptions}
	case "menu.credits":
		return intent{kind: intentPush, screen: portspkg.ScreenCredits}
	case "menu.quit", "pause.quit":
		return intent{kind: intentQuit}
	case "wizard.back", "options.back", "load.back", "credits.back":
		return intent{kind: intentBack}
	case "wizard.next":
		return intent{kind: intentPush, screen: wizardNext(a.nav.screen())}
	case "wizard.start":
		return intent{kind: intentStartGame}
	case "pause.resume":
		return intent{kind: intentResume}
	case "pause.options":
		return intent{kind: intentPush, screen: portspkg.ScreenOptions}
	case "pause.menu":
		return intent{kind: intentReturnToMenu}
	default:
		return intent{}
	}
}

func (a *App) apply(in intent) {
	switch in.kind {
	case intentPush:
		a.nav.push(frame{screen: in.screen})
	case intentBack:
		a.goBack() // newgame_state.go: confirms before it discards a setup
	case intentStartGame:
		a.startGame()
	case intentQuit:
		if !a.opt.SaveBeforeLeaving() {
			return
		}
		a.shouldQuit = true
	case intentReturnToMenu:
		if !a.opt.SaveBeforeLeaving() {
			return
		}
		a.dropSession()
		a.nav.reset(frame{screen: portspkg.ScreenMainMenu})
	case intentResume:
		a.nav.setOverlay(overlayNone)
	case intentPause:
		a.nav.setOverlay(overlayPause)
	case intentFinishBoot:
		a.finishBoot()
	}
}

// markUpdate stamps the renderer advance that Draw interpolates from.
func (a *App) markUpdate() { a.lastTick = a.now() }

// frameAlpha is the fraction of the update interval elapsed since the
// renderer last advanced (C7), in [0, 1). The wall clock stays in the
// adapter; captures use 0 so their frames are deterministic.
func (a *App) frameAlpha() float64 {
	if a.cfg.Capture != nil || a.lastTick.IsZero() {
		return 0
	}
	tps := ebiten.TPS()
	if tps <= 0 {
		tps = 60
	}
	f := float64(a.now().Sub(a.lastTick)) / float64(time.Second/time.Duration(tps))
	return math.Max(0, math.Min(f, motionpkg.AlphaMax))
}

// startGame starts the legacy colony (no wizard draft): the configured
// seed and biome. The New Colony wizard starts through beginLoading.
func (a *App) startGame() {
	seed := a.cfg.DefaultSeed
	if seed == 0 {
		seed = wizardpkg.RandomSeed()
	}
	b, err := a.simBuilder(NewGame{Seed: seed, Biome: a.cfg.DefaultBiome})
	if err != nil {
		a.sessionErr = err.Error()
		return
	}
	for {
		if _, _, done := b.Step(wizardpkg.LoadUnits); done {
			break
		}
	}
	sim, err := b.Sim()
	if err != nil {
		a.sessionErr = err.Error()
		return
	}
	a.finishStart(sim)
}

// startSession installs a simulation (new or loaded) as the colony.
func (a *App) startSession(sim Simulation) {
	a.sessionErr = ""
	a.simsStarted++
	a.win.Open = false
	a.arch.ClosePanel()
	a.arch.Cat = ""
	a.arch.Facing = 0
	a.ord.CloseFloat()
	a.insp.Tab = "character"
	a.renderer = New(sim)
	a.renderer.cueBus().SoundHas = a.cfg.Sound.Has // animal voices fall back by what exists
	a.renderer.art = a.art
	a.syncRenderer()
	a.nav.reset(frame{screen: portspkg.ScreenColony})
}

func (a *App) dropSession() {
	a.renderer = nil
	a.win.Open = false
	a.arch.Open = false
	a.arch.ToolID = ""
	a.arch.Drag = false
	a.insp.Tab = "character"
}

// background.go: ~20 fps while running in the background
// perf_stats.go: runs before the throttle sleeps (PERF-002)
// ui_scale.go: the canvas and the font draw at this scale

// logical

// the viewport is in this frame's map pixels

// tree_focus.go: hover, selection and ghost see through canopies
// world_scale.go: the map, and what the tools mark on it, draw
// into the render-scaled offscreen when there is one; its canvas
// labels are queued and drawn on the screen after it

// mapfields_overlay.go: dev only

// hud.go: selection marks, ghosts, the drag box

// minimap.go: big maps only

// perf_overlay.go: also with the UI hidden (PERF-002)

// widget kit: widgets, modals, tooltips (ui_state.go)

// F10: this frame, before the toast that reports it

// uiHidden reports F11 hid the interface (the colony map only).
func (a *App) uiHidden() bool { return a.chrome.Hidden && a.nav.screen() == portspkg.ScreenColony }

func (a *App) drawHoverHint(screen *ebiten.Image, sw, sh int) {
	if a.uiHidden() {
		return
	}
	mx, my := a.currentCursor()
	if b, ok := uikit.HitButton(a.layoutHits(sw, sh), mx, my); ok {
		if b.Disabled && b.Reason != "" {
			a.drawBottomHint(screen, b.Reason, sh)
			return
		}
		if b.Tip != "" { // a small card by the cursor, clear of the tab strip
			tw, _ := gfx.Measure(b.Tip, gfx.Body)
			w := int(tw) + 16
			ch := int(gfx.LineH(gfx.Body)) + 6
			x, y := min(mx+14, sw-w-4), my+20
			if y+ch+2 > sh {
				y = my - ch - 8
			}
			uikit.DrawPanel(screen, float32(x), float32(y), float32(w), float32(ch))
			gfx.Text(screen, b.Tip, float64(x+8), float64(y+3), gfx.Body)
		}
	}
	if a.hint != "" {
		a.drawBottomHint(screen, a.hint, sh)
	}
	if a.artWarn != "" && a.nav.screen() == portspkg.ScreenMainMenu {
		uikit.DebugPrintAt(screen, "asset warning: "+a.artWarn, 16, sh-40)
	}
}

// drawBottomHint prints a one-line hint at the bottom left: on the footer
// row of a menu screen, or on a card just above the colonist bar so it
// does not run over the tab strip.
func (a *App) drawBottomHint(screen *ebiten.Image, s string, sh int) {
	if a.nav.screen() != portspkg.ScreenColony {
		uikit.DebugPrintAt(screen, s, 16, sh-22)
		return
	}
	w := int(gfx.TextW(s, gfx.Body)) + 16
	h := int(gfx.LineStep(gfx.Body)) + 4
	hx, hb := a.hintAnchor() // hud_commands.go: above the command buttons or the pane
	x, y := int(hx), int(hb)-h
	uikit.DrawPanel(screen, float32(x), float32(y), float32(w), float32(h))
	uikit.DrawLabel(screen, x+8, y+4, s)
}

func (a *App) layoutButtons(sw, sh int) []uikit.Button {
	if a.nav.overlay() == overlayPause {
		return a.pauseButtons(sw, sh)
	}
	switch a.nav.screen() {
	case portspkg.ScreenMainMenu:
		return a.menuScreen.Buttons(sw, sh)
	case portspkg.ScreenScenario:
		return a.scenarioButtons(sw, sh)
	case portspkg.ScreenStoryteller:
		return a.storytellerButtons(sw, sh)
	case portspkg.ScreenWorldGen:
		return a.worldGenButtons(sw, sh)
	case portspkg.ScreenLanding:
		return a.landingButtons(sw, sh)
	case portspkg.ScreenPawns:
		return a.pawnButtons(sw, sh)
	case portspkg.ScreenOptions:
		return a.opt.Buttons(sw, sh)
	case portspkg.ScreenLoad:
		return a.opt.LoadButtons(sw, sh)
	case portspkg.ScreenCredits:
		return a.creditsButtons(sw, sh)
	case portspkg.ScreenColony:
		return a.colonyButtons(sw, sh)
	default:
		return nil
	}
}

func (a *App) drawButtons(screen *ebiten.Image, buttons []uikit.Button, mx, my int) {
	for _, b := range buttons {
		uikit.DrawButton(screen, b, b.Contains(mx, my))
	}
}

func footerY(sh int) float32 {
	return float32(sh - footerYOff)
}

func isFooterButton(b uikit.Button, sh int) bool {
	return b.Y >= footerY(sh)-1
}

// bodyOrigin puts copy to the right of left-rail buttons so it is not
// drawn under them. Footer Back/Next are ignored.
func bodyOrigin(buttons []uikit.Button, sh int) (x, y int) {
	x, y = bodyDefaultX, bodyDefaultY
	maxRight := 0
	for _, b := range buttons {
		if isFooterButton(b, sh) {
			continue
		}
		if right := int(b.X + b.W); right > maxRight {
			maxRight = right
		}
	}
	if maxRight > 0 {
		x = maxRight + bodyRailGap
	}
	return x, y
}

func (a *App) fillMenu(screen *ebiten.Image, sw, sh int, title string, body []string, bodyX int) {
	if !a.menuScreen.DrawDiorama(screen, sw, sh, 0.55) {
		uikit.DrawMenuBackdrop(screen, sw, sh)
	}
	uikit.DrawPanel(screen, 40, 24, float32(sw-80), 56)
	gfx.Text(screen, title, 56, 40, gfx.Large)
	if len(body) == 0 {
		return
	}
	if bodyX < 1 {
		bodyX = bodyDefaultX
	}
	uikit.DrawPanel(screen, 40, 96, float32(sw-80), float32(sh-180))
	y := bodyDefaultY
	for _, line := range body {
		uikit.DrawLabel(screen, bodyX, y, line)
		y += int(gfx.LineH(gfx.Body))
	}
}

// currentCursor is the pointer in logical UI pixels: what the widget kit,
// layoutHits, hitButton and every HUD/menu hit test read. a.cursor
// (captures, tests) is logical.
func (a *App) currentCursor() (int, int) {
	if a.cursor != nil {
		return a.cursor[0], a.cursor[1]
	}
	return a.in.Ui.Mx, a.in.Ui.My
}

// mapCursor is the pointer in map pixels, for the map (screenToWorld,
// selection): the raw device position x the render scale, or a.cursor x
// mapScale. Every TileAt / screenToWorld argument comes from here
// (TestMapPixelsComeFromMapCursor).
func (a *App) mapCursor() (int, int) {
	if a.cursor != nil {
		s := a.mapScale()
		return int(float64(a.cursor[0]) * s), int(float64(a.cursor[1]) * s)
	}
	if k := a.scale.WorldK(); k != 1 {
		return int(float64(a.in.Raw.Mx) * k), int(float64(a.in.Raw.My) * k)
	}
	return a.in.Raw.Mx, a.in.Raw.My
}

func (a *App) paintMenu(screen *ebiten.Image, sw, sh int, title string, body []string, buttons []uikit.Button) {
	bx, _ := bodyOrigin(buttons, sh)
	a.fillMenu(screen, sw, sh, title, body, bx)
	mx, my := a.currentCursor()
	a.drawButtons(screen, buttons, mx, my)
}

func drawDim(screen *ebiten.Image, sw, sh int) {
	gfx.FillRect(screen, 0, 0, float32(sw), float32(sh), uikit.Dim, false)
}

func (a *App) toggleWorkCell(id string) {
	if a.renderer == nil {
		return
	}
	rest := strings.TrimPrefix(id, "work.cell.")
	parts := strings.Split(rest, ".")
	if len(parts) != 2 {
		return
	}
	pawnN, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return
	}
	col, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}
	wt, ok := windowspkg.WorkTypeForCol(col)
	if !ok {
		return
	}
	next := work3
	for _, p := range a.renderer.snapshot() {
		if uint64(p.ID) == pawnN {
			next = windowspkg.NextPriority(windowspkg.WorkPriority(p, col))
		}
	}
	a.renderer.enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: pawnN, Work: wt, Count: next})
}

const work3 = 3

// NewApp returns an App on the boot screen. Simulation is not created
// until Start is pressed.
func NewApp(cfg AppConfig) *App {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.LoadArt == nil {
		cfg.LoadArt = func() (*artpkg.Catalog, error) {
			root, err := artpkg.Locate()
			if err != nil {
				return nil, err
			}
			c, err := loadArtCatalog(root)
			if err == nil {
				c.PrewarmAsync() // art_atlas.go: pages cut in the background, not mid-frame
			}
			return c, err
		}
	}
	if cfg.LoadSplash == nil {
		cfg.LoadSplash = func() (*ebiten.Image, error) {
			path, err := mainmenu.LocateMenuSplash()
			if err != nil {
				return nil, err
			}
			return mainmenu.LoadPNG(path)
		}
	}
	if cfg.LoadMenuScene == nil {
		cfg.LoadMenuScene = func() (*mainmenu.Diorama, error) {
			root, err := artpkg.Locate()
			if err != nil {
				return nil, err
			}
			return mainmenu.LoadDiorama(root)
		}
	}
	if cfg.Sound == nil {
		cfg.Sound = noSound{}
	}
	if cfg.WindowHooks == nil {
		cfg.WindowHooks = ebitenWindow{}
	}
	if cfg.Background == nil {
		cfg.Background = ebitenBG{}
	}
	a := &App{
		cfg: cfg,
		now: time.Now,
		nav: newNav(),
		sw:  DefaultWindowWidth,
		sh:  DefaultWindowHeight,

		scale:  gfx.UnitScale,
		devRaw: 1,
	}
	a.shell = newShell(a)
	a.sel = selection.New(a.shell, selHost{a})
	a.menuScreen = mainmenu.New(menuHost{a})
	a.wiz = wizard.New(wizHost{a})
	a.opt, a.win, a.arch, a.ord = newOptions(a), newWindows(a), newArchitect(a), newOrders(a)
	a.deal, a.lrn, a.tools, a.meter = newDialogs(a), newLearn(a), newDevtools(a), newPerf(a)
	a.insp, a.chrome = newInspector(a), newHud(a)
	a.loadSettings()
	a.applySettings()
	return a
}

// Update routes input, advances boot/colony, and applies screen intents.
// Input is polled once (pollInput) and each handler reads the resolved
// frame; the context stack makes one key or click do one thing.
func (a *App) Update() error {
	if a.shouldQuit {
		return ebiten.Termination
	}
	if a.fpsProbeUpdate() { // dev frame-rate probe (fps_probe.go)
		return ebiten.Termination
	}
	defer a.fpsProbeAfterUpdate()
	a.perfBeginUpdate() // perf_stats.go: the overlay's timing (PERF-002)
	defer a.perfEndUpdate()
	if a.handleWindowClose() { // commitment: save before the window closes (saves.go)
		return ebiten.Termination
	}
	a.updates++
	a.syncUIScale() // ui_scale.go: before any measure or hit test
	a.pollInput()
	a.syncBackground() // background.go: Run in background, from this frame's focus
	a.handleTextInput()
	if a.in.Fired(input.Cancel) {
		a.pressEscape()
		if a.shouldQuit {
			return ebiten.Termination
		}
	}
	if a.in.Fired(input.Accept) {
		a.pressAccept()
	}
	a.handleViewKeys() // F10 screenshot, F11 hide UI (hotkeys.go)

	sw, sh := a.size()
	if a.in.Clicked(inputframe.Left) {
		if a.nav.screen() == portspkg.ScreenBoot {
			a.finishBoot()
			a.in.Take(inputframe.Left)
		} else {
			mx, my := a.currentCursor()
			if b, ok := uikit.HitButton(a.layoutHits(sw, sh), mx, my); ok {
				a.in.Take(inputframe.Left)
				a.uiSound(b)
				if b.Disabled {
					a.hint = b.Reason
				} else {
					a.hint = ""
					a.applyClick(b.ID)
				}
			} else if a.colonyLive() && a.colonyChromeHit(mx, my, sw, sh) {
				a.in.Take(inputframe.Left)
			} else if a.colonyLive() && !a.chrome.Hidden && a.renderer.minimapClick(mx, my, camerapkg.ColonyViewport(sw, sh), a.mapViewport()) {
				a.in.Take(inputframe.Left) // minimap (minimap.go): the camera moved
			}
		}
	}

	if a.colonyLive() && !a.chrome.Hidden && a.in.Clicked(inputframe.Right) {
		if mx, my := a.currentCursor(); a.hudRightClick(mx, my) { // hud.go: letters, alerts, command options
			a.in.Take(inputframe.Right)
		}
	}

	if a.colonyLive() {
		a.insp.Commands().PruneGroup()
		a.handleColonyKeys() // hotkeys.go
		a.renderer.wheelTaken = a.insp.Pawn().HandleLogWheel()
		if a.cfg.Capture == nil { // captures frame the camera themselves
			a.renderer.applyCamera(a.mapViewport(), a.cameraInput(a.mapViewport())) // physical (camera.go)
		}
		a.win.Schedule().Drag()
		if (a.ord.Float.Open || a.tools.State.Menu) && a.in.Clicked(inputframe.Left) {
			a.ord.CloseFloat() // click outside a menu just closes it
			a.tools.State.Menu = false
			a.in.Take(inputframe.Left)
		}
		moving := a.arch.Mover.Handle() // move_mode.go: placing a moved building
		zoning := a.arch.Zones.Handle() // zone_select.go: a selected zone's Expand or Shrink tool
		a.arch.HandleInput()
		if a.arch.ToolID == "" && !moving && !zoning {
			a.handleMapMouse() // selection.go / multiselect.go / formation.go
		}
	}
	if a.nav.screen() != portspkg.ScreenColony && a.cfg.Capture == nil {
		a.menuTick++
	}
	if a.cfg.Capture != nil {
		a.tickCapture()
	} else {
		a.tick()
	}
	a.soundFrame()
	if a.shouldQuit && (a.cfg.Capture == nil || a.capture.done) {
		return ebiten.Termination
	}
	return nil
}

// Draw paints the current full screen and any overlay.
func (a *App) Draw(screen *ebiten.Image) {
	defer a.fpsProbeDraw(time.Now())
	defer a.capBackgroundDraw()  // background.go: ~20 fps while running in the background
	defer a.perfEndDraw(a.now()) // perf_stats.go: runs before the throttle sleeps (PERF-002)
	a.syncUIScale()              // ui_scale.go: the canvas and the font draw at this scale
	a.setPhysical(screen.Bounds().Dx(), screen.Bounds().Dy())
	sw, sh := a.sw, a.sh // logical

	switch a.nav.screen() {
	case portspkg.ScreenBoot:
		a.lastDraw = time.Time{}
		a.drawBoot(screen, sw, sh)
	case portspkg.ScreenMainMenu:
		a.lastDraw = time.Time{}
		a.menuScreen.Draw(screen, sw, sh)
	case portspkg.ScreenScenario:
		a.drawScenario(screen, sw, sh)
	case portspkg.ScreenScenarioEditor:
		a.drawScenarioEditor(screen, sw, sh)
	case portspkg.ScreenStoryteller:
		a.drawStoryteller(screen, sw, sh)
	case portspkg.ScreenWorldGen:
		a.drawWorldGen(screen, sw, sh)
	case portspkg.ScreenLanding:
		a.drawLanding(screen, sw, sh)
	case portspkg.ScreenPawns:
		a.drawPawns(screen, sw, sh)
	case portspkg.ScreenColony:
		if a.renderer != nil {
			now := a.now()
			if a.cfg.Capture == nil && !a.lastDraw.IsZero() && a.nav.overlay() != overlayPause {
				a.renderer.SyncDev() // the viewport is in this frame's map pixels
				a.renderer.stepCamera(now.Sub(a.lastDraw).Seconds(), a.mapViewport())
			}
			a.lastDraw = now
			a.renderer.mo.SetAlpha(a.frameAlpha())
			a.syncTreeFocus() // tree_focus.go: hover, selection and ghost see through canopies
			// world_scale.go: the map, and what the tools mark on it, draw
			// into the render-scaled offscreen when there is one; its canvas
			// labels are queued and drawn on the screen after it
			dst, scaled := a.worldTarget(screen)
			if scaled {
				a.beginWorldPass(dst)
			}
			gfx.SetPass("world")
			a.renderer.Draw(dst)
			a.tools.DrawMapFields(dst) // mapfields_overlay.go: dev only
			gfx.SetPass("hud")
			if !a.chrome.Hidden {
				a.drawMapOverlays(dst) // hud.go: selection marks, ghosts, the drag box
			}
			if scaled {
				a.endWorldPass()
				gfx.CompositeWorld(screen, dst, a.scale.WorldK())
				a.wr.q.Flush(screen)
			}
		} else {
			screen.Fill(uikit.Space)
		}
		if !a.chrome.Hidden {
			a.renderer.drawMinimap(screen, camerapkg.ColonyViewport(sw, sh), a.mapViewport()) // minimap.go: big maps only
			a.drawColonyHUD(screen, sw, sh)
		}
		a.drawPerfOverlay(screen, sw, sh) // perf_overlay.go: also with the UI hidden (PERF-002)
		if a.nav.overlay() == overlayPause {
			a.drawPause(screen, sw, sh)
		}
	case portspkg.ScreenOptions:
		a.lastDraw = time.Time{}
		a.opt.DrawOptions(screen, sw, sh)
	case portspkg.ScreenLoad:
		a.opt.DrawLoad(screen, sw, sh)
	case portspkg.ScreenCredits:
		a.drawCredits(screen, sw, sh)
	default:
		a.lastDraw = time.Time{}
		screen.Fill(uikit.Space)
	}

	a.uiSt().Paint(screen) // widget kit: widgets, modals, tooltips (ui_state.go)
	a.drawHoverHint(screen, sw, sh)
	if a.meter.Shot.Pending { // F10: this frame, before the toast that reports it
		a.meter.Shot.Pending = false
		a.meter.TakeScreenshot(func(img *image.RGBA) { screen.ReadPixels(img.Pix) }, sw, sh)
	}
	if !a.uiHidden() {
		a.chrome.DrawToasts(screen, sw, sh)
	}
	a.drawCapture(screen)
}
