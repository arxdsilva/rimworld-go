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
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
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
// become torch lamps (the v < 5 step). migrateSave runs the steps a save's
// version is missing.

// SaveVersion is the version new saves are written with.
const SaveVersion = 5

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
	Racks      map[entity.EntityID]dryBatch
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
	OrderQueue map[entity.EntityID][]savedOrder
	Apparel    map[entity.EntityID]worn
	// Prio is the priorities as saved before animal handling (11 work
	// types); Prios replaces it (SIM-026), with 12 types; PrioList (ENG-003)
	// replaces that, as plain slices so appending a work type never breaks
	// a save again (gob refuses an array whose length changed).
	Prio       map[entity.EntityID]legacyPriorities
	Prios      map[entity.EntityID]priorities12 // absent in older saves
	PrioList   map[entity.EntityID][]uint8      // absent in older saves
	RaiderIDs  []entity.EntityID
	Raiders    map[entity.EntityID]raiderState
	RaidGroups []raidGroup
	NextThreat uint64
	ThreatsOn  bool
	Scheds     map[entity.EntityID]schedule.Grid
	Joy        map[entity.EntityID]int
	Breaks     map[entity.EntityID]savedBreak
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
	Wounds      map[entity.EntityID]wound
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
	Pens        []pen
	RockDamage  map[[2]int]int
	Residual    map[int]temperature.Temp
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
	Deliveries map[entity.EntityID][]haulShare
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
	Bodies     []entry[body.Body]
	Surgery    *surgerySave
	Drugs      []entry[drug.State]
	Prisoners  []entry[prisoner.State]
	World      *worldSave
	Areas      *area.Set
	History    []historyEntry
	RNGStreams map[string][]byte
	// BedRest (PLAY-020): "Rest when hurt" settings, who rests since
	// when, who found no bed, and the medical beds held for a stay; nil
	// when there is none (absent in older saves).
	BedRest *bedRestSave
	// Power (PLAY-021): the colony's electricity (power.go); nil without
	// power devices, so saves without power hash as before (absent in
	// older saves).
	Power *powerSave
}

// powerSave is the saved electricity, in id order (PLAY021_PLAN.md
// Appendix): battery charge (W·ticks), the devices and switches the
// player wants off and those actually off (a flick is pending where they
// differ), the consumers the last balance left unpowered, shed consumers
// with the tick they stay off until, thermostats, the solar flare's end
// tick (0: none) and the tick of the last balance. Nets, outputs and the
// flick index are derived, not saved.
type powerSave struct {
	Charge     []entry[int64]
	WantOff    []entity.EntityID
	IsOff      []entity.EntityID
	Unpowered  []entity.EntityID
	Shed       []entry[uint64]
	Thermo     []entry[thermoSave]
	FlareUntil uint64
	Last       uint64
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
	return info
}

// Save writes the whole game to w under the given save name.
func (g *Game) Save(w io.Writer, name string) error {
	rng, err := g.rng.MarshalBinary()
	if err != nil {
		return fmt.Errorf("save rng: %w", err)
	}
	s := saveState{
		Tick: g.clock.Now(), W: g.grid.Width(), H: g.grid.Height(), NextEntity: g.entities.Peek(),
		RNG: rng, Sched: g.sched, BlockedBits: g.grid.BlockedBits(), Messages: g.messages,
		Letters: g.letters.list, NextLetter: g.letters.next,
		PawnIDs: g.pawnIDs, Positions: exportStore(g.positions), Names: exportStore(g.names),
		Jobs: exportStore(g.jobs), Need: exportStore(g.need), Vital: exportStore(g.vital), Labor: exportStore(g.labor),
		ItemIDs: g.itemIDs, Stacks: exportStore(g.stacks), FrameIDs: g.frameIDs, Frames: exportStore(g.frames),
		BldgIDs: g.bldgIDs, Bldgs: exportStore(g.bldgs), PlantIDs: g.plantIDs, Plants: exportStore(g.plants),
		Terr: g.terr, Mines: g.mines,
		LastMeal: g.lastMeal, Bills: g.bills, Racks: g.racks, Fuel: g.fuel,
		Exposure: map[entity.EntityID][2]int{}, NoComfort: g.noComfort,
		NoFood: g.noFood, Postponed: g.exportPostponed(), PostSweep: g.postponed.sweepAt, PostOrders: g.exportOrders(), RestedFree: g.restedFree, Calm: g.calm, Banked: g.banked,
		OrderQueue: map[entity.EntityID][]savedOrder{}, Apparel: g.apparel, Joy: g.joy,
		PrioList: exportPrios(g.prio), Scheds: g.scheds,
		RaiderIDs: g.raiderIDs, Raiders: g.raiders, RaidGroups: g.raidGroups, NextThreat: g.nextThreat, ThreatsOn: g.threatsOn,
		Breaks: map[entity.EntityID]savedBreak{}, RoomGrid: g.roomGrid, RoomTemps: g.roomTemps,
		Memories: g.memories, Claims: g.reserve.Export(), Pending: g.pending,
		AteAt: g.ateAt, Draft: g.draft, SleptAt: g.sleptAt,
		AnimalIDs: g.animalIDs, Animals: exportStore(g.animals), Equip: g.equip, Traces: g.traces, Wild: g.wild, Wildlife: g.wl,
		Carrying: g.carrying, TendedUntil: g.tendedUntil, Wounds: g.wounds,
		Research: g.research.V1(), ResearchAll: g.research.Slice(), NG1: g.exportSetupState(), ResearchCur: g.researchCur, ResearchSet: g.researchSet, RockDamage: g.rockDamage, Residual: g.residual,
		Motion: g.exportMotion(), Strikes: g.lastStrike, Hits: g.lastHit,
		Weather: g.weather, Graves: g.graves, Roofs: g.exportRoofs(),
		DoorsOriented: true, // PLAY-011: door facings are stored
	}
	s.Grass, s.GrassCut = g.exportGrass()
	s.GrassGrowth = g.exportGrassGrowth() // grass.go
	s.Landing = g.exportLanding()         // setup_mapgen.go: nil for the map centre
	s.Bios, s.Skills, s.WorkCarry = g.exportBios()
	s.SkillLive = g.exportSkillLive()
	s.Biome, s.GrassRegrow, s.Incidents, s.Troughs = g.biome, g.grass.regrow, g.incidents, g.troughs
	s.Stains, s.StainDrip = g.exportStains(), g.stains.dripN
	s.Dirt, s.Carried = g.exportDirt()
	s.Home, s.CleanWired = g.exportHome(), true
	s.NoHomeAuto = g.homeArea.off
	s.Snow, s.SnowClear = g.exportSnow() // snow_clear.go: nil without snow or an area
	s.CleanQ, s.HomeVer = slices.Clone(g.filth.cleanQ), g.homeArea.ver
	s.Plan = g.exportPlan() // plan.go
	s.Fog = g.exportFog()   // fog.go (MAPGEN-2): nil without fog

	s.Vaults = g.exportVaults() // vault.go (MAPGEN-3): nil without vaults
	s.Fires = g.exportFires()   // fire.go (PLAY-015): nil while nothing burns
	// pens.go (PLAY-015): marker settings, unpenned clocks
	s.PenMarkers, s.PenAnimals = slices.Clone(g.pens), g.exportPenAnimals()
	for id, e := range g.exposure {
		s.Exposure[id] = [2]int{e.hypo, e.heat}
	}
	// bridge.go (MAPGEN-3): the sea bitset, nil without a sea
	s.Ocean = g.exportOcean()
	for id, q := range g.orderQueue {
		for _, o := range q {
			s.OrderQueue[id] = append(s.OrderQueue[id], savedOrder{Cmd: o.cmd, Label: o.label, RetryAt: o.retryAt, Tries: o.tries})
		}
	}
	for id, b := range g.breaks {
		s.Breaks[id] = savedBreak{b.kind, b.until}
	}
	s.ZoneMap, s.Logs, s.Incoming = g.exportZones(), g.exportLogs(), g.exportIncoming()
	s.Deliveries = g.exportDeliveries()
	s.Geo, s.Smooths, s.StructVer = g.geo, g.smooths, g.structVer
	s.Social, s.People, s.World = g.exportSocial(), g.exportPeople(), g.exportWorld()
	s.Bodies, s.Drugs, s.Prisoners = g.exportBodies(), g.exportDrugs(), g.exportPrisoners()
	s.Surgery = g.exportSurgery()
	s.BedRest = g.exportBedRest() // bedrest.go (PLAY-020)
	s.Power = g.exportPower()     // power.go (PLAY-021): nil without power
	s.Areas, s.History = g.areas.Export(), slices.Clone(g.history)
	if s.RNGStreams, err = g.exportStreams(); err != nil {
		return fmt.Errorf("save rng streams: %w", err)
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
	g.grid = grid.Restore(s.W, s.H, s.blockedTiles())
	g.entities = entity.Resume(s.NextEntity)
	g.sched = s.Sched
	g.messages = s.Messages
	g.pawnIDs, g.positions, g.names = s.PawnIDs, importStore(s.Positions), importStore(s.Names)
	g.jobs, g.need, g.vital, g.labor = importStore(s.Jobs), importStore(s.Need), importStore(s.Vital), importStore(s.Labor)
	g.itemIDs, g.stacks = s.ItemIDs, importStore(s.Stacks)
	g.frameIDs, g.frames = s.FrameIDs, importStore(s.Frames)
	g.bldgIDs, g.bldgs = s.BldgIDs, importStore(s.Bldgs)
	g.plantIDs, g.plants = s.PlantIDs, importStore(s.Plants)
	g.terr = s.Terr
	g.roomGrid, g.roomTemps, g.roomDirty = s.RoomGrid, s.RoomTemps, true
	g.reserve = reservation.Import(s.Claims)
	g.pending = s.Pending
	g.animalIDs, g.animals = s.AnimalIDs, importStore(s.Animals)
	g.traces, g.wild = s.Traces, s.Wild
	g.wl = s.Wildlife
	setMap(&g.mines, s.Mines)
	setMap(&g.lastMeal, s.LastMeal)
	setMap(&g.bills, s.Bills)
	setMap(&g.racks, s.Racks)
	setMap(&g.fuel, s.Fuel)
	setMap(&g.noComfort, s.NoComfort)
	setMap(&g.noFood, s.NoFood)
	g.importPostponed(s.Postponed)
	g.postponed.sweepAt = s.PostSweep
	g.importOrders(s.PostOrders)
	setMap(&g.restedFree, s.RestedFree)
	setMap(&g.calm, s.Calm)
	setMap(&g.banked, s.Banked)
	setMap(&g.apparel, s.Apparel)
	switch {
	case s.PrioList != nil:
		g.prio = importPrios(s.PrioList)
	case s.Prios != nil:
		g.migrateWarden(s.Prios)
	default:
		g.migrateHandling(s.Prio)
	}
	setMap(&g.raiders, s.Raiders)
	g.raiderIDs, g.raidGroups, g.nextThreat, g.threatsOn = s.RaiderIDs, s.RaidGroups, s.NextThreat, s.ThreatsOn
	setMap(&g.scheds, s.Scheds)
	setMap(&g.joy, s.Joy)
	setMap(&g.memories, s.Memories)
	setMap(&g.ateAt, s.AteAt)
	setMap(&g.draft, s.Draft)
	setMap(&g.sleptAt, s.SleptAt)
	setMap(&g.equip, s.Equip)
	setMap(&g.carrying, s.Carrying)
	setMap(&g.tendedUntil, s.TendedUntil)
	setMap(&g.wounds, s.Wounds)
	g.research, g.researchCur, g.researchSet = research.FromV1(s.Research), s.ResearchCur, s.ResearchSet
	if s.ResearchAll != nil {
		g.research = research.FromSlice(s.ResearchAll)
	}
	setMap(&g.rockDamage, s.RockDamage)
	g.residual = s.Residual
	g.motion = importStore(s.Motion) // a mover without a record reads motionOf's due-now default
	setMap(&g.lastStrike, s.Strikes)
	setMap(&g.lastHit, s.Hits)
	g.weather = s.Weather
	setMap(&g.graves, s.Graves)
	g.afterLoadBurial()
	g.importRoofs(s.Roofs)
	g.importGrass(s.Grass, s.GrassCut)
	g.importGrassGrowth(s.GrassGrowth) // grass.go: older saves load full
	g.biome, g.climate, g.incidents = s.Biome, biome.DefOf(s.Biome).Climate, s.Incidents
	g.importSetupState(s.NG1)                   // setup.go: overrides the climate from the site
	g.importLanding(s.Landing)                  // setup_mapgen.go: after importSetupState
	g.importBios(s.Bios, s.Skills, s.WorkCarry) // skills.go: older saves get neutral bios
	g.importSkillLive(s.SkillLive)
	g.grass.regrow = s.GrassRegrow
	setMap(&g.troughs, s.Troughs)
	g.importStains(s.Stains)
	setMap(&g.stains.dripN, s.StainDrip)
	g.importPlan(s.Plan)            // plan.go: older saves have no marks
	g.importFog(s.Fog)              // fog.go: older saves have no fog
	g.importVaults(s.Vaults)        // vault.go: older saves have no vaults
	g.importFires(s.Fires)          // fire.go: older saves have no fires
	g.importHome(s.Home, s.HomeVer) // home.go: an older save gets Home around its buildings
	g.homeArea.off = s.NoHomeAuto
	// snow_clear.go (PLAY-015): an older save seeds from its global depth
	g.importSnow(s.Snow, s.SnowClear)
	g.importDirt(s.Dirt, s.Carried) // filth.go
	g.importCleanQ(s.CleanQ)        // filth.go: half-swept tiles stay queued
	if !s.CleanWired {
		g.wireCleaning() // clean.go: an older save switches cleaning on once
	}
	// bridge.go (MAPGEN-3): older saves have no sea
	g.importOcean(s.Ocean)
	g.seedGrass() // an older save: grass from terrain, bare under buildings
	for id, e := range s.Exposure {
		g.exposure[id] = exposureState{e[0], e[1]}
	}
	for id, q := range s.OrderQueue {
		for _, o := range q {
			g.orderQueue[id] = append(g.orderQueue[id], queuedOrder{cmd: o.Cmd, label: o.Label, retryAt: o.RetryAt, tries: o.Tries})
		}
	}
	for id, b := range s.Breaks {
		g.breaks[id] = breakState{b.Kind, b.Until}
	}
	s.ZoneMap.legacy = legacyZones{s.Stockpiles, s.GrowZones, s.Pens} // older saves: rectangles
	g.importZones(s.ZoneMap)
	g.importLogs(s.Logs)
	g.importIncoming(s.Incoming)
	g.importDeliveries(s.Deliveries)
	g.afterLoadAnimals() // legacy corpses and bills (wildlife.go)
	g.importGeology(s.Geo, s.Smooths, s.StructVer)
	g.importSocial(s.Social)
	g.importPeople(s.People)
	g.importWorld(s.World)
	g.importBodies(s.Bodies)
	g.importSurgery(s.Surgery)
	g.importBedRest(s.BedRest)
	g.importPower(s.Power) // power.go (PLAY-021)
	g.importDrugs(s.Drugs)
	g.importPrisoners(s.Prisoners)
	g.areas, g.history = area.Import(s.Areas), s.History
	if err := g.importStreams(s.RNGStreams); err != nil {
		return nil, info, fmt.Errorf("restore rng streams: %w", err)
	}
	if !s.DoorsOriented {
		g.orientDoors() // placement.go: an older save's doors take their walls' axis (PLAY-011)
	}
	// pens.go (PLAY-015): settings and clocks; an older save's painted
	// pens become pen markers
	g.importPens(s.PenMarkers, s.PenAnimals)
	g.migratePenZones()
	g.importLetters(s.Letters, s.NextLetter) // letters.go: an older save's letter messages become letters
	g.migrateSave(info.Version)
	g.warmIndexes(true) // jobindex.go: indexes, rooms and regions now, not on the first tick
	return g, info, nil
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
		g.migrateMedicalBeds() // bedrest.go (PLAY-020): hospital beds become medical
		fallthrough
	case v < 5:
		// TODO(INFRA, PLAY-021): lamps need power now; existing lamps
		// become torch lamps so old colonies keep their light.
	}
}

// savedSkillLive mirrors the skill state skills.go keeps beside the bios
// (skillState's learned, carry, working and research).
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
	if len(sk.learned) == 0 && len(sk.carry) == 0 && len(sk.working) == 0 && sk.research == 0 {
		return nil
	}
	out := &savedSkillLive{Research: sk.research}
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
	g.sk.research = s.Research
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
		p[work.Handle] = p[work.Hunt]
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
			g.stacks.Set(id, st)
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
