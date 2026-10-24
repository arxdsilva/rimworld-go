package game

import (
	"strings"

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
	// ReasonSame: the same piece in the same material is already here
	// (a neutral refusal: the Architect skips it silently, PLAY-011).
	ReasonSame = "already that material"
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
	return g.canPlaceStuffAt(k, s, parseFacingName(facing), x, y)
}

func parseFacingName(f string) construction.Facing {
	for v := construction.South; v <= construction.West; v++ {
		if v.String() == f {
			return v
		}
	}
	return construction.South
}

// noStuff: no material given (CanPlace, BuildNow, arrivals): the same
// kind on a built piece is then never a swap, only occupied.
const noStuff = item.Kind(255)

// canPlaceAt is canPlaceStuffAt without a material.
func (g *Game) canPlaceAt(k construction.Kind, f construction.Facing, x, y int) (bool, string) {
	return g.canPlaceStuffAt(k, noStuff, f, x, y)
}

// canPlaceStuffAt is the full footprint rule for kind k in stuff facing f
// at (x, y). The same piece in another material is an in-place swap: its
// cells skip the occupied and blocked checks; in the same material it is
// the neutral ReasonSame (PLAY-011).
func (g *Game) canPlaceStuffAt(k construction.Kind, stuff item.Kind, f construction.Facing, x, y int) (bool, string) {
	swap, same := g.swapCheck(k, stuff, f, x, y)
	if same {
		return false, ReasonSame
	}
	cells, spot, hasSpot := construction.Footprint(k, f, x, y)
	for _, c := range cells {
		if ok, r := g.canPlaceCell(k, c[0], c[1], swap != 0); !ok {
			return false, r
		}
	}
	if swap != 0 {
		return true, "" // the piece already stands there, spot and all
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

// canPlace is the one-cell rule for a new piece of kind k at (x, y).
func (g *Game) canPlace(k construction.Kind, x, y int) (bool, string) {
	return g.canPlaceCell(k, x, y, false)
}

// canPlaceCell is the one-cell rule; swap: the cell holds the piece an
// in-place swap converts, so it is neither occupied nor blocked.
func (g *Game) canPlaceCell(k construction.Kind, x, y int, swap bool) (bool, string) {
	if !g.grid.InBounds(x, y) {
		return false, ReasonOutside
	}
	if g.scenarioForbids(k) { // setup.go: the scenario disallows it (NG-1)
		return false, ReasonScenario
	}
	layer := construction.DefOf(k).Layer
	rep := g.replaceTarget(k, x, y) // a door or gate may stand where a wall or fence does
	if swap {
		rep = g.layerBldgAt(x, y, layer) // the swapped piece stands here (PLAY-011)
	}
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

// AutoFacingAt is the facing a piece of kind placed at (x, y) with the
// player's facing would get (doors and gates face along their walls and
// fences, PLAY-011); read-only, for the Architect ghost.
func (g *Game) AutoFacingAt(kind, facing string, x, y int) string {
	k, ok := construction.Parse(kind)
	if !ok || !autoOriented(k) {
		return facing
	}
	return g.autoFacing(k, x, y, parseFacingName(facing)).String()
}

// autoOriented: doors and fence gates face along their walls or fences.
func autoOriented(k construction.Kind) bool {
	return k == construction.Door || k == construction.FenceGate
}

// orientPiece: a piece a door or gate lines up with (wall, door, fence,
// gate).
func orientPiece(k construction.Kind) bool {
	switch k {
	case construction.Wall, construction.Door, construction.Fence, construction.FenceGate:
		return true
	}
	return false
}

// solidAt reports a neighbour a door or gate lines up with: a wall, door,
// fence or gate on the object layer, built or planned, or natural rock.
func (g *Game) solidAt(x, y int) bool {
	if !g.grid.InBounds(x, y) {
		return false
	}
	if terrain.IsRockWall(g.terr.At(x, y)) {
		return true
	}
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		if b, ok := g.bldgs.Get(bid); ok && orientPiece(b.Kind) {
			return true
		}
	}
	if fid := g.layerFrameAt(x, y, construction.LayerObject); fid != 0 {
		if fr, ok := g.frames.Get(fid); ok && orientPiece(fr.Kind) {
			return true
		}
	}
	return false
}

// autoFacing is the facing a door or gate of kind k at (x, y) takes: it
// spans the axis with more solid neighbours (east+west: south, art "ew";
// north+south: east, art "ns"). A tie keeps prefer, the current facing,
// so a piece never flips back and forth (hysteresis). Other kinds keep
// prefer. Neighbours are read in fixed N, E, S, W order; no RNG.
func (g *Game) autoFacing(k construction.Kind, x, y int, prefer construction.Facing) construction.Facing {
	if !autoOriented(k) {
		return prefer
	}
	h, v := 0, 0
	if g.solidAt(x, y-1) {
		v++
	}
	if g.solidAt(x+1, y) {
		h++
	}
	if g.solidAt(x, y+1) {
		v++
	}
	if g.solidAt(x-1, y) {
		h++
	}
	switch {
	case h > v:
		return construction.South
	case v > h:
		return construction.East
	}
	return prefer
}

// reorientAround re-evaluates the doors and gates (built and planned) on
// the four orthogonal neighbours of (x, y) after that cell changed; O(1).
func (g *Game) reorientAround(x, y int) {
	for _, d := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} { // N, E, S, W
		g.reorientAt(x+d[0], y+d[1])
	}
}

// reorientAt re-faces the door or gate building and frame on (x, y); a
// piece is written only when its facing changes.
func (g *Game) reorientAt(x, y int) {
	if !g.grid.InBounds(x, y) {
		return
	}
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		if b, ok := g.bldgs.Get(bid); ok && autoOriented(b.Kind) {
			if f := g.autoFacing(b.Kind, x, y, b.Facing); f != b.Facing {
				b.Facing = f
				g.bldgs.Set(bid, b)
				g.structVer++ // SIM-029: link/autotile caches
			}
		}
	}
	if fid := g.layerFrameAt(x, y, construction.LayerObject); fid != 0 {
		if fr, ok := g.frames.Get(fid); ok && autoOriented(fr.Kind) {
			if f := g.autoFacing(fr.Kind, x, y, fr.Facing); f != fr.Facing {
				fr.Facing = f
				g.frames.Set(fid, fr)
				g.structVer++
			}
		}
	}
}

// reorientFootprint calls reorientAround for every cell of a wall, door,
// fence or gate of kind k that appeared or went away (other kinds change
// nothing a door lines up with).
func (g *Game) reorientFootprint(k construction.Kind, f construction.Facing, x, y int) {
	if !orientPiece(k) {
		return
	}
	cells, _, _ := construction.Footprint(k, f, x, y)
	for _, c := range cells {
		g.reorientAround(c[0], c[1])
	}
}

// orientDoors gives every door (built or planned) of an older save the
// facing of its neighbours, once on load; a tie keeps the stored south.
// Gates keep their player-chosen facing.
func (g *Game) orientDoors() {
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && b.Kind == construction.Door {
			if f := g.autoFacing(b.Kind, b.X, b.Y, b.Facing); f != b.Facing {
				b.Facing = f
				g.bldgs.Set(id, b)
			}
		}
	}
	for _, id := range g.frameIDs {
		if fr, ok := g.frames.Get(id); ok && fr.Kind == construction.Door {
			if f := g.autoFacing(fr.Kind, fr.X, fr.Y, fr.Facing); f != fr.Facing {
				fr.Facing = f
				g.frames.Set(id, fr)
			}
		}
	}
}

// ReplacesAt names what a piece of kind in stuff would replace at (x, y):
// a door or gate over a wall or fence gives its name ("wall"); the same
// piece in another material gives the swap ("wood wall → steel",
// PLAY-011); "" otherwise. Read-only, for the Architect ghost.
func (g *Game) ReplacesAt(kind, stuff string, x, y int) string {
	k, ok := construction.Parse(kind)
	if !ok {
		return ""
	}
	if bid := g.replaceTarget(k, x, y); bid != 0 {
		b, _ := g.bldgs.Get(bid)
		return construction.DefOf(b.Kind).Name
	}
	s, ok := item.ParseKind(stuff)
	if !ok || !g.grid.InBounds(x, y) {
		return ""
	}
	b, ok := g.bldgs.Get(g.layerBldgAt(x, y, construction.DefOf(k).Layer))
	if !ok {
		return ""
	}
	if swap, _ := g.swapCheck(k, s, b.Facing, x, y); swap != 0 {
		return stuffWord(b.Stuff) + " " + construction.DefOf(k).Name + " → " + stuffWord(s)
	}
	return ""
}

// stuffWord is a material's short label: "granite" for granite blocks,
// "cloth" for a cloth carpet.
func stuffWord(s item.Kind) string {
	w := strings.TrimSuffix(s.String(), " carpet")
	return strings.TrimSuffix(w, " blocks")
}

// swapCheck: the same piece of kind k already stands at origin (x, y)
// with the same footprint (and facing, for a multi-tile kind; doors and
// gates face automatically), alone on its cells and with no frame
// planned there. swap is that building when stuff is another valid
// material; same reports it is already of stuff. A missing material
// (noStuff) is neither.
func (g *Game) swapCheck(k construction.Kind, stuff item.Kind, f construction.Facing, x, y int) (swap entity.EntityID, same bool) {
	if stuff == noStuff || !k.Valid() || !g.grid.InBounds(x, y) {
		return 0, false
	}
	def := construction.DefOf(k)
	bid := g.layerBldgAt(x, y, def.Layer)
	b, ok := g.bldgs.Get(bid)
	if !ok || b.Kind != k || b.X != x || b.Y != y {
		return 0, false
	}
	if !def.Rotatable {
		f = construction.South
	}
	if multiTile(k) && b.Facing != f {
		return 0, false
	}
	cells, _, _ := construction.Footprint(k, b.Facing, x, y)
	for _, c := range cells {
		if g.layerBldgAt(c[0], c[1], def.Layer) != bid || g.layerFrameAt(c[0], c[1], def.Layer) != 0 {
			return 0, false
		}
	}
	if b.Stuff == stuff {
		return 0, true
	}
	if construction.StuffOK(k, stuff) {
		return bid, false
	}
	return 0, false
}

// multiTile: kind k covers more than one cell.
func multiTile(k construction.Kind) bool {
	s := construction.DefOf(k).Size
	return s[0] > 1 || s[1] > 1
}

// swapFrameFor is the pending swap frame that converts building bid (0:
// none); O(1) through the frame index at its origin.
func (g *Game) swapFrameFor(bid entity.EntityID) entity.EntityID {
	b, ok := g.bldgs.Get(bid)
	if !ok {
		return 0
	}
	fid := g.layerFrameAt(b.X, b.Y, construction.DefOf(b.Kind).Layer)
	if fr, ok := g.frames.Get(fid); ok && fr.Swaps == bid {
		return fid
	}
	return 0
}

// releaseSwap turns the swap frame of building bid into an ordinary
// blueprint, because the building is going away (deconstructed, smashed or
// moved): it will be built new, components and all.
func (g *Game) releaseSwap(bid entity.EntityID) {
	fid := g.swapFrameFor(bid)
	if fid == 0 {
		return
	}
	fr, _ := g.frames.Get(fid)
	fr.Swaps = 0
	fr.CompNeed = construction.ComponentCost(fr.Kind)
	g.frames.Set(fid, fr)
}

// completeSwap finishes swap frame id: the same building takes the new
// material at full hit points, keeping its facing, owners, bills and
// contents; the old material is refunded as a deconstruction would.
func (g *Game) completeSwap(id entity.EntityID, fr construction.Frame, b construction.Building) {
	g.frames.Delete(id)
	kept := g.frameIDs[:0]
	for _, fid := range g.frameIDs {
		if fid != id {
			kept = append(kept, fid)
		}
	}
	g.frameIDs = kept
	old := b
	b.Stuff, b.Damage = fr.Stuff, 0
	g.bldgs.Set(fr.Swaps, b)
	g.markRoomsDirty() // stone walls change beauty and impressiveness
	g.structVer++      // SIM-029: the tint changes
	r := construction.Refund(old.Kind)
	if r <= 0 {
		return
	}
	g.dropStack(refundStuff(old), r, old.X, old.Y)
	if !construction.DefOf(old.Kind).BlocksMove {
		return
	}
	cells, _, _ := construction.Footprint(old.Kind, old.Facing, old.X, old.Y)
	g.clearItemsFrom(old.X, old.Y, cells) // defense.go: not inside the wall
	if g.itemAt(old.X, old.Y) != 0 {
		g.message(old.X, old.Y, "No room to drop the %s from the old %s.", refundStuff(old), construction.DefOf(old.Kind).Name)
	}
}

// fillSwapView sets a building view's pending swap: the new material and
// the percent of its work done.
func (g *Game) fillSwapView(bv *BuildingView, id entity.EntityID) {
	fr, ok := g.frames.Get(g.swapFrameFor(id))
	if !ok {
		return
	}
	total := max(1, construction.WorkFor(fr.Kind, fr.Stuff))
	bv.SwapTo = fr.Stuff.String()
	bv.SwapPct = min(100, max(0, (total-fr.WorkLeft)*100/total))
}

// replacedStanding is the piece frame fr replaces while it still stands
// (0: none, the frame may be built).
func (g *Game) replacedStanding(fr construction.Frame) entity.EntityID {
	if fr.Replaces == 0 {
		return 0
	}
	if !g.bldgs.Has(fr.Replaces) {
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
	swap, same := g.swapCheck(k, s, construction.South, x, y)
	if same {
		return false, ReasonSame
	}
	return g.canPlaceCell(k, x, y, swap != 0)
}

func (g *Game) placeFrameStuff(kind construction.Kind, stuff item.Kind, f construction.Facing, x, y int) {
	if !kind.Valid() || !construction.StuffOK(kind, stuff) {
		return
	}
	if !construction.DefOf(kind).Rotatable && kind != construction.Door {
		f = construction.South // a door's facing is stored: it follows its walls (PLAY-011)
	}
	f = g.autoFacing(kind, x, y, f) // the player's facing only breaks a tie
	if ok, _ := g.canPlaceStuffAt(kind, stuff, f, x, y); !ok {
		return
	}
	swap, _ := g.swapCheck(kind, stuff, f, x, y)
	if b, ok := g.bldgs.Get(swap); ok {
		f = b.Facing // a swap keeps the piece as it stands
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
	if swap != 0 {
		fr.Swaps = swap // converted in place: nothing comes down (PLAY-011)
		fr.CompNeed = 0 // the components already in it are kept
	} else if cells, _, _ := construction.Footprint(kind, f, x, y); len(cells) > 0 {
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
	g.noteFrame(id)                    // indexes.go
	g.reorientFootprint(kind, f, x, y) // a planned wall already turns the doors beside it
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
	g.releaseSwap(id) // a pending material change becomes a new build
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
	g.reorientFootprint(b.Kind, b.Facing, b.X, b.Y)
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
