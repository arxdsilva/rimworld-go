package game

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017: the player is warned (and the game auto-pauses, threat_ui.go)
// when an animal starts chasing a colonist.

// chaseRewarn: after a warned chase ends, the same animal chasing the
// same colonist is warned again only this long after (hysteresis, so
// charges rolling every animal.ChargeCooldown never re-pause the game).
const chaseRewarn = clock.TicksPerHour

// chaseTarget is the colonist animal a is chasing (0 = none).
func (g *Game) chaseTarget(a animal.State) entity.EntityID {
	if a.Mode == animal.Attack && a.Target != 0 && g.isPawn(a.Target) && !g.isRaider(a.Target) {
		return a.Target
	}
	return 0
}

// noteChase is called where animal aid starts chasing colonist target
// (combat.go: it turned on the colonist who hurt it). It works on the
// stored animal; thinking code holding its own copy calls warnChase.
func (g *Game) noteChase(aid, target entity.EntityID) {
	a, ok := g.animals.Get(aid)
	if !ok {
		return
	}
	if g.warnChase(aid, &a, target, false) {
		g.animals.Set(aid, a)
	}
}

// warnChase posts the danger message for animal id (state a) starting to
// chase colonist target: "A timber wolf is chasing Ada!", or "A cougar
// charges Ada!" for a charge. Anyone but a colonist, an enraged animal
// (its rage has its own message) and a chase already warned are skipped.
// It reports whether a changed.
func (g *Game) warnChase(id entity.EntityID, a *animal.State, target entity.EntityID, charge bool) bool {
	now := uint64(g.clock.Now())
	if target == 0 || a.Enraged(now) || !slices.Contains(g.pawnIDs, target) { // colonists only: never raiders, visitors or prey
		return false
	}
	if target == a.ChaseWarnedTarget && a.ChaseWarnedAt != 0 && now-a.ChaseWarnedAt < chaseRewarn {
		a.ChaseWarnedAt = now // the same chase, or back on it too soon: no new warning
		return true
	}
	a.ChaseWarnedTarget, a.ChaseWarnedAt = target, now
	p, _ := g.positions.Get(id)
	g.post(MessageView{Tick: now, Text: chaseText(a.Species, g.name(target), charge), X: p.X, Y: p.Y, Danger: true, Focus: id})
	return true
}

// keepChaseWarned marks a warned chase still on (phaseAnimals, every
// tick): the re-warn hour counts from when the chase ends.
func keepChaseWarned(a *animal.State, now uint64) {
	if a.ChaseWarnedTarget != 0 && a.Mode == animal.Attack && a.Target == a.ChaseWarnedTarget {
		a.ChaseWarnedAt = now
	}
}

// chaseLabel is how a chase message names species s ("timber wolf").
func chaseLabel(s animal.Species) string { return s.Label() }

// chaseText is the warning: "A timber wolf is chasing Ada!", "An arctic
// fox is chasing Ada!", or for a charge "A cougar charges Ada!".
func chaseText(s animal.Species, who string, charge bool) string {
	l := withArticle(chaseLabel(s))
	l = strings.ToUpper(l[:1]) + l[1:]
	if charge {
		return l + " charges " + who + "!"
	}
	return l + " is chasing " + who + "!"
}

// withArticle is label with "a" or "an" ("a hare", "an ibex").
func withArticle(label string) string {
	if label != "" && strings.ContainsRune("aeiou", rune(label[0])) {
		return "an " + label
	}
	return "a " + label
}
