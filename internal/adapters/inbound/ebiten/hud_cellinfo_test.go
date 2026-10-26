package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// cellInfoSim is a colony with one pawn and a plant, so a selection and a
// plant line can both be had.
func cellInfoSim() *globalSim {
	s := newGlobalSim()
	s.pawns = []game.PawnView{{ID: 1, Name: "Ada", X: 2, Y: 2, Status: "alive"}}
	s.world.Plants = []game.PlantView{{ID: 5, Species: "corn", X: 4, Y: 3, Growth: 62}}
	return s
}

func hasLine(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

// With nothing selected the cell under the cursor describes itself.
func TestCellInfoDescribesHoveredCell(t *testing.T) {
	t.Parallel()
	a, f := globalColonyP(t, cellInfoSim())
	f.moveTo(a, 4, 3)
	f.frame(t, a)
	lines := a.cellInfoLines()
	if len(lines) < 3 {
		t.Fatalf("cell info = %q, want the ground's lines", lines)
	}
	if !hasLine(lines, "Fertility") || !hasLine(lines, "Corn, 62% grown") {
		t.Errorf("cell info = %q, want fertility and the plant", lines)
	}
	if !hasLine(lines, "Temperature 14C") { // TemperatureAt(4,3) = 140, outdoors
		t.Errorf("cell info = %q, want the sim's temperature for this cell", lines)
	}
	f.moveTo(a, 9, 9) // inside the room
	f.frame(t, a)
	if lines = a.cellInfoLines(); !hasLine(lines, "(indoors)") || !hasLine(lines, "Temperature 19C") {
		t.Errorf("room cell info = %q, want the indoor temperature of (9,9)", lines)
	}
}

// Cell info shows only with nothing selected and no Architect or window
// open, and never over the HUD.
func TestCellInfoOnlyWithNothingSelected(t *testing.T) {
	t.Parallel()
	a, f := globalColonyP(t, cellInfoSim())
	f.moveTo(a, 4, 3)
	f.frame(t, a)
	if len(a.cellInfoLines()) == 0 {
		t.Fatal("nothing selected: expected cell info")
	}

	a.renderer.Select(entity.EntityID(1))
	if got := a.cellInfoLines(); len(got) != 0 {
		t.Errorf("a pawn is selected, cell info = %q", got)
	}
	a.renderer.ClearSelect()
	if len(a.cellInfoLines()) == 0 {
		t.Fatal("selection cleared: expected cell info back")
	}

	// PLAY-020: a selected zone has its pane there too
	a.zoneSel, a.hasZone = 1, true
	if got := a.cellInfoLines(); len(got) != 0 {
		t.Errorf("a zone is selected, cell info = %q", got)
	}
	if a.learnSelection() == "" {
		t.Error("a selected zone counts as a selection for the learning helper")
	}
	a.clearZone()

	a.archOpen = true
	if got := a.cellInfoLines(); len(got) != 0 {
		t.Errorf("the Architect is open, cell info = %q", got)
	}
	a.archOpen = false

	a.workOpen, a.panelMode = true, "work"
	if got := a.cellInfoLines(); len(got) != 0 {
		t.Errorf("a window is open, cell info = %q", got)
	}
	a.workOpen = false

	f.onRect(a, a.hud().Global) // the cursor over the HUD
	f.frame(t, a)
	if got := a.cellInfoLines(); len(got) != 0 {
		t.Errorf("cursor over the HUD, cell info = %q", got)
	}
}

// Cell info is plain text bottom-up from (8, Tabs.Y - 8) on a translucent
// backing, and it is not chrome: a click on it reaches the map.
func TestCellInfoIsNotChrome(t *testing.T) {
	t.Parallel()
	a, f := globalColonyP(t, cellInfoSim())
	f.moveTo(a, 4, 3)
	f.frame(t, a)
	r := a.cellInfoRect()
	if r.empty() {
		t.Fatal("no cell info rect while hovering a cell")
	}
	l := a.hud()
	if r.X != hudMargin || r.bottom() != l.Tabs.Y-hudMargin {
		t.Errorf("cell info %+v, want its bottom-left corner at (8, %v)", r, l.Tabs.Y-hudMargin)
	}
	cx, cy := r.center()
	if a.colonyChromeHit(int(cx), int(cy), a.sw, a.sh) {
		t.Errorf("cell info's centre (%v,%v) blocks the map: it must not be chrome", cx, cy)
	}
	for _, c := range a.chromeRects(a.sw, a.sh) {
		if c.contains(cx, cy) {
			t.Errorf("chrome rect %+v covers the cell info", c)
		}
	}
	screen := ebiten.NewImage(a.sw, a.sh)
	a.drawCellInfo(screen, a.sh) // paints with a cell hovered
}

// Alt adds the detailed cell lines.
func TestCellInfoAltAddsDetail(t *testing.T) {
	t.Parallel()
	a, f := globalColonyP(t, cellInfoSim())
	f.moveTo(a, 4, 3)
	f.frame(t, a)
	if hasLine(a.cellInfoLines(), "Cell (4, 3)") {
		t.Fatal("detail lines without Alt")
	}
	f.cur.mods = input.ModAlt
	f.frame(t, a)
	if !hasLine(a.cellInfoLines(), "Cell (4, 3)") {
		t.Fatalf("Alt held: cell info = %q, want the detail lines", a.cellInfoLines())
	}
}

// The fogged cell reads "Unexplored" and nothing else.
func TestCellInfoUnexplored(t *testing.T) {
	t.Parallel()
	s := cellInfoSim()
	s.world.Fog = make([]uint8, 16*16)
	s.world.Fog[3*16+4] = 1
	a, f := globalColonyP(t, s)
	f.moveTo(a, 4, 3)
	f.frame(t, a)
	if got := a.cellInfoLines(); len(got) != 1 || got[0] != "Unexplored" {
		t.Fatalf("fogged cell info = %q", got)
	}
}

// The minimap is HUD for hovering too: the cell under it is not described.
func TestCellInfoHidesOverTheMinimap(t *testing.T) {
	t.Parallel()
	big := &globalSim{stubSim: &stubSim{w: 200, h: 200}, world: game.WorldView{TerrainW: 200, Terrain: make([]uint8, 200*200), OutdoorTemp: 70}}
	a, f := globalColonyP(t, big)
	m := a.hud().Minimap
	if m.empty() {
		t.Fatal("a 200x200 map has no minimap rect")
	}
	f.onRect(a, m)
	f.frame(t, a)
	if got := a.cellInfoLines(); len(got) != 0 {
		t.Errorf("cursor over the minimap, cell info = %q", got)
	}
	if got, want := a.temperatureLine(), "Outdoors 7 °C"; got != want {
		t.Errorf("over the minimap: %q, want %q", got, want)
	}
}

// A disabled-button hint (or a toast) never lands on the cell info: with
// nothing selected both start at the lower-left, so the hint sits above
// the info box.
func TestHintSitsAboveCellInfo(t *testing.T) {
	t.Parallel()
	a, f := globalColonyP(t, cellInfoSim())
	f.moveTo(a, 4, 3)
	f.frame(t, a)
	lines := a.cellInfoLines()
	if len(lines) == 0 {
		t.Fatal("no cell info to test against")
	}
	box := cellInfoBox(lines, a.hud().Tabs.Y)
	if _, bottom := a.hintAnchor(); bottom > box.Y {
		t.Fatalf("hint rests at y %v, inside the cell info box from y %v", bottom, box.Y)
	}
}
