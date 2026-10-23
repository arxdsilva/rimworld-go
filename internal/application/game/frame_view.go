package game

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// Frame inspector read model and blueprint cancelling (UI-015).

// Frame status lines (FrameView.Status), first match wins.
const (
	frameMoving     = "Moving %s here: needs no materials"
	frameNoStock    = "No %s in stock"
	frameOnTheWay   = "Materials on the way"
	frameCompsWay   = "Components on the way"
	frameNeedComps  = "Needs %d components"
	frameNoHauler   = "Waiting for a hauler"
	frameBuiltBy    = "Being built by %s"
	frameNoBuilder  = "Waiting for a builder"
	frameCraftHint  = ": %s one at a %s"
	frameHaulerLine = "%s (%d %s)"
)

// frameStock is the free stock of one material on the map: every ground
// stack that is not forbidden (inStock), and the part of it someone has
// reserved.
type frameStock struct{ inStock, reserved int }

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
					fid, _ := slotFrame(s.Frame) // a components share names the frame too
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
		fv.Comp, fv.CompNeed, fv.CompIncoming = fr.Comp, fr.CompNeed, g.haul.incoming[fv.ID|compSlot]
		if s := stock[fr.Stuff]; s != nil {
			fv.InStock, fv.Reserved = s.inStock, s.reserved
		}
		fv.Builder = builders[fv.ID]
		fv.Haulers = haulers[fv.ID]
		fv.Status = frameStatus(fr, *fv)
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
	fid, comp := slotFrame(key)
	fr, _ := g.frames.Get(fid)
	if comp {
		return fmt.Sprintf(frameHaulerLine, g.name(pid), qty, item.Components.String())
	}
	return fmt.Sprintf(frameHaulerLine, g.name(pid), qty, fr.Stuff.String())
}

// frameStatus is the frame inspector's one-line status (plan Part B).
func frameStatus(fr construction.Frame, fv FrameView) string {
	switch {
	case fr.Reinstall != 0:
		return fmt.Sprintf(frameMoving, construction.DefOf(fr.Kind).Name)
	case !fr.Ready() && fr.Wood+fv.Incoming < fr.Need && fv.InStock-fv.Reserved <= 0:
		return fmt.Sprintf(frameNoStock, fr.Stuff.String()) + craftHint(fr.Stuff)
	case fr.Wood >= fr.Need && fr.Comp < fr.CompNeed && fv.CompIncoming > 0:
		return frameCompsWay
	case fr.Wood >= fr.Need && fr.Comp < fr.CompNeed:
		return fmt.Sprintf(frameNeedComps, fr.CompNeed-fr.Comp)
	case !fr.Ready() && fv.Incoming > 0:
		return frameOnTheWay
	case !fr.Ready():
		return frameNoHauler
	case fv.Builder != "":
		return fmt.Sprintf(frameBuiltBy, fv.Builder)
	}
	return frameNoBuilder
}

// craftHint tells where a crafted material is made (": sew one at a
// tailoring bench"), from the first offered recipe whose output it is;
// "" for gathered materials.
func craftHint(k item.Kind) string {
	for _, d := range recipe.All() {
		if d.Output != k || d.Legacy || d.Name == "" || d.OutQty <= 0 {
			continue
		}
		verb, _, _ := strings.Cut(strings.ToLower(d.Label), " ")
		return fmt.Sprintf(frameCraftHint, verb, construction.DefOf(d.Bench).Name)
	}
	return ""
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
	g.frames.Delete(id)
	kept := g.frameIDs[:0]
	for _, fid := range g.frameIDs {
		if fid != id {
			kept = append(kept, fid)
		}
	}
	g.frameIDs = kept
	g.refundFrame(id, fr)
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
			if len(g.haul.deliveries[pid]) > 0 {
				g.consumeShare(pid, id)
				g.consumeShare(pid, id|compSlot)
			}
		}
	}
	delete(g.haul.incoming, id)
	delete(g.haul.incoming, id|compSlot)
	if fr.Reinstall == 0 && fr.Wood > 0 {
		g.dropStack(fr.Stuff, fr.Wood, fr.X, fr.Y)
	}
	if fr.Reinstall == 0 && fr.Comp > 0 {
		g.dropStack(item.Components, fr.Comp, fr.X, fr.Y)
	}
}
