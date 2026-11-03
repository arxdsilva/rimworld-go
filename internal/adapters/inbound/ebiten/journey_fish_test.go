package ebiten

// journey_fish_test.go: the fish tank through the real UI loop (FISH): the
// Architect card, the stock picker, the pane lines and the dying alert.

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fishTankView is the tank building id's view.
func fishTankView(j *journey, id entity.EntityID) game.BuildingView {
	j.t.Helper()
	for _, b := range j.world().Buildings {
		if b.ID == id {
			return b
		}
	}
	j.t.Fatalf("no building %d", id)
	return game.BuildingView{}
}

// TestJourneyFishTankStock: with Aquaculture done and a tank built, the
// pane's Stock menu picks a species through the mouse; SetTankStock leaves,
// the read model shows the stock and the pane says so.
func TestJourneyFishTankStock(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	if !slices.ContainsFunc(architect.Tools, func(d architect.ToolDef) bool { return d.Build == "fish tank" && d.Icon == "fish_tank" }) {
		t.Fatal("the Architect has no fish tank card")
	}
	rsFinish(j, int(research.Aquaculture))
	wdBuild(j, "fish tank", 10, 5)
	id := wdSelect(j, 10, 5)
	j.update(2)
	tv := fishTankView(j, id).Tank
	if tv == nil || len(tv.Options) == 0 {
		t.Fatalf("the tank has no stock options: %+v", tv)
	}
	if tv.Stock != "" {
		t.Fatalf("a new tank holds %q", tv.Stock)
	}
	var pick game.TankOption
	for _, o := range tv.Options {
		if o.Reason == "" {
			pick = o
			break
		}
	}
	if pick.ID == 0 {
		t.Fatalf("every stock is refused: %+v", tv.Options)
	}

	cmds := j.a.insp.Commands().SelectionCommands()
	i := slices.IndexFunc(cmds, func(b inspector.Button) bool { return b.ID == inspector.TankStockBtnID })
	if i < 0 {
		t.Fatal("the pane has no Stock command")
	}
	j.reset()
	if !j.clickID("cmd.options." + strconv.Itoa(i)) {
		t.Fatal("no options arrow on the Stock command")
	}
	j.pick(strings.ToUpper(pick.Label[:1]) + pick.Label[1:])
	c, ok := j.lastCmd(command.SetTankStock)
	if !ok || c.Target != uint64(id) || c.Count != pick.ID {
		t.Fatalf("SetTankStock %+v ok %v, want tank %d stock %d", c, ok, id, pick.ID)
	}
	j.g.Step(2)
	j.update(2)
	if got := fishTankView(j, id).Tank; got.Stock != pick.Label {
		t.Fatalf("the read model holds %q, want %q", got.Stock, pick.Label)
	}
	spec := j.a.insp.Producers[selection.Building].Pane(id)
	lines := strings.Join(spec.Lines, "|")
	if !strings.Contains(lines, "Stock: "+pick.Label) {
		t.Fatalf("pane lines %q lack the stock", lines)
	}
	hasFeed := false
	for _, b := range spec.Bars {
		hasFeed = hasFeed || b.Label == "Feed"
	}
	if !hasFeed {
		t.Fatalf("no feed bar: %+v", spec.Bars)
	}
	j.draw(2) // the tank, its glow and its fish draw
}
