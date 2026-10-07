// Package reservation tracks exclusive entity and tile claims.
package reservation

import (
	"sort"

	"rimworld-go/internal/domain/sim/entity"
)

type tile struct{ X, Y int }

// Table maps reserved entities and tiles to a single owner pawn.
type Table struct {
	entityOwner map[entity.EntityID]entity.EntityID
	tileOwner   map[tile]entity.EntityID
	ownedEnt    map[entity.EntityID][]entity.EntityID
	ownedTile   map[entity.EntityID][]tile
}

// NewTable returns an empty reservation table.
func NewTable() *Table {
	return &Table{
		entityOwner: make(map[entity.EntityID]entity.EntityID),
		tileOwner:   make(map[tile]entity.EntityID),
		ownedEnt:    make(map[entity.EntityID][]entity.EntityID),
		ownedTile:   make(map[entity.EntityID][]tile),
	}
}

// TryEntity claims target for owner. False if another pawn holds it.
func (t *Table) TryEntity(owner, target entity.EntityID) bool {
	if owner == 0 || target == 0 {
		return false
	}
	if cur, ok := t.entityOwner[target]; ok && cur != owner {
		return false
	}
	if _, ok := t.entityOwner[target]; !ok {
		t.entityOwner[target] = owner
		t.ownedEnt[owner] = append(t.ownedEnt[owner], target)
	}
	return true
}

// TryTile claims (x,y) for owner. False if another pawn holds it.
func (t *Table) TryTile(owner entity.EntityID, x, y int) bool {
	if owner == 0 {
		return false
	}
	k := tile{X: x, Y: y}
	if cur, ok := t.tileOwner[k]; ok && cur != owner {
		return false
	}
	if _, ok := t.tileOwner[k]; !ok {
		t.tileOwner[k] = owner
		t.ownedTile[owner] = append(t.ownedTile[owner], k)
	}
	return true
}

// EntityOwner returns the pawn holding target, if any.
func (t *Table) EntityOwner(target entity.EntityID) (entity.EntityID, bool) {
	id, ok := t.entityOwner[target]
	return id, ok
}

// TileOwner returns the pawn holding (x,y), if any.
func (t *Table) TileOwner(x, y int) (entity.EntityID, bool) {
	id, ok := t.tileOwner[tile{X: x, Y: y}]
	return id, ok
}

// ReleaseOwner drops every claim held by owner.
func (t *Table) ReleaseOwner(owner entity.EntityID) {
	for _, e := range t.ownedEnt[owner] {
		if t.entityOwner[e] == owner {
			delete(t.entityOwner, e)
		}
	}
	delete(t.ownedEnt, owner)
	for _, k := range t.ownedTile[owner] {
		if t.tileOwner[k] == owner {
			delete(t.tileOwner, k)
		}
	}
	delete(t.ownedTile, owner)
}

// ReleaseEntity drops a single entity claim if owner holds it.
func (t *Table) ReleaseEntity(owner, target entity.EntityID) {
	if t.entityOwner[target] != owner {
		return
	}
	delete(t.entityOwner, target)
	kept := t.ownedEnt[owner][:0]
	for _, e := range t.ownedEnt[owner] {
		if e != target {
			kept = append(kept, e)
		}
	}
	t.ownedEnt[owner] = kept
}

// Claim is one reservation: Owner holds entity Target, or tile X, Y when
// Tile is set.
type Claim struct {
	Owner, Target entity.EntityID
	Tile          bool
	X, Y          int
}

// Export lists every claim, owners in ascending order and each owner's
// claims in the order it made them (saved games).
func (t *Table) Export() []Claim {
	owners := map[entity.EntityID]bool{}
	for o := range t.ownedEnt {
		owners[o] = true
	}
	for o := range t.ownedTile {
		owners[o] = true
	}
	ids := make([]entity.EntityID, 0, len(owners))
	for o := range owners {
		ids = append(ids, o)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var out []Claim
	for _, o := range ids {
		for _, e := range t.ownedEnt[o] {
			out = append(out, Claim{Owner: o, Target: e})
		}
		for _, tl := range t.ownedTile[o] {
			out = append(out, Claim{Owner: o, Tile: true, X: tl.X, Y: tl.Y})
		}
	}
	return out
}

// Import rebuilds a table from Export output.
func Import(claims []Claim) *Table {
	t := NewTable()
	for _, c := range claims {
		if c.Tile {
			t.TryTile(c.Owner, c.X, c.Y)
		} else {
			t.TryEntity(c.Owner, c.Target)
		}
	}
	return t
}
