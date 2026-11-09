// Package sched is the game's phase scheduler (GAR): the simulation's
// systems as descriptors (phase, scope, cadence, run condition, the
// stores they read and write, order edges) put in one checked order at
// construction. The order is Kahn's algorithm over the After/Before edges,
// ties broken by (phase rank, name), so it is the same on every build and
// every machine; a cycle, an unknown name or two unordered systems of a
// phase that share a store are construction errors.
//
// A Pass runs the order once for one map: a Colony system only on the
// first map's pass (MAP-0), a system with Every only on its ticks
// (cadence.Due with id 0), one with RunIf only while it holds. Timing is
// taken only while an observer is set; it never feeds the simulation.
//
// It is a leaf package of internal/application/game: deterministic, no
// randomness, wall-clock time only for the observer.
package sched

import (
	"container/heap"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Scope is which map passes run a system.
type Scope uint8

// The scopes.
const (
	PerMap Scope = iota // every open map's pass
	Colony              // only the first map's pass: the colony-wide work
)

func (s Scope) String() string {
	if s == Colony {
		return "colony"
	}
	return "map"
}

// StoreID names a piece of game state a system reads or writes.
type StoreID uint16

// Legacy is the whole game state: a wrapped phase that has not yet
// declared its stores reads and writes it, so it conflicts with every
// system of its phase and must be ordered against each.
const Legacy StoreID = 0

// System is one piece of simulation work.
type System struct {
	Name  string
	Phase string // one of the schedule's phases; its rank orders ties
	Scope Scope
	// Every and Offset: with Every > 0 the system runs only on the ticks
	// where (tick+Offset)%Every == 0 (cadence.Due with id 0).
	Every, Offset uint64
	RunIf         func() bool // nil: always
	Reads, Writes []StoreID
	After, Before []string // names this system runs after or before
	Ambiguous     []string // systems of its phase it may share a store with, unordered
	Run           func()   // bound once at construction (a method value)
}

// Wrap is a system around an existing phase function: it reads and
// writes Legacy and runs on every pass of every map.
func Wrap(name, phase string, run func()) System {
	return System{Name: name, Phase: phase, Reads: []StoreID{Legacy}, Writes: []StoreID{Legacy}, Run: run}
}

// At sets the system's cadence: it runs only where (tick+offset)%every == 0.
func (y System) At(every, offset uint64) System {
	y.Every, y.Offset = every, offset
	return y
}

// ForColony makes it a Colony system.
func (y System) ForColony() System {
	y.Scope = Colony
	return y
}

// When sets its run condition.
func (y System) When(f func() bool) System {
	y.RunIf = f
	return y
}

// Then orders it before the named systems.
func (y System) Then(names ...string) System {
	y.Before = append(slices.Clip(y.Before), names...)
	return y
}

// New orders systems for the phases (their names in rank order).
func New(phases []string, systems []System) (*Schedule, error) {
	order, err := sortSystems(phases, systems)
	if err != nil {
		return nil, err
	}
	s := &Schedule{sys: make([]System, len(order)), end: make([]bool, len(order)), stats: make([]stat, len(order))}
	for i, j := range order {
		s.sys[i] = systems[j]
	}
	for i := range s.sys {
		s.end[i] = i == len(s.sys)-1 || s.sys[i+1].Phase != s.sys[i].Phase
	}
	return s, nil
}

// Must is New for a fixed table: an error is a programming error.
func Must(phases []string, systems []System) *Schedule {
	s, err := New(phases, systems)
	if err != nil {
		panic(err)
	}
	return s
}

// sortSystems checks the descriptors and returns their run order as
// indexes into systems.
func sortSystems(phases []string, systems []System) ([]int, error) {
	rank := map[string]int{}
	for i, p := range phases {
		rank[p] = i
	}
	byName := map[string]int{}
	for i, y := range systems {
		if _, ok := rank[y.Phase]; !ok {
			return nil, fmt.Errorf("sched: %s: unknown phase %q", y.Name, y.Phase)
		}
		if _, dup := byName[y.Name]; dup || y.Name == "" {
			return nil, fmt.Errorf("sched: system name %q empty or twice", y.Name)
		}
		if y.Run == nil {
			return nil, fmt.Errorf("sched: %s: no Run", y.Name)
		}
		byName[y.Name] = i
	}
	succ := make([][]int, len(systems)) // succ[i]: the systems that run after i
	for i, y := range systems {
		for k, refs := range [2][]string{y.After, y.Before} {
			for _, r := range refs {
				j, ok := byName[r]
				if !ok {
					return nil, fmt.Errorf("sched: %s: unknown system %q", y.Name, r)
				}
				if k == 0 {
					succ[j] = append(succ[j], i) // i After j
				} else {
					succ[i] = append(succ[i], j) // i Before j
				}
			}
		}
	}
	order, err := kahn(systems, rank, succ)
	if err != nil {
		return nil, err
	}
	return order, ambiguity(systems, succ)
}

// kahn is the topological order, the ready systems taken lowest
// (phase rank, name) first.
func kahn(systems []System, rank map[string]int, succ [][]int) ([]int, error) {
	in := make([]int, len(systems))
	for _, ss := range succ {
		for _, j := range ss {
			in[j]++
		}
	}
	ready := &readyHeap{less: func(a, b int) bool {
		ra, rb := rank[systems[a].Phase], rank[systems[b].Phase]
		return ra < rb || ra == rb && systems[a].Name < systems[b].Name
	}}
	for i, n := range in {
		if n == 0 {
			ready.ids = append(ready.ids, i)
		}
	}
	heap.Init(ready)
	order := make([]int, 0, len(systems))
	for ready.Len() > 0 {
		i := heap.Pop(ready).(int)
		order = append(order, i)
		for _, j := range succ[i] {
			if in[j]--; in[j] == 0 {
				heap.Push(ready, j)
			}
		}
	}
	if len(order) < len(systems) {
		return nil, cycle(systems, succ, in)
	}
	return order, nil
}

// cycle names a chain of systems left with unmet edges: from the first
// left, follow successors that are left until one repeats.
func cycle(systems []System, succ [][]int, in []int) error {
	start := slices.IndexFunc(in, func(n int) bool { return n > 0 })
	seen := map[int]int{}
	var path []int
	for i := start; ; {
		if at, ok := seen[i]; ok {
			names := make([]string, 0, len(path)-at+1)
			for _, j := range append(path[at:], i) {
				names = append(names, systems[j].Name)
			}
			return fmt.Errorf("sched: cycle %s", strings.Join(names, " -> "))
		}
		seen[i] = len(path)
		path = append(path, i)
		next := slices.IndexFunc(succ[i], func(j int) bool { return in[j] > 0 })
		if next < 0 {
			return errors.New("sched: cycle") // unreachable: a node left has a predecessor left
		}
		i = succ[i][next]
	}
}

// ambiguity rejects two systems of a phase that share a store (one
// writing it) with no path between them, unless one names the other in
// Ambiguous.
func ambiguity(systems []System, succ [][]int) error {
	reach := make([][]bool, len(systems))
	for i := range systems {
		reach[i] = make([]bool, len(systems))
		stack := append([]int(nil), succ[i]...)
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !reach[i][j] {
				reach[i][j] = true
				stack = append(stack, succ[j]...)
			}
		}
	}
	for i, a := range systems {
		for j := i + 1; j < len(systems); j++ {
			b := systems[j]
			if a.Phase != b.Phase || reach[i][j] || reach[j][i] || slices.Contains(a.Ambiguous, b.Name) || slices.Contains(b.Ambiguous, a.Name) {
				continue
			}
			if st, ok := conflict(a, b); ok {
				return fmt.Errorf("sched: %s and %s share store %d with no order between them", a.Name, b.Name, st)
			}
		}
	}
	return nil
}

// conflict is a store one of a and b writes and the other uses (Legacy
// is every store).
func conflict(a, b System) (StoreID, bool) {
	for _, p := range [2][2][]StoreID{{a.Writes, append(slices.Clone(b.Reads), b.Writes...)}, {b.Writes, a.Reads}} {
		for _, w := range p[0] {
			for _, u := range p[1] {
				if w == u || w == Legacy || u == Legacy {
					return w, true
				}
			}
		}
	}
	return 0, false
}

// readyHeap is the ready systems, lowest first.
type readyHeap struct {
	ids  []int
	less func(a, b int) bool
}

func (h *readyHeap) Len() int           { return len(h.ids) }
func (h *readyHeap) Less(a, b int) bool { return h.less(h.ids[a], h.ids[b]) }
func (h *readyHeap) Swap(a, b int)      { h.ids[a], h.ids[b] = h.ids[b], h.ids[a] }
func (h *readyHeap) Push(x any)         { h.ids = append(h.ids, x.(int)) }
func (h *readyHeap) Pop() any {
	x := h.ids[len(h.ids)-1]
	h.ids = h.ids[:len(h.ids)-1]
	return x
}
