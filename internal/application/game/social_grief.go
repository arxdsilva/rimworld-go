package game

import (
	"slices"

	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Death and loss (ENG-003, M7). Grief follows ties, not proximity: the
// dead pawn's social edges and its family graph, so a relative across the
// map, in a caravan or held prisoner grieves and a stranger standing by
// does not. The dead stay in the registry (and the Social tab).

// tiesOf lists the people tied to id (social edges and family), ascending.
func (g *Game) tiesOf(id entity.EntityID) []entity.EntityID {
	out := g.kinCandidates(id)
	if g.soc != nil {
		for _, o := range g.soc.adj[id] {
			if i, found := slices.BinarySearch(out, o); !found {
				out = slices.Insert(out, i, o)
			}
		}
	}
	return out
}

// griefKind is the relation of dead to c with the strongest grief.
func griefKind(ks []social.Kind) (social.Kind, bool) {
	best, ok := social.NoKind, false
	bestMood := 0
	for _, k := range ks {
		if d, has := social.GriefFor(k, false, false, false); has && (!ok || d.Mood < bestMood) {
			best, bestMood, ok = k, d.Mood, true
		}
	}
	return best, ok
}

// killerKind is the relation of dead to c with the heaviest killer
// opinion.
func killerKind(ks []social.Kind) (social.Kind, bool) {
	best, ok := social.NoKind, false
	var bestV int32
	for _, k := range ks {
		if v := social.KillerOpinion(k); v != 0 && (!ok || v < bestV) {
			best, bestV, ok = k, v, true
		}
	}
	return best, ok
}

// canGrieve reports c has a mood to grieve with: alive and with the colony
// (at home, in a caravan) or held prisoner.
func (g *Game) canGrieve(c entity.EntityID) bool {
	p := g.personOf(c)
	if p == nil || p.Where != whereColony && p.Where != wherePrisoner && p.Where != whereCaravan {
		return false
	}
	if v, ok := g.vital.Get(c); ok && v.Status == health.Dead {
		return false
	}
	return true
}

// addGrief gives c a grief thought about subject lasting days.
func (g *Game) addGrief(c, subject entity.EntityID, d social.GriefDef, now uint64) {
	m := g.memories[c].AddValue(mood.Grief, uint64(subject), d.Mood, now)
	// mood.Grief has no duration of its own: the relation's sets it.
	for i := range m {
		if m[i].Kind == mood.Grief && m[i].Subject == uint64(subject) && m[i].Expires == now {
			m[i].Expires = now + uint64(d.Days)*clock.TicksPerDay
		}
	}
	g.memories[c] = m
}

// grieve gives every tie of id its grief (lost: missing, not dead) and,
// for a death, the opinion of killer (0: none).
func (g *Game) grieve(id, killer entity.EntityID, lost bool, now uint64) {
	killerTie := killer != 0 && killer != id && g.humanTie(killer)
	for _, c := range g.tiesOf(id) {
		cp := g.personOf(c)
		if cp == nil || cp.Where == whereDead {
			continue
		}
		ks := g.kinOf(c, id)
		k, _ := griefKind(ks)
		if g.canGrieve(c) {
			bio, _ := g.personBio(c)
			if !social.ExemptFromGrief(bio.Traits) {
				if d, ok := social.GriefFor(k, g.friendOf(c, id), g.rivalOf(c, id), lost); ok {
					g.addGrief(c, id, d, now)
				}
			}
		}
		if killerTie && c != killer {
			if kk, ok := killerKind(ks); ok {
				if mk, ok := social.KillerMemory(kk); ok {
					g.ensurePerson(killer)
					g.addOpinionMemory(c, killer, mk, now)
				}
			}
		}
	}
}

// socialOnDeath is called from dieBy before mourn (ENG-003, M7): the dead
// pawn's relations grieve, and they think less of killer (0: none).
func (g *Game) socialOnDeath(dead, killer entity.EntityID) {
	if _, isAnimal := g.animals.Get(dead); isAnimal {
		return
	}
	now := uint64(g.clock.Now())
	p := g.ensurePerson(dead)
	wasColonist := p.Where == whereColony
	p.Where, p.DiedAt, p.KilledBy = whereDead, now, killer
	if g.soc != nil {
		delete(g.soc.next, dead)
	}
	g.grieve(dead, killer, false, now)
	if wasColonist {
		g.addHistory("death", g.personName(dead)+" died.", dead)
	}
}

// socialOnLost: colonist id went missing (kidnapped, a caravan lost:
// M9). Its ties grieve by the lost table, the colony misses it.
func (g *Game) socialOnLost(id entity.EntityID) {
	now := uint64(g.clock.Now())
	g.ensurePerson(id)
	g.grieve(id, 0, true, now)
	for _, c := range g.pawnIDs {
		if c == id || !g.canGrieve(c) {
			continue
		}
		if bio, _ := g.personBio(c); social.ExemptFromGrief(bio.Traits) {
			continue
		}
		g.remember(c, mood.ColonistLost)
	}
	g.addHistory("lost", g.personName(id)+" went missing.", id)
}

// socialOnReturned: missing colonist id is back, so the grief over it
// ends early.
func (g *Game) socialOnReturned(id entity.EntityID) {
	for _, c := range g.tiesOf(id) {
		m := g.memories[c]
		if len(m) == 0 {
			continue
		}
		out := m[:0]
		for _, x := range m {
			if !(x.Kind == mood.Grief && x.Subject == uint64(id)) {
				out = append(out, x)
			}
		}
		g.memories[c] = out
	}
	g.addHistory("returned", g.personName(id)+" came back.", id)
}
