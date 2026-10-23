package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Minimap (NG-1 §2.12): on maps over 150² a 180×180 overview sits in the
// bottom-right corner of the map viewport, toggled with the Minimap
// action. It shows the ground (one pixel per tile from the read model
// Simulation.MinimapLayer: terrain colours, water, floors, buildings and
// plants), colonists as light dots, hostiles as red dots and the camera's
// view as a rectangle; a click on it centres the camera there.

const (
	minimapSide    = 180 // px, the box
	minimapMargin  = 10  // px from the viewport's corner
	minimapMinSide = 150 // tiles: maps larger than this get a minimap
	// minimapAbove keeps the box above the hover cell readout, which shares
	// the bottom-right corner (hud.go, up to ~8 lines).
	minimapAbove = 150
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
	polled uint64 // present frame of the last poll
	polls  int    // layer fetches that changed it (tests)
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

// minimapBox is the minimap's box and the map's rectangle inside it (the
// map keeps its aspect, centred) for viewport v.
func (r *Renderer) minimapBox(v viewport) (bx, by float64, mx, my, mw, mh float64) {
	bx, by = v.x1-minimapMargin-minimapSide, v.y1-minimapMargin-minimapSide-minimapAbove
	gw, gh := float64(r.game.Width()), float64(r.game.Height())
	s := minimapSide / max(gw, gh, 1)
	mw, mh = gw*s, gh*s
	return bx, by, bx + (minimapSide-mw)/2, by + (minimapSide-mh)/2, mw, mh
}

// minimapClick centres the camera on the tile under (sx, sy) when it is
// on the minimap; false (nothing done) elsewhere or when it is hidden.
func (r *Renderer) minimapClick(sx, sy int, v viewport) bool {
	if !r.minimapOn() {
		return false
	}
	bx, by, mx, my, mw, mh := r.minimapBox(v)
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
	minimapColonistClr  = color.RGBA{240, 240, 220, 255}
	minimapHostileClr   = color.RGBA{230, 50, 40, 255}
	minimapFrameClr     = color.RGBA{250, 236, 160, 255}
	minimapBackClr      = color.RGBA{14, 12, 10, 220}
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
	v, changed := r.game.MinimapLayer(m.ver)
	if !changed || v.W <= 0 || v.H <= 0 || len(v.Cells) < v.W*v.H {
		return
	}
	m.ver = v.Ver
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
		m.pix[4*i], m.pix[4*i+1], m.pix[4*i+2], m.pix[4*i+3] = c.R, c.G, c.B, 255
	}
	m.img.WritePixels(m.pix)
	m.polls++
}

// drawMinimap draws the minimap for viewport v when it is shown.
func (r *Renderer) drawMinimap(screen *ebiten.Image, v viewport) {
	if !r.minimapOn() {
		return
	}
	r.syncMinimap()
	m := &r.mini
	bx, by, mx, my, mw, mh := r.minimapBox(v)
	vector.FillRect(screen, float32(bx-2), float32(by-2), minimapSide+4, minimapSide+4, minimapBackClr, false)
	if m.img != nil {
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(mw/float64(m.w), mh/float64(m.h))
		op.GeoM.Translate(mx, my)
		screen.DrawImage(m.img, op)
	}
	gw, gh := float64(r.game.Width()), float64(r.game.Height())
	dot := func(x, y int, c color.RGBA) {
		px, py := mx+(float64(x)+0.5)/gw*mw, my+(float64(y)+0.5)/gh*mh
		vector.FillRect(screen, float32(px-1.5), float32(py-1.5), 3, 3, c, false)
	}
	for _, p := range r.game.Snapshot() {
		if p.Embodied || p.CarriedBy != 0 {
			continue
		}
		if p.Hostile {
			dot(p.X, p.Y, minimapHostileClr)
		} else {
			dot(p.X, p.Y, minimapColonistClr)
		}
	}
	animals := r.game.World().Animals
	for i := range animals {
		if an := &animals[i]; an.Manhunter {
			dot(an.X, an.Y, minimapHostileClr)
		}
	}
	// the camera's view
	t := r.tilePx()
	if t <= 0 {
		return
	}
	vx0, vy0 := (v.x0+r.cam.offsetX)/t, (v.y0+r.cam.offsetY)/t
	vx1, vy1 := (v.x1+r.cam.offsetX)/t, (v.y1+r.cam.offsetY)/t
	fx0, fy0 := mx+min(max(vx0/gw, 0), 1)*mw, my+min(max(vy0/gh, 0), 1)*mh
	fx1, fy1 := mx+min(max(vx1/gw, 0), 1)*mw, my+min(max(vy1/gh, 0), 1)*mh
	vector.StrokeRect(screen, float32(fx0), float32(fy0), float32(fx1-fx0), float32(fy1-fy0), 1, minimapFrameClr, false)
}
