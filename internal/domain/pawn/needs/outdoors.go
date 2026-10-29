package needs

// Outdoors (BED-3, ENG-003 wiki numbers; wiki: Outdoors). The need fills
// under the open sky and drains under a roof, fastest under overhead
// rock; in bed every loss is a fifth as fast.

// Place is where a pawn is, for the outdoors need.
type Place uint8

const (
	// OutsideOpen: not in a room, no roof.
	OutsideOpen Place = iota
	// OutsideRoofed: not in a room, under a built or thin rock roof.
	OutsideRoofed
	// OutsideMountain: not in a room, under thick overhead rock.
	OutsideMountain
	// IndoorOpen: in a room, on an unroofed cell.
	IndoorOpen
	// IndoorRoofed: in a room, under a built or thin rock roof.
	IndoorRoofed
	// IndoorMountain: in a room, under thick overhead rock.
	IndoorMountain
)

// PlaceOf is the Place of a cell in a room or not, roofed or not, under
// thick rock or not.
func PlaceOf(indoors, roofed, thick bool) Place {
	p := OutsideOpen
	switch {
	case thick:
		p = OutsideMountain
	case roofed:
		p = OutsideRoofed
	}
	if indoors {
		p += IndoorOpen
	}
	return p
}

// outdoorsRate is the change per StepTicks by Place, millionths (wiki:
// outside +2.0 %, +0.25 % roofed, -0.1 % under a mountain; indoors +1.25 %
// unroofed, -0.08 % roofed, -0.1125 % under a mountain).
var outdoorsRate = [...]int32{
	OutsideOpen: 20_000, OutsideRoofed: 2_500, OutsideMountain: -1_000,
	IndoorOpen: 12_500, IndoorRoofed: -800, IndoorMountain: -1_125,
}

// InBedLossPct is how much of a loss applies in bed (wiki: x0.2).
const InBedLossPct = 20

// OutdoorsStart is a new pawn's outdoors need (original tuning: full).
const OutdoorsStart = LevelFull

// OutdoorsStep is one step of the outdoors need at level in place p.
func OutdoorsStep(level int32, p Place, inBed bool) int32 {
	d := outdoorsRate[p]
	if d < 0 && inBed {
		d = d * InBedLossPct / 100
	}
	return clampLevel(level + d)
}

// OutdoorsLadder is the outdoors thoughts: none above 80 %, then -1, -3,
// -5 and -7 down to 60, 40, 20 and 0.05 %, and -9 at or below 0.05 %
// (wiki: Outdoors; the labels are original).
var OutdoorsLadder = Ladder{
	Up:    []int32{LevelPct / 20, 20 * LevelPct, 40 * LevelPct, 60 * LevelPct, 80 * LevelPct},
	Value: []int{-9, -7, -5, -3, -1, 0},
	Label: []string{"buried away from the sky", "shut in for days", "cabin fever", "cooped up", "missing the open air", ""},
}
