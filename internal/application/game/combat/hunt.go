package combat

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Hunting assigns hunts: moved unchanged from the game's combat.go
// (DEF-3, package budget); the hunt itself is Strikes.Do.
type Hunting struct {
	h  Host
	hu Hunts
}

// huntDownedFirst is added to a standing animal's distance in Assign, so
// any downed one is nearer.
const huntDownedFirst = 1 << 20

// huntTries is the game's workTries (postpone.go): how many animals a
// hunter tries before giving up for this pass.
const huntTries = 8

// Assign sends a hunter after the nearest designated animal (a downed one
// first: the hunter finishes it), first fetching a free ranged weapon if
// it has none.
// The next nearest animal is tried when one can't be started (out of
// reach, postponed: SIM-027), at most huntTries animals.
func (hn *Hunting) Assign(id entity.EntityID) bool {
	h := hn.h
	if !hn.hu.AnyMarked() {
		return false // jobindex.go: nothing marked for hunting
	}
	px, py, _ := h.Pos(id)
	var tried []entity.EntityID
	for range huntTries {
		best, bestD := entity.EntityID(0), 1<<30
		var bx, by int
		for _, aid := range h.AnimalIDs() {
			a, ok := h.Animal(aid)
			if !ok || !a.Designated || a.Dead() {
				continue
			}
			if hn.hu.Reserved(aid) {
				continue
			}
			if slices.Contains(tried, aid) || hn.hu.Postponed(id, aid) {
				continue
			}
			ax, ay, _ := h.Pos(aid)
			d := abs(px-ax) + abs(py-ay)
			if !a.Downed {
				d += huntDownedFirst // a downed animal is finished first (animal_health.go)
			}
			if d < bestD {
				best, bestD, bx, by = aid, d, ax, ay
			}
		}
		if best == 0 {
			return false
		}
		if !Hunter(h.Weapon(id)) { // DEF-2: never hunt with explosives
			return hn.Equip(id, huntArm)
		}
		if h.StartJob(id, job.Record{Kind: job.Hunt, Target: best, TX: bx, TY: by, WorkLeft: AimTicks(weapon.DefOf(h.Weapon(id)))}) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

// Hunter reports a weapon a colonist hunts with (CBT-4, wiki: Hunting):
// any ranged weapon but an explosive one.
func Hunter(wk weapon.Kind) bool {
	d := weapon.DefOf(wk)
	return d.Ranged && !d.Explosive()
}

// huntArm accepts an item a hunter fetches: any ranged, non-explosive
// weapon (CBT-4: not only a rifle).
func huntArm(k item.Kind) bool {
	return (k.IsWeapon() || k.IsContractWeapon()) && Hunter(weapon.ForItem(k))
}

// Equip walks to the nearest free weapon of a kind match accepts.
func (hn *Hunting) Equip(id entity.EntityID, match func(item.Kind) bool) bool {
	sid, x, y := hn.hu.NearestFree(id, match)
	if sid == 0 {
		return false
	}
	return hn.h.StartJob(id, job.Record{Kind: job.Equip, Target: sid, TX: x, TY: y})
}
