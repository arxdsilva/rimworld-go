package game

import (
	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/terrain"
)

// restHost implements rest.Host (rest/deps.go) over the game: one small
// adapter type per role, each a thin view of *Game, so no method is added
// to Game itself (.cursor/SOLID_GO.md §10).
type restHost struct {
	restBodies
	restBedding
	restClaims
	restRoutine
	restPatients
	restRooms
	restEvents
	restCouples
}

var _ rest.Host = restHost{}

func newRestHost(g *Game) restHost {
	return restHost{restBodies{g}, restBedding{g}, restClaims{g}, restRoutine{g}, restPatients{g}, restRooms{g}, restEvents{g}, restCouples{g}}
}

type restBodies struct{ g *Game }

func (h restBodies) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h restBodies) Vital(id entity.EntityID) (health.Record, bool) { return h.g.vital.Get(id) }
func (h restBodies) SetVital(id entity.EntityID, v health.Record)   { h.g.vital.Set(id, v) }
func (h restBodies) Needs(id entity.EntityID) needs.Pair {
	n, _ := h.g.need.Get(id)
	return n
}
func (h restBodies) SetNeeds(id entity.EntityID, n needs.Pair)        { h.g.need.Set(id, n) }
func (h restBodies) SetJob(id entity.EntityID, j job.Record)          { h.g.jobs.Set(id, j) }
func (h restBodies) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }

// FloorSpot is where pawn id at (x, y) lies down with no bed: where it
// stands, or, standing outside its area, the nearest cell inside.
func (h restBodies) FloorSpot(id entity.EntityID, x, y int) (int, int) {
	g := h.g
	r := g.restrictedRef(id)
	if r == 0 || g.refContains(r, x, y) {
		return x, y
	}
	if fx, fy, ok := g.nearestRefCell(r, Position{X: x, Y: y}); ok {
		return fx, fy
	}
	return x, y
}

type restBedding struct{ g *Game }

func (h restBedding) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h restBedding) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h restBedding) SetBuilding(id entity.EntityID, b construction.Building) { h.g.bldgs.Set(id, b) }
func (h restBedding) SleptAt(id entity.EntityID) (entity.EntityID, bool) {
	bed, ok := h.g.sleptAt[id]
	return bed, ok
}
func (h restBedding) SetSleptAt(id, bed entity.EntityID) { h.g.sleptAt[id] = bed }
func (h restBedding) ForgetSleptAt(id entity.EntityID)   { delete(h.g.sleptAt, id) }
func (h restBedding) ReleaseBed(id entity.EntityID)      { h.g.releaseBed(id) }
func (h restBedding) SleepCell(id entity.EntityID, b construction.Building) (int, int) {
	return h.g.sleepCellFor(id, b)
}

type restClaims struct{ g *Game }

func (h restClaims) AreaAllows(id entity.EntityID, x, y int) bool { return h.g.areaAllows(id, x, y) }
func (h restClaims) MayOwnBed(id entity.EntityID) bool {
	if !h.g.colonist(id) {
		return false
	}
	v, ok := h.g.vital.Get(id)
	return ok && v.Status != health.Dead
}
func (h restClaims) Partner(id entity.EntityID) entity.EntityID {
	g := h.g
	if !g.romanceable(id) {
		return 0
	}
	p := g.partnerOf(id) // social_romance.go
	if p == 0 || !g.onMap(p) || !g.romanceable(p) {
		return 0
	}
	return p
}

func (h restClaims) Reachable(id entity.EntityID, x, y int) bool { return h.g.reachable(id, x, y) }
func (h restClaims) ReserveTile(id entity.EntityID, x, y int)    { h.g.reserve.TryTile(id, x, y) }
func (h restClaims) HeldByOther(id, b entity.EntityID) bool {
	return h.g.reserve.FreeFor(id, b, 1) == 0
}
func (h restClaims) TileFree(id entity.EntityID, x, y int) bool {
	if !h.g.grid.Walkable(x, y) || h.g.bedCovering(x, y) {
		return false
	}
	owner, taken := h.g.reserve.TileOwner(x, y)
	return !taken || owner == id
}

type restRoutine struct{ g *Game }

func (h restRoutine) Activity(id entity.EntityID) schedule.Activity { return h.g.activityOf(id) }
func (h restRoutine) SeekFood(id entity.EntityID) bool              { return h.g.seekFood(id) }
func (h restRoutine) Job(id entity.EntityID) job.Record {
	j, _ := h.g.jobs.Get(id)
	return j
}

func (h restRoutine) Band(id entity.EntityID) (int, int) {
	b := h.g.bandsOf(id)
	return b.Min, b.Max
}

type restPatients struct{ g *Game }

func (h restPatients) OnBedRest(id entity.EntityID) bool { return h.g.care.Rest.On(id) }
func (h restPatients) MayUseMedicalBed(id entity.EntityID) bool {
	return h.g.care.Rest.MayUseMedicalBed(id)
}

// SurgeryWait: one only waiting for a surgeon who never came gets up
// after patientWait (surgery.go: don't lie straight back down).
func (h restPatients) SurgeryWait(id entity.EntityID, workLeft int) {
	if workLeft <= -patientWait && h.g.awaitingSurgery(id) {
		h.g.surgeryGaveUp(id)
	}
}

type restRooms struct{ g *Game }

func (h restRooms) RoomAt(x, y int) int    { return h.g.roomAt(x, y) }
func (h restRooms) UnderSky(x, y int) bool { return h.g.underSky(x, y) }

// RoomQuality reads the rebuild's tally with the live filth, items and
// bed flags (BED-2).
func (h restRooms) RoomQuality(r int) (room.Role, int) {
	if h.g.ensureRooms(); r < 0 || r >= len(h.g.rooms) {
		return room.General, 0
	}
	f, rs := h.g.regs.rooms.countFacts(h.g)[r], h.g.rooms[r]
	sum, _ := h.g.roomFilth(r)
	return rs.tally.Role(rs.contents, f.MedicalBeds, f.PrisonerBeds), rs.tally.Stats(rs.contents, sum, f.Wealth).ImpressivenessTenths
}
func (h restRooms) ObjectAt(x, y int) entity.EntityID {
	return h.g.layerBldgAt(x, y, construction.LayerObject)
}
func (h restRooms) TameAnimal(id entity.EntityID) bool {
	a, ok := h.g.animals.Get(id)
	return ok && a.Tame && !a.Dead()
}

func (h restRooms) TempAt(x, y int) int { return int(h.g.temperatureAt(x, y)) }
func (h restRooms) Roof(x, y int) (bool, bool) {
	c := h.g.roofAt(x, y)
	return c.Roofed(), c.Thick()
}

// Tile is what lies on (x, y), for its beauty (BED-3).
func (h restRooms) Tile(x, y int) rest.Tile {
	g := h.g
	if !g.grid.InBounds(x, y) {
		return rest.Tile{}
	}
	fb, floored := g.bldgs.Get(g.layerBldgAt(x, y, construction.LayerFloor))
	t := rest.Tile{Object: g.layerBldgAt(x, y, construction.LayerObject), Indoors: g.roomAt(x, y) >= 0,
		Floor: room.FloorUnder(fb.Stuff, floored, g.terr.At(x, y) == terrain.StoneSmooth), Filth: g.filthTotal(y*g.grid.Width() + x)}
	if p, ok := g.plants.Get(g.plantAt(x, y)); ok {
		t.Plant, t.Planted, t.Mature = p.Species, true, p.Ripe()
	}
	return t
}

type restEvents struct{ g *Game }

func (h restEvents) RememberWith(id entity.EntityID, k mood.Kind, o entity.EntityID) {
	h.g.memories[id] = h.g.memories[id].AddFor(k, uint64(o), uint64(h.g.clock.Now()))
}
func (h restEvents) Remember(id entity.EntityID, k mood.Kind) { h.g.remember(id, k) }
func (h restEvents) RememberValue(id entity.EntityID, k mood.Kind, v int, label string) {
	h.g.memories[id] = h.g.memories[id].AddLabeled(k, 0, v, label, uint64(h.g.clock.Now()))
}
func (h restEvents) ForgetGroup(id entity.EntityID, k mood.Kind) {
	h.g.memories[id] = h.g.memories[id].ForgetGroupOf(k)
}
func (h restEvents) Slept(id entity.EntityID, inBed bool) {
	if inBed {
		h.g.logEvent(id, logEntry{Kind: logSleptBed})
	} else {
		h.g.logEvent(id, logEntry{Kind: logSleptGround})
	}
}

type restCouples struct{ g *Game }

func (h restCouples) Mate(id entity.EntityID) entity.EntityID  { return h.g.partnerOf(id) }
func (h restCouples) Opinion(from, to entity.EntityID) int32   { return h.g.opinion(from, to) }
func (h restCouples) Age(id entity.EntityID) int               { return h.g.socialPerson(id).AgeYears }
func (h restCouples) LovinAt(id entity.EntityID) uint64        { return h.g.gatherSt().lovinAt[id] }
func (h restCouples) SetLovinAt(id entity.EntityID, at uint64) { h.g.gatherSt().lovinAt[id] = at }
func (h restCouples) SocialRoll(n int) int                     { return h.g.stream(streamSocial).IntN(n) }
func (h restCouples) Standing(id entity.EntityID) bool {
	return h.g.alive(id) && !h.g.breaking(id) && !h.g.drafted(id)
}
func (h restCouples) Hurt(id entity.EntityID) (int32, int32, bool) {
	if bd := h.g.bodies[id]; bd != nil {
		return bd.Pain(), bd.Capacity(body.Consciousness), true
	}
	return 0, 0, false
}
