package craft

import (
	"fmt"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Quality rolls the quality of what colonists make (PRD-2, wiki:
// Quality): apparel and weapons at a bench (item.HasQuality), furniture
// and beds when built (QualityBuilding). The roll comes from the maker's
// skill and the host's craft stream; a maker with inspired creativity
// spends it on the roll, two levels up (Legendary reachable).
type Quality struct{ h Host }

// AnnounceFrom is the lowest level whose making is announced.
const AnnounceFrom = quality.Masterwork

// QualityBuilding reports a building kind made with a quality roll
// (wiki: Quality: furniture, beds and joy objects; walls, doors,
// benches, spots and machines have none).
func QualityBuilding(k construction.Kind) bool {
	switch k {
	case construction.Bed, construction.DoubleBed, construction.HospitalBed, construction.Bedroll,
		construction.DoubleBedroll, construction.AnimalBed, construction.EndTable, construction.Dresser,
		construction.Table, construction.Chair, construction.HorseshoePin:
		return true
	}
	return false
}

// Roll is the quality maker id turns out with skill sk: inspired when it
// spends a creativity inspiration on it.
func (q *Quality) Roll(id entity.EntityID, sk skill.ID) quality.Level {
	h := q.h
	lvl := int(h.SkillLevel(id, sk))
	inspired := h.UseInspiration(id)
	return quality.Roll(lvl, uint32(h.Roll(1<<32)), inspired)
}

// Built is the stored quality of a building of kind k that builder id
// just finished (0: the kind has none), announced when it is masterwork
// or better.
func (q *Quality) Built(id entity.EntityID, k construction.Kind, x, y int) uint8 {
	if !QualityBuilding(k) {
		return 0
	}
	l := q.Roll(id, skill.Construction)
	q.announce(id, l, construction.DefOf(k).Name, x, y)
	return quality.Stored(l)
}

// Product is one unit of recipe d's output that crafter id made from
// material stuff (the carried ingredient's kind): its quality rolled and
// its material kept when the output has a condition (item.HasQuality).
// ok is false for any other output (dropped as plain stacks).
func (q *Quality) Product(id entity.EntityID, d recipe.Def, stuff item.Kind, x, y int) (item.Stack, bool) {
	if !item.HasQuality(d.Output) {
		return item.Stack{}, false
	}
	st := item.Stack{Kind: d.Output, Qty: 1, X: x, Y: y}
	if d.Stuffed() && stuff.StuffCategory() != 0 {
		st.Stuff = stuff
	}
	l := q.Roll(id, d.Learn)
	st.Quality = quality.Stored(l)
	q.announce(id, l, d.Output.String(), x, y)
	return st, true
}

// announce posts a message for a masterwork or legendary piece.
func (q *Quality) announce(id entity.EntityID, l quality.Level, what string, x, y int) {
	if l < AnnounceFrom {
		return
	}
	who := q.h.Name(id)
	if who == "" {
		who = "Someone"
	}
	q.h.Message(x, y, fmt.Sprintf("%s made a %s %s.", who, l, what))
}
