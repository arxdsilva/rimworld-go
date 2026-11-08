package travel

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/pkg/mathx"
	"rimworld-go/pkg/randx"
)

// The long-range mineral scanner (wiki: Long-range mineral scanner): a
// researcher works it on power from its console (job.ScanMinerals,
// Research work). A find comes after 1 to 480000 work units (hashed:
// mean 240000, the wiki's 4 days; guaranteed by 8), the operator's work
// units standing in for Research Speed. A find is a world mining site 6
// to 25 tiles from the colony with a lump of the scanner's target
// mineral (any by default, by weight), of the wiki's size; it expires
// after 30 days. A caravan or pods visiting it mine it (Mine).
const (
	ScanMax     = 480000                 // wiki: guaranteed within 8 days
	SiteMinDist = 6                      // wiki
	SiteMaxDist = 25                     // wiki
	SiteDays    = 30                     // wiki: the site's quest expires
	scanSitting = 2 * clock.TicksPerHour // one sitting (orig: the research bench's)
)

// Mineral is one mineral a scanner finds: its lump's size (wiki) and how
// often it comes up with no target set (original weights).
type Mineral struct {
	Kind     item.Kind
	Min, Max int
	weight   int
}

// Minerals are the scanner's targets, in the target cycle's order.
func Minerals() []Mineral {
	return []Mineral{
		{item.Steel, 2200, 2400, 30}, {item.Plasteel, 400, 600, 10}, {item.Components, 100, 140, 15},
		{item.Silver, 2400, 3200, 15}, {item.Gold, 320, 480, 10}, {item.Uranium, 600, 800, 10}, {item.Jade, 700, 875, 10},
	}
}

// hashN is the next number below n of the stream at *v.
func hashN(v *uint64, n int) int {
	*v = randx.SplitMix64(*v)
	return int(*v % uint64(max(1, n)))
}

// scanTarget is the work the next find takes.
func (o scanner) scanTarget() int {
	v := o.h.Seed() ^ uint64(o.s.finds+1)*0x9e3779b97f4a7c15 ^ 0x5ca9
	return 1 + hashN(&v, ScanMax)
}

// Cycle is scanner id's progress to its next find, permille.
func (o scanner) Cycle(id entity.EntityID) int {
	return min(999, o.s.scans[id].Work*1000/o.scanTarget())
}

// ScanFor is scanner id's target (0: any).
func (o scanner) ScanFor(id entity.EntityID) item.Kind { return o.s.scans[id].For }

// cmdTarget sets scanner Target's mineral to Stuff (0 or not a mineral:
// any).
func (o scanner) cmdTarget(c command.Command) {
	id := entity.EntityID(c.Target)
	if b, ok := o.h.Building(id); !ok || b.Kind != construction.MineralScanner {
		return
	}
	sc := o.s.scans[id]
	sc.For = 0
	if slices.ContainsFunc(Minerals(), func(m Mineral) bool { return m.Kind == item.Kind(c.Stuff) }) {
		sc.For = item.Kind(c.Stuff)
	}
	o.s.setScan(id, sc)
	o.h.Changed()
}

// AssignScan sends researcher id to the nearest usable scanner, trying
// the next when one can't be started, at most tries scanners.
func (o scanner) AssignScan(id entity.EntityID, tries int) bool {
	var tried []entity.EntityID
	for range tries {
		px, py := o.h.Pos(id)
		var best entity.EntityID
		var bx, by int
		bestD := 1 << 30
		for _, sid := range o.h.BuildingIDs() {
			b, ok := o.h.Building(sid)
			if !ok || b.Kind != construction.MineralScanner || b.Deconstruct || b.Forbidden || slices.Contains(tried, sid) ||
				!o.h.Powered(sid) {
				continue
			}
			_, spot, ok := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
			if !ok || !o.h.Free(id, sid, job.ScanMinerals, spot[0], spot[1]) {
				continue
			}
			if d := mathx.Abs(px-spot[0]) + mathx.Abs(py-spot[1]); d < bestD || d == bestD && sid < best {
				best, bestD, bx, by = sid, d, spot[0], spot[1]
			}
		}
		if best == 0 {
			return false
		}
		if o.h.Start(id, best, job.ScanMinerals, bx, by, scanSitting) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

// doScan is one tick of a scanner sitting.
func (o scanner) doScan(id entity.EntityID, j job.Record) {
	b, ok := o.h.Building(j.Target)
	switch {
	case !ok || b.Kind != construction.MineralScanner:
		o.end(id, j, 0)
		return
	case !o.h.Powered(j.Target):
		o.end(id, j, job.Blocked) // the power went: put off a while (the retry cooldown)
		return
	}
	if u := o.h.Work(id, false); u > 0 {
		sc := o.s.scans[j.Target]
		if sc.Work += u; sc.Work >= o.scanTarget() {
			sc.Work = 0
			o.find(sc.For, b)
		}
		o.s.setScan(j.Target, sc)
	}
	if j.WorkLeft--; j.WorkLeft <= 0 {
		o.end(id, j, 0)
		return
	}
	o.h.SetJob(id, j)
}

// find lays the next mineral site (target 0: any mineral, by weight):
// false when no tile in reach would do.
func (o scanner) find(target item.Kind, b construction.Building) bool {
	p, home := o.h.Planet(), o.h.ColonyTile()
	o.s.finds++
	if p == nil || home < 0 {
		return false
	}
	v := o.h.Seed() ^ uint64(o.s.finds)*0xd1b54a32d192ed03 ^ 0x51e5
	ms := Minerals()
	m := ms[0]
	if k := slices.IndexFunc(ms, func(m Mineral) bool { return m.Kind == target }); k >= 0 {
		m = ms[k]
	} else {
		total := 0
		for _, c := range ms {
			total += c.weight
		}
		r := hashN(&v, total)
		for _, c := range ms {
			if r -= c.weight; r < 0 {
				m = c
				break
			}
		}
	}
	var cands []int
	for _, t := range ring(p, home) {
		if _, settled, _, _ := o.settlement(t); settled || !p.Land(t) {
			continue
		}
		if _, taken := o.siteAt(t); !taken {
			cands = append(cands, t)
		}
	}
	if len(cands) == 0 {
		return false
	}
	t := cands[hashN(&v, len(cands))]
	s := Site{Tile: t, Mineral: m.Kind, Qty: m.Min + hashN(&v, m.Max-m.Min+1), Expires: o.h.Now() + SiteDays*clock.TicksPerDay}
	o.s.sites = append(o.s.sites, s)
	if !o.h.ScannerFound(t) {
		o.h.Letter("Mineral site found", b.X, b.Y, "The long-range scanner found a lump of %d %s on tile %d. "+
			"Send a caravan or transport pods within %d days to mine it.", s.Qty, s.Mineral, t, SiteDays)
	}
	o.h.Changed()
	return true
}

// ring is every tile SiteMinDist to SiteMaxDist hops from home, in
// breadth-first order (deterministic: the planet's neighbour order).
func ring(p *planet.Planet, home int) []int { return ringOf(p, home, SiteMinDist, SiteMaxDist, false) }

// ringOf is every tile lo to hi hops from home, in breadth-first order;
// overland walks only through land, so each tile has a caravan route home
// (planet.Route crosses no water).
func ringOf(p *planet.Planet, home, lo, hi int, overland bool) []int {
	seen := map[int32]bool{int32(home): true}
	front, out := []int32{int32(home)}, []int(nil)
	for d := 1; d <= hi && len(front) > 0; d++ {
		var next []int32
		for _, t := range front {
			for _, nb := range p.Neighbours(int(t)) {
				if !seen[nb] && (!overland || p.Land(int(nb))) {
					seen[nb] = true
					next = append(next, nb)
					if d >= lo {
						out = append(out, int(nb))
					}
				}
			}
		}
		front = next
	}
	return out
}

// FreeTile is a land tile lo..hi overland hops from home (a caravan can
// reach it and come back, QST-059) with no settlement, no mineral site and
// not taken, picked from seed (a quest site, QST-117).
func (o Ops) FreeTile(seed uint64, lo, hi int, taken func(t int) bool) (int, bool) {
	p, home := o.h.Planet(), o.h.ColonyTile()
	if p == nil || home < 0 {
		return 0, false
	}
	var cands []int
	for _, t := range ringOf(p, home, lo, hi, true) {
		_, settled, _, _ := o.settlement(t)
		if _, site := scanner(o).siteAt(t); !settled && !site && p.Land(t) && !taken(t) {
			cands = append(cands, t)
		}
	}
	if len(cands) == 0 {
		return 0, false
	}
	return cands[seed%uint64(len(cands))], true
}

// siteAt is the mineral site on tile t.
func (o scanner) siteAt(t int) (Site, bool) {
	for _, s := range o.s.sites {
		if s.Tile == t {
			return s, true
		}
	}
	return Site{}, false
}

// Deposit is the mineral site on tile t as its lump (quests, QST-116).
func (o Ops) Deposit(t int) (item.Kind, int, bool) {
	s, ok := scanner(o).siteAt(t)
	return s.Mineral, s.Qty, ok
}

// siteTick drops expired sites.
func (o scanner) siteTick(now uint64) {
	o.s.sites = slices.DeleteFunc(o.s.sites, func(s Site) bool { return now >= s.Expires })
}

// Find lays a mineral site now, as a scanner's find does (dev tools and
// tests); false when no tile would do.
func (o scanner) Find(target item.Kind) bool {
	hx, hy := o.h.Home()
	return o.find(target, construction.Building{X: hx, Y: hy})
}
