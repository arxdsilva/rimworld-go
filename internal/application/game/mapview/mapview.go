// Package mapview is the plain read models of the map and its overlays:
// zones, rooms, ground, stains and dirt, recent roof collapses and bursts,
// per-chunk shares of the world view and the minimap, plant growth, fires,
// lightning strikes, crop raids, power nets and aid groups. The game
// fills them and names them game.ZoneView, game.RoomView and so on. The
// ground's names and labels (ground.go), the zone labels (zone.go) and the
// chunk index's buckets and change hashes (buckets.go) are pure helpers
// beside them. No host.
package mapview

// Zone is one zone for the UI: its tiles and settings.
type Zone struct {
	ID    uint16
	Kind  string // "stockpile" | "grow" | "pen"
	Crop  string // grow zones: the species name ("tree" for tree zones)
	Food  bool   // stockpile filters
	Wood  bool
	Stone bool
	Cells [][2]int // row-major
	// The bounding box, for labels and panels.
	MinX, MinY, MaxX, MaxY int
	// PLAY-020 settings: sowing or cutting off, hidden overlay, the
	// player's name ("" = the default).
	NoSow, NoCut, Hidden bool
	Name                 string
	// PLAY-023 storage settings: Priority is the stockpile's priority
	// name ("Critical" .. "Low"; "" for zones without one), Corpses and
	// Chunks a dumping stockpile's filters.
	Priority        string
	Corpses, Chunks bool
	// PRD-4 storage filters (stockpile_filter.go): Filter is the allowed
	// filter-tree nodes, compact (zone.Filter.Names); Special the special
	// filters on and the narrowed ranges (zone.Special.Names). Shelf is a
	// shelf's storage zone's building id (0: no shelf).
	Filter, Special []string
	Shelf           uint64
	Profile         string // NST: the storage furniture's build name; "" for the rest
	// STO/CBT contract (0 until STO-2): storage Group id, Stored stacks of Capacity.
	Group            uint64
	Stored, Capacity int
}

// Room is an enclosed room in the read model. BED-2 stats: Space, Beauty, Impressiveness
// in tenths, CleanStat hundredths, Wealth silver; Score is whole points, Label the stage.
type Room struct {
	ID, Cells                            int
	Role                                 string
	Score                                int
	Label                                string // awful .. wondrously impressive
	Space, Beauty, Wealth, CleanStat     int
	SpaceLabel, BeautyLabel, WealthLabel string
	Impressiveness                       int
	Lit                                  bool // a light shines in it: a powered lamp, a burning torch lamp or campfire
	Temp                                 int  // air temperature, tenths of a degree C
	// Roofed is the roofed share of the room's cells, 0..1 (SIM-018).
	Roofed float64
	// Filth is the average dirt plus stain per cell (0..filth.Full),
	// Cleanliness the CleanStat label and FilthPenalty the impressiveness
	// points the filth costs (already taken off Score) (SIM-021, BED-2).
	Filth        int
	Cleanliness  string
	FilthPenalty int
	// TEMP-1 (game/thermo), the room stats line: wall faces with outdoor air beyond, single and double (half the
	// loss); its sources' net heat (tenths of heat/s); RoofKind "thin", "thick" (mountain), "mixed" or "".
	ExposedWalls, DoubleWalls, HeatRate int
	RoofKind                            string
}

// Soil describes a ground type for the hover readout.
type Soil struct {
	Name      string // terrain code name, e.g. "rich_soil"
	Label     string // display name, e.g. "Rich soil"
	Fertility int    // growth rate in percent; 0 = nothing grows
	Farmable  bool   // growing zones may be placed here
	Walkable  bool
	// WalkCost is the time to cross it, percent of plain ground (marsh 200:
	// "slow ground" in the hover; floors cancel it).
	WalkCost int
	// Smoothable: a builder may smooth it (rough rock without ore, rough
	// stone ground without a building; SIM-029).
	Smoothable bool
}

// PlantGrowth is why a plant is or isn't growing now (PLAY-017), for
// the plant inspector. State is "growing", "resting" (the rest hours),
// "dark" (too little growing light: a sun lamp or the open sky), "cold",
// "hot" or "ripe"; RipeIn the estimated ticks until ripe at the current
// conditions (-1 unknown or not growing). It follows the hour, the
// weather and the lamps rather than the plant, so it is asked for one
// plant on demand instead of riding on every PlantView.
type PlantGrowth struct {
	State  string
	RipeIn int
}

// Stain is one stained tile for presentation.
type Stain struct {
	X, Y     int
	Kind     string  // "blood" or "ichor"
	Amount   float64 // 0..1 strength
	AgeTicks uint64  // since the last fresh blood (drying)
	Variant  uint8
}

// Dirt is the dirt on one tile.
type Dirt struct {
	X, Y   int
	Amount float64 // 0..1 of filth.Full
}

// RoofCollapse is a recent roof collapse, for presentation (dust).
type RoofCollapse struct {
	X, Y int
	Tick uint64
}

// Burst is a recent explosive death, for the renderer's flash and
// scorch (kept an hour, like roof collapses).
type Burst struct {
	X, Y    int
	Tick    uint64
	Species string
	// Turret: a destroyed sentry turret's blast (PLAY-024); Species is "".
	// Blast names an explosion ("bomb", "flame", "EMP", "smoke"; DEF-2)
	// of Radius tenths of a tile; "" otherwise.
	Turret bool
	Blast  string
	Radius int
}

// Chunk is one chunk's share of the current World(): indexes into its
// Plants, Items, Buildings and Frames, ascending (list order), of the
// entries whose anchor tile (X, Y) lies in the chunk. The slices are
// shared and read-only.
//
// Ver changes whenever anything a baked image of the chunk's ground can
// show may have changed: terrain, stone and ore on the chunk and a one-
// tile ring around it (edges and links reach across), three rows south
// (tall rock reaches north), and every building whose footprint touches
// that area. PlantVer changes when a plant in the chunk appears, goes,
// changes species, growth stage or harvestability.
type Chunk struct {
	Ver, PlantVer                    uint64
	Plants, Items, Buildings, Frames []int32
}

// AllyGroup is one aid group on the map for the alerts: its faction,
// how many of its fighters stand and are on the map, whether it is
// leaving, the hours left at most, and where its standing fighters are.
type AllyGroup struct {
	Faction   string
	Standing  int
	Total     int
	Leaving   bool
	HoursLeft int
	Spots     [][2]int
}

// Fire is one burning cell.
type Fire struct {
	X, Y int
	Size int // 1..100
}

// Strike is one lightning strike: where and when (tick).
type Strike struct {
	X, Y int
	Tick uint64
}

// Minimap is the minimap layer: Cells is W×H, row-major, each a
// terrain code with one of the game.Minimap* kinds; Ver changes whenever any
// cell may have. Cells is shared and read-only.
type Minimap struct {
	Ver   uint64
	W, H  int
	Cells []uint8
}

// CropRaid is a crop a wild animal ate in the last hour.
type CropRaid struct {
	X, Y    int
	Tick    uint64
	Species string
}

// PowerNet is one power net for the overlay and the inspector: its id
// (game.BuildingView.PowerNet), W generated and W drawn now, the batteries'
// stored energy and capacity in Wh, and whether consumers are shed.
type PowerNet struct {
	ID, Gen, Load, StoredWh, CapWh int
	Brownout                       bool
}
