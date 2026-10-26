package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
)

// Bridges coming down (MAPGEN-3 §3.2). Deconstruction is refused while
// anything stands or is planned on the deck; a destroyed bridge takes
// what it held up with it (no refund), and loose things on deep water
// sink. Either way the creatures on it step to the nearest dry footing
// and the cells' walkability is recomputed.

// relocateR is how far a creature on a vanished deck looks for footing.
const relocateR = 8

// supportedLayers are the layers a deck holds up (a conduit laid across a
// bridge, PLAY-021, included).
var supportedLayers = [...]construction.Layer{construction.LayerObject, construction.LayerFloor, construction.LayerConduit}

// bridgeInUse reports a foundation b with a building or a planned piece
// on any of its cells (false for every other kind).
func (g *Game) bridgeInUse(b construction.Building) bool {
	if !construction.DefOf(b.Kind).Foundation {
		return false
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		for _, l := range supportedLayers {
			if g.layerBldgAt(c[0], c[1], l) != 0 || g.layerFrameAt(c[0], c[1], l) != 0 {
				return true
			}
		}
	}
	return false
}

// refuseBridge posts why a bridge can't be deconstructed yet.
func (g *Game) refuseBridge(b construction.Building) {
	g.message(b.X, b.Y, "The bridge can't come down: %s.", ReasonBridgeInUse)
}

// needsDeck reports a piece of kind k in stuff that the bare ground of
// (x, y) can't hold up (it stood on the deck).
func (g *Game) needsDeck(k construction.Kind, stuff item.Kind, x, y int) bool {
	ground := terrain.GroundSupport(g.terr.At(x, y))
	return construction.Requires(k, stuff) > ground ||
		construction.DefOf(k).Dug && ground < terrain.SupportHeavy
}

// collapseBridge runs before a destroyed bridge b is removed: every
// building it held up breaks (no refund), every piece planned on it that
// needs the deck is cancelled, and loose things on deep water sink (one
// message for the lot).
func (g *Game) collapseBridge(b construction.Building) {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		for _, l := range supportedLayers {
			if fid := g.layerFrameAt(c[0], c[1], l); fid != 0 {
				if fr, ok := g.frames.Get(fid); ok && g.needsDeck(fr.Kind, fr.Stuff, c[0], c[1]) {
					g.cancelFrame(fid) // frame_view.go: delivered materials drop on the deck
				}
			}
			if bid := g.layerBldgAt(c[0], c[1], l); bid != 0 {
				if ob, ok := g.bldgs.Get(bid); ok && g.needsDeck(ob.Kind, ob.Stuff, c[0], c[1]) {
					g.message(c[0], c[1], "The %s fell into the water.", construction.DefOf(ob.Kind).Name)
					g.removeBuildingAs(bid, false)
				}
			}
		}
	}
	first, n := "", 0
	for _, c := range cells {
		if g.terr.At(c[0], c[1]) != terrain.DeepWater {
			continue // shallow water, marsh and mud keep what lies there
		}
		for _, sid := range g.looseAt(c[0], c[1]) {
			st, _ := g.stacks.Get(sid)
			if n == 0 {
				first = st.Kind.String()
			}
			n++
			g.deleteItem(sid)
		}
	}
	switch {
	case n == 1:
		g.message(b.X, b.Y, "The %s sank.", first)
	case n > 1:
		g.message(b.X, b.Y, "The %s and %d more sank.", first, n-1)
	}
}

// looseAt lists the stacks lying (not carried) on (x, y), in list order.
func (g *Game) looseAt(x, y int) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.itemIDs {
		if st := g.stacks.Ref(id); st != nil && st.Carrier == 0 && st.X == x && st.Y == y {
			out = append(out, id)
		}
	}
	return out
}

// foundationGone runs after foundation b was removed (deconstructed or
// destroyed): its cells' walkability is recomputed, creatures left on
// water they can't stand in step to the nearest dry footing, and loose
// things still on deep water float ashore (or sink when there is none).
func (g *Game) foundationGone(b construction.Building) {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		g.grid.SetWalkable(c[0], c[1], g.cellWalkable(c[0], c[1]))
	}
	for _, ids := range [][]entity.EntityID{g.pawnIDs, g.animalIDs} {
		for _, id := range ids {
			pos, ok := g.positions.Get(id)
			if !ok || !covers(cells, pos.X, pos.Y) || g.grid.Walkable(pos.X, pos.Y) {
				continue
			}
			if x, y, ok := g.walkableNear(pos.X, pos.Y, relocateR); ok {
				g.positions.Set(id, Position{X: x, Y: y})
			}
		}
	}
	for _, c := range cells {
		if g.terr.At(c[0], c[1]) != terrain.DeepWater {
			continue
		}
		for _, sid := range g.looseAt(c[0], c[1]) {
			st, _ := g.stacks.Get(sid)
			if x, y, ok := g.spillTile(st.Kind, c[0], c[1]); ok {
				st.X, st.Y = x, y
				g.mergeOrSet(sid, st)
				continue
			}
			g.message(c[0], c[1], "The %s sank.", st.Kind)
			g.deleteItem(sid)
		}
	}
}

// applyBridges lays a generated map's sea mask and its old road bridges
// (finished wooden decks; the water underneath stays).
func (g *Game) applyBridges(res mapgen.Result) {
	g.setOcean(res.Ocean)
	for _, p := range res.Bridges {
		g.placeGenerated(construction.Building{Kind: construction.Bridge, Stuff: item.Wood, X: p.X, Y: p.Y})
	}
}
