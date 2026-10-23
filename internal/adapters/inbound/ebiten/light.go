package ebiten

import (
	_ "embed"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
)

// Dynamic light (ART §6.1): a sun that rises in the east, crosses the
// southern sky and sets in the west, a low cool moon at night, indoor
// ambient under roofs and warm lamp light in lit rooms. Every world sprite
// is drawn through drawLit, which shades its normal map (made by
// pkg/spritenormals, a port of spritefusion-sprite-normals) with lit.kage.
// The light comes from the sim clock only, so captures are deterministic.

//go:embed lit.kage
var litKage []byte

// lightState is the light for one frame. Vectors point toward the light
// in screen space (+x right, +y down, +z toward the viewer); colours are
// linear multipliers (1 = the sprite's own colour).
type lightState struct {
	SunL, MoonL               [3]float32
	SunColor, MoonColor       [3]float32
	SkyAmbient, IndoorAmbient [3]float32
	LampColor                 [3]float32
	ShadowDir                 [2]float64 // unit ground direction shadows fall
	ShadowLen                 func(h float64) float64
	ShadowAlpha               float64 // 0 when no shadows
}

// Light model numbers (plan §C, tuned against lit.kage; a flat normal at
// a summer noon sums to ≈1.0, today's unlit look):
//   - The sun's peak elevation is 65° in summer, 50° in spring and fall
//     and 32° in winter; the moon peaks at 40°.
//   - Sun colour × intensity by hour: 06 (1, .62, .38)×.27, 08
//     (1, .88, .72)×.50, 12 (1, .98, .94)×.50, 16 (1, .88, .70)×.50,
//     18 (1, .58, .40)×.27, and none from 19 to 05. The plan's .30/.45/.55
//     were scaled to .27/.50/.50 because the y-foreshortened light vector
//     puts more of the sun on a flat normal (L.z ≈ .97 at a summer noon)
//     and the mornings read too dark at .45.
//   - Sky ambient: 00 (.19, .21, .29), 05 (.22, .22, .30), 06 (.34, .30,
//     .34), 08 (.50, .49, .50), 12 (.52, .52, .53), 16 (.50, .48, .49),
//     18 (.32, .27, .32), 19 (.22, .22, .30). The night keys are the
//     plan's +.02..+.03, so midnight under the moon reads ≈0.33, the top
//     of the 0.25–0.35 band (the multiply is on sRGB values, so a low
//     factor reads darker than its number).
//   - Indoor ambient: 00 (.12, .12, .14), 05 (.14, .14, .15), 06 (.24,
//     .22, .21), 08 (.36, .34, .32), 12 (.42, .40, .38), 16 (.36, .34,
//     .32), 18 (.22, .20, .20), 19 (.14, .14, .15).
//   - Moon (.55, .65, .90)×.22.
//   - Lamp (.78, .60, .36) at night (the ART §6.1 hue (.55, .42, .25)
//     ×1.42, so a lamp-lit room at midnight reads ≈0.75), easing to ×0.6
//     in full daylight so lit rooms at noon stay below the open sky.
//   - Weather dims the sun and moon by d = (1−f)·(0.5 + 0.5·intensity),
//     f = cloudy .75, rain .55, storm and snow .5, blizzard .4; ambient
//     moves 60%·d toward its grey and lifts by 25%·d (scattered light).
const (
	moonPeakEl     = 40.0 // degrees
	litShadowMinEl = 2.0  // degrees: below this a light casts no shadow
	// Shadow strength is physical: a shadow removes the light's direct
	// share of a flat ground's light, S/(A+S) (S the light's colour on the
	// ground, A the sky ambient), times litShadowGain. litShadowFloor
	// keeps long dawn and dusk shadows legible in clear weather (scaled by
	// how much weather dims the light); litShadowCap stops a hard noon.
	litShadowGain  = 0.95
	litShadowFloor = 0.34
	litShadowCap   = 0.62
	// litShadowFade is how long (hours) shadows take to fade in after the
	// light rises and out before it sets.
	litShadowFade   = 0.5
	litMoonFloor    = 0.12 // moonlight shadow floor (clear sky)
	litMoonCap      = 0.30
	litShadowMaxLen = 3.0 // tiles
)

var (
	moonColor = [3]float32{0.55 * 0.22, 0.65 * 0.22, 0.90 * 0.22}
	lampNight = [3]float32{0.78, 0.60, 0.36}
)

// lightKey is one colour keyframe of the day (hour 0..24).
type lightKey struct {
	h                 float64
	sun, sky, indoors [3]float32
}

func litScaled(r, g, b, k float32) [3]float32 { return [3]float32{r * k, g * k, b * k} }

var lightKeys = [...]lightKey{
	{0, [3]float32{}, [3]float32{0.19, 0.21, 0.29}, [3]float32{0.12, 0.12, 0.14}},
	{5, [3]float32{}, [3]float32{0.22, 0.22, 0.30}, [3]float32{0.14, 0.14, 0.15}},
	{6, litScaled(1.00, 0.62, 0.38, 0.27), [3]float32{0.34, 0.30, 0.34}, [3]float32{0.24, 0.22, 0.21}},
	{8, litScaled(1.00, 0.88, 0.72, 0.50), [3]float32{0.50, 0.49, 0.50}, [3]float32{0.36, 0.34, 0.32}},
	{12, litScaled(1.00, 0.98, 0.94, 0.50), [3]float32{0.52, 0.52, 0.53}, [3]float32{0.42, 0.40, 0.38}},
	{16, litScaled(1.00, 0.88, 0.70, 0.50), [3]float32{0.50, 0.48, 0.49}, [3]float32{0.36, 0.34, 0.32}},
	{18, litScaled(1.00, 0.58, 0.40, 0.27), [3]float32{0.32, 0.27, 0.32}, [3]float32{0.22, 0.20, 0.20}},
	{19, [3]float32{}, [3]float32{0.22, 0.22, 0.30}, [3]float32{0.14, 0.14, 0.15}},
	{24, [3]float32{}, [3]float32{0.19, 0.21, 0.29}, [3]float32{0.12, 0.12, 0.14}},
}

// weatherSun is how much of the sun each sky lets through at full
// intensity.
func weatherSun(weather string) float64 {
	switch weather {
	case "cloudy":
		return 0.75
	case "rain":
		return 0.55
	case "storm", "snow":
		return 0.5
	case "blizzard":
		return 0.4
	}
	return 1
}

// seasonMaxEl is the sun's noon elevation (degrees) on dayOfYear.
func seasonMaxEl(dayOfYear int) float64 {
	d := ((dayOfYear % clock.DaysPerYear) + clock.DaysPerYear) % clock.DaysPerYear
	switch {
	case d >= 8 && d <= 22:
		return 65 // summer
	case d >= 38 && d <= 52:
		return 32 // winter
	}
	return 50 // spring (0–7, 53–59) and fall (23–37)
}

// lightDir is the unit vector toward a light at elevation el and azimuth
// az (radians; 0 east, π/2 south, π west), y foreshortened by the 0.5
// shear of the projection.
func lightDir(el, az float64) [3]float64 {
	v := [3]float64{math.Cos(az) * math.Cos(el), 0.5 * math.Sin(az) * math.Cos(el), math.Sin(el)}
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	return [3]float64{v[0] / n, v[1] / n, v[2] / n}
}

func f32v(v [3]float64) [3]float32 { return [3]float32{float32(v[0]), float32(v[1]), float32(v[2])} }

func lerp3(a, b [3]float32, t float32) [3]float32 {
	return [3]float32{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

func scale3(a [3]float32, k float32) [3]float32 { return [3]float32{a[0] * k, a[1] * k, a[2] * k} }

func lum3(a [3]float32) float32 { return 0.2126*a[0] + 0.7152*a[1] + 0.0722*a[2] }

// lightAt is the light at hour (0..24) on dayOfYear in the given weather.
func lightAt(hour float64, dayOfYear int, weather string, intensity float64) lightState {
	hour = math.Mod(hour, 24)
	if hour < 0 {
		hour += 24
	}
	var ls lightState
	for i := 1; i < len(lightKeys); i++ {
		a, b := lightKeys[i-1], lightKeys[i]
		if hour <= b.h {
			t := float32((hour - a.h) / (b.h - a.h))
			ls.SunColor, ls.SkyAmbient, ls.IndoorAmbient = lerp3(a.sun, b.sun, t), lerp3(a.sky, b.sky, t), lerp3(a.indoors, b.indoors, t)
			break
		}
	}
	ls.MoonColor = moonColor

	maxEl := seasonMaxEl(dayOfYear) * math.Pi / 180
	var sunEl, moonEl float64
	if dayFrac := (hour - 6) / 12; dayFrac > 0 && dayFrac < 1 {
		sunEl = math.Sin(math.Pi*dayFrac) * maxEl
		ls.SunL = f32v(lightDir(sunEl, math.Pi*dayFrac))
	}
	if nightFrac := math.Mod(hour-18+24, 24) / 12; nightFrac > 0 && nightFrac < 1 {
		moonEl = math.Sin(math.Pi*nightFrac) * moonPeakEl * math.Pi / 180
		ls.MoonL = f32v(lightDir(moonEl, math.Pi*nightFrac))
	}

	// Lamps: full at night, eased toward ×0.6 as the sky brightens.
	day := (lum3(ls.SkyAmbient) - 0.18) / (0.52 - 0.18)
	day = min(max(day, 0), 1)
	ls.LampColor = scale3(lampNight, 1-0.4*day)

	clear := float32(1) // how much of the light reaches the ground through weather
	if f := weatherSun(weather); f < 1 {
		d := float32((1 - f) * (0.5 + 0.5*min(max(intensity, 0), 1)))
		clear = 1 - d
		ls.SunColor = scale3(ls.SunColor, 1-d)
		ls.MoonColor = scale3(ls.MoonColor, 1-d)
		grey := func(a [3]float32) [3]float32 {
			g := (a[0] + a[1] + a[2]) / 3
			return scale3(lerp3(a, [3]float32{g, g, g}, 0.6*d), 1+0.25*d)
		}
		ls.SkyAmbient, ls.IndoorAmbient = grey(ls.SkyAmbient), grey(ls.IndoorAmbient)
	}

	// Shadows (plan §D) follow the active light: the sun by day, the moon
	// by night.
	ls.ShadowLen = func(float64) float64 { return 0 }
	el, l := sunEl, ls.SunL
	alpha := 0.0
	switch {
	case sunEl > 0:
		alpha = shadowStrength(ls.SunColor, ls.SunL, ls.SkyAmbient, litShadowFloor*clear, litShadowCap) * horizonFade(hour-6)
	case moonEl > 0:
		alpha = shadowStrength(ls.MoonColor, ls.MoonL, ls.SkyAmbient, litMoonFloor*clear, litMoonCap) * horizonFade(math.Mod(hour-18+24, 24))
		el, l = moonEl, ls.MoonL
	}
	if el*180/math.Pi <= litShadowMinEl {
		return ls
	}
	if g := math.Hypot(float64(l[0]), float64(l[1])); g > 1e-9 {
		ls.ShadowDir = [2]float64{-float64(l[0]) / g, -float64(l[1]) / g}
	}
	cot := 1 / math.Tan(el)
	ls.ShadowLen = func(h float64) float64 { return min(max(h*cot, 0.25*h), litShadowMaxLen) }
	ls.ShadowAlpha = alpha
	return ls
}

// horizonFade is 0..1 over the first and last litShadowFade hours of a
// light's 12-hour arc, given the hours since it rose.
func horizonFade(sinceRise float64) float64 {
	return min(max(min(sinceRise, 12-sinceRise)/litShadowFade, 0), 1)
}

// litFactor is lit.kage's light multiplier for a normal n (already
// decoded) on a tile with sky exposure and lamp light; the Go twin of the
// shader for tests and calibration.
func litFactor(ls *lightState, n [3]float32, sky, lamp float32) [3]float32 {
	dot := func(a, b [3]float32) float32 { return max(a[0]*b[0]+a[1]*b[1]+a[2]*b[2], 0) }
	ds, dm := dot(n, ls.SunL), dot(n, ls.MoonL)
	var out [3]float32
	for i := range out {
		amb := ls.IndoorAmbient[i] + (ls.SkyAmbient[i]-ls.IndoorAmbient[i])*sky
		dif := (ls.SunColor[i]*ds + ls.MoonColor[i]*dm) * sky
		out[i] = amb + dif + ls.LampColor[i]*lamp
	}
	return out
}

// decodeNormal is lit.kage's normal decode of a premultiplied texel
// (0..1): un-premultiply, map to -1..1, flip green (OpenGL green-up to
// screen y-down) and normalise. Without a normal map, or for a texel
// that decodes to nothing, the normal is flat (0, 0, 1).
func decodeNormal(texel [4]float32, has bool) [3]float32 {
	flat := [3]float32{0, 0, 1}
	if !has || texel[3] <= 1.0/255 {
		return flat
	}
	n := [3]float32{texel[0]/texel[3]*2 - 1, -(texel[1]/texel[3]*2 - 1), texel[2]/texel[3]*2 - 1}
	l := float32(math.Sqrt(float64(n[0]*n[0] + n[1]*n[1] + n[2]*n[2])))
	if l < 1e-3 {
		return flat
	}
	return [3]float32{n[0] / l, n[1] / l, n[2] / l}
}

// litShader is lit.kage, compiled on first use.
func (r *Renderer) litShader() *ebiten.Shader {
	if r.lit.shader == nil && r.lit.err == nil {
		r.lit.shader, r.lit.err = ebiten.NewShader(litKage)
	}
	return r.lit.shader
}

// litRenderer is the lighting state the Renderer keeps between frames.
type litRenderer struct {
	shader *ebiten.Shader
	err    error
	light  lightState // this frame's light (set at the start of drawArt)

	grid lightGrid
	// pairs holds, for each sprite with a normal map, one image with the
	// sprite at x 0 and its normal map litGap pixels to its right, so a
	// lit draw samples both from one texture and needs no per-sprite
	// uniforms (which would break Ebiten's batching).
	pairs    map[*ebiten.Image]*ebiten.Image
	verts    [4]ebiten.Vertex
	opts     ebiten.DrawTrianglesShaderOptions
	uniforms [7][]float32
	// onDraw, when set (tests), sees every lit draw's image and its
	// Custom3 blood value.
	onDraw func(img *ebiten.Image, blood float32)
	// calls counts world draw calls (lit sprites, water tiles, baked
	// chunk and map pages) for the draw benchmarks; it only ever grows.
	calls int
}

// litGap is the transparent column between a sprite and its normal map
// in a pair image, so bilinear taps at an edge never bleed across.
const litGap = 1

var litIndices = [6]uint32{0, 1, 2, 1, 3, 2}

// lightGrid is the per-tile sky exposure and lamp light, filled lazily a
// chunk (16×16 tiles) at a time: only the chunks a frame reads are built,
// and they are rebuilt only after the light's inputs change (NG-1 §6.7).
// The roof and room layers are shared copy-on-write slices (a new backing
// array means a change), and the lit rooms and burning campfires are
// compared by content, since the room and building lists are rebuilt
// every tick.
type lightGrid struct {
	w, h      int
	cw        int // chunks per row
	sky, lamp []float32
	litRooms  []bool
	fires     [][2]int // burning campfires
	// built[c] is the generation chunk c was last built at; gen moves
	// whenever an input changes, so every chunk goes stale at once and is
	// rebuilt when next read.
	built []uint64
	gen   uint64
	view  *game.WorldView // the view the chunks are built from
	// key is the view the inputs were last checked against (identity of
	// the shared layers and lists, plus their lengths); sum is the content
	// key (layer identities, lit rooms, fires) the generation stands for.
	keyRooms *int
	keyRoofs *uint8
	keyBlds  *game.BuildingView
	keyRm    *game.RoomView
	keyLens  [5]int
	sum      uint64
	genRooms *int
	genRoofs *uint8
}

func litFirst[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

// campfireReach and campfireFalloff: a burning campfire lights tiles
// within 3 tiles by 1 − dist/3.5.
const (
	campfireReach   = 3
	campfireFalloff = 3.5
)

// sync points g at w, moving the generation when the light's inputs
// differ from the last view's. The check is pointer and length compares
// while the view's lists are the same; a new list costs one scan of the
// rooms and buildings. It reuses g's slices, so it allocates only while
// growing.
func (g *lightGrid) sync(w *game.WorldView) {
	g.view = w
	lens := [5]int{len(w.RoomGrid), len(w.RoofCells), len(w.Buildings), len(w.Rooms), w.TerrainW}
	if g.built != nil && g.keyRooms == litFirst(w.RoomGrid) && g.keyRoofs == litFirst(w.RoofCells) &&
		g.keyBlds == litFirst(w.Buildings) && g.keyRm == litFirst(w.Rooms) && g.keyLens == lens {
		return
	}
	g.keyRooms, g.keyRoofs, g.keyBlds, g.keyRm, g.keyLens = litFirst(w.RoomGrid), litFirst(w.RoofCells), litFirst(w.Buildings), litFirst(w.Rooms), lens
	g.litRooms = g.litRooms[:0]
	sum := uint64(0x6c69676874)
	for _, rm := range w.Rooms {
		if rm.Lit && rm.ID >= 0 {
			for len(g.litRooms) <= rm.ID {
				g.litRooms = append(g.litRooms, false)
			}
			g.litRooms[rm.ID] = true
			sum = mix64(sum ^ uint64(rm.ID+1))
		}
	}
	g.fires = g.fires[:0]
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.Kind == "campfire" && b.Burning {
			g.fires = append(g.fires, [2]int{b.X, b.Y})
			sum = mix64(sum ^ uint64(b.X)<<20 ^ uint64(b.Y)<<40 ^ 0xf1)
		}
	}
	for _, n := range [3]int{lens[0], lens[1], lens[4]} { // layer sizes; the lists are compared by content
		sum = mix64(sum ^ uint64(n))
	}
	layersSame := g.genRooms == g.keyRooms && g.genRoofs == g.keyRoofs
	g.genRooms, g.genRoofs = g.keyRooms, g.keyRoofs
	w0, h0 := g.w, g.h
	g.w = w.TerrainW
	g.h = 0
	if g.w > 0 {
		g.h = max(len(w.RoomGrid), len(w.RoofCells), len(w.Terrain)) / g.w
	}
	g.cw = (g.w + chunkTiles - 1) / chunkTiles
	n := g.w * g.h
	nc := g.cw * ((g.h + chunkTiles - 1) / chunkTiles)
	if sum != g.sum || !layersSame || g.w != w0 || g.h != h0 || len(g.built) != nc {
		g.sum = sum
		g.gen++
		g.sky = litGrow(g.sky, n)
		g.lamp = litGrow(g.lamp, n)
		if cap(g.built) < nc {
			g.built = make([]uint64, nc)
		}
		g.built = g.built[:nc]
	}
}

// chunkTiles is the light grid's chunk side (game.ChunkSize).
const chunkTiles = game.ChunkSize

// base is tile i's own light: sky 0 under a built or natural roof, else
// 1; lamp 1 in lit rooms and up to 1 − dist/3.5 near burning campfires.
func (g *lightGrid) base(w *game.WorldView, i int) (sky, lamp float32) {
	const roofed = uint8(roof.Built | roof.Natural)
	sky = 1
	if i < len(w.RoofCells) && w.RoofCells[i]&roofed != 0 {
		sky = 0
	}
	if i < len(w.RoomGrid) {
		if id := w.RoomGrid[i]; id >= 0 && id < len(g.litRooms) && g.litRooms[id] {
			return sky, 1
		}
	}
	x, y := i%g.w, i/g.w
	for _, f := range g.fires {
		dx, dy := x-f[0], y-f[1]
		if dx < -campfireReach || dx > campfireReach || dy < -campfireReach || dy > campfireReach {
			continue
		}
		if d := math.Hypot(float64(dx), float64(dy)); d <= campfireReach {
			lamp = max(lamp, float32(1-d/campfireFalloff))
		}
	}
	return sky, lamp
}

// buildChunk fills chunk c. Boundary tiles (walls, doors and rock,
// RoomGrid −2) are lit as their open faces are: sky 1 when unroofed or
// next to an open-sky floor tile (an outer wall, a mountain face), lamp
// the brightest of their floor neighbours (a lit room's walls glow with
// it). Lighting every wall as indoors turned outer walls, doors and rock
// into near-black slabs at noon. Neighbours are read through base, so a
// chunk never depends on another being built.
func (g *lightGrid) buildChunk(c int) {
	w := g.view
	x0, y0 := (c%g.cw)*chunkTiles, (c/g.cw)*chunkTiles
	x1, y1 := min(x0+chunkTiles, g.w), min(y0+chunkTiles, g.h)
	walls := len(w.RoomGrid) >= g.w*g.h
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*g.w + x
			sky, lamp := g.base(w, i)
			if walls && w.RoomGrid[i] == room.Wall {
				// sky from the four sides; lamp also from the diagonals, so a
				// lit room's wall corners glow with the rest of its walls
				for k, d := range [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < 0 || ny < 0 || nx >= g.w || ny >= g.h {
						continue
					}
					j := ny*g.w + nx
					if w.RoomGrid[j] == room.Wall {
						continue
					}
					ns, nl := g.base(w, j)
					if k < 4 {
						sky = max(sky, ns)
					}
					lamp = max(lamp, nl)
				}
			}
			g.sky[i], g.lamp[i] = sky, lamp
		}
	}
	g.built[c] = g.gen
}

func litGrow(s []float32, n int) []float32 {
	if cap(s) < n {
		return make([]float32, n)
	}
	return s[:n]
}

// at is the light at (tx, ty); off the map is open sky without lamps.
func (g *lightGrid) at(tx, ty int) (sky, lamp float32) {
	if tx < 0 || ty < 0 || tx >= g.w || ty >= g.h {
		return 1, 0
	}
	if c := (ty/chunkTiles)*g.cw + tx/chunkTiles; g.built[c] != g.gen {
		g.buildChunk(c)
	}
	i := ty*g.w + tx
	return g.sky[i], g.lamp[i]
}

// lightTile is the light at tile (tx, ty): sky 1 under open sky, 0 under
// a roof; lamp 0..1 from lit rooms and nearby campfires.
func (r *Renderer) lightTile(tx, ty int) (sky, lamp float32) {
	r.lit.grid.sync(&r.drawW)
	return r.lit.grid.at(tx, ty)
}

// litQuad writes the four vertices (top-left, top-right, bottom-left,
// bottom-right) of a source rectangle (sx, sy, w, h) placed by geo: the
// colour is cs premultiplied as DrawImage's ColorScale is (rgb×a), and
// the custom attributes are sky (Custom0), lamp (Custom1) and the x
// offset in pixels from the sprite to its normal map in the source image
// (Custom2; 0 means none: lit flat). Custom3 (a figure's blood) is left
// 0; drawLit sets it from r.figureBlood.
func litQuad(vs *[4]ebiten.Vertex, sx, sy, w, h float32, geo ebiten.GeoM, cs colorScale, sky, lamp, normalOff float32) {
	cr, cg, cb, ca := cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3]
	for i := range vs {
		u, v := float32(i&1)*w, float32(i>>1)*h
		dx, dy := geo.Apply(float64(u), float64(v))
		vs[i] = ebiten.Vertex{
			DstX: float32(dx), DstY: float32(dy),
			SrcX: sx + u, SrcY: sy + v,
			ColorR: cr, ColorG: cg, ColorB: cb, ColorA: ca,
			Custom0: sky, Custom1: lamp, Custom2: normalOff,
		}
	}
}

// litSource is the image a lit draw of img samples and the x offset of
// its normal map in it (0: none, img itself is drawn). Ground planes
// (terrain and floors) are lit flat: they lie on the ground, and the
// generator's bevel dome on an opaque square tile showed as a quilt of
// pyramids, harshest under the low dawn and dusk sun.
func (r *Renderer) litSource(img *ebiten.Image) (*ebiten.Image, float32) {
	n := r.art.normalOf(img)
	if n == nil {
		return img, 0
	}
	b := img.Bounds()
	off := float32(b.Dx() + litGap)
	if p, ok := r.lit.pairs[img]; ok {
		if p == img {
			return img, 0
		}
		return p, off
	}
	if r.lit.pairs == nil {
		r.lit.pairs = map[*ebiten.Image]*ebiten.Image{}
	}
	if r.art.groundPlane(img) {
		r.lit.pairs[img] = img
		return img, 0
	}
	p := ebiten.NewImage(2*b.Dx()+litGap, b.Dy())
	p.DrawImage(img, nil)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(off), 0)
	p.DrawImage(n, op)
	r.lit.pairs[img] = p
	return p, off
}

// isGroundKey reports a ground-plane sprite: terrain tiles and edges,
// and floors.
func isGroundKey(key string) bool {
	return strings.HasPrefix(key, "terrain/") || strings.HasPrefix(key, "structure/floor")
}

// groundPlane reports whether img is a ground-plane sprite's image (plain
// or tinted). It scans the catalog, so callers cache the answer.
func (c *artCatalog) groundPlane(img *ebiten.Image) bool {
	if c == nil {
		return false
	}
	for key, sp := range c.sprites {
		if sp.img == img {
			return isGroundKey(key)
		}
	}
	for k, t := range c.tinted {
		if t == img {
			return isGroundKey(k.key)
		}
	}
	return false
}

// syncUniforms copies this frame's light into the reused uniform map.
func (l *litRenderer) syncUniforms() {
	if l.opts.Uniforms == nil {
		l.opts.Uniforms = map[string]any{}
		for i, name := range [...]string{"SunL", "MoonL", "SunColor", "MoonColor", "SkyAmbient", "IndoorAmbient", "LampColor"} {
			l.uniforms[i] = make([]float32, 3)
			l.opts.Uniforms[name] = l.uniforms[i]
		}
	}
	ls := &l.light
	for i, v := range [...]*[3]float32{&ls.SunL, &ls.MoonL, &ls.SunColor, &ls.MoonColor, &ls.SkyAmbient, &ls.IndoorAmbient, &ls.LampColor} {
		copy(l.uniforms[i], v[:])
	}
}

// drawLit draws a world sprite img with geo and colour scale cs, lit as
// the ground tile (tx, ty) is (its normal map comes from the catalog; a
// sprite without one is lit flat). Every draw uses the same shader and
// uniforms; per-sprite light rides in vertex attributes, so Ebiten merges
// consecutive draws from one atlas page into one batch.
func (r *Renderer) drawLit(dst, img *ebiten.Image, geo ebiten.GeoM, cs colorScale, tx, ty int) {
	if r.lod.baking { // a bake (lod.go): unlit albedo, lit when it is drawn
		r.drawUnlit(dst, img, geo, cs)
		return
	}
	if r.cast != nil { // the shadow pass: a silhouette, not a lit sprite
		r.castSilhouette(img, geo)
		return
	}
	if r.lit.onDraw != nil {
		r.lit.onDraw(img, r.figureBlood)
	}
	r.lit.calls++
	if cs == (colorScale{}) {
		cs = noCS
	}
	sh := r.litShader()
	if sh == nil { // shader failed to compile: the unlit draw
		op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
		if cs != noCS {
			op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
		}
		dst.DrawImage(img, op)
		return
	}
	src, off := r.litSource(img)
	sky, lamp := r.lightTile(tx, ty)
	b := img.Bounds()
	sx, sy := float32(b.Min.X), float32(b.Min.Y)
	if src != img {
		sx, sy = 0, 0
	}
	litQuad(&r.lit.verts, sx, sy, float32(b.Dx()), float32(b.Dy()), geo, cs, sky, lamp, off)
	for i := range r.lit.verts { // Custom3: the figure's blood (blood_draw.go), 0 for everything else
		r.lit.verts[i].Custom3 = r.figureBlood
	}
	r.lit.syncUniforms()
	r.lit.opts.Images[0] = src
	dst.DrawTrianglesShader32(r.lit.verts[:], litIndices[:], sh, &r.lit.opts)
}

// shadowStrength is a light's shadow alpha on flat ground: the share of
// the ground's light it supplies, S/(A+S), times litShadowGain, between
// floor and limit.
func shadowStrength(c, dir, ambient [3]float32, floor float32, limit float32) float64 {
	s := lum3(c) * max(dir[2], 0)
	a := lum3(ambient)
	if s+a <= 0 {
		return 0
	}
	return float64(min(max(litShadowGain*s/(a+s), floor), limit))
}
