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
}

// App is the ebiten.Game shell: screen routing, widgets, and (once
// started) the colony renderer. Menus do not own or step a simulation.
type App struct {
	// fps is the dev frame-rate probe (fps_probe.go).
	fps fpsProbeState
	// alerts memoises currentAlerts per update (alerts.go).
	alerts alertsMemo
	// ui is the widget kit's state (NG-1 §1.4, ui_state.go).
	ui  uiState
	cfg AppConfig
	nav nav

	// sw, sh are the logical screen size every layout and hit test reads;
	// pw, ph the physical one (sw x scale.s). outW, outH are the window in
	// points and devRaw the screen's device factor from the last LayoutF;
	// devFn replaces the monitor in tests (ui_scale.go).
	sw, sh     int
	pw, ph     int
	scale      uiScaleState
	outW, outH float64
	devRaw     float64
	devFn      func() float64
	bootFrames int
	artTried   bool
	art        *artCatalog
	artWarn    string
	// win is the window close hooks (saves.go), closeHandled what they
	// were last told.
	win          windowHooks
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
	selCycle                       selCycle        // last map click, for cycling a tile's stack
	bldgMove                       bldgMoveState   // the building panel's Move mode (move_mode.go)
	equipCache                     equipCacheState // the pawn Gear tab's nearby weapons (inspector.go)
	alertCycle                     map[string]int
	archDrag                       bool
	archX0, archY0, archX1, archY1 int
	inspectTab                     string
	logScroll, animalLogScroll     int // log_ui.go
	logFor, animalLogFor           entity.EntityID
	animalLogOpen                  bool
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

	// Input (UI-016): the device source (a fake in tests), this Update's
	// resolved frame, the live bindings and an Update counter.
	src     inputSource
	in      inputFrame
	binds   input.Bindings
	updates uint64
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
	a := &App{
		cfg: cfg,
		win: appWindow,
		now: time.Now,
		nav: newNav(),
		sw:  DefaultWindowWidth,
		sh:  DefaultWindowHeight,

		scale:  unitUIScale,
		devRaw: 1,
	}
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
	if a.handleWindowClose() { // commitment: save before the window closes (saves.go)
		return ebiten.Termination
	}
	a.updates++
	a.syncUIScale() // ui_scale.go: before any measure or hit test
	a.pollInput()
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
		a.handleArchitectInput()
		if a.archTool == "" && !moving {
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
	if a.hasBuilding {
		a.clearBuilding()
		return true
	}
	if a.hasFrame {
		a.clearFrame()
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
		if a.nav.overlay() == overlayNone && a.renderer != nil {
			a.renderer.voice.view = a.mapViewport() // animal calls only on screen (physical, like worldToScreen)
			a.renderer.advance()
			a.markUpdate()
			a.tickAutosave()
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

func (a *App) applyClick(id string) {
	if a.uiClick(id) { // widget kit ids (ui_state.go)
		return
	}
	switch {
	case id == roofButtonID:
		a.renderer.toggleRoofOverlayIfAny()
	case id == homeButtonID:
		a.renderer.toggleHomeOverlayIfAny()
	case id == homeAutoButtonID:
		a.toggleHomeAuto() // home_auto_ui.go
	case id == planButtonID:
		a.renderer.togglePlanOverlayIfAny()
	case id == roomsButtonID:
		a.rooms = !a.rooms
	case strings.HasPrefix(id, "group."):
		a.applyGroupClick(id) // multiselect.go
	case strings.HasPrefix(id, "search."):
		a.applySearchClick(id) // search_ui.go
	case strings.HasPrefix(id, "dev."):
		a.applyDevClick(id) // devmode.go
	case strings.HasPrefix(id, "bar.pawn."):
		n, err := strconv.ParseUint(strings.TrimPrefix(id, "bar.pawn."), 10, 64)
		if err == nil && a.renderer != nil {
			if a.in.shift() { // Shift+click on the bar adds or removes
				a.toggleInSet(selCand{selPawn, entity.EntityID(n)})
				break
			}
			a.group = selGroup{}
			a.renderer.Select(entity.EntityID(n))
			a.clearBuilding()
		}
	case strings.HasPrefix(id, "research.pick."):
		a.pickResearch(id)
	case strings.HasPrefix(id, "animals."):
		a.applyAnimalsTab(id)
	case id == "tab.work", id == "tab.schedule", id == "tab.research", id == "tab.animals":
		mode := strings.TrimPrefix(id, "tab.")
		if a.workOpen && a.panelMode == mode {
			a.workOpen = false
		} else {
			a.workOpen, a.panelMode = true, mode
			a.closeArchitect()
			a.closeFloat()
		}
	case strings.HasPrefix(id, "sched.brush."):
		if v, err := strconv.Atoi(strings.TrimPrefix(id, "sched.brush.")); err == nil {
			a.schedBrush = schedule.Activity(v)
		}
	case strings.HasPrefix(id, "sched.cell."):
		a.paintSchedule(id)
	case id == "tab.architect":
		if a.archOpen {
			a.closeArchitect()
		} else {
			a.openArchitect()
		}
	case strings.HasPrefix(id, "arch.cat."):
		a.selectArchCategory(strings.TrimPrefix(id, "arch.cat."))
	case strings.HasPrefix(id, "arch.stuff."):
		a.archStuff = strings.TrimPrefix(id, "arch.stuff.")
	case strings.HasPrefix(id, "arch.crop."):
		a.archCrop = strings.TrimPrefix(id, "arch.crop.")
	case strings.HasPrefix(id, "arch.tool."):
		a.selectArchTool(strings.TrimPrefix(id, "arch.tool."))
	case strings.HasPrefix(id, "float."):
		a.chooseFloat(id)
	case strings.HasPrefix(id, "load.slot."):
		if i, err := strconv.Atoi(strings.TrimPrefix(id, "load.slot.")); err == nil {
			a.loadSlot(i)
		}
	case id == "pause.savequit": // commitment: Save & Quit to Menu (saves.go)
		a.apply(intent{kind: intentReturnToMenu})
	case id == "pause.save":
		a.saveNow(a.saveSlot)
		a.apply(intent{kind: intentResume})
	case id == "pause.load", id == "menu.load":
		a.refreshSlots()
		a.apply(intent{kind: intentPush, screen: screenLoad})
	case id == "menu.continue":
		a.refreshSlots()
		if i := a.newestSlot(); i >= 0 { // the most recent save, commitment ones too
			a.loadSlot(i)
		}
	case id == "pawn.draft":
		a.toggleDraft()
	case id == "pawn.clearorders":
		if p, ok := a.selectedPawn(); ok {
			a.renderer.enqueue(command.Command{Kind: command.ClearOrders, Pawn: uint64(p.ID)})
		}
	case strings.HasPrefix(id, "animal."):
		a.applyAnimalClick(id)
	case strings.HasPrefix(id, "pawn.equip."):
		a.applyPawnEquipClick(id) // inspector.go
	case strings.HasPrefix(id, "plant."):
		a.applyPlantClick(id) // plant_inspector.go
	case strings.HasPrefix(id, "item."):
		a.applyItemClick(id) // item_inspector.go
	case strings.HasPrefix(id, "frame."):
		a.applyFrameClick(id) // frame_inspector.go
	case strings.HasPrefix(id, "msg."):
		a.focusMessage(id)
	case strings.HasPrefix(id, "alert."):
		a.focusAlert(id)
	case strings.HasPrefix(id, "bldg.") || strings.HasPrefix(id, "bill."):
		a.applyBuildingClick(id)
	case strings.HasPrefix(id, "work.cell."):
		a.toggleWorkCell(id)
	case id == "tab.menu":
		a.apply(intent{kind: intentPause})
	case strings.HasPrefix(id, "options.") && id != "options.back":
		a.applyOption(id)
	case id == "menu.options" || id == "pause.options":
		a.optionsTab = "" // always open on the first tab
		a.rebind = rebindState{}
		a.apply(a.clickIntent(id))
	case id == "inspector.close":
		if a.renderer != nil {
			a.renderer.ClearSelect()
		}
	case id == "inspector.character":
		a.inspectTab = "character"
	case id == "inspector.needs":
		a.inspectTab = "needs"
	case id == "inspector.gear":
		a.inspectTab = "gear"
	case id == "inspector.log":
		a.inspectTab = "log"
	case id == "inspector.health":
		a.inspectTab = "health"
	case id == "inspector.social":
		a.inspectTab = "social"
	case a.applyWizardClick(id):
	default:
		a.apply(a.clickIntent(id))
	}
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
	a.syncUIScale() // ui_scale.go: the canvas and the font draw at this scale
	a.setPhysical(screen.Bounds().Dx(), screen.Bounds().Dy())
	sw, sh := a.sw, a.sh // logical

	switch a.nav.screen() {
	case screenBoot:
		a.drawBoot(screen, sw, sh)
	case screenMainMenu:
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
			a.renderer.setAlpha(a.frameAlpha())
			a.syncTreeFocus() // tree_focus.go: hover, selection and ghost see through canopies
			a.renderer.Draw(screen)
			a.drawMapFields(screen) // mapfields_overlay.go: dev only
		} else {
			screen.Fill(uiSpace)
		}
		if !a.hideUI {
			a.renderer.drawMinimap(screen, colonyViewport(sw, sh), a.mapViewport()) // minimap.go: big maps only
			a.drawColonyHUD(screen, sw, sh)
		}
		if a.nav.overlay() == overlayPause {
			a.drawPause(screen, sw, sh)
		}
	case screenOptions:
		a.drawOptions(screen, sw, sh)
	case screenLoad:
		a.drawLoad(screen, sw, sh)
	case screenCredits:
		a.drawCredits(screen, sw, sh)
	default:
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
	y := int(hudBarY(sh)) - h - 4
	drawPanel(screen, 8, float32(y), float32(w), float32(h))
	drawLabel(screen, 16, y+4, s)
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

// mapCursor is the pointer in physical pixels, for the map (screenToWorld,
// selection, globe picking): the raw device position, or a.cursor x s.
func (a *App) mapCursor() (int, int) {
	if a.cursor != nil {
		s := a.scale.s
		return int(float64(a.cursor[0]) * s), int(float64(a.cursor[1]) * s)
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
