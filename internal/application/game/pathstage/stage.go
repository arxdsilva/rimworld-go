// Package pathstage runs the movement phase's path searches as a stage
// (GAR-035, ARCH-010): decide all, then apply. The phase queues its
// requests in ascending creature ID while creatures step; after the
// movement loop the owner warms every cache the searches read, the stage
// runs each search as a pure function of that frozen state (one
// pathing.Searcher per lane, so lanes may run them at once), and the phase
// applies the answers in request order. A search writes nothing shared:
// its answer lands in its own result slot.
package pathstage

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
)

// Pool runs n independent tasks, each exactly once, on up to Lanes()
// lanes; lane 0 is the caller's. Run returns once every task has. A
// task's lane is any in [0, Lanes()); the answers must not depend on it.
// internal/platform/pool implements it; nil means serial.
type Pool interface {
	Lanes() int
	Run(n int, task func(lane, i int))
}

// Kind is how a request is searched.
type Kind uint8

const (
	Exact    Kind = iota // a search over the whole map
	Corridor             // inside its region corridor first, then the whole map
	Failed               // decided while planning: no route
	Here                 // decided while planning: already there (an empty route)
)

// Search answers one search of kind k from (sx, sy) to (tx, ty) on sr;
// allow is a Corridor search's cells.
func Search(sr *pathing.Searcher, k Kind, q pathing.Query, allow []uint64, sx, sy, tx, ty int) ([]pathing.Point, bool) {
	switch k {
	case Exact:
		return sr.Find(q, sx, sy, tx, ty)
	case Corridor:
		if p, ok := sr.FindIn(q, allow, sx, sy, tx, ty); ok {
			return p, true
		}
		return sr.Find(q, sx, sy, tx, ty)
	}
	return nil, k == Here
}

// Token is a walk as it stood when its request was queued: the path's
// backing array, its length, the step index and the cell. Apply skips an
// answer whose creature's walk no longer matches (someone rewrote it).
type Token struct {
	path *pathing.Point
	n, i int
	at   pathing.Point
}

// TokenOf is the token of a walk on path at step i from cell at.
func TokenOf(path []pathing.Point, i int, at pathing.Point) Token {
	t := Token{n: len(path), i: i, at: at}
	if len(path) > 0 {
		t.path = &path[0]
	}
	return t
}

// Request is one queued search: creature ID, standing on From and kept
// out of To (the cell it meant to enter), wants a route to Goal. The
// owner plans it (SetPlan) before Compute.
type Request struct {
	ID             entity.EntityID
	From, To, Goal pathing.Point
	Token          Token
	Query          pathing.Query
	Kind           Kind
	lo, hi         int // the corridor's span of the arena
}

// Result is a search's answer.
type Result struct {
	Path []pathing.Point
	OK   bool
}

// Stage is one map's queue, arena and answers (transient: emptied every
// movement phase, never saved). The zero value is a serial stage.
type Stage struct {
	pool   Pool
	open   bool
	frozen bool
	reqs   []Request
	res    []Result
	arena  []uint64           // the requests' corridors, copied out of the region map
	lanes  []pathing.Searcher // lanes 1..: lane 0 searches with the owner's
	own    *pathing.Searcher
	task   func(lane, i int) // s.search, bound once
	runs   uint64            // Compute calls that searched (tests, benchmarks)
	asked  uint64            // requests searched
}

// SetPool makes Compute run its searches on p (nil: serial).
func (s *Stage) SetPool(p Pool) { s.pool = p }

// Pool is what Compute runs on (nil: serial).
func (s *Stage) Pool() Pool { return s.pool }

// Open starts a movement phase's queue; Queue is refused until then.
func (s *Stage) Open() {
	s.open = true
	s.reqs, s.res, s.arena = s.reqs[:0], s.res[:0], s.arena[:0]
}

// Close ends the phase: later searches run at once, as before the stage.
func (s *Stage) Close() { s.open = false }

// Queue adds a request if a phase is open and reports whether it did.
// Requests must come in ascending ID (the movers' order).
func (s *Stage) Queue(r Request) bool {
	if !s.open {
		return false
	}
	if n := len(s.reqs); n > 0 && s.reqs[n-1].ID >= r.ID {
		panic("pathstage: requests out of ID order")
	}
	s.reqs = append(s.reqs, r)
	return true
}

// Requests is the phase's queue, for planning (fill Query and Kind).
func (s *Stage) Requests() []Request { return s.reqs }

// SetPlan makes request i a search of kind k over q, copying a Corridor
// search's cells (valid only until the region map's next corridor) into
// the arena.
func (s *Stage) SetPlan(i int, q pathing.Query, k Kind, allow []uint64) {
	r := &s.reqs[i]
	r.Query, r.Kind, r.lo = q, k, len(s.arena)
	s.arena = append(s.arena, allow...)
	r.hi = len(s.arena)
}

// Frozen reports that searches are running on several lanes: every cache
// they read must be read only (lazy fills and self-check counters skip).
func (s *Stage) Frozen() bool { return s.frozen }

// Compute answers every request; own is lane 0's searcher. Planning must
// have warmed every cache the queries read.
func (s *Stage) Compute(own *pathing.Searcher) {
	n := len(s.reqs)
	if n == 0 {
		return
	}
	s.runs++
	s.asked += uint64(n)
	if cap(s.res) < n {
		s.res = make([]Result, n)
	}
	s.res = s.res[:n]
	s.own = own
	if s.task == nil {
		s.task = s.search
	}
	if s.pool == nil || s.pool.Lanes() <= 1 {
		for i := range n {
			s.search(0, i)
		}
		return
	}
	if l := s.pool.Lanes() - 1; len(s.lanes) < l {
		s.lanes = make([]pathing.Searcher, l)
	}
	s.frozen = true
	defer func() { s.frozen = false }()
	s.pool.Run(n, s.task)
}

// Result is request i's answer (after Compute).
func (s *Stage) Result(i int) Result { return s.res[i] }

// Stats counts the Compute calls that had requests and the requests.
func (s *Stage) Stats() (runs, requests uint64) { return s.runs, s.asked }

// search answers request i on lane's searcher.
func (s *Stage) search(lane, i int) {
	sr := s.own
	if lane > 0 {
		sr = &s.lanes[lane-1]
	}
	r := &s.reqs[i]
	p, ok := Search(sr, r.Kind, r.Query, s.arena[r.lo:r.hi], r.From.X, r.From.Y, r.Goal.X, r.Goal.Y)
	s.res[i] = Result{Path: p, OK: ok}
}
