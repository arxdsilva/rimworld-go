package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/roof"
)

// Applying a generated map (MAPGEN-2 §7): the generator's Result laid on a
// new Setup colony — terrain, geology, natural roofs (thin and thick),
// ruins, steam vents, stone debris, plants, the grass layer and the landing
// spot. Owned by INTEGRATE. Everything here runs once at build time, in
// the Result's order (no RNG of its own), so the colony is a pure function
// of the Setup.

// ReasonNatural is the refusal for a natural map feature (a steam vent):
// it can't be built over, deconstructed, moved or swapped.
const ReasonNatural = "natural feature"

// applySite lays res on the map. A Result of another size (a failed
// generator) leaves the plain map, with the site's stones.
func (g *Game) applySite(res mapgen.Result) {
	w, h := g.grid.Width(), g.grid.Height()
	sized := res.W == w && res.H == h && len(res.Terrain.Cells) == w*h
	if sized {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				g.setTerrain(x, y, res.Terrain.At(x, y))
			}
		}
	}
	if gm := res.Geology; sized && gm.W == w && gm.H == h && len(gm.Stone) == w*h {
		g.geo = geology.Map{W: w, H: h, Stone: slices.Clone(gm.Stone), Ore: slices.Clone(gm.Ore)}
		if len(g.geo.Ore) != w*h {
			g.geo.Ore = make([]geology.Ore, w*h)
		}
	} else if s := g.ng.setup; s != nil {
		g.geo = geology.GenerateWith(s.Seed, g.terr, s.Site.Stones)
	}
	g.structVer++
	if !sized {
		return
	}
	g.applyNaturalRoofs(res.Roof)
	if g.grid.InBounds(res.Start.X, res.Start.Y) {
		g.ng.landing, g.ng.hasLanding = [2]int{res.Start.X, res.Start.Y}, true
	}
	g.applyBridges(res) // bridge_remove.go (MAPGEN-3): the sea mask and old road bridges
	for _, r := range res.Ruins {
		g.placeRuin(r)
	}
	g.placeVaults(res.Vaults) // vault.go (MAPGEN-3): sealed vaults and their sleepers
	vs := g.ventStuff()
	for _, v := range res.Vents {
		if !g.ventSiteFree(v.X, v.Y) {
			continue // vents.go: a ruin or vault piece stands there
		}
		g.placeGenerated(construction.Building{Kind: construction.SteamVent, Stuff: vs, X: v.X, Y: v.Y})
	}
	for _, c := range res.Chunks {
		g.dropChunk(c)
	}
	for _, p := range res.Plants {
		if g.layerBlockerAt(p.X, p.Y) != 0 || g.naturalAt(p.X, p.Y) || g.bridged(p.X, p.Y) {
			continue
		}
		_, _ = g.spawnPlant(p.Species, p.X, p.Y, p.Growth)
	}
	g.seedGrassFrom(res.Grass)
}

// applyNaturalRoofs merges the generator's natural roofs (thin and thick)
// into the roof grid; a thick roof is always natural too.
func (g *Game) applyNaturalRoofs(cells []roof.Cell) {
	if len(cells) != g.grid.Width()*g.grid.Height() {
		return
	}
	g.ensureRoofGrid()
	for i, c := range cells {
		f := c & (roof.Natural | roof.NaturalThick)
		if f == 0 {
			continue
		}
		if f.Has(roof.NaturalThick) {
			f |= roof.Natural
		}
		g.setRoof(i, g.roofs[i]&^(roof.Built|roofPlans)|f)
	}
}

// placeRuin lays one ruin: its old floors, its standing (damaged) walls,
// the loot left inside and the rubble around it.
func (g *Game) placeRuin(r mapgen.Ruin) {
	for _, f := range r.Floors {
		g.placeGenerated(construction.Building{Kind: construction.Floor, Stuff: f.Stuff, X: f.X, Y: f.Y, Ruin: true})
	}
	for _, wp := range r.Walls {
		dmg := construction.MaxHP(construction.Wall, wp.Stuff) * min(max(wp.DamagePct, 0), 99) / 100
		g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: wp.Stuff, X: wp.X, Y: wp.Y, Ruin: true, Damage: dmg})
	}
	for _, l := range r.Loot {
		if l.Qty > 0 && g.dryWalkable(l.X, l.Y) {
			g.spawnUnclaimed(l.Kind, l.Qty, l.X, l.Y, false) // haul_claim.go: found, not the colony's
		}
	}
	for _, c := range r.Rubble {
		g.dropChunk(c)
	}
}

// dropChunk lays one stone chunk, found on the map (haul_claim.go), on
// open dry ground.
func (g *Game) dropChunk(c mapgen.ChunkSpawn) {
	if c.Stone == geology.None || !g.dryWalkable(c.X, c.Y) {
		return
	}
	g.spawnUnclaimed(item.ChunkOf(c.Stone), 1, c.X, c.Y, false)
}

// placeGenerated stands a finished map-feature building (no frame, no
// Home, no bills): skipped when its footprint leaves the map, meets rock
// or water, or its layer is already taken. A foundation (a bridge) goes
// over bridgeable ground instead (MAPGEN-3).
func (g *Game) placeGenerated(b construction.Building) entity.EntityID {
	def := construction.DefOf(b.Kind)
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	if len(cells) == 0 {
		return 0
	}
	for _, c := range cells {
		if !g.grid.InBounds(c[0], c[1]) || g.layerBldgAt(c[0], c[1], def.Layer) != 0 {
			return 0
		}
		if def.Foundation { // bridge.go: over water, marsh or mud, never the sea
			if ok, _ := g.bridgePlacement(c[0], c[1]); !ok {
				return 0
			}
			continue
		}
		if !g.terr.At(c[0], c[1]).Walkable() {
			return 0
		}
		if ok, _ := g.waterPlacement(c[0], c[1]); !ok {
			return 0
		}
	}
	id := g.entities.Next()
	g.bldgIDs = append(g.bldgIDs, id)
	g.bldgs.Set(id, b)
	g.noteBuilding(id) // indexes.go
	for _, c := range cells {
		if pid := g.plantAt(c[0], c[1]); pid != 0 && def.Layer != construction.LayerFloor {
			g.removePlant(pid)
		}
		if def.BlocksMove || def.Foundation {
			g.grid.SetWalkable(c[0], c[1], g.cellWalkable(c[0], c[1])) // bridge.go
		}
	}
	g.markRoomsDirty()
	g.structVer++
	return id
}

// seedGrassFrom seeds a Setup colony's grass layer from the generator's
// mask (soft ground only), bare under buildings, frames and plants; a
// mask of the wrong size falls back to the terrain rule (seedGrass).
func (g *Game) seedGrassFrom(mask []bool) {
	w, h := g.grid.Width(), g.grid.Height()
	if len(mask) != w*h {
		g.seedGrass()
		return
	}
	s := g.grassSt()
	s.seeded = true
	s.ver++
	s.cells = make([]uint8, w*h)
	s.cut = map[[2]int]bool{}
	for i, on := range mask {
		if on && g.terr.At(i%w, i/w).Fertility() > 0 {
			s.cells[i] = 1
		}
	}
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok {
			g.bareFootprint(s, b.Kind, b.Facing, b.X, b.Y)
		}
	}
	for _, id := range g.frameIDs {
		if f, ok := g.frames.Get(id); ok {
			g.bareFootprint(s, f.Kind, f.Facing, f.X, f.Y)
		}
	}
	for _, id := range g.plantIDs {
		if p, ok := g.plants.Get(id); ok && g.grid.InBounds(p.X, p.Y) {
			s.cells[p.Y*w+p.X] = 0
		}
	}
}

// --- natural features and ruins -------------------------------------------

// ventStuff is a steam vent's material (its look): the site's first
// stone, granite without one. Never the legacy item.Stone, which a load
// migrates to granite.
func (g *Game) ventStuff() item.Kind {
	if s := g.ng.setup; s != nil {
		for _, st := range s.Site.Stones {
			if k := item.BlocksOf(st); item.IsStoneBlocks(k) {
				return k
			}
		}
	}
	return item.GraniteBlocks
}

// isNatural reports a natural map feature (a steam vent).
func isNatural(k construction.Kind) bool { return construction.DefOf(k).Natural }

// naturalAt reports a natural feature covering (x, y). Natural features
// (steam vents) lie in the foundation layer (PLAY-021), under whatever is
// built over them.
func (g *Game) naturalAt(x, y int) bool {
	if !g.grid.InBounds(x, y) {
		return false
	}
	b, ok := g.bldgs.Get(g.layerBldgAt(x, y, construction.LayerFoundation))
	return ok && isNatural(b.Kind)
}

// refundOf is what deconstructing b gives back: a ruin gives half.
func refundOf(b construction.Building) int {
	r := construction.Refund(b.Kind)
	if b.Ruin {
		r = (r + 1) / 2 // half, rounded up: a ruin still gives something back
	}
	return r
}

// notColonyPiece reports a building that is not the colony's: a ruin, an
// ancient vault piece (MAPGEN-3) or a natural feature (no wealth, no Home
// anchor, no repairs).
func notColonyPiece(b construction.Building) bool { return b.Ruin || b.Ancient || isNatural(b.Kind) }

// refuseNatural posts why a natural feature can't be deconstructed.
func (g *Game) refuseNatural(b construction.Building) {
	g.message(b.X, b.Y, "The %s is a natural feature: it can't be deconstructed.", construction.DefOf(b.Kind).Name)
}

// --- landing spot (saves) ---------------------------------------------------

// exportLanding is the landing spot for a save (nil: the map centre).
func (g *Game) exportLanding() []int {
	if !g.ng.hasLanding {
		return nil
	}
	return []int{g.ng.landing[0], g.ng.landing[1]}
}

// importLanding restores the landing spot; an older save (nil) keeps the
// map centre. It runs after importSetupState, which resets g.ng.
func (g *Game) importLanding(l []int) {
	if len(l) != 2 || !g.grid.InBounds(l[0], l[1]) {
		g.ng.landing, g.ng.hasLanding = [2]int{}, false
		return
	}
	g.ng.landing, g.ng.hasLanding = [2]int{l[0], l[1]}, true
}
