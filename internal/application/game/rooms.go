package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/terrain"
)

// RoomView is an enclosed room in the read model.
type RoomView struct {
	ID    int
	Cells int
	Role  string
	Score int
	Label string // awful .. impressive
	Lit   bool   // has a lamp
	Temp  int    // air temperature, tenths of a degree C
	// Roofed is the roofed share of the room's cells, 0..1 (SIM-018).
	Roofed float64
	// Filth is the average dirt plus stain per cell (0..filth.Full),
	// Cleanliness its label and FilthPenalty the impressiveness it costs
	// (already taken off Score) (SIM-021).
	Filth        int
	Cleanliness  string
	FilthPenalty int
}

// ThoughtView is one line of a pawn's mood.
type ThoughtView struct {
	Label string
	Value int
}

type roomStats struct {
	contents room.Contents
	score    int
	role     room.Role
	lit      bool
}

func (g *Game) markRoomsDirty() { g.roomDirty = true; g.fenceDirty() } // fences.go: a building changed

func (g *Game) isBoundary(x, y int) bool {
	if terrain.IsRockWall(g.terr.At(x, y)) {
		return true
	}
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	if b, ok := g.bldgs.Get(bid); ok && (b.Kind == construction.Wall || b.Kind == construction.Door) {
		return true
	}
	return false
}

// ensureRooms rebuilds the rooms after a building or terrain change:
// incrementally through the room map (rooms_index.go), or in full with
// regions off.
func (g *Game) ensureRooms() {
	if !g.roomDirty && g.roomGrid != nil {
		return
	}
	if g.noRegions() {
		g.rebuildRoomsFull()
		return
	}
	g.rebuildRoomsIncremental()
}

// rebuildRoomsFull detects every room and recounts its contents from
// scratch (the reference the incremental rebuild must match).
func (g *Game) rebuildRoomsFull() {
	w, h := g.grid.Width(), g.grid.Height()
	grid, sizes := room.Detect(w, h, g.isBoundary)
	stats := g.roomStatsFull(grid, sizes)
	g.roomTemps = g.remapRoomTemps(grid, len(stats))
	g.rebuildFilth(grid, len(stats)) // filth.go: tread and room filth sums
	g.roomGrid, g.rooms, g.roomDirty = grid, stats, false
	g.afterRoomsRebuilt() // roof.go
}

// roomStatsFull scores every room of a detected grid by a full scan.
func (g *Game) roomStatsFull(grid []int, sizes []int) []roomStats {
	w, h := g.grid.Width(), g.grid.Height()
	stats := make([]roomStats, len(sizes))
	for i, n := range sizes {
		stats[i].contents.Cells = n
	}
	g.countRoomBuildings(grid, stats) // rooms_index.go
	// walls around each room (adjacency count; stone share matters)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := grid[y*w+x]
			if r < 0 {
				continue
			}
			// SIM-029: a smooth stone floor counts as floored (once)
			if g.terr.At(x, y) == terrain.StoneSmooth && g.layerBldgAt(x, y, construction.LayerFloor) == 0 {
				stats[r].contents.Floored++
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h || grid[ny*w+nx] != room.Wall {
					continue
				}
				stats[r].contents.TotalWalls++
				bid := g.layerBldgAt(nx, ny, construction.LayerObject)
				if b, ok := g.bldgs.Get(bid); ok && (item.IsStoneBlocks(b.Stuff) || b.Stuff == item.Stone) || terrain.IsRockWall(g.terr.At(nx, ny)) {
					stats[r].contents.StoneWalls++
				}
			}
		}
	}
	for i := range stats {
		stats[i].score = room.Impressiveness(stats[i].contents)
		stats[i].role = room.RoleOf(stats[i].contents)
		stats[i].lit = stats[i].contents.Lamps > 0
	}
	return stats
}

// roomAt is the room id at (x, y), or room.Outdoors / room.Wall.
func (g *Game) roomAt(x, y int) int {
	g.ensureRooms()
	if !g.grid.InBounds(x, y) {
		return room.Outdoors
	}
	return g.roomGrid[y*g.grid.Width()+x]
}

// roomFilth is room r's dirt plus stain and the impressiveness it costs.
func (g *Game) roomFilth(r int) (sum, penalty int) {
	if r < 0 || r >= len(g.rooms) || r >= len(g.filth.roomSum) {
		return 0, 0
	}
	sum = g.filth.roomSum[r]
	return sum, filth.Penalty(sum, g.rooms[r].contents.Cells)
}

// roomScore is room r's impressiveness less what its filth costs (rooms
// must be current).
func (g *Game) roomScore(r int) int {
	_, pen := g.roomFilth(r)
	return g.rooms[r].score - pen
}

func (g *Game) roomViews() []RoomView {
	g.ensureRooms()
	out := make([]RoomView, len(g.rooms))
	for i, r := range g.rooms {
		sum, pen := g.roomFilth(i)
		avg := sum / max(1, r.contents.Cells)
		score := r.score - pen
		out[i] = RoomView{ID: i, Cells: r.contents.Cells, Role: r.role.String(), Score: score,
			Label: room.Label(score), Lit: r.lit, Temp: int(g.roomTemps[i]),
			Roofed: float64(g.roomRoofed(i)) / 1000,
			Filth:  avg, Cleanliness: filth.Label(avg), FilthPenalty: pen}
	}
	for _, bid := range g.bldgIDs { // a burning fire lights its room (not one doused by rain or snow)
		if b, ok := g.bldgs.Get(bid); ok && b.Kind == construction.Campfire && g.fuel[bid] > 0 && g.burning(bid, b) {
			if r := g.roomAt(b.X, b.Y); r >= 0 && r < len(out) {
				out[r].Lit = true
			}
		}
	}
	return out
}

// RoomInfo describes the room at a tile ("" outdoors).
func (g *Game) RoomInfo(x, y int) (RoomView, bool) {
	r := g.roomAt(x, y)
	if r < 0 {
		return RoomView{}, false
	}
	return g.roomViews()[r], true
}

// --- thoughts and mood ---------------------------------------------

func (g *Game) remember(id entity.EntityID, k mood.Kind) {
	g.memories[id] = g.memories[id].Add(k, uint64(g.clock.Now()))
}

func (g *Game) thoughts(id entity.EntityID) []mood.Thought {
	return g.appendThoughts(nil, id)
}

// thoughtLevel is mood.Level(g.thoughts(id)), the list built in a reused
// buffer (the mood checks run it for every working colonist).
func (g *Game) thoughtLevel(id entity.EntityID) int {
	g.thoughtBuf = g.appendThoughts(g.thoughtBuf[:0], id)
	return mood.Level(g.thoughtBuf)
}

// appendThoughts appends the colonist's thoughts to dst.
func (g *Game) appendThoughts(dst []mood.Thought, id entity.EntityID) []mood.Thought {
	n, _ := g.need.Get(id)
	var stateBuf [12]mood.Kind
	states := stateBuf[:0]
	if n.Hungry() {
		states = append(states, mood.Hungry)
	}
	if n.Tired() {
		states = append(states, mood.Tired)
	}
	switch joy := g.joy[id]; {
	case joy < recreation.Starved:
		states = append(states, mood.RecreationStarved)
	case joy < recreation.Bored:
		states = append(states, mood.Bored)
	case joy >= recreation.Entertained:
		states = append(states, mood.Entertained)
	}
	if g.wounds[id].Infection >= infection.Sick {
		states = append(states, mood.Sick)
	}
	if v, _ := g.vital.Get(id); v.Injury >= painInjury {
		states = append(states, mood.InPain)
	}
	pos, _ := g.positions.Get(id)
	b := g.bandsOf(id)
	switch t := int(g.temperatureAt(pos.X, pos.Y)); {
	case t < b.Freezing():
		states = append(states, mood.Freezing)
	case t < b.Min:
		states = append(states, mood.Cold)
	case t > b.Sweltering():
		states = append(states, mood.Sweltering)
	case t > b.Max:
		states = append(states, mood.Hot)
	}
	if v, _ := g.vital.Get(id); v.Status != health.Dead && g.unburiedNear(pos) {
		states = append(states, mood.UnburiedCorpse)
	}
	ts := g.traitThoughts(id, mood.AppendActive(dst, g.memories[id].Prune(uint64(g.clock.Now())), states...)) // skills.go
	ts = append(ts, g.bodyThoughts(id)...)                                                                    // body.go
	ts = append(ts, g.drugThoughts(id)...)                                                                    // drugs.go
	return append(ts, g.socialThoughts(id)...)                                                                // social_mood.go
}

// moodOf is the colonist's mood: the level its thoughts add up to, as the
// social layer smooths it (social_mood.go).
func (g *Game) moodOf(id entity.EntityID) int { return g.smoothedMood(id, g.thoughtLevel(id)) }

// afterSleep records how and where pawn id slept.
func (g *Game) afterSleep(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	r := g.roomAt(pos.X, pos.Y)
	onFloor := g.sleptAt[id] == 0
	if b, ok := g.bldgs.Get(g.sleptAt[id]); ok && b.Kind == construction.SleepingSpot {
		onFloor = true
	}
	if onFloor {
		if r < 0 && g.underSky(pos.X, pos.Y) {
			g.remember(id, mood.SleptOutside)
		} else {
			g.remember(id, mood.SleptOnGround)
		}
		return
	}
	g.remember(id, mood.SleptInBed)
	if r < 0 {
		if g.underSky(pos.X, pos.Y) {
			g.remember(id, mood.SleptOutside)
		}
		return
	}
	switch s := g.roomScore(r); {
	case s < 0:
		g.remember(id, mood.SleptInAwfulRoom)
	case s >= 12:
		g.remember(id, mood.SleptInNiceRoom)
	default:
		g.memories[id] = g.memories[id].ForgetGroupOf(mood.SleptInNiceRoom)
	}
}

// afterEat records what and where pawn id ate.
func (g *Game) afterEat(id entity.EntityID, k item.Kind) {
	switch {
	case k == item.Meal:
		g.remember(id, mood.AteMeal)
	case k.Raw():
		g.remember(id, mood.AteRaw)
	}
	pos, _ := g.positions.Get(id)
	if g.nextToTable(pos.X, pos.Y) {
		g.remember(id, mood.AteAtTable)
	} else {
		g.remember(id, mood.AteWithoutTable)
	}
}

// workTick reports whether a work job progresses this tick: unhappy
// pawns lose one tick in four (deterministic on the clock).
func (g *Game) workTick(id entity.EntityID) bool {
	if uint64(g.clock.Now())%4 != 0 {
		return true
	}
	return g.moodOf(id) >= mood.LowMood
}

func thoughtViews(ts []mood.Thought) []ThoughtView {
	out := make([]ThoughtView, len(ts))
	for i, t := range ts {
		out[i] = ThoughtView{Label: t.Label, Value: t.Value}
	}
	return out
}

// acting reports the hands-on work a pawn is doing and the worked tile.
func (g *Game) acting(id entity.EntityID, j job.Record) (string, int, int) {
	if j.State != job.Do || !g.alive(id) {
		return "", 0, 0
	}
	switch j.Kind {
	case job.Construct, job.Deconstruct, job.Cut, job.Harvest, job.Sow:
		return j.Kind.String(), j.TX, j.TY
	case job.Mine, job.Smooth, job.Roof, job.Unroof, job.Clean, job.ClearSnow:
		return j.Kind.String(), j.DestX, j.DestY
	case job.Bill:
		if j.Carry == 0 { // picking the ingredient up, not working yet
			return "", 0, 0
		}
		if b, ok := g.bldgs.Get(j.Dest); ok {
			return "work at " + b.Kind.String(), b.X, b.Y
		}
	case job.Refuel:
		if j.Carry == 0 {
			return "", 0, 0
		}
		if b, ok := g.bldgs.Get(j.Dest); ok {
			return "refuel", b.X, b.Y
		}
	case job.Horseshoes:
		return "play horseshoes", j.DestX, j.DestY
	case job.Tend:
		if p, ok := g.positions.Get(j.Target); ok {
			return j.Kind.String(), p.X, p.Y
		}
	case job.Eat:
		pos, _ := g.positions.Get(id)
		return j.Kind.String(), pos.X, pos.Y
	case job.Milk, job.Shear:
		if p, ok := g.positions.Get(j.Target); ok {
			return g.collectVerb(j), p.X, p.Y // animal_products.go
		}
	}
	return "", 0, 0
}
