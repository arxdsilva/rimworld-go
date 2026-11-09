package overlay

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
)

// PLAY-021 power overlay (POWERUI owns this file). Presentation only: it
// reads the BuildingView power fields and WorldView.PowerNets.
//
//   - Each net's conduits and switches are tinted in the net's own colour
//     (PowerPalette, handed out in turn and kept while the net lives;
//     grey on no net), with RENDER's conduit markings over them, so
//     hidden conduits show here and only here.
//   - Every linked device has a line in its net's colour from its footprint
//     to the conductor it joined, over the wire the map always shows
//     (power_cords.go); a device on no grid gets a red frame.
//   - Producers wear a disc, consumers a ring, batteries a square and a
//     charge bar; an unpowered consumer the no-power sign, steady, on the
//     spot of the world's blinking badge (power_draw.go).
//   - It is the "Power grid" global toggle (hud_global.go, Shift+P) and
//     shows by itself while a power tool is in hand (syncPowerForce).
//   - The placement ghost of a powered building links to the nearest
//     conductor in range by the simulation's rule, joins by touching, or
//     says "Not connected" (drawPowerGhostLinks).

// powerDetailPx: below this tile size the overlay keeps only the grid
// tint, the links and the frames (marks would be specks).
const powerDetailPx = 16

// PowerPalette are the grid colours (ui.power_net*), handed out in turn.
var PowerPalette = [...]color.RGBA{uikit.PowerNet1, uikit.PowerNet2, uikit.PowerNet3, uikit.PowerNet4, uikit.PowerNet5, uikit.PowerNet6}

// TokenNRGBA is an opaque UI token as the NRGBA the world draws take (the
// same bytes: opaque, so premultiplying changes nothing).
func TokenNRGBA(c color.RGBA) color.NRGBA { return color.NRGBA(c) }

// PowerOverlay is the power overlay's state (Set.Grid; its toggle is
// Set.Power): the palette slot of every live net and the transmitter
// cells the placement ghost links to. Trace, when set (tests), sees
// every part.
type PowerOverlay struct {
	slots map[int]int // live net id -> PowerPalette index
	ids   []int       // the net ids slots was last synced to
	tr    powerTransCache
	Trace func(part string, x, y int, c color.NRGBA)
}

// powerTransCache is the transmitter cells by net, rebuilt only when the
// structure or the power state moves.
type powerTransCache struct {
	ok                  bool
	structVer, powerVer uint64
	n                   int
	gen                 uint64         // counts the rebuilds
	cells               map[[2]int]int // transmitter cell -> its net (0: none)
}

// Emit reports a part to the test hook.
func (o *PowerOverlay) Emit(part string, x, y int, c color.NRGBA) {
	if o.Trace != nil {
		o.Trace(part, x, y, c)
	}
}

// SyncNets keeps every live net's palette slot and gives each new net the
// least used slot (the lowest on a tie), in ascending id order, so a new
// net never repaints an old one and live nets share a colour only once
// the palette runs out. A no-op while the net ids stay the same.
func (o *PowerOverlay) SyncNets(nets []game.PowerNetView) {
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
	var used [len(PowerPalette)]int
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

// NetColour is net's colour: its palette slot, grey for no net.
func (o *PowerOverlay) NetColour(net int) color.NRGBA {
	if net == 0 {
		return TokenNRGBA(uikit.PowerNone)
	}
	if s, ok := o.slots[net]; ok {
		return TokenNRGBA(PowerPalette[s])
	}
	return TokenNRGBA(PowerPalette[uint(net)%uint(len(PowerPalette))]) // not synced yet
}

// NetAt is the net of transmitter cell c as Transmitters last cached it
// (0: none).
func (o *PowerOverlay) NetAt(c [2]int) int { return o.tr.cells[c] }

// Transmitters reports the cells a device can link to: built conduits,
// switches that are on, generators and batteries (a lone one has net 0 and
// still counts), less every cell under a switch that is off (nothing
// conducts there; power.Builder.Build).
func (o *PowerOverlay) Transmitters(w *game.WorldView) func(x, y int) bool {
	o.syncTransmitters(w)
	return func(x, y int) bool { return o.isTransmitter(x, y) }
}

// isTransmitter is Transmitters' answer as last synced.
func (o *PowerOverlay) isTransmitter(x, y int) bool {
	_, ok := o.tr.cells[[2]int{x, y}]
	return ok
}

// syncTransmitters rebuilds the cells when the structure or the power
// state moved.
func (o *PowerOverlay) syncTransmitters(w *game.WorldView) {
	c := &o.tr
	if c.ok && c.structVer == w.StructVer && c.powerVer == w.PowerVer && c.n == len(w.Buildings) {
		return
	}
	c.ok, c.structVer, c.powerVer, c.n = true, w.StructVer, w.PowerVer, len(w.Buildings)
	c.gen++
	if c.cells == nil {
		c.cells = map[[2]int]int{}
	}
	clear(c.cells)
	var cut [][2]int
	for i := range w.Buildings {
		b := &w.Buildings[i]
		switch b.PowerRole {
		case "transmitter", "switch", "producer", "storage":
		default:
			continue
		}
		open := b.PowerRole == "switch" && !b.PowerOn
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

// GhostWire is what planning a kind on footprint cells of an mw x mh map
// would make, by the simulation's rule (power.Builder.Build). A generator or
// battery touching a conductor (the same cell or a 4-neighbour) on a grid
// joins it: no wire, joined. One touching only a lone generator or battery
// (net 0, so nothing is in that one's range) makes a grid with it, and wires
// the pair out if it reaches anything else: joined, wired or not. Anything
// else takes the nearest conductor (GhostLink); neither wired nor joined, it
// is not connected.
func (o *PowerOverlay) GhostWire(w *game.WorldView, kind string, cells [][2]int, mw, mh int) (from, to [2]int, wired, joined bool) {
	o.syncTransmitters(w)
	var lone map[[2]int]bool
	if ghostConducts(kind) {
		for _, c := range cells {
			if c[0] < 0 || c[1] < 0 || c[0] >= mw || c[1] >= mh {
				continue
			}
			x, y := c[0], c[1]
			for _, n := range [5][2]int{c, {x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
				net, ok := o.tr.cells[n]
				if !ok {
					continue
				}
				if net != 0 {
					return from, to, false, true
				}
				if lone == nil {
					lone = map[[2]int]bool{}
				}
				o.markLone(lone, n)
			}
		}
	}
	from, to, wired = GhostLink(cells, mw, mh, func(x, y int) bool {
		return o.isTransmitter(x, y) && !lone[[2]int{x, y}]
	})
	return from, to, wired, lone != nil
}

// ghostConducts: a planned generator or battery. A device that only draws
// power always needs its wire.
func ghostConducts(kind string) bool {
	k, ok := construction.Parse(kind)
	if !ok {
		return false
	}
	r := power.SpecOf(k).Role
	return r == power.Producer || r == power.Storage
}

// markLone adds the cells of the lone device at c: nothing on a grid touches
// one, so its net-0 cells in a row are all it.
func (o *PowerOverlay) markLone(lone map[[2]int]bool, c [2]int) {
	if net, ok := o.tr.cells[c]; !ok || net != 0 || lone[c] {
		return
	}
	lone[c] = true
	for _, n := range [4][2]int{{c[0] - 1, c[1]}, {c[0] + 1, c[1]}, {c[0], c[1] - 1}, {c[0], c[1] + 1}} {
		o.markLone(lone, n)
	}
}

// GhostLink is the link a device with footprint cells would make on a
// w x h map, by the simulation's rule (power.Builder.link): the nearest
// conductor cell (a conduit, a closed switch, a generator or a battery)
// within power.ConnectRange of the footprint (Chebyshev; ties by straight
// line, then y, then x), and the footprint cell the line leaves from (at that distance, the
// nearest in a straight line, then by y, then x). What a generator or
// battery does touching a conductor is GhostWire's.
func GhostLink(cells [][2]int, w, h int, is func(x, y int) bool) (from, to [2]int, ok bool) {
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
	best, bestS := rng+1, 0
scan:
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !is(x, y) {
				continue
			}
			d, s := math.MaxInt, 0
			for _, c := range cells {
				if in(c) {
					dx, dy := c[0]-x, c[1]-y
					if k, e := chebDist(dx, dy), dx*dx+dy*dy; k < d || (k == d && e < s) {
						d, s = k, e
					}
				}
			}
			if d < best || (d == best && s < bestS) {
				best, bestS, to, ok = d, s, [2]int{x, y}, true
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

func chebDist(dx, dy int) int { return max(dx, -dx, dy, -dy) }

// ---------------------------------------------------------------- overlay

// DrawOverlay draws the power overlay over the world pass (Set.Power
// says when it is shown; renderer.go Draw calls it after the roof overlay): the grid
// tint, the conduit markings, then each device's link, frame, mark,
// no-power sign and charge. Only the chunks in view are visited.
func (o *Wiring) DrawOverlay(v *render.View, ch *Chunks, links *Links, screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	g := &o.Nets
	if w == nil {
		return
	}
	g.SyncNets(w.PowerNets)
	v.LayerDrawn("power overlay")
	for _, cv := range ch.Views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if (b.PowerRole != "transmitter" && b.PowerRole != "switch") || !ch.InView(b.X, b.Y, 1, 1) {
				continue
			}
			c := g.NetColour(b.PowerNet)
			if b.PowerRole == "switch" && !b.PowerOn {
				c = TokenNRGBA(uikit.PowerNone) // open: it carries nothing
			}
			sx, sy := v.WorldToScreen(b.X, b.Y)
			fill := c
			fill.A = 0x58
			o.solidRect(screen, sx, sy, float32(tile)*float32(max(b.W, 1)), float32(tile)*float32(max(b.H, 1)), fill)
			g.Emit("grid", b.X, b.Y, c)
		}
	}
	if v.Art != nil {
		o.DrawPowerMarkings(v, ch, links, screen, w, tile, sw, sh, g.NetColour, false) // power_draw.go
	}
	detail := tile >= powerDetailPx
	for _, cv := range ch.Views {
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
			if ch.InView(b.X, b.Y, 3, 3) {
				o.drawPowerDevice(v, screen, b, tile, detail)
			}
		}
	}
}

// drawPowerDevice draws device b's overlay parts: its link line, a red
// frame on no grid (a generator touching a conduit has no line and needs
// no frame) and, with detail, its mark, the no-power sign and a battery's
// charge.
func (o *Wiring) drawPowerDevice(v *render.View, dst *ebiten.Image, b *game.BuildingView, tile float64, detail bool) {
	g := &o.Nets
	c := g.NetColour(b.PowerNet)
	if b.HasLink {
		o.DrawPowerLink(v, dst, [2]int{b.LinkFromX, b.LinkFromY}, [2]int{b.LinkX, b.LinkY}, c, tile)
		g.Emit("link", b.LinkFromX, b.LinkFromY, c)
	}
	if b.PowerNet == 0 {
		o.drawPowerFrame(v, dst, b, tile)
		g.Emit("disconnected", b.X, b.Y, TokenNRGBA(uikit.Danger))
	}
	if !detail {
		return
	}
	sx, sy := v.WorldToScreenF(float64(b.X), float64(b.Y))
	mx, my, rad := float64(sx)+tile*0.2, float64(sy)+tile*0.2, tile*0.11
	switch b.PowerRole {
	case "producer":
		powerDot(dst, mx, my, rad, c, b.PowerOut > 0, tile) // hollow while it makes nothing
	case "consumer":
		powerDot(dst, mx, my, rad, c, false, tile)
	case "storage":
		o.solidRect(dst, float32(mx-rad-1), float32(my-rad-1), float32(2*rad+2), float32(2*rad+2), powerShade)
		o.solidRect(dst, float32(mx-rad), float32(my-rad), float32(2*rad), float32(2*rad), c)
		o.drawPowerCharge(v, dst, b, tile)
		g.Emit("battery", b.X, b.Y, batteryBarNRGBA(b))
	}
	g.Emit(b.PowerRole, b.X, b.Y, c)
	if b.PowerRole == "consumer" && b.NoPowerReason != "" {
		o.drawPowerUnpowered(v, dst, b, tile)
		g.Emit("unpowered", b.X, b.Y, TokenNRGBA(uikit.Danger))
	}
}

// powerShade is the dark rim under the overlay's marks and bars.
var powerShade = color.NRGBA{R: 8, G: 12, B: 20, A: 200}

// batteryBarNRGBA is the charge bar's colour for battery b.
func batteryBarNRGBA(b *game.BuildingView) color.NRGBA { return BatteryBarColour(BatteryFill(b)) }

// drawPowerCharge draws battery b's charge as a bar along the bottom of
// its footprint, empty part dark.
func (o *Wiring) drawPowerCharge(v *render.View, dst *ebiten.Image, b *game.BuildingView, tile float64) {
	sx, sy := v.WorldToScreenF(float64(b.X), float64(b.Y))
	w := float32(tile*float64(max(b.W, 1)) - tile*0.24)
	h := float32(max(3, tile*0.14))
	x, y := sx+float32(tile*0.12), sy+float32(tile*float64(max(b.H, 1))-tile*0.3)
	o.solidRect(dst, x-1, y-1, w+2, h+2, powerShade)
	o.solidRect(dst, x, y, w*float32(BatteryFill(b)), h, batteryBarNRGBA(b))
}

// drawPowerUnpowered draws the no-power sign over consumer b, steady, on
// the spot and at the size of the world's blinking badge
// (DrawNoPowerBadges), so the overlay marks every unpowered consumer,
// switched-off ones included, once.
func (o *Wiring) drawPowerUnpowered(v *render.View, dst *ebiten.Image, b *game.BuildingView, tile float64) {
	sx, sy := v.WorldToScreenF(float64(b.X)+float64(max(b.W, 1))/2, float64(b.Y)+float64(max(b.H, 1))/2-badgeLift)
	size := tile * badgeTiles
	if v.Art != nil {
		if img := v.Art.Image(NoPowerIcon, color.NRGBA{}); img != nil {
			worldIconCS(dst, img, float64(sx)-size/2, float64(sy)-size/2, size/gfx.UIScale(), sprite.NoCS)
			return
		}
	}
	powerDot(dst, float64(sx), float64(sy), size/2, TokenNRGBA(uikit.Danger), true, tile)
}

// powerDot is a mark at physical (x, y): a filled disc, or a ring, over a
// dark rim.
func powerDot(dst *ebiten.Image, x, y, rad float64, c color.NRGBA, fill bool, tile float64) {
	fx, fy, fr := float32(x), float32(y), float32(rad)
	gfx.PxDisc(dst, fx, fy, fr+1.5, powerShade)
	if fill {
		gfx.PxDisc(dst, fx, fy, fr, c)
		return
	}
	gfx.PxRing(dst, fx, fy, fr-0.5, float32(max(1.5, tile*0.05)), c)
}

// DrawPowerLink draws a link line from the centre of cell from to the
// centre of cell to, a dot on the transmitter end, over a dark rim.
func (o *Wiring) DrawPowerLink(v *render.View, dst *ebiten.Image, from, to [2]int, c color.NRGBA, tile float64) {
	x0, y0 := v.WorldToScreenF(float64(from[0])+0.5, float64(from[1])+0.5)
	x1, y1 := v.WorldToScreenF(float64(to[0])+0.5, float64(to[1])+0.5)
	wd := float32(max(1.5, tile*0.05))
	gfx.PxLine(dst, x0, y0, x1, y1, wd+2, powerShade)
	gfx.PxLine(dst, x0, y0, x1, y1, wd, c)
	gfx.PxDisc(dst, x1, y1, wd*1.6, c)
}

// drawPowerFrame frames device b's footprint in the danger colour: it is
// on no grid.
func (o *Wiring) drawPowerFrame(v *render.View, dst *ebiten.Image, b *game.BuildingView, tile float64) {
	sx, sy := v.WorldToScreenF(float64(b.X), float64(b.Y))
	wd := float32(max(2, tile*0.06))
	w, h := float32(tile*float64(max(b.W, 1))), float32(tile*float64(max(b.H, 1)))
	gfx.PxFrame(dst, sx+wd/2, sy+wd/2, w-wd, h-wd, wd, uikit.Danger)
}

// ---------------------------------------------------------------- placement ghost
