package game

import (
	"slices"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// combatHost implements combat.Host (combat/deps.go) over the game: one
// small adapter type per role, each a thin view of *Game, so no method is
// added to Game itself (.cursor/SOLID_GO.md §10). Every method passes
// values only: they run on the tick path (§7).
type combatHost struct {
	combatCreatures
	combatRosters
	combatGround
	combatSkills
	combatJobs
	combatHarm
	combatEvents
	combatDice
}

var _ combat.Host = combatHost{}

func newCombat(g *Game) *combat.Combat {
	h := combatHost{combatCreatures{g}, combatRosters{g}, combatGround{g}, combatSkills{g}, combatJobs{g}, combatHarm{g}, combatEvents{g}, combatDice{g}}
	return combat.New(h, combat.Roles{Power: combatPower{g}, Flight: combatFlight{g}, Hunts: combatHunts{g}, Breach: combatBreach{g},
		Field: combatField{g}, Supply: combatSupply{g}, Arms: combatArms{g}})
}

type combatCreatures struct{ g *Game }

func (h combatCreatures) Now() uint64 { return uint64(h.g.clock.Now()) }
func (h combatCreatures) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h combatCreatures) Animal(id entity.EntityID) (animal.State, bool) { return h.g.animals.Get(id) }
func (h combatCreatures) Weapon(id entity.EntityID) weapon.Kind          { return h.g.weaponOf(id) }
func (h combatCreatures) StandingPawn(id entity.EntityID) bool           { return h.g.standingPawn(id) }
func (h combatCreatures) StandingRaider(id entity.EntityID) bool         { return h.g.standingRaider(id) }
func (h combatCreatures) Hostile(a animal.State) bool                    { return h.g.hostile(a) }
func (h combatCreatures) Armor(id entity.EntityID, buf []armor.Layer) []armor.Layer {
	if a, ok := h.g.animals.Get(id); ok {
		if m, mech := animal.MechOf(a.Species); mech {
			return append(buf, m.Layer()) // DEF-4: a mechanoid's plating
		}
	}
	w := h.g.apparel[id] // apparel.go: the outer body layer first; each piece by its stuff and quality
	for _, p := range [...]craft.Piece{w.On[item.Body], w.On[item.Legs], w.On[item.Head]} {
		buf = armor.AppendPiece(buf, p.Kind, p.Stuff, p.Quality)
	}
	return buf
}

type combatRosters struct{ g *Game }

func (h combatRosters) AnimalIDs() []entity.EntityID    { return h.g.animalIDs }
func (h combatRosters) RaiderIDs() []entity.EntityID    { return h.g.raiderIDs }
func (h combatRosters) Drafted(id entity.EntityID) bool { return h.g.drafted(id) }
func (h combatRosters) Name(id entity.EntityID) string  { return h.g.name(id) }

// RaiderOn answers the threat scan's raider loop in one call (combat
// defend.go ThreatTo): a raider on its feet with an attack job on id.
func (h combatRosters) RaiderOn(t, id entity.EntityID) (int, int, bool) {
	g := h.g
	rj, _ := g.jobs.Get(t)
	if !g.isRaider(t) || !g.standingRaider(t) || rj.Kind != job.Attack || rj.Target != id {
		return 0, 0, false
	}
	p, _ := g.positions.Get(t)
	return p.X, p.Y, true
}

// Near reads the creature index (animal_index.go), as nearestWhere and
// within do; combat.Who has the index's kind bits.
func (h combatRosters) Near(x, y, r int, who combat.Who, buf []combat.Seen) []combat.Seen {
	g := h.g
	g.creatures().grid.each(x, y, r+1, func(e idxEntry) bool {
		if p, ok := g.positions.Get(e.id); ok && e.kind&uint8(who) != 0 && weapon.Distance(x, y, p.X, p.Y) <= r {
			buf = append(buf, combat.Seen{ID: e.id, X: p.X, Y: p.Y})
		}
		return true
	})
	return buf
}

type combatGround struct{ g *Game }

func (h combatGround) InBounds(x, y int) bool { return h.g.grid.InBounds(x, y) }
func (h combatGround) Rock(x, y int) bool     { return terrain.IsRockWall(h.g.terr.At(x, y)) }
func (h combatGround) Object(x, y int) (entity.EntityID, construction.Kind) {
	bid := h.g.layerBldgAt(x, y, construction.LayerObject)
	if bid == 0 {
		return 0, 0
	}
	b, _ := h.g.bldgs.Get(bid)
	return bid, b.Kind
}
func (h combatGround) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h combatGround) AnyBuilt(match func(construction.Kind) bool) bool {
	for _, id := range h.g.bldgIDs {
		if b := h.g.bldgs.Ref(id); b != nil && match(b.Kind) {
			return true
		}
	}
	return false
}
func (h combatGround) RemoveBuilding(id entity.EntityID) { h.g.removeBuilding(id) }
func (h combatGround) PlaceFrame(k construction.Kind, stuff item.Kind, f construction.Facing, x, y int) {
	h.g.placeFrameStuff(k, stuff, f, x, y)
}
func (h combatGround) DigRock(x, y, dmg int) { h.g.raid.Rock.Dig(x, y, dmg) } // raid/rock.go

type combatSkills struct{ g *Game }

func (h combatSkills) HitChance(id entity.EntityID, ranged bool, base int) int {
	return h.g.hitChanceFor(id, ranged, base)
}
func (h combatSkills) ShotAim(id entity.EntityID) (int, int) {
	if a, ok := h.g.animals.Get(id); ok && animal.IsMech(a.Species) {
		return attack.ShooterPerTile(animal.MechShootingSkill), 0 // DEF-4: wiki: Lancer (96 %)
	}
	return attack.ShooterPerTile(int(h.g.skillOf(id, skill.Shooting).Level)), 10 * int(h.g.statOf(id, stat.ShootAcc))
}
func (h combatSkills) Learn(id entity.EntityID, ranged bool) { h.g.combatLearn(id, ranged) }
func (h combatSkills) AimTicks(id entity.EntityID, cooldown int) int {
	return h.g.aimTicksFor(id, cooldown)
}
func (h combatSkills) CanViolence(id entity.EntityID) bool { return h.g.canViolence(id) }
func (h combatSkills) FightsBack(id entity.EntityID) bool  { return h.g.fightsBack(id) }

type combatJobs struct{ g *Game }

func (h combatJobs) Job(id entity.EntityID) (job.Record, bool)      { return h.g.jobs.Get(id) }
func (h combatJobs) SetJob(id entity.EntityID, j job.Record)        { h.g.jobs.Set(id, j) }
func (h combatJobs) StartJob(id entity.EntityID, j job.Record) bool { return h.g.startJob(id, j) }
func (h combatJobs) Interrupt(id entity.EntityID, r job.FailReason) { h.g.interrupt(id, r) }
func (h combatJobs) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, x, y)
}
func (h combatJobs) ReleaseOwner(id entity.EntityID) { h.g.reserve.ReleaseOwner(id) } // raid.Jobs

type combatHarm struct{ g *Game }

func (h combatHarm) Hurt(target entity.EntityID, dmg int, by entity.EntityID) {
	up := h.g.standingPawn(target) && !h.g.isRaider(target)
	h.g.hurt(target, dmg, by)
	h.g.combat.Log.Down(target, up) // combat/log.go (DEF-4)
}
func (h combatHarm) HurtAs(target entity.EntityID, dmg int, by entity.EntityID, kind body.WoundKind, cause health.Cause) {
	up := h.g.standingPawn(target) && !h.g.isRaider(target)
	h.g.hurtAs(target, dmg, by, kind, cause)
	h.g.combat.Log.Down(target, up)
}
func (h combatHarm) DamageBuilding(id entity.EntityID, dmg int) { h.g.damageBuilding(id, dmg) }
func (h combatHarm) Stain(target entity.EntityID, ax, ay, tx, ty, dmg int) {
	h.g.stainHit(target, Position{X: ax, Y: ay}, Position{X: tx, Y: ty}, dmg) // stains.go
}
func (h combatHarm) Trace(t projectile.Trace) { h.g.traces = append(h.g.traces, t) }
func (h combatHarm) Struck(attacker, target entity.EntityID, tick uint64) {
	h.g.lastStrike[attacker] = strikeRec{Target: target, Tick: tick}
}
func (h combatHarm) Hit(target entity.EntityID, tick uint64, fromX, fromY, dmg int) {
	h.g.lastHit[target] = hitRec{Tick: tick, FromX: fromX, FromY: fromY, Damage: dmg}
}

type combatEvents struct{ g *Game }

func (h combatEvents) LogStrike(attacker, target entity.EntityID, wk weapon.Kind, hit bool, dmg int) {
	h.g.logStrike(attacker, target, wk, hit, dmg) // eventlog.go
}
func (h combatEvents) Fled(id, threat entity.EntityID) {
	h.g.logEvent(id, logEntry{Kind: logFled, Other: threat, Name: h.g.creatureName(threat)})
}
func (h combatEvents) Shot(target entity.EntityID, hit bool, dmg int, by string) {
	if !hit {
		h.g.logEvent(target, logEntry{Kind: logWasMissed, Name: by})
		return
	}
	h.g.logEvent(target, logEntry{Kind: logAttacked, Amount: dmg, Name: by})
}
func (h combatEvents) Trapped(victim entity.EntityID, dmg int) {
	h.g.logEvent(victim, logEntry{Kind: logTrap, Amount: dmg})
}
func (h combatEvents) Message(x, y int, text string) { h.g.message(x, y, "%s", text) }
func (h combatEvents) Burst(x, y int, tick uint64, b explosion.Def) { // a turret's: b zero
	h.g.wl.Bursts = append(h.g.wl.Bursts, burstStamp{X: x, Y: y, Tick: tick, Turret: b.Zero(), Blast: b.Kind, Radius: b.Radius / 100})
	h.g.raid.Mechs.EMP(x, y, b, tick) // raid/mech.go (DEF-4): an EMP stuns the mechanoids it reaches
}
func (h combatEvents) Ignite(x, y int) { h.g.igniteBurst(x, y) } // fire.go

// combatDice rolls on the game's streams: strikes and traps on g.rng as
// they always have, turrets on streamDefense (ENG-003).
type combatDice struct{ g *Game }

func (h combatDice) Roll(n int) int        { return h.g.rng.IntN(n) }
func (h combatDice) DefenseRoll(n int) int { return h.g.stream(streamDefense).IntN(n) }

type combatPower struct{ g *Game }

func (h combatPower) Wired() bool                     { return h.g.pw != nil }
func (h combatPower) Powered(id entity.EntityID) bool { return h.g.powered(id) }
func (h combatPower) PowerGen() uint64                { return h.g.powerList().gen }
func (h combatPower) AppendDefense(buf []entity.EntityID) []entity.EntityID {
	c := h.g.powerList()
	for i, s := range c.specs {
		if s.Class == power.ClassDefense && s.Role == power.Consumer {
			buf = append(buf, c.ids[i])
		}
	}
	return buf
}

type combatFlight struct{ g *Game }

func (h combatFlight) FleeGoal(id entity.EntityID, x, y, tx, ty int) (int, int, bool) {
	return h.g.fleeGoal(id, Position{X: x, Y: y}, Position{X: tx, Y: ty}) // flee.go
}
func (h combatFlight) DropForFlight(id entity.EntityID, j job.Record) { h.g.dropForFlight(id, j) }
func (h combatFlight) Calm(id entity.EntityID) (uint64, bool) {
	until, ok := h.g.calm[id]
	return until, ok
}
func (h combatFlight) SetCalm(id entity.EntityID, until uint64) { h.g.calm[id] = until }
func (h combatFlight) ForgetCalm(id entity.EntityID)            { delete(h.g.calm, id) }
func (h combatFlight) CheckThreats(id entity.EntityID, x, y, animalR, raiderR int, got entity.EntityID, gotD int) {
	if selfCheck() {
		h.g.checkThreatScan(id, Position{X: x, Y: y}, animalR, raiderR, got, gotD) // paths.go
	}
}

type combatHunts struct{ g *Game }

func (h combatHunts) ClearHuntOut(id entity.EntityID) { h.g.clearHuntOut(id) }
func (h combatHunts) HuntGivenUp(id entity.EntityID, j job.Record, tx, ty int, now uint64) bool {
	return h.g.huntGivenUp(id, j, Position{X: tx, Y: ty}, now) // area_dest.go
}
func (h combatHunts) AbandonHunt(id entity.EntityID, j job.Record) { h.g.abandonHunt(id, j) }
func (h combatHunts) AnyMarked() bool { // jobindex.go
	return h.g.anyAnimal(func(_ entity.EntityID, a *animal.State) bool { return a.Designated && !a.Dead() })
}
func (h combatHunts) Reserved(id entity.EntityID) bool {
	_, taken := h.g.reserve.EntityOwner(id)
	return taken
}
func (h combatHunts) Postponed(id, prey entity.EntityID) bool {
	return h.g.postponedFor(id, job.Hunt, prey, 0, 0) // postpone.go (SIM-027)
}
func (h combatHunts) NearestFree(id entity.EntityID, k item.Kind) (entity.EntityID, int, int) {
	return h.g.nearestFree(id, k)
}

// combatField is natural cover and the building lists (DEF-3).
type combatField struct{ g *Game }

func (h combatField) Plant(x, y int) (plant.Species, bool, bool) {
	if p := h.g.plants.Ref(h.g.plantAt(x, y)); p != nil { // grown: at least half way
		return p.Species, 2*p.Growth >= plant.DefOf(p.Species).GrowTicks, true
	}
	return 0, false, false
}
func (h combatField) Size() (int, int)          { return h.g.grid.Width(), h.g.grid.Height() }
func (h combatField) DryWalkable(x, y int) bool { return h.g.dryWalkable(x, y) } // water.go
func (h combatField) TileTaken(x, y int) bool   { _, taken := h.g.reserve.TileOwner(x, y); return taken }
func (h combatField) AppendChunks(buf [][2]int) [][2]int {
	for _, id := range h.g.itemIDs {
		if st := h.g.stacks.Ref(id); st != nil && st.Carrier == 0 && item.IsChunk(st.Kind) {
			buf = append(buf, [2]int{st.X, st.Y})
		}
	}
	return buf
}
func (h combatField) AppendBuilt(match func(construction.Kind) bool, buf []entity.EntityID) []entity.EntityID {
	for _, id := range h.g.bldgIDs {
		if b := h.g.bldgs.Ref(id); b != nil && match(b.Kind) {
			buf = append(buf, id)
		}
	}
	slices.Sort(buf)
	return buf
}

// combatSupply is the stock a rearm haul fetches (DEF-3).
type combatSupply struct{ g *Game }

func (h combatSupply) Stock(id entity.EntityID, k item.Kind, qty int) (entity.EntityID, int, int, bool) {
	sid, st := h.g.ingredientFor(id, recipe.Input{Kind: k, Qty: qty}) // craft_host.go
	return sid, st.X, st.Y, sid != 0
}
func (h combatSupply) PickUp(id, stack entity.EntityID, qty int) (entity.EntityID, item.Kind, int, bool) {
	g := h.g
	st, ok := g.stacks.Get(stack)
	if !ok || st.Carrier != 0 {
		return 0, 0, 0, false
	}
	carry, take := stack, min(st.Qty, max(1, qty))
	if st.Qty > take { // split off what is carried, as refuelling does (fuel.go)
		st.Qty -= take
		g.stacks.Set(stack, st)
		carry = g.entities.Next()
		g.itemIDs = append(g.itemIDs, carry)
		g.stacks.Set(carry, item.Stack{Kind: st.Kind, Qty: take, X: st.X, Y: st.Y, Carrier: id})
	} else {
		st.Carrier = id
		g.stacks.Set(stack, st)
	}
	return carry, st.Kind, take, true
}
func (h combatSupply) Spend(carry entity.EntityID) { h.g.deleteItem(carry) }

type combatBreach struct{ g *Game }

// BreachRoute plans over breachMap (defense.go): walls and doors down, and
// rock too for a sapper.
func (h combatBreach) BreachRoute(id entity.EntityID, tx, ty int) ([]pathing.Point, bool) {
	g := h.g
	pos, _ := g.positions.Get(id)
	breaks := g.buildingCells(construction.Wall, construction.Door, construction.Autodoor)
	if g.sapper(id) {
		breaks = g.sapperBreaks() // sappers tunnel through rock too
	}
	return g.searcher().Find(breachMap{g: g, breaks: breaks}, pos.X, pos.Y, tx, ty)
}
func (h combatBreach) Query(id entity.EntityID) pathing.Query { return h.g.pathQuery(id) }
