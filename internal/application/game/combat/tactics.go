package combat

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Tactics (CBT-4) is how fighters use the ground: the best cover within a
// few steps (raiders under fire, drafted colonists with fire at will,
// colonists answering a threat), the grenades about to burst that raiders
// step away from, the colonists a raider weighs as targets, and the first
// door or wall a mechanoid must bash on its way in.
//
// Cover seeking holds: a fighter moves only for CoverMargin more cover
// than it has, and after a move (or a look that found nothing better) it
// does not look again for CoverHold ticks (TestCoverSeekDoesNotFlap).
type Tactics struct {
	h      Host
	sight  *Sight
	f      Field // nil in tests without one: no free-tile check but Near
	br     Breaches
	doors  *Doors
	shots  *Shots
	drafts *Drafts
	seen   []Seen
	out    []Seen // Colonists' answer, reused
	// danger is the explosive shots in flight, rebuilt once a tick.
	danger   []projectile.Shot
	dangerAt uint64
	dangerOK bool
}

const (
	// CoverRadius is how far a raider under fire looks for a firing spot
	// with cover; CoverStep how far a colonist steps (an adjacent tile).
	// CoverMargin is the cover (permille) a new spot must add; CoverHold
	// the ticks between looks. original tuning (wiki: Cover gives the
	// values, not the search).
	CoverRadius = 8
	CoverStep   = 1
	CoverMargin = 150
	CoverHold   = 600
	// coverWalk is the cover a tile of walking costs a raider's choice,
	// so a near spot beats a far one as good.
	coverWalk = 10
)

// BestCover is the free tile within r of (x, y), (x, y) itself included,
// from which fighter id wielding d can hit (tx, ty), with the most cover
// against fire from there (less coverWalk a tile walked; row-major first
// on a tie). here is the cover on (x, y) now; ok false when no tile can
// hit the target.
func (t *Tactics) BestCover(id entity.EntityID, x, y, r int, d weapon.Def, tx, ty int) (cx, cy, best, here int, ok bool) {
	here = t.sight.Cover(tx, ty, x, y)
	cx, cy, best, ok = x, y, here, t.sight.CanHit(x, y, d, tx, ty)
	score := here
	if !ok {
		score = -1 << 30
	}
	for py := y - r; py <= y+r; py++ {
		for px := x - r; px <= x+r; px++ {
			if px == x && py == y || !t.h.InBounds(px, py) {
				continue
			}
			c := t.sight.Cover(tx, ty, px, py) // cheap first: most tiles have none
			if s := c - coverWalk*weapon.Distance(x, y, px, py); s > score && t.free(id, px, py) && t.sight.CanHit(px, py, d, tx, ty) {
				cx, cy, best, score, ok = px, py, c, s, true
			}
		}
	}
	return cx, cy, best, here, ok
}

// free: a creature may stand on (x, y): dry, walkable, nobody's and
// nobody on it but id.
func (t *Tactics) free(id entity.EntityID, x, y int) bool {
	if !t.h.InBounds(x, y) || t.f != nil && (!t.f.DryWalkable(x, y) || t.f.TileTaken(x, y)) {
		return false
	}
	t.seen = t.h.Near(x, y, 0, Everyone, t.seen[:0])
	for _, s := range t.seen {
		if s.ID != id {
			return false
		}
	}
	return true
}

// TakeCover steps colonist id at (x, y), fighting the target at (tx, ty)
// with d, onto an adjacent tile with CoverMargin more cover, at most once
// in CoverHold ticks; only against a target that shoots back, and never
// while holding position. It reports whether it set off.
func (t *Tactics) TakeCover(id, target entity.EntityID, x, y int, d weapon.Def, tx, ty int) bool {
	now := t.h.Now()
	s := t.drafts.set[id]
	if !d.Ranged || now < s.CoverAt || s.Hold || !weapon.DefOf(t.h.Weapon(target)).Ranged {
		return false
	}
	s.CoverAt = now + CoverHold
	t.drafts.put(id, s)
	cx, cy, best, here, ok := t.BestCover(id, x, y, CoverStep, d, tx, ty)
	if !ok || cx == x && cy == y || best < here+CoverMargin {
		return false
	}
	s.CoverX, s.CoverY = cx, cy
	t.drafts.put(id, s)
	return t.h.StartJob(id, job.Record{Kind: job.Move, TX: cx, TY: cy})
}

// Covering reports colonist id walking j into the cover TakeCover chose.
func (t *Tactics) Covering(id entity.EntityID, j job.Record) bool {
	s := t.drafts.set[id]
	return j.Kind == job.Move && j.Active() && !j.Forced && t.h.Now() < s.CoverAt && j.TX == s.CoverX && j.TY == s.CoverY
}

// Danger is the landing tile and reach of an explosive in flight (a
// grenade or a shell) whose blast would catch (x, y) (ok false: none).
// wiki: Raider, raiders run from grenades.
func (t *Tactics) Danger(x, y int) (lx, ly, reach int, ok bool) {
	if now := t.h.Now(); !t.dangerOK || t.dangerAt != now {
		t.danger, t.dangerAt, t.dangerOK = t.danger[:0], now, true
		for _, sh := range t.shots.live {
			if weapon.DefOf(sh.Weapon).Explosive() {
				t.danger = append(t.danger, sh)
			}
		}
	}
	for _, sh := range t.danger {
		b := weapon.DefOf(sh.Weapon).Blast
		if b.Within(x-sh.ToX, y-sh.ToY) {
			return sh.ToX, sh.ToY, b.Reach(), true
		}
	}
	return 0, 0, 0, false
}

// Colonists is the standing colonists within r of (x, y) (the creature
// index), in no particular order; the slice is reused by the next call.
func (t *Tactics) Colonists(x, y, r int) []Seen {
	t.out = t.h.Near(x, y, r, Pawns, t.out[:0])
	t.out = slices.DeleteFunc(t.out, func(s Seen) bool { return !t.h.StandingPawn(s.ID) })
	return t.out
}

// Block is the first wall or shut door (one not held open) on creature
// id's way to (tx, ty) planned as if walls and doors were gone, and the
// tile it stands on to hit it (ok false: no such way, or nothing in it).
// A mechanoid bashes its way in by it (game/raid/mech.go).
func (t *Tactics) Block(id entity.EntityID, tx, ty int) (b entity.EntityID, sx, sy int, ok bool) {
	path, ok := t.br.BreachRoute(id, tx, ty)
	if !ok {
		return 0, 0, 0, false
	}
	sx, sy, _ = t.h.Pos(id)
	q := t.br.Query(id)
	for _, p := range path {
		if bid, k := t.h.Object(p.X, p.Y); bid != 0 && (k.IsDoor() && !t.doors.Held(bid) || !q.Walkable(p.X, p.Y)) {
			return bid, sx, sy, true
		}
		sx, sy = p.X, p.Y
	}
	return 0, 0, 0, false
}

// Bash is one blow of creature id with d on building b beside it (a gun
// is swung, as a raider's rifle butt); false when b is gone or out of
// touch.
func (t *Tactics) Bash(id, b entity.EntityID, d weapon.Def) bool {
	h := t.h
	bd, ok := h.Building(b)
	px, py, _ := h.Pos(id)
	if !ok || !touches(px, py, bd) {
		return false
	}
	dmg := d.Damage
	if d.Ranged {
		dmg = buttDamage
	}
	now := h.Now()
	h.Trace(projectile.Trace{FromX: px, FromY: py, ToX: bd.X, ToY: bd.Y, Tick: now, Hit: true, Melee: true,
		Damage: dmg, Building: true, Attacker: id, Weapon: d.Name})
	h.Struck(id, 0, now)
	h.DamageBuilding(b, dmg)
	return true
}
