package combat

import (
	"cmp"
	"slices"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/entity"
)

// Gear (CBT-3): what a fighter carries beyond the weapon kind the game
// keeps. The wielded weapon is a stack: its quality, stuff and wear ride
// here beside the game's kind, and count only while that kind is still
// wielded (an entry for another kind is stale and ignored), so every
// path that arms a creature without a stack (raids, dev hooks, caskets)
// wields a Normal weapon of the def's own steel. Old saves carry no
// entries: their weapons load as that. Gear also keeps the shield belts'
// charge, the smokepop packs' uses and the fighters' records (kills and
// damage dealt).
//
// Shield belt: a charged belt stops every ranged and explosive hit
// (bullets, arrows, bombs, flame) at 3.3 energy per point of damage; one
// that runs dry, or meets an EMP, breaks and comes back 3,200 ticks
// later at 20 energy, then recharges 13 a second up to 110 (scaled by
// quality: 66 Awful, 231 Legendary). Its wearer cannot shoot out: a gun
// is a club (Reach). The charge is computed when asked from the last
// change (no tick work). wiki: Shield belt
//
// Smokepop pack: three uses; each pops a smoke cloud of 4.9 tiles at
// Normal (4.1 Awful .. 7.4 Legendary) on the wearer. A raider pops it
// when a ranged hit strikes it; a colonist on the player's order (Pop).
// wiki: Smokepop pack
type Gear struct {
	h      Host
	arms   Arms
	belts  Belts
	blasts *Blasts
	drafts *Drafts
	set    map[entity.EntityID]Kit // saved
	near   []Seen
}

// Belts is what creatures wear at the belt (CBT-3).
type Belts interface {
	// Belt is id's belt-layer apparel and its stored quality (k 0: none).
	Belt(id entity.EntityID) (k item.Kind, q uint8)
}

// Kit is one fighter's gear state. All fields are exported for the save.
type Kit struct {
	// Held is the wielded weapon's make: the kind it was taken as, its
	// stored quality, stuff and wear.
	Held item.Stack
	// Shield is the shield belt's charge.
	Shield Charge
	// Pops is the smokepop uses spent; Kills and Damage the records.
	Pops, Kills, Damage int
	// LastBy is who last wounded this fighter: a death of its wounds
	// later (blood loss) is that attacker's kill.
	LastBy entity.EntityID
}

// Charge is a shield belt's energy (thousandths) at tick At; a broken
// belt is down until ResetAt. Live false: never struck (full).
type Charge struct {
	Energy      int
	At, ResetAt uint64
	Live        bool
}

// KitSave is one Kit on disk.
type KitSave struct {
	ID entity.EntityID
	K  Kit
}

// The shield belt's and smokepop pack's numbers. wiki: Shield belt,
// Smokepop pack
const (
	ShieldMax       = 110_000 // energy at Normal, thousandths
	shieldRecharge  = 13_000  // energy a second at Normal, thousandths
	shieldPerDamage = 3_300   // energy a point of damage costs, thousandths
	ShieldReset     = 3_200   // ticks a broken belt is down
	shieldOnReset   = 20_000  // energy a belt comes back with
	SmokepopUses    = 3
)

// Quality factors (permille, Awful..Legendary): the belt's energy (the
// wiki's 66 and 231 at the ends; the steps between are original
// interpolation), its recharge (11.7 and 16.9 at the ends; between,
// original) and the smokepop's radius (all wiki, tenths of a tile).
var (
	shieldMaxQ  = [...]int{600, 800, 1000, 1150, 1300, 1550, 2100}
	shieldRateQ = [...]int{900, 950, 1000, 1050, 1100, 1200, 1300}
	smokepopR   = [...]int{4100, 4500, 4900, 5300, 5700, 6400, 7400}
)

// level is stored quality q (Normal when none).
func level(q uint8) quality.Level {
	if l, ok := quality.FromStored(q); ok {
		return l
	}
	return quality.Normal
}

func (g *Gear) put(id entity.EntityID, k Kit) {
	if k == (Kit{}) {
		delete(g.set, id)
		return
	}
	if g.set == nil {
		g.set = map[entity.EntityID]Kit{}
	}
	g.set[id] = k
}

// Hold records the stack id just took in hand (one item of it).
func (g *Gear) Hold(id entity.EntityID, st item.Stack) {
	k := g.set[id]
	k.Held = item.Stack{Kind: st.Kind, Quality: st.Quality, Stuff: st.Stuff, Wear: st.Wear}
	g.put(id, k)
}

// held is id's wielded weapon's make (zero: none recorded, or stale).
func (g *Gear) held(id entity.EntityID) item.Stack {
	k, ok := g.set[id]
	if !ok || k.Held.Kind == 0 || g.arms == nil || g.h.Weapon(id) == weapon.Fists || g.arms.Wielded(id) != k.Held.Kind {
		return item.Stack{}
	}
	return k.Held
}

// Make is how id's wielded weapon was made (zero: Normal, the def's
// steel).
func (g *Gear) Make(id entity.EntityID) weapon.Held {
	st := g.held(id)
	return weapon.Held{Quality: st.Quality, Stuff: st.Stuff}
}

// Release is n of kind k leaving id's hands as a stack (its make kept
// when k is the weapon recorded); the record is dropped.
func (g *Gear) Release(id entity.EntityID, k item.Kind, n int) item.Stack {
	st := item.Stack{Kind: k, Qty: n}
	if kit, ok := g.set[id]; ok && kit.Held.Kind == k {
		st.Quality, st.Stuff, st.Wear = kit.Held.Quality, kit.Held.Stuff, kit.Held.Wear
		kit.Held = item.Stack{}
		g.put(id, kit)
	}
	return st
}

// Def is id's profile with wk as made (quality and stuff, Held.Tuned).
func (g *Gear) Def(id entity.EntityID, wk weapon.Kind) weapon.Def {
	return g.Make(id).Tuned(weapon.DefOf(wk))
}

// Reach is Def as id can use it: a shield belt's wearer cannot shoot out,
// so its gun reaches only a target beside it (its stock).
func (g *Gear) Reach(id entity.EntityID, wk weapon.Kind) weapon.Def {
	d := g.Def(id, wk)
	if d.Ranged && g.Shielded(id) {
		d.Ranged, d.Range, d.MinRange, d.Bands, d.Speed, d.Blast = false, 1, 0, weapon.Def{}.Bands, 0, explosion.Def{}
	}
	return d
}

// Spend uses up id's single-use weapon wk (a rocket launcher fired its
// burst): its hands are empty. It reports whether it did.
func (g *Gear) Spend(id entity.EntityID, wk weapon.Kind) bool {
	if !weapon.SingleUse(wk) || g.arms == nil || g.h.Weapon(id) != wk {
		return false
	}
	g.Release(id, g.arms.Wielded(id), 1)
	g.arms.Disarm(id)
	return true
}

// belt is id's belt and its quality level (k 0: none).
func (g *Gear) belt(id entity.EntityID) (item.Kind, quality.Level) {
	if g.belts == nil {
		return 0, quality.Normal
	}
	k, q := g.belts.Belt(id)
	return k, level(q)
}

// Shielded: id wears a shield belt.
func (g *Gear) Shielded(id entity.EntityID) bool {
	k, _ := g.belt(id)
	return k == item.ShieldBelt
}

// charge is id's shield energy now and its most (thousandths; 0, 0
// without a shield belt).
func (g *Gear) charge(id entity.EntityID) (e, most int) {
	k, l := g.belt(id)
	if k != item.ShieldBelt {
		return 0, 0
	}
	most = ShieldMax * shieldMaxQ[l] / 1000
	s := g.set[id].Shield
	now := g.h.Now()
	switch {
	case !s.Live:
		return most, most
	case now < s.ResetAt:
		return 0, most
	}
	e, from := s.Energy, s.At
	if s.ResetAt > s.At {
		e, from = shieldOnReset, s.ResetAt
	}
	return min(most, e+int(now-from)*shieldRecharge*shieldRateQ[l]/60_000), most
}

// setCharge stores id's shield energy e now; broken, it goes down for
// ShieldReset ticks.
func (g *Gear) setCharge(id entity.EntityID, e int, broken bool) {
	k := g.set[id]
	now := g.h.Now()
	k.Shield.Energy, k.Shield.At, k.Shield.Live = max(e, 0), now, true
	if broken {
		k.Shield.Energy, k.Shield.ResetAt = 0, now+ShieldReset
	}
	g.put(id, k)
}

// ShieldPct is id's shield charge, percent of its most (0 without one).
func (g *Gear) ShieldPct(id entity.EntityID) int {
	e, most := g.charge(id)
	if most == 0 {
		return 0
	}
	return e * 100 / most
}

// stops reports a hit a shield belt stops: ranged and explosive damage
// (EMP is apart: it breaks the belt).
func stops(f damage.Form) bool {
	switch f {
	case damage.FormBullet, damage.FormArrow, damage.FormBomb, damage.FormFlame:
		return true
	}
	return false
}

// Shield is the hit package's shield hook (game/hit Shields): hit about
// to strike target. A raider with a smokepop pops it when a ranged hit
// strikes it; a charged shield belt stops a ranged or explosive hit (true:
// nothing reaches the body) and breaks when it runs dry; an EMP breaks it.
func (g *Gear) Shield(target entity.EntityID, hit damage.Hit) bool {
	k, _ := g.belt(target)
	switch {
	case k == item.SmokepopPack && (hit.Form == damage.FormBullet || hit.Form == damage.FormArrow) && g.h.StandingRaider(target):
		g.Pop(target)
		return false
	case k != item.ShieldBelt:
		return false
	case hit.Form == damage.FormEMP:
		g.setCharge(target, 0, true)
		return true
	case !stops(hit.Form):
		return false
	}
	e, _ := g.charge(target)
	if e <= 0 {
		return false // down: the hit goes through
	}
	e -= max(hit.Amount, 0) * shieldPerDamage
	g.setCharge(target, e, e < 0)
	return true
}

// EMP breaks the shield belts of everyone within an EMP blast of radius
// (thousandths of a tile) at (x, y).
func (g *Gear) EMP(x, y, radius int) {
	d := explosion.Def{Kind: explosion.EMP, Radius: radius}
	g.near = g.h.Near(x, y, d.Reach(), Pawns|Raiders, g.near[:0])
	slices.SortFunc(g.near, func(a, b Seen) int { return cmp.Compare(a.ID, b.ID) })
	for _, s := range g.near {
		if d.Within(s.X-x, s.Y-y) && g.Shielded(s.ID) {
			g.setCharge(s.ID, 0, true)
		}
	}
}

// Smokepops is the smokepop uses id has left (0 without a pack).
func (g *Gear) Smokepops(id entity.EntityID) int {
	if k, _ := g.belt(id); k != item.SmokepopPack {
		return 0
	}
	return max(SmokepopUses-g.set[id].Pops, 0)
}

// Pop has id pop its smokepop pack: a smoke cloud on its tile (DEF-2
// smoke). It reports whether it did (no pack, no use left or no tile:
// false).
func (g *Gear) Pop(id entity.EntityID) bool {
	_, l := g.belt(id)
	x, y, ok := g.h.Pos(id)
	if !ok || g.Smokepops(id) == 0 || g.blasts == nil {
		return false
	}
	k := g.set[id]
	k.Pops++
	g.put(id, k)
	g.blasts.Explode(id, weapon.Fists, x, y, explosion.Def{Kind: explosion.Smoke, Radius: smokepopR[l]})
	return true
}

// Count records a hit by by on target that dealt dmg; killed: it was
// the killing blow. A target left alive remembers its attacker (Died).
func (g *Gear) Count(by, target entity.EntityID, dmg int, killed bool) {
	if by == 0 || dmg <= 0 && !killed {
		return
	}
	k := g.set[by]
	k.Damage += max(dmg, 0)
	if killed {
		k.Kills++
	}
	g.put(by, k)
	if t := g.set[target]; !killed && target != 0 {
		t.LastBy = by
		g.put(target, t)
	}
}

// Died: id died, killer dealing the last blow (0: of its wounds, a
// turret or no one). With no killer the kill is its last attacker's; a
// killing blow was counted by Count.
func (g *Gear) Died(id, killer entity.EntityID) {
	t, ok := g.set[id]
	if !ok || t.LastBy == 0 {
		return
	}
	by := t.LastBy
	t.LastBy = 0
	g.put(id, t)
	if killer == 0 {
		k := g.set[by]
		k.Kills++
		g.put(by, k)
	}
}

// GearView is a fighter's gear and records for its PawnView: the
// wielded weapon's stored quality and stuff ("" for none or the def's
// own), the spare weapon's name ("": none), shield charge percent,
// smokepop uses left, kills and damage dealt.
type GearView struct {
	Quality       uint8
	Stuff, Spare  string
	ShieldPct     int
	Smokepops     int
	Kills, Damage int
}

// View is id's GearView.
func (g *Gear) View(id entity.EntityID) GearView {
	st, k := g.held(id), g.set[id]
	v := GearView{Quality: st.Quality, ShieldPct: g.ShieldPct(id), Smokepops: g.Smokepops(id), Kills: k.Kills, Damage: k.Damage}
	if st.Stuff != 0 {
		v.Stuff = st.Stuff.String()
	}
	if g.drafts != nil {
		if sk, _, ok := g.drafts.Spare(id); ok {
			v.Spare = sk.String()
		}
	}
	return v
}

// export is the gear state for the save, ascending by id: a weapon make
// only while that weapon is still wielded.
func (g *Gear) export() []KitSave {
	var out []KitSave
	for id, k := range g.set {
		if k.Held.Kind != 0 && g.held(id).Kind == 0 {
			k.Held = item.Stack{}
		}
		if k != (Kit{}) {
			out = append(out, KitSave{ID: id, K: k})
		}
	}
	slices.SortFunc(out, func(a, b KitSave) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// importGear restores saved gear state.
func (g *Gear) importGear(s []KitSave) {
	g.set = nil
	for _, v := range s {
		g.put(v.ID, v.K)
	}
}

// KitOutfit is a raider's kit of apparel kinds as an outfit: each piece
// in its slot, Normal and of its default material (domain combat/kit).
func KitOutfit(kinds []item.Kind) craft.Outfit {
	var o craft.Outfit
	for _, k := range kinds {
		if s := k.Slot(); s != item.NoSlot && int(s) < craft.Slots {
			o.On[s] = craft.Piece{Kind: k}
		}
	}
	return o
}
