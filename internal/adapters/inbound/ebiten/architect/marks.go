package architect

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// Marks is what a held tool shows on the map besides its ghost: the
// overlays it forces on (roof, home, plan, power, snow clearing; the
// overlays themselves are render/overlay), the power ghost's links, the
// power legend and the pen outlines (pen_draw.go).
type Marks struct{ *Tool }

// ForcesRoof reports a roof tool is the selected tool of an open
// Architect: the Roof overlay is forced on.
func (m *Marks) ForcesRoof() bool {
	return m.Open && (m.ToolID == "roof" || m.ToolID == "noroof")
}

// ForcesHome reports a Home tool is in hand: the Home overlay is forced on.
func (m *Marks) ForcesHome() bool {
	return m.Open && (m.ToolID == "home" || m.ToolID == "unhome")
}

// ForcesPlan reports a Plan tool is in hand: the Plan overlay is forced on.
func (m *Marks) ForcesPlan() bool {
	return m.Open && (m.ToolID == "plan" || m.ToolID == "unplan")
}

// ForcesPower reports a power tool is the selected tool of an open
// Architect: the power overlay is forced on.
func (m *Marks) ForcesPower() bool { return powerForced(m.Open, m.ToolID) }

// ForcesSnowClear reports a snow clearing tool is held: its overlay shows.
func (m *Marks) ForcesSnowClear() bool { return m.Open && SnowClearToolID(m.ToolID) }

// snowClearTools are the two snow clearing Architect tools.
var snowClearTools = [2]string{"snow_clear", "unsnow_clear"}

// SnowClearToolID reports id is one of the two snow clearing tools.
func SnowClearToolID(id string) bool { return id == snowClearTools[0] || id == snowClearTools[1] }

// ToggleHomeAuto asks the sim to flip automatic Home expansion (PLAY-015:
// the house-with-arrows toggle among the global controls).
func (t *Tool) ToggleHomeAuto() {
	if t.sim() == nil {
		return
	}
	t.sh.Enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: !t.sh.World().HomeAuto})
}

// HomeAutoButtonID is the Home auto-expand toggle (the global controls
// place it).
const HomeAutoButtonID = "hud.homeauto"

// PowerToggleTip is the Power grid toggle's hover card.
func (m *Marks) PowerToggleTip() string {
	hint := ""
	if l := m.sh.Env().Keys.Label(input.PowerOverlay); l != "" {
		hint = " [" + l + "]"
	}
	return "Power grid" + hint +
		"\nEach grid's conduits in its own colour, the links, unpowered and unconnected devices and battery charge. Shown by itself while you place a power building."
}

// PowerLegend is the key shown in the global stack while the overlay is on.
func (m *Marks) PowerLegend() string {
	if m.sim() == nil || !m.sh.Overlays().Power.Shown() {
		return ""
	}
	return powerLegendTxt
}

// drawPowerGhostLinks draws, for each placeable cell of a powered
// building's ghost, the link it would make (in the target net's colour)
// or a "Not connected" tag over its footprint.
func (m *Marks) drawPowerGhostLinks(screen *ebiten.Image, d ToolDef, cells []GhostCell) {
	if m.sim() == nil || !overlay.PowerDeviceKinds[d.Build] {
		return
	}
	bv, ok := Buildable(d.Build)
	if !ok {
		return
	}
	w := m.sh.World()
	g := &m.sh.Overlays().Wiring.Nets
	is := g.Transmitters(&w)
	g.SyncNets(w.PowerNets)
	mw, mh := m.sh.MapSize()
	tile := m.sh.TilePx()
	for _, c := range cells {
		if !c.OK || c.Same {
			continue
		}
		f := m.Facing
		if c.Facing != "" {
			f = FacingIndex(c.Facing) // move_mode.go
		}
		foot, _, _ := construction.Footprint(construction.Kind(bv.ID), construction.Facing(f), c.X, c.Y)
		if from, to, linked := overlay.GhostLink(foot, mw, mh, is); linked {
			col := g.NetColour(g.NetAt(to))
			m.sh.DrawPowerLink(screen, from, to, col, tile)
			g.Emit("ghost link", c.X, c.Y, col)
			continue
		}
		x0, y0, x1 := c.X, c.Y, c.X
		for _, f := range foot {
			x0, y0, x1 = min(x0, f[0]), min(y0, f[1]), max(x1, f[0])
		}
		sx, sy := m.sh.WorldToScreenF(float64(x0+x1+1)/2, float64(y0))
		worldTag(screen, PowerGhostLabel, float64(sx), float64(sy), gfx.WithColor(gfx.Small, uikit.Warning))
		g.Emit("not connected", c.X, c.Y, overlay.TokenNRGBA(uikit.Warning))
	}
}

// PowerButtonID is the power overlay's global toggle.
const PowerButtonID = "hud.power"

const (
	powerLegendTxt = "Power: a colour per grid · lines link devices · no-power sign unpowered · red frame not connected · bar battery charge"
	// PowerGhostLabel tags a powered ghost no grid would reach.
	PowerGhostLabel = "Not connected"
)

// powerToolKind reports kind belongs to the grid: a conduit, a switch, or
// a building that makes, draws or stores power.
func powerToolKind(kind string) bool {
	return overlay.IsConduitKind(kind) || kind == overlay.PowerSwitchKind || overlay.PowerDeviceKinds[kind]
}

// powerForced is whether the overlay must be forced on: a power tool is
// the selected tool of an open Architect.
func powerForced(archOpen bool, toolID string) bool {
	d, ok := ToolByID(toolID)
	return archOpen && ok && powerToolKind(d.Build)
}

// worldTag draws s centred on the physical point (cx, bottom) with its
// bottom edge there, on a small panel; the text keeps the UI scale.
func worldTag(dst *ebiten.Image, s string, cx, bottom float64, st gfx.TextStyle) {
	k := 1 / gfx.UIScale()
	w, _ := gfx.Measure(s, st)
	h := float64(gfx.LineStep(st))
	x, y := cx*k-w/2, bottom*k-h-4
	gfx.FillRect(dst, float32(x-4), float32(y-1), float32(w+8), float32(h+2), uikit.Panel, false)
	gfx.Text(dst, s, x, y, st)
}
