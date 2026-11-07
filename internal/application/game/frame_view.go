package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/framestatus"
	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Frame inspector read model and blueprint cancelling (UI-015).

// frameStock is the free stock of one material on the map: every ground
// stack that is not forbidden (inStock), and the part of it someone has
// reserved.
type frameStock struct{ inStock, reserved int }

// frameMemo is the view cache's last frame list (PERF-002): rebuilt into
// scratch each read model and handed out as the old slice while it reads
// the same. views (and the Haulers it points at) is never written once
// handed out.
type frameMemo struct {
	views, scratch []FrameView
	ok             bool
}

// frameViews is World().Frames, in frameIDs order. With the view cache on
// it is shared and read only (frameMemo).
func (g *Game) frameViews() []FrameView {
	if !g.views.on {
		out := g.appendFrameViews(make([]FrameView, 0, len(g.frameIDs)))
		g.fillFrameViews(out)
		return out
	}
	m := &g.views.frames
	m.scratch = g.appendFrameViews(m.scratch[:0])
	g.fillFrameViews(m.scratch)
	same := m.ok && len(m.scratch) == len(m.views)
	for i := range m.scratch {
		if i >= len(m.views) {
			break
		}
		old, fv := &m.views[i], &m.scratch[i]
		if slices.Equal(old.Haulers, fv.Haulers) && (old.Haulers == nil) == (fv.Haulers == nil) {
			fv.Haulers = old.Haulers // the old slice: nothing writes either
		}
		if same && !equalFrameView(fv, old) {
			same = false
		}
	}
	if same {
		return m.views
	}
	m.views = make([]FrameView, len(m.scratch))
	copy(m.views, m.scratch)
	m.ok = true
	return m.views
}

// appendFrameViews appends every frame's base view (the inspector fields
// follow in fillFrameViews) to out.
func (g *Game) appendFrameViews(out []FrameView) []FrameView {
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if !ok {
			continue
		}
		out = append(out, FrameView{
			ID: id, Kind: fr.Kind.String(), X: fr.X, Y: fr.Y, Wood: fr.Wood, Need: fr.Need, WorkLeft: fr.WorkLeft,
			Facing: fr.Facing.String(), Floor: construction.DefOf(fr.Kind).Layer == construction.LayerFloor,
			Stuff: fr.Stuff.String(),
		})
		fv := &out[len(out)-1]
		fv.W, fv.H, _, _, _ = g.footprintOf(fr.Kind, fr.Facing, fr.X, fr.Y)
		fv.AX.Colour = construction.TintName(fr.Kind, fr.Tint) // AX-3: a carpet floor's colour
		if fr.Reinstall != 0 {
			fv.Moving, fv.MovingFrom = true, entity.EntityID(fr.Reinstall)
		}
	}
	return out
}

// equalFrameView reports two frame views that read the same (Haulers
// element by element). TestFrameViewEqual proves every field is compared.
func equalFrameView(a, b *FrameView) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.X == b.X && a.Y == b.Y && a.Wood == b.Wood &&
		a.Need == b.Need && a.WorkLeft == b.WorkLeft && a.Facing == b.Facing && a.Floor == b.Floor &&
		a.Stuff == b.Stuff && a.W == b.W && a.H == b.H && a.Moving == b.Moving && a.MovingFrom == b.MovingFrom &&
		a.WorkTotal == b.WorkTotal && a.Incoming == b.Incoming && a.InStock == b.InStock &&
		a.Reserved == b.Reserved && a.Builder == b.Builder && slices.Equal(a.Haulers, b.Haulers) &&
		a.Status == b.Status && a.Comp == b.Comp && a.CompNeed == b.CompNeed && a.CompIncoming == b.CompIncoming && a.AX == b.AX
}

// fillFrameViews completes the inspector fields of the frames World()
// just listed. It makes one pass over the items (only the frames'
// materials are counted) and one over the colonists' jobs.
func (g *Game) fillFrameViews(views []FrameView) {
	if len(views) == 0 {
		return
	}
	stock := map[item.Kind]*frameStock{}
	for i := range views {
		fr, _ := g.frames.Get(views[i].ID)
		if stock[fr.Stuff] == nil {
			stock[fr.Stuff] = &frameStock{}
		}
	}
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || st.Carrier != 0 || st.Qty <= 0 || st.Forbidden {
			continue
		}
		s := stock[st.Kind]
		if s == nil {
			continue
		}
		s.inStock += st.Qty
		if _, taken := g.reserve.EntityOwner(iid); taken {
			s.reserved += st.Qty
		}
	}
	builders := map[entity.EntityID]string{}
	haulers := map[entity.EntityID][]string{}
	for _, pid := range g.pawnIDs {
		j, ok := g.jobs.Get(pid)
		if !ok || !j.Active() {
			continue
		}
		switch j.Kind {
		case job.Construct:
			if _, isFrame := g.frames.Get(j.Target); isFrame {
				if _, dup := builders[j.Target]; !dup {
					builders[j.Target] = g.name(pid)
				}
			}
		case job.Haul:
			if shares := g.haul.deliveries[pid]; len(shares) > 0 {
				for _, s := range shares {
					fid, _ := haul.SlotFrame(s.Frame) // a components share names the frame too
					haulers[fid] = append(haulers[fid], g.haulerLine(pid, s.Qty, s.Frame))
				}
			} else if _, isFrame := g.frames.Get(j.Dest); isFrame && j.Dest != 0 {
				haulers[j.Dest] = append(haulers[j.Dest], g.haulerLine(pid, g.haulQty(j), j.Dest))
			}
		}
	}
	for i := range views {
		fv := &views[i]
		fr, _ := g.frames.Get(fv.ID)
		fv.WorkTotal = frameWork(fr)
		fv.Incoming = g.haul.incoming[fv.ID]
		fv.Comp, fv.CompNeed, fv.CompIncoming = fr.Comp, fr.CompNeed, g.haul.incoming[fv.ID|haul.CompSlot]
		fv.AX.Extra, fv.AX.ExtraGot, fv.AX.Need = haul.ExtraView(fr) // AX-4: secondary materials
		if s := stock[fr.Stuff]; s != nil {
			fv.InStock, fv.Reserved = s.inStock, s.reserved
		}
		fv.Builder = builders[fv.ID]
		fv.Haulers = haulers[fv.ID]
		fv.Status = framestatus.Of(fr, fv.Incoming, fv.InStock-fv.Reserved, fv.CompIncoming, fv.Builder)
		if fr.Swaps != 0 && g.bldgs.Has(fr.Swaps) {
			fv.Status = fmt.Sprintf(framestatus.Changing, fr.Stuff.String())
		}
		if bid := g.replacedStanding(fr); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			fv.Status = "Waiting for the " + construction.DefOf(b.Kind).Name + " to be removed"
		}
	}
}

// frameWork is the total construct work of frame fr: a reinstall takes
// the work it was given when the move was planned (move.go).
func frameWork(fr construction.Frame) int {
	if fr.Reinstall != 0 {
		return max(minReinstallWork, construction.WorkFor(fr.Kind, fr.Stuff)/2)
	}
	return construction.WorkFor(fr.Kind, fr.Stuff)
}

// haulQty is what a single (non-route) haul job brings: the carried
// stack once picked up, else the planned count.
func (g *Game) haulQty(j job.Record) int {
	if st, ok := g.stacks.Get(j.Carry); ok && j.Carry != 0 {
		return st.Qty
	}
	return j.Count
}

// haulerLine names a hauler's share: key is a frame id, or its
// components slot (haul_route.go).
func (g *Game) haulerLine(pid entity.EntityID, qty int, key entity.EntityID) string {
	fid, _ := haul.SlotFrame(key)
	fr, _ := g.frames.Get(fid)
	sl, _ := haul.SlotOf(fid, fr, key) // the material, the components or a secondary material (AX-4)
	return fmt.Sprintf(framestatus.HaulerLine, g.name(pid), qty, sl.Kind.String())
}

// StockOf is the free stock of material kind on the map: ground stacks
// that are not forbidden (Architect cards: "In stock: 2"). Unknown kinds
// count 0.
func (g *Game) StockOf(kind string) int {
	k, ok := item.ParseKind(kind)
	if !ok {
		return 0
	}
	n := 0
	for _, iid := range g.itemIDs {
		if st, ok := g.stacks.Get(iid); ok && st.Kind == k && st.Carrier == 0 && !st.Forbidden {
			n += st.Qty
		}
	}
	return n
}

// cancelFrame removes blueprint/frame id; its delivered materials go back on
// the map.
func (g *Game) cancelFrame(id entity.EntityID) {
	fr, ok := g.frames.Get(id)
	if !ok {
		return
	}
	g.unmarkReplaced(fr)
	g.frames.Delete(id)
	kept := g.frameIDs[:0]
	for _, fid := range g.frameIDs {
		if fid != id {
			kept = append(kept, fid)
		}
	}
	g.frameIDs = kept
	g.refundFrame(id, fr)
	g.reorientFootprint(fr.Kind, fr.Facing, fr.X, fr.Y) // a cancelled wall no longer lines doors up
}

// refundFrame settles a frame that was just deleted (cancelled): the
// materials delivered to it drop as a stack on its tile, its reservation
// is freed and every hauler's share for it is given back.
func (g *Game) refundFrame(id entity.EntityID, fr construction.Frame) {
	if owner, taken := g.reserve.EntityOwner(id); taken {
		g.reserve.ReleaseEntity(owner, id)
	}
	if len(g.haul.deliveries) > 0 {
		for _, pid := range g.pawnIDs {
			for _, key := range haul.Keys(id) { // AX-4: the secondary slots too
				g.consumeShare(pid, key)
			}
		}
	}
	for _, key := range haul.Keys(id) {
		delete(g.haul.incoming, key)
	}
	if fr.Reinstall == 0 && fr.Wood > 0 {
		g.dropStack(fr.Stuff, fr.Wood, fr.X, fr.Y)
	}
	if fr.Reinstall == 0 && fr.Comp > 0 {
		g.dropStack(item.Components, fr.Comp, fr.X, fr.Y)
	}
	for i, c := range construction.DefOf(fr.Kind).Extra {
		if fr.ExtraGot[i] > 0 {
			g.dropStack(c.Item, fr.ExtraGot[i], fr.X, fr.Y)
		}
	}
}
