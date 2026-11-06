package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/industry"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// refuelRig is a pawn, a wood generator at (8, 3) and the order's command.
func refuelRig(t *testing.T) (*Game, entity.EntityID, entity.EntityID, command.Command) {
	t.Helper()
	g := fuelGame(t)
	pid, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(pid, needs.Full, needs.Full)
	gid := placeFuel(t, g, construction.WoodGenerator, 8, 3)
	g.Step(1)
	return g, pid, gid, command.Command{Kind: command.Prioritize, Pawn: uint64(pid), X: 8, Y: 3, Order: command.OrderRefuel}
}

func refuelOpt(t *testing.T, g *Game, pid entity.EntityID) OrderOption {
	t.Helper()
	for _, o := range g.OrdersAt(pid, 8, 3) {
		if o.Cmd.Order == command.OrderRefuel {
			return o
		}
	}
	t.Fatal("no refuel row")
	return OrderOption{}
}

func TestForcedRefuelFillsAboveThreshold(t *testing.T) {
	t.Parallel()
	g, pid, gid, cmd := refuelRig(t)
	spec, _ := fuelSpec(construction.WoodGenerator)
	g.setFuel(gid, spec.CapTicks*6/10)
	if _, err := g.SpawnStack(item.Wood, 90, 2, 5); err != nil {
		t.Fatal(err)
	}
	g.Step(300)
	if g.fuel[gid] > spec.CapTicks*6/10 {
		t.Fatal("auto refuel must not start above the half-tank threshold")
	}
	if o := refuelOpt(t, g, pid); o.Disabled || !strings.HasPrefix(o.Label, "Prioritize refueling") {
		t.Fatalf("row: %+v", o)
	}
	g.Enqueue(cmd)
	g.Step(1500)
	if g.fuel[gid] < spec.CapTicks-TicksPerWood-1800*generatorBurn/1000 {
		t.Fatalf("fuel %d of %d after the order", g.fuel[gid], spec.CapTicks)
	}
}

func TestForcedRefuelWorksWithAutoRefuelOff(t *testing.T) {
	t.Parallel()
	g, _, gid, cmd := refuelRig(t)
	b, _ := g.bldgs.Get(gid)
	b.NoAutoRefuel = true
	g.bldgs.Set(gid, b)
	g.setFuel(gid, 0)
	if _, err := g.SpawnStack(item.Wood, 90, 2, 5); err != nil {
		t.Fatal(err)
	}
	g.Step(300)
	if g.fuel[gid] != 0 {
		t.Fatal("haulers leave an auto-off burner alone")
	}
	g.Enqueue(cmd)
	g.Step(1500)
	if g.fuel[gid] == 0 {
		t.Fatal("the order must refuel it")
	}
}

func TestForcedRefuelReasons(t *testing.T) {
	t.Parallel()
	g, pid, gid, _ := refuelRig(t)
	spec, _ := fuelSpec(construction.WoodGenerator)
	g.setFuel(gid, spec.CapTicks)
	if o := refuelOpt(t, g, pid); !o.Disabled || o.Reason != "already full" {
		t.Fatalf("full: %+v", o)
	}
	g.setFuel(gid, 0)
	if o := refuelOpt(t, g, pid); !o.Disabled || o.Reason != "no free wood" {
		t.Fatalf("no wood: %+v", o)
	}
	g.SpawnStack(item.Wood, 90, 2, 5)
	if o := refuelOpt(t, g, pid); o.Disabled {
		t.Fatalf("wood: %+v", o)
	}
	g.SetWork(pid, work.Haul, false)
	if o := refuelOpt(t, g, pid); !o.Disabled || o.Reason != ReasonNoHaul {
		t.Fatalf("haul off: %+v", o)
	}
	g.SetWork(pid, work.Haul, true)
	other, _ := g.SpawnNamedPawn(3, 2, "Bob")
	g.reserve.TryEntity(other, gid)
	if o := refuelOpt(t, g, pid); !o.Disabled || o.Reason != ReasonBusyTarget+"Bob" {
		t.Fatalf("reserved: %+v", o)
	}
}

func TestForcedRefuelUnreachable(t *testing.T) {
	t.Parallel()
	g, pid, _, _ := refuelRig(t)
	g.SpawnStack(item.Wood, 90, 2, 5)
	pos, _ := g.positions.Get(pid)
	for dx := -1; dx <= 1; dx++ { // wall the pawn in
		for dy := -1; dy <= 1; dy++ {
			if dx != 0 || dy != 0 {
				g.grid.SetWalkable(pos.X+dx, pos.Y+dy, false)
			}
		}
	}
	g.Step(1)
	if o := refuelOpt(t, g, pid); !o.Disabled || o.Reason != "cannot reach" {
		t.Fatalf("walled: %+v", o)
	}
}

func TestRefuelGateWordsMatchOrderReasons(t *testing.T) {
	t.Parallel()
	if industry.RefuseGone != ReasonGone || industry.RefuseNoHaul != ReasonNoHaul || industry.RefuseUnreachable != ReasonUnreachable {
		t.Fatal("industry's refusal words drifted from the order reasons")
	}
}

func TestForcedRefuelChemfuelAndTank(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	pid, _ := g.SpawnNamedPawn(2, 2, "Ada")
	cid := placeFuel(t, g, construction.ChemfuelGenerator, 8, 3)
	placeFuel(t, g, construction.FishTank, 10, 3)
	g.Step(1)
	g.SpawnStack(item.Wood, 90, 2, 5)
	if o := refuelOpt(t, g, pid); !o.Disabled || o.Reason != "no free chemfuel" {
		t.Fatalf("chemfuel: %+v (%d)", o, cid)
	}
	for _, o := range g.OrdersAt(pid, 10, 3) {
		if o.Cmd.Order == command.OrderRefuel {
			t.Fatalf("fish tank has a refuel row: %+v", o)
		}
	}
}

func TestForcedRefuelEmberOilFallback(t *testing.T) {
	t.Parallel()
	g, pid, gid, cmd := refuelRig(t)
	g.setFuel(gid, 0)
	g.SpawnStack(item.EmberOil, 10, 2, 5)
	if o := refuelOpt(t, g, pid); o.Disabled {
		t.Fatalf("ember oil: %+v", o)
	}
	g.Enqueue(cmd)
	g.Step(1500)
	if g.fuel[gid] == 0 {
		t.Fatal("ember oil should fuel it")
	}
}

func TestQueuedRefuelRunsAfterCurrentOrder(t *testing.T) {
	t.Parallel()
	g, pid, gid, cmd := refuelRig(t)
	g.setFuel(gid, 0)
	g.SpawnStack(item.Wood, 90, 2, 5)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(pid), X: 5, Y: 8})
	cmd.Queue = true
	g.Enqueue(cmd)
	g.Step(1)
	if p := pawn(t, g, pid); len(p.QueuedOrders) != 1 {
		t.Fatalf("queue = %+v", p.QueuedOrders)
	}
	g.Step(2500)
	if g.fuel[gid] == 0 {
		t.Fatal("the queued refuel should have run")
	}
}
