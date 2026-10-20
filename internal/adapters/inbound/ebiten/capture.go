package ebiten

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
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
	// Frames > 0 records that many consecutive frames at normal speed
	// (Path gets a _NNN suffix per frame) instead of one still.
	Frames int
	// Speed is the game speed during a frame sequence: 1, 2 or 3
	// (0 means 1). Lets captures check motion at higher speeds.
	Speed int
}

// captureFrames lets presentation state (lerp, walk bob) settle.
const captureFrames = 30

type captureState struct {
	started bool
	frames  int
	saved   int  // sequence frames written
	pending bool // a sequence frame is waiting to be drawn
	follow  entity.EntityID
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
	bk("stove", 0, cx+1, cy+6)
	bk("workbench", 0, cx+4, cy+6)
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
	bk("sleeping spot", 0, x0+1, y0+1)
	bk("bed", 0, x0+3, y0+1)
	bk("table", 0, x0+2, y0+2)
	bk("chair", 1, x0+1, y0+2)
	bk("lamp", 0, x0+3, y0+3)
	out = append(out,
		command.Command{Kind: command.DesignateStockpile, X: cx + 1, Y: cy + 3, X2: cx + 3, Y2: cy + 4},
		command.Command{Kind: command.DesignateHarvest, X: cx + 2, Y: cy - 4, X2: cx + 5, Y2: cy - 4},
	)
	for i, crop := range []string{"radish", "lettuce", "corn", "wheat"} {
		id, _ := game.CropID(crop)
		out = append(out, command.Command{Kind: command.DesignateGrowZone, X: cx + 1 + i, Y: cy - 2, X2: cx + 1 + i, Y2: cy, Build: id})
	}
	out = append(out,
		command.Command{Kind: command.DesignateCut, X: cx + 6, Y: cy - 12, X2: cx + 20, Y2: cy + 10},
		command.Command{Kind: command.DesignateCut, X: cx - 20, Y: cy - 4, X2: cx - 8, Y2: cy + 10},
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
		a.renderer.settleVisuals()
		return
	}
	if c.Frames > 0 && a.capture.frames >= captureFrames {
		if a.capture.pending {
			return // wait until this frame is on disk
		}
		a.renderer.speed = speedNormal
		a.renderer.advance()
		for _, p := range a.renderer.snapshot() {
			if p.ID == a.capture.follow { // keep the followed colonist mid-screen
				wx, wy := a.renderer.interpWorld(p)
				a.renderer.centerOn(wx+0.5, wy+0.3, a.sw, a.sh)
			}
		}
		a.capture.pending = true
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
	case "bills":
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		a.renderer.enqueue(command.Command{Kind: command.AddBill, X: cx + 1, Y: cy + 6, Recipe: 0, Mode: billModeForever, Count: 1})
		for i := 0; i < 50; i++ {
			a.renderer.game.Step(60)
			a.renderer.syncPawns()
		}
		a.selectBuildingAt(cx+1, cy+6)
		tile := a.renderer.tilePx()
		a.renderer.cam.offsetX = float64(cx+1)*tile - float64(a.sw)/2 + 180
		a.renderer.cam.offsetY = float64(cy+5)*tile - float64(a.sh)/2
		x, y := at(cx+1, cy+6)
		a.cursor = &[2]int{x, y}
	case "zoomout", "zoomedge":
		// furthest zoom-out, centred, or pushed to the top-left limit
		v := colonyViewport(a.sw, a.sh)
		a.renderer.cam.zoom = a.renderer.minZoomFor(v)
		a.renderer.cam.placed = false
		if ui == "zoomedge" {
			a.renderer.cam.zoom *= 1.6
			a.renderer.clampCamera(v)
			a.renderer.cam.offsetX, a.renderer.cam.offsetY = -1e6, -1e6
		}
		a.renderer.clampCamera(v)
	case "work", "schedule", "research", "animals":
		a.workOpen, a.panelMode = true, ui
		if ui == "schedule" {
			a.schedBrush = 3
			pawns := a.renderer.snapshot()
			if len(pawns) > 1 { // give Bo a night-owl schedule to show
				for h := 0; h < 24; h++ {
					act := uint8(1)
					switch {
					case h >= 4 && h < 12:
						act = 3
					case h >= 12 && h < 15:
						act = 2
					}
					a.renderer.enqueue(command.Command{Kind: command.SetSchedule, Pawn: uint64(pawns[1].ID), Index: h, Mode: act})
				}
				a.renderer.enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: uint64(pawns[0].ID), Work: command.WorkGrow, Count: 1})
				a.renderer.game.Step(1)
				a.renderer.syncPawns()
			}
		}
	case "corridor", "crowd":
		// the -scenario corridor / crowd set pieces, north / east of the start
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		dx, dy := 0.0, -11.0
		if ui == "crowd" {
			dx, dy = 13, 0
		}
		sw, sh := a.size()
		a.renderer.centerOn(cx+dx, cy+dy, sw, sh)
	case "pen":
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		sw, sh := a.size()
		a.renderer.centerOn(cx+5, cy-2, sw, sh)
	case "medic":
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		sw, sh := a.size()
		a.renderer.centerOn(cx+2, cy-8, sw, sh)
		if pawns := a.renderer.snapshot(); len(pawns) > 1 {
			a.renderer.Select(pawns[1].ID)
		}
	case "defense":
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		sw, sh := a.size()
		a.renderer.centerOn(cx+1, cy, sw, sh)
	case "raid":
		// centre on the leading raider
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		if hs := a.renderer.hostiles(); len(hs) > 0 {
			cx, cy = float64(hs[0].X), float64(hs[0].Y)
		}
		sw, sh := a.size()
		a.renderer.centerOn(cx, cy, sw, sh)
	case "fun":
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		pawns := a.renderer.snapshot()
		sw, sh := a.size()
		a.renderer.centerOn(float64(cx)+4, float64(cy)+1, sw, sh)
		if len(pawns) > 1 {
			a.renderer.selected, a.renderer.hasSelect = pawns[1].ID, true
			a.renderer.centerOn(float64(pawns[1].X)-3, float64(pawns[1].Y), sw, sh)
		}
	case "dressed":
		// centre on the first colonist with its Gear tab open
		if pawns := a.renderer.snapshot(); len(pawns) > 0 {
			p := pawns[0]
			a.renderer.selected, a.renderer.hasSelect = p.ID, true
			a.inspectTab = "gear"
			sw, sh := a.size()
			a.renderer.centerOn(float64(p.X)-2, float64(p.Y)+0.5, sw, sh)
		}
	case "queue":
		// first colonist: go somewhere, then (queued) grab the rifle and
		// walk on
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		pawns := a.renderer.snapshot()
		if len(pawns) > 0 {
			id := uint64(pawns[0].ID)
			a.renderer.selected, a.renderer.hasSelect = pawns[0].ID, true
			a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: cx - 6, Y: cy - 3})
			a.renderer.enqueue(command.Command{Kind: command.Prioritize, Pawn: id, Order: command.OrderEquip, X: cx - 2, Y: cy + 2, Queue: true})
			a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: id, X: cx + 3, Y: cy + 1, Queue: true})
			a.renderer.game.Step(2)
			a.renderer.syncPawns()
		}
		sw, sh := a.size()
		a.renderer.centerOn(float64(cx)-1, float64(cy), sw, sh)
	case "winter":
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		sw, sh := a.size()
		a.renderer.centerOn(float64(cx)+12, float64(cy)+3.5, sw, sh)
		x, y := at(cx+9, cy+4)
		a.cursor = &[2]int{x, y}
	case "drying":
		// the -scenario drying racks, with the cursor on the first one
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		sw, sh := a.size()
		a.renderer.centerOn(float64(cx)+8, float64(cy)+3.5, sw, sh)
		x, y := at(cx+5, cy+6)
		a.cursor = &[2]int{x, y}
	case "hover":
		// hover a sown crop tile so the soil readout shows with plant info
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		x, y := at(cx+3, cy-1)
		a.cursor = &[2]int{x, y}
	case "grow":
		a.openArchitect()
		a.selectArchCategory("zones")
		a.selectArchTool("grow")
		a.archCrop = "corn"
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		a.archDrag = true
		a.archX0, a.archY0, a.archX1, a.archY1 = cx+6, cy-2, cx+8, cy
		x, y := at(cx+8, cy)
		a.cursor = &[2]int{x, y}
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
	case "hits":
		// run until a blow lands, then show it with its damage number
		g := a.renderer.game
		var hit game.TraceView
		found := false
		for i := 0; i < 20000 && !found; i++ {
			g.Step(1)
			if i%15 == 0 {
				a.renderer.syncPawns()
			}
			for _, t := range g.World().Traces {
				if t.Hit && t.Damage > 0 && t.Age == 1 {
					hit, found = t, true
				}
			}
		}
		a.renderer.syncPawns()
		for k := 0; k < 3; k++ { // the number has drifted up a little
			a.renderer.present++
			a.renderer.animate(g.World(), g.Snapshot(), true)
		}
		sw, sh := a.size()
		a.renderer.centerOn(float64(hit.ToX)+0.5, float64(hit.ToY), sw, sh)
	case "combat", "melee":
		melee := ui == "melee"
		g := a.renderer.game
		a.renderer.enqueue(command.Command{Kind: command.DesignateHunt, X: 0, Y: 0, X2: g.Width() - 1, Y2: g.Height() - 1})
		var shot game.TraceView
		found := false
		for i := 0; i < 20000 && !found; i++ {
			g.Step(1)
			if i%15 == 0 {
				a.renderer.syncPawns()
			}
			for _, t := range g.World().Traces {
				if t.Melee == melee && t.Age == 1 {
					shot, found = t, true
				}
			}
		}
		a.renderer.syncPawns()
		if !found {
			return
		}
		tile := a.renderer.tilePx()
		a.renderer.cam.offsetX = float64(shot.FromX+shot.ToX+1)/2*tile - float64(a.sw)/2
		a.renderer.cam.offsetY = float64(shot.FromY+shot.ToY+1)/2*tile - float64(a.sh)/2
		for _, p := range a.renderer.snapshot() {
			if p.X == shot.FromX && p.Y == shot.FromY {
				if a.cfg.Capture.Frames > 0 {
					a.capture.follow = p.ID
				} else {
					a.renderer.Select(p.ID)
				}
			}
		}
	case "follow":
		// run on until a colonist is doing hands-on work, then follow it
		for i := 0; i < 600; i++ {
			for _, p := range a.renderer.snapshot() {
				if strings.HasPrefix(p.Job, "rescue") || (p.Acting != "" && p.Acting != "eat" && a.capture.follow == 0) {
					a.capture.follow = p.ID
				}
			}
			if a.capture.follow != 0 {
				break
			}
			a.renderer.game.Step(10)
			a.renderer.syncPawns()
		}
	case "pawn":
		if snap := a.renderer.snapshot(); len(snap) > 0 {
			a.renderer.Select(snap[0].ID)
			x, y := at(snap[0].X, snap[0].Y)
			a.cursor = &[2]int{x, y}
		}
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
	path := c.Path
	if c.Frames > 0 {
		if !a.capture.pending {
			return
		}
		a.capture.pending = false
		n := a.capture.saved
		a.capture.saved++
		if a.capture.saved >= c.Frames {
			a.capture.done, a.shouldQuit = true, true
		}
		ext := filepath.Ext(path)
		path = fmt.Sprintf("%s_%03d%s", strings.TrimSuffix(path, ext), n, ext)
	} else {
		a.capture.done, a.shouldQuit = true, true
	}
	b := screen.Bounds()
	img := image.NewRGBA(b)
	screen.ReadPixels(img.Pix)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.capture.err = err
		return
	}
	f, err := os.Create(path)
	if err != nil {
		a.capture.err = err
		return
	}
	defer f.Close()
	a.capture.err = png.Encode(f, img)
}

// CaptureErr reports a failed capture run after RunGame returns.
func (a *App) CaptureErr() error { return a.capture.err }
