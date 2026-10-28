// Package env is the colony view's ground cover and weather: grass tufts
// and the grass batch, crop furrows and the crop batch, see-through tree
// canopies, water edges and the water shader, puddles, the snow cache and
// veil, cave plants, map features, wind and contact springs, the weather
// drops and lightning. Its one type, Layer, reads the frame's
// *render.View, the sprite painter and the light layer, never the
// renderer (SOLID_GO.md §7); what it needs from the renderer's chunk
// index and ground pipeline comes in through the small interfaces in
// deps.go.
package env

import (
	"fmt"
	"image"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/roof"
)

// Environment presentation (ART §27.3, §16): the wind field, plants and
// grass waving in it and bending around creatures, rain and snow,
// snow cover and puddles (puddle_veil.go), seasonal looks. Presentation-only: reads
// WorldView weather and creature motion, never feeds the simulation.
//
// Clocks: ambient waving and weather run on the presentation frame
// (View.Present: they keep going while the game is paused); creature
// contact reads positions from the motion API, so a paused creature
// pushes nothing.

// PlantKind groups vegetation by how it moves.
type PlantKind uint8

const (
	KindGrass PlantKind = iota
	KindCrop
	KindBush
	KindTree
)

// Environment motion numbers (ART §27.3).
const (
	frameDT = 1.0 / 60 // one presentation frame, seconds

	springK    = 38.0 // plant stiffness, 1/s²
	springZeta = 0.35 // damping ratio

	gustSpeed   = 1.2  // gust fronts move downwind at this times the wind
	gustLambda1 = 9.5  // gust front wavelengths, tiles (9-14)
	gustLambda2 = 13.0 //
	gustAmp1    = 0.38 // gust strength, fraction of the wind
	gustAmp2    = 0.24
	gustCross   = 0.15 // sideways eddies, fraction of the wind speed

	ContactRadius = 0.6 // a creature this close (tiles) pushes a plant
	contactPush   = 0.9 // push per tile/s of creature speed (fraction of the lean limit)
	contactCap    = 1.2 // the push saturates here
	contactMinSpd = 0.3 // tiles/s; slower creatures are standing
	// ContactViewPad: creatures this many tiles past the view still push
	// plants (the view's own pad, the drawn reach and a margin).
	ContactViewPad = VisPad + int(motion.DrawnReach) + 2

	canopyLag   = 0.3 // s: a tree's canopy follows its trunk this late
	shakeFrames = 45  // a brushed tree shakes this long

	rainCap = 1400 // weather particle pool caps
	snowCap = 1100
)

// Lean limits per kind (radians) and the wind gain (radians per tile/s)
// at the call sites' reference sway amount.
var (
	leanLimits = [...]float64{KindGrass: 0.35, KindCrop: 0.22, KindBush: 0.12, KindTree: 0.06}
	leanGain   = [...]float64{KindGrass: 0.05, KindCrop: 0.035, KindBush: 0.018, KindTree: 0.012}
	leanRefAmt = [...]float64{KindGrass: 0.05, KindCrop: 0.055, KindBush: 0.035, KindTree: 0.022}
)

const (
	trunkLimit = 0.02
	trunkGain  = 0.0045
)

// Layer is the environment layer: the sky of this frame (Sky, embedded:
// weather, wind, contact springs, drops) and the state of its passes (the
// grass and crop batches, the water shader, the snow and wet cache, the
// tree canopies). Its passes draw with P and light with Lit.
type Layer struct {
	Sky
	// P draws sprites and Lit is the light layer (render/sprite,
	// render/light); Keys is the plant key table (New fills the crops'
	// and trees' keys; the renderer adds its ground and biome plants).
	P    *sprite.Painter
	Lit  *light.Lit
	Keys *PlantKeyTable
	// The passes' state (New allocates each; the renderer may point them
	// at its own).
	Water *Water      // the water shader (water_draw.go)
	SnowC *SnowCache  // the snow veil and wet masks per chunk (snow_cache.go)
	Trees *TreeFx     // see-through tree canopies (tree_fade.go)
	Grass *GrassBatch // the tuft atlas, mask and batch (grass_batch.go)
	Crops *CropBatch  // the crop unit atlas, batches and furrows (crop_batch.go)
	// keys caches SeasonKey per key and season for the art keyArt.
	keyArt *art.Catalog
	keys   map[seasonCacheKey]string
}

// Sky is this frame's weather read from the world, the wind field, the
// plants' contact springs, the weather drops and the visible rectangle.
type Sky struct {
	wind
	drawBatch
	kind        weatherKind
	intensity   float64
	Snow, wet   float64
	SnowCells   []uint8 // PLAY-015: per-tile snow depth 0..255 (WorldView.Snow), may be empty
	season      uint8
	outdoorTemp int
	roomGrid    []int
	roofs       []uint8 // roof flags per tile (SIM-018), may be empty
	TerrW       int
	Springs     map[uint32]*Spring // contact springs by tile
	movers      []mover
	treeBuf     []int32 // treeAt's PlantsIn buffer
	// Leaves lists the trees a creature brushed this Step in autumn: the
	// renderer drops their leaves (particles are not this layer's).
	Leaves               [][2]int
	View                 Rect // visible world rectangle, tiles (set while drawing)
	Drops                []Drop
	seq                  uint64
	FlashBucket, FlashAt uint64
	FlashSeen            bool
}

// wind is the world's wind this frame and the plant spring's response to
// its gusts.
type wind struct {
	wx, wy, speed  float64 // world wind, tiles/s
	dx, dy         float64 // its direction (east when still)
	celerity       float64 // gust front speed, tiles/s
	h1, d1, h2, d2 float64 // spring gain and phase lag at each gust frequency
}

// drawBatch is the weather's reused triangle batch over a white pixel or
// a soft dot.
type drawBatch struct {
	white, dot *ebiten.Image
	verts      []ebiten.Vertex
	idx        []uint16
}

// New is the environment layer drawing with p and lighting with lit.
func New(p *sprite.Painter, lit *light.Lit) *Layer {
	return &Layer{P: p, Lit: lit, Keys: NewPlantKeys(),
		Water: &Water{}, SnowC: &SnowCache{}, Trees: &TreeFx{}, Grass: &GrassBatch{}, Crops: &CropBatch{}}
}

// Rect is a world rectangle in tiles.
type Rect struct{ X0, Y0, X1, Y1 float64 }

func (v Rect) Empty() bool { return v.X1 <= v.X0 || v.Y1 <= v.Y0 }

// Spring is one plant's contact lean: a damped spring in fractions of
// its kind's lean limit, pulled towards whatever creature pushes it.
type Spring struct {
	Cx, Cy, Vx, vy float64 // lean and lean rate
	tx, ty         float64 // this frame's push target
	touch, was     bool
	shakeAt        uint64
	shaking        bool
}

// mover is a creature moving this frame (tiles, tiles/s).
type mover struct{ x, y, vx, vy, spd float64 }

type weatherKind uint8

const (
	wxNone weatherKind = iota
	wxClear
	wxCloudy
	wxRain
	wxStorm
	wxSnow
	wxBlizzard
)

func parseWeather(s string) weatherKind {
	switch s {
	case "clear":
		return wxClear
	case "cloudy":
		return wxCloudy
	case "rain":
		return wxRain
	case "storm":
		return wxStorm
	case "snow":
		return wxSnow
	case "blizzard":
		return wxBlizzard
	}
	return wxNone
}

func (k weatherKind) falling() bool { return k >= wxRain }
func (k weatherKind) frozen() bool  { return k == wxSnow || k == wxBlizzard }

// TileKey is tile (x, y) as a contact-spring or canopy-fade key.
func TileKey(x, y int) uint32 { return uint32(y)<<16 | uint32(x)&0xffff }

// envTime is the presentation clock in seconds.
func envTime(v *render.View) float64 { return float64(v.Present) / 60 }

// --- wind ---------------------------------------------------------------------

// WindAt is the local wind at world (x, y) in tiles per second: the
// world's wind plus travelling gusts.
func (e *Sky) WindAt(v *render.View, x, y float64) (vx, vy float64) {
	return e.field(x, y, envTime(v), 0, false)
}

var (
	gustK1 = 2 * math.Pi / gustLambda1
	gustK2 = 2 * math.Pi / gustLambda2
)

// field samples the wind field at (px, py) and time t. Both gust fronts
// are functions of (along-wind position - celerity*t), so the whole
// pattern travels downwind at gustSpeed times the wind. ph is a per-plant
// phase. With respond, each gust is passed through the plant spring's
// steady-state response (gain and lag at that gust's frequency): that is
// exactly how a damped spring driven by this wind leans.
func (e *wind) field(px, py, t, ph float64, respond bool) (float64, float64) {
	if e.speed == 0 {
		return 0, 0
	}
	along := e.dx*px + e.dy*py
	perp := -e.dy*px + e.dx*py
	u := along - e.celerity*t
	a1, a2, l1, l2 := gustAmp1, gustAmp2, 0.0, 0.0
	cross := gustCross
	if respond {
		a1, a2, l1, l2 = a1*e.h1, a2*e.h2, e.d1, e.d2
		cross *= e.h2
	}
	arg1 := gustK1*u + 0.6*math.Sin(0.17*perp+1.3) + ph - l1
	arg2 := gustK2*u + 0.5*math.Sin(0.11*perp) + 2.1 + 0.7*ph - l2
	g := a1*math.Sin(arg1) + a2*math.Sin(arg2)
	c := cross * e.speed * math.Sin(arg2+1.7)
	return e.wx*(1+g) - e.dy*c, e.wy*(1+g) + e.dx*c
}

// springResponse is the steady-state gain and phase lag of the plant
// spring driven at angular frequency w.
func springResponse(w float64) (gain, lag float64) {
	c := 2 * springZeta * math.Sqrt(springK)
	re, im := springK-w*w, c*w
	return springK / math.Hypot(re, im), math.Atan2(im, re)
}

// SetWeather reads this frame's weather from the world.
func (e *Sky) SetWeather(w game.WorldView) {
	e.kind = parseWeather(w.Weather)
	e.intensity = gfx.Clamp01(w.WeatherIntensity)
	e.Snow, e.wet = gfx.Clamp01(w.SnowDepth), gfx.Clamp01(w.Wetness)
	e.SnowCells = w.Snow // snow_veil.go: the veil reads it per tile
	e.season = seasonIndex(w.Season)
	e.outdoorTemp = w.OutdoorTemp
	e.roomGrid, e.TerrW, e.roofs = w.RoomGrid, w.TerrainW, w.RoofCells
	if w.WindX != e.wx || w.WindY != e.wy || e.dx == 0 && e.dy == 0 {
		e.wx, e.wy = w.WindX, w.WindY
		e.speed = math.Hypot(e.wx, e.wy)
		e.dx, e.dy = 1, 0
		if e.speed > 0 {
			e.dx, e.dy = e.wx/e.speed, e.wy/e.speed
		}
		e.celerity = gustSpeed * e.speed
		e.h1, e.d1 = springResponse(gustK1 * e.celerity)
		e.h2, e.d2 = springResponse(gustK2 * e.celerity)
	}
}

// --- plants -------------------------------------------------------------------

// PlantPose is how a plant (or grass tuft) at tile (x, y) leans this
// frame: wind plus any creature brushing through. scale is its growth
// scale; amount the call site's sway amount (taller crops sway more).
func (e *Sky) PlantPose(v *render.View, x, y int, kind PlantKind, amount, scale float64) motion.Pose {
	ps := motion.Pose{Px: 0.5, Py: 1, Sx: scale, Sy: scale}
	fx, fy := float64(x), float64(y)
	if r := e.View; !r.Empty() && (fx < r.X0-2 || fx > r.X1+1 || fy < r.Y0-1 || fy > r.Y1+3) {
		return ps // off screen
	}
	if kind > KindTree {
		kind = KindBush
	}
	t := envTime(v)
	h := sprite.TileHash(x, y)
	hp := float64(h>>40&0xffff) / 65536
	ph := 0.8 * (hp - 0.5)
	amt := 1.0
	if amount > 0 {
		amt = math.Max(0.5, math.Min(1.6, amount/leanRefAmt[kind]))
	}
	gain := leanGain[kind] * amt
	wx, wy := e.field(fx+0.5, fy+0.5, t, ph, true)
	// turbulence makes every plant tremble near its natural frequency
	air := math.Max(e.speed, 0.35)
	wn := math.Sqrt(springK) * (0.85 + 0.3*float64(h>>24&0xff)/256)
	fl := air * (0.12*math.Sin(wn*t+2*math.Pi*hp) + 0.05*math.Sin(1.7*wn*t+4*hp))
	var cx, cy, shake float64
	if len(e.Springs) > 0 {
		if s := e.Springs[TileKey(x, y)]; s != nil {
			cx, cy = s.Cx, s.Cy
			if s.shaking {
				age := float64(v.Present - s.shakeAt)
				shake = 0.8 * math.Exp(-age/14) * math.Sin(age*2*math.Pi/9)
			}
		}
	}
	L := leanLimits[kind]
	var leanX, leanY float64
	if kind == KindTree {
		trunk := trunkGain * amt * (wx + 0.3*fl*e.dx)
		lx, ly := e.field(fx+0.5, fy+0.5, t-canopyLag, ph, true)
		canopyX := gain*(lx+fl*e.dx) + L*(0.3*cx+shake)
		canopyY := gain*(ly+0.5*fl*e.dy) + L*0.3*cy
		leanX = trunkLimit*math.Tanh(trunk/trunkLimit) + L*math.Tanh(canopyX/L)
		leanY = L * math.Tanh(canopyY/L)
	} else {
		leanX = L * math.Tanh((gain*(wx+fl*e.dx)+L*cx)/L)
		leanY = L * math.Tanh((gain*(wy+0.5*fl*e.dy)+L*cy)/L)
	}
	ps.Skew = -leanX                // GeoM skew: negative tips the top east
	ps.Sy = scale * (1 - 0.6*leanY) // leaning south (towards the viewer) looks shorter
	if kind == KindGrass && e.Snow > 0 && e.outdoors(x, y) {
		thr := 0.15 + 0.6*float64(h>>8&0xffff)/65536
		a := 1 - gfx.Clamp01((e.Snow-thr)/0.2)
		ps.Sy *= 1 - 0.5*e.Snow
		ps.Cs = sprite.ColorScale{1, 1, 1, float32(a)}
	}
	return ps
}

// --- creature contact -----------------------------------------------------------

// Step advances the contact springs and the weather particles once per
// frame from world w and the colonists pawns: m gives the creatures'
// motion (stepped is the ticks the latest advance stepped, 0 paused) and
// c the plants on a tile. Leaves lists the autumn trees brushed.
func (e *Layer) Step(v *render.View, w game.WorldView, pawns []game.PawnView, stepped int, m Movers, c Chunks) {
	if e.Springs == nil {
		e.Springs = map[uint32]*Spring{}
	}
	e.SetWeather(w)
	e.stepContacts(v, w, pawns, stepped, m, c)
	e.stepWeather(v, e.SnowC)
}

// stepContacts indexes this frame's moving creatures, pushes the plants
// within ContactRadius of them and integrates every live contact spring.
func (e *Sky) stepContacts(v *render.View, w game.WorldView, pawns []game.PawnView, stepped int, m Movers, c Chunks) {
	e.movers = e.movers[:0]
	e.Leaves = e.Leaves[:0]
	perSec := float64(stepped) * 60 // tiles/tick -> on-screen tiles/s
	add := func(ms motion.MoveState) {
		vx, vy := ms.VX*perSec, ms.VY*perSec
		if spd := math.Hypot(vx, vy); spd >= contactMinSpd {
			e.movers = append(e.movers, mover{ms.X, ms.Y, vx, vy, spd})
		}
	}
	if perSec > 0 {
		// only creatures near the last frame's view push plants: a spring
		// settles in about a second, so one off screen is never seen
		for _, p := range pawns {
			if !p.Embodied && m.InView(p.X, p.Y, ContactViewPad, ContactViewPad) {
				add(m.PawnMotion(p.ID))
			}
		}
		for i := range w.Animals {
			if an := &w.Animals[i]; m.InView(an.X, an.Y, ContactViewPad, ContactViewPad) {
				add(m.AnimalMotion(an.ID))
			}
		}
	}
	for _, mv := range e.movers {
		for ty := int(math.Floor(mv.y - ContactRadius)); ty <= int(math.Ceil(mv.y+ContactRadius)); ty++ {
			for tx := int(math.Floor(mv.x - ContactRadius)); tx <= int(math.Ceil(mv.x+ContactRadius)); tx++ {
				if tx < 0 || ty < 0 {
					continue
				}
				e.push(tx, ty, mv)
			}
		}
	}
	cd := 2 * springZeta * math.Sqrt(springK)
	for k, s := range e.Springs {
		if s.touch && !s.was && e.treeAt(&w, int(k&0xffff), int(k>>16), c) {
			s.shakeAt, s.shaking = v.Present, true
			if e.season == seasonFall {
				e.Leaves = append(e.Leaves, [2]int{int(k & 0xffff), int(k >> 16)})
			}
		}
		ax := springK*(s.tx-s.Cx) - cd*s.Vx
		ay := springK*(s.ty-s.Cy) - cd*s.vy
		s.Vx += ax * frameDT
		s.vy += ay * frameDT
		s.Cx += s.Vx * frameDT
		s.Cy += s.vy * frameDT
		s.was, s.touch, s.tx, s.ty = s.touch, false, 0, 0
		if s.shaking && v.Present-s.shakeAt >= shakeFrames {
			s.shaking = false
		}
		if !s.was && !s.shaking && math.Abs(s.Cx) < 0.004 && math.Abs(s.Cy) < 0.004 &&
			math.Abs(s.Vx) < 0.02 && math.Abs(s.vy) < 0.02 {
			delete(e.Springs, k)
		}
	}
}

// push leans the plant at tile (tx, ty) away from mover m: forward along
// its path and out to the side, never back towards it.
func (e *Sky) push(tx, ty int, m mover) {
	ox, oy := float64(tx)-m.x, float64(ty)-m.y
	d := math.Hypot(ox, oy)
	if d >= ContactRadius {
		return
	}
	ax, ay := 0.0, 0.0
	if d > 0.05 {
		ax, ay = ox/d, oy/d
	}
	px, py := ax+m.vx/m.spd, ay+m.vy/m.spd
	n := math.Hypot(px, py)
	if n < 1e-6 {
		return // directly behind it
	}
	mag := math.Min(contactPush*m.spd*motion.Smoothstep01(1-d/ContactRadius)*n/2, contactCap)
	k := TileKey(tx, ty)
	s := e.Springs[k]
	if s == nil {
		s = &Spring{}
		e.Springs[k] = s
	}
	if mag*mag > s.tx*s.tx+s.ty*s.ty {
		s.tx, s.ty = mag*px/n, mag*py/n
	}
	s.touch = true
}

// treeAt reports a tree on tile (x, y) (a brush shake), from the plants
// on that tile alone (c.PlantsOn), never a pass over every plant.
func (e *Sky) treeAt(w *game.WorldView, x, y int, c Chunks) bool {
	e.treeBuf = c.PlantsOn(w.Plants, x, y, x, y, e.treeBuf[:0])
	for _, i := range e.treeBuf {
		if int(i) < len(w.Plants) && sprite.IsTreePlant(w.Plants[i].Species) {
			return true
		}
	}
	return false
}

// --- sprite keys ----------------------------------------------------------------

const (
	seasonNone uint8 = iota
	seasonFall
	seasonWinter
)

func seasonIndex(s string) uint8 {
	switch s {
	case "Fall":
		return seasonFall
	case "Winter":
		return seasonWinter
	}
	return seasonNone
}

type seasonCacheKey struct {
	key    string
	season uint8
}

// SeasonKey swaps a plant sprite for its seasonal look (autumn, bare)
// and resolves it to art that exists: the variant, then variant 0, then
// the pre-variant sprite. Results are cached per key and season.
func (e *Layer) SeasonKey(v *render.View, key string, w game.WorldView) string {
	if e.keys == nil || e.keyArt != v.Art {
		e.keys, e.keyArt = map[seasonCacheKey]string{}, v.Art
	}
	ck := seasonCacheKey{key, seasonIndex(w.Season)}
	if v, ok := e.keys[ck]; ok {
		return v
	}
	var cands []string
	if ck.season != seasonNone {
		if s, ok := e.Keys.Seasonal[key]; ok {
			cands = append(cands, s[ck.season-1])
		}
	}
	cands = append(cands, key)
	cands = append(cands, e.Keys.Fallback[key]...)
	out := cands[0]
	for _, c := range cands {
		if v.Art.Has(c) {
			out = c
			break
		}
	}
	e.keys[ck] = out
	return out
}

// Crop stages as drawn (the sim's 0..3 plus sprout and wilted).
const (
	StSown = iota
	StSprout
	StYoung
	StMature
	StHarvestable
	StWilted
	NumCropStages
)

// CropStageNames names the drawn stages in sprite keys.
var CropStageNames = [NumCropStages]string{"sown", "sprout", "young", "mature", "harvestable", "wilted"}

// TreeSpecies are the generic trees' species (TreeSpeciesAt).
var TreeSpecies = [...]string{"oak", "birch", "pine"}

// PlantKeyTable holds every plant sprite key, built once so drawing never
// concatenates strings: the crops' and trees' (NewPlantKeys) and, added
// by the renderer to the same maps, its ground and biome plants'.
type PlantKeyTable struct {
	Crops    map[string]*[NumCropStages][CropVariants]string
	Trees    [3][3]string
	Fallback map[string][]string
	Seasonal map[string][2]string // autumn, bare
}

// NewPlantKeys builds the crops' and the trees' sprite keys, their
// fallbacks and seasonal looks.
func NewPlantKeys() *PlantKeyTable {
	t := &PlantKeyTable{Crops: map[string]*[NumCropStages][CropVariants]string{}, Fallback: map[string][]string{}, Seasonal: map[string][2]string{}}
	for _, c := range game.Crops() {
		ks := &[NumCropStages][CropVariants]string{}
		for st, name := range CropStageNames {
			for v := 0; v < CropVariants; v++ {
				k := fmt.Sprintf("plant/%s_%s_v%d", c.Name, name, v)
				ks[st][v] = k
				var fb []string
				if v != 0 {
					fb = append(fb, fmt.Sprintf("plant/%s_%s_v0", c.Name, name))
				}
				if st == StSprout {
					fb = append(fb, fmt.Sprintf("plant/%s_sown_v%d", c.Name, v))
					if v != 0 {
						fb = append(fb, "plant/"+c.Name+"_sown_v0")
					}
				}
				if len(fb) > 0 {
					t.Fallback[k] = fb
				}
			}
		}
		t.Crops[c.Name] = ks
	}
	for s, sp := range TreeSpecies {
		for v := 0; v < 3; v++ {
			k := ArtKey("plant", "tree_"+sp, v)
			t.Trees[s][v] = k
			if sp != "pine" {
				t.Seasonal[k] = [2]string{k + "_autumn", k + "_bare"}
				t.Fallback[k+"_autumn"] = []string{k}
				t.Fallback[k+"_bare"] = []string{k}
			}
		}
	}
	return t
}

// CropStage is a crop's drawn stage: the wilted look from CropWiltDamage
// (cold or heat, PLAY-016), else the stage its growth has reached.
func CropStage(p game.PlantView) int {
	if p.Damage >= CropWiltDamage {
		return StWilted
	}
	return CropGrowthStage(p)
}

// CropGrowthStage is the stage a crop's growth has reached, damage aside.
func CropGrowthStage(p game.PlantView) int {
	switch min(max(p.Stage, 0), 3) {
	case 0:
		if p.Growth >= 8 {
			return StSprout
		}
		return StSown
	case 1:
		return StYoung
	case 2:
		return StMature
	}
	return StHarvestable
}

// TreeSpeciesAt is the tree species growing on a tile: mostly oak, then
// birch, some pine.
func TreeSpeciesAt(h uint64) int {
	switch v := art.Mix64(h^0x7ee5) % 20; {
	case v < 9:
		return 0
	case v < 15:
		return 1
	}
	return 2
}

// --- weather particles ------------------------------------------------------------

type DropKind uint8

const (
	DropRain DropKind = iota
	DropSnow
	DropSplash  // a raindrop landing on open ground
	DropRipple  // a raindrop landing in a puddle
	DropSettled // a snowflake resting on the ground before it melts into the cover
)

// Drop is one weather particle: a falling drop or flake headed for
// ground point (gx, gy), or a short landing effect there.
type Drop struct {
	Kind              DropKind
	v                 uint8 // sprite variant
	Age, life         uint16
	Gx, Gy            float64 // landing point, world tiles
	T, t0             float64 // seconds until it lands; its whole fall
	Vx, vy, Vz        float64 // drift with the wind and fall speed, tiles/s
	Size              float32 // radius, tiles
	amp, omega, phase float32 // snow flutter
	alpha             float32
}

// ScreenVel is the drop's velocity on screen (tiles/s): its drift plus the
// fall, which projects to straight down.
func (d *Drop) ScreenVel() (float64, float64) { return d.Vx, d.vy + d.Vz }

// PosAt is where a falling drop is drawn t seconds before it lands: its
// ground point back along its drift, raised by its height.
func (d *Drop) PosAt(t float64) (float64, float64) {
	x := d.Gx - d.Vx*t
	if d.Kind == DropSnow {
		x += float64(d.amp) * (math.Sin(float64(d.omega)*t+float64(d.phase)) - math.Sin(float64(d.phase)))
	}
	return x, d.Gy - (d.vy+d.Vz)*t
}

// weatherDensity is falling drops per visible tile at full intensity.
func (k weatherKind) density() float64 {
	switch k {
	case wxRain:
		return 2.4
	case wxStorm:
		return 3.0
	case wxSnow:
		return 1.3
	case wxBlizzard:
		return 1.8
	}
	return 0
}

// stepWeather ages the weather pool: falling drops land (as splashes,
// ripples or settled flakes), effects expire, and new drops spawn over
// the view up to the density the intensity asks for.
func (e *Sky) stepWeather(v *render.View, sc *SnowCache) {
	cap := rainCap
	if e.kind.frozen() {
		cap = snowCap
	}
	want := 0
	if e.kind.falling() && !e.View.Empty() {
		area := (e.View.X1 - e.View.X0) * (e.View.Y1 - e.View.Y0)
		want = min(int(e.kind.density()*e.intensity*area*fxOr1(v)), cap*85/100)
	}
	room := cap - want // landing effects share what the drops leave
	live, effects := 0, 0
	kept := e.Drops[:0]
	for _, d := range e.Drops {
		switch d.Kind {
		case DropRain, DropSnow:
			d.T -= frameDT
			if d.T > 0 {
				live++
				kept = append(kept, d)
				continue
			}
			if effects >= room {
				continue
			}
			x, y := int(math.Floor(d.Gx)), int(math.Floor(d.Gy))
			if !e.outdoors(x, y) { // roofed over since it spawned: nothing lands
				continue
			}
			d.Age, d.T = 0, 0
			switch {
			case d.Kind == DropSnow:
				d.Kind, d.life = DropSettled, 40
			case e.wet > 0 && sc.PuddleAt(d.Gx, d.Gy) >= PuddleRippleMin: // puddle_veil.go: in the water
				d.Kind, d.life = DropRipple, 26
			default:
				d.Kind, d.life = DropSplash, 9
			}
		default:
			d.Age++
			if d.Age >= d.life {
				continue
			}
		}
		effects++
		kept = append(kept, d)
	}
	e.Drops = kept
	if live >= want {
		return
	}
	for n := min(want-live, max(want/8, 8)); n > 0 && len(e.Drops) < cap; n-- {
		e.spawn()
	}
}

// spawn adds one falling drop or flake over open ground in the view.
func (e *Sky) spawn() {
	v := e.View
	for try := 0; try < 3; try++ {
		e.seq++
		h := func(k uint64) float64 { return motion.Hash01(e.seq, k, 0x5eed) }
		gx := v.X0 - 1 + h(1)*(v.X1-v.X0+2)
		gy := v.Y0 - 0.5 + h(2)*(v.Y1-v.Y0+3)
		if !e.outdoors(int(math.Floor(gx)), int(math.Floor(gy))) {
			continue
		}
		d := Drop{Gx: gx, Gy: gy}
		if e.kind.frozen() {
			d.Kind = DropSnow
			d.Size = [...]float32{0.018, 0.018, 0.026, 0.026, 0.036, 0.05}[int(h(3)*6)%6]
			d.v = uint8(h(4) * 4)
			d.Vz = 0.7 + 8*float64(d.Size)
			jit := 0.85 + 0.3*h(5)
			d.Vx, d.vy = e.wx*jit, e.wy*jit
			d.amp = float32(0.08 + 0.12*h(6))
			d.omega = float32(3 + 2*h(7))
			d.phase = float32(2 * math.Pi * h(8))
			d.t0 = (1.5 + 4.5*h(9)) / d.Vz
			d.alpha = float32(0.7 + 0.3*h(10))
		} else {
			d.Kind = DropRain
			d.Vz = 13 + 4*h(3)
			jit := 1.5 * (0.9 + 0.2*h(5)) // windage, exaggerated a little for readability
			d.Vx, d.vy = e.wx*jit, e.wy*jit*0.6
			d.t0 = 0.1 + 0.25*h(9)
			d.alpha = float32(0.45 + 0.3*h(10))
		}
		d.T = d.t0
		e.Drops = append(e.Drops, d)
		return
	}
}

// outdoors reports ground open to the sky: weather falls only there. With
// roofs (SIM-018) that is any unroofed tile but a wall; without roof data,
// any tile outside a room.
func (e *Sky) outdoors(x, y int) bool {
	if len(e.roomGrid) == 0 || e.TerrW <= 0 {
		return true
	}
	if x < 0 || y < 0 || x >= e.TerrW {
		return true
	}
	i := y*e.TerrW + x
	if i >= len(e.roomGrid) {
		return true
	}
	if len(e.roofs) == len(e.roomGrid) {
		return e.roomGrid[i] != -2 && e.roofs[i]&uint8(roof.Built|roof.Natural) == 0
	}
	return e.roomGrid[i] == -1
}

// LightningAt reports a lightning flash starting in tick bucket b: rare,
// storms only, and the same every run.
func LightningAt(kind string, intensity float64, b uint64) bool {
	return kind == "storm" && motion.Hash01(b, 0x11647) < 0.012*gfx.Clamp01(intensity)
}

// LightningBucket is the lightning roll's tick bucket.
const LightningBucket = 20 // ticks

// WeatherVeil is the sky's tint over the scene (darkening, premultiplied)
// and the fog that cuts visibility in snow.
func WeatherVeil(w game.WorldView) (veil, fog color.RGBA) {
	i := gfx.Clamp01(w.WeatherIntensity)
	pm := func(r, g, b uint8, a float64) color.RGBA {
		a = gfx.Clamp01(a)
		return color.RGBA{uint8(float64(r) * a), uint8(float64(g) * a), uint8(float64(b) * a), uint8(255 * a)}
	}
	switch parseWeather(w.Weather) {
	case wxCloudy:
		veil = pm(34, 40, 52, 0.10*(0.6+0.4*i))
	case wxRain:
		veil = pm(30, 38, 52, 0.10+0.12*i)
	case wxStorm:
		veil = pm(18, 22, 36, 0.18+0.22*i)
	case wxSnow:
		veil = pm(60, 66, 80, 0.04+0.04*i)
		fog = pm(226, 232, 240, 0.08*i)
	case wxBlizzard:
		veil = pm(40, 46, 60, 0.12+0.08*i)
		fog = pm(226, 232, 240, 0.10+0.25*i)
	}
	return veil, fog
}

// --- ground cover -----------------------------------------------------------------

// The weather sprites (weather/<name>_NN), spelled out so the tables are
// immutable.
var (
	snowCoverKeys = [2]string{"weather/snow_cover_04", "weather/snow_cover_05"} // tileable
	// SnowBankKeys are the snow cover's edge masks: N, E, S, W.
	SnowBankKeys  = [4]string{"weather/snow_cover_00", "weather/snow_cover_01", "weather/snow_cover_02", "weather/snow_cover_03"}
	snowDriftKeys = [4]string{"weather/snow_drift_00", "weather/snow_drift_01", "weather/snow_drift_02", "weather/snow_drift_03"}
	frostKeys     = [4]string{"weather/frost_00", "weather/frost_01", "weather/frost_02", "weather/frost_03"}
	splashKeys    = [3]string{"weather/splash_00", "weather/splash_01", "weather/splash_02"}
	flakeKeys     = [4]string{"weather/snowflake_00", "weather/snowflake_01", "weather/snowflake_02", "weather/snowflake_03"}
)

// SnowCover is the snow decal on a tile (a tileable cover) and its
// opacity at this depth. Ground fills in at its own depth, smoothly
// across several tiles, so early snow lies in soft patches rather than a
// checkerboard.
func SnowCover(x, y int, depth float64) (string, float64) {
	return snowCoverKeys[art.Mix64(sprite.TileHash(x, y)^0x51)%2], snowField(float64(x)+0.5, float64(y)+0.5, depth)
}

// snowField is the snow cover's opacity at world point (fx, fy).
func snowField(fx, fy, depth float64) float64 {
	if depth <= 0 {
		return 0
	}
	return gfx.Clamp01((depth - 0.3*ValueNoise(fx/5, fy/5, 0x5000)) / 0.4)
}

// ValueNoise is smooth 2D noise in [0, 1): hashed lattice values eased
// between, so neighbouring points get similar values.
func ValueNoise(fx, fy float64, salt uint64) float64 {
	ix, iy := math.Floor(fx), math.Floor(fy)
	u, v := motion.Smoothstep01(fx-ix), motion.Smoothstep01(fy-iy)
	at := func(dx, dy float64) float64 { return motion.Hash01(uint64(int64(ix+dx)), uint64(int64(iy+dy)), salt) }
	top := at(0, 0) + (at(1, 0)-at(0, 0))*u
	bot := at(0, 1) + (at(1, 1)-at(0, 1))*u
	return top + (bot-top)*v
}

// SnowDrift is a drift heaped on some tiles once the snow is deep.
func SnowDrift(x, y int, depth float64) (string, float64) {
	th := sprite.TileHash(x, y)
	if depth <= 0.4 || motion.Hash01(th, 0xd21f) > 0.18 {
		return "", 0
	}
	return snowDriftKeys[art.Mix64(th^0xd2)%4], gfx.Clamp01((depth - 0.4) / 0.3)
}

// FrostStrength is how strongly rime shows (0..0.35): on freezing
// mornings without snow, colder is whiter, gone by mid-morning.
func FrostStrength(tempTenths int, snow float64, d clock.Date) float64 {
	hour := float64(d.Hour) + float64(d.Minute)/60
	if tempTenths > 0 || snow >= 0.2 || hour < 3 || hour >= 10 {
		return 0
	}
	return 0.35 * gfx.Clamp01(float64(-tempTenths)/50) * gfx.Clamp01((10-hour)/4)
}

// SetView records the visible world rectangle in tiles: the view's
// camera over sw × sh map pixels.
func (e *Sky) SetView(v *render.View, sw, sh int, tile float64) {
	if tile <= 0 {
		return
	}
	e.View = Rect{v.Cam.OffsetX / tile, v.Cam.OffsetY / tile,
		(v.Cam.OffsetX + float64(sw)) / tile, (v.Cam.OffsetY + float64(sh)) / tile}
}

// VisibleTiles is the on-map tile range in view.
func (e *Sky) VisibleTiles(v *render.View) (x0, y0, x1, y1 int) {
	r := e.View
	x0, y0 = max(int(math.Floor(r.X0)), 0), max(int(math.Floor(r.Y0)), 0)
	x1, y1 = min(int(math.Ceil(r.X1)), v.MapW-1), min(int(math.Ceil(r.Y1)), v.MapH-1)
	return
}

// DrawGroundCover draws snow cover and frost over the terrain: first a
// seamless snow veil shaded smoothly corner to corner in one batch, then
// the per-tile decals for texture. The wet ground and its puddles are the
// wet veil under the grass (DrawPuddleVeil, puddle_veil.go). The decals
// draw through s; c is the chunk index the snow bakes read.
func (e *Layer) DrawGroundCover(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64, s Stamp, c Chunks) {
	e.SetView(v, sw, sh, tile)
	snow := gfx.Clamp01(w.SnowDepth)
	frost := FrostStrength(w.OutdoorTemp, snow, v.Date)
	anySnow := snow > 0 || len(w.Snow) > 0 // PLAY-015: per-tile snow (snow_veil.go)
	if !anySnow && frost <= 0 {
		return
	}
	e.SetWeather(w)
	x0, y0, x1, y1 := e.VisibleTiles(v)
	veil := 0.85 // snow veil strength: all of it without decals, a base under them
	if v.Art.Has(snowCoverKeys[0]) && tile >= DecalMinTile {
		veil = 0.5
	}
	if anySnow {
		e.drawSnowLayer(v, screen, w, tile, veil, s, c)
	}
	if tile < DecalMinTile { // zoomed out: the veil alone (lod.go)
		return
	}
	open := func(x, y int) bool { return e.GroundOpen(w, x, y) }
	none := color.NRGBA{}
	decal := func(k string, x, y int, a float64) {
		if a > 0 && v.Art.Has(k) {
			s.DrawSpritePose(screen, k, none, float64(x), float64(y), tile, motion.Pose{Cs: sprite.ColorScale{1, 1, 1, float32(a)}})
		}
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !open(x, y) {
				continue
			}
			if d := e.SnowDepthAt(x, y); d > 0 {
				k, a := SnowCover(x, y, d)
				// texture where the snow lies full; thin snow is the smooth veil alone
				decal(k, x, y, 0.8*motion.Smoothstep01((a-0.6)/0.4))
				for d, n := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
					if !open(x+n[0], y+n[1]) { // snow banks up against walls and rock
						decal(SnowBankKeys[d], x, y, a)
					}
				}
				k, a = SnowDrift(x, y, d)
				decal(k, x, y, a)
			}
			if frost > 0 {
				a := frost * motion.Smoothstep01((0.6-ValueNoise(float64(x)/4, float64(y)/4, 0xf505))/0.4)
				decal(frostKeys[art.Mix64(sprite.TileHash(x, y)^0xf5)%4], x, y, a)
			}
		}
	}
}

// GroundOpen reports ground that takes snow cover, drifts, the wet veil
// and frost: open to the sky (not roofed, not a wall) and not rock (rough or
// smoothed).
func (e *Sky) GroundOpen(w game.WorldView, x, y int) bool {
	return e.outdoors(x, y) && !sprite.IsRockWallName(TerrainName(w, x, y))
}

// --- drawing ------------------------------------------------------------------------

// DrawWeather draws falling rain or snow and the storm's darkening; the
// lightning flash is DrawLightning's, after the renderer shows the sim's
// strikes.
func (e *Sky) DrawWeather(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	e.SetView(v, sw, sh, tile)
	veil, fog := WeatherVeil(w)
	if veil.A > 0 {
		vector.FillRect(screen, 0, 0, float32(sw), float32(sh), veil, false)
	}
	if fog.A > 0 {
		vector.FillRect(screen, 0, 0, float32(sw), float32(sh), fog, false)
	}
	if len(e.Drops) > 0 {
		e.drawDrops(v, screen, sw, sh, tile)
	}
}

// maxFlashReduced caps the lightning flash with Reduced flashing on.
const maxFlashReduced = 0.15

// fxOr1 is the Options effects multiplier (1 for a renderer built
// without settings).
func fxOr1(v *render.View) float64 {
	if v.Fx <= 0 {
		return 1
	}
	return v.Fx
}

// DrawLightning draws the flash of the last strike seen (FlashSeen,
// FlashAt: the renderer's strikes set them), capped with Reduced flashing.
func (e *Sky) DrawLightning(v *render.View, screen *ebiten.Image, sw, sh int) {
	if !e.FlashSeen {
		return
	}
	age := float64(v.Present - e.FlashAt)
	var a float64
	switch {
	case age < 3:
		a = 0.5
	case age < 6:
		a = 0.12
	case age < 9:
		a = 0.38
	case age < 40:
		a = 0.38 * math.Exp(-(age-9)/6)
	}
	if v.ReducedFlashing { // Options: accessibility
		a = math.Min(a, maxFlashReduced)
	}
	if a > 0.005 {
		vector.FillRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{uint8(235 * a), uint8(240 * a), uint8(255 * a), uint8(255 * a)}, false)
	}
}

// drawDrops draws the weather pool: rain as wind-slanted streaks fading
// to their tail (motion blur), flakes as soft dots or their sprites,
// splashes, ripples and settling flakes.
func (e *Sky) drawDrops(v *render.View, screen *ebiten.Image, sw, sh int, tile float64) {
	const blur = 0.022 // s of motion blur in a streak
	inside := func(x, y float32) bool {
		return x > -40 && y > -40 && x < float32(sw)+40 && y < float32(sh)+40
	}
	rainClr := color.RGBA{196, 210, 228, 255}
	width := math.Max(1, 0.022*tile)
	// streaks, splash crowns and ripple rings share the white texture
	e.beginBatch()
	var flakes, splashes []int
	for i := range e.Drops {
		d := &e.Drops[i]
		switch d.Kind {
		case DropRain:
			wx, wy := d.PosAt(d.T)
			hx, hy := v.WorldToScreenF(wx, wy)
			if !inside(hx, hy) {
				continue
			}
			vx, vy := d.ScreenVel()
			fade := float64(d.alpha) * gfx.Clamp01((d.t0-d.T)/0.08)
			e.streak(float64(hx), float64(hy), vx*blur*tile, vy*blur*tile, width, rainClr, fade)
		case DropRipple:
			sx, sy := v.WorldToScreenF(d.Gx, d.Gy)
			k := float64(d.Age) / float64(d.life)
			rad := (0.05 + 0.2*k) * tile
			e.ring(float64(sx), float64(sy), rad, rad*0.45, math.Max(1, 0.015*tile), color.RGBA{214, 226, 240, 255}, 0.5*(1-k))
		case DropSplash:
			if v.Art.Has(splashKeys[0]) {
				splashes = append(splashes, i)
				continue
			}
			sx, sy := v.WorldToScreenF(d.Gx, d.Gy)
			k := float64(d.Age) / float64(d.life)
			for j := -1; j <= 1; j += 2 { // two droplets jump out
				px := float64(sx) + float64(j)*(0.03+0.09*k)*tile
				py := float64(sy) - (0.12*k-0.12*k*k)*tile
				e.quad(px-width/2, py-width/2, width, width, rainClr, 0.6*(1-k))
			}
		case DropSnow, DropSettled:
			flakes = append(flakes, i)
		}
	}
	e.flush(screen, e.WhiteImg())
	for _, i := range splashes {
		d := &e.Drops[i]
		sx, sy := v.WorldToScreenF(d.Gx, d.Gy)
		k := float64(d.Age) / float64(d.life)
		drawCentered(v.Art, screen, splashKeys[int(d.v)%3], float64(sx), float64(sy), tile*(0.2+0.18*k), 0.75*(1-k))
	}
	if len(flakes) == 0 {
		return
	}
	useArt := v.Art.Has(flakeKeys[0])
	e.beginBatch()
	for _, i := range flakes {
		d := &e.Drops[i]
		wx, wy := d.Gx, d.Gy
		a := float64(d.alpha)
		if d.Kind == DropSnow {
			wx, wy = d.PosAt(d.T)
			a *= gfx.Clamp01((d.t0 - d.T) / 0.4)
		} else {
			a *= 1 - float64(d.Age)/float64(d.life)
		}
		sx, sy := v.WorldToScreenF(wx, wy)
		if !inside(sx, sy) {
			continue
		}
		rad := float64(d.Size) * tile
		if useArt {
			drawCentered(v.Art, screen, flakeKeys[int(d.v)%4], float64(sx), float64(sy), rad*5, a)
			continue
		}
		e.dotQuad(float64(sx), float64(sy), rad, a)
	}
	if !useArt {
		e.flush(screen, e.dotImg())
	}
}

// drawCentered draws a weather sprite centred on (sx, sy), size px wide.
func drawCentered(c *art.Catalog, screen *ebiten.Image, key string, sx, sy, size, alpha float64) {
	img := c.Image(key, color.NRGBA{})
	if img == nil || alpha <= 0 {
		return
	}
	b := img.Bounds()
	k := size / float64(max(b.Dx(), 1))
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-float64(b.Dx())/2, -float64(b.Dy())/2)
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(sx, sy)
	op.ColorScale.ScaleAlpha(float32(alpha))
	screen.DrawImage(img, op)
}

// --- triangle batching ----------------------------------------------------------------

// WhiteImg is a one-pixel white source image for triangle batches.
func (e *drawBatch) WhiteImg() *ebiten.Image {
	if e.white == nil {
		img := ebiten.NewImage(3, 3)
		img.Fill(color.White)
		e.white = img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
	}
	return e.white
}

const dotPx = 16

// dotImg is a soft round dot (white, alpha falling off to the edge).
func (e *drawBatch) dotImg() *ebiten.Image {
	if e.dot == nil {
		src := image.NewNRGBA(image.Rect(0, 0, dotPx, dotPx))
		for y := 0; y < dotPx; y++ {
			for x := 0; x < dotPx; x++ {
				d := math.Hypot(float64(x)+0.5-dotPx/2, float64(y)+0.5-dotPx/2) / (dotPx / 2)
				src.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * gfx.Clamp01(1-d*d))})
			}
		}
		e.dot = ebiten.NewImageFromImage(src)
	}
	return e.dot
}

func (e *drawBatch) beginBatch() { e.verts, e.idx = e.verts[:0], e.idx[:0] }

func (e *drawBatch) vert(x, y, sx, sy float64, c color.RGBA, a float64) {
	e.verts = append(e.verts, ebiten.Vertex{DstX: float32(x), DstY: float32(y), SrcX: float32(sx), SrcY: float32(sy),
		ColorR: float32(c.R) / 255, ColorG: float32(c.G) / 255, ColorB: float32(c.B) / 255, ColorA: float32(gfx.Clamp01(a))})
}

// quadIdx indexes the last four vertices as two triangles.
func (e *drawBatch) quadIdx() {
	n := uint16(len(e.verts) - 4)
	e.idx = append(e.idx, n, n+1, n+2, n+1, n+3, n+2)
}

// room reports space for n more vertices under the 16-bit index limit.
func (e *drawBatch) room(n int) bool { return len(e.verts)+n < 65000 }

// quad is a solid rectangle.
func (e *drawBatch) quad(x, y, w, h float64, c color.RGBA, a float64) {
	e.quad4(x, y, w, h, c, a, a, a, a)
}

// quad4 is a rectangle shaded between per-corner alphas (top-left,
// top-right, bottom-left, bottom-right), so neighbours meet seamlessly.
func (e *drawBatch) quad4(x, y, w, h float64, c color.RGBA, a00, a10, a01, a11 float64) {
	if a00+a10+a01+a11 <= 0 || !e.room(4) {
		return
	}
	e.vert(x, y, 1.5, 1.5, c, a00)
	e.vert(x+w, y, 1.5, 1.5, c, a10)
	e.vert(x, y+h, 1.5, 1.5, c, a01)
	e.vert(x+w, y+h, 1.5, 1.5, c, a11)
	e.quadIdx()
}

// streak is a line from (hx, hy) back along (dx, dy), opaque at the head
// and clear at the tail.
func (e *drawBatch) streak(hx, hy, dx, dy, width float64, c color.RGBA, a float64) {
	n := math.Hypot(dx, dy)
	if n == 0 || a <= 0 || !e.room(4) {
		return
	}
	px, py := -dy/n*width/2, dx/n*width/2
	tx, ty := hx-dx, hy-dy
	e.vert(tx-px, ty-py, 1.5, 1.5, c, 0)
	e.vert(tx+px, ty+py, 1.5, 1.5, c, 0)
	e.vert(hx-px, hy-py, 1.5, 1.5, c, a)
	e.vert(hx+px, hy+py, 1.5, 1.5, c, a)
	e.quadIdx()
}

// dotQuad is a soft dot of radius rad textured with dotImg.
func (e *drawBatch) dotQuad(x, y, rad, a float64) {
	if a <= 0 || !e.room(4) {
		return
	}
	rad = math.Max(rad, 0.8)
	c := color.RGBA{248, 250, 255, 255}
	e.vert(x-rad, y-rad, 0, 0, c, a)
	e.vert(x+rad, y-rad, dotPx, 0, c, a)
	e.vert(x-rad, y+rad, 0, dotPx, c, a)
	e.vert(x+rad, y+rad, dotPx, dotPx, c, a)
	e.quadIdx()
}

const ellipseSegs = 14

// ring is an elliptical outline of the given stroke width.
func (e *drawBatch) ring(cx, cy, rx, ry, width float64, c color.RGBA, a float64) {
	if a <= 0 || !e.room(2*ellipseSegs) {
		return
	}
	base := uint16(len(e.verts))
	for i := 0; i < ellipseSegs; i++ {
		th := 2 * math.Pi * float64(i) / ellipseSegs
		cs, sn := math.Cos(th), math.Sin(th)
		e.vert(cx+(rx-width/2)*cs, cy+(ry-width/2)*sn, 1.5, 1.5, c, a)
		e.vert(cx+(rx+width/2)*cs, cy+(ry+width/2)*sn, 1.5, 1.5, c, a)
	}
	for i := 0; i < ellipseSegs; i++ {
		a0, b0 := base+uint16(2*i), base+uint16(2*i+1)
		j := (i + 1) % ellipseSegs
		a1, b1 := base+uint16(2*j), base+uint16(2*j+1)
		e.idx = append(e.idx, a0, b0, a1, b0, b1, a1)
	}
}

func (e *drawBatch) flush(screen *ebiten.Image, src *ebiten.Image) {
	if len(e.idx) > 0 {
		screen.DrawTriangles(e.verts, e.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
	e.beginBatch()
}
