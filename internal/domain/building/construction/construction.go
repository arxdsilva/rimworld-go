// Package construction is buildable definitions, frames (blueprints that
// accept wood then work) and completed buildings.
package construction

import "rimworld-go/internal/domain/economy/item"

// Kind is a buildable structure.
type Kind uint8

const (
	Wall Kind = iota
	Bed
	Door
	Floor
	Table
	Chair
	Lamp
	// SleepingSpot is a bed-sized (1x2, 2x1 on its side) marking on the
	// floor: free, placed instantly, slept on like the ground. The Bed is
	// the built upgrade with the same footprint.
	SleepingSpot
	Workbench
	Stove
	ButcherTable
	// DoubleBed is a 2x2 bed for two sleepers side by side.
	DoubleBed
	// DryingRack is a 2x1 wooden frame food is laid out on to dry.
	DryingRack
	// Campfire burns wood to heat (and light) its room.
	Campfire
	// TailoringBench is a 2x1 table where clothes are sewn.
	TailoringBench
	// HorseshoePin is a stake in a patch of sand for pitching horseshoes.
	HorseshoePin
	// Sandbags are a waist-high line of sacks: walkable, and cover for
	// anyone crouched behind them.
	Sandbags
	// SpikeTrap is a hidden frame of sharpened stakes that springs on
	// hostiles walking over it, then has to be rebuilt.
	SpikeTrap
	// ResearchBench is a 2x1 desk where colonists research.
	ResearchBench
	// Grave is a dug 1x2 plot that holds one body under a headstone;
	// Pyre is a 2x1 log stack where bodies are cremated (SIM-017).
	Grave
	Pyre
	kindCount
)

// Layer separates what can share a tile: one floor plus one object.
type Layer uint8

const (
	LayerObject Layer = iota
	LayerFloor
)

// Def is the static definition of a buildable kind.
type Def struct {
	Kind       Kind
	Name       string
	Wood       int // delivered wood required before work starts
	Work       int // construct ticks after materials are in place
	Layer      Layer
	BlocksMove bool // completed building makes its tile unwalkable
	Rotatable  bool
	StoneOK    bool // may be built from stone blocks instead of wood
	// Size is the footprint (w, h) when facing south; zero means 1x1.
	Size [2]int
	// Spot is the interaction cell relative to the footprint's top-left
	// when facing south (e.g. where a cook stands); HasSpot gates it.
	Spot    [2]int
	HasSpot bool
	Bench   bool // a workstation that runs bills
	// Dug: made by digging, so it needs work but no materials (a grave).
	Dug bool
}

// StoneWorkPct is the work multiplier (percent) for stone construction.
const StoneWorkPct = 160

var defs = [kindCount]Def{
	Wall:         {Kind: Wall, Name: "wall", Wood: 2, Work: 40, Layer: LayerObject, BlocksMove: true, StoneOK: true},
	Bed:          {Kind: Bed, Name: "bed", Wood: 5, Work: 90, Layer: LayerObject, Rotatable: true, Size: [2]int{1, 2}},
	Door:         {Kind: Door, Name: "door", Wood: 2, Work: 50, Layer: LayerObject},
	Floor:        {Kind: Floor, Name: "floor", Wood: 1, Work: 20, Layer: LayerFloor, StoneOK: true},
	Table:        {Kind: Table, Name: "table", Wood: 3, Work: 50, Layer: LayerObject},
	Chair:        {Kind: Chair, Name: "chair", Wood: 2, Work: 40, Layer: LayerObject, Rotatable: true},
	Lamp:         {Kind: Lamp, Name: "lamp", Wood: 2, Work: 40, Layer: LayerObject},
	SleepingSpot: {Kind: SleepingSpot, Name: "sleeping spot", Layer: LayerObject, Rotatable: true, Size: [2]int{1, 2}},
	Workbench: {Kind: Workbench, Name: "workbench", Wood: 6, Work: 160, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	Stove: {Kind: Stove, Name: "stove", Wood: 5, Work: 140, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	DoubleBed: {Kind: DoubleBed, Name: "double bed", Wood: 9, Work: 150, Layer: LayerObject, Rotatable: true,
		Size: [2]int{2, 2}},
	ButcherTable: {Kind: ButcherTable, Name: "butcher table", Wood: 5, Work: 120, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	HorseshoePin: {Kind: HorseshoePin, Name: "horseshoe pin", Wood: 3, Work: 50, Layer: LayerObject},
	TailoringBench: {Kind: TailoringBench, Name: "tailoring bench", Wood: 7, Work: 150, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	Sandbags:  {Kind: Sandbags, Name: "sandbags", Wood: 2, Work: 60, Layer: LayerObject, StoneOK: true},
	SpikeTrap: {Kind: SpikeTrap, Name: "spike trap", Wood: 4, Work: 80, Layer: LayerObject},
	ResearchBench: {Kind: ResearchBench, Name: "research bench", Wood: 8, Work: 180, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true},
	// A campfire is also a (slow) cooking spot; the cook stands south of it.
	Campfire: {Kind: Campfire, Name: "campfire", Wood: 5, Work: 60, Layer: LayerObject, BlocksMove: true,
		Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	DryingRack: {Kind: DryingRack, Name: "drying rack", Wood: 6, Work: 100, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	// A grave is dug, not built: no materials, just a long work.
	Grave: {Kind: Grave, Name: "grave", Work: 400, Layer: LayerObject, Rotatable: true, Size: [2]int{1, 2}, Dug: true},
	Pyre: {Kind: Pyre, Name: "funeral pyre", Wood: 6, Work: 120, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
}

// MaxHP is how much damage a building takes before it is destroyed.
func MaxHP(k Kind, s item.Kind) int {
	hp := 60
	switch k {
	case Wall:
		hp = 160
	case Door:
		hp = 110
	case Sandbags:
		hp = 140
	case SpikeTrap:
		hp = 40
	}
	if s == item.Stone {
		hp = hp * 2
	}
	return hp
}

// CoverPct is how much a building on the shooter's side of a target cuts
// the chance to hit it (percent).
func CoverPct(k Kind) int {
	switch k {
	case Sandbags:
		return 55
	case Wall, Door:
		return 40 // a corner peeked around
	}
	return 0
}

// Sleeps reports whether k is something to sleep in.
func (k Kind) Sleeps() bool { return k == Bed || k == SleepingSpot || k == DoubleBed }

// Sleepers is how many pawns a sleeping place holds side by side.
func (k Kind) Sleepers() int {
	switch {
	case k == DoubleBed:
		return 2
	case k.Sleeps():
		return 1
	}
	return 0
}

// SleepCell is the head-end cell of sleeping slot `slot` (pillow side by
// side across the bed's width; the sleeper's feet point to the facing).
func SleepCell(k Kind, f Facing, x, y, slot int) (int, int) {
	cells, _, _ := Footprint(k, f, x, y)
	if slot < 0 || slot >= len(cells) || slot >= k.Sleepers() {
		slot = 0
	}
	return cells[slot][0], cells[slot][1]
}

// Instant kinds need no materials or work: designating one places it,
// and deconstructing removes it on the spot.
func (k Kind) Instant() bool {
	d := DefOf(k)
	return d.Wood == 0 && d.Work == 0
}

// rotate maps a south-frame point to facing f (screen coords, y down),
// matching the art: east = (x, y) -> (y, -x).
func rotate(f Facing, x, y int) (int, int) {
	switch f {
	case East:
		return y, -x
	case North:
		return -x, -y
	case West:
		return -y, x
	}
	return x, y
}

// Footprint returns the covered cells and the interaction spot (absolute,
// ok=false if none) for kind k with its top-left at (x, y) facing f.
func Footprint(k Kind, f Facing, x, y int) (cells [][2]int, spot [2]int, ok bool) {
	d := DefOf(k)
	w, h := d.Size[0], d.Size[1]
	if w <= 0 || h <= 0 {
		w, h = 1, 1
	}
	if !d.Rotatable {
		f = South
	}
	type pt struct{ x, y int }
	var raw []pt
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			rx, ry := rotate(f, cx, cy)
			raw = append(raw, pt{rx, ry})
		}
	}
	minX, minY := raw[0].x, raw[0].y
	for _, p := range raw {
		if p.x < minX {
			minX = p.x
		}
		if p.y < minY {
			minY = p.y
		}
	}
	for _, p := range raw {
		cells = append(cells, [2]int{x + p.x - minX, y + p.y - minY})
	}
	if d.HasSpot {
		sx, sy := rotate(f, d.Spot[0], d.Spot[1])
		return cells, [2]int{x + sx - minX, y + sy - minY}, true
	}
	return cells, [2]int{}, false
}

// Covers reports whether (px, py) is inside k's footprint at (x, y, f).
func Covers(k Kind, f Facing, x, y, px, py int) bool {
	cells, _, _ := Footprint(k, f, x, y)
	for _, c := range cells {
		if c[0] == px && c[1] == py {
			return true
		}
	}
	return false
}

// DefOf returns the definition for k.
func DefOf(k Kind) Def {
	if k >= kindCount {
		return Def{}
	}
	return defs[k]
}

// All lists every buildable definition in Kind order.
func All() []Def {
	out := make([]Def, 0, kindCount)
	for _, d := range defs {
		out = append(out, d)
	}
	return out
}

// Parse returns the Kind for a definition name.
func Parse(name string) (Kind, bool) {
	for _, d := range defs {
		if d.Name == name {
			return d.Kind, true
		}
	}
	return 0, false
}

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool { return k < kindCount }

func (k Kind) String() string {
	if k >= kindCount {
		return "unknown"
	}
	return defs[k].Name
}

// WoodCost is delivered wood required before construction work.
func WoodCost(k Kind) int { return DefOf(k).Wood }

// WorkTicks is Do-phase labor after materials are delivered.
func WorkTicks(k Kind) int { return DefOf(k).Work }

// DeconstructTicks is the labor to take a building down.
func DeconstructTicks(k Kind) int { return (DefOf(k).Work + 1) / 2 }

// Refund is the material returned by deconstruction (half, rounded down).
func Refund(k Kind) int { return DefOf(k).Wood / 2 }

// StuffOK reports whether k may be built from material s.
func StuffOK(k Kind, s item.Kind) bool {
	switch s {
	case item.Wood:
		return true
	case item.Stone:
		return DefOf(k).StoneOK
	}
	return false
}

// WorkFor is the construct work for k built from s.
func WorkFor(k Kind, s item.Kind) int {
	w := DefOf(k).Work
	if s == item.Stone {
		w = w * StoneWorkPct / 100
	}
	return w
}

// Facing is the orientation of a rotatable building.
type Facing uint8

const (
	South Facing = iota
	East
	North
	West
)

func (f Facing) String() string {
	switch f {
	case East:
		return "east"
	case North:
		return "north"
	case West:
		return "west"
	default:
		return "south"
	}
}

// ParseFacing maps a raw command value to a Facing (unknown -> South).
func ParseFacing(v uint8) Facing {
	if v > uint8(West) {
		return South
	}
	return Facing(v)
}

// Frame is a blueprint that accepts wood then construct work.
type Frame struct {
	Kind     Kind
	Stuff    item.Kind
	Facing   Facing
	X, Y     int
	Wood     int
	Need     int
	WorkLeft int
	// Rebuild marks the blueprint of a smashed building: builders and
	// haulers see to it before anything else.
	Rebuild bool
}

// Ready reports materials are in place.
func (f Frame) Ready() bool { return f.Wood >= f.Need }

// Building is a completed structure.
type Building struct {
	Kind        Kind
	Stuff       item.Kind
	Facing      Facing
	X, Y        int
	Claimant    uint64 // EntityID of the pawn who claimed a bed; 0 if free
	Claimant2   uint64 // second sleeper of a double bed
	Deconstruct bool   // designated for removal
	WorkLeft    int    // remaining deconstruct work once started
	Damage      int    // damage taken (raiders bashing, bullets); repaired by builders
}

// HP is the building's remaining hit points.
func (b Building) HP() int { return max(0, MaxHP(b.Kind, b.Stuff)-b.Damage) }

// SlotOf is the sleeping slot pawn id holds here, or -1.
func (b Building) SlotOf(id uint64) int {
	switch {
	case id == 0:
		return -1
	case b.Claimant == id:
		return 0
	case b.Claimant2 == id && b.Kind.Sleepers() > 1:
		return 1
	}
	return -1
}

// CanClaim reports whether pawn id may sleep here (it already holds a
// slot, or one is free).
func (b Building) CanClaim(id uint64) bool {
	if b.SlotOf(id) >= 0 {
		return true
	}
	return b.Claimant == 0 || (b.Kind.Sleepers() > 1 && b.Claimant2 == 0)
}

// Claim gives pawn id a slot (keeping one it already holds).
func (b Building) Claim(id uint64) Building {
	switch {
	case b.SlotOf(id) >= 0:
	case b.Claimant == 0:
		b.Claimant = id
	case b.Kind.Sleepers() > 1 && b.Claimant2 == 0:
		b.Claimant2 = id
	}
	return b
}

// Release frees pawn id's slot.
func (b Building) Release(id uint64) Building {
	if b.Claimant == id {
		b.Claimant = 0
	}
	if b.Claimant2 == id {
		b.Claimant2 = 0
	}
	return b
}

// Owner is the first claimant that is not id (for "owned by" reasons).
func (b Building) Owner(id uint64) uint64 {
	if b.Claimant != 0 && b.Claimant != id {
		return b.Claimant
	}
	if b.Claimant2 != id {
		return b.Claimant2
	}
	return 0
}
