package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/roof"
)

func homeAt(g *Game, x, y int) bool { return g.inHome(y*g.grid.Width() + x) }

func TestHomeAutoDefaultOnAndToggle(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	if !g.World().HomeAuto {
		t.Fatal("Home auto-expansion starts on")
	}
	g.Enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: false})
	g.Step(1)
	if g.World().HomeAuto {
		t.Fatal("SetHomeAuto off")
	}
	placeBuilt(t, g, construction.Lamp, construction.South, 10, 6)
	if homeAt(g, 10, 6) {
		t.Fatal("a building expanded Home while auto was off")
	}
	g.Enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: true})
	g.Step(1)
	if !g.World().HomeAuto {
		t.Fatal("SetHomeAuto on")
	}
	placeBuilt(t, g, construction.Lamp, construction.South, 20, 6)
	if !homeAt(g, 20, 6) || !homeAt(g, 22, 8) {
		t.Fatal("a building expands Home when auto is on")
	}
}

func TestHomeAutoOffKeepsExistingHome(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.paintHome(2, 2, 5, 5, true)
	ver := g.World().HomeVer
	g.Enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: false})
	g.Step(1)
	if !homeAt(g, 3, 3) || g.World().HomeVer != ver {
		t.Fatal("turning auto off keeps Home")
	}
	// manual painting still works
	g.Enqueue(command.Command{Kind: command.DesignateHome, X: 8, Y: 8, X2: 9, Y2: 9, Enabled: true})
	g.Step(1)
	if !homeAt(g, 8, 8) {
		t.Fatal("manual Home paint works with auto off")
	}
}

func TestHomeAutoSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.Enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: false})
	g.Step(1)
	var buf bytes.Buffer
	if err := g.Save(&buf, "x"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if h.World().HomeAuto {
		t.Fatal("auto off is saved")
	}
	// legacy: NoHomeAuto zero value means on
	g2 := dirtGame(t)
	buf.Reset()
	if err := g2.Save(&buf, "x"); err != nil {
		t.Fatal(err)
	}
	h2, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !h2.World().HomeAuto {
		t.Fatal("old saves and defaults load with auto on")
	}
}

func TestHomeAutoZonesExpandHome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cmd  command.Command
	}{
		{"stockpile", command.Command{Kind: command.DesignateStockpile, X: 20, Y: 5, X2: 21, Y2: 6}},
		{"grow zone", command.Command{Kind: command.DesignateGrowZone, X: 20, Y: 5, X2: 21, Y2: 6}},
		{"pen", command.Command{Kind: command.DesignatePen, X: 20, Y: 5, X2: 21, Y2: 6}},
	} {
		g := dirtGame(t)
		g.Enqueue(tc.cmd)
		g.Step(1)
		if !homeAt(g, 20, 5) || !homeAt(g, 18, 3) || !homeAt(g, 23, 8) {
			t.Errorf("%s: Home not expanded with a 2-cell margin", tc.name)
		}
		if homeAt(g, 17, 5) || homeAt(g, 24, 5) {
			t.Errorf("%s: margin too wide", tc.name)
		}
		g2 := dirtGame(t)
		g2.Enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: false})
		g2.Enqueue(tc.cmd)
		g2.Step(1)
		if homeAt(g2, 20, 5) {
			t.Errorf("%s: expanded with auto off", tc.name)
		}
	}
}

func TestHomeAutoPenMarkerExpandsHome(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	placeBuilt(t, g, construction.PenMarker, construction.South, 20, 6)
	if !homeAt(g, 20, 6) || !homeAt(g, 22, 8) || !homeAt(g, 18, 4) {
		t.Fatal("a pen marker expands Home by 2 cells")
	}
}

func TestRepairOnlyInsideHome(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 40, Height: 10})
	if !g.BuildNow("wall", "wood", "south", 20, 5) { // inside the landing Home
		t.Fatal("wall in")
	}
	g.Enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: false})
	g.Step(1)
	g.paintHome(0, 0, 39, 9, false)
	if !g.BuildNow("wall", "wood", "south", 33, 5) { // outside Home, auto off
		t.Fatal("wall out")
	}
	var out, in = g.layerBldgAt(33, 5, construction.LayerObject), g.layerBldgAt(20, 5, construction.LayerObject)
	g.paintHome(18, 3, 22, 7, true)
	g.damageBuilding(out, 40)
	g.damageBuilding(in, 40)
	ada, _ := g.SpawnNamedPawn(25, 5, "Ada")
	for i := 0; i < 40; i++ {
		g.Step(100)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	if b, _ := g.bldgs.Get(in); b.Damage != 0 {
		t.Fatalf("building inside Home not repaired: %d", b.Damage)
	}
	if b, _ := g.bldgs.Get(out); b.Damage != 40 {
		t.Fatalf("building outside Home repaired: %d", b.Damage)
	}
	g.paintHome(30, 3, 36, 7, true)
	for i := 0; i < 40; i++ {
		g.Step(100)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	if b, _ := g.bldgs.Get(out); b.Damage != 0 {
		t.Fatalf("building brought into Home not repaired: %d", b.Damage)
	}
}

func TestRoofAreaPersistsAndRebuilds(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 10})
	hut(t, g, 1, 1, 12, 5)
	g.designateRoof(14, 3, 16, 3, true) // outside the hut: no auto roof
	if countRoof(g, roof.AreaBuild) != 3 {
		t.Fatalf("area cells %d", countRoof(g, roof.AreaBuild))
	}
	g.RoofRooms()
	if !g.roofAt(15, 3).Has(roof.Built) {
		t.Fatal("roof built")
	}
	knockDown(g, 1, 1, 12, 5, nil)
	g.ensureRooms()
	g.Step(climateInterval)
	if g.roofAt(15, 3).Has(roof.Built) {
		t.Fatal("roof should have collapsed")
	}
	if !g.roofAt(15, 3).Has(roof.AreaBuild) {
		t.Fatal("area survives a collapse")
	}
	if !g.BuildNow("wall", "wood", "south", 13, 3) {
		t.Fatal("support wall")
	}
	g.ensureRooms()
	for x := 14; x <= 16; x++ {
		if c := g.roofAt(x, 3); !c.Has(roof.PlanBuild) {
			t.Errorf("x=%d: roof not re-planned (%b)", x, c)
		}
	}
	g.designateRoof(14, 3, 16, 3, false)
	if countRoof(g, roof.AreaBuild) != 0 {
		t.Fatal("remove-roof clears the area")
	}
	if g.roofAt(15, 3).Has(roof.PlanBuild) {
		t.Fatal("pending plans from the area are dropped")
	}
}

func TestRoofAreaPlanReraisedAfterCollapse(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 10})
	// a lone tile far from any support, then a wall that supports it
	g.ensureRooms()
	g.designateRoof(20, 5, 20, 5, true)
	if g.roofAt(20, 5).Has(roof.PlanBuild) {
		t.Fatal("unsupported tile is not planned yet")
	}
	if !g.BuildNow("wall", "wood", "south", 24, 5) {
		t.Fatal("wall")
	}
	g.ensureRooms()
	if !g.roofAt(20, 5).Has(roof.PlanBuild) {
		t.Fatal("support arrived: the area plans its roof")
	}
	g.RoofRooms()
	if !g.roofAt(20, 5).Has(roof.Built) {
		t.Fatal("built")
	}
}

func TestCancelClearsRoofArea(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 10})
	hut(t, g, 1, 1, 12, 5)
	g.designateRoof(3, 3, 5, 3, true)
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 3, Y: 3, X2: 5, Y2: 3})
	g.Step(1)
	if countRoof(g, roof.AreaBuild) != 0 {
		t.Fatal("cancel clears the roof area")
	}
}

func TestLegacyRoofPlansUntouched(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 10})
	hut(t, g, 1, 1, 12, 5)
	if countRoof(g, roof.AreaBuild) != 0 {
		t.Fatal("auto roofing sets no area bit")
	}
}
