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
	// card + placement ghost), "orders" (a pawn's right-click menu), or a
	// menu screen ("menu", "credits", "options[-tab]"; see menuCaptures;
	// no game is started).
	UI string
	// Frames > 0 records that many consecutive frames at normal speed
	// (Path gets a _NNN suffix per frame) instead of one still.
	Frames int
	// Speed is the game speed during a frame sequence: 1, 2 or 3
	// (0 means 1). Lets captures check motion at higher speeds.
	Speed int
	// Timelapse > 0 steps the simulation this many ticks per recorded
	// frame instead of playing in real time (day-cycle clips).
	Timelapse int
	// Bare skips the scripted starter orders (blueprints, cut and mine
	// marks), for set-piece scenarios that build their own scene.
	Bare bool
	// UIScale > 0 shoots at that UI scale (a 1280s x 720s frame, device
	// scale 1) to review scaled layouts; 0 keeps captures at scale 1
	// (PLAY-017).
	UIScale float64
}

// captureScale is the UI scale a capture asks for (0: none).
func (a *App) captureScale() float64 {
	if c := a.cfg.Capture; c != nil && c.UIScale > 0 {
		return c.UIScale
	}
	return 0
}

// speed is the game speed tier a frame sequence records at.
func (c *Capture) speed() speed {
	switch c.Speed {
	case 2:
		return speedFast
	case 3:
		return speedVeryFast
	}
	return speedNormal
}

// captureFrames lets presentation state (lerp, walk bob) settle.
const captureFrames = 30

type captureState struct {
	opened  int // menu-capture clicks applied so far
	started bool
	frames  int
	saved   int  // sequence frames written
	pending bool // a sequence frame is waiting to be drawn
	follow  entity.EntityID
	pan     [][2]float64 // camera waypoints (tiles) eased through over a frame sequence
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
	if screen, ok := menuCaptures[c.UI]; ok {
		a.tickMenuCapture(screen)
		return
	}
	if !a.capture.started {
		if a.nav.screen() == screenBoot {
			a.finishBoot()
		}
		a.captureStart(c.UI) // capture_colony.go: a setup colony for the colony UI states
		if a.renderer == nil {
			a.capture.err = fmt.Errorf("capture: %s", a.sessionErr)
			a.capture.done, a.shouldQuit = true, true
			return
		}
		a.renderer.speed = speedPaused
		if c.Zoom > 0 {
			a.renderer.cam.Zoom = clampZoom(c.Zoom)
		}
		// centre the camera on the colony
		tile := a.renderer.tilePx()
		a.renderer.cam.OffsetX = (float64(a.renderer.game.Width())/2+0.5)*tile - float64(a.sw)/2
		a.renderer.cam.OffsetY = (float64(a.renderer.game.Height())/2)*tile - float64(a.sh)/2
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		if !c.Bare {
			for _, cmd := range captureOrders(cx, cy) {
				a.renderer.enqueue(cmd)
			}
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
		a.renderer.speed = c.speed()
		if c.Timelapse > 0 {
			for left := c.Timelapse; left > 0; left -= 60 {
				a.renderer.game.Step(min(left, 60))
				a.renderer.syncPawns()
			}
			a.renderer.speed = speedPaused
			a.renderer.settleVisuals()
		}
		a.renderer.advance()
		if wp := a.capture.pan; len(wp) > 1 && c.Frames > 1 {
			x, y := panAt(wp, float64(a.capture.saved)/float64(c.Frames-1))
			a.renderer.centerOn(x, y, a.sw, a.sh)
		}
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

// menuCaptures are the -capture-ui states shot without starting a game,
// each the button clicks that open it from the main menu.
var menuCaptures = map[string][]string{
	"menu":             nil,
	"picker":           capLanding,
	"seed":             capWorld,
	"credits":          {"menu.credits"},
	"options":          {"menu.options"},
	"options-graphics": {"menu.options", "options.tab.graphics"},
	"options-gameplay": {"menu.options", "options.tab.gameplay"},
	"options-access":   {"menu.options", "options.tab.access"},
	"controls":         {"menu.options", "options.tab.controls"},
	// NG-1 setup screens. Each click runs on its own tick, so the screen
	// it opened has declared its widgets before the next id is clicked.
	"scenario":                {"menu.newgame"},
	"scenario-editor":         {"menu.newgame", "scen.edit"},
	"scenario-editor-addpart": {"menu.newgame", "scen.edit", "sced.add"},
	"storyteller":             {"menu.newgame", "wizard.next"},
	"difficulty-custom":       {"menu.newgame", "wizard.next", "diff.custom"},
	"worldgen":                capWorld,
	"worldgen-progress":       append(append([]string(nil), capWorld...), "@worldgen-progress"),
	"landing":                 capLanding,
	"landing-advanced":        append(append([]string(nil), capLanding...), "land.advanced"),
	"landing-factions":        append(append([]string(nil), capLanding...), "land.factions"),
	"pawns":                   append(append([]string(nil), capLanding...), "wizard.next"),
	"pawns-reroll":            append(append([]string(nil), capLanding...), "wizard.next", "pawns.reroll"),
}

// The click scripts that reach Create World and Select Landing Site. A
// token starting with "@" is a scripted step (captureStep) instead of a
// click.
var (
	capWorld   = []string{"menu.newgame", "wizard.next", "wizard.next", "@seed"}
	capLanding = []string{"menu.newgame", "wizard.next", "wizard.next", "@seed", "@landing"}
)

// captureStep runs a scripted step of a menu capture: a fixed world seed,
// a generated world with a landing site chosen, or a generation frozen
// part-way.
func (a *App) captureStep(step string) {
	ng := a.draft()
	switch step {
	case "@seed":
		ng.world.SeedText = "amber-hollow"
	case "@landing":
		a.worldGenerate()
		for ng.builder != nil { // run the generation to the end at once
			a.worldStep()
		}
		a.uiSt().modals = nil
		if ng.planet != nil {
			ng.tile = ng.planet.RandomLanding(0)
			if ng.tile >= 0 {
				ng.landRolls = 1
			}
		}
	case "@worldgen-progress":
		ng.genHold = 0.4
		a.worldGenerate()
		ng.genStep = 1500
	}
}

// tickMenuCapture shoots a menu screen without starting a game: Ticks
// picks the moment of the menu's motion, and a frame sequence advances it
// one tick per frame.
func (a *App) tickMenuCapture(open []string) {
	c := a.cfg.Capture
	if !a.capture.started {
		if a.nav.screen() == screenBoot {
			a.finishBoot()
		}
		if a.capture.opened < len(open) {
			if a.cursor == nil {
				a.cursor = &[2]int{-100, -100} // no hover state in the shot
			}
			if step := open[a.capture.opened]; strings.HasPrefix(step, "@") {
				a.captureStep(step)
			} else {
				a.applyClick(step)
			}
			a.capture.opened++
			return
		}
		a.menuTick = uint64(max(c.Ticks, 0))
		a.capture.started = true
		return
	}
	if c.Frames > 0 && a.capture.frames >= captureFrames {
		if !a.capture.pending {
			a.menuTick++
			a.capture.pending = true
		}
		return
	}
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
	case "perf":
		a.settings.Graphics.PerfOverlay = true
		a.seedPerfCapture() // perf_overlay.go: fixed samples, frozen clock
	case "bills":
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		a.renderer.enqueue(command.Command{Kind: command.AddBill, X: cx + 1, Y: cy + 6, Recipe: 0, Mode: billModeForever, Count: 1})
		for i := 0; i < 50; i++ {
			a.renderer.game.Step(60)
			a.renderer.syncPawns()
		}
		a.selectBuildingAt(cx+1, cy+6)
		a.openPaneTab("bills") // hud_inspect.go: the Bills tab open above the pane
		tile := a.renderer.tilePx()
		a.renderer.cam.OffsetX = float64(cx+1)*tile - float64(a.sw)/2 + 180
		a.renderer.cam.OffsetY = float64(cy+5)*tile - float64(a.sh)/2
		x, y := at(cx+1, cy+6)
		a.cursor = &[2]int{x, y}
	case "zoomout", "zoomedge":
		// furthest zoom-out, centred, or pushed to the top-left limit
		v := colonyViewport(a.sw, a.sh)
		a.renderer.cam.Zoom = a.renderer.MinZoomFor(v)
		a.renderer.cam.Placed = false
		if ui == "zoomedge" {
			a.renderer.cam.Zoom *= 1.6
			a.renderer.ClampCamera(v)
			a.renderer.cam.OffsetX, a.renderer.cam.OffsetY = -1e6, -1e6
		}
		a.renderer.ClampCamera(v)
	case "wild":
		// the Animals tab's Wild list
		a.workOpen, a.panelMode, a.animalsWild = true, "animals", true
	case "inspector", "trough":
		// the -scenario pen: an animal with needs, or the trough, selected
		a.captureBiomeInspect(ui)
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
	case "lategame":
		// a slow pan: the farms, the compound, the sandbag line, the
		// graveyard (the -scenario lategame layout)
		a.capture.pan = [][2]float64{{18, 10}, {33, 10}, {42, 18}, {16, 17}}
		sw, sh := a.size()
		a.renderer.centerOn(a.capture.pan[0][0], a.capture.pan[0][1], sw, sh)
	case "zoo":
		// the -scenario zoo pen (6 rows of 12 animals from (4, 3), 5 apart)
		sw, sh := a.size()
		a.renderer.centerOn(4+11*5/2.0, 3+5*5/2.0, sw, sh)
	case "frame":
		// a wall frame with part of its materials delivered, selected
		w := a.renderer.world()
		var pick *game.FrameView
		for i := range w.Frames {
			f := &w.Frames[i]
			if f.Floor || f.Kind == "carpet" {
				continue
			}
			if pick == nil || (f.Wood > 0 && f.Wood < f.Need && !(pick.Wood > 0 && pick.Wood < pick.Need)) {
				pick = f
			}
		}
		if pick != nil {
			a.selectCand(selCand{selFrame, pick.ID})
			sw, sh := a.size()
			a.renderer.centerOn(float64(pick.X)+3, float64(pick.Y), sw, sh)
		}
	case "fence":
		// the -scenario fence field
		sw, sh := a.size()
		a.renderer.centerOn(float64(a.renderer.game.Width()/2+12), float64(a.renderer.game.Height()/2-3), sw, sh)
	case "forbid":
		// the -scenario forbid items, the first forbidden one selected
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		sw, sh := a.size()
		a.renderer.centerOn(float64(cx+6), float64(cy-5), sw, sh)
		for _, it := range a.renderer.world().Items {
			if it.Forbidden && !it.Carried {
				a.selectCand(selCand{selItem, it.ID})
				break
			}
		}
	case "plant", "item", "pawnactions", "move", "copy":
		// UI-011 panels: the nearest tree, ground item, colonist or
		// building to the map centre, selected, its panel open
		a.captureInspect(ui)
	case "zones":
		a.captureZones(at)
	case "power", "power-overlay":
		// PLAY-021 (with -scenario power): the base north of the start,
		// and the power grid overlay on for power-overlay
		sw, sh := a.size()
		a.renderer.centerOn(float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2-9, sw, sh)
		a.renderer.pov.on = ui == "power-overlay"
	case "zone", "zone-shrink":
		a.captureZone(ui == "zone-shrink", at)
	case "paths":
		// every colonist walking somewhere: dotted routes, one drafted
		g := a.renderer.game
		pawns := a.renderer.snapshot()
		for i, p := range pawns {
			tx, ty := p.X+9-3*i, p.Y+(i%2)*10-4
			if i == 1 {
				g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: true})
			}
			g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: tx, Y: ty})
		}
		for i := 0; i < 4; i++ {
			g.Step(3)
			a.renderer.syncPawns()
		}
		if len(pawns) > 0 {
			sw, sh := a.size()
			a.renderer.centerOn(float64(pawns[0].X)+4, float64(pawns[0].Y)+3, sw, sh)
			if len(pawns) > 2 {
				a.renderer.Select(pawns[2].ID)
			}
		}
	case "log":
		// the colonist inspector's Log tab
		if pawns := a.renderer.snapshot(); len(pawns) > 0 {
			a.renderer.Select(pawns[0].ID)
			a.openPaneTab("log")
		}
	case "paused-stockpile":
		// a stockpile painted while paused shows at once
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		a.renderer.speed = speedPaused
		a.renderer.enqueue(command.Command{Kind: command.DesignateStockpile, X: cx - 6, Y: cy + 2, X2: cx - 2, Y2: cy + 5})
	case "roofs", "roofs-overlay":
		// the -scenario roofs set piece: rooms A and B, the porch
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		sw, sh := a.size()
		a.renderer.centerOn(cx+6, cy-3, sw, sh)
		a.renderer.roofOverlay = ui == "roofs-overlay"
	case "burial":
		// the -scenario burial graveyard
		sw, sh := a.size()
		a.renderer.centerOn(16, 17, sw, sh)
	case "weather":
		// open ground with trees and grass north-west of the start
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		sw, sh := a.size()
		a.renderer.centerOn(cx-8, cy-6, sw, sh)
	case "grass":
		// the first colonist walks across open grass; the camera follows
		if pawns := a.renderer.snapshot(); len(pawns) > 0 {
			p := pawns[0]
			a.capture.follow = p.ID
			a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: p.X - 10, Y: p.Y - 8})
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
	case "turret":
		// -scenario turret: the house and both turrets east of it
		cx, cy := float64(a.renderer.game.Width())/2, float64(a.renderer.game.Height())/2
		sw, sh := a.size()
		a.renderer.centerOn(cx+2, cy+1, sw, sh)
	case "trade":
		// -scenario trade: a colonist goes to the caravan's trader; the
		// trade dialog opens once they talk
		a.captureTrade()
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
			a.openPaneTab("gear")
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
	case "home":
		// the Home tool in hand, overlay on, a rectangle being dragged
		a.openArchitect()
		a.selectArchCategory("zones")
		a.selectArchTool("home")
		a.renderer.homeOverlay = true
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		a.archDrag = true
		a.archX0, a.archY0, a.archX1, a.archY1 = cx-3, cy-2, cx+3, cy+2
		x, y := at(cx+3, cy+2)
		a.cursor = &[2]int{x, y}
	case "carpet":
		// the Carpet tool with its four material buttons, cloth picked
		a.openArchitect()
		a.selectArchCategory("furniture")
		a.selectArchTool("carpet")
		a.archStuff = "cloth carpet"
		cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
		x, y := at(cx+1, cy+1)
		a.cursor = &[2]int{x, y}
	case "search", "rooms", "multiselect", "drawstyle", "hideui":
		a.captureInput(ui, at)
	case "factions-colony", "bio", "work-incapable", "arrival", "options-story", "letter-intro",
		"arch-hospital", "arch-cabinet", "arch-locked", "chips":
		a.captureColonyUI(ui) // capture_colony.go
	case "architect":
		a.openArchitect()
		a.selectArchCategory("structure")
		a.selectArchTool("wall")
		a.archStuff = defaultStoneStuff(a.renderer.world())
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
		a.renderer.cam.OffsetX = float64(shot.FromX+shot.ToX+1)/2*tile - float64(a.sw)/2
		a.renderer.cam.OffsetY = float64(shot.FromY+shot.ToY+1)/2*tile - float64(a.sh)/2
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
	case "social", "health", "prisoner", "history", "world", "assign":
		a.captureSocialUI(ui) // capture_social.go: the -scenario social tabs and windows
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

// captureInput sets up the input-system shots (UI-016): the search box
// with results, the room stats overlay with a room card, three colonists
// selected with the group panel, an empty-oval wall preview and F11.
func (a *App) captureInput(ui string, at func(tx, ty int) (int, int)) {
	cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
	switch ui {
	case "search":
		a.openSearch()
		a.search.text = "wo"
		a.search.idx = 1
	case "rooms":
		a.rooms = true
		x, y := at(cx-3, cy+1) // inside the walled room of captureOrders
		a.cursor = &[2]int{x, y}
	case "multiselect":
		var ids []entity.EntityID
		for _, p := range a.renderer.snapshot() {
			ids = append(ids, p.ID)
		}
		a.setGroup(selPawn, ids)
		x, y := at(cx+6, cy-1)
		a.cursor = &[2]int{x, y}
	case "drawstyle":
		a.openArchitect()
		a.selectArchCategory("structure")
		a.selectArchTool("wall")
		a.archStyle = styleOvalEdge
		a.archDrag = true
		a.archX0, a.archY0, a.archX1, a.archY1 = cx+5, cy-4, cx+9, cy+1
		x, y := at(cx+9, cy+1)
		a.cursor = &[2]int{x, y}
	case "hideui":
		a.hideUI = true
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

// panAt is the camera point at progress t (0..1) along waypoints, eased
// in and out on each leg.
func panAt(wp [][2]float64, t float64) (float64, float64) {
	if t <= 0 {
		return wp[0][0], wp[0][1]
	}
	if t >= 1 {
		last := wp[len(wp)-1]
		return last[0], last[1]
	}
	legs := float64(len(wp) - 1)
	i := int(t * legs)
	k := t*legs - float64(i)
	k = k * k * (3 - 2*k)
	a, b := wp[i], wp[i+1]
	return a[0] + (b[0]-a[0])*k, a[1] + (b[1]-a[1])*k
}

// captureInspect selects, for a panel capture, the thing nearest the map
// centre: a tree ("plant"), a ground item ("item"), a colonist
// ("pawnactions", Gear tab) or a movable building ("move" in move mode,
// "copy" after Copy).
func (a *App) captureInspect(ui string) {
	w := a.renderer.world()
	cx, cy := a.renderer.game.Width()/2, a.renderer.game.Height()/2
	near := func(x, y int) int { return abs(x-cx) + abs(y-cy) }
	best, bx, by := 1<<30, cx, cy
	pick := func(x, y int) {
		if d := near(x, y); d < best {
			best, bx, by = d, x, y
		}
	}
	switch ui {
	case "plant":
		for _, p := range w.Plants {
			if p.Species == "tree" {
				pick(p.X, p.Y)
			}
		}
		a.selectPlantAt(bx, by)
	case "item":
		for _, it := range w.Items {
			if !it.Carried {
				pick(it.X, it.Y)
			}
		}
		a.selectItemAt(bx, by)
	case "pawnactions":
		if pawns := a.renderer.snapshot(); len(pawns) > 0 {
			a.renderer.Select(pawns[0].ID)
			bx, by = pawns[0].X, pawns[0].Y
			a.openPaneTab("gear")
		}
	case "move", "copy":
		for _, b := range w.Buildings {
			if movableKind(b.Kind, b.Floor) {
				pick(b.X, b.Y)
			}
		}
		a.selectBuildingAt(bx, by)
		if ui == "move" {
			a.applyBuildingClick("bldg.move")
			a.bldgMove.fresh = false
		} else {
			a.applyBuildingClick("bldg.copy")
		}
	}
	sw, sh := a.size()
	a.renderer.centerOn(float64(bx)+2, float64(by), sw, sh)
	if ui == "move" { // hover the ghost three tiles east of the original
		sx, sy := a.renderer.worldToScreenF(float64(bx)+3.5, float64(by)+0.5)
		a.cursor = &[2]int{int(sx), int(sy)}
	}
}

// captureZones lays out two touching stockpiles, a grow zone and a grow
// drag ghost running over rough stone (red where it cannot be sown).
// captureZone selects the colony's first growing zone, its pane and
// commands showing (PLAY-020); with shrink, its Shrink tool is held over
// the zone's west half mid-drag.
func (a *App) captureZone(shrink bool, at func(tx, ty int) (int, int)) {
	for _, z := range a.renderer.game.World().Zones {
		if z.Kind != game.ZoneGrow {
			continue
		}
		a.selectCand(selCand{kind: selZone, id: entity.EntityID(z.ID)})
		sw, sh := a.size()
		a.renderer.centerOn(float64(z.MinX+z.MaxX)/2, float64(z.MinY+z.MaxY)/2, sw, sh)
		if shrink {
			a.startZoneTool(zoneToolShrink)
			st := a.zoneSt()
			st.fresh, st.drag = false, true
			st.x0, st.y0, st.x1, st.y1 = z.MinX, z.MinY, (z.MinX+z.MaxX)/2, z.MaxY
			x, y := at(st.x1, st.y1)
			a.cursor = &[2]int{x, y}
		}
		return
	}
}

func (a *App) captureZones(at func(tx, ty int) (int, int)) {
	g := a.renderer.game
	cx, cy := g.Width()/2, g.Height()/2
	free := func(kind string, x0, y0, w, h int) bool {
		for y := y0; y < y0+h; y++ {
			for x := x0; x < x0+w; x++ {
				if !g.CanZoneAt(kind, x, y) {
					return false
				}
			}
		}
		return true
	}
	// the nearest open 8x6 block to the centre for the stockpiles
	sx, sy, best := cx, cy, 1<<30
	for y := 2; y < g.Height()-8; y++ {
		for x := 2; x < g.Width()-12; x++ {
			if d := abs(x-cx) + abs(y-cy); d < best && free(game.ZoneStockpile, x, y, 8, 6) {
				sx, sy, best = x, y, d
			}
		}
	}
	a.renderer.enqueue(command.Command{Kind: command.DesignateStockpile, X: sx, Y: sy, X2: sx + 3, Y2: sy + 1})
	a.renderer.enqueue(command.Command{Kind: command.DesignateStockpile, X: sx + 2, Y: sy + 2, X2: sx + 6, Y2: sy + 4})
	id, _ := game.CropID("potato")
	a.renderer.enqueue(command.Command{Kind: command.DesignateGrowZone, X: sx + 5, Y: sy, X2: sx + 7, Y2: sy + 1, Build: id})
	// the second stockpile's centre sits on a built wood floor
	if id, ok := kindID("floor"); ok {
		for y := sy + 2; y <= sy+3; y++ {
			for x := sx + 2; x <= sx+4; x++ {
				a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: id, X: x, Y: y})
			}
		}
	}
	g.Step(1)
	for i := 0; i < 30; i++ { // the colonists lay it
		g.Step(60)
		a.renderer.syncPawns()
	}
	// the drag: start at rough stone (walkable, not sowable) near the centre
	gx, gy, gbest := sx, sy+7, 1<<30
	for y := 2; y < g.Height()-4; y++ {
		for x := 2; x < g.Width()-6; x++ {
			if d := abs(x-cx) + abs(y-cy); d < gbest && g.CanZoneAt(game.ZoneStockpile, x, y) && !g.CanZoneAt(game.ZoneGrow, x, y) {
				gx, gy, gbest = x, y, d
			}
		}
	}
	a.openArchitect()
	a.selectArchCategory("zones")
	a.selectArchTool("grow")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = gx-3, gy-1, gx+3, gy+2
	x, y := at(gx+3, gy+2)
	a.cursor = &[2]int{x, y}
	sw, sh := a.size()
	a.renderer.centerOn(float64(gx+sx)/2+1, float64(gy+sy)/2+2, sw, sh)
}

// captureBiomeInspect selects what the wildlife captures show: the first
// tame animal (inspector) or the first feeding trough (trough), and
// centres the camera on it.
func (a *App) captureBiomeInspect(ui string) {
	w := a.renderer.world()
	sw, sh := a.size()
	if ui == "trough" {
		for _, b := range w.Buildings {
			if isTrough(b) {
				a.selectBuildingAt(b.X, b.Y)
				a.renderer.centerOn(float64(b.X)+0.5, float64(b.Y)+0.5, sw, sh)
				return
			}
		}
		return
	}
	for _, an := range w.Animals {
		if an.Tame {
			a.selectCand(selCand{selAnimal, an.ID})
			a.renderer.centerOn(float64(an.X)+0.5, float64(an.Y)+0.5, sw, sh)
			return
		}
	}
}

// captureTrade orders the first colonist to trade with a visitor that
// offers "Trade with" and centres the camera on that trader; the clip
// shows the walk over and the dialog opening (syncDealings).
func (a *App) captureTrade() {
	var colonist entity.EntityID
	var visitors []game.PawnView
	for _, p := range a.renderer.snapshot() {
		switch {
		case p.Visitor:
			visitors = append(visitors, p)
		case colonist == 0 && !p.Hostile && !p.Prisoner && p.Status == "alive":
			colonist = p.ID
		}
	}
	if colonist == 0 {
		return
	}
	for _, v := range visitors {
		for _, o := range a.renderer.game.OrdersAt(colonist, v.X, v.Y) {
			if strings.HasPrefix(o.Label, "Trade with") && !o.Disabled {
				a.renderer.enqueue(o.Cmd)
				sw, sh := a.size()
				a.renderer.centerOn(float64(v.X), float64(v.Y), sw, sh)
				return
			}
		}
	}
}
