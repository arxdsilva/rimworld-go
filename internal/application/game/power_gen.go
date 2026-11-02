package game

import (
	"math"

	"rimworld-go/internal/application/game/industry"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// PLAY-021 generator outputs (FUELGEN owns this file): what each producer
// makes right now, in integer W. The balance (power.go) adds them up per
// net and applies the switches and the solar flare; genOutput is the
// potential output of the piece itself.
//
//   - wood-fired: 800 W while it holds fuel;
//   - solar: up to 900 W following daylight (lighting.Outdoor, zero at the
//     night floor), times the sky's share of the sun (weather.SunFactor,
//     the factor the renderer dims daylight by) and the share of its
//     footprint not roofed;
//   - wind: 250 W per unit of wind speed, at most 1100 W, times the clear
//     share of a band beside the turbine;
//   - geothermal: 1800 W, always.
//
// Floats become permille once per call and the rest is integer maths.
const (
	woodGenW  = 800
	solarMaxW = 900
	// solarNightPermil is lighting.Outdoor's night floor (0.12): the sun
	// gives nothing at or below it.
	solarNightPermil = 120
	windPerSpeedW    = 250
	windMaxW         = 1100
	geothermalW      = 1800
	chemfuelGenW     = 1000 // AX-4, wiki: Chemfuel powered generator
)

// genOutput is generator id's output in W (0 for a piece that makes none,
// or one that is gone).
func (g *Game) genOutput(id entity.EntityID) int {
	b, ok := g.bldgs.Get(id)
	if !ok {
		return 0
	}
	switch b.Kind {
	case construction.WoodGenerator:
		if g.fuel[id] > 0 {
			return woodGenW
		}
	case construction.GeothermalGenerator:
		return geothermalW
	case construction.ShipReactor: // AX-8, wiki: Ship reactor (1000 W, started or not)
		return 1000
	case construction.SolarPanel:
		return g.solarOutput(b)
	case construction.WindTurbine:
		return g.windOutput(id, b)
	case construction.ChemfuelGenerator: // AX-4: burns chemfuel (fuel.go)
		if g.fuel[id] > 0 {
			return chemfuelGenW
		}
	case construction.Watermill:
		return g.br.ind.Mill(industryHost{g}).Output(id) // game/industry: the wheel in the river
	}
	return 0
}

// permil turns a fraction into rounded permille.
func permil(f float64) int { return int(math.Round(f * 1000)) }

// solarOutput is the panel's output: the sun above the night floor, scaled
// by the weather's share of the sun and the share of its footprint open
// to the sky (walls and roofs cover).
func (g *Game) solarOutput(b construction.Building) int {
	sun := max(0, permil(float64(skyLight(g)))-solarNightPermil) // light.go: none in an eclipse (FOOD-6)
	if sun == 0 {
		return 0
	}
	w := solarMaxW * sun / (1000 - solarNightPermil)
	w = w * int(weather.SunFactor(g.weather.Kind, g.weather.Intensity)) / 1000
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if len(cells) == 0 {
		return 0
	}
	sky := 0
	for _, c := range cells {
		if g.underSky(c[0], c[1]) {
			sky++
		}
	}
	return w * (sky * 1000 / len(cells)) / 1000
}

// windOutput is the turbine's output: the wind, times the clear share of
// its band.
func (g *Game) windOutput(id entity.EntityID, b construction.Building) int {
	speed := max(0, permil(g.weather.WindSpeed)) // thousandths of a tile per second
	w := min(windMaxW, windPerSpeedW*speed/1000)
	if w == 0 {
		return 0
	}
	return w * g.br.ind.WindClear(industryHost{g}, g.bldgs, id, b) / 1000 // game/industry: the clear band
}

// WindBlocked reports a cell that stops the wind: a roof, rock, a wall or
// door, a building that blocks movement, or a tree. Off the map is open.
func (h industryHost) WindBlocked(x, y int) bool {
	g := h.g
	if !g.grid.InBounds(x, y) {
		return false
	}
	if g.roofAt(x, y).Roofed() || terrain.IsRockWall(g.terr.At(x, y)) || g.layerBlockerAt(x, y) != 0 {
		return true
	}
	if b := g.bldgs.Ref(g.layerBldgAt(x, y, construction.LayerObject)); b != nil && (b.Kind == construction.Wall || b.Kind.IsDoor()) {
		return true
	}
	if p := g.plants.Ref(g.plantAt(x, y)); p != nil {
		return plant.DefOf(p.Species).Trunk && p.Species != plant.Cactus
	}
	return false
}

// industryHost is game/industry's roles (game/industry/deps.go, AX-4).
type industryHost struct{ g *Game }

func (h industryHost) StructVer() uint64              { return h.g.structVer }
func (h industryHost) RoofVer() uint64                { return h.g.roofVer() }
func (h industryHost) Hour() uint64                   { return uint64(h.g.clock.Now()) / clock.TicksPerHour }
func (h industryHost) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h industryHost) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h industryHost) Powered(id entity.EntityID) bool     { return h.g.powered(id) }
func (h industryHost) Size() (int, int)                    { return h.g.grid.Width(), h.g.grid.Height() }
func (h industryHost) Terrain(x, y int) terrain.Type       { return h.g.terr.At(x, y) }
func (h industryHost) SetTerrain(x, y int, t terrain.Type) { h.g.setTerrain(x, y, t) }
func (h industryHost) Ocean(x, y int) bool                 { return h.g.isOcean(x, y) }
func (h industryHost) StoneAt(x, y int) geology.Stone      { return h.g.stoneAt(x, y) }
func (h industryHost) Seed() uint64                        { return h.g.streams.seed }
func (h industryHost) Drop(k item.Kind, n, x, y int)       { h.g.dropStack(k, n, x, y) }
func (h industryHost) Changed()                            { h.g.viewGen++ }
func (h industryHost) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h industryHost) Work(id entity.EntityID, mining bool) int {
	if mining {
		return h.g.workProgress(id, skMining, true) // skills.go: trains Mining
	}
	return h.g.workProgress(id, skIntellectual, true)
}
func (h industryHost) Letter(title, text string, x, y int) {
	h.g.post(MessageView{Tick: uint64(h.g.clock.Now()), Text: text, X: x, Y: y, Letter: true, Title: title, Kind: letterGood})
}
func (h industryHost) Free(worker, bench entity.EntityID, k job.Kind, sx, sy int) bool {
	if owner, taken := h.g.reserve.EntityOwner(bench); taken && owner != worker || h.g.postponedFor(worker, k, bench, 0, 0) {
		return false
	}
	o, held := h.g.reserve.TileOwner(sx, sy)
	return h.g.grid.Walkable(sx, sy) && (!held || o == worker)
}
func (h industryHost) Start(worker, bench entity.EntityID, k job.Kind, sx, sy, ticks int) bool {
	return h.g.startJob(worker, job.Record{Kind: k, Target: bench, TX: sx, TY: sy, WorkLeft: ticks}) &&
		h.g.reserve.TryTile(worker, sx, sy) // the held machine draws its full power (power.go powerWant)
}

// doOperate is one tick of a deep drill's or scanner's sitting (AX-4).
func (h industryHost) doOperate(id entity.EntityID, j job.Record) {
	switch h.g.br.ind.Ops(h).Do(id, j.Target) {
	case industry.Stop:
		j.State = job.Complete
	case industry.Blocked:
		j.State, j.Fail = job.Fail, job.Blocked // the power went (put off a while: postpone.go)
	default:
		if j.WorkLeft--; j.WorkLeft <= 0 {
			j.State = job.Complete
		}
	}
	h.g.jobs.Set(id, j)
}
