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
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
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
	// OrdersAt lists what a pawn can be ordered to do at a tile.
	OrdersAt(pawn entity.EntityID, x, y int) []game.OrderOption
	// CanZoneAt reports whether a zone of kind ("stockpile", "grow",
	// "pen") could claim a tile: the zone drag ghost per cell.
	CanZoneAt(kind string, x, y int) bool
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

// Presentation animation is paced by Update frames (Ebitengine default
// 60 TPS), never by authoritative simulation ticks. Periods are chosen
// so a full idle/walk cycle is about a second at normal zoom.
const (
	idleAnimPeriod    = 12
	walkAnimPeriod    = 8
	walkSustainFrames = 20
)

// speed is a presentation-only game-speed tier. It controls how often
// the simulation advances; it never changes simulation rules.
type speed int

const (
	speedPaused speed = iota
	speedNormal
	speedFast
	speedVeryFast
)

func (s speed) ticksPerFrame() int {
	switch s {
	case speedNormal:
		return 1
	case speedFast:
		return 3
	case speedVeryFast:
		return 6
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
	layerTrace func(string) // test hook: ground layers in draw order
	game       Simulation
	zoneGeoms  map[uint16]*zoneGeom // zones_draw.go outline cache
	wheelTaken bool                 // the mouse wheel scrolled a log this frame
	speed      speed
	cam        camera
	pack       *atlas
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
	itemsInit bool
	drawW     game.WorldView  // world for the frame being drawn
	drawPawns []game.PawnView // pawns for the frame being drawn
	mo        motionState     // per-creature motion and the animT clock (motion.go)
	anims     *animState      // body-animation bookkeeping (anim.go)
	smooth    camSmooth       // eased pan and zoom (camera_smooth.go)
	env       envState        // wind, contact springs and weather (environment.go)
	lit       litRenderer     // sun, moon and lamp lighting (light.go)
	// cast, while set, turns world sprite draws into shadow silhouettes
	// (shadow_cast.go).
	cast *shadowCast
	// roofOverlay is the Roof overlay toggle (R); roofOverlayForced holds
	// it on while a roof tool is selected (roof_draw.go).
	roofOverlay, roofOverlayForced bool

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

	// sound (sound_signals.go): cues for the App's director, last frame's
	// world to hear changes, the pending thunder and the world this frame.
	cues       []soundCue
	memo       soundMemo
	thunderAt  uint64
	threatAt   uint64 // present frame of the newest threat
	heard      game.WorldView
	dangerInit bool
	lastDanger uint64
	floaters   []floater
}

// New returns a Renderer for sim. Sprites are optional: call LoadPack
// after New. Without a pack, pawns fall back to colored rectangles.
func New(sim Simulation) *Renderer {
	return &Renderer{
		game:          sim,
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

// LoadPack discovers and decodes the vendor sprite pack. Only colonist
// idle/walk, ground, rocks, and the crosshair are uploaded as textures.
func (r *Renderer) LoadPack(root string) error {
	p, err := DiscoverPack(root)
	if err != nil {
		return err
	}
	a, err := loadAtlas(p)
	if err != nil {
		return err
	}
	r.pack = a
	return nil
}

// LoadDefaultPack locates the vendor pack from cwd/parents and loads it.
func (r *Renderer) LoadDefaultPack() error {
	root, err := LocatePack()
	if err != nil {
		return err
	}
	return r.LoadPack(root)
}

func (r *Renderer) advance() {
	r.present++
	n := r.speed.ticksPerFrame()
	r.mo.stepped = n
	if n > 0 {
		r.game.Step(n)
	}
	r.syncPawns()
	w := r.game.World()
	r.animate(w, r.game.Snapshot(), n > 0)
	r.stepEnvironment(w, r.game.Snapshot())
	r.watchThreats(w)
	r.heard = w
	if n > 0 {
		r.listenWorld(w, r.game.Snapshot())
	}
	r.listenThunder()
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

func (r *Renderer) handleSpeedInput() {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if r.speed == speedPaused {
			r.speed = speedNormal
			r.globalCue("ui.resume")
		} else {
			r.speed = speedPaused
			r.globalCue("ui.pause")
		}
	case inpututil.IsKeyJustPressed(ebiten.Key1):
		r.speed = speedNormal
		r.globalCue("ui.speed")
	case inpututil.IsKeyJustPressed(ebiten.Key2):
		r.speed = speedFast
		r.globalCue("ui.speed")
	case inpututil.IsKeyJustPressed(ebiten.Key3):
		r.speed = speedVeryFast
		r.globalCue("ui.speed")
	case inpututil.IsKeyJustPressed(ebiten.KeyR):
		r.roofOverlay = !r.roofOverlay
	}
}

func (r *Renderer) handleCameraInput(v viewport) {
	var dx, dy float64
	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		dx--
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		dx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		dy--
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		dy++
	}
	if r.edgeScroll && ebiten.IsFocused() {
		mx, my := ebiten.CursorPosition()
		ex, ey := edgePan(mx, my, v)
		dx, dy = dx+ex, dy+ey
	}
	r.smooth.pushPan(dx, dy)

	cx, cy := (v.x0+v.x1)/2, (v.y0+v.y1)/2
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		r.smooth.zoomAt(1+zoomStep, cx, cy)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		r.smooth.zoomAt(1/(1+zoomStep), cx, cy)
	}
	if _, wy := ebiten.Wheel(); wy != 0 && !r.wheelTaken {
		mx, my := ebiten.CursorPosition()
		r.smooth.zoomAt(math.Pow(1+zoomStep, wy), float64(mx), float64(my))
	}
	r.clampCamera(v) // first-call centring and min-zoom guard
	r.smooth.step(1/float64(ebiten.TPS()), &r.cam, r.camLimits(v))
}

// edgeBand is how close (px) the cursor must be to a viewport edge to pan.
const edgeBand = 6

// edgePan is the pan direction for a cursor at (mx, my): towards any
// viewport edge within edgeBand, nothing when the cursor is outside it.
func edgePan(mx, my int, v viewport) (dx, dy float64) {
	x, y := float64(mx), float64(my)
	if x < v.x0 || x >= v.x1 || y < v.y0 || y >= v.y1 {
		return 0, 0
	}
	switch {
	case x < v.x0+edgeBand:
		dx = -1
	case x >= v.x1-edgeBand:
		dx = 1
	}
	switch {
	case y < v.y0+edgeBand:
		dy = -1
	case y >= v.y1-edgeBand:
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

func (r *Renderer) screenToWorld(sx, sy int) (float64, float64) {
	tile := float64(TileSize) * r.cam.zoom
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
	tile := float64(TileSize) * r.cam.zoom
	sx := wx*tile - r.cam.offsetX
	sy := wy*tile - r.cam.offsetY
	return float32(sx), float32(sy)
}

func (r *Renderer) tilePx() float64 {
	return float64(TileSize) * r.cam.zoom
}

func visible(x, y, w, h float32, sw, sh int) bool {
	return x+w >= 0 && y+h >= 0 && x < float32(sw) && y < float32(sh)
}

// Draw renders terrain, pawns, and selection. HUD chrome is App-owned.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 28, G: 42, B: 32, A: 255})

	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.tilePx()

	if r.art != nil {
		r.drawArt(screen, sw, sh, tile) // lit by light.go: no veil
		r.drawRoofOverlay(screen, r.drawW, sw, sh, tile)
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
			if g := r.pack.groundAt(x, y); g != nil {
				drawImage(screen, g, float64(sx), float64(sy), tile, tile, false)
			}
			if rock := r.pack.rockAt(x, y); rock != nil {
				pad := tile * 0.15
				drawImage(screen, rock, float64(sx)+pad, float64(sy)+pad, tile-2*pad, tile-2*pad, false)
			}
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

func (r *Renderer) drawPawn(screen *ebiten.Image, p game.PawnView, sx, sy float32, tile float64) {
	ss := r.pack.colonist(p.ID)
	vis := r.last[p.ID]
	inTransit := vis.fromX != p.X || vis.fromY != p.Y

	if ss == nil {
		vector.FillRect(screen, sx+2, sy+2, float32(tile)-4, float32(tile)-4, r.pawnClr, false)
		return
	}
	frames := ss.idle
	period := uint64(idleAnimPeriod)
	if len(ss.walk) > 0 && (inTransit || walkHold(r.present, vis.lastMove, walkSustainFrames)) {
		frames = ss.walk
		period = uint64(walkAnimPeriod)
	}
	img := animFrame(frames, r.present, period)
	if img == nil {
		vector.FillRect(screen, sx+2, sy+2, float32(tile)-4, float32(tile)-4, r.pawnClr, false)
		return
	}
	// Side-view characters are taller than a tile; sit them on the cell.
	h := tile * 1.7
	w := h * float64(img.Bounds().Dx()) / float64(img.Bounds().Dy())
	x := float64(sx) + (tile-w)/2
	y := float64(sy) + tile - h
	drawImage(screen, img, x, y, w, h, vis.flip)
}

func (r *Renderer) drawSelection(screen *ebiten.Image, sx, sy float32, tile float64) {
	if r.pack != nil && r.pack.crosshair != nil {
		pad := tile * 0.05
		drawImage(screen, r.pack.crosshair, float64(sx)+pad, float64(sy)+pad, tile-2*pad, tile-2*pad, false)
		return
	}
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
	all := r.game.Snapshot()
	out := all[:0:0]
	for _, p := range all {
		if !p.Hostile {
			out = append(out, p)
		}
	}
	return out
}

// hostiles is the raiders on the map.
func (r *Renderer) hostiles() []game.PawnView {
	var out []game.PawnView
	for _, p := range r.game.Snapshot() {
		if p.Hostile {
			out = append(out, p)
		}
	}
	return out
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
