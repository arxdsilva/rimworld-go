package orders

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Install (AX-1, wiki: Minified thing): Reinstall places a crated
// building as a blueprint (a frame whose Reinstall is the crate: no
// materials). A builder walks to the crate, carries it to the blueprint
// and installs it; the building comes back with the crate's material,
// quality and wear. The work is the move order's (game/move.go: half the
// build work, at least MinInstallWork; original tuning).
type Install struct {
	h Host
	// ids are the reinstall blueprints, ascending; built from the frames
	// on first use (derived, never saved).
	ids   []entity.EntityID
	built bool
}

// MinInstallWork is the least work an install takes (ticks), as a move.
const MinInstallWork = 60

// Reasons a reinstall is refused.
const (
	ReasonNoCrate = "no crated building"
)

// InstallWork is the work installing kind of stuff takes.
func InstallWork(kind construction.Kind, stuff item.Kind) int {
	return max(MinInstallWork, construction.WorkFor(kind, stuff)/2)
}

// Place lays the reinstall blueprint of crate at (x, y) facing f: a crate
// has one blueprint, so a second placement replaces the first. It reports
// why it refused ("" when placed).
func (in *Install) Place(crate entity.EntityID, f construction.Facing, x, y int) string {
	h := in.h
	st, ok := h.Stack(crate)
	if !ok || st.Kind != item.MinifiedBuilding || st.Qty <= 0 || !construction.Kind(st.Building).Valid() {
		return ReasonNoCrate
	}
	kind := construction.Kind(st.Building)
	if f > construction.West || !construction.DefOf(kind).Rotatable {
		f = construction.South
	}
	if ok, why := h.Fits(kind, st.Stuff, f, x, y); !ok {
		return why
	}
	if fid := in.FrameOf(crate); fid != 0 {
		h.DropFrame(fid)
		in.ids = setMark(in.ids, fid, false)
	}
	fid := h.AddFrame(construction.Frame{Kind: kind, Stuff: st.Stuff, Facing: f, X: x, Y: y,
		WorkLeft: InstallWork(kind, st.Stuff), Reinstall: uint64(crate)})
	in.ids = setMark(in.ids, fid, true)
	return ""
}

// FrameOf is crate's reinstall blueprint (0: none).
func (in *Install) FrameOf(crate entity.EntityID) entity.EntityID {
	in.index()
	for _, fid := range in.ids {
		if fr, ok := in.h.Frame(fid); ok && entity.EntityID(fr.Reinstall) == crate {
			return fid
		}
	}
	return 0
}

// index builds the blueprint list from the frames once.
func (in *Install) index() {
	if in.built {
		return
	}
	in.built = true
	for _, fid := range in.h.FrameIDs() {
		if fr, ok := in.h.Frame(fid); ok && in.crated(fr) {
			in.ids = setMark(in.ids, fid, true)
		}
	}
}

// crated reports a reinstall blueprint whose crate still exists.
func (in *Install) crated(fr construction.Frame) bool {
	if fr.Reinstall == 0 {
		return false
	}
	st, ok := in.h.Stack(entity.EntityID(fr.Reinstall))
	return ok && st.Kind == item.MinifiedBuilding
}

// Assign starts builder id on the nearest crate whose blueprint waits;
// a blueprint whose crate is gone (sold, burned) is taken off.
func (in *Install) Assign(id entity.EntityID) bool {
	in.index()
	if len(in.ids) == 0 {
		return false
	}
	h := in.h
	px, py := h.Pos(id)
	var best, crate entity.EntityID
	bestD := 1 << 30
	var keep, gone []entity.EntityID
	for _, fid := range in.ids {
		fr, ok := h.Frame(fid)
		if !ok {
			continue
		}
		if !in.crated(fr) {
			gone = append(gone, fid)
			continue
		}
		keep = append(keep, fid)
		cid := entity.EntityID(fr.Reinstall)
		st, _ := h.Stack(cid)
		if st.Carrier != 0 && st.Carrier != id || st.Forbidden || !h.Free(id, fid, job.Install) || !h.Free(id, cid, job.Install) {
			continue
		}
		if d := dist(px, py, st.X, st.Y); d < bestD {
			best, crate, bestD = fid, cid, d
		}
	}
	in.ids = keep
	for _, fid := range gone {
		h.DropFrame(fid)
	}
	if best == 0 {
		return false
	}
	st, _ := h.Stack(crate)
	tx, ty, ok := h.Spot(id, st.X, st.Y)
	if !ok {
		return false
	}
	fr, _ := h.Frame(best)
	if !h.StartJob(id, job.Record{Kind: job.Install, Target: best, Dest: crate, TX: tx, TY: ty, WorkLeft: fr.WorkLeft}) {
		return false
	}
	h.Claim(id, crate)
	return true
}

// Do runs an install: pick the crate up, carry it to the blueprint, work
// it, install.
func (in *Install) Do(id entity.EntityID, j job.Record) {
	h := in.h
	fr, ok := h.Frame(j.Target)
	st, has := h.Stack(j.Dest)
	if !ok || !has || entity.EntityID(fr.Reinstall) != j.Dest || st.Carrier != 0 && st.Carrier != id {
		finish(h, id, j, job.Fail, job.TargetGone)
		return
	}
	if j.Carry == 0 {
		h.Carry(j.Dest, id)
		j.Carry = j.Dest
		tx, ty, ok := h.Spot(id, fr.X, fr.Y)
		if !ok || !h.Walk(id, j, tx, ty) {
			finish(h, id, j, job.Fail, job.Unreachable) // the crate is put down where it stands
		}
		return
	}
	w := h.Work(id, true)
	if w == 0 {
		h.SetJob(id, j)
		return
	}
	fr.WorkLeft -= w
	j.WorkLeft = fr.WorkLeft
	if fr.WorkLeft > 0 {
		h.SetFrame(j.Target, fr)
		h.SetJob(id, j)
		return
	}
	j.Carry = 0
	h.Install(j.Target, j.Dest)
	in.ids = setMark(in.ids, j.Target, false)
	finish(h, id, j, job.Complete, job.FailNone)
}

// WorkTotal is the full work of an install (fr its blueprint) or of
// opening a grave (the job's progress bar); 0 for any other job.
func WorkTotal(k job.Kind, fr *construction.Frame) int {
	switch {
	case k == job.Install && fr != nil:
		return InstallWork(fr.Kind, fr.Stuff)
	case k == job.OpenContainer:
		return OpenWork
	}
	return 0
}
