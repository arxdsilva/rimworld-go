package game

import (
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/terrain"
)

// raidHost implements raid.Host (raid/deps.go) over the game: one small
// adapter type per role, each a thin view of *Game, so no method is added
// to Game itself (.cursor/SOLID_GO.md §10). Jobs and Dice are combat's
// adapters (combat_host.go): the same calls on the same main stream.
type raidHost struct {
	raidRoster
	raidCaptives
	raidBodies
	combatJobs
	raidFight
	raidGround
	raidEvents
	combatDice
}

var _ raid.Host = raidHost{}

func newRaid(g *Game) *raid.Raid {
	h := raidHost{raidRoster{g}, raidCaptives{g}, raidBodies{g}, combatJobs{g}, raidFight{g}, raidGround{g}, raidEvents{g}, combatDice{g}}
	return raid.New(h, raid.Roles{Wild: raidWild{g}, Story: raidStory{g}, Mech: raidMech{g},
		Arrive: raidArrive{g}, Mischief: raidMischief{g}, Siege: raidSiege{g}, Chronicle: raidChronicle{g}}) // raid_def4.go
}

type raidRoster struct{ g *Game }

func (h raidRoster) Raider(id entity.EntityID) raid.Raider       { return h.g.raiders[id] }
func (h raidRoster) SetRaider(id entity.EntityID, r raid.Raider) { h.g.raiders[id] = r }
func (h raidRoster) RaiderIDs() []entity.EntityID                { return h.g.raiderIDs }
func (h raidRoster) Groups() []raid.Group                        { return h.g.raidGroups }
func (h raidRoster) StandingRaider(id entity.EntityID) bool      { return h.g.standingRaider(id) }
func (h raidRoster) RemoveRaider(id entity.EntityID)             { h.g.removeRaider(id) }
func (h raidRoster) DropGear(id entity.EntityID)                 { h.g.dropRaiderGear(id) }

type raidCaptives struct{ g *Game }

func (h raidCaptives) Prisoner(id entity.EntityID) bool            { return h.g.prison.Is(id) }
func (h raidCaptives) LivePrisoner(id entity.EntityID, now uint64) { h.g.prison.Live(id, now) }
func (h raidCaptives) DropPatient(id entity.EntityID)              { h.g.care.DropPatient(id) }
func (h raidCaptives) BeingCaptured(id entity.EntityID) bool       { return h.g.care.BeingCaptured(id) }
func (h raidCaptives) StepKidnap(id entity.EntityID, j job.Record, now uint64) {
	h.g.stepKidnap(id, j, now)
}
func (h raidCaptives) TryKidnap(id entity.EntityID, x, y int, now uint64) bool {
	return h.g.tryKidnap(id, Position{X: x, Y: y}, now) // kidnap.go
}

type raidBodies struct{ g *Game }

func (h raidBodies) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h raidBodies) Vital(id entity.EntityID) health.Record { v, _ := h.g.vital.Get(id); return v }
func (h raidBodies) Heal(id entity.EntityID, v health.Record, now uint64) {
	h.g.heal(id, &v, now) // combat.go
}
func (h raidBodies) Name(id entity.EntityID) string { return h.g.name(id) }
func (h raidBodies) Home() (int, int)               { return h.g.home() }
func (h raidBodies) NearestColonist(x, y int) (entity.EntityID, int, int, bool) {
	id, p, ok := h.g.nearestColonist(Position{X: x, Y: y})
	return id, p.X, p.Y, ok
}
func (h raidBodies) RaidTarget(x, y int) (entity.EntityID, int, int, bool) {
	id, p, ok := h.g.raidTarget(Position{X: x, Y: y}) // allies.go
	return id, p.X, p.Y, ok
}

type raidFight struct{ g *Game }

func (h raidFight) WalkableNear(x, y, r int) (int, int, bool) {
	return h.g.combat.Tiles.WalkableNear(x, y, r)
}
func (h raidFight) Strike(id entity.EntityID, j job.Record, now uint64) {
	h.g.combat.Strikes.Do(id, j, now)
}
func (h raidFight) Weapon(id entity.EntityID) weapon.Def { return weaponDefOf(h.g, id) }
func (h raidFight) CanHit(x, y int, d weapon.Def, tx, ty int) bool {
	return h.g.combat.Sight.CanHit(x, y, d, tx, ty)
}
func (h raidFight) AimTicks(d weapon.Def) int               { return combat.AimTicks(d) }
func (h raidFight) Breach(id entity.EntityID, j job.Record) { h.g.combat.Breach.Do(id, j) }
func (h raidFight) StartBreach(id entity.EntityID, tx, ty int) bool {
	return h.g.combat.Breach.Start(id, tx, ty)
}
func (h raidFight) TurretFor(x, y, cd, sight int) (entity.EntityID, int, int, bool) {
	return h.g.combat.Turrets.ForRaider(x, y, cd, sight)
}

type raidGround struct{ g *Game }

func (h raidGround) Size() (int, int)          { return h.g.grid.Width(), h.g.grid.Height() }
func (h raidGround) InBounds(x, y int) bool    { return h.g.grid.InBounds(x, y) }
func (h raidGround) Rock(x, y int) bool        { return terrain.IsRockWall(h.g.terr.At(x, y)) }
func (h raidGround) DryWalkable(x, y int) bool { return h.g.dryWalkable(x, y) } // water.go
func (h raidGround) Shape() (*grid.Grid, uint64, uint64) {
	return h.g.grid, h.g.grid.Version(), h.g.structVer
}
func (h raidGround) Mine(x, y int) {
	h.g.setTerrain(x, y, terrain.Stone) // clears any ore (SIM-029)
	h.g.roofOnMined(x, y)
}
func (h raidGround) NearestEdge(x, y int) (int, int) {
	return nearestEdge(Position{X: x, Y: y}, h.g.grid.Width(), h.g.grid.Height())
}
func (h raidGround) Build(kind, stuff, facing string, x, y int) {
	h.g.BuildNow(kind, stuff, facing, x, y)
}

type raidEvents struct{ g *Game }

func (h raidEvents) Message(x, y int, text string) { h.g.message(x, y, "%s", text) }
func (h raidEvents) Threat(title string, x, y int, text string) {
	h.g.threat(title, x, y, "%s", text) // climate.go
}

type raidWild struct{ g *Game }

func (h raidWild) PickSpecies(keep func(animal.Species) bool) (animal.Species, bool) {
	return h.g.pickSpecies(keep) // wildlife.go
}
func (h raidWild) SpawnAnimal(s animal.Species, x, y int) (entity.EntityID, error) {
	return h.g.spawnAnimal(s, x, y)
}
func (h raidWild) Animal(id entity.EntityID) animal.State       { a, _ := h.g.animals.Get(id); return a }
func (h raidWild) SetAnimal(id entity.EntityID, a animal.State) { h.g.animals.Set(id, a) }
func (h raidWild) Plural(s animal.Species) string               { return pluralLabel(s) }
func (h raidWild) EdgeTile() (int, int, bool)                   { return h.g.edgeTile() }

type raidStory struct{ g *Game }

func (h raidStory) ManhunterPack(n int)       { h.g.manhunterPack(n) } // handling.go
func (h raidStory) MadAnimal(now uint64) bool { return h.g.madAnimal(now, false) }
func (h raidStory) ScaledRaid(day int)        { h.g.scaledRaid(day) }
func (h raidStory) Enclosed() bool            { return h.g.enclosed() }
func (h raidStory) RaidPoints(day int) int    { return h.g.raidPoints(day) }
func (h raidStory) ShipPart() bool            { return h.g.FireIncident("crashed_ship_part") }

// raidMech is the mechanoids' and ship parts' role (raid/mech.go, DEF-4).
type raidMech struct{ g *Game }

func (h raidMech) Shoot(id, target entity.EntityID, wk weapon.Kind) {
	h.g.combat.Strikes.Strike(id, target, wk)
}
func (h raidMech) GoTo(id entity.EntityID, a animal.State, x, y int) animal.State {
	h.g.animalGoTo(id, &a, x, y)
	return a
}
func (h raidMech) Line(x, y, tx, ty int) bool { return h.g.combat.Sight.Line(x, y, tx, ty) }
func (h raidMech) MechsNear(x, y, r int, buf []entity.EntityID) []entity.EntityID {
	h.g.within(x, y, r, ckAnimal, func(id entity.EntityID, _ Position) { // animal_index.go
		if a, ok := h.g.animals.Get(id); ok && animal.IsMech(a.Species) {
			buf = append(buf, id)
		}
	})
	return buf
}
func (h raidMech) ColonistNear(x, y, r int) bool {
	_, _, ok := h.g.nearestWhere(x, y, r, ckPawn, func(o entity.EntityID, _ Position) bool { return h.g.standingPawn(o) })
	return ok
}
func (h raidMech) PlaceShipPart(x, y int) (entity.EntityID, bool) {
	g := h.g
	if ok, _ := g.canPlaceAt(construction.ShipPart, construction.South, x, y); !ok {
		return 0, false
	}
	g.placeFrameStuff(construction.ShipPart, item.Wood, construction.South, x, y) // instant: no cost, no work
	cells, _, _ := construction.Footprint(construction.ShipPart, construction.South, x, y)
	if len(cells) == 0 {
		return 0, false
	}
	id := g.layerBldgAt(cells[0][0], cells[0][1], construction.LayerObject)
	b, ok := g.bldgs.Get(id)
	if !ok || b.Kind != construction.ShipPart {
		return 0, false
	}
	b.Ancient = true // not the colony's: no wealth, no Home anchor, no repair, no blueprint once broken
	g.bldgs.Set(id, b)
	return id, true
}
func (h raidMech) KillPlant(x, y int) {
	if id := h.g.plantAt(x, y); id != 0 {
		h.g.removePlant(id)
	}
}
func (h raidMech) Drop(kind item.Kind, qty, x, y int) { _, _ = h.g.SpawnStack(kind, qty, x, y) }
