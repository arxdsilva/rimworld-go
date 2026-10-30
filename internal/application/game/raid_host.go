package game

import (
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
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
	return raid.New(h, raid.Roles{Wild: raidWild{g}, Story: raidStory{g}, Mech: raidMech{g}, Arrive: raidArrive{g},
		Mischief: raidMischief{g}, Siege: raidSiege{g}, Chronicle: raidChronicle{g}, Tactics: raidTactics{g}, Nest: raidNest{g}, Kidnap: raidKidnap{g}})
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
	id := placeAncient(h.g, construction.ShipPart, x, y)
	return id, id != 0
}

// placeAncient sets a building of kind k down whole with its anchor on
// (x, y), instantly and no one's (no wealth, Home anchor, repair or
// blueprint once broken): a ship part, a hive mound (0: no room).
func placeAncient(g *Game, k construction.Kind, x, y int) entity.EntityID {
	if ok, _ := g.canPlaceAt(k, construction.South, x, y); !ok {
		return 0
	}
	g.placeFrameStuff(k, item.Wood, construction.South, x, y) // instant: no cost, no work
	cells, _, _ := construction.Footprint(k, construction.South, x, y)
	if len(cells) == 0 {
		return 0
	}
	id := g.layerBldgAt(cells[0][0], cells[0][1], construction.LayerObject)
	b, ok := g.bldgs.Get(id)
	if !ok || b.Kind != k {
		return 0
	}
	b.Ancient = true
	g.bldgs.Set(id, b)
	return id
}
func (h raidMech) KillPlant(x, y int) {
	if id := h.g.plantAt(x, y); id != 0 {
		h.g.removePlant(id)
	}
}
func (h raidMech) Drop(kind item.Kind, qty, x, y int) { _, _ = h.g.SpawnStack(kind, qty, x, y) }

// raidTactics is the raiders' and mechanoids' ground (raid/tactics.go,
// CBT-4): combat's Tactics, crops, cargo and siege mortars.
type raidTactics struct{ g *Game }

func (h raidTactics) CoverSpot(id entity.EntityID, x, y, r int, d weapon.Def, tx, ty int) (int, int, int, int, bool) {
	return h.g.combat.Tactics.BestCover(id, x, y, r, d, tx, ty)
}
func (h raidTactics) Danger(x, y int) (int, int, int, bool) { return h.g.combat.Tactics.Danger(x, y) }
func (h raidTactics) Colonists(x, y, r int, buf []raid.Spot) []raid.Spot {
	for _, s := range h.g.combat.Tactics.Colonists(x, y, r) {
		buf = append(buf, raid.Spot(s))
	}
	return buf
}
func (h raidTactics) Block(id entity.EntityID, tx, ty int) (entity.EntityID, int, int, bool) {
	return h.g.combat.Tactics.Block(id, tx, ty)
}
func (h raidTactics) Bash(id, b entity.EntityID, d weapon.Def) bool {
	return h.g.combat.Tactics.Bash(id, b, d)
}
func (h raidTactics) Crop(x, y int) bool {
	p := h.g.plants.Ref(h.g.plantAt(x, y))
	return p != nil && p.Zone && h.g.fireAt(x, y) == 0
}
func (h raidTactics) Drop(k item.Kind, qty, x, y int) { _, _ = h.g.SpawnStack(k, qty, x, y) }
func (h raidTactics) Restock(m entity.EntityID)       { h.g.combat.Mortars.Raise(m) }

// raidNest places hive mounds (raid/nest.go, CBT-4): dark (below 30 %
// light, original), warmer than -17 °C (wiki: Infestation) and roofed.
type raidNest struct{ g *Game }

func (h raidNest) HiveSpot(x, y int) bool {
	g := h.g
	return g.grid.InBounds(x, y) && g.roofAt(x, y).Roofed() && g.dryWalkable(x, y) && g.layerBldgAt(x, y, construction.LayerObject) == 0 &&
		g.temperatureAt(x, y) > -170 && g.cellLightPermil(x, y) < 300
}
func (h raidNest) PlaceHive(x, y int) entity.EntityID {
	return placeAncient(h.g, construction.HiveMound, x, y)
}
func (h raidNest) HiveStands(id entity.EntityID) bool {
	b, ok := h.g.bldgs.Get(id)
	return ok && b.Kind == construction.HiveMound
}

// raidKidnap is what kidnappers need (raid/kidnap.go, moved from the
// game's kidnap.go by CBT-4).
type raidKidnap struct{ g *Game }

func (h raidKidnap) Colonists() []entity.EntityID { return h.g.pawnIDs }
func (h raidKidnap) Carrier(t entity.EntityID) (entity.EntityID, bool) {
	return h.g.care.Carry.Carrier(t), h.g.isStashed(t)
}
func (h raidKidnap) Carrying(id entity.EntityID) entity.EntityID { return h.g.care.Carry.Patient(id) }
func (h raidKidnap) Pick(id, t entity.EntityID)                  { h.g.care.Carry.Pick(id, t) }
func (h raidKidnap) Claimed(t, id entity.EntityID) bool {
	owner, held := h.g.reserve.EntityOwner(t)
	return held && owner != id
}
func (h raidKidnap) Beside(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.besideSpot(id, x, y)
}
func (h raidKidnap) Path(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, x, y)
}

// Off: raider id reached the edge with colonist t; both leave the map, t
// held by the raider's faction (a kidnapped pawn's stash holder is its
// captor faction, 1 + index).
func (h raidKidnap) Off(id, t entity.EntityID) {
	g := h.g
	g.care.Carry.Drop(id)
	pos, _ := g.positions.Get(id)
	g.positions.Set(t, pos)
	fac := 0
	if p := g.personOf(id); p != nil {
		fac = p.Faction
	}
	by := "raiders"
	if f := g.factionOf(fac); f != nil {
		by = f.Name
	}
	if g.stashPawn(t, entity.EntityID(fac)) {
		g.setWhere(t, whereKidnapped)
		name := g.personName(t)
		g.socialOnLostAs(t, "kidnapped", name+" was kidnapped by "+by+".") // social_grief.go
		g.letter(letterThreat, "Kidnapped", name+" was kidnapped by "+by+".", t)
	}
	g.removeRaider(id) // raids.go: off the map, an off-map person again
	g.invalidateCreatures()
}
