// Package light is the renderer's sun, moon and lamp lighting: the light
// grid (sky exposure and lamp light per tile), the lit-sprite shader
// (lit.kage) and its pair images, the light sources, contact and cast
// shadows with their blur, daylight and the no-art sky overlay. It reads
// the frame's *render.View and its own state, never the renderer
// (SOLID_GO.md §7); the shadow pass asks the renderer for its casters and
// figure silhouettes through Casters.
package light

import (
	"container/list"
	_ "embed"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/weather"
)

// Dynamic light (ART §6.1): a sun that rises in the east, crosses the
// southern sky and sets in the west, a low cool moon at night, indoor
// ambient under roofs and warm point lamps (light_lamp.go: a falloff,
// stopped by walls, sprites shading toward the lamp). Every world sprite
// is drawn through Lit.Draw, which shades its normal map (made by
// pkg/spritenormals, a port of spritefusion-sprite-normals) with lit.kage.
// The light comes from the sim clock only, so captures are deterministic.

//go:embed lit.kage
var LitKage []byte

// State is the light for one frame. Vectors point toward the light
// in screen space (+x right, +y down, +z toward the viewer); colours are
// linear multipliers (1 = the sprite's own colour).
type State struct {
	SunL, MoonL               [3]float32
	SunColor, MoonColor       [3]float32
	SkyAmbient, IndoorAmbient [3]float32
	LampColor                 [3]float32
	// LampSky is the share of lamp light (lamps, campfires, fires) that
	// open sky drowns: 0 at night, 1 in full daylight; roofed tiles keep
	// it all (PLAY-020 §3b: outdoors by day the sun wins).
	LampSky     float32
	ShadowDir   [2]float64 // unit ground direction shadows fall
	ShadowLen   func(h float64) float64
	ShadowAlpha float64 // 0 when no shadows
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
	// LampSky ramps over the clear sky ambient's luminance: 0 through the
	// night keys (≈.21-.23), ≈.17 at 18:00, ≈.27 at 06:00, 1 from ≈08:00.
	lampSkyLo = 0.24
	lampSkyHi = 0.50
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

// WeatherSun is the share of the sun (0..1) the sky named sky lets
// through at intensity: weather.SunFactor, the factor solar panels make
// power by (game/power_gen.go), so a dim day and a weak panel agree.
func WeatherSun(sky string, intensity float64) float64 {
	return float64(weather.SunFactor(weather.KindOf(sky), intensity)) / 1000
}

// SeasonMaxEl is the sun's noon elevation (degrees) on dayOfYear.
func SeasonMaxEl(dayOfYear int) float64 {
	d := ((dayOfYear % clock.DaysPerYear) + clock.DaysPerYear) % clock.DaysPerYear
	switch {
	case d >= 8 && d <= 22:
		return 65 // summer
	case d >= 38 && d <= 52:
		return 32 // winter
	}
	return 50 // spring (0–7, 53–59) and fall (23–37)
}

// ProjShearK is the art projection's oblique shear: a point z tiles high
// is drawn ProjShearK*z tiles north. It mirrors SHEAR_K in
// assets/source/blender/artlib/contracts.py (metadata projection.shear_k).
const ProjShearK = 0.35

// LightDir is the unit vector toward a light at elevation el and azimuth
// az (radians; 0 east, π/2 south, π west), y foreshortened by the
// projection's shear.
func LightDir(el, az float64) [3]float64 {
	v := [3]float64{math.Cos(az) * math.Cos(el), ProjShearK * math.Sin(az) * math.Cos(el), math.Sin(el)}
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	return [3]float64{v[0] / n, v[1] / n, v[2] / n}
}

func F32v(v [3]float64) [3]float32 { return [3]float32{float32(v[0]), float32(v[1]), float32(v[2])} }

func Lerp3(a, b [3]float32, t float32) [3]float32 {
	return [3]float32{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

func scale3(a [3]float32, k float32) [3]float32 { return [3]float32{a[0] * k, a[1] * k, a[2] * k} }

func Lum3(a [3]float32) float32 { return 0.2126*a[0] + 0.7152*a[1] + 0.0722*a[2] }

// LightAt is the light at hour (0..24) on dayOfYear in the given weather.
func LightAt(hour float64, dayOfYear int, weather string, intensity float64) State {
	hour = math.Mod(hour, 24)
	if hour < 0 {
		hour += 24
	}
	var ls State
	for i := 1; i < len(lightKeys); i++ {
		a, b := lightKeys[i-1], lightKeys[i]
		if hour <= b.h {
			t := float32((hour - a.h) / (b.h - a.h))
			ls.SunColor, ls.SkyAmbient, ls.IndoorAmbient = Lerp3(a.sun, b.sun, t), Lerp3(a.sky, b.sky, t), Lerp3(a.indoors, b.indoors, t)
			break
		}
	}
	ls.MoonColor = moonColor

	maxEl := SeasonMaxEl(dayOfYear) * math.Pi / 180
	var sunEl, moonEl float64
	if dayFrac := (hour - 6) / 12; dayFrac > 0 && dayFrac < 1 {
		sunEl = math.Sin(math.Pi*dayFrac) * maxEl
		ls.SunL = F32v(LightDir(sunEl, math.Pi*dayFrac))
	}
	if nightFrac := math.Mod(hour-18+24, 24) / 12; nightFrac > 0 && nightFrac < 1 {
		moonEl = math.Sin(math.Pi*nightFrac) * moonPeakEl * math.Pi / 180
		ls.MoonL = F32v(LightDir(moonEl, math.Pi*nightFrac))
	}

	// Lamps: full at night, eased toward ×0.6 as the sky brightens.
	day := (Lum3(ls.SkyAmbient) - 0.18) / (0.52 - 0.18)
	day = min(max(day, 0), 1)
	ls.LampColor = scale3(lampNight, 1-0.4*day)
	ls.LampSky = min(max((Lum3(ls.SkyAmbient)-lampSkyLo)/(lampSkyHi-lampSkyLo), 0), 1)

	clear := float32(1) // how much of the light reaches the ground through weather
	if f := WeatherSun(weather, intensity); f < 1 {
		d := float32(1 - f)
		clear = 1 - d
		ls.SunColor = scale3(ls.SunColor, 1-d)
		ls.MoonColor = scale3(ls.MoonColor, 1-d)
		grey := func(a [3]float32) [3]float32 {
			g := (a[0] + a[1] + a[2]) / 3
			return scale3(Lerp3(a, [3]float32{g, g, g}, 0.6*d), 1+0.25*d)
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

// LitFactor is lit.kage's light multiplier for a normal n (already
// decoded) on a tile with sky exposure and lamp light (the packed vertex
// value, light_lamp.go); the Go twin of the shader for tests and
// calibration. Lamp light is level·(1 + n.xy·V)·(1 − LampSky·sky).
func LitFactor(ls *State, n [3]float32, sky, lamp float32) [3]float32 {
	dot := func(a, b [3]float32) float32 { return max(a[0]*b[0]+a[1]*b[1]+a[2]*b[2], 0) }
	ds, dm := dot(n, ls.SunL), dot(n, ls.MoonL)
	level, vx, vy := DecodeLamp(lamp)
	k := level * (1 + (n[0]*vx + n[1]*vy)) * (1 - ls.LampSky*sky)
	var out [3]float32
	for i := range out {
		amb := ls.IndoorAmbient[i] + (ls.SkyAmbient[i]-ls.IndoorAmbient[i])*sky
		dif := (ls.SunColor[i]*ds + ls.MoonColor[i]*dm) * sky
		out[i] = amb + dif + ls.LampColor[i]*k
	}
	return out
}

// DecodeNormal is lit.kage's normal decode of a premultiplied texel
// (0..1): un-premultiply, map to -1..1, flip green (OpenGL green-up to
// screen y-down) and normalise. Without a normal map, or for a texel
// that decodes to nothing, the normal is flat (0, 0, 1).
func DecodeNormal(texel [4]float32, has bool) [3]float32 {
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

// LitShader is lit.kage, compiled on first use; nil when it failed (Err).
func (l *Lit) LitShader() *ebiten.Shader {
	if l.Shader == nil && l.Err == nil {
		l.Shader, l.Err = ebiten.NewShader(LitKage)
	}
	return l.Shader
}

// Lit is the lighting state the renderer keeps between frames.
type Lit struct {
	Shader *ebiten.Shader
	Err    error
	Light  State // this frame's light (set at the start of drawArt)

	Grid Grid
	// Pairs holds, for each sprite with a normal map, one image with the
	// sprite at x 0 and its normal map LitGap pixels to its right, so a
	// lit draw samples both from one texture and needs no per-sprite
	// uniforms (which would break Ebiten's batching).
	Pairs    PairCache
	Verts    [4]ebiten.Vertex
	Opts     ebiten.DrawTrianglesShaderOptions
	Uniforms [8][]float32
	// radArt is the catalog the grid's lamp radius was read from (radOK:
	// read at all), so a new catalog re-reads light_radius.
	radArt *art.Catalog
	radOK  bool
	// Blaze, when set, fills the lamp light of the burning cells per tile
	// (the renderer's fire light, fire_world_draw.go), reusing buf; nil:
	// fires light nothing.
	Blaze func(buf []float32, fires []game.FireView, w, h int) []float32
	// Off turns the normal-map lighting Off (Options > Graphics, PLAY-019):
	// sprites draw plain with the daylight as a colour scale, and no pair
	// images are made.
	Off bool
	// plain is the reused option block of the plain (lighting off) draws.
	plain ebiten.DrawImageOptions
	// OnDraw, when set (tests), sees every lit draw's image and its
	// Custom3 blood value.
	OnDraw func(img *ebiten.Image, blood float32)
	// ground marks the next Draw as a ground plane (DrawGround).
	ground bool
	// Calls counts world draw Calls (lit sprites, water tiles, baked
	// chunk and map pages) for the draw benchmarks; it only ever grows.
	Calls int
}

// LitGap is the transparent column between a sprite and its normal map
// in a pair image, so bilinear taps at an edge never bleed across.
const LitGap = 1

var Indices = [6]uint32{0, 1, 2, 1, 3, 2}

// Grid is the per-tile sky exposure and lamp light, filled lazily a
// chunk (16×16 tiles) at a time: only the chunks a frame reads are built,
// and they are rebuilt only after the light's inputs change (NG-1 §6.7).
// The roof and room layers are shared copy-on-write slices (a new backing
// array means a change), and the light sources are compared by content,
// since the building list is rebuilt every tick.
type Grid struct {
	w, h int
	cw   int // chunks per row
	sky  []float32
	// lamp is the packed lamp value per tile (light_lamp.go): the light
	// level, and the direction toward the dominant lamp.
	lamp []float32
	// dom is the dominant lamp's source index per tile (-1: none) and
	// other the tile's light from campfires and burning cells, so a
	// sprite's lamp light can follow its foot between tile centres (AtF).
	dom   []int32
	other []float32
	// srcs are the light sources (light_sources.go) and srcRoom the room
	// id at each source's tile (-1 outdoors).
	srcs    []Source
	srcRoom []int
	// Sources lists the light Sources; nil is AppendSources (tests
	// substitute it to prove every light comes from that one helper).
	Sources func(dst []Source, w *game.WorldView, lampRadius float64) []Source
	lampR   float64   // a lamp's radius (its sprite's light_radius)
	near    []int32   // the sources reaching the chunk being built
	blaze   []float32 // lamp light of burning cells per tile (fire_world_draw.go)
	keyFire uint64    // WorldView.FireVer the blaze was built from
	// Built[c] is the generation chunk c was last Built at; gen moves
	// whenever an input changes, so every chunk goes stale at once and is
	// rebuilt when next read.
	Built []uint64
	Gen   uint64
	view  *game.WorldView // the view the chunks are built from
	// key is the view the inputs were last checked against (identity of
	// the shared layers and lists, plus their lengths); sum is the content
	// key (layer identities, sources) the generation stands for.
	keyRooms *int
	keyRoofs *uint8
	keyBlds  *game.BuildingView
	keyLens  [4]int
	keyR     float64
	sum      uint64
	genRooms *int
	genRoofs *uint8
}

func First[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}
	return &s[0]
}

// CampfireReach and campfireFalloff: a burning campfire lights tiles
// within 3 tiles by 1 − dist/3.5.
const (
	CampfireReach   = 3
	campfireFalloff = 3.5
)

// sync points g at w, moving the generation when the light's inputs
// differ from the last view's. The check is pointer and length compares
// while the view's lists are the same; a new list costs one scan of the
// buildings (AppendSources). blaze lights the burning cells (Lit.Blaze;
// nil: none). It reuses g's slices, so it allocates only while growing.
func (g *Grid) sync(w *game.WorldView, blaze func(buf []float32, fires []game.FireView, w, h int) []float32) {
	g.view = w
	lens := [4]int{len(w.RoomGrid), len(w.RoofCells), len(w.Buildings), w.TerrainW}
	if g.Built != nil && g.keyRooms == First(w.RoomGrid) && g.keyRoofs == First(w.RoofCells) &&
		g.keyBlds == First(w.Buildings) && g.keyLens == lens && g.keyFire == w.FireVer && g.keyR == g.lampR {
		return
	}
	g.keyRooms, g.keyRoofs, g.keyBlds, g.keyLens = First(w.RoomGrid), First(w.RoofCells), First(w.Buildings), lens
	g.keyFire, g.keyR = w.FireVer, g.lampR
	list := g.Sources
	if list == nil {
		list = AppendSources
	}
	g.srcs = list(g.srcs[:0], w, g.lampR)
	g.srcRoom = g.srcRoom[:0]
	sum := uint64(0x6c69676874)
	for _, s := range g.srcs {
		home := room.Outdoors
		if i := s.Y*w.TerrainW + s.X; s.X >= 0 && s.Y >= 0 && s.X < w.TerrainW && i < len(w.RoomGrid) {
			home = w.RoomGrid[i]
		}
		g.srcRoom = append(g.srcRoom, home)
		sum = art.Mix64(sum ^ uint64(s.Kind) ^ uint64(uint32(s.X))<<8 ^ uint64(uint32(s.Y))<<36)
		sum = art.Mix64(sum ^ math.Float64bits(s.Radius))
	}
	sum = art.Mix64(sum ^ w.FireVer ^ uint64(len(w.Fires))<<32 ^ 0xb1a2e)
	for _, n := range [3]int{lens[0], lens[1], lens[3]} { // layer sizes; the sources are compared by content
		sum = art.Mix64(sum ^ uint64(n))
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
	if blaze != nil {
		g.blaze = blaze(g.blaze, w.Fires, g.w, g.h)
	} else {
		g.blaze = g.blaze[:0]
	}
	nc := g.cw * ((g.h + chunkTiles - 1) / chunkTiles)
	if sum != g.sum || !layersSame || g.w != w0 || g.h != h0 || len(g.Built) != nc {
		g.sum = sum
		g.Gen++
		g.sky = litGrow(g.sky, n)
		g.lamp = litGrow(g.lamp, n)
		g.dom = litGrow(g.dom, n)
		g.other = litGrow(g.other, n)
		if cap(g.Built) < nc {
			g.Built = make([]uint64, nc)
		}
		g.Built = g.Built[:nc]
	}
}

// chunkTiles is the light grid's chunk side (game.ChunkSize).
const chunkTiles = game.ChunkSize

// selectNear keeps in g.near the sources whose reach box meets the tiles
// x0..x1, y0..y1, so a chunk's tiles loop over a few sources, not all.
func (g *Grid) selectNear(x0, y0, x1, y1 int) {
	g.near = g.near[:0]
	for j := range g.srcs {
		s := &g.srcs[j]
		r := int(math.Ceil(s.Radius))
		if s.X+r < x0 || s.X-r > x1 || s.Y+r < y0 || s.Y-r > y1 {
			continue
		}
		g.near = append(g.near, int32(j))
	}
}

// base is tile (x, y)'s own light: sky 0 under a built or natural roof,
// else 1; other the brightest of burning campfires (up to 1 − dist/3.5)
// and burning cells; lampLvl and k the brightest lamp's light (lampAt) and
// source index (-1: none). The tile's level is max(other, lampLvl). It
// reads only g.near.
func (g *Grid) base(w *game.WorldView, x, y int) (sky, level, lampLvl float32, k int) {
	const roofed = uint8(roof.Built | roof.Natural)
	i := y*g.w + x
	sky, k = 1, -1
	if i < len(w.RoofCells) && w.RoofCells[i]&roofed != 0 {
		sky = 0
	}
	if i < len(g.blaze) {
		level = g.blaze[i]
	}
	for _, j := range g.near {
		s := &g.srcs[j]
		switch s.Kind {
		case Lamp:
			if l := g.lampAt(int(j), x, y); l > lampLvl {
				lampLvl, k = l, int(j)
			}
		case Campfire:
			dx, dy := float64(x-s.X), float64(y-s.Y)
			if math.Abs(dx) > s.Radius || math.Abs(dy) > s.Radius {
				continue
			}
			if d := math.Hypot(dx, dy); d <= s.Radius {
				level = max(level, float32(1-d/campfireFalloff))
			}
		}
	}
	return sky, level, lampLvl, k
}

// buildChunk fills chunk c. Boundary tiles (walls, doors and rock,
// RoomGrid −2) are lit as their open faces are: sky 1 when unroofed or
// next to an open-sky floor tile (an outer wall, a mountain face), lamp
// the brightest of their floor neighbours (a lit room's walls glow with
// it), shading toward that neighbour's lamp. Lighting every wall as
// indoors turned outer walls, doors and rock into near-black slabs at
// noon. Neighbours are read through base, so a chunk never depends on
// another being built.
func (g *Grid) buildChunk(c int) {
	w := g.view
	x0, y0 := (c%g.cw)*chunkTiles, (c/g.cw)*chunkTiles
	x1, y1 := min(x0+chunkTiles, g.w), min(y0+chunkTiles, g.h)
	g.selectNear(x0-1, y0-1, x1, y1) // a tile wider: the walls' neighbours
	walls := len(w.RoomGrid) >= g.w*g.h
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*g.w + x
			sky, other, lampLvl, k := g.base(w, x, y)
			level := max(other, lampLvl)
			if walls && w.RoomGrid[i] == room.Wall {
				// sky from the four sides; lamp also from the diagonals, so a
				// lit room's wall corners glow with the rest of its walls
				for n, d := range [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < 0 || ny < 0 || nx >= g.w || ny >= g.h {
						continue
					}
					if w.RoomGrid[ny*g.w+nx] == room.Wall {
						continue
					}
					ns, nl, nll, nk := g.base(w, nx, ny)
					if n < 4 {
						sky = max(sky, ns)
					}
					level = max(level, nl, nll)
					if nll > lampLvl {
						lampLvl, k = nll, nk
					}
				}
			}
			g.sky[i], g.lamp[i] = sky, g.pack(x, y, level, lampLvl, k)
			g.dom[i], g.other[i] = int32(k), other
		}
	}
	g.Built[c] = g.Gen
}

func litGrow[T float32 | int32](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	return s[:n]
}

// At is the light At (tx, ty); off the map is open sky without lamps.
func (g *Grid) At(tx, ty int) (sky, lamp float32) {
	if tx < 0 || ty < 0 || tx >= g.w || ty >= g.h {
		return 1, 0
	}
	if c := (ty/chunkTiles)*g.cw + tx/chunkTiles; g.Built[c] != g.Gen {
		g.buildChunk(c)
	}
	i := ty*g.w + tx
	return g.sky[i], g.lamp[i]
}

// AtF is the light of a sprite whose foot is at (fx, fy) in tile
// coordinates (tile x spans x..x+1 and the grid measures from tile to
// tile, so a sprite anchored on its tile's corner, as buildings and
// standing colonists are, gets the grid's value): sky and the room test
// come from the tile under the foot, but the dominant lamp's light and
// direction follow the continuous distance, so a colonist walking past a
// lamp brightens and darkens smoothly instead of stepping at each tile.
// One packed value per sprite: its direction code is never interpolated.
// Boundary tiles (lit from their neighbours) and tiles no lamp leads keep
// the grid's value, as does a foot exactly on a tile's corner.
func (g *Grid) AtF(fx, fy float64) (sky, lamp float32) {
	tx, ty := int(math.Floor(fx)), int(math.Floor(fy))
	sky, lamp = g.At(tx, ty)
	if fx == float64(tx) && fy == float64(ty) || tx < 0 || ty < 0 || tx >= g.w || ty >= g.h {
		return sky, lamp
	}
	i := ty*g.w + tx
	k := int(g.dom[i])
	if k < 0 || g.srcs[k].Kind != Lamp || g.lampAt(k, tx, ty) <= 0 {
		return sky, lamp
	}
	s := &g.srcs[k]
	lc := lampFalloff(math.Hypot(fx-float64(s.X), fy-float64(s.Y)), s.Radius)
	return sky, g.packAt(fx, fy, max(g.other[i], lc), lc, k)
}

// GroundTile is a tile's plain lamp level as the ground's corners read it
// (Grid.levelRow): kind 0 open, 1 boundary (walls, doors, rock), 2
// off the map.
type GroundTile struct {
	l    float32
	kind uint8
}

// LevelRow is dst refilled with tiles x0-1..x1 of row y (x1-x0+2 values),
// each looked up once, for CornerRow.
func (g *Grid) LevelRow(dst []GroundTile, x0, x1, y int) []GroundTile {
	dst = dst[:0]
	if y < 0 || y >= g.h {
		for x := x0 - 1; x <= x1; x++ {
			dst = append(dst, GroundTile{kind: 2})
		}
		return dst
	}
	rg := g.view.RoomGrid
	walls := len(rg) >= g.w*g.h
	for x := x0 - 1; x <= x1; x++ {
		if x < 0 || x >= g.w {
			dst = append(dst, GroundTile{kind: 2})
			continue
		}
		_, lamp := g.At(x, y)
		t := GroundTile{l: lamp}
		if lamp >= 4 { // a packed direction: keep the level
			t.l = LampLevel(lamp)
		}
		if walls && rg[y*g.w+x] == room.Wall {
			t.kind = 1
		}
		dst = append(dst, t)
	}
	return dst
}

// CornerRow is dst refilled with the plain lamp levels at the top-left
// corners of tiles x0..x1+1 on the line between rows above (y-1) and below
// (y), both from levelRow over x0..x1+1. A corner is the mean of those of
// its four tiles that are on the map and open, so the ground's light ramps
// smoothly between tile centres but never bleeds past the far side of a
// wall; a corner inside solid boundary takes its brightest tile.
func CornerRow(dst []float32, above, below []GroundTile) []float32 {
	dst = dst[:0]
	for i := 1; i < len(above); i++ {
		var sum, peak float32
		n := 0
		for _, t := range [4]GroundTile{above[i-1], above[i], below[i-1], below[i]} {
			switch t.kind {
			case 0:
				sum += t.l
				n++
			case 1:
				peak = max(peak, t.l)
			}
		}
		if n > 0 {
			peak = sum / float32(n)
		}
		dst = append(dst, peak)
	}
	return dst
}

// GroundLevel is the plain lamp level of the ground at world point (px,
// py) in tile coordinates (tile x spans x..x+1): bilinear between the
// centres of the four tiles around it, dropping boundary (walls, doors,
// rock) and off-map tiles and renormalising, so it never bleeds past the
// far side of a wall; among boundary tiles only, the brightest. At a tile
// corner it is CornerRow's value, so live ground and the bakes agree.
func (g *Grid) GroundLevel(px, py float64) float32 {
	u, v := px-0.5, py-0.5
	x0, y0 := int(math.Floor(u)), int(math.Floor(v))
	fx, fy := float32(u-float64(x0)), float32(v-float64(y0))
	rg := g.view.RoomGrid
	walls := len(rg) >= g.w*g.h
	var sum, wsum, peak float32
	for k := 0; k < 4; k++ {
		x, y := x0+k&1, y0+k>>1
		wt := (1 - fx) * (1 - fy)
		switch k {
		case 1:
			wt = fx * (1 - fy)
		case 2:
			wt = (1 - fx) * fy
		case 3:
			wt = fx * fy
		}
		if wt <= 0 || x < 0 || y < 0 || x >= g.w || y >= g.h {
			continue
		}
		_, lamp := g.At(x, y)
		l := LampLevel(lamp)
		if walls && rg[y*g.w+x] == room.Wall {
			peak = max(peak, l)
			continue
		}
		sum += wt * l
		wsum += wt
	}
	if wsum > 0 {
		return sum / wsum
	}
	return peak
}

// GroundLamp sets each vertex's lamp light (Custom1) to the ground's plain
// level under it (GroundLevel): a ground plane is flat, so the direction
// adds nothing, and plain levels interpolate exactly across the quad. The
// vertices are in map pixels of v's camera.
func (l *Lit) GroundLamp(v *render.View, vs *[4]ebiten.Vertex) {
	g := l.Sync(v)
	tile := v.Cam.Tile()
	if tile <= 0 {
		return
	}
	for i := range vs {
		vs[i].Custom1 = g.GroundLevel((float64(vs[i].DstX)+v.Cam.OffsetX)/tile, (float64(vs[i].DstY)+v.Cam.OffsetY)/tile)
	}
}

// LightFoot is the light of a sprite whose foot is at world (wx, wy)
// (Grid.AtF).
func (l *Lit) LightFoot(v *render.View, wx, wy float64) (sky, lamp float32) {
	return l.Sync(v).AtF(wx, wy)
}

// Sync points the light grid at this frame's world (v.W), with the lamp
// radius from the art's metadata (v.Art, re-read when the catalog
// changes), and returns it.
func (l *Lit) Sync(v *render.View) *Grid {
	g := &l.Grid
	if !l.radOK || l.radArt != v.Art {
		l.radArt, l.radOK = v.Art, true
		g.lampR = LampRadiusOf(v.Art)
	}
	g.sync(&v.W, l.Blaze)
	return g
}

// LightTile is the light at tile (tx, ty): sky 1 under open sky, 0 under
// a roof; lamp the packed lamp value (light_lamp.go) from lamps, nearby
// campfires and burning cells, passed through as the Custom1 vertex value.
func (l *Lit) LightTile(v *render.View, tx, ty int) (sky, lamp float32) {
	return l.Sync(v).At(tx, ty)
}

// LitQuad writes the four vertices (top-left, top-right, bottom-left,
// bottom-right) of a source rectangle (sx, sy, w, h) placed by geo: the
// colour is cs premultiplied as DrawImage's ColorScale is (rgb×a), and
// the custom attributes are sky (Custom0), lamp (Custom1) and the x
// offset in pixels from the sprite to its normal map in the source image
// (Custom2; 0 means none: lit flat; negative: drawn mirrored, the normal's
// x inverted). Custom3 (a figure's blood) is left 0; Draw sets it from
// its blood argument.
func LitQuad(vs *[4]ebiten.Vertex, sx, sy, w, h float32, geo ebiten.GeoM, cs [4]float32, sky, lamp, normalOff float32) {
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

// MirrorNormalOff is the normal-map offset a lit draw passes (Custom2):
// negative when geo mirrors the sprite (a west-facing figure is its east
// art flipped), which tells lit.kage to invert the normal's x so the
// sprite is still lit from the light's side (DecodeNormalMirrored).
func MirrorNormalOff(off float32, geo ebiten.GeoM) float32 {
	if off != 0 && geo.Element(0, 0)*geo.Element(1, 1)-geo.Element(0, 1)*geo.Element(1, 0) < 0 {
		return -off
	}
	return off
}

// DecodeNormalMirrored is DecodeNormal for a sprite drawn mirrored: the
// normal's x is inverted (lit.kage, Custom2 < 0).
func DecodeNormalMirrored(texel [4]float32, has, mirror bool) [3]float32 {
	n := DecodeNormal(texel, has)
	if mirror {
		n[0] = -n[0]
	}
	return n
}

// SourceOf is the image a lit draw of img samples and the x offset of
// its normal map in it (0: none, img itself is drawn). Ground planes
// (terrain and floors) are lit flat: they lie on the ground, and the
// generator's bevel dome on an opaque square tile showed as a quilt of
// pyramids, harshest under the low dawn and dusk sun. ground reports one,
// so Draw ramps its lamp light per vertex. The normal maps come from v.Art
// and v.Present ages the pair cache.
func (l *Lit) SourceOf(v *render.View, img *ebiten.Image) (src *ebiten.Image, off float32, ground bool) {
	if l.Off { // lighting off: nothing is paired, the sprite is lit flat
		return img, 0, false
	}
	n := v.Art.NormalOf(img)
	if n == nil {
		return img, 0, false
	}
	b := img.Bounds()
	off = float32(b.Dx() + LitGap)
	if p, ok := l.Pairs.Get(img, v.Present); ok {
		if p == img {
			return img, 0, true
		}
		return p, off, false
	}
	if GroundPlane(v.Art, img) {
		l.Pairs.Put(img, img, v.Present)
		return img, 0, true
	}
	p := ebiten.NewImage(2*b.Dx()+LitGap, b.Dy())
	p.DrawImage(img, nil)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(off), 0)
	p.DrawImage(n, op)
	l.Pairs.Put(img, p, v.Present)
	return p, off, false
}

// The lit-pair cache is a bounded LRU: a pair image is two sprites wide,
// and an unbounded map held one for every sprite ever drawn (tinted
// variants and long sessions included). It holds about one and a half
// times the distinct sprites the last frame drew, so the frame's working
// set stays resident, between a floor for small scenes and a ceiling for
// huge ones.
const (
	PairsMin = 256
	PairsMax = 1024
)

// PairCapFor is the cache size for a frame that drew visible distinct
// sprites.
func PairCapFor(visible int) int {
	return min(max(visible+visible/2, PairsMin), PairsMax)
}

// PairCache maps a sprite image to its pair image (the sprite and its
// normal map side by side), or to itself for a ground plane lit flat, which
// is a note that costs no image. The front of order is the most recently
// used entry. The zero value is ready.
type PairCache struct {
	m     map[*ebiten.Image]*list.Element
	order list.List
	// Limit, when above 0, replaces the visible-sprite capacity (tests).
	Limit int
	// frame is the presentation frame of the last use; seen counts the
	// distinct sprites used in it, and prevSeen in the frame before.
	frame          uint64
	seen, prevSeen int
	Evicted        int // pairs dropped to stay within capacity, for tests
}

type pairEntry struct {
	src, pair *ebiten.Image
	frame     uint64 // the last frame this entry was used in
}

// Size is how many sprites the cache holds.
func (c *PairCache) Size() int { return len(c.m) }

// Capacity is how many entries the cache may hold now.
func (c *PairCache) Capacity() int {
	if c.Limit > 0 {
		return c.Limit
	}
	return PairCapFor(c.prevSeen)
}

// advance moves the working-set count on to frame.
func (c *PairCache) advance(frame uint64) {
	if frame != c.frame {
		c.prevSeen, c.seen, c.frame = c.seen, 0, frame
	}
}

// touch marks e used in frame.
func (c *PairCache) touch(e *list.Element, frame uint64) {
	c.advance(frame)
	c.order.MoveToFront(e)
	if en := e.Value.(*pairEntry); en.frame != frame {
		en.frame = frame
		c.seen++
	}
}

// Get is src's pair (src itself: lit flat), refreshing its place.
func (c *PairCache) Get(src *ebiten.Image, frame uint64) (*ebiten.Image, bool) {
	e, ok := c.m[src]
	if !ok {
		return nil, false
	}
	c.touch(e, frame)
	return e.Value.(*pairEntry).pair, true
}

// Put stores src's pair and drops the least recently used entries beyond
// the capacity; their pair images are deallocated.
func (c *PairCache) Put(src, pair *ebiten.Image, frame uint64) {
	if c.m == nil {
		c.m = map[*ebiten.Image]*list.Element{}
	}
	c.advance(frame)
	if e, ok := c.m[src]; ok {
		e.Value.(*pairEntry).pair = pair
		c.touch(e, frame)
		return
	}
	c.m[src] = c.order.PushFront(&pairEntry{src: src, pair: pair, frame: frame})
	c.seen++
	for len(c.m) > max(c.Capacity(), 1) {
		old := c.order.Back()
		en := old.Value.(*pairEntry)
		c.order.Remove(old)
		delete(c.m, en.src)
		if en.pair != en.src {
			en.pair.Deallocate()
		}
		c.Evicted++
	}
}

// IsGroundKey reports a ground-plane sprite: terrain tiles and edges,
// floors, and the flat ground decals (snow cover, frost; cmd/normalgen
// gives them no normal map).
func IsGroundKey(key string) bool {
	return strings.HasPrefix(key, "terrain/") || strings.HasPrefix(key, "structure/floor") ||
		strings.HasPrefix(key, "weather/snow_cover") || strings.HasPrefix(key, "weather/frost")
}

// GroundPlane reports whether img is a ground-plane sprite's image (plain
// or tinted). It scans the catalog, so callers cache the answer.
func GroundPlane(c *art.Catalog, img *ebiten.Image) bool {
	if c == nil {
		return false
	}
	for key, sp := range c.Sprites {
		if sp.Img == img {
			return IsGroundKey(key)
		}
	}
	for k, t := range c.Tinted {
		if t == img {
			return IsGroundKey(k.Key)
		}
	}
	return false
}

// SyncUniforms copies this frame's light into the reused uniform map.
func (l *Lit) SyncUniforms() {
	if l.Opts.Uniforms == nil {
		l.Opts.Uniforms = map[string]any{}
		for i, name := range [...]string{"SunL", "MoonL", "SunColor", "MoonColor", "SkyAmbient", "IndoorAmbient", "LampColor", "LampSky"} {
			l.Uniforms[i] = make([]float32, 3)
			if name == "LampSky" { // a float: one value (a slice, so no boxing per draw)
				l.Uniforms[i] = l.Uniforms[i][:1]
			}
			l.Opts.Uniforms[name] = l.Uniforms[i]
		}
	}
	ls := &l.Light
	for i, v := range [...]*[3]float32{&ls.SunL, &ls.MoonL, &ls.SunColor, &ls.MoonColor, &ls.SkyAmbient, &ls.IndoorAmbient, &ls.LampColor} {
		copy(l.Uniforms[i], v[:])
	}
	l.Uniforms[7][0] = ls.LampSky
}

// DrawGround is Draw for a ground plane (terrain tiles and edges,
// floors): flat on the ground, its lamp light ramps per vertex
// (GroundLamp) like the bakes', instead of one level per tile.
func (l *Lit) DrawGround(v *render.View, dst, img *ebiten.Image, geo ebiten.GeoM, cs [4]float32, wx, wy float64, cast *Cast, baking bool, blood float32) {
	l.ground = true
	l.Draw(v, dst, img, geo, cs, wx, wy, cast, baking, blood)
}

// noCS is the identity colour scale.
var noCS = [4]float32{1, 1, 1, 1}

// Draw draws a world sprite img with geo and colour scale cs, lit as the
// ground under its foot at world (wx, wy) is (LightFoot: the tile's sky,
// lamp light by continuous distance; its normal map comes from the
// catalog; a sprite without one is lit flat). Every draw uses the same
// shader and uniforms; per-sprite light rides in vertex attributes, so
// Ebiten merges consecutive draws from one atlas page into one batch.
// The renderer's pass decides the rest: baking (a chunk or map bake: the
// unlit albedo, lit when the bake is drawn), cast (the shadow pass: a
// silhouette into the shadow layer, not a lit sprite) and blood (a
// figure's stain, Custom3; 0 for everything else).
func (l *Lit) Draw(v *render.View, dst, img *ebiten.Image, geo ebiten.GeoM, cs [4]float32, wx, wy float64, cast *Cast, baking bool, blood float32) {
	ground := l.ground
	l.ground = false
	if baking { // a bake: unlit albedo, lit when it is drawn
		op := &l.plain
		op.GeoM, op.Filter = geo, ebiten.FilterLinear
		op.ColorScale.Reset()
		if cs != noCS && cs != ([4]float32{}) {
			op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
		}
		dst.DrawImage(img, op)
		return
	}
	if cast != nil { // the shadow pass: a silhouette, not a lit sprite
		cast.Silhouette(img, geo)
		return
	}
	if l.OnDraw != nil {
		l.OnDraw(img, blood)
	}
	l.Calls++
	if cs == ([4]float32{}) {
		cs = noCS
	}
	// Lighting off (Options > Graphics): no shader and no pair image; the
	// tile's daylight is a plain colour scale. A bloody figure keeps the
	// shader, which paints the blood (flat: SourceOf pairs nothing).
	plain := l.Off && blood == 0
	var sh *ebiten.Shader
	if !plain {
		sh = l.LitShader()
	}
	if plain || sh == nil { // lighting off, or the shader failed to compile: the plain draw
		op := &l.plain
		op.GeoM, op.Filter = geo, ebiten.FilterLinear
		op.ColorScale.Reset()
		if plain {
			sky, lamp := l.LightFoot(v, wx, wy)
			k := FlatLitScale(&l.Light, cs, sky, lamp)
			op.ColorScale.Scale(k[0], k[1], k[2], k[3])
		} else if cs != noCS {
			op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
		}
		dst.DrawImage(img, op)
		return
	}
	src, off, flat := l.SourceOf(v, img)
	ground = ground || flat
	sky, lamp := l.LightFoot(v, wx, wy)
	b := img.Bounds()
	sx, sy := float32(b.Min.X), float32(b.Min.Y)
	if src != img {
		sx, sy = 0, 0
	}
	LitQuad(&l.Verts, sx, sy, float32(b.Dx()), float32(b.Dy()), geo, cs, sky, lamp, MirrorNormalOff(off, geo))
	for i := range l.Verts { // Custom3: the figure's blood (blood_draw.go), 0 for everything else
		l.Verts[i].Custom3 = blood
	}
	if ground { // flat on the ground: the lamp light ramps across it, as on the bakes
		l.GroundLamp(v, &l.Verts)
	}
	l.SyncUniforms()
	l.Opts.Images[0] = src
	dst.DrawTrianglesShader32(l.Verts[:], Indices[:], sh, &l.Opts)
}

// FlatLitScale is the colour scale a sprite draws with when lighting is
// off: its own scale cs (premultiplied as DrawImage's ColorScale is) times
// the light a flat normal gets on a tile with sky exposure and lamp light,
// so the hour, the weather, roofs and lamps still tint it.
func FlatLitScale(ls *State, cs [4]float32, sky, lamp float32) [4]float32 {
	f := LitFactor(ls, [3]float32{0, 0, 1}, sky, lamp)
	return [4]float32{cs[0] * cs[3] * f[0], cs[1] * cs[3] * f[1], cs[2] * cs[3] * f[2], cs[3]}
}

// shadowStrength is a light's shadow alpha on flat ground: the share of
// the ground's light it supplies, S/(A+S), times litShadowGain, between
// floor and limit.
func shadowStrength(c, dir, ambient [3]float32, floor float32, limit float32) float64 {
	s := Lum3(c) * max(dir[2], 0)
	a := Lum3(ambient)
	if s+a <= 0 {
		return 0
	}
	return float64(min(max(litShadowGain*s/(a+s), floor), limit))
}
