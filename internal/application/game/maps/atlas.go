package maps

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/caravan"
)

// The open maps (MAP-01..04). Map 0 is the first colony and has no row
// here; every other open map has a Meta. Opening, entering and closing
// are requests queued during a tick and applied by Flush at its end,
// never inside a phase.

// Kind is what an open map is (append only: saved).
type Kind uint8

const (
	Colony Kind = iota
	Site
	Settlement
	Encounter
)

var kindNames = [...]string{"colony", "site", "settlement", "encounter"}

// String is the kind's view name ("site").
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return ""
}

const (
	// SiteSide and EncounterSide are temporary maps' sides in cells
	// (original tuning: the wiki publishes none).
	SiteSide      = 150
	EncounterSide = 100
	// closeStreak is how many hourly census results in a row close a map
	// or start its pack-up (original tuning: hysteresis, no flapping).
	closeStreak = 3
	// packTicks is the time to pack up once a temporary map's job is done
	// (wiki: Caravan, 24 hours).
	packTicks = clock.TicksPerDay
	// raidDays is when hostiles come to a mineral site (wiki: Long-range
	// mineral scanner, 10 days).
	raidDays = 10
)

// SizeFor is a new map's side: a colony keeps the setup's side.
func SizeFor(k Kind, colonySide int) int {
	switch k {
	case Site, Settlement:
		return SiteSide
	case Encounter:
		return EncounterSide
	}
	return colonySide
}

// SeedFor is map id's generator seed on tile, from the game seed: a pure
// splitmix64 mix, so opening a map draws from no stream.
func SeedFor(seed uint64, tile int, id uint32) uint64 {
	v := seed ^ uint64(tile)*0x9e3779b97f4a7c15 ^ uint64(id)<<40
	v += 0x9e3779b97f4a7c15
	v = (v ^ v>>30) * 0xbf58476d1ce4e5b9
	v = (v ^ v>>27) * 0x94d049bb133111eb
	return v ^ v>>31
}

// Meta is one open map's bookkeeping (saved).
type Meta struct {
	ID      uint32
	Kind    Kind
	Tile    int
	Faction int // a settlement's faction (1 + index), 0 none
	// Opened is when it opened; Expires is reserved for maps that time
	// out unvisited; PackBy is when its pack-up ends (0: not started; once
	// started it never restarts).
	Opened, Expires, PackBy uint64
	// ResumeDest is where a caravan reformed here heads (-1: home).
	ResumeDest int
	Defeated   bool
	// Lump is a mineral site's mineral and Cells the lump's ore cells
	// (index y*w+x); Raided: its hostiles were sent.
	Lump   item.Kind
	Cells  []int
	Raided bool
	// Hostile: hostiles were seen here. Empty and Done count hourly
	// census results in a row (nobody of ours left; the job done).
	Hostile     bool
	Empty, Done uint8
	// Routed counts hourly results in a row with no standing defender on
	// a settlement map (MAP-05, settle.go besiege).
	Routed uint8
	// Land is where the map's generator set people down (HasLand: it did).
	Land    [2]int
	HasLand bool
}

// Op is a queued map request.
type Op uint8

const (
	OpOpen   Op = iota + 1 // open a Kind map at Tile for Caravan, then enter it
	OpEnter                // Caravan enters map Map
	OpReform               // the pawns on map Map leave as a caravan, then it closes
	OpClose                // map Map closes, everything left on it lost
	OpSettle               // Caravan founds a colony on Tile
)

// Request is one queued map request (saved).
type Request struct {
	Op      Op
	Map     uint32
	Kind    Kind
	Tile    int
	Caravan uint64
	Lump    item.Kind
	Qty     int
	Centre  bool // a settlement attack enters at the map's centre (pods, MAP-05)
	// Trouble is what Caravan met, from Faction (1 + index); Resume its
	// state on the road before it stopped (MAP-08).
	Trouble Trouble
	Faction int
	Resume  caravan.State
	// Spot: pods come down at (X, Y) (or at the centre with Centre) instead
	// of walking in from the edge (MAP-10, pod.go).
	Spot bool
	X, Y int
}

// Atlas is the open maps beyond the first and the queued requests
// (saved; the zero value is a single-map game).
type Atlas struct {
	// Next is the next map id to hand out (0 means 1: map 0 exists).
	Next  uint32
	Maps  []Meta // ascending ID
	Queue []Request
}

// Empty reports a single-map game that never opened another map.
func (a *Atlas) Empty() bool { return a == nil || a.Next == 0 && len(a.Maps) == 0 && len(a.Queue) == 0 }

// Clone is a deep copy of a.
func (a *Atlas) Clone() *Atlas {
	c := &Atlas{Next: a.Next, Maps: slices.Clone(a.Maps), Queue: slices.Clone(a.Queue)}
	for i := range c.Maps {
		c.Maps[i].Cells = slices.Clone(c.Maps[i].Cells)
	}
	return c
}

// Meta is map id's row (nil: map 0 or not open).
func (a *Atlas) Meta(id uint32) *Meta {
	for i := range a.Maps {
		if a.Maps[i].ID == id {
			return &a.Maps[i]
		}
	}
	return nil
}

// Add registers a map of kind k on tile, opened at now, that the host
// built itself (a dev fixture): its new id.
func (a *Atlas) Add(k Kind, tile int, now uint64) uint32 {
	id := a.newID()
	a.add(id, k, tile, now)
	return id
}

// add files map id's row (ascending id).
func (a *Atlas) add(id uint32, k Kind, tile int, now uint64) *Meta {
	i, _ := slices.BinarySearchFunc(a.Maps, id, func(m Meta, id uint32) int { return int(m.ID) - int(id) })
	a.Maps = slices.Insert(a.Maps, i, Meta{ID: id, Kind: k, Tile: tile, Opened: now, ResumeDest: -1})
	return &a.Maps[i]
}

// newID hands out the next map id.
func (a *Atlas) newID() uint32 {
	a.Next = max(a.Next, 1)
	a.Next++
	return a.Next - 1
}

// Census is one hourly count of an open map.
type Census struct {
	// Ours counts our colonists, prisoners and tame animals; Hostiles the
	// standing hostiles; Ore the lump's ore cells left.
	Ours, Hostiles, Ore int
	// Exiting: a forming caravan still has members walking out.
	Exiting bool
}

// streak counts hourly results in a row: ok adds one (up to the streak
// needed), anything else resets it; true once closeStreak is reached.
func streak(n *uint8, ok bool) bool {
	if !ok {
		*n = 0
		return false
	}
	*n = min(*n+1, closeStreak)
	return *n >= closeStreak
}

// ShouldClose folds census c into m: true once nobody of ours has been on
// the map, nor walking out of it, for closeStreak hours in a row.
func ShouldClose(m *Meta, c Census) bool {
	return streak(&m.Empty, c.Ours == 0 && !c.Exiting)
}

// JobDone reports a temporary map's job finished on census c: no
// standing hostiles, and the fight won, the lump mined out, the
// settlement defeated, or (a plain site) nothing to do.
func JobDone(m *Meta, c Census) bool {
	switch {
	case m.Kind == Colony || c.Hostiles > 0:
		return false
	case m.Hostile:
		return true
	case len(m.Cells) > 0:
		return c.Ore == 0
	case m.Kind == Settlement:
		return m.Defeated
	}
	return true
}

// LumpCells picks n cells for a mineral lump on a w×h map: the free cells
// nearest a point east of the centre (the centre stays open ground to
// walk to), ring by ring (no randomness).
func LumpCells(w, h, n int, free func(x, y int) bool) []int {
	cx, cy := w/2+w/5, h/2
	var out []int
	for d := 0; len(out) < n && d <= max(w, h)/2; d++ {
		for y := cy - d; y <= cy+d && len(out) < n; y++ {
			for x := cx - d; x <= cx+d && len(out) < n; x++ {
				if (abs(x-cx) == d || abs(y-cy) == d) && x > 0 && y > 0 && x < w-1 && y < h-1 && free(x, y) {
					out = append(out, y*w+x)
				}
			}
		}
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Countdown is map id's pack-up ticks left at now (0: none running).
func (a *Atlas) Countdown(id uint32, now uint64) int {
	if m := a.Meta(id); m != nil && m.PackBy > now {
		return int(m.PackBy - now)
	}
	return 0
}

// exiting reports a forming caravan from map id with members still on it.
func (f Fleet) exiting(id uint32) bool {
	return slices.ContainsFunc(*f.list, func(c Caravan) bool { return c.Map == id && len(c.Exiting) > 0 })
}
