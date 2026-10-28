// Package ebiten adapts the headless application game to an interactive
// Ebitengine client: screen routing plus a colony map viewport. It never
// mutates domain state directly; it only calls Step/Snapshot/World and
// Enqueue on a consumer-owned Simulation interface.
package ebiten

import (
	"fmt"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Simulation is the behavior this renderer needs from the application
// game. It is consumer-owned (defined here, not by package game) so the
// renderer depends on an abstraction rather than a concrete type — any
// implementation satisfying it can be swapped in without touching this
// adapter.
type Simulation interface {
	// TemperatureAt is the air temperature on a tile, in tenths of a degree C
	// (rooms, weather, vents, fire): the lower-right temperature line
	// follows the cursor with it (PLAY-018).
	TemperatureAt(x, y int) int
	Step(ticks int)
	Snapshot() []game.PawnView
	World() game.WorldView
	Enqueue(command.Command)
	Width() int
	Height() int
	Tick() clock.Tick
	Date() clock.Date
	Activity() schedule.Activity
	// CanPlaceAt is the read-only placement rule the Architect previews
	// (material, facing and every footprint cell).
	CanPlaceAt(kind, stuff, facing string, x, y int) (bool, string)
	// ReplacesAt names the wall or fence a door or gate would replace
	// there ("" when none): the ghost reads "replaces wall".
	ReplacesAt(kind, stuff string, x, y int) string
	// AutoFacingAt is the facing the sim would give a piece placed here
	// (doors and gates face along their walls, PLAY-011).
	AutoFacingAt(kind, facing string, x, y int) string
	// OrdersAt lists what a pawn can be ordered to do at a tile.
	OrdersAt(pawn entity.EntityID, x, y int) []game.OrderOption
	// CanZoneAt reports whether a zone of kind ("stockpile", "grow",
	// "pen") could claim a tile: the zone drag ghost per cell.
	CanZoneAt(kind string, x, y int) bool
	// Wildlife lists the wild species on the map (the Animals tab's Wild
	// list).
	Wildlife() []game.WildlifeRow
	// NG-1: the Factions window, storyteller/difficulty and a colonist's Bio.
	Factions() []game.FactionView
	Story() game.StoryView
	Bio(id entity.EntityID) (game.BioView, bool)
	// NG-1 colony UI (UI-COLONY): the building catalog with the scenario's
	// locks (BuildableView.Locked), the Work tab's "Incapable: <why>" for a
	// colonist's work type ("" when it can) and the Bio tab's experience
	// gained in the last 7 days per skill.
	Buildables() []game.BuildableView
	IncapableReason(id entity.EntityID, t work.Type) string
	LearnedThisWeek(id entity.EntityID) [skill.NumSkills]int
	// NG-1 E5 renderer read models (chunk_view.go, minimap_view.go): a
	// chunk's share of World() and its bake versions, the plants in a tile
	// rectangle, and the minimap layer when it changed since ver.
	ChunkLayer(cx, cy int) game.ChunkView
	PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32
	MinimapLayer(ver uint64) (game.MinimapView, bool)
	// PlantGrowth is why a plant is or isn't growing now and when it
	// ripens (PLAY-017): the plant inspector asks for the one it shows
	// (it follows the clock, so it is not on every PlantView).
	PlantGrowth(id entity.EntityID) (game.PlantGrowthView, bool)
	// ENG-003 social and host-system read models: a colonist's Social and
	// Health tabs, a prisoner's tab (ok false: nothing to show), the World
	// tab, the allowed areas and the colony History.
	Social(id entity.EntityID) (game.SocialView, bool)
	Health(id entity.EntityID) (game.HealthView, bool)
	Prisoner(id entity.EntityID) (game.PrisonerView, bool)
	WorldMap() game.WorldMapView
	Areas() []game.AreaView
	History() []game.HistoryEntryView
}

var _ Simulation = (*game.Game)(nil)

// TileSize is the on-screen pixel size of one logical tile at zoom 1,
// matching the art-spec reference of 64 px/tile.
const TileSize = 64

const (
	panPixels = 8
	minZoom   = 0.1 // absolute floor; the map-fit limit is minZoomFor
	maxZoom   = 3.0
	zoomStep  = 0.1
)

// speed is a presentation-only game-speed tier. It controls how often
// the simulation advances; it never changes simulation rules.
type speed int

const (
	speedPaused speed = iota
	speedNormal
	speedFast
	speedVeryFast
	speedUltra // developer mode only (UI-016)
)

// ultraTicks is the developer ultrafast tier's ticks per frame.
const ultraTicks = 15

func (s speed) ticksPerFrame() int {
	switch s {
	case speedNormal:
		return 1
	case speedFast:
		return 3
	case speedVeryFast:
		return 6
	case speedUltra:
		return ultraTicks
	default:
		return 0
	}
}

func (s speed) label() string {
	switch s {
	case speedNormal:
		return "normal"
	case speedFast:
		return "fast"
	case speedVeryFast:
		return "superfast"
	case speedUltra:
		return "ultrafast"
	default:
		return "paused"
	}
}

func (s speed) rate() string {
	if n := s.ticksPerFrame(); n > 0 {
		return fmt.Sprintf("%s %dx", s.label(), n)
	}
	return "paused"
}

// camera is presentation-only state: it never affects authoritative
// simulation, only where/how the world is drawn.
type camera struct {
	offsetX, offsetY float64
	zoom             float64
	placed           bool // clampCamera has centred it on the map once
	// dev is the screen's device pixel density the tile size is
	// multiplied by (PLAY-012: the map renders at physical resolution);
	// 0 means 1, as in a bake's camera (lod.go withBake). syncDev sets it.
	dev float64
}

// tile is the on-screen size of one tile in physical pixels.
func (c *camera) tile() float64 {
	if c.dev > 0 {
		return float64(TileSize) * c.zoom * c.dev
	}
	return float64(TileSize) * c.zoom
}

type pawnVisual struct {
	x, y         int
	fromX, fromY int
	flip         bool
	lastMove     uint64
	moveTick     clock.Tick
	facing       facing
	steps        uint64 // tiles walked (walk-cycle phase)
	lying        bool
	lyingSince   uint64 // present frame when it went down
}

// Renderer drives an Ebitengine window from a running Simulation.
type Renderer struct {
	// onEnqueue, when set, sees every command the player sends (PLAY-018:
	// the learning helper marks lessons done from the player's own orders).
	onEnqueue  func(command.Command)
	view       render.View // this frame's context for the draw layers (render/view.go)
	treeFx     treeFx      // see-through tree canopies (tree_fade.go)
	game       Simulation
	zoneGeoms  map[uint16]*zoneGeom // zones_draw.go outline cache
	zoneBatch  triBatch             // zones_draw.go: every zone outline of a frame, one draw
	zonePlants map[[2]int]bool      // zones_draw.go: cells holding their zone's own plant, this frame (PLAY-020)
	pov        powerOverlay         // power_overlay.go: the power overlay toggle, forced flag and net colours (PLAY-021)
	wheelTaken bool                 // the mouse wheel scrolled a log this frame
	speed      speed
	cam        camera
	art        *artCatalog

	// present is a presentation-only frame counter. Animation reads it;
	// simulation never does. Tile lerp uses sim ticks vs moveTick.
	present   uint64
	selected  entity.EntityID
	hasSelect bool
	last      map[entity.EntityID]pawnVisual

	// animation state (presentation only)
	parts     []particle
	seenTrace map[[3]uint64]bool
	itemBorn  map[entity.EntityID]uint64
	itemLive  map[entity.EntityID]bool // syncItems' reused live set
	// viewCached: the simulation caches its read models (viewCacher), so a
	// read model handed out again is the same, unchanged one and memos
	// keyed on its identity hold (split, App.alerts).
	viewCached bool
	split      pawnSplit
	itemsInit  bool
	mo         motionState // per-creature motion and the animT clock (motion.go)
	anims      *animState  // body-animation bookkeeping (anim.go)
	smooth     camSmooth   // eased pan and zoom (camera_smooth.go)
	env        envState    // wind, contact springs and weather (environment.go)
	lit        litRenderer // sun, moon and lamp lighting (light.go)
	// cast, while set, turns world sprite draws into shadow silhouettes
	// (shadow_cast.go).
	cast *shadowCast
	// bursts remembers which explosive deaths already burst (animal_burst.go).
	bursts burstTracker
	// voice is the animals' sound memory (animal_sounds.go); soundHas
	// reports a sound id exists (the App wires Sound.Has; nil: all do).
	voice    animalVoice
	soundHas func(id string) bool
	// figureBlood, while non-zero, is the blood stain drawLit puts on the
	// figure layer being drawn (lit.kage Custom3, blood_draw.go).
	figureBlood float32
	// roofOverlay is the Roof overlay toggle (Shift+R); roofOverlayForced holds
	// it on while a roof tool is selected (roof_draw.go).
	roofOverlay, roofOverlayForced bool
	// stepTicks are single ticks the developer asked for while paused.
	stepTicks int
	// homeOverlay is the Home area overlay toggle; homeOverlayForced holds
	// it on while a Home tool is in hand (SIM-021, home_draw.go, hud.go).
	homeOverlay, homeOverlayForced bool
	// planHidden hides the Plan overlay (visible by default); planForced
	// holds it on while a Plan tool is in hand (PLAY-012, plan_draw.go).
	planHidden, planForced bool
	planDraw               planDrawState // the Plan overlay's cached geometry
	areaOv                 areaOverlay   // the allowed-area overlay (area_overlay.go)

	gridClr   color.RGBA
	pawnClr   color.RGBA
	selectClr color.RGBA

	// threats: pause when one arrives (option), remembering the newest
	// danger message already handled.
	pauseOnThreat bool
	threatPaused  bool

	// Options (settings.go syncRenderer): particle and weather detail
	// multiplier, edge-of-screen panning, and a capped lightning flash.
	fx              float64
	edgeScroll      bool
	reducedFlashing bool
	reducedMotion   bool // freezes the water ripple and foam (water_draw.go)
	water           waterRenderer

	// sound (sound_signals.go): cues for the App's director, last frame's
	// world to hear changes, the pending thunder and the world this frame.
	cues []soundCue
	// steamHeard is each vent's last burst already cued (features_draw.go).
	steamHeard map[[2]int]uint64
	caskets    casketMemo  // vault_draw.go: sealed state seen last frame
	bridges    bridgeCache // bridge_draw.go: the foundation layer's cell set
	memo       soundMemo
	thunderAt  uint64
	fireFx     fireFx // fire_world_draw.go: clusters and shown strikes
	threatAt   uint64 // present frame of the newest threat
	heard      game.WorldView
	dangerInit bool
	lastDanger uint64
	floaters   []floater

	// links is the cached wall/rock/fence connectivity (linkgrid.go);
	// stoneArt memoises stone tints and resolved linked keys, and draws
	// is the reused per-frame object queue (stone_art.go, draw_art.go).
	links    linkGrid
	power    powerDraw // power_draw.go: turbine blade phases, glow keys, vent covers
	stoneArt stoneArtCache
	draws    []artDraw
	// ENG-D fields
	// lod is the zoom tier, the chunk and whole-map bakes and the frame's
	// chunk lists (lod.go, chunk_cache.go); mini the minimap (minimap.go).
	lod   lodState
	mini  minimapState
	fogc  fogCache  // fog_draw.go
	snowc snowCache // snow_cache.go: snow veil and wet masks per chunk
}

// viewCacher is a simulation that can cache its read models between
// calls (game.Game.SetViewCache, NG-1 §6.3): the renderer only changes it
// through Step, Enqueue and ApplyPending and never writes the views it
// gets back, so it turns the cache on for every simulation it draws.
type viewCacher interface{ SetViewCache(on bool) }

// New returns a Renderer for sim. Art is optional (the app sets r.art)
// after New. Without art, pawns fall back to colored rectangles.
func New(sim Simulation) *Renderer {
	vc, cached := sim.(viewCacher)
	if cached {
		vc.SetViewCache(true)
	}
	return &Renderer{
		game:          sim,
		viewCached:    cached,
		speed:         speedNormal,
		cam:           camera{zoom: 1},
		pauseOnThreat: true,
		fx:            1,
		last:          map[entity.EntityID]pawnVisual{},
		seenTrace:     map[[3]uint64]bool{},
		gridClr:       color.RGBA{R: 40, G: 40, B: 40, A: 180},
		pawnClr:       color.RGBA{R: 220, G: 60, B: 60, A: 255},
		selectClr:     color.RGBA{R: 250, G: 220, B: 60, A: 255},
	}
}

// ticksWanted is this Update's nominal tick count: the speed tier's ticks
// per frame, or one while paused when a developer single step is pending.
// It does not consume the pending step; advanceTicks does once it steps.
func (r *Renderer) ticksWanted() int {
	if n := r.speed.ticksPerFrame(); n > 0 {
		return n
	}
	if r.stepTicks > 0 { // developer single-tick step while paused
		return 1
	}
	return 0
}

// advance steps the speed tier's full tick count (captures, tests).
func (r *Renderer) advance() { r.advanceTicks(r.ticksWanted(), nil) }

// advanceTicks steps one tick at a time up to want; after each tick but
// the last it asks more(done) whether to continue (nil: always), so the
// App's Update budget (update_budget.go) can stop early. The ticks run in
// the same order whichever Update runs them. mo.stepped is the count done.
func (r *Renderer) advanceTicks(want int, more func(done int) bool) (done int) {
	r.present++
	if want > 0 && r.speed.ticksPerFrame() == 0 && r.stepTicks > 0 {
		r.stepTicks--
	}
	for done < want {
		r.game.Step(1)
		done++
		if more != nil && done < want && !more(done) {
			break
		}
	}
	r.mo.stepped = done
	r.syncPawns()
	w := r.game.World()
	r.animate(w, r.game.Snapshot(), done > 0)
	r.stepEnvironment(w, r.game.Snapshot())
	r.emitWorldBursts(w) // animal_burst.go: explosive animal deaths
	r.listenAnimals(w)   // animal_sounds.go: calls, flee cries, deaths
	r.watchThreats(w)
	r.heard = w
	if done > 0 {
		r.listenWorld(w, r.game.Snapshot())
	}
	r.listenThunder()
	return done
}

func (r *Renderer) syncPawns() {
	tick := r.game.Tick()
	w := r.game.World()
	r.mo.beginSync(tick)
	running := r.mo.stepped > 0
	for _, p := range r.game.Snapshot() {
		prev, seen := r.last[p.ID]
		v := nextVisual(prev, seen, p.X, p.Y, tick, r.present)
		k := r.mo.observe(p.ID, pawnMoView(p), v.facing)
		want, moving := k.wantFacing(r.mo.ref)
		if !moving { // standing: attacks and work turn it
			want = faceAttack(pawnVisual{facing: want}, w.Traces, p.X, p.Y).facing
			if p.Acting != "" && (p.ActX != p.X || p.ActY != p.Y) {
				want = facingFor(p.ActX-p.X, p.ActY-p.Y, want)
			}
		}
		v.facing = k.turn(want, p.ID, running)
		if p.Lying && !v.lying {
			v.lyingSince = r.present
		}
		v.lying = p.Lying
		r.last[p.ID] = v
	}
	r.syncAnimals(w)
	r.syncItems(w)
	r.mo.endSync()
}

// Speed and camera intents (UI-016): the App resolves the keys and the
// mouse (input_frame.go) and the renderer only applies what they mean.

// togglePause pauses, or resumes at normal speed.
func (r *Renderer) togglePause() {
	if r.speed == speedPaused {
		r.speed = speedNormal
		r.globalCue("ui.resume")
		return
	}
	r.speed = speedPaused
	r.globalCue("ui.pause")
}

// setSpeed picks a speed tier.
func (r *Renderer) setSpeed(s speed) {
	r.speed = s
	r.globalCue("ui.speed")
}

// stepSpeed moves one tier up (d > 0) or down, paused .. superfast
// (ultrafast is dev-only and steps down to superfast).
func (r *Renderer) stepSpeed(d int) {
	s := r.speed
	if s > speedVeryFast {
		s = speedVeryFast
		if d > 0 {
			d = 0
		}
	}
	r.setSpeed(speed(max(int(speedPaused), min(int(speedVeryFast), int(s)+d))))
}

// camInput is one frame of camera intent.
type camInput struct {
	dx, dy       float64 // held pan direction (keys plus edge scroll)
	fast         bool    // Shift: pan faster
	zoom         int     // keyboard zoom steps (+in) about the viewport centre
	wheel        float64 // mouse wheel steps (+in) about (wx, wy)
	wx, wy       float64
	dragX, dragY float64 // middle-mouse drag this frame, screen px
}

// fastPanBoost is the pan speed multiplier while Shift is held.
const fastPanBoost = 2.5

// applyCamera pans and zooms by one frame of intent, then clamps.
func (r *Renderer) applyCamera(v viewport, in camInput) {
	// The held direction is this Update's: clear the last one first, then
	// push what is held now. It stays set through every Draw until the next
	// Update (stepCamera steps with it), so a released key stops the push
	// at the next Update and the pan coasts to a halt.
	r.smooth.dirX, r.smooth.dirY = 0, 0
	r.smooth.boost = 1
	if in.fast {
		r.smooth.boost = fastPanBoost
	}
	r.smooth.pushPan(in.dx, in.dy)
	cx, cy := (v.x0+v.x1)/2, (v.y0+v.y1)/2
	for ; in.zoom > 0; in.zoom-- {
		r.smooth.zoomAt(1+zoomStep, cx, cy)
	}
	for ; in.zoom < 0; in.zoom++ {
		r.smooth.zoomAt(1/(1+zoomStep), cx, cy)
	}
	if in.wheel != 0 {
		r.smooth.zoomAt(math.Pow(1+zoomStep, in.wheel), in.wx, in.wy)
	}
	if in.dragX != 0 || in.dragY != 0 { // the map follows the hand
		r.cam.offsetX -= in.dragX
		r.cam.offsetY -= in.dragY
		r.smooth.vx, r.smooth.vy = 0, 0
	}
	r.clampCamera(v)      // first-call centring and min-zoom guard
	if r.smooth.instant { // Reduced Motion: no easing, the whole step lands here; live play steps per Draw (stepCamera)
		r.smooth.step(1/float64(ebiten.TPS()), &r.cam, r.camLimits(v))
	}
}

// camMaxDt caps one camera step so a stall never jumps the view.
const camMaxDt = 0.1

// stepCamera advances the smoothed camera by dt wall seconds, once per Draw,
// so panning and zoom run at the display rate rather than at 60 TPS. Input
// mapping in Update sees the previous Draw's camera: one frame of lag.
func (r *Renderer) stepCamera(dt float64, v viewport) {
	if r.smooth.instant || r.smooth.idle() {
		return
	}
	dt = math.Max(0, math.Min(dt, camMaxDt))
	r.smooth.step(dt, &r.cam, r.camLimits(v))
}

// edgeBand is how close (points) the cursor must be to a viewport edge to
// pan.
const edgeBand = 6

// edgePan is the pan direction for a physical cursor at (mx, my) in the
// physical viewport v: towards any edge within edgeBand points, nothing
// when the cursor is outside it.
func edgePan(mx, my int, v viewport) (dx, dy float64) {
	x, y := float64(mx), float64(my)
	if x < v.x0 || x >= v.x1 || y < v.y0 || y >= v.y1 {
		return 0, 0
	}
	band := edgeBand * screenDev()
	switch {
	case x < v.x0+band:
		dx = -1
	case x >= v.x1-band:
		dx = 1
	}
	switch {
	case y < v.y0+band:
		dy = -1
	case y >= v.y1-band:
		dy = 1
	}
	return dx, dy
}

func clampZoom(z float64) float64 {
	if z < minZoom {
		return minZoom
	}
	if z > maxZoom {
		return maxZoom
	}
	return z
}

// screenToWorld maps a physical screen pixel (App.mapCursor) to world
// tile coordinates.
func (r *Renderer) screenToWorld(sx, sy int) (float64, float64) {
	tile := r.cam.tile()
	return (float64(sx) + r.cam.offsetX) / tile, (float64(sy) + r.cam.offsetY) / tile
}

// interpWorld is where a pawn is drawn: its motion at animT (ART §27.1),
// or its tile before the first sync.
func (r *Renderer) interpWorld(p game.PawnView) (float64, float64) {
	m, ok := r.motionOf(p.ID)
	if !ok {
		return float64(p.X), float64(p.Y)
	}
	return m.X, m.Y
}

func (r *Renderer) worldToScreen(wx, wy int) (float32, float32) {
	return r.worldToScreenF(float64(wx), float64(wy))
}

func (r *Renderer) worldToScreenF(wx, wy float64) (float32, float32) {
	tile := r.cam.tile()
	sx := wx*tile - r.cam.offsetX
	sy := wy*tile - r.cam.offsetY
	return float32(sx), float32(sy)
}

// tilePx is one tile in physical pixels: TileSize x zoom x dev (the UI
// scale never zooms the map).
func (r *Renderer) tilePx() float64 {
	return r.cam.tile()
}

func visible(x, y, w, h float32, sw, sh int) bool {
	return x+w >= 0 && y+h >= 0 && x < float32(sw) && y < float32(sh)
}

// Draw renders terrain, pawns, and selection. HUD chrome is App-owned.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 28, G: 42, B: 32, A: 255})
	r.syncDev() // camera.go: the screen is physical

	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.tilePx()
	// this frame's context for the draw layers (render.View); drawArt adds
	// the world and pawns it reads
	v := &r.view
	v.SW, v.SH, v.Tile = sw, sh, tile
	v.Zoom, v.Dev = r.cam.zoom, r.cam.dev
	v.MapW, v.MapH = r.game.Width(), r.game.Height()
	v.Tick = r.game.Tick()
	v.Present, v.AnimT, v.Alpha = r.present, r.animT(), r.mo.alpha
	v.Paused = r.speed == speedPaused
	v.Fx, v.ReducedMotion, v.ReducedFlashing, v.Cached = r.fx, r.reducedMotion, r.reducedFlashing, r.viewCached
	v.Selected, v.HasSelect = r.selected, r.hasSelect

	if r.art != nil {
		r.drawArt(screen, sw, sh, tile) // lit by light.go: no veil
		r.drawRoofOverlay(screen, r.view.W, sw, sh, tile)
		r.drawPowerOverlay(screen, &r.view.W, sw, sh, tile) // power_overlay.go (PLAY-021)
		r.drawTraces(screen, r.game.World(), tile)
		return
	}

	r.drawTerrain(screen, sw, sh, tile)
	r.drawWorld(screen, sw, sh, tile)
	r.drawGrid(screen)

	for _, p := range r.game.Snapshot() {
		wx, wy := r.interpWorld(p)
		sx, sy := r.worldToScreenF(wx, wy)
		if !visible(sx, sy, float32(tile), float32(tile)*2, sw, sh) {
			continue
		}
		r.drawPawn(screen, p, sx, sy, tile)
		if r.hasSelect && p.ID == r.selected {
			r.drawSelection(screen, sx, sy, tile)
		}
	}

	r.drawDaylight(screen, sw, sh)
}

func (r *Renderer) drawDaylight(screen *ebiten.Image, sw, sh int) {
	c := skyOverlay(r.game.Tick())
	if c.A == 0 {
		return
	}
	vector.FillRect(screen, 0, 0, float32(sw), float32(sh), c, false)
}

func (r *Renderer) drawTerrain(screen *ebiten.Image, sw, sh int, tile float64) {
	for y := 0; y < r.game.Height(); y++ {
		for x := 0; x < r.game.Width(); x++ {
			sx, sy := r.worldToScreen(x, y)
			if !visible(sx, sy, float32(tile), float32(tile), sw, sh) {
				continue
			}
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), fallbackGround, false)
		}
	}
}

func (r *Renderer) drawGrid(screen *ebiten.Image) {
	for x := 0; x <= r.game.Width(); x++ {
		x0, y0 := r.worldToScreen(x, 0)
		_, y1 := r.worldToScreen(x, r.game.Height())
		vector.StrokeLine(screen, x0, y0, x0, y1, 1, r.gridClr, false)
	}
	for y := 0; y <= r.game.Height(); y++ {
		x0, y0 := r.worldToScreen(0, y)
		x1, _ := r.worldToScreen(r.game.Width(), y)
		vector.StrokeLine(screen, x0, y0, x1, y0, 1, r.gridClr, false)
	}
}

func nextVisual(prev pawnVisual, seen bool, x, y int, tick clock.Tick, present uint64) pawnVisual {
	v := pawnVisual{x: x, y: y, flip: prev.flip, lastMove: prev.lastMove, moveTick: prev.moveTick, facing: prev.facing}
	if !seen {
		v.fromX, v.fromY = x, y
		v.moveTick = tick
		return v
	}
	v.steps, v.lying, v.lyingSince = prev.steps, prev.lying, prev.lyingSince
	if prev.x != x || prev.y != y {
		v.fromX, v.fromY = prev.x, prev.y
		v.moveTick = tick
		v.lastMove = present
		v.steps++
		if x != prev.x {
			v.flip = x < prev.x
		}
		v.facing = facingFor(x-prev.x, y-prev.y, prev.facing)
		return v
	}
	v.fromX, v.fromY = prev.fromX, prev.fromY
	if tick-prev.moveTick >= clock.Tick(game.TicksPerCell) {
		v.fromX, v.fromY = x, y
	}
	return v
}

func moveProgress(sinceTick, period int) float64 {
	if period <= 0 {
		return 1
	}
	if sinceTick <= 0 {
		return 0
	}
	if sinceTick >= period {
		return 1
	}
	return float64(sinceTick) / float64(period)
}

func lerp(a, b int, t float64) float64 {
	if t <= 0 {
		return float64(a)
	}
	if t >= 1 {
		return float64(b)
	}
	return float64(a) + (float64(b)-float64(a))*t
}

// fallbackGround is the terrain colour when no art is loaded.
var fallbackGround = color.RGBA{R: 74, G: 66, B: 50, A: 255}

// drawPawn is the no-art fallback: a plain coloured square.
func (r *Renderer) drawPawn(screen *ebiten.Image, p game.PawnView, sx, sy float32, tile float64) {
	vector.FillRect(screen, sx+2, sy+2, float32(tile)-4, float32(tile)-4, r.pawnClr, false)
}

func (r *Renderer) drawSelection(screen *ebiten.Image, sx, sy float32, tile float64) {
	vector.StrokeRect(screen, sx+1, sy+1, float32(tile)-2, float32(tile)-2, 2, r.selectClr, false)
}

// Select marks id as the presentation selection.
func (r *Renderer) Select(id entity.EntityID) {
	r.selected, r.hasSelect = id, true
}

// ClearSelect drops the presentation selection.
func (r *Renderer) ClearSelect() {
	r.hasSelect = false
}

// HasSelect reports whether a pawn is selected.
func (r *Renderer) HasSelect() bool { return r.hasSelect }

// Selected is the current presentation selection. Valid only if HasSelect.
func (r *Renderer) Selected() entity.EntityID { return r.selected }

// snapshot is the player's colonists (raiders are left out: they are only
// drawn, never listed, selected or ordered).
func (r *Renderer) snapshot() []game.PawnView {
	r.splitPawns()
	return r.split.colonists
}

// hostiles is the raiders on the map.
func (r *Renderer) hostiles() []game.PawnView {
	r.splitPawns()
	return r.split.raiders
}

// pawnSplit is the snapshot split into colonists and raiders, kept while
// the simulation hands out the same snapshot (the view cache: the HUD
// asks many times a frame). Both lists are full slices (cap == len), so
// a caller's append copies instead of writing into them.
type pawnSplit struct {
	src                []game.PawnView
	colonists, raiders []game.PawnView
}

func (r *Renderer) splitPawns() {
	all := r.game.Snapshot()
	s := &r.split
	if r.viewCached && s.src != nil && len(all) == len(s.src) && litFirst(all) == litFirst(s.src) {
		return
	}
	s.src = all
	if all == nil {
		s.src = []game.PawnView{}
	}
	s.colonists, s.raiders = all[:0:0], nil
	for _, p := range all {
		if !p.Hostile {
			s.colonists = append(s.colonists, p)
		} else {
			s.raiders = append(s.raiders, p)
		}
	}
	s.colonists = s.colonists[:len(s.colonists):len(s.colonists)]
	s.raiders = s.raiders[:len(s.raiders):len(s.raiders)]
}

func (r *Renderer) world() game.WorldView { return r.game.World() }

func (r *Renderer) date() clock.Date { return r.game.Date() }

func (r *Renderer) activity() schedule.Activity { return r.game.Activity() }

// pendingApplier applies queued commands without advancing time
// (consumer-owned: test fakes need not implement it).
type pendingApplier interface{ ApplyPending() }

// enqueue sends a player command. While paused it is applied at once, so
// zones, blueprints, designations, priorities and orders show immediately
// instead of on unpause; running, it waits for the next Step.
func (r *Renderer) enqueue(c command.Command) {
	r.game.Enqueue(c)
	if r.onEnqueue != nil {
		r.onEnqueue(c) // learn_ui.go: lessons done by the player's own orders
	}
	if r.speed == speedPaused {
		if p, ok := r.game.(pendingApplier); ok {
			p.ApplyPending()
		}
	}
}

// TileAt converts a screen pixel to a map tile.
func (r *Renderer) TileAt(sx, sy int) (int, int) {
	wx, wy := r.screenToWorld(sx, sy)
	return int(wx), int(wy)
}

func (r *Renderer) drawWorld(screen *ebiten.Image, sw, sh int, tile float64) {
	w := r.game.World()
	floorClr := color.RGBA{R: 90, G: 90, B: 95, A: 255}
	for _, b := range w.Buildings { // floors lie under the zones
		if !b.Floor {
			continue
		}
		if sx, sy := r.worldToScreen(b.X, b.Y); visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			vector.FillRect(screen, sx+4, sy+4, float32(tile)-8, float32(tile)-8, floorClr, false)
		}
	}
	r.drawZones(screen, w, sw, sh, tile)
	plantClr := color.RGBA{R: 70, G: 150, B: 70, A: 220}
	desigClr := color.RGBA{R: 240, G: 220, B: 40, A: 200}
	for _, p := range w.Plants {
		sx, sy := r.worldToScreen(p.X, p.Y)
		if !visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			continue
		}
		pad := float32(tile) * 0.25
		vector.FillRect(screen, sx+pad, sy+pad, float32(tile)-2*pad, float32(tile)-2*pad, plantClr, false)
		if p.Designated {
			vector.StrokeRect(screen, sx+2, sy+2, float32(tile)-4, float32(tile)-4, 2, desigClr, false)
		}
	}
	foodClr := color.RGBA{R: 200, G: 90, B: 40, A: 255}
	woodClr := color.RGBA{R: 140, G: 90, B: 40, A: 255}
	for _, it := range w.Items {
		if it.Carried {
			continue
		}
		sx, sy := r.worldToScreen(it.X, it.Y)
		if !visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			continue
		}
		c := foodClr
		if it.Kind == "wood" {
			c = woodClr
		}
		pad := float32(tile) * 0.3
		vector.FillRect(screen, sx+pad, sy+pad, float32(tile)-2*pad, float32(tile)-2*pad, c, false)
	}
	ghostClr := color.RGBA{R: 90, G: 180, B: 220, A: 120}
	for _, f := range w.Frames {
		sx, sy := r.worldToScreen(f.X, f.Y)
		if visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			vector.StrokeRect(screen, sx+3, sy+3, float32(tile)-6, float32(tile)-6, 2, ghostClr, false)
		}
	}
	wallClr := color.RGBA{R: 90, G: 90, B: 95, A: 255}
	bedClr := color.RGBA{R: 70, G: 90, B: 160, A: 255}
	for _, b := range w.Buildings {
		if b.Floor {
			continue // drawn before the zones
		}
		sx, sy := r.worldToScreen(b.X, b.Y)
		if !visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			continue
		}
		c := wallClr
		if b.Kind == "bed" {
			c = bedClr
		}
		vector.FillRect(screen, sx+4, sy+4, float32(tile)-8, float32(tile)-8, c, false)
	}
	r.drawZoneLabels(screen, w, sw, sh, tile) // zones_draw.go: over the objects
}

func (r *Renderer) drawGhostRect(screen *ebiten.Image, x0, y0, x1, y1 int) {
	ghostClr := color.RGBA{R: 90, G: 180, B: 220, A: 180}
	minX, maxX := x0, x1
	if maxX < minX {
		minX, maxX = maxX, minX
	}
	minY, maxY := y0, y1
	if maxY < minY {
		minY, maxY = maxY, minY
	}
	tile := r.tilePx()
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			sx, sy := r.worldToScreen(x, y)
			if visible(sx, sy, float32(tile), float32(tile), sw, sh) {
				vector.StrokeRect(screen, sx+2, sy+2, float32(tile)-4, float32(tile)-4, 2, ghostClr, false)
			}
		}
	}
}
