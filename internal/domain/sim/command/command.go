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
	// SetForbidden forbids (Enabled) or allows item stack or building Target (BED-1): forbidden
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
	// PaintArea paints (Enabled) or clears the area with id Index over
	// X..X2, Y..Y2 (an unknown id is ignored).
	PaintArea
	// RenameArea names the area with id Index Name.
	RenameArea
	// AssignArea restricts Pawn (a colonist or a tame non-penned animal)
	// to area ref Index: an area id, AreaUnrestricted or AreaHome.
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
	// SetAnimalFollow sets whether trained animal Target follows its
	// drafted master (Enabled).
	SetAnimalFollow
	// DesignateHaul marks (Enabled) or unmarks every unclaimed stack in
	// X..X2, Y..Y2 for hauling; SetHaulMark does it for stack Target
	// (PLAY-017).
	DesignateHaul
	SetHaulMark
	// DismissLetter removes letter Target from the letter stack (PLAY-018).
	DismissLetter
	// PLAY-020 medical beds and bed rest: SetBedMedical keeps bed Target
	// for the sick and injured (Enabled) or not, that bed only;
	// SetPawnBedRest turns colonist Target's rest when hurt on (Enabled)
	// or off.
	SetBedMedical
	SetPawnBedRest
	// PLAY-020 zone management, on zone id Target: SetZoneSow, SetZoneCut
	// and SetZoneHidden allow sowing, allow cutting and hide the zone
	// (Enabled); RenameZone names it Name ("" = the default); SetZoneCrop
	// grows species id Build there; ExpandZone adds the valid cells of
	// X..X2, Y..Y2 to it; RemoveZoneCells removes its cells there;
	// DeleteZone removes it.
	SetZoneSow
	SetZoneCut
	SetZoneHidden
	RenameZone
	SetZoneCrop
	ExpandZone
	RemoveZoneCells
	DeleteZone
	// PLAY-021 electricity: SetPowerWanted asks for building Target's power
	// (a device's toggle or a switch) on (Enabled) or off; a colonist then
	// flicks it. SetThermostat sets heater or cooler Target's target
	// temperature to Count tenths of a degree C.
	SetPowerWanted
	SetThermostat
	// MED-00 medical care (implemented in MED-03): SetMedicalCare sets
	// patient Pawn's medical care policy to Mode; SetDefaultCare sets the
	// default policy of group Index (0 colonists, 1 prisoners, 2 guests,
	// 3 animals) to Mode; SetSelfTend lets colonist Pawn tend itself
	// (Enabled) or not; EmergencyTend sends drafted doctor Pawn to tend
	// patient Target now; ReorderOperation moves patient Pawn's queued
	// operation in slot Index by Count (-1 up, +1 down).
	SetMedicalCare
	SetDefaultCare
	SetSelfTend
	EmergencyTend
	ReorderOperation
	// SetZonePriority (PLAY-023) sets storage zone Target's priority to
	// the one at Index of the priority list, highest first (Critical,
	// Important, Preferred, Normal, Low). A DesignateStockpile with Build
	// 1 paints a dumping stockpile instead of a stockpile.
	SetZonePriority
	// PLAY-024 trade and comms, acting on the open session (one at a
	// time; PLAY024_PLAN.md): ProposeTrade offers the deal in Name
	// (game.EncodeTradeOffer of []trade.Delta: positive Qty the colony
	// buys, negative it sells), as a gift when Enabled; CloseTrade ends
	// the trade session. CommsCallTrader opens trade with the orbital ship
	// in range; CommsGift gives faction Index Count silver;
	// CommsRequestCaravan and CommsRequestAid ask faction Index for a
	// trade caravan or military aid; CloseComms ends the comms session.
	ProposeTrade
	CloseTrade
	CommsCallTrader
	CommsGift
	CommsRequestCaravan
	CommsRequestAid
	CloseComms
	// AcceptLetter accepts the offer in letter Target (SIM-063: a rescued
	// stranger asks to join); DismissLetter declines it.
	AcceptLetter
	// PRD-1 bill settings, on bill Index of the bench at X, Y:
	// ConfigureBill sets Store (recipe.StoreMode) and Zone (the stockpile
	// for "take to a specific stockpile"), Radius (ingredient search,
	// tiles; 0 unlimited), SkillMin..SkillMax, Worker (only this colonist;
	// 0 anyone) and UnpauseAt ("until you have": resume below it; 0 the
	// default). SetBillIngredient allows (Enabled) or forbids ingredient
	// kind Stuff. ReorderBill moves the bill by Count places (negative:
	// up).
	ConfigureBill
	SetBillIngredient
	ReorderBill
	// DEF-3 defences, on the building at X, Y (wave D contract; game
	// ignores them until the combat package handles them):
	// SetDoorHoldOpen holds the door open (Enabled) or lets it close;
	// SetMortarTarget forces the mortar to fire at X2, Y2 and
	// ClearMortarTarget drops that target; SetTurretHold makes the turret
	// hold fire (Enabled) or fire at will.
	SetDoorHoldOpen
	SetMortarTarget
	ClearMortarTarget
	SetTurretHold
	// Wave E contract (game ignores them until PRD-4 and DEF-4 handle
	// them). PRD-4 storage, on storage zone Target: SetStockpileFilter
	// allows (Enabled) or forbids the filter-tree node Name (a category
	// path such as "Food/Meals" or an item kind name);
	// SetStockpileSpecial allows (Enabled) or forbids special filter Index
	// (allow rotten, fresh only, ...); PasteStockpile copies storage zone
	// Zone's filter, special filters and priority onto Target (the copy
	// itself is a UI clipboard holding the source zone). RenameZone
	// names a stockpile too. DEF-4 drafted control, on Pawn:
	// SetFireAtWill lets it shoot on its own (Enabled) or only when
	// ordered; SetHoldPosition keeps it on its tile (Enabled), not
	// chasing targets; ThrowGrenade throws its grenades at X, Y.
	SetStockpileFilter
	SetStockpileSpecial
	PasteStockpile
	SetFireAtWill
	SetHoldPosition
	ThrowGrenade
	// STO/CBT contract (game ignores them until STO-2, STO-3 and CBT-2
	// handle them): HaulUrgently marks (Enabled) or unmarks every stack in
	// X..X2, Y..Y2 to be hauled before other hauling; DropWeapon makes
	// colonist Pawn drop its weapon where it stands; LinkStorage links
	// storage building Target into building Zone's storage group (sharing
	// its settings) and UnlinkStorage takes Target out of its group;
	// SetStorageSource makes bill Index of the bench at X, Y take its
	// ingredients only from storage zone or group Zone (0: anywhere in its
	// radius); SetZoneOverlay shows every zone (Enabled) or only the
	// selected one's.
	HaulUrgently
	DropWeapon
	LinkStorage
	UnlinkStorage
	SetStorageSource
	SetZoneOverlay
	// CBT-3/CBT-4 (append only): PopSmokepop makes colonist Pawn set off a
	// worn smokepop pack where it stands; SetThreatResponse sets colonist
	// Pawn's undrafted threat response in X (0 default, 1 flee, 2 attack,
	// 3 ignore).
	PopSmokepop
	SetThreatResponse
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
	// WorkFirefight (PLAY-015): beating out fires in Home.
	WorkFirefight
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
	// OrderFlick (PLAY-021): flick the switch or power toggle at X, Y now.
	OrderFlick
	// OrderTrade (PLAY-024): walk to the trader at X, Y and open trade;
	// OrderUseComms: work the radio console at X, Y and open comms.
	OrderTrade
	OrderUseComms
	// Pawn-targeted orders (SIM-062…065) name the pawn in Command.Target;
	// X, Y is the tile the player clicked. Rescue carries a downed
	// non-hostile to a safe bed, Capture a downed pawn to a prison bed,
	// Arrest takes a standing pawn prisoner, TendPatient is an undrafted
	// "prioritize tending", FeedPatient brings a meal, Strip drops a
	// pawn's apparel and weapon, CarryToPrisonBed returns a downed
	// prisoner to its bed.
	OrderRescue
	OrderCapture
	OrderArrest
	OrderTendPatient
	OrderFeedPatient
	OrderStrip
	OrderCarryToPrisonBed
)

// TargetsPawn reports an order that acts on the pawn in Command.Target
// rather than on the tile (SIM-062).
func (o Order) TargetsPawn() bool { return o >= OrderRescue && o <= OrderCarryToPrisonBed }

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
	// Target is a building id (MoveBuilding) or the pawn a pawn-targeted
	// order acts on (EmergencyTend, Capture, Arrest, Prioritize with
	// OrderRescue…OrderCarryToPrisonBed).
	Target uint64
	// Name carries a key for SetStoryteller / SetDifficulty.
	Name string
	// Bill settings for ConfigureBill (PRD-1).
	Store              uint8
	Zone               uint64
	Radius             int
	SkillMin, SkillMax uint8
	Worker             uint64
	UnpauseAt          int
}
