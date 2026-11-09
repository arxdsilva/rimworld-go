// Package pool runs a simulation stage's independent tasks on a few lanes
// (GAR-035, ARCH-010): the owner's goroutine is lane 0 and up to seven
// parked workers join it for one Run, claiming chunks of task indexes from
// an atomic counter. It is the only place the simulation starts
// goroutines (internal/arch TestNoGoStatementsInSim): a task must write
// only its own result slot and read state nobody writes during the Run,
// so the answers never depend on which lane ran what.
package pool

import (
	"fmt"
	"math/rand/v2"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	// MaxLanes is the most lanes a pool runs (lane 0 is the owner's).
	MaxLanes = 8
	// minParallelTasks: a Run of fewer tasks runs inline on the owner.
	minParallelTasks = 8
	// chunksPerRun: a Run is cut into about this many chunks, whatever the
	// lane count (the chunk size depends on the task count only).
	chunksPerRun = 4 * MaxLanes
	// laneBytes is a lane's state padded to two cache lines (no lane's
	// writes share a line with another's).
	laneBytes = 128
)

// Options configure a pool. The zero value is MaxLanes lanes.
type Options struct {
	Lanes int // 1..MaxLanes; 0 means MaxLanes, and 1 runs everything inline
	// MinTasks: a Run of fewer tasks runs inline (0: minParallelTasks).
	MinTasks int
	// Shuffle, when nonzero, claims one task at a time in an order
	// shuffled from it (tests: an answer must not depend on the order).
	Shuffle uint64
}

// Panic is a task's panic re-raised on the owner after the Run: the one
// with the lowest task index, whichever lane met it first.
type Panic struct {
	Task  int
	Value any
	Stack []byte
}

func (p *Panic) Error() string {
	return fmt.Sprintf("pool: task %d panicked: %v\n%s", p.Task, p.Value, p.Stack)
}

// laneState is one lane's own state during a Run.
type laneState struct {
	wake  chan struct{} // a worker parks here between Runs (nil for lane 0)
	low   int           // the lowest task index that panicked this Run, -1 none
	val   any
	stack []byte
}

type lane struct {
	laneState
	_ [(laneBytes - unsafe.Sizeof(laneState{})%laneBytes) % laneBytes]byte
}

// Pool is a fixed set of lanes; its workers start on the first parallel
// Run and park between Runs. One owner calls Run, never concurrently.
type Pool struct {
	next    atomic.Int64 // the next unclaimed task (claim order) index
	_       [laneBytes - 8]byte
	lanes   [MaxLanes]lane
	n, min  int
	started bool
	closed  bool
	tasks   int
	chunk   int
	task    func(lane, i int)
	order   []int      // Shuffle: claim k runs task order[k]
	rng     *rand.Rand // Shuffle's source (tests only)
	wg      sync.WaitGroup
}

// New builds a pool; no worker starts until a Run needs it.
func New(o Options) *Pool {
	p := &Pool{n: o.Lanes, min: o.MinTasks}
	if p.n <= 0 || p.n > MaxLanes {
		p.n = MaxLanes
	}
	if p.min <= 0 {
		p.min = minParallelTasks
	}
	if o.Shuffle != 0 {
		p.rng = rand.New(rand.NewPCG(o.Shuffle, o.Shuffle))
	}
	return p
}

// Lanes is the number of lanes a Run may use.
func (p *Pool) Lanes() int { return p.n }

// Run calls task(lane, i) once for every i in [0, n) and returns when all
// have returned. A panic in a task is re-raised here as a *Panic after the
// Run (inline Runs let it through as it is).
func (p *Pool) Run(n int, task func(lane, i int)) {
	if n <= 0 {
		return
	}
	if p.n == 1 || n < p.min || p.closed {
		for i := range n {
			task(0, i)
		}
		return
	}
	p.task, p.tasks, p.chunk = task, n, max(1, n/chunksPerRun)
	if p.rng != nil {
		p.chunk, p.order = 1, p.rng.Perm(n)
	}
	for l := range p.n {
		p.lanes[l].low = -1
	}
	p.next.Store(0)
	p.start()
	p.wg.Add(p.n - 1)
	for l := 1; l < p.n; l++ {
		p.lanes[l].wake <- struct{}{}
	}
	p.work(0)
	p.wg.Wait()
	p.task, p.order = nil, nil
	p.raise()
}

// Close stops the parked workers; later Runs run inline.
func (p *Pool) Close() {
	if p.closed {
		return
	}
	p.closed = true
	if p.started {
		for l := 1; l < p.n; l++ {
			close(p.lanes[l].wake)
		}
	}
}

// start launches the workers the first time a Run needs them.
func (p *Pool) start() {
	if p.started {
		return
	}
	p.started = true
	for l := 1; l < p.n; l++ {
		p.lanes[l].wake = make(chan struct{}, 1)
		go p.worker(l)
	}
}

// worker is lane l's goroutine: it parks until woken, works, and parks.
func (p *Pool) worker(l int) {
	for range p.lanes[l].wake {
		p.work(l)
		p.wg.Done()
	}
}

// work claims chunks until none are left.
func (p *Pool) work(l int) {
	for {
		start := int(p.next.Add(int64(p.chunk))) - p.chunk
		if start >= p.tasks {
			return
		}
		for k := start; k < min(start+p.chunk, p.tasks); k++ {
			i := k
			if p.order != nil {
				i = p.order[k]
			}
			p.do(l, i)
		}
	}
}

// do runs task i on lane l, keeping the lane's lowest panic.
func (p *Pool) do(l, i int) {
	defer func() {
		if v := recover(); v != nil {
			if s := &p.lanes[l]; s.low < 0 || i < s.low {
				s.low, s.val, s.stack = i, v, debug.Stack()
			}
		}
	}()
	p.task(l, i)
}

// raise re-raises the Run's lowest-index panic, if any.
func (p *Pool) raise() {
	best := -1
	for l := range p.n {
		if s := &p.lanes[l]; s.low >= 0 && (best < 0 || s.low < p.lanes[best].low) {
			best = l
		}
	}
	if best < 0 {
		return
	}
	s := &p.lanes[best]
	err := &Panic{Task: s.low, Value: s.val, Stack: s.stack}
	for l := range p.n {
		p.lanes[l].val, p.lanes[l].stack = nil, nil
	}
	panic(err)
}
