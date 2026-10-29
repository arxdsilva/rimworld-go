package room

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
)

// What a room's buildings and floors bring to its vanilla statistics
// (BED-2), summed once per room rebuild. Items and filth change all the
// time, so the game adds them when it reads the stats (Tally.Inputs).

// Floor is what covers a room cell, for its beauty and cleanliness.
type Floor uint8

const (
	// Bare is natural ground: soil, sand, rough stone and the like.
	Bare Floor = iota
	// Smooth is a worked stone floor (no floor building on it).
	Smooth
	WoodFloor
	StoneTile
	CarpetFloor
)

// floorBeauty (tenths) and floorClean (hundredths) per Floor. wiki: Beauty
// (bare ground -1), Floors (wood 0, stone tile +1, carpet +2); Room
// (natural ground cleanliness -1, built floors 0). The smoothed stone
// floor is not published: original tuning, as a stone tile.
var (
	floorBeauty = [...]int{Bare: -10, Smooth: 10, WoodFloor: 0, StoneTile: 10, CarpetFloor: 20}
	floorClean  = [...]int{Bare: -100, CarpetFloor: 0}
)

// Filth per filth.Full of dirt plus stain on a cell. wiki: Filth (dirt
// -15 beauty indoors) and Room (dirt -5 cleanliness). The Beauty page's
// dirt -5 conflicts with the Filth page: the Filth page wins (it is the
// filth's own page). The game's room filth sum does not split dirt from
// stains, so stains count as dirt.
const (
	filthBeautyTenths      = -150
	filthCleanHundredths   = -500
	buildingValueWorkRatio = 20 // work ticks per silver (colony Wealth's building rule)
)

// FloorUnder is the Floor beneath a carpet: the floor building's (stuff,
// when floored), else smooth or bare ground.
func FloorUnder(stuff item.Kind, floored, smooth bool) Floor {
	switch {
	case floored:
		return FloorOf(stuff)
	case smooth:
		return Smooth
	}
	return Bare
}

// FloorOf is the Floor a floor building of stuff makes.
func FloorOf(stuff item.Kind) Floor {
	if item.IsStoneBlocks(stuff) || stuff == item.Stone {
		return StoneTile
	}
	return WoodFloor
}

// Tally sums the static part of a room's stats: blocking furniture cells,
// beauty, wealth and cleanliness of the buildings and floor buildings,
// and the furniture the new roles count. Comparable (the rooms self-check
// compares it).
type Tally struct {
	Impassable           int // cells taken by furniture that cuts space
	Covered              int // cells with a floor building
	BeautyTenths, Wealth int
	CleanHundredths      int // sum over covered cells
	RecBuildings, Stoves int
}

// AddBuilding adds an object-layer building of kind k made of stuff at
// quality q, covering cells room cells.
func (t *Tally) AddBuilding(k construction.Kind, stuff item.Kind, q quality.Level, cells int) {
	t.BeautyTenths += BuildingBeauty(k, stuff, q) / 100
	t.Wealth += BuildingValue(k, stuff, q)
	if cutsSpace(k) {
		t.Impassable += cells
	}
	switch k {
	case construction.HorseshoePin:
		t.RecBuildings++
	case construction.Stove, construction.ElectricStove:
		t.Stoves++
	}
}

// AddFloor adds a floor building of stuff.
func (t *Tally) AddFloor(stuff item.Kind) {
	f := FloorOf(stuff)
	t.Covered++
	t.BeautyTenths += floorBeauty[f]
	t.CleanHundredths += floorClean[f]
	t.Wealth += BuildingValue(construction.Floor, stuff, quality.Default)
}

// AddCarpet adds a carpet laid over a cell whose floor is under; Bare
// and Smooth cells are counted by Inputs, so only the change shows here.
func (t *Tally) AddCarpet(under Floor, stuff item.Kind) {
	t.BeautyTenths += floorBeauty[CarpetFloor] - floorBeauty[under]
	t.CleanHundredths += floorClean[CarpetFloor] - floorClean[under]
	t.Wealth += BuildingValue(construction.Carpet, stuff, quality.Default)
}

// Inputs completes the tally for a room of cells cells, smooth of them
// worked stone with no floor building, holding filthSum dirt plus stain
// and items worth itemWealth.
func (t Tally) Inputs(cells, smooth, filthSum, itemWealth int) Inputs {
	bare := max(0, cells-t.Covered-smooth)
	beauty := t.BeautyTenths + bare*floorBeauty[Bare] + smooth*floorBeauty[Smooth] + filthSum*filthBeautyTenths/filth.Full
	clean := t.CleanHundredths + bare*floorClean[Bare] + filthSum*filthCleanHundredths/filth.Full
	return Inputs{Cells: cells, Impassable: t.Impassable, BeautySumTenths: beauty,
		Wealth: t.Wealth + itemWealth, CleanlinessHundredths: clean / max(1, cells)}
}

// BuildingBeauty is a building's beauty in thousandths: its kind's (the
// furniture table, scaled by quality) times its material's beauty factor
// plus the material's offset (wiki: Materials; a gold hospital bed is
// 0 x 4 + 20). Kinds outside the furniture table have base 0.
func BuildingBeauty(k construction.Kind, stuff item.Kind, q quality.Level) int {
	b := construction.FurnitureOf(k, stuff, q).Beauty
	if p, ok := item.StuffOf(stuff); ok {
		b = b*p.BeautyFactor/1000 + p.BeautyOffset*1000
	}
	return b
}

// cutsSpace: furniture that takes a cell out of the room's space (beds,
// tables and anything that blocks movement; chairs, spots and lamps do
// not). wiki: Room.
func cutsSpace(k construction.Kind) bool {
	switch {
	case construction.DefOf(k).BlocksMove, k.Sleeps() && !k.Instant(), k == construction.AnimalBed:
		return true
	}
	return k == construction.Table || k == construction.EndTable || k == construction.Dresser
}

// BuildingValue is a building's market value: its material at twice the
// unit count, a silver per buildingValueWorkRatio work ticks, stone half
// again (the colony Wealth rule), times the quality's market-value factor.
func BuildingValue(k construction.Kind, stuff item.Kind, q quality.Level) int {
	v := construction.WoodCost(k)*2 + construction.WorkTicks(k)/buildingValueWorkRatio
	if item.IsStoneBlocks(stuff) || stuff == item.Stone {
		v = v * 3 / 2
	}
	return quality.Apply(v, q, quality.MarketValue)
}

// ItemValue is a stack's market value: item.StackValue (kind, material,
// quality and hit points, PRD-2) per unit times the quantity.
func ItemValue(st item.Stack) int { return item.StackValue(st) * st.Qty }

// Stats is the room's statistics now: c its contents at the rebuild (its
// Floored less the tally's Covered is its smooth stone), filthSum its dirt
// plus stain and itemWealth the items lying in it.
func (t Tally) Stats(c Contents, filthSum, itemWealth int) Stats {
	return StatsOf(t.Inputs(c.Cells, c.Floored-t.Covered, filthSum, itemWealth))
}

// Role is the room's role by RoleOfFull, given its medical and prisoner
// beds (bed flags are read live, not at the rebuild).
func (t Tally) Role(c Contents, medicalBeds, prisonerBeds int) Role {
	return RoleOfFull(FullContents{Contents: c, MedicalBeds: medicalBeds, PrisonerBeds: prisonerBeds,
		RecBuildings: t.RecBuildings, Stoves: t.Stoves})
}

// Add counts building b, covering cells room cells, into a room's
// contents and tally; under is the floor beneath it when it is a carpet.
func Add(c *Contents, t *Tally, b construction.Building, cells int, under Floor) {
	switch q, _ := quality.FromStored(b.Quality); {
	case b.Kind == construction.Floor:
		c.Floored++
		t.AddFloor(b.Stuff)
		return
	case b.Kind == construction.Carpet:
		t.AddCarpet(under, b.Stuff)
		return
	default:
		t.AddBuilding(b.Kind, b.Stuff, q, cells)
	}
	switch k := b.Kind; {
	case k.Sleeps():
		c.Beds++
	case k == construction.Table:
		c.Tables++
	case k == construction.Chair:
		c.Chairs++
	case k == construction.Lamp, k == construction.TorchLamp:
		c.Lamps++ // a light fitting, lit or not
	case construction.DefOf(k).Bench:
		c.Benches++
	}
}

// Fact is what of a room is read live rather than at a rebuild: the
// market value of the items lying in it and its medical and prisoner
// beds (flipping a bed flag rebuilds nothing).
type Fact struct{ Wealth, MedicalBeds, PrisonerBeds int }

// CountFacts fills dst (reused when large enough) with each of rooms
// rooms' facts: one pass over the stacks and the buildings, by the room
// id grid (w wide). Sums do not depend on store order.
func CountFacts(dst []Fact, grid []int, w, rooms int, stacks *component.Store[item.Stack], bldgs *component.Store[construction.Building]) []Fact {
	if cap(dst) < rooms {
		dst = make([]Fact, rooms)
	}
	dst = dst[:rooms]
	clear(dst)
	at := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y*w+x >= len(grid) {
			return Outdoors
		}
		return grid[y*w+x]
	}
	stacks.AnyRef(func(_ entity.EntityID, st *item.Stack) bool {
		if r := at(st.X, st.Y); r >= 0 && r < rooms && st.Carrier == 0 {
			dst[r].Wealth += ItemValue(*st)
		}
		return false
	})
	bldgs.AnyRef(func(_ entity.EntityID, b *construction.Building) bool {
		if r := at(b.X, b.Y); r >= 0 && r < rooms && b.Kind.Sleeps() {
			if b.ForPrisoners { // a prison bed outranks its medical flag
				dst[r].PrisonerBeds++
			} else if b.Medical {
				dst[r].MedicalBeds++
			}
		}
		return false
	})
	return dst
}
