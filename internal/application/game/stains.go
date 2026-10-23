package game

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/stain"
	"rimworld-go/internal/domain/world/terrain"
)

const (
	hitStain       = 40 // a hit leaves this, plus hitStainPerDmg per damage: a rifle hit (14) is a splat, two make a pool
	hitStainPerDmg = 25
	sprayDamage    = 6  // hits this heavy spray the tile behind the target
	spraySharePct  = 40 // of the hit stain
	deathPool      = 700
	dripStain      = 80
	dripEvery      = 3 // tiles walked per drip
	downedPoolStep = 60
	downedPoolMax  = 600
	colonistBleed  = 15 // injury at which an untended wound drips
	raiderBleed    = 30
)

// Blood and ichor on the ground (SIM-020): hits, deaths and bleeding
// creatures stain tiles; stains fade over 7 days and wash off open ground
// in rain and snow (stain package). Deterministic and saved.

// StainView is one stained tile for presentation.
type StainView struct {
	X, Y     int
	Kind     string  // "blood" or "ichor"
	Amount   float64 // 0..1 strength
	AgeTicks uint64  // since the last fresh blood (drying)
	Variant  uint8
}

// stainState is the stains on the map and their fade/wash remainders.
type stainState struct {
	tiles map[[2]int]stain.Stain
	frac  map[[2]int]int          // fixed-point remainder per tile (stain.Fade/Wash)
	dripN map[entity.EntityID]int // tiles walked since the last drip, per creature
}

// stainKindOf is what creature id bleeds: hive insects ichor, else blood.
func (g *Game) stainKindOf(id entity.EntityID) stain.Kind {
	if a, ok := g.animals.Get(id); ok && animal.RosterOf(a.Species).Behaviour == animal.Hive {
		return stain.Ichor
	}
	return stain.Blood
}

// stainLess orders tiles y then x.
func stainLess(a, b [2]int) int {
	if c := cmp.Compare(a[1], b[1]); c != 0 {
		return c
	}
	return cmp.Compare(a[0], b[0])
}

// stainVariant picks the splat shape from the tile alone (no RNG).
func stainVariant(x, y int) uint8 {
	h := uint32(x)*73856093 ^ uint32(y)*19349663
	return uint8(h % 6)
}

// stamp adds blood of kind to tile (x, y): out of bounds and rock stay
// clean; it merges into an existing stain, which is fresh again.
func (g *Game) stamp(x, y int, kind stain.Kind, add int) {
	if add <= 0 || !g.grid.InBounds(x, y) || terrain.IsRockWall(g.terr.At(x, y)) {
		return
	}
	if g.stains.tiles == nil {
		g.stains.tiles = map[[2]int]stain.Stain{}
	}
	k := [2]int{x, y}
	s := g.stains.tiles[k]
	old := s.Amount
	s.X, s.Y, s.Kind = x, y, kind
	s.Amount = stain.Add(s.Amount, add)
	s.Born = uint64(g.clock.Now())
	s.Variant = stainVariant(x, y)
	g.stains.tiles[k] = s
	g.stainMoved(x, y, int(old), int(s.Amount))
	if len(g.stains.tiles) > stain.Cap {
		g.dropFaintestStain()
	}
}

// dropFaintestStain removes the weakest stain: ties go to the oldest
// Born, then y, then x, so the choice never depends on map order.
func (g *Game) dropFaintestStain() {
	var worst stain.Stain
	first := true
	for _, s := range g.stains.tiles {
		if first || stainWeaker(s, worst) {
			worst, first = s, false
		}
	}
	k := [2]int{worst.X, worst.Y}
	delete(g.stains.tiles, k)
	delete(g.stains.frac, k)
	g.stainMoved(worst.X, worst.Y, int(worst.Amount), 0)
}

// stainMoved keeps the room filth sums and the cleaning queue (filth.go)
// in step with the stain on (x, y) going from old to nv. Every stain
// change ends here.
func (g *Game) stainMoved(x, y, old, nv int) {
	if old == nv || !g.grid.InBounds(x, y) {
		return
	}
	i := y*g.grid.Width() + x
	d := g.dirtAtIdx(i)
	g.filthMoved(i, d+old, d+nv)
}

// setStainAmount sets the stain on (x, y) (cleaning); 0 removes it.
func (g *Game) setStainAmount(x, y, a int) {
	k := [2]int{x, y}
	s, ok := g.stains.tiles[k]
	if !ok || int(s.Amount) == a {
		return
	}
	old := int(s.Amount)
	if a <= 0 {
		delete(g.stains.tiles, k)
		delete(g.stains.frac, k)
		a = 0
	} else {
		s.Amount = uint16(min(a, stain.Full))
		g.stains.tiles[k] = s
	}
	g.stainMoved(x, y, old, a)
}

func stainWeaker(a, b stain.Stain) bool {
	if a.Amount != b.Amount {
		return a.Amount < b.Amount
	}
	if a.Born != b.Born {
		return a.Born < b.Born
	}
	return stainLess([2]int{a.X, a.Y}, [2]int{b.X, b.Y}) < 0
}

// stainHit stains a hit: the target's tile, and a spray beyond it away from
// the attacker for heavy hits. Called before hurt (the target still
// exists).
func (g *Game) stainHit(target entity.EntityID, from, to Position, dmg int) {
	kind := g.stainKindOf(target)
	amt := hitStain + hitStainPerDmg*dmg
	g.stamp(to.X, to.Y, kind, amt)
	if dmg < sprayDamage {
		return
	}
	dx, dy := sign(to.X-from.X), sign(to.Y-from.Y)
	if dx == 0 && dy == 0 {
		return
	}
	g.stamp(to.X+dx, to.Y+dy, kind, amt*spraySharePct/100)
}

// stainDeath pools blood where creature id died (pawns; called from die).
func (g *Game) stainDeath(id entity.EntityID) {
	if pos, ok := g.positions.Get(id); ok {
		g.stamp(pos.X, pos.Y, g.stainKindOf(id), deathPool)
	}
}

// stainAnimalDeath pools blood (or ichor) where an animal of species s
// died at pos (called from killAnimal).
func (g *Game) stainAnimalDeath(s animal.Species, pos Position) {
	kind := stain.Blood
	if animal.RosterOf(s).Behaviour == animal.Hive {
		kind = stain.Ichor
	}
	g.stamp(pos.X, pos.Y, kind, deathPool)
}

// bleeding reports a creature that is losing blood: an animal under half
// its hit points, a hurt raider, or a colonist with an open wound nobody
// has tended.
func (g *Game) bleeding(id entity.EntityID) bool {
	if a, ok := g.animals.Get(id); ok {
		return a.HP()*2 < animal.DefOf(a.Species).MaxHP
	}
	v, ok := g.vital.Get(id)
	if !ok || v.Status == health.Dead {
		return false
	}
	if g.isRaider(id) {
		return v.Injury >= raiderBleed
	}
	return g.wounds[id].At != 0 && v.Injury >= colonistBleed && !g.tended(id)
}

// stainStep drips blood behind a bleeding creature every 3rd tile it walks
// (called from stepPawn and stepAnimal with the tile it left).
func (g *Game) stainStep(id entity.EntityID, from Position) {
	if !g.bleeding(id) {
		if len(g.stains.dripN) > 0 {
			delete(g.stains.dripN, id)
		}
		return
	}
	if g.stains.dripN == nil {
		g.stains.dripN = map[entity.EntityID]int{}
	}
	n := g.stains.dripN[id] + 1
	if n < dripEvery {
		g.stains.dripN[id] = n
		return
	}
	g.stains.dripN[id] = 0
	g.stamp(from.X, from.Y, g.stainKindOf(id), dripStain)
}

// stainDowned grows a pool under a downed, untended colonist once an hour
// (called each tick from phaseNeeds for downed pawns).
func (g *Game) stainDowned(id entity.EntityID, now uint64) {
	if now%clock.TicksPerHour != 0 || g.isRaider(id) || g.tended(id) {
		return
	}
	pos, ok := g.positions.Get(id)
	if !ok {
		return
	}
	room := downedPoolMax - int(g.stains.tiles[[2]int{pos.X, pos.Y}].Amount)
	g.stamp(pos.X, pos.Y, g.stainKindOf(id), min(downedPoolStep, room))
}

// stainDecay fades every stain and washes those under the open sky while
// rain or snow falls (called every climate interval).
func (g *Game) stainDecay() {
	g.pruneDrips()
	g.washDirt() // filth.go: rain rinses dirt off open ground too
	if len(g.stains.tiles) == 0 {
		return
	}
	falling := g.weather.Kind.Falling()
	frozen := g.weather.Kind.Frozen()
	for k, s := range g.stains.tiles {
		f := g.stains.frac[k]
		a := stain.Fade(s.Amount, climateInterval, &f)
		if falling && a > 0 && g.underSky(s.X, s.Y) {
			a = stain.Wash(a, climateInterval, g.weather.Intensity, frozen, &f)
		}
		if a == 0 {
			delete(g.stains.tiles, k)
			delete(g.stains.frac, k)
			g.stainMoved(s.X, s.Y, int(s.Amount), 0)
			continue
		}
		if a != s.Amount {
			g.stainMoved(s.X, s.Y, int(s.Amount), int(a))
			s.Amount = a
			g.stains.tiles[k] = s
		}
		if g.stains.frac == nil {
			g.stains.frac = map[[2]int]int{}
		}
		g.stains.frac[k] = f
	}
}

// pruneDrips forgets drip counters of creatures that are gone, and the
// boots of pawns that are (filth.go).
func (g *Game) pruneDrips() {
	g.pruneCarried()
	for id := range g.stains.dripN {
		if _, ok := g.animals.Get(id); ok {
			continue
		}
		if v, ok := g.vital.Get(id); ok && v.Status != health.Dead {
			continue
		}
		delete(g.stains.dripN, id)
	}
}

// exportStains is the stains for a save, sorted y then x.
func (g *Game) exportStains() []stain.Stain {
	if len(g.stains.tiles) == 0 {
		return nil
	}
	out := make([]stain.Stain, 0, len(g.stains.tiles))
	for k, s := range g.stains.tiles {
		s.Rem = g.stains.frac[k]
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b stain.Stain) int {
		return stainLess([2]int{a.X, a.Y}, [2]int{b.X, b.Y})
	})
	return out
}

// fillStainView copies the stains into the view, sorted y then x.
func (g *Game) fillStainView(w *WorldView) {
	now := uint64(g.clock.Now())
	src := g.sortedStains()
	if len(src) > 0 {
		w.Stains = make([]StainView, 0, len(src))
	}
	for i := range src {
		s := &src[i].s
		age := uint64(0)
		if now > s.Born {
			age = now - s.Born
		}
		w.Stains = append(w.Stains, StainView{
			X: s.X, Y: s.Y, Kind: s.Kind.String(),
			Amount: float64(s.Amount) / stain.Full, AgeTicks: age, Variant: s.Variant,
		})
	}
}

// stainMemo is the view cache's last sorted stain list: every stain
// with its tile key, in exportStains' order. Stains fade a few at a time,
// so most ticks the list still matches the map and the sort is skipped.
type stainMemo struct {
	list []stainKeyed
	ok   bool
}

type stainKeyed struct {
	k [2]int
	s stain.Stain
}

// sortedStains is the stains sorted y then x (as exportStains, without
// the fade remainders): the memo while it still matches every tile, else
// a fresh sort. Read only.
func (g *Game) sortedStains() []stainKeyed {
	tiles := g.stains.tiles
	m := &g.views.stains
	if g.views.on && m.ok && len(m.list) == len(tiles) {
		same := true
		for i := range m.list {
			if s, ok := tiles[m.list[i].k]; !ok || s != m.list[i].s {
				same = false
				break
			}
		}
		if same {
			return m.list
		}
	}
	if len(tiles) == 0 {
		m.list, m.ok = m.list[:0], g.views.on
		return nil
	}
	// a fresh array: the views built from the last one keep reading it
	out := make([]stainKeyed, 0, len(tiles))
	for k, s := range tiles {
		out = append(out, stainKeyed{k, s})
	}
	slices.SortFunc(out, func(a, b stainKeyed) int {
		return stainLess([2]int{a.s.X, a.s.Y}, [2]int{b.s.X, b.s.Y})
	})
	if g.views.on {
		m.list, m.ok = out, true
	}
	return out
}

// importStains installs saved stains (nil: an older save, no stains).
// The fade remainder (Rem) is restored so a loaded game fades in lock step.
func (g *Game) importStains(s []stain.Stain) {
	g.stains = stainState{}
	for _, st := range s {
		if st.Amount == 0 || !g.grid.InBounds(st.X, st.Y) {
			continue
		}
		st.Amount = min(st.Amount, stain.Full)
		if g.stains.tiles == nil {
			g.stains.tiles = map[[2]int]stain.Stain{}
		}
		k := [2]int{st.X, st.Y}
		if st.Rem > 0 {
			if g.stains.frac == nil {
				g.stains.frac = map[[2]int]int{}
			}
			g.stains.frac[k] = st.Rem
		}
		st.Rem = 0
		g.stains.tiles[k] = st
	}
	g.refreshFilth() // stains count as filth: requeue and recount
}
