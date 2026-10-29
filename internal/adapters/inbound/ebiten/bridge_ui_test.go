package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
)

func TestBridgeArchitectToolInStructure(t *testing.T) {
	t.Parallel()
	tool, ok := toolByID("bridge")
	if !ok {
		t.Fatal("the Architect needs a Bridge tool")
	}
	if tool.Category != "structure" || tool.Mode != architect.PlaceFill || tool.Build != "bridge" || tool.Icon != "bridge" {
		t.Fatalf("bridge tool = %+v", tool)
	}
	if _, ok := architect.Buildable("bridge"); !ok {
		t.Error("the simulation catalog must offer a bridge")
	}
	var floorAt, bridgeAt = -1, -1
	for i, tl := range toolsIn("structure") {
		switch tl.ID {
		case "floor":
			floorAt = i
		case "bridge":
			bridgeAt = i
		}
	}
	if bridgeAt != floorAt+1 {
		t.Errorf("bridge sits at %d, want right after floor (%d)", bridgeAt, floorAt)
	}
}

func TestBridgeToolLockedBeforeResearch(t *testing.T) {
	t.Parallel()
	tool, _ := toolByID("bridge")
	locked := &stubSim{world: game.WorldView{Locked: map[string]string{"build:bridge": "Bridges"}}}
	a, _ := colonyApp(t, locked)
	if got := a.toolLock(tool); got != "Needs research: Bridges" {
		t.Errorf("locked tool reason = %q", got)
	}
	a2, _ := colonyApp(t, &stubSim{})
	if got := a2.toolLock(tool); got != "" {
		t.Errorf("researched tool reason = %q, want free", got)
	}
}

func TestBridgeInspectorTitleAndLine(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{Kind: "bridge", Stuff: "wood"}
	if got := inspector.BuildingTitle(b); got != "Wooden bridge" {
		t.Errorf("title = %q", got)
	}
	if got := overlay.BridgeLine(b); got != "Holds light things: no stone walls or floors" {
		t.Errorf("line = %q", got)
	}
	if got := overlay.BridgeLine(game.BuildingView{Kind: "wall"}); got != "" {
		t.Errorf("non-bridge line = %q", got)
	}
}

func TestBridgeSelectableWhenNothingStandsOnIt(t *testing.T) {
	t.Parallel()
	w := bridgeWorld(3, 3, 11, [2]int{1, 1})
	if b, ok := inspector.BuildingAt(w, 1, 1); !ok || b.Kind != "bridge" {
		t.Fatalf("bridge alone: %+v %v", b, ok)
	}
	w.Buildings = append(w.Buildings, game.BuildingView{ID: 7, Kind: "wall", X: 1, Y: 1, W: 1, H: 1})
	if b, _ := inspector.BuildingAt(w, 1, 1); b.Kind != "wall" {
		t.Errorf("a wall on the bridge must win, got %q", b.Kind)
	}
}
