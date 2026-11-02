package game

import (
	"fmt"

	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Research lives in game/research (RSCH-1: the wiki's speed, costs, bench
// tiers, analyzer link, letter and alert); this file is its adapter.
// Sandbox games (New) start with everything researched; colonies
// (NewColony) with their scenario's starting research.

// ResearchView is one project for the Research tab.
type ResearchView = rlab.View

// researchHost is game/research's roles (game/research/deps.go).
type researchHost struct{ g *Game }

func (h researchHost) StructVer() uint64              { return h.g.structVer }
func (h researchHost) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h researchHost) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h researchHost) Powered(id entity.EntityID) bool { return h.g.powered(id) }
func (h researchHost) Room(x, y int) (int, bool) {
	r := h.g.roomAt(x, y)
	return r, r >= 0 && r < len(h.g.rooms) && !h.g.thermo.PsychOutdoors(r) // TEMP-6: a room that feels outdoors
}
func (h researchHost) Cleanliness(r int) int {
	sum, _ := h.g.roomFilth(r) // rooms.go, as craftShop.Cleanliness
	rs := h.g.rooms[r]
	return rs.tally.Inputs(rs.contents.Cells, rs.contents.Floored-rs.tally.Covered, sum, 0).CleanlinessHundredths
}
func (h researchHost) Contents(r int) (int, int) {
	return h.g.rooms[r].contents.Beds, h.g.rooms[r].contents.Benches
}
func (h researchHost) TemperatureAt(x, y int) int { return int(h.g.temperatureAt(x, y)) }
func (h researchHost) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h researchHost) Free(worker, bench entity.EntityID, sx, sy int) bool {
	g := h.g
	if owner, taken := g.reserve.EntityOwner(bench); taken && owner != worker || g.postponedFor(worker, job.Research, bench, 0, 0) {
		return false
	}
	o, held := g.reserve.TileOwner(sx, sy)
	return g.grid.Walkable(sx, sy) && (!held || o == worker) // RES-1: another worker holds the work spot
}
func (h researchHost) Start(worker, bench entity.EntityID, sx, sy, ticks int) bool {
	return h.g.startJob(worker, job.Record{Kind: job.Research, Target: bench, TX: sx, TY: sy, WorkLeft: ticks}) &&
		h.g.reserve.TryTile(worker, sx, sy) // RES-1: the work spot is the researcher's
}
func (h researchHost) Work(id entity.EntityID) bool {
	ok := h.g.workTick(id)
	h.g.workProgress(id, skIntellectual, true) // skills.go: trains Intellectual
	return ok
}
func (h researchHost) Speed(id entity.EntityID) rlab.Pawn {
	g, manip, sight, trait := h.g, int32(1000), int32(1000), 100
	if b := g.bodies[id]; b != nil {
		manip, sight = b.Capacity(body.Manipulation), b.Capacity(body.Sight)
	}
	ws := int(g.statOf(id, stat.WorkSpeed))
	if manip != 1000 { // body.go: WorkSpeed carries manipulation at full weight; research counts it at half
		ws = ws * 1000 / int(max(manip, 1))
	}
	if fx := g.sk.stats[id]; fx != nil {
		trait = int(fx.workPct[skIntellectual])
	}
	return rlab.Pawn{Intellectual: g.skillOf(id, skIntellectual).Level, GlobalWork: ws, Manipulation: int(manip), Sight: int(sight),
		TraitPct: trait}
}
func (h researchHost) Researcher() bool {
	for _, id := range h.g.pawnIDs {
		m, _ := h.g.labor.Get(id)
		if h.g.colonist(id) && h.g.alive(id) && !h.g.prison.Is(id) && m.Enabled(work.Research) && !h.g.incapableWork(id).Enabled(work.Research) {
			return true
		}
	}
	return false
}
func (h researchHost) Tech() research.TechLevel {
	if !h.g.hasSetup() {
		return research.Industrial
	}
	return research.LevelOf(h.g.ng.fx.Tech)
}
func (h researchHost) CostPct() int {
	if !h.g.hasSetup() || h.g.ng.fx.ResearchCostPct <= 0 {
		return 100
	}
	return h.g.ng.fx.ResearchCostPct
}
func (h researchHost) Difficulty() int {
	if !h.g.hasSetup() {
		return 100
	}
	return h.g.ng.setup.Difficulty.ResearchSpeed
}
func (h researchHost) Now() uint64 { return uint64(h.g.clock.Now()) }
func (h researchHost) Letter(title, text string, x, y int) {
	h.g.post(MessageView{Tick: uint64(h.g.clock.Now()), Text: text, X: x, Y: y, Letter: true, Title: title, Kind: letterGood})
}
func (h researchHost) Changed() { h.g.viewGen++ } // view cache (indexes.go)

func (g *Game) buildLocked(k construction.Kind) bool {
	return g.lab.LockedBy("build:"+k.String()) != ""
}

func (g *Game) cropLocked(s plant.Species) bool {
	return g.lab.LockedBy("crop:"+plant.DefOf(s).Name) != ""
}

func (g *Game) recipeLocked(id recipe.ID) bool {
	return g.lab.LockedBy("recipe:"+recipe.DefOf(id).Name) != ""
}

func (g *Game) doResearch(id entity.EntityID, j job.Record) {
	switch g.lab.Work(id, j.Target) {
	case rlab.Stop:
		j.State = job.Complete
	case rlab.Blocked:
		j.State, j.Fail = job.Fail, job.Blocked // PLAY-021: the power went (put off a while: postpone.go)
	default:
		if j.WorkLeft--; j.WorkLeft <= 0 {
			j.State = job.Complete
		}
	}
	g.jobs.Set(id, j)
}

// CompleteResearch finishes every project (dev hook).
func (g *Game) CompleteResearch() { g.lab.Complete() }

// AddResearch puts points into a project by name (dev scenarios).
func (g *Game) AddResearch(name string, pts int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if p, ok := research.Parse(name); ok {
		g.lab.Prog.Add(p, pts)
	}
}

// PickResearch sets the current project by name (dev scenarios).
func (g *Game) PickResearch(name string) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if p, ok := research.Parse(name); ok {
		g.lab.Pick(p)
	}
}

// ResetResearch forgets all research (fixtures: a fresh colony's tree).
func (g *Game) ResetResearch() { g.lab.Reset() }

// migrateBenches (save v9 RSCH-1, v10 STK-BN; game/resize) handles kinds
// whose footprint grew. A bench whose new footprint and work spot still
// fit (in bounds, standable ground, no other object or blueprint) keeps
// its origin and facing and blocks its new cells; any other is taken down
// and refund's materials (its older cost) returned, with a message. Their
// blueprints are cancelled (delivered materials drop): the player places
// them again at the new size.
func (g *Game) migrateBenches(grew func(construction.Kind) bool, refund func(construction.Building) (item.Kind, int, int)) {
	changed := false
	for _, id := range append([]entity.EntityID(nil), g.frameIDs...) {
		if fr, ok := g.frames.Get(id); ok && grew(fr.Kind) {
			g.cancelFrame(id) // frame_view.go
			changed = true
		}
	}
	for _, id := range append([]entity.EntityID(nil), g.bldgIDs...) {
		b, ok := g.bldgs.Get(id)
		if !ok || !grew(b.Kind) {
			continue
		}
		changed = true
		cells, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		if g.benchFits(id, append(cells, spot)) {
			for _, c := range cells {
				g.grid.SetWalkable(c[0], c[1], g.grid.Walkable(c[0], c[1]) && !construction.DefOf(b.Kind).BlocksMove) // A2: a table blocks nothing
			}
			continue
		}
		g.removeBuildingAs(id, false)
		stuff, n, comps := refund(b)
		g.dropStack(stuff, n, b.X, b.Y)
		g.dropStack(item.Components, comps, b.X, b.Y) // none: SpawnStack refuses 0
		g.message(b.X, b.Y, "%s", fmt.Sprintf("The %s no longer fits at its larger size; it was taken down and its materials returned.",
			map[bool]string{true: rlab.HiTechLabel, false: construction.DefOf(b.Kind).Name}[b.Kind == construction.ElectricResearchBench]))
	}
	if changed {
		g.markRoomsDirty()
		g.structVer++
	}
}

// benchFits reports every cell in bounds, on standable ground, clear of
// any other object-layer building and of blueprints.
func (g *Game) benchFits(self entity.EntityID, cells [][2]int) bool {
	for _, c := range cells {
		if !g.grid.InBounds(c[0], c[1]) || !g.terr.At(c[0], c[1]).Walkable() && !g.bridged(c[0], c[1]) || g.blockingFrameAt(c[0], c[1]) != 0 {
			return false
		}
		for _, oid := range g.bldgIDs {
			o, ok := g.bldgs.Get(oid)
			if ok && oid != self && construction.DefOf(o.Kind).Layer == construction.LayerObject && construction.Covers(o.Kind, o.Facing, o.X, o.Y, c[0], c[1]) {
				return false
			}
		}
	}
	return true
}
