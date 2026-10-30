// Package reservation tracks entity and tile claims (RES-1): a whole claim
// holds a target for one pawn; count claims share a stack, each for the
// part a pawn will take, while their sum fits the stack's quantity.
package reservation

import (
	"slices"
	"sort"

	"rimworld-go/internal/domain/sim/entity"
)

type tile struct{ X, Y int }

// part is one count claim: owner takes n of the target.
type part struct {
	owner entity.EntityID
	n     int
}

// Table maps reserved entities and tiles to their owner pawns. A target
// holds either one whole claim or count claims, never both.
type Table struct {
	entityOwner map[entity.EntityID]entity.EntityID
	counts      map[entity.EntityID][]part // count claims per target, in claim order
	tileOwner   map[tile]entity.EntityID
	ownedEnt    map[entity.EntityID][]entity.EntityID // whole and count targets, in claim order
	ownedTile   map[entity.EntityID][]tile
}

// NewTable returns an empty reservation table.
func NewTable() *Table {
	return &Table{
		entityOwner: make(map[entity.EntityID]entity.EntityID),
		counts:      make(map[entity.EntityID][]part),
		tileOwner:   make(map[tile]entity.EntityID),
		ownedEnt:    make(map[entity.EntityID][]entity.EntityID),
		ownedTile:   make(map[entity.EntityID][]tile),
	}
}

// TryEntity claims the whole of target for owner. False if another pawn
// holds it, in whole or in part. Owner's own count claim becomes whole.
func (t *Table) TryEntity(owner, target entity.EntityID) bool {
	if owner == 0 || target == 0 {
		return false
	}
	if cur, ok := t.entityOwner[target]; ok {
		return cur == owner
	}
	ps := t.counts[target]
	for _, p := range ps {
		if p.owner != owner {
			return false
		}
	}
	if len(ps) == 0 {
		t.ownedEnt[owner] = append(t.ownedEnt[owner], target)
	}
	delete(t.counts, target)
	t.entityOwner[target] = owner
	return true
}

// TryCount claims n of stack target, which holds total, for owner. Count
// claims coexist while their sum is at most total; a whole claim by
// another pawn excludes them (owner's own whole claim already covers n).
// Claiming again replaces owner's count.
func (t *Table) TryCount(owner, target entity.EntityID, n, total int) bool {
	if owner == 0 || target == 0 || n <= 0 {
		return false
	}
	if cur, ok := t.entityOwner[target]; ok {
		return cur == owner
	}
	ps := t.counts[target]
	mine, others := -1, 0
	for i, p := range ps {
		if p.owner == owner {
			mine = i
		} else {
			others += p.n
		}
	}
	if others+n > total {
		return false
	}
	if mine >= 0 {
		ps[mine].n = n
		return true
	}
	t.addCount(owner, target, n)
	return true
}

func (t *Table) addCount(owner, target entity.EntityID, n int) {
	t.counts[target] = append(t.counts[target], part{owner: owner, n: n})
	t.ownedEnt[owner] = append(t.ownedEnt[owner], target)
}

// FreeFor is how much of stack target (holding total) owner may still
// claim: total less every other pawn's count claim; 0 when another pawn
// holds it whole, total when owner does. Never below 0.
func (t *Table) FreeFor(owner, target entity.EntityID, total int) int {
	if cur, ok := t.entityOwner[target]; ok {
		if cur == owner && owner != 0 {
			return total
		}
		return 0
	}
	for _, p := range t.counts[target] {
		if p.owner != owner {
			total -= p.n
		}
	}
	return max(0, total)
}

// Free is how much of stack target (holding total) nobody has claimed.
func (t *Table) Free(target entity.EntityID, total int) int { return t.FreeFor(0, target, total) }

// TryTile claims (x,y) for owner. False if another pawn holds it.
func (t *Table) TryTile(owner entity.EntityID, x, y int) bool {
	if owner == 0 {
		return false
	}
	k := tile{X: x, Y: y}
	if cur, ok := t.tileOwner[k]; ok {
		return cur == owner
	}
	t.tileOwner[k] = owner
	t.ownedTile[owner] = append(t.ownedTile[owner], k)
	return true
}

// EntityOwner returns the pawn holding target, if any: the whole claim's
// owner, else the lowest owner of a count claim (a shared stack reads as
// taken to callers that don't share; the lowest, so a loaded game, whose
// claims come back in owner order, answers the same).
func (t *Table) EntityOwner(target entity.EntityID) (entity.EntityID, bool) {
	if id, ok := t.entityOwner[target]; ok {
		return id, true
	}
	var low entity.EntityID
	for _, p := range t.counts[target] {
		if low == 0 || p.owner < low {
			low = p.owner
		}
	}
	return low, low != 0
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
		} else {
			t.dropCount(e, owner)
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

// dropCount removes owner's count claim on target, reporting one existed.
func (t *Table) dropCount(target, owner entity.EntityID) bool {
	ps := t.counts[target]
	i := slices.IndexFunc(ps, func(p part) bool { return p.owner == owner })
	if i < 0 {
		return false
	}
	ps = slices.Delete(ps, i, i+1)
	if len(ps) == 0 {
		delete(t.counts, target)
	} else {
		t.counts[target] = ps
	}
	return true
}

// ReleaseEntity drops owner's claim on target, whole or count.
func (t *Table) ReleaseEntity(owner, target entity.EntityID) {
	if t.entityOwner[target] == owner && owner != 0 {
		delete(t.entityOwner, target)
	} else if !t.dropCount(target, owner) {
		return
	}
	t.forget(owner, target)
}

// forget takes target off owner's claim list.
func (t *Table) forget(owner, target entity.EntityID) {
	kept := slices.DeleteFunc(t.ownedEnt[owner], func(e entity.EntityID) bool { return e == target })
	if len(kept) == 0 {
		delete(t.ownedEnt, owner)
	} else {
		t.ownedEnt[owner] = kept
	}
}

// ReleaseTarget drops every claim on target (it left the map): its owners'
// jobs then find it gone.
func (t *Table) ReleaseTarget(target entity.EntityID) {
	if cur, ok := t.entityOwner[target]; ok {
		delete(t.entityOwner, target)
		t.forget(cur, target)
	}
	ps := t.counts[target]
	delete(t.counts, target)
	for _, p := range ps {
		t.forget(p.owner, target)
	}
}

// ReleaseTile drops the claim on (x,y), whoever holds it.
func (t *Table) ReleaseTile(x, y int) {
	k := tile{X: x, Y: y}
	owner, ok := t.tileOwner[k]
	if !ok {
		return
	}
	delete(t.tileOwner, k)
	kept := slices.DeleteFunc(t.ownedTile[owner], func(o tile) bool { return o == k })
	if len(kept) == 0 {
		delete(t.ownedTile, owner)
	} else {
		t.ownedTile[owner] = kept
	}
}

// Claim is one reservation: Owner holds entity Target (Count of it when
// above 0, else the whole), or tile X, Y when Tile is set. Fields are
// append only (saved games).
type Claim struct {
	Owner, Target entity.EntityID
	Tile          bool
	X, Y          int
	Count         int
}

// Export lists every claim, owners in ascending order and each owner's
// claims in the order it made them, entities before tiles (saved games).
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
			c := Claim{Owner: o, Target: e}
			for _, p := range t.counts[e] {
				if p.owner == o {
					c.Count = p.n
				}
			}
			out = append(out, c)
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
		switch {
		case c.Tile:
			t.TryTile(c.Owner, c.X, c.Y)
		case c.Count > 0 && c.Owner != 0 && c.Target != 0:
			if _, whole := t.entityOwner[c.Target]; !whole {
				t.dropCount(c.Target, c.Owner)
				t.forget(c.Owner, c.Target)
				t.addCount(c.Owner, c.Target, c.Count)
			}
		default:
			t.TryEntity(c.Owner, c.Target)
		}
	}
	return t
}
