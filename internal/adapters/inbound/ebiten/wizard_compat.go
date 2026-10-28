package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The New Colony wizard lives in wizard (ARCH-005). These aliases and
// one-line wrappers keep the root's call sites (the screen table, the
// click routes, the colony's Bio tab and Factions window, the tests)
// unchanged while the features move out one package at a time; the last
// mover deletes this file (SOLID_GO.md §11).

type (
	newGameState = wizard.Draft
	customDiff   = wizard.CustomDiff
	facRow       = wizard.FacRow
	scenEntry    = wizard.ScenEntry
)

const (
	backdropDiscArea   = wizard.BackdropDiscArea
	backdropDiscDist   = wizard.BackdropDiscDist
	backdropRenderGap  = wizard.BackdropRenderGap
	defaultCoverage    = wizard.DefaultCoverage
	defaultMapSide     = wizard.DefaultMapSide
	defaultStoryteller = wizard.DefaultStoryteller
	diffColGap         = wizard.DiffColGap
	diffModalH         = wizard.DiffModalH
	diffModalW         = wizard.DiffModalW
	diffTopRowH        = wizard.DiffTopRowH
	disabledNote       = wizard.DisabledNote
	diseaseFreqNote    = wizard.DiseaseFreqNote
	diseaseFreqTip     = wizard.DiseaseFreqTip
	facHighlight       = wizard.FacHighlight
	facModalID         = wizard.FacModalID
	facRowH            = wizard.FacRowH
	facTableID         = wizard.FacTableID
	maxSeedText        = wizard.MaxSeedText
	noHostileWarning   = wizard.NoHostileWarning
	preReleaseSide     = wizard.PreReleaseSide
	rowEntry           = wizard.RowEntry
	rowHeader          = wizard.RowHeader
	rowNote            = wizard.RowNote
	sparkDays          = wizard.SparkDays
	worldProgress      = wizard.WorldProgress
)

// The wizard's lists and the backdrop's one shared cache (drawn under its
// mutex), as the root's tests read them.
var (
	backdropShowcase = wizard.BackdropShowcase
	edItems          = wizard.EdItems
	saveModes        = wizard.SaveModes
	worldAdjectives  = wizard.WorldAdjectives
	worldNouns       = wizard.WorldNouns
	setupBackdrop    = &wizard.Backdrop
	setupBackdropMu  = &wizard.BackdropMu
)

// noTint draws a sprite untinted.
var noTint color.NRGBA

// The wizard's pure helpers.
var (
	addableTypes          = wizard.AddableTypes
	autoSeason            = wizard.AutoSeason
	buildDetail           = wizard.BuildDetail
	cloneDef              = wizard.CloneDef
	coordText             = wizard.CoordText
	copyForEditing        = wizard.CopyForEditing
	dayMean               = wizard.DayMean
	defaultMapSideEnabled = wizard.DefaultMapSideEnabled
	discSilhouette        = wizard.DiscSilhouette
	drawSilhouette        = wizard.DrawSilhouette
	factionRows           = wizard.FactionRows
	fnv64a                = wizard.Fnv64a
	forageLevel           = wizard.ForageLevel
	goodwillText          = wizard.GoodwillText
	goodwillTip           = wizard.GoodwillTip
	hostileCapable        = wizard.HostileCapable
	indexWhere            = wizard.IndexWhere
	landRects             = wizard.LandRects
	landingDetail         = wizard.LandingDetail
	mapSizeLabel          = wizard.MapSizeLabel
	mapSizeOptions        = wizard.MapSizeOptions
	modeCaption           = wizard.ModeCaption
	movementDifficulty    = wizard.MovementDifficulty
	pacingOf              = wizard.PacingOf
	pacingTip             = wizard.PacingTip
	pctText               = wizard.PctText
	presetTip             = wizard.PresetTip
	prettyKey             = wizard.PrettyKey
	randomSeed            = wizard.RandomSeed
	randomWorldPhrase     = wizard.RandomWorldPhrase
	relationColor         = wizard.RelationColor
	relationLabel         = wizard.RelationLabel
	resetSetupBackdrop    = wizard.ResetBackdrop
	rosterTarget          = wizard.RosterTarget
	rosterY               = wizard.RosterY
	scenarioLines         = wizard.ScenarioLines
	scenarioRects         = wizard.ScenarioRects
	seasonMid             = wizard.SeasonMid
	seedTextRune          = wizard.SeedTextRune
	setupSeedOf           = wizard.SetupSeedOf
	signedText            = wizard.SignedText
	skillLevelText        = wizard.SkillLevelText
	skillTip              = wizard.SkillTip
	slotIndex             = wizard.SlotIndex
	slugOf                = wizard.SlugOf
	storyRects            = wizard.StoryRects
	wildlifeLevel         = wizard.WildlifeLevel
	worldPhaseName        = wizard.WorldPhaseName
)

// buildTileInfo is the landing panel's info for tile i of p with a side x
// side map on a computer with total bytes of memory (0 unknown).
func buildTileInfo(p *planet.Planet, i, side int, total uint64) wizard.TileInfo {
	return wizard.BuildTileInfo(p, i, mapSizeNote(side, total))
}

// ---- the App's wizard methods

func (a *App) draft() *newGameState            { return a.wiz.Draft() }
func (a *App) applyWizardClick(id string) bool { return a.wiz.Click(id) }
func (a *App) coverageOptions() []int          { return a.wiz.CoverageOptions() }
func (a *App) poolKeyFor(ng *newGameState) string {
	return a.wiz.PoolKeyFor(ng)
}
func (a *App) worldGenerate() { a.wiz.WorldGenerate() }
func (a *App) worldStep()     { a.wiz.WorldStep() }

func (a *App) scenarioUI(sw, sh int)    { a.wiz.Declare(screenScenario, sw, sh) }
func (a *App) editorUI(sw, sh int)      { a.wiz.Declare(screenScenarioEditor, sw, sh) }
func (a *App) storytellerUI(sw, sh int) { a.wiz.Declare(screenStoryteller, sw, sh) }
func (a *App) worldGenUI(sw, sh int)    { a.wiz.Declare(screenWorldGen, sw, sh) }
func (a *App) landingUI(sw, sh int)     { a.wiz.Declare(screenLanding, sw, sh) }
func (a *App) pawnsUI(sw, sh int)       { a.wiz.Declare(screenPawns, sw, sh) }

func (a *App) scenarioButtons(sw, sh int) []button {
	return a.wiz.Buttons(screenScenario, sw, sh)
}
func (a *App) storytellerButtons(sw, sh int) []button {
	return a.wiz.Buttons(screenStoryteller, sw, sh)
}
func (a *App) worldGenButtons(sw, sh int) []button { return a.wiz.Buttons(screenWorldGen, sw, sh) }
func (a *App) landingButtons(sw, sh int) []button  { return a.wiz.Buttons(screenLanding, sw, sh) }
func (a *App) pawnButtons(sw, sh int) []button     { return a.wiz.Buttons(screenPawns, sw, sh) }

func (a *App) drawScenario(s *ebiten.Image, sw, sh int) { a.wiz.Draw(screenScenario, s, sw, sh) }
func (a *App) drawScenarioEditor(s *ebiten.Image, sw, sh int) {
	a.wiz.Draw(screenScenarioEditor, s, sw, sh)
}
func (a *App) drawStoryteller(s *ebiten.Image, sw, sh int) {
	a.wiz.Draw(screenStoryteller, s, sw, sh)
}
func (a *App) drawWorldGen(s *ebiten.Image, sw, sh int) { a.wiz.Draw(screenWorldGen, s, sw, sh) }
func (a *App) drawLanding(s *ebiten.Image, sw, sh int)  { a.wiz.Draw(screenLanding, s, sw, sh) }
func (a *App) drawPawns(s *ebiten.Image, sw, sh int)    { a.wiz.Draw(screenPawns, s, sw, sh) }
func (a *App) drawSetupBackdrop(s *ebiten.Image, sw, sh int, p *planet.Planet) {
	a.wiz.DrawBackdrop(s, sw, sh, p)
}

func (a *App) drawEmblem(dst *ebiten.Image, key string, tint color.NRGBA, x, y float32, size float64) {
	a.wiz.DrawEmblem(dst, key, tint, x, y, size)
}
func (a *App) paintStoryCard(dst *ebiten.Image, r rect, p storyteller.Profile, selected, hot bool) {
	a.wiz.PaintStoryCard(dst, r, p, selected, hot)
}
func (a *App) factionColumns(rows []facRow) []tableCol { return a.wiz.FactionColumns(rows) }
func (a *App) factionsTable(r rect, rows []facRow, sel *int, pick func(facRow)) {
	a.wiz.FactionsTable(r, rows, sel, pick)
}
func (a *App) backstoryLine(x, y, w float32, label, title, desc, id string) {
	a.wiz.BackstoryLine(x, y, w, label, title, desc, id)
}
func (a *App) traitChips(x, y, w float32, v game.CandidateView) float32 {
	return a.wiz.TraitChips(x, y, w, v)
}

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
		DevMode:     a.devMode(),
		MenuTick:    a.menuTick,
		SessionErr:  a.sessionErr,
		Now:         a.now(),
	}
}

func (h wizHost) KeepDraft(d *wizard.Draft)                 { h.a.ng = d }
func (h wizHost) SaveSettings(s config.Settings)            { h.a.setSettings(s) }
func (h wizHost) SliderTick(sp uikit.SliderSpec) sliderSpec { return h.a.withTick(sp) }
func (h wizHost) Load(s *game.Setup) (wizard.Loader, error) { return h.a.loadColony(s) }
func (h wizHost) MapSizeNote(side int) string               { return mapSizeNote(side, h.a.hardwareTotal()) }
func (h wizHost) DrawDiorama(dst *ebiten.Image, sw, sh int, dim float64) bool {
	return h.a.drawMenuDiorama(dst, sw, sh, dim)
}
func (h wizHost) DrawPortrait(dst *ebiten.Image, l look.Look, x, y, w, ht float64) bool {
	return drawLookPortrait(dst, h.a.art, l, x, y, w, ht)
}
