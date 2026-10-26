package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
	minimapSide    = minimapSize // px, the box (hud_layout.go)
	minimapInset   = 2           // px of frame inside the box
	minimapMinSide = 150         // tiles: maps larger than this get a minimap
	// minimapPoll is how many frames apart the layer is asked for (it
	// changes when the ground or the plants' places change).
	minimapPoll = 30
)

// minimapState is the renderer's minimap.
type minimapState struct {
	off    bool // hidden by the player
	ver    uint64
	img    *ebiten.Image
	pix    []byte
	w, h   int
	fogVer uint64 // FogVer the image was built for
	polled uint64 // present frame of the last poll
	polls  int    // layer fetches that changed it (tests)
	// globalH is the global controls' height (hud_global.go sets it): the
	// minimap sits above them.
	globalH float32
}

// minimapOn reports the minimap is shown: a large map, not hidden.
func (r *Renderer) minimapOn() bool {
	return r != nil && !r.mini.off && max(r.game.Width(), r.game.Height()) > minimapMinSide
}

// toggleMinimap shows or hides the minimap (the Minimap action).
func (r *Renderer) toggleMinimap() {
	if r != nil {
		r.mini.off = !r.mini.off
	}
}

// minimapRect is the minimap's box for the logical screen ui: the
// layout's Minimap rect.
func (r *Renderer) minimapRect(ui viewport) rect {
	return computeHUDLayout(int(ui.x1), int(ui.y1), hudState{MinimapOn: true, GlobalH: r.mini.globalH}).Minimap
}

// minimapBox is the minimap's box and the map's rectangle inside it (the
// map keeps its aspect, centred) for the logical screen ui, in logical
// pixels.
func (r *Renderer) minimapBox(ui viewport) (bx, by float64, mx, my, mw, mh float64) {
	b := r.minimapRect(ui)
	bx, by = float64(b.X), float64(b.Y)
	gw, gh := float64(r.game.Width()), float64(r.game.Height())
	s := float64(minimapSide-2*minimapInset) / max(gw, gh, 1)
	mw, mh = gw*s, gh*s
	return bx, by, bx + (minimapSide-mw)/2, by + (minimapSide-mh)/2, mw, mh
}

// minimapClick centres the camera on the tile under the logical point
// (sx, sy) when it is on the minimap; false (nothing done) elsewhere or
// when it is hidden. ui is the logical screen (the box), v the physical
// map viewport the camera is centred in.
func (r *Renderer) minimapClick(sx, sy int, ui, v viewport) bool {
	if !r.minimapOn() {
		return false
	}
	bx, by, mx, my, mw, mh := r.minimapBox(ui)
	x, y := float64(sx), float64(sy)
	if x < bx || y < by || x >= bx+minimapSide || y >= by+minimapSide {
		return false
	}
	tx := min(max((x-mx)/mw, 0), 1) * float64(r.game.Width())
	ty := min(max((y-my)/mh, 0), 1) * float64(r.game.Height())
	t := r.tilePx()
	r.cam.offsetX = tx*t - (v.x0+v.x1)/2
	r.cam.offsetY = ty*t - (v.y0+v.y1)/2
	r.smooth.vx, r.smooth.vy = 0, 0
	r.clampCamera(v)
	return true
}

// minimapColors are the terrain colours by terrain name.
var minimapColors = map[string]color.RGBA{
	"soil": {104, 84, 56, 255}, "rich_soil": {78, 62, 40, 255}, "gravel": {128, 120, 106, 255},
	"stone": {118, 114, 108, 255}, "rock": {74, 70, 66, 255}, "sand": {192, 172, 122, 255},
	"marsh": {82, 92, 60, 255}, "ice": {206, 222, 232, 255}, "rock_smooth": {96, 92, 88, 255},
	"stone_smooth": {136, 132, 126, 255}, "mud": {88, 70, 48, 255}, "track": {122, 102, 72, 255},
	"shallow_water": {72, 122, 150, 255}, "deep_water": {38, 70, 112, 255},
}

var (
	minimapTerrain = func() (out [64]color.RGBA) {
		for i, n := range terrainNames {
			if i < len(out) {
				c, ok := minimapColors[n]
				if !ok {
					c = color.RGBA{104, 84, 56, 255}
				}
				out[i] = c
			}
		}
		return out
	}()
	minimapPlantClr     = color.RGBA{48, 88, 40, 255}
	minimapFloorClr     = color.RGBA{136, 122, 100, 255}
	minimapStructureClr = color.RGBA{196, 186, 166, 255}
)

// minimapColor is one cell's colour.
func minimapColor(cell uint8) color.RGBA {
	c := minimapTerrain[cell&game.MinimapTerrain]
	switch cell & game.MinimapKind {
	case game.MinimapPlant:
		return color.RGBA{uint8((int(c.R) + int(minimapPlantClr.R)*2) / 3), uint8((int(c.G) + int(minimapPlantClr.G)*2) / 3),
			uint8((int(c.B) + int(minimapPlantClr.B)*2) / 3), 255}
	case game.MinimapFloor:
		return minimapFloorClr
	case game.MinimapStructure:
		return minimapStructureClr
	}
	return c
}

// syncMinimap refreshes the minimap image from the read model, polling at
// most every minimapPoll frames.
func (r *Renderer) syncMinimap() {
	m := &r.mini
	if m.img != nil && r.present-m.polled < minimapPoll {
		return
	}
	m.polled = r.present
	fw := r.game.World()
	ver := m.ver
	if len(fw.Fog) > 0 && fw.FogVer != m.fogVer {
		ver = 0 // the fog moved: rebuild from the layer
	}
	v, changed := r.game.MinimapLayer(ver)
	if !changed || v.W <= 0 || v.H <= 0 || len(v.Cells) < v.W*v.H {
		return
	}
	m.ver, m.fogVer = v.Ver, fw.FogVer
	if m.img == nil || m.w != v.W || m.h != v.H {
		if m.img != nil {
			m.img.Deallocate()
		}
		m.img, m.w, m.h = ebiten.NewImage(v.W, v.H), v.W, v.H
	}
	n := v.W * v.H
	if cap(m.pix) < 4*n {
		m.pix = make([]byte, 4*n)
	}
	m.pix = m.pix[:4*n]
	for i, cell := range v.Cells[:n] {
		c := minimapColor(cell)
		if len(fw.Fog) > i && fw.Fog[i] != 0 && v.W == fw.TerrainW {
			c = fogClr // unexplored: black
		}
		m.pix[4*i], m.pix[4*i+1], m.pix[4*i+2], m.pix[4*i+3] = c.R, c.G, c.B, 255
	}
	m.img.WritePixels(m.pix)
	m.polls++
}

// minimapView is the camera's view as a rectangle inside the map's
// rectangle (mx, my, mw, mh), clamped to it: the physical map viewport v,
// which is the whole window.
func (r *Renderer) minimapView(v viewport, mx, my, mw, mh float64) (fx0, fy0, fx1, fy1 float64) {
	t := r.tilePx()
	if t <= 0 {
		return mx, my, mx, my
	}
	gw, gh := float64(r.game.Width()), float64(r.game.Height())
	vx0, vy0 := (v.x0+r.cam.offsetX)/t, (v.y0+r.cam.offsetY)/t
	vx1, vy1 := (v.x1+r.cam.offsetX)/t, (v.y1+r.cam.offsetY)/t
	return mx + min(max(vx0/gw, 0), 1)*mw, my + min(max(vy0/gh, 0), 1)*mh,
		mx + min(max(vx1/gw, 0), 1)*mw, my + min(max(vy1/gh, 0), 1)*mh
}

// drawMinimap draws the minimap when it is shown: the box in the logical
// screen ui, the view frame for the physical map viewport v.
func (r *Renderer) drawMinimap(screen *ebiten.Image, ui, v viewport) {
	if !r.minimapOn() {
		return
	}
	r.syncMinimap()
	m := &r.mini
	bx, by, mx, my, mw, mh := r.minimapBox(ui)
	drawPanel(screen, float32(bx), float32(by), minimapSide, minimapSide)
	if m.img != nil {
		uiImage(screen, m.img, mx, my, mw, mh, &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear})
	}
	gw, gh := float64(r.game.Width()), float64(r.game.Height())
	fw0 := r.game.World()
	dot := func(x, y int, c color.RGBA) {
		px, py := mx+(float64(x)+0.5)/gw*mw, my+(float64(y)+0.5)/gh*mh
		uiFillRect(screen, float32(px-1.5), float32(py-1.5), 3, 3, c, false)
	}
	for _, p := range r.game.Snapshot() {
		if p.Embodied || p.CarriedBy != 0 {
			continue
		}
		if worldFogged(fw0, p.X, p.Y) {
			continue
		}
		if p.Hostile {
			dot(p.X, p.Y, uiDanger)
		} else {
			dot(p.X, p.Y, uiText)
		}
	}
	animals := fw0.Animals
	for i := range animals {
		if an := &animals[i]; an.Manhunter && !worldFogged(fw0, an.X, an.Y) {
			dot(an.X, an.Y, uiDanger)
		}
	}
	// the camera's view
	if r.tilePx() <= 0 {
		return
	}
	fx0, fy0, fx1, fy1 := r.minimapView(v, mx, my, mw, mh)
	uiStrokeRect(screen, float32(fx0), float32(fy0), float32(fx1-fx0), float32(fy1-fy0), 1, uiAccent, false)
}
