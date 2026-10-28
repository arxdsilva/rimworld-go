package ebiten

import (
	"bufio"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 power presentation (power_draw.go): sprite keys, the conduit
// autotile, glow layers, turbine blades, the torch flame, the battery
// bar, the no-power badge and the light sources from Glow.

func TestPowerSpriteKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind, facing string
		mask         int
		open         bool
		want         string
	}{
		{"power conduit", "", 5, false, "power/power_conduit_05"},
		{"power conduit", "", 0, false, "power/power_conduit_00"},
		{"hidden conduit", "", 10, false, "power/hidden_conduit_10"},
		{"power switch", "", 0, false, "power/power_switch_on"},
		{"power switch", "", 0, true, "power/power_switch_off"}, // an open switch is off
		{"wood-fired generator", "", 0, false, "power/wood_fired_generator"},
		{"solar panel", "", 0, false, "power/solar_panel"},
		{"wind turbine", "east", 0, false, "power/wind_turbine_east"},
		{"wind turbine", "", 0, false, "power/wind_turbine_south"},
		{"geothermal generator", "", 0, false, "power/geothermal_generator"},
		{"battery", "north", 0, false, "power/battery_north"},
		{"battery", "west", 0, false, "power/battery_west"},
		{"torch lamp", "", 0, false, "furniture/torch_lamp"},
		{"heater", "", 0, false, "power/heater"},
		{"cooler", "west", 0, false, "power/cooler_west"},
		{"cooler", "", 0, false, "power/cooler_south"},
		{"electric stove", "east", 0, false, "furniture/electric_stove_east"},
		{"electric tailoring bench", "north", 0, false, "furniture/electric_tailoring_bench_north"},
		{"electric research bench", "", 0, false, "furniture/electric_research_bench_south"},
		{"lamp", "", 0, false, "furniture/lamp_standing"},
	}
	for _, c := range cases {
		if got := buildSprite(c.kind, c.facing, c.mask, false, c.open); got != c.want {
			t.Errorf("buildSprite(%q, %q, %d, open %v) = %q, want %q", c.kind, c.facing, c.mask, c.open, got, c.want)
		}
		// steel is the only stuff electric pieces take: same key
		if got := buildSpriteStuff(c.kind, "steel", c.facing, c.mask, false, c.open); got != c.want {
			t.Errorf("buildSpriteStuff(%q, steel) = %q, want %q", c.kind, got, c.want)
		}
	}
	for _, k := range []string{"power/battery_south", "power/heater", "furniture/electric_stove_east", "power/wind_turbine_west"} {
		want := map[string]string{
			"power/battery_south":           "power/battery_glow_south",
			"power/heater":                  "power/heater_glow",
			"furniture/electric_stove_east": "furniture/electric_stove_glow_east",
			"power/wind_turbine_west":       "power/wind_turbine_glow_west",
		}[k]
		if got := glowKey(k); got != want {
			t.Errorf("glowKey(%q) = %q, want %q", k, got, want)
		}
	}
}

// pdKinds are the PLAY-021 kinds the renderer maps.
var pdKinds = []string{"power conduit", "hidden conduit", "power switch", "wood-fired generator", "solar panel",
	"wind turbine", "geothermal generator", "battery", "torch lamp", "heater", "cooler", "electric stove",
	"electric tailoring bench", "electric research bench", "lamp"}

// readMeta reads one generated metadata file's top-level scalars, or
// reports it missing.
func pdMeta(t *testing.T, root, key string) (map[string]string, bool) {
	t.Helper()
	f, err := os.Open(filepath.Join(root, artMetadataRel, key+".yaml"))
	if err != nil {
		return nil, false
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			out[k] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return out, true
}

// TestPowerArtCoversRenderer: every sprite the power renderer asks for is
// generated (metadata on disk, no decoding), the glow keys are the ones
// the base metadata names, and the constants mirror the metadata.
func TestPowerArtCoversRenderer(t *testing.T) {
	t.Parallel()
	root, err := LocateArt()
	if err != nil {
		t.Skip(err)
	}
	for _, kind := range pdKinds {
		for _, f := range facingNames {
			for _, open := range []bool{false, true} {
				for m := 0; m < 16; m++ {
					if m > 0 && !isConduitKind(kind) {
						break
					}
					key := buildSprite(kind, f, m, false, open)
					meta, ok := pdMeta(t, root, key)
					if !ok {
						t.Errorf("%s (%s %s open %v mask %d): no generated sprite", key, kind, f, open, m)
						continue
					}
					g, has := meta["glow"]
					_, glowFile := pdMeta(t, root, glowKey(key))
					switch {
					case has && g != glowKey(key):
						t.Errorf("%s: metadata glow %q, renderer asks %q", key, g, glowKey(key))
					case has && !glowFile:
						t.Errorf("%s: glow layer %q not generated", key, g)
					case !has && glowFile:
						t.Errorf("%s: a glow layer %q exists but the metadata names none", key, glowKey(key))
					}
				}
			}
		}
	}
	for i := 1; i <= turbineFrames; i++ {
		meta, ok := pdMeta(t, root, bladeKey(i))
		if !ok {
			t.Fatalf("%s missing", bladeKey(i))
		}
		if meta["frames"] != strconv.Itoa(turbineFrames) || meta["step_deg"] != strconv.Itoa(turbineStepDeg) {
			t.Errorf("%s: frames %s step %s, renderer assumes %d at %d°", bladeKey(i), meta["frames"], meta["step_deg"], turbineFrames, turbineStepDeg)
		}
	}
	torch, _ := pdMeta(t, root, buildSprite(torchLampKind, "", 0, false, false))
	if v, err := strconv.ParseFloat(torch["flame_scale"], 64); err != nil || v != torchFlameScale {
		t.Errorf("torch flame_scale %q, renderer draws %v", torch["flame_scale"], torchFlameScale)
	}
	for _, k := range []string{noPowerIcon} {
		if _, ok := pdMeta(t, root, k); !ok {
			t.Errorf("%s missing", k)
		}
	}
	for _, f := range facingNames { // the attachments the parts hang on
		for key, names := range map[string][]string{
			buildSprite(windTurbineKind, f, 0, false, false): {"rotor"},
			buildSprite(batteryKind, f, 0, false, false):     {"gauge0", "gauge1"},
		} {
			raw, err := os.ReadFile(filepath.Join(root, artMetadataRel, key+".yaml"))
			if err != nil {
				t.Fatal(err)
			}
			m, err := parseArtMeta(string(raw))
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range names {
				if _, ok := m.Attach[n]; !ok {
					t.Errorf("%s: no %s attachment", key, n)
				}
			}
		}
	}
}

// powerWorld builds a read model from rows: C conduit, H hidden conduit,
// X power switch, L a lamp (consumer), B a battery (storage, 1x1 here),
// W wall, G a 2x2 generator anchored at G (its other cells are '.').
func pdWorld(rows ...string) (game.WorldView, int, int) {
	h, wd := len(rows), len(rows[0])
	w := game.WorldView{TerrainW: wd, Terrain: make([]uint8, wd*h), StructVer: 1}
	id := entity.EntityID(1)
	add := func(b game.BuildingView) {
		b.ID = id
		id++
		if b.W == 0 {
			b.W, b.H = 1, 1
		}
		w.Buildings = append(w.Buildings, b)
	}
	for y, row := range rows {
		for x, ch := range row {
			switch ch {
			case 'C':
				add(game.BuildingView{Kind: conduitKind, Stuff: "steel", X: x, Y: y, PowerRole: "transmitter", PowerNet: 1})
			case 'H':
				add(game.BuildingView{Kind: hiddenConduitKind, Stuff: "steel", X: x, Y: y, PowerRole: "transmitter", PowerNet: 2})
			case 'X':
				add(game.BuildingView{Kind: powerSwitchKind, Stuff: "steel", X: x, Y: y, PowerRole: "switch", PowerOn: true})
			case 'L':
				add(game.BuildingView{Kind: "lamp", X: x, Y: y, PowerRole: "consumer"})
			case 'B':
				add(game.BuildingView{Kind: batteryKind, Stuff: "steel", X: x, Y: y, PowerRole: "storage"})
			case 'W':
				add(game.BuildingView{Kind: "wall", Stuff: "wood", X: x, Y: y})
			case 'G':
				add(game.BuildingView{Kind: woodGeneratorKind, Stuff: "steel", X: x, Y: y, W: 2, H: 2, PowerRole: "producer"})
			}
		}
	}
	return w, wd, h
}

func pdGrid(rows ...string) *linkGrid {
	w, wd, h := pdWorld(rows...)
	g := &linkGrid{}
	g.Sync(w, wd, h)
	return g
}

func TestConduitMaskAll16(t *testing.T) {
	t.Parallel()
	for m := 0; m < 16; m++ {
		cells := []byte(".........")
		cells[4] = 'C'
		for bit, at := range map[int]int{wallN: 1, wallE: 5, wallS: 7, wallW: 3} {
			if m&bit != 0 {
				cells[at] = 'C'
			}
		}
		g := pdGrid(string(cells[0:3]), string(cells[3:6]), string(cells[6:9]))
		if got := g.ConduitMask(1, 1); got != m {
			t.Fatalf("mask %d: got %d", m, got)
		}
	}
}

// TestConduitLinksToTransmittersAndDevices is the linking rule: a conduit
// joins conduits (shown or hidden), switches and the power devices beside
// it (the art draws a stub into them), never walls; walls never join
// conduits, even one laid under them.
func TestConduitLinksToTransmittersAndDevices(t *testing.T) {
	t.Parallel()
	g := pdGrid(
		".H.",
		"XCL",
		".W.",
	)
	if got := g.ConduitMask(1, 1); got != wallN|wallE|wallW {
		t.Fatalf("conduit mask %d, want hidden N, lamp E, switch W (not the wall S)", got)
	}
	if got := g.ConduitMask(1, 0); got != wallS {
		t.Fatalf("hidden conduit mask %d, want the conduit S", got)
	}
	walls := pdGrid(
		"WCW",
		".C.",
	)
	if got := walls.Mask(0, 0); got != 0 {
		t.Fatalf("a wall beside a conduit links %d", got)
	}
	// a conduit under a wall: the wall keeps its wall mask, the conduit its own
	w, wd, h := pdWorld("WW.", "...")
	w.Buildings = append(w.Buildings, game.BuildingView{ID: 99, Kind: conduitKind, X: 1, Y: 0, W: 1, H: 1})
	w.Buildings = append(w.Buildings, game.BuildingView{ID: 98, Kind: conduitKind, X: 1, Y: 1, W: 1, H: 1})
	under := &linkGrid{}
	under.Sync(w, wd, h)
	if got := under.Mask(1, 0); got != wallW {
		t.Fatalf("wall over a conduit: mask %d, want the wall W only", got)
	}
	if got := under.ConduitMask(1, 0); got != wallS {
		t.Fatalf("conduit under a wall: mask %d, want the conduit S only", got)
	}
	// a 2x2 generator's whole footprint is a device; a battery and a lamp too
	gen := pdGrid(
		"G..",
		"...",
		".C.",
		".B.",
	)
	if got := gen.ConduitMask(1, 2); got != wallN|wallS {
		t.Fatalf("conduit under a generator's second row: mask %d, want N (generator) and S (battery)", got)
	}
}

func TestConduitFramesAndGhostsLink(t *testing.T) {
	t.Parallel()
	w, wd, h := pdWorld("C..", "...")
	g := &linkGrid{}
	g.Sync(w, wd, h)
	p := g.Plan([]game.FrameView{{Kind: conduitKind, X: 1, Y: 0, W: 1, H: 1}}, []plannedCell{{X: 2, Y: 0, Flags: conduitLink.Flags}})
	if got := g.PowerMask(conduitLink.Accepts, 1, 0, p); got != wallE|wallW {
		t.Fatalf("planned conduit mask %d, want the built conduit W and the ghost E", got)
	}
	// a planned device draws a stub too; walls planned beside do not
	p = g.Plan([]game.FrameView{{Kind: heaterKind, X: 0, Y: 1, W: 1, H: 1}, {Kind: "wall", X: 1, Y: 0, W: 1, H: 1}}, nil)
	if got := g.PowerMask(conduitLink.Accepts, 0, 0, p); got != wallS {
		t.Fatalf("conduit beside a planned heater and wall: mask %d, want S", got)
	}
	if got := g.PlannedMask(p, wallLink.Accepts, 1, 0); got != 0 {
		t.Fatalf("a planned wall joined a conduit: %d", got)
	}
}

// powerArt is a catalog with every key the power parts use, plus the
// attachments the generated metadata gives them.
func pdArt() *artCatalog {
	c := envArt()
	tile := func(k string, canvas [2]int, origin pt, attach map[string]pt) {
		c.Sprites[k] = fakeSprite(canvas, origin, attach)
	}
	for m := 0; m < 16; m++ {
		tile("power/power_conduit_"+twoDigits(m), [2]int{160, 171}, pt{16, 27}, nil)
		tile("power/hidden_conduit_"+twoDigits(m), [2]int{128, 128}, pt{0, 0}, nil)
	}
	for _, k := range []string{"power/heater", "power/heater_glow", "power/wood_fired_generator", "power/wood_fired_generator_glow",
		"power/geothermal_generator", "power/geothermal_generator_glow", "power/power_switch_on", "power/power_switch_off",
		"furniture/lamp_standing", "furniture/lamp_standing_glow", "effect/steam_puff_0", "effect/steam_puff_1", "effect/steam_puff_2", "effect/steam_puff_3",
		noPowerIcon, "structure/steam_vent"} {
		tile(k, [2]int{160, 216}, pt{16, 72}, nil)
	}
	c.Sprites["power/wood_fired_generator"].Meta.Attach = map[string]pt{"smoke": {67.2, 63.8}}
	c.Sprites["power/geothermal_generator"].Meta.Attach = map[string]pt{"steam": {215.7, 71.4}}
	tile("furniture/torch_lamp", [2]int{160, 235}, pt{16, 91}, map[string]pt{"fire": {80, 87.2}})
	for _, f := range facingNames {
		tile("power/battery_"+f, [2]int{160, 340}, pt{16, 68}, map[string]pt{"gauge0": {80, 233}, "gauge1": {80, 94.8}})
		tile("power/battery_glow_"+f, [2]int{160, 340}, pt{16, 68}, nil)
		tile("power/wind_turbine_"+f, [2]int{160, 401}, pt{16, 129}, map[string]pt{"rotor": {80, 127.7}})
		tile("power/wind_turbine_glow_"+f, [2]int{160, 401}, pt{16, 129}, nil)
	}
	for i := 1; i <= turbineFrames; i++ {
		tile(bladeKey(i), [2]int{218, 312}, pt{45, 168}, map[string]pt{"hub": {109, 166.7}})
	}
	return c
}

// powerRenderer is a renderer over powerArt that records the power parts
// it draws as "part key".
func pdRenderer(t *testing.T, w game.WorldView) (*Renderer, *[]string) {
	t.Helper()
	r := New(&stubSim{w: 20, h: 12, world: w})
	r.art = pdArt()
	r.view.W = w
	r.links.Sync(w, 20, 12)
	cv := game.ChunkView{}
	for i := range w.Buildings {
		cv.Buildings = append(cv.Buildings, int32(i))
	}
	for i := range w.Frames {
		cv.Frames = append(cv.Frames, int32(i))
	}
	r.lod.Views = []game.ChunkView{cv}
	got := &[]string{}
	r.overlays().Wiring.Parts.Trace = func(part, key string) { *got = append(*got, part+" "+key) }
	return r, got
}

func pdHas(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func pdCount(list []string, p string) int {
	n := 0
	for _, v := range list {
		if strings.HasPrefix(v, p) {
			n++
		}
	}
	return n
}

// TestHiddenConduitsOnlyInOverlay: the normal pass draws shown conduits,
// autotiled, and nothing for hidden ones; the overlay markings draw both
// kinds' marking, each multiplied by its net's colour.
func TestHiddenConduitsOnlyInOverlay(t *testing.T) {
	w, _, _ := pdWorld(
		"CCH",
		"....",
	)
	r, got := pdRenderer(t, w)
	dst := ebiten.NewImage(640, 400)
	tile := r.tilePx()
	r.drawConduits(dst, w, 640, 400, tile)
	if !pdHas(*got, "conduit power/power_conduit_02") || !pdHas(*got, "conduit power/power_conduit_10") {
		t.Fatalf("normal pass drew %v, want the two linked conduits", *got)
	}
	if pdCount(*got, "conduit power/hidden") != 0 || len(*got) != 2 {
		t.Fatalf("normal pass drew a hidden conduit: %v", *got)
	}
	*got = nil
	nets := map[int]int{}
	colour := func(net int) color.NRGBA { nets[net]++; return color.NRGBA{R: uint8(net * 40), A: 255} }
	r.drawPowerMarkings(dst, &w, tile, 640, 400, colour, true)
	if len(*got) != 1 || (*got)[0] != "marking power/hidden_conduit_08" || nets[2] != 1 || nets[1] != 0 {
		t.Fatalf("hidden-only markings %v, nets %v; want the hidden conduit (W link) on net 2", *got, nets)
	}
	*got = nil
	r.drawPowerMarkings(dst, &w, tile, 640, 400, colour, false)
	if len(*got) != 3 || nets[1] != 2 {
		t.Fatalf("all markings %v, nets %v", *got, nets)
	}
}

// TestConduitsLeaveTheObjectPass: conduits are drawn in their own pass
// under objects, so neither the live object pass nor the map bake queues
// them.
func TestConduitsLeaveTheObjectPass(t *testing.T) {
	t.Parallel()
	for _, k := range []string{conduitKind, hiddenConduitKind} {
		if !onFloor(k) || !isConduitKind(k) {
			t.Fatalf("%s is drawn in the object pass", k)
		}
	}
	if isConduitKind(powerSwitchKind) || onFloor(powerSwitchKind) {
		t.Fatal("the switch is an object")
	}
}

// TestGlowOnlyWhilePowered: a glow layer is drawn unlit over the sprite
// only while the device runs: a consumer while Powered, a producer while
// it makes power, a battery while it holds charge on a net.
func TestGlowOnlyWhilePowered(t *testing.T) {
	cases := []struct {
		name string
		b    game.BuildingView
		glow string // "" none
	}{
		{"powered heater", game.BuildingView{Kind: heaterKind, PowerRole: "consumer", Powered: true}, "power/heater_glow"},
		{"unpowered heater", game.BuildingView{Kind: heaterKind, PowerRole: "consumer", NoPowerReason: game.NoPowerBrownout}, ""},
		{"switched-off heater", game.BuildingView{Kind: heaterKind, PowerRole: "consumer", NoPowerReason: game.NoPowerSwitchedOff}, ""},
		{"running generator", game.BuildingView{Kind: woodGeneratorKind, PowerRole: "producer", PowerOn: true, PowerOut: 800, W: 2, H: 2}, "power/wood_fired_generator_glow"},
		{"idle generator", game.BuildingView{Kind: woodGeneratorKind, PowerRole: "producer", PowerOn: true, W: 2, H: 2}, ""},
		{"charged battery", game.BuildingView{Kind: batteryKind, Facing: "east", PowerRole: "storage", PowerNet: 3, StoredWh: 10, CapWh: 1200}, "power/battery_glow_east"},
		{"empty battery", game.BuildingView{Kind: batteryKind, Facing: "east", PowerRole: "storage", PowerNet: 3, CapWh: 1200}, ""},
		{"loose battery", game.BuildingView{Kind: batteryKind, Facing: "east", PowerRole: "storage", StoredWh: 10, CapWh: 1200}, ""},
	}
	for _, c := range cases {
		c.b.X, c.b.Y, c.b.ID = 4, 4, 7
		if c.b.W == 0 {
			c.b.W, c.b.H = 1, 1
		}
		w := game.WorldView{Buildings: []game.BuildingView{c.b}}
		r, got := pdRenderer(t, w)
		r.drawBuildingObj(ebiten.NewImage(640, 400), &w.Buildings[0], r.tilePx(), 640, 400)
		n := pdCount(*got, "glow ")
		switch {
		case c.glow == "" && n != 0:
			t.Errorf("%s: drew a glow %v", c.name, *got)
		case c.glow != "" && !pdHas(*got, "glow "+c.glow):
			t.Errorf("%s: drew %v, want glow %s", c.name, *got, c.glow)
		}
	}
	// never while baking (bakes are cached; power moves without a rebake)
	b := game.BuildingView{Kind: heaterKind, PowerRole: "consumer", Powered: true, X: 4, Y: 4, W: 1, H: 1}
	w := game.WorldView{Buildings: []game.BuildingView{b}}
	r, got := pdRenderer(t, w)
	r.lod.Baking = true
	r.drawBuildingObj(ebiten.NewImage(640, 400), &w.Buildings[0], r.tilePx(), 640, 400)
	r.lod.Baking = false
	if len(*got) != 0 {
		t.Fatalf("a bake drew power parts: %v", *got)
	}
}

// TestUnpoweredLampDrawsNoGlow: the standing lamp's bulb is plain glass in
// its sprite; the lit bulb is the glow layer, drawn only while the lamp
// has power.
func TestUnpoweredLampDrawsNoGlow(t *testing.T) {
	for _, c := range []struct {
		name string
		b    game.BuildingView
		glow bool
	}{
		{"unpowered lamp", game.BuildingView{Kind: "lamp", PowerRole: "consumer", NoPowerReason: game.NoPowerNotConnected}, false},
		{"switched-off lamp", game.BuildingView{Kind: "lamp", PowerRole: "consumer", NoPowerReason: game.NoPowerSwitchedOff}, false},
		{"powered lamp", game.BuildingView{Kind: "lamp", PowerRole: "consumer", Powered: true}, true},
	} {
		c.b.X, c.b.Y, c.b.W, c.b.H, c.b.ID = 4, 4, 1, 1, 7
		w := game.WorldView{Buildings: []game.BuildingView{c.b}}
		r, got := pdRenderer(t, w)
		r.drawBuildingObj(ebiten.NewImage(640, 400), &w.Buildings[0], r.tilePx(), 640, 400)
		lit := pdHas(*got, "glow furniture/lamp_standing_glow")
		if lit != c.glow || pdCount(*got, "glow ") != map[bool]int{false: 0, true: 1}[c.glow] {
			t.Errorf("%s: drew %v, want lit bulb %v", c.name, *got, c.glow)
		}
	}
}

func TestSwitchSpriteFollowsActualState(t *testing.T) {
	for _, on := range []bool{true, false} {
		b := game.BuildingView{Kind: powerSwitchKind, PowerRole: "switch", PowerOn: on, PowerWantOn: !on, X: 2, Y: 2, W: 1, H: 1}
		w := game.WorldView{Buildings: []game.BuildingView{b}}
		r, _ := pdRenderer(t, w)
		var drawn []string
		r.lit.OnDraw = func(img *ebiten.Image, _ float32) { drawn = append(drawn, keyOf(r.art, img)) }
		r.drawBuildingObj(ebiten.NewImage(640, 400), &w.Buildings[0], r.tilePx(), 640, 400)
		want := map[bool]string{true: "power/power_switch_on", false: "power/power_switch_off"}[on]
		if len(drawn) != 1 || drawn[0] != want {
			t.Fatalf("switch on=%v (wanted %v) drew %v, want %s", on, !on, drawn, want)
		}
	}
}

func TestTurbineDrawsBladesAtTheRotor(t *testing.T) {
	b := game.BuildingView{ID: 3, Kind: windTurbineKind, PowerRole: "producer", PowerOn: true, PowerOut: 300, X: 4, Y: 4, W: 1, H: 2}
	w := game.WorldView{WindX: 1.2, Buildings: []game.BuildingView{b}}
	r, got := pdRenderer(t, w)
	r.drawBuildingObj(ebiten.NewImage(640, 400), &w.Buildings[0], r.tilePx(), 640, 400)
	if pdCount(*got, "blades power/wind_turbine_blades_") != 1 || !pdHas(*got, "glow power/wind_turbine_glow_south") {
		t.Fatalf("turbine parts %v", *got)
	}
	// the hub lands on the rotor: rotor (80, 127.7) - origin (16, 129) of
	// the base, minus hub (109, 166.7) - origin (45, 168) of the blades
	wx, wy, ok := r.bladeAt(&w.Buildings[0], buildSprite(windTurbineKind, "south", 0, false, false), bladeKey(1))
	if !ok || math.Abs(wx-4) > 1e-9 || math.Abs(wy-4) > 1e-9 {
		t.Fatalf("blades at (%v, %v) ok %v, want (4, 4)", wx, wy, ok)
	}
}

// TestTorchFlameOnlyWhileBurning: the torch draws its flame at the fire
// attachment while it burns (Burning, or giving light), never unlit.
func TestTorchFlameOnlyWhileBurning(t *testing.T) {
	for _, c := range []struct {
		b    game.BuildingView
		want bool
	}{
		{game.BuildingView{Kind: torchLampKind, Fuel: true, FuelPct: 50, Burning: true}, true},
		{game.BuildingView{Kind: torchLampKind, Glow: true, GlowRadius: 4}, true},
		{game.BuildingView{Kind: torchLampKind, Fuel: true, FuelPct: 50}, false}, // put out by rain
		{game.BuildingView{Kind: torchLampKind, Fuel: true}, false},
	} {
		c.b.X, c.b.Y, c.b.W, c.b.H, c.b.ID = 3, 3, 1, 1, 9
		w := game.WorldView{Buildings: []game.BuildingView{c.b}}
		r, got := pdRenderer(t, w)
		r.drawBuildingObj(ebiten.NewImage(640, 400), &w.Buildings[0], r.tilePx(), 640, 400)
		if n := pdCount(*got, "flame "); (n == 1) != c.want || n > 1 {
			t.Errorf("%+v: parts %v, flame %v", c.b, *got, c.want)
		}
	}
	x, y, ok := torchFireAt(pdArt(), &game.BuildingView{Kind: torchLampKind, X: 3, Y: 3})
	if !ok || math.Abs(x-3.5) > 1e-9 || math.Abs(y-(3+(87.2-91)/artTilePx)) > 1e-9 {
		t.Fatalf("fire at (%v, %v), want the fire attachment", x, y)
	}
	if !ownsFlame(torchLampKind) || !ownsFlame(woodGeneratorKind) || ownsFlame("campfire") {
		t.Fatal("drawFires must leave the torch and generator to power_draw.go and keep the campfire")
	}
}

// TestLightSourcesFromGlow: lights come from BuildingView.Glow: powered
// lamps (their metadata radius), burning torches (their GlowRadius) and
// burning campfires (the renderer's 3-tile look, not the sim's 5); an
// unpowered lamp or a campfire the sim says gives no light are dark.
func TestLightSourcesFromGlow(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Buildings: []game.BuildingView{
		{ID: 1, Kind: "lamp", X: 1, Y: 1, Glow: true, GlowRadius: 6},
		{ID: 2, Kind: "lamp", X: 2, Y: 1}, // unpowered
		{ID: 3, Kind: torchLampKind, X: 3, Y: 1, Glow: true, GlowRadius: 4, Burning: true},
		{ID: 4, Kind: torchLampKind, X: 4, Y: 1, Fuel: true}, // out
		{ID: 5, Kind: "campfire", X: 5, Y: 1, Glow: true, GlowRadius: 5, Burning: true},
		{ID: 6, Kind: "campfire", X: 6, Y: 1, Burning: true}, // no Glow: dark
		{ID: 7, Kind: heaterKind, X: 7, Y: 1, Powered: true},
		{ID: 8, Kind: "lamp", X: 8, Y: 1, Glow: true}, // no GlowRadius: the lamp radius
	}}
	got := appendLightSources(nil, &w, 5.5)
	want := []lightSource{
		{ID: 1, X: 1, Y: 1, Radius: 5.5, Kind: lightLamp},
		{ID: 3, X: 3, Y: 1, Radius: 4, Kind: lightLamp},
		{ID: 5, X: 5, Y: 1, Radius: campfireReach, Kind: lightCampfire},
		{ID: 8, X: 8, Y: 1, Radius: 5.5, Kind: lightLamp},
	}
	if len(got) != len(want) {
		t.Fatalf("sources %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("source %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if campfireReach != 3 {
		t.Fatal("the campfire keeps its 3-tile look")
	}
}

func TestBatteryBarFraction(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		stored, cap int
		want        float64
	}{{0, 1200, 0}, {600, 1200, 0.5}, {1300, 1200, 1}, {5, 0, 0}, {-3, 1200, 0}} {
		if got := batteryFill(&game.BuildingView{StoredWh: c.stored, CapWh: c.cap}); got != c.want {
			t.Errorf("fill %d/%d = %v, want %v", c.stored, c.cap, got, c.want)
		}
	}
	// the bar grows from gauge0 toward gauge1 by the fill, across the slot
	x, y, w, h := batteryBarRect(100, 300, 100, 160, 0.5, 8)
	if x != 96 || y != 230 || w != 8 || h != 70 {
		t.Fatalf("vertical half bar (%v, %v, %v, %v)", x, y, w, h)
	}
	x, y, w, h = batteryBarRect(300, 50, 100, 50, 0.25, 6)
	if x != 250 || y != 47 || w != 50 || h != 6 {
		t.Fatalf("horizontal quarter bar (%v, %v, %v, %v)", x, y, w, h)
	}
	if batteryBarColour(0.1) == batteryBarColour(0.9) {
		t.Fatal("a low battery reads differently")
	}

	b := game.BuildingView{Kind: batteryKind, PowerRole: "storage", StoredWh: 600, CapWh: 1200, X: 4, Y: 4, W: 1, H: 2}
	wv := game.WorldView{Buildings: []game.BuildingView{b}}
	r, got := pdRenderer(t, wv)
	r.drawBuildingObj(ebiten.NewImage(640, 400), &wv.Buildings[0], r.tilePx(), 640, 400)
	if !pdHas(*got, "battery power/battery_south") {
		t.Fatalf("battery parts %v", *got)
	}
	*got = nil
	wv.Buildings[0].StoredWh = 0
	r.drawBuildingObj(ebiten.NewImage(640, 400), &wv.Buildings[0], r.tilePx(), 640, 400)
	if pdCount(*got, "battery ") != 0 {
		t.Fatalf("an empty battery drew a bar: %v", *got)
	}
}

// TestNoPowerBadge: unpowered consumers get the badge (not those the
// player switched off, nor producers); it blinks softly, steady under
// reduced flashing.
func TestNoPowerBadge(t *testing.T) {
	cases := []struct {
		b    game.BuildingView
		want bool
	}{
		{game.BuildingView{PowerRole: "consumer", NoPowerReason: game.NoPowerBrownout}, true},
		{game.BuildingView{PowerRole: "consumer", NoPowerReason: game.NoPowerNotConnected}, true},
		{game.BuildingView{PowerRole: "consumer", NoPowerReason: game.NoPowerSolarFlare}, true},
		{game.BuildingView{PowerRole: "consumer", NoPowerReason: game.NoPowerSwitchedOff}, false},
		{game.BuildingView{PowerRole: "consumer", Powered: true}, false},
		{game.BuildingView{PowerRole: "producer", NoPowerReason: game.NoPowerNoFuel}, false},
		{game.BuildingView{}, false},
	}
	for _, c := range cases {
		if got := noPowerBadge(&c.b); got != c.want {
			t.Errorf("%+v: badge %v, want %v", c.b, got, c.want)
		}
	}
	lo, hi := 2.0, -1.0
	for now := uint64(0); now < 600; now++ {
		a := badgeAlpha(now, false)
		lo, hi = math.Min(lo, a), math.Max(hi, a)
		if d := math.Abs(badgeAlpha(now+1, false) - a); d > 0.05 {
			t.Fatalf("badge snaps by %v at %d: blink softly", d, now)
		}
	}
	if lo < 0.3 || hi != 1 || hi-lo < 0.3 {
		t.Fatalf("badge alpha %v..%v: a soft blink that never vanishes", lo, hi)
	}
	for now := uint64(0); now < 200; now += 7 {
		if badgeAlpha(now, true) != 1 {
			t.Fatal("reduced flashing keeps the badge steady")
		}
	}

	w := game.WorldView{Buildings: []game.BuildingView{
		{ID: 1, Kind: heaterKind, PowerRole: "consumer", NoPowerReason: game.NoPowerBrownout, X: 2, Y: 2, W: 1, H: 1},
		{ID: 2, Kind: heaterKind, PowerRole: "consumer", NoPowerReason: game.NoPowerSwitchedOff, X: 4, Y: 2, W: 1, H: 1},
		{ID: 3, Kind: heaterKind, PowerRole: "consumer", Powered: true, X: 6, Y: 2, W: 1, H: 1},
	}}
	r, got := pdRenderer(t, w)
	r.drawNoPowerBadges(ebiten.NewImage(640, 400), &w, r.tilePx())
	if len(*got) != 1 || (*got)[0] != "badge "+noPowerIcon {
		t.Fatalf("badges %v, want one", *got)
	}
}

// TestMachinePuffs: a running wood-fired generator smokes and a running
// geothermal generator steams at their attachments; idle ones don't; a
// vent under a geothermal generator no longer puffs on its own.
func TestMachinePuffs(t *testing.T) {
	gen := game.BuildingView{ID: 11, Kind: woodGeneratorKind, PowerRole: "producer", PowerOn: true, PowerOut: 800, X: 2, Y: 2, W: 2, H: 2}
	geo := game.BuildingView{ID: 12, Kind: geothermalKind, PowerRole: "producer", PowerOn: true, PowerOut: 1800, X: 8, Y: 2, W: 2, H: 2}
	vent := game.BuildingView{ID: 13, Kind: steamVentKind, X: 8, Y: 2, W: 2, H: 2}
	free := game.BuildingView{ID: 14, Kind: steamVentKind, X: 14, Y: 6, W: 2, H: 2}
	w := game.WorldView{Buildings: []game.BuildingView{gen, geo, vent, free}}
	r, got := pdRenderer(t, w)
	s := r.game.(*stubSim)
	smoke, steam, freeVent := false, false, false
	for tick := 0; tick < 2000; tick++ {
		s.tick = clock.Tick(tick)
		*got = nil
		r.drawSteamPuffs(ebiten.NewImage(640, 400), w, r.tilePx())
		for _, e := range *got {
			switch {
			case strings.HasPrefix(e, "smoke "):
				smoke = true
			case strings.HasPrefix(e, "steam "):
				steam = true
			case strings.HasPrefix(e, "vent "):
				if !strings.HasSuffix(e, "@14,6") {
					t.Fatalf("a covered vent puffed: %s", e)
				}
				freeVent = true
			}
		}
	}
	if !smoke || !steam || !freeVent {
		t.Fatalf("smoke %v steam %v free vent %v", smoke, steam, freeVent)
	}
	w.Buildings[0].PowerOut, w.Buildings[1].PowerOut = 0, 0
	for tick := 0; tick < 600; tick++ {
		s.tick = clock.Tick(tick)
		*got = nil
		r.drawSteamPuffs(ebiten.NewImage(640, 400), w, r.tilePx())
		if pdCount(*got, "smoke ")+pdCount(*got, "steam ") != 0 {
			t.Fatalf("idle generators puffed: %v", *got)
		}
	}
}

// TestPowerPassesInTheFrame: a whole frame draws the conduits once, in
// their own pass between the floors and the zones (not in the object
// pass), links a conduit blueprint to the built conduit, and badges the
// unpowered heater after the objects.
func TestPowerPassesInTheFrame(t *testing.T) {
	w, _, _ := pdWorld(
		"CC..",
		"....",
	)
	w.TerrainW, w.Terrain = 20, make([]uint8, 20*12)
	w.Buildings = append(w.Buildings, game.BuildingView{ID: 50, Kind: heaterKind, PowerRole: "consumer",
		NoPowerReason: game.NoPowerNotConnected, X: 6, Y: 6, W: 1, H: 1})
	w.Frames = []game.FrameView{{ID: 60, Kind: conduitKind, X: 2, Y: 0, W: 1, H: 1}}
	r, got := pdRenderer(t, w)
	var order []string
	r.view.Trace = &render.Trace{Layer: func(n string) { order = append(order, n) }}
	conduitDraws := 0
	r.lit.OnDraw = func(img *ebiten.Image, _ float32) {
		if strings.HasPrefix(keyOf(r.art, img), "power/power_conduit_") {
			conduitDraws++
		}
	}
	screen := ebiten.NewImage(640, 400)
	r.drawArt(screen, 640, 400, r.tilePx())
	at := map[string]int{}
	for i, n := range order {
		if _, ok := at[n]; !ok {
			at[n] = i
		}
	}
	fl, ok1 := at["floors"]
	co, ok2 := at["conduits"]
	zo, ok3 := at["zones"]
	if !ok1 || !ok2 || !ok3 || fl >= co || co >= zo {
		t.Fatalf("layer order %v: want floors, conduits, zones", order)
	}
	if !pdHas(*got, "conduit power/power_conduit_02") || !pdHas(*got, "conduit power/power_conduit_08") ||
		!pdHas(*got, "conduit frame power/power_conduit_08") || !pdHas(*got, "badge "+noPowerIcon) {
		t.Fatalf("power parts %v", *got)
	}
	if conduitDraws != 3 {
		t.Fatalf("conduit sprites drawn %d times, want 3 (two built, one blueprint; none in the object pass)", conduitDraws)
	}
}

// TestWallsJoinCoolers: a cooler stands in a wall gap, so the walls
// either side join it like a door (its art carries the wall's sides).
func TestWallsJoinCoolers(t *testing.T) {
	t.Parallel()
	w, wd, h := pdWorld("W.W", "...")
	w.Buildings = append(w.Buildings, game.BuildingView{ID: 40, Kind: coolerKind, Facing: "east", X: 1, Y: 0, W: 1, H: 1, PowerRole: "consumer"})
	g := &linkGrid{}
	g.Sync(w, wd, h)
	if g.Mask(0, 0) != wallE || g.Mask(2, 0) != wallW {
		t.Fatalf("walls beside a cooler: %d, %d", g.Mask(0, 0), g.Mask(2, 0))
	}
	p := g.Plan([]game.FrameView{{Kind: coolerKind, X: 1, Y: 1, W: 1, H: 1}}, nil)
	if got := g.PlannedMask(p, wallLink.Accepts, 1, 0); got&wallS == 0 {
		t.Fatalf("a planned cooler under a gap: wall mask %d", got)
	}
}
