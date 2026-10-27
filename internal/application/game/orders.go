package game

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

func plantLabel(s plant.Species) string {
	switch s {
	case plant.BerryBush:
		return "berry bush"
	case plant.Tree:
		return "tree"
	case plant.Bramble:
		return "bramble thicket"
	case plant.Moss:
		return "ground moss"
	case plant.Agave:
		return "desert rosette"
	case plant.Dandelion:
		return "dandelions"
	}
	return strings.ReplaceAll(s.String(), "_", " ")
}

// OrderOption is one entry of a pawn's right-click menu at a tile. When
// Disabled, Reason says why; Cmd is still the command it would issue.
type OrderOption struct {
	Label    string
	Cmd      command.Command
	Disabled bool
	Reason   string
}

// Order-menu reasons (player-facing).
const (
	ReasonUnreachable = "cannot reach"
	ReasonNoHaul      = "hauling disabled"
	ReasonNoConstruct = "construction disabled"
	ReasonNoGrow      = "growing disabled"
	ReasonNoWood      = "no free wood"
	ReasonNotReady    = "needs wood first"
	ReasonSupplied    = "already supplied"
	ReasonNotRipe     = "not ready to harvest"
	ReasonOwned       = "owned by "
	ReasonNoStockpile = "no stockpile space"
	ReasonStored      = "already stored"
	ReasonBusyTarget  = "in use by "
	ReasonIncapable   = "not able to act"
	// ReasonNoStuff is followed by the material's name ("no free
	// granite blocks"; SIM-029).
	ReasonNoStuff    = "no free "
	ReasonNoMine     = "mining disabled"
	ReasonNotSowable = "cannot sow here"
	ReasonSowingOff  = "sowing is off in this zone" // PLAY-020: the zone's Allow sowing
	ReasonGone       = "no longer there"
	// ReasonNotYet: a PLAY-024 action not hosted yet (the comms dialog's
	// military aid until Phase 4).
	ReasonNotYet = "not yet"
)

func (g *Game) drafted(id entity.EntityID) bool { return g.draft[id] }

func (g *Game) setDraft(id entity.EntityID, on bool) {
	if !g.alive(id) {
		return
	}
	if g.draft[id] == on {
		return
	}
	g.interrupt(id, job.Cancelled)
	if on {
		g.draft[id] = true
	} else {
		delete(g.draft, id)
	}
}

func (g *Game) name(id entity.EntityID) string {
	n, _ := g.names.Get(id)
	if n == "" {
		return fmt.Sprintf("Colonist %d", id)
	}
	return n
}

// reachable reports whether pawn id could path to (x, y): a region-label
// comparison (regions.go), never a search.
func (g *Game) reachable(id entity.EntityID, x, y int) bool {
	if g.noRegions() {
		_, ok := g.pathToVia(id, x, y, false)
		return ok
	}
	ok := g.reachRegions(id, x, y)
	if selfCheck() {
		if _, want := g.pathToVia(id, x, y, false); want != ok {
			panic(fmt.Sprintf("regions: reachable(%d, %d, %d) = %v, A* says %v", id, x, y, ok, want))
		}
	}
	return ok
}

// forcedJob builds the job a Prioritize order would start, or a reason.
func (g *Game) forcedJob(id entity.EntityID, o command.Order, x, y int, layer construction.Layer) (job.Record, string) {
	if g.fogOrderRefused(o, x, y) { // fog_view.go: only hidden rock can be ordered mined
		return job.Record{}, ReasonFogged
	}
	if why := g.incapableOrder(id, o); why != "" {
		return job.Record{}, why // skills.go: "Incapable: Violence (trait: Gentle)"
	}
	m, _ := g.labor.Get(id)
	switch o {
	case command.OrderBuild:
		fid := g.layerFrameAt(x, y, layer)
		fr, ok := g.frames.Get(fid)
		if !ok {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Construct) {
			return job.Record{}, ReasonNoConstruct
		}
		if why := g.replaceWhy(fr); why != "" {
			return job.Record{}, why
		}
		if !fr.Ready() {
			return job.Record{}, ReasonNotReady
		}
		if r := g.busyBy(id, fid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Construct, Target: fid, TX: x, TY: y, WorkLeft: fr.WorkLeft}, ""
	case command.OrderSupply:
		fid := g.layerFrameAt(x, y, layer)
		fr, ok := g.frames.Get(fid)
		if !ok {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Haul) {
			return job.Record{}, ReasonNoHaul
		}
		if fr.Ready() {
			return job.Record{}, ReasonSupplied
		}
		sk := supplyKind(fr) // items_ng1.go: the material, then the components
		wid, wx, wy := g.nearestFree(id, sk)
		if wid == 0 {
			if sk != item.Wood {
				return job.Record{}, ReasonNoStuff + sk.String()
			}
			return job.Record{}, ReasonNoWood
		}
		return job.Record{Kind: job.Haul, Target: wid, TX: wx, TY: wy, DestX: x, DestY: y, Dest: fid}, ""
	case command.OrderDeconstruct:
		bid := g.layerBldgAt(x, y, layer)
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Deconstruct {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Construct) {
			return job.Record{}, ReasonNoConstruct
		}
		if r := g.busyBy(id, bid); r != "" {
			return job.Record{}, r
		}
		tx, ty := g.workSpot(id, b.X, b.Y)
		return job.Record{Kind: job.Deconstruct, Target: bid, TX: tx, TY: ty, WorkLeft: b.WorkLeft}, ""
	case command.OrderHarvest:
		pid := g.plantAt(x, y)
		p, ok := g.plants.Get(pid)
		if !ok {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Grow) {
			return job.Record{}, ReasonNoGrow
		}
		if !p.Harvestable() {
			return job.Record{}, ReasonNotRipe
		}
		if r := g.busyBy(id, pid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Harvest, Target: pid, TX: x, TY: y, WorkLeft: plant.DefOf(p.Species).HarvestWork}, ""
	case command.OrderSow:
		z, ok := g.zoneAt(x, y)
		if !ok {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Grow) {
			return job.Record{}, ReasonNoGrow
		}
		if z.NoSow {
			return job.Record{}, ReasonSowingOff
		}
		if !g.sowable(x, y) {
			return job.Record{}, ReasonNotSowable
		}
		if owner, taken := g.reserve.TileOwner(x, y); taken && owner != id {
			return job.Record{}, ReasonBusyTarget + g.name(owner) // another grower holds the cell (PLAY-020)
		}
		return job.Record{Kind: job.Sow, TX: x, TY: y, WorkLeft: sowWorkOf(z)}, "" // food.go: the species' own work
	case command.OrderCut:
		pid := g.plantAt(x, y)
		p, ok := g.plants.Get(pid)
		if !ok {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Grow) {
			return job.Record{}, ReasonNoGrow
		}
		if r := g.busyBy(id, pid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Cut, Target: pid, TX: x, TY: y, WorkLeft: plant.DefOf(p.Species).CutWork}, ""
	case command.OrderSmooth:
		return g.forcedSmooth(id, x, y, m.Enabled(work.Construct)) // smooth.go
	case command.OrderMine:
		if !terrain.IsRockWall(g.terr.At(x, y)) {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Mine) {
			return job.Record{}, ReasonNoMine
		}
		if owner, taken := g.reserve.TileOwner(x, y); taken && owner != id {
			return job.Record{}, ReasonBusyTarget + g.name(owner)
		}
		tx, ty, ok := g.reachableSpot(id, x, y)
		if !ok {
			return job.Record{}, ReasonUnreachable
		}
		return job.Record{Kind: job.Mine, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: g.mineWorkAt(x, y)}, ""
	case command.OrderSleep:
		bid := g.layerBldgAt(x, y, construction.LayerObject)
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Kind.Sleeps() {
			return job.Record{}, ReasonGone
		}
		if g.isMedicalBed(b) && !g.mayUseMedicalBed(id) {
			return job.Record{}, ReasonMedicalBed // bedrest.go: for patients only
		}
		if !b.CanClaim(uint64(id)) {
			return job.Record{}, ReasonOwned + g.name(entity.EntityID(b.Owner(uint64(id))))
		}
		sx, sy := g.sleepCellFor(id, b)
		return job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}, ""
	case command.OrderEat:
		iid := g.itemAt(x, y)
		st, ok := g.stacks.Get(iid)
		if !ok || !st.Kind.IsFood() {
			return job.Record{}, ReasonGone
		}
		if r := g.busyBy(id, iid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Eat, Target: iid, TX: x, TY: y, WorkLeft: needs.EatWorkTicks}, ""
	case command.OrderHaul:
		iid := g.itemAt(x, y)
		st, ok := g.stacks.Get(iid)
		if !ok {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Haul) {
			return job.Record{}, ReasonNoHaul
		}
		if g.inMatchingStockpile(st) {
			return job.Record{}, ReasonStored
		}
		dx, dy, ok := g.stockpileCell(st.Kind, x, y)
		if !ok || !g.filterMatches(st.Kind) {
			return job.Record{}, ReasonNoStockpile
		}
		if r := g.busyBy(id, iid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Haul, Target: iid, TX: x, TY: y, DestX: dx, DestY: dy}, ""
	case command.OrderHunt:
		aid := g.animalAt(x, y)
		if aid == 0 {
			return job.Record{}, ReasonGone
		}
		if !m.Enabled(work.Hunt) {
			return job.Record{}, ReasonNoHunt
		}
		if !g.hasRanged(id) {
			return job.Record{}, ReasonNoRanged
		}
		if r := g.busyBy(id, aid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Hunt, Target: aid, TX: x, TY: y, WorkLeft: aimTicks(weaponDefOf(g, id))}, ""
	case command.OrderAttack:
		aid := g.animalAt(x, y)
		if aid == 0 {
			if rid := g.raiderAt(x, y); rid != 0 && g.standingRaider(rid) {
				aid = rid
			}
		}
		if aid == 0 {
			return job.Record{}, ReasonGone
		}
		return job.Record{Kind: job.Attack, Target: aid, TX: x, TY: y, WorkLeft: aimTicks(weaponDefOf(g, id))}, ""
	case command.OrderExtinguish:
		return g.extinguishJob(id, x, y) // firefight.go
	case command.OrderFlick:
		return g.flickJob(id, x, y) // power_flick.go (PLAY-021)
	case command.OrderTrade:
		return g.tradeJob(id, x, y) // trade_session.go (PLAY-024)
	case command.OrderUseComms:
		return g.commsJob(id, x, y) // comms.go (PLAY-024)
	case command.OrderEquip:
		iid := g.itemAt(x, y)
		st, ok := g.stacks.Get(iid)
		if !ok || !st.Kind.IsWeapon() && !st.Kind.IsApparel() {
			return job.Record{}, ReasonGone
		}
		if r := g.busyBy(id, iid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Equip, Target: iid, TX: x, TY: y}, ""
	}
	return job.Record{}, ReasonGone
}

func (g *Game) busyBy(id, target entity.EntityID) string {
	if owner, taken := g.reserve.EntityOwner(target); taken && owner != id {
		return ReasonBusyTarget + g.name(owner)
	}
	return ""
}

func (g *Game) nearestFree(id entity.EntityID, k item.Kind) (entity.EntityID, int, int) {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	var bx, by int
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || st.Kind != k || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(iid); taken && owner != id {
			continue
		}
		d := manhattan(pos.X, pos.Y, st.X, st.Y)
		if d < bestD || (d == bestD && iid < best) {
			best, bestD, bx, by = iid, d, st.X, st.Y
		}
	}
	return best, bx, by
}

func manhattan(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// workSpot is where a pawn stands to work on (x, y): the tile itself if
// walkable, else the nearest walkable neighbour it can reach.
func (g *Game) workSpot(id entity.EntityID, x, y int) (int, int) {
	if g.grid.Walkable(x, y) {
		return x, y
	}
	for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
		nx, ny := x+d[0], y+d[1]
		if g.grid.Walkable(nx, ny) && g.reachable(id, nx, ny) {
			return nx, ny
		}
	}
	return x, y
}

func (g *Game) prioritize(id entity.EntityID, o command.Order, x, y int, layer construction.Layer) {
	if !g.alive(id) {
		return
	}
	rec, reason := g.forcedJob(id, o, x, y, layer)
	if reason != "" {
		return
	}
	if o == command.OrderHarvest {
		if p, ok := g.plants.Get(rec.Target); ok {
			p.Designated = true
			g.plants.Set(rec.Target, p)
			g.notePlantMark(rec.Target) // jobindex.go
		}
	}
	if o == command.OrderMine {
		g.interrupt(id, job.Cancelled)
		g.mines[[2]int{x, y}] = true
		g.minesVer++
		g.startMine(id, x, y, true)
		return
	}
	if o == command.OrderSmooth {
		g.interrupt(id, job.Cancelled)
		g.smooths[[2]int{x, y}] = true
		g.startSmooth(id, x, y, true)
		return
	}
	if o == command.OrderHunt {
		if a, ok := g.animals.Get(rec.Target); ok {
			a.Designated = true
			g.animals.Set(rec.Target, a)
		}
	}
	if o == command.OrderCut {
		if p, ok := g.plants.Get(rec.Target); ok {
			p.Cut = true
			g.plants.Set(rec.Target, p)
			g.notePlantMark(rec.Target) // jobindex.go
		}
	}
	if o == command.OrderSleep {
		if b, _ := g.bldgs.Get(rec.Target); g.isMedicalBed(b) {
			g.bldgs.Set(rec.Target, b.Claim(uint64(id)))
			g.holdRestBed(id, rec.Target) // bedrest.go: a patient's stay; its own bed stays its own
		} else {
			g.releaseBed(id)
			b, _ := g.bldgs.Get(rec.Target)
			g.bldgs.Set(rec.Target, b.Claim(uint64(id)))
		}
		g.sleptAt[id] = rec.Target
	}
	g.interrupt(id, job.Cancelled)
	if o == command.OrderSupply {
		// HAUL-001: the clicked frame first (its whole remaining need),
		// then a route chained from it like any delivery
		fr, _ := g.frames.Get(rec.Dest)
		st, _ := g.stacks.Get(rec.Target)
		route, shares := g.routeFrom(rec.Target, st, slotKey(rec.Dest, st.Kind), supplyNeed(fr, st.Kind))
		route.Forced = true
		g.startDelivery(id, route, shares)
		return
	}
	rec.Forced = true
	g.startJob(id, rec)
}

// OrdersAt lists what pawn id can be told to do at (x, y): "Go here"
// first, then one entry per actionable thing on the tile. Disabled
// entries carry the reason. Read-only.
func (g *Game) OrdersAt(pawn entity.EntityID, x, y int) []OrderOption {
	if !g.grid.InBounds(x, y) {
		return nil
	}
	if g.fogged(x, y) {
		return g.fogOrders(pawn, x, y) // fog_view.go: the dark shows nothing to do
	}
	if v, ok := g.vital.Get(pawn); !ok || v.Status != health.Alive {
		return []OrderOption{{Label: "Go here", Disabled: true, Reason: ReasonIncapable}}
	}
	reach := g.reachable(pawn, x, y)
	var out []OrderOption
	move := OrderOption{Label: "Go here", Cmd: command.Command{Kind: command.MovePawn, Pawn: uint64(pawn), X: x, Y: y}}
	if terrain.IsRockWall(g.terr.At(x, y)) {
		move.Disabled, move.Reason = true, ReasonRock
	} else if !g.grid.Walkable(x, y) {
		move.Disabled, move.Reason = true, ReasonBlocked
	} else if !reach {
		move.Disabled, move.Reason = true, ReasonUnreachable
	}
	out = append(out, move)

	add := func(label string, o command.Order, layer construction.Layer) {
		_, reason := g.forcedJob(pawn, o, x, y, layer)
		opt := OrderOption{Label: label, Cmd: command.Command{
			Kind: command.Prioritize, Pawn: uint64(pawn), X: x, Y: y, Order: o, Layer: uint8(layer),
		}}
		if reason == "" {
			tx, ty := x, y
			switch o {
			case command.OrderDeconstruct:
				tx, ty = g.workSpot(pawn, x, y)
			case command.OrderMine, command.OrderSmooth, command.OrderTrade, command.OrderUseComms:
				tx, ty = -1, -1 // forcedJob already checked a reachable spot (PLAY-024: beside the trader, the console's spot)
			}
			if tx >= 0 && !g.reachable(pawn, tx, ty) {
				reason = ReasonUnreachable
			}
		}
		if reason != "" {
			opt.Disabled, opt.Reason = true, reason
		}
		out = append(out, opt)
	}

	// every layer a piece can stand on: a conduit under a wall (PLAY-021) is
	// ordered apart from the wall on top of it
	for _, layer := range []construction.Layer{construction.LayerObject, construction.LayerFloor, construction.LayerConduit} {
		if fid := g.layerFrameAt(x, y, layer); fid != 0 {
			fr, _ := g.frames.Get(fid)
			if fr.Ready() {
				add("Prioritize building "+fr.Kind.String(), command.OrderBuild, layer)
			} else {
				sk := supplyKind(fr)
				add(fmt.Sprintf("Haul %s to %s (%d/%d)", sk, fr.Kind, supplyHave(fr, sk), supplyNeed(fr, sk)+supplyHave(fr, sk)), command.OrderSupply, layer)
			}
		}
		if bid := g.layerBldgAt(x, y, layer); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			if b.Deconstruct {
				add("Prioritize deconstructing "+b.Kind.String(), command.OrderDeconstruct, layer)
			}
			if b.Kind.Sleeps() {
				add("Sleep in "+b.Kind.String(), command.OrderSleep, layer)
			}
			if g.flickAt(x, y) == bid { // power_flick.go: a pending power switch or toggle (PLAY-021)
				add("Flick "+b.Kind.String(), command.OrderFlick, layer)
			}
		}
	}
	if terrain.IsRockWall(g.terr.At(x, y)) {
		add("Mine rock", command.OrderMine, construction.LayerObject)
	}
	if g.smoothable(x, y) { // smooth.go
		label := "Smooth floor"
		if terrain.IsRockWall(g.terr.At(x, y)) {
			label = "Smooth wall"
		}
		add(label, command.OrderSmooth, construction.LayerObject)
	}
	if z, ok := g.zoneAt(x, y); ok && g.plantAt(x, y) == 0 {
		add("Sow "+plant.Species(z.Crop).String(), command.OrderSow, construction.LayerObject)
	}
	if pid := g.plantAt(x, y); pid != 0 {
		p, _ := g.plants.Get(pid)
		name := plantLabel(p.Species)
		if plant.DefOf(p.Species).Yield > 0 {
			add("Harvest "+name, command.OrderHarvest, construction.LayerObject)
		}
		verb := "Cut "
		if plant.DefOf(p.Species).Trunk {
			verb = "Chop "
		}
		add(verb+name, command.OrderCut, construction.LayerObject)
	}
	if aid := g.animalAt(x, y); aid != 0 {
		a, _ := g.animals.Get(aid)
		if g.drafted(pawn) {
			verb := "Melee attack "
			if g.hasRanged(pawn) {
				verb = "Shoot "
			}
			add(verb+a.Species.String(), command.OrderAttack, construction.LayerObject)
		} else {
			add("Hunt "+a.Species.String(), command.OrderHunt, construction.LayerObject)
		}
	}
	if rid := g.raiderAt(x, y); rid != 0 && g.standingRaider(rid) && g.drafted(pawn) {
		verb := "Melee attack "
		if g.hasRanged(pawn) {
			verb = "Shoot "
		}
		add(verb+"raider "+g.name(rid), command.OrderAttack, construction.LayerObject)
	}
	if iid := g.itemAt(x, y); iid != 0 {
		st, _ := g.stacks.Get(iid)
		if st.Kind.IsWeapon() {
			add(fmt.Sprintf("Equip %s", st.Kind), command.OrderEquip, construction.LayerObject)
		}
		if st.Kind.IsApparel() {
			add(fmt.Sprintf("Wear %s", st.Kind), command.OrderEquip, construction.LayerObject)
		}
		if st.Kind.IsFood() {
			add(fmt.Sprintf("Eat %s", st.Kind), command.OrderEat, construction.LayerObject)
		}
		add(fmt.Sprintf("Haul %d %s to stockpile", st.Qty, st.Kind), command.OrderHaul, construction.LayerObject)
	}
	if g.drafted(pawn) && g.fireAt(x, y) > 0 { // firefight.go: any fire, forced
		add("Extinguish fire", command.OrderExtinguish, construction.LayerObject)
	}
	out = append(out, g.tendOrders(pawn, x, y)...) // care.go (MED-03): drafted "Tend <name>"
	g.tradeOrders(add, x, y)                       // trade_session.go (PLAY-024): "Trade with <name>"
	g.commsOrders(add, x, y)                       // comms.go (PLAY-024): "Use radio console"
	return out
}
