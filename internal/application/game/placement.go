package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// Placement reasons are player-facing strings shown by the build preview.
const (
	ReasonOutside  = "outside the map"
	ReasonBlocked  = "blocked by a wall"
	ReasonOccupied = "already built here"
	ReasonPlanned  = "already planned here"
	ReasonItem     = "items in the way"
	ReasonUnknown  = "unknown building"
	ReasonRock     = "rock in the way"
	ReasonTree     = "tree in the way"
	ReasonStuff    = "cannot be built from that"
	ReasonSpot     = "work spot is blocked"
)

// BuildableView is the read model of one buildable definition.
type BuildableView struct {
	ID        uint8 // value for command.Command.Build
	Kind      string
	Wood      int
	Work      int
	Rotatable bool
	Floor     bool // floor layer: can sit under furniture
	Blocks    bool // completed building blocks movement
	StoneOK   bool // can be built from stone blocks
	W, H      int  // footprint facing south
	Bench     bool // runs bills
	// Stuffs are the materials it may be built from, default first ("wood",
	// "stone"; a carpet lists the carpet items). Carpet marks the entrance
	// carpet, installed from a crafted carpet item (SIM-021).
	Stuffs []string
	// Catches is the percent of tracked dirt each material wipes off,
	// parallel to Stuffs; 0 for non-carpets.
	Catches []int
	Carpet  bool
	// Locked is why this colony cannot build it ("" when it can):
	// (*Game).Buildables sets ReasonScenario for a disallowed building.
	Locked string
}

// Buildables lists every buildable in catalog order.
func Buildables() []BuildableView {
	defs := construction.All()
	out := make([]BuildableView, 0, len(defs))
	for _, d := range defs {
		out = append(out, BuildableView{
			ID: uint8(d.Kind), Kind: d.Name, Wood: d.Wood, Work: d.Work, Rotatable: d.Rotatable,
			Floor: d.Layer == construction.LayerFloor, Blocks: d.BlocksMove, StoneOK: d.StoneOK,
			W: max1(d.Size[0]), H: max1(d.Size[1]), Bench: d.Bench,
		})
		for _, s := range construction.Stuffs(d.Kind) {
			out[len(out)-1].Stuffs = append(out[len(out)-1].Stuffs, s.String())
			out[len(out)-1].Catches = append(out[len(out)-1].Catches, filth.Catch(s)/10)
		}
		out[len(out)-1].Carpet = d.Carpet
	}
	return out
}

// StuffID is the command value (command.Command.Stuff) for a material name.
func StuffID(name string) (uint8, bool) {
	if k, ok := stuffByName(name); ok {
		return uint8(k), true
	}
	k, ok := item.ParseKind(name)
	if !ok || !k.IsCarpet() {
		return 0, false
	}
	return uint8(k), true
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// CanPlace reports whether a south-facing blueprint of kind could be
// placed with its top-left at (x, y) now, and why not.
func (g *Game) CanPlace(kind string, x, y int) (bool, string) {
	k, ok := construction.Parse(kind)
	if !ok {
		return false, ReasonUnknown
	}
	return g.canPlaceAt(k, construction.South, x, y)
}

// CanPlaceAt is the full placement rule the Architect preview and
// DesignateBuild share: material, facing and every footprint cell.
func (g *Game) CanPlaceAt(kind, stuff, facing string, x, y int) (bool, string) {
	k, ok := construction.Parse(kind)
	if !ok {
		return false, ReasonUnknown
	}
	s, ok := item.ParseKind(stuff)
	if !ok || !construction.StuffOK(k, s) {
		return false, ReasonStuff
	}
	if p := g.lockedBy("build:" + k.String()); p != "" {
		return false, "needs research: " + p
	}
	return g.canPlaceAt(k, parseFacingName(facing), x, y)
}

func parseFacingName(f string) construction.Facing {
	for v := construction.South; v <= construction.West; v++ {
		if v.String() == f {
			return v
		}
	}
	return construction.South
}

func (g *Game) canPlaceAt(k construction.Kind, f construction.Facing, x, y int) (bool, string) {
	cells, spot, hasSpot := construction.Footprint(k, f, x, y)
	for _, c := range cells {
		if ok, r := g.canPlace(k, c[0], c[1]); !ok {
			return false, r
		}
	}
	if hasSpot {
		sx, sy := spot[0], spot[1]
		if !g.grid.InBounds(sx, sy) || !g.grid.Walkable(sx, sy) || g.blockingFrameAt(sx, sy) != 0 {
			return false, ReasonSpot
		}
	}
	// a new blocker must not cover an existing bench's work spot
	if construction.DefOf(k).BlocksMove {
		for _, c := range cells {
			if g.isWorkSpot(c[0], c[1]) {
				return false, ReasonSpot
			}
		}
	}
	return true, ""
}

func (g *Game) isWorkSpot(x, y int) bool {
	check := func(k construction.Kind, f construction.Facing, bx, by int) bool {
		_, s, ok := construction.Footprint(k, f, bx, by)
		return ok && s[0] == x && s[1] == y
	}
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && check(b.Kind, b.Facing, b.X, b.Y) {
			return true
		}
	}
	for _, id := range g.frameIDs {
		if fr, ok := g.frames.Get(id); ok && check(fr.Kind, fr.Facing, fr.X, fr.Y) {
			return true
		}
	}
	return false
}

func (g *Game) canPlace(k construction.Kind, x, y int) (bool, string) {
	if !g.grid.InBounds(x, y) {
		return false, ReasonOutside
	}
	if g.scenarioForbids(k) { // setup.go: the scenario disallows it (NG-1)
		return false, ReasonScenario
	}
	layer := construction.DefOf(k).Layer
	rep := g.replaceTarget(k, x, y) // a door or gate may stand where a wall or fence does
	if rep == 0 && g.layerBldgAt(x, y, layer) != 0 {
		return false, ReasonOccupied
	}
	if g.layerFrameAt(x, y, layer) != 0 {
		return false, ReasonPlanned
	}
	if terrain.IsRockWall(g.terr.At(x, y)) {
		return false, ReasonRock
	}
	if ok, r := g.waterPlacement(x, y); !ok { // water.go: nothing is built on water, floors included
		return false, r
	}
	if layer == construction.LayerFloor {
		// floors go under furniture, even benches that block the way, but
		// never into a wall, a fence post or a grave (SIM-028)
		if g.floorBarrierAt(x, y) {
			return false, ReasonBlocked
		}
	} else if rep == 0 && !g.grid.Walkable(x, y) {
		return false, ReasonBlocked
	}
	if pid := g.plantAt(x, y); pid != 0 {
		if p, _ := g.plants.Get(pid); plant.DefOf(p.Species).Trunk && p.Species != plant.Cactus {
			return false, ReasonTree
		}
	}
	if construction.DefOf(k).BlocksMove && g.itemAt(x, y) != 0 {
		return false, ReasonItem
	}
	return true, ""
}

// replaceTarget is the wall, fence, door or gate a door or gate of kind
// k placed at (x, y) would replace (0: none). Same kind over same kind
// is not a replacement.
func (g *Game) replaceTarget(k construction.Kind, x, y int) entity.EntityID {
	if k != construction.Door && k != construction.FenceGate {
		return 0
	}
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	b, ok := g.bldgs.Get(bid)
	if !ok || b.Kind == k {
		return 0
	}
	switch b.Kind {
	case construction.Wall, construction.Fence, construction.Door, construction.FenceGate:
		return bid
	}
	return 0
}

// ReplacesAt names what a door or gate of kind would replace at (x, y)
// ("wall", "fence", ...); "" when it replaces nothing. Read-only; the
// Architect ghost reads "replaces wall".
func (g *Game) ReplacesAt(kind string, x, y int) string {
	k, ok := construction.Parse(kind)
	if !ok {
		return ""
	}
	if bid := g.replaceTarget(k, x, y); bid != 0 {
		b, _ := g.bldgs.Get(bid)
		return construction.DefOf(b.Kind).Name
	}
	return ""
}

// replacedStanding is the piece frame fr replaces while it still stands
// (0: none, the frame may be built).
func (g *Game) replacedStanding(fr construction.Frame) entity.EntityID {
	if fr.Replaces == 0 {
		return 0
	}
	if _, ok := g.bldgs.Get(fr.Replaces); !ok {
		return 0
	}
	return fr.Replaces
}

// replaceWhy is the build refusal while the old piece stands ("" when
// the frame is free to build).
func (g *Game) replaceWhy(fr construction.Frame) string {
	bid := g.replacedStanding(fr)
	if bid == 0 {
		return ""
	}
	b, _ := g.bldgs.Get(bid)
	return "The " + construction.DefOf(b.Kind).Name + " must come down first"
}

// unmarkReplaced clears the deconstruct mark a replacing frame set, when
// the frame is cancelled.
func (g *Game) unmarkReplaced(fr construction.Frame) {
	if !fr.ReplaceMarked || fr.Replaces == 0 {
		return
	}
	if b, ok := g.bldgs.Get(fr.Replaces); ok && b.Deconstruct {
		b.Deconstruct = false
		g.bldgs.Set(fr.Replaces, b)
	}
}

// cancelReplacing cancels the frame that replaces building bid (its
// deconstruct mark was cleared).
func (g *Game) cancelReplacing(bid entity.EntityID) {
	for _, fid := range append([]entity.EntityID(nil), g.frameIDs...) {
		if fr, ok := g.frames.Get(fid); ok && fr.Replaces == bid {
			g.cancelFrame(fid)
		}
	}
}

// floorBarrierAt: (x, y) holds, or is planned to hold, an object no
// floor goes under: a wall, a fence or gate, or a dug grave.
func (g *Game) floorBarrierAt(x, y int) bool {
	barrier := func(k construction.Kind) bool {
		d := construction.DefOf(k)
		return k == construction.Wall || d.AnimalBlock || d.Dug
	}
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		if b, _ := g.bldgs.Get(bid); barrier(b.Kind) {
			return true
		}
	}
	if fid := g.layerFrameAt(x, y, construction.LayerObject); fid != 0 {
		if fr, _ := g.frames.Get(fid); barrier(fr.Kind) {
			return true
		}
	}
	return false
}

func (g *Game) placeFrame(kind construction.Kind, x, y int) {
	g.placeFrameStuff(kind, item.Wood, construction.South, x, y)
}

// CanPlaceStuff is CanPlace plus the material check.
func (g *Game) CanPlaceStuff(kind, stuff string, x, y int) (bool, string) {
	k, ok := construction.Parse(kind)
	if !ok {
		return false, ReasonUnknown
	}
	s, ok := item.ParseKind(stuff)
	if !ok || !construction.StuffOK(k, s) {
		return false, ReasonStuff
	}
	return g.canPlace(k, x, y)
}

func (g *Game) placeFrameStuff(kind construction.Kind, stuff item.Kind, f construction.Facing, x, y int) {
	if !kind.Valid() || !construction.StuffOK(kind, stuff) {
		return
	}
	if !construction.DefOf(kind).Rotatable {
		f = construction.South
	}
	if ok, _ := g.canPlaceAt(kind, f, x, y); !ok {
		return
	}
	id := g.entities.Next()
	g.frameIDs = append(g.frameIDs, id)
	fr := construction.Frame{
		Kind:     kind,
		Stuff:    stuff,
		Facing:   f,
		X:        x,
		Y:        y,
		Need:     construction.WoodCost(kind),
		CompNeed: construction.ComponentCost(kind), // delivered after the material (haul_route.go)
		WorkLeft: construction.WorkFor(kind, stuff),
	}
	if cells, _, _ := construction.Footprint(kind, f, x, y); len(cells) > 0 {
		if rep := g.replaceTarget(kind, cells[0][0], cells[0][1]); rep != 0 {
			fr.Replaces = rep // the old piece comes down first
			if b, _ := g.bldgs.Get(rep); !b.Deconstruct {
				b.Deconstruct, b.WorkLeft = true, construction.DeconstructTicks(b.Kind)
				g.bldgs.Set(rep, b)
				fr.ReplaceMarked = true
			}
		}
	}
	g.frames.Set(id, fr)
	g.noteFrame(id) // indexes.go
	if kind.Instant() {
		g.completeFrame(id, fr) // a marking: nothing to haul or build
	}
}

// ejectPawns moves anyone standing on a tile that just became blocked to
// the first walkable neighbour (fixed order, so deterministic).
func (g *Game) ejectPawns(x, y int) {
	for _, id := range g.pawnIDs {
		pos, _ := g.positions.Get(id)
		if pos.X != x || pos.Y != y {
			continue
		}
		for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
			nx, ny := x+d[0], y+d[1]
			if g.grid.Walkable(nx, ny) {
				g.positions.Set(id, Position{X: nx, Y: ny})
				break
			}
		}
	}
}

func (g *Game) markDeconstruct(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := normalizeRect(x1, y1, x2, y2)
	var erase []entity.EntityID
	defer func() {
		for _, id := range erase {
			g.removeBuilding(id) // markings are simply erased
		}
	}()
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok || b.X < minX || b.X > maxX || b.Y < minY || b.Y > maxY {
			continue
		}
		if b.Deconstruct == on {
			continue
		}
		if on && b.Kind.Instant() && !construction.DefOf(b.Kind).Placed { // a pod shell is salvaged, not erased
			erase = append(erase, id)
			continue
		}
		if on && g.graveFilled(id) {
			continue // someone lies there (burial.go)
		}
		b.Deconstruct = on
		if on {
			b.WorkLeft = construction.DeconstructTicks(b.Kind)
		}
		g.bldgs.Set(id, b)
		if !on {
			g.cancelReplacing(id) // a door or gate waiting for this piece is dropped
		}
	}
}

func normalizeRect(x1, y1, x2, y2 int) (int, int, int, int) {
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	return x1, y1, x2, y2
}

// removeBuilding deletes a completed building, refunds wood and frees
// its tile and bed claim.
func (g *Game) removeBuilding(id entity.EntityID) {
	b, ok := g.bldgs.Get(id)
	if !ok {
		return
	}
	g.bldgs.Delete(id)
	delete(g.fuel, id)
	delete(g.banked, id)
	if b.Kind == construction.DryingRack {
		_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		g.emptyRack(id, spot[0], spot[1])
	}
	kept := g.bldgIDs[:0]
	for _, bid := range g.bldgIDs {
		if bid != id {
			kept = append(kept, bid)
		}
	}
	g.bldgIDs = kept
	if construction.DefOf(b.Kind).BlocksMove {
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			g.grid.SetWalkable(c[0], c[1], g.terr.At(c[0], c[1]).Walkable())
		}
	}
	delete(g.bills, id)
	g.markRoomsDirty()
	g.structVer++ // SIM-029: link/autotile caches
	for _, c := range []uint64{b.Claimant, b.Claimant2} {
		if c != 0 {
			delete(g.sleptAt, entity.EntityID(c))
		}
	}
	if r := construction.Refund(b.Kind); r > 0 {
		g.dropStack(refundStuff(b), r, b.X, b.Y)
	}
	if n := componentRefund(b.Kind); n > 0 {
		g.dropStack(item.Components, n, b.X, b.Y) // half the components, rounded down
	}
}
