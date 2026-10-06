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

// captureOrders is a small colony layout around the map centre (cx, cy):
// a walled room with a door, floor and furniture, a stockpile, harvest
// orders on the start bushes, and cut/mine orders nearby.
func captureOrders(cx, cy int) []command.Command {
	var out []command.Command
	bk := func(kind string, f uint8, x, y int) {
		if id, ok := kindID(kind); ok {
			out = append(out, command.Command{Kind: command.DesignateBuild, Build: id, Facing: f, X: x, Y: y})
		}
	}
	x0, y0, x1, y1 := cx-5, cy-1, cx-1, cy+3
	for x := x0; x <= x1; x++ {
		for _, y := range []int{y0, y1} {
			if x == x0+2 && y == y1 {
				bk("door", 0, x, y)
				continue
			}
			bk("wall", 0, x, y)
		}
	}
	for y := y0 + 1; y < y1; y++ {
		bk("wall", 0, x0, y)
		bk("wall", 0, x1, y)
	}
	for y := y0 + 1; y < y1; y++ {
		for x := x0 + 1; x < x1; x++ {
			bk("floor", 0, x, y)
		}
	}
	bk("bed", 0, x0+1, y0+1)
	bk("bed", 0, x0+3, y0+1)
	bk("table", 0, x0+2, y0+2)
	bk("chair", 1, x0+1, y0+2)
	bk("lamp", 0, x0+3, y0+2)
	out = append(out,
		command.Command{Kind: command.DesignateStockpile, X: cx + 1, Y: cy + 3, X2: cx + 3, Y2: cy + 4},
		command.Command{Kind: command.DesignateHarvest, X: cx + 2, Y: cy - 4, X2: cx + 5, Y2: cy - 4},
		command.Command{Kind: command.DesignateGrowZone, X: cx + 1, Y: cy - 2, X2: cx + 4, Y2: cy},
		command.Command{Kind: command.DesignateCut, X: cx + 6, Y: cy - 6, X2: cx + 10, Y2: cy - 1},
		command.Command{Kind: command.DesignateMine, X: cx - 14, Y: cy - 8, X2: cx - 9, Y2: cy + 2},
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
		// centre the camera on the colony
		tile := a.renderer.tilePx()
		a.renderer.cam.offsetX = (float64(a.renderer.game.Width())/2+0.5)*tile - float64(a.sw)/2
		a.renderer.cam.offsetY = (float64(a.renderer.game.Height())/2)*tile - float64(a.sh)/2
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		for _, cmd := range captureOrders(cx, cy) {
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
		a.selectArchCategory("structure")
		a.selectArchTool("wall")
		a.archStuff = "stone"
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		a.archDrag = true
		a.archX0, a.archY0, a.archX1, a.archY1 = cx+1, cy-2, cx+5, cy-2
		x, y := at(cx+5, cy-2)
		a.cursor = &[2]int{x, y}
	case "orders":
		snap := a.renderer.snapshot()
		if len(snap) == 0 {
			return
		}
		a.renderer.Select(snap[0].ID)
		tx, ty := a.renderer.game.Width()/2+8, a.renderer.game.Height()/2-3 // near the cut orders
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
