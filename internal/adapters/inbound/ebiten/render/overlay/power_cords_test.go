package overlay

import (
	"image/color"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/sim/entity"
)

// cordScene is w as one chunk over a 40x40 map, seen at zoom 1 on 640x640.
func cordScene(w *game.WorldView) (*Set, *render.View, *Chunks) {
	o := testSet(&testHost{})
	v := testView(*w)
	v.MapW, v.MapH = 40, 40
	cv := game.ChunkView{}
	for i := range w.Buildings {
		cv.Buildings = append(cv.Buildings, int32(i))
	}
	for i := range w.Frames {
		cv.Frames = append(cv.Frames, int32(i))
	}
	return o, v, &Chunks{Views: []game.ChunkView{cv}}
}

// cordsOf draws the wires once and lists the parts traced.
func cordsOf(o *Set, v *render.View, ch *Chunks, w *game.WorldView) []string {
	var parts []string
	o.Wiring.Nets.Trace = func(part string, x, y int, c color.NRGBA) { parts = append(parts, part) }
	o.Wiring.DrawCords(v, ch, ebiten.NewImage(640, 640), w, 640, 640, 64)
	return parts
}

// wired is a device on net 1 with a wire from (x, y) to (lx, ly).
func wired(kind string, x, y, lx, ly int) game.BuildingView {
	return game.BuildingView{Kind: kind, X: x, Y: y, W: 1, H: 1, PowerRole: "consumer", PowerNet: 1,
		HasLink: true, LinkFromX: x, LinkFromY: y, LinkX: lx, LinkY: ly}
}

// Every device with a real wire gets one, in the cord colour, from its
// from cell, whatever the wire crosses and however hidden its conduit is:
// the read model's link decides, not the sprites around it.
func TestCordsForWiredDevices(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{Buildings: []game.BuildingView{
		{Kind: "wall", X: 3, Y: 1, W: 1, H: 1},
		{Kind: HiddenConduitKind, X: 3, Y: 1, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 1}, // under the wall
		{Kind: "wall", X: 3, Y: 2, W: 1, H: 1},
		wired("wall lamp", 2, 1, 3, 1), // hung on it, wired to the conduit in it
		wired(HeaterKind, 6, 1, 3, 1),
		wired(CoolerKind, 3, 2, 3, 1), // set in the wall
		{Kind: WoodGeneratorKind, X: 1, Y: 5, W: 2, H: 2, PowerRole: "producer", PowerNet: 1, HasLink: true, LinkFromX: 2, LinkFromY: 5, LinkX: 3, LinkY: 1},
	}}
	o, v, ch := cordScene(w)
	got := map[[2]int]color.NRGBA{}
	o.Wiring.Nets.Trace = func(part string, x, y int, c color.NRGBA) {
		if part != "cord" {
			t.Errorf("part %s", part)
		}
		got[[2]int{x, y}] = c
	}
	o.Wiring.DrawCords(v, ch, ebiten.NewImage(640, 640), w, 640, 640, 64)
	for _, from := range [][2]int{{2, 1}, {6, 1}, {3, 2}, {2, 5}} {
		if got[from] != TokenNRGBA(uikit.PowerCord) {
			t.Errorf("wire from %v: %v in %v", from, got[from], got)
		}
	}
	if len(got) != 4 {
		t.Errorf("wires %v, want the four devices", got)
	}
}

// No wire without a link, or with nowhere to run.
func TestCordsNoneWithoutARealWire(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{Buildings: []game.BuildingView{
		{Kind: ConduitKind, X: 1, Y: 1, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 1},
		{Kind: WoodGeneratorKind, X: 2, Y: 1, W: 2, H: 2, PowerRole: "producer", PowerNet: 1}, // touching: on the grid
		{Kind: BatteryKind, X: 8, Y: 8, W: 1, H: 2, PowerRole: "storage"},                     // alone
		wired("lamp", 1, 1, 1, 1), // a conduit right under it
		{Kind: "vent", X: 5, Y: 5, W: 1, H: 1},
		{Kind: "sentry turret", X: 6, Y: 6, W: 1, H: 1, PowerRole: "consumer"}, // off the grid
		{Kind: "wall", X: 7, Y: 7, W: 1, H: 1},
	}}
	o, v, ch := cordScene(w)
	if parts := cordsOf(o, v, ch, w); len(parts) != 0 {
		t.Fatalf("drew %v", parts)
	}
	o.Wiring.Cords.batch.Reset()
	o.Wiring.fillCords(v, ch, w, 640, 640, 64)
	if len(o.Wiring.Cords.batch.Idx) != 0 {
		t.Fatal("no wire must leave nothing to draw: the scene's draw calls stay the same")
	}
}

// Buildings are baked on the whole-map tier and in a bake: no wires there.
func TestCordsNotOnTheMapTierOrInABake(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{Buildings: []game.BuildingView{wired("lamp", 2, 2, 4, 2)}}
	o, v, ch := cordScene(w)
	var layers []string
	v.Trace = &render.Trace{Layer: func(n string) { layers = append(layers, n) }}
	ch.WholeMap = true
	if parts := cordsOf(o, v, ch, w); len(parts) != 0 || len(layers) != 0 {
		t.Fatalf("map tier drew %v %v", parts, layers)
	}
	ch.WholeMap, ch.Baking = false, true
	if parts := cordsOf(o, v, ch, w); len(parts) != 0 || len(layers) != 0 {
		t.Fatalf("a bake drew %v %v", parts, layers)
	}
	ch.Baking = false
	if parts := cordsOf(o, v, ch, w); len(parts) != 1 || len(layers) != 1 || layers[0] != "power cords" {
		t.Fatalf("live: %v %v", parts, layers)
	}
}

// A wire is cut by its own box, not by its device's: one from a device off
// the screen still crosses it, one wholly off it is not batched.
func TestCordsCutToTheView(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{Buildings: []game.BuildingView{
		wired("lamp", 12, 3, 8, 3),   // off to the east, its wire reaches in
		wired("lamp", 20, 3, 15, 3),  // wholly off to the east
		wired("lamp", 3, 12, 3, 8),   // off to the south, reaching in
		wired("lamp", 3, 20, 3, 15),  // wholly off to the south
		wired("lamp", -3, 3, 1, 3),   // off to the west, reaching in
		wired("lamp", 3, -3, 3, -10), // wholly off to the north
	}}
	o, v, ch := cordScene(w)
	var got [][2]int
	o.Wiring.Nets.Trace = func(part string, x, y int, c color.NRGBA) { got = append(got, [2]int{x, y}) }
	o.Wiring.DrawCords(v, ch, ebiten.NewImage(640, 640), w, 640, 640, 64)
	if len(got) != 3 || got[0] != [2]int{12, 3} || got[1] != [2]int{3, 12} || got[2] != [2]int{-3, 3} {
		t.Fatalf("drew %v", got)
	}
}

// One wire is its line and its plug; the plug sits on the wiring end.
func TestCordGeometry(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{Buildings: []game.BuildingView{wired("lamp", 2, 2, 5, 2)}}
	o, v, ch := cordScene(w)
	o.Wiring.fillCords(v, ch, w, 640, 640, 64)
	b := &o.Wiring.Cords.batch
	if len(b.Verts) != 2*(8+4) || len(b.Idx) != 2*(6*3+6) {
		t.Fatalf("%d vertices, %d indices", len(b.Verts), len(b.Idx))
	}
	// the rim first (line, plug), wider and fainter, so the wire lies on top
	if r := b.Verts[2]; r.ColorA != cordRimA || r.DstX != 3*64 {
		t.Fatalf("rim %+v", r)
	}
	if rl, cl := b.Verts[8].DstX, b.Verts[20].DstX; rl >= cl {
		t.Fatalf("the rim's plug starts at %v, the wire's at %v", rl, cl)
	}
	rimC, wireC := b.Verts[2], b.Verts[14]
	if rimC.ColorR <= wireC.ColorR || rimC.ColorG <= wireC.ColorG || rimC.ColorB <= wireC.ColorB {
		t.Fatalf("rim %+v is no paler than the wire %+v", rimC, wireC)
	}
	// the core line (the middle strip) is 64 * 0.04 - 1 px across, from the
	// lamp cell's edge to the wiring cell's centre
	if a, c := b.Verts[14], b.Verts[15]; a.DstX != 3*64 || c.DstX != 5.5*64 || a.ColorA != 1 {
		t.Fatalf("core %+v -> %+v", a, c)
	}
	if b.Verts[12].ColorA != 0 || b.Verts[19].ColorA != 0 {
		t.Fatal("the edge fades out")
	}
	if want := float32(5.5 * 64); b.Verts[20].DstX >= want || b.Verts[21].DstX <= want {
		t.Fatalf("plug %v..%v does not sit on the wiring end", b.Verts[20].DstX, b.Verts[21].DstX)
	}
	if tile := 16.0; max(1, tile*cordWidth) != 1 {
		t.Fatal("a wire stays at least a pixel wide")
	}
}

// A planned device shows the wire it will have, dimmer, in the colour of
// the grid it will join; what joins by touching, or isn't a device, doesn't.
func TestBlueprintCords(t *testing.T) {
	t.Parallel()
	frame := func(id int, kind string, x, y, fw, fh int) game.FrameView {
		return game.FrameView{ID: entity.EntityID(id), Kind: kind, X: x, Y: y, W: fw, H: fh}
	}
	w := &game.WorldView{StructVer: 1, PowerVer: 1, PowerNets: []game.PowerNetView{{ID: 2}, {ID: 3}}, Buildings: []game.BuildingView{
		{Kind: ConduitKind, X: 5, Y: 5, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 3},
	}, Frames: []game.FrameView{
		frame(10, "lamp", 7, 5, 1, 1),            // in range
		frame(11, "wall lamp", 5, 2, 1, 1),       // in range
		frame(12, "lamp", 5, 5, 1, 1),            // on the conduit: nowhere to run
		frame(13, "lamp", 20, 20, 1, 1),          // out of range
		frame(14, BatteryKind, 6, 5, 1, 2),       // touching
		frame(15, WoodGeneratorKind, 3, 5, 2, 2), // touching
		frame(16, BatteryKind, 4, 3, 1, 2),       // only corner to corner with the conduit: a wire
		frame(17, ConduitKind, 8, 8, 1, 1),
		{ID: 18, Kind: "floor", Floor: true, X: 6, Y: 7, W: 1, H: 1},
	}}
	o, v, ch := cordScene(w)
	seen := map[[2]int]color.NRGBA{}
	o.Wiring.Nets.Trace = func(part string, x, y int, c color.NRGBA) {
		if part != "blueprint cord" {
			t.Errorf("part %s", part)
		}
		seen[[2]int{x, y}] = c
	}
	screen := ebiten.NewImage(640, 640)
	o.Wiring.DrawCords(v, ch, screen, w, 640, 640, 64)
	n3 := o.Wiring.Nets.NetColour(3)
	if n3 == o.Wiring.Nets.NetColour(0) || n3 == o.Wiring.Nets.NetColour(2) {
		t.Fatalf("net 3's colour %v", n3)
	}
	for _, from := range [][2]int{{7, 5}, {5, 2}, {4, 4}} {
		if seen[from] != n3 {
			t.Errorf("wire from %v: %v in %v", from, seen[from], seen)
		}
	}
	if len(seen) != 3 {
		t.Errorf("wires %v, want the lamp, the wall lamp and the corner battery", seen)
	}
	o.Wiring.fillCords(v, ch, w, 640, 640, 64)
	for _, vx := range o.Wiring.Cords.batch.Verts {
		if vx.ColorA != 0 && vx.ColorA != blueprintCordA && vx.ColorA != blueprintCordA*cordRimA {
			t.Fatalf("alpha %v, want the blueprint's %v or its rim's", vx.ColorA, blueprintCordA)
		}
	}
	o.Wiring.Cords.batch.Reset()

	// a conduit built beside the far lamp: the kept link is dropped with the
	// transmitters it was read from
	w.Buildings = append(w.Buildings, game.BuildingView{Kind: ConduitKind, X: 22, Y: 20, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 2})
	w.StructVer++
	clear(seen)
	o.Wiring.DrawCords(v, ch, screen, w, 640, 640, 64)
	if len(seen) != 3 { // the far lamp is off this screen
		t.Errorf("after a build: %v", seen)
	}
	if p := o.Wiring.planOf(v, w, &w.Frames[3]); !p.wired || p.to != [2]int{22, 20} || p.net != 2 {
		t.Errorf("far lamp's plan %+v, want a wire to the new conduit", p)
	}
}

// A wire to a lone generator is grey: no grid yet, though it will be one.
// A battery touching that generator wires the pair out, past the generator,
// to what it reaches alone; with nothing to reach, none.
func TestBlueprintCordsByALoneGenerator(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{StructVer: 1, Buildings: []game.BuildingView{
		{Kind: WoodGeneratorKind, X: 7, Y: 5, W: 2, H: 2, PowerRole: "producer"},
	}, Frames: []game.FrameView{{ID: 9, Kind: "lamp", X: 11, Y: 5, W: 1, H: 1}}}
	o, v, ch := cordScene(w)
	seen := map[[2]int]color.NRGBA{}
	o.Wiring.Nets.Trace = func(part string, x, y int, col color.NRGBA) { seen[[2]int{x, y}] = col }
	screen := ebiten.NewImage(640, 640)
	o.Wiring.DrawCords(v, ch, screen, w, 640, 640, 64)
	if c := seen[[2]int{11, 5}]; len(seen) != 1 || c != TokenNRGBA(uikit.PowerNone) {
		t.Fatalf("the lamp's wire: %v", seen)
	}

	w.Frames = []game.FrameView{{ID: 10, Kind: BatteryKind, X: 6, Y: 5, W: 1, H: 2}}
	ch.Views[0].Frames = []int32{0}
	clear(seen)
	o.Wiring.DrawCords(v, ch, screen, w, 640, 640, 64)
	if len(seen) != 0 {
		t.Fatalf("a battery with only the generator: %v", seen)
	}

	w.StructVer++
	w.PowerNets = []game.PowerNetView{{ID: 20}}
	w.Buildings = append(w.Buildings, game.BuildingView{Kind: ConduitKind, X: 0, Y: 5, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 20}) // seven from the generator, six from the battery
	o.Wiring.DrawCords(v, ch, screen, w, 640, 640, 64)
	if p := o.Wiring.planOf(v, w, &w.Frames[0]); !p.wired || p.to != [2]int{0, 5} || p.net != 20 {
		t.Fatalf("the battery's plan %+v, want a wire to the conduit", p)
	}
}

// cordAllocWorld is a colony's worth of wires and planned wires.
func cordAllocWorld() *game.WorldView {
	w := &game.WorldView{StructVer: 1, PowerVer: 1, PowerNets: []game.PowerNetView{{ID: 1}}}
	w.Buildings = append(w.Buildings, game.BuildingView{Kind: ConduitKind, X: 4, Y: 4, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 1})
	for i := range 40 {
		w.Buildings = append(w.Buildings, wired("lamp", 1+i%8, 1+i/8, 4, 4))
		w.Frames = append(w.Frames, game.FrameView{ID: entity.EntityID(100 + i), Kind: "lamp", X: 1 + i%8, Y: 6 + i/8, W: 1, H: 1})
	}
	w.Frames = append(w.Frames, game.FrameView{ID: 99, Kind: BatteryKind, X: 5, Y: 4, W: 1, H: 2}) // touching
	return w
}

// The pass allocates nothing per frame: the batching outright, and the
// whole draw nothing over Ebitengine's own call.
func TestCordsAllocateNothing(t *testing.T) { // not parallel: AllocsPerRun
	w := cordAllocWorld()
	o, v, ch := cordScene(w)
	screen := ebiten.NewImage(640, 640)
	o.Wiring.DrawCords(v, ch, screen, w, 640, 640, 64) // warm: the buffers and the kept links
	fill := testing.AllocsPerRun(100, func() {
		o.Wiring.Cords.batch.Reset()
		o.Wiring.fillCords(v, ch, w, 640, 640, 64)
	})
	if fill != 0 {
		t.Fatalf("batching allocates %v per frame", fill)
	}
	if n := len(o.Wiring.Cords.batch.Idx); n == 0 {
		t.Fatal("nothing batched")
	}
	verts, idx := len(o.Wiring.Cords.batch.Verts), len(o.Wiring.Cords.batch.Idx)
	var b gfx.TriBatch
	b.Verts, b.Idx = append(b.Verts, o.Wiring.Cords.batch.Verts...), append(b.Idx, o.Wiring.Cords.batch.Idx...)
	white := o.whiteImg()
	ebitenOnly := testing.AllocsPerRun(200, func() {
		b.Verts, b.Idx = b.Verts[:verts], b.Idx[:idx]
		b.Flush(screen, white, ebiten.BlendSourceOver)
	})
	draw := testing.AllocsPerRun(200, func() { o.Wiring.DrawCords(v, ch, screen, w, 640, 640, 64) })
	// +1: Ebiten's own sync.Pool (invokeUsageCallbacks) jitters under GC and -race
	if draw > ebitenOnly+1 {
		t.Fatalf("DrawCords allocates %v per frame, Ebiten alone %v", draw, ebitenOnly)
	}
}

// A batch past 16-bit vertex numbers still draws, in one call: TriBatch's
// indices are uint32.
func TestCordsPastAUint16Batch(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{}
	for range 3000 { // 72000 vertices
		w.Buildings = append(w.Buildings, wired("lamp", 2, 2, 5, 2))
	}
	o, v, ch := cordScene(w)
	n := 0
	o.Wiring.Nets.Trace = func(string, int, int, color.NRGBA) { n++ }
	o.Wiring.fillCords(v, ch, w, 640, 640, 64)
	if n != len(w.Buildings) || len(o.Wiring.Cords.batch.Verts) <= math.MaxUint16 {
		t.Fatalf("%d wires, %d vertices", n, len(o.Wiring.Cords.batch.Verts))
	}
	o.Wiring.Cords.batch.Flush(ebiten.NewImage(640, 640), o.whiteImg(), ebiten.BlendSourceOver)
	if len(o.Wiring.Cords.batch.Idx) != 0 {
		t.Fatal("the batch was not left empty")
	}
}

// The overlay's red frame means "on no grid": a generator touching a
// conduit has no line and no frame; a lone one has the frame.
func TestPowerFrameOnNoNetOnly(t *testing.T) {
	t.Parallel()
	o := testSet(&testHost{})
	w := &game.WorldView{PowerNets: []game.PowerNetView{{ID: 1}}, Buildings: []game.BuildingView{
		{Kind: ConduitKind, X: 1, Y: 1, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 1},
		{Kind: WoodGeneratorKind, X: 2, Y: 1, W: 2, H: 2, PowerRole: "producer", PowerNet: 1},
		{Kind: BatteryKind, X: 7, Y: 7, W: 1, H: 2, PowerRole: "storage", CapWh: 100},
		wired("lamp", 5, 1, 1, 1),
	}}
	v := testView(*w)
	ch := &Chunks{Views: []game.ChunkView{{Buildings: []int32{0, 1, 2, 3}}}}
	type part struct {
		name string
		x, y int
	}
	seen := map[part]bool{}
	o.Wiring.Nets.Trace = func(p string, x, y int, c color.NRGBA) { seen[part{p, x, y}] = true }
	o.Power.On = true
	o.DrawPowerOverlay(v, ch, &Links{}, ebiten.NewImage(640, 640), w, 640, 640, 64)
	if seen[part{"disconnected", 2, 1}] || seen[part{"link", 2, 1}] {
		t.Errorf("a generator touching a conduit: %v", seen)
	}
	if !seen[part{"disconnected", 7, 7}] {
		t.Errorf("the lone battery has no red frame: %v", seen)
	}
	if !seen[part{"link", 5, 1}] || seen[part{"disconnected", 5, 1}] {
		t.Errorf("the wired lamp: %v", seen)
	}
}

// A device can wire to any conductor: built conduits and closed switches,
// generators and batteries, even lone ones; an open switch cuts them too.
func TestTransmittersIncludeGeneratorsAndBatteries(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{StructVer: 1, Buildings: []game.BuildingView{
		{Kind: WoodGeneratorKind, X: 1, Y: 1, W: 2, H: 2, PowerRole: "producer", PowerNet: 4},
		{Kind: BatteryKind, X: 5, Y: 1, W: 1, H: 2, PowerRole: "storage"},
		{Kind: "lamp", X: 7, Y: 1, W: 1, H: 1, PowerRole: "consumer", PowerNet: 4},
		{Kind: BatteryKind, X: 9, Y: 1, W: 1, H: 2, PowerRole: "storage"},
		{Kind: PowerSwitchKind, X: 9, Y: 2, W: 1, H: 1, PowerRole: "switch"},
	}}
	var o PowerOverlay
	is := o.Transmitters(w)
	for c, want := range map[[2]int]bool{{1, 1}: true, {2, 2}: true, {5, 1}: true, {5, 2}: true, {7, 1}: false, {9, 1}: true, {9, 2}: false} {
		if is(c[0], c[1]) != want {
			t.Errorf("cell %v: want %v", c, want)
		}
	}
	if o.NetAt([2]int{2, 2}) != 4 || o.NetAt([2]int{5, 1}) != 0 {
		t.Errorf("nets %d %d", o.NetAt([2]int{2, 2}), o.NetAt([2]int{5, 1}))
	}
}

// Touching means a shared cell or a 4-neighbour, and only what conducts
// joins that way; what it joins decides the rest.
func TestGhostWire(t *testing.T) {
	t.Parallel()
	bat := func(x, y int) [][2]int { return [][2]int{{x, y}, {x, y + 1}} }
	conduit := &game.WorldView{Buildings: []game.BuildingView{
		{Kind: ConduitKind, X: 5, Y: 5, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 5},
	}}
	loner := &game.WorldView{Buildings: []game.BuildingView{
		{Kind: WoodGeneratorKind, X: 5, Y: 5, W: 2, H: 2, PowerRole: "producer"},
	}}
	pair := &game.WorldView{Buildings: []game.BuildingView{ // a wireless grid of a generator and a battery
		{Kind: WoodGeneratorKind, X: 5, Y: 5, W: 2, H: 2, PowerRole: "producer", PowerNet: 9},
		{Kind: BatteryKind, X: 7, Y: 5, W: 1, H: 2, PowerRole: "storage", PowerNet: 9},
	}}
	apart := &game.WorldView{Buildings: []game.BuildingView{
		{Kind: WoodGeneratorKind, X: 5, Y: 5, W: 2, H: 2, PowerRole: "producer"},
		{Kind: ConduitKind, X: 12, Y: 8, W: 1, H: 1, PowerRole: "transmitter", PowerNet: 5}, // seven from the generator
	}}
	for _, c := range []struct {
		name          string
		w             *game.WorldView
		kind          string
		cells         [][2]int
		to            [2]int
		wired, joined bool
	}{
		{"on it", conduit, BatteryKind, bat(5, 5), [2]int{}, false, true},
		{"north of it", conduit, BatteryKind, bat(5, 3), [2]int{}, false, true},
		{"south of it", conduit, BatteryKind, bat(5, 6), [2]int{}, false, true},
		{"west of it", conduit, BatteryKind, bat(4, 5), [2]int{}, false, true},
		{"east of it", conduit, BatteryKind, bat(6, 5), [2]int{}, false, true},
		{"corner to corner", conduit, BatteryKind, bat(4, 3), [2]int{5, 5}, true, false},
		{"a gap", conduit, BatteryKind, bat(5, 7), [2]int{5, 5}, true, false},
		{"out of range", conduit, BatteryKind, bat(12, 5), [2]int{}, false, false},
		{"a generator on its side", conduit, WoodGeneratorKind, [][2]int{{3, 5}, {4, 5}, {3, 6}, {4, 6}}, [2]int{}, false, true},
		{"a lamp beside it", conduit, "lamp", [][2]int{{6, 5}}, [2]int{5, 5}, true, false},
		{"a lamp alone", conduit, "lamp", [][2]int{{15, 5}}, [2]int{}, false, false},
		{"no such kind", conduit, "no such kind", [][2]int{{15, 5}}, [2]int{}, false, false},
		{"off the map", conduit, BatteryKind, [][2]int{{-1, 5}, {25, 5}}, [2]int{}, false, false},

		{"a lone generator, nothing else", loner, BatteryKind, bat(4, 5), [2]int{}, false, true},
		{"a lamp by a lone generator", loner, "lamp", [][2]int{{4, 5}}, [2]int{5, 5}, true, false},
		{"a battery in reach of a lone one", loner, BatteryKind, bat(3, 5), [2]int{5, 5}, true, false},
		{"a lone generator, a conduit past it", apart, BatteryKind, bat(7, 5), [2]int{12, 8}, true, true},
		{"on a wireless pair: the read model can't tell", pair, BatteryKind, bat(4, 5), [2]int{}, false, true},
	} {
		var o PowerOverlay
		_, to, wired, joined := o.GhostWire(c.w, c.kind, c.cells, 20, 20)
		if to != c.to || wired != c.wired || joined != c.joined {
			t.Errorf("%s: to %v wired %v joined %v", c.name, to, wired, joined)
		}
	}
}

// Every kind that makes, draws or stores power is listed, so its blueprint
// and ghost show their wire, and nothing else is.
func TestPowerDeviceKindsAreComplete(t *testing.T) {
	t.Parallel()
	n := 0
	for _, d := range construction.All() {
		r := power.SpecOf(d.Kind).Role
		device := r == power.Producer || r == power.Consumer || r == power.Storage
		if device != PowerDeviceKinds[d.Name] {
			t.Errorf("%s: role %q, listed %v", d.Name, r, PowerDeviceKinds[d.Name])
		}
		if device {
			n++
		}
	}
	if n != len(PowerDeviceKinds) {
		t.Errorf("%d devices, %d listed", n, len(PowerDeviceKinds))
	}
}

// agreeLayout is a seeded layout on a mw x mh map: conduit runs, generators,
// batteries, closed and open switches. IDs ascend from 1.
func agreeLayout(r *rand.Rand, mw, mh int) []power.Node {
	var nodes []power.Node
	add := func(role power.Role, closed bool, cells ...[2]int) {
		nodes = append(nodes, power.Node{ID: uint64(len(nodes) + 1), Role: role, Closed: closed, Cells: cells})
	}
	for range r.IntN(3) { // conduit runs
		x, y, dx := r.IntN(mw), r.IntN(mh), r.IntN(2)
		for k := range 2 + r.IntN(6) {
			if c := [2]int{x + k*dx, y + k*(1-dx)}; c[0] < mw && c[1] < mh {
				add(power.Transmitter, false, c)
			}
		}
	}
	for range r.IntN(4) {
		x, y := r.IntN(mw-1), r.IntN(mh-1)
		add(power.Producer, false, [2]int{x, y}, [2]int{x + 1, y}, [2]int{x, y + 1}, [2]int{x + 1, y + 1})
	}
	for range r.IntN(4) {
		x, y := r.IntN(mw), r.IntN(mh-1)
		add(power.Storage, false, [2]int{x, y}, [2]int{x, y + 1})
	}
	for range r.IntN(3) {
		add(power.Switch, r.IntN(2) == 0, [2]int{r.IntN(mw), r.IntN(mh)})
	}
	return nodes
}

// agreeView is the read model of nodes built on a mw x mh map, and the graph
// behind it: what the ghost sees.
func agreeView(b *power.Builder, nodes []power.Node, mw, mh int) (*game.WorldView, power.Graph) {
	g := b.Build(mw, mh, nodes)
	w := &game.WorldView{StructVer: 1}
	for i := range nodes {
		n := &nodes[i]
		x0, y0, x1, y1 := n.Cells[0][0], n.Cells[0][1], n.Cells[0][0], n.Cells[0][1]
		for _, c := range n.Cells {
			x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
		}
		w.Buildings = append(w.Buildings, game.BuildingView{ID: entity.EntityID(n.ID), X: x0, Y: y0, W: x1 - x0 + 1, H: y1 - y0 + 1,
			PowerRole: n.Role.String(), PowerNet: int(g.Net(n.ID)), PowerOn: n.Closed})
	}
	return w, g
}

// touchesWirelessDevices: foot touches a grid of only generators and
// batteries with no wire out.
func touchesWirelessDevices(nodes []power.Node, g *power.Graph, foot [][2]int) bool {
	for i := range nodes {
		n := &nodes[i]
		net := g.Net(n.ID)
		if net == 0 || !slices.ContainsFunc(n.Cells, func(c [2]int) bool {
			return slices.ContainsFunc(foot, func(f [2]int) bool {
				dx, dy := f[0]-c[0], f[1]-c[1]
				return dx*dx+dy*dy <= 1
			})
		}) {
			continue
		}
		wireless := true
		for j := range nodes {
			m := &nodes[j]
			_, linked := g.Link(m.ID)
			if g.Net(m.ID) == net && (linked || (m.Role != power.Producer && m.Role != power.Storage)) {
				wireless = false
			}
		}
		if wireless {
			return true
		}
	}
	return false
}

// The ghost and the simulation choose the same wire for a new device: the
// same from and to, the same "joined, no wire", the same "none in range".
//
// Three cases they cannot agree on, and why they are left out:
//   - A new device on an open switch's cell. Build leaves that cell out of
//     the conductors, so a battery there can't touch through it; the ghost
//     can't tell a cut cell from an empty one. Placement refuses the spot:
//     the switch is on the same layer.
//   - "Not connected" on a new generator or battery that a consumer already
//     standing nearby and unwired would wire to. Build puts the two on a net
//     (a consumer counts); the ghost only reads conductors. It is a tag on a
//     ghost, and the layouts here hold no consumers, so it can't arise.
//   - A new generator or battery touching a wireless grid of only generators
//     and batteries (net != 0), with a conductor in the new one's own range.
//     Build wires the new one out; the ghost, which can't tell that grid from
//     one with a wire without reading every member, shows no wire. Skipped
//     only when that is exactly what happened (gaps below). A lone battery
//     with a consumer wired to it is the same gap; no consumers here.
func TestGhostWireAgreesWithBuild(t *testing.T) {
	t.Parallel()
	const mw, mh, newID = 36, 28, 100000
	kinds := []string{"lamp", HeaterKind, "electric stove", BatteryKind, WoodGeneratorKind, SolarPanelKind, WindTurbineKind}
	r := rand.New(rand.NewPCG(7, 21))
	var b power.Builder
	wires, joins, lone, joinWires, gaps := 0, 0, 0, 0, 0
	for layout := range 600 {
		nodes := agreeLayout(r, mw, mh)
		w, g0 := agreeView(&b, nodes, mw, mh)
		cut := map[[2]int]bool{}
		for i := range nodes {
			if nodes[i].Role == power.Switch && !nodes[i].Closed {
				cut[nodes[i].Cells[0]] = true
			}
		}
		var o PowerOverlay
		for try := range 16 {
			name := kinds[r.IntN(len(kinds))]
			k, ok := construction.Parse(name)
			if !ok {
				t.Fatalf("no kind %s", name)
			}
			x, y := r.IntN(mw), r.IntN(mh)
			if try%2 == 0 && len(nodes) > 0 { // beside something, half the time
				c := nodes[r.IntN(len(nodes))].Cells[0]
				x, y = max(c[0]+r.IntN(5)-2, 0), max(c[1]+r.IntN(5)-2, 0)
			}
			foot, _, _ := construction.Footprint(k, construction.Facing(r.IntN(4)), x, y)
			on := false
			for _, c := range foot {
				on = on || cut[c] || c[0] >= mw || c[1] >= mh
			}
			if on {
				continue
			}
			g := b.Build(mw, mh, append(nodes[:len(nodes):len(nodes)], power.Node{ID: newID, Role: power.SpecOf(k).Role, Cells: foot}))
			l, linked := g.Link(newID)
			from, to, wired, joined := o.GhostWire(w, name, foot, mw, mh)
			switch {
			case joined && !wired && linked && touchesWirelessDevices(nodes, &g0, foot):
				gaps++
			case wired != linked:
				t.Fatalf("layout %d: %s %v: ghost wired %v to %v (joined %v), Build %v %+v", layout, name, foot, wired, to, joined, linked, l)
			case wired:
				wires++
				if joined {
					joinWires++
				}
				if from != [2]int{l.FromX, l.FromY} || to != [2]int{l.X, l.Y} {
					t.Fatalf("layout %d: %s %v: ghost %v -> %v, Build %+v", layout, name, foot, from, to, l)
				}
				if g.Net(newID) == 0 {
					t.Fatalf("layout %d: %s %v: wired on no net", layout, name, foot)
				}
			case joined:
				joins++
				if g.Net(newID) == 0 {
					t.Fatalf("layout %d: %s %v: joined in the ghost, on no net in Build", layout, name, foot)
				}
			default:
				lone++
				if g.Net(newID) != 0 {
					t.Fatalf("layout %d: %s %v: Not connected in the ghost, Build net %d", layout, name, foot, g.Net(newID))
				}
			}
		}
	}
	t.Logf("%d wires (%d out of a pair), %d joined, %d alone, %d in the known gap", wires, joinWires, joins, lone, gaps)
	if wires < 100 || joins < 20 || lone < 100 || joinWires == 0 {
		t.Fatalf("the layouts barely test it: %d wires (%d out of a pair), %d joined, %d alone", wires, joinWires, joins, lone)
	}
}

// A wire leaves its device at the from cell's edge, on the line to the wiring
// cell's centre, in every direction: it never crosses the sprite it powers.
func TestCordStartsAtTheCellEdge(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		from, to [2]int
		x, y     float64
	}{
		{[2]int{2, 2}, [2]int{5, 2}, 3, 2.5},    // east
		{[2]int{5, 2}, [2]int{2, 2}, 5, 2.5},    // west
		{[2]int{2, 2}, [2]int{2, 3}, 2.5, 3},    // south, next door
		{[2]int{2, 6}, [2]int{2, 2}, 2.5, 6},    // north
		{[2]int{2, 2}, [2]int{4, 4}, 3, 3},      // the corner on a diagonal
		{[2]int{2, 2}, [2]int{6, 4}, 3, 2.75},   // shallow
		{[2]int{8, 8}, [2]int{7, 2}, 8.4167, 8}, // steep, back and up
	} {
		x, y := cordStart(c.from, c.to)
		if math.Abs(x-c.x) > 1e-3 || math.Abs(y-c.y) > 1e-3 {
			t.Errorf("%v -> %v starts at (%v, %v), want (%v, %v)", c.from, c.to, x, y, c.x, c.y)
		}
	}
}

// Under the objects a wire starts at its device's cell centre and stops where
// the wire over them takes over, so the two meet whatever the sprite's size.
// No stub for a device on its wiring, and none off the view.
func TestCordStubJoinsTheDevice(t *testing.T) {
	t.Parallel()
	w := &game.WorldView{Buildings: []game.BuildingView{
		wired("lamp", 2, 2, 5, 2),
		wired("lamp", 6, 6, 6, 6),
		wired("lamp", 20, 20, 23, 20),
	}}
	o, v, ch := cordScene(w)
	o.Wiring.fillCordStubs(v, ch, w, 640, 640, 64)
	b := &o.Wiring.Cords.stubs
	if len(b.Verts) != 2*8 {
		t.Fatalf("%d vertices, want one stub with its rim", len(b.Verts))
	}
	o.Wiring.fillCords(v, ch, w, 640, 640, 64)
	if a, c, main := b.Verts[10], b.Verts[11], o.Wiring.Cords.batch.Verts[14]; a.DstX != 2.5*64 || c.DstX != main.DstX {
		t.Fatalf("stub %v..%v, the wire over the objects starts at %v", a.DstX, c.DstX, main.DstX)
	}
	o.Wiring.Cords.batch.Reset()
	o.Wiring.Cords.stubs.Reset()
}
