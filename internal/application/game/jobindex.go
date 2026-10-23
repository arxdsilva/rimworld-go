package game

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Job-giver indexes (NG-1 §6.6, E4): the work searches an idle colonist
// runs every idleRecheck ticks look at a maintained candidate set instead
// of every plant, item or building on the map. Like the cell indexes
// (indexes.go) they are verified, not trusted: a set may hold ids that no
// longer qualify (dropped when met), never miss one that does, and every
// answer goes through the same filters, in the same order, as the scan it
// replaces, so side effects (lazily pruned postponements) are identical.
// Under the tests' index self-check each answer is compared with the
// original scan and a difference panics.

// jobIndexes are the job-giver structures (never saved).
type jobIndexes struct {
	// marks: plants with a harvest, cut or zone flag — the only ones
	// nearestPlant's wants (wantsHarvest, Cut, wantsZoneChop) can match.
	marks idSet
	// cands, scanCands: nearestPlant scratch.
	cands, scanCands []plantCand
	// loose, looseCells: stockpileCell scratch (first loose item per
	// stockpile tile, and the tiles written).
	loose      []entity.EntityID
	looseCells []int32
	// plantGen counts changes to the plant list (spawnPlant, removePlant,
	// the frost pass): the food index keeps its plant grid while it holds.
	plantGen uint64
}

// plantsChanged records a plant added to or removed from the lists.
func (g *Game) plantsChanged() { g.idx.jobs.plantGen++ }

// idSet is an insertion-ordered set of ids built from one store.
type idSet struct {
	ids   []entity.EntityID
	in    map[entity.EntityID]struct{}
	src   any
	built bool
}

func (s *idSet) reset(src any) {
	s.ids, s.src, s.built = s.ids[:0], src, true
	if s.in == nil {
		s.in = map[entity.EntityID]struct{}{}
	} else {
		clear(s.in)
	}
}

func (s *idSet) add(id entity.EntityID) {
	if _, ok := s.in[id]; !ok {
		s.in[id] = struct{}{}
		s.ids = append(s.ids, id)
	}
}

// keep drops every id ok rejects (in place, order kept).
func (s *idSet) keep(ok func(entity.EntityID) bool) {
	kept := s.ids[:0]
	for _, id := range s.ids {
		if ok(id) {
			kept = append(kept, id)
		} else {
			delete(s.in, id)
		}
	}
	clear(s.ids[len(kept):])
	s.ids = kept
}

// warmIndexes builds what a loaded or newly set-up game would otherwise
// build on its first tick (E4: no first-tick spike): the cell indexes,
// the region map and, with rooms, the rooms. The room build is not
// history-free (new rooms take over the temperatures of the rooms they
// replace), so a colony that is still being built by hand (NewColony,
// then dev hooks: fixtures, tests) leaves it to its first tick as before.
func (g *Game) warmIndexes(rooms bool) {
	g.plantIndex()
	g.bldgIndex()
	g.frameIndex()
	g.plantMarks()
	if n := g.grid.Width() * g.grid.Height(); len(g.idx.jobs.loose) != n {
		g.idx.jobs.loose = make([]entity.EntityID, n) // stockpileCell's scratch
	}
	if rooms {
		g.ensureRooms()
	}
	if !g.noRegions() {
		g.regionsNow().Update() // flood and label now: the first query would
	}
}

// ---- plant marks (nearestPlant) ----

func plantMarked(p plant.Plant) bool { return p.Designated || p.Cut || p.Zone }

// plantMarks is the marked-plant set, built from the lists on first use
// and whenever the plant store was replaced (Load).
func (g *Game) plantMarks() *idSet {
	s := &g.idx.jobs.marks
	if !s.built || s.src != g.plants {
		s.reset(g.plants)
		for _, id := range g.plantIDs {
			if p, ok := g.plants.Get(id); ok && plantMarked(p) {
				s.add(id)
			}
		}
	}
	return s
}

// notePlantMark records that plant id may have gained a harvest, cut or
// zone flag (every place that sets one calls it).
func (g *Game) notePlantMark(id entity.EntityID) {
	if s := &g.idx.jobs.marks; s.built && s.src == g.plants {
		s.add(id)
	}
}

type plantCand struct {
	id entity.EntityID
	d  int
}

func cmpPlantCand(a, b plantCand) int {
	if c := cmp.Compare(a.d, b.d); c != 0 {
		return c
	}
	return cmp.Compare(a.id, b.id)
}

// plantCands is nearestPlant's candidate list, nearest first (ties: lower
// id), from the marked set.
func (g *Game) plantCands(id entity.EntityID, k job.Kind, want func(plant.Plant) bool, pos Position) []plantCand {
	s := g.plantMarks()
	cs := g.idx.jobs.cands[:0]
	stale := false
	for _, pid := range s.ids {
		p, ok := g.plants.Get(pid)
		if !ok || !plantMarked(p) {
			stale = true
			continue
		}
		if !want(p) {
			continue
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			continue
		}
		if g.postponedFor(id, k, pid, 0, 0) {
			continue
		}
		cs = append(cs, plantCand{pid, pathing.Dist(pos.X, pos.Y, p.X, p.Y)})
	}
	if stale {
		s.keep(func(pid entity.EntityID) bool {
			p, ok := g.plants.Get(pid)
			return ok && plantMarked(p)
		})
	}
	slices.SortFunc(cs, cmpPlantCand)
	g.idx.jobs.cands = cs
	return cs
}

// plantCandsScan is plantCands over every plant (the original search).
func (g *Game) plantCandsScan(id entity.EntityID, k job.Kind, want func(plant.Plant) bool, pos Position) []plantCand {
	cs := g.idx.jobs.scanCands[:0]
	for _, pid := range g.plantIDs {
		p, ok := g.plants.Get(pid)
		if !ok || !want(p) {
			continue
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			continue
		}
		if g.postponedFor(id, k, pid, 0, 0) {
			continue
		}
		cs = append(cs, plantCand{pid, pathing.Dist(pos.X, pos.Y, p.X, p.Y)})
	}
	slices.SortFunc(cs, cmpPlantCand)
	g.idx.jobs.scanCands = cs
	return cs
}

// checkPlantCands panics when the indexed candidates differ from the scan.
func (g *Game) checkPlantCands(id entity.EntityID, k job.Kind, want func(plant.Plant) bool, pos Position, cs []plantCand) {
	scan := g.plantCandsScan(id, k, want, pos)
	if !slices.Equal(cs, scan) {
		panic(fmt.Sprintf("nearestPlant(%d, %v): index %v, scan %v", id, k, cs, scan))
	}
}

// ---- stockpiles ----

// looseOnStockpiles fills the per-tile scratch with the first loose
// (uncarried) item on every stockpile tile, in item-list order (as
// itemAt), and returns it; releaseLoose clears it again.
func (g *Game) looseOnStockpiles() []entity.EntityID {
	jx := &g.idx.jobs
	w, n := g.grid.Width(), g.grid.Width()*g.grid.Height()
	if len(jx.loose) != n {
		jx.loose, jx.looseCells = make([]entity.EntityID, n), jx.looseCells[:0]
	}
	for _, iid := range g.itemIDs {
		st, has := g.stacks.Get(iid)
		if !has || st.Carrier != 0 {
			continue
		}
		if _, zoned := g.zoneOf(zone.KindStockpile, st.X, st.Y); !zoned {
			continue
		}
		c := st.Y*w + st.X
		if uint(c) < uint(n) && jx.loose[c] == 0 {
			jx.loose[c] = iid
			jx.looseCells = append(jx.looseCells, int32(c))
		}
	}
	return jx.loose
}

func (g *Game) releaseLoose() {
	jx := &g.idx.jobs
	for _, c := range jx.looseCells {
		jx.loose[c] = 0
	}
	jx.looseCells = jx.looseCells[:0]
}

// ---- animals ----

// anyAnimal reports whether some animal in the store passes ok: a quick
// pass over the store's dense values (no id lookups) that lets the animal
// job givers (hunt, tame, collect) skip their search when nothing is
// marked or ready — the common case. The store holds every animal those
// searches can find, so "none here" means they would find none.
func (g *Game) anyAnimal(ok func(entity.EntityID, animal.State) bool) bool {
	found := false
	g.animals.All(func(id entity.EntityID, a animal.State) {
		found = found || ok(id, a)
	})
	return found
}

// ---- beds ----

// sleepLayers are the construction layers a sleeping place can be on.
var sleepLayers = func() []construction.Layer {
	var out []construction.Layer
	for _, k := range []construction.Kind{construction.Bed, construction.SleepingSpot, construction.DoubleBed, construction.HospitalBed} {
		if l := layerOf(k); !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out
}()

// bedUnderIndexed is bedUnder from the building cell index: the sleeping
// place claimed by id that covers its tile.
func (g *Game) bedUnderIndexed(id entity.EntityID, pos Position) entity.EntityID {
	lx := g.bldgIndex()
	var found entity.EntityID
	for _, l := range sleepLayers {
		bid, scan := lx.at(pos.X, pos.Y, l)
		if scan {
			return g.bedUnderScan(id, pos)
		}
		if bid == 0 {
			continue
		}
		if b, _ := g.bldgs.Get(bid); b.Kind.Sleeps() && b.SlotOf(uint64(id)) >= 0 {
			if found != 0 {
				return g.bedUnderScan(id, pos) // two: the list order decides
			}
			found = bid
		}
	}
	return found
}

// bedCoveringIndexed is bedCovering from the building cell index: some
// sleeping place covers (x, y).
func (g *Game) bedCoveringIndexed(x, y int) bool {
	lx := g.bldgIndex()
	for _, l := range sleepLayers {
		bid, scan := lx.at(x, y, l)
		if scan {
			return g.bedCoveringScan(x, y)
		}
		if b, ok := g.bldgs.Get(bid); bid != 0 && ok && b.Kind.Sleeps() {
			return true
		}
	}
	return false
}

func (g *Game) bedUnderScan(id entity.EntityID, pos Position) entity.EntityID {
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Kind.Sleeps() || b.SlotOf(uint64(id)) < 0 {
			continue
		}
		if construction.Covers(b.Kind, b.Facing, b.X, b.Y, pos.X, pos.Y) {
			return bid
		}
	}
	return 0
}
