// Package minimap is the colony minimap: the baked map layer the simulation
// hands out by version, its box on screen, the viewport frame and clicks
// that pan the camera. It reads the frame's *render.View, never the
// renderer (SOLID_GO.md §7).
package minimap

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// Minimap (NG-1 §2.12; PLAY-018): on maps over 150² a 180×180 overview sits
// in the right column, just above the global controls (the layout's
// Minimap rect), toggled with the Minimap action. It shows the ground (one
// pixel per tile from the read model Simulation.MinimapLayer: terrain
// colours, water, floors, buildings and plants), colonists as light dots,
// hostiles as red dots and the camera's view (the whole window, the map
// fills it) as a rectangle; a click on it centres the camera there.
//
// The box is UI (PLAY-012 B.4): laid out in logical pixels against the
// logical screen and drawn through the canvas, the map image stretched
// into the scaled box. The camera works in physical pixels, so a click
// converts logical box coordinates to tiles and centres the physical
// map viewport on them.

const (
	Side    = hudlayout.MinimapSize // px, the box
	Inset   = 2                     // px of frame inside the box
	MinSide = 150                   // tiles: maps larger than this get a minimap
	// Poll is how many frames apart the layer is asked for (it changes
	// when the ground or the plants' places change).
	Poll = 30
)

// fogClr is the colour of unexplored ground (#0B0B0E), as the fog draws it.
var fogClr = color.RGBA{R: 0x0B, G: 0x0B, B: 0x0E, A: 255}

// Source is what the minimap reads from the simulation.
type Source interface {
	Width() int
	Height() int
	World() game.WorldView
	Snapshot() []game.PawnView
	MinimapLayer(ver uint64) (game.MinimapView, bool)
}

// Map is the renderer's minimap. The zero value is a shown, empty map.
type Map struct {
	Off    bool // hidden by the player
	ver    uint64
	Img    *ebiten.Image
	Pix    []byte
	W, h   int
	fogVer uint64 // FogVer the image was built for
	polled uint64 // present frame of the last poll
	Polls  int    // layer fetches that changed it (tests)
	// GlobalH is the global controls' height (hud_global.go sets it): the
	// minimap sits above them.
	GlobalH float32
}

// New is an empty minimap.
func New() *Map { return &Map{} }

// On reports the minimap is shown: a large map, not hidden.
func (m *Map) On(src Source) bool {
	return m != nil && !m.Off && max(src.Width(), src.Height()) > MinSide
}

// Toggle shows or hides the minimap (the Minimap action).
func (m *Map) Toggle() {
	if m != nil {
		m.Off = !m.Off
	}
}

// rect is the minimap's box for the logical screen ui: the layout's
// Minimap rect.
func (m *Map) rect(ui camera.Viewport) gfx.Rect {
	return hudlayout.Compute(int(ui.X1), int(ui.Y1), hudlayout.Inputs{MinimapOn: true, GlobalH: m.GlobalH}).Minimap
}

// Box is the minimap's box and the map's rectangle inside it (the map
// keeps its aspect, centred) for the logical screen ui, in logical pixels.
func (m *Map) Box(src Source, ui camera.Viewport) (bx, by float64, mx, my, mw, mh float64) {
	b := m.rect(ui)
	bx, by = float64(b.X), float64(b.Y)
	gw, gh := float64(src.Width()), float64(src.Height())
	s := float64(Side-2*Inset) / max(gw, gh, 1)
	mw, mh = gw*s, gh*s
	return bx, by, bx + (Side-mw)/2, by + (Side-mh)/2, mw, mh
}

// Click reports the logical point (sx, sy) is on the minimap and, when it
// is, the camera offsets (physical pixels) that centre the physical map
// viewport v on the tile under it; false (nothing to do) elsewhere or when
// it is hidden. The caller applies them to its camera, stops the
// smoothing and clamps. ui is the logical screen (the box).
func (m *Map) Click(rv *render.View, src Source, sx, sy int, ui, v camera.Viewport) (ok bool, offX, offY float64) {
	if !m.On(src) {
		return false, 0, 0
	}
	bx, by, mx, my, mw, mh := m.Box(src, ui)
	x, y := float64(sx), float64(sy)
	if x < bx || y < by || x >= bx+Side || y >= by+Side {
		return false, 0, 0
	}
	tx := min(max((x-mx)/mw, 0), 1) * float64(src.Width())
	ty := min(max((y-my)/mh, 0), 1) * float64(src.Height())
	t := rv.Cam.Tile()
	return true, tx*t - (v.X0+v.X1)/2, ty*t - (v.Y0+v.Y1)/2
}

// Colors are the terrain colours by terrain name.
var Colors = map[string]color.RGBA{
	"soil": {104, 84, 56, 255}, "rich_soil": {78, 62, 40, 255}, "gravel": {128, 120, 106, 255},
	"stone": {118, 114, 108, 255}, "rock": {74, 70, 66, 255}, "sand": {192, 172, 122, 255},
	"marsh": {82, 92, 60, 255}, "ice": {206, 222, 232, 255}, "rock_smooth": {96, 92, 88, 255},
	"stone_smooth": {136, 132, 126, 255}, "mud": {88, 70, 48, 255}, "track": {122, 102, 72, 255},
	"shallow_water": {72, 122, 150, 255}, "deep_water": {38, 70, 112, 255},
}

var (
	terrain = func() (out [64]color.RGBA) {
		for i, n := range game.TerrainNames() {
			if i < len(out) {
				c, ok := Colors[n]
				if !ok {
					c = color.RGBA{104, 84, 56, 255}
				}
				out[i] = c
			}
		}
		return out
	}()
	plantClr = color.RGBA{48, 88, 40, 255}
	floorClr = color.RGBA{136, 122, 100, 255}
	// StructureClr is a building's cell.
	StructureClr = color.RGBA{196, 186, 166, 255}
)

// Color is one cell's colour.
func Color(cell uint8) color.RGBA {
	c := terrain[cell&game.MinimapTerrain]
	switch cell & game.MinimapKind {
	case game.MinimapPlant:
		return color.RGBA{uint8((int(c.R) + int(plantClr.R)*2) / 3), uint8((int(c.G) + int(plantClr.G)*2) / 3),
			uint8((int(c.B) + int(plantClr.B)*2) / 3), 255}
	case game.MinimapFloor:
		return floorClr
	case game.MinimapStructure:
		return StructureClr
	}
	return c
}

// sync refreshes the minimap image from the read model, polling at most
// every Poll frames.
func (m *Map) sync(present uint64, src Source) {
	if m.Img != nil && present-m.polled < Poll {
		return
	}
	m.polled = present
	fw := src.World()
	ver := m.ver
	if len(fw.Fog) > 0 && fw.FogVer != m.fogVer {
		ver = 0 // the fog moved: rebuild from the layer
	}
	v, changed := src.MinimapLayer(ver)
	if !changed || v.W <= 0 || v.H <= 0 || len(v.Cells) < v.W*v.H {
		return
	}
	m.ver, m.fogVer = v.Ver, fw.FogVer
	if m.Img == nil || m.W != v.W || m.h != v.H {
		if m.Img != nil {
			m.Img.Deallocate()
		}
		m.Img, m.W, m.h = ebiten.NewImage(v.W, v.H), v.W, v.H
	}
	n := v.W * v.H
	if cap(m.Pix) < 4*n {
		m.Pix = make([]byte, 4*n)
	}
	m.Pix = m.Pix[:4*n]
	for i, cell := range v.Cells[:n] {
		c := Color(cell)
		if len(fw.Fog) > i && fw.Fog[i] != 0 && v.W == fw.TerrainW {
			c = fogClr // unexplored: black
		}
		m.Pix[4*i], m.Pix[4*i+1], m.Pix[4*i+2], m.Pix[4*i+3] = c.R, c.G, c.B, 255
	}
	m.Img.WritePixels(m.Pix)
	m.Polls++
}

// View is the camera's view as a rectangle inside the map's rectangle
// (mx, my, mw, mh), clamped to it: the physical map viewport v, which is
// the whole window.
func (m *Map) View(rv *render.View, src Source, v camera.Viewport, mx, my, mw, mh float64) (fx0, fy0, fx1, fy1 float64) {
	t := rv.Cam.Tile()
	if t <= 0 {
		return mx, my, mx, my
	}
	gw, gh := float64(src.Width()), float64(src.Height())
	vx0, vy0 := (v.X0+rv.Cam.OffsetX)/t, (v.Y0+rv.Cam.OffsetY)/t
	vx1, vy1 := (v.X1+rv.Cam.OffsetX)/t, (v.Y1+rv.Cam.OffsetY)/t
	return mx + min(max(vx0/gw, 0), 1)*mw, my + min(max(vy0/gh, 0), 1)*mh,
		mx + min(max(vx1/gw, 0), 1)*mw, my + min(max(vy1/gh, 0), 1)*mh
}

// fogged reports tile (x, y) is unexplored in w.
func fogged(w game.WorldView, x, y int) bool {
	if len(w.Fog) == 0 || w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return false
	}
	i := y*w.TerrainW + x
	return i < len(w.Fog) && w.Fog[i] != 0
}

// Draw draws the minimap when it is shown: the box in the logical screen
// ui, the view frame for the physical map viewport v.
func (m *Map) Draw(rv *render.View, screen *ebiten.Image, src Source, ui, v camera.Viewport) {
	if !m.On(src) {
		return
	}
	m.sync(rv.Present, src)
	bx, by, mx, my, mw, mh := m.Box(src, ui)
	uikit.DrawPanel(screen, float32(bx), float32(by), Side, Side)
	if m.Img != nil {
		gfx.Image(screen, m.Img, mx, my, mw, mh, &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear})
	}
	gw, gh := float64(src.Width()), float64(src.Height())
	fw0 := src.World()
	dot := func(x, y int, c color.RGBA) {
		px, py := mx+(float64(x)+0.5)/gw*mw, my+(float64(y)+0.5)/gh*mh
		gfx.FillRect(screen, float32(px-1.5), float32(py-1.5), 3, 3, c, false)
	}
	for _, p := range src.Snapshot() {
		if p.Embodied || p.CarriedBy != 0 {
			continue
		}
		if fogged(fw0, p.X, p.Y) {
			continue
		}
		if p.Hostile {
			dot(p.X, p.Y, uikit.Danger)
		} else {
			dot(p.X, p.Y, uikit.Text)
		}
	}
	animals := fw0.Animals
	for i := range animals {
		if an := &animals[i]; an.Manhunter && !fogged(fw0, an.X, an.Y) {
			dot(an.X, an.Y, uikit.Danger)
		}
	}
	// the camera's view
	if rv.Cam.Tile() <= 0 {
		return
	}
	fx0, fy0, fx1, fy1 := m.View(rv, src, v, mx, my, mw, mh)
	gfx.StrokeRect(screen, float32(fx0), float32(fy0), float32(fx1-fx0), float32(fy1-fy0), 1, uikit.Accent, false)
}
