package haul

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Ground is the per-cell index of the stacks lying loose (not carried) on
// the map (STO-1): each cell lists the ids on it in item-list order, so
// the first stack on a tile is the one a scan of the item list finds
// first. Like the game's other cell indexes it is verified, not trusted:
// the lists are rebuilt from the item list whenever the generation moves
// (the game calls Moved when a stack is spawned, set down, moved, picked
// up, merged or deleted) or the store behind them was replaced (a load),
// and every lookup re-checks each listed stack against the store (still
// there, loose, on that cell). Under the game's index self-check each
// answer is compared with the item-list scan, and a difference panics.
// The zero value is ready to use; it is never saved.
type Ground struct {
	gen, at uint64 // Moved's count; the count the lists were built at
	ok      bool
	src     any // the store the lists were built from
	w, h    int
	off     bool    // a loose stack lay off the map: off-map lookups scan
	start   []int32 // per cell: first index into ids
	n       []int32 // per cell: how many ids (0 on cells not in used)
	used    []int32 // the cells with ids
	ids     []entity.EntityID
	buf     []entity.EntityID // Loose's answer
	scan    []entity.EntityID // the self-check's scan
}

// Items is what the ground index reads of the simulation (a
// consumer-owned role): the item list and store, the map size and
// whether this lookup is checked against the scan.
type Items interface {
	ItemIDs() []entity.EntityID
	// StackRef is stack id in the store (nil: none), read only.
	StackRef(id entity.EntityID) *item.Stack
	// Store is the store's identity: a new one (a load) rebuilds.
	Store() any
	MapSize() (w, h int)
	SelfCheck() bool
}

// Moved records that a stack may have arrived on, or left, a cell.
func (gr *Ground) Moved() { gr.gen++ }

// Warm builds the lists now (no first-lookup spike after a load).
func (gr *Ground) Warm(h Items) { gr.sync(h) }

// Loose lists the stacks lying loose on (x, y), in item-list order. The
// slice is reused by the next call.
func (gr *Ground) Loose(h Items, x, y int) []entity.EntityID {
	gr.buf = gr.looseIndexed(h, x, y, gr.buf[:0])
	if h.SelfCheck() {
		gr.scan = looseScan(h, x, y, gr.scan[:0])
		if !slices.Equal(gr.buf, gr.scan) {
			panic(fmt.Sprintf("haul.Ground.Loose(%d,%d) index %v, scan %v", x, y, gr.buf, gr.scan))
		}
	}
	return gr.buf
}

// First is the first stack lying loose on (x, y) (0: none).
func (gr *Ground) First(h Items, x, y int) entity.EntityID {
	if l := gr.Loose(h, x, y); len(l) > 0 {
		return l[0]
	}
	return 0
}

// Others counts the stacks lying loose on (x, y) besides id.
func (gr *Ground) Others(h Items, id entity.EntityID, x, y int) int {
	n := 0
	for _, oid := range gr.Loose(h, x, y) {
		if oid != id {
			n++
		}
	}
	return n
}

// Room is how much of kind k storage cell (x, y), holding up to slots
// stacks, still takes (Cell.Room).
func (gr *Ground) Room(h Items, k item.Kind, x, y, slots int) int {
	var c Cell
	for _, oid := range gr.Loose(h, x, y) {
		o := h.StackRef(oid)
		c.Add(o.Kind == k, o.Qty)
	}
	return c.Room(k.StackLimit(), slots)
}

// Shelf is how many stacks lie loose on shelf cell (x, y) and whether
// one of them takes a load of kind k (Toppable: as a plain cell, the kind
// decides).
func (gr *Ground) Shelf(h Items, k item.Kind, x, y int) (n int, top bool) {
	for _, oid := range gr.Loose(h, x, y) {
		o := h.StackRef(oid)
		n++
		top = top || o.Kind == k && Toppable(*o)
	}
	return n, top
}

// looseIndexed is Loose from the lists.
func (gr *Ground) looseIndexed(h Items, x, y int, out []entity.EntityID) []entity.EntityID {
	gr.sync(h)
	c := gr.cell(x, y)
	if c < 0 {
		if gr.off {
			return looseScan(h, x, y, out)
		}
		return out
	}
	if gr.n[c] == 0 {
		return out
	}
	for _, id := range gr.ids[gr.start[c] : gr.start[c]+gr.n[c]] {
		if st := h.StackRef(id); st != nil && st.Carrier == 0 && st.X == x && st.Y == y {
			out = append(out, id)
		}
	}
	return out
}

// looseScan is Loose over the whole item list (the original search).
func looseScan(h Items, x, y int, out []entity.EntityID) []entity.EntityID {
	for _, id := range h.ItemIDs() {
		if st := h.StackRef(id); st != nil && st.Carrier == 0 && st.X == x && st.Y == y {
			out = append(out, id)
		}
	}
	return out
}

// cell is the list index of (x, y), or -1 off the map.
func (gr *Ground) cell(x, y int) int {
	if uint(x) >= uint(gr.w) || uint(y) >= uint(gr.h) {
		return -1
	}
	return y*gr.w + x
}

// sync rebuilds the lists when the generation moved, the store was
// replaced or the map changed size: a count per cell, then the ids
// grouped by cell in item-list order (two passes, nothing allocated once
// the arrays have grown).
func (gr *Ground) sync(h Items) {
	w, ht := h.MapSize()
	src := h.Store()
	if gr.ok && gr.at == gr.gen && gr.src == src && gr.w == w && gr.h == ht {
		return
	}
	if len(gr.n) != w*ht {
		gr.start, gr.n, gr.used = make([]int32, w*ht), make([]int32, w*ht), gr.used[:0]
	}
	for _, c := range gr.used {
		gr.n[c] = 0
	}
	gr.used = gr.used[:0]
	gr.ok, gr.at, gr.src, gr.w, gr.h, gr.off = true, gr.gen, src, w, ht, false
	ids := h.ItemIDs()
	for _, id := range ids {
		if c := gr.looseCell(h, id); c >= 0 {
			if gr.n[c] == 0 {
				gr.used = append(gr.used, int32(c))
			}
			gr.n[c]++
		}
	}
	total := int32(0)
	for _, c := range gr.used {
		gr.start[c], total, gr.n[c] = total, total+gr.n[c], 0
	}
	gr.ids = slices.Grow(gr.ids[:0], int(total))[:total]
	for _, id := range ids {
		if c := gr.looseCell(h, id); c >= 0 {
			gr.ids[gr.start[c]+gr.n[c]] = id
			gr.n[c]++
		}
	}
}

// looseCell is the cell stack id lies loose on, -1 when it is carried,
// gone or off the map (noting the last).
func (gr *Ground) looseCell(h Items, id entity.EntityID) int {
	st := h.StackRef(id)
	if st == nil || st.Carrier != 0 {
		return -1
	}
	c := gr.cell(st.X, st.Y)
	if c < 0 {
		gr.off = true
	}
	return c
}
