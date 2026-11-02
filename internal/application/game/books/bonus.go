package books

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/book"
	"rimworld-go/internal/domain/sim/entity"
)

// The reading bonus (wiki: Reading bonus): only books shelved in the
// bookcases of a room count, by number alone (book.Bonus). It multiplies
// reading recreation and learning in that room and research at its
// benches. Outdoors there is none.

// shelved is the books on each indoor room's bookcases, recounted when a
// building or a loose stack changed.
func (b *Books) shelved() map[int]int {
	b.structure()
	v, g := b.s.StructVer(), b.c.Gen()
	if b.bonusOK && v == b.bonusVer && g == b.bonusGen {
		return b.rooms
	}
	b.bonusVer, b.bonusGen, b.bonusOK = v, g, true
	clear(b.rooms)
	for _, id := range b.cases {
		bl, ok := b.s.Building(id)
		if !ok {
			continue
		}
		r, indoors := b.s.Room(bl.X, bl.Y)
		if !indoors {
			continue
		}
		if b.rooms == nil {
			b.rooms = map[int]int{}
		}
		b.rooms[r] += b.held(bl)
	}
	return b.rooms
}

// held is the books a bookcase holds.
func (b *Books) held(bl construction.Building) int {
	cells, _, _ := construction.Footprint(bl.Kind, bl.Facing, bl.X, bl.Y)
	n := 0
	for _, c := range cells {
		n += b.it.StacksAt(c[0], c[1])
	}
	return n
}

// RoomBonus is indoor room r's reading bonus, permille over 1000.
func (b *Books) RoomBonus(r int) int { return book.Bonus(b.shelved()[r]) }

// BonusAt is the reading bonus where (x, y) stands (0 outdoors).
func (b *Books) BonusAt(x, y int) int {
	r, indoors := b.s.Room(x, y)
	if !indoors {
		return 0
	}
	return b.RoomBonus(r)
}

// CaseBonus is bookcase id's room's reading bonus (BuildingView.AX
// .ReadingBonus; 0 for any other building).
func (b *Books) CaseBonus(id entity.EntityID, bl construction.Building) int {
	if !IsBookcase(bl.Kind) {
		return 0
	}
	return b.BonusAt(bl.X, bl.Y)
}
