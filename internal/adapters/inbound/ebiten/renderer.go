// Package ebiten adapts the headless application game to an interactive,
// on-screen renderer using Ebitengine. It never mutates domain state
// directly; it only calls the application game's Step/Snapshot surface.
package ebiten

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
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

func (s speed) framesPerTick() int {
	switch s {
	case speedNormal:
		return 15
	case speedFast:
		return 6
	case speedVeryFast:
		return 2
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
		return "very fast"
	default:
		return "paused"
	}
}

// camera is presentation-only state: it never affects authoritative
// simulation, only where/how the world is drawn.
type camera struct {
	offsetX, offsetY float64
	zoom             float64
}

type pawnVisual struct {
	x, y     int
	flip     bool
	lastMove uint64
}

// Renderer drives an Ebitengine window from a running Simulation.
type Renderer struct {
	game  Simulation
	speed speed
	cam   camera
	pack  *atlas

	// present is a presentation-only frame counter. Animation reads it;
	// simulation never does.
	present   uint64
	tickWait  int
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

// Update advances the simulation according to the current speed tier and
// handles camera, speed, and selection input. It satisfies ebiten.Game.
func (r *Renderer) Update() error {
	r.handleSpeedInput()
	r.handleCameraInput()
	r.handleSelectionInput()

	r.present++
	if period := r.speed.framesPerTick(); period > 0 {
		r.tickWait++
		if r.tickWait >= period {
			r.tickWait = 0
			r.game.Step(1)
		}
	} else {
		r.tickWait = 0
	}
	return nil
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
	wx, wy := r.screenToWorldTile(mx, my)

	for _, p := range r.game.Snapshot() {
		if p.X == wx && p.Y == wy {
			r.selected, r.hasSelect = p.ID, true
			return
		}
	}
	r.hasSelect = false
}

func (r *Renderer) screenToWorldTile(sx, sy int) (int, int) {
	tile := float64(TileSize) * r.cam.zoom
	wx := (float64(sx) + r.cam.offsetX) / tile
	wy := (float64(sy) + r.cam.offsetY) / tile
	return int(wx), int(wy)
}

func (r *Renderer) worldToScreen(wx, wy int) (float32, float32) {
	tile := float64(TileSize) * r.cam.zoom
	sx := float64(wx)*tile - r.cam.offsetX
	sy := float64(wy)*tile - r.cam.offsetY
	return float32(sx), float32(sy)
}

func (r *Renderer) tilePx() float64 {
	return float64(TileSize) * r.cam.zoom
}

func visible(x, y, w, h float32, sw, sh int) bool {
	return x+w >= 0 && y+h >= 0 && x < float32(sw) && y < float32(sh)
}

// Draw renders terrain, pawns, selection, and the HUD.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 28, G: 42, B: 32, A: 255})

	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	tile := r.tilePx()

	r.drawTerrain(screen, sw, sh, tile)
	r.drawGrid(screen)

	for _, p := range r.game.Snapshot() {
		sx, sy := r.worldToScreen(p.X, p.Y)
		if !visible(sx, sy, float32(tile), float32(tile)*2, sw, sh) {
			continue
		}
		r.drawPawn(screen, p, sx, sy, tile)
		if r.hasSelect && p.ID == r.selected {
			r.drawSelection(screen, sx, sy, tile)
		}
	}

	r.drawHUD(screen)
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

func (r *Renderer) drawPawn(screen *ebiten.Image, p game.PawnView, sx, sy float32, tile float64) {
	ss := r.pack.colonist(p.ID)
	prev, seen := r.last[p.ID]
	moved := seen && (prev.x != p.X || prev.y != p.Y)
	flip := prev.flip
	if moved && p.X != prev.x {
		flip = p.X < prev.x
	}
	lastMove := prev.lastMove
	if moved {
		lastMove = r.present
	}
	r.last[p.ID] = pawnVisual{x: p.X, y: p.Y, flip: flip, lastMove: lastMove}

	if ss == nil {
		vector.FillRect(screen, sx+2, sy+2, float32(tile)-4, float32(tile)-4, r.pawnClr, false)
		return
	}
	frames := ss.idle
	period := uint64(idleAnimPeriod)
	if len(ss.walk) > 0 && walkHold(r.present, lastMove, walkSustainFrames) {
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
	drawImage(screen, img, x, y, w, h, flip)
}

func (r *Renderer) drawSelection(screen *ebiten.Image, sx, sy float32, tile float64) {
	if r.pack != nil && r.pack.crosshair != nil {
		pad := tile * 0.05
		drawImage(screen, r.pack.crosshair, float64(sx)+pad, float64(sy)+pad, tile-2*pad, tile-2*pad, false)
		return
	}
	vector.StrokeRect(screen, sx+1, sy+1, float32(tile)-2, float32(tile)-2, 2, r.selectClr, false)
}

func (r *Renderer) drawHUD(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, fmt.Sprintf(
		"tick=%d %s [%s]  space=pause 1/2/3=speed  wasd/arrows=pan +/-=zoom",
		r.game.Tick(), r.game.Date(), r.speed.label()))

	if !r.hasSelect {
		return
	}
	for _, p := range r.game.Snapshot() {
		if p.ID != r.selected {
			continue
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("selected pawn %d\nposition (%d, %d)", p.ID, p.X, p.Y), 8, 32)
		if ss := r.pack.colonist(p.ID); ss != nil {
			if img := animFrame(ss.idle, r.present, uint64(idleAnimPeriod)); img != nil {
				drawImage(screen, img, 8, 64, 48, 72, false)
			}
		}
		return
	}
	r.hasSelect = false
}

// Layout satisfies ebiten.Game.
func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.game.Width() * TileSize, r.game.Height() * TileSize
}
