package game

import (
	"fmt"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal bonds and masters (ENG-003, M10). A tame animal may bond with a
// colonist: when tamed (training.BondTame), on a successful training
// step (BondTrain), when tended (BondTend), or, every
// training.BondCheckTicks in its 60-tick decision cycle (staggered by
// id), with its master or a colonist within bondNear tiles in sight
// (BondProximity). A bond makes that colonist its master when it has
// none. The bonded colonist is glad to be the master and sorry when
// someone else is (state thoughts), and grieves when the animal dies, is
// lost, given away or released (cold-hearted people never do). When the
// bonded colonist dies, the animal wanders confused for a day, or goes
// manhunter (bondManhunterPermille). Every roll draws from the bond
// stream, never g.rng.

const (
	// bondNear: the proximity bond needs the colonist this close, in sight.
	bondNear = 3
	// bondConfusedTicks: a bonded animal whose colonist died wanders this
	// long.
	bondConfusedTicks = clock.TicksPerDay
	// bondManhunterPermille: the chance it goes manhunter instead.
	bondManhunterPermille = 100
	// bondStagger spreads the first proximity checks over the period.
	bondStagger = 977
	// giftGoodwill: a visitor's faction thinks better of the colony for
	// an animal given.
	giftGoodwill = 5
)

// bondRoll draws a permille chance from the bond stream.
func (g *Game) bondRoll(permille int32) bool {
	return int32(g.stream(streamBond).IntN(1000)) < permille
}

// bondable reports whether c can bond with or own an animal: a living
// colonist (not a raider or prisoner).
func (g *Game) bondable(c entity.EntityID) bool {
	if c == 0 || !g.colonist(c) || g.isPrisoner(c) {
		return false
	}
	v, ok := g.vital.Get(c)
	return ok && v.Status != health.Dead
}

// tryBond rolls permille for tame animal id (state a, written back by the
// caller) to bond with colonist c; a bond makes c its master if it has
// none. It reports whether the bond formed.
func (g *Game) tryBond(id entity.EntityID, a *animal.State, c entity.EntityID, permille int32) bool {
	if !a.Tame || a.Bond != 0 || !g.bondable(c) || !g.bondRoll(permille) {
		return false
	}
	a.Bond = uint64(c)
	if a.Master == 0 {
		a.Master = uint64(c)
	}
	p, _ := g.positions.Get(id)
	g.message(p.X, p.Y, "%s has bonded with %s the %s.", g.name(c), a.Name, a.Species.Label())
	return true
}

// onTamed is called when tamer tames animal (ENG-003, M10): a chance of a
// bond with the tamer.
func (g *Game) onTamed(tamer, id entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok {
		return
	}
	if g.tryBond(id, &a, tamer, training.BondTame) {
		g.animals.Set(id, a)
	}
}

// bondDecide is a tame animal's bond, training and healing upkeep in its
// full decision (decideAnimal): it heals on its decision cycle and, every
// BondCheckTicks, may bond with whoever is close. Never per tick.
func (g *Game) bondDecide(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	if !a.Tame {
		return
	}
	if (now+uint64(id))%animal.DecideEvery == 0 {
		g.naturalHeal(id, a, now) // vet_animals.go
	}
	if a.Bond != 0 || now < a.NextBondCheck {
		return
	}
	if a.NextBondCheck == 0 { // first look: spread the checks out
		a.NextBondCheck = now + 1 + uint64(id)*bondStagger%training.BondCheckTicks
		return
	}
	a.NextBondCheck = now + training.BondCheckTicks
	if c := g.bondCompanion(id, *a, pos); c != 0 {
		g.tryBond(id, a, c, training.BondProximity)
	}
}

// bondCompanion is who a proximity bond check is about: the master when
// it is within bondNear in sight, else (with no master) the lowest-id
// colonist that is; 0 for nobody. It looks only in the creature index.
func (g *Game) bondCompanion(id entity.EntityID, a animal.State, pos Position) entity.EntityID {
	near := func(c entity.EntityID, p Position) bool {
		return g.bondable(c) && g.lineOfSight(pos.X, pos.Y, p.X, p.Y)
	}
	if a.Master != 0 {
		m := entity.EntityID(a.Master)
		if p, ok := g.positions.Get(m); ok && chebyshev(pos, p) <= bondNear && near(m, p) {
			return m
		}
		return 0
	}
	var found entity.EntityID
	g.within(pos.X, pos.Y, bondNear, ckPawn, func(c entity.EntityID, p Position) {
		if found == 0 && near(c, p) {
			found = c
		}
	})
	return found
}

// bondMemory gives the colonist bonded with animal id the memory k about
// it (grief: cold-hearted people are exempt).
func (g *Game) bondMemory(bond uint64, id entity.EntityID, k mood.Kind) {
	c := entity.EntityID(bond)
	if !g.bondable(c) {
		return
	}
	if bio, _ := g.personBio(c); social.ExemptFromGrief(bio.Traits) {
		return
	}
	g.memories[c] = g.memories[c].AddFor(k, uint64(id), uint64(g.clock.Now()))
}

// bondThoughts are pawn id's bond state thoughts: glad to be its bonded
// animal's master, sorry when someone else is (one of each at most). The
// lead appends them in thoughts.
func (g *Game) bondThoughts(id entity.EntityID) []mood.Thought {
	master, kept := false, false
	for _, aid := range g.animalIDs {
		a := g.animals.Ref(aid)
		if a == nil || !a.Tame || a.Bond != uint64(id) {
			continue
		}
		switch a.Master {
		case uint64(id):
			master = true
		case 0:
		default:
			kept = true
		}
	}
	var out []mood.Thought
	if master {
		out = append(out, stateThought(mood.BondedMaster))
	}
	if kept {
		if bio, _ := g.personBio(id); !social.ExemptFromGrief(bio.Traits) {
			out = append(out, stateThought(mood.BondedNotMaster))
		}
	}
	return out
}

func stateThought(k mood.Kind) mood.Thought {
	d := mood.DefOf(k)
	return mood.Thought{Kind: k, Label: d.Label, Value: d.Value}
}

// onAnimalDied is called from killAnimal before the animal is removed;
// by is its killer (0: none). The bonded colonist grieves.
func (g *Game) onAnimalDied(id, by entity.EntityID) {
	if a, ok := g.animals.Get(id); ok && a.Bond != 0 {
		g.bondMemory(a.Bond, id, mood.BondedDied)
	}
}

// onAnimalLost: tame animal id left the map for good (walked off, was
// kidnapped, a caravan lost it); the bonded colonist misses it.
func (g *Game) onAnimalLost(id entity.EntityID) {
	if a, ok := g.animals.Get(id); ok && a.Bond != 0 {
		g.bondMemory(a.Bond, id, mood.BondedLost)
	}
}

// bondsOnDeath: pawn dead died. Animals it was master of lose their
// master; those bonded with it lose the bond and wander confused for a
// day, or (bondManhunterPermille) go manhunter. The lead calls it from
// dieBy.
func (g *Game) bondsOnDeath(dead entity.EntityID) {
	if dead == 0 {
		return
	}
	now := uint64(g.clock.Now())
	for _, aid := range g.animalIDs {
		ar := g.animals.Ref(aid)
		if ar == nil || ar.Master != uint64(dead) && ar.Bond != uint64(dead) {
			continue
		}
		a := *ar
		if a.Master == uint64(dead) {
			a.Master = 0
		}
		if a.Bond == uint64(dead) {
			a.Bond = 0
			g.bereave(aid, &a, dead, now)
		}
		g.animals.Set(aid, a)
	}
}

// bereave sends a bonded animal whose colonist died confused, or rarely
// manhunter.
func (g *Game) bereave(id entity.EntityID, a *animal.State, dead entity.EntityID, now uint64) {
	p, _ := g.positions.Get(id)
	a.Path, a.PathI, a.Feed, a.FeedLeft, a.Target = nil, 0, animal.FeedTarget{}, 0, 0
	if g.bondRoll(bondManhunterPermille) {
		a.Manhunter, a.Mode = now+animal.ManhunterTicks, animal.Attack
		g.logEvent(id, logEntry{Kind: logManhunter, Other: dead})
		g.post(MessageView{Tick: now, Text: fmt.Sprintf("%s the %s has gone manhunter with grief for %s!", a.Name, a.Species.Label(), g.name(dead)),
			X: p.X, Y: p.Y, Danger: true, Focus: id})
		return
	}
	a.Mode, a.ConfusedUntil = animal.Confused, now+bondConfusedTicks
	g.message(p.X, p.Y, "%s the %s is lost without %s.", a.Name, a.Species.Label(), g.name(dead))
}

// confusedStep is a confused animal's tick: it wanders aimlessly, pens
// or not, until ConfusedUntil (or hunger ends it sooner). It reports
// false once the confusion is over.
func (g *Game) confusedStep(id entity.EntityID, a *animal.State, pos Position, def animal.Def, now uint64) bool {
	if now >= a.ConfusedUntil || a.HasSex && !animal.NeedsFree(a.Species) && a.Food < animal.FeedStart/2 {
		a.Mode, a.ConfusedUntil, a.NextDecide = animal.Wander, 0, now
		return false
	}
	if a.PathI < len(a.Path) || now < a.NextThink || now < a.RetryAt {
		return true
	}
	r := def.WanderRadius * 2
	rng := g.stream(streamBond)
	a.NextThink = now + 120 + uint64(rng.IntN(240))
	g.animalGoTo(id, a, pos.X+rng.IntN(2*r+1)-r, pos.Y+rng.IntN(2*r+1)-r)
	return true
}

// cmdSetAnimalMaster sets animal's master (0 clears): a tame animal and a
// living colonist.
func (g *Game) cmdSetAnimalMaster(id, master entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame || master != 0 && !g.bondable(master) {
		return
	}
	a.Master = uint64(master)
	g.animals.Set(id, a)
}

// cmdGiftAnimal gives tame animal id away to visitor (who leaves with
// it): the bonded colonist grieves, and the visitor's faction, if known,
// gains giftGoodwill.
func (g *Game) cmdGiftAnimal(id, visitor entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame || visitor == 0 || g.colonist(visitor) {
		return
	}
	p, _ := g.positions.Get(id)
	who := "a visitor"
	if g.personOf(visitor) != nil {
		who = g.personName(visitor)
	}
	g.message(p.X, p.Y, "%s the %s was given to %s.", a.Name, a.Species.Label(), who)
	if a.Bond != 0 {
		g.bondMemory(a.Bond, id, mood.BondedGivenAway)
	}
	if pp := g.personOf(visitor); pp != nil {
		if f := g.factionOf(pp.Faction); f != nil {
			faction.Adjust(f, giftGoodwill, fmt.Sprintf("Day %d: given %s (%+d goodwill)", g.relDay(uint64(g.clock.Now()))+1, a.Name, giftGoodwill))
		}
	}
	g.removeAnimal(id)
}

// unbondReleased: tame animal a is being released; its bonded colonist
// grieves and the bond and master go with it.
func (g *Game) unbondReleased(id entity.EntityID, a *animal.State) {
	if a.Bond != 0 {
		g.bondMemory(a.Bond, id, mood.BondedReleased)
	}
	a.Bond, a.Master = 0, 0
}
