package game

import (
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"rimworld-go/internal/domain/research"
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
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
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

// SaveVersion is bumped whenever the format changes incompatibly.
const SaveVersion = 1

// SaveInfo is the header of a save: enough to list it without loading.
type SaveInfo struct {
	Version   int
	Name      string // colony / save name
	Date      string // in-game date
	Day       int    // days since landing
	Colonists int    // alive
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
	Blocked    []int

	Messages []MessageView

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
	NoFood     map[entity.EntityID]uint64 // absent in older saves
	RestedFree map[entity.EntityID]bool   // absent in older saves
	Calm       map[entity.EntityID]uint64 // absent in older saves
	Banked     map[entity.EntityID]bool   // absent in older saves
	OrderQueue map[entity.EntityID][]savedOrder
	Apparel    map[entity.EntityID]worn
	Prio       map[entity.EntityID]work.Priorities
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
	Research    research.Progress
	ResearchCur research.Project
	ResearchSet bool
	Pens        []pen
	RockDamage  map[[2]int]int
	Residual    map[int]temperature.Temp
	// Motion: step timing per creature on the map (absent in older saves,
	// which default to due-now straight steps).
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
	// Grass and GrassCut (UI-011); absent in older saves, which reseed.
	Grass    []uint8
	GrassCut [][2]int
	// ZoneMap (ZONE-001): the per-tile zones; absent in older saves, whose
	// stockpile, grow zone and pen rectangles are painted into it on load.
	ZoneMap zoneSave
	// Logs (LOG-001): per-creature event logs; absent in older saves.
	Logs map[entity.EntityID][]logEntry
	// Stains (SIM-020): blood and ichor on the ground; absent in older
	// saves (none).
	Stains []stain.Stain
	// Dirt, Carried, Home (SIM-021): tracked-in dirt, what each pawn carries
	// on its boots and the Home area; absent in older saves. CleanWired
	// marks a save made after cleaning work was switched on for everyone.
	Dirt       []dirtCell
	Carried    map[entity.EntityID]uint16
	Home       []uint8
	CleanWired bool
	// Incoming (HAUL-001): material on its way to each frame or fuel user.
	Incoming map[entity.EntityID]int
	// Deliveries (HAUL-001): each hauler's shares of Incoming, route order.
	Deliveries map[entity.EntityID][]haulShare
}

// Info describes the game for a save header.
func (g *Game) Info(name string) SaveInfo {
	alive := 0
	for _, id := range g.pawnIDs {
		if v, ok := g.vital.Get(id); ok && v.Status != health.Dead {
			alive++
		}
	}
	return SaveInfo{Version: SaveVersion, Name: name, Date: g.Date().String(),
		Day: int(g.clock.Now()) / clock.TicksPerDay, Colonists: alive}
}

// Save writes the whole game to w under the given save name.
func (g *Game) Save(w io.Writer, name string) error {
	rng, err := g.rng.MarshalBinary()
	if err != nil {
		return fmt.Errorf("save rng: %w", err)
	}
	s := saveState{
		Tick: g.clock.Now(), W: g.grid.Width(), H: g.grid.Height(), NextEntity: g.entities.Peek(),
		RNG: rng, Sched: g.sched, Blocked: g.grid.Blocked(), Messages: g.messages,
		PawnIDs: g.pawnIDs, Positions: exportStore(g.positions), Names: exportStore(g.names),
		Jobs: exportStore(g.jobs), Need: exportStore(g.need), Vital: exportStore(g.vital), Labor: exportStore(g.labor),
		ItemIDs: g.itemIDs, Stacks: exportStore(g.stacks), FrameIDs: g.frameIDs, Frames: exportStore(g.frames),
		BldgIDs: g.bldgIDs, Bldgs: exportStore(g.bldgs), PlantIDs: g.plantIDs, Plants: exportStore(g.plants),
		Terr: g.terr, Mines: g.mines,
		LastMeal: g.lastMeal, Bills: g.bills, Racks: g.racks, Fuel: g.fuel,
		Exposure: map[entity.EntityID][2]int{}, NoComfort: g.noComfort,
		NoFood: g.noFood, RestedFree: g.restedFree, Calm: g.calm, Banked: g.banked,
		OrderQueue: map[entity.EntityID][]savedOrder{}, Apparel: g.apparel, Joy: g.joy,
		Prio: g.prio, Scheds: g.scheds,
		RaiderIDs: g.raiderIDs, Raiders: g.raiders, RaidGroups: g.raidGroups, NextThreat: g.nextThreat, ThreatsOn: g.threatsOn,
		Breaks: map[entity.EntityID]savedBreak{}, RoomGrid: g.roomGrid, RoomTemps: g.roomTemps,
		Memories: g.memories, Claims: g.reserve.Export(), Pending: g.pending,
		AteAt: g.ateAt, Draft: g.draft, SleptAt: g.sleptAt,
		AnimalIDs: g.animalIDs, Animals: exportStore(g.animals), Equip: g.equip, Traces: g.traces, Wild: g.wild, Wildlife: g.wl,
		Carrying: g.carrying, TendedUntil: g.tendedUntil, Wounds: g.wounds,
		Research: g.research, ResearchCur: g.researchCur, ResearchSet: g.researchSet, RockDamage: g.rockDamage, Residual: g.residual,
		Motion: g.exportMotion(), Strikes: g.lastStrike, Hits: g.lastHit,
		Weather: g.weather, Graves: g.graves, Roofs: g.exportRoofs(),
	}
	s.Grass, s.GrassCut = g.exportGrass()
	s.Stains = g.exportStains()
	s.Dirt, s.Carried = g.exportDirt()
	s.Home, s.CleanWired = g.exportHome(), true
	for id, e := range g.exposure {
		s.Exposure[id] = [2]int{e.hypo, e.heat}
	}
	for id, q := range g.orderQueue {
		for _, o := range q {
			s.OrderQueue[id] = append(s.OrderQueue[id], savedOrder{o.cmd, o.label})
		}
	}
	for id, b := range g.breaks {
		s.Breaks[id] = savedBreak{b.kind, b.until}
	}
	s.ZoneMap, s.Logs, s.Incoming = g.exportZones(), g.exportLogs(), g.exportIncoming()
	s.Deliveries = g.exportDeliveries()
	zw := gzip.NewWriter(w)
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
	if info.Version != SaveVersion {
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
	g.grid = grid.Restore(s.W, s.H, s.Blocked)
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
	setMap(&g.restedFree, s.RestedFree)
	setMap(&g.calm, s.Calm)
	setMap(&g.banked, s.Banked)
	setMap(&g.apparel, s.Apparel)
	setMap(&g.prio, s.Prio)
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
	g.research, g.researchCur, g.researchSet = s.Research, s.ResearchCur, s.ResearchSet
	setMap(&g.rockDamage, s.RockDamage)
	g.residual = s.Residual
	g.motion = importStore(s.Motion)
	g.defaultMotion()
	setMap(&g.lastStrike, s.Strikes)
	setMap(&g.lastHit, s.Hits)
	g.weather = s.Weather
	setMap(&g.graves, s.Graves)
	g.afterLoadBurial()
	g.importRoofs(s.Roofs)
	g.importGrass(s.Grass, s.GrassCut)
	g.importStains(s.Stains)
	g.importHome(s.Home)            // home.go: an older save gets Home around its buildings
	g.importDirt(s.Dirt, s.Carried) // filth.go
	if !s.CleanWired {
		g.wireCleaning() // clean.go: an older save switches cleaning on once
	}
	g.seedGrass() // an older save: grass from terrain, bare under buildings
	for id, e := range s.Exposure {
		g.exposure[id] = exposureState{e[0], e[1]}
	}
	for id, q := range s.OrderQueue {
		for _, o := range q {
			g.orderQueue[id] = append(g.orderQueue[id], queuedOrder{o.Cmd, o.Label})
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
	return g, info, nil
}

// setMap installs a decoded map, keeping the empty one gob leaves nil.
func setMap[K comparable, V any](dst *map[K]V, src map[K]V) {
	if src != nil {
		*dst = src
	}
}
