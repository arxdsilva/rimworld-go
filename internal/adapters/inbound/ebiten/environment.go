package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Environment presentation (ART §27.3, §16): the wind field, plants and
// grass waving in it and bending around creatures, rain and snow,
// snow cover and puddles, seasonal looks. Presentation-only: reads
// WorldView weather and creature motion, never feeds the simulation.
//
// Clocks: ambient waving and weather run on r.present (they keep going
// while the game is paused); creature contact reads positions from the
// motion API, so a paused creature pushes nothing.

// plantKind groups vegetation by how it moves.
type plantKind uint8

const (
	kindGrass plantKind = iota
	kindCrop
	kindBush
	kindTree
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

	contactRadius = 0.6 // a creature this close (tiles) pushes a plant
	contactPush   = 0.9 // push per tile/s of creature speed (fraction of the lean limit)
	contactCap    = 1.2 // the push saturates here
	contactMinSpd = 0.3 // tiles/s; slower creatures are standing
	// contactViewPad: creatures this many tiles past the view still push
	// plants (the view's own pad, the drawn reach and a margin).
	contactViewPad = visPad + int(drawnReach) + 2

	canopyLag   = 0.3 // s: a tree's canopy follows its trunk this late
	shakeFrames = 45  // a brushed tree shakes this long

	rainCap = 1400 // weather particle pool caps
	snowCap = 1100
)

// Lean limits per kind (radians) and the wind gain (radians per tile/s)
// at the call sites' reference sway amount.
var (
	leanLimits = [...]float64{kindGrass: 0.35, kindCrop: 0.22, kindBush: 0.12, kindTree: 0.06}
	leanGain   = [...]float64{kindGrass: 0.05, kindCrop: 0.035, kindBush: 0.018, kindTree: 0.012}
	leanRefAmt = [...]float64{kindGrass: 0.05, kindCrop: 0.055, kindBush: 0.035, kindTree: 0.022}
)

const (
	trunkLimit = 0.02
	trunkGain  = 0.0045
)

// envState is the environment's presentation state, kept on the Renderer.
type envState struct {
	// the weather this frame (from WorldView)
	kind                 weatherKind
	intensity            float64
	wx, wy, speed        float64 // world wind, tiles/s
	dx, dy               float64 // its direction (east when still)
	celerity             float64 // gust front speed, tiles/s
	h1, d1, h2, d2       float64 // spring gain and phase lag at each gust frequency
	snow, wet            float64
	season               uint8
	outdoorTemp          int
	roomGrid             []int
	roofs                []uint8 // roof flags per tile (SIM-018), may be empty
	terrW                int
	springs              map[uint32]*envSpring // contact springs by tile
	movers               []envMover
	treeBuf              []int32 // treeAt's PlantsIn buffer
	view                 envRect // visible world rectangle, tiles (set while drawing)
	drops                []weatherDrop
	seq                  uint64
	keyArt               *artCatalog
	keys                 map[seasonCacheKey]string
	flashBucket, flashAt uint64
	flashSeen            bool
	white, dot           *ebiten.Image
	verts                []ebiten.Vertex
	idx                  []uint16
	shadow               shadowState   // the shadow pass (shadow.go)
	roof                 roofDrawState // eaves and the Roof overlay (roof_draw.go)
	home                 homeDrawState // the Home overlay's cached outline (home_draw.go)
	// collapses are the roof-collapse stamps already shown as dust.
	collapses map[game.RoofCollapseView]bool
}

type envRect struct{ x0, y0, x1, y1 float64 }

func (v envRect) empty() bool { return v.x1 <= v.x0 || v.y1 <= v.y0 }

// envSpring is one plant's contact lean: a damped spring in fractions of
// its kind's lean limit, pulled towards whatever creature pushes it.
type envSpring struct {
	cx, cy, vx, vy float64 // lean and lean rate
	tx, ty         float64 // this frame's push target
	touch, was     bool
	shakeAt        uint64
	shaking        bool
}

// envMover is a creature moving this frame (tiles, tiles/s).
type envMover struct{ x, y, vx, vy, spd float64 }

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

func envTileKey(x, y int) uint32 { return uint32(y)<<16 | uint32(x)&0xffff }

func (r *Renderer) envTime() float64 { return float64(r.present) / 60 }

// --- wind ---------------------------------------------------------------------

// windAt is the local wind at world (x, y) in tiles per second: the
// world's wind plus travelling gusts.
func (r *Renderer) windAt(x, y float64) (vx, vy float64) {
	return r.env.field(x, y, r.envTime(), 0, false)
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
func (e *envState) field(px, py, t, ph float64, respond bool) (float64, float64) {
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

// setWeather reads this frame's weather from the world.
func (e *envState) setWeather(w game.WorldView) {
	e.kind = parseWeather(w.Weather)
	e.intensity = clamp01(w.WeatherIntensity)
	e.snow, e.wet = clamp01(w.SnowDepth), clamp01(w.Wetness)
	e.season = seasonIndex(w.Season)
	e.outdoorTemp = w.OutdoorTemp
	e.roomGrid, e.terrW, e.roofs = w.RoomGrid, w.TerrainW, w.RoofCells
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

// plantPose is how a plant (or grass tuft) at tile (x, y) leans this
// frame: wind plus any creature brushing through. scale is its growth
// scale; amount the call site's sway amount (taller crops sway more).
func (r *Renderer) plantPose(x, y int, kind plantKind, amount, scale float64) pose {
	ps := pose{px: 0.5, py: 1, sx: scale, sy: scale}
	e := &r.env
	fx, fy := float64(x), float64(y)
	if v := e.view; !v.empty() && (fx < v.x0-2 || fx > v.x1+1 || fy < v.y0-1 || fy > v.y1+3) {
		return ps // off screen
	}
	if kind > kindTree {
		kind = kindBush
	}
	t := r.envTime()
	h := tileHash(x, y)
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
	if len(e.springs) > 0 {
		if s := e.springs[envTileKey(x, y)]; s != nil {
			cx, cy = s.cx, s.cy
			if s.shaking {
				age := float64(r.present - s.shakeAt)
				shake = 0.8 * math.Exp(-age/14) * math.Sin(age*2*math.Pi/9)
			}
		}
	}
	L := leanLimits[kind]
	var leanX, leanY float64
	if kind == kindTree {
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
	ps.skew = -leanX                // GeoM skew: negative tips the top east
	ps.sy = scale * (1 - 0.6*leanY) // leaning south (towards the viewer) looks shorter
	if kind == kindGrass && e.snow > 0 && e.outdoors(x, y) {
		thr := 0.15 + 0.6*float64(h>>8&0xffff)/65536
		a := 1 - clamp01((e.snow-thr)/0.2)
		ps.sy *= 1 - 0.5*e.snow
		ps.cs = colorScale{1, 1, 1, float32(a)}
	}
	return ps
}

// --- creature contact -----------------------------------------------------------

// stepEnvironment advances springs and weather particles once per frame.
func (r *Renderer) stepEnvironment(w game.WorldView, pawns []game.PawnView) {
	e := &r.env
	if e.springs == nil {
		e.springs = map[uint32]*envSpring{}
	}
	e.setWeather(w)
	r.stepContacts(w, pawns)
	r.stepWeather()
	r.stepCollapses(w)
}

// stepCollapses puffs dust and debris once for every roof collapse stamp
// not seen before (SIM-018); stamps the view no longer lists are forgotten.
func (r *Renderer) stepCollapses(w game.WorldView) {
	e := &r.env
	if len(w.RoofCollapses) == 0 {
		e.collapses = nil
		return
	}
	seen := make(map[game.RoofCollapseView]bool, len(w.RoofCollapses))
	for _, c := range w.RoofCollapses {
		if !e.collapses[c] {
			seed := mix64(c.Tick ^ uint64(envTileKey(c.X, c.Y))<<20 ^ 0xc011)
			r.burst(partDust, float64(c.X)+0.5, float64(c.Y)+0.5, 12, seed)
			r.burst(partChip, float64(c.X)+0.5, float64(c.Y)+0.4, 4, seed^0x5a)
		}
		seen[c] = true
	}
	e.collapses = seen
}

// stepContacts indexes this frame's moving creatures, pushes the plants
// within contactRadius of them and integrates every live contact spring.
func (r *Renderer) stepContacts(w game.WorldView, pawns []game.PawnView) {
	e := &r.env
	e.movers = e.movers[:0]
	perSec := float64(r.mo.stepped) * 60 // tiles/tick -> on-screen tiles/s
	add := func(m moveState) {
		vx, vy := m.VX*perSec, m.VY*perSec
		if spd := math.Hypot(vx, vy); spd >= contactMinSpd {
			e.movers = append(e.movers, envMover{m.X, m.Y, vx, vy, spd})
		}
	}
	if perSec > 0 {
		// only creatures near the last frame's view push plants: a spring
		// settles in about a second, so one off screen is never seen
		l := &r.lod
		for _, p := range pawns {
			if !p.Embodied && l.inView(p.X, p.Y, contactViewPad, contactViewPad) {
				add(r.pawnMotion(p.ID))
			}
		}
		for i := range w.Animals {
			if an := &w.Animals[i]; l.inView(an.X, an.Y, contactViewPad, contactViewPad) {
				add(r.animalMotion(an.ID))
			}
		}
	}
	for _, m := range e.movers {
		for ty := int(math.Floor(m.y - contactRadius)); ty <= int(math.Ceil(m.y+contactRadius)); ty++ {
			for tx := int(math.Floor(m.x - contactRadius)); tx <= int(math.Ceil(m.x+contactRadius)); tx++ {
				if tx < 0 || ty < 0 {
					continue
				}
				e.push(tx, ty, m)
			}
		}
	}
	cd := 2 * springZeta * math.Sqrt(springK)
	for k, s := range e.springs {
		if s.touch && !s.was && r.treeAt(&w, int(k&0xffff), int(k>>16)) {
			s.shakeAt, s.shaking = r.present, true
			if e.season == seasonFall {
				r.dropLeaves(int(k&0xffff), int(k>>16))
			}
		}
		ax := springK*(s.tx-s.cx) - cd*s.vx
		ay := springK*(s.ty-s.cy) - cd*s.vy
		s.vx += ax * frameDT
		s.vy += ay * frameDT
		s.cx += s.vx * frameDT
		s.cy += s.vy * frameDT
		s.was, s.touch, s.tx, s.ty = s.touch, false, 0, 0
		if s.shaking && r.present-s.shakeAt >= shakeFrames {
			s.shaking = false
		}
		if !s.was && !s.shaking && math.Abs(s.cx) < 0.004 && math.Abs(s.cy) < 0.004 &&
			math.Abs(s.vx) < 0.02 && math.Abs(s.vy) < 0.02 {
			delete(e.springs, k)
		}
	}
}

// push leans the plant at tile (tx, ty) away from mover m: forward along
// its path and out to the side, never back towards it.
func (e *envState) push(tx, ty int, m envMover) {
	ox, oy := float64(tx)-m.x, float64(ty)-m.y
	d := math.Hypot(ox, oy)
	if d >= contactRadius {
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
	mag := math.Min(contactPush*m.spd*smoothstep01(1-d/contactRadius)*n/2, contactCap)
	k := envTileKey(tx, ty)
	s := e.springs[k]
	if s == nil {
		s = &envSpring{}
		e.springs[k] = s
	}
	if mag*mag > s.tx*s.tx+s.ty*s.ty {
		s.tx, s.ty = mag*px/n, mag*py/n
	}
	s.touch = true
}

// treeAt reports a tree on tile (x, y) (a brush shake), from the plants
// on that tile alone (Simulation.PlantsIn), never a pass over every plant.
func (r *Renderer) treeAt(w *game.WorldView, x, y int) bool {
	e := &r.env
	e.treeBuf = r.plantsOn(w, x, y, x, y, e.treeBuf[:0])
	for _, i := range e.treeBuf {
		if int(i) < len(w.Plants) && isTreePlant(w.Plants[i].Species) {
			return true
		}
	}
	return false
}

var autumnLeaves = []color.RGBA{{214, 140, 52, 255}, {178, 74, 44, 255}, {226, 184, 74, 255}, {160, 104, 48, 255}}

// dropLeaves lets one or two autumn leaves fall from a brushed tree.
func (r *Renderer) dropLeaves(x, y int) {
	seed := tileHash(x, y) ^ r.present*0x9E3779B97F4A7C15
	n := 1 + int(hash01(seed, 7)*2)
	from := len(r.parts)
	r.burst(partLeaf, float64(x)+0.5, float64(y)-0.55, n, seed)
	for i := from; i < len(r.parts); i++ {
		r.parts[i].clr = autumnLeaves[int(hash01(seed, uint64(i), 9)*float64(len(autumnLeaves)))%len(autumnLeaves)]
		r.parts[i].life += 30 // leaves flutter down slowly
		r.parts[i].g *= 0.5
	}
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

// seasonKey swaps a plant sprite for its seasonal look (autumn, bare)
// and resolves it to art that exists: the variant, then variant 0, then
// the pre-variant sprite. Results are cached per key and season.
func (r *Renderer) seasonKey(key string, w game.WorldView) string {
	e := &r.env
	if e.keys == nil || e.keyArt != r.art {
		e.keys, e.keyArt = map[seasonCacheKey]string{}, r.art
	}
	ck := seasonCacheKey{key, seasonIndex(w.Season)}
	if v, ok := e.keys[ck]; ok {
		return v
	}
	var cands []string
	if ck.season != seasonNone {
		if s, ok := plantKeys.seasonal[key]; ok {
			cands = append(cands, s[ck.season-1])
		}
	}
	cands = append(cands, key)
	cands = append(cands, plantKeys.fallback[key]...)
	out := cands[0]
	for _, c := range cands {
		if r.art.has(c) {
			out = c
			break
		}
	}
	e.keys[ck] = out
	return out
}

// Crop stages as drawn (the sim's 0..3 plus sprout and wilted).
const (
	stSown = iota
	stSprout
	stYoung
	stMature
	stHarvestable
	stWilted
	numCropStages
)

var cropStageNames = [numCropStages]string{"sown", "sprout", "young", "mature", "harvestable", "wilted"}

var treeSpecies = [...]string{"oak", "birch", "pine"}

// plantKeyTable holds every plant sprite key, built once so drawing never
// concatenates strings.
type plantKeyTable struct {
	crops    map[string]*[numCropStages][3]string
	trees    [3][3]string
	bushes   [4][2]string // [variant][harvestable]
	berry    [3][2]string
	fallback map[string][]string
	seasonal map[string][2]string // autumn, bare
}

var plantKeys = buildPlantKeys()

func buildPlantKeys() plantKeyTable {
	t := plantKeyTable{crops: map[string]*[numCropStages][3]string{}, fallback: map[string][]string{}, seasonal: map[string][2]string{}}
	for _, c := range game.Crops() {
		ks := &[numCropStages][3]string{}
		for st, name := range cropStageNames {
			legacy := "plant/" + c.Name + "_" + name
			switch st {
			case stSown, stSprout:
				legacy = "plant/crop_sown"
			case stWilted:
				legacy = "plant/" + c.Name + "_mature"
			}
			for v := 0; v < 3; v++ {
				k := fmt.Sprintf("plant/%s_%s_v%d", c.Name, name, v)
				ks[st][v] = k
				var fb []string
				if v != 0 {
					fb = append(fb, fmt.Sprintf("plant/%s_%s_v0", c.Name, name))
				}
				if st == stSprout {
					fb = append(fb, fmt.Sprintf("plant/%s_sown_v%d", c.Name, v))
					if v != 0 {
						fb = append(fb, "plant/"+c.Name+"_sown_v0")
					}
				}
				t.fallback[k] = append(fb, legacy)
			}
		}
		t.crops[c.Name] = ks
	}
	for s, sp := range treeSpecies {
		for v := 0; v < 3; v++ {
			k := artKey("plant", "tree_"+sp, v)
			t.trees[s][v] = k
			legacy := artKey("plant", "tree", v)
			t.fallback[k] = []string{legacy}
			if sp != "pine" {
				t.seasonal[k] = [2]string{k + "_autumn", k + "_bare"}
				t.fallback[k+"_autumn"] = []string{k, legacy}
				t.fallback[k+"_bare"] = []string{k, legacy}
			}
		}
	}
	for v := 0; v < 4; v++ {
		legacy := artKey("plant", "bush", v%2)
		k := artKey("plant", "bush", v)
		t.bushes[v] = [2]string{k, k + "_harvestable"}
		if k != legacy {
			t.fallback[k] = []string{legacy}
			t.fallback[k+"_harvestable"] = []string{legacy + "_harvestable"}
		}
		if v < 3 {
			b := artKey("plant", "berry_bush", v)
			t.berry[v] = [2]string{b, b + "_harvestable"}
			t.fallback[b] = []string{legacy}
			t.fallback[b+"_harvestable"] = []string{legacy + "_harvestable"}
		}
	}
	addBiomePlantFallbacks(&t)
	return t
}

// cropStage is a crop's drawn stage.
func cropStage(p game.PlantView) int {
	if p.Wilted {
		return stWilted
	}
	switch clampStage(p.Stage) {
	case 0:
		if p.Growth >= 8 {
			return stSprout
		}
		return stSown
	case 1:
		return stYoung
	case 2:
		return stMature
	}
	return stHarvestable
}

// treeSpeciesAt is the tree species growing on a tile: mostly oak, then
// birch, some pine.
func treeSpeciesAt(h uint64) int {
	switch v := mix64(h^0x7ee5) % 20; {
	case v < 9:
		return 0
	case v < 15:
		return 1
	}
	return 2
}

func boolIdx(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- weather particles ------------------------------------------------------------

type dropKind uint8

const (
	dropRain dropKind = iota
	dropSnow
	dropSplash  // a raindrop landing on open ground
	dropRipple  // a raindrop landing in a puddle
	dropSettled // a snowflake resting on the ground before it melts into the cover
)

// weatherDrop is one weather particle: a falling drop or flake headed for
// ground point (gx, gy), or a short landing effect there.
type weatherDrop struct {
	kind              dropKind
	v                 uint8 // sprite variant
	age, life         uint16
	gx, gy            float64 // landing point, world tiles
	t, t0             float64 // seconds until it lands; its whole fall
	vx, vy, vz        float64 // drift with the wind and fall speed, tiles/s
	size              float32 // radius, tiles
	amp, omega, phase float32 // snow flutter
	alpha             float32
}

// screenVel is the drop's velocity on screen (tiles/s): its drift plus the
// fall, which projects to straight down.
func (d *weatherDrop) screenVel() (float64, float64) { return d.vx, d.vy + d.vz }

// posAt is where a falling drop is drawn t seconds before it lands: its
// ground point back along its drift, raised by its height.
func (d *weatherDrop) posAt(t float64) (float64, float64) {
	x := d.gx - d.vx*t
	if d.kind == dropSnow {
		x += float64(d.amp) * (math.Sin(float64(d.omega)*t+float64(d.phase)) - math.Sin(float64(d.phase)))
	}
	return x, d.gy - (d.vy+d.vz)*t
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
func (r *Renderer) stepWeather() {
	e := &r.env
	cap := rainCap
	if e.kind.frozen() {
		cap = snowCap
	}
	want := 0
	if e.kind.falling() && !e.view.empty() {
		area := (e.view.x1 - e.view.x0) * (e.view.y1 - e.view.y0)
		want = min(int(e.kind.density()*e.intensity*area*r.fxOr1()), cap*85/100)
	}
	room := cap - want // landing effects share what the drops leave
	live, effects := 0, 0
	kept := e.drops[:0]
	for _, d := range e.drops {
		switch d.kind {
		case dropRain, dropSnow:
			d.t -= frameDT
			if d.t > 0 {
				live++
				kept = append(kept, d)
				continue
			}
			if effects >= room {
				continue
			}
			x, y := int(math.Floor(d.gx)), int(math.Floor(d.gy))
			if !e.outdoors(x, y) { // roofed over since it spawned: nothing lands
				continue
			}
			d.age, d.t = 0, 0
			switch {
			case d.kind == dropSnow:
				d.kind, d.life = dropSettled, 40
			case e.wet > 0 && func() bool { _, a := puddle(x, y, e.wet); return a > 0 }():
				d.kind, d.life = dropRipple, 26
			default:
				d.kind, d.life = dropSplash, 9
			}
		default:
			d.age++
			if d.age >= d.life {
				continue
			}
		}
		effects++
		kept = append(kept, d)
	}
	e.drops = kept
	if live >= want {
		return
	}
	for n := min(want-live, max(want/8, 8)); n > 0 && len(e.drops) < cap; n-- {
		e.spawn()
	}
}

// spawn adds one falling drop or flake over open ground in the view.
func (e *envState) spawn() {
	v := e.view
	for try := 0; try < 3; try++ {
		e.seq++
		h := func(k uint64) float64 { return hash01(e.seq, k, 0x5eed) }
		gx := v.x0 - 1 + h(1)*(v.x1-v.x0+2)
		gy := v.y0 - 0.5 + h(2)*(v.y1-v.y0+3)
		if !e.outdoors(int(math.Floor(gx)), int(math.Floor(gy))) {
			continue
		}
		d := weatherDrop{gx: gx, gy: gy}
		if e.kind.frozen() {
			d.kind = dropSnow
			d.size = [...]float32{0.018, 0.018, 0.026, 0.026, 0.036, 0.05}[int(h(3)*6)%6]
			d.v = uint8(h(4) * 4)
			d.vz = 0.7 + 8*float64(d.size)
			jit := 0.85 + 0.3*h(5)
			d.vx, d.vy = e.wx*jit, e.wy*jit
			d.amp = float32(0.08 + 0.12*h(6))
			d.omega = float32(3 + 2*h(7))
			d.phase = float32(2 * math.Pi * h(8))
			d.t0 = (1.5 + 4.5*h(9)) / d.vz
			d.alpha = float32(0.7 + 0.3*h(10))
		} else {
			d.kind = dropRain
			d.vz = 13 + 4*h(3)
			jit := 1.5 * (0.9 + 0.2*h(5)) // windage, exaggerated a little for readability
			d.vx, d.vy = e.wx*jit, e.wy*jit*0.6
			d.t0 = 0.1 + 0.25*h(9)
			d.alpha = float32(0.45 + 0.3*h(10))
		}
		d.t = d.t0
		e.drops = append(e.drops, d)
		return
	}
}

// outdoors reports ground open to the sky: weather falls only there. With
// roofs (SIM-018) that is any unroofed tile but a wall; without roof data,
// any tile outside a room.
func (e *envState) outdoors(x, y int) bool {
	if len(e.roomGrid) == 0 || e.terrW <= 0 {
		return true
	}
	if x < 0 || y < 0 || x >= e.terrW {
		return true
	}
	i := y*e.terrW + x
	if i >= len(e.roomGrid) {
		return true
	}
	if len(e.roofs) == len(e.roomGrid) {
		return e.roomGrid[i] != -2 && !roofed(e.roofs[i])
	}
	return e.roomGrid[i] == -1
}

// lightningAt reports a lightning flash starting in tick bucket b: rare,
// storms only, and the same every run.
func lightningAt(kind string, intensity float64, b uint64) bool {
	return kind == "storm" && hash01(b, 0x11647) < 0.012*clamp01(intensity)
}

const lightningBucket = 20 // ticks

// weatherVeil is the sky's tint over the scene (darkening, premultiplied)
// and the fog that cuts visibility in snow.
func weatherVeil(w game.WorldView) (veil, fog color.RGBA) {
	i := clamp01(w.WeatherIntensity)
	pm := func(r, g, b uint8, a float64) color.RGBA {
		a = clamp01(a)
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

var (
	snowCoverKeys = [2]string{artKey("weather", "snow_cover", 4), artKey("weather", "snow_cover", 5)} // tileable
	snowBankKeys  = [4]string{}                                                                       // N, E, S, W edge masks
	snowDriftKeys = [4]string{}
	puddleKeys    = [6]string{}
	frostKeys     = [4]string{}
	splashKeys    = [3]string{}
	flakeKeys     = [4]string{}
)

func init() {
	for i := range puddleKeys {
		puddleKeys[i] = artKey("weather", "puddle", i)
	}
	for i := range snowDriftKeys {
		snowBankKeys[i] = artKey("weather", "snow_cover", i)
		snowDriftKeys[i] = artKey("weather", "snow_drift", i)
		frostKeys[i] = artKey("weather", "frost", i)
		flakeKeys[i] = artKey("weather", "snowflake", i)
	}
	for i := range splashKeys {
		splashKeys[i] = artKey("weather", "splash", i)
	}
}

// snowCover is the snow decal on a tile (a tileable cover) and its
// opacity at this depth. Ground fills in at its own depth, smoothly
// across several tiles, so early snow lies in soft patches rather than a
// checkerboard.
func snowCover(x, y int, depth float64) (string, float64) {
	return snowCoverKeys[mix64(tileHash(x, y)^0x51)%2], snowField(float64(x)+0.5, float64(y)+0.5, depth)
}

// snowField is the snow cover's opacity at world point (fx, fy).
func snowField(fx, fy, depth float64) float64 {
	if depth <= 0 {
		return 0
	}
	return clamp01((depth - 0.3*valueNoise(fx/5, fy/5, 0x5000)) / 0.4)
}

// valueNoise is smooth 2D noise in [0, 1): hashed lattice values eased
// between, so neighbouring points get similar values.
func valueNoise(fx, fy float64, salt uint64) float64 {
	ix, iy := math.Floor(fx), math.Floor(fy)
	u, v := smoothstep01(fx-ix), smoothstep01(fy-iy)
	at := func(dx, dy float64) float64 { return hash01(uint64(int64(ix+dx)), uint64(int64(iy+dy)), salt) }
	top := at(0, 0) + (at(1, 0)-at(0, 0))*u
	bot := at(0, 1) + (at(1, 1)-at(0, 1))*u
	return top + (bot-top)*v
}

// snowDrift is a drift heaped on some tiles once the snow is deep.
func snowDrift(x, y int, depth float64) (string, float64) {
	th := tileHash(x, y)
	if depth <= 0.4 || hash01(th, 0xd21f) > 0.18 {
		return "", 0
	}
	return snowDriftKeys[mix64(th^0xd2)%4], clamp01((depth - 0.4) / 0.3)
}

// frostStrength is how strongly rime shows (0..0.35): on freezing
// mornings without snow, colder is whiter, gone by mid-morning.
func frostStrength(tempTenths int, snow float64, d clock.Date) float64 {
	hour := float64(d.Hour) + float64(d.Minute)/60
	if tempTenths > 0 || snow >= 0.2 || hour < 3 || hour >= 10 {
		return 0
	}
	return 0.35 * clamp01(float64(-tempTenths)/50) * clamp01((10-hour)/4)
}

// puddle is the puddle on a tile and its opacity at this wetness: more
// tiles hold water as the ground gets wetter.
func puddle(x, y int, wet float64) (string, float64) {
	th := tileHash(x, y)
	lim := 0.35 * wet
	h := hash01(th, 0x9dd1)
	if h >= lim {
		return "", 0
	}
	return puddleKeys[mix64(th^0x9d)%6], clamp01((lim - h) / 0.06)
}

// setView records the visible world rectangle in tiles.
func (r *Renderer) setView(sw, sh int, tile float64) {
	if tile <= 0 {
		return
	}
	r.env.view = envRect{r.cam.offsetX / tile, r.cam.offsetY / tile,
		(r.cam.offsetX + float64(sw)) / tile, (r.cam.offsetY + float64(sh)) / tile}
}

// visibleTiles is the on-map tile range in view.
func (r *Renderer) visibleTiles() (x0, y0, x1, y1 int) {
	v := r.env.view
	x0, y0 = max(int(math.Floor(v.x0)), 0), max(int(math.Floor(v.y0)), 0)
	x1, y1 = min(int(math.Ceil(v.x1)), r.game.Width()-1), min(int(math.Ceil(v.y1)), r.game.Height()-1)
	return
}

// drawGroundCover draws snow cover and puddles over the terrain: first a
// seamless layer (wet darkening, a snow veil shaded smoothly corner to
// corner, drawn puddles when their art is missing) in one batch, then
// the per-tile decals for texture.
func (r *Renderer) drawGroundCover(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	e := &r.env
	snow, wet := clamp01(w.SnowDepth), clamp01(w.Wetness)
	frost := frostStrength(w.OutdoorTemp, snow, r.game.Date())
	if snow <= 0 && wet <= 0 && frost <= 0 {
		return
	}
	e.setWeather(w)
	x0, y0, x1, y1 := r.visibleTiles()
	open := func(x, y int) bool { return r.groundOpen(w, x, y) }
	veil := 0.85 // snow veil strength: all of it without decals, a base under them
	if r.art.has(snowCoverKeys[0]) && tile >= decalMinTile {
		veil = 0.5
	}
	e.beginBatch()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !open(x, y) {
				continue
			}
			sx, sy := r.worldToScreen(x, y)
			if wet > 0 { // wet ground darkens
				e.quad(float64(sx), float64(sy), tile, tile, color.RGBA{20, 24, 30, 255}, 0.16*wet)
			}
			if k, a := puddle(x, y, wet); a > 0 && snow < 0.5 && !r.art.has(k) {
				r.puddleShape(x, y, tile, a)
			}
			if snow > 0 {
				fx, fy := float64(x), float64(y)
				e.quad4(float64(sx), float64(sy), tile, tile, color.RGBA{236, 240, 246, 255},
					veil*snowField(fx, fy, snow), veil*snowField(fx+1, fy, snow),
					veil*snowField(fx, fy+1, snow), veil*snowField(fx+1, fy+1, snow))
			}
		}
	}
	e.flush(screen, e.whiteImg())
	if tile < decalMinTile { // zoomed out: the veil alone (lod.go)
		return
	}
	none := color.NRGBA{}
	decal := func(k string, x, y int, a float64) {
		if a > 0 && r.art.has(k) {
			r.drawSpritePose(screen, k, none, float64(x), float64(y), tile, pose{cs: colorScale{1, 1, 1, float32(a)}})
		}
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !open(x, y) {
				continue
			}
			if snow < 0.5 {
				k, a := puddle(x, y, wet)
				decal(k, x, y, a)
			}
			if snow > 0 {
				k, a := snowCover(x, y, snow)
				// texture where the snow lies full; thin snow is the smooth veil alone
				decal(k, x, y, 0.8*smoothstep01((a-0.6)/0.4))
				for d, n := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
					if !open(x+n[0], y+n[1]) { // snow banks up against walls and rock
						decal(snowBankKeys[d], x, y, a)
					}
				}
				k, a = snowDrift(x, y, snow)
				decal(k, x, y, a)
			}
			if frost > 0 {
				a := frost * smoothstep01((0.6-valueNoise(float64(x)/4, float64(y)/4, 0xf505))/0.4)
				decal(frostKeys[mix64(tileHash(x, y)^0xf5)%4], x, y, a)
			}
		}
	}
}

// groundOpen reports ground that takes snow cover, drifts, puddles and
// frost: open to the sky (not roofed, not a wall) and not rock (rough or
// smoothed).
func (r *Renderer) groundOpen(w game.WorldView, x, y int) bool {
	return r.env.outdoors(x, y) && !isRockWallName(r.terrainName(w, x, y))
}

// puddleShape is a drawn puddle when its art is missing: a dark ellipse
// with a lighter band of reflected sky.
func (r *Renderer) puddleShape(x, y int, tile, a float64) {
	h := tileHash(x, y)
	cx := float64(x) + 0.35 + 0.3*hash01(h, 1)
	cy := float64(y) + 0.4 + 0.25*hash01(h, 2)
	rx := 0.22 + 0.14*hash01(h, 3)
	sx, sy := r.worldToScreenF(cx, cy)
	r.env.ellipse(float64(sx), float64(sy), rx*tile, rx*0.5*tile, color.RGBA{52, 64, 78, 255}, 0.55*a)
	r.env.ellipse(float64(sx)-rx*0.2*tile, float64(sy)-rx*0.12*tile, rx*0.55*tile, rx*0.18*tile, color.RGBA{170, 186, 204, 255}, 0.35*a)
}

// --- drawing ------------------------------------------------------------------------

// drawWeather draws falling rain or snow and the storm's darkening.
func (r *Renderer) drawWeather(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	e := &r.env
	veil, fog := weatherVeil(w)
	if veil.A > 0 {
		vector.FillRect(screen, 0, 0, float32(sw), float32(sh), veil, false)
	}
	if fog.A > 0 {
		vector.FillRect(screen, 0, 0, float32(sw), float32(sh), fog, false)
	}
	if len(e.drops) > 0 {
		r.drawDrops(screen, sw, sh, tile)
	}
	r.drawLightning(screen, w, sw, sh)
}

// maxFlashReduced caps the lightning flash with Reduced flashing on.
const maxFlashReduced = 0.15

// fxOr1 is the Options effects multiplier (1 for a renderer built
// without settings).
func (r *Renderer) fxOr1() float64 {
	if r.fx <= 0 {
		return 1
	}
	return r.fx
}

func (r *Renderer) drawLightning(screen *ebiten.Image, w game.WorldView, sw, sh int) {
	e := &r.env
	b := uint64(r.game.Tick()) / lightningBucket
	if lightningAt(w.Weather, w.WeatherIntensity, b) && (!e.flashSeen || b != e.flashBucket) {
		e.flashBucket, e.flashAt, e.flashSeen = b, r.present, true
		r.thunderAt = r.present + thunderDelay(b)
	}
	if !e.flashSeen {
		return
	}
	age := float64(r.present - e.flashAt)
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
	if r.reducedFlashing { // Options: accessibility
		a = math.Min(a, maxFlashReduced)
	}
	if a > 0.005 {
		vector.FillRect(screen, 0, 0, float32(sw), float32(sh), color.RGBA{uint8(235 * a), uint8(240 * a), uint8(255 * a), uint8(255 * a)}, false)
	}
}

// drawDrops draws the weather pool: rain as wind-slanted streaks fading
// to their tail (motion blur), flakes as soft dots or their sprites,
// splashes, ripples and settling flakes.
func (r *Renderer) drawDrops(screen *ebiten.Image, sw, sh int, tile float64) {
	e := &r.env
	const blur = 0.022 // s of motion blur in a streak
	inside := func(x, y float32) bool {
		return x > -40 && y > -40 && x < float32(sw)+40 && y < float32(sh)+40
	}
	rainClr := color.RGBA{196, 210, 228, 255}
	width := math.Max(1, 0.022*tile)
	// streaks, splash crowns and ripple rings share the white texture
	e.beginBatch()
	var flakes, splashes []int
	for i := range e.drops {
		d := &e.drops[i]
		switch d.kind {
		case dropRain:
			wx, wy := d.posAt(d.t)
			hx, hy := r.worldToScreenF(wx, wy)
			if !inside(hx, hy) {
				continue
			}
			vx, vy := d.screenVel()
			fade := float64(d.alpha) * clamp01((d.t0-d.t)/0.08)
			e.streak(float64(hx), float64(hy), vx*blur*tile, vy*blur*tile, width, rainClr, fade)
		case dropRipple:
			sx, sy := r.worldToScreenF(d.gx, d.gy)
			k := float64(d.age) / float64(d.life)
			rad := (0.05 + 0.2*k) * tile
			e.ring(float64(sx), float64(sy), rad, rad*0.45, math.Max(1, 0.015*tile), color.RGBA{214, 226, 240, 255}, 0.5*(1-k))
		case dropSplash:
			if r.art.has(splashKeys[0]) {
				splashes = append(splashes, i)
				continue
			}
			sx, sy := r.worldToScreenF(d.gx, d.gy)
			k := float64(d.age) / float64(d.life)
			for j := -1; j <= 1; j += 2 { // two droplets jump out
				px := float64(sx) + float64(j)*(0.03+0.09*k)*tile
				py := float64(sy) - (0.12*k-0.12*k*k)*tile
				e.quad(px-width/2, py-width/2, width, width, rainClr, 0.6*(1-k))
			}
		case dropSnow, dropSettled:
			flakes = append(flakes, i)
		}
	}
	e.flush(screen, e.whiteImg())
	for _, i := range splashes {
		d := &e.drops[i]
		sx, sy := r.worldToScreenF(d.gx, d.gy)
		k := float64(d.age) / float64(d.life)
		r.drawCentered(screen, splashKeys[int(d.v)%3], float64(sx), float64(sy), tile*(0.2+0.18*k), 0.75*(1-k))
	}
	if len(flakes) == 0 {
		return
	}
	useArt := r.art.has(flakeKeys[0])
	e.beginBatch()
	for _, i := range flakes {
		d := &e.drops[i]
		wx, wy := d.gx, d.gy
		a := float64(d.alpha)
		if d.kind == dropSnow {
			wx, wy = d.posAt(d.t)
			a *= clamp01((d.t0 - d.t) / 0.4)
		} else {
			a *= 1 - float64(d.age)/float64(d.life)
		}
		sx, sy := r.worldToScreenF(wx, wy)
		if !inside(sx, sy) {
			continue
		}
		rad := float64(d.size) * tile
		if useArt {
			r.drawCentered(screen, flakeKeys[int(d.v)%4], float64(sx), float64(sy), rad*5, a)
			continue
		}
		e.dotQuad(float64(sx), float64(sy), rad, a)
	}
	if !useArt {
		e.flush(screen, e.dotImg())
	}
}

// drawCentered draws a weather sprite centred on (sx, sy), size px wide.
func (r *Renderer) drawCentered(screen *ebiten.Image, key string, sx, sy, size, alpha float64) {
	img := r.art.image(key, color.NRGBA{})
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

func (e *envState) whiteImg() *ebiten.Image {
	if e.white == nil {
		img := ebiten.NewImage(3, 3)
		img.Fill(color.White)
		e.white = img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
	}
	return e.white
}

const dotPx = 16

// dotImg is a soft round dot (white, alpha falling off to the edge).
func (e *envState) dotImg() *ebiten.Image {
	if e.dot == nil {
		src := image.NewNRGBA(image.Rect(0, 0, dotPx, dotPx))
		for y := 0; y < dotPx; y++ {
			for x := 0; x < dotPx; x++ {
				d := math.Hypot(float64(x)+0.5-dotPx/2, float64(y)+0.5-dotPx/2) / (dotPx / 2)
				src.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * clamp01(1-d*d))})
			}
		}
		e.dot = ebiten.NewImageFromImage(src)
	}
	return e.dot
}

func (e *envState) beginBatch() { e.verts, e.idx = e.verts[:0], e.idx[:0] }

func (e *envState) vert(x, y, sx, sy float64, c color.RGBA, a float64) {
	e.verts = append(e.verts, ebiten.Vertex{DstX: float32(x), DstY: float32(y), SrcX: float32(sx), SrcY: float32(sy),
		ColorR: float32(c.R) / 255, ColorG: float32(c.G) / 255, ColorB: float32(c.B) / 255, ColorA: float32(clamp01(a))})
}

// quadIdx indexes the last four vertices as two triangles.
func (e *envState) quadIdx() {
	n := uint16(len(e.verts) - 4)
	e.idx = append(e.idx, n, n+1, n+2, n+1, n+3, n+2)
}

// room reports space for n more vertices under the 16-bit index limit.
func (e *envState) room(n int) bool { return len(e.verts)+n < 65000 }

// quad is a solid rectangle.
func (e *envState) quad(x, y, w, h float64, c color.RGBA, a float64) {
	e.quad4(x, y, w, h, c, a, a, a, a)
}

// quad4 is a rectangle shaded between per-corner alphas (top-left,
// top-right, bottom-left, bottom-right), so neighbours meet seamlessly.
func (e *envState) quad4(x, y, w, h float64, c color.RGBA, a00, a10, a01, a11 float64) {
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
func (e *envState) streak(hx, hy, dx, dy, width float64, c color.RGBA, a float64) {
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
func (e *envState) dotQuad(x, y, rad, a float64) {
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

// ellipse is a filled ellipse (a triangle fan).
func (e *envState) ellipse(cx, cy, rx, ry float64, c color.RGBA, a float64) {
	if !e.room(ellipseSegs + 1) {
		return
	}
	base := uint16(len(e.verts))
	e.vert(cx, cy, 1.5, 1.5, c, a)
	for i := 0; i < ellipseSegs; i++ {
		th := 2 * math.Pi * float64(i) / ellipseSegs
		e.vert(cx+rx*math.Cos(th), cy+ry*math.Sin(th), 1.5, 1.5, c, a)
	}
	for i := 0; i < ellipseSegs; i++ {
		e.idx = append(e.idx, base, base+1+uint16(i), base+1+uint16((i+1)%ellipseSegs))
	}
}

// ring is an elliptical outline of the given stroke width.
func (e *envState) ring(cx, cy, rx, ry, width float64, c color.RGBA, a float64) {
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

func (e *envState) flush(screen *ebiten.Image, src *ebiten.Image) {
	if len(e.idx) > 0 {
		screen.DrawTriangles(e.verts, e.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
	e.beginBatch()
}

// --- HUD ----------------------------------------------------------------------------

var windArrows = [8]string{"→", "↘", "↓", "↙", "←", "↖", "↑", "↗"}

// weatherLabel is the HUD weather readout, e.g. "Rain ↗ 2.1": the kind,
// an arrow the way the wind blows and its speed in tiles per second.
func weatherLabel(w game.WorldView) string {
	if w.Weather == "" {
		return ""
	}
	name := strings.ToUpper(w.Weather[:1]) + w.Weather[1:]
	s := math.Hypot(w.WindX, w.WindY)
	if s < 0.05 {
		return name + ", calm"
	}
	oct := int(math.Round(math.Atan2(w.WindY, w.WindX)/(math.Pi/4))) & 7
	return fmt.Sprintf("%s %s %.1f", name, windArrows[oct], s)
}

// drawWeatherLabel draws weatherLabel at (x, y). The debug font has no
// arrows, so the wind arrow is drawn as lines, pointing the true way.
func drawWeatherLabel(screen *ebiten.Image, x, y int, w game.WorldView) {
	lbl := weatherLabel(w)
	if lbl == "" {
		return
	}
	if strings.HasSuffix(lbl, ", calm") {
		drawLabel(screen, x, y, lbl)
		return
	}
	name, rest, _ := strings.Cut(lbl, " ")
	_, speed, _ := strings.Cut(rest, " ")
	drawLabel(screen, x, y, name)
	cx, cy := float32(x+len(name)*6+10), float32(y+8)
	ux, uy := unit(w.WindX, w.WindY)
	fx, fy := float32(ux), float32(uy)
	clr := color.RGBA{230, 230, 220, 255}
	tipX, tipY := cx+fx*6, cy+fy*6
	vector.StrokeLine(screen, cx-fx*6, cy-fy*6, tipX, tipY, 1.5, clr, true)
	for _, s := range [2]float32{1, -1} { // arrow head
		hx := tipX - fx*4 + s*-fy*3
		hy := tipY - fy*4 + s*fx*3
		vector.StrokeLine(screen, tipX, tipY, hx, hy, 1.5, clr, true)
	}
	drawLabel(screen, x+len(name)*6+20, y, speed)
}
