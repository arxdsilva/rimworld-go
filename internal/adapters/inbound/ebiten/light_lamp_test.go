package ebiten

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Point lamps (PLAY-020 §3b): lamps are real point lights with a falloff,
// walls stop them, outdoor lamps light what they see, and sprites shade
// toward the dominant lamp.

// lampTestWorld is a w×h map, all outdoors under open sky.
func lampTestWorld(w, h int) game.WorldView {
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), RoomGrid: make([]int, w*h), RoofCells: make([]uint8, w*h)}
	for i := range v.RoomGrid {
		v.RoomGrid[i] = -1
	}
	return v
}

// lampRoom walls the rectangle x0..x1, y0..y1 (walls on its border) and
// makes its inside room id, roofed (walls included).
func lampRoom(v *game.WorldView, id, x0, y0, x1, y1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := y*v.TerrainW + x
			v.RoofCells[i] = roofBuilt
			if x == x0 || y == y0 || x == x1 || y == y1 {
				v.RoomGrid[i] = -2
			} else {
				v.RoomGrid[i] = id
			}
		}
	}
	v.Rooms = append(v.Rooms, game.RoomView{ID: id, Lit: true})
}

// lampBuilding is a powered standing lamp: PLAY-021 lights only what the
// sim says glows (BuildingView.Glow), and a powered lamp reaches 6 tiles.
func lampBuilding(id, x, y int) game.BuildingView {
	return game.BuildingView{ID: entity.EntityID(id), Kind: "lamp", Glow: true, GlowRadius: 6, X: x, Y: y, W: 1, H: 1}
}

// levelAt is the lamp light level at (x, y), the packed vertex value decoded.
func levelAt(r *Renderer, x, y int) float32 {
	_, l := r.lightTile(x, y)
	return lampLevel(l)
}

func TestLampNearBrighterThanFarInRoom(t *testing.T) {
	t.Parallel()
	v := lampTestWorld(12, 8)
	lampRoom(&v, 0, 0, 0, 10, 6) // inside x 1..9, y 1..5
	v.Buildings = []game.BuildingView{lampBuilding(1, 2, 3)}
	r := &Renderer{}
	r.view.W = v
	if l := levelAt(r, 2, 3); l != 1 {
		t.Fatalf("under the lamp %v, want 1", l)
	}
	near, mid, far := levelAt(r, 4, 3), levelAt(r, 6, 3), levelAt(r, 7, 3)
	if !(1 > near && near > mid && mid > far && far > 0) {
		t.Fatalf("falloff near %v mid %v far %v: want 1 > near > mid > far > 0", near, mid, far)
	}
	if l := levelAt(r, 8, 3); l != 0 {
		t.Fatalf("at the radius (6 tiles) %v, want 0", l)
	}
	// lighting off keeps the falloff (flat, no direction)
	night := lightAt(0, summerDay, "clear", 0)
	_, ln := r.lightTile(4, 3)
	_, lf := r.lightTile(7, 3)
	if a, b := flatLitScale(&night, noCS, 0, ln), flatLitScale(&night, noCS, 0, lf); !(a[0] > b[0]) {
		t.Fatalf("lighting off: near %v should be brighter than far %v", a, b)
	}
}

func TestLampNoLightThroughWall(t *testing.T) {
	t.Parallel()
	v := lampTestWorld(10, 6)
	lampRoom(&v, 0, 0, 0, 4, 5) // inside x 1..3
	lampRoom(&v, 1, 4, 0, 8, 5) // shares the wall at x 4; inside x 5..7
	v.Buildings = []game.BuildingView{lampBuilding(1, 3, 2)}
	r := &Renderer{}
	r.view.W = v
	if l := levelAt(r, 5, 2); l != 0 {
		t.Fatalf("the next room, 2 tiles from the lamp through a wall: %v, want 0", l)
	}
	if l := levelAt(r, 1, 2); l <= 0 {
		t.Fatalf("the lamp's own room is lit: %v", l)
	}
	if l := levelAt(r, 4, 2); l != 1 {
		t.Fatalf("the shared wall glows with its brightest side: %v, want 1", l)
	}
}

func TestOutdoorLampLightsOpenGroundNotBehindWall(t *testing.T) {
	t.Parallel()
	v := lampTestWorld(16, 12)
	for y := 3; y <= 7; y++ {
		v.RoomGrid[y*16+8] = -2 // a free-standing wall east of the lamp
	}
	lampRoom(&v, 0, 1, 7, 3, 9) // an enclosed room south-west; inside (2, 8)
	v.Buildings = []game.BuildingView{lampBuilding(1, 5, 5)}
	r := &Renderer{}
	r.view.W = v
	for _, c := range [][2]int{{5, 5}, {7, 5}, {5, 8}, {5, 10}, {3, 3}} {
		if l := levelAt(r, c[0], c[1]); l <= 0 {
			t.Fatalf("open ground %v in reach: lamp %v", c, l)
		}
	}
	if l := levelAt(r, 9, 5); l != 0 {
		t.Fatalf("behind the wall (9,5): %v, want 0", l)
	}
	if l := levelAt(r, 2, 8); l != 0 {
		t.Fatalf("inside an enclosed room (2,8): %v, want 0", l)
	}
	if l := levelAt(r, 12, 5); l != 0 {
		t.Fatalf("beyond the radius (12,5): %v, want 0", l)
	}
	if l := levelAt(r, 8, 5); l <= 0 {
		t.Fatalf("the wall's lit face glows: %v", l)
	}
}

func TestLampWestFacingBrighterEastOfLamp(t *testing.T) {
	t.Parallel()
	v := lampTestWorld(12, 8)
	lampRoom(&v, 0, 0, 0, 10, 6)
	v.Buildings = []game.BuildingView{lampBuilding(1, 2, 3)}
	r := &Renderer{}
	r.view.W = v
	ls := lightAt(0, summerDay, "clear", 0)
	west, east := [3]float32{-0.7071, 0, 0.7071}, [3]float32{0.7071, 0, 0.7071}
	lampPart := func(n [3]float32, l float32) float32 { return litFactor(&ls, n, 0, l)[0] - ls.IndoorAmbient[0] }
	_, l := r.lightTile(5, 3) // 3 tiles east of the lamp
	fw, ff, fe := lampPart(west, l), lampPart(flatN, l), lampPart(east, l)
	if !(fw > ff && ff > fe) {
		t.Fatalf("east of the lamp: west-facing %v flat %v east-facing %v", fw, ff, fe)
	}
	if want := ls.LampColor[0] * lampLevel(l); math.Abs(float64(ff-want)) > 1e-6 {
		t.Fatalf("a flat normal gets the plain level: %v, want %v", ff, want)
	}
	if fe < 0.4*ff {
		t.Fatalf("the back side keeps a share: %v of flat %v", fe, ff)
	}
	_, l = r.lightTile(2, 1) // north of the lamp: lit from the south (+y)
	if s, n := lampPart([3]float32{0, 0.7071, 0.7071}, l), lampPart([3]float32{0, -0.7071, 0.7071}, l); !(s > n) {
		t.Fatalf("north of the lamp: south-facing %v should beat north-facing %v", s, n)
	}
	_, l = r.lightTile(2, 3) // under the lamp: flat
	if a, b := lampPart(west, l), lampPart(east, l); a != b {
		t.Fatalf("under the lamp west %v and east %v should match (flat)", a, b)
	}
}

func TestRoofedRoomAtNoonNoSunTerm(t *testing.T) {
	t.Parallel()
	v := lampTestWorld(12, 8)
	lampRoom(&v, 0, 0, 0, 10, 6)
	v.Buildings = []game.BuildingView{lampBuilding(1, 2, 3)}
	r := &Renderer{}
	r.view.W = v
	noon := lightAt(12, summerDay, "clear", 0)
	dark := noon
	dark.SunColor, dark.MoonColor = [3]float32{}, [3]float32{}
	sunward := noon.SunL
	sky, l := r.lightTile(4, 3)
	if sky != 0 {
		t.Fatalf("roofed tile sky %v", sky)
	}
	if a, b := litFactor(&noon, sunward, sky, l), litFactor(&dark, sunward, sky, l); a != b {
		t.Fatalf("a roofed room at noon has a sun term: %v vs %v without the sun", a, b)
	}
	if lit, unlit := litFactor(&noon, flatN, sky, l), litFactor(&noon, flatN, sky, 0); !(lit[0] > unlit[0]+0.1) {
		t.Fatalf("the lamp still lights a roofed room at noon: %v vs %v", lit, unlit)
	}
}

func TestLampSkyFadesOpenGroundByDay(t *testing.T) {
	t.Parallel()
	noon, night := lightAt(12, summerDay, "clear", 0), lightAt(0, summerDay, "clear", 0)
	if noon.LampSky < 0.99 || night.LampSky != 0 || lightAt(20, summerDay, "clear", 0).LampSky != 0 {
		t.Fatalf("LampSky noon %v night %v", noon.LampSky, night.LampSky)
	}
	if dusk := lightAt(18, summerDay, "clear", 0).LampSky; !(dusk > 0 && dusk < 0.5) {
		t.Fatalf("LampSky at dusk %v", dusk)
	}
	if a, b := litFactor(&noon, flatN, 1, 1), litFactor(&noon, flatN, 1, 0); a != b {
		t.Fatalf("the sun drowns a lamp on open ground at noon: %v vs %v", a, b)
	}
	if a, b := litFactor(&night, flatN, 1, 1), litFactor(&night, flatN, 1, 0); !(a[0] > b[0]+0.5) {
		t.Fatalf("a lamp lights open ground at night: %v vs %v", a, b)
	}
	if a, b := flatLitScale(&noon, noCS, 0, 1), flatLitScale(&noon, noCS, 0, 0); !(a[0] > b[0]) {
		t.Fatalf("lighting off: a roofed lamp still shows at noon: %v vs %v", a, b)
	}
}

func TestLampPackDecodeTwin(t *testing.T) {
	t.Parallel()
	for _, plain := range []float32{0, 0.25, 0.5, 1} {
		if l, vx, vy := decodeLamp(plain); l != plain || vx != 0 || vy != 0 {
			t.Fatalf("plain %v decoded to (%v, %v, %v)", plain, l, vx, vy)
		}
	}
	if v := packLamp(0.7, 0, 0); v != 0.7 {
		t.Fatalf("no direction packs plain: %v", v)
	}
	step := float32(lampDirMax / lampDirSteps)
	for _, level := range []float32{0.01, 0.3, 0.999, 1} {
		for _, d := range [][2]float64{{0.6, 0}, {-0.6, 0.0}, {0, -0.6}, {0.31, -0.42}, {-0.05, 0.2}, {0.6, 0.6}} {
			v := packLamp(level, d[0], d[1])
			l, vx, vy := decodeLamp(v)
			if math.Abs(float64(l-level)) > 1e-3 {
				t.Fatalf("level %v dir %v: decoded level %v", level, d, l)
			}
			if math.Abs(float64(vx)-d[0]) > float64(step)/2+1e-6 || math.Abs(float64(vy)-d[1]) > float64(step)/2+1e-6 {
				t.Fatalf("level %v dir %v: decoded (%v, %v)", level, d, vx, vy)
			}
			// varying interpolation noise never moves the direction
			for _, k := range []float32{1 - 2e-6, 1 + 2e-6} {
				l2, vx2, vy2 := decodeLamp(v * k)
				if vx2 != vx || vy2 != vy || math.Abs(float64(l2-l)) > 2e-2 {
					t.Fatalf("noise x%v: (%v, %v, %v) vs (%v, %v, %v)", k, l2, vx2, vy2, l, vx, vy)
				}
			}
		}
	}
	// the shader decodes exactly as the Go twin
	src := string(litKage)
	for _, want := range []string{"floor(v / 4)", "floor((c + 0.5) / 33)", "LampSky*sky", "dot(n.xy, lp.yz)"} {
		if !strings.Contains(src, want) {
			t.Fatalf("lit.kage lacks %q (decodeLamp / litFactor twin)", want)
		}
	}
}

func TestLampRadiusFromMetadata(t *testing.T) {
	t.Parallel()
	m, err := parseArtMeta(sampleMeta + "light_radius: 3\n")
	if err != nil || m.LightRadius != 3 {
		t.Fatalf("light_radius: %v (%v)", m.LightRadius, err)
	}
	if got := lampRadiusOf(nil); got != lampDefaultRadius || lampDefaultRadius != 6 {
		t.Fatalf("default radius %v", got)
	}
	art := &artCatalog{sprites: map[string]*artSprite{buildSprite("lamp", "", 0, false, false): {meta: m}}}
	if got := lampRadiusOf(art); got != 3 {
		t.Fatalf("metadata radius %v, want 3", got)
	}
	v := lampTestWorld(16, 10)
	v.Buildings = []game.BuildingView{lampBuilding(1, 5, 5)}
	r := &Renderer{art: art}
	r.view.W = v
	if a, b := levelAt(r, 7, 5), levelAt(r, 8, 5); a <= 0 || b != 0 {
		t.Fatalf("radius 3: 2 tiles %v, 3 tiles %v", a, b)
	}
	r2 := &Renderer{}
	r2.view.W = v
	if l := levelAt(r2, 9, 5); l <= 0 {
		t.Fatalf("default radius 6: 4 tiles %v", l)
	}
}

func TestLightGridRebuildsOnlyOnStructureOrLampChange(t *testing.T) {
	t.Parallel()
	v := lampTestWorld(20, 20)
	v.Buildings = []game.BuildingView{lampBuilding(1, 5, 5)}
	r := &Renderer{}
	r.view.W = v
	levelAt(r, 5, 5)
	gen := r.lit.grid.gen
	same := v
	same.Buildings = []game.BuildingView{lampBuilding(1, 5, 5), {ID: 2, Kind: "chair", X: 9, Y: 9, W: 1, H: 1}}
	r.view.W = same
	levelAt(r, 5, 5)
	if r.lit.grid.gen != gen {
		t.Fatal("a new building list with the same lamps rebuilt the light")
	}
	moved := v
	moved.Buildings = []game.BuildingView{lampBuilding(1, 6, 5)}
	r.view.W = moved
	if l := levelAt(r, 6, 5); l != 1 || r.lit.grid.gen == gen {
		t.Fatalf("a moved lamp: level %v, generation kept %v", l, r.lit.grid.gen == gen)
	}
	gen = r.lit.grid.gen
	walled := moved
	walled.RoomGrid = append([]int(nil), moved.RoomGrid...)
	walled.RoomGrid[5*20+7] = -2
	r.view.W = walled
	if l := levelAt(r, 8, 5); l != 0 || r.lit.grid.gen == gen {
		t.Fatalf("a new wall: behind it %v, generation kept %v", l, r.lit.grid.gen == gen)
	}
}

// TestLightsComeOnlyFromLightSources: the grid and the point lights take
// every light from lightSources (PLAY-021 gates that one helper on power
// and fuel): a substituted source list moves both, and no other code
// tests a building's kind for being a lamp.
func TestLightsComeOnlyFromLightSources(t *testing.T) {
	t.Parallel()
	v := lampTestWorld(20, 12)
	v.Buildings = []game.BuildingView{lampBuilding(1, 3, 3), {ID: 2, Kind: "campfire", X: 15, Y: 8, W: 1, H: 1, Burning: true, Glow: true, GlowRadius: 5},
		{ID: 3, Kind: "campfire", X: 15, Y: 2, W: 1, H: 1}, {ID: 4, Kind: "chair", X: 9, Y: 9, W: 1, H: 1}}
	got := appendLightSources(nil, &v, 6)
	want := []lightSource{{ID: 1, X: 3, Y: 3, Radius: 6, Kind: lightLamp}, {ID: 2, X: 15, Y: 8, Radius: campfireReach, Kind: lightCampfire}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("light sources %+v, want %+v", got, want)
	}

	r := shadowRenderer(t, v)
	r.view.W = v
	r.lit.grid.sources = func(dst []lightSource, _ *game.WorldView, _ float64) []lightSource {
		return append(dst, lightSource{ID: 9, X: 10, Y: 6, Radius: 6, Kind: lightLamp}) // where no lamp stands
	}
	if l := levelAt(r, 10, 6); l != 1 {
		t.Fatalf("the grid ignores the source list: %v at the listed lamp", l)
	}
	if l := levelAt(r, 3, 3); l != 0 {
		t.Fatalf("the grid lit an unlisted lamp building: %v", l)
	}
	pl := r.pointLights()
	if len(pl) != 1 || pl[0].id != 9 || pl[0].x != 10.5 || pl[0].y != 6.5 {
		t.Fatalf("point lights %+v, want only the listed lamp", pl)
	}

	lampKind := regexp.MustCompile(`Kind\s*==\s*"lamp"|"lamp"\s*==\s*\w*\.?Kind`)
	for _, f := range packageFiles(t, ".") {
		if filepath.Base(f) == "light_sources.go" {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if lampKind.Match(src) {
			t.Errorf("%s tests Kind == \"lamp\": take lights from lightSources (light_sources.go)", f)
		}
		if (f == "light.go" || f == "shadow_cast.go" || f == "shadow.go") && strings.Contains(string(src), `"campfire"`) {
			t.Errorf("%s names campfires: take lights from lightSources (light_sources.go)", f)
		}
	}
}

// BenchmarkLightGridLamps: a 250×250 map with 30 lit rooms and 30 outdoor
// lamps. rebuild is the cost after a structure or lamp change (every
// chunk of a 1280×800 view at the live tier, about 6×4 chunks, rebuilt
// once); frame is a frame's steady reads of that view (lightTile per tile).
func BenchmarkLightGridLamps(b *testing.B) {
	const n = 250
	v := lampTestWorld(n, n)
	for i := 0; i < 30; i++ {
		x, y := 4+(i%6)*40, 4+(i/6)*40
		lampRoom(&v, i, x, y, x+7, y+6)
		v.Buildings = append(v.Buildings, lampBuilding(1+i, x+2, y+3), lampBuilding(100+i, x+20, y+20))
	}
	r := &Renderer{}
	r.view.W = v
	const vw, vh = 96, 64 // tiles in view
	read := func() {
		for y := 0; y < vh; y++ {
			for x := 0; x < vw; x++ {
				r.lightTile(x, y)
			}
		}
	}
	read()
	b.Run("rebuild", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.lit.grid.gen++ // every chunk stale, as after a change
			read()
		}
	})
	b.Run("frame", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			read()
		}
	})
}

// TestGroundLampLightInterpolatesAcrossTiles: the baked ground carries the
// lamp light per vertex, each corner the plain level averaged from the
// open tiles around it, so it ramps across a tile and agrees on every seam
// (no step at tile edges); walls stop it at their near side; unlit runs
// still merge into one quad.
func TestGroundLampLightInterpolatesAcrossTiles(t *testing.T) {
	const w, h = 30, 12
	v := lampTestWorld(w, h)
	for y := 2; y <= 8; y++ {
		v.RoomGrid[y*w+9] = -2 // a free-standing wall east of the lamp
	}
	v.Buildings = []game.BuildingView{lampBuilding(1, 5, 5)}
	r := New(&stubSim{w: w, h: h, world: v})
	r.view.W = v
	r.cam.offsetX, r.cam.offsetY = 0, 0
	tile := r.tilePx()
	r.drawBaked(ebiten.NewImage(64, 64), ebiten.NewImage(64, 64), 0, 0, 0, 0, 2, 0, 0, w-1, h-1)

	type pos struct{ x, y int32 }
	key := func(vx ebiten.Vertex) pos {
		return pos{int32(math.Round(float64(vx.DstX) / tile * 8)), int32(math.Round(float64(vx.DstY) / tile * 8))}
	}
	at := map[pos]float32{}
	for _, vx := range r.lod.verts {
		if vx.Custom1 < 0 || vx.Custom1 > 1 {
			t.Fatalf("ground vertex carries a packed lamp value %v; want a plain level", vx.Custom1)
		}
		k := key(vx)
		if old, ok := at[k]; ok && old != vx.Custom1 {
			t.Fatalf("seam at corner (%v, %v): %v and %v; the ground steps", float64(k.x)/8, float64(k.y)/8, old, vx.Custom1)
		}
		at[k] = vx.Custom1
	}
	corner := func(x, y int) float32 {
		l, ok := at[pos{int32(x * 8), int32(y * 8)}]
		if !ok {
			t.Fatalf("no vertex at corner (%d, %d)", x, y)
		}
		return l
	}
	// corner (8, 5) is shared by tiles 7..8 × 4..5, all open and lit
	want := (levelAt(r, 7, 4) + levelAt(r, 8, 4) + levelAt(r, 7, 5) + levelAt(r, 8, 5)) / 4
	if got := corner(8, 5); math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("corner (8, 5) = %v, want the mean of its four tiles %v", got, want)
	}
	// across tile (7, 5) the light falls away from the lamp
	if l, r := corner(7, 5), corner(8, 5); !(l > r && r > 0) {
		t.Fatalf("tile (7, 5): west corner %v, east corner %v; want a ramp down eastward", l, r)
	}
	// the wall's far side (corner (10, 5): wall tiles x 9, dark tiles x 10) is dark
	if got := corner(10, 5); got != 0 {
		t.Fatalf("behind the wall the ground is lit %v", got)
	}
	if got := corner(9, 5); got <= 0 {
		t.Fatalf("the wall's lit side is dark: %v", got)
	}
	// the live ground (drawLit of a ground plane) reads the same field: the
	// bakes' corners, and each tile's own level at its centre
	g := r.syncLight()
	for k, l := range at {
		if k.x%8 != 0 || k.y%8 != 0 {
			continue
		}
		if got := g.groundLevel(float64(k.x/8), float64(k.y/8)); math.Abs(float64(got-l)) > 1e-6 {
			t.Fatalf("corner (%d, %d): live ground %v, bake %v", k.x/8, k.y/8, got, l)
		}
	}
	if got, want := g.groundLevel(7.5, 5.5), levelAt(r, 7, 5); math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("tile (7, 5)'s centre: %v, want its level %v", got, want)
	}
	var vs [4]ebiten.Vertex
	var geo ebiten.GeoM
	geo.Scale(tile/16, tile/16)
	geo.Translate(7*tile, 5*tile)
	litQuad(&vs, 0, 0, 16, 16, geo, noCS, 1, 0.5, 0)
	r.groundLamp(&vs)
	for i, c := range [4][2]int{{7, 5}, {8, 5}, {7, 6}, {8, 6}} {
		if want := corner(c[0], c[1]); math.Abs(float64(vs[i].Custom1-want)) > 1e-6 {
			t.Fatalf("live ground tile (7, 5) vertex %d: %v, want corner %v", i, vs[i].Custom1, want)
		}
	}
	// the dark east (x ≥ 12) merges: fewer quads than tiles
	if quads, tiles := len(r.lod.verts)/4, w*h; quads >= tiles/2 {
		t.Fatalf("%d quads for %d tiles: unlit runs did not merge", quads, tiles)
	}
}

// TestLampLevelContinuousAtSpriteFoot: a sprite's lamp light comes from
// the continuous distance at its foot, so a colonist walking away from a
// lamp dims smoothly; a sprite on its tile's corner (a building, a
// standing colonist) gets the grid's value; the direction stays one packed
// code per sprite, toward the lamp; a wall still stops the light.
func TestLampLevelContinuousAtSpriteFoot(t *testing.T) {
	t.Parallel()
	const w, h = 20, 12
	v := lampTestWorld(w, h)
	for y := 2; y <= 8; y++ {
		v.RoomGrid[y*w+12] = -2
	}
	v.Buildings = []game.BuildingView{lampBuilding(1, 5, 5)}
	r := &Renderer{}
	r.view.W = v
	for _, c := range [][2]int{{5, 5}, {7, 5}, {8, 3}, {10, 6}} {
		s0, l0 := r.lightTile(c[0], c[1])
		if s1, l1 := r.lightFoot(float64(c[0]), float64(c[1])); s1 != s0 || l1 != l0 {
			t.Fatalf("on tile %v's corner: (%v, %v), want the grid's (%v, %v)", c, s1, l1, s0, l0)
		}
	}
	prev := float32(1)
	for i := 0; i <= 50; i++ {
		fx := 6.0 + float64(i)*0.1
		_, lamp := r.lightFoot(fx, 5)
		lvl, vx, vy := decodeLamp(lamp)
		if lvl > prev || prev-lvl > 0.06 {
			t.Fatalf("at x %.1f the level %v after %v: want a smooth fall", fx, lvl, prev)
		}
		if lvl > 0 && (vx >= 0 || vy != 0) {
			t.Fatalf("at x %.1f the direction (%v, %v) does not point west to the lamp", fx, vx, vy)
		}
		prev = lvl
	}
	if a, b := levelAtF(r, 7.99, 5), levelAtF(r, 8, 5); math.Abs(float64(a-b)) > 0.01 {
		t.Fatalf("across a tile edge %v then %v: a step", a, b)
	}
	if l := levelAtF(r, 13.4, 5); l != 0 {
		t.Fatalf("behind the wall a sprite is lit %v", l)
	}
}

func levelAtF(r *Renderer, x, y float64) float32 {
	_, l := r.lightFoot(x, y)
	return lampLevel(l)
}
