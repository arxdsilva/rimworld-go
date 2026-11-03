package game

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// AX-8: the escape ship end to end in the game.

// shipColony is a researched colony with the wiki's minimum ship built
// (beam, casket, core, reactor, sensor, three engines) and one colonist.
func shipColony(t *testing.T) (g *Game, ada, casket, reactor entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 3, Width: 30, Height: 20})
	g.CompleteResearch()
	ada, _ = g.SpawnNamedPawn(4, 16, "Ada")
	for _, p := range []struct {
		kind string
		x, y int
	}{
		{"ship structural beam", 10, 2}, {"ship reactor", 12, 2}, {"ship computer core", 8, 2},
		{"ship sensor cluster", 8, 4}, {"ship stasis casket", 9, 6},
		{"ship engine", 10, 9}, {"ship engine", 13, 9}, {"ship engine", 16, 9},
	} {
		if !g.BuildNow(p.kind, "steel", "south", p.x, p.y) {
			t.Fatalf("could not build the %s at (%d, %d)", p.kind, p.x, p.y)
		}
	}
	casket = g.layerBldgAt(9, 6, construction.LayerObject)
	reactor = g.layerBldgAt(12, 2, construction.LayerObject)
	return g, ada, casket, reactor
}

func shipBuilding(g *Game, id entity.EntityID) BuildingView {
	for _, b := range g.World().Buildings {
		if b.ID == id {
			return b
		}
	}
	return BuildingView{}
}

// The ship parts are buildable, join into one ship and report what they
// lack; the reactor powers the grid at 1000 W.
func TestShipPartsReportAndPower(t *testing.T) {
	t.Parallel()
	g, _, casket, reactor := shipColony(t)
	w := g.World()
	if w.AX.ShipID == 0 || w.AX.Countdown != -1 || len(w.AX.LaunchIssues) != 2 {
		t.Fatalf("world ship view: %+v", w.AX)
	}
	if b := shipBuilding(g, reactor); b.AX.Countdown != -1 || b.AX.ShipID != w.AX.ShipID {
		t.Fatalf("reactor view %+v", b.AX)
	}
	if b := shipBuilding(g, casket); b.AX.ShipID != w.AX.ShipID {
		t.Fatalf("casket view %+v", b.AX)
	}
	if g.genOutput(reactor) != 1000 {
		t.Fatalf("reactor output %d W", g.genOutput(reactor))
	}
}

// A colonist boards the casket on a right-click order, the reactor wakes
// through its countdown under raids, and the launch ends the colony's
// story: the ship leaves with its sleeper, the save says so.
func TestShipBoardCountdownLaunch(t *testing.T) {
	t.Parallel()
	g, ada, casket, reactor := shipColony(t)
	var board *OrderOption
	for _, o := range g.OrdersAt(ada, 9, 6) {
		if o.Cmd.Kind == command.BoardShip {
			board = &o
		}
	}
	if board == nil || board.Cmd.Target != uint64(casket) {
		t.Fatalf("no board order on the casket: %+v", g.OrdersAt(ada, 9, 6))
	}
	g.Enqueue(board.Cmd)
	for i := 0; i < 3000 && shipOf(g).Sleeper(casket) == 0; i++ {
		g.Step(1)
	}
	if shipOf(g).Sleeper(casket) != ada || !g.isStashed(ada) {
		t.Fatal("Ada did not board the casket")
	}
	if b := shipBuilding(g, casket); b.AX.Sleeper != "Ada" {
		t.Fatalf("casket view %+v", b.AX)
	}
	g.Enqueue(command.Command{Kind: command.LaunchShip, Target: uint64(casket)})
	g.Step(1)
	if shipOf(g).Launched() || !strings.Contains(g.messages[len(g.messages)-1].Text, "The reactor has not been started") {
		t.Fatalf("an unready ship launched or said nothing: %q", g.messages[len(g.messages)-1].Text)
	}
	g.Enqueue(command.Command{Kind: command.StartReactor, Target: uint64(reactor)})
	g.Step(1)
	if b := shipBuilding(g, reactor); b.AX.Countdown <= 0 {
		t.Fatalf("reactor countdown %d", b.AX.Countdown)
	}
	raiders := len(g.raiderIDs)
	DevWakeShip(g, ship.FirstRaid)
	g.Step(250)
	if len(g.raiderIDs) <= raiders {
		t.Fatal("no countdown raid came")
	}
	// the countdown's raiders gone, the reactor awake
	for _, id := range slices.Clone(g.raiderIDs) {
		g.Injure(id, 10000)
	}
	DevWakeShip(g, ship.CountdownTicks)
	g.Step(250)
	if w := g.World(); w.AX.Countdown != 0 || len(w.AX.LaunchIssues) != 0 {
		t.Fatalf("an awake ship still reports %v (countdown %d)", w.AX.LaunchIssues, w.AX.Countdown)
	}
	g.Enqueue(command.Command{Kind: command.LaunchShip, Target: uint64(reactor)})
	g.Step(1)
	w := g.World()
	if !w.AX.ShipLaunched || w.AX.Ending != ship.EndingShip || !slices.Equal(w.AX.Escaped, []string{"Ada"}) {
		t.Fatalf("world after the launch: %+v", w.AX)
	}
	if _, ok := g.bldgs.Get(reactor); ok {
		t.Fatal("the reactor is still on the map")
	}
	if info := g.Info("c"); info.Ending != ship.EndingShip {
		t.Fatalf("save header ending %q", info.Ending)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "c"); err != nil {
		t.Fatal(err)
	}
	h, info, err := Load(&buf)
	if err != nil || info.Ending != ship.EndingShip || !h.World().AX.ShipLaunched {
		t.Fatalf("loaded: %v ending %q", err, info.Ending)
	}
}

// launchedColony boards Ada, wakes the reactor and launches.
func launchedColony(t *testing.T, g *Game, ada, casket, reactor entity.EntityID) {
	t.Helper()
	g.Enqueue(command.Command{Kind: command.BoardShip, Pawn: uint64(ada), Target: uint64(casket)})
	for i := 0; i < 3000 && shipOf(g).Sleeper(casket) == 0; i++ {
		g.Step(1)
	}
	if shipOf(g).Sleeper(casket) != ada {
		t.Fatal("Ada did not board")
	}
	g.Enqueue(command.Command{Kind: command.StartReactor, Target: uint64(reactor)})
	g.Step(1)
	DevWakeShip(g, ship.CountdownTicks)
	g.Step(250)
	for _, id := range slices.Clone(g.raiderIDs) {
		g.Injure(id, 10000)
	}
	g.Step(1)
	if w := g.World(); len(w.AX.LaunchIssues) != 0 {
		t.Fatalf("not ready: %v", w.AX.LaunchIssues)
	}
	g.Enqueue(command.Command{Kind: command.LaunchShip, Target: uint64(reactor)})
	g.Step(1)
	if !g.World().AX.ShipLaunched {
		t.Fatal("no launch")
	}
}

// TestLaunchWithEveryoneAboardIsEveryoneGone (MAP-22; wiki: Ending: "dead,
// kidnapped, or have left the planet"): the passengers count as gone for good,
// and the save agrees.
func TestLaunchWithEveryoneAboardIsEveryoneGone(t *testing.T) {
	t.Parallel()
	g, ada, casket, reactor := shipColony(t)
	launchedColony(t, g, ada, casket, reactor)
	if p := g.personOf(ada); p == nil || p.Where != whereEscaped {
		t.Fatalf("the passenger's place: %+v", p)
	}
	if !everyoneGone(g) || !g.World().AX.EveryoneGone {
		t.Fatal("nobody left, but it doesn't say so")
	}
	if h := saveLoad(t, g); !everyoneGone(h) {
		t.Fatal("a save brought someone back")
	}
}

// TestLaunchLeavingSomeoneBehindIsNot: whoever stays keeps the colony going, and
// so does someone away with a caravan's worth of stashing.
func TestLaunchLeavingSomeoneBehindIsNot(t *testing.T) {
	t.Parallel()
	g, ada, casket, reactor := shipColony(t)
	bo, err := g.SpawnNamedPawn(4, 14, "Bo")
	if err != nil {
		t.Fatal(err)
	}
	launchedColony(t, g, ada, casket, reactor)
	if everyoneGone(g) {
		t.Fatal("Bo stayed")
	}
	g.die(bo, 0)
	if !everyoneGone(g) {
		t.Fatal("Bo died, and everyone's gone now")
	}
}

// A sleeper climbs back out on the casket's wake order and walks again.
func TestShipSleeperWakes(t *testing.T) {
	t.Parallel()
	g, ada, casket, _ := shipColony(t)
	g.Enqueue(command.Command{Kind: command.BoardShip, Pawn: uint64(ada), Target: uint64(casket)})
	for i := 0; i < 3000 && !g.isStashed(ada); i++ {
		if i%500 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
	}
	g.Enqueue(command.Command{Kind: command.LeaveShip, Target: uint64(casket)})
	g.Step(1)
	if g.isStashed(ada) || shipOf(g).Sleeper(casket) != 0 || !slices.Contains(g.pawnIDs, ada) {
		t.Fatal("Ada did not wake")
	}
}
