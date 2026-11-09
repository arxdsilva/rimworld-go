package rage

import (
	"strings"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
)

// PLAY-017: the player is warned (and the game auto-pauses, the game's
// threat_ui.go) when an animal starts chasing a colonist.

// chaseRewarn: after a warned chase ends, the same animal chasing the
// same colonist is warned again only this long after (hysteresis, so
// charges rolling every animal.ChargeCooldown never re-pause the game).
const chaseRewarn = clock.TicksPerHour

// ChaseTarget is the colonist animal a is chasing (0 = none).
func (r *Rage) ChaseTarget(a animal.State) entity.EntityID {
	if a.Mode == animal.Attack && a.Target != 0 && r.h.IsPawn(a.Target) && !r.h.IsRaider(a.Target) {
		return a.Target
	}
	return 0
}

// NoteChase is called where animal aid starts chasing colonist target
// (the game's combat.go: it turned on the colonist who hurt it). It works
// on the stored animal; thinking code holding its own copy calls
// WarnChase.
func (r *Rage) NoteChase(aid, target entity.EntityID) {
	a, ok := r.h.Animal(aid)
	if !ok {
		return
	}
	if r.WarnChase(aid, &a, target, false) {
		r.h.SetAnimal(aid, a)
	}
}

// WarnChase posts the danger message for animal id (state a) starting to
// chase colonist target: "A timber wolf is chasing Ada!", or "A cougar
// charges Ada!" for a charge. Anyone but a colonist, an enraged animal
// (its rage has its own message) and a chase already warned are skipped.
// It reports whether a changed.
func (r *Rage) WarnChase(id entity.EntityID, a *animal.State, target entity.EntityID, charge bool) bool {
	now := r.h.Now()
	if target == 0 || a.Enraged(now) || !r.h.Colonist(target) { // colonists only: never raiders, visitors or prey
		return false
	}
	if target == a.ChaseWarnedTarget && a.ChaseWarnedAt != 0 && now-a.ChaseWarnedAt < chaseRewarn {
		a.ChaseWarnedAt = now // the same chase, or back on it too soon: no new warning
		return true
	}
	a.ChaseWarnedTarget, a.ChaseWarnedAt = target, now
	p, _ := r.h.Pos(id)
	r.h.Alarm(now, p.X, p.Y, id, ChaseText(a.Species, r.h.Name(target), charge), msgfmt.Text{})
	return true
}

// KeepChaseWarned marks a warned chase still on (the game's phaseAnimals,
// every tick): the re-warn hour counts from when the chase ends.
func KeepChaseWarned(a *animal.State, now uint64) {
	if a.ChaseWarnedTarget != 0 && a.Mode == animal.Attack && a.Target == a.ChaseWarnedTarget {
		a.ChaseWarnedAt = now
	}
}

// chaseLabel is how a chase message names species s ("timber wolf").
func chaseLabel(s animal.Species) string { return s.Label() }

// ChaseText is the warning: "A timber wolf is chasing Ada!", "An arctic
// fox is chasing Ada!", or for a charge "A cougar charges Ada!".
func ChaseText(s animal.Species, who string, charge bool) string {
	l := WithArticle(chaseLabel(s))
	l = strings.ToUpper(l[:1]) + l[1:]
	if charge {
		return l + " charges " + who + "!"
	}
	return l + " is chasing " + who + "!"
}

// WithArticle is label with "a" or "an" ("a hare", "an ibex").
func WithArticle(label string) string {
	if label != "" && strings.ContainsRune("aeiou", rune(label[0])) {
		return "an " + label
	}
	return "a " + label
}
