package craft

import "rimworld-go/internal/domain/sim/entity"

// The fake's reservation table (RES-1): claims holds each target's whole
// owner, or its first count owner; counts the count claims of a target
// that has a claims entry (a test that resets claims drops them too);
// tiles the work spots held.

func (f *fakeHost) Claims() Claims { return f }

func (f *fakeHost) EntityOwner(target entity.EntityID) (entity.EntityID, bool) {
	o, ok := f.claims[target]
	return o, ok
}

// countsOf is target's live count claims (nil: none, or held whole).
func (f *fakeHost) countsOf(target entity.EntityID) map[entity.EntityID]int {
	if _, ok := f.claims[target]; !ok {
		delete(f.counts, target)
		return nil
	}
	return f.counts[target]
}

func (f *fakeHost) TryEntity(owner, target entity.EntityID) bool {
	if o, ok := f.claims[target]; ok && o != owner {
		return false
	}
	for o := range f.countsOf(target) {
		if o != owner {
			return false
		}
	}
	delete(f.counts, target)
	f.claims[target] = owner
	return true
}

func (f *fakeHost) TryCount(owner, target entity.EntityID, n, total int) bool {
	cs := f.countsOf(target)
	if o, ok := f.claims[target]; ok && len(cs) == 0 {
		return o == owner
	}
	if n <= 0 || n > f.FreeFor(owner, target, total) {
		return false
	}
	if cs == nil {
		cs = map[entity.EntityID]int{}
		if f.counts == nil {
			f.counts = map[entity.EntityID]map[entity.EntityID]int{}
		}
		f.counts[target] = cs
	}
	cs[owner] = n
	if _, ok := f.claims[target]; !ok {
		f.claims[target] = owner
	}
	return true
}

func (f *fakeHost) FreeFor(owner, target entity.EntityID, total int) int {
	cs := f.countsOf(target)
	if o, ok := f.claims[target]; ok && len(cs) == 0 {
		if o == owner {
			return total
		}
		return 0
	}
	for o, n := range cs {
		if o != owner {
			total -= n
		}
	}
	return max(0, total)
}

func (f *fakeHost) ReleaseEntity(owner, target entity.EntityID) {
	cs := f.countsOf(target)
	delete(cs, owner)
	if f.claims[target] != owner {
		return
	}
	delete(f.claims, target)
	for o := range cs {
		if cur, ok := f.claims[target]; !ok || o < cur {
			f.claims[target] = o
		}
	}
}

func (f *fakeHost) TileOwner(x, y int) (entity.EntityID, bool) {
	o, ok := f.tiles[[2]int{x, y}]
	return o, ok
}

func (f *fakeHost) TryTile(owner entity.EntityID, x, y int) bool {
	if o, ok := f.tiles[[2]int{x, y}]; ok && o != owner {
		return false
	}
	if f.tiles == nil {
		f.tiles = map[[2]int]entity.EntityID{}
	}
	f.tiles[[2]int{x, y}] = owner
	return true
}

// InSource: (x, y) lies in the fake's source rectangle for src.
func (f *fakeHost) InSource(src uint64, x, y int) bool {
	r, ok := f.sources[src]
	return ok && x >= r[0] && y >= r[1] && x <= r[2] && y <= r[3]
}

func (f *fakeHost) SourceName(src uint64) string {
	if _, ok := f.sources[src]; ok {
		return "Stockpile zone"
	}
	return ""
}
