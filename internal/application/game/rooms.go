package game

import (
	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/terrain"
)

// RoomView is an enclosed room in the read model. BED-2 stats: Space, Beauty, Impressiveness
// in tenths, CleanStat hundredths, Wealth silver; Score is whole points, Label the stage.
type RoomView struct {
	ID, Cells                            int
	Role                                 string
	Score                                int
	Label                                string // awful .. wondrously impressive
	Space, Beauty, Wealth, CleanStat     int
	SpaceLabel, BeautyLabel, WealthLabel string
	Impressiveness                       int
	Lit                                  bool // a light shines in it: a powered lamp, a burning torch lamp or campfire
	Temp                                 int  // air temperature, tenths of a degree C
	// Roofed is the roofed share of the room's cells, 0..1 (SIM-018).
	Roofed float64
	// Filth is the average dirt plus stain per cell (0..filth.Full),
	// Cleanliness the CleanStat label and FilthPenalty the impressiveness
	// points the filth costs (already taken off Score) (SIM-021, BED-2).
	Filth        int
	Cleanliness  string
	FilthPenalty int
}

// ThoughtView is one line of a pawn's mood.
type ThoughtView struct {
	Label string
	Value int
}

// roomStats is a rebuild's count per room (filth, items, bed flags: live).
type roomStats struct {
	contents room.Contents
	tally    room.Tally
}

func (g *Game) markRoomsDirty() { g.roomDirty = true; g.fenceDirty() } // fences.go: a building changed

func (g *Game) isBoundary(x, y int) bool {
	if terrain.IsRockWall(g.terr.At(x, y)) {
		return true
	}
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	if b, ok := g.bldgs.Get(bid); ok && roomWall(b.Kind) {
		return true
	}
	return false
}

// roomWall reports a building that bounds rooms: a wall, a door, or a
// cooler standing in a wall gap (PLAY-021: it moves heat between the two
// rooms it separates).
func roomWall(k construction.Kind) bool {
	return k == construction.Wall || k.IsDoor() || k == construction.Cooler
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
	w := g.grid.Width()
	stats := make([]roomStats, len(sizes))
	for i, n := range sizes {
		stats[i].contents.Cells = n
	}
	g.countRoomBuildings(grid, stats) // rooms_index.go
	for i, r := range grid {          // SIM-029: a smooth stone floor counts as floored (once)
		if r >= 0 && g.terr.At(i%w, i/w) == terrain.StoneSmooth && g.layerBldgAt(i%w, i/w, construction.LayerFloor) == 0 {
			stats[r].contents.Floored++
		}
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

// roomFilth is room r's dirt plus stain and its SIM-021 penalty.
func (g *Game) roomFilth(r int) (sum, penalty int) {
	if r < 0 || r >= len(g.rooms) || r >= len(g.filth.roomSum) {
		return 0, 0
	}
	sum = g.filth.roomSum[r]
	return sum, filth.Penalty(sum, g.rooms[r].contents.Cells)
}

func (g *Game) roomViews() []RoomView {
	g.ensureRooms()
	facts := g.regs.rooms.countFacts(g)
	out := make([]RoomView, len(g.rooms))
	for i, r := range g.rooms {
		f := facts[i]
		sum, _ := g.roomFilth(i)
		st, clean := r.tally.Stats(r.contents, sum, f.Wealth), r.tally.Stats(r.contents, 0, f.Wealth)
		imp := st.ImpressivenessTenths
		out[i] = RoomView{ID: i, Cells: r.contents.Cells, Role: r.tally.Role(r.contents, f.MedicalBeds, f.PrisonerBeds).String(),
			Score: imp / 10, Label: room.ImpressivenessLabel(imp), Impressiveness: imp, Temp: int(g.roomTemps[i]),
			Space: st.SpaceTenths, SpaceLabel: room.SpaceLabel(st.SpaceTenths), Beauty: st.BeautyTenths,
			BeautyLabel: room.BeautyLabel(st.BeautyTenths), Wealth: st.Wealth, WealthLabel: room.WealthLabel(st.Wealth),
			Roofed: float64(g.roomRoofed(i)) / 1000, Filth: sum / max(1, r.contents.Cells), CleanStat: st.CleanlinessHundredths,
			Cleanliness: room.CleanlinessLabel(st.CleanlinessHundredths), FilthPenalty: (clean.ImpressivenessTenths - imp) / 10}
	}
	// PLAY-021: a room is lit by a light shining in it now (a powered
	// lamp, a torch lamp or campfire burning, not doused by rain or snow)
	for _, bid := range g.lightSources().ids {
		b := g.bldgs.Ref(bid)
		if b == nil {
			continue
		}
		if on, _, _ := g.shining(bid, *b); on {
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
	if g.feelsSick(id) { // disease.go: infections (MED-02), diseases and parasites (MED-04)
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

// afterEat records what and where pawn id ate from stack st, and rolls
// the meal's food poisoning (food_poison.go).
func (g *Game) afterEat(id entity.EntityID, st item.Stack) {
	k := st.Kind
	switch {
	case k == item.Meal:
		g.remember(id, mood.AteMeal)
	case k.Raw():
		g.remember(id, mood.AteRaw)
	case k == item.NutrientPasteMeal: // STO-2
		g.remember(id, mood.AtePaste)
	}
	pos, _ := g.positions.Get(id)
	if g.nextToTable(pos.X, pos.Y) {
		g.remember(id, mood.AteAtTable)
		g.rest.Waking.Felt(id, rest.Ate) // BED-2: a fine dining room
	} else {
		g.remember(id, mood.AteWithoutTable)
	}
	g.rollFoodPoison(id, st)
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
	case job.Tend, job.EmergencyTend: // EmergencyTend: care/tend.go (MED-03)
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
