package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// Social fights (ENG-003, M6): an insult (4%) or a slight (0.5%) may make
// its recipient start a brawl with the one who said it. Both fighters
// enter the SocialFight break (no orders, no work, no drafting) and trade
// fist blows through the normal strike/hurt path for at least
// FightMinTicks. The fight ends at once when either goes down, dies or
// leaves; after the minimum, a recovery roll every fightRecoverEvery
// ticks ends it. Each fighter then rolls its own outcome: an angering or
// a cathartic opinion memory of the other. That memory is also the pair
// cooldown: no new fight between them for FightPairCooldown after one
// ends (no flap). A social fight is not a mood break: it gives no
// catharsis.
//
// State lives only in saved records: the break (kind, until = start +
// FightMinTicks) and the brawl job (Target = the partner). Hit, recovery
// and outcome rolls come from the social stream, never g.rng.

const (
	// fightRecoverEvery is how often (ticks) a fight past its minimum
	// rolls to end, fightRecoverChance the chance (permille).
	fightRecoverEvery  = 60
	fightRecoverChance = 150
	// brawlRepath: a chasing fighter re-paths this often.
	brawlRepath = 60
)

// inSocialFight reports whether id is in a social fight (M6).
func (g *Game) inSocialFight(id entity.EntityID) bool {
	b, ok := g.breaks[id]
	return ok && b.kind == mentalbreak.SocialFight
}

// hungerBand maps food to the malnutrition bands MalnutritionMult takes:
// 0 fed (above needs.CriticalHunger), 1 hungry, 2 very hungry (half of
// it), 3 ravenous (a quarter), 4 starving (empty).
func hungerBand(hunger int) int {
	switch {
	case hunger > needs.CriticalHunger:
		return 0
	case hunger > needs.CriticalHunger/2:
		return 1
	case hunger > needs.CriticalHunger/4:
		return 2
	case hunger > 0:
		return 3
	}
	return 4
}

// fightChanceFor is the permille chance that b, on the receiving end of
// interaction k, starts a fight: the base chance scaled by its
// Bloodthirsty trait, its hunger band and its drink (alcohol band and
// withdrawal).
func (g *Game) fightChanceFor(b entity.EntityID, k social.Interaction) int32 {
	if social.FightChance(k) == 0 {
		return 0
	}
	mults := make([]int32, 0, 3)
	if bio, ok := g.personBio(b); ok {
		for _, t := range bio.Traits {
			if t == trait.Bloodthirsty {
				mults = append(mults, social.BloodthirstyMult)
				break
			}
		}
	}
	n, _ := g.need.Get(b)
	mults = append(mults, social.MalnutritionMult(hungerBand(n.Hunger)))
	if s := g.drugs[b]; s != nil {
		mults = append(mults, s.FightMultiplier())
	}
	return social.FightChance(k, mults...)
}

// fightCandidate reports id can be in a fight now: a standing colony
// member, not drafted, not already breaking.
func (g *Game) fightCandidate(id entity.EntityID) bool {
	if !g.standingPawn(id) || g.isRaider(id) || g.drafted(id) {
		return false
	}
	if p := g.personOf(id); p != nil && p.Where != whereColony {
		return false
	}
	_, breaking := g.breaks[id]
	return !breaking
}

// fightCooldown reports a and b fought within the last FightPairCooldown:
// either holds a fight-outcome memory of the other younger than that.
func (g *Game) fightCooldown(a, b entity.EntityID, now uint64) bool {
	e := g.edgeOf(a, b)
	if e == nil {
		return false
	}
	for d := range e.mem {
		for _, m := range e.mem[d] {
			if m.Kind != social.MemAngeringFight && m.Kind != social.MemCatharticFight || m.Expires <= now {
				continue
			}
			if added := m.Expires - social.MemDef(m.Kind).Duration; now < added+social.FightPairCooldown {
				return true
			}
		}
	}
	return false
}

// maybeSocialFight rolls whether interaction k that a started with b
// turns into a fight b starts (M6).
func (g *Game) maybeSocialFight(a, b entity.EntityID, k social.Interaction) {
	if a == b || social.FightChance(k) == 0 {
		return
	}
	if !g.canViolence(b) || !g.fightCandidate(a) || !g.fightCandidate(b) {
		return // a pawn that can't do violence never starts a fight
	}
	if g.fightCooldown(a, b, uint64(g.clock.Now())) {
		return
	}
	if int32(g.stream(streamSocial).IntN(1000)) >= g.fightChanceFor(b, k) {
		return
	}
	g.startSocialFight(b, a)
}

// startSocialFight puts starter and other into a brawl with each other.
func (g *Game) startSocialFight(starter, other entity.EntityID) {
	now := uint64(g.clock.Now())
	label := mentalbreak.DefOf(mentalbreak.SocialFight).Label
	for _, p := range [2][2]entity.EntityID{{starter, other}, {other, starter}} {
		id, foe := p[0], p[1]
		g.interrupt(id, job.Cancelled)
		delete(g.draft, id)
		delete(g.orderQueue, id)
		g.breaks[id] = breakState{kind: mentalbreak.SocialFight, until: now + social.FightMinTicks}
		g.setBrawl(id, foe)
		g.logEvent(id, logEntry{Kind: logMental, Name: label, Other: foe})
	}
	pos, _ := g.positions.Get(starter)
	text := fmt.Sprintf("%s and %s are fighting!", g.name(starter), g.name(other))
	g.post(MessageView{Tick: now, Text: text, X: pos.X, Y: pos.Y, Danger: true, Focus: starter, Letter: true, Title: "Social fight"})
	g.addHistory("fight", fmt.Sprintf("%s got into a fight with %s.", g.name(starter), g.name(other)), starter, other)
}

// setBrawl gives id a fresh brawl job against foe, set directly (the
// two-pawn pattern: no reservation, see mirrorChat).
func (g *Game) setBrawl(id, foe entity.EntityID) {
	fp, _ := g.positions.Get(foe)
	g.jobs.Set(id, job.Record{Kind: job.Brawl, State: job.Goto, Target: foe, TX: fp.X, TY: fp.Y,
		WorkLeft: aimTicks(weapon.DefOf(weapon.Fists))})
}

// fightPartner is who id is fighting (0: unknown): its brawl job's
// target, else the fighter whose brawl targets it, else the one other
// fighter who started at the same tick and lost its job too.
func (g *Game) fightPartner(id entity.EntityID) entity.EntityID {
	if j, _ := g.jobs.Get(id); j.Kind == job.Brawl && j.Target != 0 {
		return j.Target
	}
	until := g.breaks[id].until
	var orphan entity.EntityID
	for _, p := range g.pawnIDs {
		if p == id || !g.inSocialFight(p) {
			continue
		}
		pj, _ := g.jobs.Get(p)
		if pj.Kind == job.Brawl && pj.Target == id {
			return p
		}
		if orphan == 0 && g.breaks[p].until == until && (pj.Kind != job.Brawl || pj.Target == 0) {
			orphan = p
		}
	}
	return orphan
}

// fightTick runs a fight for id in phaseAI (mentalBreakTick): it ends the
// fight when the partner is gone or the recovery roll comes up, else
// keeps the brawl going. It reports whether id is still fighting.
func (g *Game) fightTick(id entity.EntityID, j job.Record, now uint64) bool {
	foe := g.fightPartner(id)
	if foe == 0 || !g.inSocialFight(foe) || !g.standingPawn(foe) || !g.standingPawn(id) {
		g.endSocialFight(id, foe)
		return false
	}
	b := g.breaks[id]
	// the lower id of the pair rolls, once per period past the minimum
	if id < foe && now >= b.until && (now-b.until)%fightRecoverEvery == 0 &&
		g.stream(streamSocial).IntN(1000) < fightRecoverChance {
		g.endSocialFight(id, foe)
		return false
	}
	if j.Kind != job.Brawl || j.Target != foe {
		g.setBrawl(id, foe)
		j, _ = g.jobs.Get(id)
	}
	if !g.brawlApproach(id, foe, j, now) {
		g.endSocialFight(id, foe) // they can't reach each other
		return false
	}
	return true
}

// brawlApproach closes in on foe: a fighter next to it (or one who won't
// fight, who just stands) holds its ground for doBrawl; otherwise it
// paths to it. False when foe can't be reached.
func (g *Game) brawlApproach(id, foe entity.EntityID, j job.Record, now uint64) bool {
	pos, _ := g.positions.Get(id)
	fp, ok := g.positions.Get(foe)
	if !ok {
		return false
	}
	if !g.canViolence(id) || weapon.InRange(weapon.DefOf(weapon.Fists), weapon.Distance(pos.X, pos.Y, fp.X, fp.Y)) {
		if j.State != job.Do || j.Path != nil {
			j.State, j.Path, j.PathI = job.Do, nil, 0
			g.jobs.Set(id, j)
		}
		return true
	}
	stale := j.State != job.Goto || j.PathI >= len(j.Path) ||
		weapon.Distance(j.TX, j.TY, fp.X, fp.Y) > 1 || (now+uint64(id))%brawlRepath == 0
	if !stale {
		return true
	}
	path, ok := g.pathTo(id, fp.X, fp.Y)
	if !ok {
		return false
	}
	j.Path, j.PathI, j.TX, j.TY, j.State = path, 0, fp.X, fp.Y, job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	g.jobs.Set(id, j)
	return true
}

// doBrawl runs a social fight in phaseJobs: next to its partner a fighter
// who can do violence throws a punch on the fists' cooldown, whatever
// weapon it carries. One who can't just stands there.
func (g *Game) doBrawl(id entity.EntityID, j job.Record) {
	if !g.inSocialFight(id) {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	foe := j.Target
	if !g.standingPawn(foe) || !g.inSocialFight(foe) {
		g.endSocialFight(id, foe)
		return
	}
	if !g.canViolence(id) {
		return
	}
	pos, _ := g.positions.Get(id)
	fp, _ := g.positions.Get(foe)
	fists := weapon.DefOf(weapon.Fists)
	if !weapon.InRange(fists, weapon.Distance(pos.X, pos.Y, fp.X, fp.Y)) {
		j.State = job.Goto // fightTick re-paths next tick
		g.jobs.Set(id, j)
		return
	}
	if j.WorkLeft > 0 {
		j.WorkLeft--
	}
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.strikeRoll(id, foe, weapon.Fists, g.stream(streamSocial).IntN)
	if !g.inSocialFight(id) {
		return // the blow ended the fight (endSocialFight reset the job)
	}
	j.WorkLeft = g.aimTicksFor(id, fists.Cooldown)
	g.jobs.Set(id, j)
}

// endSocialFight ends id's fight with foe (0: unknown) for both: the
// breaks and brawl jobs go, and each fighter still alive rolls its own
// outcome memory of the other. No catharsis: it was not a mood break.
func (g *Game) endSocialFight(id, foe entity.EntityID) {
	fighters := []entity.EntityID{id}
	if foe != 0 && foe != id && g.inSocialFight(foe) && g.fightPartner(foe) == id {
		fighters = append(fighters, foe)
		if foe < id {
			fighters[0], fighters[1] = foe, id
		}
	}
	now := uint64(g.clock.Now())
	for _, f := range fighters {
		delete(g.breaks, f)
		if j, _ := g.jobs.Get(f); j.Kind == job.Brawl {
			g.interrupt(f, job.Cancelled)
		}
	}
	if len(fighters) < 2 {
		return
	}
	r := g.stream(streamSocial)
	for i, f := range fighters {
		other := fighters[1-i]
		if v, ok := g.vital.Get(f); !ok || v.Status == health.Dead {
			continue // the dead remember nothing
		}
		g.ensurePerson(f)
		g.ensurePerson(other)
		g.addOpinionMemory(f, other, social.RollOutcome(r.IntN).Memory(), now)
	}
}

// fightOnDown ends target's fight when it can no longer stand (hurt,
// dieBy).
func (g *Game) fightOnDown(target entity.EntityID) {
	if g.inSocialFight(target) && !g.standingPawn(target) {
		g.endSocialFight(target, g.fightPartner(target))
	}
}
