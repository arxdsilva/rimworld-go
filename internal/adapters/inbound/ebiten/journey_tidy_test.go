package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/command"
)

// CSE-019: with its rule on, a bench's pane has the switch; the click sends
// SetBenchClean and the read model follows.
func TestJourneyBenchClean(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("workbench", "wood", "south", 9, 3) {
		t.Fatal("no bench")
	}
	bench := func() (id uint64, off bool) {
		for _, b := range j.world().Buildings {
			if b.Bench {
				return uint64(b.ID), b.CleanOff
			}
		}
		t.Fatal("no bench in the world")
		return 0, false
	}
	j.clickTile(9, 3, inputframe.Left, 0)
	j.update(2)
	if _, ok := j.a.uiSt().Find(inspector.BenchCleanID); ok {
		t.Fatal("the switch shows with its rule off")
	}

	j.g.Enqueue(command.Command{Kind: command.SetGameplayToggle, Index: int(gameplay.BenchCleanToggle), Enabled: true})
	j.g.Step(1)
	j.update(2)
	if !j.world().Gameplay.On(gameplay.BenchCleanToggle) {
		t.Fatal("the rule didn't turn on")
	}
	if !j.clickID(inspector.BenchCleanID) {
		t.Fatal("no switch")
	}
	id, _ := bench()
	c, ok := j.lastCmd(command.SetBenchClean)
	if !ok || c.Target != id || c.Enabled {
		t.Fatalf("SetBenchClean %+v ok=%v, want off for %d", c, ok, id)
	}
	j.g.Step(1)
	j.update(2)
	if _, off := bench(); !off {
		t.Fatal("the read model didn't follow")
	}

	if !j.clickID(inspector.BenchCleanID) {
		t.Fatal("no switch after the click")
	}
	if c, ok = j.lastCmd(command.SetBenchClean); !ok || !c.Enabled {
		t.Fatalf("SetBenchClean %+v ok=%v, want on", c, ok)
	}
	j.g.Step(1)
	j.update(2)
	if _, off := bench(); off {
		t.Fatal("the read model didn't come back")
	}
}
