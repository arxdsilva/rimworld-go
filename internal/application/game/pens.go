package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Pens (PLAY-015 §2). A pen is the region around a pen marker that
// livestock can't walk out of on their own: a flood fill from the marker
// over the cells a penned animal may stand on, stopped by fences, gates,
// doors, walls, rock and deep water. A fill that reaches the map edge or
// grows past penMaxCells is "Not enclosed" and holds nothing. Each
// marker's fill is cached and redone lazily when the structure changes.
//
// Livestock (animal.Roster.Penned) live in the closest reachable enclosed
// pen accepting their species and sex; a handler ropes one that is
// outside it (rope.go). With no such pen they roam, and after
// penLeaveDays unpenned an animal far from Home may wander off the map.
// Every other tame animal keeps to its allowed area instead (areas.go).

const (
	// penMaxCells: a larger fill is open country, not a pen.
	penMaxCells = 2500
	// penThinkEvery: the pen bookkeeping pass (unpenned days, alerts,
	// auto-cut, leaving) runs on ticks divisible by this.
	penThinkEvery = 250
	// penAutoCutEvery: auto-cut designations are refreshed this often.
	penAutoCutEvery = 2 * clock.TicksPerHour
	// penRoamRadius: an unpenned animal wanders this far from its spot.
	penRoamRadius = 30
	// penLeaveDays: unpenned this long, an animal far from Home may leave;
	// penLeaveFar is "far" (cells, Chebyshev, to the nearest Home cell);
	// penLeavePct the daily chance.
	penLeaveDays = 3
	penLeaveFar  = 40
	penLeavePct  = 12
)

// penCache is one marker's last enclosure fill and what it was built at.
type penCache struct {
	structVer, fenceGen, gridVer uint64
	x, y                         int
	enclosed                     bool
	bits                         []uint64 // the pen's cells (nil when open)
	cells                        int
	minX, minY, maxX, maxY       int
}

// penState is the pens' derived state (fills, alert memory) and the
// unpenned-since ticks (saved as PenAnimals).
type penState struct {
	cache map[entity.EntityID]*penCache
	// markers: the standing pen markers, ascending, synced with g.pens at
	// syncVer/syncGen.
	markers         []entity.EntityID
	syncVer, syncGn uint64
	synced          bool
	ver             uint64 // WorldView.PenVer
	// was: each marker's enclosure at the last bookkeeping pass (the open
	// alert fires on a change from enclosed to open).
	was map[entity.EntityID]bool
	// since: tick each unpenned animal was first seen with no pen.
	since map[entity.EntityID]uint64
}

// penned reports tame livestock: kept in pens rather than areas.
func penned(a *animal.State) bool {
	return a.Tame && animal.RosterOf(a.Species).Penned
}

// defaultPenSettings: every penned species, both sexes; auto-cut trees,
// cacti and brambles.
func defaultPenSettings() PenSettings {
	var s PenSettings
	for _, sp := range animal.All() {
		if animal.RosterOf(sp).Penned {
			s.Species = append(s.Species, PenSpecies{Key: sp.String(), Allow: true, Male: true, Female: true})
		}
	}
	s.AutoCut = defaultAutoCut()
	return s
}

// defaultAutoCut is the auto-cut list a new pen starts with.
func defaultAutoCut() []string {
	var out []string
	for s := plant.Species(0); ; s++ {
		d := plant.DefOf(s)
		if d.Name == "" {
			break
		}
		if d.Trunk || s == plant.Bramble || s == plant.Cactus || s == plant.BallCactus {
			out = append(out, d.Name)
		}
	}
	return out
}

// penAccepts reports that settings s take an animal of species sp and sex
// female. A species the list doesn't name is taken.
func penAccepts(s PenSettings, sp animal.Species, female bool) bool {
	key := sp.String()
	for _, e := range s.Species {
		if e.Key == key {
			return e.Allow && (female && e.Female || !female && e.Male)
		}
	}
	return true
}

// penMarkers is the standing pen markers, ascending, with g.pens synced:
// new markers get default settings, removed ones lose theirs.
func (g *Game) penMarkers() []entity.EntityID {
	ps := &g.pen
	f := g.fences()
	if ps.synced && ps.syncVer == g.structVer && ps.syncGn == f.gen {
		return ps.markers
	}
	ps.synced, ps.syncVer, ps.syncGn = true, g.structVer, f.gen
	var ms []entity.EntityID
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.PenMarker {
			ms = append(ms, id)
		}
	}
	slices.Sort(ms)
	if !slices.Equal(ms, ps.markers) {
		ps.ver++
	}
	ps.markers = ms
	kept := g.pens[:0]
	for _, p := range g.pens {
		if _, ok := slices.BinarySearch(ms, p.Marker); ok {
			kept = append(kept, p)
		}
	}
	g.pens = kept
	for _, m := range ms {
		if g.penIndex(m) < 0 {
			g.pens = append(g.pens, penSave{Marker: m, Settings: defaultPenSettings()})
		}
	}
	slices.SortFunc(g.pens, func(a, b penSave) int { return int(a.Marker) - int(b.Marker) })
	for id := range ps.cache {
		if _, ok := slices.BinarySearch(ms, id); !ok {
			delete(ps.cache, id)
		}
	}
	return ms
}

// penIndex is marker m's entry in g.pens (-1: none).
func (g *Game) penIndex(m entity.EntityID) int {
	for i, p := range g.pens {
		if p.Marker == m {
			return i
		}
	}
	return -1
}

// penSettings is marker m's settings (defaults when unknown).
func (g *Game) penSettings(m entity.EntityID) PenSettings {
	if i := g.penIndex(m); i >= 0 {
		return g.pens[i].Settings
	}
	return defaultPenSettings()
}

// penName is "Pen N", N the marker's rank among the standing markers.
func (g *Game) penName(m entity.EntityID) string {
	ms := g.penMarkers()
	i, _ := slices.BinarySearch(ms, m)
	return fmt.Sprintf("Pen %d", i+1)
}

// penRegion is marker m's enclosure, refilled when the structure changed
// since the last fill (nil: m is no pen marker).
func (g *Game) penRegion(m entity.EntityID) *penCache {
	b := g.bldgs.Ref(m)
	if b == nil || b.Kind != construction.PenMarker {
		return nil
	}
	ps := &g.pen
	f := g.fences()
	gv := g.grid.Version()
	if c, ok := ps.cache[m]; ok && c.structVer == g.structVer && c.fenceGen == f.gen && c.gridVer == gv && c.x == b.X && c.y == b.Y {
		return c
	}
	c := g.fillPen(b.X, b.Y)
	c.structVer, c.fenceGen, c.gridVer = g.structVer, f.gen, gv
	if old, ok := ps.cache[m]; !ok || old.enclosed != c.enclosed || old.cells != c.cells || !slices.Equal(old.bits, c.bits) {
		ps.ver++
	}
	if ps.cache == nil {
		ps.cache = map[entity.EntityID]*penCache{}
	}
	ps.cache[m] = c
	return c
}

// penStand reports a cell a penned animal may stand on by itself: walkable
// (no wall, rock or deep water), and no fence, gate or door.
func (g *Game) penStand(x, y int, doors tileSet) bool {
	if !g.grid.Walkable(x, y) || g.fenceAt(x, y) != fenceNone {
		return false
	}
	return !doors.has(y*g.grid.Width() + x)
}

// fillPen floods from (x, y) over penStand cells (4-connected: no
// animal cuts a corner).
func (g *Game) fillPen(x, y int) *penCache {
	c := &penCache{x: x, y: y}
	w, h := g.grid.Width(), g.grid.Height()
	doors := g.buildingCells(construction.Door)
	if !g.grid.InBounds(x, y) || !g.penStand(x, y, doors) {
		return c
	}
	bits := make([]uint64, (w*h+63)/64)
	set := func(i int) { bits[i>>6] |= 1 << (uint(i) & 63) }
	has := func(i int) bool { return bits[i>>6]&(1<<(uint(i)&63)) != 0 }
	queue := []int32{int32(y*w + x)}
	set(y*w + x)
	c.minX, c.minY, c.maxX, c.maxY = x, y, x, y
	for qi := 0; qi < len(queue); qi++ {
		i := int(queue[qi])
		cx, cy := i%w, i/w
		if cx == 0 || cy == 0 || cx == w-1 || cy == h-1 || len(queue) > penMaxCells {
			return &penCache{x: x, y: y} // open: the edge, or open country
		}
		c.minX, c.maxX = min(c.minX, cx), max(c.maxX, cx)
		c.minY, c.maxY = min(c.minY, cy), max(c.maxY, cy)
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+d[0], cy+d[1]
			j := ny*w + nx
			if has(j) || !g.penStand(nx, ny, doors) {
				continue
			}
			set(j)
			queue = append(queue, int32(j))
		}
	}
	c.enclosed, c.bits, c.cells = true, bits, len(queue)
	return c
}

// inPenCells reports cell (x, y) inside pen c.
func (g *Game) inPenCells(c *penCache, x, y int) bool {
	if c == nil || !c.enclosed || !g.grid.InBounds(x, y) {
		return false
	}
	i := y*g.grid.Width() + x
	return c.bits[i>>6]&(1<<(uint(i)&63)) != 0
}

// penAt is the enclosed pen holding cell (x, y) (lowest marker first).
func (g *Game) penAt(x, y int) (entity.EntityID, *penCache) {
	for _, m := range g.penMarkers() {
		if c := g.penRegion(m); g.inPenCells(c, x, y) {
			return m, c
		}
	}
	return 0, nil
}

// homePen is the enclosed pen animal id stands in that accepts it (0:
// none).
func (g *Game) homePen(id entity.EntityID, a *animal.State, pos Position) (entity.EntityID, *penCache) {
	m, c := g.penAt(pos.X, pos.Y)
	if m == 0 || !penAccepts(g.penSettings(m), a.Species, animalFemale(id, *a)) {
		return 0, nil
	}
	return m, c
}

// penTarget is where penned animal id belongs: the pen it stands in when
// that accepts it, else the closest enclosed pen accepting it that a
// handler could lead it to (0: none).
func (g *Game) penTarget(id entity.EntityID, a *animal.State, pos Position) entity.EntityID {
	if m, _ := g.homePen(id, a, pos); m != 0 {
		return m
	}
	female := animalFemale(id, *a)
	q := g.animalPathQuery(true, false) // led: gates and doors open
	best, bestD := entity.EntityID(0), 1<<30
	for _, m := range g.penMarkers() {
		c := g.penRegion(m)
		if c == nil || !c.enclosed || !penAccepts(g.penSettings(m), a.Species, female) {
			continue
		}
		d := octileDist(pos.X, pos.Y, c.x, c.y)
		if d >= bestD || !g.layerReach(q, pos, c.x, c.y) {
			continue
		}
		best, bestD = m, d
	}
	return best
}

func octileDist(ax, ay, bx, by int) int {
	dx, dy := abs(ax-bx), abs(ay-by)
	return 10*max(dx, dy) + 4*min(dx, dy)
}

// anyEnclosedPen reports a standing enclosed pen.
func (g *Game) anyEnclosedPen() bool {
	for _, m := range g.penMarkers() {
		if c := g.penRegion(m); c != nil && c.enclosed {
			return true
		}
	}
	return false
}

// setPenSettings applies a SetPenSettings command.
func (g *Game) setPenSettings(marker entity.EntityID, enc string) {
	s, ok := DecodePenSettings(enc)
	if !ok {
		return
	}
	g.penMarkers()
	i := g.penIndex(marker)
	if i < 0 {
		return
	}
	g.pens[i].Settings = s
	g.pen.ver++
}

// penTick is the pen bookkeeping pass (phaseAnimals): unpenned days and
// leaving, the open-pen message, auto-cut. It runs on fixed ticks only,
// so a loaded game runs it exactly when the original would have.
func (g *Game) penTick(now uint64) {
	if now%penThinkEvery != 0 {
		return
	}
	ms := g.penMarkers()
	g.penOpenAlerts(ms)
	g.penUnpenned(now)
	if now%penAutoCutEvery == 0 {
		g.penAutoCut(ms)
	}
}

// penOpenAlerts posts "Pen N is open" when an enclosed pen opens.
func (g *Game) penOpenAlerts(ms []entity.EntityID) {
	ps := &g.pen
	if ps.was == nil {
		ps.was = map[entity.EntityID]bool{}
	}
	for _, m := range ms {
		c := g.penRegion(m)
		was, known := ps.was[m]
		if known && was && !c.enclosed {
			g.message(c.x, c.y, "%s is open: its animals can get out.", g.penName(m))
		}
		ps.was[m] = c.enclosed
	}
	for m := range ps.was {
		if _, ok := slices.BinarySearch(ms, m); !ok {
			delete(ps.was, m)
		}
	}
}

// penUnpenned keeps the unpenned-since ticks: an animal in an accepting
// pen, on a rope or with a pen to be roped to is penned (the count
// resets); any other penned animal counts from the first pass that saw
// it so. Once a day, an animal unpenned for penLeaveDays and far from
// Home may wander off.
func (g *Game) penUnpenned(now uint64) {
	ps := &g.pen
	for id := range ps.since {
		if a := g.animals.Ref(id); a == nil || !penned(a) {
			delete(ps.since, id)
		}
	}
	day := now%clock.TicksPerDay == 0
	for _, id := range g.animalIDs {
		a := g.animals.Ref(id)
		if a == nil || !penned(a) || a.Dead() {
			continue
		}
		pos, _ := g.positions.Get(id)
		if g.ropeHandler(id) != 0 || g.penTarget(id, a, pos) != 0 {
			delete(ps.since, id)
			continue
		}
		since, ok := ps.since[id]
		if !ok {
			if ps.since == nil {
				ps.since = map[entity.EntityID]uint64{}
			}
			ps.since[id] = now
			continue
		}
		if !day || a.Leaving || now-since < penLeaveDays*clock.TicksPerDay || !g.farFromHome(pos) {
			continue
		}
		if g.rng.IntN(100) >= penLeavePct {
			continue
		}
		ex, ey := g.nearestEdge(pos)
		g.startLeaving(id, ex, ey)
		g.message(pos.X, pos.Y, "%s wandered off.", g.animalName(id, a))
	}
}

// animalName is a tame animal's name, else its species.
func (g *Game) animalName(_ entity.EntityID, a *animal.State) string {
	if a.Name != "" {
		return a.Name
	}
	return "A " + a.Species.Label()
}

// farFromHome: no Home cell within penLeaveFar-1 of pos.
func (g *Game) farFromHome(pos Position) bool {
	w, h := g.grid.Width(), g.grid.Height()
	r := penLeaveFar - 1
	for y := max(0, pos.Y-r); y <= min(h-1, pos.Y+r); y++ {
		for x := max(0, pos.X-r); x <= min(w-1, pos.X+r); x++ {
			if g.inHome(y*w + x) {
				return false
			}
		}
	}
	return true
}

// nearestEdge is a walkable map-edge cell nearest pos (straight out to the
// closest side, nudged to a walkable cell).
func (g *Game) nearestEdge(pos Position) (int, int) {
	w, h := g.grid.Width(), g.grid.Height()
	x, y := pos.X, pos.Y
	switch min(pos.X, pos.Y, w-1-pos.X, h-1-pos.Y) {
	case pos.X:
		x = 0
	case pos.Y:
		y = 0
	case w - 1 - pos.X:
		x = w - 1
	default:
		y = h - 1
	}
	for r := 0; r < max(w, h); r++ {
		for d := -r; d <= r; d++ {
			for _, c := range [][2]int{{x + d, y}, {x, y + d}} {
				cx, cy := c[0], c[1]
				if (cx == 0 || cy == 0 || cx == w-1 || cy == h-1) && g.dryWalkable(cx, cy) {
					return cx, cy
				}
			}
		}
	}
	return x, y
}

// penAutoCut designates the plants of each enclosed pen's auto-cut list
// for cutting.
func (g *Game) penAutoCut(ms []entity.EntityID) {
	for _, m := range ms {
		c := g.penRegion(m)
		s := g.penSettings(m)
		if c == nil || !c.enclosed || len(s.AutoCut) == 0 {
			continue
		}
		for y := c.minY; y <= c.maxY; y++ {
			for x := c.minX; x <= c.maxX; x++ {
				if !g.inPenCells(c, x, y) || g.fogged(x, y) {
					continue
				}
				pid := g.plantAt(x, y)
				if pid == 0 {
					continue
				}
				p, ok := g.plants.Get(pid)
				if !ok || p.Cut || p.Zone || !slices.Contains(s.AutoCut, p.Species.String()) {
					continue
				}
				p.Cut, p.Designated = true, false
				g.notePlantMark(pid) // jobindex.go
				g.plants.Set(pid, p)
			}
		}
	}
}

// penFood is a pen's food figures (per day, in medium-animal graze units):
// what its grass and browse grow, what the animals assigned to it eat,
// and what its troughs hold.
func (g *Game) penFood(m entity.EntityID, c *penCache, herd []entity.EntityID) (graze, demand, stock int) {
	if c != nil && c.enclosed {
		for y := c.minY; y <= c.maxY; y++ {
			for x := c.minX; x <= c.maxX; x++ {
				if !g.inPenCells(c, x, y) {
					continue
				}
				// grass fills out over GrassFillTicks: a tile feeds about a
				// meal per fill, scaled by how grown it stands now
				graze += animal.GrazeFood * g.grassGrowthAt(x, y) / plant.GrassFull * clock.TicksPerDay / plant.GrassFillTicks
				if pid := g.plantAt(x, y); pid != 0 {
					if p, ok := g.plants.Get(pid); ok && plant.DefOf(p.Species).Browse && !p.Zone {
						graze += animal.BrowseFood * p.Percent() / 100 * clock.TicksPerDay / plant.GrassFillTicks
					}
				}
			}
		}
	}
	for _, id := range herd {
		if a := g.animals.Ref(id); a != nil {
			demand += animal.FoodPerDay * 100 / max(1, animal.Meal(a.Species, 100))
		}
	}
	for _, bid := range g.penTroughs(m, c) {
		for k, q := range g.troughs[bid].Food {
			stock += q * max(animal.FeedValue(animal.Herbivore, k), animal.FeedValue(animal.Carnivore, k))
		}
	}
	return graze, demand, stock
}

// penTroughs lists the troughs serving pen m: on or beside its cells.
func (g *Game) penTroughs(m entity.EntityID, c *penCache) []entity.EntityID {
	if c == nil || !c.enclosed {
		return nil
	}
	var out []entity.EntityID
	for _, bid := range g.bldgIDs {
		b, ok := g.isTrough(bid)
		if !ok {
			continue
		}
		if p, _ := g.troughMarker(b); p == m {
			out = append(out, bid)
		}
	}
	return out
}

// troughMarker is the enclosed pen trough b serves: the one holding its
// tile or a neighbouring cell (lowest marker first).
func (g *Game) troughMarker(b construction.Building) (entity.EntityID, *penCache) {
	for _, m := range g.penMarkers() {
		c := g.penRegion(m)
		if c == nil || !c.enclosed {
			continue
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if g.inPenCells(c, b.X+dx, b.Y+dy) {
					return m, c
				}
			}
		}
	}
	return 0, nil
}

// penHerds maps each pen to the penned animals assigned to it (in id
// order).
func (g *Game) penHerds() map[entity.EntityID][]entity.EntityID {
	out := map[entity.EntityID][]entity.EntityID{}
	if len(g.penMarkers()) == 0 {
		return out
	}
	for _, id := range g.animalIDs {
		a := g.animals.Ref(id)
		if a == nil || !penned(a) || a.Dead() {
			continue
		}
		pos, _ := g.positions.Get(id)
		if m := g.penTarget(id, a, pos); m != 0 {
			out[m] = append(out[m], id)
		}
	}
	return out
}

// penViews is the pens for the renderer, in marker order.
func (g *Game) penViews() []PenView {
	ms := g.penMarkers()
	if len(ms) == 0 {
		return nil
	}
	herds := g.penHerds()
	out := make([]PenView, 0, len(ms))
	for _, m := range ms {
		c := g.penRegion(m)
		if c == nil {
			continue
		}
		v := PenView{ID: m, X: c.x, Y: c.y, Name: g.penName(m), Enclosed: c.enclosed, Bits: c.bits,
			Settings: g.penSettings(m), Animals: len(herds[m])}
		v.GrazePerDay, v.DemandPerDay, v.TroughStock = g.penFood(m, c, herds[m])
		out = append(out, v)
	}
	return out
}

// fillPenView fills WorldView.Pens and PenVer.
func (g *Game) fillPenView(w *WorldView) {
	w.Pens = g.penViews()
	w.PenVer = g.pen.ver
}

// penLabel is a penned animal's Animals-tab pen line: its pen's name,
// "Being roped" or "Roaming".
func (g *Game) penLabel(id entity.EntityID, a *animal.State, pos Position) string {
	if g.ropeHandler(id) != 0 {
		return "Being roped"
	}
	if m, _ := g.homePen(id, a, pos); m != 0 {
		return g.penName(m)
	}
	return "Roaming"
}

// fillAnimalPen fills AnimalView.Penned and Pen.
func (g *Game) fillAnimalPen(av *AnimalView, id entity.EntityID, a *animal.State) {
	if !penned(a) {
		return
	}
	av.Penned = true
	av.Pen = g.penLabel(id, a, Position{X: av.X, Y: av.Y})
}

// exportPenAnimals is the unpenned-since ticks, by animal id.
func (g *Game) exportPenAnimals() []penAnimalSave {
	out := make([]penAnimalSave, 0, len(g.pen.since))
	for id, s := range g.pen.since {
		out = append(out, penAnimalSave{Animal: id, Since: s})
	}
	slices.SortFunc(out, func(a, b penAnimalSave) int { return int(a.Animal) - int(b.Animal) })
	return out
}

// importPens restores the pens' settings and unpenned-since ticks.
func (g *Game) importPens(markers []penSave, since []penAnimalSave) {
	g.pens = slices.Clone(markers)
	g.pen = penState{}
	for _, s := range since {
		if g.pen.since == nil {
			g.pen.since = map[entity.EntityID]uint64{}
		}
		g.pen.since[s.Animal] = s.Since
	}
}

// migratePenZones turns an older save's painted pens into pen markers
// (PLAY-015 §2.7): each pen zone gets a marker on the free walkable cell
// nearest its middle, with default settings, and its cells are unzoned.
// A zone with no fence around it becomes an open pen, with the alert.
func (g *Game) migratePenZones() {
	var pens []zone.Zone
	g.zones.Each(func(z zone.Zone) bool {
		if z.Kind == zone.KindPen {
			pens = append(pens, z)
		}
		return true
	})
	for _, z := range pens {
		cells := g.zones.Cells(z.ID)
		if len(cells) == 0 {
			continue
		}
		sx, sy := 0, 0
		for _, c := range cells {
			sx, sy = sx+c[0], sy+c[1]
		}
		mx, my := sx/len(cells), sy/len(cells)
		for _, c := range cells {
			g.zones.Clear(zone.RectOf(c[0], c[1], c[0], c[1]))
		}
		if m, ok := g.placePenMarker(mx, my); ok {
			g.markRoomsDirty()
			if c := g.penRegion(m); c != nil && !c.enclosed {
				g.message(c.x, c.y, "%s is open: its animals can get out.", g.penName(m))
			}
		}
	}
}

// placePenMarker builds a pen marker on the free walkable cell nearest
// (x, y).
func (g *Game) placePenMarker(x, y int) (entity.EntityID, bool) {
	w, h := g.grid.Width(), g.grid.Height()
	for r := 0; r < max(w, h); r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				cx, cy := x+dx, y+dy
				if !g.dryWalkable(cx, cy) || g.fenceAt(cx, cy) != fenceNone ||
					g.layerBldgAt(cx, cy, construction.LayerObject) != 0 || g.layerFrameAt(cx, cy, construction.LayerObject) != 0 {
					continue
				}
				if g.BuildNow(construction.DefOf(construction.PenMarker).Name, "wood", "south", cx, cy) {
					if id := g.layerBldgAt(cx, cy, construction.LayerObject); id != 0 {
						return id, true
					}
				}
			}
		}
		if r > 40 {
			break
		}
	}
	return 0, false
}
