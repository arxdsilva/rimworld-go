package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/platform/input"
)

// PLAY-021 power overlay (POWERUI owns this file). Presentation only: it
// reads the BuildingView power fields and WorldView.PowerNets.
//
//   - Each net's conduits and switches are tinted in the net's own colour
//     (powerPalette, handed out in turn and kept while the net lives;
//     grey on no net), with RENDER's conduit markings over them, so
//     hidden conduits show here and only here.
//   - Every linked device has a line from its footprint to the
//     transmitter it joined; a device on no grid gets a red frame.
//   - Producers wear a disc, consumers a ring, batteries a square and a
//     charge bar; an unpowered consumer the no-power sign, steady, on the
//     spot of the world's blinking badge (power_draw.go).
//   - It is the "Power grid" global toggle (hud_global.go, Shift+P) and
//     shows by itself while a power tool is in hand (syncPowerForce).
//   - The placement ghost of a powered building links to the nearest
//     transmitter in range by the simulation's rule, or says "Not
//     connected" (drawPowerGhostLinks).

// powerButtonID is the power overlay's global toggle.
const powerButtonID = "hud.power"

const (
	powerLegendTxt  = "Power: a colour per grid · lines link devices · no-power sign unpowered · red frame not connected · bar battery charge"
	powerGhostLabel = "Not connected"
	// powerDetailPx: below this tile size the overlay keeps only the
	// grid tint, the links and the frames (marks would be specks).
	powerDetailPx = 16
)

// powerPalette are the grid colours (ui.power_net*), handed out in turn.
var powerPalette = [...]color.RGBA{uiPowerNet1, uiPowerNet2, uiPowerNet3, uiPowerNet4, uiPowerNet5, uiPowerNet6}

// tokenNRGBA is an opaque UI token as the NRGBA the world draws take (the
// same bytes: opaque, so premultiplying changes nothing).
func tokenNRGBA(c color.RGBA) color.NRGBA { return color.NRGBA(c) }

// powerOverlay is the Renderer's power overlay state (Renderer.pov): the
// player's toggle, the flag forcing it on while a power tool is held, the
// palette slot of every live net, and the transmitter cells the
// placement ghost links to. trace, when set (tests), sees every part.
type powerOverlay struct {
	on, forced bool
	slots      map[int]int // live net id -> powerPalette index
	ids        []int       // the net ids slots was last synced to
	tr         powerTransCache
	trace      func(part string, x, y int, c color.NRGBA)
}

// powerTransCache is the transmitter cells by net, rebuilt only when the
// structure or the power state moves.
type powerTransCache struct {
	ok                  bool
	structVer, powerVer uint64
	n                   int
	cells               map[[2]int]int // transmitter cell -> its net (0: none)
}

func (o *powerOverlay) shown() bool { return o.on || o.forced }

func (o *powerOverlay) emit(part string, x, y int, c color.NRGBA) {
	if o.trace != nil {
		o.trace(part, x, y, c)
	}
}

// syncNets keeps every live net's palette slot and gives each new net the
// least used slot (the lowest on a tie), in ascending id order, so a new
// net never repaints an old one and live nets share a colour only once
// the palette runs out. A no-op while the net ids stay the same.
func (o *powerOverlay) syncNets(nets []game.PowerNetView) {
	if len(nets) == len(o.ids) {
		same := true
		for i := range nets {
			if nets[i].ID != o.ids[i] {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	live := make(map[int]bool, len(nets))
	o.ids = o.ids[:0]
	for _, n := range nets {
		o.ids = append(o.ids, n.ID)
		live[n.ID] = true
	}
	if o.slots == nil {
		o.slots = map[int]int{}
	}
	for id := range o.slots {
		if !live[id] {
			delete(o.slots, id)
		}
	}
	var used [len(powerPalette)]int
	for _, s := range o.slots {
		used[s]++
	}
	for _, n := range nets {
		if _, ok := o.slots[n.ID]; ok || n.ID == 0 {
			continue
		}
		best := 0
		for k := range used {
			if used[k] < used[best] {
				best = k
			}
		}
		o.slots[n.ID] = best
		used[best]++
	}
}

// netColour is net's colour: its palette slot, grey for no net.
func (o *powerOverlay) netColour(net int) color.NRGBA {
	if net == 0 {
		return tokenNRGBA(uiPowerNone)
	}
	if s, ok := o.slots[net]; ok {
		return tokenNRGBA(powerPalette[s])
	}
	return tokenNRGBA(powerPalette[uint(net)%uint(len(powerPalette))]) // not synced yet
}

// transmitters reports the cells a device can link to: built conduits
// and switches that are on, less every cell under a switch that is off
// (nothing conducts there; power.Builder.Build).
func (o *powerOverlay) transmitters(w *game.WorldView) func(x, y int) bool {
	c := &o.tr
	if !c.ok || c.structVer != w.StructVer || c.powerVer != w.PowerVer || c.n != len(w.Buildings) {
		c.ok, c.structVer, c.powerVer, c.n = true, w.StructVer, w.PowerVer, len(w.Buildings)
		if c.cells == nil {
			c.cells = map[[2]int]int{}
		}
		clear(c.cells)
		var cut [][2]int
		for i := range w.Buildings {
			b := &w.Buildings[i]
			open := b.PowerRole == "switch" && !b.PowerOn
			if b.PowerRole != "transmitter" && b.PowerRole != "switch" {
				continue
			}
			for y := b.Y; y < b.Y+max(b.H, 1); y++ {
				for x := b.X; x < b.X+max(b.W, 1); x++ {
					if open {
						cut = append(cut, [2]int{x, y})
					} else {
						c.cells[[2]int{x, y}] = b.PowerNet
					}
				}
			}
		}
		for _, k := range cut {
			delete(c.cells, k)
		}
	}
	return func(x, y int) bool {
		_, ok := c.cells[[2]int{x, y}]
		return ok
	}
}

// ghostLink is the link a device with footprint cells would make on a
// w x h map, by the simulation's rule (power.Builder.link): the nearest
// transmitter cell within power.ConnectRange of the footprint (Chebyshev;
// ties by y, then x), and the footprint cell the line leaves from (at
// that distance, the nearest in a straight line, then by y, then x).
func ghostLink(cells [][2]int, w, h int, is func(x, y int) bool) (from, to [2]int, ok bool) {
	in := func(c [2]int) bool { return c[0] >= 0 && c[1] >= 0 && c[0] < w && c[1] < h }
	x0, y0, x1, y1 := w, h, -1, -1
	for _, c := range cells {
		if in(c) {
			x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
		}
	}
	if x1 < 0 {
		return from, to, false
	}
	rng := power.ConnectRange
	x0, y0 = max(x0-rng, 0), max(y0-rng, 0)
	x1, y1 = min(x1+rng, w-1), min(y1+rng, h-1)
	best := rng + 1
scan:
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !is(x, y) {
				continue
			}
			d := math.MaxInt
			for _, c := range cells {
				if in(c) {
					d = min(d, chebDist(c[0]-x, c[1]-y))
				}
			}
			if d < best {
				best, to, ok = d, [2]int{x, y}, true
				if d == 0 {
					break scan // nothing nearer, nothing earlier
				}
			}
		}
	}
	if !ok {
		return from, to, false
	}
	bestE := -1
	for _, c := range cells {
		if !in(c) || chebDist(c[0]-to[0], c[1]-to[1]) != best {
			continue
		}
		dx, dy := c[0]-to[0], c[1]-to[1]
		e := dx*dx + dy*dy
		if bestE < 0 || e < bestE || (e == bestE && (c[1] < from[1] || (c[1] == from[1] && c[0] < from[0]))) {
			bestE, from = e, c
		}
	}
	return from, to, true
}

func chebDist(dx, dy int) int { return max(abs(dx), abs(dy)) }

// ---------------------------------------------------------------- toggle

// powerShown reports the power overlay is showing (toggle or tool held).
func (r *Renderer) powerShown() bool { return r.pov.shown() }

// togglePowerOverlay is the Shift+P key and HUD button action.
func (r *Renderer) togglePowerOverlay() { r.pov.on = !r.pov.on }

// togglePowerOverlayIfAny tolerates a nil renderer (menus).
func (r *Renderer) togglePowerOverlayIfAny() {
	if r != nil {
		r.togglePowerOverlay()
	}
}

// powerToolKind reports kind belongs to the grid: a conduit, a switch, or
// a building that makes, draws or stores power.
func powerToolKind(kind string) bool {
	return isConduitKind(kind) || kind == powerSwitchKind || powerDeviceKinds[kind]
}

// powerForced is whether the overlay must be forced on: a power tool is
// the selected tool of an open Architect.
func powerForced(archOpen bool, toolID string) bool {
	t, ok := toolByID(toolID)
	return archOpen && ok && powerToolKind(t.build)
}

// syncPowerForce keeps the power overlay forced on while a power tool is
// the selected tool of an open Architect (beside syncRoofForce).
func (a *App) syncPowerForce() {
	if a.renderer != nil {
		a.renderer.pov.forced = powerForced(a.archOpen, a.archTool)
	}
}

// powerToggleTip is the Power grid toggle's hover card.
func (a *App) powerToggleTip() string {
	return "Power grid" + a.keyHint(input.PowerOverlay) +
		"\nEach grid's conduits in its own colour, the links, unpowered and unconnected devices and battery charge. Shown by itself while you place a power building."
}

// powerLegend is the key shown in the global stack while the overlay is on.
func (a *App) powerLegend() string {
	if a.renderer == nil || !a.shell.PowerShown() {
		return ""
	}
	return powerLegendTxt
}

// ---------------------------------------------------------------- overlay

// drawPowerOverlay draws the power overlay over the world pass when it is
// shown (renderer.go Draw calls it after the roof overlay): the grid
// tint, the conduit markings, then each device's link, frame, mark,
// no-power sign and charge. Only the chunks in view are visited.
func (r *Renderer) drawPowerOverlay(screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	o := &r.pov
	if !o.shown() || w == nil {
		return
	}
	o.syncNets(w.PowerNets)
	r.layer("power overlay")
	for _, cv := range r.lod.views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if (b.PowerRole != "transmitter" && b.PowerRole != "switch") || !r.lod.inView(b.X, b.Y, 1, 1) {
				continue
			}
			c := o.netColour(b.PowerNet)
			if b.PowerRole == "switch" && !b.PowerOn {
				c = tokenNRGBA(uiPowerNone) // open: it carries nothing
			}
			sx, sy := r.worldToScreen(b.X, b.Y)
			fill := c
			fill.A = 0x58
			r.solidRect(screen, sx, sy, float32(tile)*float32(max(b.W, 1)), float32(tile)*float32(max(b.H, 1)), fill)
			o.emit("grid", b.X, b.Y, c)
		}
	}
	if r.art != nil {
		r.drawPowerMarkings(screen, w, tile, sw, sh, o.netColour, false) // power_draw.go
	}
	detail := tile >= powerDetailPx
	for _, cv := range r.lod.views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			switch b.PowerRole {
			case "producer", "consumer", "storage":
			default:
				continue
			}
			if r.lod.inView(b.X, b.Y, 3, 3) {
				r.drawPowerDevice(screen, b, tile, detail)
			}
		}
	}
}

// drawPowerDevice draws device b's overlay parts: its link line (or a red
// frame on no grid) and, with detail, its mark, the no-power sign and a
// battery's charge.
func (r *Renderer) drawPowerDevice(dst *ebiten.Image, b *game.BuildingView, tile float64, detail bool) {
	o := &r.pov
	c := o.netColour(b.PowerNet)
	if b.HasLink {
		r.drawPowerLink(dst, [2]int{b.LinkFromX, b.LinkFromY}, [2]int{b.LinkX, b.LinkY}, c, tile)
		o.emit("link", b.LinkFromX, b.LinkFromY, c)
	} else {
		r.drawPowerFrame(dst, b, tile)
		o.emit("disconnected", b.X, b.Y, tokenNRGBA(uiDanger))
	}
	if !detail {
		return
	}
	sx, sy := r.worldToScreenF(float64(b.X), float64(b.Y))
	mx, my, rad := float64(sx)+tile*0.2, float64(sy)+tile*0.2, tile*0.11
	switch b.PowerRole {
	case "producer":
		powerDot(dst, mx, my, rad, c, b.PowerOut > 0, tile) // hollow while it makes nothing
	case "consumer":
		powerDot(dst, mx, my, rad, c, false, tile)
	case "storage":
		r.solidRect(dst, float32(mx-rad-1), float32(my-rad-1), float32(2*rad+2), float32(2*rad+2), powerShade)
		r.solidRect(dst, float32(mx-rad), float32(my-rad), float32(2*rad), float32(2*rad), c)
		r.drawPowerCharge(dst, b, tile)
		o.emit("battery", b.X, b.Y, batteryBarNRGBA(b))
	}
	o.emit(b.PowerRole, b.X, b.Y, c)
	if b.PowerRole == "consumer" && b.NoPowerReason != "" {
		r.drawPowerUnpowered(dst, b, tile)
		o.emit("unpowered", b.X, b.Y, tokenNRGBA(uiDanger))
	}
}

// powerShade is the dark rim under the overlay's marks and bars.
var powerShade = color.NRGBA{R: 8, G: 12, B: 20, A: 200}

// batteryBarNRGBA is the charge bar's colour for battery b.
func batteryBarNRGBA(b *game.BuildingView) color.NRGBA { return batteryBarColour(batteryFill(b)) }

// drawPowerCharge draws battery b's charge as a bar along the bottom of
// its footprint, empty part dark.
func (r *Renderer) drawPowerCharge(dst *ebiten.Image, b *game.BuildingView, tile float64) {
	sx, sy := r.worldToScreenF(float64(b.X), float64(b.Y))
	w := float32(tile*float64(max(b.W, 1)) - tile*0.24)
	h := float32(max(3, tile*0.14))
	x, y := sx+float32(tile*0.12), sy+float32(tile*float64(max(b.H, 1))-tile*0.3)
	r.solidRect(dst, x-1, y-1, w+2, h+2, powerShade)
	r.solidRect(dst, x, y, w*float32(batteryFill(b)), h, batteryBarNRGBA(b))
}

// drawPowerUnpowered draws the no-power sign over consumer b, steady, on
// the spot and at the size of the world's blinking badge
// (drawNoPowerBadges), so the overlay marks every unpowered consumer,
// switched-off ones included, once.
func (r *Renderer) drawPowerUnpowered(dst *ebiten.Image, b *game.BuildingView, tile float64) {
	sx, sy := r.worldToScreenF(float64(b.X)+float64(max(b.W, 1))/2, float64(b.Y)+float64(max(b.H, 1))/2-badgeLift)
	size := tile * badgeTiles
	if r.art != nil {
		if img := r.art.image(noPowerIcon, color.NRGBA{}); img != nil {
			worldIconCS(dst, img, float64(sx)-size/2, float64(sy)-size/2, size/uiScale(), noCS)
			return
		}
	}
	powerDot(dst, float64(sx), float64(sy), size/2, tokenNRGBA(uiDanger), true, tile)
}

// powerDot is a mark at physical (x, y): a filled disc, or a ring, over a
// dark rim.
func powerDot(dst *ebiten.Image, x, y, rad float64, c color.NRGBA, fill bool, tile float64) {
	fx, fy, fr := float32(x), float32(y), float32(rad)
	vector.FillCircle(dst, fx, fy, fr+1.5, powerShade, true)
	if fill {
		vector.FillCircle(dst, fx, fy, fr, c, true)
		return
	}
	vector.StrokeCircle(dst, fx, fy, fr-0.5, float32(max(1.5, tile*0.05)), c, true)
}

// drawPowerLink draws a link line from the centre of cell from to the
// centre of cell to, a dot on the transmitter end, over a dark rim.
func (r *Renderer) drawPowerLink(dst *ebiten.Image, from, to [2]int, c color.NRGBA, tile float64) {
	x0, y0 := r.worldToScreenF(float64(from[0])+0.5, float64(from[1])+0.5)
	x1, y1 := r.worldToScreenF(float64(to[0])+0.5, float64(to[1])+0.5)
	wd := float32(max(1.5, tile*0.05))
	vector.StrokeLine(dst, x0, y0, x1, y1, wd+2, powerShade, true)
	vector.StrokeLine(dst, x0, y0, x1, y1, wd, c, true)
	vector.FillCircle(dst, x1, y1, wd*1.6, c, true)
}

// drawPowerFrame frames device b's footprint in the danger colour: it is
// on no grid.
func (r *Renderer) drawPowerFrame(dst *ebiten.Image, b *game.BuildingView, tile float64) {
	sx, sy := r.worldToScreenF(float64(b.X), float64(b.Y))
	wd := float32(max(2, tile*0.06))
	w, h := float32(tile*float64(max(b.W, 1))), float32(tile*float64(max(b.H, 1)))
	vector.StrokeRect(dst, sx+wd/2, sy+wd/2, w-wd, h-wd, wd, uiDanger, true)
}

// ---------------------------------------------------------------- placement ghost

// drawPowerGhostLinks draws, for each placeable cell of a powered
// building's ghost, the link it would make (in the target net's colour)
// or a "Not connected" tag over its footprint.
func (a *App) drawPowerGhostLinks(screen *ebiten.Image, t archTool, cells []ghostCell) {
	r := a.renderer
	if r == nil || !powerDeviceKinds[t.build] {
		return
	}
	bv, ok := buildable(t.build)
	if !ok {
		return
	}
	w := a.shell.World()
	is := r.pov.transmitters(&w)
	r.pov.syncNets(w.PowerNets)
	mw, mh := a.shell.MapSize()
	tile := a.shell.TilePx()
	for _, c := range cells {
		if !c.ok || c.same {
			continue
		}
		f := a.archFacing
		if c.facing != "" {
			f = facingIndex(c.facing) // move_mode.go
		}
		foot, _, _ := construction.Footprint(construction.Kind(bv.ID), construction.Facing(f), c.x, c.y)
		if from, to, linked := ghostLink(foot, mw, mh, is); linked {
			col := r.pov.netColour(r.pov.tr.cells[to])
			r.drawPowerLink(screen, from, to, col, tile)
			r.pov.emit("ghost link", c.x, c.y, col)
			continue
		}
		x0, y0, x1 := c.x, c.y, c.x
		for _, f := range foot {
			x0, y0, x1 = min(x0, f[0]), min(y0, f[1]), max(x1, f[0])
		}
		sx, sy := a.shell.WorldToScreenF(float64(x0+x1+1)/2, float64(y0))
		worldTag(screen, powerGhostLabel, float64(sx), float64(sy), withColor(tsSmall, uiWarning))
		r.pov.emit("not connected", c.x, c.y, tokenNRGBA(uiWarning))
	}
}

// worldTag draws s centred on the physical point (cx, bottom) with its
// bottom edge there, on a small panel; the text keeps the UI scale.
func worldTag(dst *ebiten.Image, s string, cx, bottom float64, st textStyle) {
	k := 1 / uiScale()
	w, _ := measure(s, st)
	h := float64(lineStep(st))
	x, y := cx*k-w/2, bottom*k-h-4
	uiFillRect(dst, float32(x-4), float32(y-1), float32(w+8), float32(h+2), uiPanel, false)
	drawText(dst, s, x, y, st)
}
