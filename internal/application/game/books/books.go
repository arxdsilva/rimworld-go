// Package books is reading (AX-7, vanilla 1.5 core books; ENG-003 wiki
// numbers, original names): colonists read novels, textbooks and
// schematics for recreation, textbooks teach skills, schematics put
// research points into their projects, and books shelved in a room's
// bookcases give that room a reading bonus (reading recreation and
// learning, and research at its benches). The books themselves are
// domain economy/book; the bookcases are storage buildings (game/store,
// zone.KindBookcase).
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package; the roles
// in deps.go are its only way in. It keeps no clock; its only dice are
// the game's (Colony.Roll), drawn only when a book can be picked, so a
// colony without books draws nothing. Everything it keeps is derived
// (never saved): a reading job carries its own progress.
package books

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Books is the reading component, built by New.
type Books struct {
	s  Sites
	it Items
	hd Hands
	m  Mind
	c  Colony

	// loose: every loose book, ascending, at item generation looseGen.
	loose    []entity.EntityID
	looseGen uint64
	looseOK  bool
	// cases, seats: the bookcases and seat cells, at structure caseVer.
	cases   []entity.EntityID
	seats   [][2]int
	caseVer uint64
	casesOK bool
	// rooms: books shelved per room, at (bonusVer, bonusGen).
	rooms              map[int]int
	bonusVer, bonusGen uint64
	bonusOK            bool
	cands              []entity.EntityID // Seek's scratch
}

// New builds the reading component over the game's roles.
func New(s Sites, it Items, hd Hands, m Mind, c Colony) *Books {
	return &Books{s: s, it: it, hd: hd, m: m, c: c}
}

// IsBookcase reports a bookcase kind.
func IsBookcase(k construction.Kind) bool {
	return k == construction.Bookcase || k == construction.SmallBookcase
}

// structure rebuilds the bookcase and seat lists when the buildings
// changed.
func (b *Books) structure() {
	if v := b.s.StructVer(); b.casesOK && v == b.caseVer {
		return
	}
	b.caseVer, b.casesOK, b.cases, b.seats = b.s.StructVer(), true, b.cases[:0], b.seats[:0]
	for _, id := range b.s.BuildingIDs() {
		bl, ok := b.s.Building(id)
		switch {
		case !ok:
		case IsBookcase(bl.Kind):
			b.cases = append(b.cases, id)
		case construction.Seats(bl.Kind) > 0 && !bl.Kind.IsTable(): // a chair, stool, armchair or couch
			cells, _, _ := construction.Footprint(bl.Kind, bl.Facing, bl.X, bl.Y)
			b.seats = append(b.seats, cells...)
		}
	}
}

// looseBooks is every loose book, rebuilt when a stack moved since.
func (b *Books) looseBooks() []entity.EntityID {
	if g := b.c.Gen(); b.looseOK && g == b.looseGen {
		return b.loose
	}
	b.looseGen, b.looseOK, b.loose = b.c.Gen(), true, b.loose[:0]
	for _, id := range b.it.ItemIDs() {
		if st := b.it.StackRef(id); st != nil && st.Kind.IsBook() && st.Carrier == 0 {
			b.loose = append(b.loose, id)
		}
	}
	return b.loose
}
