package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// TestJourneyBench3StoveRotatedCoversThreeCells (STK-BN): a stove armed
// from the Architect and turned east with E is a 1x3 ghost whose work
// spot is in front of its middle cell: a wall on its third cell or on that
// spot turns the ghost red; on clear ground it is placed, built by a
// colonist, and the built stove covers the same three cells with the spot
// east of the middle one.
func TestJourneyBench3StoveRotatedCoversThreeCells(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.g.ClearPlants(0, 0, 23, 23)
	for _, w := range [][2]int{{14, 6}, {11, 13}} { // the third cell of a ghost at (14, 4); the spot of one at (10, 12)
		if !j.g.BuildNow("wall", "wood", "south", w[0], w[1]) {
			t.Fatalf("no wall at %v", w)
		}
	}
	if _, err := j.g.SpawnStack(item.Wood, 30, 6, 6); err != nil {
		t.Fatal(err)
	}
	j.lookAt(12, 8)
	j.openArch("production", "stove")
	j.key("E", 0)
	ghost := func(x, y int) (ok bool, reason string) {
		t.Helper()
		j.f.MoveTo(j.a, x, y)
		j.update(1)
		_, cells := j.a.previewCells()
		if len(cells) != 1 || cells[0].X != x || cells[0].Y != y || cells[0].Facing != "" && cells[0].Facing != "east" {
			t.Fatalf("ghost at (%d, %d): %+v", x, y, cells)
		}
		return cells[0].OK, cells[0].Reason
	}
	if ok, why := ghost(14, 4); ok {
		t.Fatal("a wall on the east stove's third cell: the ghost is placeable")
	} else if why == game.ReasonSpot {
		t.Fatalf("third cell blocked, reason %q", why)
	}
	if ok, why := ghost(10, 12); ok || why != game.ReasonSpot {
		t.Fatalf("a wall in front of the middle cell: ok %v reason %q, want %q", ok, why, game.ReasonSpot)
	}
	if ok, why := ghost(10, 8); !ok {
		t.Fatalf("clear ground: the ghost is red (%q)", why)
	}
	j.clickTile(10, 8, inputframe.Left, 0)
	c, ok := j.lastCmd(command.DesignateBuild)
	if !ok || c.X != 10 || c.Y != 8 || c.Facing != 1 {
		t.Fatalf("DesignateBuild %+v ok=%v, want an east stove at (10, 8)", c, ok)
	}
	fv := j.world().Frames
	if len(fv) != 1 || fv[0].W != 1 || fv[0].H != 3 {
		t.Fatalf("frames %+v, want one 1x3 blueprint", fv)
	}
	j.leaveArch()
	j.key("Digit3", 0)
	var st game.BuildingView
	built := func() bool {
		for _, b := range j.world().Buildings {
			if b.Kind == "stove" {
				st = b
				return true
			}
		}
		return false
	}
	for i := 0; i < 12000 && !built(); i++ {
		careFeed(j)
		j.update(1)
	}
	j.key("Space", 0)
	if !built() {
		t.Fatalf("the stove was not built: %+v", j.world().Frames)
	}
	if st.X != 10 || st.Y != 8 || st.Facing != "east" || st.W != 1 || st.H != 3 || !st.HasSpot || st.SpotX != 11 || st.SpotY != 9 {
		t.Fatalf("built stove %+v, want 1x3 at (10, 8) facing east, spot (11, 9)", st)
	}
	for y := 8; y <= 10; y++ {
		if ok, _ := j.g.CanPlaceAt("wall", "wood", "south", 10, y); ok {
			t.Fatalf("stove cell (10, %d) is free", y)
		}
	}
}

// TestBench3ArtFootprints (STK-BN): every resized bench's world sprite
// spans its wiki footprint in each facing (the renderer anchors it from
// the metadata footprint and origin).
func TestBench3ArtFootprints(t *testing.T) {
	t.Parallel()
	art := realArt(t)
	for _, k := range []construction.Kind{construction.Workbench, construction.Stove, construction.ButcherTable,
		construction.TailoringBench, construction.StonecutterTable, construction.BrewingBench, construction.DrugTable,
		construction.ElectricStove, construction.ElectricTailoringBench, construction.FueledSmithy, construction.ElectricSmithy,
		construction.MachiningTable, construction.ArtBench, construction.ElectricSmelter, construction.FabricationBench,
		construction.BiofuelRefinery, construction.RadioConsole} {
		d := construction.DefOf(k)
		for f := construction.South; f <= construction.West; f++ {
			key := world.BuildSprite(d.Name, f.String(), 0, false, false)
			want := d.Size
			if f == construction.East || f == construction.West {
				want = [2]int{d.Size[1], d.Size[0]}
			}
			if m := art.MetaOf(key); m == nil || m.Footprint != want {
				t.Errorf("%s %s: sprite %q footprint %+v, want %v", d.Name, f, key, m, want)
			}
		}
	}
}
