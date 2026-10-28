// Package globe is the New Colony wizard's planet view: the baked sphere,
// starfield, view modes and hit testing. It is built on globemath, gfx and
// art and never changes the planet it reads.
package globe

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
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

// Mode is the landing screen's view mode (recolour only).
type Mode uint8

const (
	ModeBiome Mode = iota
	ModeTemperature
	ModeRainfall
	ModeElevation
	NumModes
)

func (m Mode) String() string {
	switch m {
	case ModeTemperature:
		return "Temperature"
	case ModeRainfall:
		return "Rainfall"
	case ModeElevation:
		return "Elevation"
	}
	return "Biome"
}

// Pointer results of Update: no tile, or the uncovered part of the sphere
// (a click there selects nothing; the screen says "Not part of this world").
const (
	ModeNone      = -1
	ModeUncovered = -2
)

const (
	globeDragPx      = 4  // a click moves less than this between press and release
	DoubleFrames     = 24 // frames between the clicks of a double-click
	globeDoublePx    = 6
	CullSlack        = 0.08 // tiles this far past the horizon are still drawn
	globeCornerShade = 0.94 // tile corners are slightly darker than the centre: a bevel
	FlyFrames        = 24
	globeTilePx1     = 0.0265 // a tile's angular spacing, radians
	SphereBakeSize   = 640    // baked sphere image (halo + shaded disc), px
	SphereBakeR      = 256    // the disc's radius in that image
)

// WorldPalette mirrors assets/palettes/world.yaml (without the "world."
// prefix); TestGlobePaletteMatchesYAML keeps the two equal.
var WorldPalette = map[string]color.RGBA{
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

func PalRGB(key string) rgb { return toRGB(WorldPalette[key]) }

func MixRGB(a, b rgb, t float32) rgb {
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

// HillMix is how far a tile's colour moves towards the mountain tint
// (NG-1 §1.2): flat 0, small hills 12 %, large hills 25 %, mountainous 40 %.
var HillMix = [...]float32{site.HillsDefault: 0, site.Flat: 0, site.SmallHills: 0.12, site.LargeHills: 0.25, site.Mountainous: 0.40}

// ColourStop is a stop of a data-view colour Ramp.
type ColourStop struct {
	At float32
	c  rgb
}

func stop(at float32, v uint32) ColourStop { return ColourStop{at, toRGB(hex6(v))} }

func Ramp(stops []ColourStop, v float32) rgb {
	if v <= stops[0].At {
		return stops[0].c
	}
	for i := 1; i < len(stops); i++ {
		if v <= stops[i].At {
			a, b := stops[i-1], stops[i]
			return MixRGB(a.c, b.c, (v-a.At)/(b.At-a.At))
		}
	}
	return stops[len(stops)-1].c
}

var (
	TempRamp = []ColourStop{stop(-30, 0x25318F), stop(-15, 0x4A7BC8), stop(-2, 0xA9D6E8),
		stop(8, 0xA8D58A), stop(18, 0xE6D96A), stop(26, 0xEE9A3E), stop(34, 0xC8412E)}
	RainRamp = []ColourStop{stop(0, 0xC9A66B), stop(300, 0xD8C98A), stop(800, 0x9CC47A),
		stop(1500, 0x3F9A5E), stop(2400, 0x2B6FA8), stop(3500, 0x1B3F8A)}
	ElevRamp = []ColourStop{stop(0, 0x3E7A45), stop(300, 0x7FA050), stop(800, 0xB7A862),
		stop(1600, 0x9C7B55), stop(3000, 0x8A7F78), stop(4500, 0xF0F0F0)}
)

// Accent is a faction's colour on the globe (emblem tint).
func Accent(t faction.Type) color.NRGBA {
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

// Shade is the Lambert factor k1 for a unit view-space normal and the
// atmosphere share k2 (a rim that grows towards the limb): the lit colour
// is base*k1 + rim*k2.
func Shade(vx, vy, vz float32) (k1, k2 float32) {
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
	Tile  int32
	Key   string
	Tint  color.NRGBA
	Label string
}

// Input is one frame of pointer and keyboard state for the globe.
type Input struct {
	W, H    int // screen size
	MX, MY  int
	Wheel   float64
	Down    [inputframe.NumMouse]bool
	Press   [inputframe.NumMouse]bool
	Release [inputframe.NumMouse]bool
	Held    []string // keys down now (input key names)
	Keys    []string // keys that went down this frame
	Shift   bool
	// Blocked: the pointer is over a panel or modal; the globe ignores
	// hover, wheel and new presses (a drag in progress continues).
	Blocked bool
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

// View draws and drives the planet globe.
type View struct {
	P   *planet.Planet
	cat *art.Catalog
	Cam *globemath.Camera

	mode     Mode
	selected int
	hovered  int
	Reduced  bool
	tick     int
	drag     globeDrag
	Hl       []int
	HlLeft   int

	area   [4]float64 // x, y, w, h of the globe's screen area (zero w: whole screen)
	areaPx bool       // area is in physical pixels (SetAreaPx), not logical UI pixels
	Sw, Sh int        // the logical screen (the default area)

	// geometry, built once per planet
	cornerID  []uint32
	cornerPos [][3]float32
	Base      []rgb
	Rivers    []globeSeg
	Roads     []globeSeg
	Marks     []settleMark
	capTiles  int

	// per-frame state (build)
	bf                 uint32
	stamp              []uint32
	csx, csy, ck1, ck2 []float32
	mat                [9]float32
	CamDir             [3]float32
	Zh, limbR          float32
	cx, cy, focal      float32
	dist               float32
	Verts              []ebiten.Vertex
	Idx                []uint32
	NV, NI             int
	Ov                 []ebiten.Vertex
	Oi                 []uint32

	// GPU resources, created on the first Draw
	white    *ebiten.Image
	whiteSub *ebiten.Image
	sphere   *ebiten.Image
	stars    *ebiten.Image
	StarW    int
	StarH    int
	opts     ebiten.DrawTrianglesOptions
	ovMax    int
	rimColor rgb
}

// New prepares the globe for p. cat supplies the settlement
// emblems and may be nil (circles are drawn instead).
func New(p *planet.Planet, cat *art.Catalog) *View {
	n := p.TileCount()
	g := &View{P: p, cat: cat, Cam: globemath.NewCamera(), selected: -1, hovered: -1}
	g.cornerID, g.cornerPos = DedupeCorners(p.Corners)
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
	g.Verts = make([]ebiten.Vertex, 7*g.capTiles)
	for i := range g.Verts {
		g.Verts[i].SrcX, g.Verts[i].SrcY, g.Verts[i].ColorA = 1.5, 1.5, 1
	}
	g.Idx = make([]uint32, 18*g.capTiles)
	g.ovMax = 4 * (len(p.Roads)*2 + countRiverSegs(p) + 4*1024)
	g.Ov = make([]ebiten.Vertex, 0, g.ovMax)
	g.Oi = make([]uint32, 0, g.ovMax/4*6)
	g.rimColor = scaleRGB(PalRGB("atmosphere_rim"), 0.85)
	g.buildLines()
	g.buildMarks()
	g.recolour()
	g.drag.lastTile = ModeNone
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

// DedupeCorners merges the per-tile corner copies into unique corners (a
// corner is shared by three tiles). The copies differ by float rounding,
// so each is matched against its grid cell and the neighbouring cells it
// lies close to.
func DedupeCorners(corners [][3]float32) (ids []uint32, pos [][3]float32) {
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
					if gfx.Abs32(q[0]-c[0]) < tol && gfx.Abs32(q[1]-c[1]) < tol && gfx.Abs32(q[2]-c[2]) < tol {
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

func norm3(v [3]float32) [3]float32 {
	l := float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
	if l == 0 {
		return v
	}
	return [3]float32{v[0] / l, v[1] / l, v[2] / l}
}

// buildLines turns the planet's rivers and roads into surface segments.
func (g *View) buildLines() {
	p := g.P
	for i := range p.River {
		if p.River[i] == site.NoRiver || p.RiverTo[i] < 0 {
			continue
		}
		a, b := p.Center[i], p.Center[p.RiverTo[i]]
		m := norm3([3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]})
		w := uint8(p.River[i])
		g.Rivers = append(g.Rivers, globeSeg{a, m, w}, globeSeg{m, b, w})
	}
	for _, r := range p.Roads {
		g.Roads = append(g.Roads, globeSeg{p.Center[r[0]], p.Center[r[1]], 1})
	}
}

// buildMarks lists the settlements with their emblem key, tint and label.
func (g *View) buildMarks() {
	p := g.P
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
		g.Marks = append(g.Marks, settleMark{Tile: t, Key: key, Tint: Accent(ft), Label: p.SettlementName(int(t))})
	}
}

// --- colours -----------------------------------------------------------

func OceanColour(elev int16) rgb {
	d := clampF32(float32(-elev)/3500, 0, 1)
	t := 1 - float32(math.Sqrt(float64(d)))
	return MixRGB(PalRGB("ocean_deep"), PalRGB("ocean_shallow"), t)
}

// tileColour is tile i's flat colour in the current view mode.
func (g *View) tileColour(i int) rgb {
	p := g.P
	switch p.Water[i] {
	case planet.Ocean:
		c := OceanColour(p.Elev[i])
		if g.mode != ModeBiome {
			c = scaleRGB(c, 0.55)
		}
		return c
	case planet.Lake:
		c := PalRGB("lake")
		if g.mode != ModeBiome {
			c = scaleRGB(c, 0.7)
		}
		return c
	}
	switch g.mode {
	case ModeTemperature:
		return Ramp(TempRamp, float32(p.Temp[i])/10)
	case ModeRainfall:
		return Ramp(RainRamp, float32(p.Rain[i]))
	case ModeElevation:
		return Ramp(ElevRamp, float32(p.Elev[i]))
	}
	c := PalRGB(biome.DefOf(p.Biome[i]).Key)
	h := int(p.Hill[i])
	if h < len(HillMix) {
		c = MixRGB(c, PalRGB("mountain_tint"), HillMix[h])
	}
	// A touch of per-tile variation so the hex mosaic reads.
	j := float32(art.Mix64(uint64(i)*0x9E3779B97F4A7C15+0x77)>>40) / float32(1<<24)
	return scaleRGB(c, 0.97+0.06*j)
}

// recolour recomputes every tile's colour for the current mode.
func (g *View) recolour() {
	n := g.P.TileCount()
	if cap(g.Base) < n {
		g.Base = make([]rgb, n)
	}
	g.Base = g.Base[:n]
	for i := 0; i < n; i++ {
		g.Base[i] = g.tileColour(i)
	}
}

// SetMode switches the view mode and recolours the tiles.
func (g *View) SetMode(m Mode) {
	if m >= NumModes || m == g.mode {
		return
	}
	g.mode = m
	g.recolour()
}

// Mode is the current view mode.
func (g *View) Mode() Mode { return g.mode }

// --- state and API for the screen ---------------------------------------

// Select marks tile i as the selection (-1 clears).
func (g *View) Select(i int) {
	if i < 0 || i >= g.P.TileCount() {
		i = -1
	}
	g.selected = i
}

// Selected is the selected tile, -1 when none.
func (g *View) Selected() int { return g.selected }

// Hovered is the tile under the pointer after the last Update, -1 when none.
func (g *View) Hovered() int { return g.hovered }

// SetHovered marks tile i as hovered (-1 clears it); the pointer does it in
// Update, the screenshot capture does it by hand.
func (g *View) SetHovered(i int) { g.hovered = i }

// Center flies the view to tile i (instantly with Reduced Motion).
func (g *View) Center(i int) {
	if i < 0 || i >= g.P.TileCount() {
		return
	}
	c := g.P.Center[i]
	frames := FlyFrames
	if g.Reduced {
		frames = 0
	}
	g.Cam.PanX, g.Cam.PanY = 0, 0
	g.Cam.CentreOn(globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])}, frames)
}

// Highlight pulses the given tiles for the next frames Draw calls (the
// pulse is steady with Reduced Motion).
func (g *View) Highlight(tiles []int, frames int) {
	g.Hl = append(g.Hl[:0], tiles...)
	g.HlLeft = frames
}

// SetArea sets the screen rectangle the globe is centred in, in logical
// UI pixels (default: the whole logical screen). Call it when the layout
// changes, e.g. to leave room for a side panel.
//
// The globe itself works in physical pixels (PLAY-012 B.4): it is drawn at
// physical size inside the rectangle x the UI scale, and picks and drags
// with the raw (physical) cursor, so it stays sharp at any scale.
func (g *View) SetArea(x, y, w, h float64) {
	g.area, g.areaPx = [4]float64{x, y, w, h}, false
}

// SetAreaPx is SetArea in physical pixels, unscaled: for a globe rendered
// into its own image (the setup backdrop's disc).
func (g *View) SetAreaPx(x, y, w, h float64) {
	g.area, g.areaPx = [4]float64{x, y, w, h}, true
}

// k is the factor from the area's units to physical pixels.
func (g *View) k() float64 {
	if g.areaPx {
		return 1
	}
	return gfx.UIScale()
}

// AreaRect is the globe's area in physical pixels.
func (g *View) AreaRect() (x, y, w, h float64) {
	k := g.k()
	if g.area[2] > 0 && g.area[3] > 0 {
		return g.area[0] * k, g.area[1] * k, g.area[2] * k, g.area[3] * k
	}
	return 0, 0, float64(g.Sw) * k, float64(g.Sh) * k
}

func (g *View) Viewport() globemath.Viewport {
	x, y, w, h := g.AreaRect()
	return g.Cam.Viewport(x, y, w, h)
}

// PickTile is the tile under a physical screen point (the raw cursor): a
// tile index, ModeUncovered
// over the part of the sphere without tiles, or ModeNone off the globe.
func (g *View) PickTile(mx, my int) int {
	w, ok := g.Cam.Pick(g.Viewport(), float64(mx), float64(my))
	if !ok {
		return ModeNone
	}
	t := g.P.TileAt([3]float32{float32(w[0]), float32(w[1]), float32(w[2])})
	if t < 0 {
		return ModeUncovered
	}
	return t
}

// TilePx is roughly how many physical pixels one tile spans at the view
// centre.
func (g *View) TilePx() float64 {
	vp := g.Viewport()
	return globeTilePx1 * vp.F / (g.Cam.Dist - 1)
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
// selected this frame (ModeNone if none; ModeUncovered for a click on the
// uncovered sphere, which clears the selection) and the tile under the
// pointer (ModeNone, or ModeUncovered).
//
//	left click (release within 4 px)  select; a second click on the same
//	                                  tile within 24 frames also centres it
//	left drag                         rotate, with inertia (not with Reduced Motion)
//	middle drag                       pan the globe on screen
//	wheel, PageUp/PageDown, +/-       zoom (1.25..4.0 radii)
//	WASD / arrows                     rotate (Shift 3x)
//	right click                       clear the selection
//	Home                              centre on the selection (and reset the pan)
func (g *View) Update(in Input, reducedMotion bool) (clicked, hovered int) {
	g.tick++
	g.Reduced = reducedMotion
	if in.W > 0 && in.H > 0 {
		g.Sw, g.Sh = in.W, in.H
	}
	cam := g.Cam
	clicked = ModeNone
	if g.HlLeft > 0 {
		g.HlLeft--
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
	vp := g.Viewport()
	if in.Press[inputframe.Left] && !in.Blocked && !d.on {
		*d = globeDrag{on: true, sx: in.MX, sy: in.MY, lx: in.MX, ly: in.MY,
			lastClickT: d.lastClickT, lastClickX: d.lastClickX, lastClickY: d.lastClickY, lastTile: d.lastTile}
		cam.StopSpin()
	}
	if d.on {
		if in.Down[inputframe.Left] {
			if dp := globeDragPx * g.k(); !d.moved && float64(sqr(in.MX-d.sx)+sqr(in.MY-d.sy)) > dp*dp {
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
		if in.Release[inputframe.Left] || !in.Down[inputframe.Left] && !in.Press[inputframe.Left] {
			if d.moved {
				cam.Fling(d.vx, d.vy, vp, reducedMotion)
			} else {
				clicked = g.click(in.MX, in.MY)
			}
			d.on, d.moved, d.vx, d.vy = false, false, 0, 0
		}
	}

	// Middle button: pan.
	if in.Press[inputframe.Middle] && !in.Blocked {
		d.panning = true
		d.plx, d.ply = in.MX, in.MY
	}
	if d.panning {
		if in.Down[inputframe.Middle] {
			cam.PanX += float64(in.MX - d.plx)
			cam.PanY += float64(in.MY - d.ply)
			_, _, w, h := g.AreaRect()
			cam.ClampPan(w*0.8, h*0.8)
			d.plx, d.ply = in.MX, in.MY
		} else {
			d.panning = false
		}
	}

	// Right button: clear.
	if in.Press[inputframe.Right] && !in.Blocked {
		g.selected = -1
	}

	cam.Step(reducedMotion)

	g.hovered = ModeNone
	if !in.Blocked && !d.moved && !d.panning {
		g.hovered = g.PickTile(in.MX, in.MY)
	}
	return clicked, g.hovered
}

func Clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func sqr(v int) int { return v * v }

// click selects the tile under (mx, my); a double-click also centres it.
func (g *View) click(mx, my int) int {
	d := &g.drag
	t := g.PickTile(mx, my)
	if t < 0 {
		if t == ModeUncovered {
			g.selected = -1
		}
		d.lastTile = ModeNone
		return t
	}
	g.selected = t
	if d.lastTile == t && g.tick-d.lastClickT <= DoubleFrames &&
		float64(sqr(mx-d.lastClickX)+sqr(my-d.lastClickY)) <= globeDoublePx*globeDoublePx*g.k()*g.k() {
		g.Center(t)
		d.lastTile = ModeNone
	} else {
		d.lastTile, d.lastClickT, d.lastClickX, d.lastClickY = t, g.tick, mx, my
	}
	return t
}

// --- per-frame mesh ------------------------------------------------------

// setupFrame loads the camera into the per-frame fields.
func (g *View) setupFrame() {
	m := g.Cam.Orientation().Matrix()
	for i, v := range m {
		g.mat[i] = float32(v)
	}
	c := g.Cam.Centre()
	g.CamDir = [3]float32{float32(c[0]), float32(c[1]), float32(c[2])}
	g.dist = float32(g.Cam.Dist)
	g.Zh = float32(globemath.HorizonZ(g.Cam.Dist))
	g.limbR = float32(math.Sqrt(float64(1 - g.Zh*g.Zh)))
	vp := g.Viewport()
	g.cx, g.cy, g.focal = float32(vp.CX), float32(vp.CY), float32(vp.F)
	g.bf++
}

// view rotates a world vector into view space.
func (g *View) view(p *[3]float32) (vx, vy, vz float32) {
	m := &g.mat
	return m[0]*p[0] + m[1]*p[1] + m[2]*p[2],
		m[3]*p[0] + m[4]*p[1] + m[5]*p[2],
		m[6]*p[0] + m[7]*p[1] + m[8]*p[2]
}

// project maps a view-space point to the screen; points past the horizon
// are pushed onto the limb so a tile that straddles it keeps a sane shape.
func (g *View) project(vx, vy, vz float32) (sx, sy float32) {
	if vz < g.Zh {
		r := float32(math.Hypot(float64(vx), float64(vy)))
		if r < 1e-6 {
			r = 1e-6
		}
		s := g.limbR / r
		vx, vy, vz = vx*s, vy*s, g.Zh
	}
	k := g.focal / (g.dist - vz)
	return g.cx + vx*k, g.cy - vy*k
}

// corner makes sure unique corner id has this frame's screen position and
// light, and returns them.
func (g *View) corner(id uint32) (sx, sy, k1, k2 float32) {
	if g.stamp[id] != g.bf {
		vx, vy, vz := g.view(&g.cornerPos[id])
		g.csx[id], g.csy[id] = g.project(vx, vy, vz)
		g.ck1[id], g.ck2[id] = Shade(vx, vy, vz)
		g.stamp[id] = g.bf
	}
	return g.csx[id], g.csy[id], g.ck1[id], g.ck2[id]
}

// Build fills the tile and overlay meshes for the current camera. It
// allocates nothing once the buffers exist.
func (g *View) Build() {
	g.setupFrame()
	p := g.P
	thresh := g.Zh - CullSlack
	cd := &g.CamDir
	rim := g.rimColor
	nv, ni := 0, 0
	verts, idx := g.Verts, g.Idx
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
		base := g.Base[i]

		vx, vy, vz := g.view(c)
		sx, sy := g.project(vx, vy, vz)
		k1, k2 := Shade(vx, vy, vz)
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
	g.NV, g.NI = nv, ni
	g.buildOverlay()
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// WorldPt projects a world point: screen position and view-space z.
func (g *View) WorldPt(p *[3]float32) (sx, sy, vz float32) {
	vx, vy, vz := g.view(p)
	sx, sy = g.project(vx, vy, vz)
	return sx, sy, vz
}

// ovColour is a straight-alpha colour for the overlay mesh.
type ovColour struct{ r, g, b, a float32 }

func ovFrom(c color.RGBA, a float32) ovColour {
	return ovColour{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, a}
}

func (g *View) ovVert(x, y float32, c ovColour) {
	g.Ov = append(g.Ov, ebiten.Vertex{DstX: x, DstY: y, SrcX: 1.5, SrcY: 1.5,
		ColorR: c.r, ColorG: c.g, ColorB: c.b, ColorA: c.a})
}

// ovQuad adds a w-wide line from (ax, ay) to (bx, by), its ends extended
// by a third of the width so consecutive pieces overlap at bends.
func (g *View) ovQuad(ax, ay, bx, by, w float32, c ovColour) {
	if len(g.Ov)+4 > g.ovMax {
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
	b := uint32(len(g.Ov))
	g.ovVert(ax+nx, ay+ny, c)
	g.ovVert(ax-nx, ay-ny, c)
	g.ovVert(bx-nx, by-ny, c)
	g.ovVert(bx+nx, by+ny, c)
	g.Oi = append(g.Oi, b, b+1, b+2, b, b+2, b+3)
}

// ovLine draws a world-space line piece when both ends face the eye.
func (g *View) ovLine(a, b *[3]float32, w float32, c ovColour) {
	ax, ay, az := g.WorldPt(a)
	bx, by, bz := g.WorldPt(b)
	lim := g.Zh + 0.01
	if az < lim || bz < lim {
		return
	}
	g.ovQuad(ax, ay, bx, by, w, c)
}

func globeLerp3(a, b *[3]float32, t float32) [3]float32 {
	return [3]float32{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// ovOutline strokes tile i's border.
func (g *View) ovOutline(i int, w float32, c ovColour) {
	o0, o1 := g.P.CornerOff[i], g.P.CornerOff[i+1]
	n := int(o1 - o0)
	for k := 0; k < n; k++ {
		ax, ay, _, _ := g.corner(g.cornerID[int(o0)+k])
		bx, by, _, _ := g.corner(g.cornerID[int(o0)+(k+1)%n])
		g.ovQuad(ax, ay, bx, by, w, c)
	}
}

// ovFill shades tile i's polygon.
func (g *View) ovFill(i int, c ovColour) {
	o0, o1 := g.P.CornerOff[i], g.P.CornerOff[i+1]
	n := int(o1 - o0)
	if len(g.Ov)+n+1 > g.ovMax {
		return
	}
	b := uint32(len(g.Ov))
	cx, cy, _ := g.WorldPt(&g.P.Center[i])
	g.ovVert(cx, cy, c)
	for k := 0; k < n; k++ {
		x, y, _, _ := g.corner(g.cornerID[int(o0)+k])
		g.ovVert(x, y, c)
		g.Oi = append(g.Oi, b, b+1+uint32(k), b+1+uint32((k+1)%n))
	}
}

func (g *View) tileFacing(i int) bool {
	c, cd := &g.P.Center[i], &g.CamDir
	return c[0]*cd[0]+c[1]*cd[1]+c[2]*cd[2] >= g.Zh+0.02
}

// buildOverlay adds rivers, roads and the outlines on top of the tiles.
func (g *View) buildOverlay() {
	g.Ov, g.Oi = g.Ov[:0], g.Oi[:0]
	lk := float32(g.k()) // line widths are in area units: physical x lk
	zs := float32(Clamp(g.TilePx()/(14*float64(lk)), 0.6, 3))
	scale := zs * lk
	river := ovFrom(WorldPalette["river"], 1)
	if zs >= 0.75 {
		for k := range g.Rivers {
			s := &g.Rivers[k]
			g.ovLine(&s.a, &s.b, (0.7+0.9*float32(s.w))*scale, river)
		}
	}
	road := ovFrom(WorldPalette["road"], 1)
	casing := ovColour{0.06, 0.05, 0.04, 0.55}
	for pass := 0; pass < 2; pass++ { // a dark casing under the dashes keeps them readable on sand
		w, c := 2.9*scale, casing
		if pass == 1 {
			w, c = 1.5*scale, road
		}
		for k := range g.Roads {
			s := &g.Roads[k]
			for _, d := range [2][2]float32{{0.08, 0.46}, {0.54, 0.92}} {
				a, b := globeLerp3(&s.a, &s.b, d[0]), globeLerp3(&s.a, &s.b, d[1])
				g.ovLine(&a, &b, w, c)
			}
		}
	}
	if g.HlLeft > 0 && len(g.Hl) > 0 {
		a := float32(0.35)
		if !g.Reduced {
			a = 0.28 + 0.2*float32(math.Sin(float64(g.tick)*0.2))
		}
		warm := ovColour{1, 0.86, 0.45, a}
		edge := ovColour{1, 0.9, 0.55, 0.9}
		for _, t := range g.Hl {
			if t >= 0 && t < g.P.TileCount() && g.tileFacing(t) {
				g.ovFill(t, warm)
				g.ovOutline(t, 1.6*lk, edge)
			}
		}
	}
	if h := g.hovered; h >= 0 && h != g.selected && g.tileFacing(h) {
		g.ovOutline(h, 1.6*lk, ovColour{0.95, 0.94, 0.9, 0.9})
	}
	if s := g.selected; s >= 0 && g.tileFacing(s) {
		g.ovOutline(s, 4.2*lk, ovColour{0.05, 0.04, 0.03, 0.55})
		g.ovOutline(s, 2.4*lk, ovColour{0.91, 0.75, 0.44, 1})
	}
}

// --- drawing -------------------------------------------------------------

func (g *View) EnsureGPU(sw, sh int) {
	if g.white == nil {
		g.white = ebiten.NewImage(3, 3)
		g.white.Fill(color.White)
		g.whiteSub = g.white.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
		g.opts = ebiten.DrawTrianglesOptions{}
		g.sphere = ebiten.NewImageFromImage(BakeSphere(SphereBakeSize, SphereBakeR, PalRGB("uncovered"), PalRGB("atmosphere_rim")))
	}
	if g.stars == nil || g.StarW != sw || g.StarH != sh {
		g.stars = ebiten.NewImageFromImage(BakeStarfield(g.P.Seed, sw, sh, 1, 0))
		g.StarW, g.StarH = sw, sh
	}
}

// Draw renders the globe over a starfield into dst (the logical screen
// sw x sh; dst and the starfield are physical).
func (g *View) Draw(dst *ebiten.Image, sw, sh int) {
	g.Sw, g.Sh = sw, sh
	if b := dst.Bounds(); b.Dx() > 0 && b.Dy() > 0 {
		g.EnsureGPU(b.Dx(), b.Dy())
	} else {
		g.EnsureGPU(sw, sh)
	}
	dst.Fill(spaceColour)
	dst.DrawImage(g.stars, nil)
	g.DrawBody(dst)
}

// DrawBody draws the sphere, tiles, overlays and emblems into dst without
// a background (g.ensureGPU must have run).
func (g *View) DrawBody(dst *ebiten.Image) {
	vp := g.Viewport()
	r := globemath.SilhouetteRadius(vp, g.Cam.Dist)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-SphereBakeSize/2, -SphereBakeSize/2)
	op.GeoM.Scale(r/SphereBakeR, r/SphereBakeR)
	op.GeoM.Translate(vp.CX, vp.CY)
	dst.DrawImage(g.sphere, op)

	g.Build()
	dst.DrawTriangles32(g.Verts[:g.NV], g.Idx[:g.NI], g.whiteSub, &g.opts)
	if len(g.Oi) > 0 {
		dst.DrawTriangles32(g.Ov, g.Oi, g.whiteSub, &g.opts)
	}
	g.drawMarks(dst)
}

// drawMarks draws the settlement emblems (scaled with zoom) and, when
// zoomed in, their names. Positions and sizes are physical; the names go
// through the canvas's text path (logical).
func (g *View) drawMarks(dst *ebiten.Image) {
	pk := g.k()
	size := Clamp(g.TilePx()*2, 12*pk, 46*pk)
	ts := gfx.UIScale() // drawText takes logical coordinates
	labels := g.Cam.Dist <= 1.9
	for k := range g.Marks {
		m := &g.Marks[k]
		c := &g.P.Center[m.Tile]
		sx, sy, vz := g.WorldPt(c)
		if vz < g.Zh+0.04 {
			continue
		}
		x, y := float64(sx), float64(sy)
		vector.FillCircle(dst, sx, sy, float32(size*0.56), color.RGBA{10, 14, 20, 150}, true)
		if img := g.cat.Image(m.Key, color.NRGBA{}); img != nil {
			// The glyphs are white: the faction accent is a colour scale.
			b := img.Bounds()
			s := size / float64(b.Dx())
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
			op.ColorScale.ScaleWithColor(m.Tint)
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(x-size/2, y-float64(b.Dy())*s/2)
			dst.DrawImage(img, op)
		} else {
			vector.FillCircle(dst, sx, sy, float32(size*0.32), color.RGBA{m.Tint.R, m.Tint.G, m.Tint.B, 255}, true)
		}
		if labels && m.Label != "" {
			st := gfx.Small
			st.Align = gfx.AlignCenter
			st.Color = color.RGBA{10, 14, 20, 230}
			gfx.Text(dst, m.Label, (x+pk)/ts, (y+size*0.58+pk)/ts, st)
			st.Color = gfx.TextColor
			gfx.Text(dst, m.Label, x/ts, (y+size*0.58)/ts, st)
		}
	}
}

// --- baked images --------------------------------------------------------

// BakeSphere renders the sphere's backdrop in view space: a shaded disc of
// the uncovered colour (radius r px) and a soft atmosphere halo around it,
// in a size x size image.
func BakeSphere(size, r int, uncovered, rimC rgb) *image.NRGBA {
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
				k1, k2 := Shade(float32(dx), float32(dy), float32(nz))
				col = rgb{
					uncovered[0]*k1 + rimC[0]*0.85*k2,
					uncovered[1]*k1 + rimC[1]*0.85*k2,
					uncovered[2]*k1 + rimC[2]*0.85*k2,
				}
				a = gfx.Clamp01((1 - rr) * float64(r) / 1.5) // anti-aliased edge
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

// BakeStarfield scatters a starfield for seed over a w x h image
// (transparent between the stars). Positions come from the seed in unit
// coordinates, so a resize keeps the same sky. strength scales brightness;
// skyFade (0..1) dims the stars towards the bottom edge.
func BakeStarfield(seed uint64, w, h int, strength, skyFade float64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	n := w * h / 2600
	for i := 0; i < n; i++ {
		hs := art.Mix64(seed ^ uint64(i+1)*0x9E3779B97F4A7C15)
		fx, fy := hashUnit(hs), hashUnit(art.Mix64(hs))
		b := hashUnit(art.Mix64(hs ^ 0x51))
		tone := hashUnit(art.Mix64(hs ^ 0xA7))
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

// spaceColour is the backdrop behind the planet (the root's uiSpace).
var spaceColour = color.RGBA{R: 0x0E, G: 0x0F, B: 0x12, A: 0xFF}

// hashUnit maps a hash to [0, 1).
func hashUnit(h uint64) float64 { return float64(h>>11) / (1 << 53) }
