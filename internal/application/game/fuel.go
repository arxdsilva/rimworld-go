package game

import (
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// The shared fuel table (PLAY-021): everything that burns logs.
//
// A campfire heats and lights its room; a torch lamp is a small wood-burning
// light; a wood-fired generator makes power. All three hold fuel as ticks of
// burning in g.fuel (setFuel in light.go is its only writer), burn it in
// phaseClimate (heatRooms), and are fed by haulers (assignRefuel and
// doRefuel). Rain puts out whatever stands in the open and burns, except a
// machine: a generator burns while it is running (generatorRunning).
//
// The users are listed once per structVer in building order, so a colony
// with campfires only behaves exactly as it did before the table existed.

const (
	// refuelBelow: haulers top a campfire up once it is half empty.
	refuelBelow = CampfireCapacity * TicksPerWood / 2
	refuelWork  = 30
	// oilWood: one ember oil burns as long as this many logs (SIM-026).
	oilWood = 3
	// torchLogs and generatorLogs are how many logs a full tank takes.
	torchLogs     = 8
	generatorLogs = 15
	// torchBurn and generatorBurn are the ticks of fuel used per 1000 ticks
	// of burning (campfire: 1000, one for one).
	torchBurn     = 600
	generatorBurn = 1600
)

// fuelDef is how one kind of building holds and burns fuel.
type fuelDef struct {
	// CapTicks is a full tank, in ticks of fuel.
	CapTicks int
	// Below: haulers bring more once the tank falls under this.
	Below int
	// BurnPermil is the fuel used per 1000 ticks of burning: 1000 burns
	// one tick of fuel per tick, 1600 one and a half times as fast.
	BurnPermil int
	// Banks: the fire banks down in warm air (campfires only).
	Banks bool
	// Heat is what burning adds to the room per climate interval, in
	// tenths of a degree times room cells (0: none).
	Heat int
}

// fuelSpec is kind k's fuel table row; ok is false for a kind that burns
// nothing.
func fuelSpec(k construction.Kind) (fuelDef, bool) {
	switch k {
	case construction.Campfire:
		return fuelDef{CapTicks: CampfireCapacity * TicksPerWood, Below: refuelBelow, BurnPermil: 1000,
			Banks: true, Heat: campfireHeat}, true
	case construction.TorchLamp:
		return fuelDef{CapTicks: torchLogs * TicksPerWood, Below: torchLogs * TicksPerWood / 2, BurnPermil: torchBurn}, true
	case construction.WoodGenerator:
		return fuelDef{CapTicks: generatorLogs * TicksPerWood, Below: generatorLogs * TicksPerWood / 2, BurnPermil: generatorBurn}, true
	case construction.FueledSmithy, construction.Stove: // PRD-3, FOOD-4, wiki: Fueled smithy, Fueled stove (50 logs, scaled / 10); burns only while worked (craft.WorkPerWood)
		return fuelDef{CapTicks: 5 * TicksPerWood, Below: 5 * TicksPerWood / 2}, true
	}
	return fuelDef{}, false
}

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
	if b.Kind == construction.WoodGenerator {
		return g.fuel[bid] > 0 && g.generatorRunning(bid)
	}
	return g.fuel[bid] > 0 && !g.doused(bid) && !g.banked[bid]
}

// burnFuel burns one climate interval of fuel in every burner and adds the
// heat of the ones that heat their room to heat (per room id). A campfire
// banks down in warm air and flares up again in cool air.
func (g *Game) burnFuel(heat []int) {
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
			case !g.banked[bid] && t >= campfireBank:
				g.banked[bid] = true
			case g.banked[bid] && t < campfireRelight:
				delete(g.banked, bid)
			}
		}
		if !g.burning(bid, b) {
			continue // banked, doused by rain or snow, or switched off
		}
		g.setFuel(bid, max(0, g.fuel[bid]-climateInterval*spec.BurnPermil/1000)) // light.go: a burnt-out fire stops lighting
		if spec.Heat > 0 {
			if r := g.roomAt(b.X, b.Y); r >= 0 && r < len(heat) {
				heat[r] += spec.Heat
			}
		}
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
		need := g.fuelRoom(f.bid, fuel)
		sid, st := g.ingredientFor(id, recipe.Input{Kind: fuel, Qty: 1, Max: need})
		if sid == 0 {
			fuel = item.EmberOil
			need = g.fuelRoom(f.bid, fuel)
			if sid, st = g.ingredientFor(id, recipe.Input{Kind: fuel, Qty: 1, Max: need}); sid == 0 {
				return false
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
