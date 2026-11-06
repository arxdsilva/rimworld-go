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
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// The landing-site globe (NG-1 §2.7, plan §1.2): the planet's hex tiles as
// one faceted relief batch per frame, with coasts, rivers, roads,
// settlement emblems and outlines on top. Presentation only: it reads a
// finished *planet.Planet and never changes it. The relief, face normals
// and lines are built once per planet (relief.go); the vertex colours
// change only with the view mode; each frame projects the tiles facing the
// eye, far to near, and lights every face from a sun turned into planet
// space once.

// Mode is the landing screen's view mode (recolour only).
type Mode uint8

const (
	ModeBiome Mode = iota
	ModeTemperature
	ModeRainfall
	ModeElevation
	ModeHilliness
	NumModes
)

func (m Mode) String() string { return m.Label(lang.English{}) }

// Label is the mode's name in the player's language.
func (m Mode) Label(tr lang.Tr) string {
	switch m {
	case ModeTemperature:
		return tr.T("Temperature")
	case ModeRainfall:
		return tr.T("Rainfall")
	case ModeElevation:
		return tr.T("Elevation")
	case ModeHilliness:
		return tr.T("Hilliness")
	}
	return tr.T("Biome")
}

// Pointer results of Update: no tile, or the uncovered part of the sphere
// (a click there selects nothing; the screen says "Not part of this world").
const (
	ModeNone      = -1
	ModeUncovered = -2
)

const (
	globeDragPx    = 4  // a click moves less than this between press and release
	DoubleFrames   = 24 // frames between the clicks of a double-click
	globeDoublePx  = 6
	CullSlack      = 0.08 // tiles this far past the horizon are still drawn
	FlyFrames      = 24
	globeTilePx1   = 0.0265 // a tile's angular spacing, radians
	SphereBakeSize = 640    // baked sphere image (halo + shaded disc), px
	SphereBakeR    = 256    // the disc's radius in that image
)

// WorldPalette mirrors assets/palettes/world.yaml (without the "world."
// prefix); TestGlobePaletteMatchesYAML keeps the two equal.
var WorldPalette = map[string]color.RGBA{
	"ocean_deep":          hex6(0x0E2A47),
	"ocean_shallow":       hex6(0x1F4E79),
	"lake":                hex6(0x2D6A9A),
	"river":               hex6(0x3D73A0),
	"road_path":           hex6(0xB39C74),
	"road_dirt":           hex6(0x8C7355),
	"road_stone":          hex6(0x9C978C),
	"road_asphalt":        hex6(0x3C3D3F),
	"road_centre":         hex6(0xD6D0B8),
	"label":               hex6(0xE6E0CF),
	"label_ocean":         hex6(0x9FC0DA),
	"uncovered":           hex6(0x121820),
	"atmosphere_rim":      hex6(0x4A7FD4),
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
	"snow":                hex6(0xE2E7EA),
	"rock":                hex6(0x76706A),
	"shallow":             hex6(0x2F6A8C),
	"shore":               hex6(0xCFC6A6),
	"grid":                hex6(0x1C2228),
	"hill_flat":           hex6(0x6E8A5C),
	"hill_small":          hex6(0xA79F66),
	"hill_large":          hex6(0xA87A52),
	"hill_mountain":       hex6(0x6F625C),
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

// HillKey is the palette colour of each hilliness in the Hilliness mode.
var HillKey = [...]string{site.HillsDefault: "hill_flat", site.Flat: "hill_flat", site.SmallHills: "hill_small",
	site.LargeHills: "hill_large", site.Mountainous: "hill_mountain"}

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
// viewer). Build turns it into planet space once per frame and lights each
// relief face by its normal; Shade lights the baked backdrop disc.
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
	s := lightAmbient + lightDiffuse*diff
	rim := rimOf(vz)
	return s * (1 - rim), rim
}

// globeSeg is a river, road or coast piece between two surface points: w
// is the river size or the road class; a river's width runs from wa to wb
// (times the zoom scale).
type globeSeg struct {
	a, b   [3]float32
	w      uint8
	wa, wb float32
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
	geom     *planetGeom
	Base     []rgb // each tile's flat colour in the current mode
	Marks    []settleMark
	capTiles int

	// per-frame state (build)
	cc cornerCache
	xform
	Verts  []ebiten.Vertex
	Idx    []uint32
	NV, NI int
	ovMesh

	// GPU resources, created on the first Draw
	white    *ebiten.Image
	whiteSub *ebiten.Image
	sphere   *ebiten.Image
	stars    *ebiten.Image
	StarW    int
	StarH    int
	opts     ebiten.DrawTrianglesOptions
	rimColor rgb
}

// New prepares the globe for p. cat supplies the settlement
// emblems and may be nil (circles are drawn instead).
func New(p *planet.Planet, cat *art.Catalog) *View {
	n := p.TileCount()
	g := &View{P: p, cat: cat, Cam: globemath.NewCamera(), selected: -1, hovered: -1}
	g.geom = newGeom(p)
	// On the full sphere at most the cap seen from MaxDist (plus the cull
	// slack) faces the eye; a partial world can be seen whole.
	g.capTiles = n
	if p.CapCos <= -0.999 {
		g.capTiles = min(n, int(0.45*float64(n))+64)
	}
	g.cc = newCornerCache(len(g.geom.cornerPos), g.capTiles)
	// Every fan triangle has its own three vertices (faceted light), at most
	// six triangles per tile; the index list is the identity, the vertex
	// order is the draw order.
	g.Verts = make([]ebiten.Vertex, 18*g.capTiles)
	g.Idx = make([]uint32, 18*g.capTiles)
	for i := range g.Verts {
		g.Verts[i].SrcX, g.Verts[i].SrcY, g.Verts[i].ColorA = 1.5, 1.5, 1
		g.Idx[i] = uint32(i)
	}
	geo := g.geom
	g.ovMax = 4 * (len(p.Roads)*6 + len(geo.Rivers) + len(geo.Coast) + len(geo.edges) + 4*1024)
	g.Ov = make([]ebiten.Vertex, 0, g.ovMax)
	g.Oi = make([]uint32, 0, g.ovMax/4*6)
	g.rimColor = scaleRGB(PalRGB("atmosphere_rim"), 0.85)
	g.buildMarks()
	g.recolour()
	g.drag.lastTile = ModeNone
	return g
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
		if coastalShallow(p, i) {
			c = PalRGB("shallow")
		}
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
	if g.mode == ModeHilliness && p.Biome[i] != biome.SeaIce {
		if h := int(p.Hill[i]); h < len(HillKey) {
			c = PalRGB(HillKey[h])
		}
	}
	// A touch of per-tile variation so the hex mosaic reads.
	j := float32(randx.Mix64(uint64(i)*0x9E3779B97F4A7C15+0x77)>>40) / float32(1<<24)
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
	g.geom.vertexColours(g.P, g.Base, g.mode == ModeBiome)
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

// SetGrid shows or hides the tile grid (thin tile edges, drawn once zoomed
// in to Dist 2.2 or closer).
func (g *View) SetGrid(on bool) { g.geom.grid = on }

// Grid reports whether the tile grid is shown.
func (g *View) Grid() bool { return g.geom.grid }

// Bare drops everything but the tiles: rivers, roads, coastline, grid and
// settlement emblems (the far-away backdrop disc).
func (g *View) Bare() {
	g.geom.Rivers, g.geom.Roads, g.geom.Coast, g.geom.grid = nil, nil, nil, false
	g.geom.feats, g.geom.labels = nil, g.geom.labels[:0]
	g.Marks = nil
}

// Labels are the names placed by the last Build (feature and settlement
// labels after the collision pass; drawMarks draws them).
func (g *View) Labels() []Label { return g.geom.labels }

// Surface is tile i's centre on the relief, lifted a little: where markers
// on the globe sit so they never sink into a mountain.
func (g *View) Surface(i int) [3]float32 {
	return scale3(g.geom.centreDisp[i], 1+overlayLift)
}

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

// setupFrame loads the camera into the per-frame transform.
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
	// The view-space sun in planet space: the transpose turns view into world.
	mt := &g.mat
	for j := 0; j < 3; j++ {
		g.sun[j] = mt[j]*globeSun[0] + mt[3+j]*globeSun[1] + mt[6+j]*globeSun[2]
	}
	g.eye = scale3(g.CamDir, g.dist)
	g.cc.bf++
}

// Build fills the tile and overlay meshes for the current camera. It
// allocates nothing once the buffers exist.
//
// Tiles go far to near (a depth bucket sort of the facing tiles), and
// inside a tile the faces turned away from the eye go first, so a peak or
// ridge always covers what lies behind it without a depth buffer.
func (g *View) Build() {
	g.setupFrame()
	p := g.P
	cc := &g.cc
	thresh := g.Zh - CullSlack
	cd := g.CamDir
	kscale := float32(depthBuckets-1) / (1.0001 - thresh)
	n := 0
	for i := range p.Center {
		c := &p.Center[i]
		d := c[0]*cd[0] + c[1]*cd[1] + c[2]*cd[2]
		if d < thresh {
			continue
		}
		if n >= g.capTiles {
			break
		}
		cc.list[n] = int32(i)
		cc.key[n] = uint8((d - thresh) * kscale)
		n++
	}
	nv := 0
	for _, t := range cc.sortFarToNear(n) {
		nv = g.emitTile(int(t), nv)
	}
	g.NV, g.NI = nv, nv
	g.buildOverlay()
	g.geom.layoutLabels(g)
}

// emitTile writes tile i's faces from vertex nv on and returns the next
// free vertex.
func (g *View) emitTile(i, nv int) int {
	p, geo, cc, x := g.P, g.geom, &g.cc, &g.xform
	o0 := p.CornerOff[i]
	n := int(p.CornerOff[i+1] - o0)
	ctr := &geo.centreDisp[i]
	vx, vy, vz := x.view(ctr)
	csx, csy := x.project(vx, vy, vz)
	crim := rimOf(vz / reliefR(geo.centreH[i]))
	ccol := geo.ccol[i]
	rimC := g.rimColor
	sun, eye := &x.sun, &x.eye
	ex, ey, ez := eye[0]-ctr[0], eye[1]-ctr[1], eye[2]-ctr[2]
	var front [6]bool
	for k := 0; k < n; k++ {
		nr := &geo.faceN[int(o0)+k]
		front[k] = nr[0]*ex+nr[1]*ey+nr[2]*ez > 0
	}
	verts := g.Verts
	for pass := 0; pass < 2; pass++ {
		for k := 0; k < n; k++ {
			if front[k] != (pass == 1) {
				continue
			}
			o := int(o0) + k
			nr := &geo.faceN[o]
			diff := nr[0]*sun[0] + nr[1]*sun[1] + nr[2]*sun[2]
			if diff < 0 {
				diff = 0
			}
			s := lightAmbient + lightDiffuse*diff
			k2 := (k + 1) % n
			shadeVert(&verts[nv], csx, csy, ccol, s, crim, rimC)
			ax, ay, ar := cc.at(geo.cornerID[o], x, geo)
			shadeVert(&verts[nv+1], ax, ay, geo.kcol[o], s, ar, rimC)
			bx, by, br := cc.at(geo.cornerID[int(o0)+k2], x, geo)
			shadeVert(&verts[nv+2], bx, by, geo.kcol[int(o0)+k2], s, br, rimC)
			nv += 3
		}
	}
	return nv
}

// shadeVert writes one lit vertex: base x light x (1 - rim) + rim colour x rim.
func shadeVert(v *ebiten.Vertex, sx, sy float32, c rgb, s, rim float32, rimC rgb) {
	k := s * (1 - rim)
	v.DstX, v.DstY = sx, sy
	v.ColorR = min32(c[0]*k+rimC[0]*rim, 1)
	v.ColorG = min32(c[1]*k+rimC[1]*rim, 1)
	v.ColorB = min32(c[2]*k+rimC[2]*rim, 1)
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func ovFrom(c color.RGBA, a float32) ovColour {
	return ovColour{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, a}
}

func globeLerp3(a, b *[3]float32, t float32) [3]float32 {
	return [3]float32{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// liftedCorner is unique corner id on the relief, lifted for an overlay,
// on screen.
func (g *View) liftedCorner(id uint32) (sx, sy float32) {
	q := scale3(g.geom.cornerDisp[id], 1+overlayLift)
	sx, sy, _ = g.WorldPt(&q)
	return sx, sy
}

// ovOutline strokes tile i's border on the relief.
func (g *View) ovOutline(i int, w float32, c ovColour) {
	o0, o1 := g.P.CornerOff[i], g.P.CornerOff[i+1]
	n := int(o1 - o0)
	ids := g.geom.cornerID
	for k := 0; k < n; k++ {
		ax, ay := g.liftedCorner(ids[int(o0)+k])
		bx, by := g.liftedCorner(ids[int(o0)+(k+1)%n])
		g.quad(ax, ay, bx, by, w, c)
	}
}

// ovFill shades tile i's polygon on the relief.
func (g *View) ovFill(i int, c ovColour) {
	o0, o1 := g.P.CornerOff[i], g.P.CornerOff[i+1]
	n := int(o1 - o0)
	if len(g.Ov)+n+1 > g.ovMax {
		return
	}
	b := uint32(len(g.Ov))
	ctr := g.Surface(i)
	cx, cy, _ := g.WorldPt(&ctr)
	g.vert(cx, cy, c)
	for k := 0; k < n; k++ {
		x, y := g.liftedCorner(g.geom.cornerID[int(o0)+k])
		g.vert(x, y, c)
		g.Oi = append(g.Oi, b, b+1+uint32(k), b+1+uint32((k+1)%n))
	}
}

func (g *View) tileFacing(i int) bool {
	c, cd := &g.P.Center[i], &g.CamDir
	return c[0]*cd[0]+c[1]*cd[1]+c[2]*cd[2] >= g.Zh+0.02
}

// gridDist is the camera distance at and below which the grid is drawn.
const gridDist = 2.2

// ovGrid strokes every facing tile edge (the Grid toggle).
func (g *View) ovGrid(w float32) {
	geo, cd := g.geom, &g.CamDir
	c := ovFrom(WorldPalette["grid"], 0.38)
	lim := g.Zh + 0.01
	for _, e := range geo.edges {
		a, b := &geo.cornerPos[e[0]], &geo.cornerPos[e[1]]
		if a[0]*cd[0]+a[1]*cd[1]+a[2]*cd[2] < lim || b[0]*cd[0]+b[1]*cd[1]+b[2]*cd[2] < lim {
			continue
		}
		ax, ay, _ := g.cc.at(e[0], &g.xform, geo)
		bx, by, _ := g.cc.at(e[1], &g.xform, geo)
		g.quad(ax, ay, bx, by, w, c)
	}
}

// buildOverlay adds the grid, coastline, rivers, roads and the outlines on
// top of the tiles.
func (g *View) buildOverlay() {
	g.Ov, g.Oi = g.Ov[:0], g.Oi[:0]
	geo := g.geom
	lk := float32(g.k()) // line widths are in area units: physical x lk
	zs := float32(Clamp(g.TilePx()/(14*float64(lk)), 0.6, 3))
	scale := zs * lk
	if geo.grid && g.Cam.Dist <= gridDist {
		g.ovGrid(0.8 * lk)
	}
	if zs >= riverZoom {
		geo.strokeRivers(&g.xform, &g.ovMesh, zs, scale)
	}
	if zs >= 0.5 { // after the rivers, so the stroke caps a river's mouth
		shore := ovFrom(WorldPalette["shore"], 0.8)
		w := max(0.9*scale, lk)
		for k := range geo.Coast {
			s := &geo.Coast[k]
			g.line(&g.xform, &s.a, &s.b, w, shore)
		}
	}
	if zs >= roadZoom {
		geo.strokeRoads(&g.xform, &g.ovMesh, zs, scale)
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
	size := markSize(g.TilePx(), pk)
	for k := range g.Marks {
		m := &g.Marks[k]
		c := g.Surface(int(m.Tile))
		sx, sy, vz := g.WorldPt(&c)
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
	}
	// Names, as placed by Build's collision pass: a 1 px dark shadow, then
	// the text (settlements in the UI text colour, features in label,
	// oceans in label_ocean).
	ts := gfx.UIScale() // gfx.Text takes logical coordinates
	st := gfx.Small
	st.Align = gfx.AlignCenter
	shadow := color.RGBA{10, 14, 20, 230}
	for _, l := range g.geom.labels {
		x, y := float64(l.X+l.W/2), float64(l.Y)
		st.Color = shadow
		gfx.Text(dst, l.Text, (x+pk)/ts, (y+pk)/ts, st)
		switch {
		case l.Settlement:
			st.Color = gfx.TextColor
		case l.Ocean:
			st.Color = WorldPalette["label_ocean"]
		default:
			st.Color = WorldPalette["label"]
		}
		gfx.Text(dst, l.Text, x/ts, y/ts, st)
	}
}

// --- baked images --------------------------------------------------------

// haloOuter is where the atmosphere halo ends, in sphere radii.
const haloOuter = 1.28

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
			case rr < haloOuter:
				// The halo stays full up to the highest relief, so peaks on the
				// limb stand inside it, then fades out.
				t := math.Min(1, 1-(rr-ReliefMax)/(haloOuter-ReliefMax))
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
		hs := randx.Mix64(seed ^ uint64(i+1)*0x9E3779B97F4A7C15)
		fx, fy := hashUnit(hs), hashUnit(randx.Mix64(hs))
		b := hashUnit(randx.Mix64(hs ^ 0x51))
		tone := hashUnit(randx.Mix64(hs ^ 0xA7))
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
