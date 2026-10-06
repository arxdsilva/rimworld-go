package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

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

// reachable reports whether pawn id could path to (x, y).
func (g *Game) reachable(id entity.EntityID, x, y int) bool {
	_, ok := g.pathTo(id, x, y)
	return ok
}

// forcedJob builds the job a Prioritize order would start, or a reason.
func (g *Game) forcedJob(id entity.EntityID, o command.Order, x, y int, layer construction.Layer) (job.Record, string) {
	m, _ := g.labor.Get(id)
	switch o {
	case command.OrderBuild:
		fid := g.layerFrameAt(x, y, layer)
		fr, ok := g.frames.Get(fid)
		if !ok {
			return job.Record{}, ReasonUnknown
		}
		if !m.Enabled(work.Construct) {
			return job.Record{}, ReasonNoConstruct
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
			return job.Record{}, ReasonUnknown
		}
		if !m.Enabled(work.Haul) {
			return job.Record{}, ReasonNoHaul
		}
		if fr.Ready() {
			return job.Record{}, ReasonSupplied
		}
		wid, wx, wy := g.nearestFree(id, item.Wood)
		if wid == 0 {
			return job.Record{}, ReasonNoWood
		}
		return job.Record{Kind: job.Haul, Target: wid, TX: wx, TY: wy, DestX: x, DestY: y, Dest: fid}, ""
	case command.OrderDeconstruct:
		bid := g.layerBldgAt(x, y, layer)
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Deconstruct {
			return job.Record{}, ReasonUnknown
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
			return job.Record{}, ReasonUnknown
		}
		if !m.Enabled(work.Grow) {
			return job.Record{}, ReasonNoGrow
		}
		if !p.Harvestable {
			return job.Record{}, ReasonNotRipe
		}
		if r := g.busyBy(id, pid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Harvest, Target: pid, TX: x, TY: y, WorkLeft: plant.HarvestWork}, ""
	case command.OrderSleep:
		bid := g.layerBldgAt(x, y, construction.LayerObject)
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.Bed {
			return job.Record{}, ReasonUnknown
		}
		if b.Claimant != 0 && entity.EntityID(b.Claimant) != id {
			return job.Record{}, ReasonOwned + g.name(entity.EntityID(b.Claimant))
		}
		if r := g.busyBy(id, bid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Sleep, Target: bid, TX: x, TY: y, WorkLeft: needs.SleepWorkMin}, ""
	case command.OrderEat:
		iid := g.itemAt(x, y)
		st, ok := g.stacks.Get(iid)
		if !ok || st.Kind != item.Food {
			return job.Record{}, ReasonUnknown
		}
		if r := g.busyBy(id, iid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Eat, Target: iid, TX: x, TY: y, WorkLeft: needs.EatWorkTicks}, ""
	case command.OrderHaul:
		iid := g.itemAt(x, y)
		st, ok := g.stacks.Get(iid)
		if !ok {
			return job.Record{}, ReasonUnknown
		}
		if !m.Enabled(work.Haul) {
			return job.Record{}, ReasonNoHaul
		}
		if g.inMatchingStockpile(st) {
			return job.Record{}, ReasonStored
		}
		dx, dy, ok := g.emptyStockpileCell(st.Kind)
		if !ok || !g.filterMatches(st.Kind) {
			return job.Record{}, ReasonNoStockpile
		}
		if r := g.busyBy(id, iid); r != "" {
			return job.Record{}, r
		}
		return job.Record{Kind: job.Haul, Target: iid, TX: x, TY: y, DestX: dx, DestY: dy}, ""
	}
	return job.Record{}, ReasonUnknown
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
		if !ok || st.Kind != k || st.Qty <= 0 || st.Carrier != 0 {
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

func (g *Game) plantAt(x, y int) entity.EntityID {
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if ok && p.X == x && p.Y == y {
			return id
		}
	}
	return 0
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
		}
	}
	if o == command.OrderSleep {
		b, _ := g.bldgs.Get(rec.Target)
		b.Claimant = uint64(id)
		g.bldgs.Set(rec.Target, b)
		g.sleptAt[id] = rec.Target
	}
	g.interrupt(id, job.Cancelled)
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
	if v, ok := g.vital.Get(pawn); !ok || v.Status != health.Alive {
		return []OrderOption{{Label: "Go here", Disabled: true, Reason: ReasonIncapable}}
	}
	reach := g.reachable(pawn, x, y)
	var out []OrderOption
	move := OrderOption{Label: "Go here", Cmd: command.Command{Kind: command.MovePawn, Pawn: uint64(pawn), X: x, Y: y}}
	if !g.grid.Walkable(x, y) {
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
			if o == command.OrderDeconstruct {
				tx, ty = g.workSpot(pawn, x, y)
			}
			if !g.reachable(pawn, tx, ty) {
				reason = ReasonUnreachable
			}
		}
		if reason != "" {
			opt.Disabled, opt.Reason = true, reason
		}
		out = append(out, opt)
	}

	for _, layer := range []construction.Layer{construction.LayerObject, construction.LayerFloor} {
		if fid := g.layerFrameAt(x, y, layer); fid != 0 {
			fr, _ := g.frames.Get(fid)
			if fr.Ready() {
				add("Prioritize building "+fr.Kind.String(), command.OrderBuild, layer)
			} else {
				add(fmt.Sprintf("Haul wood to %s (%d/%d)", fr.Kind, fr.Wood, fr.Need), command.OrderSupply, layer)
			}
		}
		if bid := g.layerBldgAt(x, y, layer); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			if b.Deconstruct {
				add("Prioritize deconstructing "+b.Kind.String(), command.OrderDeconstruct, layer)
			}
			if b.Kind == construction.Bed {
				add("Sleep in bed", command.OrderSleep, layer)
			}
		}
	}
	if pid := g.plantAt(x, y); pid != 0 {
		add("Harvest berry bush", command.OrderHarvest, construction.LayerObject)
	}
	if iid := g.itemAt(x, y); iid != 0 {
		st, _ := g.stacks.Get(iid)
		if st.Kind == item.Food {
			add(fmt.Sprintf("Eat %s", st.Kind), command.OrderEat, construction.LayerObject)
		}
		add(fmt.Sprintf("Haul %d %s to stockpile", st.Qty, st.Kind), command.OrderHaul, construction.LayerObject)
	}
	return out
}
