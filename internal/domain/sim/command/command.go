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
	// DesignateGrowZone paints a growing zone sowing crop Build (plant species id).
	DesignateGrowZone
	// AddBill adds recipe Recipe with Mode/Count to the bench covering X, Y.
	AddBill
	// SetBill updates bill Index on the bench at X, Y (Mode, Count, Enabled=suspended).
	SetBill
	// RemoveBill deletes bill Index on the bench at X, Y.
	RemoveBill
	// DesignateHunt marks animals in X,Y..X2,Y2 for hunting.
	DesignateHunt
	// ClearOrders drops a pawn's queued orders.
	ClearOrders
	// SetWorkPriority sets Pawn's Work column to priority Count (0 = off).
	SetWorkPriority
	// SetSchedule sets Pawn's hour Index to activity Mode (Pawn 0: everyone).
	SetSchedule
	// SetResearch picks research project Build as the current project.
	SetResearch
	// DesignateTame marks animals in the rectangle for taming.
	DesignateTame
	// DesignatePen paints an animal pen over the rectangle.
	DesignatePen
	// ReleaseAnimal lets the tame animal Pawn (an animal id) go wild.
	ReleaseAnimal
	// DesignateRoof plans roofs over the supported, unroofed tiles of the
	// X..X2, Y..Y2 rectangle (SIM-018).
	DesignateRoof
	// DesignateRemoveRoof plans the removal of roofs in the rectangle and
	// keeps them off, even over closed rooms (SIM-018).
	DesignateRemoveRoof
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
	WorkCraft
	WorkDoctor
	WorkResearch
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
	OrderSow
	OrderHunt   // hunt the animal at X, Y
	OrderAttack // drafted: attack the animal at X, Y
	OrderEquip  // pick up and wield the weapon at X, Y
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
	Recipe  uint8 // recipe id for AddBill
	Index   int   // bill index for SetBill/RemoveBill
	Mode    uint8 // recipe.Mode for AddBill/SetBill
	Count   int   // bill target for AddBill/SetBill
	// Queue adds a MovePawn/Prioritize order behind the pawn's current
	// order instead of replacing it.
	Queue bool
}
