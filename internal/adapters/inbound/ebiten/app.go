package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
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
type SimFactory func() (Simulation, error)

// AppConfig is the composition-root wiring for the interactive client.
type AppConfig struct {
	Version  string
	Assets   string // empty: locate the vendor pack from cwd
	NewSim   SimFactory
	LoadPack func() (*atlas, error)
	// LoadArt loads the original generated sprites (assets/metadata).
	// Nil: locate from cwd. When it fails the vendor pack is used.
	LoadArt    func() (*artCatalog, error)
	LoadSplash func() (*ebiten.Image, error)
	// Capture, when set, runs a scripted screenshot session and quits.
	Capture *Capture
}

// App is the ebiten.Game shell: screen routing, widgets, and (once
// started) the colony renderer. Menus do not own or step a simulation.
type App struct {
	cfg AppConfig
	nav nav

	sw, sh     int
	bootFrames int
	packTried  bool
	pack       *atlas
	art        *artCatalog
	packWarn   string
	splash     *ebiten.Image
	sessionErr string
	hint       string
	shouldQuit bool

	scenario    int
	storyteller int
	difficulty  int

	simsStarted                    int
	renderer                       *Renderer
	workOpen                       bool
	archOpen                       bool
	archCat                        string
	archTool                       string
	archFacing                     uint8
	archStuff                      string // "wood" or "stone" for tools that allow both
	float                          floatMenu
	cursor                         *[2]int // capture-only synthetic cursor
	selBuilding                    entity.EntityID
	hasBuilding                    bool
	selAnimal                      entity.EntityID
	hasAnimal                      bool
	alertCycle                     map[string]int
	archDrag                       bool
	archX0, archY0, archX1, archY1 int
	inspectTab                     string
	capture                        captureState
}

// NewApp returns an App on the boot screen. Simulation is not created
// until Start is pressed.
func NewApp(cfg AppConfig) *App {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.LoadPack == nil {
		assets := cfg.Assets
		cfg.LoadPack = func() (*atlas, error) {
			if assets != "" {
				return loadAtlasFromRoot(assets)
			}
			root, err := LocatePack()
			if err != nil {
				return nil, err
			}
			return loadAtlasFromRoot(root)
		}
	}
	if cfg.LoadArt == nil {
		cfg.LoadArt = func() (*artCatalog, error) {
			root, err := LocateArt()
			if err != nil {
				return nil, err
			}
			return loadArtCatalog(root)
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
	return &App{
		cfg: cfg,
		nav: newNav(),
		sw:  DefaultWindowWidth,
		sh:  DefaultWindowHeight,
	}
}

func loadAtlasFromRoot(root string) (*atlas, error) {
	p, err := DiscoverPack(root)
	if err != nil {
		return nil, err
	}
	return loadAtlas(p)
}

// Update routes input, advances boot/colony, and applies screen intents.
func (a *App) Update() error {
	if a.shouldQuit {
		return ebiten.Termination
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.pressEscape()
		if a.shouldQuit {
			return ebiten.Termination
		}
	}

	sw, sh := a.size()
	consumed := false
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if a.nav.screen() == screenBoot {
			a.finishBoot()
			consumed = true
		} else {
			mx, my := ebiten.CursorPosition()
			if b, ok := hitButton(a.layoutButtons(sw, sh), mx, my); ok {
				consumed = true
				if b.disabled {
					a.hint = b.reason
				} else {
					a.hint = ""
					a.applyClick(b.id)
				}
			} else if a.nav.screen() == screenColony && a.nav.overlay() == overlayNone && a.colonyChromeHit(mx, my, sw, sh) {
				consumed = true
			}
		}
	}

	if a.nav.screen() == screenColony && a.nav.overlay() == overlayNone && a.renderer != nil {
		a.renderer.handleSpeedInput()
		a.renderer.handleCameraInput()
		if a.float.open && !consumed && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			a.closeFloat() // click outside the order menu just closes it
			consumed = true
		}
		a.handleArchitectInput(consumed)
		if a.archTool == "" {
			if !consumed && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				a.renderer.handleSelectionInput()
				if a.renderer.HasSelect() {
					a.clearBuilding()
					a.clearAnimal()
				} else {
					mx, my := a.currentCursor()
					tx, ty := a.renderer.TileAt(mx, my)
					if !a.selectAnimalAt(tx, ty) {
						a.selectBuildingAt(tx, ty)
					}
				}
			}
			a.handleOrders()
		}
	}
	if a.cfg.Capture != nil {
		a.tickCapture()
	} else {
		a.tick()
	}
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
	if a.float.open {
		a.closeFloat()
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
			a.renderer.advance()
		}
	}
}

func (a *App) tickBoot() {
	a.bootFrames++
	phase := a.bootPhase()
	if phase >= bootAssetPhase && !a.packTried {
		a.loadPack()
	}
	if phase >= len(bootPhases) {
		a.finishBoot()
	}
}

func (a *App) bootPhase() int {
	return a.bootFrames / bootFramesPerPhase
}

func (a *App) loadPack() {
	a.packTried = true
	if a.cfg.LoadPack != nil {
		p, err := a.cfg.LoadPack()
		if err != nil {
			a.packWarn = err.Error()
		} else {
			a.pack = p
		}
	}
	if a.cfg.LoadArt != nil {
		art, err := a.cfg.LoadArt()
		if err != nil {
			if a.packWarn == "" {
				a.packWarn = err.Error()
			}
		} else {
			a.art = art
		}
	}
	a.loadSplash()
}

func (a *App) finishBoot() {
	if !a.packTried {
		a.loadPack()
	}
	a.nav.reset(frame{screen: screenMainMenu})
}

func (a *App) applyClick(id string) {
	switch {
	case strings.HasPrefix(id, "bar.pawn."):
		n, err := strconv.ParseUint(strings.TrimPrefix(id, "bar.pawn."), 10, 64)
		if err == nil && a.renderer != nil {
			a.renderer.Select(entity.EntityID(n))
			a.clearBuilding()
		}
	case id == "tab.work":
		a.workOpen = !a.workOpen
		if a.workOpen {
			a.closeArchitect()
			a.closeFloat()
		}
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
	case strings.HasPrefix(id, "arch.tool."):
		a.selectArchTool(strings.TrimPrefix(id, "arch.tool."))
	case strings.HasPrefix(id, "float."):
		a.chooseFloat(id)
	case id == "pawn.draft":
		a.toggleDraft()
	case strings.HasPrefix(id, "animal."):
		a.applyAnimalClick(id)
	case strings.HasPrefix(id, "alert."):
		a.focusAlert(id)
	case strings.HasPrefix(id, "bldg.") || strings.HasPrefix(id, "bill."):
		a.applyBuildingClick(id)
	case strings.HasPrefix(id, "work.cell."):
		a.toggleWorkCell(id)
	case id == "tab.menu":
		a.apply(intent{kind: intentPause})
	case id == "inspector.close":
		if a.renderer != nil {
			a.renderer.ClearSelect()
		}
	case id == "inspector.character":
		a.inspectTab = "character"
	case id == "inspector.needs":
		a.inspectTab = "needs"
	case id == "scenario.0":
		a.scenario = 0
	case id == "scenario.1":
		a.scenario = 1
	case id == "scenario.2":
		a.scenario = 2
	case id == "storyteller.0":
		a.storyteller = 0
	case id == "storyteller.1":
		a.storyteller = 1
	case id == "storyteller.2":
		a.storyteller = 2
	case id == "difficulty.0":
		a.difficulty = 0
	case id == "difficulty.1":
		a.difficulty = 1
	case id == "difficulty.2":
		a.difficulty = 2
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
		a.nav.pop()
	case intentStartGame:
		a.startGame()
	case intentQuit:
		a.shouldQuit = true
	case intentReturnToMenu:
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

func (a *App) startGame() {
	if a.cfg.NewSim == nil {
		a.sessionErr = "no simulation factory"
		return
	}
	sim, err := a.cfg.NewSim()
	if err != nil {
		a.sessionErr = err.Error()
		return
	}
	a.sessionErr = ""
	a.simsStarted++
	a.workOpen = false
	a.closeArchitect()
	a.archCat = ""
	a.archFacing = 0
	a.closeFloat()
	a.inspectTab = "character"
	a.renderer = New(sim)
	a.renderer.pack = a.pack
	a.renderer.art = a.art
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
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	a.sw, a.sh = sw, sh

	switch a.nav.screen() {
	case screenBoot:
		a.drawBoot(screen, sw, sh)
	case screenMainMenu:
		a.drawMainMenu(screen, sw, sh)
	case screenScenario:
		a.drawScenario(screen, sw, sh)
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
			a.renderer.Draw(screen)
		} else {
			screen.Fill(uiSpace)
		}
		a.drawColonyHUD(screen, sw, sh)
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

	a.drawHoverHint(screen, sw, sh)
	a.drawCapture(screen)
}

func (a *App) drawHoverHint(screen *ebiten.Image, sw, sh int) {
	mx, my := ebiten.CursorPosition()
	if b, ok := hitButton(a.layoutButtons(sw, sh), mx, my); ok && b.disabled && b.reason != "" {
		ebitenutil.DebugPrintAt(screen, b.reason, 16, sh-22)
		return
	}
	if a.hint != "" {
		ebitenutil.DebugPrintAt(screen, a.hint, 16, sh-22)
	}
	if a.packWarn != "" && a.nav.screen() == screenMainMenu {
		ebitenutil.DebugPrintAt(screen, "asset warning: "+a.packWarn, 16, sh-40)
	}
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
	drawMenuBackdrop(screen, sw, sh)
	drawPanel(screen, 40, 24, float32(sw-80), 56)
	drawLabel(screen, 56, 44, title)
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
		y += 16
	}
}

func (a *App) footer(sw, sh int, nextLabel, nextID string) []button {
	y := footerY(sh)
	out := []button{{
		id:    "wizard.back",
		label: "Back",
		x:     40,
		y:     y,
		w:     140,
		h:     36,
	}}
	if nextID != "" {
		out = append(out, button{
			id:    nextID,
			label: nextLabel,
			x:     float32(sw - 180),
			y:     y,
			w:     140,
			h:     36,
		})
	}
	return out
}

// Layout uses the window size so menus and the colony viewport share one surface.
func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth < 1 {
		outsideWidth = DefaultWindowWidth
	}
	if outsideHeight < 1 {
		outsideHeight = DefaultWindowHeight
	}
	a.sw, a.sh = outsideWidth, outsideHeight
	return outsideWidth, outsideHeight
}

func formatVersion(v string) string {
	return fmt.Sprintf("rimworld-go  %s", v)
}

func (a *App) currentCursor() (int, int) {
	if a.cursor != nil {
		return a.cursor[0], a.cursor[1]
	}
	return ebiten.CursorPosition()
}

func (a *App) paintMenu(screen *ebiten.Image, sw, sh int, title string, body []string, buttons []button) {
	bx, _ := bodyOrigin(buttons, sh)
	a.fillMenu(screen, sw, sh, title, body, bx)
	mx, my := a.currentCursor()
	a.drawButtons(screen, buttons, mx, my)
}

func drawDim(screen *ebiten.Image, sw, sh int) {
	vector.FillRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)
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
	on := true
	for _, p := range a.renderer.snapshot() {
		if uint64(p.ID) != pawnN {
			continue
		}
		if w, ok := workLive[col]; ok {
			on = !w.on(p)
		}
	}
	a.renderer.enqueue(command.Command{
		Kind:    command.SetWorkEnabled,
		Pawn:    pawnN,
		Work:    wt,
		Enabled: on,
	})
}

func workCommand(col int) (command.WorkType, bool) {
	return workTypeForCol(col)
}
