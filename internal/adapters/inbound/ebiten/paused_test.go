package ebiten

import (
	"fmt"
	"rimworld-go/internal/application/game"
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

// Actions taken while paused apply at once: the clock does not move, but
// the zone, blueprint, priority or draft shows immediately.

func pausedColony(t *testing.T) (*App, func() uint64) {
	t.Helper()
	a, g, _ := liveColony(t, 0)
	a.renderer.speed = speedPaused
	return a, func() uint64 { return uint64(g.Tick()) }
}

func TestPausedStockpileShowsAtOnce(t *testing.T) {
	t.Parallel()
	a, tick := pausedColony(t)
	before := tick()
	a.renderer.enqueue(command.Command{Kind: command.DesignateStockpile, X: 5, Y: 5, X2: 7, Y2: 6})
	if countZones(a.renderer.world(), game.ZoneStockpile) != 1 {
		t.Fatal("a stockpile painted while paused must show at once")
	}
	if tick() != before {
		t.Fatal("applying a paused action must not advance time")
	}
}

func TestPausedBlueprintAppears(t *testing.T) {
	t.Parallel()
	a, _ := pausedColony(t)
	a.renderer.enqueue(command.Command{Kind: command.DesignateWall, X: 9, Y: 9})
	if len(a.renderer.world().Frames) != 1 {
		t.Fatal("a wall blueprint placed while paused must show at once")
	}
}

func TestPausedWorkPriorityCycles(t *testing.T) {
	t.Parallel()
	a, _ := pausedColony(t)
	p := a.renderer.snapshot()[0]
	click(t, a, "tab.work")
	cell := fmt.Sprintf("work.cell.%d.6", p.ID) // Haul
	var seen []int
	for i := 0; i < 3; i++ {
		click(t, a, cell)
		seen = append(seen, workPriority(a.renderer.snapshot()[0], 6))
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] == seen[i-1] {
			t.Fatalf("each paused click must change the shown priority: %v", seen)
		}
	}
}

func TestPausedDraftToggles(t *testing.T) {
	t.Parallel()
	a, _ := pausedColony(t)
	id := a.renderer.snapshot()[0].ID
	a.renderer.Select(id)
	a.toggleDraft()
	if !a.renderer.snapshot()[0].Drafted {
		t.Fatal("drafting while paused must show at once")
	}
	a.toggleDraft()
	if a.renderer.snapshot()[0].Drafted {
		t.Fatal("a second press while paused must undraft")
	}
}

func TestRunningStillQueuesForStep(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	a.renderer.speed = speedNormal
	a.renderer.enqueue(command.Command{Kind: command.DesignateStockpile, X: 5, Y: 5, X2: 7, Y2: 6})
	if countZones(a.renderer.world(), game.ZoneStockpile) != 0 {
		t.Fatal("while running, commands still wait for the next Step")
	}
	g.Step(1)
	if countZones(a.renderer.world(), game.ZoneStockpile) != 1 {
		t.Fatal("the next Step applies it")
	}
}
