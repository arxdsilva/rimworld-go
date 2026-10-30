// Package hit lands combat hits on creatures (CBT-1): body part first,
// then the armor covering that part, outermost layer first, then the
// wound the hit's own form leaves, with overkill, stagger and stun.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The
// simulation comes in through the small roles below, which the game
// implements with adapter types; a Wounds value is stateless (the stagger
// lives in the game's motion records), so the game builds one per call.
// Rolls come from the caller (armor) and the body stream (parts, stun).
package hit

import (
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	// StaggerTicks and StaggerSlow: a staggered body steps at a sixth of
	// its speed for 95 ticks. wiki: Combat (stopping power)
	StaggerTicks = 95
	StaggerSlow  = 6
	// StunPerPoint is the ticks of stun per point of stun damage (0.33 s).
	// wiki: Damage types
	StunPerPoint = 20
	// BluntStunTicks is the stun of a hard blunt blow. original tuning:
	// about two thirds of a second
	BluntStunTicks = 40
	// The blunt stun chances: a torso blow of 90 % of its max hit points
	// stuns 15 % of the time, a brain blow of half of its hit points
	// always. wiki: Damage types
	torsoStunAt, torsoStunPct = 900, 15
	brainStunAt               = 500
)

// Bodies is the targets' bodies.
type Bodies interface {
	// Body is humanlike id's living body, made on first use (nil: an
	// animal, the dead, an unknown id).
	Body(id entity.EntityID) *body.Body
	// Plating is a mechanoid's own armor (ok false: not a machine).
	Plating(id entity.EntityID) (armor.Set, bool)
	// Size is id's body size, permille (a human 1000).
	Size(id entity.EntityID) int
	// Roll draws from the body stream.
	Roll(n int) int
}

// Wardrobe is what the targets wear.
type Wardrobe interface {
	Outfit(id entity.EntityID) craft.Outfit
	SetOutfit(id entity.EntityID, o craft.Outfit)
	// Destroyed tells the player id's garment of kind k was shot to rags.
	Destroyed(id entity.EntityID, k item.Kind)
}

// Harm wounds and holds the targets.
type Harm interface {
	Now() uint64
	// Wound deals b to id (by: the attacker, 0 for none): downing, death
	// and every reaction follow as for any hurt.
	Wound(id entity.EntityID, b Blow, by entity.EntityID)
	// StaggerUntil and SetStaggerUntil are id's stagger timer (its
	// motion record).
	StaggerUntil(id entity.EntityID) uint64
	SetStaggerUntil(id entity.EntityID, until uint64)
	// Hold stops id until tick until: no step, and an attack under way
	// waits it out.
	Hold(id entity.EntityID, until uint64)
}

// Blow is one wound: Dmg of Kind, on Part when a hit placed it, with the
// overkill window Lo..Hi (damage.Form.Overkill); an unplaced blow spreads
// over parts picked by coverage.
type Blow struct {
	Dmg    int
	Kind   body.WoundKind
	Part   body.Part
	Placed bool
	Lo, Hi int
}

// Shields may stop a hit before it reaches the body (CBT-3: a shield
// belt; combat.Gear).
type Shields interface {
	// Shield reports that hit, about to strike target, was stopped.
	Shield(target entity.EntityID, hit damage.Hit) bool
}

// Wounds lands hits: it is combat's Wounds role. S (nil: none) stops a
// hit before its part is picked.
type Wounds struct {
	B Bodies
	W Wardrobe
	H Harm
	S Shields
}

// Absorb is hit landing on target: on a humanlike, a part picked by
// coverage and the garments covering it (LandOn); a mechanoid's plating
// covers it whole; any other creature's body picks its own part when hurt.
func (w Wounds) Absorb(target entity.EntityID, hit damage.Hit, roll func(int) int) armor.Landed {
	if w.S != nil && w.S.Shield(target, hit) {
		hit.Amount = 0
		return armor.Landed{Hit: hit, Outcome: armor.Deflected} // a shield belt took it
	}
	if s, ok := w.B.Plating(target); ok {
		return armor.Absorb([]armor.Set{s}, hit, roll)
	}
	b := w.B.Body(target)
	if b == nil {
		return armor.Landed{Hit: hit}
	}
	p, ok := b.PickPart(w.B.Roll)
	if !ok {
		return armor.Landed{Hit: hit}
	}
	return w.LandOn(target, p, hit, roll)
}

// LandOn runs hit on humanlike target's part p through the garments
// covering it, outermost first, and wears each by what reached it
// (craft.Outfit.HitWear); a garment worn to nothing is destroyed.
func (w Wounds) LandOn(target entity.EntityID, p body.Part, hit damage.Hit, roll func(int) int) armor.Landed {
	o := w.W.Outfit(target)
	var worn [craft.Slots]armor.Layer
	var slots [craft.Slots]item.Slot
	n := 0
	for s, pc := range o.On {
		if l, ok := armor.Piece(pc.Kind, pc.Stuff, pc.Quality); ok {
			worn[n], slots[n] = l, item.Slot(s)
			n++
		}
	}
	var took [craft.Slots]int
	l := armor.Struck(worn[:n], armor.GroupOf(p), hit, roll, took[:n])
	l.Part, l.Placed = uint8(p), true
	if n == 0 {
		return l
	}
	var gone [craft.Slots]item.Kind
	for i := range n {
		gone[i] = o.HitWear(slots[i], took[i])
	}
	w.W.SetOutfit(target, o)
	for _, k := range gone[:n] {
		if k != 0 {
			w.W.Destroyed(target, k)
		}
	}
	return l
}

// HurtHit wounds target with what Absorb let through: a stopping hit
// staggers a body no bigger than its stopping power, stun damage stuns
// without a wound, EMP leaves flesh alone, and the hit's form picks the
// wound and its overkill window; a hard blunt blow may stun.
func (w Wounds) HurtHit(target entity.EntityID, l armor.Landed, by entity.EntityID) {
	now := w.H.Now()
	if damage.Staggers(l.Stopping, w.B.Size(target)) {
		w.Stagger(target, now)
	}
	switch {
	case l.Form == damage.FormStun:
		w.H.Hold(target, now+uint64(max(l.Amount, 0)*StunPerPoint))
		return
	case l.Stopped() || l.Form == damage.FormEMP: // EMP stuns machines only (raid/mech.go)
		return
	}
	lo, hi := l.Form.Overkill()
	p := body.Part(l.Part)
	w.H.Wound(target, Blow{Dmg: l.Amount, Kind: body.WoundOf(l.Form), Part: p, Placed: l.Placed, Lo: lo, Hi: hi}, by)
	if l.Placed && l.Form == damage.FormBlunt && w.bluntStuns(p, l.Amount) {
		w.H.Hold(target, now+BluntStunTicks)
	}
}

// bluntStuns rolls whether a blunt blow of dmg on part p stuns (the body
// stream; no roll for other parts or weaker blows).
func (w Wounds) bluntStuns(p body.Part, dmg int) bool {
	switch mx := body.DefOf(p).MaxHP; p {
	case body.Brain:
		return dmg*1000 >= brainStunAt*mx
	case body.Torso:
		return dmg*1000 >= torsoStunAt*mx && w.B.Roll(100) < torsoStunPct
	}
	return false
}

// Stagger slows id for StaggerTicks from now. A stagger is never
// extended while it lasts, so a body under steady fire is not held slow
// by every round: it ends StaggerTicks after it began.
func (w Wounds) Stagger(id entity.EntityID, now uint64) {
	if now < w.H.StaggerUntil(id) {
		return
	}
	w.H.SetStaggerUntil(id, now+StaggerTicks)
}

// Staggered reports a stagger timer of until still running at now.
func Staggered(until, now uint64) bool { return now < until }

// StepCost is a step costing cost ticks at now under a stagger timer of
// until.
func StepCost(cost int, until, now uint64) int {
	if Staggered(until, now) {
		return cost * StaggerSlow
	}
	return cost
}
