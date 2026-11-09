// Package cadence spreads the simulation's periodic work over the ticks
// of its period (GAR: the day tick fired every periodic site at once).
// A per-entity site visits entity id on the ticks where
// (now+off+id)%every == 0: every entity keeps exactly its rate, but
// consecutive ids fall on consecutive ticks. The rule reads only the tick
// and the plain sequential id, so it keeps no state and adds nothing to a
// save: a loaded game visits each entity on the same ticks. A site that
// is one pass for the whole map uses id 0 and its offset alone.
//
// It is a leaf package of internal/application/game: deterministic, no
// wall clock, no randomness, imports only the entity ids and iter.
package cadence

import (
	"iter"

	"rimworld-go/internal/domain/sim/entity"
)

// Due reports that id's turn of a site with period every and offset off
// is tick now.
func Due(now, every, off uint64, id entity.EntityID) bool {
	return (now+off+uint64(id))%every == 0
}

// Each yields, in ascending order, exactly the ids in [1, end) that are
// Due at now: one every ticks apart, so a tick costs end/every steps and
// never a scan of the entities.
func Each(now, every, off uint64, end entity.EntityID) iter.Seq[entity.EntityID] {
	return func(yield func(entity.EntityID) bool) {
		first := (every - (now+off)%every) % every
		if first == 0 {
			first = every // 0 is no entity
		}
		for id := entity.EntityID(first); id < end; id += entity.EntityID(every) {
			if !yield(id) {
				return
			}
		}
	}
}

// Next is the first tick at or after now on which id is Due (tests step
// to it).
func Next(now, every, off uint64, id entity.EntityID) uint64 {
	if r := (now + off + uint64(id)) % every; r != 0 {
		return now + every - r
	}
	return now
}
