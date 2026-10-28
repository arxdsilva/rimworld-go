package ebiten

// The power overlay's App side: the forcing tool, the toggle's card, the
// legend and the placement ghost's links (the overlay itself is
// render/overlay, power_overlay.go).

import (
	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/platform/input"
)

// syncPowerForce keeps the power overlay forced on while a power tool is
// the selected tool of an open Architect (beside syncRoofForce).
func (a *App) syncPowerForce() {
	if a.renderer != nil {
		a.renderer.overlays().Power.Forced = powerForced(a.archOpen, a.archTool)
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
	g := &r.overlays().Wiring.Nets
	is := g.Transmitters(&w)
	g.SyncNets(w.PowerNets)
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
			col := g.NetColour(g.NetAt(to))
			r.drawPowerLink(screen, from, to, col, tile)
			g.Emit("ghost link", c.x, c.y, col)
			continue
		}
		x0, y0, x1 := c.x, c.y, c.x
		for _, f := range foot {
			x0, y0, x1 = min(x0, f[0]), min(y0, f[1]), max(x1, f[0])
		}
		sx, sy := a.shell.WorldToScreenF(float64(x0+x1+1)/2, float64(y0))
		worldTag(screen, powerGhostLabel, float64(sx), float64(sy), withColor(tsSmall, uiWarning))
		g.Emit("not connected", c.x, c.y, tokenNRGBA(uiWarning))
	}
}

// powerButtonID is the power overlay's global toggle.
const powerButtonID = "hud.power"

const (
	powerLegendTxt  = "Power: a colour per grid · lines link devices · no-power sign unpowered · red frame not connected · bar battery charge"
	powerGhostLabel = "Not connected"
)

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
