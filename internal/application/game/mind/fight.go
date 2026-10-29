package mind

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Social fights (ENG-003, M6): two colonists in the SocialFight break
// brawl (the game rolls whether an insult starts one); Brawl throws the
// fists. The fight ends at once when either goes down, dies
// or leaves; after FightMinTicks a recovery roll every fightRecoverEvery
// ticks ends it. Each fighter then rolls its own outcome, an opinion
// memory of the other, which is also the pair's cooldown. A social fight
// is not a mood break: no catharsis, no calm window.
//
// State lives only in the game's saved records: the break (kind, until =
// start + FightMinTicks) and the brawl job (Target = the partner).

const (
	// fightRecoverEvery is how often (ticks) a fight past its minimum
	// rolls to end, fightRecoverChance the chance (permille).
	fightRecoverEvery  = 60
	fightRecoverChance = 150
)

// StartFight puts starter and other into a brawl with each other.
func (m *Mind) StartFight(starter, other entity.EntityID) {
	h := m.h
	until := h.Now() + social.FightMinTicks
	for _, p := range [2][2]entity.EntityID{{starter, other}, {other, starter}} {
		h.Seize(p[0])
		m.forget(p[0]) // a spree that ends in a fight
		h.SetBreak(p[0], mentalbreak.SocialFight, until)
		h.Brawl(p[0], p[1])
	}
	h.FightStarted(starter, other)
}

// InFight reports id is in a social fight.
func (m *Mind) InFight(id entity.EntityID) bool {
	k, _, ok := m.h.Break(id)
	return ok && k == mentalbreak.SocialFight
}

// partner is who id is fighting (0: unknown): its brawl job's target,
// else the fighter whose brawl targets it, else the one other fighter who
// started at the same tick and lost its job too.
func (m *Mind) partner(id entity.EntityID) entity.EntityID {
	h := m.h
	if j := h.Job(id); j.Kind == job.Brawl && j.Target != 0 {
		return j.Target
	}
	_, until, _ := h.Break(id)
	var orphan entity.EntityID
	for _, p := range h.Colonists() {
		if p == id || !m.InFight(p) {
			continue
		}
		pj := h.Job(p)
		if pj.Kind == job.Brawl && pj.Target == id {
			return p
		}
		if _, pu, _ := h.Break(p); orphan == 0 && pu == until && (pj.Kind != job.Brawl || pj.Target == 0) {
			orphan = p
		}
	}
	return orphan
}

// fight runs a fight for id in the AI phase: it ends the fight when the
// partner is gone or the recovery roll comes up, else keeps the brawl
// going. It reports whether id is still fighting.
func (m *Mind) fight(id entity.EntityID, j job.Record, now, until uint64) bool {
	h := m.h
	foe := m.partner(id)
	if foe == 0 || !m.InFight(foe) || !h.Standing(foe) || !h.Standing(id) {
		m.EndFight(id, foe)
		return false
	}
	// the lower id of the pair rolls, once per period past the minimum
	if id < foe && now >= until && (now-until)%fightRecoverEvery == 0 && h.Roll(1000) < fightRecoverChance {
		m.EndFight(id, foe)
		return false
	}
	if j.Kind != job.Brawl || j.Target != foe {
		h.Brawl(id, foe)
		j = h.Job(id)
	}
	if !m.close(id, foe, j, now) {
		m.EndFight(id, foe) // they can't reach each other
		return false
	}
	return true
}

// EndFight ends id's fight with foe (0: unknown) for both: the breaks and
// brawl jobs go, and each fighter rolls its own outcome memory of the
// other.
func (m *Mind) EndFight(id, foe entity.EntityID) {
	h := m.h
	fighters := [2]entity.EntityID{id}
	n := 1
	if foe != 0 && foe != id && m.InFight(foe) && m.partner(foe) == id {
		fighters[1], n = foe, 2
		if foe < id {
			fighters[0], fighters[1] = foe, id
		}
	}
	for _, f := range fighters[:n] {
		h.ClearBreak(f)
		h.StopBrawl(f)
	}
	if n == 2 {
		h.Outcome(fighters[0], fighters[1])
		h.Outcome(fighters[1], fighters[0])
	}
}

// close closes in on foe for id's brawl job j: a fighter next to it (or
// one who won't fight, who just stands) holds its ground for Brawl;
// otherwise it paths to it. False when foe can't be reached.
func (m *Mind) close(id, foe entity.EntityID, j job.Record, now uint64) bool {
	h := m.h
	x, y, _ := h.Pos(id)
	fx, fy, ok := h.Pos(foe)
	if !ok {
		return false
	}
	if !h.CanViolence(id) || weapon.InRange(weapon.DefOf(weapon.Fists), weapon.Distance(x, y, fx, fy)) {
		if j.State != job.Do || j.Path != nil {
			j.State, j.Path, j.PathI = job.Do, nil, 0
			h.SetJob(id, j)
		}
		return true
	}
	stale := j.State != job.Goto || j.PathI >= len(j.Path) ||
		weapon.Distance(j.TX, j.TY, fx, fy) > 1 || (now+uint64(id))%repathTicks == 0
	if !stale {
		return true
	}
	path, ok := h.PathTo(id, fx, fy)
	if !ok {
		return false
	}
	j.Path, j.PathI, j.TX, j.TY, j.State = path, 0, fx, fy, job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	h.SetJob(id, j)
	return true
}

// Brawl runs id's brawl job j in the game's job phase: next to its
// partner a fighter who can do violence throws a punch on the fists'
// cooldown, whatever weapon it carries. One who can't just stands there.
func (m *Mind) Brawl(id entity.EntityID, j job.Record) {
	h := m.h
	if !m.InFight(id) {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	foe := j.Target
	if !h.Fighter(foe) || !m.InFight(foe) {
		m.EndFight(id, foe)
		return
	}
	if !h.CanViolence(id) {
		return
	}
	x, y, _ := h.Pos(id)
	fx, fy, _ := h.Pos(foe)
	if !weapon.InRange(weapon.DefOf(weapon.Fists), weapon.Distance(x, y, fx, fy)) {
		j.State = job.Goto // fight re-paths next tick
		h.SetJob(id, j)
		return
	}
	if j.WorkLeft > 0 {
		j.WorkLeft--
	}
	if j.WorkLeft > 0 {
		h.SetJob(id, j)
		return
	}
	h.Punch(id, foe)
	if !m.InFight(id) {
		return // the blow ended the fight (EndFight reset the job)
	}
	j.WorkLeft = h.PunchTicks(id)
	h.SetJob(id, j)
}
