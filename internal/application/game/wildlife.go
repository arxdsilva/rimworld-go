package game

import (
	"slices"
	"sort"
	"strings"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
)

// Wildlife (SIM-019, SIM-025). Which species live on the map follows the
// season and their habitat: cold-climate animals come in winter,
// hot-climate ones in summer. Start herds and later arrivals are drawn by
// those weights from g.rng (deterministic); how many live on the map
// follows its carrying capacity (population.go). Rarer events: a
// legendary animal wanders in at most once a year, explosive animals
// burst when they die, and hive swarms come out of the rock (hive.go).

// wildlifeState is the saved wildlife event state.
type wildlifeState struct {
	// LegendaryYear is 1 + the year a legendary last wandered in (0: never).
	LegendaryYear int
	// Bursts are explosive deaths of the last hour (presentation).
	Bursts []burstStamp
	// LastHerd is the last herd id given out (SIM-024: herds bed down,
	// leave and migrate together); CropRaids the crops wild animals ate
	// in the last hour (animal_needs.go).
	LastHerd  uint32
	CropRaids []cropRaid
	// Population (population.go, SIM-025): Count is the wild count of the
	// in-game hour CountHour-1 (0: not counted yet); PopSettle the tick
	// before which no arrival or departure follows the last one; Departs
	// the herds walking off an overfull map; Gnaw the timbergnaws of the
	// current gnawing event (incidents.go).
	Count     int
	Preds     int // predators among Count
	CountHour uint64
	PopSettle uint64
	Departs   []departure
	Gnaw      gnawEvent

	// idx and food are the per-tick creature and food indexes
	// (animal_index.go): derived, never saved.
	idx  creatureIndex
	food foodIndex
	ids  []entity.EntityID // phaseAnimals' scratch copy of animalIDs
}

type burstStamp struct {
	X, Y    int
	Tick    uint64
	Species animal.Species
	Turret  bool // a destroyed sentry turret's blast (turret.go, PLAY-024)
	Blast   explosion.Kind
	Radius  int // DEF-2: an explosion's kind and radius (tenths of a tile)
}

// BurstView is a recent explosive death, for the renderer's flash and
// scorch (kept an hour, like roof collapses).
type BurstView struct {
	X, Y    int
	Tick    uint64
	Species string
	// Turret: a destroyed sentry turret's blast (PLAY-024); Species is "".
	// Blast names an explosion ("bomb", "flame", "EMP", "smoke"; DEF-2)
	// of Radius tenths of a tile; "" otherwise.
	Turret bool
	Blast  string
	Radius int
}

const (
	// startPredatorMax caps a starting predator herd so a new colony
	// meets a pair, not a whole pack.
	startPredatorMax = 2
	// packRadius: pack mates this close join a hunt.
	packRadius = 10
	// legendaryRadius: legendaries this close answer one's rage.
	legendaryRadius = 30
	// burstDamage is a burst's damage at an adjacent tile; diagonal
	// neighbours take burstFalloff less (Euclidean 1.41 of 1.5 tiles).
	burstDamage  = 15
	burstFalloff = 5
	burstKeep    = clock.TicksPerHour
	// The legendary visit: from legendaryFromDay, a legendaryChance
	// percent roll once a day at legendaryHour, at most once a year.
	legendaryFromDay = 20
	legendaryHour    = 10
	legendaryChance  = 2
)

// Season weights in permille.
const (
	weightFavoured = 3000
	weightNormal   = 1000
	weightShunned  = 300
	weightAnyHab   = 600
)

// habitatWeight is how welcome one habitat is in season.
func habitatWeight(h animal.Habitat, season string) int {
	switch h {
	case animal.HabCold:
		switch season {
		case "Winter":
			return weightFavoured
		case "Summer":
			return weightShunned
		}
	case animal.HabHot:
		switch season {
		case "Summer":
			return weightFavoured
		case "Winter":
			return weightShunned
		}
	}
	return weightNormal
}

// seasonWeight is how much season favours species s (permille): the mean
// over its habitats, an any-climate species at 0.6.
func seasonWeight(s animal.Species, season string) int {
	r := animal.RosterOf(s)
	if r.Habitat == animal.HabAny {
		return weightAnyHab
	}
	w, n := 0, 0
	for _, h := range []animal.Habitat{animal.HabTemperate, animal.HabCold, animal.HabHot} {
		if r.Habitat&h != 0 {
			w += habitatWeight(h, season)
			n++
		}
	}
	if n == 0 {
		return weightNormal
	}
	return w / n
}

// wildWeight is species s's chance to be drawn as a wild herd in biome b
// in season: how common it is there (animal.Weight; 0 for domestic, pet,
// event and hive animals and for species of other biomes) times the
// season's welcome.
func wildWeight(s animal.Species, b biome.ID, season string) int {
	return animal.Weight(s, b) * seasonWeight(s, season)
}

// pickSpecies draws one species of the map's biome passing keep, weighted
// for the current season; ok is false when none qualifies.
func (g *Game) pickSpecies(keep func(animal.Species) bool) (animal.Species, bool) {
	season := g.season(uint64(g.clock.Now())) // setup.go: the map's own calendar
	// Setup games never draw domestic or pet species wild; legacy colonies
	// keep their draws (and RNG sequence) exactly.
	setup := g.hasSetup()
	weight := func(s animal.Species) int {
		if setup && !isWildSpecies(s) {
			return 0
		}
		return wildWeight(s, g.biome, season)
	}
	total := 0
	for _, s := range animal.All() {
		if keep(s) {
			total += weight(s)
		}
	}
	if total <= 0 {
		return 0, false
	}
	roll := g.rng.IntN(total)
	for _, s := range animal.All() {
		if !keep(s) {
			continue
		}
		if roll -= weight(s); roll < 0 {
			return s, true
		}
	}
	return 0, false
}

func anySpecies(animal.Species) bool { return true }

func isPredator(s animal.Species) bool { return animal.DefOf(s).Predator }

func isPrey(s animal.Species) bool { return !animal.DefOf(s).Predator }

// herdSizeOf draws a herd size for s.
func (g *Game) herdSizeOf(s animal.Species) int {
	r := animal.RosterOf(s)
	return r.HerdMin + g.rng.IntN(r.HerdMax-r.HerdMin+1)
}

// spawnHerd places n wild animals of s around (x, y) as one herd: each
// with a random sex, an age from 0.2 to 0.8 of its lifespan (at least one
// grown), and food and rest between 60 and 100 percent.
func (g *Game) spawnHerd(s animal.Species, x, y, n int) []entity.EntityID {
	return g.spawnHerdIn(nil, s, x, y, n)
}

// spawnHerdIn is spawnHerd with its members kept on open habitat cells
// (wildlife_habitat.go) when h is set; h nil is the legacy placement.
func (g *Game) spawnHerdIn(h *habitat, s animal.Species, x, y, n int) []entity.EntityID {
	var out []entity.EntityID
	g.wl.LastHerd++
	herd := g.wl.LastHerd
	life := animal.LifeOf(s)
	now := uint64(g.clock.Now())
	for i := 0; i < n; i++ {
		for try := 0; try < 12; try++ {
			hx, hy := x+g.rng.IntN(5)-2, y+g.rng.IntN(5)-2
			if h != nil && !h.open(hx, hy) {
				continue
			}
			id, err := g.spawnAnimal(s, hx, hy)
			if err != nil {
				continue
			}
			a, _ := g.animals.Get(id)
			lo, hi := life.LifeDays*2/10, life.LifeDays*8/10
			days := lo + g.rng.IntN(max(hi-lo, 0)+1)
			if i == 0 {
				days = max(days, life.AdultDays) // a herd has a grown leader
			}
			a.Born = animal.BornAt(now, uint64(days)*clock.TicksPerDay)
			a.Female = g.rng.IntN(2) == 0
			a.Food = animal.Full * (60 + g.rng.IntN(41)) / 100
			a.Rest = animal.Full * (60 + g.rng.IntN(41)) / 100
			a.Herd = herd
			g.animals.Set(id, a)
			out = append(out, id)
			break
		}
	}
	return out
}

// spawnWildlife places 4-6 season-weighted herds away from the start
// site, at least one of prey and, when the season has any, one of
// predators (kept to a pair, farther out, placed right after the first
// herd), then more herds until the map
// holds about startCapPct of its capacity; no herd takes the count past
// that target except the first.
func (g *Game) spawnWildlife() {
	g.wild = true
	target := g.startTarget()
	n := startHerdsMin + g.rng.IntN(startHerdsMax-startHerdsMin+1)
	picks := make([]animal.Species, 0, n)
	for i := 0; i < n; i++ {
		if s, ok := g.pickSpecies(anySpecies); ok {
			picks = append(picks, s)
		}
	}
	has := func(f func(animal.Species) bool) bool {
		for _, s := range picks {
			if f(s) {
				return true
			}
		}
		return false
	}
	if len(picks) > 0 && !has(isPrey) {
		if s, ok := g.pickSpecies(isPrey); ok {
			picks[0] = s
		}
	}
	if len(picks) > 1 && !has(isPredator) {
		if s, ok := g.pickSpecies(isPredator); ok {
			picks[len(picks)-1] = s
		}
	}
	// the first herd, then the predators, then the rest: a small map's
	// target is not used up before the predators are placed
	if len(picks) > 1 {
		rest := append([]animal.Species(nil), picks[1:]...)
		sort.SliceStable(rest, func(i, j int) bool { return isPredator(rest[i]) && !isPredator(rest[j]) })
		copy(picks[1:], rest)
	}
	count, preds := 0, 0
	var hab *habitat
	if g.hasSetup() {
		hab = g.newHabitat() // wildlife_habitat.go: Setup games place by habitat
	}
	place := func(s animal.Species, room int) {
		size, r := min(g.herdSizeOf(s), room), wildStartRadius
		if isPredator(s) {
			size, r = min(size, startPredatorMax), wolfStartRadius
		}
		var x, y int
		var ok bool
		if hab != nil {
			x, y, ok = hab.pick(s)
		} else {
			x, y, ok = g.randomTileAway(r)
		}
		if ok {
			k := len(g.spawnHerdIn(hab, s, x, y, size))
			count += k
			if isPredator(s) {
				preds += k
			}
		}
	}
	for i, s := range picks {
		room := target - count
		if i == 0 {
			room = max(room, animal.RosterOf(s).HerdMin)
		}
		if room > 0 {
			place(s, room)
		}
	}
	for tries := 0; count < target && tries < 4*target; tries++ {
		if s, ok := g.pickWildGroup(count, preds); ok {
			place(s, target-count)
		}
	}
}

// migrate is the wildlife's upkeep: a legendary may wander in
// (legendaryVisit) and, every hour, the population is checked against the
// map's capacity (population.go). It replaced the dawn top-up that
// brought a prey herd in whenever fewer than six grazers were left.
func (g *Game) migrate(now uint64) {
	if !g.wild {
		return
	}
	if g.legendaryDue(now) {
		g.legendaryVisit(now, g.rng.IntN(100))
	}
	if now%uint64(clock.TicksPerHour) == 0 {
		g.populationHour(now)
	}
}

// mapEdgeTile is a random walkable tile one in from a map edge.
func (g *Game) mapEdgeTile() (int, int, bool) {
	w, h := g.grid.Width(), g.grid.Height()
	for try := 0; try < 40; try++ {
		var x, y int
		switch g.rng.IntN(4) {
		case 0:
			x, y = g.rng.IntN(w), 1
		case 1:
			x, y = g.rng.IntN(w), h-2
		case 2:
			x, y = 1, g.rng.IntN(h)
		default:
			x, y = w-2, g.rng.IntN(h)
		}
		if g.dryWalkable(x, y) {
			return x, y, true
		}
	}
	return 0, 0, false
}

// legendaryDue: the once-a-day moment a legendary may arrive.
func (g *Game) legendaryDue(now uint64) bool {
	day := int(now / clock.TicksPerDay)
	return g.wild && day >= legendaryFromDay && now%uint64(clock.TicksPerDay) == uint64(legendaryHour*clock.TicksPerHour) &&
		g.wl.LegendaryYear != day/clock.DaysPerYear+1
}

// legendaryVisit brings a glimmerstag in at a map edge when roll (a
// percent) is under legendaryChance; at most once a year.
func (g *Game) legendaryVisit(now uint64, roll int) {
	if !g.legendaryDue(now) || roll >= legendaryChance {
		return
	}
	x, y, ok := g.mapEdgeTile()
	if !ok {
		return
	}
	if _, err := g.spawnAnimal(animal.Glimmerstag, x, y); err != nil {
		return
	}
	g.wl.LegendaryYear = int(now/clock.TicksPerDay)/clock.DaysPerYear + 1
	g.message(x, y, "A glimmerstag wanders in.")
}

// rallyLegendaries: a legendary's rage calls every wild legendary within
// legendaryRadius to the same fight.
func (g *Game) rallyLegendaries(id entity.EntityID, who entity.EntityID, now uint64) {
	pos, _ := g.positions.Get(id)
	g.within(pos.X, pos.Y, legendaryRadius, ckAnimal, func(aid entity.EntityID, _ Position) {
		if aid == id {
			return
		}
		o, ok := g.animals.Get(aid)
		if !ok || o.Tame || o.Enraged(now) || !animal.DefOf(o.Species).Legendary {
			return
		}
		o.Manhunter = now + animal.ManhunterTicks
		o.Mode, o.Target, o.TameMark, o.FleeUntil, o.Path, o.PathI = animal.Attack, who, false, 0, nil, 0
		g.animals.Set(aid, o)
		g.logEvent(aid, logEntry{Kind: logManhunter, Other: who})
	})
}

func chebyshev(a, b Position) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return max(dx, dy)
}

// burst: an explosive animal dying at pos hurts every creature within 1.5
// tiles (15 beside it, 10 diagonally) and leaves a stamp for the renderer;
// its cell and the burnable cells around it catch fire (fire.go).
func (g *Game) burst(s animal.Species, pos Position) {
	now := uint64(g.clock.Now())
	g.wl.Bursts = append(g.wl.Bursts, burstStamp{X: pos.X, Y: pos.Y, Tick: now, Species: s})
	g.message(pos.X, pos.Y, "The %s burst into flames!", s.Label())
	g.igniteBurst(pos.X, pos.Y) // fire.go (PLAY-015)
	var hit []entity.EntityID
	g.within(pos.X, pos.Y, 1, ckAll, func(id entity.EntityID, _ Position) { hit = append(hit, id) })
	for _, id := range hit {
		p, ok := g.positions.Get(id)
		if !ok {
			continue // a chained burst already took it
		}
		dmg := burstDamage
		if p.X != pos.X && p.Y != pos.Y {
			dmg -= burstFalloff
		}
		g.hurt(id, dmg, 0) // a dead or removed one is skipped by hurt
	}
}

// pruneBursts forgets bursts older than an hour.
func (g *Game) pruneBursts(now uint64) {
	kept := g.wl.Bursts[:0]
	for _, b := range g.wl.Bursts {
		if now-b.Tick <= burstKeep {
			kept = append(kept, b)
		}
	}
	g.wl.Bursts = kept
}

// fillWildlifeView copies the recent bursts into the view.
func (g *Game) fillWildlifeView(w *WorldView) {
	if len(g.wl.Bursts) == 0 {
		return
	}
	w.Bursts = make([]BurstView, len(g.wl.Bursts))
	for i, b := range g.wl.Bursts {
		w.Bursts[i] = BurstView{X: b.X, Y: b.Y, Tick: b.Tick, Species: b.Species.String(), Turret: b.Turret, Blast: b.Blast.String(), Radius: b.Radius}
		if b.Turret || b.Blast != 0 {
			w.Bursts[i].Species = ""
		}
	}
}

// dropCorpse leaves the colony's own carcass of s at (x, y)
// (dropCorpseAs).
func (g *Game) dropCorpse(s animal.Species, x, y int) entity.EntityID {
	return g.dropCorpseAs(s, x, y, false)
}

// dropCorpseAs leaves a carcass of s at (x, y), stacking with its
// species, and returns the stack it lies in (0: none). An unclaimed
// carcass (the colony had no hand in the death: PLAY-017) waits for a
// haul mark, and stacks only with other unclaimed ones.
func (g *Game) dropCorpseAs(s animal.Species, x, y int, unclaimed bool) entity.EntityID {
	sid, err := g.SpawnStack(item.CorpseAnimal, 1, x, y)
	if err != nil {
		return 0
	}
	st, _ := g.stacks.Get(sid)
	// a mechanoid's wreck never rots (Dried: it keeps; DEF-4)
	st.Animal, st.Unclaimed, st.Dried = uint8(s), unclaimed, animal.IsMech(s)
	g.mergeOrSet(sid, st)
	if _, ok := g.stacks.Get(sid); ok {
		return sid
	}
	for _, id := range g.itemIDs { // merged into the carcasses already there
		if o, ok := g.stacks.Get(id); ok && o.Carrier == 0 && o.X == x && o.Y == y && o.Kind == item.CorpseAnimal && o.Animal == uint8(s) &&
			o.Unclaimed == unclaimed && !o.HaulMarked {
			return id
		}
	}
	return 0
}

// colonyKill reports a death the colony caused (PLAY-017): by is one of
// its colonists or tame animals, or (by 0) a spike trap sprang on at this
// tick (defense.go springTrap leaves its trace just before the hit).
func (g *Game) colonyKill(by entity.EntityID, at Position) bool {
	if by == 0 {
		if n := len(g.traces); n > 0 {
			t := g.traces[n-1]
			return t.Attacker == 0 && t.Melee && t.Hit && t.Tick == uint64(g.clock.Now()) &&
				t.FromX == at.X && t.FromY == at.Y && t.ToX == at.X && t.ToY == at.Y
		}
		return false
	}
	if k := g.animals.Ref(by); k != nil {
		return k.Tame
	}
	return slices.Contains(g.pawnIDs, by) // colonists, never raiders, prisoners or visitors
}

// afterLoadAnimals converts an older save: dustbuck and ridgewolf
// corpses become deer and timber wolf corpses, and their butcher bills
// one butcher-animal bill per bench.
func (g *Game) afterLoadAnimals() {
	for _, id := range g.animalIDs { // SIM-024: fed, rested, sexed
		if a, ok := g.animals.Get(id); ok && (!a.HasSex || a.Hunger != 0) {
			a.Normalise(uint64(id))
			g.animals.Set(id, a)
		}
	}
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok {
			if c, conv := item.ConvertLegacy(st); conv {
				g.stacks.Set(id, c)
			}
		}
	}
	g.craft.Bills.UpgradeLegacy() // craft/bills.go
}

// pluralLabel is a species' plural display name ("timber wolves").
func pluralLabel(s animal.Species) string {
	l := s.Label()
	switch {
	case strings.HasSuffix(l, "wolf"):
		return l[:len(l)-1] + "ves"
	case strings.HasSuffix(l, "goose"):
		return l[:len(l)-4] + "eese"
	case strings.HasSuffix(l, "x"), strings.HasSuffix(l, "ch"):
		return l + "es"
	case strings.HasSuffix(l, "deer"), strings.HasSuffix(l, "sheep"), strings.HasSuffix(l, "bison"),
		strings.HasSuffix(l, "elk"), strings.HasSuffix(l, "caribou"):
		return l
	case strings.HasSuffix(l, "y") && !strings.HasSuffix(l, "ey") && !strings.HasSuffix(l, "ay"):
		return l[:len(l)-1] + "ies"
	case strings.HasSuffix(l, "s"):
		return l + "es"
	}
	return l + "s"
}
