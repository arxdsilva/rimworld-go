package ebiten

// journey_ax7_test.go: the AX-7 journeys: the bookcase family card, a
// book's pane and its Read button, a colonist reading, and a bookcase's
// pane with its books and reading bonus, all through the real input path.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// ax7Item is the item view of stack id.
func ax7Item(j *journey, id uint64) (game.ItemView, bool) {
	for _, it := range j.world().Items {
		if uint64(it.ID) == id {
			return it, true
		}
	}
	return game.ItemView{}, false
}

// TestJourneyAX7BookcaseFamily (AX-7): the bookcase card stands for both
// bookcases: a click on it arms the small one, the next puts them away;
// each builds from its blueprint.
func TestJourneyAX7BookcaseFamily(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.g.CompleteResearch() // both bookcases need Upholstery
	ax2Place(j, "furniture", "bookcase", "bookcase", 10, 2)
	if w, ok := ax2Frame(j, "bookcase", 10, 2); !ok || w != 2 {
		t.Fatalf("bookcase blueprint %v width %d, want 2", ok, w)
	}
	if !j.clickID("arch.tool.bookcase") || j.a.arch.ToolID != "small_bookcase" {
		t.Fatalf("the card's second click armed %q, want the small bookcase", j.a.arch.ToolID)
	}
	j.reset()
	j.clickTile(14, 2, inputframe.Left, 0)
	if _, ok := j.lastCmd(command.DesignateBuild); !ok {
		t.Fatal("the small bookcase sent no blueprint")
	}
	if _, ok := ax2Frame(j, "small bookcase", 14, 2); !ok {
		t.Fatal("no small bookcase blueprint")
	}
	if !j.clickID("arch.tool.bookcase") || j.a.arch.ToolID != "" {
		t.Fatalf("the card's third click armed %q, want none", j.a.arch.ToolID)
	}
	j.draw(1)
}

// TestJourneyAX7ReadABook (AX-7): clicking a novel shows its title,
// quality and what reading it gives; its Read button sends the nearest
// colonist, who reads it, and the colonist's pane says what it reads.
func TestJourneyAX7ReadABook(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	ada := j.g.Snapshot()[0].ID
	id, err := j.g.SpawnStack(item.Novel, 1, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	it, _ := ax7Item(j, uint64(id))
	if it.AX.Title == "" {
		t.Fatal("the novel has no title")
	}
	j.lookAt(12, 4)
	j.update(1)
	j.settle()
	j.clickTile(12, 4, inputframe.Left, 0)
	pane := j.a.insp.Thing().ItemPane(id)
	if !strings.HasPrefix(pane.Title, it.AX.Title) || !strings.Contains(pane.Title, it.Quality) || pane.Subtitle != "novel" {
		t.Fatalf("pane title %q / %q, want %q (%s)", pane.Title, pane.Subtitle, it.AX.Title, it.Quality)
	}
	if lines := strings.Join(pane.Lines, "\n"); !strings.Contains(lines, "Recreation power") || !strings.Contains(lines, "Intellectual") {
		t.Fatalf("pane lines %q", lines)
	}
	j.reset()
	if !j.clickID("cmd.item.read") {
		t.Fatal("Read is disabled")
	}
	if c, ok := j.lastCmd(command.Prioritize); !ok || c.Order != command.OrderRead || c.X != 12 || c.Y != 4 {
		t.Fatalf("Read sent %+v (%v)", c, ok)
	}
	j.tick(200)
	if p := j.pawn(uint64(ada)); p.AX.Reading != it.AX.Title ||
		!strings.Contains(strings.Join(j.a.insp.Pawn().PawnLines(p), "\n"), "Reading: "+it.AX.Title) {
		t.Fatalf("pawn reading %q, job %q", p.AX.Reading, p.Job)
	}
	j.draw(1) // the reader holding its book
}

// TestJourneyAX7BookcasePane (AX-7): a bookcase's pane lists its books
// and its room's reading bonus, and its Storage button opens the books
// filter; the shelves draw filled.
func TestJourneyAX7BookcasePane(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.g.CompleteResearch()
	for x := 9; x <= 15; x++ { // a roofed room around the bookcase
		for _, y := range []int{6, 11} {
			kind := "wall"
			if y == 11 && x == 12 {
				kind = "door"
			}
			j.g.BuildNow(kind, "wood", "south", x, y)
		}
	}
	for y := 7; y <= 10; y++ {
		j.g.BuildNow("wall", "wood", "south", 9, y)
		j.g.BuildNow("wall", "wood", "south", 15, y)
	}
	j.g.RoofRooms()
	if !j.g.BuildNow("bookcase", "wood", "south", 11, 8) {
		t.Fatal("no bookcase")
	}
	var titles []string
	for i := 0; i < 2; i++ {
		id, _ := j.g.SpawnStack(item.Textbook, 1, 11+i, 8)
		it, _ := ax7Item(j, uint64(id))
		titles = append(titles, it.AX.Title)
	}
	b, ok := ax1Building(j, "bookcase", 11, 8)
	if !ok || b.AX.ReadingBonus != 16 {
		t.Fatalf("bookcase %v bonus %d, want 16", ok, b.AX.ReadingBonus)
	}
	j.lookAt(11, 8)
	j.update(1)
	j.settle()
	for i := 0; i < 4 && !j.a.sel.HasBuilding; i++ { // the books lie on top: clicking again cycles down to the bookcase
		j.settle()
		j.clickTile(11, 8, inputframe.Left, 0)
	}
	lines := strings.Join(j.a.insp.Building().BuildingPane(b.ID).Lines, "\n")
	for _, want := range append(titles, "Reading bonus +1.6%") {
		if !strings.Contains(lines, want) {
			t.Fatalf("bookcase pane %q lacks %q", lines, want)
		}
	}
	if !j.clickID(inspector.StorageID) {
		t.Fatal("the bookcase has no Storage button")
	}
	j.draw(1) // the half-filled bookcase
}
