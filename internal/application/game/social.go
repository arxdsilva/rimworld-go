package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
)

// Social interactions (ENG-003, M3). Every colonist is looked at once per
// socialPeriod ticks, staggered by id; one that is free to talk and past
// its cooldown starts an interaction with the nearest pawn that can hear
// it in the same room (or both outdoors) with line of sight. The choice
// and every roll come from the social stream, never g.rng.

const (
	socialPeriod  = 60
	socialStagger = 7
	// interactCooldown (+ up to interactJitter) is the wait between a
	// pawn's interactions; chatting or relaxing together cuts chatBonus.
	interactCooldown = 1250
	interactJitter   = 250
	chatBonus        = 750
	partnerRadius    = 6
)

// phaseSocial runs after phaseJobs: the daily upkeep, then each due
// colonist's interaction check and mood step.
func (g *Game) phaseSocial() {
	if len(g.pawnIDs) == 0 {
		return
	}
	now := uint64(g.clock.Now())
	if g.soc != nil && now%clock.TicksPerDay == 0 {
		g.socialDay(now)
	}
	g.gatherTick(now) // gathering.go: weddings and parties (M5)
	for _, id := range g.pawnIDs {
		u := uint64(id)
		if (now+u*socialStagger)%socialPeriod == 0 {
			g.socialCheck(id, now)
		}
		if (now+u*moodStagger)%moodPeriod == 0 {
			g.stepMood(id) // social_mood.go
		}
	}
	if g.worldSt != nil {
		// visitors.go: visitors start conversations too (arrival order)
		for _, v := range g.worldSt.visitors {
			if (now+uint64(v.ID)*socialStagger)%socialPeriod == 0 {
				g.socialCheck(v.ID, now)
			}
		}
	}
}

// socialCheck is colonist (or visitor) id's periodic social turn.
func (g *Game) socialCheck(id entity.EntityID, now uint64) {
	if !g.alive(id) {
		return
	}
	g.ensurePerson(id)
	g.refreshSig(id)
	g.tendEdges(id, now)
	if !g.canSocialize(id, true) {
		return
	}
	ready := g.soc.next[id]
	if g.chatting(id) {
		ready -= min(ready, chatBonus)
	}
	if now < ready {
		return
	}
	if b := g.socialPartner(id); b != 0 {
		g.interact(id, b, now)
	}
}

// canSocialize reports id can take part in an interaction: a colonist,
// visitor or prisoner, conscious, awake, not breaking, drafted or
// fighting, and able to talk (initiator) or hear (recipient).
func (g *Game) canSocialize(id entity.EntityID, initiator bool) bool {
	if v, ok := g.vital.Get(id); !ok || v.Status != health.Alive {
		return false
	}
	p := g.personOf(id)
	if p == nil || p.Where != whereColony && p.Where != whereVisitor && p.Where != wherePrisoner {
		return false
	}
	if _, breaking := g.breaks[id]; breaking || g.drafted(id) {
		return false
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Sleep && j.State == job.Do || j.Kind == job.Brawl {
		return false
	}
	if initiator {
		return g.capacity(id, body.Talking) > 0
	}
	return g.capacity(id, body.Hearing) > 0
}

// chatting reports id is chatting or relaxing in company (an interaction
// comes sooner).
func (g *Game) chatting(id entity.EntityID) bool {
	j, _ := g.jobs.Get(id)
	return (j.Kind == job.Chat || j.Kind == job.SocialRelax) && j.State == job.Do
}

// socialPartner is who a talks to: its chat partner when it is chatting,
// else the nearest pawn that can hear it.
func (g *Game) socialPartner(a entity.EntityID) entity.EntityID {
	pa, ok := g.positions.Get(a)
	if !ok {
		return 0
	}
	ra := g.roomAt(pa.X, pa.Y)
	if j, _ := g.jobs.Get(a); j.Kind == job.Chat && j.Target != 0 {
		if pt, ok := g.positions.Get(j.Target); ok && g.canHear(a, pa, ra, j.Target, pt) {
			return j.Target
		}
	}
	b, _, _ := g.nearestWhere(pa.X, pa.Y, partnerRadius, ckPawn|ckRaider, func(id entity.EntityID, p Position) bool {
		return id != a && g.canHear(a, pa, ra, id, p)
	})
	return b
}

// canHear reports b (at pb) can be a's partner: close, free to listen, in
// a's room (or both outdoors) and in sight.
func (g *Game) canHear(a entity.EntityID, pa Position, ra int, b entity.EntityID, pb Position) bool {
	if b == a || max(abs(pa.X-pb.X), abs(pa.Y-pb.Y)) > partnerRadius || !g.canSocialize(b, false) {
		return false
	}
	rb := g.roomAt(pb.X, pb.Y)
	if !(ra >= 0 && ra == rb || ra == room.Outdoors && rb == room.Outdoors) {
		return false
	}
	return g.lineOfSight(pa.X, pa.Y, pb.X, pb.Y)
}

// interact resolves one interaction a starts with b.
func (g *Game) interact(a, b entity.EntityID, now uint64) {
	r := g.stream(streamSocial)
	pa, pb := g.socialPerson(a), g.socialPerson(b)
	ctx := social.InteractCtx{
		Initiator: pa, Recipient: pb,
		OpinionIR: g.opinion(a, b), OpinionRI: g.opinion(b, a),
		Compat: social.Compatibility(g.streams.seed, social.ID(a), social.ID(b), pa.AgeYears, pb.AgeYears),
		Pair:   g.pairOf(a, b).Kind,
		// romance (social_romance.go, M5): an affair weighs less; no
		// attempt while they are a couple or a rebuff cooldown runs
		Partnered: g.partneredExcept(a, b), Attraction: g.romanceAttraction(a, b, now),
	}
	k := social.Choose(r.IntN, ctx)
	switch k {
	case social.RomanceAttempt, social.Breakup, social.Proposal:
		if !g.onRomanceInteraction(k, a, b) {
			k = social.Chitchat // romance not resolved yet (M5): small talk instead
		}
	}
	g.applyInteraction(a, b, k, now)
	g.soc.next[a] = now + interactCooldown + uint64(r.IntN(interactJitter))
}

// onRomanceInteraction resolves a romance attempt, breakup or proposal a
// started with b (M5); false leaves it unresolved.
func (g *Game) onRomanceInteraction(k social.Interaction, a, b entity.EntityID) bool {
	return g.romanceInteraction(k, a, b) // social_romance.go (M5)
}

// humanTie reports by is a person whose deeds others remember (not an
// animal).
func (g *Game) humanTie(by entity.EntityID) bool {
	if by == 0 {
		return false
	}
	_, isAnimal := g.animals.Get(by)
	return !isAnimal
}

// onHarmed is called from hurt when pawn victim takes damage from by: a
// colonist (or visitor, prisoner) remembers who hurt it.
func (g *Game) onHarmed(victim, by entity.EntityID) {
	if victim == by || !g.humanTie(by) || g.isRaider(victim) {
		return
	}
	if _, isAnimal := g.animals.Get(victim); isAnimal {
		return
	}
	g.ensurePerson(victim)
	g.ensurePerson(by)
	g.addOpinionMemory(victim, by, social.MemHarmedMe, uint64(g.clock.Now()))
}

// onRescued is called when rescuer lays patient in a bed: gratitude, more
// of it from a stranger.
func (g *Game) onRescued(rescuer, patient entity.EntityID) {
	if rescuer == patient || rescuer == 0 || patient == 0 {
		return
	}
	g.ensurePerson(rescuer)
	g.ensurePerson(patient)
	k := social.MemRescuedMe
	if g.edgeOf(patient, rescuer) == nil {
		k = social.MemRescuedMeStranger
	}
	g.addOpinionMemory(patient, rescuer, k, uint64(g.clock.Now()))
}

// socialBotchedSurgery: patient holds a botched operation against surgeon.
func (g *Game) socialBotchedSurgery(surgeon, patient entity.EntityID) {
	if surgeon == patient || surgeon == 0 || patient == 0 {
		return
	}
	g.ensurePerson(surgeon)
	g.ensurePerson(patient)
	g.addOpinionMemory(patient, surgeon, social.MemBotchedSurgery, uint64(g.clock.Now()))
}

// doSocialRelax runs a social-recreation job: lingering in company at a
// table or fire, with the joy of a chat (idle.go doIdle).
func (g *Game) doSocialRelax(id entity.EntityID, j job.Record) {
	g.doIdle(id, j)
}

// endStubJob ends a job whose milestone has not filled its runner yet,
// as phaseJobs ends an unknown kind.
func (g *Game) endStubJob(id entity.EntityID, j job.Record) {
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// applyInteraction leaves interaction k's effects (a to b): b's opinion
// memory of a (both ways for a conversation), b's mood, the log line on
// both, and the fight roll. Romance kinds are M5's.
func (g *Game) applyInteraction(a, b entity.EntityID, k social.Interaction, now uint64) {
	if k > social.KindWords {
		return
	}
	eff := social.Effects(k)
	if eff.HasMemory {
		g.addOpinionMemory(b, a, eff.Memory, now)
		if k == social.Chitchat || k == social.DeepTalk {
			g.addOpinionMemory(a, b, eff.Memory, now) // a conversation is mutual
		}
	}
	if eff.HasMood {
		g.memories[b] = g.memories[b].AddFor(eff.Mood, uint64(a), now)
	}
	lk := socialLogKind(k)
	var topic uint8
	if ts := topicsOf(lk); len(ts) > 0 {
		topic = uint8(g.stream(streamSocial).IntN(len(ts)))
	}
	g.addSocialLog(a, b, lk, topic, now)
	g.maybeSocialFight(a, b, k) // social_fight.go (M6)
}
