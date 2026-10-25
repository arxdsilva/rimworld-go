package game

import (
	"fmt"
	"math/bits"
	"slices"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/area"
)

// Allowed areas (ENG-003 M9, rebuilt for PLAY-015): g.areas is the
// colony's area set (up to area.MaxAreas named bitsets with stable ids,
// overlapping zones freely); nil until the first area is made. Saved as
// Areas. Each colonist's and tame animal's assignment is an area ref
// (worldState.areaOf, saved with World): an area id, or command.AreaHome
// for the live Home mask; absent means unrestricted.
//
// Areas are soft (spec-faithful): a restricted pawn's paths prefer its
// area (cells outside cost areaOutsidePct more, area_soft.go) but may
// cross outside; only destinations must lie inside (startJob turns the
// rest down, area_dest.go, with the postponement or food cooldown so the
// giver moves on). Drafted, breaking and downed pawns ignore areas; so do
// the player's own orders, rescue, medical care, warmth, fights, fleeing,
// a caravan's walk out, and animals following a drafted master. Penned
// livestock (animal.Roster.Penned) keep to pens instead. A restricted pawn
// or animal caught outside its area (painted out, newly assigned) walks
// back in.

const (
	areaNameMax = 24
	// areaDefaultName names an area created unnamed
	// ("Area 1").
	areaDefaultName = "Area %d"
)

// areaSet is the colony's area set, created on first use.
func (g *Game) areaSet() *area.Set {
	if g.areas == nil {
		g.areas = area.New(g.grid.Width(), g.grid.Height())
	}
	return g.areas
}

// areaRef is id's assigned area ref: an area id (> 0), command.AreaHome,
// or 0 for unrestricted (also once its area is gone).
func (g *Game) areaRef(id entity.EntityID) int {
	if g.worldSt == nil || len(g.worldSt.areaOf) == 0 {
		return 0
	}
	r, ok := g.worldSt.areaOf[id]
	switch {
	case !ok:
		return 0
	case r == command.AreaHome:
		return r
	case r > 0 && g.areas.Index(uint16(r)) >= 0:
		return r
	}
	return 0
}

// restrictedRef is the area ref that binds id right now (0: none). The
// overrides: drafted, breaking or downed colonists; wild animals, penned
// livestock (pens bind them, pens.go) and animals following a drafted
// master.
func (g *Game) restrictedRef(id entity.EntityID) int {
	r := g.areaRef(id)
	if r == 0 {
		return 0
	}
	if a := g.animals.Ref(id); a != nil {
		if !areaAnimal(*a) || a.Master != 0 && g.drafted(entity.EntityID(a.Master)) {
			return 0
		}
		return r
	}
	if g.drafted(id) || g.breaking(id) {
		return 0
	}
	if v, ok := g.vital.Get(id); ok && v.Status == health.Downed {
		return 0
	}
	return r
}

// areaAnimal reports a tame animal that allowed areas bind (not penned
// livestock, which pens hold).
func areaAnimal(a animal.State) bool {
	return a.Tame && !a.Dead() && !animal.RosterOf(a.Species).Penned
}

// inArea reports whether pawn or animal id may go to (x, y) under its
// allowed area: the one shared area test (always true for the
// unrestricted, the overridden and penned livestock).
func (g *Game) inArea(id entity.EntityID, x, y int) bool {
	r := g.restrictedRef(id)
	return r == 0 || g.refContains(r, x, y)
}

// refContains reports whether (x, y) lies in area ref r (an id or Home).
func (g *Game) refContains(r, x, y int) bool {
	if r == command.AreaHome {
		return g.grid.InBounds(x, y) && g.inHome(y*g.grid.Width()+x)
	}
	return g.areas.Contains(g.areas.Index(uint16(r)), x, y)
}

// refAllows reports that area ref r admits a destination at (x, y): the
// cell itself, or for a cell nobody stands on (a building or wall worked
// from beside) any cell beside it.
func (g *Game) refAllows(r, x, y int) bool {
	if g.refContains(r, x, y) {
		return true
	}
	if g.grid.Walkable(x, y) {
		return false
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx != 0 || dy != 0) && g.refContains(r, x+dx, y+dy) {
				return true
			}
		}
	}
	return false
}

// areaAllows is refAllows for id's binding area (true when unrestricted):
// the filter work givers use on a target cell.
func (g *Game) areaAllows(id entity.EntityID, x, y int) bool {
	r := g.restrictedRef(id)
	return r == 0 || g.refAllows(r, x, y)
}

// AreaOf is colonist or animal id's allowed area ref for the Schedule and
// Animals tabs: command.AreaUnrestricted, command.AreaHome or an area id
// (AreaView.ID).
func (g *Game) AreaOf(id entity.EntityID) int {
	if r := g.areaRef(id); r != 0 {
		return r
	}
	return command.AreaUnrestricted
}

// areasTick (every worldStep) walks each restricted colonist and tame
// animal standing outside its area, with nothing pressing to do, back in.
func (g *Game) areasTick(now uint64) {
	for _, id := range g.pawnIDs {
		r := g.restrictedRef(id)
		if r == 0 || !g.standingPawn(id) {
			continue
		}
		p, _ := g.positions.Get(id)
		if g.refContains(r, p.X, p.Y) {
			continue
		}
		j, _ := g.jobs.Get(id)
		if j.Active() && !j.Kind.IsIdle() {
			continue
		}
		x, y, ok := g.nearestRefCell(r, p)
		if !ok {
			continue
		}
		if j.Active() {
			g.interrupt(id, job.Interrupted)
		}
		g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y})
	}
	g.areaAnimalsTick(now)
}

// areaAnimalsTick walks restricted tame animals outside their area, idle
// (wandering, not on the way somewhere), back in.
func (g *Game) areaAnimalsTick(now uint64) {
	for _, id := range g.animalIDs {
		r := g.restrictedRef(id)
		if r == 0 {
			continue
		}
		a := g.animals.Ref(id)
		if a == nil || a.Mode != animal.Wander || a.PathI < len(a.Path) && g.refContains(r, a.GoalX, a.GoalY) {
			continue
		}
		p, _ := g.positions.Get(id)
		if g.refContains(r, p.X, p.Y) || now < a.RetryAt {
			continue
		}
		if x, y, ok := g.nearestRefCell(r, p); ok {
			g.animalGoTo(id, a, x, y)
		}
	}
}

// nearestRefCell is area ref r's walkable cell nearest to p (octile,
// then row-major), ok false for an empty area.
func (g *Game) nearestRefCell(r int, p Position) (int, int, bool) {
	w := g.grid.Width()
	best, bx, by := 1<<30, 0, 0
	try := func(n int) {
		x, y := n%w, n/w
		if d := pathing.Octile(x, y, p.X, p.Y); d < best && g.grid.Walkable(x, y) {
			best, bx, by = d, x, y
		}
	}
	if r == command.AreaHome {
		for n, c := range g.homeArea.cells {
			if c != 0 {
				try(n)
			}
		}
		return bx, by, best < 1<<30
	}
	i := g.areas.Index(uint16(r))
	if i < 0 {
		return 0, 0, false
	}
	for wi, word := range g.areas.Areas[i].Bits {
		for ; word != 0; word &= word - 1 {
			try(wi*64 + bits.TrailingZeros64(word))
		}
	}
	return bx, by, best < 1<<30
}

// areaViews lists the colony's allowed areas. Bits is shared read-only:
// every change to an area replaces its slice (copy-on-write) and bumps
// its Ver, so the renderer may cache by (ID, Ver).
func (g *Game) areaViews() []AreaView {
	if g.areas == nil {
		return nil
	}
	out := make([]AreaView, 0, len(g.areas.Areas))
	for i, a := range g.areas.Areas {
		out = append(out, AreaView{Index: i, Name: a.Name, Cells: g.areas.Count(i), ID: a.ID, Color: a.Color,
			Bits: a.Bits, Ver: g.areaVer(a.ID)})
	}
	return out
}

// areaVer is area id's version (0 until its first change this session).
func (g *Game) areaVer(id uint16) uint64 {
	if g.worldSt == nil {
		return 0
	}
	return g.worldSt.areaVer[id]
}

// touchArea readies area i for a change: its bits get a fresh copy (views
// handed out keep the old one) and its version a new number.
func (g *Game) touchArea(i int, copyBits bool) {
	a := &g.areas.Areas[i]
	if copyBits {
		a.Bits = slices.Clone(a.Bits)
	}
	w := g.worldS()
	if w.areaVer == nil {
		w.areaVer = map[uint16]uint64{}
	}
	w.areaGen++
	w.areaVer[a.ID] = w.areaGen
}

// cmdPaintArea paints (on) or clears area id ref (AreaView.ID) over the
// rectangle; an unknown id is ignored (CreateArea makes areas).
func (g *Game) cmdPaintArea(ref, x, y, x2, y2 int, on bool) {
	i := g.areaIndexOf(ref)
	if i < 0 {
		return
	}
	g.touchArea(i, true)
	g.areas.Paint(i, x, y, x2, y2, on)
}

// areaName trims a player-typed name to areaNameMax runes ("" if blank).
func areaName(name string) string {
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > areaNameMax {
		name = string(r[:areaNameMax])
	}
	return name
}

// cmdRenameArea renames area id ref (a blank name or unknown id: no-op).
func (g *Game) cmdRenameArea(ref int, name string) {
	i := g.areaIndexOf(ref)
	if name = areaName(name); name == "" || i < 0 {
		return
	}
	g.areas.Rename(i, name)
	g.touchArea(i, false)
}

// cmdCreateArea adds an empty area named name ("Area n", n its id, when
// blank), below the cap.
func (g *Game) cmdCreateArea(name string) {
	s := g.areaSet()
	if name = areaName(name); name == "" {
		name = fmt.Sprintf(areaDefaultName, s.LastID+1)
	}
	if i := s.Add(name); i >= 0 {
		g.touchArea(i, false)
	}
}

// areaIndexOf is the position of the area with id ref (a command's
// Index), or -1.
func (g *Game) areaIndexOf(ref int) int {
	if ref <= 0 || ref > 0xffff || g.areas == nil {
		return -1
	}
	return g.areas.Index(uint16(ref))
}

// cmdDeleteArea deletes area id ref; every colonist and animal assigned
// to it becomes unrestricted, with one message saying how many.
func (g *Game) cmdDeleteArea(ref int) {
	i := g.areaIndexOf(ref)
	if i < 0 {
		return
	}
	g.areas.Delete(uint16(ref))
	n := 0
	if w := g.worldSt; w != nil {
		for id, r := range w.areaOf {
			if r == ref {
				delete(w.areaOf, id) // ids are counted, not ordered: no output depends on the order
				n++
			}
		}
		delete(w.areaVer, uint16(ref))
	}
	switch {
	case n == 1:
		g.message(0, 0, "1 pawn is now unrestricted") // no place to jump to
	case n > 1:
		g.message(0, 0, "%d pawns are now unrestricted", n)
	}
}

// cmdInvertArea swaps area id ref's member and non-member cells.
func (g *Game) cmdInvertArea(ref int) {
	if i := g.areaIndexOf(ref); i >= 0 {
		g.touchArea(i, true)
		g.areas.Invert(i)
	}
}

// cmdAssignArea restricts colonist or tame animal pawn to area ref r (an
// area id, command.AreaHome, or command.AreaUnrestricted / 0 to free
// it); a job or walk ending outside the new area stops.
func (g *Game) cmdAssignArea(pawn entity.EntityID, r int) {
	if r == command.AreaUnrestricted || r == 0 {
		if g.worldSt != nil {
			delete(g.worldSt.areaOf, pawn)
		}
		return
	}
	if r != command.AreaHome && g.areaIndexOf(r) < 0 {
		return
	}
	a := g.animals.Ref(pawn)
	switch {
	case a != nil:
		if !areaAnimal(*a) {
			return // wild, dead, or penned livestock (pens hold those)
		}
	case !g.isPawn(pawn) || g.isRaider(pawn) || !g.alive(pawn) && !g.isStashed(pawn):
		return
	}
	w := g.worldS()
	if w.areaOf == nil {
		w.areaOf = map[entity.EntityID]int{}
	}
	w.areaOf[pawn] = r
	if a != nil {
		if a.PathI < len(a.Path) && !g.inArea(pawn, a.GoalX, a.GoalY) {
			a.Path, a.PathI = nil, 0 // it decides again, inside
		}
		return
	}
	if j, ok := g.jobs.Get(pawn); ok && j.Active() && g.areaRejects(pawn, j) {
		g.interrupt(pawn, job.Interrupted)
	}
}

// importAreaRef is a saved assignment as an area ref: a Version 1 save
// held area positions, which import as ids 1..n (area.Import).
func importAreaRef(version uint8, v int) int {
	if version < worldSaveVersion && v >= 0 {
		return v + 1
	}
	return v
}
