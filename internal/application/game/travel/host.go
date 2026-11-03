package travel

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

// Small compositions over the host's roles.

// job is pawn id's current job (the zero record when it has none).
func (o core) job(id entity.EntityID) job.Record {
	j, _ := o.h.Job(id)
	return j
}

// settlement describes the settlement on tile t: its name, whether one
// stands there (a faction not yet defeated), hostile, and of a human
// people.
func (o core) settlement(t int) (name string, ok, hostile, human bool) {
	f, name := o.h.Faction(t)
	if f == nil || f.Defeated {
		return "", false, false, false
	}
	return name, true, f.Relation == faction.Hostile, f.Type.Human()
}

// gift gives goods worth value to the settlement on tile t: the goodwill
// it gained (wiki: Transport pod, by market value; faction.GiftGoodwill).
func (o core) gift(t, value int) int {
	f, _ := o.h.Faction(t)
	gw := faction.GiftGoodwill(value)
	if f == nil || gw <= 0 || f.Besieged {
		return 0
	}
	faction.Adjust(f, gw, fmt.Sprintf("Gifts by transport pod (%+d goodwill)", gw))
	return gw
}

// ringAround are the offsets members spread over around a spot.
var ringAround = [...][2]int{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1},
	{2, 0}, {-2, 0}, {0, 2}, {0, -2}}

// moveNear walks drafted pawn id to the i-th free cell around (x, y).
func (o core) moveNear(id entity.EntityID, x, y, i int) {
	d := ringAround[i%len(ringAround)]
	if tx, ty, ok := o.h.WalkableNear(x+d[0], y+d[1], 3); ok {
		o.h.Move(id, tx, ty)
	}
}

// tame is the colony's tame animals, in the host's order.
func (o core) tame() []entity.EntityID {
	var out []entity.EntityID
	for _, id := range o.h.AnimalIDs() {
		if a, ok := o.h.Animal(id); ok && a.Tame {
			out = append(out, id)
		}
	}
	return out
}

// takeAnimal lifts animal id off the map, keeping its state.
func (o core) takeAnimal(id entity.EntityID) (animal.State, bool) {
	a, ok := o.h.Animal(id)
	if ok {
		o.h.RemoveAnimal(id)
	}
	return a, ok
}
