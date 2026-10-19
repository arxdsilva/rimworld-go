package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
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
