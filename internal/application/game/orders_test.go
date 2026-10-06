package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
)

func build(g *Game, kind construction.Kind, f construction.Facing, x, y int) {
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(kind), Facing: uint8(f), X: x, Y: y})
}

func building(g *Game, x, y int, kind string) (BuildingView, bool) {
	for _, b := range g.World().Buildings {
		if b.X == x && b.Y == y && b.Kind == kind {
			return b, true
		}
	}
	return BuildingView{}, false
}

func TestCanPlaceReasons(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	if _, err := g.SpawnStack(item.Wood, 5, 3, 3); err != nil {
		t.Fatal(err)
	}
	if ok, r := g.CanPlace("wall", 9, 0); ok || r != ReasonOutside {
		t.Fatalf("outside: %v %q", ok, r)
	}
	if ok, r := g.CanPlace("wall", 3, 3); ok || r != ReasonItem {
		t.Fatalf("wall on items: %v %q", ok, r)
	}
	if ok, _ := g.CanPlace("table", 3, 3); !ok {
		t.Fatal("furniture may be planned over items")
	}
	if ok, r := g.CanPlace("castle", 1, 1); ok || r != ReasonUnknown {
		t.Fatalf("unknown: %v %q", ok, r)
	}
	build(g, construction.Floor, construction.South, 1, 1)
	build(g, construction.Table, construction.South, 1, 1)
	g.Step(1)
	if ok, r := g.CanPlace("floor", 1, 1); ok || r != ReasonPlanned {
		t.Fatalf("second floor: %v %q", ok, r)
	}
	if ok, r := g.CanPlace("chair", 1, 1); ok || r != ReasonPlanned {
		t.Fatalf("second object: %v %q", ok, r)
	}
	if n := len(g.World().Frames); n != 2 {
		t.Fatalf("floor and table should share the tile, frames=%d", n)
	}
	g.grid.SetWalkable(5, 5, false)
	if ok, r := g.CanPlace("bed", 5, 5); ok || r != ReasonBlocked {
		t.Fatalf("blocked: %v %q", ok, r)
	}
}

func TestBuildFloorAndTableOnSameTile(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	if _, err := g.SpawnNamedPawn(1, 1, "Ada"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Wood, 10, 2, 1); err != nil {
		t.Fatal(err)
	}
	build(g, construction.Floor, construction.South, 5, 3)
	build(g, construction.Chair, construction.West, 5, 3)
	g.Step(TicksPerCell * 200)
	if _, ok := building(g, 5, 3, "floor"); !ok {
		t.Fatalf("floor not built; frames=%+v", g.World().Frames)
	}
	c, ok := building(g, 5, 3, "chair")
	if !ok {
		t.Fatalf("chair not built; frames=%+v", g.World().Frames)
	}
	if c.Facing != "west" || c.Floor {
		t.Fatalf("chair = %+v", c)
	}
	if g.World().Wood != 10-1-2 {
		t.Fatalf("wood left = %d", g.World().Wood)
	}
}

func TestDoorIsWalkableWallIsNotAndEjectsBuilder(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 5})
	id, _ := g.SpawnNamedPawn(1, 2, "Bo")
	if _, err := g.SpawnStack(item.Wood, 10, 2, 2); err != nil {
		t.Fatal(err)
	}
	build(g, construction.Door, construction.South, 4, 2)
	build(g, construction.Wall, construction.South, 4, 1)
	g.Step(TicksPerCell * 200)
	if _, ok := building(g, 4, 2, "door"); !ok {
		t.Fatal("door not built")
	}
	if _, ok := building(g, 4, 1, "wall"); !ok {
		t.Fatal("wall not built")
	}
	if !g.grid.Walkable(4, 2) || g.grid.Walkable(4, 1) {
		t.Fatal("door must stay walkable, wall must block")
	}
	p := pawn(t, g, id)
	if p.X == 4 && p.Y == 1 {
		t.Fatal("builder left standing inside the wall")
	}
}

func TestDeconstructRefundsAndFreesTile(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 5})
	g.SpawnNamedPawn(1, 2, "Cora")
	g.SpawnStack(item.Wood, 4, 1, 1)
	build(g, construction.Wall, construction.South, 4, 2)
	g.Step(TicksPerCell * 120)
	if g.grid.Walkable(4, 2) {
		t.Fatal("wall should be up")
	}
	g.Enqueue(command.Command{Kind: command.DesignateDeconstruct, X: 4, Y: 2, X2: 4, Y2: 2})
	g.Step(1)
	if b, _ := building(g, 4, 2, "wall"); !b.Deconstruct {
		t.Fatal("wall should be marked for deconstruction")
	}
	g.Step(TicksPerCell * 80)
	if _, ok := building(g, 4, 2, "wall"); ok {
		t.Fatal("wall not removed")
	}
	if !g.grid.Walkable(4, 2) {
		t.Fatal("tile should be walkable again")
	}
	if g.World().Wood != 4-2+construction.Refund(construction.Wall) {
		t.Fatalf("wood after refund = %d", g.World().Wood)
	}
}

func TestCancelClearsDeconstructMark(t *testing.T) {
	g := New(Config{Seed: 1, Width: 6, Height: 4})
	g.SpawnStack(item.Wood, 4, 0, 0)
	build(g, construction.Table, construction.South, 3, 2)
	g.Step(1)
	fid := g.layerFrameAt(3, 2, construction.LayerObject)
	fr, _ := g.frames.Get(fid)
	g.completeFrame(fid, fr)
	g.Enqueue(command.Command{Kind: command.DesignateDeconstruct, X: 0, Y: 0, X2: 5, Y2: 3})
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 3, Y: 2, X2: 3, Y2: 2})
	g.Step(1)
	if b, _ := building(g, 3, 2, "table"); b.Deconstruct {
		t.Fatal("cancel should clear the deconstruct mark")
	}
}

func TestDraftedPawnIgnoresWorkUntilUndrafted(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	id, _ := g.SpawnNamedPawn(1, 2, "Ada")
	g.SpawnStack(item.Wood, 4, 2, 2)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	build(g, construction.Wall, construction.South, 6, 2)
	g.Step(TicksPerCell * 60)
	if p := pawn(t, g, id); !p.Drafted || p.Job != "idle" {
		t.Fatalf("drafted pawn should hold, drafted=%v job=%s", p.Drafted, p.Job)
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 8, Y: 4})
	g.Step(TicksPerCell * 20)
	if p := pawn(t, g, id); p.X != 8 || p.Y != 4 {
		t.Fatalf("drafted move: at (%d,%d)", p.X, p.Y)
	}
	g.Step(TicksPerCell * 20)
	if p := pawn(t, g, id); p.X != 8 || p.Y != 4 {
		t.Fatal("drafted pawn must stay where ordered")
	}
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: false})
	g.Step(TicksPerCell * 150)
	if _, ok := building(g, 6, 2, "wall"); !ok {
		t.Fatal("undrafted pawn should resume building")
	}
}

func TestPrioritizeOverridesCurrentJobAndTiredness(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 5})
	id, _ := g.SpawnNamedPawn(1, 2, "Bo")
	g.SpawnStack(item.Food, 3, 10, 2)
	g.SpawnStack(item.Wood, 6, 1, 3)
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 0, Y: 0, X2: 1, Y2: 0})
	build(g, construction.Bed, construction.South, 6, 2) // 1x2: covers (6,2) and (6,3)
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: uint64(id), Order: command.OrderSupply, X: 6, Y: 3})
	g.Step(2)
	if p := pawn(t, g, id); !strings.HasPrefix(p.Job, "haul") {
		t.Fatalf("prioritized supply should start a haul, job=%s", p.Job)
	}
	g.SetNeeds(id, 9000, needs.CriticalRest)
	g.Step(1)
	if p := pawn(t, g, id); !strings.HasPrefix(p.Job, "haul") {
		t.Fatalf("tiredness must not interrupt a prioritized job, job=%s", p.Job)
	}
}

func TestOrdersAtReasons(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 5})
	id, _ := g.SpawnNamedPawn(1, 2, "Cora")
	build(g, construction.Bed, construction.South, 5, 2)
	g.Step(1)
	opts := g.OrdersAt(id, 5, 2)
	if len(opts) != 2 || opts[0].Label != "Go here" || opts[0].Disabled {
		t.Fatalf("opts = %+v", opts)
	}
	if !opts[1].Disabled || opts[1].Reason != ReasonNoWood {
		t.Fatalf("supply without wood should be disabled: %+v", opts[1])
	}
	g.SpawnStack(item.Wood, 5, 8, 2)
	g.SetWork(id, work.Haul, false)
	if o := g.OrdersAt(id, 5, 2)[1]; !o.Disabled || o.Reason != ReasonNoHaul {
		t.Fatalf("haul disabled: %+v", o)
	}
	g.SetWork(id, work.Haul, true)
	o := g.OrdersAt(id, 5, 2)[1]
	if o.Disabled || o.Cmd.Kind != command.Prioritize || o.Cmd.Order != command.OrderSupply {
		t.Fatalf("supply should be available: %+v", o)
	}
	items := g.OrdersAt(id, 8, 2)
	if len(items) != 2 || items[1].Reason != ReasonNoStockpile {
		t.Fatalf("haul with no stockpile: %+v", items)
	}
	g.grid.SetWalkable(3, 0, false)
	if o := g.OrdersAt(id, 3, 0)[0]; !o.Disabled || o.Reason != ReasonBlocked {
		t.Fatalf("go here on blocked: %+v", o)
	}
}

func TestBuildablesMatchCatalog(t *testing.T) {
	b := Buildables()
	if len(b) != len(construction.All()) || b[0].Kind != "wall" || !b[0].Blocks {
		t.Fatalf("buildables = %+v", b)
	}
}
