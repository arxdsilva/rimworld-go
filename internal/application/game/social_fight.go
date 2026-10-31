package game

import (
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
// ends (no flap). The fight's course and blows run in mind/fight.go
// (SIM-066); this file rolls whether one starts.
// Hit, recovery and outcome rolls come from the social stream.

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
	mults = append(mults, social.MalnutritionMult(needs.FightBand(n.Hunger)))
	if s := g.drugs[b]; s != nil {
		mults = append(mults, s.FightMultiplier())
	}
	return social.FightChance(k, mults...)
}

// fightCandidate reports id can be in a fight now: a standing colony
// member, not drafted, not already breaking (an insulting spree may end
// in a fight: SIM-066).
func (g *Game) fightCandidate(id entity.EntityID) bool {
	if !g.standingPawn(id) || g.isRaider(id) || g.drafted(id) {
		return false
	}
	if p := g.personOf(id); p != nil && p.Where != whereColony {
		return false
	}
	b, breaking := g.breaks[id]
	return !breaking || b.kind == mentalbreak.InsultingSpree || b.kind == mentalbreak.TargetedInsult
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
	g.mind.StartFight(b, a)
}
