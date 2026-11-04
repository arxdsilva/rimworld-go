package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// senseTidy is sense.TidyRoles over the game (CSE-017…027): the cleaning
// adapters and what only a tidy asks.
type senseTidy struct {
	senseFilth
	senseGround
	senseWalks
	senseLabor
	tidyCrew
	tidyRooms
	tidyHands
}

func newSenseTidy(g *Game) senseTidy {
	return senseTidy{senseFilth{g}, senseGround{g}, senseWalks{g}, senseLabor{g}, tidyCrew{g}, tidyRooms{g}, tidyHands{g}}
}

type tidyCrew struct{ g *Game }

func (h tidyCrew) Now() uint64                      { return uint64(h.g.clock.Now()) }
func (h tidyCrew) Colonist(id entity.EntityID) bool { return careRoster(h).IsColonist(id) }
func (h tidyCrew) CanClean(id entity.EntityID) bool {
	return !h.g.incapableWork(id).Enabled(work.Clean) && h.g.prioOf(id)[work.Clean] > 0
}
func (h tidyCrew) Fit(id entity.EntityID) bool {
	g := h.g
	p, ok := g.positions.Get(id)
	return ok && g.alive(id) && !g.drafted(id) && !g.breaking(id) && !g.needsTending(id) && !g.care.Rest.On(id) && g.fireAt(p.X, p.Y) == 0
}
func (h tidyCrew) Ravenous(id entity.EntityID) bool {
	n, _ := h.g.need.Get(id)
	return n.Hunger <= needs.RavenousAt
}
func (h tidyCrew) OrdersWaiting(id entity.EntityID) bool       { return h.g.orderDue(id) }
func (h tidyCrew) HoldHunger(id entity.EntityID, until uint64) { h.g.noFood[id] = until }
func (h tidyCrew) ReleaseHunger(id entity.EntityID)            { delete(h.g.noFood, id) }

type tidyRooms struct{ g *Game }

func (h tidyRooms) RoomAt(x, y int) int { return h.g.roomAt(x, y) }
func (h tidyRooms) RoomDirt(r int) (sum, cells int) {
	if r < 0 || r >= len(h.g.rooms) {
		return 0, 0
	}
	sum, _ = h.g.roomFilth(r)
	return sum, h.g.rooms[r].contents.Cells
}
func (h tidyRooms) RoomGrid() []int {
	h.g.ensureRooms()
	return h.g.roomGrid
}
func (h tidyRooms) MealSeat(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.rest.Seats.AtTable(id, x, y)
}

// PathLen from where id stands spends the tick's long-search budget, as a
// job start does; only a walk that already was one searches on from elsewhere.
func (h tidyRooms) PathLen(id entity.EntityID, sx, sy, tx, ty int) (int, bool) {
	g := h.g
	if p, _ := g.positions.Get(id); p.X == sx && p.Y == sy {
		path, ok := g.pathToJob(id, tx, ty)
		return len(path), ok
	}
	path, ok := g.findPath(id, g.pathQuery(id), sx, sy, tx, ty)
	return len(path), ok
}
func (h tidyRooms) PathFor(id entity.EntityID, tx, ty int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, tx, ty)
}
func (h tidyRooms) TendUrgent(id, patient entity.EntityID) bool {
	return h.g.care.Doctor.Urgent(id, patient, h)
}
func (h tidyRooms) BleedDeathHours(id entity.EntityID) int { return h.g.bleedDeathHours(id) }
func (h tidyRooms) BenchThere(id entity.EntityID) bool {
	for _, m := range h.g.maps {
		if _, ok := m.bldgs.Get(id); ok {
			return true
		}
	}
	return false
}

type tidyHands struct{ g *Game }

func (h tidyHands) JobKind(id entity.EntityID) job.Kind {
	for _, m := range h.g.maps {
		if j, ok := m.jobs.Get(id); ok {
			return j.Kind
		}
	}
	return 0
}

// DropHeld puts down a stack id still holds for the job a tidy interrupted,
// and whatever its current job holds with it: dropCarry reads the job.
func (h tidyHands) DropHeld(id, stack entity.EntityID) {
	g := h.g
	if st, ok := g.stacks.Get(stack); !ok || st.Carrier != id {
		return
	}
	g.dropCarry(id)
	j, _ := g.jobs.Get(id)
	j.Carry = stack
	g.jobs.Set(id, j)
	g.dropCarry(id)
}
func (h tidyHands) TryTile(id entity.EntityID, x, y int) bool { return h.g.reserve.TryTile(id, x, y) }
func (h tidyHands) LeaveTile(id entity.EntityID, x, y int) {
	if owner, taken := h.g.reserve.TileOwner(x, y); taken && owner == id {
		h.g.reserve.ReleaseTile(x, y)
	}
}
func (h tidyHands) ClaimEntity(id, target entity.EntityID) { h.g.reserve.TryEntity(id, target) }
