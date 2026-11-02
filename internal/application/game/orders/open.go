package orders

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Open (AX-1, wiki: Orders): the Open order marks closed containers with
// someone inside. A sealed sleeper casket takes the casket pane's own
// open order (game/casket.go: a builder breaks the seal, the whole vault
// group opens). A filled grave or sarcophagus is marked here; a hauler
// walks over, works OpenWork ticks and lifts the body out beside it (it
// is unburied again). The stasis casket (A9) ejects through its own
// order.
type Open struct {
	h     Host
	marks []entity.EntityID // graves and sarcophagi, ascending (saved)
}

// OpenWork is the ticks opening a grave takes: the sleeper casket's
// seal work (casketOpenWork, original tuning).
const OpenWork = 250

// IsGrave reports a kind that holds a body: a grave or a sarcophagus.
func IsGrave(k construction.Kind) bool {
	return k == construction.Grave || k == construction.Sarcophagus
}

// Mark marks (on) or unmarks every openable container whose origin lies
// in the rectangle.
func (o *Open) Mark(minX, minY, maxX, maxY int, on bool) {
	h := o.h
	for _, id := range h.BuildingIDs() {
		b, ok := h.Building(id)
		if !ok || !in(b.X, b.Y, minX, minY, maxX, maxY) || !h.Seen(b.X, b.Y) {
			continue
		}
		switch {
		case b.Kind == construction.SleeperCasket && h.Sealed(id):
			h.OpenCasket(id, on)
		case IsGrave(b.Kind) && (!on || h.Interred(id)):
			o.marks = setMark(o.marks, id, on)
		}
	}
}

// Marked reports container id marked to open: a marked grave still
// filled, or a sealed casket whose seal a colonist is to break.
func (o *Open) Marked(id entity.EntityID, b construction.Building) bool {
	if b.Kind == construction.SleeperCasket {
		return b.Deconstruct && o.h.Sealed(id)
	}
	return marked(o.marks, id) && o.h.Interred(id)
}

// Assign starts hauler id on the nearest marked grave it may take.
func (o *Open) Assign(id entity.EntityID) bool {
	if len(o.marks) == 0 {
		return false
	}
	h := o.h
	px, py := h.Pos(id)
	var best entity.EntityID
	bestD := 1 << 30
	keep := o.marks[:0]
	for _, gid := range o.marks {
		b, ok := h.Building(gid)
		if !ok || !h.Interred(gid) {
			continue // emptied or gone: the mark is done
		}
		keep = append(keep, gid)
		if !h.Free(id, gid, job.OpenContainer) {
			continue
		}
		if d := dist(px, py, b.X, b.Y); d < bestD {
			best, bestD = gid, d
		}
	}
	o.marks = keep
	if best == 0 {
		return false
	}
	b, _ := h.Building(best)
	tx, ty, ok := h.Spot(id, b.X, b.Y)
	if !ok {
		return false
	}
	return h.StartJob(id, job.Record{Kind: job.OpenContainer, Target: best, TX: tx, TY: ty, WorkLeft: OpenWork})
}

// Do works the grave open, then lifts the body out.
func (o *Open) Do(id entity.EntityID, j job.Record) {
	h := o.h
	if _, ok := h.Building(j.Target); !ok || !h.Interred(j.Target) || !marked(o.marks, j.Target) {
		finish(h, id, j, job.Fail, job.TargetGone)
		return
	}
	w := h.Work(id, false)
	if w == 0 {
		h.SetJob(id, j)
		return
	}
	if j.WorkLeft -= w; j.WorkLeft > 0 {
		h.SetJob(id, j)
		return
	}
	h.Exhume(j.Target)
	o.marks = setMark(o.marks, j.Target, false)
	finish(h, id, j, job.Complete, job.FailNone)
}
