package industry

import (
	"slices"

	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/sim/entity"
)

// State is the industry's state. The zero State is a map with no river,
// no deep ore found and no machine started; Save is what a save keeps of
// it, the rest is derived.
type State struct {
	w, h int
	// flow is each cell's river flow (Dir8+1; 0: still or dry), nil
	// without a river.
	flow []uint8
	// ore is the deep ore found so far, ascending by cell index.
	ore   []OreCell
	finds int    // finds so far (each hashes the next)
	ver   uint64 // moves whenever ore changes (the overlay's cache key)
	// work is each running drill's, scanner's and pump's progress:
	// drill and scanner work units, pump powered ticks.
	work map[entity.EntityID]int

	dep    []axview.Deposit // DeepOre's last list, at depVer and depW
	depVer uint64
	depW   int
	depOK  bool
	sites  siteCache // drills, scanners, pumps and mills per structure
	mill   millCache // watermill outputs per structure
	wind   windCache // turbine bands per structure, roof and hour
}

// OreCell is one deep ore cell found: its index, ore and units left.
type OreCell struct {
	I    int32
	Ore  uint8 // item.Kind
	Left int32
}

// Save is State's saved form (game save field AX4; absent in older
// saves: no river flow, no ore found).
type Save struct {
	W, H  int
	Flow  []uint8
	Ore   []OreCell
	Finds int
	Ver   uint64 // the deep ore overlay's key (WorldView.AX.DeepOreVer)
	Work  []WorkEntry
}

// WorkEntry is one machine's progress, in building id order.
type WorkEntry struct {
	ID   entity.EntityID
	Work int
}

// Export is the saved form; machines that are gone (alive reports
// whether a building still stands) are left out.
func (s *State) Export(alive func(entity.EntityID) bool) Save {
	out := Save{W: s.w, H: s.h, Flow: slices.Clone(s.flow), Ore: slices.Clone(s.ore), Finds: s.finds, Ver: s.ver}
	for id, w := range s.work {
		if alive(id) {
			out.Work = append(out.Work, WorkEntry{id, w})
		}
	}
	slices.SortFunc(out.Work, func(a, b WorkEntry) int { return int(a.ID) - int(b.ID) })
	return out
}

// Import restores a saved state.
func (s *State) Import(v Save) {
	*s = State{w: v.W, h: v.H, flow: slices.Clone(v.Flow), ore: slices.Clone(v.Ore), finds: v.Finds, ver: v.Ver}
	if len(s.flow) != s.w*s.h {
		s.flow = nil
	}
	for _, e := range v.Work {
		s.setWork(e.ID, e.Work)
	}
}

func (s *State) setWork(id entity.EntityID, w int) {
	if s.work == nil {
		s.work = map[entity.EntityID]int{}
	}
	s.work[id] = w
}

// SetFlow keeps the generator's river flow (mapgen Result.River) for a
// w x h map; nil or a mask of another size is no river.
func (s *State) SetFlow(flow []uint8, w, h int) {
	s.w, s.h, s.flow = w, h, nil
	if len(flow) == w*h && slices.ContainsFunc(flow, func(d uint8) bool { return d != 0 }) {
		s.flow = slices.Clone(flow)
	}
}

// Moving reports river water flowing on (x, y): the cells a watermill's
// wheel must sit in, and that a moisture pump never dries.
func (s *State) Moving(x, y int) bool { return s.FlowAt(x, y) != 0 }

// FlowAt is (x, y)'s flow direction (Dir8+1; 0 still or dry).
func (s *State) FlowAt(x, y int) uint8 {
	if s.flow == nil || x < 0 || y < 0 || x >= s.w || y >= s.h {
		return 0
	}
	return s.flow[y*s.w+x]
}
