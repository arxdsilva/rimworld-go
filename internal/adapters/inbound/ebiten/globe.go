package ebiten

import (
	"image"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
)

// The landing-site globe (NG-1 §2.7): the planet's hex tiles as one
// triangle batch per frame, with rivers, roads, settlement emblems and
// outlines on top. Presentation only: it reads a finished *planet.Planet
// and never changes it. The geometry is built once per planet; the tile
// colours change only with the view mode; positions and lighting are
// recomputed every frame for the tiles facing the eye.

// globeMode is the landing screen's view mode (recolour only).
type globeMode uint8

const (
	globeBiome globeMode = iota
	globeTemperature
	globeRainfall
	globeElevation
	numGlobeModes
)

func (m globeMode) String() string {
	switch m {
	case globeTemperature:
		return "Temperature"
	case globeRainfall:
		return "Rainfall"
	case globeElevation:
		return "Elevation"
	}
	return "Biome"
}

// Pointer results of Update: no tile, or the uncovered part of the sphere
// (a click there selects nothing; the screen says "Not part of this world").
const (
	globeNone      = -1
	globeUncovered = -2
)

const (
	globeDragPx         = 4  // a click moves less than this between press and release
	globeDoubleFrames   = 24 // frames between the clicks of a double-click
	globeDoublePx       = 6
	globeCullSlack      = 0.08 // tiles this far past the horizon are still drawn
	globeCornerShade    = 0.94 // tile corners are slightly darker than the centre: a bevel
	globeFlyFrames      = 24
	globeTilePx1        = 0.0265 // a tile's angular spacing, radians
	globeSphereBakeSize = 640    // baked sphere image (halo + shaded disc), px
	globeSphereBakeR    = 256    // the disc's radius in that image
)

// worldPalette mirrors assets/palettes/world.yaml (without the "world."
// prefix); TestGlobePaletteMatchesYAML keeps the two equal.
var worldPalette = map[string]color.RGBA{
	"ocean_deep":          hex6(0x0E2A47),
	"ocean_shallow":       hex6(0x1F4E79),
	"lake":                hex6(0x2D6A9A),
	"river":               hex6(0x3F86C2),
	"road":                hex6(0x8C7355),
	"uncovered":           hex6(0x121820),
	"atmosphere_rim":      hex6(0x4A7FD4),
	"mountain_tint":       hex6(0x6E6A62),
	"ice_sheet":           hex6(0xE6EEF2),
	"sea_ice":             hex6(0xCFE0EA),
	"tundra":              hex6(0x9AA58A),
	"boreal_forest":       hex6(0x3F5E46),
	"cold_bog":            hex6(0x5B6B4E),
	"temperate_forest":    hex6(0x4E7A3A),
	"temperate_swamp":     hex6(0x4A6B47),
	"tropical_rainforest": hex6(0x2F6B2A),
	"tropical_swamp":      hex6(0x3C6A3A),
	"arid_shrubland":      hex6(0xA58E5A),
	"desert":              hex6(0xCDAE72),
	"extreme_desert":      hex6(0xDDBE86),
}

func hex6(v uint32) color.RGBA {
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

type rgb = [3]float32

func toRGB(c color.RGBA) rgb {
	return rgb{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255}
}

func palRGB(key string) rgb { return toRGB(worldPalette[key]) }

func mixRGB(a, b rgb, t float32) rgb {
	return rgb{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

func scaleRGB(a rgb, k float32) rgb { return rgb{a[0] * k, a[1] * k, a[2] * k} }

func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// hillMix is how far a tile's colour moves towards the mountain tint
// (NG-1 §1.2): flat 0, small hills 12 %, large hills 25 %, mountainous 40 %.
var hillMix = [...]float32{site.HillsDefault: 0, site.Flat: 0, site.SmallHills: 0.12, site.LargeHills: 0.25, site.Mountainous: 0.40}

// colourStop is a stop of a data-view colour ramp.
type colourStop struct {
	at float32
	c  rgb
}

func stop(at float32, v uint32) colourStop { return colourStop{at, toRGB(hex6(v))} }

func ramp(stops []colourStop, v float32) rgb {
	if v <= stops[0].at {
		return stops[0].c
	}
	for i := 1; i < len(stops); i++ {
		if v <= stops[i].at {
			a, b := stops[i-1], stops[i]
			return mixRGB(a.c, b.c, (v-a.at)/(b.at-a.at))
		}
	}
	return stops[len(stops)-1].c
}

var (
	tempRamp = []colourStop{stop(-30, 0x25318F), stop(-15, 0x4A7BC8), stop(-2, 0xA9D6E8),
		stop(8, 0xA8D58A), stop(18, 0xE6D96A), stop(26, 0xEE9A3E), stop(34, 0xC8412E)}
	rainRamp = []colourStop{stop(0, 0xC9A66B), stop(300, 0xD8C98A), stop(800, 0x9CC47A),
		stop(1500, 0x3F9A5E), stop(2400, 0x2B6FA8), stop(3500, 0x1B3F8A)}
	elevRamp = []colourStop{stop(0, 0x3E7A45), stop(300, 0x7FA050), stop(800, 0xB7A862),
		stop(1600, 0x9C7B55), stop(3000, 0x8A7F78), stop(4500, 0xF0F0F0)}
)

// globeAccent is a faction's colour on the globe (emblem tint).
func globeAccent(t faction.Type) color.NRGBA {
	switch t {
	case faction.Settlers:
		return color.NRGBA{0x5B, 0x9B, 0xD5, 0xFF}
	case faction.Clans:
		return color.NRGBA{0xD9, 0xA4, 0x41, 0xFF}
	case faction.Drifters:
		return color.NRGBA{0xA9, 0x8B, 0xD0, 0xFF}
	case faction.Freebooters:
		return color.NRGBA{0xD2, 0x56, 0x4A, 0xFF}
	case faction.Swarm:
		return color.NRGBA{0x8F, 0xBF, 0x5A, 0xFF}
	}
	return color.NRGBA{0xF2, 0xD9, 0xA6, 0xFF}
}

// Lighting: the sun is fixed in view space (upper left, towards the
// viewer), so the shade of a vertex depends only on its view-space normal.
var globeSun = func() [3]float32 {
	v := [3]float64{-0.42, 0.42, 0.80}
	l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	return [3]float32{float32(v[0] / l), float32(v[1] / l), float32(v[2] / l)}
}()

// globeShade is the Lambert factor k1 for a unit view-space normal and the
// atmosphere share k2 (a rim that grows towards the limb): the lit colour
// is base*k1 + rim*k2.
func globeShade(vx, vy, vz float32) (k1, k2 float32) {
	diff := vx*globeSun[0] + vy*globeSun[1] + vz*globeSun[2]
	if diff < 0 {
		diff = 0
	}
	s := 0.35 + 0.65*diff
	t := 1 - vz
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	rim := t * t * t * 0.45
	return s * (1 - rim), rim
}

// globeSeg is a river or road piece between two surface points.
type globeSeg struct {
	a, b [3]float32
	w    uint8
}

type settleMark struct {
	tile  int32
	key   string
	tint  color.NRGBA
	label string
}

// globeInput is one frame of pointer and keyboard state for the globe.
type globeInput struct {
	W, H    int // screen size
	MX, MY  int
	Wheel   float64
	Down    [numMouse]bool
	Press   [numMouse]bool
	Release [numMouse]bool
	Held    []string // keys down now (input key names)
	Keys    []string // keys that went down this frame
	Shift   bool
	// Blocked: the pointer is over a panel or modal; the globe ignores
	// hover, wheel and new presses (a drag in progress continues).
	Blocked bool
}

// globeInputFrom adapts the App's frame. blocked is whether the pointer is
// over screen chrome.
func globeInputFrom(in *inputFrame, w, h int, blocked bool) globeInput {
	return globeInput{
		W: w, H: h, MX: in.raw.mx, MY: in.raw.my, Wheel: in.raw.wheelY,
		Down: in.raw.down, Press: in.raw.press, Release: in.raw.release,
		Held: in.raw.held, Keys: in.raw.pressed, Shift: in.shift(), Blocked: blocked,
	}
}

type globeDrag struct {
	on         bool
	moved      bool
	sx, sy     int
	lx, ly     int
	vx, vy     float64
	panning    bool
	plx, ply   int
	lastClickT int
	lastClickX int
	lastClickY int
	lastTile   int
}

// globeView draws and drives the planet globe.
type globeView struct {
	p   *planet.Planet
	art *artCatalog
	cam *globemath.Camera

	mode     globeMode
	selected int
	hovered  int
	reduced  bool
	tick     int
	drag     globeDrag
	hl       []int
	hlLeft   int

	area   [4]float64 // x, y, w, h of the globe's screen area (zero w: whole screen)
	sw, sh int

	// geometry, built once per planet
	cornerID  []uint32
	cornerPos [][3]float32
	base      []rgb
	rivers    []globeSeg
	roads     []globeSeg
	marks     []settleMark
	capTiles  int

	// per-frame state (build)
	bf                 uint32
	stamp              []uint32
	csx, csy, ck1, ck2 []float32
	mat                [9]float32
	camDir             [3]float32
	zh, limbR          float32
	cx, cy, focal      float32
	dist               float32
	verts              []ebiten.Vertex
	idx                []uint32
	nV, nI             int
	ov                 []ebiten.Vertex
	oi                 []uint32

	// GPU resources, created on the first Draw
	white    *ebiten.Image
	whiteSub *ebiten.Image
	sphere   *ebiten.Image
	stars    *ebiten.Image
	starW    int
	starH    int
	opts     ebiten.DrawTrianglesOptions
	ovMax    int
	rimColor rgb
}

// newGlobeView prepares the globe for p. art supplies the settlement
// emblems and may be nil (circles are drawn instead).
func newGlobeView(p *planet.Planet, art *artCatalog) *globeView {
	n := p.TileCount()
	g := &globeView{p: p, art: art, cam: globemath.NewCamera(), selected: -1, hovered: -1}
	g.cornerID, g.cornerPos = dedupeCorners(p.Corners)
	g.stamp = make([]uint32, len(g.cornerPos))
	g.csx = make([]float32, len(g.cornerPos))
	g.csy = make([]float32, len(g.cornerPos))
	g.ck1 = make([]float32, len(g.cornerPos))
	g.ck2 = make([]float32, len(g.cornerPos))
	// On the full sphere at most the cap seen from MaxDist (plus the cull
	// slack) faces the eye; a partial world can be seen whole.
	g.capTiles = n
	if p.CapCos <= -0.999 {
		g.capTiles = min(n, int(0.45*float64(n))+64)
	}
	g.verts = make([]ebiten.Vertex, 7*g.capTiles)
	for i := range g.verts {
		g.verts[i].SrcX, g.verts[i].SrcY, g.verts[i].ColorA = 1.5, 1.5, 1
	}
	g.idx = make([]uint32, 18*g.capTiles)
	g.ovMax = 4 * (len(p.Roads)*2 + countRiverSegs(p) + 4*1024)
	g.ov = make([]ebiten.Vertex, 0, g.ovMax)
	g.oi = make([]uint32, 0, g.ovMax/4*6)
	g.rimColor = scaleRGB(palRGB("atmosphere_rim"), 0.85)
	g.buildLines()
	g.buildMarks()
	g.recolour()
	g.drag.lastTile = globeNone
	return g
}

func countRiverSegs(p *planet.Planet) int {
	n := 0
	for i := range p.River {
		if p.River[i] != site.NoRiver && p.RiverTo[i] >= 0 {
			n += 2
		}
	}
	return n
}

type cornerKey [3]int32

// dedupeCorners merges the per-tile corner copies into unique corners (a
// corner is shared by three tiles). The copies differ by float rounding,
// so each is matched against its grid cell and the neighbouring cells it
// lies close to.
func dedupeCorners(corners [][3]float32) (ids []uint32, pos [][3]float32) {
	const cell = 4e-3
	const edge = 1e-3 / cell // distance to a cell border, in cells, that also checks the next cell
	const tol = 1e-3
	ids = make([]uint32, len(corners))
	pos = make([][3]float32, 0, len(corners)/3+16)
	seen := make(map[cornerKey]uint32, len(corners)/3+16)
	for i, c := range corners {
		var k cornerKey
		var lo, hi [3]bool
		for a := 0; a < 3; a++ {
			f := float64(c[a]) / cell
			fl := math.Floor(f)
			k[a] = int32(fl)
			lo[a] = f-fl < edge
			hi[a] = fl+1-f < edge
		}
		found := false
		for dx := -1; dx <= 1 && !found; dx++ {
			if (dx < 0 && !lo[0]) || (dx > 0 && !hi[0]) {
				continue
			}
			for dy := -1; dy <= 1 && !found; dy++ {
				if (dy < 0 && !lo[1]) || (dy > 0 && !hi[1]) {
					continue
				}
				for dz := -1; dz <= 1 && !found; dz++ {
					if (dz < 0 && !lo[2]) || (dz > 0 && !hi[2]) {
						continue
					}
					id, ok := seen[cornerKey{k[0] + int32(dx), k[1] + int32(dy), k[2] + int32(dz)}]
					if !ok {
						continue
					}
					q := pos[id]
					if abs32(q[0]-c[0]) < tol && abs32(q[1]-c[1]) < tol && abs32(q[2]-c[2]) < tol {
						ids[i], found = id, true
					}
				}
			}
		}
		if !found {
			id := uint32(len(pos))
			pos = append(pos, c)
			seen[k] = id
			ids[i] = id
		}
	}
	return ids, pos
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func norm3(v [3]float32) [3]float32 {
	l := float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
	if l == 0 {
		return v
	}
	return [3]float32{v[0] / l, v[1] / l, v[2] / l}
}

// buildLines turns the planet's rivers and roads into surface segments.
func (g *globeView) buildLines() {
	p := g.p
	for i := range p.River {
		if p.River[i] == site.NoRiver || p.RiverTo[i] < 0 {
			continue
		}
		a, b := p.Center[i], p.Center[p.RiverTo[i]]
		m := norm3([3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]})
		w := uint8(p.River[i])
		g.rivers = append(g.rivers, globeSeg{a, m, w}, globeSeg{m, b, w})
	}
	for _, r := range p.Roads {
		g.roads = append(g.roads, globeSeg{p.Center[r[0]], p.Center[r[1]], 1})
	}
}

// buildMarks lists the settlements with their emblem key, tint and label.
func (g *globeView) buildMarks() {
	p := g.p
	for _, t := range p.SettlementTiles() {
		f, _ := p.SettlementFaction(int(t))
		if f < 0 || f >= len(p.Factions) {
			continue
		}
		ft := p.Factions[f].Type
		key := "ui_icon/settlement_industrial"
		if faction.DefOf(ft).Tech == faction.Neolithic {
			key = "ui_icon/settlement_neolithic"
		}
		g.marks = append(g.marks, settleMark{tile: t, key: key, tint: globeAccent(ft), label: p.SettlementName(int(t))})
	}
}

// --- colours -----------------------------------------------------------

func oceanColour(elev int16) rgb {
	d := clampF32(float32(-elev)/3500, 0, 1)
	t := 1 - float32(math.Sqrt(float64(d)))
	return mixRGB(palRGB("ocean_deep"), palRGB("ocean_shallow"), t)
}

// tileColour is tile i's flat colour in the current view mode.
func (g *globeView) tileColour(i int) rgb {
	p := g.p
	switch p.Water[i] {
	case planet.Ocean:
		c := oceanColour(p.Elev[i])
		if g.mode != globeBiome {
			c = scaleRGB(c, 0.55)
		}
		return c
	case planet.Lake:
		c := palRGB("lake")
		if g.mode != globeBiome {
			c = scaleRGB(c, 0.7)
		}
		return c
	}
	switch g.mode {
	case globeTemperature:
		return ramp(tempRamp, float32(p.Temp[i])/10)
	case globeRainfall:
		return ramp(rainRamp, float32(p.Rain[i]))
	case globeElevation:
		return ramp(elevRamp, float32(p.Elev[i]))
	}
	c := palRGB(biome.DefOf(p.Biome[i]).Key)
	h := int(p.Hill[i])
	if h < len(hillMix) {
		c = mixRGB(c, palRGB("mountain_tint"), hillMix[h])
	}
	// A touch of per-tile variation so the hex mosaic reads.
	j := float32(mix64(uint64(i)*0x9E3779B97F4A7C15+0x77)>>40) / float32(1<<24)
	return scaleRGB(c, 0.97+0.06*j)
}

// recolour recomputes every tile's colour for the current mode.
func (g *globeView) recolour() {
	n := g.p.TileCount()
	if cap(g.base) < n {
		g.base = make([]rgb, n)
	}
	g.base = g.base[:n]
	for i := 0; i < n; i++ {
		g.base[i] = g.tileColour(i)
	}
}

// SetMode switches the view mode and recolours the tiles.
func (g *globeView) SetMode(m globeMode) {
	if m >= numGlobeModes || m == g.mode {
		return
	}
	g.mode = m
	g.recolour()
}

// Mode is the current view mode.
func (g *globeView) Mode() globeMode { return g.mode }

// --- state and API for the screen ---------------------------------------

// Select marks tile i as the selection (-1 clears).
func (g *globeView) Select(i int) {
	if i < 0 || i >= g.p.TileCount() {
		i = -1
	}
	g.selected = i
}

// Selected is the selected tile, -1 when none.
func (g *globeView) Selected() int { return g.selected }

// Hovered is the tile under the pointer after the last Update, -1 when none.
func (g *globeView) Hovered() int { return g.hovered }

// Center flies the view to tile i (instantly with Reduced Motion).
func (g *globeView) Center(i int) {
	if i < 0 || i >= g.p.TileCount() {
		return
	}
	c := g.p.Center[i]
	frames := globeFlyFrames
	if g.reduced {
		frames = 0
	}
	g.cam.PanX, g.cam.PanY = 0, 0
	g.cam.CentreOn(globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])}, frames)
}

// Highlight pulses the given tiles for the next frames Draw calls (the
// pulse is steady with Reduced Motion).
func (g *globeView) Highlight(tiles []int, frames int) {
	g.hl = append(g.hl[:0], tiles...)
	g.hlLeft = frames
}

// SetArea sets the screen rectangle the globe is centred in (default: the
// whole screen). Call it when the layout changes, e.g. to leave room for a
// side panel.
func (g *globeView) SetArea(x, y, w, h float64) { g.area = [4]float64{x, y, w, h} }

func (g *globeView) areaRect() (x, y, w, h float64) {
	if g.area[2] > 0 && g.area[3] > 0 {
		return g.area[0], g.area[1], g.area[2], g.area[3]
	}
	return 0, 0, float64(g.sw), float64(g.sh)
}

func (g *globeView) viewport() globemath.Viewport {
	x, y, w, h := g.areaRect()
	return g.cam.Viewport(x, y, w, h)
}

// pickTile is the tile under a screen point: a tile index, globeUncovered
// over the part of the sphere without tiles, or globeNone off the globe.
func (g *globeView) pickTile(mx, my int) int {
	w, ok := g.cam.Pick(g.viewport(), float64(mx), float64(my))
	if !ok {
		return globeNone
	}
	t := g.p.TileAt([3]float32{float32(w[0]), float32(w[1]), float32(w[2])})
	if t < 0 {
		return globeUncovered
	}
	return t
}

// tilePx is roughly how many pixels one tile spans at the view centre.
func (g *globeView) tilePx() float64 {
	vp := g.viewport()
	return globeTilePx1 * vp.F / (g.cam.Dist - 1)
}

func isHeld(keys []string, names ...string) bool {
	for _, k := range keys {
		for _, n := range names {
			if k == n {
				return true
			}
		}
	}
	return false
}

// Update handles one frame of input. It returns the tile the player
// selected this frame (globeNone if none; globeUncovered for a click on the
// uncovered sphere, which clears the selection) and the tile under the
// pointer (globeNone, or globeUncovered).
//
//	left click (release within 4 px)  select; a second click on the same
//	                                  tile within 24 frames also centres it
//	left drag                         rotate, with inertia (not with Reduced Motion)
//	middle drag                       pan the globe on screen
//	wheel, PageUp/PageDown, +/-       zoom (1.25..4.0 radii)
//	WASD / arrows                     rotate (Shift 3x)
//	right click                       clear the selection
//	Home                              centre on the selection (and reset the pan)
func (g *globeView) Update(in globeInput, reducedMotion bool) (clicked, hovered int) {
	g.tick++
	g.reduced = reducedMotion
	if in.W > 0 && in.H > 0 {
		g.sw, g.sh = in.W, in.H
	}
	cam := g.cam
	clicked = globeNone
	if g.hlLeft > 0 {
		g.hlLeft--
	}

	// Keyboard: rotate and zoom.
	step := 0.010 * (cam.Dist - 0.8)
	if in.Shift {
		step *= 3
	}
	var dLon, dLat float64
	if isHeld(in.Held, "A", "ArrowLeft") {
		dLon -= step
	}
	if isHeld(in.Held, "D", "ArrowRight") {
		dLon += step
	}
	if isHeld(in.Held, "W", "ArrowUp") {
		dLat += step
	}
	if isHeld(in.Held, "S", "ArrowDown") {
		dLat -= step
	}
	if dLon != 0 || dLat != 0 {
		cam.Look(dLon/math.Max(math.Cos(cam.Lat), 0.25), dLat)
	}
	if isHeld(in.Held, "PageUp", "Equal", "NumpadAdd") {
		cam.Zoom(0.96)
	}
	if isHeld(in.Held, "PageDown", "Minus", "NumpadSubtract") {
		cam.Zoom(1.04)
	}
	if !in.Blocked && in.Wheel != 0 {
		cam.Zoom(math.Pow(0.88, in.Wheel))
	}
	if isHeld(in.Keys, "Home") {
		g.Center(g.selected)
		if g.selected < 0 {
			cam.PanX, cam.PanY = 0, 0
		}
	}

	// Left button: click or drag.
	d := &g.drag
	vp := g.viewport()
	if in.Press[mbLeft] && !in.Blocked && !d.on {
		*d = globeDrag{on: true, sx: in.MX, sy: in.MY, lx: in.MX, ly: in.MY,
			lastClickT: d.lastClickT, lastClickX: d.lastClickX, lastClickY: d.lastClickY, lastTile: d.lastTile}
		cam.StopSpin()
	}
	if d.on {
		if in.Down[mbLeft] {
			if !d.moved && sqr(in.MX-d.sx)+sqr(in.MY-d.sy) > globeDragPx*globeDragPx {
				d.moved = true
				d.lx, d.ly = d.sx, d.sy
			}
			if d.moved {
				dx, dy := float64(in.MX-d.lx), float64(in.MY-d.ly)
				cam.Rotate(dx, dy, vp)
				d.vx = 0.6*dx + 0.4*d.vx
				d.vy = 0.6*dy + 0.4*d.vy
				d.lx, d.ly = in.MX, in.MY
			}
		}
		if in.Release[mbLeft] || !in.Down[mbLeft] && !in.Press[mbLeft] {
			if d.moved {
				cam.Fling(d.vx, d.vy, vp, reducedMotion)
			} else {
				clicked = g.click(in.MX, in.MY)
			}
			d.on, d.moved, d.vx, d.vy = false, false, 0, 0
		}
	}

	// Middle button: pan.
	if in.Press[mbMiddle] && !in.Blocked {
		d.panning = true
		d.plx, d.ply = in.MX, in.MY
	}
	if d.panning {
		if in.Down[mbMiddle] {
			cam.PanX += float64(in.MX - d.plx)
			cam.PanY += float64(in.MY - d.ply)
			_, _, w, h := g.areaRect()
			cam.ClampPan(w*0.8, h*0.8)
			d.plx, d.ply = in.MX, in.MY
		} else {
			d.panning = false
		}
	}

	// Right button: clear.
	if in.Press[mbRight] && !in.Blocked {
		g.selected = -1
	}

	cam.Step(reducedMotion)

	g.hovered = globeNone
	if !in.Blocked && !d.moved && !d.panning {
		g.hovered = g.pickTile(in.MX, in.MY)
	}
	return clicked, g.hovered
}

func globeClamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func sqr(v int) int { return v * v }

// click selects the tile under (mx, my); a double-click also centres it.
func (g *globeView) click(mx, my int) int {
	d := &g.drag
	t := g.pickTile(mx, my)
	if t < 0 {
		if t == globeUncovered {
			g.selected = -1
		}
		d.lastTile = globeNone
		return t
	}
	g.selected = t
	if d.lastTile == t && g.tick-d.lastClickT <= globeDoubleFrames &&
		sqr(mx-d.lastClickX)+sqr(my-d.lastClickY) <= globeDoublePx*globeDoublePx {
		g.Center(t)
		d.lastTile = globeNone
	} else {
		d.lastTile, d.lastClickT, d.lastClickX, d.lastClickY = t, g.tick, mx, my
	}
	return t
}

// --- per-frame mesh ------------------------------------------------------

// setupFrame loads the camera into the per-frame fields.
func (g *globeView) setupFrame() {
	m := g.cam.Orientation().Matrix()
	for i, v := range m {
		g.mat[i] = float32(v)
	}
	c := g.cam.Centre()
	g.camDir = [3]float32{float32(c[0]), float32(c[1]), float32(c[2])}
	g.dist = float32(g.cam.Dist)
	g.zh = float32(globemath.HorizonZ(g.cam.Dist))
	g.limbR = float32(math.Sqrt(float64(1 - g.zh*g.zh)))
	vp := g.viewport()
	g.cx, g.cy, g.focal = float32(vp.CX), float32(vp.CY), float32(vp.F)
	g.bf++
}

// view rotates a world vector into view space.
func (g *globeView) view(p *[3]float32) (vx, vy, vz float32) {
	m := &g.mat
	return m[0]*p[0] + m[1]*p[1] + m[2]*p[2],
		m[3]*p[0] + m[4]*p[1] + m[5]*p[2],
		m[6]*p[0] + m[7]*p[1] + m[8]*p[2]
}

// project maps a view-space point to the screen; points past the horizon
// are pushed onto the limb so a tile that straddles it keeps a sane shape.
func (g *globeView) project(vx, vy, vz float32) (sx, sy float32) {
	if vz < g.zh {
		r := float32(math.Hypot(float64(vx), float64(vy)))
		if r < 1e-6 {
			r = 1e-6
		}
		s := g.limbR / r
		vx, vy, vz = vx*s, vy*s, g.zh
	}
	k := g.focal / (g.dist - vz)
	return g.cx + vx*k, g.cy - vy*k
}

// corner makes sure unique corner id has this frame's screen position and
// light, and returns them.
func (g *globeView) corner(id uint32) (sx, sy, k1, k2 float32) {
	if g.stamp[id] != g.bf {
		vx, vy, vz := g.view(&g.cornerPos[id])
		g.csx[id], g.csy[id] = g.project(vx, vy, vz)
		g.ck1[id], g.ck2[id] = globeShade(vx, vy, vz)
		g.stamp[id] = g.bf
	}
	return g.csx[id], g.csy[id], g.ck1[id], g.ck2[id]
}

// build fills the tile and overlay meshes for the current camera. It
// allocates nothing once the buffers exist.
func (g *globeView) build() {
	g.setupFrame()
	p := g.p
	thresh := g.zh - globeCullSlack
	cd := &g.camDir
	rim := g.rimColor
	nv, ni := 0, 0
	verts, idx := g.verts, g.idx
	tiles := 0
	for i := range p.Center {
		c := &p.Center[i]
		if c[0]*cd[0]+c[1]*cd[1]+c[2]*cd[2] < thresh {
			continue
		}
		if tiles++; tiles > g.capTiles {
			break
		}
		o0, o1 := p.CornerOff[i], p.CornerOff[i+1]
		n := int(o1 - o0)
		base := g.base[i]

		vx, vy, vz := g.view(c)
		sx, sy := g.project(vx, vy, vz)
		k1, k2 := globeShade(vx, vy, vz)
		v := &verts[nv]
		v.DstX, v.DstY = sx, sy
		v.ColorR = min32(base[0]*k1+rim[0]*k2, 1)
		v.ColorG = min32(base[1]*k1+rim[1]*k2, 1)
		v.ColorB = min32(base[2]*k1+rim[2]*k2, 1)
		first := uint32(nv)
		nv++
		for k := 0; k < n; k++ {
			x, y, c1, c2 := g.corner(g.cornerID[int(o0)+k])
			v := &verts[nv]
			v.DstX, v.DstY = x, y
			c1 *= globeCornerShade
			v.ColorR = min32(base[0]*c1+rim[0]*c2, 1)
			v.ColorG = min32(base[1]*c1+rim[1]*c2, 1)
			v.ColorB = min32(base[2]*c1+rim[2]*c2, 1)
			nv++
			idx[ni] = first
			idx[ni+1] = first + 1 + uint32(k)
			idx[ni+2] = first + 1 + uint32((k+1)%n)
			ni += 3
		}
	}
	g.nV, g.nI = nv, ni
	g.buildOverlay()
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// worldPt projects a world point: screen position and view-space z.
func (g *globeView) worldPt(p *[3]float32) (sx, sy, vz float32) {
	vx, vy, vz := g.view(p)
	sx, sy = g.project(vx, vy, vz)
	return sx, sy, vz
}

// ovColour is a straight-alpha colour for the overlay mesh.
type ovColour struct{ r, g, b, a float32 }

func ovFrom(c color.RGBA, a float32) ovColour {
	return ovColour{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, a}
}

func (g *globeView) ovVert(x, y float32, c ovColour) {
	g.ov = append(g.ov, ebiten.Vertex{DstX: x, DstY: y, SrcX: 1.5, SrcY: 1.5,
		ColorR: c.r, ColorG: c.g, ColorB: c.b, ColorA: c.a})
}

// ovQuad adds a w-wide line from (ax, ay) to (bx, by), its ends extended
// by a third of the width so consecutive pieces overlap at bends.
func (g *globeView) ovQuad(ax, ay, bx, by, w float32, c ovColour) {
	if len(g.ov)+4 > g.ovMax {
		return
	}
	dx, dy := bx-ax, by-ay
	l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if l < 0.05 {
		return
	}
	dx, dy = dx/l, dy/l
	e := w / 3
	ax, ay, bx, by = ax-dx*e, ay-dy*e, bx+dx*e, by+dy*e
	nx, ny := -dy*w/2, dx*w/2
	b := uint32(len(g.ov))
	g.ovVert(ax+nx, ay+ny, c)
	g.ovVert(ax-nx, ay-ny, c)
	g.ovVert(bx-nx, by-ny, c)
	g.ovVert(bx+nx, by+ny, c)
	g.oi = append(g.oi, b, b+1, b+2, b, b+2, b+3)
}

// ovLine draws a world-space line piece when both ends face the eye.
func (g *globeView) ovLine(a, b *[3]float32, w float32, c ovColour) {
	ax, ay, az := g.worldPt(a)
	bx, by, bz := g.worldPt(b)
	lim := g.zh + 0.01
	if az < lim || bz < lim {
		return
	}
	g.ovQuad(ax, ay, bx, by, w, c)
}

func globeLerp3(a, b *[3]float32, t float32) [3]float32 {
	return [3]float32{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// ovOutline strokes tile i's border.
func (g *globeView) ovOutline(i int, w float32, c ovColour) {
	o0, o1 := g.p.CornerOff[i], g.p.CornerOff[i+1]
	n := int(o1 - o0)
	for k := 0; k < n; k++ {
		ax, ay, _, _ := g.corner(g.cornerID[int(o0)+k])
		bx, by, _, _ := g.corner(g.cornerID[int(o0)+(k+1)%n])
		g.ovQuad(ax, ay, bx, by, w, c)
	}
}

// ovFill shades tile i's polygon.
func (g *globeView) ovFill(i int, c ovColour) {
	o0, o1 := g.p.CornerOff[i], g.p.CornerOff[i+1]
	n := int(o1 - o0)
	if len(g.ov)+n+1 > g.ovMax {
		return
	}
	b := uint32(len(g.ov))
	cx, cy, _ := g.worldPt(&g.p.Center[i])
	g.ovVert(cx, cy, c)
	for k := 0; k < n; k++ {
		x, y, _, _ := g.corner(g.cornerID[int(o0)+k])
		g.ovVert(x, y, c)
		g.oi = append(g.oi, b, b+1+uint32(k), b+1+uint32((k+1)%n))
	}
}

func (g *globeView) tileFacing(i int) bool {
	c, cd := &g.p.Center[i], &g.camDir
	return c[0]*cd[0]+c[1]*cd[1]+c[2]*cd[2] >= g.zh+0.02
}

// buildOverlay adds rivers, roads and the outlines on top of the tiles.
func (g *globeView) buildOverlay() {
	g.ov, g.oi = g.ov[:0], g.oi[:0]
	scale := float32(globeClamp(g.tilePx()/14, 0.6, 3))
	river := ovFrom(worldPalette["river"], 1)
	if scale >= 0.75 {
		for k := range g.rivers {
			s := &g.rivers[k]
			g.ovLine(&s.a, &s.b, (0.7+0.9*float32(s.w))*scale, river)
		}
	}
	road := ovFrom(worldPalette["road"], 1)
	casing := ovColour{0.06, 0.05, 0.04, 0.55}
	for pass := 0; pass < 2; pass++ { // a dark casing under the dashes keeps them readable on sand
		w, c := 2.9*scale, casing
		if pass == 1 {
			w, c = 1.5*scale, road
		}
		for k := range g.roads {
			s := &g.roads[k]
			for _, d := range [2][2]float32{{0.08, 0.46}, {0.54, 0.92}} {
				a, b := globeLerp3(&s.a, &s.b, d[0]), globeLerp3(&s.a, &s.b, d[1])
				g.ovLine(&a, &b, w, c)
			}
		}
	}
	if g.hlLeft > 0 && len(g.hl) > 0 {
		a := float32(0.35)
		if !g.reduced {
			a = 0.28 + 0.2*float32(math.Sin(float64(g.tick)*0.2))
		}
		warm := ovColour{1, 0.86, 0.45, a}
		edge := ovColour{1, 0.9, 0.55, 0.9}
		for _, t := range g.hl {
			if t >= 0 && t < g.p.TileCount() && g.tileFacing(t) {
				g.ovFill(t, warm)
				g.ovOutline(t, 1.6, edge)
			}
		}
	}
	if h := g.hovered; h >= 0 && h != g.selected && g.tileFacing(h) {
		g.ovOutline(h, 1.6, ovColour{0.95, 0.94, 0.9, 0.9})
	}
	if s := g.selected; s >= 0 && g.tileFacing(s) {
		g.ovOutline(s, 4.2, ovColour{0.05, 0.04, 0.03, 0.55})
		g.ovOutline(s, 2.4, ovColour{0.91, 0.75, 0.44, 1})
	}
}

// --- drawing -------------------------------------------------------------

func (g *globeView) ensureGPU(sw, sh int) {
	if g.white == nil {
		g.white = ebiten.NewImage(3, 3)
		g.white.Fill(color.White)
		g.whiteSub = g.white.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
		g.opts = ebiten.DrawTrianglesOptions{}
		g.sphere = ebiten.NewImageFromImage(bakeSphere(globeSphereBakeSize, globeSphereBakeR, palRGB("uncovered"), palRGB("atmosphere_rim")))
	}
	if g.stars == nil || g.starW != sw || g.starH != sh {
		g.stars = ebiten.NewImageFromImage(bakeStarfield(g.p.Seed, sw, sh, 1, 0))
		g.starW, g.starH = sw, sh
	}
}

// Draw renders the globe over a starfield into dst (sw x sh).
func (g *globeView) Draw(dst *ebiten.Image, sw, sh int) {
	g.sw, g.sh = sw, sh
	g.ensureGPU(sw, sh)
	dst.Fill(uiSpace)
	dst.DrawImage(g.stars, nil)
	g.drawBody(dst)
}

// drawBody draws the sphere, tiles, overlays and emblems into dst without
// a background (g.ensureGPU must have run).
func (g *globeView) drawBody(dst *ebiten.Image) {
	vp := g.viewport()
	r := globemath.SilhouetteRadius(vp, g.cam.Dist)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-globeSphereBakeSize/2, -globeSphereBakeSize/2)
	op.GeoM.Scale(r/globeSphereBakeR, r/globeSphereBakeR)
	op.GeoM.Translate(vp.CX, vp.CY)
	dst.DrawImage(g.sphere, op)

	g.build()
	dst.DrawTriangles32(g.verts[:g.nV], g.idx[:g.nI], g.whiteSub, &g.opts)
	if len(g.oi) > 0 {
		dst.DrawTriangles32(g.ov, g.oi, g.whiteSub, &g.opts)
	}
	g.drawMarks(dst)
}

// drawMarks draws the settlement emblems (scaled with zoom) and, when
// zoomed in, their names.
func (g *globeView) drawMarks(dst *ebiten.Image) {
	size := globeClamp(g.tilePx()*2, 12, 46)
	labels := g.cam.Dist <= 1.9
	for k := range g.marks {
		m := &g.marks[k]
		c := &g.p.Center[m.tile]
		sx, sy, vz := g.worldPt(c)
		if vz < g.zh+0.04 {
			continue
		}
		x, y := float64(sx), float64(sy)
		vector.FillCircle(dst, sx, sy, float32(size*0.56), color.RGBA{10, 14, 20, 150}, true)
		if img := g.art.image(m.key, color.NRGBA{}); img != nil {
			// The glyphs are white: the faction accent is a colour scale.
			b := img.Bounds()
			s := size / float64(b.Dx())
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.ColorScale.ScaleWithColor(m.tint)
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(x-size/2, y-float64(b.Dy())*s/2)
			dst.DrawImage(img, op)
		} else {
			vector.FillCircle(dst, sx, sy, float32(size*0.32), color.RGBA{m.tint.R, m.tint.G, m.tint.B, 255}, true)
		}
		if labels && m.label != "" {
			st := tsSmall
			st.Align = alignCenter
			st.Color = color.RGBA{10, 14, 20, 230}
			drawText(dst, m.label, x+1, y+size*0.58+1, st)
			st.Color = uiText
			drawText(dst, m.label, x, y+size*0.58, st)
		}
	}
}

// --- baked images --------------------------------------------------------

// bakeSphere renders the sphere's backdrop in view space: a shaded disc of
// the uncovered colour (radius r px) and a soft atmosphere halo around it,
// in a size x size image.
func bakeSphere(size, r int, uncovered, rimC rgb) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	c := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := (float64(x)+0.5-c)/float64(r), -(float64(y)+0.5-c)/float64(r)
			rr := math.Hypot(dx, dy)
			var col rgb
			var a float64
			switch {
			case rr <= 1:
				nz := math.Sqrt(math.Max(0, 1-rr*rr))
				k1, k2 := globeShade(float32(dx), float32(dy), float32(nz))
				col = rgb{
					uncovered[0]*k1 + rimC[0]*0.85*k2,
					uncovered[1]*k1 + rimC[1]*0.85*k2,
					uncovered[2]*k1 + rimC[2]*0.85*k2,
				}
				a = clamp01((1 - rr) * float64(r) / 1.5) // anti-aliased edge
			case rr < 1.28:
				t := 1 - (rr-1)/0.28
				a = 0.6 * t * t * t
				col = rimC
			default:
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(clampF32(col[0], 0, 1)*255 + 0.5),
				G: uint8(clampF32(col[1], 0, 1)*255 + 0.5),
				B: uint8(clampF32(col[2], 0, 1)*255 + 0.5),
				A: uint8(a*255 + 0.5),
			})
		}
	}
	return img
}

// bakeStarfield scatters a starfield for seed over a w x h image
// (transparent between the stars). Positions come from the seed in unit
// coordinates, so a resize keeps the same sky. strength scales brightness;
// skyFade (0..1) dims the stars towards the bottom edge.
func bakeStarfield(seed uint64, w, h int, strength, skyFade float64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	n := w * h / 2600
	for i := 0; i < n; i++ {
		hs := mix64(seed ^ uint64(i+1)*0x9E3779B97F4A7C15)
		fx, fy := hashUnit(hs), hashUnit(mix64(hs))
		b := hashUnit(mix64(hs ^ 0x51))
		tone := hashUnit(mix64(hs ^ 0xA7))
		bright := (0.25 + 0.75*b*b*b) * strength * (1 - skyFade*fy)
		c := [3]float64{0.85 + 0.15*tone, 0.88, 1 - 0.18*tone}
		if tone > 0.8 {
			c = [3]float64{1, 0.9, 0.7}
		}
		x, y := int(fx*float64(w)), int(fy*float64(h))
		plot := func(px, py int, k float64) {
			if px < 0 || py < 0 || px >= w || py >= h {
				return
			}
			o := img.NRGBAAt(px, py)
			a := math.Min(1, float64(o.A)/255+bright*k)
			img.SetNRGBA(px, py, color.NRGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), uint8(a*255 + 0.5)})
		}
		plot(x, y, 1)
		if b > 0.93 { // the few bright ones get a small cross
			plot(x-1, y, 0.45)
			plot(x+1, y, 0.45)
			plot(x, y-1, 0.45)
			plot(x, y+1, 0.45)
		}
	}
	return img
}
