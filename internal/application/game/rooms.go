package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
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

func (g *Game) markRoomsDirty() { g.roomDirty = true }

func (g *Game) isBoundary(x, y int) bool {
	if g.terr.At(x, y) == terrain.Rock {
		return true
	}
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	if b, ok := g.bldgs.Get(bid); ok && (b.Kind == construction.Wall || b.Kind == construction.Door) {
		return true
	}
	return false
}

func (g *Game) ensureRooms() {
	if !g.roomDirty && g.roomGrid != nil {
		return
	}
	w, h := g.grid.Width(), g.grid.Height()
	grid, sizes := room.Detect(w, h, g.isBoundary)
	stats := make([]roomStats, len(sizes))
	for i, n := range sizes {
		stats[i].contents.Cells = n
	}
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		r := grid[cells[0][1]*w+cells[0][0]]
		if r < 0 {
			continue
		}
		c := &stats[r].contents
		switch {
		case b.Kind == construction.Floor:
			c.Floored++
		case b.Kind.Sleeps():
			c.Beds++
		case b.Kind == construction.Table:
			c.Tables++
		case b.Kind == construction.Chair:
			c.Chairs++
		case b.Kind == construction.Lamp:
			c.Lamps++
		case construction.DefOf(b.Kind).Bench:
			c.Benches++
		}
	}
	// walls around each room (adjacency count; stone share matters)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := grid[y*w+x]
			if r < 0 {
				continue
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h || grid[ny*w+nx] != room.Wall {
					continue
				}
				stats[r].contents.TotalWalls++
				bid := g.layerBldgAt(nx, ny, construction.LayerObject)
				if b, ok := g.bldgs.Get(bid); ok && b.Stuff == item.Stone || g.terr.At(nx, ny) == terrain.Rock {
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
	g.roomGrid, g.rooms, g.roomDirty = grid, stats, false
}

// roomAt is the room id at (x, y), or room.Outdoors / room.Wall.
func (g *Game) roomAt(x, y int) int {
	g.ensureRooms()
	if !g.grid.InBounds(x, y) {
		return room.Outdoors
	}
	return g.roomGrid[y*g.grid.Width()+x]
}

func (g *Game) roomViews() []RoomView {
	g.ensureRooms()
	out := make([]RoomView, len(g.rooms))
	for i, r := range g.rooms {
		out[i] = RoomView{ID: i, Cells: r.contents.Cells, Role: r.role.String(), Score: r.score,
			Label: room.Label(r.score), Lit: r.lit}
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
	n, _ := g.need.Get(id)
	var states []mood.Kind
	if n.Hungry() {
		states = append(states, mood.Hungry)
	}
	if n.Tired() {
		states = append(states, mood.Tired)
	}
	return mood.Active(g.memories[id].Prune(uint64(g.clock.Now())), states...)
}

func (g *Game) moodOf(id entity.EntityID) int { return mood.Level(g.thoughts(id)) }

// afterSleep records how and where pawn id slept.
func (g *Game) afterSleep(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	r := g.roomAt(pos.X, pos.Y)
	if g.sleptAt[id] == 0 {
		if r < 0 {
			g.remember(id, mood.SleptOutside)
		} else {
			g.remember(id, mood.SleptOnGround)
		}
		return
	}
	g.remember(id, mood.SleptInBed)
	if r < 0 {
		g.remember(id, mood.SleptOutside)
		return
	}
	switch s := g.rooms[r].score; {
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
	if r := g.roomAt(pos.X, pos.Y); r >= 0 && g.rooms[r].contents.Tables > 0 {
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
