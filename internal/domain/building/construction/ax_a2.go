package construction

import "rimworld-go/internal/domain/economy/item"

// plannedA2 is A2's kinds not live yet (structure, furniture and decor;
// AX-0 contract). This file is A2's, and so are the predicates below.
// Every A2 kind is live since AX-2 (.cursor/handoffs/AX-2-structure-furniture.md).
func plannedA2(Kind) bool { return false }

// IsTable reports a dining table of any size (wiki: Table 1x2, 2x2, 2x4,
// 3x3).
func (k Kind) IsTable() bool { return k == Table || k == Table2x2 || k == Table2x4 || k == Table3x3 }

// Seats is how many diners or sitters a piece serves (A2): a table one
// per cell along its sides, corners excluded (wiki: Table, 6/8/12/12), a
// couch two (wiki: Couch), any other seat one, everything else none.
func Seats(k Kind) int {
	if k.IsTable() {
		d := DefOf(k)
		return 2 * (max(d.Size[0], 1) + max(d.Size[1], 1))
	}
	switch k {
	case Couch:
		return 2
	case Chair, Stool, Armchair:
		return 1
	}
	return 0
}

// SeatCells is where the diners of table k (facing f, top-left at x, y)
// sit: every cell orthogonally beside its footprint and outside it, in
// a fixed order (north edge, then south, west and east, each left to
// right or top to bottom). Not a table: nil.
func SeatCells(k Kind, f Facing, x, y int) [][2]int {
	if !k.IsTable() {
		return nil
	}
	cells, _, _ := Footprint(k, f, x, y)
	x0, y0, x1, y1 := cells[0][0], cells[0][1], cells[0][0], cells[0][1]
	for _, c := range cells {
		x0, y0, x1, y1 = min(x0, c[0]), min(y0, c[1]), max(x1, c[0]), max(y1, c[1])
	}
	out := make([][2]int, 0, Seats(k))
	for cx := x0; cx <= x1; cx++ {
		out = append(out, [2]int{cx, y0 - 1})
	}
	for cx := x0; cx <= x1; cx++ {
		out = append(out, [2]int{cx, y1 + 1})
	}
	for cy := y0; cy <= y1; cy++ {
		out = append(out, [2]int{x0 - 1, cy})
	}
	for cy := y0; cy <= y1; cy++ {
		out = append(out, [2]int{x1 + 1, cy})
	}
	return out
}

// WallHung reports a light hung on a wall (wiki: Wall torch lamp, Wall
// lamp): it stands on the cell in front of a wall, facing away from it,
// and blocks nothing.
func (k Kind) WallHung() bool { return k == WallTorch || k == WallLamp }

// HoldsRoof reports a piece that holds a roof up like a wall without
// bounding a room (wiki: Column).
func (k Kind) HoldsRoof() bool { return k == Column }

// Behind is the cell a wall-hung piece facing f at (x, y) hangs on: the
// one opposite its facing (a south-facing lamp hangs on the wall north of
// it).
func Behind(f Facing, x, y int) (int, int) {
	switch f {
	case North:
		return x, y + 1
	case East:
		return x - 1, y
	case West:
		return x + 1, y
	}
	return x, y - 1
}

// PreciousWalls are the extra wall materials (wiki: Walls): silver, gold,
// plasteel, uranium and jade.
var PreciousWalls = [...]item.Kind{item.Silver, item.Gold, item.Plasteel, item.Uranium, item.Jade}

// IsPreciousWall reports one of the extra wall materials.
func IsPreciousWall(s item.Kind) bool {
	for _, p := range PreciousWalls {
		if p == s {
			return true
		}
	}
	return false
}

// smallVolume reports a material sold and used by the small unit (wiki:
// Walls: silver, gold and jade cost ten times the stuff).
func smallVolume(s item.Kind) bool { return s == item.Silver || s == item.Gold || s == item.Jade }

// CostFor is the material a frame of k in s needs: the def's, ten times
// over for a wall of a small-volume material (wiki: Walls, 5 stuff = 50
// silver).
func CostFor(k Kind, s item.Kind) int {
	if k == Wall && smallVolume(s) {
		return WoodCost(k) * 10
	}
	return WoodCost(k)
}

// RefundFor is what deconstructing k built of s returns (Refund, ten
// times over for a small-volume wall).
func RefundFor(k Kind, s item.Kind) int {
	if k == Wall && smallVolume(s) {
		return Refund(k) * 10
	}
	return Refund(k)
}

// preciousStats is the extra wall materials' hit point and work percent
// (wiki: Walls, each material's HP and work against wood's, 195 HP and
// 1.58 s; steel keeps its legacy 140/90).
func preciousStats(s item.Kind) (hpPct, workPct int, ok bool) {
	switch s {
	case item.Plasteel:
		return 430, 198, true
	case item.Uranium:
		return 385, 171, true
	case item.Silver:
		return 110, 90, true
	case item.Gold:
		return 90, 81, true
	case item.Jade:
		return 75, 450, true
	}
	return 0, 0, false
}

// PotPlant is a plant pot's decorative plant (Building.Plant; 0 none).
type PotPlant uint8

// Pot plants (wiki: Plant pot, Rose, Daylily). Append only: saved.
const (
	PotNone PotPlant = iota
	PotRose
	PotDaylily
	numPotPlants
)

// PotPlantDef is a pot plant's grow days, beauty (whole points) and the
// grower's work to sow it (ticks).
type PotPlantDef struct {
	Name     string
	GrowDays float64
	Beauty   int
	SowWork  int
}

var potPlants = [numPotPlants]PotPlantDef{
	PotRose:    {Name: "rose", GrowDays: 5, Beauty: 14, SowWork: 750},    // wiki: Rose
	PotDaylily: {Name: "daylily", GrowDays: 3, Beauty: 18, SowWork: 540}, // wiki: Daylily
}

// DefOf is pot plant p's definition (zero for none or unknown).
func (p PotPlant) DefOf() PotPlantDef {
	if p >= numPotPlants {
		return PotPlantDef{}
	}
	return potPlants[p]
}

// Valid reports a real pot plant (not none).
func (p PotPlant) Valid() bool { return p > PotNone && p < numPotPlants }

// ParsePotPlant is the pot plant named name.
func ParsePotPlant(name string) (PotPlant, bool) {
	for p := PotRose; p < numPotPlants; p++ {
		if potPlants[p].Name == name {
			return p, true
		}
	}
	return PotNone, false
}

// PotPlants lists every pot plant, in order.
func PotPlants() []PotPlant {
	out := make([]PotPlant, 0, numPotPlants-1)
	for p := PotRose; p < numPotPlants; p++ {
		out = append(out, p)
	}
	return out
}

// PotBeauty is plant pot b's beauty in thousandths: its plant's beauty
// times its growth (original tuning: a seedling adds little, a grown
// plant its full beauty).
func PotBeauty(b Building) int {
	if b.Kind != PlantPot {
		return 0
	}
	return PotPlant(b.Plant).DefOf().Beauty * int(b.Growth)
}

// DoorKinds are every door kind (IsDoor), for building cell sets.
var DoorKinds = [...]Kind{Door, Autodoor, OrnateDoor, AnimalFlap}
