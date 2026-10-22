// Package filth is tracked-in dirt: what colonists pick up on their boots
// outdoors, shed on indoor floors and wipe off on entrance carpets, and
// how dirty a room is (SIM-021). Pure integer math, no state.
package filth

import "rimworld-go/internal/domain/economy/item"

const (
	// Full caps both a tile's dirt and the dirt one pawn carries.
	Full = 1000
	// StepGain is picked up per step onto bare outdoor soil or gravel;
	// grass halves it (GrassPct).
	StepGain = 6
	GrassPct = 50
	// WorkGain is picked up every WorkEvery ticks of outdoor field work
	// (sowing, harvesting, cutting, mining, digging).
	WorkGain  = 12
	WorkEvery = 30
	// ShedPct of carried dirt falls off per step on clean outdoor ground
	// (stone, built floors).
	ShedPct = 3
	// DepositPct of carried dirt is left on each indoor tile walked,
	// at least 1, once a pawn carries DepositMin or more.
	DepositPct = 8
	DepositMin = 10
	// MatSoilPct of what a carpet wipes off stays on the carpet's tile.
	MatSoilPct = 15
	// CleanStart is the dirt plus stain on a Home tile that queues it for
	// cleaning; a queued tile is cleaned all the way to 0 (hysteresis).
	CleanStart = 120
	// CleanPerTick is removed per tick of cleaning work.
	CleanPerTick = 20
	// MaxPenalty is the most impressiveness a filthy room loses.
	MaxPenalty = 10
	// penaltyAvg is the average filth per cell that costs MaxPenalty.
	penaltyAvg = 200
)

// Gain is the dirt picked up from base at map wetness wetPm (0..1000,
// rain triples it at full wetness), halved on grass.
func Gain(base, wetPm int, grass bool) int {
	wetPm = max(0, min(1000, wetPm))
	g := base * (1000 + 2*wetPm) / 1000
	if grass {
		g = g * GrassPct / 100
	}
	return g
}

// Carry adds gain to carried dirt c, capped at Full.
func Carry(c, gain int) int { return max(0, min(Full, c+gain)) }

// Shed is carried dirt c after a step on clean outdoor ground.
func Shed(c int) int { return c - c*ShedPct/100 }

// Deposit splits carried dirt c on an indoor step: keep stays on the
// boots, drop lands on the tile. Below DepositMin nothing falls.
func Deposit(c int) (keep, drop int) {
	if c < DepositMin {
		return c, 0
	}
	drop = max(1, c*DepositPct/100)
	return c - drop, drop
}

// Wipe is carried dirt c after stepping on a carpet catching catchPm
// (permille): keep stays on the boots, soil is left on the carpet tile.
func Wipe(c, catchPm int) (keep, soil int) {
	caught := c * max(0, min(1000, catchPm)) / 1000
	return c - caught, caught * MatSoilPct / 100
}

// Catch is the share of tracked dirt (permille) a carpet of kind k wipes
// off: dense cloth pile best, wood slats worst. Zero for non-carpets.
func Catch(k item.Kind) int {
	switch k {
	case item.CarpetCloth, item.CarpetWool:
		return 700
	case item.CarpetLeather:
		return 550
	case item.CarpetStraw:
		return 450
	case item.CarpetWood:
		return 300
	}
	return 0
}

// Penalty is the impressiveness a room of cells tiles loses to sum filth
// (dirt plus stains), 0..MaxPenalty.
func Penalty(sum, cells int) int {
	if sum <= 0 || cells <= 0 {
		return 0
	}
	return min(MaxPenalty, sum*MaxPenalty/(cells*penaltyAvg))
}

// Label names a room's cleanliness from its average filth per cell.
func Label(avg int) string {
	switch {
	case avg < 15:
		return "clean"
	case avg < 60:
		return "slightly dirty"
	case avg < 150:
		return "dirty"
	}
	return "filthy"
}

// CleanStep is one tick of cleaning a tile holding dirt and stain:
// CleanPerTick comes off, dirt first.
func CleanStep(dirt, stain int) (int, int) {
	left := CleanPerTick
	d := min(dirt, left)
	dirt, left = dirt-d, left-d
	s := min(stain, left)
	return dirt, stain - s
}
