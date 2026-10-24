package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

func replaceGame(t *testing.T, base construction.Kind) (*Game, int, int) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	placeBuilt(t, g, base, construction.South, 5, 4)
	return g, 5, 4
}

func frameAtObj(g *Game, x, y int) (construction.Frame, bool) {
	fr, ok := g.frames.Get(g.layerFrameAt(x, y, construction.LayerObject))
	return fr, ok
}

func TestReplaceCanPlaceRules(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Wall)
	placeBuilt(t, g, construction.Door, construction.South, 8, 4)
	placeBuilt(t, g, construction.Fence, construction.South, 2, 4)
	if ok, r := g.CanPlaceAt("door", "wood", "south", x, y); !ok {
		t.Fatalf("door over wall: %q", r)
	}
	if got := g.ReplacesAt("door", "wood", x, y); got != "wall" {
		t.Fatalf("ReplacesAt = %q", got)
	}
	if g.ReplacesAt("door", "wood", 1, 1) != "" || g.ReplacesAt("wall", "wood", x, y) != "" {
		t.Fatal("replaces nothing expected")
	}
	if ok, r := g.CanPlaceAt("wall", "wood", "south", x, y); ok || r != ReasonOccupied {
		t.Fatalf("wall on wall: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("wall", "wood", "south", 8, 4); ok || r != ReasonOccupied {
		t.Fatalf("wall over door: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("door", "wood", "south", 8, 4); ok || r != ReasonOccupied {
		t.Fatalf("door on door: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("carpet", "straw carpet", "south", x, y); ok || r != ReasonOccupied {
		t.Fatalf("carpet on wall: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("fence gate", "wood", "south", 2, 4); !ok {
		t.Fatalf("gate over fence: %q", r)
	}
	if ok, r := g.CanPlaceAt("door", "wood", "south", 8, 4); ok {
		t.Fatalf("door on door: %q", r)
	}
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	if ok, r := g.CanPlaceAt("door", "wood", "south", x, y); ok || r != ReasonPlanned {
		t.Fatalf("door on planned replacement: %v %q", ok, r)
	}
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 11, 4)
	if ok, r := g.CanPlaceAt("door", "wood", "south", 11, 4); ok || r != ReasonPlanned {
		t.Fatalf("door on planned wall: %v %q", ok, r)
	}
}

func TestReplaceWallWithDoorEndsAsDoor(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Wall)
	if _, err := g.SpawnNamedPawn(2, 2, "Ada"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Wood, 20, 3, 3); err != nil {
		t.Fatal(err)
	}
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	fr, ok := frameAtObj(g, x, y)
	if !ok || fr.Replaces == 0 || !fr.ReplaceMarked {
		t.Fatalf("frame = %+v ok=%v", fr, ok)
	}
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	if b, _ := g.bldgs.Get(bid); !b.Deconstruct {
		t.Fatal("wall should be marked")
	}
	for i := 0; i < 400; i++ {
		g.Step(TicksPerCell)
		if _, standing := g.bldgs.Get(bid); standing {
			if _, built := building(g, x, y, "door"); built {
				t.Fatal("door completed while the wall stands")
			}
		}
	}
	if _, ok := building(g, x, y, "door"); !ok {
		t.Fatalf("no door; frames=%+v", g.World().Frames)
	}
	if _, ok := building(g, x, y, "wall"); ok {
		t.Fatal("wall still there")
	}
	if g.grid.Walkable(x, y) != true {
		t.Log("door cell not walkable (doors block until opened?)")
	}
}

func TestReplaceGateOverFenceAndDoorOverGate(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Fence)
	g.placeFrameStuff(construction.FenceGate, item.Wood, construction.South, x, y)
	if fr, ok := frameAtObj(g, x, y); !ok || fr.Replaces == 0 {
		t.Fatalf("gate over fence: %+v", fr)
	}
	g2, x, y := replaceGame(t, construction.FenceGate)
	g2.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	if fr, ok := frameAtObj(g2, x, y); !ok || fr.Replaces == 0 {
		t.Fatalf("door over gate: %+v", fr)
	}
}

func TestReplaceRefusesWorkWhileStanding(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Wall)
	pid, err := g.SpawnNamedPawn(2, 2, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	fid := g.layerFrameAt(x, y, construction.LayerObject)
	fr, _ := g.frames.Get(fid)
	fr.Wood = fr.Need
	g.frames.Set(fid, fr)
	if _, why := g.forcedJob(pid, command.OrderBuild, x, y, construction.LayerObject); why != "The wall must come down first" {
		t.Fatalf("forced build: %q", why)
	}
	if tgt, started := g.constructTry(pid, nil); tgt == fid && started {
		t.Fatal("constructTry started the replacing frame")
	}
	g.completeFrame(fid, fr)
	if _, ok := building(g, x, y, "door"); ok {
		t.Fatal("completeFrame built a door over a wall")
	}
	for _, f := range g.World().Frames {
		if f.ID == fid && f.Status != "Waiting for the wall to be removed" {
			t.Fatalf("status = %q", f.Status)
		}
	}
}

func TestReplaceCancelFrameRestoresWall(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Wall)
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	g.cancelFrame(g.layerFrameAt(x, y, construction.LayerObject))
	if b, _ := g.bldgs.Get(bid); b.Deconstruct {
		t.Fatal("wall still marked after cancelling the frame")
	}
	// cancelling by rectangle does the same
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	g.cancelRect(x, y, x, y)
	if b, _ := g.bldgs.Get(bid); b.Deconstruct {
		t.Fatal("wall still marked after cancelRect")
	}
	if _, ok := frameAtObj(g, x, y); ok {
		t.Fatal("frame survived cancelRect")
	}
}

func TestReplaceKeepsExistingMark(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Wall)
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	g.markDeconstruct(x, y, x, y, true)
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	fr, _ := frameAtObj(g, x, y)
	if fr.ReplaceMarked {
		t.Fatal("frame did not set the mark, so must not claim it")
	}
	g.cancelFrame(g.layerFrameAt(x, y, construction.LayerObject))
	if b, _ := g.bldgs.Get(bid); !b.Deconstruct {
		t.Fatal("the user's own mark was cleared")
	}
}

func TestReplaceUnmarkingWallCancelsFrame(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Wall)
	if _, err := g.SpawnStack(item.Wood, 4, 3, 3); err != nil {
		t.Fatal(err)
	}
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	fid := g.layerFrameAt(x, y, construction.LayerObject)
	fr, _ := g.frames.Get(fid)
	fr.Wood = 2
	g.frames.Set(fid, fr)
	g.markDeconstruct(x, y, x, y, false)
	if _, ok := frameAtObj(g, x, y); ok {
		t.Fatal("frame survived unmarking the wall")
	}
	if g.itemAt(x, y) == 0 {
		t.Fatal("delivered wood was not refunded")
	}
}

func TestReplaceSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g, x, y := replaceGame(t, construction.Wall)
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, x, y)
	want, _ := frameAtObj(g, x, y)
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	g2, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := frameAtObj(g2, x, y)
	if !ok || got.Replaces != want.Replaces || !got.ReplaceMarked || got.Replaces == 0 {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
