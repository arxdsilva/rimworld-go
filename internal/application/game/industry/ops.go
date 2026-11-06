package industry

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// The operated machines: a deep drill worked by a miner (wiki: Deep
// drill, a 14000-work cycle per portion, Deep Drilling Speed) and a
// ground-penetrating scanner worked by a researcher (wiki: a find every
// 180000 ticks on average, guaranteed by 360000, Research Speed). Both
// need power, are worked from their interaction spot in sittings and
// draw their full power only while worked (a held bench). An operator's
// work units (1 a tick at plain speed, scaled by the skill and work
// speed) stand in for the wiki's speed stats.
const (
	DrillCycle = 14000 // wiki: Deep drill
	ScanMax    = 360000
	// Sitting is one operating sitting's ticks (orig: the research
	// bench's).
	Sitting = 2 * clock.TicksPerHour
)

// Outcome is what one operating tick did.
type Outcome uint8

const (
	// Continue: keep at it.
	Continue Outcome = iota
	// Stop: the sitting is over (the machine is gone).
	Stop
	// Blocked: the machine cannot work now (no power); the job fails and
	// the machine is put off a while (the retry cooldown).
	Blocked
)

// Ops is the operated machines over a State and its host.
type Ops struct {
	s *State
	h Host
}

// Ops is the operated machines of s.
func (s *State) Ops(h Host) Ops { return Ops{s, h} }

// JobOf is the job that operates kind k (0: none).
func JobOf(k construction.Kind) job.Kind {
	switch k {
	case construction.DeepDrill:
		return job.OperateDrill
	case construction.GroundScanner:
		return job.ScanGround
	}
	return 0
}

// Assign sends worker to the nearest usable deep drill (drill true) or
// scanner, trying the next nearest when one can't be started, at most
// tries machines.
func (o Ops) Assign(worker entity.EntityID, drill bool, tries int) bool {
	ids := o.s.sitesOf(o.h).scanners
	if drill {
		ids = o.s.sitesOf(o.h).drills
	}
	var tried []entity.EntityID
	for range tries {
		px, py := o.h.Pos(worker)
		var best entity.EntityID
		var bx, by int
		bestD := 1 << 30
		for _, id := range ids {
			b, ok := o.h.Building(id)
			if !ok || b.Deconstruct || b.Forbidden || slices.Contains(tried, id) || !o.h.Powered(id) {
				continue
			}
			_, spot, ok := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
			if !ok || !o.h.Free(worker, id, JobOf(b.Kind), spot[0], spot[1]) {
				continue
			}
			if d := mathx.Abs(px-spot[0]) + mathx.Abs(py-spot[1]); d < bestD || d == bestD && id < best {
				best, bestD, bx, by = id, d, spot[0], spot[1]
			}
		}
		if best == 0 {
			return false
		}
		b, _ := o.h.Building(best)
		if o.h.Start(worker, best, JobOf(b.Kind), bx, by, Sitting) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

// Do is one tick of worker operating machine id.
func (o Ops) Do(worker, id entity.EntityID) Outcome {
	b, ok := o.h.Building(id)
	if !ok || JobOf(b.Kind) == 0 {
		return Stop
	}
	if !o.h.Powered(id) {
		return Blocked
	}
	u := o.h.Work(worker, b.Kind == construction.DeepDrill)
	if u == 0 {
		return Continue
	}
	w := o.s.work[id] + u
	if b.Kind == construction.DeepDrill {
		if w >= DrillCycle {
			w = 0
			o.portion(b)
		}
	} else if w >= o.scanTarget() {
		w = 0
		o.s.find(o.h)
	}
	o.s.setWork(id, w)
	return Continue
}

// scanTarget is the work the next find takes: spread evenly over 1 to
// ScanMax (mean 180000, wiki), hashed from the seed and the finds so far.
func (o Ops) scanTarget() int {
	r := stream{o.h.Seed() ^ uint64(o.s.finds+1)*0x9e3779b97f4a7c15 ^ 0x5ca7}
	return 1 + r.next(ScanMax)
}

// portion brings up one drill cycle beside drill b: its nearest found ore
// in reach, else chunks of the stone below (wiki).
func (o Ops) portion(b construction.Building) {
	_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	w, _ := o.h.Size()
	k, ok := o.s.nearestOre(b.X, b.Y, w)
	if !ok {
		o.h.Drop(item.ChunkOf(o.h.StoneAt(b.X, b.Y)), 1, spot[0], spot[1])
		return
	}
	kind, n, out := o.s.take(k)
	o.h.Drop(kind, n, spot[0], spot[1])
	if _, more := o.s.nearestOre(b.X, b.Y, w); out && !more {
		o.h.Letter("Deep ore exhausted", b.X, b.Y, "The deep drill has brought up all the %s in its reach. It brings up stone chunks from now on.", kind.String())
	}
	o.h.Changed()
}

// Cycle is machine id's progress through its cycle, permille.
func (o Ops) Cycle(id entity.EntityID, k construction.Kind) int {
	switch k {
	case construction.DeepDrill:
		return o.s.work[id] * 1000 / DrillCycle
	case construction.GroundScanner:
		return min(999, o.s.work[id]*1000/o.scanTarget())
	}
	return 0
}

// DrillOre is the ore deep drill b would bring up next (item kind name,
// "" none: stone chunks).
func (o Ops) DrillOre(b construction.Building) string {
	w, _ := o.h.Size()
	if k, ok := o.s.nearestOre(b.X, b.Y, w); ok {
		return item.Kind(o.s.ore[k].Ore).String()
	}
	return ""
}
