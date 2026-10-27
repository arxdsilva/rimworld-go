package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
)

// bldgMemo is the view cache's last building list (PERF-002). Each read
// model rebuilds the list into scratch and compares it with the last one:
// an equal list hands out the old slice again (the renderer's identity
// memos hold), a changed one a fresh copy. A building whose bills or move
// target read the same keeps its old Bills slice and MovingTo pointer.
// views (and every Bills and MovingTo it points at) is never written once
// handed out; scratch, bills and moves are memo-private.
type bldgMemo struct {
	views, scratch []BuildingView
	bills          []BillView    // this build's bills, all benches back to back
	moves          []reinstallTo // this build's planned moves
	ok             bool
}

// reinstallTo is a planned move: building bid reinstalls at at.
type reinstallTo struct {
	bid entity.EntityID
	at  [2]int
}

// buildingViews is World().Buildings, in bldgIDs order. With the view
// cache on it is shared and read only (bldgMemo).
func (g *Game) buildingViews() []BuildingView {
	if !g.views.on {
		moves := g.appendReinstalls(nil)
		out := make([]BuildingView, 0, len(g.bldgIDs))
		for _, id := range g.bldgIDs {
			b, ok := g.bldgs.Get(id)
			if !ok {
				continue
			}
			out = append(out, BuildingView{})
			bv := &out[len(out)-1]
			g.fillBuildingView(bv, id, b)
			bv.Bills = g.appendBillViews(nil, id)
			if at, ok := reinstallOf(moves, id); ok {
				bv.MovingTo = newCell(at)
			}
		}
		return out
	}
	m := &g.views.bldgs
	m.moves = g.appendReinstalls(m.moves[:0])
	m.scratch, m.bills = m.scratch[:0], m.bills[:0]
	same := m.ok
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		i := len(m.scratch)
		m.scratch = append(m.scratch, BuildingView{})
		bv := &m.scratch[i]
		g.fillBuildingView(bv, id, b)
		var old *BuildingView // the last view of this building, when in place
		if i < len(m.views) && m.views[i].ID == id {
			old = &m.views[i]
		}
		start := len(m.bills)
		m.bills = g.appendBillViews(m.bills, id)
		switch nb := m.bills[start:]; {
		case len(nb) == 0:
		case old != nil && slices.Equal(old.Bills, nb):
			bv.Bills = old.Bills
		default:
			bv.Bills = slices.Clone(nb)
		}
		if at, ok := reinstallOf(m.moves, id); ok {
			if old != nil && old.MovingTo != nil && *old.MovingTo == at {
				bv.MovingTo = old.MovingTo
			} else {
				bv.MovingTo = newCell(at)
			}
		}
		if same && (old == nil || !equalBuildingView(bv, old)) {
			same = false
		}
	}
	if same && len(m.scratch) == len(m.views) {
		return m.views
	}
	m.views = make([]BuildingView, len(m.scratch))
	copy(m.views, m.scratch)
	m.ok = true
	return m.views
}

// appendReinstalls appends every planned move (reinstall frames, in
// frameIDs order) to out.
func (g *Game) appendReinstalls(out []reinstallTo) []reinstallTo {
	for _, fid := range g.frameIDs {
		if fr, ok := g.frames.Get(fid); ok && fr.Reinstall != 0 {
			out = append(out, reinstallTo{bid: entity.EntityID(fr.Reinstall), at: [2]int{fr.X, fr.Y}})
		}
	}
	return out
}

// newCell is a fresh copy of c on the heap (taking the address of a
// local would move it to the heap on every call, moving or not).
func newCell(c [2]int) *[2]int {
	p := new([2]int)
	*p = c
	return p
}

// footShape is a footprint's size and work spot relative to its anchor
// (construction.Footprint is the same shape wherever it stands).
type footShape struct {
	w, h, sx, sy int
	has          bool
}

// footprintOf is footprintView without building the cell list each time:
// the shape per kind and facing is kept in the view cache.
func (g *Game) footprintOf(k construction.Kind, f construction.Facing, x, y int) (w, h, sx, sy int, has bool) {
	key := [2]int{int(k), int(f)}
	s, ok := g.views.feet[key]
	if !ok {
		s.w, s.h, s.sx, s.sy, s.has = footprintView(k, f, 0, 0)
		if g.views.feet == nil {
			g.views.feet = map[[2]int]footShape{}
		}
		g.views.feet[key] = s
	}
	if s.has {
		return s.w, s.h, x + s.sx, y + s.sy, true
	}
	return s.w, s.h, s.sx, s.sy, false
}

// reinstallOf is where building bid's planned move goes (move.go
// movingTo: the first reinstall frame for it).
func reinstallOf(moves []reinstallTo, bid entity.EntityID) ([2]int, bool) {
	for _, mv := range moves {
		if mv.bid == bid {
			return mv.at, true
		}
	}
	return [2]int{}, false
}

// fillBuildingView fills every BuildingView field but Bills and MovingTo
// (buildingViews sets those).
func (g *Game) fillBuildingView(bv *BuildingView, id entity.EntityID, b construction.Building) {
	*bv = BuildingView{
		ID: id, Kind: b.Kind.String(), X: b.X, Y: b.Y, Claimant: entity.EntityID(b.Claimant), Claimant2: entity.EntityID(b.Claimant2),
		Facing: b.Facing.String(), Floor: construction.DefOf(b.Kind).Layer == construction.LayerFloor,
		Deconstruct: b.Deconstruct, Stuff: b.Stuff.String(),
		Bench: construction.DefOf(b.Kind).Bench,
	}
	bv.HPPct = b.HP() * 100 / max(1, construction.MaxHP(b.Kind, b.Stuff))
	if b.Kind == construction.Carpet {
		bv.Catch = filth.Catch(b.Stuff) / 10
	}
	if pct, ok := g.fuelPct(id, b); ok { // campfire, torch lamp, wood-fired generator (fuel.go)
		bv.Fuel, bv.FuelPct, bv.Burning = true, pct, g.burning(id, b)
	}
	if b.Kind == construction.FeedingTrough {
		bv.TroughFill, bv.TroughCap = g.troughFill(id), construction.TroughCapacity
		bv.TroughFilter = troughFilterNames[min(int(g.troughs[id].Filter), len(troughFilterNames)-1)]
	}
	if rb := g.racks[id]; rb.Qty > 0 {
		bv.DryKind, bv.DryQty, bv.DryPct = rb.Kind.String(), rb.Qty, rb.percent()
	}
	bv.W, bv.H, bv.SpotX, bv.SpotY, bv.HasSpot = g.footprintOf(b.Kind, b.Facing, b.X, b.Y)
	g.fillGraveView(bv, id, b)  // burial.go
	g.fillSwapView(bv, id)      // placement.go: pending material change (PLAY-011)
	bv.Ruin = b.Ruin            // MAPGEN-2: an abandoned piece
	bv.Medical = b.Medical      // PLAY-020: kept for the sick and injured
	g.fillCasketView(bv, id, b) // casket.go (MAPGEN-3)
	// PLAY-021: the light the renderer draws (light.go), then power
	// (power_view.go)
	bv.Glow, bv.GlowRadius = g.glowOf(id, b)
	g.fillPowerBuilding(bv, id, b)
	g.fillTurretView(bv, id, b, uint64(g.clock.Now())) // turret.go (PLAY-024)
}

// equalBuildingView reports two views that read the same: every field
// equal, Bills element by element and MovingTo by the cell it points at.
// TestBuildingViewEqual proves every field is compared (a field added to
// BuildingView must be added here).
func equalBuildingView(a, b *BuildingView) bool {
	if (a.MovingTo == nil) != (b.MovingTo == nil) || (a.MovingTo != nil && *a.MovingTo != *b.MovingTo) {
		return false
	}
	return a.ID == b.ID && a.Kind == b.Kind && a.X == b.X && a.Y == b.Y &&
		a.Claimant == b.Claimant && a.Claimant2 == b.Claimant2 && a.Facing == b.Facing &&
		a.Floor == b.Floor && a.Deconstruct == b.Deconstruct && a.Stuff == b.Stuff &&
		a.SwapTo == b.SwapTo && a.SwapPct == b.SwapPct && a.Ruin == b.Ruin && a.Medical == b.Medical &&
		a.W == b.W && a.H == b.H && a.HasSpot == b.HasSpot && a.SpotX == b.SpotX && a.SpotY == b.SpotY &&
		a.Bench == b.Bench && slices.Equal(a.Bills, b.Bills) &&
		a.DryKind == b.DryKind && a.DryQty == b.DryQty && a.DryPct == b.DryPct &&
		a.Fuel == b.Fuel && a.FuelPct == b.FuelPct && a.Burning == b.Burning &&
		a.HPPct == b.HPPct && a.Catch == b.Catch &&
		a.Occupant == b.Occupant && a.Epitaph == b.Epitaph && a.Headstone == b.Headstone &&
		a.TroughFill == b.TroughFill && a.TroughCap == b.TroughCap && a.TroughFilter == b.TroughFilter &&
		a.Sealed == b.Sealed && a.OpenReason == b.OpenReason &&
		a.PowerRole == b.PowerRole && a.PowerNet == b.PowerNet && a.Powered == b.Powered &&
		a.PowerOn == b.PowerOn && a.PowerWantOn == b.PowerWantOn &&
		a.PowerUse == b.PowerUse && a.PowerOut == b.PowerOut &&
		a.StoredWh == b.StoredWh && a.CapWh == b.CapWh && a.NoPowerReason == b.NoPowerReason &&
		a.LinkX == b.LinkX && a.LinkY == b.LinkY && a.LinkFromX == b.LinkFromX && a.LinkFromY == b.LinkFromY &&
		a.HasLink == b.HasLink && a.Glow == b.Glow && a.GlowRadius == b.GlowRadius &&
		a.ThermoTarget == b.ThermoTarget && a.ThermoActive == b.ThermoActive && a.PowerMax == b.PowerMax &&
		a.AimDir == b.AimDir && a.ShotAge == b.ShotAge
}
