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

func TestLampRadiusFromMetadata(t *testing.T) {
	t.Parallel()
	m, err := parseArtMeta(sampleMeta + "light_radius: 3\n")
	if err != nil || m.LightRadius != 3 {
		t.Fatalf("light_radius: %v (%v)", m.LightRadius, err)
	}
	if got := lampRadiusOf(nil); got != lampDefaultRadius || lampDefaultRadius != 6 {
		t.Fatalf("default radius %v", got)
	}
	art := &artCatalog{Sprites: map[string]*artSprite{buildSprite("lamp", "", 0, false, false): {Meta: m}}}
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
	r.lit.Grid.Sources = func(dst []lightSource, _ *game.WorldView, _ float64) []lightSource {
		return append(dst, lightSource{ID: 9, X: 10, Y: 6, Radius: 6, Kind: lightLamp}) // where no lamp stands
	}
	if l := levelAt(r, 10, 6); l != 1 {
		t.Fatalf("the grid ignores the source list: %v at the listed lamp", l)
	}
	if l := levelAt(r, 3, 3); l != 0 {
		t.Fatalf("the grid lit an unlisted lamp building: %v", l)
	}
	pl := r.pointLights()
	if len(pl) != 1 || pl[0].ID != 9 || pl[0].X != 10.5 || pl[0].Y != 6.5 {
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
		switch f {
		case "render/light/light.go", "render/light/shadow_cast.go", "render/light/shadow.go", "shadow.go":
		default:
			continue
		}
		if strings.Contains(string(src), `"campfire"`) {
			t.Errorf("%s names campfires: take lights from lightSources (light_sources.go)", f)
		}
	}
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
	r.cam.OffsetX, r.cam.OffsetY = 0, 0
	tile := r.tilePx()
	r.drawBaked(ebiten.NewImage(64, 64), ebiten.NewImage(64, 64), 0, 0, 0, 0, 2, 0, 0, w-1, h-1)

	type pos struct{ x, y int32 }
	key := func(vx ebiten.Vertex) pos {
		return pos{int32(math.Round(float64(vx.DstX) / tile * 8)), int32(math.Round(float64(vx.DstY) / tile * 8))}
	}
	at := map[pos]float32{}
	for _, vx := range r.lod.Verts {
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
		if got := g.GroundLevel(float64(k.x/8), float64(k.y/8)); math.Abs(float64(got-l)) > 1e-6 {
			t.Fatalf("corner (%d, %d): live ground %v, bake %v", k.x/8, k.y/8, got, l)
		}
	}
	if got, want := g.GroundLevel(7.5, 5.5), levelAt(r, 7, 5); math.Abs(float64(got-want)) > 1e-6 {
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
	if quads, tiles := len(r.lod.Verts)/4, w*h; quads >= tiles/2 {
		t.Fatalf("%d quads for %d tiles: unlit runs did not merge", quads, tiles)
	}
}
