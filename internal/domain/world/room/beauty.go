package room

import "rimworld-go/internal/domain/world/filth"

// Tile beauty (BED-3): what a cell's floor and filth bring to the beauty a
// pawn sees around it, with the same values as the room tally. Buildings
// add room.BuildingBeauty once per building, plants their own value; the
// caller sums those.

// outdoorFilthPct is how much filth outside a room spoils the view
// (wiki: Filth, "outdoors about 30 % of indoors").
const outdoorFilthPct = 30

// FloorBeautyTenths is floor f's beauty in tenths (wiki: Beauty, Floors).
// Natural ground outside a room is 0 rather than the bare floor's -1, so
// open country reads as neutral rather than ugly (original tuning: the
// wiki's -1 is the rough floor of a room).
func FloorBeautyTenths(f Floor, indoors bool) int {
	if int(f) >= len(floorBeauty) || !indoors && f == Bare {
		return 0
	}
	return floorBeauty[f]
}

// FilthBeautyTenths is the beauty, tenths, of filthSum dirt plus stain on
// a cell (filth.Full is one full unit): the room tally's -15 a unit in a
// room, 30 % of that outside one.
func FilthBeautyTenths(filthSum int, indoors bool) int {
	b := filthSum * filthBeautyTenths / filth.Full
	if !indoors {
		b = b * outdoorFilthPct / 100
	}
	return b
}
