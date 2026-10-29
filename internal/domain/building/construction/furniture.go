package construction

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// Bed and bed-furniture stats (BED-1, ENG-003: published wiki numbers).
// Every value is permille: comfort and rest effectiveness 1000 = 1.0,
// beauty in thousandths of a beauty point. The tables hold the Normal
// quality value; quality scales it (economy/quality.Factor).

// Furniture is a piece's comfort, rest effectiveness and beauty.
type Furniture struct {
	Comfort, RestEffectiveness, Beauty int
}

// GroundRestEffectiveness is how well a pawn rests lying on the ground
// with no bed (the sleeping spot's value, which is the bare floor's).
const GroundRestEffectiveness = 800

// StoneRestPct is the rest effectiveness a bed of stone keeps (wiki:
// Double bed, "stone gives 90% of nonstone").
const StoneRestPct = 90

// DresserReach is how far (tiles, centre to centre) a dresser serves a
// bed (wiki: Dresser).
const DresserReach = 6

// furniture is the Normal-quality table. wiki: Sleeping spot, Bedroll,
// Double bedroll, Bed, Double bed, Hospital bed, Animal sleeping spot,
// Animal bed, End table, Dresser. Spots' beauty is not published: 0.
var furniture = [kindCount]Furniture{
	SleepingSpot:       {Comfort: 400, RestEffectiveness: 800},
	Bedroll:            {Comfort: 680, RestEffectiveness: 950},
	DoubleBedroll:      {Comfort: 680, RestEffectiveness: 950},
	Bed:                {Comfort: 750, RestEffectiveness: 1000, Beauty: 1000},
	DoubleBed:          {Comfort: 750, RestEffectiveness: 1000, Beauty: 2000},
	HospitalBed:        {Comfort: 800, RestEffectiveness: 1000},
	AnimalSleepingSpot: {RestEffectiveness: 800},
	AnimalBed:          {Comfort: 750, RestEffectiveness: 1000},
	EndTable:           {Beauty: 3000},
	Dresser:            {Beauty: 5000},
	// BED-3 seats, wiki: Dining chair (Chair), Stool, Armchair.
	Chair:    {Comfort: 700, Beauty: 8000},
	Stool:    {Comfort: 500},
	Armchair: {Comfort: 800, Beauty: 4000},
	// PRD-3 sculptures, wiki: Sculpture (base beauty 50/100/400).
	SmallSculpture: {Beauty: 50000},
	LargeSculpture: {Beauty: 100000},
	GrandSculpture: {Beauty: 400000},
}

// facilityComfort is an end table's or dresser's comfort offset by the
// bed's quality, Awful..Legendary (wiki: End table, Dresser).
var facilityComfort = [...]int{40, 40, 50, 60, 60, 70, 90}

// FurnitureOf is kind k's stats built from stuff at quality q: quality
// scales comfort, rest effectiveness and beauty (a spot is never crafted,
// so it has none); a stone bed rests StoneRestPct as well. Kinds outside
// the table are all zero.
func FurnitureOf(k Kind, stuff item.Kind, q quality.Level) Furniture {
	if k >= kindCount || furniture[k] == (Furniture{}) {
		return Furniture{}
	}
	f := furniture[k]
	if !k.Instant() {
		f.Comfort = quality.Apply(f.Comfort, q, quality.Comfort)
		f.RestEffectiveness = quality.Apply(f.RestEffectiveness, q, quality.RestEffectiveness)
		f.Beauty = quality.Apply(f.Beauty, q, quality.Beauty)
	}
	if item.IsStoneBlocks(stuff) {
		f.RestEffectiveness = f.RestEffectiveness * StoneRestPct / 100
	}
	return f
}

// FacilityComfort is the comfort one end table or one dresser adds to a
// bed of quality q.
func FacilityComfort(q quality.Level) int {
	if !q.Valid() {
		q = quality.Default
	}
	return facilityComfort[q]
}

// IsFacility reports a piece that links to a nearby bed (end table,
// dresser).
func (k Kind) IsFacility() bool { return k == EndTable || k == Dresser }

// SmallFootprint is Footprint's cells for a kind of at most four tiles
// (every bed and bed furniture), in the same order, without allocating;
// n is 0 for a bigger kind.
func SmallFootprint(k Kind, f Facing, x, y int) (cells [4][2]int, n int) {
	d := DefOf(k)
	w, h := max(d.Size[0], 1), max(d.Size[1], 1)
	if w*h > len(cells) {
		return cells, 0
	}
	if !d.Rotatable {
		f = South
	}
	minX, minY := 1<<30, 1<<30
	for cy := range h {
		for cx := range w {
			rx, ry := rotate(f, cx, cy)
			cells[n] = [2]int{rx, ry}
			minX, minY = min(minX, rx), min(minY, ry)
			n++
		}
	}
	for i := range n {
		cells[i] = [2]int{x + cells[i][0] - minX, y + cells[i][1] - minY}
	}
	return cells, n
}
