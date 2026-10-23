package scenario

import (
	"testing"
	"time"
)

func TestBuildCounts(t *testing.T) {
	g, n, err := Build(1, 120, 120)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("counts %+v", n)
	if n.Colonists != 20 || n.Tame != 50 || n.Stacks != 1000 {
		t.Fatalf("counts %+v", n)
	}
	if n.Buildings < 480 || n.Buildings > 540 { // ~430 in rooms, ~80 fences
		t.Fatalf("buildings = %d, want about 510", n.Buildings)
	}
	g.Step(1) // apply the zone and designation commands
	w := g.World()
	if len(w.Mines) != 200 {
		t.Errorf("mines = %d, want 200", len(w.Mines))
	}
	if len(w.Frames) != 50 {
		t.Errorf("blueprints = %d, want 50", len(w.Frames))
	}
	grow := 0
	for _, z := range w.Zones {
		t.Logf("zone %s cells %d", z.Kind, len(z.Cells))
		if z.Kind == "grow" {
			grow += len(z.Cells)
		}
	}
	if grow != 600 {
		t.Errorf("grow cells = %d, want 600", grow)
	}
}

func TestBuildTooSmall(t *testing.T) {
	if _, _, err := Build(1, 48, 40); err == nil {
		t.Fatal("want an error for a small map")
	}
}

func TestGrownColonyProbe(t *testing.T) {
	for _, n := range []int{250, 500} {
		t0 := time.Now()
		g, _, err := Build(1, n, n)
		if err != nil {
			t.Fatal(err)
		}
		t1 := time.Now()
		g.Step(500)
		t.Logf("%d: build %v, 500 ticks: %v", n, t1.Sub(t0), time.Since(t1))
	}
}

// TestGrownGolden pins the state of the grown colony after Build and
// 5 000 ticks (NG-1 §6.2): it exercises the scale paths (hundreds of
// buildings, stacks, zones, designations, a pen) that the start colony
// of TestScaleGolden never reaches.
func TestGrownGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("long golden run")
	}
	// Re-pinned: CompleteResearch now also completes the NG-1 research
	// projects (more unlocks), and StateHash became format-independent;
	// identical with and without the E1 engine changes.
	const want uint64 = 0xefdfa4b86fff8e25
	g, _, err := Build(7, 120, 120)
	if err != nil {
		t.Fatal(err)
	}
	g.Step(5000)
	if got := g.StateHash(); got != want {
		t.Fatalf("state hash = %#x, want %#x", got, want)
	}
}
