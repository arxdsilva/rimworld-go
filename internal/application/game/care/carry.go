package care

import (
	"maps"
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// Carries is who carries whom: a two-way index of carrier → patient and
// patient → carrier, so both questions are one lookup. A carrier holds at
// most one patient and a patient has at most one carrier. Carriers are
// kept in ascending order so Each is deterministic without sorting on the
// tick path.
type Carries struct {
	byCarrier map[entity.EntityID]entity.EntityID
	byPatient map[entity.EntityID]entity.EntityID
	order     []entity.EntityID // carriers, ascending
}

// NewCarries is an empty carry index.
func NewCarries() *Carries {
	return &Carries{
		byCarrier: make(map[entity.EntityID]entity.EntityID),
		byPatient: make(map[entity.EntityID]entity.EntityID),
	}
}

// Pick makes carrier c carry patient p. Whatever c held before is put
// down, and p's previous carrier, if any, lets go of it.
func (cs *Carries) Pick(c, p entity.EntityID) {
	if c == 0 || p == 0 {
		return
	}
	cs.Drop(c)
	if prev, ok := cs.byPatient[p]; ok {
		cs.Drop(prev)
	}
	cs.byCarrier[c] = p
	cs.byPatient[p] = c
	i, _ := slices.BinarySearch(cs.order, c)
	cs.order = slices.Insert(cs.order, i, c)
}

// Drop makes carrier c let go; it returns the patient it held.
func (cs *Carries) Drop(c entity.EntityID) (entity.EntityID, bool) {
	p, ok := cs.byCarrier[c]
	if !ok {
		return 0, false
	}
	delete(cs.byCarrier, c)
	delete(cs.byPatient, p)
	if i, found := slices.BinarySearch(cs.order, c); found {
		cs.order = slices.Delete(cs.order, i, i+1)
	}
	return p, true
}

// Patient is the pawn carrier c holds (0 when empty-handed).
func (cs *Carries) Patient(c entity.EntityID) entity.EntityID { return cs.byCarrier[c] }

// Carrier is the pawn carrying patient p (0 when not carried).
func (cs *Carries) Carrier(p entity.EntityID) entity.EntityID { return cs.byPatient[p] }

// Forget removes id from the index on both sides: as a carrier its
// patient is let go, as a patient its carrier lets go.
func (cs *Carries) Forget(id entity.EntityID) {
	if c, ok := cs.byPatient[id]; ok {
		cs.Drop(c)
	}
	cs.Drop(id)
}

// Len is the number of pawns being carried.
func (cs *Carries) Len() int { return len(cs.order) }

// Each calls fn for every carrier and its patient in ascending carrier
// order. fn must not change the index.
func (cs *Carries) Each(fn func(carrier, patient entity.EntityID)) {
	for _, c := range cs.order {
		fn(c, cs.byCarrier[c])
	}
}

// Export is the carrier → patient map for the save (a copy).
func (cs *Carries) Export() map[entity.EntityID]entity.EntityID {
	return maps.Clone(cs.byCarrier)
}

// Import replaces the index with a saved carrier → patient map. A nil map
// (a save without the field) keeps the index as it is. Carriers are
// applied in ascending order, so a corrupt save naming one patient twice
// loads the same way every time.
func (cs *Carries) Import(m map[entity.EntityID]entity.EntityID) {
	if m == nil {
		return
	}
	cs.byCarrier = make(map[entity.EntityID]entity.EntityID, len(m))
	cs.byPatient = make(map[entity.EntityID]entity.EntityID, len(m))
	cs.order = cs.order[:0]
	for _, c := range slices.Sorted(maps.Keys(m)) {
		cs.Pick(c, m[c])
	}
}
