package quest

import (
	"slices"

	qd "rimworld-go/internal/domain/quest"
)

// keyKind tags an index key.
type keyKind uint8

const (
	keyMap keyKind = iota + 1
	keyPawn
	keyTile
	keyFaction
)

// key is one index key of a live quest.
type key struct {
	K keyKind
	V uint64
}

// index finds the live quests a signal or a relation change concerns, by
// map, pawn, world tile and faction, without a scan over all quests. Each
// list is ascending; maps are only looked up, never ranged.
type index struct {
	by   map[key][]qd.ID
	keys map[qd.ID][]key // each live quest's current keys
	buf  []key
}

func newIndex() index {
	return index{by: map[key][]qd.ID{}, keys: map[qd.ID][]key{}}
}

// keysOf collects q's keys into buf: giver and target, and per part its
// map, bound pawn and (site, delivery, talk) tile.
func keysOf(q qd.Quest, buf []key) []key {
	buf = buf[:0]
	if q.Giver > 0 {
		buf = append(buf, key{keyFaction, uint64(q.Giver - 1)})
	}
	if q.Target > 0 {
		buf = append(buf, key{keyFaction, uint64(q.Target - 1)})
	}
	for _, p := range q.Parts {
		if p.Map != 0 {
			buf = append(buf, key{keyMap, uint64(p.Map)})
		}
		if p.Pawn != 0 {
			buf = append(buf, key{keyPawn, p.Pawn})
		}
		switch p.Kind {
		case qd.PartSite, qd.PartDelivery, qd.PartTalk:
			if p.Tile >= 0 {
				buf = append(buf, key{keyTile, uint64(p.Tile)})
			}
		}
	}
	slices.SortFunc(buf, func(a, b key) int {
		if a.K != b.K {
			return int(a.K) - int(b.K)
		}
		switch {
		case a.V < b.V:
			return -1
		case a.V > b.V:
			return 1
		}
		return 0
	})
	return slices.Compact(buf)
}

// set indexes live quest q under its current keys (none once terminal).
// Unchanged keys cost one comparison and no allocation.
func (x *index) set(q qd.Quest) {
	x.buf = x.buf[:0]
	if !q.State.Terminal() {
		x.buf = keysOf(q, x.buf)
	}
	old := x.keys[q.ID]
	if slices.Equal(old, x.buf) {
		return
	}
	for _, k := range old {
		x.del(k, q.ID)
	}
	for _, k := range x.buf {
		x.add(k, q.ID)
	}
	if len(x.buf) == 0 {
		delete(x.keys, q.ID)
		return
	}
	x.keys[q.ID] = slices.Clone(x.buf)
}

func (x *index) add(k key, id qd.ID) {
	l := x.by[k]
	i, found := slices.BinarySearch(l, id)
	if !found {
		x.by[k] = slices.Insert(l, i, id)
	}
}

func (x *index) del(k key, id qd.ID) {
	l := x.by[k]
	if i, found := slices.BinarySearch(l, id); found {
		l = slices.Delete(l, i, i+1)
	}
	if len(l) == 0 {
		delete(x.by, k)
		return
	}
	x.by[k] = l
}

// find appends the quests under k to dst.
func (x *index) find(dst []qd.ID, k key) []qd.ID {
	return append(dst, x.by[k]...)
}

// route is the ascending, de-duplicated live quests signal s concerns.
func (x *index) route(dst []qd.ID, s Signal) []qd.ID {
	dst = dst[:0]
	if s.Quest != 0 {
		return append(dst, s.Quest)
	}
	if s.Map != 0 {
		dst = x.find(dst, key{keyMap, uint64(s.Map)})
	}
	if s.Pawn != 0 {
		dst = x.find(dst, key{keyPawn, uint64(s.Pawn)})
	}
	if s.Kind.byTile() && s.Tile >= 0 {
		dst = x.find(dst, key{keyTile, uint64(s.Tile)})
	}
	if s.Faction > 0 {
		dst = x.find(dst, key{keyFaction, uint64(s.Faction - 1)})
	}
	slices.Sort(dst)
	return slices.Compact(dst)
}
