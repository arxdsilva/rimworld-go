package ebiten

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/command"
)

// Capture is a developer screenshot run (ART Wave 1 "screenshot test
// scene"): boot, start a colony, issue a fixed set of ordinary player
// commands, fast-forward the simulation, save one rendered frame to Path
// and quit. It only uses the same Enqueue/Step calls as the interactive
// client, so the simulation stays deterministic for a given seed.
type Capture struct {
	Path  string
	Ticks int // simulation ticks to advance before the shot
	Zoom  float64
	// UI opens a UI state for the shot: "" (none), "architect" (furniture
	// card + placement ghost) or "orders" (a pawn's right-click menu).
	UI string
}

// captureFrames lets presentation state (lerp, walk bob) settle.
const captureFrames = 30

type captureState struct {
	started bool
	frames  int
	done    bool
	err     error
}

// captureOrders is a small colony layout built from the start kit:
// a walled room with a door gap and two beds, a stockpile, and harvest
// orders on the wild plants.
func captureOrders() []command.Command {
	var out []command.Command
	bk := func(kind string, f uint8, x, y int) {
		if id, ok := kindID(kind); ok {
			out = append(out, command.Command{Kind: command.DesignateBuild, Build: id, Facing: f, X: x, Y: y})
		}
	}
	bk("door", 0, 4, 12)
	bk("bed", 0, 3, 9)
	bk("bed", 0, 5, 9)
	bk("table", 0, 4, 10)
	bk("chair", 1, 3, 10)
	bk("lamp", 0, 5, 11)
	for y := 9; y <= 11; y++ {
		for x := 3; x <= 5; x++ {
			bk("floor", 0, x, y)
		}
	}
	x0, y0, x1, y1 := 2, 8, 6, 12
	for x := x0; x <= x1; x++ {
		for _, y := range []int{y0, y1} {
			if x == 4 && y == y1 {
				continue // door gap
			}
			out = append(out, command.Command{Kind: command.DesignateWall, X: x, Y: y})
		}
	}
	for y := y0 + 1; y < y1; y++ {
		out = append(out, command.Command{Kind: command.DesignateWall, X: x0, Y: y},
			command.Command{Kind: command.DesignateWall, X: x1, Y: y})
	}
	out = append(out,
		command.Command{Kind: command.DesignateStockpile, X: 9, Y: 9, X2: 11, Y2: 10},
		command.Command{Kind: command.DesignateHarvest, X: 12, Y: 4, X2: 15, Y2: 4},
	)
	return out
}

func (a *App) tickCapture() {
	c := a.cfg.Capture
	if c == nil || a.capture.done {
		return
	}
	if !a.capture.started {
		if a.nav.screen() == screenBoot {
			a.finishBoot()
		}
		a.startGame()
		if a.renderer == nil {
			a.capture.err = fmt.Errorf("capture: %s", a.sessionErr)
			a.capture.done, a.shouldQuit = true, true
			return
		}
		a.renderer.speed = speedPaused
		if c.Zoom > 0 {
			a.renderer.cam.zoom = clampZoom(c.Zoom)
		}
		// centre the camera on the colony (tile 8, 8)
		tile := a.renderer.tilePx()
		a.renderer.cam.offsetX = 8.5*tile - float64(a.sw)/2
		a.renderer.cam.offsetY = 8*tile - float64(a.sh)/2
		for _, cmd := range captureOrders() {
			a.renderer.enqueue(cmd)
		}
		for left := c.Ticks; left > 0; left -= 60 {
			n := 60
			if left < n {
				n = left
			}
			a.renderer.game.Step(n)
			a.renderer.syncPawns()
		}
		a.capture.started = true
		a.captureUI(c.UI)
		return
	}
	a.renderer.present++
	a.capture.frames++
}

// captureUI puts the client into a UI state for the screenshot, with a
// synthetic cursor so hover-driven previews render.
func (a *App) captureUI(ui string) {
	at := func(tx, ty int) (int, int) {
		sx, sy := a.renderer.worldToScreenF(float64(tx)+0.5, float64(ty)+0.5)
		return int(sx), int(sy)
	}
	switch ui {
	case "architect":
		a.openArchitect()
		a.selectArchCategory("furniture")
		a.selectArchTool("bed")
		a.archFacing = 1
		x, y := at(8, 6)
		a.cursor = &[2]int{x, y}
	case "orders":
		snap := a.renderer.snapshot()
		if len(snap) == 0 {
			return
		}
		a.renderer.Select(snap[0].ID)
		tx, ty := 9, 10 // a food crate in the stockpile
		x, y := at(tx, ty)
		a.cursor = &[2]int{x + 20, y}
		a.openOrders(tx, ty, float32(x), float32(y))
	}
}

// drawCapture saves the frame once presentation has settled.
func (a *App) drawCapture(screen *ebiten.Image) {
	c := a.cfg.Capture
	if c == nil || a.capture.done || !a.capture.started || a.capture.frames < captureFrames {
		return
	}
	a.capture.done, a.shouldQuit = true, true
	b := screen.Bounds()
	img := image.NewRGBA(b)
	screen.ReadPixels(img.Pix)
	if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
		a.capture.err = err
		return
	}
	f, err := os.Create(c.Path)
	if err != nil {
		a.capture.err = err
		return
	}
	defer f.Close()
	a.capture.err = png.Encode(f, img)
}

// CaptureErr reports a failed capture run after RunGame returns.
func (a *App) CaptureErr() error { return a.capture.err }
