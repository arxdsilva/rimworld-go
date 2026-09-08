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
)

const tileSize = 24

// Renderer drives an Ebitengine window from a running Game.
type Renderer struct {
	game    *game.Game
	paused  bool
	tilePx  int
	pawnClr color.RGBA
	gridClr color.RGBA
}

// New returns a Renderer for g.
func New(g *game.Game) *Renderer {
	return &Renderer{
		game:    g,
		tilePx:  tileSize,
		pawnClr: color.RGBA{R: 220, G: 60, B: 60, A: 255},
		gridClr: color.RGBA{R: 40, G: 40, B: 40, A: 255},
	}
}

// Update advances the simulation by one tick per frame unless paused, and
// handles pause input. It satisfies ebiten.Game.
func (r *Renderer) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		r.paused = !r.paused
	}
	if !r.paused {
		r.game.Step(1)
	}
	return nil
}

// Draw renders the grid and every pawn. It satisfies ebiten.Game.
func (r *Renderer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 15, G: 15, B: 18, A: 255})

	for x := 0; x <= r.game.Width(); x++ {
		px := float32(x * r.tilePx)
		vector.StrokeLine(screen, px, 0, px, float32(r.game.Height()*r.tilePx), 1, r.gridClr, false)
	}
	for y := 0; y <= r.game.Height(); y++ {
		py := float32(y * r.tilePx)
		vector.StrokeLine(screen, 0, py, float32(r.game.Width()*r.tilePx), py, 1, r.gridClr, false)
	}

	for _, p := range r.game.Snapshot() {
		x := float32(p.X*r.tilePx + 2)
		y := float32(p.Y*r.tilePx + 2)
		size := float32(r.tilePx - 4)
		vector.FillRect(screen, x, y, size, size, r.pawnClr, false)
	}

	status := "running"
	if r.paused {
		status = "paused"
	}
	ebitenutil.DebugPrint(screen, fmt.Sprintf("tick=%d [%s]  space=pause", r.game.Tick(), status))
}

// Layout satisfies ebiten.Game.
func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.game.Width() * r.tilePx, r.game.Height() * r.tilePx
}
