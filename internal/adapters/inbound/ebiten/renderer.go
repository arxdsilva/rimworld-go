// Package ebiten adapts the headless application game to an interactive
// Ebitengine client: screen routing plus a colony map viewport. It never
// mutates domain state directly; it only calls Step/Snapshot/World and
// Enqueue on a consumer-owned Simulation interface.
package ebiten

import (
	"fmt"
	"image/color"
	"math"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	camerapkgpkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	envpkg "rimworld-go/internal/adapters/inbound/ebiten/render/env"
	lightpkg "rimworld-go/internal/adapters/inbound/ebiten/render/light"
	minimappkg "rimworld-go/internal/adapters/inbound/ebiten/render/minimap"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Simulation is the composition root's union of the readers and the
// command sink the client needs of the colony (sim_ports.go): cmd/game
// hands one in and the root stores it; every feature and layer depends on
// its own smaller interface (SOLID_GO.md §6), never on this union.
type Simulation interface {
	SimStepper
	SimCommander
	SimFrame
	SimWorld
	SimPlacement
	SimColony
	SimTabs
	SimChunks
}

var _ Simulation = (*game.Game)(nil)

const (
	// absolute floor; the map-fit limit is minZoomFor

	zoomStep = 0.1
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

type pawnVisual struct {
	x, y         int
	fromX, fromY int
	flip         bool
	lastMove     uint64
	moveTick     clock.Tick
	facing       artpkg.Facing
	steps        uint64 // tiles walked (walk-cycle phase)
	lying        bool
	lyingSince   uint64 // present frame when it went down
}

// Renderer drives an Ebitengine window from a running Simulation.
type Renderer struct {
	// onEnqueue, when set, sees every command the player sends (PLAY-018:
	// the learning helper marks lessons done from the player's own orders).
	onEnqueue  func(command.Command)
	view       render.View   // this frame's context for the draw layers (render/view.go)
	treeFx     envpkg.TreeFx // see-through tree canopies (tree_fade.go)
	game       Simulation
	wheelTaken bool // the mouse wheel scrolled a log this frame
	speed      speed
	cam        camerapkgpkg.Camera
	art        *artpkg.Catalog

	// present is a presentation-only frame counter. Animation reads it;
	// simulation never does. Tile lerp uses sim ticks vs moveTick.
	present   uint64
	selected  entity.EntityID
	hasSelect bool
	last      map[entity.EntityID]pawnVisual

	// animation state (presentation only)
	parts    []creaturepkg.Particle
	itemBorn map[entity.EntityID]uint64
	// viewCached: the simulation caches its read models (viewCacher), so a
	// read model handed out again is the same, unchanged one and memos
	// keyed on its identity hold (split, App.alerts).
	viewCached bool
	split      pawnSplit
	mo         motionpkg.State[creaturepkg.Body] // per-creature motion and the animT clock (render/motion); each track carries its animal's body player
	smooth     camerapkgpkg.Smooth               // eased pan and zoom (camera/smooth.go)
	env        envState                          // wind, contact springs and weather (environment.go)
	lit        lightpkg.Lit                      // sun, moon and lamp lighting (render/light)
	// cast, while set, turns world sprite draws into shadow silhouettes
	// (shadow_cast.go).
	cast *lightpkg.Cast
	// bursts remembers which explosive deaths already burst (animal_burst.go).
	// voice is the animals' sound memory (animal_sounds.go); soundHas
	// reports a sound id exists (the App wires Sound.Has; nil: all do).
	// sound is the world's sound-cue bus (render/cue); cueBus() builds it on
	// first use.
	sound *cue.Bus
	// figureBlood, while non-zero, is the blood stain drawLit puts on the
	// figure layer being drawn (lit.kage Custom3, blood_draw.go).
	figureBlood float32
	// stepTicks are single ticks the developer asked for while paused.
	stepTicks int
	// wp is the world draw order and every draw layer, the zoom state, the
	// link grid and the stone cache (render/world, layers.go: pipe).
	wp *world.Pipeline

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
	water           envpkg.Water

	// sound (sound_signals.go): cues for the App's director, last frame's
	// world to hear changes, the pending thunder and the world this frame.
	threatAt   uint64 // present frame of the newest threat
	heard      game.WorldView
	dangerInit bool
	lastDanger uint64
	floaters   []creaturepkg.Floater

	mini  minimappkg.Map   // the minimap (render/minimap)
	snowc envpkg.SnowCache // snow_cache.go: snow veil and wet masks per chunk
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
		cam:           camerapkgpkg.Camera{Zoom: 1},
		pauseOnThreat: true,
		fx:            1,
		last:          map[entity.EntityID]pawnVisual{},
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
	r.mo.Stepped = done
	r.syncPawns()
	w := r.game.World()
	r.animate(w, r.game.Snapshot(), done > 0)
	r.stepEnvironment(w, r.game.Snapshot())
	p, cb, v := r.pipe(), r.cueBus(), r.liveView()
	p.Creatures.EmitWorldBursts(v, w)    // render/creature: explosive animal deaths
	cb.ListenBursts(w.Bursts)            // render/cue: grenade and launcher blasts (DEF-2)
	cb.ListenAnimals(v, w, r.mo.Stepped) // render/cue: calls, flee cries, deaths
	r.watchThreats(w)
	r.heard = w
	if done > 0 {
		cb.ListenWorld(r.liveView(), w, r.game.Snapshot())
	}
	cb.ListenThunder(r.liveView())
	return done
}

func (r *Renderer) syncPawns() {
	tick := r.game.Tick()
	w := r.game.World()
	r.mo.BeginSync(tick)
	running := r.mo.Stepped > 0
	for _, p := range r.game.Snapshot() {
		prev, seen := r.last[p.ID]
		v := nextVisual(prev, seen, p.X, p.Y, tick, r.present)
		k := r.mo.Observe(p.ID, motionpkg.PawnMoView(p), v.facing)
		want, moving := k.WantFacing(r.mo.Ref)
		if !moving { // standing: attacks and work turn it
			want = faceAttack(pawnVisual{facing: want}, w.Traces, p.X, p.Y).facing
			if p.Acting != "" && (p.ActX != p.X || p.ActY != p.Y) {
				want = artpkg.FacingFor(p.ActX-p.X, p.ActY-p.Y, want)
			}
		}
		v.facing = k.Turn(want, p.ID, running)
		if p.Lying && !v.lying {
			v.lyingSince = r.present
		}
		v.lying = p.Lying
		r.last[p.ID] = v
	}
	r.syncAnimals(w)
	r.pipe().Creatures.SyncItems(r.liveView(), w)
	r.mo.EndSync()
}

// Speed and camera intents (UI-016): the App resolves the keys and the
// mouse (input_frame.go) and the renderer only applies what they mean.

// togglePause pauses, or resumes at normal speed.
func (r *Renderer) togglePause() {
	if r.speed == speedPaused {
		r.speed = speedNormal
		r.cueBus().Global("ui.resume")
		return
	}
	r.speed = speedPaused
	r.cueBus().Global("ui.pause")
}

// setSpeed picks a speed tier.
func (r *Renderer) setSpeed(s speed) {
	r.speed = s
	r.cueBus().Global("ui.speed")
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
func (r *Renderer) applyCamera(v camerapkgpkg.Viewport, in camInput) {
	// The held direction is this Update's: clear the last one first, then
	// push what is held now. It stays set through every Draw until the next
	// Update (stepCamera steps with it), so a released key stops the push
	// at the next Update and the pan coasts to a halt.
	r.smooth.DirX, r.smooth.DirY = 0, 0
	r.smooth.Boost = 1
	if in.fast {
		r.smooth.Boost = fastPanBoost
	}
	r.smooth.PushPan(in.dx, in.dy)
	cx, cy := (v.X0+v.X1)/2, (v.Y0+v.Y1)/2
	for ; in.zoom > 0; in.zoom-- {
		r.smooth.ZoomAt(1+zoomStep, cx, cy)
	}
	for ; in.zoom < 0; in.zoom++ {
		r.smooth.ZoomAt(1/(1+zoomStep), cx, cy)
	}
	if in.wheel != 0 {
		r.smooth.ZoomAt(math.Pow(1+zoomStep, in.wheel), in.wx, in.wy)
	}
	if in.dragX != 0 || in.dragY != 0 { // the map follows the hand
		r.cam.OffsetX -= in.dragX
		r.cam.OffsetY -= in.dragY
		r.smooth.Vx, r.smooth.Vy = 0, 0
	}
	r.ClampCamera(v)      // first-call centring and min-zoom guard
	if r.smooth.Instant { // Reduced Motion: no easing, the whole step lands here; live play steps per Draw (stepCamera)
		r.smooth.Step(1/float64(ebiten.TPS()), &r.cam, r.CamLimits(v))
	}
}

// SyncDev adopts the screen's density (camera.Camera.SyncDev).
func (r *Renderer) SyncDev() { r.cam.SyncDev(camerapkgpkg.ScreenDev(), &r.smooth) }

// MinZoomFor is the furthest zoom-out for this map and viewport.
func (r *Renderer) MinZoomFor(v camerapkgpkg.Viewport) float64 {
	return r.cam.MinZoomFor(v, r.game.Width(), r.game.Height())
}

// ClampCamera keeps the zoom in range and the map in view.
func (r *Renderer) ClampCamera(v camerapkgpkg.Viewport) {
	r.SyncDev()
	r.cam.Clamp(v, r.game.Width(), r.game.Height(), r.game.World().Landing)
}

// CamLimits are the smoothing bounds for this map and viewport.
func (r *Renderer) CamLimits(v camerapkgpkg.Viewport) camerapkgpkg.Limits {
	return r.cam.LimitsFor(v, r.game.Width(), r.game.Height())
}

// camMaxDt caps one camera step so a stall never jumps the view.
const camMaxDt = 0.1

// stepCamera advances the smoothed camera by dt wall seconds, once per Draw,
// so panning and zoom run at the display rate rather than at 60 TPS. Input
// mapping in Update sees the previous Draw's camera: one frame of lag.
func (r *Renderer) stepCamera(dt float64, v camerapkgpkg.Viewport) {
	if r.smooth.Instant || r.smooth.Idle() {
		return
	}
	dt = math.Max(0, math.Min(dt, camMaxDt))
	r.smooth.Step(dt, &r.cam, r.CamLimits(v))
}

// edgeBand is how close (points) the cursor must be to a viewport edge to
// pan.
const edgeBand = 6

// edgePan is the pan direction for a physical cursor at (mx, my) in the
// physical viewport v: towards any edge within edgeBand points, nothing
// when the cursor is outside it.
func edgePan(mx, my int, v camerapkgpkg.Viewport) (dx, dy float64) {
	x, y := float64(mx), float64(my)
	if x < v.X0 || x >= v.X1 || y < v.Y0 || y >= v.Y1 {
		return 0, 0
	}
	band := edgeBand * camerapkgpkg.ScreenDev()
	switch {
	case x < v.X0+band:
		dx = -1
	case x >= v.X1-band:
		dx = 1
	}
	switch {
	case y < v.Y0+band:
		dy = -1
	case y >= v.Y1-band:
		dy = 1
	}
	return dx, dy
}

func clampZoom(z float64) float64 {
	if z < camerapkgpkg.MinZoom {
		return camerapkgpkg.MinZoom
	}
	if z > camerapkgpkg.MaxZoom {
		return camerapkgpkg.MaxZoom
	}
	return z
}

// screenToWorld maps a physical screen pixel (App.mapCursor) to world
// tile coordinates.
func (r *Renderer) screenToWorld(sx, sy int) (float64, float64) {
	tile := r.cam.Tile()
	return (float64(sx) + r.cam.OffsetX) / tile, (float64(sy) + r.cam.OffsetY) / tile
}

// interpWorld is where a pawn is drawn: its motion at animT (ART §27.1),
// or its tile before the first sync.
func (r *Renderer) interpWorld(p game.PawnView) (float64, float64) {
	t := 0.0
	if r.game != nil {
		t = r.mo.AnimT(r.game.Tick())
	}
	m, ok := r.mo.MotionOf(p.ID, t)
	if !ok {
		return float64(p.X), float64(p.Y)
	}
	return m.X, m.Y
}

func (r *Renderer) worldToScreen(wx, wy int) (float32, float32) {
	return r.worldToScreenF(float64(wx), float64(wy))
}

func (r *Renderer) worldToScreenF(wx, wy float64) (float32, float32) {
	tile := r.cam.Tile()
	sx := wx*tile - r.cam.OffsetX
	sy := wy*tile - r.cam.OffsetY
	return float32(sx), float32(sy)
}

// Draw renders terrain, pawns, and selection. HUD chrome is App-owned.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 28, G: 42, B: 32, A: 255})
	r.SyncDev() // the screen is physical

	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.cam.Tile()
	// this frame's context for the draw layers (render.View); the pipeline adds
	// the world and pawns it reads
	v := r.liveView()
	v.SW, v.SH, v.Tile = sw, sh, tile
	v.Zoom, v.Dev = r.cam.Zoom, r.cam.Dev
	v.Alpha = r.mo.Alpha
	v.Paused = r.speed == speedPaused
	v.Cached = r.viewCached

	p := r.pipe()
	if r.art != nil {
		p.Draw(screen) // render/world, lit by render/light: no veil
		w := r.game.World()
		ch := p.OverlayChunks()
		p.Env.SetView(v, sw, sh, tile)
		p.Overlays.DrawRoofOverlay(v, screen, v.W, r.game.Areas, sw, sh, tile)
		p.Overlays.DrawPowerOverlay(v, &ch, p.Links, screen, &v.W, sw, sh, tile) // render/overlay power (PLAY-021)
		p.Creatures.DrawTraces(v, screen, w, r.game.Snapshot(), tile)
		return
	}

	// no art: plain coloured cells (tests, a missing asset tree)
	v.W, v.Pawns = r.game.World(), r.game.Snapshot()
	drawFallbackTerrain(screen, v)
	drawFallbackWorld(screen, v, p)
	drawFallbackGrid(screen, v)
	drawFallbackPawns(screen, v, &r.mo, r.selectClr)
	drawDaylight(screen, v)
}

// drawDaylight tints the map with the sky's light (no-art fallback; with
// art render/light lights every sprite).
func drawDaylight(screen *ebiten.Image, v *render.View) {
	if c := lightpkg.SkyOverlay(v.Tick); c.A != 0 {
		vector.FillRect(screen, 0, 0, float32(v.SW), float32(v.SH), c, false)
	}
}

// drawFallbackPawns draws the frame's pawns (v.Pawns) as squares where
// their motion puts them, the selected one boxed in sel.
func drawFallbackPawns(screen *ebiten.Image, v *render.View, mo *motionpkg.State[creaturepkg.Body], sel color.RGBA) {
	t := float32(v.Tile)
	for _, pw := range v.Pawns {
		wx, wy := float64(pw.X), float64(pw.Y)
		if m, ok := mo.MotionOf(pw.ID, v.AnimT); ok {
			wx, wy = m.X, m.Y
		}
		sx, sy := v.WorldToScreenF(wx, wy)
		if !spritepkg.Visible(sx, sy, t, t*2, v.SW, v.SH) {
			continue
		}
		vector.FillRect(screen, sx+2, sy+2, t-4, t-4, fallbackPawn, false)
		if v.HasSelect && pw.ID == v.Selected {
			vector.StrokeRect(screen, sx+1, sy+1, t-2, t-2, 2, sel, false)
		}
	}
}

// The no-art fallback's colours.
var (
	fallbackGround = color.RGBA{R: 74, G: 66, B: 50, A: 255}
	fallbackGrid   = color.RGBA{R: 40, G: 40, B: 40, A: 180}
	fallbackPawn   = color.RGBA{R: 220, G: 60, B: 60, A: 255}
)

// drawFallbackTerrain fills every visible cell with the plain ground.
func drawFallbackTerrain(screen *ebiten.Image, v *render.View) {
	t := float32(v.Tile)
	for y := 0; y < v.MapH; y++ {
		for x := 0; x < v.MapW; x++ {
			if sx, sy := v.WorldToScreen(x, y); spritepkg.Visible(sx, sy, t, t, v.SW, v.SH) {
				vector.FillRect(screen, sx, sy, t, t, fallbackGround, false)
			}
		}
	}
}

// drawFallbackGrid strokes the cell grid.
func drawFallbackGrid(screen *ebiten.Image, v *render.View) {
	for x := 0; x <= v.MapW; x++ {
		x0, y0 := v.WorldToScreen(x, 0)
		_, y1 := v.WorldToScreen(x, v.MapH)
		vector.StrokeLine(screen, x0, y0, x0, y1, 1, fallbackGrid, false)
	}
	for y := 0; y <= v.MapH; y++ {
		x0, y0 := v.WorldToScreen(0, y)
		x1, _ := v.WorldToScreen(v.MapW, y)
		vector.StrokeLine(screen, x0, y0, x1, y0, 1, fallbackGrid, false)
	}
}

// drawFallbackWorld draws the frame's world (v.W) as plain cells: floors,
// the zones (render/overlay), plants, items, frames, buildings and the
// zone labels over them.
func drawFallbackWorld(screen *ebiten.Image, v *render.View, p *worldpkg.Pipeline) {
	w, t, sw, sh := v.W, float32(v.Tile), v.SW, v.SH
	visible := func(x, y int) (float32, float32, bool) {
		sx, sy := v.WorldToScreen(x, y)
		return sx, sy, spritepkg.Visible(sx, sy, t, t, sw, sh)
	}
	floorClr := color.RGBA{R: 90, G: 90, B: 95, A: 255}
	for _, b := range w.Buildings { // floors lie under the zones
		if sx, sy, ok := visible(b.X, b.Y); b.Floor && ok {
			vector.FillRect(screen, sx+4, sy+4, t-8, t-8, floorClr, false)
		}
	}
	ch := p.OverlayChunks()
	p.Overlays.Zones.DrawZones(v, p.Pass(), &ch, screen, w, sw, sh, v.Tile)
	plantClr := color.RGBA{R: 70, G: 150, B: 70, A: 220}
	desigClr := color.RGBA{R: 240, G: 220, B: 40, A: 200}
	for _, pl := range w.Plants {
		sx, sy, ok := visible(pl.X, pl.Y)
		if !ok {
			continue
		}
		pad := t * 0.25
		vector.FillRect(screen, sx+pad, sy+pad, t-2*pad, t-2*pad, plantClr, false)
		if pl.Designated {
			vector.StrokeRect(screen, sx+2, sy+2, t-4, t-4, 2, desigClr, false)
		}
	}
	foodClr := color.RGBA{R: 200, G: 90, B: 40, A: 255}
	woodClr := color.RGBA{R: 140, G: 90, B: 40, A: 255}
	for _, it := range w.Items {
		sx, sy, ok := visible(it.X, it.Y)
		if it.Carried || !ok {
			continue
		}
		c := foodClr
		if it.Kind == "wood" {
			c = woodClr
		}
		pad := t * 0.3
		vector.FillRect(screen, sx+pad, sy+pad, t-2*pad, t-2*pad, c, false)
	}
	ghostClr := color.RGBA{R: 90, G: 180, B: 220, A: 120}
	for _, f := range w.Frames {
		if sx, sy, ok := visible(f.X, f.Y); ok {
			vector.StrokeRect(screen, sx+3, sy+3, t-6, t-6, 2, ghostClr, false)
		}
	}
	wallClr := color.RGBA{R: 90, G: 90, B: 95, A: 255}
	bedClr := color.RGBA{R: 70, G: 90, B: 160, A: 255}
	for _, b := range w.Buildings {
		sx, sy, ok := visible(b.X, b.Y)
		if b.Floor || !ok {
			continue // floors are drawn before the zones
		}
		c := wallClr
		if b.Kind == "bed" {
			c = bedClr
		}
		vector.FillRect(screen, sx+4, sy+4, t-8, t-8, c, false)
	}
	p.Overlays.Zones.DrawZoneLabels(v, screen, w, sw, sh, v.Tile) // over the objects
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
		v.facing = artpkg.FacingFor(x-prev.x, y-prev.y, prev.facing)
		return v
	}
	v.fromX, v.fromY = prev.fromX, prev.fromY
	if tick-prev.moveTick >= clock.Tick(game.TicksPerCell) {
		v.fromX, v.fromY = x, y
	}
	return v
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
	if r.viewCached && s.src != nil && len(all) == len(s.src) && lightpkg.First(all) == lightpkg.First(s.src) {
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
	tile := r.cam.Tile()
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			sx, sy := r.worldToScreen(x, y)
			if spritepkg.Visible(sx, sy, float32(tile), float32(tile), sw, sh) {
				vector.StrokeRect(screen, sx+2, sy+2, float32(tile)-4, float32(tile)-4, 2, ghostClr, false)
			}
		}
	}
}
