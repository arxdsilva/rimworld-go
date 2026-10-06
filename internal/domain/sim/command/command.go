// Package command is the typed gameplay command catalog.
package command

// Kind identifies a player or fixture mutation.
type Kind uint8

const (
	None Kind = iota
	MovePawn
	DesignateStockpile
	DesignateWall
	DesignateBed
	DesignateHarvest
	DesignateCancel
	SetWorkEnabled
	// DesignateBuild places a blueprint of Build (a construction kind id)
	// with Facing at X, Y.
	DesignateBuild
	// DesignateDeconstruct marks completed buildings in the X..X2, Y..Y2
	// rectangle for removal.
	DesignateDeconstruct
	// DraftPawn sets Pawn's drafted state to Enabled.
	DraftPawn
	// Prioritize makes Pawn drop its job and do Order at X, Y now.
	Prioritize
	// DesignateCut marks plants in the rectangle for cutting.
	DesignateCut
	// DesignateMine marks rock tiles in the rectangle for mining.
	DesignateMine
)

// WorkType is a compact work-column id for SetWorkEnabled.
type WorkType uint8

const (
	WorkNone WorkType = iota
	WorkHaul
	WorkConstruct
	WorkGrow
	WorkMine
	WorkCook
	WorkHunt
)

// Order is a player-forced job for Prioritize.
type Order uint8

const (
	OrderNone Order = iota
	OrderBuild
	OrderSupply // haul wood to the frame at X, Y
	OrderHarvest
	OrderSleep
	OrderEat
	OrderDeconstruct
	OrderHaul // carry the stack at X, Y to a stockpile
	OrderCut
	OrderMine
)

// Command is applied on a simulation tick, never by the UI directly.
type Command struct {
	Kind    Kind
	Pawn    uint64
	X, Y    int
	X2, Y2  int
	Work    WorkType
	Enabled bool
	Build   uint8 // construction kind id for DesignateBuild
	Facing  uint8 // construction facing for DesignateBuild
	Order   Order
	Layer   uint8 // construction layer for Prioritize build/supply on shared tiles
	Stuff   uint8 // item kind used as building material for DesignateBuild; 0 = wood
}
