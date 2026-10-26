package game

import (
	"math"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/chunk"
	"rimworld-go/internal/domain/world/fire"
	"rimworld-go/internal/domain/world/stain"
	"rimworld-go/internal/domain/world/temperature"
)

// Fire (PLAY-015 §5.2): the burning cells, stepped every fire.StepTicks.
// The rules are the fire package's; the game sums each cell's fuel, rolls
// the spread from a hash of the stream seed, the tick and the two cells
// (never g.rng: a fire draws nothing from the main stream, and the rolls
// don't depend on the order fires are visited), and applies the damage.
// Fires are kept sorted y then x and indexed per cell and per chunk, so no
// step scans the map.

const (
	fireSalt = 0xF12E5EED
	// Sizes a fire starts at: from the developer action, a bursting
	// animal and a lightning strike.
	devFireSize       = 30
	burstFireSize     = 20
	lightningFireSize = 25
	// fireHeatSize: a fire this big heats its room like one campfire.
	fireHeatSize = 50
	// fireOutdoorHeat is the outdoor warming per size point, tenths of a
	// degree C, within fireWarmReach cells (a 5x5 patch): +20 C beside a
	// full blaze.
	fireOutdoorHeat = 2
	fireWarmReach   = 2
	// Ash left on the ground (stain amounts, of stain.Full).
	ashPlant    = 600
	ashGrass    = 250
	ashItem     = 300
	ashBuilding = 700
)

// fireRuntime is the fires' derived state, never saved.
type fireRuntime struct {
	// cell is 1 + the fire's index in fires per map cell (0: none); chunk
	// counts the fires per chunk. Both nil until the first fire.
	cell  []int32
	chunk []uint16
	cw    int // chunks per row
	// stepping: a step is running; fires lit meanwhile (a burst set off
	// by the fire) wait in pending until it ends.
	stepping bool
	pending  []fire.Fire
	// view caches the read model per ver.
	view    []FireView
	viewVer uint64
	viewOK  bool
	// strikes are the lightning strikes of the last hour (lightning.go).
	strikes []StrikeView
}

// fireIndex is the index in g.fire.fires of the fire on (x, y), or -1.
func (g *Game) fireIndex(x, y int) int {
	c := g.fire.rt.cell
	if c == nil || !g.grid.InBounds(x, y) {
		return -1
	}
	return int(c[y*g.grid.Width()+x]) - 1
}

// reindexFires sorts the fires y then x and rebuilds the cell and chunk
// lookups (dropping them with the last fire).
func (g *Game) reindexFires() {
	rt := &g.fire.rt
	if len(g.fire.fires) == 0 {
		rt.cell, rt.chunk = nil, nil
		return
	}
	slices.SortFunc(g.fire.fires, func(a, b fire.Fire) int {
		if a.Y != b.Y {
			return a.Y - b.Y
		}
		return a.X - b.X
	})
	w, h := g.grid.Width(), g.grid.Height()
	if len(rt.cell) != w*h {
		rt.cell = make([]int32, w*h)
	} else {
		clear(rt.cell)
	}
	rt.cw = (w + chunk.Size - 1) / chunk.Size
	if n := rt.cw * ((h + chunk.Size - 1) / chunk.Size); len(rt.chunk) != n {
		rt.chunk = make([]uint16, n)
	} else {
		clear(rt.chunk)
	}
	for i, f := range g.fire.fires {
		rt.cell[f.Y*w+f.X] = int32(i + 1)
		rt.chunk[(f.Y/chunk.Size)*rt.cw+f.X/chunk.Size]++
	}
}

// firesAround reports any fire in the chunks overlapping the box of
// Chebyshev radius r around (x, y): a cheap pre-check before a cell scan.
func (g *Game) firesAround(x, y, r int) bool {
	rt := &g.fire.rt
	if rt.chunk == nil {
		return false
	}
	w, h := g.grid.Width(), g.grid.Height()
	x0, y0 := max(0, x-r)/chunk.Size, max(0, y-r)/chunk.Size
	x1, y1 := min(w-1, x+r)/chunk.Size, min(h-1, y+r)/chunk.Size
	for cy := y0; cy <= y1; cy++ {
		for cx := x0; cx <= x1; cx++ {
			if rt.chunk[cy*rt.cw+cx] != 0 {
				return true
			}
		}
	}
	return false
}

// startFire lights (x, y) at size, or raises the fire already there to it.
// It reports false off the map.
func (g *Game) startFire(x, y, size int) bool {
	if !g.grid.InBounds(x, y) || size <= 0 {
		return false
	}
	size = min(size, fire.MaxSize)
	f := fire.Fire{X: x, Y: y, Size: uint8(size)}
	if g.fire.rt.stepping {
		g.fire.rt.pending = append(g.fire.rt.pending, f)
		return true
	}
	g.addFires([]fire.Fire{f})
	return true
}

// addFires merges new fires in (a cell already burning keeps the larger
// size) and reindexes.
func (g *Game) addFires(add []fire.Fire) {
	if len(add) == 0 {
		return
	}
	if g.fire.rt.cell == nil && len(g.fire.fires) > 0 {
		g.reindexFires()
	}
	w := g.grid.Width()
	var fresh map[int]int // cell -> index of a fire appended here
	for _, f := range add {
		i := g.fireIndex(f.X, f.Y)
		if j, ok := fresh[f.Y*w+f.X]; ok {
			i = j
		}
		if i >= 0 {
			g.fire.fires[i].Size = max(g.fire.fires[i].Size, f.Size)
			continue
		}
		if fresh == nil {
			fresh = map[int]int{}
		}
		fresh[f.Y*w+f.X] = len(g.fire.fires)
		g.fire.fires = append(g.fire.fires, f)
	}
	g.reindexFires()
	g.fire.ver++
	g.viewGen++
}

// ignite lights (x, y) at size when there is something there to burn.
func (g *Game) ignite(x, y, size int) bool {
	if !g.grid.InBounds(x, y) || !g.cellFuel(x, y, nil).Burns() {
		return false
	}
	return g.startFire(x, y, size)
}

// douseFire takes n off the fire on (x, y) (firefighting); it goes out at
// 0. It reports whether a fire was there.
func (g *Game) douseFire(x, y, n int) bool {
	i := g.fireIndex(x, y)
	if i < 0 {
		return false
	}
	f := &g.fire.fires[i]
	f.Size = uint8(max(0, int(f.Size)-n))
	if f.Size == 0 {
		g.fire.fires = slices.Delete(g.fire.fires, i, i+1)
		g.reindexFires()
	}
	g.fire.ver++
	g.viewGen++
	return true
}

// igniteBurst sets the cell of a bursting animal and its burnable
// neighbours alight (wildlife.go burst).
func (g *Game) igniteBurst(x, y int) {
	g.startFire(x, y, burstFireSize)
	for _, d := range neighbours8 {
		g.ignite(x+d[0], y+d[1], burstFireSize)
	}
}

var neighbours8 = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// fireItems indexes the loose flammable stacks for a step: fuel is the
// best item flammability per cell on or beside a fire, burning the
// stacks on burning cells (in item order).
func (g *Game) fireItems() (fuel map[int]int, burning map[int][]entity.EntityID) {
	w := g.grid.Width()
	fuel, burning = map[int]int{}, map[int][]entity.EntityID{} // non-nil: cellFuel reads, never scans
	for _, id := range g.itemIDs {
		st := g.stacks.Ref(id)
		if st == nil || st.Carrier != 0 {
			continue
		}
		fl := fire.ItemFlammability(st.Kind)
		if fl == 0 || !g.firesAround(st.X, st.Y, 1) {
			continue
		}
		near := false
		for dy := -1; dy <= 1 && !near; dy++ {
			for dx := -1; dx <= 1 && !near; dx++ {
				near = g.fireIndex(st.X+dx, st.Y+dy) >= 0
			}
		}
		if !near {
			continue
		}
		c := st.Y*w + st.X
		fuel[c] = max(fuel[c], fl)
		if g.fireIndex(st.X, st.Y) >= 0 {
			burning[c] = append(burning[c], id)
		}
	}
	return fuel, burning
}

// cellFuel is what (x, y) has left to burn: its plant, grass, flammable
// buildings and (items, from fireItems; nil: scan the item list) loose
// stacks.
func (g *Game) cellFuel(x, y int, items map[int]int) fire.Fuel {
	var f fire.Fuel
	if pid := g.plantAt(x, y); pid != 0 {
		if p := g.plants.Ref(pid); p != nil {
			f = f.With(fire.PlantFlammability(p.Species))
		}
	}
	if g.hasGrass(x, y) {
		f = f.With(fire.Grass)
	}
	for _, l := range fireLayers {
		if bid := g.layerBldgAt(x, y, l); bid != 0 {
			if b := g.bldgs.Ref(bid); b != nil {
				f = f.With(fire.BuildingFlammability(b.Kind, b.Stuff))
			}
		}
	}
	if items != nil {
		return f.With(items[y*g.grid.Width()+x])
	}
	for _, id := range g.looseAt(x, y) {
		if st := g.stacks.Ref(id); st != nil {
			f = f.With(fire.ItemFlammability(st.Kind))
		}
	}
	return f
}

// fireLayers are the layers whose buildings feed or take fire. Conduits
// (PLAY-021) are listed so a layer is never skipped, but they are steel
// (MetalOnly): fire.BuildingFlammability is 0 for them, so a conduit adds
// no fuel and never burns.
var fireLayers = [...]construction.Layer{construction.LayerObject, construction.LayerFloor, construction.LayerFoundation, construction.LayerConduit}

// fireMix is splitmix64's finaliser.
func fireMix(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ x>>30) * 0xBF58476D1CE4E5B9
	x = (x ^ x>>27) * 0x94D049BB133111EB
	return x ^ x>>31
}

// spreadRoll is the roll (0..999999) for the fire on cell src setting
// cell dst alight on tick now.
func spreadRoll(seed, now uint64, src, dst int) int {
	return int(fireMix(seed^fireMix(now^uint64(src)<<24^uint64(dst)<<44)) % 1_000_000)
}

// stepFires runs one fire step every fire.StepTicks while anything burns:
// each fire grows, starves or is drenched, big ones spread, and every
// fire damages what is on (and beside) its cell. Called every tick from
// the weather phase.
func (g *Game) stepFires() {
	now := uint64(g.clock.Now())
	if len(g.fire.fires) == 0 || now%fire.StepTicks != 0 {
		return
	}
	if g.fire.rt.cell == nil {
		g.reindexFires()
	}
	rt := &g.fire.rt
	rt.stepping = true
	w := g.grid.Width()
	items, burning := g.fireItems()
	seed := g.streams.seed ^ fireSalt
	s := g.weather
	wx, wy := s.WindSpeed*math.Cos(s.WindDir), s.WindSpeed*math.Sin(s.WindDir)
	falling := s.Kind.Falling()
	var spread []fire.Fire
	lit := map[int]bool{}
	for i := range g.fire.fires {
		f := &g.fire.fires[i]
		roofed := g.roofAt(f.X, f.Y).Roofed()
		size := fire.Next(int(f.Size), g.cellFuel(f.X, f.Y, items), fire.Doused(falling, s.Intensity, roofed))
		f.Size, f.Age = uint8(size), f.Age+1
		if size < fire.SpreadSize {
			continue
		}
		src := f.Y*w + f.X
		for _, d := range neighbours8 {
			nx, ny := f.X+d[0], f.Y+d[1]
			dst := ny*w + nx
			if !g.grid.InBounds(nx, ny) || g.fireIndex(nx, ny) >= 0 || lit[dst] {
				continue
			}
			shield := !roofed && g.roofAt(nx, ny).Roofed()
			c := fire.SpreadChance(size, g.cellFuel(nx, ny, items), fire.Downwind(d[0], d[1], wx, wy), shield)
			if c > 0 && spreadRoll(seed, now, src, dst) < c {
				lit[dst] = true
				spread = append(spread, fire.Fire{X: nx, Y: ny, Size: fire.IgniteSize})
			}
		}
	}
	// damage, in fire order (fires lit meanwhile, by a burst, wait in
	// pending until the step ends)
	for i := range g.fire.fires {
		if f := g.fire.fires[i]; f.Size > 0 {
			g.burnCell(f, burning[f.Y*w+f.X])
		}
	}
	g.burnCreatures()
	g.fire.fires = slices.DeleteFunc(g.fire.fires, func(f fire.Fire) bool { return f.Size == 0 })
	rt.stepping = false
	add := append(spread, rt.pending...)
	rt.pending = nil
	g.reindexFires()
	g.addFires(add)
	g.fire.ver++
	g.viewGen++
}

// burnCell damages what is on a burning cell: buildings lose hit points by
// the fire's size; the plant, grass and loose stacks burn away once the
// fire has burned long enough for each, leaving ash.
func (g *Game) burnCell(f fire.Fire, items []entity.EntityID) {
	dmg := fire.BuildingDamage(int(f.Size))
	for _, l := range fireLayers {
		bid := g.layerBldgAt(f.X, f.Y, l)
		if bid == 0 {
			continue
		}
		if b := g.bldgs.Ref(bid); b != nil && fire.BuildingFlammability(b.Kind, b.Stuff) > 0 {
			g.burnBuilding(bid, dmg)
		}
	}
	age := int(f.Age)
	if pid := g.plantAt(f.X, f.Y); pid != 0 {
		if p := g.plants.Ref(pid); p != nil && age >= fire.PlantBurnSteps(p.Species) {
			g.removePlant(pid)
			g.stamp(f.X, f.Y, stain.Ash, ashPlant)
		}
	}
	if age >= fire.GrassBurnSteps && g.hasGrass(f.X, f.Y) {
		g.clearGrass(f.X, f.Y)
		g.stamp(f.X, f.Y, stain.Ash, ashGrass)
	}
	if age >= fire.ItemBurnSteps && len(items) > 0 {
		gone := 0
		for _, id := range items {
			if st := g.stacks.Ref(id); st != nil && st.Carrier == 0 && st.X == f.X && st.Y == f.Y {
				if owner, taken := g.reserve.EntityOwner(id); taken {
					g.reserve.ReleaseEntity(owner, id) // its job fails: the target is gone
				}
				g.deleteItem(id)
				gone++
			}
		}
		if gone > 0 {
			g.stamp(f.X, f.Y, stain.Ash, ashItem)
		}
	}
}

// burnBuilding takes dmg hit points off building id; one that burns down
// is lost (nothing refunded, no rebuild blueprint), noted in the History
// when it was the colony's, and a burnt bridge drops what it held.
func (g *Game) burnBuilding(id entity.EntityID, dmg int) {
	b, ok := g.bldgs.Get(id)
	if !ok || dmg <= 0 {
		return
	}
	b.Damage += dmg
	if b.HP() > 0 {
		g.bldgs.Set(id, b)
		return
	}
	name := construction.DefOf(b.Kind).Name
	g.message(b.X, b.Y, "The %s burned down.", name)
	if !b.Ruin && !b.Ancient {
		g.addHistory("fire", "A "+name+" burned down.")
	}
	if construction.DefOf(b.Kind).Foundation {
		g.collapseBridge(b) // bridge_remove.go: what it held up goes with it
	}
	g.removeBuildingAs(id, false)
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		g.stamp(c[0], c[1], stain.Ash, ashBuilding)
	}
}

// burnCreatures hurts every creature standing in fire (and, a little,
// beside a big one): colonists and raiders take burns, animals injury.
func (g *Game) burnCreatures() {
	ids := make([]entity.EntityID, 0, g.creatureCount())
	ids = append(append(append(ids, g.pawnIDs...), g.raiderIDs...), g.animalIDs...)
	for _, id := range ids {
		p, ok := g.positions.Get(id)
		if !ok || !g.firesAround(p.X, p.Y, 1) {
			continue
		}
		dmg := 0
		if i := g.fireIndex(p.X, p.Y); i >= 0 {
			dmg = fire.CreatureDamage(int(g.fire.fires[i].Size))
		} else {
			for _, d := range neighbours8 {
				if i := g.fireIndex(p.X+d[0], p.Y+d[1]); i >= 0 {
					dmg = max(dmg, fire.AdjacentDamage(int(g.fire.fires[i].Size)))
				}
			}
		}
		if dmg == 0 {
			continue
		}
		if _, animal := g.animals.Get(id); animal {
			g.hurt(id, dmg, 0)
			continue
		}
		if v, ok := g.vital.Get(id); !ok || v.Status == health.Dead {
			continue
		}
		g.hurtAs(id, dmg, 0, body.Burn, health.CauseBurns) // combat.go
		if v, _ := g.vital.Get(id); v.Status == health.Dead && v.Cause == health.CauseBurns {
			g.message(p.X, p.Y, "%s burned to death.", g.nameOf(id))
		}
	}
}

// fireRoomHeat adds each indoor fire's heat to heat (per room, like
// heatRooms' campfires): a fire of fireHeatSize heats like one campfire.
func (g *Game) fireRoomHeat(heat []int) {
	for _, f := range g.fire.fires {
		if r := g.roomAt(f.X, f.Y); r >= 0 && r < len(heat) {
			heat[r] += campfireHeat * int(f.Size) / fireHeatSize
		}
	}
}

// fireWarmth is the outdoor warming on (x, y) from the biggest fire
// within fireWarmReach (climate.go temperatureAt).
func (g *Game) fireWarmth(x, y int) temperature.Temp {
	if !g.firesAround(x, y, fireWarmReach) {
		return 0
	}
	best := 0
	for dy := -fireWarmReach; dy <= fireWarmReach; dy++ {
		for dx := -fireWarmReach; dx <= fireWarmReach; dx++ {
			if i := g.fireIndex(x+dx, y+dy); i >= 0 {
				best = max(best, int(g.fire.fires[i].Size))
			}
		}
	}
	return temperature.Temp(best * fireOutdoorHeat)
}

// devStartFire lights (x, y) (the developer action, command.DevStartFire).
func (g *Game) devStartFire(x, y int) { g.startFire(x, y, devFireSize) }

// fillFireView sets WorldView.Fires, FireVer and Strikes.
func (g *Game) fillFireView(w *WorldView) {
	rt := &g.fire.rt
	if !rt.viewOK || rt.viewVer != g.fire.ver {
		rt.view = nil
		if len(g.fire.fires) > 0 {
			rt.view = make([]FireView, len(g.fire.fires))
			for i, f := range g.fire.fires {
				rt.view[i] = FireView{X: f.X, Y: f.Y, Size: int(f.Size)}
			}
		}
		rt.viewVer, rt.viewOK = g.fire.ver, true
	}
	w.Fires, w.FireVer = rt.view, g.fire.ver
	w.Strikes = slices.Clone(rt.strikes)
}

// exportFires is the Fires save field (nil: nothing burns).
func (g *Game) exportFires() []fire.Fire {
	if len(g.fire.fires) == 0 {
		return nil
	}
	return slices.Clone(g.fire.fires)
}

// importFires restores the fires (older saves have none).
func (g *Game) importFires(fs []fire.Fire) {
	g.fire.fires = nil
	for _, f := range fs {
		if f.Size > 0 && g.grid.InBounds(f.X, f.Y) {
			g.fire.fires = append(g.fire.fires, f)
		}
	}
	g.reindexFires()
	g.fire.rt.viewOK = false // ver stays: a loaded game reads like the saved one
}
