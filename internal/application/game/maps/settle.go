package maps

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/combat/kit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/pkg/randx"
)

// Attacking a faction base (MAP-05, wiki: Faction base). A caravan waiting
// on a settlement's tile (or pods landing there to attack) costs the
// faction 200 goodwill and turns it hostile; a settlement map opens there
// with the faction's compound (mapgen.Settlement) and its defenders
// (raid.Defend), and our people enter at the edge (or, from pods, the
// centre), drafted. Once no defender has stood on the map for
// closeStreak hourly results in a row (dead, downed or fled; turrets do
// not count) the settlement is destroyed: it leaves the faction and the
// planet, a faction with none left is defeated, the factions hostile to
// it gain 20 goodwill, a letter jumps to the map, which becomes a site and
// starts its pack-up. The loot is whatever lies on the map.

// Settler is the simulation's side of a settlement attack.
type Settler interface {
	// Settle lays compound c on the current map and spawns one defender
	// of faction fac (1 + index) per weapon in arms at its posts, drawing
	// from a stream seeded with seed (no simulation stream moves).
	Settle(c mapgen.Compound, fac int, arms []item.Kind, seed uint64)
	// Defenders counts the defenders on the current map still on their
	// feet and not fleeing.
	Defenders() int
	// Factions is the game's factions (its slice: elements are written).
	Factions() []faction.Faction
	// ThreatScale is the difficulty's threat scale, percent (100 without a
	// setup).
	ThreatScale() int
	// Day is the colony's day number (1 on the first day).
	Day() int
}

const (
	// attackGoodwill is what attacking a settlement costs (wiki: Faction
	// base, -200; goodwill clamps at -100, so the faction turns hostile).
	attackGoodwill = -200
	// enemyGoodwill is what destroying it earns with the factions hostile
	// to it (wiki: Faction base).
	enemyGoodwill = 20
	// defenderMax is the most defenders a settlement fields (original
	// tuning).
	defenderMax = 16
	// settleSalt keeps the defenders' stream apart from the map's.
	settleSalt uint64 = 0xde7e2d
)

// defenderPoints is a settlement's defence budget (original tuning): a
// base by faction tech, scaled by half the difficulty's threat scale over
// a fixed half, so a peaceful game still meets a guard and a brutal one
// half again as many.
func defenderPoints(tech faction.Tech, threatScale int) int {
	base := 450
	switch tech {
	case faction.Industrial:
		base = 650
	case faction.Spacer:
		base = 900
	}
	return base * (100 + max(threatScale, 0)) / 200
}

// AttackReason is why caravan id cannot attack the settlement on its tile
// ("" it can): it must be waiting there (or visiting), and the tile must
// hold a human faction's settlement or a settlement map already under
// attack.
func (l Lifecycle) AttackReason(id entity.EntityID) string {
	c := l.f.At(id)
	switch {
	case c == nil:
		return "No such caravan."
	case c.Back || c.C.State != caravan.Arrived && c.C.State != caravan.Visiting:
		return "Only a caravan waiting at a settlement can attack it."
	}
	if m := l.openAt(c.C.Tile); m != nil && m.Kind == Settlement {
		return "" // the attack is on: it joins
	}
	if l.factionAt(c.C.Tile) < 0 {
		return "There is no settlement here to attack."
	}
	return ""
}

// Attack is command.AttackSettlement: caravan Target attacks the
// settlement on its tile, entering at its map's edge.
func (l Lifecycle) Attack(c command.Command) {
	if why := l.f.Assault(c.Target, false); why != "" {
		l.f.h.Note("%s", why)
	}
}

// Assault has caravan cid attack the settlement on its tile, entering at
// the map's edge or centre at the end of the tick: why it cannot ("" it
// does). The first attack on a settlement costs its faction's goodwill.
func (f Fleet) Assault(cid uint64, centre bool) string {
	id, l := entity.EntityID(cid), f.Life()
	if why := l.AttackReason(id); why != "" {
		return why
	}
	c := f.At(id)
	t := c.C.Tile
	c.C.State = caravan.Arrived
	pending := slices.ContainsFunc(f.a.Queue, func(r Request) bool { return r.Op == OpOpen && r.Tile == t && r.Kind == Settlement })
	if fi := l.factionAt(t); fi >= 0 && l.openAt(t) == nil && !pending {
		fa := &f.h.Factions()[fi]
		faction.Adjust(fa, attackGoodwill, fmt.Sprintf("Day %d: settlement attacked (%d goodwill)", f.h.Day(), attackGoodwill))
		fa.Relation = faction.Hostile // wiki: Faction base, whatever goodwill it had
	}
	f.a.Queue = append(f.a.Queue, Request{Op: OpOpen, Kind: Settlement, Tile: t, Caravan: cid, Centre: centre})
	return ""
}

// storm brings caravan c onto settlement map id for request r: a freshly
// opened map first gets its compound and defenders; the attackers come
// in drafted at an edge (or the centre) and stay there.
func (l Lifecycle) storm(c *Caravan, id uint32, r Request) {
	f := l.f
	if !f.h.Switch(id) {
		l.enter(c, id) // the map is gone: the first colony
		return
	}
	m := f.a.Meta(id)
	if m != nil && m.Kind == Settlement && m.Faction == 0 {
		if fi := l.factionAt(m.Tile); fi >= 0 {
			l.populate(m, fi)
		} else {
			m.Kind = Site // nobody lives there any more
		}
	}
	names := l.walkInAt(c, r.Centre)
	if len(names) > 0 {
		f.h.Letter(true, "Attack on a settlement", entity.EntityID(c.C.Members[0]), "%s attacked %s. Defeat every defender to destroy it.", travel.JoinNames(names), f.h.TileName(c.C.Tile))
	}
	f.drop(entity.EntityID(c.C.ID))
}

// populate lays faction fi's compound and defenders on the current map,
// which is settlement map m.
func (l Lifecycle) populate(m *Meta, fi int) {
	f := l.f
	tech := faction.DefOf(f.h.Factions()[fi].Type).Tech
	seed := SeedFor(f.h.Seed(), m.Tile, m.ID) ^ settleSalt
	c := mapgen.Settlement(mapgen.CompoundConfig{W: SiteSide, H: SiteSide, Tech: tech, Seed: seed})
	arms := kit.Pick(defenderPoints(tech, f.h.ThreatScale()), defenderMax, tech, randx.New(seed).IntN)
	f.h.Settle(c, fi+1, arms, seed)
	m.Faction = fi + 1
}

// walkInAt brings c's members (drafted) and animals onto the current map
// at an edge cell, or near its centre, where its goods are set down; they
// hold there. It names the members who came in.
func (l Lifecycle) walkInAt(c *Caravan, centre bool) []string {
	f := l.f
	c.C.State = caravan.Home
	ex, ey, ok := f.h.Edge()
	if centre || !ok {
		side := SiteSide
		if m := f.a.Meta(f.h.Current()); m != nil {
			side = SizeFor(m.Kind, SiteSide)
		}
		ex, ey = side/2, side/2
		if x, y, ok := f.h.WalkableNear(ex, ey, side/4); ok {
			ex, ey = x, y
		}
	}
	var used [][2]int
	var names []string
	for i, m := range append(slices.Clip(c.C.Members), c.C.Animals...) {
		id := entity.EntityID(m)
		if !f.h.IsStashed(id) {
			continue
		}
		x, y, ok := f.h.FreeLand(ex, ey, 4, used)
		if !ok {
			if x, y, ok = f.h.WalkableNear(ex, ey, 6); !ok {
				x, y = ex, ey
			}
		}
		used = append(used, [2]int{x, y})
		if !f.h.Unstash(id, x, y) || i >= len(c.C.Members) {
			continue
		}
		names = append(names, f.h.Name(id))
		f.h.SetDraft(id, true)
	}
	f.h.Cargo().Home(c.C.ID, ex, ey)
	return names
}

// besiege folds the hour into settlement map m (current): once no
// defender has stood for closeStreak results in a row, it is destroyed.
func (l Lifecycle) besiege(m *Meta, now uint64) {
	f := l.f
	if m.Defeated || m.Faction == 0 || !streak(&m.Routed, f.h.Defenders() == 0) {
		return
	}
	name := f.h.TileName(m.Tile)
	gone := l.destroy(m.Faction-1, m.Tile)
	m.Defeated, m.Kind = true, Site
	if m.PackBy == 0 {
		m.PackBy = now + packTicks
	}
	text := fmt.Sprintf("%s has fallen: no defender is left standing. Whatever lies there is ours; whoever is still there leaves as a caravan in a day.", name)
	if gone != "" {
		text += fmt.Sprintf(" It was the last settlement of %s, who are no more.", gone)
	}
	f.h.Letter(false, "Settlement destroyed", 0, "%s", text)
}

// destroy takes the settlement on tile t away from faction fi and the
// planet; a faction left with none is defeated (its name returned, else
// ""). The factions hostile to it gain goodwill.
func (l Lifecycle) destroy(fi, t int) string {
	f := l.f
	fs := f.h.Factions()
	if fi < 0 || fi >= len(fs) {
		return ""
	}
	fa := &fs[fi]
	if k := slices.Index(fa.Settlements, t); k >= 0 {
		fa.Settlements = slices.Delete(slices.Clone(fa.Settlements), k, k+1)
		if k < len(fa.SettlementNames) {
			fa.SettlementNames = slices.Delete(slices.Clone(fa.SettlementNames), k, k+1)
		}
	}
	if p := f.h.Planet(); p != nil && t >= 0 && t < len(p.Settlement) && p.Settlement[t] >= 0 {
		p.Settlement[t] = -1
	}
	for j := range fs {
		if j != fi && Enemies(fs[j], *fa) {
			faction.Adjust(&fs[j], enemyGoodwill, fmt.Sprintf("Day %d: an enemy settlement destroyed (+%d goodwill)", f.h.Day(), enemyGoodwill))
		}
	}
	if len(fa.Settlements) > 0 {
		return ""
	}
	fa.Defeated = true
	return fa.Name
}

// Enemies reports human faction a hostile to another human faction b
// (original tuning: factions have no relations of their own here). Freebooters are
// everyone's enemies, and two factions hostile to the colony are rivals,
// as raiders of two such factions fight on the map (game raids' Rival).
func Enemies(a, b faction.Faction) bool {
	if !a.Type.Human() || !b.Type.Human() || a.Defeated {
		return false
	}
	return a.Type == faction.Freebooters || b.Type == faction.Freebooters ||
		a.Relation == faction.Hostile && b.Relation == faction.Hostile
}

// factionAt is the index of the human faction whose settlement stands on
// tile t (-1: none, or a defeated one).
func (l Lifecycle) factionAt(t int) int {
	for i, fa := range l.f.h.Factions() {
		if fa.Type.Human() && !fa.Defeated && slices.Contains(fa.Settlements, t) {
			return i
		}
	}
	return -1
}

// openAt is the open temporary map on tile t (nil: none).
func (l Lifecycle) openAt(t int) *Meta {
	a := l.f.a
	for i := range a.Maps {
		if a.Maps[i].Tile == t {
			return &a.Maps[i]
		}
	}
	return nil
}

// Prune drops from a rebuilt planet p the settlements its factions fs no
// longer hold (destroyed in this game; the planet is not saved). A planet
// whose factions do not match fs is left alone.
func Prune(p *planet.Planet, fs []faction.Faction) {
	if p == nil || len(p.Factions) != len(fs) {
		return
	}
	for t := range p.Settlement {
		if fi, _ := p.SettlementFaction(t); fi >= 0 && !slices.Contains(fs[fi].Settlements, t) {
			p.Settlement[t] = -1
		}
	}
}
