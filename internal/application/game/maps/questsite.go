package maps

import (
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
)

// A quest site's own rules beyond what its kind lays (QST-017, 051, 056).
// When its map opens the quest says whether an unknown threat may come
// (the stash and the refugee: the mineral site's roll, oresite.go) and
// which hostile faction is behind the site (the camp's, the captors', the
// guards'). Once the job there is done that faction sends reinforcements
// in reinforceHours: a letter says so, and if anyone of ours is still on
// the map then, a raid of that faction comes onto it. It comes once and
// never again (Meta.Reinforced is saved); the pack-up waits for it, then
// runs as on any site once no hostile stands. A site with no faction
// behind it packs up at once, as before.

// SiteRuler is the quests' answer for a quest site's map: approach, an
// unknown threat may come when it opens; foe, the hostile faction behind
// it (1 + index; 0 none). The game's quest adapter answers it from the
// quest's Def and its target; a host that does not lays no such rules.
type SiteRuler interface {
	SiteRules(quest uint32) (approach bool, foe int)
}

const (
	// reinforceHours is how long after a quest site's job is done its
	// faction's reinforcements come (wiki: Events, 60 hours).
	reinforceHours = 60
	reinforceTicks = reinforceHours * clock.TicksPerHour
	// reinforceSalt keeps the reinforcements' draws apart from the map's.
	reinforceSalt uint64 = 0x2e1f0
)

// questSite asks the quest of site map id, just opened and laid, for its
// rules: it files the foe and rolls the approach at caravan c.
func (l Lifecycle) questSite(c *Caravan, id, quest uint32) {
	r, ok := l.f.h.(SiteRuler)
	m := l.f.a.Meta(id)
	if !ok || m == nil {
		return
	}
	approach, foe := r.SiteRules(quest)
	m.Foe = foe
	if approach {
		l.oreThreat(c, id) // oresite.go: the same roll, from the map's seed
	}
}

// reinforcing reports a quest site whose reinforcements are still to come:
// its pack-up waits for them. A reform the player asked for that found no
// way out yet (PackBy, reform) still retries meanwhile: leaving is theirs.
func reinforcing(m *Meta) bool {
	return m.Quest != 0 && m.Foe != 0 && !m.Reinforced
}

// reinforce is a reinforcing site's hour (census c, the quest still busy):
// once the job is done the clock starts, with a letter; when it runs out
// the faction's raid comes if anyone of ours is still there. Either way
// the site is Reinforced and its pack-up count starts over. True while it
// holds the pack-up this hour; false once the foe no longer stands (made
// peace, or defeated): nobody comes, and the pack-up goes on at once.
func (l Lifecycle) reinforce(m *Meta, c Census, busy bool, now uint64) bool {
	f := l.f
	if !l.foeStands(m.Foe) {
		m.Reinforced = true
		return false
	}
	switch {
	case m.Reinforce == 0:
		if streak(&m.Done, !busy && JobDone(m, c)) {
			m.Reinforce = now + reinforceTicks
			f.h.Letter(true, "Reinforcements on the way", 0, "The work at %s is done. Enemy reinforcements will reach it in %d hours: be gone by then.", f.h.TileName(m.Tile), reinforceHours)
		}
	case now >= m.Reinforce:
		m.Reinforced, m.Done = true, 0
		if c.Ours > 0 {
			f.h.Ambush(SeedFor(f.h.Seed(), m.Tile, m.ID)^reinforceSalt, m.Foe)
			f.h.Letter(true, "Reinforcements", 0, "Enemy reinforcements have reached %s.", f.h.TileName(m.Tile))
		}
	}
	return true
}

// foeStands reports faction fac (1 + index) still hostile and standing.
func (l Lifecycle) foeStands(fac int) bool {
	fs := l.f.h.Factions()
	return fac > 0 && fac <= len(fs) && fs[fac-1].Relation == faction.Hostile && !fs[fac-1].Defeated
}

// Reinforcing is map id's ticks left at now until its reinforcements come
// (0: none on the way), for the map's read model.
func (a *Atlas) Reinforcing(id uint32, now uint64) int {
	if m := a.Meta(id); m != nil && !m.Reinforced && m.Reinforce > now {
		return int(m.Reinforce - now)
	}
	return 0
}
