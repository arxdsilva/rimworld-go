package windows

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// arrowShell is a shell with a cursor and a camera that places the tile
// (x, y) at (x*10, y*10) in map pixels.
type arrowShell struct {
	*fakeShell
	mx, my int
}

func (s *arrowShell) Cursor() (int, int)  { return s.mx, s.my }
func (s *arrowShell) Tick() clock.Tick    { return 0 }
func (s *arrowShell) MapSize() (int, int) { return 100, 100 }
func (s *arrowShell) WorldToScreenF(wx, wy float64) (float32, float32) {
	return float32(wx * 10), float32(wy * 10)
}

type wildReader struct {
	fakeReader
	rows []game.WildlifeRow
}

func (r wildReader) Wildlife() []game.WildlifeRow { return r.rows }

// openWild is the Wild list with a deer row, the cursor on it.
func openWild(animals ...game.AnimalView) (*arrowShell, Animals) {
	fs := &fakeShell{}
	sh := &arrowShell{fakeShell: fs}
	fs.env.Game = wildReader{rows: []game.WildlifeRow{{Species: "deer", Label: "Deer", Count: len(animals)}, {Species: "boar", Label: "Boar", Count: 1}}}
	fs.world.Animals = animals
	w := New(sh)
	w.Open, w.Mode, w.Wild = true, "animals", true
	a := w.Animals()
	r := a.Rect()
	sh.mx, sh.my = int(r.X)+40, int(r.Y)+animalsRowTop+animalRowH/2
	return sh, a
}

// CTL-019: the hovered species row points at its animal nearest the middle
// of the screen, and only at one that is on screen and out of the fog.
func TestWildRowArrowPointsAtAVisibleAnimal(t *testing.T) {
	t.Parallel()
	sh, a := openWild(
		game.AnimalView{ID: 1, Species: "deer", X: 5, Y: 5},     // (55, 55): near a corner
		game.AnimalView{ID: 2, Species: "deer", X: 60, Y: 34},   // (605, 345): near the middle of 1280x720
		game.AnimalView{ID: 3, Species: "deer", X: 300, Y: 300}, // off screen
		game.AnimalView{ID: 4, Species: "deer", X: 61, Y: 35, Tame: true},
	)
	_, _, x1, y1, ok := a.WildArrow()
	if !ok || x1 != 605 || y1 != 345 {
		t.Fatalf("arrow to (%v, %v) ok=%v, want the middle one (605, 345)", x1, y1, ok)
	}
	// the nearest is under fog: the next one
	fog := make([]uint8, 100*100)
	fog[34*100+60] = 1
	sh.world.Fog = fog
	if _, _, x1, y1, ok = a.WildArrow(); !ok || x1 != 55 || y1 != 55 {
		t.Fatalf("fogged animal skipped: (%v, %v) %v", x1, y1, ok)
	}
}

func TestWildRowArrowNeedsAVisibleAnimal(t *testing.T) {
	t.Parallel()
	sh, a := openWild(game.AnimalView{ID: 3, Species: "deer", X: 300, Y: 300}) // off screen
	if _, _, _, _, ok := a.WildArrow(); ok {
		t.Fatal("no arrow to an animal off screen")
	}
	sh.world.Animals = []game.AnimalView{{ID: 5, Species: "deer", X: 10, Y: 10}}
	if _, _, _, _, ok := a.WildArrow(); !ok {
		t.Fatal("an animal on screen gets an arrow")
	}
	fog := make([]uint8, 100*100)
	fog[10*100+10] = 1
	sh.world.Fog = fog
	if _, _, _, _, ok := a.WildArrow(); ok {
		t.Fatal("no arrow under fog")
	}
	sh.world.Fog = nil
	sh.mx, sh.my = -5, -5 // off the rows
	if _, _, _, _, ok := a.WildArrow(); ok {
		t.Fatal("no arrow without a hovered row")
	}
	r := a.Rect()
	sh.mx, sh.my = int(r.X)+40, int(r.Y)+animalsRowTop+animalRowH+animalRowH/2 // the boar row: none of it
	if _, _, _, _, ok := a.WildArrow(); ok {
		t.Fatal("a species with no animal shows no arrow")
	}
	a.Wild = false
	sh.mx, sh.my = int(r.X)+40, int(r.Y)+animalsRowTop+animalRowH/2
	if _, _, _, _, ok := a.WildArrow(); ok {
		t.Fatal("the Colony list has no arrow")
	}
}
