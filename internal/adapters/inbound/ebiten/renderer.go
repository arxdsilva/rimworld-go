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
	"rimworld-go/internal/domain/sim/entity"
)

const (
	baseTileSize = 24
	panPixels    = 6
	minZoom      = 0.5
	maxZoom      = 3.0
	zoomStep     = 0.1
)

// speed is a presentation-only game-speed tier. It controls how many
// simulation ticks are stepped per rendered frame; it never changes
// simulation rules.
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

// Renderer drives an Ebitengine window from a running Game.
type Renderer struct {
	game  *game.Game
	speed speed
	cam   camera

	selected  entity.EntityID
	hasSelect bool
	gridClr   color.RGBA
	pawnClr   color.RGBA
	selectClr color.RGBA
}

// New returns a Renderer for g.
func New(g *game.Game) *Renderer {
	return &Renderer{
		game:      g,
		speed:     speedNormal,
		cam:       camera{zoom: 1},
		gridClr:   color.RGBA{R: 40, G: 40, B: 40, A: 255},
		pawnClr:   color.RGBA{R: 220, G: 60, B: 60, A: 255},
		selectClr: color.RGBA{R: 250, G: 220, B: 60, A: 255},
	}
}

// Update advances the simulation according to the current speed tier and
// handles camera, speed, and selection input. It satisfies ebiten.Game.
func (r *Renderer) Update() error {
	r.handleSpeedInput()
	r.handleCameraInput()
	r.handleSelectionInput()

	if n := r.speed.ticksPerFrame(); n > 0 {
		r.game.Step(n)
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
	tile := baseTileSize * r.cam.zoom
	wx := (float64(sx) + r.cam.offsetX) / tile
	wy := (float64(sy) + r.cam.offsetY) / tile
	return int(wx), int(wy)
}

func (r *Renderer) worldToScreen(wx, wy int) (float32, float32) {
	tile := baseTileSize * r.cam.zoom
	sx := float64(wx)*tile - r.cam.offsetX
	sy := float64(wy)*tile - r.cam.offsetY
	return float32(sx), float32(sy)
}

// Draw renders the grid, every pawn, the selection marker, and the HUD.
// It satisfies ebiten.Game.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 15, G: 15, B: 18, A: 255})

	tile := float32(baseTileSize * r.cam.zoom)
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

	for _, p := range r.game.Snapshot() {
		x, y := r.worldToScreen(p.X, p.Y)
		vector.FillRect(screen, x+2, y+2, tile-4, tile-4, r.pawnClr, false)
		if r.hasSelect && p.ID == r.selected {
			vector.StrokeRect(screen, x+1, y+1, tile-2, tile-2, 2, r.selectClr, false)
		}
	}

	r.drawHUD(screen)
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
		return
	}
	// Selected pawn no longer exists (e.g. despawned); clear stale selection.
	r.hasSelect = false
}

// Layout satisfies ebiten.Game.
func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.game.Width() * baseTileSize, r.game.Height() * baseTileSize
}
