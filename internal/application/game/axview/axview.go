// Package axview is the AX-0 contract's read-model fields (every vanilla
// Architect item, .cursor/handoffs/AX-0-contract.md): one struct per game
// view, carried as its AX field (game.BuildingView.AX and so on), so the
// AX owners fill and read their own fields without editing game.go. Every
// field is zero until its owner fills it; each names its owner. Building
// stays comparable (the building view's equality check compares it).
package axview

// Building is game.BuildingView.AX.
type Building struct {
	// A1: Uninstall, marked to be crated (construction.Building.Uninstall);
	// OpenMarked, a grave, sarcophagus or casket marked to be opened.
	Uninstall, OpenMarked bool
	// A2: Seats, the diners or sitters a table or couch serves; Plant and
	// PlantPct, a plant pot's plant (species key) and its growth (percent);
	// WallMounted, a wall torch or wall lamp hung on the wall behind it.
	Seats       int
	Plant       string
	PlantPct    int
	WallMounted bool
	// A3: Colour, a carpet floor's colour name.
	Colour string
	// A4: Cycle, a deep drill's, scanner's or crematorium's work cycle done
	// (permille); Ore, the deposit under a deep drill (item kind name, ""
	// none: it brings up stone chunks); Radius, a moisture pump's dried
	// radius now (tiles); Flow, a watermill's river flow (permille of full,
	// 0 blocked).
	Cycle  int
	Ore    string
	Radius int
	Flow   int
	// A5: Charges, a foam turret's or rocket battery's shots left;
	// ChargeCap their full load; Armed, a foam popper or charge ready.
	Charges, ChargeCap int
	Armed              bool
	// A6: Joy, the recreation kind (recreation.JoyKind's name); Users, the
	// colonists using it now.
	Joy   string
	Users int
	// A7: ReadingBonus, the reading bonus of this bookcase's room
	// (permille over 1000; 0 none).
	ReadingBonus int
	// A8: Countdown, a ship reactor's ticks left (-1 not started, 0 done
	// or not a reactor); ShipID, the ship (connected parts) it belongs to.
	Countdown int
	ShipID    uint64
	// A9: Sleeper, who lies in a stasis casket ("" empty) and SleeperID;
	// LoadKg of CapKg, a pod group's load; FuelUnits, a launcher group's
	// chemfuel; Range, the tiles that fuel reaches; ScanFor, a mineral
	// scanner's target (item kind name, "" any); Group, the pod or
	// launcher group id.
	Sleeper       string
	SleeperID     uint64
	LoadKg, CapKg int
	FuelUnits     int
	Range         int
	ScanFor       string
	Group         uint64
}

// Item is game.ItemView.AX.
type Item struct {
	// A1: Building, a crated building's kind name ("" not crated).
	Building string
	// A7: Title, a book's title; Teaches, a textbook's skills or a
	// schematic's projects ("Construction, Mining"); Power, its reading
	// recreation power (permille).
	Title   string
	Teaches string
	Power   int
}

// Pawn is game.PawnView.AX.
type Pawn struct {
	// A6: Joy, the recreation kind it is enjoying now ("" none).
	Joy string
	// A7: Reading, the title of the book it reads now.
	Reading string
	// A9: Casket, the stasis casket it sleeps in (building id, 0 none).
	Casket uint64
}

// Frame is game.FrameView.AX.
type Frame struct {
	// A3: Colour, a carpet floor blueprint's colour name.
	Colour string
	// Extra-material delivery (construction.Def.Extra): kind names,
	// delivered and needed, slot by slot ("" unused).
	Extra          [3]string
	ExtraGot, Need [3]int
}

// World is game.WorldView.AX.
type World struct {
	// A4: DeepOreVer changes whenever the found deep ore does (the deep
	// ore overlay's cache key); DeepOre the cells found so far.
	DeepOreVer uint64
	DeepOre    []Deposit
	// A8: ShipLaunched, the escape ship has left (the ending); Ending, its
	// ending key ("" none); LaunchIssues, what the launch report lists.
	ShipLaunched bool
	Ending       string
	LaunchIssues []string
	// A9: Pods in flight and the mineral sites found.
	Pods  []PodFlight
	Sites []Site
}

// Deposit is one found deep-ore cell (A4).
type Deposit struct {
	X, Y int
	Ore  string // item kind name
	Left int    // units left
}

// PodFlight is a pod group in the air (A9).
type PodFlight struct {
	From, To int32  // world tiles
	Arrive   uint64 // tick
	Count    int    // pods
}

// Site is a world site a mineral scanner found (A9).
type Site struct {
	Tile    int32
	Mineral string // item kind name
	Expires uint64 // tick
}
