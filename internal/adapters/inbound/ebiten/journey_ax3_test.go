package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
)

// TestJourneyAx3StoneTilePicker (AX-3): the Stone tile card's material
// row picks the stone; the drag lays marble tiles, and the blueprint is
// named for its stone.
func TestJourneyAx3StoneTilePicker(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch(architect.CatFloors, "stone_tile")
	if !j.clickID("arch.stuff.marble blocks") {
		t.Fatal("the marble button is disabled")
	}
	j.dragTiles(inputframe.Left, 8, 2, 9, 3, 0)
	c, ok := j.lastCmd(command.DesignateBuild)
	marble, _ := game.StuffID("marble blocks")
	if n := len(j.cmdsOf(command.DesignateBuild)); !ok || n != 4 || c.Build != uint8(construction.StoneTile) || c.Stuff != marble {
		t.Fatalf("%d DesignateBuild, last %+v: want 4 marble stone tiles", n, c)
	}
	j.tick(1)
	for _, f := range j.world().Frames {
		if f.Kind == "stone tile" && inspector.FrameName(f) != "Marble tile" {
			t.Fatalf("blueprint named %q", inspector.FrameName(f))
		}
	}
}

// TestJourneyAx3CarpetColourPicker (AX-3): the Carpet floor card shows the
// colour swatches; clicking one names it in the box, the drag carries it
// in DesignateBuild's Index, and the blueprint keeps it; a built floor's
// pane says what a cell gives its room.
func TestJourneyAx3CarpetColourPicker(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch(architect.CatFloors, "carpet_floor")
	if !j.clickID("arch.colour.mint") {
		t.Fatal("the mint swatch is disabled")
	}
	named := false
	for _, ln := range j.a.arch.Plan().Lines {
		named = named || ln.Text == "Colour: Mint"
	}
	if !named {
		t.Fatal("the box does not name the picked colour")
	}
	j.dragTiles(inputframe.Left, 8, 2, 8, 2, 0)
	c, ok := j.lastCmd(command.DesignateBuild)
	mint, _ := construction.ParseColour("mint")
	if !ok || c.Build != uint8(construction.CarpetFloor) || c.Index != int(mint) {
		t.Fatalf("DesignateBuild %+v, want a carpet floor in mint (%d)", c, mint)
	}
	j.tick(1)
	var fv game.FrameView
	for _, f := range j.world().Frames {
		if f.Kind == "carpet floor" {
			fv = f
		}
	}
	if fv.AX.Colour != "mint" || inspector.FrameName(fv) != "Mint carpet floor" {
		t.Fatalf("carpet floor blueprint %+v named %q", fv.AX, inspector.FrameName(fv))
	}
	if !j.g.BuildNow("steel tile", "steel", "south", 9, 2) {
		t.Fatal("no steel tile built")
	}
	for _, b := range j.world().Buildings {
		if b.Kind == "steel tile" {
			if lines := strings.Join(inspector.FloorLines(b), " | "); lines != "Beauty 0   Cleanliness +0.2" || inspector.BuildingTitle(b) != "Steel tile" {
				t.Fatalf("steel tile pane %q titled %q", lines, inspector.BuildingTitle(b))
			}
		}
	}
}

// TestJourneyAx3StrawMattingSendsHay (AX-3): a single-material floor's
// drag names its material (hay), not the zero value.
func TestJourneyAx3StrawMattingSendsHay(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch(architect.CatFloors, "straw_matting")
	j.dragTiles(inputframe.Left, 8, 2, 8, 2, 0)
	c, ok := j.lastCmd(command.DesignateBuild)
	hay, found := game.StuffID("hay")
	if !ok || !found || hay == 0 || c.Stuff != hay || c.Build != uint8(construction.StrawMatting) {
		t.Fatalf("DesignateBuild %+v, want straw matting of hay (%d)", c, hay)
	}
}
