package cue

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

func TestThunderRollsInAfterItsDelay(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	r.b.ThunderAt = 100
	r.v.Present = 99
	r.b.ListenThunder(r.v)
	if len(r.b.Take()) != 0 {
		t.Fatal("thunder before its time")
	}
	r.v.Present = 100
	r.b.ListenThunder(r.v)
	c := r.b.Take()
	if len(c) != 1 || c[0].ID != "sfx.thunder" || !c[0].Global || r.b.ThunderAt != 0 {
		t.Fatalf("thunder = %+v, pending %d", c, r.b.ThunderAt)
	}
	if d := ThunderDelay(7); d < 20 || d >= 90 {
		t.Fatalf("thunder delay %d outside 20..90", d)
	}
}

func TestSpatialPansAndCulls(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	vp := camera.ColonyViewport(1280, 720)
	if _, pan, ok := Spatial(r.v, 2, 5, vp); !ok || pan >= 0 {
		t.Fatalf("a point left of centre pans left: pan %v ok %v", pan, ok)
	}
	if _, pan, ok := Spatial(r.v, 18, 5, vp); !ok || pan <= 0 {
		t.Fatalf("a point right of centre pans right: pan %v ok %v", pan, ok)
	}
	if _, _, ok := Spatial(r.v, 400, 5, vp); ok {
		t.Fatal("a point far off screen is out of earshot")
	}
	if _, _, ok := Spatial(r.v, 2, 5, camera.Viewport{}); ok {
		t.Fatal("an empty viewport hears nothing")
	}
}

func TestListenWorldCuesBuildDoneAndNotice(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	frame := game.WorldView{Frames: []game.FrameView{{ID: 4, X: 3, Y: 2}}}
	r.b.ListenWorld(r.v, frame, nil)
	built := game.WorldView{Buildings: []game.BuildingView{{ID: 9, Kind: "wall", X: 3, Y: 2}},
		Messages: []game.MessageView{{Tick: 5}}}
	r.b.ListenWorld(r.v, built, nil)
	c := r.b.Take()
	if len(c) != 2 || c[0].ID != "sfx.build_done" || c[0].X != 3.5 || c[1].ID != "ui.notice" || !c[1].Global {
		t.Fatalf("cues = %+v, want build_done at 3.5 and a notice", c)
	}
	r.b.ListenWorld(r.v, built, nil)
	if c := r.b.Take(); len(c) != 0 {
		t.Fatalf("nothing changed, heard %+v", c)
	}
}

func TestWaterOnScreenCountsShore(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	r.b.Host = waterHost{stubHost{}}
	terrain := make([]uint8, 20*20)
	for i := range terrain {
		if i%20 < 10 {
			terrain[i] = 1
		}
	}
	w := game.WorldView{Terrain: terrain, TerrainW: 20}
	water, shore := r.b.WaterOnScreen(r.v, w, camera.Viewport{X0: 0, Y0: 0, X1: 20 * 64, Y1: 20 * 64})
	if water < 0.4 || water > 0.6 || shore <= 0 {
		t.Fatalf("water %.2f shore %.2f, want about half the view in water with a shore", water, shore)
	}
	if w, s := r.b.WaterOnScreen(r.v, game.WorldView{}, camera.Viewport{}); w != 0 || s != 0 {
		t.Fatal("no terrain, no water")
	}
}

type waterHost struct{ stubHost }

func (waterHost) IsWater(code uint8) bool { return code == 1 }

func TestMachinesHumNearestOnly(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	var bs []game.BuildingView
	for i := 0; i < 4; i++ {
		bs = append(bs, game.BuildingView{ID: entity.EntityID(10 + i), Kind: WoodGeneratorKind, X: 4 + i, Y: 4, W: 1, H: 1, Burning: true, PowerOut: 100})
	}
	want := map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{Buildings: bs}, camera.ColonyViewport(1280, 720))
	if len(want) != MaxMachineLoops {
		t.Fatalf("%d generator loops, want the nearest %d: %+v", len(want), MaxMachineLoops, want)
	}
	for _, l := range want {
		if l.ID != "amb.generator_wood" {
			t.Fatalf("loop %+v", l)
		}
	}
}

func TestArrivalsAreHeardOnceInOrder(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	const land = 1000
	pod := func(landed bool) game.WorldView {
		return game.WorldView{Arrivals: []game.ArrivalView{{X: 4, Y: 4, Tick: land, Landed: landed}}}
	}
	ids := func(tick uint64, landed bool) []string {
		r.v.Tick = clockTick(tick)
		r.b.ListenArrivals(r.v, pod(landed))
		var out []string
		for _, c := range r.b.Take() {
			out = append(out, c.ID)
		}
		return out
	}
	if got := ids(land-PodFallLead-1, false); len(got) != 0 {
		t.Fatalf("before the descent: %v", got)
	}
	if got := ids(land-PodFallLead, false); len(got) != 1 || got[0] != "sfx.pod_fall" {
		t.Fatalf("descent: %v", got)
	}
	if got := ids(land, false); len(got) != 1 || got[0] != "sfx.pod_land" {
		t.Fatalf("landing: %v", got)
	}
	if got := ids(land+1, true); len(got) != 1 || got[0] != "sfx.pod_open" {
		t.Fatalf("hatch: %v", got)
	}
	if got := ids(land+2, true); len(got) != 0 {
		t.Fatalf("heard twice: %v", got)
	}
}

func clockTick(n uint64) clock.Tick { return clock.Tick(n) }
