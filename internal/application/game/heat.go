package game

import (
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
)

// Campfire fuel and heat.
const (
	// CampfireCapacity is how much wood a campfire holds.
	CampfireCapacity = 20
	// TicksPerWood is how long one log burns (about ten logs a day).
	TicksPerWood = 6000
	// campfireHeat is the heat a burning fire adds to its room per climate
	// interval, in tenths of a degree times room cells: it can hold a
	// 20-cell room about 30 C above the outside air.
	campfireHeat = 160
	// campfireBank: a fire banks down (burning no fuel, adding no heat)
	// once the air around it reaches 20 C, and campfireRelight: it flares
	// up again when the air cools below 16 C. The gap keeps it from
	// flickering on and off every climate interval.
	campfireBank    = 200
	campfireRelight = 160
	// refuelBelow: haulers top a campfire up once it is half empty.
	refuelBelow = CampfireCapacity * TicksPerWood / 2
	refuelWork  = 30
	// warmWait is the longest a cold colonist waits in a warm spot.
	warmWait = 2500
	// residualLeak (permille per climate interval): a breached room's air
	// loses about half its difference from the outside in an hour.
	residualLeak = 60
)

// --- room temperature ----------------------------------------------------

// remapRoomTemps carries room temperatures across a room rebuild: each new
// room starts at the average of what its cells were (old room temperature,
// or the outside air for cells that were outdoors).
func (g *Game) remapRoomTemps(newGrid []int, n int) []temperature.Temp {
	now := g.clock.Now()
	out := make([]temperature.Temp, n)
	if g.roomGrid == nil { // first build: rooms start sheltered
		for i := range out {
			out[i] = temperature.Indoor(now)
		}
		return out
	}
	sum := make([]int, n)
	cnt := make([]int, n)
	outside := int(temperature.Outdoor(now))
	if g.residual == nil {
		g.residual = map[int]temperature.Temp{}
	}
	for i, r := range newGrid {
		old := room.Outdoors
		if i < len(g.roomGrid) {
			old = g.roomGrid[i]
		}
		if r < 0 {
			// a breached room keeps its air for a while
			if r == room.Outdoors && old >= 0 && old < len(g.roomTemps) {
				g.residual[i] = g.roomTemps[old]
			}
			continue
		}
		t := outside
		switch {
		case old >= 0 && old < len(g.roomTemps):
			t = int(g.roomTemps[old])
		case g.residual[i] != 0:
			t = int(g.residual[i])
		}
		delete(g.residual, i)
		sum[r] += t
		cnt[r]++
	}
	for r := range out {
		if cnt[r] > 0 {
			out[r] = temperature.Temp(sum[r] / cnt[r])
		}
	}
	return out
}

// roomLeak is how fast (permille per climate interval) a room's air drifts
// toward the outside: small rooms swing faster than big ones.
func roomLeak(cells int) int {
	k := 120 / isqrt(cells)
	return max(10, min(80, k))
}

func isqrt(n int) int {
	if n <= 1 {
		return 1
	}
	r := 1
	for (r+1)*(r+1) <= n {
		r++
	}
	return r
}

// heatRooms moves each room toward the outside air and adds heat from its
// burning campfires, which use up fuel while they burn.
func (g *Game) heatRooms() {
	g.ensureRooms()
	now := g.clock.Now()
	outside := temperature.Outdoor(now) + temperature.IndoorBonus
	heat := make([]int, len(g.rooms))
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.Campfire || g.fuel[bid] <= 0 {
			continue
		}
		switch t := g.temperatureAt(b.X, b.Y); {
		case !g.banked[bid] && t >= campfireBank:
			g.banked[bid] = true
		case g.banked[bid] && t < campfireRelight:
			delete(g.banked, bid)
		}
		if !g.burning(bid, b) {
			continue // banked, or doused by rain or snow
		}
		g.fuel[bid] = max(0, g.fuel[bid]-climateInterval)
		if r := g.roomAt(b.X, b.Y); r >= 0 && r < len(heat) {
			heat[r] += campfireHeat
		}
	}
	// breached rooms' air mixes with the outside over a few hours
	raw := temperature.Outdoor(now)
	for i, t := range g.residual {
		t += (raw - t) * residualLeak / 1000
		if d := t - raw; d < 5 && d > -5 {
			delete(g.residual, i)
			continue
		}
		g.residual[i] = t
	}
	for r := range g.roomTemps {
		cells := max(1, g.rooms[r].contents.Cells)
		t := g.roomTemps[r]
		// an unroofed room loses the indoor bonus and leaks up to 5x as
		// fast (SIM-018); fully roofed rooms are unchanged
		target, leak := outside, temperature.Temp(roomLeak(cells))
		if f := g.roomRoofed(r); f < 1000 {
			target = raw + temperature.IndoorBonus*temperature.Temp(f)/1000
			leak = min(1000, leak*temperature.Temp(1000+4*(1000-f))/1000)
		}
		t += (target - t) * leak / 1000
		t += temperature.Temp(heat[r] / cells)
		g.roomTemps[r] = t
	}
}

// burning reports a campfire with fuel that is actually burning now.
func (g *Game) burning(bid entity.EntityID, b construction.Building) bool {
	return g.fuel[bid] > 0 && !g.doused(bid) && !g.banked[bid]
}

// --- refuelling ------------------------------------------------------------

// assignRefuel sends a hauler with wood to the emptiest campfire that is
// half empty or worse.
func (g *Game) assignRefuel(id entity.EntityID) bool {
	type cand struct {
		bid  entity.EntityID
		fuel int
	}
	var fires []cand
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.Campfire || b.Deconstruct || g.fuel[bid] >= refuelBelow {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
			continue
		}
		fires = append(fires, cand{bid, g.fuel[bid]})
	}
	sort.Slice(fires, func(i, j int) bool {
		return fires[i].fuel < fires[j].fuel || fires[i].fuel == fires[j].fuel && fires[i].bid < fires[j].bid
	})
	for _, f := range fires {
		b, _ := g.bldgs.Get(f.bid)
		need := CampfireCapacity - f.fuel/TicksPerWood
		sid, st := g.ingredientFor(id, recipe.Input{Kind: item.Wood, Qty: 1, Max: need})
		if sid == 0 {
			return false
		}
		sx, sy, ok := g.besideSpot(id, b.X, b.Y)
		if !ok {
			continue
		}
		rec := job.Record{Kind: job.Refuel, Target: sid, TX: st.X, TY: st.Y, Dest: f.bid,
			DestX: sx, DestY: sy, WorkLeft: refuelWork, Count: need}
		if st.Qty > need {
			// HAUL-001: a full load when a stockpile cell takes the rest
			if px, py, ok := g.stockpileCell(item.Wood, b.X, b.Y); ok {
				rec.Count = max(need, min(st.Qty, item.CarryLimit))
				rec.Route, rec.SpareX, rec.SpareY, rec.HasSpare = []entity.EntityID{f.bid}, px, py, true
			}
		}
		if !g.startJob(id, rec) {
			continue
		}
		g.reserve.TryEntity(id, f.bid)
		return true
	}
	return false
}

func (g *Game) doRefuel(id entity.EntityID, j job.Record) {
	if j.Carry != 0 && len(j.Route) > 0 && j.RouteI >= len(j.Route) {
		g.routeArrive(id, j) // fire fed: at the stockpile with the rest
		return
	}
	if _, ok := g.bldgs.Get(j.Dest); !ok {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if j.Carry == 0 {
		st, ok := g.stacks.Get(j.Target)
		if !ok || st.Carrier != 0 || st.Kind != item.Wood {
			j.State, j.Fail = job.Fail, job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		carry := j.Target
		if take := min(st.Qty, max(1, j.Count)); st.Qty > take {
			st.Qty -= take
			g.stacks.Set(j.Target, st)
			carry = g.entities.Next()
			g.itemIDs = append(g.itemIDs, carry)
			g.stacks.Set(carry, item.Stack{Kind: item.Wood, Qty: take, X: st.X, Y: st.Y, Carrier: id})
		} else {
			st.Carrier = id
			g.stacks.Set(j.Target, st)
		}
		j.Carry = carry
		path, ok := g.pathTo(id, j.DestX, j.DestY)
		if !ok {
			j.State, j.Fail = job.Fail, job.Unreachable
			g.jobs.Set(id, j)
			return
		}
		j.TX, j.TY = j.DestX, j.DestY
		j.Path, j.PathI = path, 0
		if len(path) > 0 {
			j.State = job.Goto
		}
		g.jobs.Set(id, j)
		return
	}
	if !g.workTick(id) {
		g.jobs.Set(id, j)
		return
	}
	if j.WorkLeft--; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	if st, ok := g.stacks.Get(j.Carry); ok && len(j.Route) > 0 {
		give := min(st.Qty, CampfireCapacity-g.fuel[j.Dest]/TicksPerWood)
		g.fuel[j.Dest] = min(CampfireCapacity*TicksPerWood, g.fuel[j.Dest]+give*TicksPerWood)
		if st.Qty -= give; st.Qty > 0 {
			g.stacks.Set(j.Carry, st)
			j.RouteI = len(j.Route)
			if !g.routeWalk(id, &j, st.Kind) {
				g.routeArrive(id, j)
			}
			return
		}
	} else if ok {
		g.fuel[j.Dest] = min(CampfireCapacity*TicksPerWood, g.fuel[j.Dest]+st.Qty*TicksPerWood)
	}
	g.deleteItem(j.Carry)
	j.Carry = 0
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// FuelCampfire fills a campfire at (x, y) (dev scenarios and fixtures).
func (g *Game) FuelCampfire(x, y int) {
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		g.fuel[bid] = CampfireCapacity * TicksPerWood
	}
}

// --- exposure ----------------------------------------------------------------

// expose builds or relieves hypothermia and heatstroke from the air around
// each colonist; serious exposure downs them, fatal exposure kills.
func (g *Game) expose() {
	for _, id := range g.pawnIDs {
		v, ok := g.vital.Get(id)
		if !ok || v.Status == health.Dead {
			continue
		}
		pos, _ := g.positions.Get(id)
		t := int(g.temperatureAt(pos.X, pos.Y))
		e := g.exposure[id]
		e.hypo, e.heat = exposure.Step(e.hypo, e.heat, t, climateInterval, g.bandsOf(id))
		g.exposure[id] = e
		switch {
		case e.hypo >= exposure.Fatal:
			g.die(id, health.CauseHypothermia)
			g.message(pos.X, pos.Y, "%s froze to death", g.nameOf(id))
		case e.heat >= exposure.Fatal:
			g.die(id, health.CauseHeatstroke)
			g.message(pos.X, pos.Y, "%s died of heatstroke", g.nameOf(id))
		case v.Status == health.Alive && e.hypo >= exposure.Serious:
			g.logEvent(id, logEntry{Kind: logHypothermia})
			g.down(id, health.CauseHypothermia)
			g.message(pos.X, pos.Y, "%s collapsed from hypothermia", g.nameOf(id))
		case v.Status == health.Alive && e.heat >= exposure.Serious:
			g.logEvent(id, logEntry{Kind: logHeatstroke})
			g.down(id, health.CauseHeatstroke)
			g.message(pos.X, pos.Y, "%s collapsed from heatstroke", g.nameOf(id))
		case v.Status == health.Downed && (v.Cause == health.CauseHypothermia && e.hypo < exposure.Recover ||
			v.Cause == health.CauseHeatstroke && e.heat < exposure.Recover):
			v.Status, v.Cause = health.Alive, health.CauseNone
			g.vital.Set(id, v)
		}
	}
}

type exposureState struct{ hypo, heat int }

func (g *Game) nameOf(id entity.EntityID) string {
	n, _ := g.names.Get(id)
	return n
}

// needsComfort reports a colonist suffering exposure where it stands.
func (g *Game) needsComfort(id entity.EntityID) bool {
	e := g.exposure[id]
	pos, _ := g.positions.Get(id)
	t := int(g.temperatureAt(pos.X, pos.Y))
	b := g.bandsOf(id)
	return e.hypo >= exposure.Mild && t < b.Min || e.heat >= exposure.Mild && t > b.Max
}

// warmSpot finds the nearest reachable spot in a room with bearable air for
// a colonist suffering exposure. ok is false when there is none.
func (g *Game) warmSpot(id entity.EntityID) (int, int, bool) {
	g.ensureRooms()
	pos, _ := g.positions.Get(id)
	cold := g.exposure[id].hypo >= g.exposure[id].heat
	bands := g.bandsOf(id)
	ok := func(r int) bool {
		if r < 0 || r >= len(g.roomTemps) {
			return false
		}
		t := int(g.roomTemps[r])
		if cold {
			return t >= bands.Min
		}
		return t <= bands.Max
	}
	type spot struct{ x, y, d int }
	var spots []spot
	w := g.grid.Width()
	for i, r := range g.roomGrid {
		if r == room.Wall || !ok(r) {
			continue
		}
		x, y := i%w, i/w
		if !g.grid.Walkable(x, y) || g.bedCovering(x, y) {
			continue
		}
		if owner, taken := g.reserve.TileOwner(x, y); taken && owner != id {
			continue
		}
		spots = append(spots, spot{x, y, pathing.Dist(pos.X, pos.Y, x, y)})
	}
	sort.Slice(spots, func(i, j int) bool {
		if spots[i].d != spots[j].d {
			return spots[i].d < spots[j].d
		}
		return spots[i].y*w+spots[i].x < spots[j].y*w+spots[j].x
	})
	for i, s := range spots {
		if i >= 12 {
			break
		}
		if g.reachable(id, s.x, s.y) {
			return s.x, s.y, true
		}
	}
	return 0, 0, false
}

// assignWarm walks pawn id to the nearest bearable spot, if any.
func (g *Game) assignWarm(id entity.EntityID) bool {
	x, y, ok := g.warmSpot(id)
	return ok && g.startJob(id, job.Record{Kind: job.Warm, TX: x, TY: y, WorkLeft: warmWait})
}

// exposureDanger: hypothermia or heatstroke bad enough to get a sleeper
// out of bed (a little chill only matters once it is awake).
func (g *Game) exposureDanger(id entity.EntityID) bool {
	e := g.exposure[id]
	return e.hypo >= exposure.Recover || e.heat >= exposure.Recover
}

// warmRetry is how long a colonist that found no bearable spot waits
// before looking again.
const warmRetry = 1000

func (g *Game) noComfortUntil(id entity.EntityID) bool {
	return uint64(g.clock.Now()) < g.noComfort[id]
}

// doWarm waits in the warm spot until the exposure has faded.
func (g *Game) doWarm(id entity.EntityID, j job.Record) {
	e := g.exposure[id]
	j.WorkLeft--
	if j.WorkLeft <= 0 || e.hypo == 0 && e.heat == 0 {
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
}

// refuelEnabled reports whether a pawn's work settings allow refuelling.

// SkipTo jumps the calendar forward to day (0-based) at hour without
// simulating the time in between (dev scenarios and fixtures).
func (g *Game) SkipTo(day, hour int) {
	target := clock.Tick(day*clock.TicksPerDay + hour*clock.TicksPerHour)
	if target > g.clock.Now() {
		g.clock.Advance(int(target - g.clock.Now()))
	}
}

// ClearPlants removes every plant in the rectangle (dev scenarios).
func (g *Game) ClearPlants(x0, y0, x1, y1 int) {
	for _, id := range append([]entity.EntityID(nil), g.plantIDs...) {
		if p, ok := g.plants.Get(id); ok && p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
			g.removePlant(id)
		}
	}
}

// woodLow reports the colony short of wood: less on the ground than two
// days of burning for every campfire, or under 20 logs.
func (g *Game) woodLow() bool {
	fires := 0
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind == construction.Campfire {
			fires++
		}
	}
	want := max(20, fires*2*clock.TicksPerDay/TicksPerWood)
	return g.stockOf(item.Wood) < want
}
