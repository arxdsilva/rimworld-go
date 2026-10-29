package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-023: the zone ghost names why a cell is refused, a refused drag
// hints the most common reason, the dumping stockpile tool and the
// storage priority picker.

func TestZoneRefusalTooltip(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if got := a.arch.Zones.RefusalAt(game.ZoneStockpile, 0, 5); got != game.ZoneRefuseEdge {
		t.Fatalf("edge cell refusal = %q", got)
	}
	if got := a.arch.Zones.RefusalAt(game.ZoneStockpile, 5, 5); got != "" {
		t.Fatalf("a free cell is refused: %q", got)
	}
	if architect.ZoneTipText(game.ZoneRefuseEdge) != "Can't zone: Too close to the map edge" || architect.ZoneTipText("") != "" ||
		architect.ZoneTipText(architect.ZoneNoReason) != "Can't zone here" {
		t.Fatal("tip text")
	}
	// the most common refusal over a drag, ties to the first met
	why, free := architect.ZoneDragRefusal(0, 0, 2, 1, func(x, y int) string {
		if x == 0 {
			return "Rock"
		}
		return "Water"
	})
	if free || why != "Water" {
		t.Fatalf("most common = %q free=%v, want Water", why, free)
	}
	if why, _ := architect.ZoneDragRefusal(1, 0, 0, 0, func(x, y int) string { return []string{"A", "B"}[x] }); why != "A" {
		t.Fatalf("tie goes to the first reason met, got %q", why)
	}
	if _, free := architect.ZoneDragRefusal(0, 0, 3, 3, func(x, y int) string {
		if x == 3 && y == 3 {
			return ""
		}
		return "Rock"
	}); !free {
		t.Fatal("one free cell makes the drag fine")
	}
	if architect.ZoneDragHint(architect.ZoneNoReason) != "No free tiles there" || architect.ZoneDragHint("Rock") != "Can't zone here: Rock" {
		t.Fatal("drag hint text")
	}
	// a drag along the map edge hints the edge
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.stockpile")
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 0, 3, 0, 8
	a.arch.Submit()
	if a.hint != "Can't zone here: "+game.ZoneRefuseEdge {
		t.Fatalf("hint = %q", a.hint)
	}
	_ = g
}

// A stub without ZoneRefusal falls back to CanZoneAt with no reason.
func TestZoneRefusalFallsBackWithoutReasons(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t)
	s.deny = func(x, y int) bool { return x == 2 }
	if a.arch.Zones.RefusalAt(game.ZoneStockpile, 2, 2) != architect.ZoneNoReason || a.arch.Zones.RefusalAt(game.ZoneStockpile, 3, 2) != "" {
		t.Fatal("fallback refusal")
	}
}

func TestDumpingStockpileTool(t *testing.T) {
	t.Parallel()
	tool, ok := toolByID("dump")
	if !ok || tool.Category != "zones" || zoneKindOf(tool) != game.ZoneDump || tool.Label != "Dumping stockpile" || tool.Desc == "" {
		t.Fatalf("dump tool %+v", tool)
	}
	sp, _ := toolByID("stockpile")
	if zoneKindOf(sp) != game.ZoneStockpile {
		t.Fatal("the plain stockpile tool still paints stockpiles")
	}
	a, g, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.dump")
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 6, 6, 8, 7
	a.arch.Submit()
	g.Step(1)
	found := false
	for _, z := range g.World().Zones {
		if z.Kind == game.ZoneDump && len(z.Cells) == 6 && z.Corpses && z.Chunks {
			found = true
		}
	}
	if !found {
		t.Fatalf("no dumping stockpile painted: %+v", g.World().Zones)
	}
}

func TestZonePriorityPickSendsCommand(t *testing.T) {
	t.Parallel()
	st := rectZone(2, game.ZoneStockpile, 3, 0, 4, 1)
	st.Priority = "Normal"
	a, s, _ := zoneApp(t, st)
	var pc inspector.Button
	for _, c := range zoneCmdsOf(a, 2) {
		if c.ID == "cmd.zone.priority" {
			pc = c
		}
	}
	if pc.Label != "Priority: Normal" || pc.Run == nil {
		t.Fatalf("priority button %+v", pc)
	}
	names := game.StoragePriorities()
	a.pickZonePriority(2, "Normal", names, 3) // Normal again: nothing
	if len(cmdsOfZone(s, command.SetZonePriority)) != 0 {
		t.Fatal("picking the current priority sends nothing")
	}
	a.pickZonePriority(2, "Normal", names, 0)
	if c := lastZoneCmd(t, s); c.Kind != command.SetZonePriority || c.Target != 2 || c.Index != 0 {
		t.Fatalf("command %+v", c)
	}
}

func cmdsOfZone(s *zoneSim, k command.Kind) []command.Command {
	var out []command.Command
	for _, c := range s.cmds {
		if c.Kind == k {
			out = append(out, c)
		}
	}
	return out
}
