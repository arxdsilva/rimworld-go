package ebiten

import (
	"errors"
	"image/color"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// ng1ArtKeys is every sprite key the NG-1 draw paths can ask for.
func ng1ArtKeys() []string {
	var keys []string
	for _, t := range []string{"shallow_water", "deep_water", "mud", "track"} {
		for v := 0; v < soilVariants; v++ {
			keys = append(keys, artKey("terrain", terrainSprite[t], v))
		}
	}
	for m := 1; m < 16; m++ {
		n := twoDigits(m)
		keys = append(keys, "terrain/mud_edge_"+n, "terrain/shallow_water_edge_"+n, "terrain/shore_foam_edge_"+n)
	}
	for _, f := range facingNames {
		keys = append(keys, "rock/cave_mouth_"+f, buildSprite("hospital bed", f, 0, false, false))
	}
	keys = append(keys, "effect/pod_fall", "effect/pod_dust", buildSprite("pod shell", "", 0, false, false),
		buildSprite("tool cabinet", "", 0, false, false))
	for _, k := range []string{"components", "industrial medicine"} {
		for _, q := range []int{1, 8, 40} {
			keys = append(keys, sizedItemKey(itemSprite(k), q))
		}
	}
	return keys
}

func TestWaterTerrainSprites(t *testing.T) {
	t.Parallel()
	// water is the lowest priority, deep below shallow, so ground edges overlap it
	for n, p := range terrainPriority {
		if n != "shallow_water" && n != "deep_water" && p < 0 {
			t.Errorf("%s has priority %d: only water draws under the ground", n, p)
		}
	}
	if !(terrainPriority["deep_water"] < terrainPriority["shallow_water"] && terrainPriority["shallow_water"] < terrainPriority["soil"]) {
		t.Fatal("deep < shallow < soil")
	}
	// the codes of the read model map to the right sprite family and priority
	codes := map[string]int{}
	for i, n := range terrainNames {
		codes[n] = i
	}
	for name, want := range map[string]string{"shallow_water": "shallow_water", "deep_water": "deep_water", "mud": "mud", "track": "track"} {
		c, ok := codes[name]
		if !ok {
			t.Fatalf("terrain %s missing from the read model", name)
		}
		if terrainSprite[name] != want || int(terrainPriByCode[c]) != terrainPriority[name] {
			t.Errorf("%s: sprite %q priority %d", name, terrainSprite[name], terrainPriByCode[c])
		}
		if isWaterCode[c] != strings.HasSuffix(name, "_water") {
			t.Errorf("%s: isWaterCode %v", name, isWaterCode[c])
		}
	}
	// mud has its own soft edge over the ground below it
	if i := slices.Index(terrainByPri, "mud"); i != terrainPriority["mud"] || terrainEdgeKeys[i][5] != "terrain/mud_edge_05" {
		t.Fatalf("mud edge key %q at %d", terrainEdgeKeys[terrainPriority["mud"]][5], terrainPriority["mud"])
	}
	// the art exists for every key the water draw asks for
	c := realArt(t)
	for _, k := range ng1ArtKeys() {
		if !c.Has(k) {
			t.Errorf("missing art %s", k)
		}
	}
	// a terrain tile of each new ground is its own art, not the fallback
	r := &Renderer{art: c}
	for _, n := range []string{"shallow_water", "deep_water", "mud", "track"} {
		if k, cs := r.terrainTileKey(n, 2); k != "terrain/"+n+"_02" || cs != noCS {
			t.Errorf("%s tile = %q %v", n, k, cs)
		}
	}
	// without art the new ground falls back to a tinted tile rather than nothing
	if k, cs := testStoneArt().terrainTileKey("shallow_water", 1); cs == noCS || k == "" {
		t.Errorf("shallow water fallback = %q %v", k, cs)
	}
	// the water overlay masks follow the neighbours: N=1 E=2 S=4 W=8
	got := edgeMask([4]int{-1, 0, -100, 3}, func(p int) bool { return p >= 0 })
	if got != wallE|wallW {
		t.Fatalf("land mask %d, want E|W", got)
	}
}

func TestMachineryOreSprites(t *testing.T) {
	t.Parallel()
	if rockPrefix("rock", "machinery") != "rock/ore_machinery" || linkFallback["rock/ore_machinery"] != "rock/rock_wall" {
		t.Fatalf("machinery prefix %q fallback %q", rockPrefix("rock", "machinery"), linkFallback["rock/ore_machinery"])
	}
	c := realArt(t)
	for _, k := range linkedKeys("rock/ore_machinery") {
		if !c.Has(k) {
			t.Errorf("missing machinery ore art %s", k)
		}
	}
}

func TestSizedItemKeys(t *testing.T) {
	t.Parallel()
	for kind, base := range map[string]string{"components": "item/components_", "industrial medicine": "item/industrial_medicine_"} {
		for q, want := range map[int]string{1: "s", 4: "s", 5: "m", 14: "m", 15: "l", 70: "l"} {
			if got := sizedItemKey(itemSprite(kind), q); got != base+want {
				t.Errorf("%s x%d = %q, want %q", kind, q, got, base+want)
			}
		}
	}
	if sizedItemKey("item/steel_bars", 50) != "item/steel_bars" {
		t.Error("other items keep their key")
	}
}

func TestCaveMouthFacing(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 1, Width: 12, Height: 8})
	g.SetGround(0, 0, 11, 7, "soil", "", "")
	g.SetGround(3, 2, 8, 5, "rock", "granite", "")
	g.SetGround(2, 3, 5, 3, "stone", "granite", "") // a tunnel opening west of the mass
	w := g.World()
	// no roof data yet: nothing is a mouth
	if f := caveMouthFacing(w, 3, 3); f != "" {
		t.Fatalf("a mined tile without a natural roof: %q", f)
	}
	w.RoofCells = make([]uint8, w.TerrainW*8)
	for x := 2; x <= 5; x++ {
		w.RoofCells[3*w.TerrainW+x] = roofNatural
	}
	if f := caveMouthFacing(w, 2, 3); f != "west" && f != "north" && f != "south" {
		t.Fatalf("the tunnel mouth at the open edge faces %q", f)
	}
	if f := caveMouthFacing(w, 4, 3); f != "" {
		t.Fatalf("a tile deep in the tunnel faces %q", f)
	}
	if f := caveMouthFacing(w, 9, 3); f != "" {
		t.Fatalf("open ground is not a mouth: %q", f)
	}
}

func TestWaterShaderCompiles(t *testing.T) {
	t.Parallel()
	s, err := ebiten.NewShader(waterKage)
	if err != nil {
		t.Fatalf("water.kage: %v", err)
	}
	s.Deallocate()
	src := string(waterKage)
	for _, banned := range []string{"imageSrc1", "imageSrcRegion"} {
		if strings.Contains(src, banned) {
			t.Fatalf("water.kage uses %s", banned)
		}
	}
	for _, want := range []string{"var Time float", "sin(", "glint", "imageSrc0Origin"} {
		if !strings.Contains(src, want) {
			t.Fatalf("water.kage lacks %q: the ripple and glints", want)
		}
	}
	// draw a water tile through it: compiles on this backend and runs
	r := &Renderer{}
	r.view.W = roomWorld()
	r.lit.light = lightAt(12, summerDay, "clear", 0)
	r.present = 90
	img := ebiten.NewImage(16, 16)
	img.Fill(color.NRGBA{40, 90, 160, 255})
	r.art = &artCatalog{Sprites: map[string]*artSprite{"terrain/shallow_water_00": {Meta: artMeta{Canvas: [2]int{16, 16}}}}}
	r.art.Sprites["terrain/shallow_water_00"].Img = img
	dst := ebiten.NewImage(64, 64)
	shaderDraws := 0
	r.water.onDraw = func(_ string, shader bool) {
		if shader {
			shaderDraws++
		}
	}
	r.setView(64, 64, 32)
	r.drawWaterTile(dst, "shallow_water", 0, 0, 32, 64, 64, noCS)
	if r.water.err != nil || shaderDraws != 1 {
		t.Fatalf("water draw: err %v shader draws %d", r.water.err, shaderDraws)
	}
	if got := r.waterClock(); got != float32(90)/60 {
		t.Fatalf("clock %v", got)
	}
	r.reducedMotion = true
	if r.waterClock() != 0 {
		t.Fatal("Reduced Motion freezes the ripple")
	}
}

func TestWaterFallbackFramesAnimate(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	r.water.err = errors.New("no shader") // as when water.kage fails to compile
	frames := map[string]bool{}
	for p := uint64(0); p < 4*waterFrameTicks; p += waterFrameTicks {
		r.present = p
		frames[r.waterFrameKey("shallow_water", 3, 4)] = true
	}
	if len(frames) != soilVariants {
		t.Fatalf("%d animated frames, want %d", len(frames), soilVariants)
	}
	r.reducedMotion = true
	frames = map[string]bool{}
	for p := uint64(0); p < 4*waterFrameTicks; p += waterFrameTicks {
		r.present = p
		frames[r.waterFrameKey("shallow_water", 3, 4)] = true
	}
	if len(frames) != 1 {
		t.Fatalf("Reduced Motion holds one frame, saw %d", len(frames))
	}
	if r.waterShader() != nil {
		t.Fatal("a failed compile is not retried: the baked frames stay")
	}
}

func TestArrivalEffectDraw(t *testing.T) {
	t.Parallel()
	pods := []game.ArrivalView{{X: 5, Y: 6, Tick: 1000}, {X: 9, Y: 2, Tick: 1100, Landed: true}}
	// long before: nothing
	if fx := arrivalEffects(pods, 100, false); len(fx) != 0 {
		t.Fatalf("effects before the fall: %+v", fx)
	}
	// falling: the streak is above its landing tile and drops as the tick nears
	early := arrivalEffects(pods, 1000-arrivalFallTicks+4, false)
	late := arrivalEffects(pods, 1000-4, false)
	if len(early) != 1 || len(late) != 1 || early[0].key != "effect/pod_fall" {
		t.Fatalf("fall effects: %+v %+v", early, late)
	}
	if !(early[0].y < late[0].y) || late[0].y >= 6.5 || late[0].x != 5.5 || late[0].alpha <= early[0].alpha {
		t.Fatalf("the streak should drop onto tile (5,6): early %+v late %+v", early[0], late[0])
	}
	// landed: dust on the tile, swelling and fading
	d0 := arrivalEffects(pods[1:], 1100, false)
	d1 := arrivalEffects(pods[1:], 1100+arrivalDustTicks/2, false)
	if len(d0) != 1 || len(d1) != 1 || d0[0].key != "effect/pod_dust" || d0[0].x != 9.5 || d0[0].y != 2.5 {
		t.Fatalf("dust: %+v %+v", d0, d1)
	}
	if !(d1[0].scale > d0[0].scale) || !(d1[0].alpha < d0[0].alpha) {
		t.Fatalf("dust swells and fades: %+v -> %+v", d0[0], d1[0])
	}
	// the first pod: dust after its landing, gone once it has settled
	if fx := arrivalEffects(pods[:1], 1000+arrivalDustTicks, false); len(fx) != 0 {
		t.Fatalf("dust never ends: %+v", fx)
	}
	// Reduced Motion: no streak, and the dust only fades
	if fx := arrivalEffects(pods[:1], 990, true); len(fx) != 0 {
		t.Fatalf("a streak under Reduced Motion: %+v", fx)
	}
	a, b := arrivalEffects(pods[:1], 1000, true), arrivalEffects(pods[:1], 1050, true)
	if a[0].scale != 1 || b[0].scale != 1 || !(b[0].alpha < a[0].alpha) {
		t.Fatalf("reduced dust %+v %+v", a, b)
	}
}

func TestArrivalsDrawWithRealArt(t *testing.T) {
	art := realArt(t)
	g := game.New(game.Config{Seed: 3, Width: 20, Height: 12})
	r := New(g)
	r.art = art
	screen := ebiten.NewImage(640, 400)
	tile := r.tilePx()
	r.setView(640, 400, tile)
	w := g.World()
	w.Arrivals = []game.ArrivalView{{X: 10, Y: 6, Tick: uint64(g.Tick()) + 10}}
	r.drawArrivals(screen, w, tile) // falling
	g.Step(30)
	w.Arrivals[0].Landed = true
	r.drawArrivals(screen, w, tile) // dust
	r.reducedMotion = true
	r.drawArrivals(screen, w, tile)
}

// A tile mined out at the edge of a rock mass keeps the rock's natural
// roof and borders open ground: the real read model calls it a mouth.
func TestCaveMouthAfterMining(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 1, Width: 16, Height: 9})
	g.SpawnNamedPawn(2, 4, "Bo")
	g.SetGround(0, 0, 15, 8, "soil", "", "")
	g.SetGround(6, 2, 12, 6, "rock", "granite", "")
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: 6, Y: 4, X2: 6, Y2: 4})
	g.Step(2000)
	w := g.World()
	if g.TerrainAt(6, 4) != "stone" {
		t.Fatalf("tile not mined: %s", g.TerrainAt(6, 4))
	}
	if f := caveMouthFacing(w, 6, 4); f != "west" {
		t.Fatalf("mined edge tile faces %q, want west (the open ground)", f)
	}
	if f := caveMouthFacing(w, 7, 4); f != "" {
		t.Fatalf("a rock wall is not a mouth: %q", f)
	}
}
