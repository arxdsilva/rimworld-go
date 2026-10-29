package craft

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/randx"
)

// fakeMakers is the fake host's Makers and Weathering roles (PRD-2):
// a seeded stream, inspirations by pawn, a roofed set, the weather and
// each pawn's outfit and tile. The zero value works.
type fakeMakers struct {
	rng      *randx.Rand
	rolls    int
	inspired map[entity.EntityID]bool
	roofed   map[[2]int]bool
	rain     bool
	wet      map[[2]int]bool
	outfits  map[entity.EntityID]Outfit
	pos      map[entity.EntityID][2]int
	set      int // SetStack calls
}

func (f *fakeHost) Roll(n int) int {
	if f.rng == nil {
		f.rng = randx.New(7)
	}
	f.rolls++
	return f.rng.IntN(n)
}
func (f *fakeHost) UseInspiration(id entity.EntityID) bool {
	if !f.inspired[id] {
		return false
	}
	delete(f.inspired, id)
	return true
}
func (f *fakeHost) Exposed(x, y int) bool { return !f.roofed[[2]int{x, y}] }
func (f *fakeHost) Raining() bool         { return f.rain }
func (f *fakeHost) Wet(x, y int) bool     { return f.wet[[2]int{x, y}] }
func (f *fakeHost) SetStack(id entity.EntityID, st item.Stack) {
	f.set++
	f.stacks[id] = st
}
func (f *fakeHost) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := f.pos[id]
	return p[0], p[1], ok
}
func (f *fakeHost) Outfit(id entity.EntityID) Outfit { return f.outfits[id] }
func (f *fakeHost) SetOutfit(id entity.EntityID, o Outfit) {
	if f.outfits == nil {
		f.outfits = map[entity.EntityID]Outfit{}
	}
	f.outfits[id] = o
}
