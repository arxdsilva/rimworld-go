package craft

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// Art is the sculptures colonists carve (PRD-3): a sculpting bench bill
// sets the finished piece down as a building on the nearest free cell
// beside the bench, its quality rolled from the carver's skill (and an
// inspiration), with a title and description generated from a seed on
// the craft stream (artgen.go). There is no minified-item path in the
// game: the player moves a sculpture with the Move order (UI-011), which
// keeps its entity and so its title. Saved: Game.Art (the pieces map).
type Art struct {
	h       Host
	quality *Quality
	pieces  map[entity.EntityID]Artwork
}

// Artwork is a sculpture's saved art: the seed its title and description
// grow from, and who carved it.
type Artwork struct {
	Seed  uint32
	Maker string
}

// artReach is how far (Chebyshev tiles from the work spot) a carver looks
// for a free cell to set a sculpture down (ours).
const artReach = 4

// IsSculpture reports a sculpture kind.
func IsSculpture(k construction.Kind) bool {
	return k == construction.SmallSculpture || k == construction.LargeSculpture || k == construction.GrandSculpture
}

// Export is the pieces of the sculptures still standing, for a save (a
// fresh map).
func (a *Art) Export() map[entity.EntityID]Artwork {
	out := make(map[entity.EntityID]Artwork, len(a.pieces))
	for id, p := range a.pieces {
		if b, ok := a.h.Building(id); ok && IsSculpture(b.Kind) {
			out[id] = p
		}
	}
	return out
}

// Import loads saved pieces (nil, an older save: none).
func (a *Art) Import(m map[entity.EntityID]Artwork) {
	a.pieces = map[entity.EntityID]Artwork{}
	for id, p := range m {
		a.pieces[id] = p
	}
}

// Artwork is sculpture id's art (ok false for any other building).
func (a *Art) Artwork(id entity.EntityID) (Artwork, bool) {
	p, ok := a.pieces[id]
	return p, ok
}

// Title is sculpture b's (building id) title ("" when it has none).
func (a *Art) Title(id entity.EntityID, b construction.Building) string {
	p, ok := a.pieces[id]
	if !ok || !IsSculpture(b.Kind) {
		return ""
	}
	return ArtTitle(p.Seed)
}

// Describe is sculpture b's (building id) description ("" when none).
func (a *Art) Describe(id entity.EntityID, b construction.Building) string {
	p, ok := a.pieces[id]
	if !ok || !IsSculpture(b.Kind) {
		return ""
	}
	l, _ := quality.FromStored(b.Quality)
	return ArtDescription(p.Seed, b.Kind, b.Stuff, l, p.Maker)
}

// make finishes sculpture bill j of recipe d at bench b for carver id:
// the piece goes down on the nearest free cell, the materials (the
// carried load and those laid on the bench) are used up, and the colony
// is told its title. False, with nothing used, when no cell is free.
func (a *Art) make(id entity.EntityID, j job.Record, b construction.Building, d recipe.Def) bool {
	h := a.h
	stuff := item.Kind(0)
	if st, ok := h.Stack(j.Carry); ok {
		stuff = st.Kind
	}
	if stuff.StuffCategory() == 0 {
		stuff = d.Input.Kind
	}
	x, y, ok := a.cell(d.Builds, j.DestX, j.DestY)
	if !ok {
		h.Message(j.DestX, j.DestY, "No room beside the sculpting bench to set the sculpture down.")
		return false
	}
	q := quality.Stored(a.quality.Roll(id, d.Learn))
	bid := h.Install(d.Builds, stuff, q, x, y)
	if bid == 0 {
		return false
	}
	p := Artwork{Seed: uint32(h.Roll(1 << 31)), Maker: h.Name(id)}
	a.pieces[bid] = p
	for _, rid := range j.Route {
		h.DeleteItem(rid)
	}
	h.DeleteItem(j.Carry)
	who := p.Maker
	if who == "" {
		who = "Someone"
	}
	l, _ := quality.FromStored(q)
	h.Message(x, y, fmt.Sprintf("%s finished %q, a %s %s.", who, ArtTitle(p.Seed), l, construction.DefOf(d.Builds).Name))
	return true
}

// cell is the nearest cell to (sx, sy), ring by ring in a fixed order,
// where a sculpture of kind k may stand: no pawn on it or the work spot,
// and the host's placement rule allows it (CanInstall).
func (a *Art) cell(k construction.Kind, sx, sy int) (int, int, bool) {
	for r := 1; r <= artReach; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				x, y := sx+dx, sy+dy
				if !a.clear(k, x, y, sx, sy) {
					continue
				}
				if a.h.CanInstall(k, x, y) {
					return x, y, true
				}
			}
		}
	}
	return 0, 0, false
}

// clear reports a sculpture of kind k at (x, y) would cover no pawn and
// not the work spot (sx, sy).
func (a *Art) clear(k construction.Kind, x, y, sx, sy int) bool {
	cells, _, _ := construction.Footprint(k, construction.South, x, y)
	for _, c := range cells {
		if c[0] == sx && c[1] == sy {
			return false
		}
		for _, pid := range a.h.PawnIDs() {
			if px, py, ok := a.h.Pos(pid); ok && px == c[0] && py == c[1] {
				return false
			}
		}
	}
	return true
}
