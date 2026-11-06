package game

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	mathbits "math/bits"
	"rimworld-go/internal/application/game/aqua"
	"rimworld-go/internal/application/game/bodytemp"
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/application/game/industry"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/application/game/orders"
	"rimworld-go/internal/application/game/outfit"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/recreate"
	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/application/game/resize"
	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/application/game/thermo"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/world/fire"
	"slices"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/stain"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/area"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/plant"
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
// Anything.
// migrateSave runs the steps a save's version is missing.

// SaveVersion is the version new saves are written with.
const SaveVersion = 14

// minSaveVersion is the oldest version Load still reads.
const minSaveVersion = 1

// SupportedSaveVersion reports whether Load can read a save of version v.
func SupportedSaveVersion(v int) bool { return v >= minSaveVersion && v <= SaveVersion }

// SaveInfo is the header of a save: enough to list it without loading.
// The fields after Colonists are v2 (zero in a v1 header).
type SaveInfo struct {
	Version   int
	Name      string // colony / save name
	Date      string // in-game date
	Day       int    // days since landing
	Colonists int    // alive
	// Commitment: one save per colony, written by the game itself
	// (NG-1 §2.13); Storyteller and Difficulty name the colony's ("" for a
	// legacy colony); MapSize is the map side; Biome the biome's name.
	Commitment  bool
	Storyteller string
	Difficulty  string
	MapSize     int
	Biome       string
	// Ending (AX-8) is "ship" once the colony's ship has left: the
	// save's story is complete ("" while it goes on).
	Ending string
}

type entry[T any] struct {
	ID entity.EntityID
	V  T
}

func exportStore[T any](s *component.Store[T]) []entry[T] {
	out := make([]entry[T], 0, s.Len())
	s.All(func(id entity.EntityID, v T) { out = append(out, entry[T]{id, v}) })
	return out
}

func importStore[T any](es []entry[T]) *component.Store[T] {
	s := component.NewStore[T]()
	for _, e := range es {
		s.Set(e.ID, e.V)
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
	Letters    []savedLetter
	NextLetter uint64

	PawnIDs   []entity.EntityID
	Positions []entry[Position]
	Names     []entry[string]
	Jobs      []entry[job.Record]
	Need      []entry[needs.Pair]
	Vital     []entry[health.Record]
	Labor     []entry[work.Mask]

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
	Racks      map[entity.EntityID]craft.Batch
	Art        map[entity.EntityID]craft.Artwork // PRD-3: each sculpture's title seed and carver; absent in older saves
	Fuel       map[entity.EntityID]int
	Exposure   map[entity.EntityID][2]int
	NoComfort  map[entity.EntityID]uint64
	NoFood     map[entity.EntityID]uint64     // absent in older saves
	Postponed  []postEntry                    // absent in older saves (SIM-027)
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

	AteAt   map[entity.EntityID]bool
	Draft   map[entity.EntityID]bool
	SleptAt map[entity.EntityID]entity.EntityID

	AnimalIDs []entity.EntityID
	Animals   []entry[animal.State]
	Equip     map[entity.EntityID]item.Kind
	Traces    []projectile.Trace
	Wild      bool
	Wildlife  wildlifeState

	Carrying    map[entity.EntityID]entity.EntityID
	TendedUntil map[entity.EntityID]uint64
	Wounds      map[entity.EntityID]legacyWound // read only: saves up to v5 (migrateWoundInfections)
	// Research is the first five projects (the pre-NG-1 array, which old
	// saves decode); ResearchAll is every project and wins when present.
	Research    research.V1
	ResearchAll []int
	// NG1 is the new-game setup and its live state (scenario, storyteller,
	// difficulty, factions, climate, conditions, arrival); nil = legacy.
	NG1 *SetupState
	// DoorsOriented: door facings are stored (PLAY-011); a save without it
	// re-derives every door's facing from its neighbours on load.
	DoorsOriented bool
	// Bios, Skills, WorkCarry (NG-1 §5.4): each colonist's identity, skills
	// and fractional work progress; absent in older saves (neutral bios).
	Bios      map[entity.EntityID]pawngen.Pawn
	Skills    map[entity.EntityID]skill.Set
	WorkCarry map[entity.EntityID]int32
	// SkillLive (v2): the rest of the skill state that moves the game —
	// learning saturation, fractional need drains, the last skilled work
	// (passion joy) and fractional research; absent in older saves.
	SkillLive   *savedSkillLive
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
	Roofs []uint8
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
	People     *peopleSave
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
	}
	info.Ending = g.worldS().ship.Ending() // game/ship (AX-8)
	return info
}

// Save writes the whole game to w under the given save name. The first map
// is the top level, whatever map is viewed; every other open map is an
// ExtraMaps entry (MAP-0).
func (g *Game) Save(w io.Writer, name string) error {
	cur := g.mapState
	defer func() { g.mapState = cur }()
	g.mapState = g.maps[0]
	rng, err := g.rng.MarshalBinary()
	if err != nil {
		return fmt.Errorf("save rng: %w", err)
	}
	s := saveState{
		Tick: g.clock.Now(), NextEntity: g.entities.Peek(),
		RNG: rng, Sched: g.sched, Messages: g.messages,
		Letters: g.letters.list, NextLetter: g.letters.next,
		Names: exportStore(g.names), Need: exportStore(g.need), Vital: exportStore(g.vital), Labor: exportStore(g.labor),
		LastMeal: g.lastMeal, Bills: g.craft.Bills.Export(), Racks: g.craft.Drying.Export(), Art: map[entity.EntityID]craft.Artwork{}, Fuel: g.fuel,
		Exposure: map[entity.EntityID][2]int{}, NoComfort: g.noComfort,
		NoFood: g.noFood, RestedFree: g.rest.Export(), RestNeeds: g.rest.Needs.Export(), Calm: g.calm, Banked: g.banked,
		OrderQueue: map[entity.EntityID][]savedOrder{}, Worn: craft.ExportOutfits(g.apparel), Joy: g.joy,
		PrioList: exportPrios(g.prio), Scheds: g.scheds, NextThreat: g.nextThreat, ThreatsOn: g.threatsOn,
		Breaks: map[entity.EntityID]savedBreak{}, Memories: g.memories, Pending: g.pending,
		AteAt: g.ateAt, Draft: g.draft, SleptAt: g.sleptAt, Equip: g.equip,
		Carrying: g.care.Carry.Export(), TendedUntil: g.care.Doctor.ExportTended(),
		Research: g.lab.Prog.V1(), ResearchAll: g.lab.Prog.Slice(), NG1: g.exportSetupState(), ResearchCur: g.lab.Cur, ResearchSet: g.lab.Set,
		ResearchCold: g.lab.ColdList(), Strikes: g.lastStrike, Hits: g.lastHit, Graves: g.graves,
	}
	exportMapFields(g, &s)
	eachMap(g, func() { // a sculpture stands on one map, its record is colony-wide
		for id, a := range g.craft.Art.Export() {
			s.Art[id] = a
		}
	})
	s.Landing = g.exportLanding() // setup_mapgen.go: nil for the map centre
	s.Bios, s.Skills, s.WorkCarry = g.exportBios()
	s.SkillLive = g.exportSkillLive()
	s.Troughs = g.troughs
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
	s.Social, s.People, s.World = g.exportSocial(), g.exportPeople(), g.exportWorld()
	if g.soc != nil { // AX-6
		s.Recreation = g.soc.rec.Export()
	}
	s.Bodies, s.Drugs, s.Prisoners = g.exportBodies(), g.exportDrugs(), g.exportPrisoners()
	s.Surgery = g.exportSurgery()
	s.BedRest = g.care.Rest.Export()    // care/rest.go (PLAY-020)
	s.Mind = g.mind.Export()            // mind (SIM-066)
	s.FeedRetry = g.care.Feed.Export()  // care/feed.go (SIM-064)
	s.Care = g.care.Policy.Export()     // care package (MED-03): nil at the defaults
	s.Trade = g.exportTrade()           // trade.go (PLAY-024): nil while there is nothing to keep
	s.Diet = g.sk.diet.Export()         // game/diet (FOOD-4): nil at the defaults
	s.Sense = senseOf(g).Export()       // game/sense (CSE): nil at the defaults
	s.Outfits = outfitBook(g).Export()  // game/outfit (GEAR-3): nil at the defaults
	s.PetNames = g.sk.exportPetNames()  // skills.go (RHUD-025): nil until a rename
	s.ColonistOrder = g.sk.bar.Export() // game/roster (CTL-011): nil until the player moves someone
	s.History = slices.Clone(g.history)
	if s.RNGStreams, err = g.exportStreams(); err != nil {
		return fmt.Errorf("save rng streams: %w", err)
	}
	for _, m := range g.maps[1:] { // MAP-0: the other open maps, in id order
		g.mapState = m
		e := mapSave{ID: m.id, Tile: m.tile, Climate: m.climate}
		exportMapFields(g, &e.State)
		s.ExtraMaps = append(s.ExtraMaps, e)
	}
	g.mapState = g.maps[0] // Info describes the first map
	if g.view != nil {
		s.View = g.view.id
	}
	// BestSpeed: a fifth of the default level's time for a file about a
	// sixth larger (NG-1 §6.8: a 500² save in ≤ 300 ms, ≤ 4 MB).
	zw, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(g.Info(name)); err != nil {
		return fmt.Errorf("save header: %w", err)
	}
	if err := enc.Encode(&s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return zw.Close()
}

// exportMapFields writes the current map's fields of s (MAP-0): the first
// map's go in the top level, each other map's in its ExtraMaps entry.
func exportMapFields(g *Game, s *saveState) {
	s.W, s.H, s.BlockedBits = g.grid.Width(), g.grid.Height(), g.grid.BlockedBits()
	s.PawnIDs, s.Positions, s.Jobs = g.pawnIDs, exportStore(g.positions), exportStore(g.jobs)
	s.ItemIDs, s.Stacks, s.FrameIDs, s.Frames = g.itemIDs, exportStore(g.stacks), g.frameIDs, exportStore(g.frames)
	s.BldgIDs, s.Bldgs, s.PlantIDs, s.Plants = g.bldgIDs, exportStore(g.bldgs), g.plantIDs, exportStore(g.plants)
	s.Terr, s.Mines = g.terr, g.mines
	s.Postponed, s.PostSweep, s.PostOrders = g.exportPostponed(), g.postponed.sweepAt, g.exportOrders()
	s.RaiderIDs, s.Raiders, s.RaidGroups = g.raiderIDs, g.raiders, g.raidGroups
	s.RoomGrid, s.RoomTemps, s.Thermo = g.roomGrid, g.thermo.Temps, g.thermo.Export()
	s.Claims = g.reserve.Export()
	s.AnimalIDs, s.Animals, s.Traces, s.Wild, s.Wildlife = g.animalIDs, exportStore(g.animals), g.traces, g.wild, g.wl
	s.RockDamage, s.Residual = g.raid.Rock.Export(), g.thermo.Residual
	s.Motion, s.Weather, s.Roofs = g.exportMotion(), g.weather, g.exportRoofs()
	s.DoorsOriented = true // PLAY-011: door facings are stored
	s.Grass, s.GrassCut = g.exportGrass()
	s.GrassGrowth = g.exportGrassGrowth() // grass.go
	s.Biome, s.GrassRegrow, s.Incidents = g.biome, g.grass.regrow, g.incidents
	s.Stains, s.StainDrip = g.exportStains(), g.stains.dripN
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
	s.PenMarkers, s.PenAnimals = slices.Clone(g.pens), g.exportPenAnimals()
	s.Ocean = g.exportOcean()            // bridge.go (MAPGEN-3): the sea bitset, nil without a sea
	s.AX4 = g.br.ind.Export(g.bldgs.Has) // game/industry (AX-4)
	s.ZoneMap, s.Incoming, s.Deliveries, s.HarvestSweep = g.exportZones(), g.exportIncoming(), g.exportDeliveries(), g.haul.sweep.Export()
	s.Geo, s.Smooths, s.StructVer = g.geo, g.smooths, g.structVer
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
func ReadSaveInfo(r io.Reader) (SaveInfo, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return SaveInfo{}, fmt.Errorf("not a save file: %w", err)
	}
	var info SaveInfo
	if err := gob.NewDecoder(zr).Decode(&info); err != nil {
		return SaveInfo{}, fmt.Errorf("read save header: %w", err)
	}
	return info, nil
}

// Load restores a game written by Save.
func Load(r io.Reader) (*Game, SaveInfo, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, SaveInfo{}, fmt.Errorf("not a save file: %w", err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	if err := dec.Decode(&info); err != nil {
		return nil, info, fmt.Errorf("read save header: %w", err)
	}
	if !SupportedSaveVersion(info.Version) {
		return nil, info, fmt.Errorf("%w (file v%d, game v%d)", ErrSaveVersion, info.Version, SaveVersion)
	}
	var s saveState
	if err := dec.Decode(&s); err != nil {
		return nil, info, fmt.Errorf("read save state: %w", err)
	}
	g := New(Config{Width: s.W, Height: s.H})
	if err := g.rng.UnmarshalBinary(s.RNG); err != nil {
		return nil, info, fmt.Errorf("restore rng: %w", err)
	}
	g.clock = clock.NewAt(s.Tick)
	g.entities = entity.Resume(s.NextEntity)
	g.sched = s.Sched
	g.messages = s.Messages
	g.names = importStore(s.Names)
	g.need, g.vital, g.labor = importStore(s.Need), importStore(s.Vital), importStore(s.Labor)
	g.pending = s.Pending
	setMap(&g.lastMeal, s.LastMeal)
	g.craft.Bills.Import(s.Bills)  // craft/bills.go
	g.craft.Drying.Import(s.Racks) // craft/drying.go
	g.craft.Art.Import(s.Art)      // craft/art.go (PRD-3)
	setMap(&g.fuel, s.Fuel)
	setMap(&g.noComfort, s.NoComfort)
	setMap(&g.noFood, s.NoFood)
	g.rest.Import(s.RestedFree)      // rest/sleep.go
	g.rest.Needs.Import(s.RestNeeds) // rest/needs.go (BED-3)
	setMap(&g.calm, s.Calm)
	setMap(&g.banked, s.Banked)
	g.apparel = craft.ImportOutfits(s.Worn, s.Apparel) // craft/garment.go (PRD-2)
	switch {
	case s.PrioList != nil:
		g.prio = importPrios(s.PrioList)
	case s.Prios != nil:
		g.migrateWarden(s.Prios)
	default:
		g.migrateHandling(s.Prio)
	}
	for id, p := range g.prio { // FOOD-2: plant cutting's enable bit follows its priority (older saves)
		if m, ok := g.labor.Get(id); ok {
			g.labor.Set(id, m.Set(work.PlantCut, p[work.PlantCut] > 0))
		}
	}
	g.nextThreat, g.threatsOn = s.NextThreat, s.ThreatsOn
	setMap(&g.scheds, s.Scheds)
	setMap(&g.joy, s.Joy)
	setMap(&g.memories, s.Memories)
	setMap(&g.ateAt, s.AteAt)
	setMap(&g.draft, s.Draft)
	setMap(&g.sleptAt, s.SleptAt)
	setMap(&g.equip, s.Equip)
	g.care.Carry.Import(s.Carrying)
	g.care.Doctor.ImportTended(s.TendedUntil)
	g.med.legacyWounds = s.Wounds // infection.go: migrated by migrateSave (v < 6)
	g.lab.Prog, g.lab.Cur, g.lab.Set = research.FromV1(s.Research), s.ResearchCur, s.ResearchSet
	if s.ResearchAll != nil {
		g.lab.Prog = research.FromSlice(s.ResearchAll)
	}
	g.lab.SetCold(s.ResearchCold)
	setMap(&g.lastStrike, s.Strikes)
	setMap(&g.lastHit, s.Hits)
	setMap(&g.graves, s.Graves)
	for id, q := range s.OrderQueue { // before importGeology's stone migration
		for _, o := range q {
			g.orderQueue[id] = append(g.orderQueue[id], queuedOrder{cmd: o.Cmd, label: o.Label, retryAt: o.RetryAt, tries: o.Tries})
		}
	}
	importMapFields(g, &s)
	g.importSetupState(s.NG1)                   // setup.go: overrides the climate from the site
	g.applyClimate(uint64(max(s.Tick, 1) - 1))  // weather_ng1.go (TEMP-2): the climate event's offset as at the last tick
	g.importLanding(s.Landing)                  // setup_mapgen.go: after importSetupState
	g.importBios(s.Bios, s.Skills, s.WorkCarry) // skills.go: older saves get neutral bios
	g.importSkillLive(s.SkillLive)
	setMap(&g.troughs, s.Troughs)
	for id, e := range s.Exposure {
		g.exposure[id] = bodytemp.Sev{Hypo: int32(e[0]), Heat: int32(e[1])}
	}
	for id, b := range s.Breaks {
		g.breaks[id] = breakState{b.Kind, b.Until}
	}
	g.importLogs(s.Logs)
	g.importSocial(s.Social)
	if s.Recreation != nil { // AX-6: after the social state it hangs off
		g.socState().rec = recreate.Load(recHost{newRestHost(g)}, s.Recreation)
	}
	g.importPeople(s.People)
	g.importWorld(s.World)
	shipOf(g).Import(s.AX8) // game/ship (AX-8)
	g.importBodies(s.Bodies)
	g.importSurgery(s.Surgery)
	g.care.Rest.Import(s.BedRest)
	g.mind.Import(s.Mind) // mind (SIM-066)
	g.care.Feed.Import(s.FeedRetry)
	g.care.Policy.Import(s.Care) // care/policy.go (MED-03)
	g.med.animalIdxOK = false    // animal_health.go: rebuilt on first use
	g.importTrade(s.Trade)       // trade.go (PLAY-024): sessions close on load
	g.sk.diet.Import(s.Diet)     // game/diet (FOOD-4)
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
		g.climate = e.Climate
	}
	eachMap(g, func() { g.warmIndexes(true) }) // jobindex.go: indexes, rooms and regions now, not on the first tick
	if v := mapByID(g, s.View); v != nil {
		g.view = v
	}
	g.mapState = g.view
	return g, info, nil
}

// importMapFields restores the current map's fields of s (MAP-0), with the
// steps that bring an older save's map up to date. Load runs it for the
// first map once the colony-wide state its steps read is in, then for
// each extra map.
func importMapFields(g *Game, s *saveState) {
	g.grid = grid.Restore(s.W, s.H, s.blockedTiles())
	g.pawnIDs, g.positions, g.jobs = s.PawnIDs, importStore(s.Positions), importStore(s.Jobs)
	g.itemIDs, g.stacks = s.ItemIDs, importStore(s.Stacks)
	g.frameIDs, g.frames = s.FrameIDs, importStore(s.Frames)
	g.bldgIDs, g.bldgs = s.BldgIDs, importStore(s.Bldgs)
	g.plantIDs, g.plants = s.PlantIDs, importStore(s.Plants)
	g.terr = s.Terr
	g.roomGrid, g.roomDirty = s.RoomGrid, true
	g.thermo.Import(s.RoomTemps, s.Residual, s.Thermo) // game/thermo (TEMP-1)
	g.reserve = reservation.Import(s.Claims)
	g.animalIDs, g.animals = s.AnimalIDs, importStore(s.Animals)
	g.migrateAnimalBodies() // animal_health.go (MED-13): one-part bodies get their anatomy
	g.traces, g.wild = s.Traces, s.Wild
	g.wl = s.Wildlife
	setMap(&g.mines, s.Mines)
	g.importPostponed(s.Postponed)
	g.postponed.sweepAt = s.PostSweep
	g.importOrders(s.PostOrders)
	setMap(&g.raiders, s.Raiders)
	g.raiderIDs, g.raidGroups = s.RaiderIDs, s.RaidGroups
	g.raid.Rock.Import(s.RockDamage) // raid/rock.go
	g.motion = importStore(s.Motion) // a mover without a record reads motionOf's due-now default
	g.weather = s.Weather
	g.tanks = aqua.Import(tankHost{herdHost{g}}, s.Tanks) // game/aqua (FISH-*)
	g.afterLoadBurial()
	g.importRoofs(s.Roofs)
	g.importGrass(s.Grass, s.GrassCut)
	g.importGrassGrowth(s.GrassGrowth) // grass.go: older saves load full
	g.biome, g.climate, g.incidents = s.Biome, biome.DefOf(s.Biome).Climate, s.Incidents
	g.grass.regrow = s.GrassRegrow
	g.importStains(s.Stains)
	setMap(&g.stains.dripN, s.StainDrip)
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
	}
	g.med.legacyWounds = nil // read only: a v6 save never writes Wounds
}

// torchFullFuel is a full tank of a torch lamp, in ticks of fuel (the
// torch row of the fuel table, fuel.go).
func torchFullFuel() int {
	spec, _ := fuelSpec(construction.TorchLamp)
	return spec.CapTicks
}

// migrateLampsToTorches (save v5, PLAY-021) turns every lamp of an older
// save into a torch lamp in place: same id, cell, material and hit points,
// with a full tank of logs. Lamps need power from now on, and an old
// colony has none, so it would sit in the dark; a torch lamp burns logs
// instead and haulers keep it fed. A lamp blueprint becomes a torch lamp
// blueprint with the logs it has already been given (capped at what a
// torch lamp takes) and the same share of work left. Lamp and torch lamp
// share a layer, so the cell indexes stay valid.
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

// savedSkillLive mirrors the skill state skills.go keeps beside the bios
// (skillState's learned, carry and working; Research is game/research's
// carried fraction of a point, billionths from v9, hundredths before).
type savedSkillLive struct {
	Learned   map[entity.EntityID]savedLearn
	NeedCarry map[entity.EntityID]savedNeedCarry
	Working   map[entity.EntityID]savedWorking
	Research  int
}

type savedLearn struct {
	Day   int32
	Today [skill.NumSkills]int32
	Week  [7][skill.NumSkills]int32
	Carry [skill.NumSkills]int16
}

type savedNeedCarry struct{ Hunger, Rest, Joy int32 }

type savedWorking struct {
	Skill skill.ID
	Tick  uint64
}

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
			out.Learned[id] = savedLearn{Day: l.day, Today: l.today, Week: l.week, Carry: l.carry}
		}
	}
	if len(sk.carry) > 0 {
		out.NeedCarry = make(map[entity.EntityID]savedNeedCarry, len(sk.carry))
		for id, c := range sk.carry {
			out.NeedCarry[id] = savedNeedCarry{c.hunger, c.rest, c.joy}
		}
	}
	if len(sk.working) > 0 {
		out.Working = make(map[entity.EntityID]savedWorking, len(sk.working))
		for id, w := range sk.working {
			out.Working[id] = savedWorking{w.sk, w.tick}
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
		g.sk.learned[id] = &learnLog{day: l.Day, today: l.Today, week: l.Week, carry: l.Carry}
	}
	for id, c := range s.NeedCarry {
		g.sk.carry[id] = &needCarry{c.Hunger, c.Rest, c.Joy}
	}
	for id, w := range s.Working {
		g.sk.working[id] = workingRec{w.Skill, w.Tick}
	}
	g.lab.Carry = s.Research
}

// blockedTiles is the blocked tile list, from whichever field the save
// carries it in (BlockedBits from v2 on).
func (s *saveState) blockedTiles() []int {
	if s.BlockedBits == nil {
		return s.Blocked
	}
	return bitIndexes(s.BlockedBits)
}

// bitIndexes lists the set bits of a bitset in ascending order.
func bitIndexes(bits []uint64) []int {
	n := 0
	for _, w := range bits {
		n += mathbits.OnesCount64(w)
	}
	out := make([]int, 0, n)
	for wi, w := range bits {
		for w != 0 {
			out = append(out, wi*64+mathbits.TrailingZeros64(w))
			w &= w - 1
		}
	}
	return out
}

// setMap installs a decoded map, keeping the empty one gob leaves nil.
func setMap[K comparable, V any](dst *map[K]V, src map[K]V) {
	if src != nil {
		*dst = src
	}
}

// legacyPriorities is work.Priorities before the Handle work type.
type legacyPriorities [work.Handle]uint8

// priorities12 is work.Priorities before the Warden work type (ENG-003).
type priorities12 [work.Warden]uint8

// exportPrios is the PrioList save field: each colonist's priorities as a
// plain slice in work.Type order.
func exportPrios(m map[entity.EntityID]work.Priorities) map[entity.EntityID][]uint8 {
	out := make(map[entity.EntityID][]uint8, len(m))
	for id, p := range m {
		out[id] = slices.Clone(p[:])
	}
	return out
}

// importPrios rebuilds exportPrios' map: work types missing from an older,
// shorter list load off, extra ones are ignored.
func importPrios(m map[entity.EntityID][]uint8) map[entity.EntityID]work.Priorities {
	out := make(map[entity.EntityID]work.Priorities, len(m))
	for id, l := range m {
		var p work.Priorities
		copy(p[:], l)
		if len(l) <= int(work.PlantCut) {
			p[work.PlantCut] = p[work.Grow] // FOOD-2: split from growing
		}
		out[id] = p
	}
	return out
}

// migrateWarden loads the priorities of a save from before the Warden work
// type (12 types): every work type keeps its priority, Warden is off.
func (g *Game) migrateWarden(old map[entity.EntityID]priorities12) {
	for id, op := range old {
		var p work.Priorities
		copy(p[:], op[:])
		p[work.PlantCut] = p[work.Grow] // FOOD-2
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
		p[work.Handle], p[work.PlantCut] = p[work.Hunt], p[work.Grow]
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
	setMap(&g.smooths, smooths)
	g.structVer = ver
	g.migrateLegacyStone() // a no-op on newer saves
	if geo.W == g.terr.W && geo.H == g.terr.H && geo.W > 0 {
		g.geo = geo
		return
	}
	g.geo = geology.Generate(terrainHash(g.terr), g.terr)
	g.structVer++
}

// terrainHash is an FNV-64a hash of the terrain: the geology seed of a
// save made before geology existed.
func terrainHash(t terrain.Map) uint64 {
	h := fnv.New64a()
	var hdr [16]byte
	binary.LittleEndian.PutUint64(hdr[:8], uint64(t.W))
	binary.LittleEndian.PutUint64(hdr[8:], uint64(t.H))
	_, _ = h.Write(hdr[:])
	cells := make([]byte, len(t.Cells))
	for i, c := range t.Cells {
		cells[i] = byte(c)
	}
	_, _ = h.Write(cells)
	return h.Sum64()
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
	for i := range g.pending {
		fix(&g.pending[i])
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
