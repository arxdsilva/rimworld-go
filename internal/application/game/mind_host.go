package game

import (
	"fmt"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
)

// breakState is a colonist's mental break: its kind and end tick. It stays
// on the game (g.breaks), which social fights and dementia write too; the
// mind package runs the mood breaks through mindHost.
type breakState struct {
	kind  mentalbreak.Kind
	until uint64
}

// bingeEat marks an Eat job that ignores fullness (j.Count).
const bingeEat = 3

// Breaking names a colonist's mental break ("" when none).
func (g *Game) Breaking(id entity.EntityID) string { return g.breaks[id].kind.String() }

// ForceBreak starts a mental break now (dev scenarios and fixtures).
func (g *Game) ForceBreak(id entity.EntityID, k mentalbreak.Kind) { g.viewGen++; g.mind.Force(id, k) }

// mindHost implements mind.Host (mind/deps.go) over the game: one small
// adapter per role, so no method is added to Game (.cursor/SOLID_GO.md §10).
type mindHost struct {
	mindPawns
	mindBreaks
	mindActs
	mindAggro
	mindBrawls
	mindEvents
	mindMuses
	mindSmash
}

var _ mind.Host = mindHost{}

func newMindHost(g *Game) mindHost {
	return mindHost{mindPawns{g}, mindBreaks{g}, mindActs{g}, mindAggro{g}, mindBrawls{g}, mindEvents{g}, mindMuses{g}, mindSmash{g}}
}

type mindPawns struct{ g *Game }

func (h mindPawns) Now() uint64 { return uint64(h.g.clock.Now()) }
func (h mindPawns) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h mindPawns) Alive(id entity.EntityID) bool     { return h.g.onMap(id) }
func (h mindPawns) Standing(id entity.EntityID) bool  { return h.g.alive(id) && h.g.onMap(id) }
func (h mindPawns) Colonists() []entity.EntityID      { return h.g.pawnIDs }
func (h mindPawns) Mood(id entity.EntityID) int       { return h.g.moodOf(id) }
func (h mindPawns) BreakShift(id entity.EntityID) int { return h.g.breakOffset(id) }

// Prisoners is a fresh list (a murderous rage picks its target once).
func (h mindPawns) Prisoners() []entity.EntityID {
	var out []entity.EntityID
	for _, id := range h.g.raiderIDs {
		if h.g.prison.Is(id) {
			out = append(out, id)
		}
	}
	return out
}

type mindBreaks struct{ g *Game }

func (h mindBreaks) Break(id entity.EntityID) (mentalbreak.Kind, uint64, bool) {
	b, ok := h.g.breaks[id]
	return b.kind, b.until, ok
}
func (h mindBreaks) SetBreak(id entity.EntityID, k mentalbreak.Kind, until uint64) {
	h.g.breaks[id] = breakState{kind: k, until: until}
}
func (h mindBreaks) ClearBreak(id entity.EntityID) { delete(h.g.breaks, id) }

// Roll draws from the social stream (ENG-003), never g.rng.
func (h mindBreaks) Roll(n int) int { return h.g.stream(streamSocial).IntN(n) }

type mindActs struct{ g *Game }

func (h mindActs) Seize(id entity.EntityID) {
	h.g.interrupt(id, job.Cancelled)
	delete(h.g.draft, id)
	delete(h.g.orderQueue, id)
}
func (h mindActs) Wander(id entity.EntityID, linger int) bool { return h.g.idleWander(id, linger) }
func (h mindActs) CanBinge(id entity.EntityID) bool {
	best, _, _, _ := h.g.bestFood(id, id)
	return best != 0
}
func (h mindActs) Binge(id entity.EntityID) bool {
	best, bx, by, _ := h.g.bestFood(id, id)
	return best != 0 && h.g.startJob(id, job.Record{Kind: job.Eat, Target: best, TX: bx, TY: by, WorkLeft: needs.EatWorkTicks, Count: bingeEat})
}
func (h mindActs) CanHide(id entity.EntityID) bool {
	_, ok := hideSpot(h.g, id)
	return ok
}

// Hide shuts id away for ticks, or wanders when it has nowhere to go.
func (h mindActs) Hide(id entity.EntityID, ticks int) {
	g := h.g
	p, ok := hideSpot(g, id)
	if !ok || !g.reachable(id, p.X, p.Y) {
		g.idleWander(id, 300)
		return
	}
	g.startJob(id, job.Record{Kind: job.Hide, TX: p.X, TY: p.Y, WorkLeft: ticks})
}
func (h mindActs) Down(id entity.EntityID) { h.g.down(id, health.CauseCatatonic) }
func (h mindActs) StandUp(id entity.EntityID) {
	if v, ok := h.g.vital.Get(id); ok && v.Status == health.Downed && v.Cause == health.CauseCatatonic {
		v.Status, v.Cause = health.Alive, health.CauseNone
		h.g.vital.Set(id, v)
	}
}

// hideSpot is where a colonist would shut itself away: its own bed if it
// has one, else the nearest cell of any room.
func hideSpot(g *Game, id entity.EntityID) (Position, bool) {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind.Sleeps() && b.SlotOf(uint64(id)) >= 0 {
			x, y := g.sleepCellFor(id, b)
			return Position{x, y}, true
		}
	}
	g.ensureRooms()
	pos, _ := g.positions.Get(id)
	w := g.grid.Width()
	var best Position
	bestD := 1 << 30
	for i, r := range g.roomGrid {
		if r < 0 || r == room.Wall {
			continue
		}
		x, y := i%w, i/w
		if !g.grid.Walkable(x, y) {
			continue
		}
		if d := pathing.Dist(pos.X, pos.Y, x, y); d < bestD {
			best, bestD = Position{x, y}, d
		}
	}
	return best, bestD < 1<<30
}

type mindAggro struct{ g *Game }

func (h mindAggro) CanViolence(id entity.EntityID) bool { return h.g.canViolence(id) }
func (h mindAggro) Attackable(id entity.EntityID) bool {
	_, _, ok := h.g.combat.Strikes.Target(id)
	return ok && h.g.onMap(id)
}
func (h mindAggro) NearestCreature(id entity.EntityID, r int) entity.EntityID {
	g := h.g
	pos, _ := g.positions.Get(id)
	best, _, _ := g.nearestWhere(pos.X, pos.Y, r, ckAll, func(o entity.EntityID, _ Position) bool {
		return o != id && h.Attackable(o)
	})
	return best
}

// Approach sets id walking to the tile beside target (the two-pawn
// pattern of mirrorChat and brawls: no reservation).
func (h mindAggro) Approach(id, target entity.EntityID) bool {
	tp, ok := h.g.positions.Get(target)
	return ok && mindSmash(h).GoBeside(id, tp.X, tp.Y)
}

// mindSmash is what a tantrum breaks (mind/tantrum.go, CBT-4): the
// colony's buildings and item stacks that wear.
type mindSmash struct{ g *Game }

// GoBeside walks id to a free tile beside (x, y) (no reservation).
func (h mindSmash) GoBeside(id entity.EntityID, x, y int) bool {
	g := h.g
	sx, sy, ok := g.besideSpot(id, x, y)
	if !ok {
		return false
	}
	path, ok := g.pathTo(id, sx, sy)
	if !ok {
		return false
	}
	j := job.Record{Kind: job.Move, State: job.Goto, TX: sx, TY: sy, Path: path}
	if len(path) == 0 {
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
	return true
}
func (h mindAggro) Insult(a, b entity.EntityID) {
	g := h.g
	g.ensurePerson(a)
	g.ensurePerson(b)
	g.applyInteraction(a, b, social.Insult, uint64(g.clock.Now())) // social.go: may start a fight
}

// Assault: a standing target in an ordinary attack (doCombat), a downed
// one struck where it lies on the weapon's cooldown.
func (h mindAggro) Assault(id, target entity.EntityID) bool {
	g := h.g
	j, _ := g.jobs.Get(id)
	tx, ty, ok := g.combat.Strikes.Target(target)
	if ok {
		if j.Kind == job.Attack && j.Target == target && j.Active() {
			return true
		}
		return g.startJob(id, job.Record{Kind: job.Attack, Target: target, TX: tx, TY: ty, WorkLeft: combat.AimTicks(weaponDefOf(g, id))})
	}
	var tp Position
	if tp, ok = g.positions.Get(target); !ok || !g.onMap(target) {
		return false
	}
	pos, _ := g.positions.Get(id)
	d := weaponDefOf(g, id)
	if !g.combat.Sight.CanHit(pos.X, pos.Y, d, tp.X, tp.Y) {
		if j.Kind == job.Move && j.Active() && weapon.Distance(j.TX, j.TY, tp.X, tp.Y) <= 1 {
			return true
		}
		return h.Approach(id, target)
	}
	if j.Active() {
		g.interrupt(id, job.Cancelled)
	}
	if now := uint64(g.clock.Now()); (now+uint64(id))%uint64(max(d.Cooldown, 1)) == 0 {
		g.combat.Strikes.Strike(id, target, weaponOf(g, id))
	}
	return true
}

type mindBrawls struct{ g *Game }

func (h mindBrawls) Job(id entity.EntityID) job.Record { j, _ := h.g.jobs.Get(id); return j }

// Brawl sets the job directly (the two-pawn pattern: no reservation).
func (h mindBrawls) Brawl(id, foe entity.EntityID) {
	fp, _ := h.g.positions.Get(foe)
	h.g.jobs.Set(id, job.Record{Kind: job.Brawl, State: job.Goto, Target: foe, TX: fp.X, TY: fp.Y,
		WorkLeft: combat.AimTicks(weapon.DefOf(weapon.Fists))})
}
func (h mindBrawls) SetJob(id entity.EntityID, j job.Record) { h.g.jobs.Set(id, j) }
func (h mindBrawls) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, x, y)
}
func (h mindBrawls) Fighter(id entity.EntityID) bool { return h.g.standingPawn(id) }
func (h mindBrawls) Punch(id, foe entity.EntityID) {
	h.g.combat.Strikes.StrikeRoll(id, foe, weapon.Fists, h.g.stream(streamSocial).IntN) // combat/strike.go
}
func (h mindBrawls) PunchTicks(id entity.EntityID) int {
	return h.g.aimTicksFor(id, weapon.DefOf(weapon.Fists).Cooldown)
}
func (h mindBrawls) StopBrawl(id entity.EntityID) {
	if j, _ := h.g.jobs.Get(id); j.Kind == job.Brawl {
		h.g.interrupt(id, job.Cancelled)
	}
}
func (h mindEvents) FightStarted(starter, other entity.EntityID) {
	g, now := h.g, uint64(h.g.clock.Now())
	label := mentalbreak.DefOf(mentalbreak.SocialFight).Label
	g.logEvent(starter, logEntry{Kind: logMental, Name: label, Other: other})
	g.logEvent(other, logEntry{Kind: logMental, Name: label, Other: starter})
	pos, _ := g.positions.Get(starter)
	text := fmt.Sprintf("%s and %s are fighting!", g.name(starter), g.name(other))
	g.post(MessageView{Tick: now, Text: text, X: pos.X, Y: pos.Y, Danger: true, Focus: starter, Letter: true, Title: "Social fight", Kind: letterCaution})
	g.addHistory("fight", fmt.Sprintf("%s got into a fight with %s.", g.name(starter), g.name(other)), starter, other)
}
func (h mindEvents) Outcome(f, other entity.EntityID) {
	g := h.g
	if v, ok := g.vital.Get(f); !ok || v.Status == health.Dead {
		return // the dead remember nothing
	}
	g.ensurePerson(f)
	g.ensurePerson(other)
	g.addOpinionMemory(f, other, social.RollOutcome(g.stream(streamSocial).IntN).Memory(), uint64(g.clock.Now()))
}

type mindEvents struct{ g *Game }

func (h mindEvents) Started(id entity.EntityID, k mentalbreak.Kind, target entity.EntityID) {
	g := h.g
	def := mentalbreak.DefOf(k)
	g.logEvent(id, logEntry{Kind: logMental, Name: def.Label, Other: target})
	pos, _ := g.positions.Get(id)
	text := fmt.Sprintf("Mental break: "+def.Message, g.name(id))
	if target != 0 {
		text += " Target: " + g.personName(target) + "."
	}
	if !mind.Hostile(k) && k != mentalbreak.Catatonic {
		g.message(pos.X, pos.Y, "%s", text)
		return
	}
	kind := letterThreat
	if k == mentalbreak.Catatonic {
		kind = letterCaution
	}
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: text, X: pos.X, Y: pos.Y, Danger: mind.Hostile(k), Focus: id,
		Letter: true, Title: "Mental break: " + def.Label, Kind: kind})
}
func (h mindEvents) Recovered(id entity.EntityID, _ mentalbreak.Kind) {
	g := h.g
	g.logEvent(id, logEntry{Kind: logRecovered})
	g.remember(id, mood.Catharsis)
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "%s has calmed down and feels a little better.", g.name(id))
	g.interrupt(id, job.Cancelled)
}

// mindMuses: inspirations' skills and news (mind/inspiration.go, PRD-2).
type mindMuses struct{ g *Game }

func (h mindMuses) SkillLevel(id entity.EntityID, sk skill.ID) uint8 {
	return h.g.skillOf(id, sk).Level
}
func (h mindMuses) Inspired(id entity.EntityID, label string) {
	h.g.letter(letterGood, "Inspiration", fmt.Sprintf("%s has %s: the next thing %s makes or builds turns out better.",
		h.g.personName(id), label, h.g.personName(id)), id)
}
func (h mindMuses) InspirationEnded(id entity.EntityID, label string) {
	p, _ := h.g.positions.Get(id)
	h.g.message(p.X, p.Y, "%s's %s faded unused.", h.g.personName(id), label)
}
func (h mindSmash) Breakable(x, y int) (entity.EntityID, bool) {
	g := h.g
	if !g.grid.InBounds(x, y) {
		return 0, false
	}
	if id := g.layerBldgAt(x, y, construction.LayerObject); id != 0 {
		b, ok := g.bldgs.Get(id)
		return id, ok && !notColonyPiece(b)
	}
	st, ok := g.stacks.Get(g.itemAt(x, y))
	return g.itemAt(x, y), ok && st.MaxHP() > 0
}
func (h mindSmash) Smash(id, target entity.EntityID) bool {
	g, dmg := h.g, weapon.DefOf(weapon.Fists).Damage
	(combatHarm{g}).Struck(id, 0, uint64(g.clock.Now()))
	if _, ok := g.bldgs.Get(target); ok {
		g.damageBuilding(target, dmg)
		return true
	}
	st, ok := g.stacks.Get(target)
	if st.Wear += dmg; ok && st.WornOut() {
		g.deleteItem(target)
	} else if ok {
		g.putStack(target, st)
	}
	return ok
}
