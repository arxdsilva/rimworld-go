package game

import (
	"rimworld-go/internal/application/game/aqua"
	"rimworld-go/internal/application/game/industry"
	"sort"

	"rimworld-go/internal/domain/ai/job"
	rules "rimworld-go/internal/domain/building/aqua"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// The shared fuel table (PLAY-021): everything that burns logs.
//
// A campfire heats and lights its room; a torch lamp is a small wood-burning
// light; a wood-fired generator makes power; a passive cooler cools its room
// (TEMP-1: their heat is game/thermo's). All hold fuel as ticks of burning in
// g.fuel (setFuel in light.go is its only writer), burn it in phaseClimate
// (burnFuel), and are fed by haulers (assignRefuel and doRefuel). Rain puts out whatever stands in the open and burns, except a
// machine: a generator burns while it is running (generatorRunning).
//
// The users are listed once per structVer in building order, so a colony
// with campfires only behaves exactly as it did before the table existed.

const (
	refuelWork = 30
	// oilWood: one ember oil burns as long as this many logs (SIM-026).
	oilWood = 3
	// the fuel table's numbers (game/industry/fuel.go)
	torchLogs, torchBurn, generatorBurn, chemfuelTicks = industry.TorchLogs, industry.TorchBurn, industry.GeneratorBurn, industry.ChemfuelTicks
)

// fuelDef is how one kind of building holds and burns fuel.
type fuelDef = industry.Fuel

// fuelSpec is kind k's fuel table row (game/industry/fuel.go, moved by
// AX-4); ok is false for a kind that burns nothing.
func fuelSpec(k construction.Kind) (fuelDef, bool) { return industry.FuelSpec(k) }

// fuelList is the cached list of fuel users, never saved.
type fuelList struct {
	ok      bool
	ver     uint64
	n       int
	bldgs   any // the store it was built from (a load swaps it)
	list    []entity.EntityID
	builds  int // rebuilds so far (tests)
	scratch []fuelCand
}

// fuelCand is a building asking for fuel (assignRefuel).
type fuelCand struct {
	bid        entity.EntityID
	fuel, fill int
}

// fuelUsers is every building that burns fuel, in building order; rebuilt
// only when the structure changes. Callers must not keep it across a
// change to the buildings.
func (g *Game) fuelUsers() []entity.EntityID {
	c := &g.vents.fuel
	if c.ok && c.ver == g.structVer && c.n == len(g.bldgIDs) && c.bldgs == any(g.bldgs) {
		return c.list
	}
	c.list = c.list[:0]
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil {
			if _, ok := fuelSpec(b.Kind); ok {
				c.list = append(c.list, id)
			}
		}
	}
	c.ok, c.ver, c.n, c.bldgs = true, g.structVer, len(g.bldgIDs), g.bldgs
	c.builds++
	return c.list
}

// fuelCap is how many ticks of fuel building bid holds when full (0: it
// takes none).
func (g *Game) fuelCap(bid entity.EntityID) int {
	if b := g.bldgs.Ref(bid); b != nil {
		if s, ok := fuelSpec(b.Kind); ok {
			return s.CapTicks
		}
	}
	return 0
}

// fuelPct is building b's fuel as a percentage of a full tank; ok is
// false for a kind that takes none (BuildingView.Fuel, FuelPct).
func (g *Game) fuelPct(bid entity.EntityID, b construction.Building) (int, bool) {
	s, ok := fuelSpec(b.Kind)
	if !ok {
		return 0, false
	}
	return g.fuel[bid] * 100 / s.CapTicks, true
}

// generatorRunning reports whether wood-fired generator id is on and
// producing, so it burns its fuel. A generator burns nothing while it is
// switched off or the grid is down in a solar flare.
func (g *Game) generatorRunning(id entity.EntityID) bool {
	return g.fuel[id] > 0 && g.powerOn(id) && !g.solarFlare()
}

// burning reports a fuel burner that is actually burning now: it has
// fuel, and a campfire or torch is neither banked nor doused by rain or
// snow. A generator burns while it is running.
func (g *Game) burning(bid entity.EntityID, b construction.Building) bool {
	if b.Kind == construction.FishTank { // FISH-*: fish eat while they live and the tank runs
		return g.fuel[bid] > 0 && g.tanks != nil && g.tanks.Stocked(bid) && g.powered(bid)
	}
	if b.Kind == construction.WoodGenerator || b.Kind == construction.ChemfuelGenerator {
		return g.fuel[bid] > 0 && g.generatorRunning(bid)
	}
	return g.fuel[bid] > 0 && !g.doused(bid) && !g.banked[bid]
}

// burnFuel burns one climate interval of fuel in every burner (game/thermo
// reads which burn for their heat). A campfire banks down in warm air and
// flares up again in cool air.
func (g *Game) burnFuel() {
	for _, bid := range g.fuelUsers() {
		b, ok := g.bldgs.Get(bid)
		if !ok || g.fuel[bid] <= 0 {
			continue
		}
		spec, isFuel := fuelSpec(b.Kind)
		if !isFuel {
			continue
		}
		if spec.Banks {
			switch t := g.temperatureAt(b.X, b.Y); {
			case !g.banked[bid] && t >= campfireBank && uint64(g.clock.Now())%campfireBankEvery == 0:
				g.banked[bid] = true
			case g.banked[bid] && t < campfireRelight:
				delete(g.banked, bid)
			}
		}
		if !g.burning(bid, b) {
			continue // banked, doused by rain or snow, or switched off
		}
		g.setFuel(bid, max(0, g.fuel[bid]-climateInterval*spec.BurnPermil/1000)) // light.go: a burnt-out fire stops lighting
	}
	if g.tanks != nil {
		g.tanks.Phase() // game/aqua (FISH-*): hourly, right after the tanks' feed
	}
}

// --- refuelling ------------------------------------------------------------

// fuelTicks is how long one item of kind k burns (0: not a fuel): a log
// TicksPerWood, an ember oil oilWood logs' worth.
func fuelTicks(k item.Kind) int {
	switch k {
	case item.Wood:
		return TicksPerWood
	case item.EmberOil:
		return oilWood * TicksPerWood
	case item.Chemfuel:
		return chemfuelTicks
	case item.Hay, item.Kibble: // FISH-*: fish feed, as raw plant food below
		return rules.FeedItemTicks
	}
	if (recipe.Input{AnyVeg: true}).Accepts(k) {
		return rules.FeedItemTicks
	}
	return 0
}

// fuelRoom is how many items of fuel k burner bid takes before it is full
// (a part-burnt one rounds up).
func (g *Game) fuelRoom(bid entity.EntityID, k item.Kind) int {
	per := fuelTicks(k)
	if per <= 0 {
		return 0
	}
	return max(0, (g.fuelCap(bid)-g.fuel[bid]+per-1)/per)
}

// assignRefuel sends a hauler with wood (or, with none about, ember oil)
// to the emptiest burner that is half empty or worse (by share of its own
// tank, so a torch and a campfire compare fairly).
func (g *Game) assignRefuel(id entity.EntityID) bool {
	fires := g.vents.fuel.scratch[:0]
	for _, bid := range g.fuelUsers() {
		b := g.bldgs.Ref(bid) // read only: nothing below changes the store
		if b == nil {
			continue
		}
		spec, ok := fuelSpec(b.Kind)
		if !ok || b.Deconstruct || g.fuel[bid] >= spec.Below {
			continue
		}
		if b.Kind == construction.FishTank && (g.tanks == nil || !g.tanks.WantsFeed(bid)) {
			continue // FISH-*: fed once a stock is chosen
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
			continue
		}
		if g.postponedFor(id, job.Refuel, bid, 0, 0) {
			continue // SIM-027: blocked or out of reach lately
		}
		fires = append(fires, fuelCand{bid, g.fuel[bid], g.fuel[bid] * 1000 / spec.CapTicks})
	}
	g.vents.fuel.scratch = fires
	sort.Slice(fires, func(i, j int) bool {
		a, b := fires[i], fires[j]
		if a.fill != b.fill {
			return a.fill < b.fill
		}
		return a.fuel < b.fuel || a.fuel == b.fuel && a.bid < b.bid
	})
	for _, f := range fires {
		b, _ := g.bldgs.Get(f.bid)
		fuel := item.Wood
		if b.Kind == construction.ChemfuelGenerator || b.Kind == construction.PodLauncher {
			fuel = item.Chemfuel // AX-4, AX-9: they take chemfuel only
		} else if b.Kind == construction.FishTank {
			fuel = item.Hay // FISH-*: hay first; tankFeed has the rest
		}
		need := g.fuelRoom(f.bid, fuel)
		sid, st := g.ingredientFor(id, recipe.Input{Kind: fuel, Qty: 1, Max: need})
		if sid == 0 && fuel == item.Hay {
			if sid, st = tankFeed(g, id, need); sid == 0 {
				continue
			}
		} else if sid == 0 && fuel == item.Chemfuel {
			continue
		} else if sid == 0 {
			fuel = item.EmberOil
			need = g.fuelRoom(f.bid, fuel)
			if sid, st = g.ingredientFor(id, recipe.Input{Kind: fuel, Qty: 1, Max: need}); sid == 0 {
				continue // no wood: a tank or a chemfuel burner further on may still be served
			}
		}
		sx, sy, ok := g.besideSpot(id, b.X, b.Y)
		if !ok {
			continue
		}
		rec := job.Record{Kind: job.Refuel, Target: sid, TX: st.X, TY: st.Y, Dest: f.bid,
			DestX: sx, DestY: sy, WorkLeft: refuelWork, Count: need}
		if st.Qty > need {
			// HAUL-001: a full load when a stockpile cell takes the rest
			if px, py, ok := g.stockpileCell(st, b.X, b.Y); ok {
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
		g.routeArrive(id, j) // burner fed: at the stockpile with the rest
		return
	}
	if _, ok := g.bldgs.Get(j.Dest); !ok || g.fuelCap(j.Dest) == 0 {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if j.Carry == 0 {
		st, ok := g.stacks.Get(j.Target)
		if !ok || st.Carrier != 0 || fuelTicks(st.Kind) == 0 {
			j.State, j.Fail = job.Fail, job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		carry := j.Target
		if take := min(st.Qty, max(1, j.Count)); st.Qty > take {
			st.Qty -= take
			g.putStack(j.Target, st)
			carry = g.entities.Next()
			g.itemIDs = append(g.itemIDs, carry)
			g.putStack(carry, item.Stack{Kind: st.Kind, Qty: take, X: st.X, Y: st.Y, Carrier: id})
		} else {
			st.Carrier = id
			g.putStack(j.Target, st)
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
	u := g.workProgress(id, skNone, false) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	full := g.fuelCap(j.Dest)
	if st, ok := g.stacks.Get(j.Carry); ok && len(j.Route) > 0 {
		give := min(st.Qty, g.fuelRoom(j.Dest, st.Kind))
		g.setFuel(j.Dest, min(full, g.fuel[j.Dest]+give*fuelTicks(st.Kind)))
		if st.Qty -= give; st.Qty > 0 {
			g.putStack(j.Carry, st)
			j.RouteI = len(j.Route)
			if !g.routeWalk(id, &j, st.Kind) {
				g.routeArrive(id, j)
			}
			return
		}
	} else if ok {
		g.setFuel(j.Dest, min(full, g.fuel[j.Dest]+st.Qty*fuelTicks(st.Kind)))
	}
	g.deleteItem(j.Carry)
	j.Carry = 0
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// FuelCampfire fills the fire at (x, y) (dev scenarios and fixtures): a
// campfire, a torch lamp or a generator takes a full tank.
func (g *Game) FuelCampfire(x, y int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		full := CampfireCapacity * TicksPerWood
		if c := g.fuelCap(bid); c > 0 {
			full = c
		}
		g.setFuel(bid, full)
	}
}

// --- fish tanks (FISH-*) ---------------------------------------------------

// TankView and TankOption are a fish tank's read model (game/aqua).
type (
	TankView   = aqua.View
	TankOption = aqua.Option
)

// tanksOf is this map's tanks (game/aqua), built on first use.
func tanksOf(g *Game) *aqua.Tanks {
	if g.tanks == nil {
		g.tanks = aqua.New(tankHost{herdHost{g}})
	}
	return g.tanks
}

// tanksApply is command.SetTankStock: tank raises stock count.
func tanksApply(g *Game, tank entity.EntityID, count int) {
	if count >= 0 && count < int(rules.NumSpecies) {
		tanksOf(g).Apply(tank, rules.Species(count))
	}
}

// tankFeed is the nearest feed besides hay for a tank taking up to need
// items: kibble, then raw plant food.
func tankFeed(g *Game, id entity.EntityID, need int) (entity.EntityID, item.Stack) {
	for _, in := range [...]recipe.Input{{Kind: item.Kibble}, {AnyVeg: true}} {
		in.Qty, in.Max = 1, need
		if sid, st := g.ingredientFor(id, in); sid != 0 {
			return sid, st
		}
	}
	return 0, item.Stack{}
}

// tankHost implements aqua.Host over the game (the herd's adapter has the
// handler's half: game/handling.go).
type tankHost struct{ herdHost }

func (h tankHost) Tank(id entity.EntityID) (int, int, bool) {
	if b := h.g.bldgs.Ref(id); b != nil && b.Kind == construction.FishTank {
		return b.X, b.Y, true
	}
	return 0, 0, false
}
func (h tankHost) Workable(id entity.EntityID) bool {
	b := h.g.bldgs.Ref(id)
	return b != nil && !b.Deconstruct && !b.Forbidden && !b.Hostile
}
func (h tankHost) Powered(id entity.EntityID) bool { return h.g.powered(id) }
func (h tankHost) Air(x, y int) temperature.Temp   { return h.g.temperatureAt(x, y) }
func (h tankHost) Feed(id entity.EntityID) int     { return h.g.fuel[id] }
func (h tankHost) Postponed(id entity.EntityID, k job.Kind, t entity.EntityID) bool {
	return h.g.postponedFor(id, k, t, 0, 0)
}
func (h tankHost) PutOff(id entity.EntityID, k job.Kind, t entity.EntityID) {
	h.g.postpone(postKey{Pawn: id, Kind: k, Target: t}, job.Unreachable)
}

// DropFish leaves qty fish, allowed, on the first tile around (x, y) with
// room, or on (x, y) itself.
func (h tankHost) DropFish(x, y, qty int) {
	if !h.g.dryWalkable(x, y) || h.g.itemAt(x, y) != 0 {
		if nx, ny, ok := h.g.spillTile(item.Fish, x, y); ok {
			x, y = nx, ny
		}
	}
	h.g.dropStack(item.Fish, qty, x, y)
}

// BestAnimals is the best Animals level among the colonists.
func (h tankHost) BestAnimals() int {
	best := 0
	for _, id := range h.g.pawnIDs {
		if h.g.colonist(id) {
			best = max(best, int(h.g.skillOf(id, skill.Animals).Level))
		}
	}
	return best
}

// BesideTank is the free tile beside tank's footprint nearest id that id can
// reach (a tank is walked through, so a neighbour of its anchor may be its
// own cell).
func (h tankHost) BesideTank(id, tank entity.EntityID) (int, int, bool) {
	b := h.g.bldgs.Ref(tank)
	if b == nil {
		return 0, 0, false
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	pos, _ := h.g.positions.Get(id)
	best, bx, by := 1<<30, 0, 0
	for _, c := range cells {
		for _, d := range [...][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}} {
			nx, ny := c[0]+d[0], c[1]+d[1]
			if footprintHas(cells, nx, ny) || !h.g.grid.Walkable(nx, ny) || h.g.bedCovering(nx, ny) {
				continue
			}
			if dd := manhattan(pos.X, pos.Y, nx, ny); dd < best && h.g.reachable(id, nx, ny) {
				best, bx, by = dd, nx, ny
			}
		}
	}
	return bx, by, best != 1<<30
}

// footprintHas reports (x, y) among cells.
func footprintHas(cells [][2]int, x, y int) bool {
	for _, c := range cells {
		if c[0] == x && c[1] == y {
			return true
		}
	}
	return false
}
