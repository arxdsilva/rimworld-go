// Package ebiten adapts the headless application game to an interactive
// Ebitengine client: screen routing plus a colony map viewport. It never
// mutates domain state directly; it only calls the application game's
// Step/Snapshot surface.
package ebiten

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
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
	Width() int
	Height() int
	Tick() clock.Tick
	Date() clock.Date
	Activity() schedule.Activity
}

// TileSize is the on-screen pixel size of one logical tile at zoom 1,
// matching the art-spec reference of 64 px/tile.
const TileSize = 64

const (
	panPixels = 8
	minZoom   = 0.5
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
}

type pawnVisual struct {
	x, y         int
	fromX, fromY int
	flip         bool
	lastMove     uint64
	moveTick     clock.Tick
}

// Renderer drives an Ebitengine window from a running Simulation.
type Renderer struct {
	game  Simulation
	speed speed
	cam   camera
	pack  *atlas

	// present is a presentation-only frame counter. Animation reads it;
	// simulation never does. Tile lerp uses sim ticks vs moveTick.
	present   uint64
	selected  entity.EntityID
	hasSelect bool
	last      map[entity.EntityID]pawnVisual

	gridClr   color.RGBA
	pawnClr   color.RGBA
	selectClr color.RGBA
}

// New returns a Renderer for sim. Sprites are optional: call LoadPack
// after New. Without a pack, pawns fall back to colored rectangles.
func New(sim Simulation) *Renderer {
	return &Renderer{
		game:      sim,
		speed:     speedNormal,
		cam:       camera{zoom: 1},
		last:      map[entity.EntityID]pawnVisual{},
		gridClr:   color.RGBA{R: 40, G: 40, B: 40, A: 180},
		pawnClr:   color.RGBA{R: 220, G: 60, B: 60, A: 255},
		selectClr: color.RGBA{R: 250, G: 220, B: 60, A: 255},
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
	if n := r.speed.ticksPerFrame(); n > 0 {
		r.game.Step(n)
	}
	r.syncPawns()
}

func (r *Renderer) syncPawns() {
	tick := r.game.Tick()
	for _, p := range r.game.Snapshot() {
		prev, seen := r.last[p.ID]
		r.last[p.ID] = nextVisual(prev, seen, p.X, p.Y, tick, r.present)
	}
}

func (r *Renderer) handleSpeedInput() {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if r.speed == speedPaused {
			r.speed = speedNormal
		} else {
			r.speed = speedPaused
		}
	case inpututil.IsKeyJustPressed(ebiten.Key1):
		r.speed = speedNormal
	case inpututil.IsKeyJustPressed(ebiten.Key2):
		r.speed = speedFast
	case inpututil.IsKeyJustPressed(ebiten.Key3):
		r.speed = speedVeryFast
	}
}

func (r *Renderer) handleCameraInput() {
	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		r.cam.offsetX -= panPixels
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		r.cam.offsetX += panPixels
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		r.cam.offsetY -= panPixels
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		r.cam.offsetY += panPixels
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		r.cam.zoom = clampZoom(r.cam.zoom + zoomStep)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		r.cam.zoom = clampZoom(r.cam.zoom - zoomStep)
	}
	if _, dy := ebiten.Wheel(); dy != 0 {
		r.cam.zoom = clampZoom(r.cam.zoom + dy*zoomStep)
	}
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

func (r *Renderer) handleSelectionInput() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	wx, wy := r.screenToWorld(mx, my)

	for _, p := range r.game.Snapshot() {
		px, py := r.interpWorld(p)
		if wx >= px && wx < px+1 && wy >= py && wy < py+1 {
			r.selected, r.hasSelect = p.ID, true
			return
		}
	}
	tx, ty := int(wx), int(wy)
	for _, p := range r.game.Snapshot() {
		if p.X == tx && p.Y == ty {
			r.selected, r.hasSelect = p.ID, true
			return
		}
	}
	r.hasSelect = false
}

func (r *Renderer) screenToWorld(sx, sy int) (float64, float64) {
	tile := float64(TileSize) * r.cam.zoom
	return (float64(sx) + r.cam.offsetX) / tile, (float64(sy) + r.cam.offsetY) / tile
}

func (r *Renderer) interpWorld(p game.PawnView) (float64, float64) {
	vis, ok := r.last[p.ID]
	if !ok {
		return float64(p.X), float64(p.Y)
	}
	t := moveProgress(int(r.game.Tick()-vis.moveTick), game.TicksPerCell)
	return lerp(vis.fromX, p.X, t), lerp(vis.fromY, p.Y, t)
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

	r.drawTerrain(screen, sw, sh, tile)
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
	v := pawnVisual{x: x, y: y, flip: prev.flip, lastMove: prev.lastMove, moveTick: prev.moveTick}
	if !seen {
		v.fromX, v.fromY = x, y
		v.moveTick = tick
		return v
	}
	if prev.x != x || prev.y != y {
		v.fromX, v.fromY = prev.x, prev.y
		v.moveTick = tick
		v.lastMove = present
		if x != prev.x {
			v.flip = x < prev.x
		}
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

func (r *Renderer) snapshot() []game.PawnView { return r.game.Snapshot() }

func (r *Renderer) date() clock.Date { return r.game.Date() }

func (r *Renderer) activity() schedule.Activity { return r.game.Activity() }

func (r *Renderer) speedRate() string { return r.speed.rate() }
