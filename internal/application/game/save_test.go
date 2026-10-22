package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

func busyColony(t *testing.T) *Game {
	t.Helper()
	g := NewColony(Config{Seed: 9, Width: 48, Height: 40})
	cx, cy := g.Width()/2, g.Height()/2
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: cx + 1, Y: cy + 3, X2: cx + 4, Y2: cy + 5})
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 0, Y: 0, X2: g.Width() - 1, Y2: g.Height() - 1})
	g.Enqueue(command.Command{Kind: command.DesignateHunt, X: 0, Y: 0, X2: g.Width() - 1, Y2: g.Height() - 1})
	id, _ := CropID("radish")
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: cx - 6, Y: cy - 6, X2: cx - 3, Y2: cy - 4, Build: id})
	g.BuildNow("campfire", "wood", "south", cx-2, cy+4)
	g.FuelCampfire(cx-2, cy+4)
	g.SpawnStack(item.Parka, 1, cx+5, cy)
	g.Step(9000)
	return g
}

func sameState(t *testing.T, label string, a, b *Game) {
	t.Helper()
	if !reflect.DeepEqual(a.Snapshot(), b.Snapshot()) {
		t.Fatalf("%s: pawns differ\n%+v\n%+v", label, a.Snapshot(), b.Snapshot())
	}
	wa, wb := a.World(), b.World()
	if !reflect.DeepEqual(wa, wb) {
		t.Fatalf("%s: world differs", label)
	}
	if a.Tick() != b.Tick() {
		t.Fatalf("%s: tick %d vs %d", label, a.Tick(), b.Tick())
	}
}

func TestSaveLoadRoundTripContinuesIdentically(t *testing.T) {
	g := busyColony(t)
	var buf bytes.Buffer
	if err := g.Save(&buf, "test colony"); err != nil {
		t.Fatal(err)
	}
	info, err := ReadSaveInfo(bytes.NewReader(buf.Bytes()))
	if err != nil || info.Name != "test colony" || info.Colonists != 3 {
		t.Fatalf("header %+v, %v", info, err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	sameState(t, "after load", g, h)
	// both keep running in lock step: RNG, allocator and stores restored
	for i := 0; i < 6; i++ {
		g.Step(2500)
		h.Step(2500)
		sameState(t, "running on", g, h)
	}
}

func TestLoadRejectsGarbage(t *testing.T) {
	if _, _, err := Load(bytes.NewReader([]byte("not a save"))); err == nil {
		t.Fatal("garbage should not load")
	}
}

func saveLoad(t *testing.T, g *Game) *Game {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "stone"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSaveRoundTripGeologySmooths(t *testing.T) {
	g := NewColony(Config{Seed: 4, Width: 40, Height: 30})
	if !g.SetGround(1, 1, 3, 1, "rock", "marble", "silver") || !g.SetGround(1, 2, 3, 2, "rock", "slate", "") {
		t.Fatal("set ground")
	}
	g.Enqueue(command.Command{Kind: command.DesignateSmooth, X: 1, Y: 2, X2: 3, Y2: 2})
	g.ApplyPending()
	h := saveLoad(t, g)
	if !reflect.DeepEqual(g.geo, h.geo) || !reflect.DeepEqual(g.smooths, h.smooths) || len(h.smooths) != 3 {
		t.Fatalf("geology or smooth marks lost: %d marks", len(h.smooths))
	}
	sameState(t, "after load", g, h)
	g.Step(600)
	h.Step(600)
	sameState(t, "running on", g, h)
}

func TestLegacyStoneMigrates(t *testing.T) {
	g := New(Config{Seed: 2, Width: 16, Height: 10})
	for x := 0; x < 16; x++ {
		g.setTerrain(x, 9, terrain.Rock)
	}
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SpawnStack(item.Stone, 7, 2, 2)
	// a stone wall and a stone wall frame from before stone types
	g.placeFrameStuff(construction.Wall, item.GraniteBlocks, construction.South, 5, 5)
	fid := g.layerFrameAt(5, 5, construction.LayerObject)
	fr, _ := g.frames.Get(fid)
	g.completeFrame(fid, fr)
	bid := g.layerBldgAt(5, 5, construction.LayerObject)
	b, _ := g.bldgs.Get(bid)
	b.Stuff = item.Stone
	g.bldgs.Set(bid, b)
	g.placeFrameStuff(construction.Wall, item.GraniteBlocks, construction.South, 7, 5)
	fid = g.layerFrameAt(7, 5, construction.LayerObject)
	fr, _ = g.frames.Get(fid)
	fr.Stuff = item.Stone
	g.frames.Set(fid, fr)
	build := command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Wall), Stuff: uint8(item.Stone), X: 9, Y: 5}
	g.pending = append(g.pending, build)
	g.orderQueue[id] = append(g.orderQueue[id], queuedOrder{cmd: build, label: "wall"})
	g.postponed.orders[id] = queuedOrder{cmd: build, label: "wall"}
	g.geo = geology.Map{} // an older save has no geology
	h := saveLoad(t, g)

	if h.stockOf(item.Stone) != 0 || h.stockOf(item.GraniteBlocks) != 7 {
		t.Fatalf("stacks: stone %d granite %d", h.stockOf(item.Stone), h.stockOf(item.GraniteBlocks))
	}
	if b, _ := h.bldgs.Get(bid); b.Stuff != item.GraniteBlocks {
		t.Fatalf("building stuff %s", b.Stuff)
	}
	if fr, _ := h.frames.Get(fid); fr.Stuff != item.GraniteBlocks {
		t.Fatalf("frame stuff %s", fr.Stuff)
	}
	want := uint8(item.GraniteBlocks)
	if h.pending[len(h.pending)-1].Stuff != want || h.orderQueue[id][0].cmd.Stuff != want || h.postponed.orders[id].cmd.Stuff != want {
		t.Fatal("commands keep the legacy stone")
	}
	if h.geo.W != 16 || h.geo.H != 10 || len(h.geo.Stone) != 160 {
		t.Fatalf("geology not regenerated: %dx%d", h.geo.W, h.geo.H)
	}
	if again := saveLoad(t, g); !reflect.DeepEqual(again.geo, h.geo) {
		t.Fatal("regenerated geology must be deterministic")
	}
	if h.World().StructVer == 0 || h.World().Stone != 7 {
		t.Fatalf("view after migration: ver %d stone %d", h.World().StructVer, h.World().Stone)
	}
}
