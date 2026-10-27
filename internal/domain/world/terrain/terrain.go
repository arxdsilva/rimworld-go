package terrain

// Type is a ground type. Rock is natural rock wall: impassable until mined.
type Type uint8

const (
	Soil Type = iota
	RichSoil
	Gravel
	Stone // rough stone ground
	Rock  // natural rock wall
	// Biome ground (SIM-023); append only, saved.
	Sand
	Marsh // wet, slow ground
	Ice   // frozen ground: nothing grows
	// Smoothed stone (SIM-029): a worked rock wall and a worked stone floor.
	RockSmooth
	StoneSmooth
	// Water and wet ground from the landing site (NG-1 §4.3); append only, saved.
	ShallowWater // wadeable, slow, unbuildable
	DeepWater    // impassable
	Mud          // wet, slow-ish ground by rivers and lakes
	Track        // packed dirt road toward the map edge
	numTypes
)

var names = [numTypes]string{"soil", "rich_soil", "gravel", "stone", "rock", "sand", "marsh", "ice", "rock_smooth", "stone_smooth",
	"shallow_water", "deep_water", "mud", "track"}

func (t Type) String() string {
	if t >= numTypes {
		return "unknown"
	}
	return names[t]
}

// Walkable reports whether pawns can stand on t.
func (t Type) Walkable() bool { return !IsRockWall(t) && t != DeepWater }

// IsWater reports shallow or deep water: nothing is built or dropped on it.
func IsWater(t Type) bool { return t == ShallowWater || t == DeepWater }

// Wetness is how wet the ground is in permille (dirt pickup, NG-1 §5.6).
func (t Type) Wetness() int {
	switch t {
	case ShallowWater, DeepWater:
		return 1000
	case Mud:
		return 800
	case Marsh:
		return 600
	}
	return 0
}

// Support is how much a cell can hold up (MAPGEN-3): None (open water),
// Light (a bridge: wooden walls and furniture), Medium, Heavy (firm
// ground: anything).
type Support uint8

const (
	SupportNone Support = iota
	SupportLight
	SupportMedium
	SupportHeavy
)

// GroundSupport is what the terrain itself holds up: nothing on water,
// anything on land (today's building rules on land are unchanged).
func GroundSupport(t Type) Support {
	if IsWater(t) {
		return SupportNone
	}
	return SupportHeavy
}

// Bridgeable reports terrain a bridge foundation may be built over: water,
// marsh and mud (the open sea is refused by the game, not by terrain).
func Bridgeable(t Type) bool {
	return t == ShallowWater || t == DeepWater || t == Marsh || t == Mud
}

// IsRockWall reports a solid natural wall: rough or smoothed rock.
func IsRockWall(t Type) bool { return t == Rock || t == RockSmooth }

// Fertility is the growth-rate multiplier in percent (0 = nothing grows).
func (t Type) Fertility() int {
	switch t {
	case Soil:
		return 100
	case RichSoil:
		return 140
	case Gravel:
		return 70
	case Marsh:
		return 60
	case Sand:
		return 10
	case Mud:
		return 70
	default:
		return 0
	}
}

// GrowSoil reports ground a growing zone may be painted on (PLAY-023):
// soil, rich soil, gravel (stony soil) and marsh (marshy soil). Sand,
// mud, ice, stone, smoothed stone, water and tracks are not soil, even
// where wild plants grow on them.
func (t Type) GrowSoil() bool {
	switch t {
	case Soil, RichSoil, Gravel, Marsh:
		return true
	}
	return false
}

// WalkCost is the time to cross t in percent of plain ground.
func (t Type) WalkCost() int {
	switch t {
	case Marsh:
		return 200
	case Ice:
		return 130
	case Sand:
		return 120
	case ShallowWater:
		return 300
	case Mud:
		return 140
	case Track:
		return 90
	default:
		return 100
	}
}

// Names lists the type names in Type order (read models index by code).
func Names() []string { return names[:] }

// Map is a dense terrain grid, row-major.
type Map struct {
	W, H  int
	Cells []Type
}

// NewMap returns a w x h map of soil.
func NewMap(w, h int) Map {
	return Map{W: w, H: h, Cells: make([]Type, w*h)}
}

// At returns the type at (x, y); out-of-bounds reads as Rock.
func (m Map) At(x, y int) Type {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return Rock
	}
	return m.Cells[y*m.W+x]
}

// Set writes the type at (x, y) if in bounds.
func (m Map) Set(x, y int, t Type) {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return
	}
	m.Cells[y*m.W+x] = t
}
