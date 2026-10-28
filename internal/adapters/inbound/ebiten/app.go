package ebiten

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
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
	LoadArt    func() (*artCatalog, error)
	LoadSplash func() (*ebiten.Image, error)
	// Scenarios stores custom scenarios (nil: custom scenarios unavailable).
	Scenarios ScenarioStore
	// FPSProbe runs a dev frame-rate measurement and quits (fps_probe.go).
	FPSProbe *FPSProbe
	// NewSimBuilder builds a wizard Setup colony step by step for the
	// loading screen (nil: NewSim in one step).
	NewSimBuilder func(NewGame) (SimBuilder, error)
	// LoadMenuScene loads the main-menu colony diorama (make art-menu).
	// Nil: locate from cwd. When it fails the gradient backdrop is drawn.
	LoadMenuScene func() (*menuDiorama, error)
	// Capture, when set, runs a scripted screenshot session and quits.
	Capture *Capture
	// Saves connects save/load to storage (zero value: disabled).
	Saves SaveHooks
	// Settings loads and stores the Options (zero value: defaults, not saved).
	Settings SettingsHooks
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
	dealings dealingsUI // trade_ui.go, comms_ui.go: the open trade/comms dialogs
	// hst is the colony HUD regions' own state (PLAY-018, hud.go): each
	// region's owner keeps its fields in its own struct type.
	hst hudStates
	// fps is the dev frame-rate probe (fps_probe.go).
	fps fpsProbeState
	// perf is the always-on frame timing behind the performance overlay
	// and debt the Update time budget's carried ticks (PERF-002).
	perf perfClock
	debt tickDebt
	// alerts memoises currentAlerts per update (alerts.go).
	alerts alertsMemo
	// ui is the widget kit's state (NG-1 §1.4, ui_state.go).
	ui  uiState
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
	scale      uiScaleState
	wr         worldRender // world_scale.go: the render-scaled map (PERF-002)
	outW, outH float64
	devRaw     float64
	devFn      func() float64
	bootFrames int
	artTried   bool
	art        *artCatalog
	artWarn    string
	// closeHandled is what the window close hooks (cfg.WindowHooks,
	// saves.go) were last told.
	closeHandled bool
	// treeFocus / treeSel are reused buffers for the see-through canopy
	// focus (tree_focus.go).
	treeFocus, treeSel [][2]int
	splash             *ebiten.Image
	menu               *menuDiorama
	menuTried          bool
	menuTick           uint64 // drives menu motion; counts while no colony is shown
	sessionErr         string
	hint               string
	shouldQuit         bool

	// ng is the New Colony wizard's draft (newgame_state.go); nil outside
	// the wizard. ngUnsaved are the custom scenarios made this session and
	// not saved to a file.
	ng          *newGameState
	ngUnsaved   unsavedDefs
	animalsWild bool               // the Animals tab shows the Wild list
	wildRows    []game.WildlifeRow // g.Wildlife(), read once per tick
	wildTick    clock.Tick
	wildRead    bool
	// settings are the player's Options (settings.go).
	settings   config.Settings
	optionsTab string
	snd        soundDirector // what is heard (sound_director.go)

	simsStarted                    int
	renderer                       *Renderer
	workOpen                       bool
	archOpen                       bool
	archCat                        string
	archTool                       string
	archFacing                     uint8
	archStuff                      string // picked material ("wood", "granite blocks", "steel", a carpet)
	archCrop                       string // crop the growing-zone tool sows
	float                          floatMenu
	cursor                         *[2]int // capture-only synthetic cursor
	selBuilding                    entity.EntityID
	hasBuilding                    bool
	selAnimal                      entity.EntityID
	hasAnimal                      bool
	selPlant, selItem              entity.EntityID // plant and item panels (selection.go)
	hasPlant, hasItem              bool
	selFrame                       entity.EntityID // blueprint/frame panel (frame_inspector.go)
	hasFrame                       bool
	zoneSel                        uint16 // the selected zone's id (zone_select.go, PLAY-020)
	hasZone                        bool
	zone                           *zoneState      // the selected zone's tools and last clicked cell (zone_tools.go)
	selCycle                       selCycle        // last map click, for cycling a tile's stack
	bldgMove                       bldgMoveState   // the building panel's Move mode (move_mode.go)
	equipCache                     equipCacheState // the pawn Gear tab's nearby weapons (inspector.go)
	alertCycle                     map[string]int
	archDrag                       bool
	archX0, archY0, archX1, archY1 int
	inspectTab                     string
	logScroll                      int // log_ui.go
	logFor, animalLogFor           entity.EntityID
	capture                        captureState
	msgSeen                        map[string]uint64 // message key -> frame first shown
	saveSlot                       string            // current colony's save slot
	autosaveDay                    int               // in-game day of the last autosave
	slots                          []SaveSlot        // load screen list
	slotsRead                      bool              // main menu read the list once
	panelMode                      string            // "work" or "schedule" while workOpen
	schedBrush                     schedule.Activity // activity the schedule paints
	lastPainted                    string            // schedule cell last painted by a drag
	now                            func() time.Time  // wall clock for alpha (injectable for tests)
	lastTick                       time.Time         // when the renderer last advanced
	lastDraw                       time.Time         // previous colony Draw: the per-Draw camera step

	// Input (UI-016): the device source (a fake in tests), this Update's
	// resolved frame, the live bindings and an Update counter.
	src     inputSource
	in      inputFrame
	binds   input.Bindings
	updates uint64
	bg      bgState       // Run in background (background.go)
	barClk  barClick      // last colonist-bar click, for the double click (hud.go)
	rebind  rebindState   // Options › Controls key capture (controls_ui.go)
	search  searchState   // Z search box (search_ui.go)
	form    formationDrag // drafted colonists' right-drag line (formation.go)
	group   selGroup      // multi-selection (multiselect.go)
	press   pressState    // the map left press: drag box and double click
	midDrag midDragState  // middle-mouse camera drag
	dev     devState      // developer mode panels (devmode.go)
	toasts  []toast       // short notices (toast.go)
	hideUI  bool          // F11: only the map is drawn
	rooms   bool          // G: room stats overlay (room_stats.go)
	// archStyle is the drag tool's draw style (0: the tool's default).
	archStyle drawStyle
	archCtrl  bool      // Ctrl held: the drag box is square (drawstyle.go)
	shot      shotState // F10 screenshot (screenshot.go)
}

// NewApp returns an App on the boot screen. Simulation is not created
// until Start is pressed.
func NewApp(cfg AppConfig) *App {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.LoadArt == nil {
		cfg.LoadArt = func() (*artCatalog, error) {
			root, err := LocateArt()
			if err != nil {
				return nil, err
			}
			c, err := loadArtCatalog(root)
			if err == nil {
				c.prewarmAsync() // art_atlas.go: pages cut in the background, not mid-frame
			}
			return c, err
		}
	}
	if cfg.LoadSplash == nil {
		cfg.LoadSplash = func() (*ebiten.Image, error) {
			path, err := LocateMenuSplash()
			if err != nil {
				return nil, err
			}
			return loadPNG(path)
		}
	}
	if cfg.LoadMenuScene == nil {
		cfg.LoadMenuScene = func() (*menuDiorama, error) {
			root, err := LocateArt()
			if err != nil {
				return nil, err
			}
			return loadMenuDiorama(root)
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

		scale:  unitUIScale,
		devRaw: 1,
	}
	a.shell = newShell(a)
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
	if a.in.fired(input.Cancel) {
		a.pressEscape()
		if a.shouldQuit {
			return ebiten.Termination
		}
	}
	if a.in.fired(input.Accept) {
		a.pressAccept()
	}
	a.handleViewKeys() // F10 screenshot, F11 hide UI (hotkeys.go)

	sw, sh := a.size()
	if a.in.clicked(mbLeft) {
		if a.nav.screen() == screenBoot {
			a.finishBoot()
			a.in.take(mbLeft)
		} else {
			mx, my := a.currentCursor()
			if b, ok := hitButton(a.layoutHits(sw, sh), mx, my); ok {
				a.in.take(mbLeft)
				a.uiSound(b)
				if b.disabled {
					a.hint = b.reason
				} else {
					a.hint = ""
					a.applyClick(b.id)
				}
			} else if a.colonyLive() && a.colonyChromeHit(mx, my, sw, sh) {
				a.in.take(mbLeft)
			} else if a.colonyLive() && !a.hideUI && a.renderer.minimapClick(mx, my, colonyViewport(sw, sh), a.mapViewport()) {
				a.in.take(mbLeft) // minimap (minimap.go): the camera moved
			}
		}
	}

	if a.colonyLive() && !a.hideUI && a.in.clicked(mbRight) {
		if mx, my := a.currentCursor(); a.hudRightClick(mx, my) { // hud.go: letters, alerts, command options
			a.in.take(mbRight)
		}
	}

	if a.colonyLive() {
		a.pruneGroup()
		a.handleColonyKeys() // hotkeys.go
		a.renderer.wheelTaken = a.handleLogWheel()
		if a.cfg.Capture == nil { // captures frame the camera themselves
			a.renderer.applyCamera(a.mapViewport(), a.cameraInput(a.mapViewport())) // physical (camera.go)
		}
		a.handleScheduleDrag()
		if (a.float.open || a.dev.menu) && a.in.clicked(mbLeft) {
			a.closeFloat() // click outside a menu just closes it
			a.dev.menu = false
			a.in.take(mbLeft)
		}
		moving := a.handleMoveMode() // move_mode.go: placing a moved building
		zoning := a.handleZoneTool() // zone_select.go: a selected zone's Expand or Shrink tool
		a.handleArchitectInput()
		if a.archTool == "" && !moving && !zoning {
			a.handleMapMouse() // selection.go / multiselect.go / formation.go
		}
	}
	if a.nav.screen() != screenColony && a.cfg.Capture == nil {
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

func (a *App) pressEscape() {
	if a.handleColonyEscape() {
		return
	}
	a.apply(a.nav.escapeIntent())
}

func (a *App) handleColonyEscape() bool {
	if a.nav.screen() != screenColony || a.nav.overlay() != overlayNone {
		return false
	}
	if a.hideUI {
		a.hideUI = false // Esc brings the interface back first
		return true
	}
	if a.float.open {
		a.closeFloat()
		return true
	}
	if a.dev.menu {
		a.dev.menu = false
		return true
	}
	if a.form.on {
		a.form = formationDrag{}
		return true
	}
	if a.bldgMove.on {
		a.bldgMove.on = false // Move mode takes Esc alone (UI-016)
		return true
	}
	if a.escapeZoneTool() { // zone_select.go: a zone's Expand or Shrink tool
		return true
	}
	if a.archOpen && a.archTool != "" {
		a.archTool = ""
		a.archDrag = false
		return true
	}
	if a.archOpen {
		a.closeArchitect()
		return true
	}
	if a.workOpen {
		a.workOpen = false
		return true
	}
	if a.closeLearnCard() { // learn_ui.go: the lesson detail card goes first
		return true
	}
	if a.closePaneTab() { // hud_inspect.go: an open inspector tab goes before the selection
		return true
	}
	if a.hasBuilding {
		a.clearBuilding()
		return true
	}
	if a.hasFrame {
		a.clearFrame()
		return true
	}
	if a.hasZone {
		a.clearZone() // zone_select.go
		return true
	}
	if a.grouped() {
		a.clearSelections(selNone)
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
	case screenBoot:
		a.tickBoot()
	case screenColony:
		if a.nav.overlay() == overlayNone && a.renderer != nil && !a.bg.paused { // background.go
			a.renderer.voice.view = a.mapViewport() // animal calls only on screen (physical, like worldToScreen)
			a.advanceColony()                       // update_budget.go: steps under the Update budget, stamps the Update
			a.tickAutosave()
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
	a.loadSplash()
	a.loadMenuScene()
}

func (a *App) finishBoot() {
	if !a.artTried {
		a.loadAssets()
	}
	a.nav.reset(frame{screen: screenMainMenu})
}

// applyClick runs a clicked id: the widget kit first, then the first
// route whose id or prefix matches (click_routes.go), else the screen
// intent the id names.
func (a *App) applyClick(id string) {
	if a.uiClick(id) { // widget kit ids (ui_state.go)
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
		return intent{kind: intentPush, screen: screenScenario}
	case "menu.load":
		return intent{kind: intentPush, screen: screenLoad}
	case "menu.options":
		return intent{kind: intentPush, screen: screenOptions}
	case "menu.credits":
		return intent{kind: intentPush, screen: screenCredits}
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
		return intent{kind: intentPush, screen: screenOptions}
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
		if !a.saveBeforeLeaving() {
			return
		}
		a.shouldQuit = true
	case intentReturnToMenu:
		if !a.saveBeforeLeaving() {
			return
		}
		a.dropSession()
		a.nav.reset(frame{screen: screenMainMenu})
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
	return math.Max(0, math.Min(f, alphaMax))
}

// startGame starts the legacy colony (no wizard draft): the configured
// seed and biome. The New Colony wizard starts through beginLoading.
func (a *App) startGame() {
	seed := a.cfg.DefaultSeed
	if seed == 0 {
		seed = randomSeed()
	}
	b, err := a.simBuilder(NewGame{Seed: seed, Biome: a.cfg.DefaultBiome})
	if err != nil {
		a.sessionErr = err.Error()
		return
	}
	for {
		if _, _, done := b.Step(loadUnits); done {
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
	a.workOpen = false
	a.closeArchitect()
	a.archCat = ""
	a.archFacing = 0
	a.closeFloat()
	a.inspectTab = "character"
	a.renderer = New(sim)
	a.renderer.soundHas = a.cfg.Sound.Has // animal voices fall back by what exists
	a.renderer.art = a.art
	a.syncRenderer()
	a.nav.reset(frame{screen: screenColony})
}

func (a *App) dropSession() {
	a.renderer = nil
	a.workOpen = false
	a.archOpen = false
	a.archTool = ""
	a.archDrag = false
	a.inspectTab = "character"
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
	case screenBoot:
		a.lastDraw = time.Time{}
		a.drawBoot(screen, sw, sh)
	case screenMainMenu:
		a.lastDraw = time.Time{}
		a.drawMainMenu(screen, sw, sh)
	case screenScenario:
		a.drawScenario(screen, sw, sh)
	case screenScenarioEditor:
		a.drawScenarioEditor(screen, sw, sh)
	case screenStoryteller:
		a.drawStoryteller(screen, sw, sh)
	case screenWorldGen:
		a.drawWorldGen(screen, sw, sh)
	case screenLanding:
		a.drawLanding(screen, sw, sh)
	case screenPawns:
		a.drawPawns(screen, sw, sh)
	case screenColony:
		if a.renderer != nil {
			now := a.now()
			if a.cfg.Capture == nil && !a.lastDraw.IsZero() && a.nav.overlay() != overlayPause {
				a.renderer.syncDev() // the viewport is in this frame's map pixels
				a.renderer.stepCamera(now.Sub(a.lastDraw).Seconds(), a.mapViewport())
			}
			a.lastDraw = now
			a.renderer.setAlpha(a.frameAlpha())
			a.syncTreeFocus() // tree_focus.go: hover, selection and ghost see through canopies
			// world_scale.go: the map, and what the tools mark on it, draw
			// into the render-scaled offscreen when there is one; its canvas
			// labels are queued and drawn on the screen after it
			dst, scaled := a.worldTarget(screen)
			if scaled {
				a.beginWorldPass(dst)
			}
			setTracePass("world")
			a.renderer.Draw(dst)
			a.drawMapFields(dst) // mapfields_overlay.go: dev only
			setTracePass("hud")
			if !a.hideUI {
				a.drawMapOverlays(dst) // hud.go: selection marks, ghosts, the drag box
			}
			if scaled {
				a.endWorldPass()
				compositeWorld(screen, dst, a.scale.WorldK())
				a.flushWorldOverlay(screen)
			}
		} else {
			screen.Fill(uiSpace)
		}
		if !a.hideUI {
			a.renderer.drawMinimap(screen, colonyViewport(sw, sh), a.mapViewport()) // minimap.go: big maps only
			a.drawColonyHUD(screen, sw, sh)
		}
		a.drawPerfOverlay(screen, sw, sh) // perf_overlay.go: also with the UI hidden (PERF-002)
		if a.nav.overlay() == overlayPause {
			a.drawPause(screen, sw, sh)
		}
	case screenOptions:
		a.lastDraw = time.Time{}
		a.drawOptions(screen, sw, sh)
	case screenLoad:
		a.drawLoad(screen, sw, sh)
	case screenCredits:
		a.drawCredits(screen, sw, sh)
	default:
		a.lastDraw = time.Time{}
		screen.Fill(uiSpace)
	}

	a.uiPaint(screen) // widget kit: widgets, modals, tooltips (ui_state.go)
	a.drawHoverHint(screen, sw, sh)
	if a.shot.pending { // F10: this frame, before the toast that reports it
		a.shot.pending = false
		a.takeScreenshot(func(img *image.RGBA) { screen.ReadPixels(img.Pix) }, sw, sh)
	}
	if !a.uiHidden() {
		a.drawToasts(screen, sw, sh)
	}
	a.drawCapture(screen)
}

// uiHidden reports F11 hid the interface (the colony map only).
func (a *App) uiHidden() bool { return a.hideUI && a.nav.screen() == screenColony }

func (a *App) drawHoverHint(screen *ebiten.Image, sw, sh int) {
	if a.uiHidden() {
		return
	}
	mx, my := a.currentCursor()
	if b, ok := hitButton(a.layoutHits(sw, sh), mx, my); ok {
		if b.disabled && b.reason != "" {
			a.drawBottomHint(screen, b.reason, sh)
			return
		}
		if b.tip != "" { // a small card by the cursor, clear of the tab strip
			tw, _ := measure(b.tip, tsBody)
			w := int(tw) + 16
			ch := int(lineH(tsBody)) + 6
			x, y := min(mx+14, sw-w-4), my+20
			if y+ch+2 > sh {
				y = my - ch - 8
			}
			drawPanel(screen, float32(x), float32(y), float32(w), float32(ch))
			drawText(screen, b.tip, float64(x+8), float64(y+3), tsBody)
		}
	}
	if a.hint != "" {
		a.drawBottomHint(screen, a.hint, sh)
	}
	if a.artWarn != "" && a.nav.screen() == screenMainMenu {
		debugPrintAt(screen, "asset warning: "+a.artWarn, 16, sh-40)
	}
}

// drawBottomHint prints a one-line hint at the bottom left: on the footer
// row of a menu screen, or on a card just above the colonist bar so it
// does not run over the tab strip.
func (a *App) drawBottomHint(screen *ebiten.Image, s string, sh int) {
	if a.nav.screen() != screenColony {
		debugPrintAt(screen, s, 16, sh-22)
		return
	}
	w := int(textW(s, tsBody)) + 16
	h := int(lineStep(tsBody)) + 4
	hx, hb := a.hintAnchor() // hud_commands.go: above the command buttons or the pane
	x, y := int(hx), int(hb)-h
	drawPanel(screen, float32(x), float32(y), float32(w), float32(h))
	drawLabel(screen, x+8, y+4, s)
}

func (a *App) layoutButtons(sw, sh int) []button {
	if a.nav.overlay() == overlayPause {
		return a.pauseButtons(sw, sh)
	}
	switch a.nav.screen() {
	case screenMainMenu:
		return a.mainMenuButtons(sw, sh)
	case screenScenario:
		return a.scenarioButtons(sw, sh)
	case screenStoryteller:
		return a.storytellerButtons(sw, sh)
	case screenWorldGen:
		return a.worldGenButtons(sw, sh)
	case screenLanding:
		return a.landingButtons(sw, sh)
	case screenPawns:
		return a.pawnButtons(sw, sh)
	case screenOptions:
		return a.optionsButtons(sw, sh)
	case screenLoad:
		return a.loadButtons(sw, sh)
	case screenCredits:
		return a.creditsButtons(sw, sh)
	case screenColony:
		return a.colonyButtons(sw, sh)
	default:
		return nil
	}
}

func (a *App) drawButtons(screen *ebiten.Image, buttons []button, mx, my int) {
	for _, b := range buttons {
		drawButton(screen, b, b.contains(mx, my))
	}
}

func footerY(sh int) float32 {
	return float32(sh - footerYOff)
}

func isFooterButton(b button, sh int) bool {
	return b.y >= footerY(sh)-1
}

// bodyOrigin puts copy to the right of left-rail buttons so it is not
// drawn under them. Footer Back/Next are ignored.
func bodyOrigin(buttons []button, sh int) (x, y int) {
	x, y = bodyDefaultX, bodyDefaultY
	maxRight := 0
	for _, b := range buttons {
		if isFooterButton(b, sh) {
			continue
		}
		if right := int(b.x + b.w); right > maxRight {
			maxRight = right
		}
	}
	if maxRight > 0 {
		x = maxRight + bodyRailGap
	}
	return x, y
}

func (a *App) fillMenu(screen *ebiten.Image, sw, sh int, title string, body []string, bodyX int) {
	if !a.drawMenuDiorama(screen, sw, sh, 0.55) {
		drawMenuBackdrop(screen, sw, sh)
	}
	drawPanel(screen, 40, 24, float32(sw-80), 56)
	drawText(screen, title, 56, 40, tsLarge)
	if len(body) == 0 {
		return
	}
	if bodyX < 1 {
		bodyX = bodyDefaultX
	}
	drawPanel(screen, 40, 96, float32(sw-80), float32(sh-180))
	y := bodyDefaultY
	for _, line := range body {
		drawLabel(screen, bodyX, y, line)
		y += int(lineH(tsBody))
	}
}

func formatVersion(v string) string {
	return fmt.Sprintf("rimworld-go  %s", v)
}

// currentCursor is the pointer in logical UI pixels: what the widget kit,
// layoutHits, hitButton and every HUD/menu hit test read. a.cursor
// (captures, tests) is logical.
func (a *App) currentCursor() (int, int) {
	if a.cursor != nil {
		return a.cursor[0], a.cursor[1]
	}
	return a.in.ui.mx, a.in.ui.my
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
		return int(float64(a.in.raw.mx) * k), int(float64(a.in.raw.my) * k)
	}
	return a.in.raw.mx, a.in.raw.my
}

func (a *App) paintMenu(screen *ebiten.Image, sw, sh int, title string, body []string, buttons []button) {
	bx, _ := bodyOrigin(buttons, sh)
	a.fillMenu(screen, sw, sh, title, body, bx)
	mx, my := a.currentCursor()
	a.drawButtons(screen, buttons, mx, my)
}

func drawDim(screen *ebiten.Image, sw, sh int) {
	uiFillRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)
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
	wt, ok := workCommand(col)
	if !ok {
		return
	}
	next := work3
	for _, p := range a.renderer.snapshot() {
		if uint64(p.ID) == pawnN {
			next = nextPriority(workPriority(p, col))
		}
	}
	a.renderer.enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: pawnN, Work: wt, Count: next})
}

const work3 = 3

func workCommand(col int) (command.WorkType, bool) {
	return workTypeForCol(col)
}
