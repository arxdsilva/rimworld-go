package ebiten

import (
	"errors"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
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
		for v := 0; v < world.SoilVariants; v++ {
			keys = append(keys, env.ArtKey("terrain", world.TerrainSprite[t], v))
		}
	}
	for m := 1; m < 16; m++ {
		n := spritepkg.TwoDigits(m)
		keys = append(keys, "terrain/mud_edge_"+n, "terrain/shallow_water_edge_"+n, "terrain/shore_foam_edge_"+n)
	}
	for _, f := range architect.FacingNames {
		keys = append(keys, "rock/cave_mouth_"+f, world.BuildSprite("hospital bed", f, 0, false, false))
	}
	keys = append(keys, "effect/pod_fall", "effect/pod_dust", world.BuildSprite("pod shell", "", 0, false, false),
		world.BuildSprite("tool cabinet", "", 0, false, false))
	for _, k := range []string{"components", "industrial medicine"} {
		for _, q := range []int{1, 8, 40} {
			keys = append(keys, world.SizedItemKey(world.ItemSprite(k), q))
		}
	}
	return keys
}

func TestWaterTerrainSprites(t *testing.T) {
	t.Parallel()
	// water is the lowest priority, deep below shallow, so ground edges overlap it
	for n, p := range world.TerrainPriority {
		if n != "shallow_water" && n != "deep_water" && p < 0 {
			t.Errorf("%s has priority %d: only water draws under the ground", n, p)
		}
	}
	if !(world.TerrainPriority["deep_water"] < world.TerrainPriority["shallow_water"] && world.TerrainPriority["shallow_water"] < world.TerrainPriority["soil"]) {
		t.Fatal("deep < shallow < soil")
	}
	// the codes of the read model map to the right sprite family and priority
	codes := map[string]int{}
	for i, n := range world.TerrainNames {
		codes[n] = i
	}
	for name, want := range map[string]string{"shallow_water": "shallow_water", "deep_water": "deep_water", "mud": "mud", "track": "track"} {
		c, ok := codes[name]
		if !ok {
			t.Fatalf("terrain %s missing from the read model", name)
		}
		if world.TerrainSprite[name] != want || int(world.TerrainPriByCode[c]) != world.TerrainPriority[name] {
			t.Errorf("%s: sprite %q priority %d", name, world.TerrainSprite[name], world.TerrainPriByCode[c])
		}
		if world.IsWaterCode[c] != strings.HasSuffix(name, "_water") {
			t.Errorf("%s: isWaterCode %v", name, world.IsWaterCode[c])
		}
	}
	// mud has its own soft edge over the ground below it
	if i := slices.Index(world.TerrainByPri, "mud"); i != world.TerrainPriority["mud"] || world.TerrainEdgeKeys[i][5] != "terrain/mud_edge_05" {
		t.Fatalf("mud edge key %q at %d", world.TerrainEdgeKeys[world.TerrainPriority["mud"]][5], world.TerrainPriority["mud"])
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
		if k, cs := r.pipe().TerrainTileKey(n, 2); k != "terrain/"+n+"_02" || cs != spritepkg.NoCS {
			t.Errorf("%s tile = %q %v", n, k, cs)
		}
	}
	// without art the new ground falls back to a tinted tile rather than nothing
	if k, cs := testStoneArt().pipe().TerrainTileKey("shallow_water", 1); cs == spritepkg.NoCS || k == "" {
		t.Errorf("shallow water fallback = %q %v", k, cs)
	}
	// the water overlay masks follow the neighbours: N=1 E=2 S=4 W=8
	got := env.EdgeMask([4]int{-1, 0, -100, 3}, func(p int) bool { return p >= 0 })
	if got != artpkg.WallE|artpkg.WallW {
		t.Fatalf("land mask %d, want E|W", got)
	}
}

func TestMachineryOreSprites(t *testing.T) {
	t.Parallel()
	if spritepkg.RockPrefix("rock", "machinery") != "rock/ore_machinery" || spritepkg.LinkFallback["rock/ore_machinery"] != "rock/rock_wall" {
		t.Fatalf("machinery prefix %q fallback %q", spritepkg.RockPrefix("rock", "machinery"), spritepkg.LinkFallback["rock/ore_machinery"])
	}
	c := realArt(t)
	for _, k := range spritepkg.LinkedKeys("rock/ore_machinery") {
		if !c.Has(k) {
			t.Errorf("missing machinery ore art %s", k)
		}
	}
}

func TestSizedItemKeys(t *testing.T) {
	t.Parallel()
	for kind, base := range map[string]string{"components": "item/components_", "industrial medicine": "item/industrial_medicine_"} {
		for q, want := range map[int]string{1: "s", 4: "s", 5: "m", 14: "m", 15: "l", 70: "l"} {
			if got := world.SizedItemKey(world.ItemSprite(kind), q); got != base+want {
				t.Errorf("%s x%d = %q, want %q", kind, q, got, base+want)
			}
		}
	}
	if world.SizedItemKey("item/steel_bars", 50) != "item/steel_bars" {
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
	if f := world.CaveMouthFacing(w, 3, 3); f != "" {
		t.Fatalf("a mined tile without a natural roof: %q", f)
	}
	w.RoofCells = make([]uint8, w.TerrainW*8)
	for x := 2; x <= 5; x++ {
		w.RoofCells[3*w.TerrainW+x] = overlaypkg.RoofNatural
	}
	if f := world.CaveMouthFacing(w, 2, 3); f != "west" && f != "north" && f != "south" {
		t.Fatalf("the tunnel mouth at the open edge faces %q", f)
	}
	if f := world.CaveMouthFacing(w, 4, 3); f != "" {
		t.Fatalf("a tile deep in the tunnel faces %q", f)
	}
	if f := world.CaveMouthFacing(w, 9, 3); f != "" {
		t.Fatalf("open ground is not a mouth: %q", f)
	}
}

func TestWaterShaderCompiles(t *testing.T) {
	t.Parallel()
	s, err := ebiten.NewShader(env.WaterKage)
	if err != nil {
		t.Fatalf("water.kage: %v", err)
	}
	s.Deallocate()
	src := string(env.WaterKage)
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
	r.lit.Light = light.LightAt(12, summerDay, "clear", 0)
	r.present = 90
	img := ebiten.NewImage(16, 16)
	img.Fill(color.NRGBA{40, 90, 160, 255})
	r.art = &artpkg.Catalog{Sprites: map[string]*artpkg.Sprite{"terrain/shallow_water_00": {Meta: artpkg.Meta{Canvas: [2]int{16, 16}}}}}
	r.art.Sprites["terrain/shallow_water_00"].Img = img
	dst := ebiten.NewImage(64, 64)
	shaderDraws := 0
	r.water.OnDraw = func(_ string, shader bool) {
		if shader {
			shaderDraws++
		}
	}
	r.pipe().Env.SetView(r.liveView(), 64, 64, 32)
	r.drawWaterTile(dst, "shallow_water", 0, 0, 32, 64, 64, spritepkg.NoCS)
	if r.water.Err != nil || shaderDraws != 1 {
		t.Fatalf("water draw: err %v shader draws %d", r.water.Err, shaderDraws)
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
	r.water.Err = errors.New("no shader") // as when water.kage fails to compile
	frames := map[string]bool{}
	for p := uint64(0); p < 4*env.WaterFrameTicks; p += env.WaterFrameTicks {
		r.present = p
		frames[r.waterFrameKey("shallow_water", 3, 4)] = true
	}
	if len(frames) != world.SoilVariants {
		t.Fatalf("%d animated frames, want %d", len(frames), world.SoilVariants)
	}
	r.reducedMotion = true
	frames = map[string]bool{}
	for p := uint64(0); p < 4*env.WaterFrameTicks; p += env.WaterFrameTicks {
		r.present = p
		frames[r.waterFrameKey("shallow_water", 3, 4)] = true
	}
	if len(frames) != 1 {
		t.Fatalf("Reduced Motion holds one frame, saw %d", len(frames))
	}
	if r.pipe().Env.Water.WaterShader() != nil {
		t.Fatal("a failed compile is not retried: the baked frames stay")
	}
}

func TestArrivalEffectDraw(t *testing.T) {
	t.Parallel()
	pods := []game.ArrivalView{{X: 5, Y: 6, Tick: 1000}, {X: 9, Y: 2, Tick: 1100, Landed: true}}
	// long before: nothing
	if fx := overlaypkg.ArrivalEffects(pods, 100, false); len(fx) != 0 {
		t.Fatalf("effects before the fall: %+v", fx)
	}
	// falling: the streak is above its landing tile and drops as the tick nears
	early := overlaypkg.ArrivalEffects(pods, 1000-overlaypkg.ArrivalFallTicks+4, false)
	late := overlaypkg.ArrivalEffects(pods, 1000-4, false)
	if len(early) != 1 || len(late) != 1 || early[0].Key != "effect/pod_fall" {
		t.Fatalf("fall effects: %+v %+v", early, late)
	}
	if !(early[0].Y < late[0].Y) || late[0].Y >= 6.5 || late[0].X != 5.5 || late[0].Alpha <= early[0].Alpha {
		t.Fatalf("the streak should drop onto tile (5,6): early %+v late %+v", early[0], late[0])
	}
	// landed: dust on the tile, swelling and fading
	d0 := overlaypkg.ArrivalEffects(pods[1:], 1100, false)
	d1 := overlaypkg.ArrivalEffects(pods[1:], 1100+overlaypkg.ArrivalDustTicks/2, false)
	if len(d0) != 1 || len(d1) != 1 || d0[0].Key != "effect/pod_dust" || d0[0].X != 9.5 || d0[0].Y != 2.5 {
		t.Fatalf("dust: %+v %+v", d0, d1)
	}
	if !(d1[0].Scale > d0[0].Scale) || !(d1[0].Alpha < d0[0].Alpha) {
		t.Fatalf("dust swells and fades: %+v -> %+v", d0[0], d1[0])
	}
	// the first pod: dust after its landing, gone once it has settled
	if fx := overlaypkg.ArrivalEffects(pods[:1], 1000+overlaypkg.ArrivalDustTicks, false); len(fx) != 0 {
		t.Fatalf("dust never ends: %+v", fx)
	}
	// Reduced Motion: no streak, and the dust only fades
	if fx := overlaypkg.ArrivalEffects(pods[:1], 990, true); len(fx) != 0 {
		t.Fatalf("a streak under Reduced Motion: %+v", fx)
	}
	a, b := overlaypkg.ArrivalEffects(pods[:1], 1000, true), overlaypkg.ArrivalEffects(pods[:1], 1050, true)
	if a[0].Scale != 1 || b[0].Scale != 1 || !(b[0].Alpha < a[0].Alpha) {
		t.Fatalf("reduced dust %+v %+v", a, b)
	}
}

func TestArrivalsDrawWithRealArt(t *testing.T) {
	art := realArt(t)
	g := game.New(game.Config{Seed: 3, Width: 20, Height: 12})
	r := New(g)
	r.art = art
	screen := ebiten.NewImage(640, 400)
	tile := r.cam.Tile()
	r.pipe().Env.SetView(r.liveView(), 640, 400, tile)
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
	if f := world.CaveMouthFacing(w, 6, 4); f != "west" {
		t.Fatalf("mined edge tile faces %q, want west (the open ground)", f)
	}
	if f := world.CaveMouthFacing(w, 7, 4); f != "" {
		t.Fatalf("a rock wall is not a mouth: %q", f)
	}
}
