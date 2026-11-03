package research

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/building/construction"
	proj "rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Session is how long one sitting at the bench lasts.
const Session = 2 * clock.TicksPerHour

// Lab is the colony's research. Prog (points into each project, at base
// cost), Cur and Set (the project picked; no queue: wiki), Carry (the
// fraction of a point, billionths) and Cold (benches in the bad
// temperature band) are saved; the rest is derived.
type Lab struct {
	h     Host
	Prog  proj.Progress
	Cur   proj.Project
	Set   bool
	Carry int
	Cold  map[entity.EntityID]bool

	benches benchCache
	// rate: raw research points (billionths) put in on tick rateTick
	// and the tick before, for the ETA; never saved.
	rateTick         uint64
	rateNow, rateWas int64
	memo             memo
	prereqs          [][]int // View.Prereqs per project, built once
}

// New is an empty lab on host h (nothing researched).
func New(h Host) *Lab { return &Lab{h: h} }

// Outcome is what a working tick did to the sitting.
type Outcome uint8

const (
	// Continue: keep at it.
	Continue Outcome = iota
	// Stop: the sitting is over (the project finished, or nothing to do).
	Stop
	// Blocked: the bench cannot work now (no power, its analyzer gone);
	// the job fails and the bench is put off a while.
	Blocked
)

// Pick sets the current project (ignored unless available).
func (l *Lab) Pick(p proj.Project) {
	if l.Prog.Available(p) {
		l.Cur, l.Set = p, true
	}
}

// Working reports a project picked and still open.
func (l *Lab) Working() bool { return l.Set && l.Prog.Available(l.Cur) }

// Assign sends worker to the nearest research bench that can work the
// current project, trying the next nearest when one can't be started,
// at most tries benches.
func (l *Lab) Assign(worker entity.EntityID, tries int) bool {
	if !l.Working() {
		return false
	}
	var tried []entity.EntityID
	for range tries {
		bid, ok := l.try(worker, tried)
		if ok || bid == 0 {
			return ok
		}
		tried = append(tried, bid)
	}
	return false
}

// try starts research at the nearest usable bench not in tried; it
// returns the bench tried (0: none left).
func (l *Lab) try(worker entity.EntityID, tried []entity.EntityID) (entity.EntityID, bool) {
	px, py := l.h.Pos(worker)
	var best entity.EntityID
	var bx, by int
	bestD := 1 << 30
	for _, bid := range l.cache().benches {
		b, ok := l.h.Building(bid)
		if !ok || b.Deconstruct || slices.Contains(tried, bid) || !l.usable(bid, b) {
			continue
		}
		_, spot, ok := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		if !ok || !l.h.Free(worker, bid, spot[0], spot[1]) {
			continue
		}
		if d := mathx.Abs(px-spot[0]) + mathx.Abs(py-spot[1]); d < bestD || (d == bestD && bid < best) {
			best, bestD, bx, by = bid, d, spot[0], spot[1]
		}
	}
	if best == 0 {
		return 0, false
	}
	return best, l.h.Start(worker, best, bx, by, Session)
}

// usable reports bench can work the current project now: powered when
// electric, and of the tier the project needs.
func (l *Lab) usable(bench entity.EntityID, b construction.Building) bool {
	return (!hiTech(b.Kind) || l.h.Powered(bench)) && l.CanWork(bench, b, l.Cur)
}

// Work is one tick of worker's sitting at bench: research points =
// 0.00825 x research speed x bench factor x difficulty factor (wiki:
// Research), divided by the project's tech-level cost and the scenario's
// cost parts, the fraction carried. Finishing posts the letter and drops
// the project (no queue).
func (l *Lab) Work(worker, bench entity.EntityID) Outcome {
	b, ok := l.h.Building(bench)
	if !ok || !l.Working() {
		return Stop
	}
	if !l.usable(bench, b) {
		return Blocked
	}
	if !l.h.Work(worker) {
		return Continue
	}
	l.feel(bench, b)
	raw := PointsNano(Speed(l.h.Speed(worker)), l.Factor(bench), l.h.Difficulty())
	l.note(raw)
	total := raw*100*100/int64(max(1, l.costPct(l.Cur)*l.h.CostPct())) + int64(l.Carry)
	l.Carry = int(total % nano)
	if !l.Prog.Add(l.Cur, int(total/nano)) {
		return Continue
	}
	l.finish(b.X, b.Y)
	return Stop
}

// costPct is what p costs this colony, percent of its base cost.
func (l *Lab) costPct(p proj.Project) int { return proj.CostPct(proj.DefOf(p).TechLevel, l.h.Tech()) }

// note adds raw points to this tick's rate.
func (l *Lab) note(raw int64) {
	if now := l.h.Now(); now != l.rateTick {
		l.rateWas = 0
		if now == l.rateTick+1 {
			l.rateWas = l.rateNow
		}
		l.rateTick, l.rateNow = now, 0
	}
	l.rateNow += raw
}

// finish ends the current project, the one way every path completes it
// (bench work, the dev finish): the research-complete letter at (x, y)
// carries its description and unlocks (wiki: a letter on completion), and
// the selection clears (no queue; the alert follows).
func (l *Lab) finish(x, y int) {
	d := proj.DefOf(l.Cur)
	text := d.Desc
	if len(d.Unlocks) > 0 {
		text += "\n\nUnlocked: " + strings.Join(UnlockLabels(d), ", ") + "."
	}
	l.h.Letter("Research complete: "+d.Label, text, x, y)
	l.Set, l.Carry = false, 0
}

// CostOf is project p's cost to this colony, percent of its base cost
// (its tech level's and the scenario's parts, as bench work pays).
func (l *Lab) CostOf(p proj.Project) int { return max(1, l.costPct(p)*l.h.CostPct()/100) }

// Study puts n points (at base cost) into p from a schematic being read
// (AX-7, game/books). Finishing p posts the research letter at (x, y)
// and drops p if it was the current project; another pick is kept.
func (l *Lab) Study(p proj.Project, n, x, y int) {
	if !l.Prog.Add(p, n) {
		return
	}
	cur, set, carry := l.Cur, l.Set, l.Carry
	l.Cur = p
	l.finish(x, y)
	if p != cur {
		l.Cur, l.Set, l.Carry = cur, set, carry
	}
	l.h.Changed()
}

// DevFinish finishes the current project at once, as if its last point
// went in (developer mode).
func (l *Lab) DevFinish() {
	if !l.Working() {
		return
	}
	l.Prog.Add(l.Cur, proj.DefOf(l.Cur).Cost)
	var x, y int // the letter points at the first research bench, if any
	if bs := l.cache().benches; len(bs) > 0 {
		b, _ := l.h.Building(bs[0])
		x, y = b.X, b.Y
	}
	l.finish(x, y)
	l.h.Changed()
}

// Grant finishes p and everything it needs, drawn and hidden (a
// scenario's starting research).
func (l *Lab) Grant(p proj.Project) {
	if p >= proj.NumProjects || l.Prog.Done(p) {
		return
	}
	d := proj.DefOf(p)
	for _, r := range slices.Concat(d.Requires, d.Hidden) {
		l.Grant(r)
	}
	l.Prog[p] = d.Cost
}

// Reset forgets all research (a fresh colony's tree).
func (l *Lab) Reset() {
	l.Prog, l.Set, l.Carry = proj.Progress{}, false, 0
	l.h.Changed()
}

// Complete finishes every project (sandbox, dev hook).
func (l *Lab) Complete() {
	l.Prog = proj.Complete()
	l.h.Changed()
}

// LockedBy names the project gating an unlock key ("" when usable).
func (l *Lab) LockedBy(key string) string {
	if p, ok := l.Prog.Locks(key); ok {
		return proj.DefOf(p).Label
	}
	return ""
}
