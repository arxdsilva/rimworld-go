package game

import "rimworld-go/internal/domain/sim/entity"

// Tracked-in dirt (SIM-021): pawns pick dirt up on outdoor soil and field
// work, leave a trail of it indoors, and wipe it off on entrance carpets.
// Indoor dirt never fades; colonists clean it inside the Home area
// (clean.go). Rooms lose impressiveness to it (rooms.go).

// DirtView is the dirt on one tile.
type DirtView struct {
	X, Y   int
	Amount float64 // 0..1 of filth.Full
}

// dirtCell is one dirty tile in a save.
type dirtCell struct {
	X, Y   int
	Amount uint16
}

// filthState is the dirt layer and its indexes.
type filthState struct {
	cells   []uint16                   // dirt per tile, row-major
	nz      []int32                    // sorted indexes of dirty tiles
	carried map[entity.EntityID]uint16 // dirt on each pawn's boots
	roomSum []int                      // dirt plus stain per room id
	tread   []uint8                    // per tile: 0x80 floored, low 7 bits carpet catch percent
	cleanQ  []int32                    // sorted Home tiles queued for cleaning
}

// carriedPct is the dirt on id's boots, percent of filth.Full.
func (g *Game) carriedPct(id entity.EntityID) int { return 0 } // SIM-021 stub

// fillDirtView lists the dirty tiles, sorted y then x.
func (g *Game) fillDirtView(w *WorldView) {} // SIM-021 stub

// exportDirt and importDirt save and restore the dirt layer.
func (g *Game) exportDirt() ([]dirtCell, map[entity.EntityID]uint16) { return nil, nil } // SIM-021 stub

func (g *Game) importDirt(cells []dirtCell, carried map[entity.EntityID]uint16) {} // SIM-021 stub
