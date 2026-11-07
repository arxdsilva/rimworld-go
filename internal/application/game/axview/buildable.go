package axview

// Buildable is the read model of one buildable definition in the
// Architect menu (game.BuildableView).
type Buildable struct {
	ID        uint8 // value for command.Command.Build
	Kind      string
	Wood      int
	Work      int
	Rotatable bool
	Floor     bool // floor layer: can sit under furniture
	Blocks    bool // completed building blocks movement
	StoneOK   bool // can be built from stone blocks
	W, H      int  // footprint facing south
	Bench     bool // runs bills
	// Stuffs are the materials it may be built from, default first ("wood",
	// "stone"; a carpet lists the carpet items). Carpet marks the entrance
	// carpet, installed from a crafted carpet item (SIM-021).
	Stuffs []string
	// Catches is the percent of tracked dirt each material wipes off,
	// parallel to Stuffs; 0 for non-carpets.
	Catches []int
	Carpet  bool
	// Locked is why this colony cannot build it ("" when it can):
	// (*game.Game).Buildables sets game.ReasonScenario for a disallowed building.
	Locked string
}
