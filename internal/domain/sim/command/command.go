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
	// Undesignate clears only the designation named by Order (harvest,
	// cut, hunt, tame, mine, deconstruct) in X..X2, Y..Y2; zones,
	// blueprints and other marks stay (UI-011).
	Undesignate
	// MoveBuilding plans the move of building Target to X, Y facing
	// Facing: builders reinstall it there, bills and owners kept (UI-011).
	MoveBuilding
	// DesignateHome paints (Enabled) or clears the Home area over X..X2,
	// Y..Y2: colonists clean only inside it (SIM-021).
	DesignateHome
	// SetTroughFilter sets feeding trough Target's feed filter to Mode
	// (0 plants, 1 meat, 2 any; SIM-024).
	SetTroughFilter
	// CancelFrame removes blueprint/frame Target, dropping its delivered
	// materials back on the map (UI-015).
	CancelFrame
	// SetForbidden forbids (Enabled) or allows item stack Target: forbidden
	// stacks are left alone by haulers, eaters and builders (UI-016).
	SetForbidden
	// DesignateForbid forbids (Enabled) or allows every stack in X..X2,
	// Y..Y2.
	DesignateForbid
	// DesignateSmooth marks rock walls (no ore) and rough stone floors in
	// X..X2, Y..Y2 for smoothing (SIM-029).
	DesignateSmooth
	// SetStoryteller switches the storyteller to the profile key in Name
	// (NG-1 §2.4); it takes effect at the next scheduling decision.
	SetStoryteller
	// SetDifficulty sets the difficulty: Name is a preset key, or
	// "custom:" + difficulty.Encode(settings) (NG-1 §2.4).
	SetDifficulty
	// Social and host systems (ENG-003). Field use per
	// .cursor/SOCIAL_SYSTEM_CONTRACT.md C1.
	//
	// AddOperation queues surgery on patient Pawn: Index is the body.OpKind,
	// Build the body part, Stuff the item kind (an implant).
	AddOperation
	// RemoveOperation drops Pawn's queued operation in slot Index.
	RemoveOperation
	// SetDrugPolicy sets Pawn's drug policy to Mode (a drug.Preset).
	SetDrugPolicy
	// AssignBedOwner gives bed Target's slot Index to Pawn (0 clears it).
	AssignBedOwner
	// SetAnimalMaster makes Pawn the master of animal Target (0 clears).
	SetAnimalMaster
	// SetTraining turns training skill Index on animal Target on or off
	// (Enabled).
	SetTraining
	// GiftAnimal gives animal Target to visitor Pawn.
	GiftAnimal
	// Capture sends captor Pawn to take downed pawn Target prisoner.
	Capture
	// Arrest sends arrester Pawn to arrest Target.
	Arrest
	// SetPrisonerMode sets prisoner Target's interaction to Mode (a
	// prisoner.Mode).
	SetPrisonerMode
	// SetBedForPrisoners marks bed Target for prisoners (Enabled) or not.
	SetBedForPrisoners
	// FormCaravan sends the comma-separated member ids in Name to world
	// tile Count.
	FormCaravan
	// CaravanReturn brings caravan Target home.
	CaravanReturn
	// PaintArea paints (Enabled) or clears area Index over X..X2, Y..Y2.
	PaintArea
	// RenameArea names area Index Name.
	RenameArea
	// AssignArea restricts Pawn to area Index (-1: unrestricted).
	AssignArea
	// ReleasePrisoner lets prisoner Target go.
	ReleasePrisoner
	// DesignatePlan paints (Enabled) or erases plan marks over the
	// rectangle X,Y..X2,Y2: purely visual layout placeholders (PLAY-012).
	DesignatePlan
	// OpenCasket orders the sealed sleeper casket Target opened (MAPGEN-3):
	// a colonist walks there and works it open, waking its whole group.
	OpenCasket
	// PLAY-015 areas: CreateArea adds an area named Name; DeleteArea and
	// InvertArea act on area id Index. AssignArea's Index is an area id,
	// AreaUnrestricted or AreaHome; its Pawn may be a colonist or an animal.
	CreateArea
	DeleteArea
	InvertArea
	// SetHomeAuto turns automatic Home expansion on (Enabled) or off.
	SetHomeAuto
	// DesignateSnowClear paints (Enabled) or clears the snow clearing area
	// over X,Y..X2,Y2.
	DesignateSnowClear
	// SetPenSettings sets pen marker Target's settings from Name (the
	// encoded species/sex and auto-cut lists, see game.EncodePenSettings).
	SetPenSettings
	// DevStartFire starts a fire at X, Y (developer mode).
	DevStartFire
)

// Area references for AssignArea's Index (PLAY-015): any other value is an
// area id.
const (
	AreaUnrestricted = -1
	AreaHome         = -2
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
	WorkClean
	WorkHandle // taming, milking, shearing (SIM-026)
	WorkWarden // prisoners (ENG-003)
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
	OrderTame   // Undesignate only: the taming mark
	OrderSmooth // smooth the rock wall or stone floor at X, Y (SIM-029)
	// OrderExtinguish (PLAY-015): beat out the fire at X, Y (anywhere: a
	// forced order ignores Home and the allowed area).
	OrderExtinguish
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
	// Target is a building id (MoveBuilding).
	Target uint64
	// Name carries a key for SetStoryteller / SetDifficulty.
	Name string
}
