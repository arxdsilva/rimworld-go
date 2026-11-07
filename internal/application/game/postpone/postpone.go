// Package postpone is the store of postponed work (SIM-027): the pawn jobs
// put off because the way was blocked or the target out of reach, indexed
// per pawn and job kind. It knows nothing of the game; the game package
// decides when to put work off and when to forget it.
package postpone

import (
	"slices"
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// ViewLines caps the postponed-work lines in a colonist's view.
const ViewLines = 3

// Key names one piece of postponed work: a pawn, a job kind and its
// target (an entity, or a tile when Target is 0).
type Key struct {
	Pawn   entity.EntityID
	Kind   job.Kind
	Target entity.EntityID
	X, Y   int
}

// Val is when the work may be tried again, and whether it was put off
// for being out of reach (cleared when walkability changes).
type Val struct {
	Until   uint64
	Unreach bool
}

// Entry is one saved postponement.
type Entry struct {
	Key     Key
	Until   uint64
	Unreach bool // absent in older saves
}

// pawnKind is the part of a Key the per-pawn index groups by.
type pawnKind struct {
	Pawn entity.EntityID
	Kind job.Kind
}

// Store is the postponed work. Maps are only looked up in decisions;
// whole-store passes (walkability clear, sweep) only delete, so iteration
// order never matters.
type Store struct {
	m map[Key]Val
	// kinds lists m's keys per pawn and job kind (in no particular order:
	// only membership is read), so a giver's candidate loop can tell which
	// targets are put off without hashing a full key per candidate. Never
	// saved: rebuilt with m.
	kinds map[pawnKind][]Key
	// Rev is the grid revision the out-of-reach entries were judged at.
	Rev uint64
	// SweepAt is the tick of the next sweep of expired entries.
	SweepAt uint64
}

// New is an empty store.
func New() Store {
	return Store{m: make(map[Key]Val), kinds: make(map[pawnKind][]Key)}
}

// Len is the number of entries.
func (s *Store) Len() int { return len(s.m) }

// Get is k's value.
func (s *Store) Get(k Key) (Val, bool) {
	v, ok := s.m[k]
	return v, ok
}

// Has reports whether pawn id has any entry of this kind (the index, not
// the map: the cheap pre-check before hashing a full key).
func (s *Store) Has(id entity.EntityID, kind job.Kind) bool {
	return len(s.kinds[pawnKind{id, kind}]) != 0
}

// Keys is the keys of pawn id's entries of this kind, in no order. The
// slice is the store's own: do not keep or change it.
func (s *Store) Keys(id entity.EntityID, kind job.Kind) []Key {
	return s.kinds[pawnKind{id, kind}]
}

// All visits every entry in no particular order. The visitor may Drop the
// entry it is given.
func (s *Store) All(visit func(Key, Val)) {
	for k, v := range s.m {
		visit(k, v)
	}
}

// Put records k (listed once however often it is renewed).
func (s *Store) Put(k Key, v Val) {
	if _, ok := s.m[k]; !ok {
		if s.kinds == nil {
			s.kinds = make(map[pawnKind][]Key)
		}
		pk := pawnKind{k.Pawn, k.Kind}
		s.kinds[pk] = append(s.kinds[pk], k)
	}
	s.m[k] = v
}

// Drop forgets k, which is in the store.
func (s *Store) Drop(k Key) {
	delete(s.m, k)
	pk := pawnKind{k.Pawn, k.Kind}
	l := s.kinds[pk]
	if i := slices.Index(l, k); i >= 0 {
		l[i] = l[len(l)-1]
		l = l[:len(l)-1]
	}
	if len(l) == 0 {
		delete(s.kinds, pk)
	} else {
		s.kinds[pk] = l
	}
}

// Reindex rebuilds the per-pawn index from the entries.
func (s *Store) Reindex() {
	if s.kinds == nil {
		s.kinds = make(map[pawnKind][]Key, len(s.m))
	}
	clear(s.kinds)
	for k := range s.m {
		pk := pawnKind{k.Pawn, k.Kind}
		s.kinds[pk] = append(s.kinds[pk], k)
	}
}

// Export is every entry in key order (the saved form).
func (s *Store) Export() []Entry {
	out := make([]Entry, 0, len(s.m))
	for k, v := range s.m {
		out = append(out, Entry{k, v.Until, v.Unreach})
	}
	SortEntries(out)
	return out
}

// Load replaces the entries with in and rebuilds the index.
func (s *Store) Load(in []Entry) {
	s.m = make(map[Key]Val, len(in))
	for _, e := range in {
		s.m[e.Key] = Val{e.Until, e.Unreach}
	}
	s.Reindex()
}

// Postponable reports job kinds whose givers skip postponed targets.
func Postponable(k job.Kind) bool {
	switch k {
	case job.Haul, job.FillHopper, job.Construct, job.Deconstruct, job.Harvest, job.Cut, job.Mine, job.Smooth, job.Sow,
		job.Bill, job.Hunt, job.Refuel, job.Repair, job.Research, job.OperateDrill, job.ScanGround, job.ScanMinerals, job.Clean, job.Bury,
		job.Roof, job.Unroof, job.Equip, job.Uninstall, job.Install, job.OpenContainer, // AX-1
		job.StockTank, job.NetTank: // FISH-*
		return true
	}
	return false
}

// KeyOf is the postponement key of pawn id's job j: its work target
// (the bench or fire for bills and refuelling; the frame, trough or load
// once a haul is carrying), else the tile worked on.
func KeyOf(id entity.EntityID, j job.Record) Key {
	if j.Kind.Hauls() {
		j.Kind = job.Haul // FOOD-4: a hopper fill is put off as a haul
	}
	target := j.Target
	switch j.Kind {
	case job.Refuel, job.Bill:
		target = j.Dest
	case job.Haul:
		if j.Carry != 0 {
			target = j.Carry
			if j.Dest != 0 {
				target = j.Dest
			}
		}
	case job.Bury:
		if j.Carry != 0 {
			target = j.Carry
		}
	}
	if target != 0 {
		return Key{Pawn: id, Kind: j.Kind, Target: target}
	}
	switch j.Kind {
	case job.Mine, job.Smooth, job.Clean, job.Roof, job.Unroof, job.Cut:
		return Key{Pawn: id, Kind: j.Kind, X: j.DestX, Y: j.DestY}
	}
	return Key{Pawn: id, Kind: j.Kind, X: j.TX, Y: j.TY}
}

// SortEntries orders out by key.
func SortEntries(out []Entry) {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Key, out[j].Key
		if a.Pawn != b.Pawn {
			return a.Pawn < b.Pawn
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Y < b.Y
	})
}

// Top is one pawn's first ViewLines postponed entries in view order:
// soonest retry first, ties by key (SortEntries' order).
type Top struct {
	Pawn entity.EntityID
	N    int
	E    [ViewLines]Entry
}

// ViewBefore is the view order of postponed entries: retry time, then
// key (the order a key sort followed by a stable sort on Until gives).
func ViewBefore(a, b *Entry) bool {
	if a.Until != b.Until {
		return a.Until < b.Until
	}
	x, y := a.Key, b.Key
	if x.Pawn != y.Pawn {
		return x.Pawn < y.Pawn
	}
	if x.Kind != y.Kind {
		return x.Kind < y.Kind
	}
	if x.Target != y.Target {
		return x.Target < y.Target
	}
	if x.X != y.X {
		return x.X < y.X
	}
	return x.Y < y.Y
}

// Add inserts e into the sorted list, dropping the last entry when full.
func (t *Top) Add(e Entry) {
	i := t.N
	for i > 0 && ViewBefore(&e, &t.E[i-1]) {
		i--
	}
	if i >= ViewLines {
		return
	}
	if t.N < ViewLines {
		t.N++
	}
	copy(t.E[i+1:t.N], t.E[i:t.N-1])
	t.E[i] = e
}
