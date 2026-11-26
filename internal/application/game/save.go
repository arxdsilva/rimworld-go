// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"slices"

	"rimworld-go/internal/application/game/animalstate"
	"rimworld-go/internal/application/game/aqua"
	"rimworld-go/internal/application/game/bodytemp"
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/application/game/industry"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/application/game/modstate"
	"rimworld-go/internal/application/game/orders"
	"rimworld-go/internal/application/game/outfit"
	"rimworld-go/internal/application/game/postpone"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/recreate"
	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/application/game/resize"
	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/application/game/savecanon"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/application/game/savelegacy"
	"rimworld-go/internal/application/game/saveutil"
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/application/game/setupview"
	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/application/game/thermo"
	"rimworld-go/internal/application/game/wealth"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/area"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/fire"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/stain"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// Saved games: the complete authoritative state, gob-encoded and
// gzipped behind a small header. Loading restores it exactly, so a loaded
// game continues tick for tick as the original would have (the RNG, the
// entity allocator and every store are captured). Derived caches (room
// stats) are rebuilt on load.
//
// Versions (NG-1 §5.8): v1 is every save before NG-1; v2 adds the header
// fields below and writes the blocked tiles as a bitset (BlockedBits).
// Every other field is additive, so Load reads both: a v1 save gets the
// legacy setup (no storyteller, difficulty or factions) and neutral bios.
// v3 (PLAY-017) adds unclaimed map items and harvest marks on ripe zone
// crops; a v1/v2 save is migrated on load (migrateClaims,
// migrateHarvestMarks). v4 (PLAY-020) adds medical beds: an older save's
// hospital beds become medical (migrateMedicalBeds). v5 (PLAY-021) adds
// electricity (Power): lamps need power, so an older save's lamps are to
// become torch lamps (the v < 5 step). v6 (MED-02) makes infections body
// conditions racing immunity: an older save's per-pawn infections
// (Wounds, now read only) become infection conditions on each pawn's
// worst open wound (migrateWoundInfections). v7 (FOOD-1) replaces the
// down-at-0 starvation with malnutrition: a pawn an older save has down
// from hunger gets up (migrateStarvation). The bars keep their scale
// (needs.Full is 1.0 nutrition), so stored hunger and animal food carry
// over as they are. v8 (TEMP-*) makes everyday clothes real garments
// (TEMP-4): an older save's colonists, who wore them implicitly, get a
// T-shirt and pants where their skin slots are free (Outfit.Everyday).
// v9 (RSCH-1) moves research to the wiki costs: each project keeps its
// fraction of the cost, and the resized research benches keep their place
// where they still fit or are refunded (migrateBenches). v10 (STK-BN)
// does the same for the benches it gives the wiki footprints (game/resize).
// v11 (AX-0) is for the AX owners' steps (A2 tables, A3 floors and carpets).
// v12 (MAP-0) adds the extra open maps (ExtraMaps): an older binary refuses
// a save it would read as the first map alone. v14 (GEAR-3) adds the
// apparel policies (Outfits): an older save gets the defaults, everyone on
// Anything. v15 (QST-1) adds the quest difficulty settings (an older save
// gets rewards at 100 % and quest threats as its big threats) and the
// quests. v16 (SAV-003) names the save's colony in the header
// (savefmt.Info); an older header gets one from its name on reading. v17
// (EAI-0) adds each map's wealth tracker (game/wealth) and the factions'
// wars (Faction.Enemies): an older save's first map is counted once, and
// no faction is at war with another yet. v18 (PLG-063) adds the Wasm
// mods' state (ModState, PLG-051): older saves have none; mods start fresh.
// v19 (DEF 3.1) adds Kit.DamageTaken and LogEntry.Kind: older saves read 0.
// v20 (SHT-007): friendly fire; older saves 100 %.
// migrateSave runs the steps a save's version is missing.

// v21 preserves the previous Labor field and stores extended work bits separately.
// v21 appends Artistic: historical eleven-skill gob arrays remain readable
// through legacy fields, while current arrays use v21-named wire fields.
// SaveVersion is the version new saves are written with.
const SaveVersion = 21

// minSaveVersion is the oldest version Load still reads.
const minSaveVersion = 1

// SupportedSaveVersion reports whether Load can read a save of version v.
func SupportedSaveVersion(v int) bool { return v >= minSaveVersion && v <= SaveVersion }

// SaveInfo is the header of a save: enough to list it without loading
// (savefmt.Info, which the save store and the client share).
type SaveInfo = savefmt.Info

type entry[T any] struct {
	ID entity.EntityID
	V  T
}

// LegacyLaborMask matches the narrow historical Gob wire type.
type legacyLaborMask uint16

func importLabor(old []entry[legacyLaborMask], wide []entry[work.Mask]) *component.Store[work.Mask] {
	if wide != nil {
		return saveutil.ImportStore(wide)
	}
	s := component.NewStore[work.Mask]()
	for _, e := range old {
		s.Set(e.ID, work.Mask(e.V))
	}
	return s
}

type savedOrder struct {
	Cmd   command.Command
	Label string
	// RetryAt/Tries: a blocked order waiting to be retried (SIM-027;
	// absent in older saves).
	RetryAt uint64
	Tries   int
}

type savedBreak struct {
	Kind  mentalbreak.Kind
	Until uint64
}

// saveState mirrors Game with exported fields.
type saveState struct {
	Calendar   clock.CalendarPolicy
	MapClimate *temperature.Climate
	Tick       clock.Tick
	W, H       int
	NextEntity entity.EntityID
	RNG        []byte
	Sched      schedule.Grid
	// Blocked lists the blocked tile indexes (v1); v2 writes BlockedBits,
	// the grid's bitset (a quarter of the size of the list), instead.
	Blocked     []int
	BlockedBits []uint64

	Messages []MessageView
	// Letters is the letter stack, oldest first, and NextLetter how many
	// ids were ever issued (PLAY-018, additive: an older save has neither
	// and importLetters builds the stack from its letter messages).
	Letters    []letters.Letter
	NextLetter uint64

	PawnIDs   []entity.EntityID
	Positions []entry[Position]
	Names     []entry[string]
	Jobs      []entry[job.Record]
	Need      []entry[needs.Pair]
	Vital     []entry[health.Record]
	Labor     []entry[legacyLaborMask]
	LaborWide []entry[work.Mask]

	ItemIDs  []entity.EntityID
	Stacks   []entry[item.Stack]
	FrameIDs []entity.EntityID
	Frames   []entry[construction.Frame]
	BldgIDs  []entity.EntityID
	Bldgs    []entry[construction.Building]
	PlantIDs []entity.EntityID
	Plants   []entry[plant.Wild]
	Terr     terrain.Map
	Mines    map[[2]int]bool

	Stockpiles []zone.Stockpile
	GrowZones  []zone.GrowZone
	LastMeal   map[entity.EntityID]item.Kind
	Bills      map[entity.EntityID][]recipe.Bill
	BillEdits  map[entity.EntityID]command.ConfigEdit
	Racks      map[entity.EntityID]craft.Batch
	Art        map[entity.EntityID]craft.Artwork // PRD-3: each sculpture's title seed and carver; absent in older saves
	Fuel       map[entity.EntityID]int
	Exposure   map[entity.EntityID][2]int
	NoComfort  map[entity.EntityID]uint64
	NoFood     map[entity.EntityID]uint64     // absent in older saves
	Postponed  []postpone.Entry               // absent in older saves (SIM-027)
	PostSweep  uint64                         // the next postponed sweep (v2; absent: the first tick)
	PostOrders map[entity.EntityID]savedOrder // orders being followed; absent in older saves
	RestedFree map[entity.EntityID]bool       // absent in older saves
	Calm       map[entity.EntityID]uint64     // absent in older saves
	Banked     map[entity.EntityID]bool       // absent in older saves
	// RestNeeds: comfort, beauty and outdoors needs (BED-3); absent in older saves
	RestNeeds  map[entity.EntityID]rest.NeedRecord
	OrderQueue map[entity.EntityID][]savedOrder
	Apparel    map[entity.EntityID][3]item.Kind     // before PRD-2: kinds only (read when Worn is absent)
	Worn       map[entity.EntityID]craft.OutfitSave // PRD-2: worn pieces with stuff, quality and wear
	// Prio is the priorities as saved before animal handling (11 work
	// types); Prios replaces it (SIM-026), with 12 types; PrioList (ENG-003)
	// replaces that, as plain slices so appending a work type never breaks
	// a save again (gob refuses an array whose length changed).
	Prio       map[entity.EntityID]legacyPriorities
	Prios      map[entity.EntityID]priorities12 // absent in older saves
	PrioList   map[entity.EntityID][]uint8      // absent in older saves
	RaiderIDs  []entity.EntityID
	Raiders    map[entity.EntityID]raid.Raider
	RaidGroups []raid.Group
	NextThreat uint64
	ThreatsOn  bool
	Scheds     map[entity.EntityID]schedule.Grid
	Joy        map[entity.EntityID]int
	Breaks     map[entity.EntityID]savedBreak
	Mind       mind.Save   // mind (SIM-066): break targets and retarget ticks
	Orders     orders.Save // AX-1: strip and open marks; absent in older saves
	RoomGrid   []int
	RoomTemps  []temperature.Temp
	Memories   map[entity.EntityID]mood.Memories
	Claims     []reservation.Claim
	Pending    []command.Command
	CmdSeq     uint64 // #8: the last command Seq issued; absent in older saves

	AteAt   map[entity.EntityID]bool
	Draft   map[entity.EntityID]bool
	SleptAt map[entity.EntityID]entity.EntityID

	AnimalIDs           []entity.EntityID
	Animals             []entry[animal.State] // legacy map records, readable through old native v21
	AnimalFormat        uint8                 // 1: grouped records; 0: legacy pre-migration save
	AnimalRecords       []animalstate.Record
	WorldAnimalRecords  []animalstate.Record // off-map stash, pod, caravan, flight and rope
	TrainingMaintenance map[entity.EntityID]training.Maintenance
	Equip               map[entity.EntityID]item.Kind
	Traces              []projectile.Trace
	Wild                bool
	Wildlife            wildlifeState

	Carrying    map[entity.EntityID]entity.EntityID
	TendedUntil map[entity.EntityID]uint64
	Wounds      map[entity.EntityID]legacyWound // read only: saves up to v5 (migrateWoundInfections)
	// Research is the first five projects (the pre-NG-1 array, which old
	// saves decode); ResearchAll is every project and wins when present.
	Research    research.V1
	ResearchAll []int
	// NG1 is the new-game setup and its live state (scenario, storyteller,
	// difficulty, factions, climate, conditions, arrival); nil = legacy.
	NG1   *savelegacy.SetupState // v1-v20: original 11-skill pawn arrays
	NG121 *SetupState            // v21: includes Artistic
	// DoorsOriented: door facings are stored (PLAY-011); a save without it
	// re-derives every door's facing from its neighbours on load.
	DoorsOriented bool
	// Bios, Skills, WorkCarry (NG-1 §5.4): each colonist's identity, skills
	// and fractional work progress; absent in older saves (neutral bios).
	Bios      map[entity.EntityID]savelegacy.Pawn     // v1-v20, read-only
	Bios21    map[entity.EntityID]pawngen.Pawn        // v21
	Skills    map[entity.EntityID]savelegacy.SkillSet // v1-v20, read-only
	Skills21  map[entity.EntityID]skill.Set           // v21
	WorkCarry map[entity.EntityID]int32
	// ContentKeys (PLG-044) is each moddable class's key per saved index;
	// a load maps the indexes onto the running content. Absent: core only.
	ContentKeys savefmt.Keys
	// SkillLive (v2): the rest of the skill state that moves the game —
	// learning saturation, fractional need drains, the last skilled work
	// (passion joy) and fractional research; absent in older saves.
	SkillLive   *savelegacy.SkillLive // v1-v20, read-only
	SkillLive21 *savedSkillLive       // v21
	ResearchCur research.Project
	ResearchSet bool
	// ResearchCold (v9, RSCH-1): research benches in the bad temperature
	// band (game/research's hysteresis).
	ResearchCold []entity.EntityID
	Pens         []pen
	RockDamage   map[[2]int]int
	Residual     map[int]temperature.Temp
	Thermo       thermo.Save // TEMP-1: the rooms' air at working precision; absent in older saves
	// Motion: step timing per creature on the map. A creature without a
	// record (it never stepped, or an older save) is not given one on
	// load: motionOf reads it as a due-now straight step, as before the save.
	Motion []entry[motionRec]
	// Strikes and Hits are the presentation stamps of action_view.go
	// (absent in older saves: no recent strike or hit).
	Strikes map[entity.EntityID]strikeRec
	Hits    map[entity.EntityID]hitRec
	// Weather (SIM-016) and Graves (SIM-017); absent in older saves.
	Weather weather.State
	Graves  map[entity.EntityID]graveRec
	// Roofs (SIM-018): roof.Cell flags per tile; absent in older saves,
	// whose closed rooms are auto-roofed on load.
	Roofs    []uint8
	RoofLoss []int32 // pending natural roof support loss
	// Fog (MAPGEN-2) is the unexplored-tile bitset; absent: no fog.
	// Landing is the landing spot (x, y); absent: the map centre.
	Fog     []uint64
	Landing []int
	// MAPGEN-3: Ocean is the open-sea bitset (absent: no sea known, every
	// deep water is bridgeable); Vaults the sealed vaults' state.
	Ocean  []uint64
	Vaults []vaultSave
	// Tanks is the fish tanks in use on this map (FISH-*, game/aqua); nil
	// without one, so older saves and goldens hold.
	Tanks []aqua.Entry
	// AX4 (game/industry): river flow, deep ore, machines' progress;
	// absent in older saves (no river flow, no ore found).
	AX4 industry.Save
	AX8 *ship.Save // game/ship (AX-8): reactors, sleepers, the launch; absent: nothing started
	// PLAY-015: NoHomeAuto (zero: Home expands automatically), per-tile
	// snow and the snow clearing area, burning fires, pen marker settings.
	NoHomeAuto bool
	Snow       []byte
	SnowClear  []uint64
	Fires      []fire.Fire
	PenMarkers []penSave
	PenAnimals []penAnimalSave
	// Grass and GrassCut (UI-011); absent in older saves, which reseed.
	Grass    []uint8
	GrassCut [][2]int
	// GrassGrowth (PLAY-010) is each regrowing grass tile's growth
	// percent (0: full); empty while nothing regrows and in older saves,
	// which load full.
	GrassGrowth []uint8
	// SIM-023..026, all absent in older saves: the biome (temperate
	// forest), regrowing grass, nature events, and feeding troughs.
	Biome       biome.ID
	GrassRegrow []grassRegrow
	Incidents   incident.State
	Wealth      *wealth.Save // EAI-0 (game/wealth): nil while it keeps nothing
	Troughs     map[entity.EntityID]troughState
	// ZoneMap (ZONE-001): the per-tile zones; absent in older saves, whose
	// stockpile, grow zone and pen rectangles are painted into it on load.
	ZoneMap zoneSave
	// Logs (LOG-001): per-creature event logs; absent in older saves.
	Logs map[entity.EntityID][]logEntry
	// Stains (SIM-020): blood and ichor on the ground; absent in older
	// saves (none).
	Stains []stain.Stain
	// StainDrip (v2): tiles each bleeding creature walked since its last
	// drip; absent in older saves (every count restarts at zero).
	StainDrip map[entity.EntityID]int
	// Dirt, Carried, Home (SIM-021): tracked-in dirt, what each pawn carries
	// on its boots and the Home area; absent in older saves. CleanWired
	// marks a save made after cleaning work was switched on for everyone.
	Dirt       []dirtCell
	Carried    map[entity.EntityID]uint16
	Home       []uint8
	CleanWired bool
	// CleanQ: Home tiles queued for cleaning, half-swept ones included,
	// so a loaded game resumes them (absent: rebuilt from the threshold).
	CleanQ []int32
	// HomeVer is the Home area's change counter (WorldView.HomeVer).
	HomeVer uint32
	// Plan is the sorted tile indexes with a plan mark (PLAY-012); nil
	// when none, so saves without plans hash as before.
	Plan []int32
	// Incoming (HAUL-001): material on its way to each frame or fuel user.
	Incoming map[entity.EntityID]int
	// Deliveries (HAUL-001): each hauler's shares of Incoming, route order.
	Deliveries    map[entity.EntityID][]haulShare
	HarvestSweep  []entity.EntityID // HAUL-003 (game/haul): growers sweeping a field, ascending; nil when none
	ColonistOrder []entity.EntityID // CTL-011 (game/roster): the colonist bar, first to last; nil until moved
	// Geo and Smooths (SIM-029): stone types, ore veins and smoothing
	// marks; absent in older saves, whose geology is regenerated from
	// their terrain and whose legacy stone becomes granite blocks.
	// StructVer is WorldView.StructVer, restored so a loaded game's view
	// matches the original's.
	Geo       geology.Map
	Smooths   map[[2]int]bool
	StructVer uint64
	// Social and host systems (ENG-003), all absent in older saves (nil:
	// fresh). Each block is filled by its milestone; RNGStreams holds the
	// base seed and the state of every stream used so far.
	Social     *socialSave
	People     *savelegacy.People // v1-v20, read-only
	People21   *peopleSave        // v21
	Bodies     []entry[savedBody] // body.go: slices since MED-00, gob reads the older shape
	Surgery    *surgerySave
	Drugs      []entry[savedDrug] // dose_host.go: slices since MED-00
	Sense      *sense.Save        // game/sense (CSE, v13): nil at the defaults
	Outfits    *outfit.Save       // game/outfit (GEAR-3, v14): nil at the defaults
	PetNames   []entry[string]    // skills.go (RHUD-025): renamed animals' rolled names; nil until a rename
	Prisoners  []entry[prisoner.State]
	World      *worldSave
	Areas      *area.Set
	History    []historyEntry
	RNGStreams map[string][]byte
	// BedRest (PLAY-020): "Rest when hurt" settings, who rests since
	// when, who found no bed, and the medical beds held for a stay; nil
	// when there is none (absent in older saves).
	BedRest *care.RestSave
	// Power (PLAY-021): the colony's electricity (power.go); nil without
	// power devices, so saves without power hash as before (absent in
	// older saves).
	Power *powerSave
	// Care (MED-03): medical care levels and self-tend settings
	// (care/policy.go); nil while all are at their defaults, so older saves
	// hash as before.
	Care *care.PolicySave
	// FeedRetry (SIM-064): when a patient nobody could feed is tried
	// again (care/feed.go); nil when there is none (absent in older saves).
	FeedRetry []care.Entry[uint64]
	// PLAY-024: the trade state (trade.go) and the sentry turrets' aims
	// (combat/turret.go); nil when there is nothing to keep, so older saves
	// and colonies without traders or turrets hash as before.
	Trade   *tradeSave
	Turrets *combat.TurretSave
	Shots   []projectile.Shot // DEF-1: projectiles in flight (nil: none)
	Smoke   []explosion.Cloud // DEF-2: smoke clouds (nil: none)
	// DEF-4 (raid/mech.go): mechanoid stuns and guard posts, crashed ship
	// parts; nil when there are none (absent in older saves).
	Mechs *raid.MechSave
	Diet  *diet.Save // FOOD-4 (game/diet): food policies and holders; nil at the defaults (absent in older saves)
	// Recreation (AX-6, game/recreate): recreation tolerances; nil when
	// there are none (absent in older saves).
	Recreation *recreate.Save
	// ExtraMaps (v12, MAP-0) is every open map after the first, in id
	// order, and View the map viewed; nil and 0 with one map.
	ExtraMaps []mapSave
	View      uint32
	// ModState (v18, PLG-063): the Wasm mods' state, one checked blob per
	// file; nil without one, first map only, never in StateHash.
	ModState       []modstate.Blob
	LegacyFactions []faction.Faction // legacy colonies have no Setup to carry their diplomacy
}

// mapSave is one extra map: only State's map fields are written
// (exportMapFields), and Climate is its curve as it stands.
type mapSave struct {
	ID      uint32
	Tile    int
	Climate temperature.Climate
	State   saveState
}

// powerSave is the saved electricity, in id order (.agents/handoffs/PLAY.md#play021_plan
// Appendix): battery charge (W·ticks), the devices and switches the
// player wants off and those actually off (a flick is pending where they
// differ), the consumers the last balance left unpowered, shed consumers
// with the tick they stay off until, thermostats, the solar flare's end
// tick (0: none) and the tick of the last balance. Nets, outputs and the
// flick index are derived, not saved.
type powerSave struct {
	Charge           []entry[int64]
	WantOff          []entity.EntityID
	IsOff            []entity.EntityID
	Unpowered        []entity.EntityID
	Shed             []entry[uint64]
	Thermo           []entry[thermoSave]
	FlareUntil, Last uint64
	Pins             []entry[uint64] // devices reconnected by hand (PWR-4): the chosen conductor
}

// thermoSave is a heater's or cooler's thermostat: its target (tenths of a
// degree C) and whether it is working (the hysteresis state).
type thermoSave struct {
	Target int
	Active bool
}

// Info describes the game for a save header.
func (g *Game) Info(name string) SaveInfo {
	alive := 0
	for _, id := range g.pawnIDs {
		if v, ok := g.vital.Get(id); ok && v.Status != health.Dead {
			alive++
		}
	}
	info := SaveInfo{Version: SaveVersion, Name: name, Date: g.Date().String(),
		Day: int(g.clock.Now()) / clock.TicksPerDay, Colonists: alive,
		MapSize: g.grid.Width(), Biome: biome.DefOf(g.biome).Name}
	if g.hasSetup() {
		s := g.ng.setup
		info.Commitment = s.SaveMode == difficulty.Commitment
		info.Storyteller = g.storyProfile().Name
		info.Difficulty = s.Preset.String()
		info.WorldSeed = s.World.SeedText
	}
	info.Ending = g.worldS().ship.Ending() // game/ship (AX-8)
	return info
}

// Save writes the whole game to w under the given save name. The first map
// is the top level, whatever map is viewed; every other open map is an
// ExtraMaps entry (MAP-0).
func (g *Game) Save(w io.Writer, name string) error {
	zw := savefmt.NewWriter(w)
	if err := EncodeSave(g, zw, name); err != nil {
		return err
	}
	return zw.Close()
}

// EncodeSave writes Save's raw gob stream, uncompressed (savefmt.Compress
// makes it a file), with a header naming only the save: reading it gives
// the colony from the name (savefmt.Info.Upgrade).
func EncodeSave(g *Game, w io.Writer, name string) error {
	return EncodeSaveAs(g, w, savefmt.Stamp{Name: name})
}

// EncodeSaveAs is EncodeSave with the client's stamp in the header: the
// slot, the colony and the build (SAV-003).
func EncodeSaveAs(g *Game, w io.Writer, stamp savefmt.Stamp) error {
	return encodeSave(g, w, stamp, g.scripts.state)
}

// CaptureSaveAs freezes the save in owned Gob bytes before simulation resumes.
func CaptureSaveAs(g *Game, stamp savefmt.Stamp, reuse []byte) (savecanon.Captured, error) {
	info, s, err := snapshotForSave(g, stamp, g.scripts.state)
	if err != nil {
		return savecanon.Captured{}, err
	}
	return savecanon.Capture(reuse, info, s)
}

// FreezeSaveAs captures one detached save on the simulation owner.
func FreezeSaveAs(g *Game, stamp savefmt.Stamp) (savecanon.Frozen, error) {
	info, s, err := snapshotForSaveMode(g, stamp, g.scripts.state, true)
	if err != nil {
		return savecanon.Frozen{}, err
	}
	return savecanon.Freeze(info, s), nil
}

// encodeSave encodes a single authoritative snapshot. StateHash uses
// the same export without serializing and decoding it first.
func encodeSave(g *Game, w io.Writer, stamp savefmt.Stamp, src modstate.Source) error {
	info, s, err := snapshotForSave(g, stamp, src)
	if err != nil {
		return err
	}
	return savecanon.Encode(w, info, s)
}

// snapshotForSave gathers the authoritative state shared by saved games
// and replay hashes. Mod callbacks run only for a nonnil source.
func snapshotForSave(g *Game, stamp savefmt.Stamp, src modstate.Source) (SaveInfo, *saveState, error) {
	return snapshotForSaveMode(g, stamp, src, false)
}

func snapshotForSaveMode(g *Game, stamp savefmt.Stamp, src modstate.Source, own bool) (SaveInfo, *saveState, error) {
	cur := g.mapState
	defer func() { g.mapState = cur }()
	g.mapState = g.maps[0]
	rng, err := g.rng.MarshalBinary()
	if err != nil {
		return SaveInfo{}, nil, fmt.Errorf("save rng: %w", err)
	}
	letterList, letterNext := g.letters.Export()
	s := &saveState{
		Calendar: g.clock.Calendar(),
		Tick:     g.clock.Now(), NextEntity: g.entities.Peek(),
		RNG: rng, Sched: g.sched, Messages: saveutil.OwnElements(own, g.messages, cloneSaveMessage),
		Letters: saveutil.OwnElements(own, letterList, saveutil.CloneLetter), NextLetter: letterNext,
		Names: saveutil.ExportStore[entry[string]](g.names), Need: saveutil.ExportStore[entry[needs.Pair]](g.need), Vital: saveutil.ExportStore[entry[health.Record]](g.vital), LaborWide: saveutil.ExportStore[entry[work.Mask]](g.labor),
		TrainingMaintenance: saveutil.OwnMap(own, g.worldS().trainingMaintenance),
		LastMeal:            saveutil.OwnMap(own, g.lastMeal), Bills: saveutil.OwnValues(own, g.craft.Bills.Export(), saveutil.CloneBills), BillEdits: saveutil.OwnMap(own, g.craft.Bills.ExportEdits()), Racks: saveutil.OwnMap(own, g.craft.Drying.Export()), Art: map[entity.EntityID]craft.Artwork{}, Fuel: saveutil.OwnMap(own, g.fuel),
		Exposure: map[entity.EntityID][2]int{}, NoComfort: saveutil.OwnMap(own, g.noComfort),
		NoFood: saveutil.OwnMap(own, g.noFood), RestedFree: saveutil.OwnMap(own, g.rest.Export()), RestNeeds: saveutil.OwnMap(own, g.rest.Needs.Export()), Calm: saveutil.OwnMap(own, g.calm), Banked: saveutil.OwnMap(own, g.banked),
		OrderQueue: map[entity.EntityID][]savedOrder{}, Worn: craft.ExportOutfits(g.apparel), Joy: saveutil.OwnMap(own, g.joy),
		PrioList: saveutil.ExportPrios(g.prio), Scheds: saveutil.OwnMap(own, g.scheds), NextThreat: g.nextThreat, ThreatsOn: g.threatsOn,
		Breaks: map[entity.EntityID]savedBreak{}, Memories: saveutil.OwnSliceValues(own, g.memories), Pending: saveutil.OwnSlice(own, g.cmds.Items()), CmdSeq: g.cmds.Last(),
		AteAt: saveutil.OwnMap(own, g.ateAt), Draft: saveutil.OwnMap(own, g.draft), SleptAt: saveutil.OwnMap(own, g.sleptAt), Equip: saveutil.OwnMap(own, g.equip),
		Carrying: g.care.Carry.Export(), TendedUntil: g.care.Doctor.ExportTended(),
		Research: g.lab.Prog.V1(), ResearchAll: g.lab.Prog.Slice(), NG121: g.exportSetupState(own), ResearchCur: g.lab.Cur, ResearchSet: g.lab.Set,
		ResearchCold: g.lab.ColdList(), Strikes: saveutil.OwnMap(own, g.lastStrike), Hits: saveutil.OwnMap(own, g.lastHit), Graves: saveutil.OwnMap(own, g.graves),
	}
	exportMapFields(g, s, own)
	eachMap(g, func() { // a sculpture stands on one map, its record is colony-wide
		for id, a := range g.craft.Art.Export() {
			s.Art[id] = a
		}
	})
	s.Landing = g.exportLanding() // setup_mapgen.go: nil for the map centre
	s.Bios21, s.Skills21, s.WorkCarry = g.exportBios(own)
	s.ContentKeys = contentKeys(g)
	s.SkillLive21 = g.exportSkillLive()
	s.Troughs = saveutil.OwnValues(own, g.troughs, cloneSaveTrough)
	for id, e := range g.exposure { // TEMP-3: the wiki scale (bodytemp.Ledger); the dead drop out
		if g.vital.Has(id) || slices.ContainsFunc(g.maps, func(m *mapState) bool { return m.animals.Has(id) }) {
			s.Exposure[id] = [2]int{int(e.Hypo), int(e.Heat)}
		}
	}
	s.AX8 = g.worldS().ship.Export() // game/ship (AX-8); an empty world state saves nothing
	for id, q := range g.orderQueue {
		for _, o := range q {
			s.OrderQueue[id] = append(s.OrderQueue[id], savedOrder{Cmd: o.cmd, Label: o.label, RetryAt: o.retryAt, Tries: o.tries})
		}
	}
	for id, b := range g.breaks {
		s.Breaks[id] = savedBreak{b.kind, b.until}
	}
	s.Logs = g.exportLogs()
	s.Social, s.People21, s.World = g.exportSocial(), g.exportPeople(own), g.exportWorld()
	if s.World != nil {
		s.World.AX9 = animalstate.CloneTravel(s.World.AX9)
		s.WorldAnimalRecords, err = animalstate.ExportWorld(worldAnimalRefs(s.World), s.World.AX9)
		if err != nil {
			return SaveInfo{}, nil, fmt.Errorf("save off-map animals: %w", err)
		}
	}
	s.WorldAnimalRecords = saveutil.DetachElements(own, s.WorldAnimalRecords, saveutil.CloneAnimalRecord)
	if !g.hasSetup() {
		s.LegacyFactions = setupview.CloneFactions(g.ng.factions)
	}
	if g.soc != nil { // AX-6
		s.Recreation = g.soc.rec.Export()
	}
	s.Bodies, s.Drugs, s.Prisoners = g.exportBodies(own), g.exportDrugs(own), g.exportPrisoners()
	s.Surgery = g.exportSurgery(own)
	s.BedRest = g.care.Rest.Export()                      // care/rest.go (PLAY-020)
	s.Mind = g.mind.Export()                              // mind (SIM-066)
	s.FeedRetry = g.care.Feed.Export()                    // care/feed.go (SIM-064)
	s.Care = g.care.Policy.Export()                       // care package (MED-03): nil at the defaults
	s.Trade = g.exportTrade()                             // trade.go (PLAY-024): nil while there is nothing to keep
	s.Diet = g.sk.diet.Export()                           // game/diet (FOOD-4): nil at the defaults
	s.Sense = saveutil.OwnSense(own, senseOf(g).Export()) // game/sense (CSE): nil at the defaults
	s.Outfits = outfitBook(g).Export()                    // game/outfit (GEAR-3): nil at the defaults
	s.PetNames = g.sk.exportPetNames()                    // skills.go (RHUD-025): nil until a rename
	s.ColonistOrder = g.sk.bar.Export()                   // game/roster (CTL-011): nil until the player moves someone
	s.History = saveutil.DetachElements(own, slices.Clone(g.history), cloneSaveHistory)
	if s.RNGStreams, err = g.exportStreams(); err != nil {
		return SaveInfo{}, nil, fmt.Errorf("save rng streams: %w", err)
	}
	for _, m := range g.maps[1:] { // MAP-0: the other open maps, in id order
		g.mapState = m
		e := mapSave{ID: m.id, Tile: m.tile, Climate: m.climate}
		exportMapFields(g, &e.State, own)
		s.ExtraMaps = append(s.ExtraMaps, e)
	}
	g.mapState = g.maps[0] // Info describes the first map
	if !g.hasSetup() && baseClimate(g) != biome.DefOf(g.biome).Climate.WithCalendar(g.clock.Calendar()) {
		cl := baseClimate(g)
		s.MapClimate = &cl
	}
	if g.view != nil {
		s.View = g.view.id
	}
	if src != nil { // what the save drops waits for TakeLoadedModState (not saved, not hashed)
		var dropped []modstate.Note
		s.ModState, dropped = modstate.Check(src.SaveBlobs())
		s.ModState = saveutil.OwnBlobs(own, s.ModState)
		g.scripts.notes = modstate.AppendNotes(g.scripts.notes, dropped)
	}
	info := g.Info(stamp.Name)
	stamp.Apply(&info)
	info.Content.Plugins = saveutil.OwnSlice(own, info.Content.Plugins)
	return info, s, nil
}

// exportMapFields writes the current map's fields of s (MAP-0): the first
// map's go in the top level, each other map's in its ExtraMaps entry.
func exportMapFields(g *Game, s *saveState, own bool) {
	s.W, s.H, s.BlockedBits = g.grid.Width(), g.grid.Height(), saveutil.OwnSlice(own, g.grid.BlockedBits())
	s.PawnIDs, s.Positions, s.Jobs = saveutil.OwnSlice(own, g.pawnIDs), saveutil.ExportStore[entry[Position]](g.positions), saveutil.DetachEntries(own, saveutil.ExportStore[entry[job.Record]](g.jobs), saveutil.CloneJob)
	s.ItemIDs, s.Stacks, s.FrameIDs, s.Frames = saveutil.OwnSlice(own, g.itemIDs), saveutil.ExportStore[entry[item.Stack]](g.stacks), saveutil.OwnSlice(own, g.frameIDs), saveutil.ExportStore[entry[construction.Frame]](g.frames)
	s.BldgIDs, s.Bldgs, s.PlantIDs, s.Plants = saveutil.OwnSlice(own, g.bldgIDs), saveutil.ExportStore[entry[construction.Building]](g.bldgs), saveutil.OwnSlice(own, g.plantIDs), saveutil.ExportStore[entry[plant.Wild]](g.plants)
	s.Terr, s.Mines = saveutil.OwnTerrain(own, g.terr), saveutil.OwnMap(own, g.mines)
	s.Postponed, s.PostSweep, s.PostOrders = g.exportPostponed(), g.postponed.SweepAt, g.exportOrders()
	s.RaiderIDs, s.Raiders, s.RaidGroups = saveutil.OwnSlice(own, g.raiderIDs), saveutil.OwnMap(own, g.raiders), saveutil.OwnElements(own, g.raidGroups, saveutil.CloneRaidGroup)
	s.RoomGrid, s.RoomTemps, s.Thermo = saveutil.OwnSlice(own, g.roomGrid), saveutil.OwnSlice(own, g.thermo.Temps), g.thermo.Export()
	if len(s.Thermo.Fine) == 0 {
		s.Thermo.Fine = nil // gob decodes an empty slice as nil
	}
	s.Claims = g.reserve.Export()
	s.AnimalIDs, s.Traces, s.Wild, s.Wildlife = saveutil.OwnSlice(own, g.animalIDs), saveutil.OwnSlice(own, g.traces), g.wild, g.wl.saved(own)
	s.Wildlife.WeightSchema = false // derived cost cache; always rebuild on load
	s.AnimalFormat, s.AnimalRecords = 1, saveutil.DetachElements(own, animalstate.Export(g.animals), saveutil.CloneAnimalRecord)
	s.RockDamage, s.Residual = saveutil.OwnMap(own, g.raid.Rock.Export()), saveutil.OwnMap(own, saveutil.NilIfEmpty(g.thermo.Residual))
	s.Motion, s.Weather, s.Roofs = g.exportMotion(), g.weather, g.exportRoofs()
	s.RoofLoss = saveutil.OwnSlice(own, g.roofx.loss)
	s.DoorsOriented = true // PLAY-011: door facings are stored
	s.Grass, s.GrassCut = g.exportGrass()
	s.GrassGrowth = g.exportGrassGrowth() // grass.go
	s.Biome, s.GrassRegrow, s.Incidents, s.Wealth = g.biome, saveutil.OwnSlice(own, g.grass.regrow), saveutil.OwnIncidents(own, g.incidents), g.wealth.Export()
	s.Stains, s.StainDrip = g.exportStains(), saveutil.OwnMap(own, g.stains.dripN)
	s.Dirt, s.Carried = g.exportDirt()
	s.Home, s.CleanWired = g.exportHome(), true
	s.NoHomeAuto = g.homeArea.off
	s.Snow, s.SnowClear = g.exportSnow() // game/snow: nil without snow or an area
	s.CleanQ, s.HomeVer = slices.Clone(g.filth.cleanQ), g.homeArea.ver
	s.Plan = g.exportPlan()     // plan.go
	s.Fog = g.exportFog()       // fog.go (MAPGEN-2): nil without fog
	s.Vaults = g.exportVaults() // vault.go (MAPGEN-3): nil without vaults
	s.Tanks = g.tanks.Export()  // game/aqua (FISH-*): nil without a tank in use
	s.Fires = g.exportFires()   // fire.go (PLAY-015): nil while nothing burns
	// pens.go (PLAY-015): marker settings, unpenned clocks
	s.PenMarkers, s.PenAnimals = saveutil.DetachElements(own, slices.Clone(g.pens), cloneSavePen), g.exportPenAnimals()
	s.Ocean = g.exportOcean()            // bridge.go (MAPGEN-3): the sea bitset, nil without a sea
	s.AX4 = g.br.ind.Export(g.bldgs.Has) // game/industry (AX-4)
	s.ZoneMap, s.Incoming, s.Deliveries, s.HarvestSweep = g.exportZones(own), saveutil.OwnMap(own, g.exportIncoming()), saveutil.OwnSliceValues(own, g.exportDeliveries()), g.haul.sweep.Export()
	s.Geo, s.Smooths, s.StructVer = saveutil.OwnGeology(own, g.geo), saveutil.OwnMap(own, g.smooths), g.structVer
	s.Orders = ordersOf(g).Export() // game/orders (AX-1)
	s.Power = g.exportPower()       // power.go (PLAY-021): nil without power
	s.Turrets, s.Shots, s.Smoke = g.combat.Turrets.Export(), g.combat.Shots.Export(), g.combat.Blasts.Export()
	s.Mechs = g.raid.Mechs.Export() // raid/mech.go (DEF-4)
	s.Areas = g.areas.Export()
}

// eachMap runs f with each open map current in turn, the first map first,
// then restores the current map.
func eachMap(g *Game, f func()) {
	cur := g.mapState
	for _, m := range g.maps {
		g.mapState = m
		f()
	}
	g.mapState = cur
}

// ErrSaveVersion means the save was written by an incompatible version.
var ErrSaveVersion = errors.New("save from an incompatible version")

// ReadSaveInfo reads just the header of a save.
func ReadSaveInfo(r io.Reader) (SaveInfo, error) { return savefmt.ReadInfo(r) }

// Load is LoadContent with core content.
func Load(r io.Reader) (*Game, SaveInfo, error) { return LoadContent(r, Content{}) }

// LoadContent restores a game written by Save, playing with c (PLG-024).
func LoadContent(r io.Reader, c Content) (*Game, SaveInfo, error) {
	zr, err := savefmt.NewReader(r)
	if err != nil {
		return nil, SaveInfo{}, fmt.Errorf("not a save file: %w", err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	if err := dec.Decode(&info); err != nil {
		return nil, info, fmt.Errorf("read save header: %w", err)
	}
	info.Upgrade()
	if !SupportedSaveVersion(info.Version) {
		return nil, info, fmt.Errorf("%w (file v%d, game v%d)", ErrSaveVersion, info.Version, SaveVersion)
	}
	var s saveState
	if err := savefmt.Finish(dec.Decode(&s), zr); err != nil { // the body decodes and the CRC holds (SAV-022)
		return nil, info, fmt.Errorf("read save state: %w", err)
	}
	if err := animalstate.CheckSave(info.Version, s.AnimalFormat, s.AnimalIDs, s.AnimalRecords, len(s.Animals)); err != nil {
		return nil, info, err
	}
	for i := range s.ExtraMaps {
		m := &s.ExtraMaps[i]
		if err := animalstate.CheckSave(info.Version, m.State.AnimalFormat, m.State.AnimalIDs, m.State.AnimalRecords, len(m.State.Animals)); err != nil {
			return nil, info, fmt.Errorf("map %d: %w", m.ID, err)
		}
		if s.AnimalFormat != m.State.AnimalFormat {
			return nil, info, fmt.Errorf("map %d: mixed grouped and legacy animal formats", m.ID)
		}
	}
	if s.World != nil {
		if err := animalstate.ImportWorld(s.AnimalFormat == 1, worldAnimalRefs(s.World), s.World.AX9, s.WorldAnimalRecords); err != nil {
			return nil, info, fmt.Errorf("restore off-map animals: %w", err)
		}
	} else if len(s.WorldAnimalRecords) != 0 {
		return nil, info, fmt.Errorf("off-map animals without a world")
	}
	if s.AnimalFormat == 1 {
		ids := make([][]entity.EntityID, 0, 1+len(s.ExtraMaps))
		ids = append(ids, s.AnimalIDs)
		for _, m := range s.ExtraMaps {
			ids = append(ids, m.State.AnimalIDs)
		}
		if err := animalstate.CheckGlobal(ids, s.WorldAnimalRecords); err != nil {
			return nil, info, err
		}
	}
	if info.Version < 21 {
		s.migrateSkillArrays()
	}
	g := New(Config{Width: s.W, Height: s.H, Content: c})
	if info.Version >= 18 { // PLG-063: an older save has no mod state
		g.scripts.loaded, g.scripts.notes = modstate.Check(s.ModState)
	}
	if err := g.rng.UnmarshalBinary(s.RNG); err != nil {
		return nil, info, fmt.Errorf("restore rng: %w", err)
	}
	g.clock = clock.NewAt(s.Tick, s.Calendar)
	g.entities = entity.Resume(s.NextEntity)
	g.sched = s.Sched
	g.messages = s.Messages
	g.names = saveutil.ImportStore(s.Names)
	g.need, g.vital, g.labor = saveutil.ImportStore(s.Need), saveutil.ImportStore(s.Vital), importLabor(s.Labor, s.LaborWide)
	saveutil.InheritArtMaskOnly(g.labor, s.LaborWide, s.PrioList)
	remapRecipes(&s, c.Recipes) // before anything reads a recipe ID
	remapItems(&s, c.Items)     // before anything reads an item kind (PLG-026)
	savefmt.MigrateConstructionStacks(&s, info.Version)
	remapTraits(&s, c.Traits)
	remapResearch(&s, c.Research)
	remapIncidents(&s, c.Incidents)
	g.cmds.Restore(s.Pending, s.CmdSeq) // a pre-#8 save's commands are stamped in order
	saveutil.SetMap(&g.lastMeal, s.LastMeal)
	g.craft.Bills.Import(s.Bills) // craft/bills.go
	g.craft.Bills.ImportEdits(s.BillEdits)
	g.craft.Drying.Import(s.Racks) // craft/drying.go
	g.craft.Art.Import(s.Art)      // craft/art.go (PRD-3)
	saveutil.SetMap(&g.fuel, s.Fuel)
	saveutil.SetMap(&g.noComfort, s.NoComfort)
	saveutil.SetMap(&g.noFood, s.NoFood)
	g.rest.Import(s.RestedFree)      // rest/sleep.go
	g.rest.Needs.Import(s.RestNeeds) // rest/needs.go (BED-3)
	saveutil.SetMap(&g.calm, s.Calm)
	saveutil.SetMap(&g.banked, s.Banked)
	g.apparel = craft.ImportOutfits(s.Worn, s.Apparel) // craft/garment.go (PRD-2)
	switch {
	case s.PrioList != nil:
		g.prio = saveutil.ImportPrios(s.PrioList)
	case s.Prios != nil:
		g.migrateWarden(s.Prios)
	default:
		g.migrateHandling(s.Prio)
	}
	saveutil.SyncWorkFlags(g.labor, g.prio, s.PrioList)
	g.nextThreat, g.threatsOn = s.NextThreat, s.ThreatsOn
	saveutil.SetMap(&g.scheds, s.Scheds)
	saveutil.SetMap(&g.joy, s.Joy)
	saveutil.SetMap(&g.memories, s.Memories)
	saveutil.SetMap(&g.ateAt, s.AteAt)
	saveutil.SetMap(&g.draft, s.Draft)
	saveutil.SetMap(&g.sleptAt, s.SleptAt)
	saveutil.SetMap(&g.equip, s.Equip)
	g.care.Carry.Import(s.Carrying)
	g.care.Doctor.ImportTended(s.TendedUntil)
	g.med.legacyWounds = s.Wounds // infection.go: migrated by migrateSave (v < 6)
	g.lab.Prog, g.lab.Cur, g.lab.Set = research.FromV1(s.Research), s.ResearchCur, s.ResearchSet
	if s.ResearchAll != nil {
		g.lab.Prog = research.FromSlice(s.ResearchAll)
	}
	g.lab.SetCold(s.ResearchCold)
	saveutil.SetMap(&g.lastStrike, s.Strikes)
	saveutil.SetMap(&g.lastHit, s.Hits)
	saveutil.SetMap(&g.graves, s.Graves)
	for id, q := range s.OrderQueue { // before importGeology's stone migration
		for _, o := range q {
			g.orderQueue[id] = append(g.orderQueue[id], queuedOrder{cmd: o.Cmd, label: o.Label, retryAt: o.RetryAt, tries: o.Tries})
		}
	}
	importMapFields(g, &s)
	if s.LaborWide == nil {
		migrateLegacyLabor(g, info.Version, s.Labor)
	}
	remapBackstories(&s, c.Backstories) // saved backstory IDs follow their keys onto c's book (PLG-044)
	g.importSetupState(s.NG121)         // setup.go: overrides the climate from the site
	if s.NG121 == nil {
		g.ng.factions = setupview.CloneFactions(s.LegacyFactions)
	}
	if s.MapClimate != nil {
		g.climate = *s.MapClimate
	}
	g.climate = g.climate.WithCalendar(g.clock.Calendar())
	g.applyClimate(uint64(max(s.Tick, 1) - 1))      // weather_ng1.go (TEMP-2): the climate event's offset as at the last tick
	g.importLanding(s.Landing)                      // setup_mapgen.go: after importSetupState
	g.importBios(s.Bios21, s.Skills21, s.WorkCarry) // skills.go: older saves get neutral bios
	g.importSkillLive(s.SkillLive21)
	saveutil.SetMap(&g.troughs, s.Troughs)
	for id, e := range s.Exposure {
		g.exposure[id] = bodytemp.Sev{Hypo: int32(min(max(e[0], -1<<31), 1<<31-1)), Heat: int32(min(max(e[1], -1<<31), 1<<31-1))}
	}
	for id, b := range s.Breaks {
		g.breaks[id] = breakState{b.Kind, b.Until}
	}
	g.importLogs(s.Logs)
	g.importSocial(s.Social)
	if s.Recreation != nil { // AX-6: after the social state it hangs off
		g.socState().rec = recreate.Load(recHost{newRestHost(g)}, s.Recreation)
	}
	g.importPeople(s.People21)
	g.importWorld(s.World)
	g.worldS().trainingMaintenance = s.TrainingMaintenance
	shipOf(g).Import(s.AX8) // game/ship (AX-8)
	g.importBodies(s.Bodies)
	g.importSurgery(s.Surgery)
	g.care.Rest.Import(s.BedRest)
	g.mind.Import(s.Mind) // mind (SIM-066)
	g.care.Feed.Import(s.FeedRetry)
	g.care.Policy.Import(s.Care) // care/policy.go (MED-03)
	g.med.animalIdxOK = false    // animal_health.go: rebuilt on first use
	g.importTrade(s.Trade)       // trade.go (PLAY-024): sessions close on load
	if s.World != nil && s.World.Version < 3 {
		rebuildVisitGroups(g) // legacy visit parties regain durable identity
	}
	g.sk.diet.Import(s.Diet) // game/diet (FOOD-4)
	g.importDrugs(s.Drugs)
	g.importPrisoners(s.Prisoners)
	senseOf(g).Import(s.Sense)      // game/sense (CSE)
	outfitBook(g).Import(s.Outfits) // game/outfit (GEAR-3): nil before v14
	g.sk.importPetNames(s.PetNames)
	g.sk.bar.Import(s.ColonistOrder) // game/roster (CTL-011): none in an older save
	g.history = s.History
	if err := g.importStreams(s.RNGStreams); err != nil {
		return nil, info, fmt.Errorf("restore rng streams: %w", err)
	}
	g.importLetters(s.Letters, s.NextLetter) // letters.go: an older save's letter messages become letters
	g.migrateSave(info.Version)
	for _, e := range s.ExtraMaps { // MAP-0: each extra map at its own size
		m := newMapState(Config{Width: e.State.W, Height: e.State.H}, e.ID)
		m.tile = e.Tile
		g.maps, g.mapState = append(g.maps, m), m
		initMapHosts(g)
		importMapFields(g, &e.State)
		g.climate = e.Climate.WithCalendar(g.clock.Calendar())
	}
	eachMap(g, func() {
		migrateConstructionBasis(g, info.Version)
		g.warmIndexes(true)
		haul.MigrateLimits(limitMigrationHost{haulHost{g}}, g.reserve)
	}) // jobindex.go: indexes, rooms and regions now, not on the first tick
	if info.Version < 21 {
		seedLegacyTrainingDecay(g) // train_animals.go: decay starts at load, not backdated
	}
	if v := mapByID(g, s.View); v != nil {
		g.view = v
	}
	g.mapState = g.view
	return g, info, nil
}

// remapRecipes moves saved recipe IDs onto book by key (PLG-044).
func remapRecipes(s *saveState, book recipe.Book) {
	to := s.ContentKeys.Recipes(book)
	moved := to.Bills(s.Bills) // each map's jobs point into the same bills
	s.Pending = to.Commands(s.Pending)
	rec := func(e *entry[job.Record]) *job.Record { return &e.V }
	savefmt.RemapJobs(to, moved, s.Jobs, rec)
	for _, e := range s.ExtraMaps {
		savefmt.RemapJobs(to, moved, e.State.Jobs, rec)
	}
}

// importMapFields restores the current map's fields of s (MAP-0), with the
// steps that bring an older save's map up to date. Load runs it for the
// first map once the colony-wide state its steps read is in, then for
// each extra map.
func importMapFields(g *Game, s *saveState) {
	g.grid = grid.Restore(s.W, s.H, s.blockedTiles())
	g.pawnIDs, g.positions, g.jobs = s.PawnIDs, saveutil.ImportStore(s.Positions), saveutil.ImportStore(s.Jobs)
	g.itemIDs, g.stacks = s.ItemIDs, saveutil.ImportStore(s.Stacks)
	g.frameIDs, g.frames = s.FrameIDs, saveutil.ImportStore(s.Frames)
	g.bldgIDs, g.bldgs = s.BldgIDs, saveutil.ImportStore(s.Bldgs)
	g.plantIDs, g.plants = s.PlantIDs, saveutil.ImportStore(s.Plants)
	g.terr = s.Terr
	g.roomGrid, g.roomDirty = s.RoomGrid, true
	g.thermo.Import(s.RoomTemps, s.Residual, s.Thermo) // game/thermo (TEMP-1)
	g.reserve = reservation.Import(s.Claims)
	g.animalIDs = s.AnimalIDs
	if s.AnimalFormat == 1 {
		g.animals = animalstate.Import(s.AnimalRecords)
	} else {
		g.animals = animalstate.New()
		for _, r := range s.Animals {
			g.animals.Set(r.ID, r.V)
		}
	}
	g.migrateAnimalBodies() // animal_health.go (MED-13): one-part bodies get their anatomy
	g.traces, g.wild = s.Traces, s.Wild
	g.wl = s.Wildlife
	// A saved hourly count may use a previous cost model.
	g.wl.WeightSchema = false
	saveutil.SetMap(&g.mines, s.Mines)
	g.importPostponed(s.Postponed)
	g.postponed.SweepAt = s.PostSweep
	g.importOrders(s.PostOrders)
	saveutil.SetMap(&g.raiders, s.Raiders)
	g.raiderIDs, g.raidGroups = s.RaiderIDs, s.RaidGroups
	g.raid.Rock.Import(s.RockDamage)          // raid/rock.go
	g.motion = saveutil.ImportStore(s.Motion) // a mover without a record reads motionOf's due-now default
	g.weather = s.Weather
	g.tanks = aqua.Import(tankHost{herdHost{g}}, s.Tanks) // game/aqua (FISH-*)
	g.afterLoadBurial()
	g.importRoofs(s.Roofs)
	g.roofx.loss = roof.NormalizeLoss(s.RoofLoss, len(g.roofs))
	g.importGrass(s.Grass, s.GrassCut)
	g.importGrassGrowth(s.GrassGrowth) // grass.go: older saves load full
	g.biome, g.climate, g.incidents = s.Biome, biome.DefOf(s.Biome).Climate, s.Incidents
	g.wealth.Import(s.Wealth) // game/wealth (EAI-0)
	g.grass.regrow = s.GrassRegrow
	g.importStains(s.Stains)
	saveutil.SetMap(&g.stains.dripN, s.StainDrip)
	g.importPlan(s.Plan)            // plan.go: older saves have no marks
	g.importFog(s.Fog)              // fog.go: older saves have no fog
	g.importVaults(s.Vaults)        // vault.go: older saves have no vaults
	g.importFires(s.Fires)          // fire.go: older saves have no fires
	g.importHome(s.Home, s.HomeVer) // home.go: an older save gets Home around its buildings
	g.homeArea.off = s.NoHomeAuto
	// game/snow (PLAY-015): an older save seeds from its global depth
	g.importSnow(s.Snow, s.SnowClear)
	g.importDirt(s.Dirt, s.Carried) // filth.go
	g.importCleanQ(s.CleanQ)        // filth.go: half-swept tiles stay queued
	if !s.CleanWired {
		cleaningOf(g).Wire() // game/sense: an older save switches cleaning on once
	}
	// bridge.go (MAPGEN-3): older saves have no sea
	g.importOcean(s.Ocean)
	g.br.ind.Import(s.AX4) // game/industry (AX-4)
	g.seedGrass()          // an older save: grass from terrain, bare under buildings
	// older saves: zone rectangles
	s.ZoneMap.legacy = legacyZones{s.Stockpiles, s.GrowZones, s.Pens}
	g.importZones(s.ZoneMap)
	g.importIncoming(s.Incoming)
	g.importDeliveries(s.Deliveries)
	g.haul.sweep.Import(s.HarvestSweep) // game/haul (HAUL-003): none in an older save
	g.afterLoadAnimals()                // legacy corpses and bills (wildlife.go)
	g.importGeology(s.Geo, s.Smooths, s.StructVer)
	ordersOf(g).Import(s.Orders) // game/orders (AX-1)
	g.importPower(s.Power)       // power.go (PLAY-021)
	g.combat.Turrets.Import(s.Turrets)
	g.combat.Shots.Import(s.Shots)
	g.combat.Blasts.Import(s.Smoke)
	g.raid.Mechs.Import(s.Mechs) // raid/mech.go (DEF-4)
	g.areas = area.Import(s.Areas)
	if !s.DoorsOriented {
		g.orientDoors() // placement.go: an older save's doors take their walls' axis (PLAY-011)
	}
	// pens.go (PLAY-015): settings and clocks; an older save's painted
	// pens become pen markers
	g.importPens(s.PenMarkers, s.PenAnimals)
	g.migratePenZones()
}

// migrateSave brings a loaded save of version v up to SaveVersion: each
// case is one version's step, and a save runs every step from its own
// version on (a new version appends a case after a fallthrough).
func (g *Game) migrateSave(v int) {
	switch {
	case v < 3:
		g.migrateClaims()       // haul_claim.go: loose chunks and carcasses become unclaimed
		g.migrateHarvestMarks() // plant_growth.go: ripe zone crops get their harvest mark
		fallthrough
	case v < 4:
		g.migrateMedicalBeds() // care/rest.go (PLAY-020): hospital beds become medical
		fallthrough
	case v < 5:
		g.migrateLampsToTorches() // PLAY-021: lamps need power now
		fallthrough
	case v < 6:
		g.migrateWoundInfections() // infection.go (MED-02): infections become conditions
		fallthrough
	case v < 7:
		g.need.All(func(id entity.EntityID, _ needs.Pair) { migrateStarvation(g, id) }) // care_host_prison.go (FOOD-1)
		// FOOD-2: plant growth units are the wiki's grow days now
		for _, id := range g.plantIDs {
			if p := g.plants.Ref(id); p != nil {
				*p = p.FromLegacyGrowth()
			}
		}
		fallthrough
	case v < 8: // TEMP-*: one block for every temperature phase's step
		for _, id := range g.pawnIDs { // TEMP-4: everyday clothes become garments
			g.apparel[id] = g.apparel[id].Everyday() // craft/garment.go
		}
		migrateExtreme(g)            // weather_ng1.go (TEMP-2): a heat wave or cold snap becomes the climate event
		g.exposure.MigratePermille() // game/bodytemp (TEMP-3): permille exposure moves to the wiki scale
		fallthrough
	case v < 9: // RSCH-1: wiki research costs (progress kept as a fraction), preset factors, bigger benches
		g.lab.Migrate()
		if g.hasSetup() {
			g.ng.setup.Difficulty = rlab.MigrateDifficulty(g.ng.setup.Difficulty)
		}
		g.migrateBenches(resize.V9, resize.RefundV9) // research.go
		fallthrough
	case v < 10: // STK-BN: wiki bench footprints
		g.migrateBenches(resize.V10, resize.RefundV10)
		fallthrough
	case v < 11: // AX-0: the AX owners' steps, empty until A2 (1x1 tables to 1x2) and A3 (stone tiles, carpets) add theirs
		construction.MigrateFloors(g.bldgs, g.frames)  // A3: stone-block floors become stone tiles
		g.migrateBenches(resize.V11, resize.RefundV10) // A2: 1x1 tables become 1x2 (taken down and refunded where they no longer fit)
		fallthrough
	case v < 12: // MAP-0: an older save has one map, nothing to change
		fallthrough
	case v < 14: // GEAR-3: no apparel policies yet: Import(nil) gave the defaults, everyone on Anything
		fallthrough
	case v < 15: // QST-1: quest rewards at 100 %, quest threats as big threats (off only on the peaceful preset)
		if g.hasSetup() {
			d := &g.ng.setup.Difficulty
			d.QuestRewardPct, d.QuestThreats = 100, d.BigThreats
		}
		fallthrough
	case v < 16: // SAV-003: the header names its colony; Load's savefmt.Info.Upgrade fills an older one, the state is unchanged
		fallthrough
	case v < 17: // EAI-015: restore faction wars for older saves
		faction.SetEnemies(g.ng.factions)
		fallthrough
	case v < 20: // PLG-063 (v18): no mod state, mods start fresh; DEF 3.1 (v19): damage taken and log kinds start at zero; SHT-007 (v20): friendly fire as before, 100 %
		if g.hasSetup() {
			g.ng.setup.Difficulty.FriendlyFire = 100
		}
	}
	g.med.legacyWounds = nil // read only: a v6 save never writes Wounds
}

// torchFullFuel is a full tank of a torch lamp, in ticks of fuel (the
// torch row of the fuel table, fuel.go).
func torchFullFuel() int {
	spec, _ := fuelSpec(construction.TorchLamp)
	return spec.CapTicks
}

// migrateLampsToTorches replaces legacy lamps with fueled torch lamps,
// retaining identity, placement, hit points and fractional build progress.
func (g *Game) migrateLampsToTorches() {
	torch := construction.TorchLamp
	changed := false
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok || b.Kind != construction.Lamp {
			continue
		}
		b.Kind = torch
		g.bldgs.Set(id, b)
		g.setFuel(id, torchFullFuel())
		changed = true
	}
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if !ok || fr.Kind != construction.Lamp {
			continue
		}
		if fr.Reinstall == 0 { // a move keeps its own work and no wood
			was := max(1, construction.WorkFor(construction.Lamp, fr.Stuff))
			fr.WorkLeft = (fr.WorkLeft*construction.WorkFor(torch, fr.Stuff) + was/2) / was
			fr.Need = construction.WoodCost(torch)
			fr.Wood = min(fr.Wood, fr.Need)
		}
		fr.Kind = torch
		g.frames.Set(id, fr)
		changed = true
	}
	if changed {
		g.structVer++ // SIM-029: link and tint caches, the light grid
		g.light.ver++ // light.go: the lamps' light goes with the lamps
	}
}

// Skill save wire types live beside their historical 11-slot counterparts.
type (
	savedSkillLive = savelegacy.CurrentSkillLive
	savedLearn     = savelegacy.CurrentLearn
	savedNeedCarry = savelegacy.NeedCarry
	savedWorking   = savelegacy.Working
)

// exportSkillLive is the skill state beyond bios, skills and work carry
// (nil when there is none).
func (g *Game) exportSkillLive() *savedSkillLive {
	sk := &g.sk
	if len(sk.learned) == 0 && len(sk.carry) == 0 && len(sk.working) == 0 && g.lab.Carry == 0 {
		return nil
	}
	out := &savedSkillLive{Research: g.lab.Carry}
	if len(sk.learned) > 0 {
		out.Learned = make(map[entity.EntityID]savedLearn, len(sk.learned))
		for id, l := range sk.learned {
			out.Learned[id] = savedLearn{Day: l.Day, Today: l.Today, Week: l.Week, Carry: l.Carry}
		}
	}
	if len(sk.carry) > 0 {
		out.NeedCarry = make(map[entity.EntityID]savedNeedCarry, len(sk.carry))
		for id, c := range sk.carry {
			out.NeedCarry[id] = savedNeedCarry{Hunger: c.hunger, Rest: c.rest, Joy: c.joy}
		}
	}
	if len(sk.working) > 0 {
		out.Working = make(map[entity.EntityID]savedWorking, len(sk.working))
		for id, w := range sk.working {
			out.Working[id] = savedWorking{Skill: w.sk, Tick: w.tick}
		}
	}
	return out
}

// importSkillLive restores exportSkillLive's state (after importBios).
func (g *Game) importSkillLive(s *savedSkillLive) {
	if s == nil {
		return
	}
	g.sk.ensure()
	for id, l := range s.Learned {
		g.sk.learned[id] = &learnLog{Day: l.Day, Today: l.Today, Week: l.Week, Carry: l.Carry}
	}
	for id, c := range s.NeedCarry {
		g.sk.carry[id] = &needCarry{c.Hunger, c.Rest, c.Joy}
	}
	for id, w := range s.Working {
		g.sk.working[id] = workingRec{w.Skill, w.Tick}
	}
	g.lab.Carry = s.Research
}

// migrateSkillArrays upgrades the historical v1-v20 wire records once.
func (s *saveState) migrateSkillArrays() {
	if s.Bios != nil {
		s.Bios21 = savelegacy.Bios(s.Bios)
	}
	if s.Skills != nil {
		s.Skills21 = savelegacy.Skills(s.Skills)
	}
	if s.SkillLive != nil {
		s.SkillLive21 = s.SkillLive.Current()
	}
	if n := s.NG1; n != nil {
		o := n.Setup
		s.NG121 = &SetupState{
			Setup: Setup{Scenario: o.Scenario, Storyteller: o.Storyteller, Preset: o.Preset,
				Difficulty: o.Difficulty.Current(), SaveMode: o.SaveMode, World: o.World,
				Tile: o.Tile, Site: o.Site, Factions: o.Factions, MapSize: o.MapSize,
				Season: Season(o.Season), Colonists: savelegacy.Pawns(o.Colonists),
				Seed: o.Seed, GenVersion: o.GenVersion, GoneAt: o.GoneAt, WandererGroups: o.WandererGroups},
			Factions: n.Factions, Story: n.Story, Plan: n.Plan, Adapt: difficulty.Adapter{Days: n.Adapt.Days, Value: n.Adapt.Value},
			Climate: n.Climate, StartDay: n.StartDay, Dead: n.Dead, Lost: n.Lost,
			Extreme: extremeEvent(n.Extreme), IntroDone: n.IntroDone,
		}
		for _, p := range n.Pods {
			s.NG121.Pods = append(s.NG121.Pods, podState(p))
		}
	}
	if p := s.People; p != nil {
		s.People21 = &peopleSave{Version: p.Version, Seeded: p.Seeded}
		for _, v := range p.Persons {
			person := personSave{ID: v.ID, Faction: v.Faction, Origin: v.Origin, Where: v.Where,
				DiedAt: v.DiedAt, KilledBy: v.KilledBy, Parents: v.Parents}
			if v.Bio != nil {
				bio := v.Bio.Current()
				person.Bio = &bio
			}
			s.People21.Persons = append(s.People21.Persons, person)
		}
	}
	s.Bios, s.Skills, s.SkillLive, s.NG1, s.People = nil, nil, nil, nil, nil
	for i := range s.ExtraMaps {
		s.ExtraMaps[i].State.migrateSkillArrays()
	}
}

// blockedTiles is the blocked tile list, from whichever field the save
// carries it in (BlockedBits from v2 on).
func (s *saveState) blockedTiles() []int {
	if s.BlockedBits == nil {
		return s.Blocked
	}
	return saveutil.BitIndexes(s.BlockedBits)
}

// migrateLegacyLabor reads the old field and fills only the added columns.
func migrateLegacyLabor(g *Game, version int, old []entry[legacyLaborMask]) {
	for _, e := range old {
		prev, had := g.labor.Get(e.ID)
		p, explicit := g.prio[e.ID]
		p = work.LegacyPriorities(version, uint16(e.V), prev, had, p, explicit)
		g.prio[e.ID] = p
		g.labor.Set(e.ID, p.Mask())
	}
}

// legacyPriorities is work.Priorities before the Handle work type.
type legacyPriorities [work.Handle]uint8

// priorities12 is work.Priorities before the Warden work type (ENG-003).
type priorities12 [work.Warden]uint8

// migrateWarden loads the priorities of a save from before the Warden work
// type (12 types): every work type keeps its priority, Warden is off.
func (g *Game) migrateWarden(old map[entity.EntityID]priorities12) {
	for id, op := range old {
		var p work.Priorities
		copy(p[:], op[:])
		p[work.PlantCut], p[work.Art] = p[work.Grow], p[work.Craft] // FOOD-2, WRK-108
		g.prio[id] = p
	}
}

// migrateHandling loads the priorities of a save from before animal
// handling: every work type keeps its priority, and Handle (taming moved
// there from Hunt) takes Hunt's, in priorities and enable masks.
func (g *Game) migrateHandling(old map[entity.EntityID]legacyPriorities) {
	for id, lp := range old {
		var p work.Priorities
		copy(p[:], lp[:])
		p[work.Handle], p[work.PlantCut], p[work.Art] = p[work.Hunt], p[work.Grow], p[work.Craft]
		g.prio[id] = p
	}
	for _, id := range g.pawnIDs {
		if m, ok := g.labor.Get(id); ok {
			g.labor.Set(id, m.Set(work.Handle, m.Enabled(work.Hunt)))
		}
	}
}

// importGeology installs saved geology and smoothing marks (SIM-029). An
// older save has none: its geology is generated from a hash of its
// terrain (deterministic, never the sim RNG) and its legacy stone is
// migrated to granite blocks.
func (g *Game) importGeology(geo geology.Map, smooths map[[2]int]bool, ver uint64) {
	saveutil.SetMap(&g.smooths, smooths)
	g.structVer = ver
	g.migrateLegacyStone() // a no-op on newer saves
	if geo.W == g.terr.W && geo.H == g.terr.H && geo.W > 0 {
		g.geo = geo
		return
	}
	g.geo = geology.Generate(saveutil.TerrainHash(g.terr), g.terr)
	g.structVer++
}

// migrateLegacyStone rewrites the generic stone of older saves (no longer
// a building material) as granite blocks, which keep its numbers: stacks,
// building and frame materials, and build commands waiting to apply, in
// order queues or being followed.
func (g *Game) migrateLegacyStone() {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.Stone {
			st.Kind = item.GraniteBlocks
			g.putStack(id, st)
		}
	}
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && b.Stuff == item.Stone {
			b.Stuff = item.GraniteBlocks
			g.bldgs.Set(id, b)
		}
	}
	for _, id := range g.frameIDs {
		if fr, ok := g.frames.Get(id); ok && fr.Stuff == item.Stone {
			fr.Stuff = item.GraniteBlocks
			g.frames.Set(id, fr)
		}
	}
	fix := func(c *command.Command) {
		if c.Stuff == uint8(item.Stone) {
			c.Stuff = uint8(item.GraniteBlocks)
		}
	}
	for i := range g.cmds.Items() { // in place: payload only, the envelope stays
		fix(&g.cmds.Items()[i])
	}
	for _, q := range g.orderQueue { // in-place edits: order never matters
		for i := range q {
			fix(&q[i].cmd)
		}
	}
	for id, o := range g.postponed.orders {
		fix(&o.cmd)
		g.postponed.orders[id] = o
	}
}
