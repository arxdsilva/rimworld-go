package orders

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Uninstall (AX-1, wiki: Orders, Minified thing): the Uninstall order
// marks finished buildings (construction.Building.Uninstall); a builder
// walks over, works the building's deconstruct work and packs it into a
// crated building (item.MinifiedBuilding) that keeps its material,
// quality and wear. Nothing is refunded: the crate is the building. The
// wiki gives no work figure, so the work is the building's deconstruct
// work (original tuning).
type Uninstall struct {
	h Host
	// ids are the marked buildings, ascending; built from the buildings
	// on first use (derived, never saved: the mark is the building's).
	ids   []entity.EntityID
	built bool
}

// Packable reports a building the Uninstall order may mark: a movable,
// non-instant colony piece (not a ruin, a vault piece or a natural
// feature) that holds no one.
func Packable(b construction.Building) bool {
	d := construction.DefOf(b.Kind)
	return construction.Movable(b.Kind) && !b.Kind.Instant() && !d.Natural && !b.Ruin && !b.Ancient && !b.Hostile && b.Occupant == 0
}

// ReasonUnpackable is why a building can't be uninstalled.
const ReasonUnpackable = "can't be uninstalled"

// Mark marks (on) or unmarks every packable building whose origin lies
// in the rectangle; a building being deconstructed is left alone.
func (u *Uninstall) Mark(minX, minY, maxX, maxY int, on bool) {
	u.index()
	h := u.h
	for _, id := range h.BuildingIDs() {
		b, ok := h.Building(id)
		if !ok || !in(b.X, b.Y, minX, minY, maxX, maxY) || !h.Seen(b.X, b.Y) || b.Uninstall == on {
			continue
		}
		if on && (b.Deconstruct || !Packable(b) || h.Interred(id)) {
			continue
		}
		b.Uninstall = on
		b.WorkLeft = 0
		if on {
			b.WorkLeft = construction.DeconstructTicks(b.Kind)
		}
		h.SetBuilding(id, b)
		u.ids = setMark(u.ids, id, on)
	}
}

// index builds the marked list from the buildings once.
func (u *Uninstall) index() {
	if u.built {
		return
	}
	u.built = true
	for _, id := range u.h.BuildingIDs() {
		if b, ok := u.h.Building(id); ok && b.Uninstall {
			u.ids = setMark(u.ids, id, true)
		}
	}
}

// Assign starts builder id on the nearest marked building it may take.
func (u *Uninstall) Assign(id entity.EntityID) bool {
	u.index()
	if len(u.ids) == 0 {
		return false
	}
	h := u.h
	px, py := h.Pos(id)
	var best entity.EntityID
	bestD := 1 << 30
	keep := u.ids[:0]
	for _, bid := range u.ids {
		b, ok := h.Building(bid)
		if !ok || !b.Uninstall {
			continue // gone, or the mark came off: no longer indexed
		}
		keep = append(keep, bid)
		if !h.Free(id, bid, job.Uninstall) {
			continue
		}
		if d := dist(px, py, b.X, b.Y); d < bestD {
			best, bestD = bid, d
		}
	}
	u.ids = keep
	if best == 0 {
		return false
	}
	b, _ := h.Building(best)
	tx, ty, ok := h.Spot(id, b.X, b.Y)
	if !ok {
		return false
	}
	return h.StartJob(id, job.Record{Kind: job.Uninstall, Target: best, TX: tx, TY: ty, WorkLeft: b.WorkLeft})
}

// Do works an uninstall at the building, then crates it.
func (u *Uninstall) Do(id entity.EntityID, j job.Record) {
	h := u.h
	b, ok := h.Building(j.Target)
	if !ok || !b.Uninstall || b.Deconstruct {
		finish(h, id, j, job.Fail, job.TargetGone)
		return
	}
	w := h.Work(id, true)
	if w == 0 {
		h.SetJob(id, j)
		return
	}
	b.WorkLeft -= w
	j.WorkLeft = b.WorkLeft
	if b.WorkLeft > 0 {
		h.SetBuilding(j.Target, b)
		h.SetJob(id, j)
		return
	}
	h.Pack(j.Target, CrateOf(b))
	u.ids = setMark(u.ids, j.Target, false)
	finish(h, id, j, job.Complete, job.FailNone)
}

// CrateOf is the crated building that packing b gives: one crate of b's
// kind, material and quality, its damage carried as wear.
func CrateOf(b construction.Building) item.Stack {
	return item.Stack{Kind: item.MinifiedBuilding, Qty: 1, X: b.X, Y: b.Y, Building: uint8(b.Kind),
		Stuff: b.Stuff, Quality: b.Quality, Wear: b.Damage}
}
