// Package game orchestrates the authoritative simulation: clock, grid,
// pawns, jobs, movement, items, needs, construction, and harvest.
package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

var errSpawn = fmt.Errorf("spawn: tile out of bounds or occupied")

// TicksPerCell is how long a pawn occupies one tile (~4 cells/s at 1x).
const TicksPerCell = clock.TicksPerSecond / 4

// Position is a pawn's location on the grid.
type Position struct {
	X, Y int
}

// Config configures a new Game.
type Config struct {
	Seed          uint64
	Width, Height int
}

// PawnView is a read-only snapshot of a single pawn.
type PawnView struct {
	ID            entity.EntityID
	Name          string
	X, Y          int
	Activity      schedule.Activity
	Job           string
	Hunger        int
	Rest          int
	Status        string
	Reason        string
	WorkHaul      bool
	WorkConstruct bool
	WorkGrow      bool
	WorkMine      bool
	WorkCook      bool
	WorkHunt      bool
	Ate           bool
	LastMeal      string // kind name of the last food eaten, "" if none
	SleptInBed    bool
	Drafted       bool
}

// ItemView is a ground stack.
type ItemView struct {
	ID      entity.EntityID
	Kind    string
	Qty     int
	X, Y    int
	Carried bool
}

// FrameView is a construction blueprint.
type FrameView struct {
	ID       entity.EntityID
	Kind     string
	X, Y     int
	Wood     int
	Need     int
	WorkLeft int
	Facing   string
	Floor    bool
	Stuff    string
}

// BuildingView is a completed wall or bed.
type BuildingView struct {
	ID          entity.EntityID
	Kind        string
	X, Y        int
	Claimant    entity.EntityID
	Facing      string
	Floor       bool
	Deconstruct bool
	Stuff       string
}

// PlantView is a wild plant.
type PlantView struct {
	ID          entity.EntityID
	X, Y        int
	Designated  bool
	Harvestable bool
	Species     string
	Growth      int // percent
	Stage       int // 0 sown .. 3 ripe
	Cut         bool
}

// StockpileView is a storage rectangle.
type StockpileView struct {
	MinX, MinY, MaxX, MaxY           int
	AllowFood, AllowWood, AllowStone bool
}

// WorldView is the non-pawn read model for HUD and the map.
type WorldView struct {
	Items      []ItemView
	Frames     []FrameView
	Buildings  []BuildingView
	Plants     []PlantView
	Stockpiles []StockpileView
	Food, Wood int
	Stone      int
	// Terrain is the ground type per tile, row-major, TerrainW wide;
	// codes index TerrainNames().
	Terrain  []uint8
	TerrainW int
	// Mines are tiles designated for mining.
	Mines [][2]int
	// GrowZones are painted growing areas.
	GrowZones []GrowZoneView
	// FoodByKind is ground food per kind name.
	FoodByKind map[string]int
}

// Game holds all authoritative simulation state.
type Game struct {
	clock    *clock.Clock
	grid     *grid.Grid
	entities *entity.Allocator
	rng      *randx.Rand
	sched    schedule.Grid

	pawnIDs   []entity.EntityID
	positions *component.Store[Position]
	names     *component.Store[string]
	jobs      *component.Store[job.Record]
	need      *component.Store[needs.Pair]
	vital     *component.Store[health.Record]
	labor     *component.Store[work.Mask]

	itemIDs  []entity.EntityID
	stacks   *component.Store[item.Stack]
	frameIDs []entity.EntityID
	frames   *component.Store[construction.Frame]
	bldgIDs  []entity.EntityID
	bldgs    *component.Store[construction.Building]
	plantIDs []entity.EntityID
	plants   *component.Store[plant.Wild]
	terr     terrain.Map
	mines    map[[2]int]bool

	stockpiles []zone.Stockpile
	growZones  []zone.GrowZone
	lastMeal   map[entity.EntityID]item.Kind
	reserve    *reservation.Table
	pending    []command.Command

	ateAt   map[entity.EntityID]bool
	draft   map[entity.EntityID]bool
	sleptAt map[entity.EntityID]entity.EntityID // pawn -> bed, 0 = ground
}

// New builds a Game from cfg. Width/Height must be positive.
func New(cfg Config) *Game {
	return &Game{
		clock:     clock.NewAt(clock.StartingMorning()),
		grid:      grid.New(cfg.Width, cfg.Height),
		entities:  entity.NewAllocator(),
		rng:       randx.New(cfg.Seed),
		sched:     schedule.DefaultColony(),
		positions: component.NewStore[Position](),
		names:     component.NewStore[string](),
		jobs:      component.NewStore[job.Record](),
		need:      component.NewStore[needs.Pair](),
		vital:     component.NewStore[health.Record](),
		labor:     component.NewStore[work.Mask](),
		stacks:    component.NewStore[item.Stack](),
		frames:    component.NewStore[construction.Frame](),
		bldgs:     component.NewStore[construction.Building](),
		plants:    component.NewStore[plant.Wild](),
		terr:      terrain.NewMap(cfg.Width, cfg.Height),
		mines:     make(map[[2]int]bool),
		lastMeal:  make(map[entity.EntityID]item.Kind),
		reserve:   reservation.NewTable(),
		ateAt:     make(map[entity.EntityID]bool),
		draft:     make(map[entity.EntityID]bool),
		sleptAt:   make(map[entity.EntityID]entity.EntityID),
	}
}

// NewColony builds a generated map (terrain, trees, bushes) with the
// first-playable start kit in its open centre.
func NewColony(cfg Config) *Game {
	g := New(cfg)
	g.generate(cfg.Seed)
	if err := g.LoadStartKit(); err != nil {
		panic(err)
	}
	return g
}

// SpawnPawn creates a pawn at (x, y) with a generated display name.
func (g *Game) SpawnPawn(x, y int) (entity.EntityID, error) {
	return g.SpawnNamedPawn(x, y, "")
}

// SpawnNamedPawn creates a pawn with an explicit display name.
func (g *Game) SpawnNamedPawn(x, y int, name string) (entity.EntityID, error) {
	if !g.grid.Walkable(x, y) {
		return 0, fmt.Errorf("spawn pawn at (%d, %d): tile not walkable", x, y)
	}
	id := g.entities.Next()
	if name == "" {
		name = fmt.Sprintf("Colonist %d", id)
	}
	g.pawnIDs = append(g.pawnIDs, id)
	g.positions.Set(id, Position{X: x, Y: y})
	g.names.Set(id, name)
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.labor.Set(id, work.DefaultColonist())
	return id, nil
}

// SpawnStack places a ground item stack.
func (g *Game) SpawnStack(kind item.Kind, qty, x, y int) (entity.EntityID, error) {
	if !g.grid.InBounds(x, y) || qty <= 0 {
		return 0, fmt.Errorf("spawn stack")
	}
	id := g.entities.Next()
	g.itemIDs = append(g.itemIDs, id)
	g.stacks.Set(id, item.Stack{Kind: kind, Qty: qty, X: x, Y: y})
	return id, nil
}

// SpawnPlant places a ripe wild berry bush.
func (g *Game) SpawnPlant(x, y int) (entity.EntityID, error) {
	return g.spawnPlant(plant.BerryBush, x, y, plant.DefOf(plant.BerryBush).GrowTicks)
}

// LoadStartKit places three named colonists, food, wood, and ripe berry
// bushes around the map centre.
func (g *Game) LoadStartKit() error {
	cx, cy := g.grid.Width()/2, g.grid.Height()/2
	names := []string{"Ada", "Bo", "Cora"}
	spots := []Position{{cx - 1, cy}, {cx, cy}, {cx + 1, cy}}
	for i, n := range names {
		if _, err := g.SpawnNamedPawn(spots[i].X, spots[i].Y, n); err != nil {
			return err
		}
	}
	if _, err := g.SpawnStack(item.Food, 20, cx+2, cy+1); err != nil {
		return err
	}
	if _, err := g.SpawnStack(item.Food, 10, cx+3, cy+1); err != nil {
		return err
	}
	if _, err := g.SpawnStack(item.Wood, 30, cx+2, cy+3); err != nil {
		return err
	}
	for i := 0; i < 4; i++ {
		if _, err := g.SpawnPlant(cx+2+i, cy-4); err != nil {
			return err
		}
	}
	return nil
}

// Tick returns the current simulation tick.
func (g *Game) Tick() clock.Tick { return g.clock.Now() }

// Date returns the calendar date for the current simulation tick.
func (g *Game) Date() clock.Date { return g.clock.Date() }

// Activity is the shared colony schedule block for the current hour.
func (g *Game) Activity() schedule.Activity {
	return g.sched.At(g.Date().Hour)
}

// Width returns the map width in tiles.
func (g *Game) Width() int { return g.grid.Width() }

// Height returns the map height in tiles.
func (g *Game) Height() int { return g.grid.Height() }

// Apply queues a typed gameplay command for the next tick's command phase.
func (g *Game) Apply(c command.Command) {
	if c.Kind == command.None {
		return
	}
	g.pending = append(g.pending, c)
}

// Enqueue is the command-gateway alias used by the inbound adapter.
func (g *Game) Enqueue(c command.Command) { g.Apply(c) }

// SetNeeds overwrites hunger/rest (fixtures).
func (g *Game) SetNeeds(id entity.EntityID, hunger, rest int) {
	g.need.Set(id, needs.Pair{Hunger: hunger, Rest: rest})
}

// SetWork sets a work-type enable flag immediately (also available via command).
func (g *Game) SetWork(id entity.EntityID, t work.Type, on bool) {
	m, ok := g.labor.Get(id)
	if !ok {
		return
	}
	g.labor.Set(id, m.Set(t, on))
}

// Step advances the simulation by n ticks.
// Phases: commands → needs/health → AI/jobs → movement → clock.
func (g *Game) Step(n int) {
	for i := 0; i < n; i++ {
		g.phaseCommands()
		g.phasePlants()
		g.phaseNeeds()
		g.phaseAI()
		g.phaseJobs()
		g.phaseMovement()
		g.clock.Advance(1)
	}
}

func (g *Game) dueToMove(id entity.EntityID) bool {
	return (uint64(g.clock.Now())+uint64(id))%uint64(TicksPerCell) == 0
}

// Snapshot returns a read-only view of every pawn, in insertion order.
func (g *Game) Snapshot() []PawnView {
	act := g.Activity()
	views := make([]PawnView, 0, len(g.pawnIDs))
	for _, id := range g.pawnIDs {
		pos, _ := g.positions.Get(id)
		name, _ := g.names.Get(id)
		j, _ := g.jobs.Get(id)
		n, _ := g.need.Get(id)
		v, _ := g.vital.Get(id)
		m, _ := g.labor.Get(id)
		reason := v.Cause.String()
		if j.Kind == job.Sleep && v.Status == health.Alive && g.sleptAt[id] == 0 && j.State != job.Idle {
			reason = health.CauseNoBed.String()
		}
		views = append(views, PawnView{
			ID:            id,
			Name:          name,
			X:             pos.X,
			Y:             pos.Y,
			Activity:      act,
			Job:           j.Line(),
			Hunger:        needs.Percent(n.Hunger),
			Rest:          needs.Percent(n.Rest),
			Status:        v.Status.String(),
			Reason:        reason,
			WorkHaul:      m.Enabled(work.Haul),
			WorkConstruct: m.Enabled(work.Construct),
			WorkGrow:      m.Enabled(work.Grow),
			WorkMine:      m.Enabled(work.Mine),
			WorkCook:      m.Enabled(work.Cook),
			WorkHunt:      m.Enabled(work.Hunt),
			Ate:           g.ateAt[id],
			LastMeal:      lastMealName(g, id),
			SleptInBed:    g.sleptAt[id] != 0,
			Drafted:       g.draft[id],
		})
	}
	return views
}

// World returns the non-pawn read model.
func (g *Game) World() WorldView {
	w := WorldView{
		Items:      make([]ItemView, 0, len(g.itemIDs)),
		Frames:     make([]FrameView, 0, len(g.frameIDs)),
		Buildings:  make([]BuildingView, 0, len(g.bldgIDs)),
		Plants:     make([]PlantView, 0, len(g.plantIDs)),
		Stockpiles: append([]StockpileView(nil), stockpileViews(g.stockpiles)...),
	}
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok {
			continue
		}
		w.Items = append(w.Items, ItemView{
			ID: id, Kind: st.Kind.String(), Qty: st.Qty, X: st.X, Y: st.Y, Carried: st.Carrier != 0,
		})
		if st.Carrier == 0 {
			switch {
			case st.Kind.IsFood():
				w.Food += st.Qty
				if w.FoodByKind == nil {
					w.FoodByKind = map[string]int{}
				}
				w.FoodByKind[st.Kind.String()] += st.Qty
			case st.Kind == item.Wood:
				w.Wood += st.Qty
			case st.Kind == item.Stone:
				w.Stone += st.Qty
			}
		}
	}
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if !ok {
			continue
		}
		w.Frames = append(w.Frames, FrameView{
			ID: id, Kind: fr.Kind.String(), X: fr.X, Y: fr.Y, Wood: fr.Wood, Need: fr.Need, WorkLeft: fr.WorkLeft,
			Facing: fr.Facing.String(), Floor: construction.DefOf(fr.Kind).Layer == construction.LayerFloor,
			Stuff: fr.Stuff.String(),
		})
	}
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		w.Buildings = append(w.Buildings, BuildingView{
			ID: id, Kind: b.Kind.String(), X: b.X, Y: b.Y, Claimant: entity.EntityID(b.Claimant),
			Facing: b.Facing.String(), Floor: construction.DefOf(b.Kind).Layer == construction.LayerFloor,
			Deconstruct: b.Deconstruct, Stuff: b.Stuff.String(),
		})
	}
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if !ok {
			continue
		}
		w.Plants = append(w.Plants, PlantView{
			ID: id, X: p.X, Y: p.Y, Designated: p.Designated, Harvestable: p.Harvestable(),
			Species: p.Species.String(), Growth: p.Percent(), Stage: p.Stage(), Cut: p.Cut,
		})
	}
	w.TerrainW = g.terr.W
	w.Terrain = make([]uint8, len(g.terr.Cells))
	for i, t := range g.terr.Cells {
		w.Terrain[i] = uint8(t)
	}
	w.Mines = g.mineList()
	w.GrowZones = g.growZoneViews()
	return w
}

func stockpileViews(z []zone.Stockpile) []StockpileView {
	out := make([]StockpileView, len(z))
	for i, s := range z {
		out[i] = StockpileView{s.MinX, s.MinY, s.MaxX, s.MaxY, s.AllowFood, s.AllowWood, s.AllowStone}
	}
	return out
}

func lastMealName(g *Game, id entity.EntityID) string {
	if k, ok := g.lastMealKind(id); ok {
		return k.String()
	}
	return ""
}
