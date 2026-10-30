package game

import (
	"fmt"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// stuffByName parses a building material name; "stone" is the legacy
// alias for granite blocks (SIM-029).
func stuffByName(name string) (item.Kind, bool) {
	if name == "" || name == "wood" {
		return item.Wood, true
	}
	if name == "stone" {
		return item.GraniteBlocks, true
	}
	k, ok := item.ParseKind(name)
	return k, ok && k.IsStuff()
}

// SetGround sets the ground of every tile in x0..x1, y0..y1 (inclusive)
// to terrain ground with the given stone and ore ("" for none). Plants on
// rock are cleared. It reports false on an unknown name.
func (g *Game) SetGround(x0, y0, x1, y1 int, ground, stone, ore string) bool {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	t, ok := terrainByName(ground)
	if !ok {
		return false
	}
	s, o := geology.None, geology.NoOre
	if stone != "" {
		if s, ok = geology.ParseStone(stone); !ok {
			return false
		}
	}
	if ore != "" {
		found := false
		for _, c := range minedOres() {
			if c.String() == ore {
				o, found = c, true
			}
		}
		if !found {
			return false
		}
	}
	if g.geo.W != g.terr.W || g.geo.H != g.terr.H {
		g.geo = geology.NewMap(g.terr.W, g.terr.H)
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !g.grid.InBounds(x, y) {
				continue
			}
			if terrain.IsRockWall(t) {
				if id := g.plantAt(x, y); id != 0 {
					g.removePlant(id)
				}
			}
			g.setTerrain(x, y, t)
			g.geo.Set(x, y, s)
			g.geo.SetOre(x, y, o)
		}
	}
	g.structVer++
	return true
}

func terrainByName(name string) (terrain.Type, bool) {
	for i, n := range terrain.Names() {
		if n == name {
			return terrain.Type(i), true
		}
	}
	return 0, false
}

// Dev hooks for set-piece scenarios (cmd/game -scenario). They change the
// colony the same way gameplay would, only at once.

// Arm puts weapon k in colonist id's hands (k must be a weapon), Normal.
func (g *Game) Arm(id entity.EntityID, k item.Kind) { _ = g.ArmQuality(id, k, quality.Normal) }

// ArmQuality is Arm with a weapon of quality q (CBT-3).
func (g *Game) ArmQuality(id entity.EntityID, k item.Kind, q quality.Level) error {
	if g.viewGen++; !k.IsWeapon() { // view cache (indexes.go): this hook changes the state
		return fmt.Errorf("%s is not a weapon", k)
	}
	g.equip[id] = k
	g.combat.Gear.Hold(id, item.Stack{Kind: k, Qty: 1, Quality: quality.Stored(q)})
	return nil
}

// RaidArmed sends an assault raid carrying exactly these weapons.
func (g *Game) RaidArmed(kit ...item.Kind) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.raidWith(kit, raid.Assault)
}

// RaidDropPods sends a drop-pod raid with that kit onto the colony (DEF-4).
func (g *Game) RaidDropPods(kit ...item.Kind) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if len(kit) > 0 {
		podRaid(g, raid.Group{Size: len(kit), Plan: raid.Assault, Arrival: raiddef.DropPodCentre, FleeLoss: raiddef.FleeLossLegacy}, kit, nil)
	}
}

// RaidMechs sends a mechanoid group worth points walking in from an edge
// (CBT-4: mechanoids bash through shut doors).
func (g *Game) RaidMechs(points int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.raid.Mechs.Raid(points, raiddef.WalkInEdge, uint64(g.clock.Now()))
}

// Tantrum throws colonist id into a tantrum now (CBT-4).
func (g *Game) Tantrum(id entity.EntityID) { g.ForceBreak(id, mentalbreak.Tantrum) }

// SpawnCrop places a plant of species at (x, y) grown to pct percent.
func (g *Game) SpawnCrop(species string, x, y, pct int) (entity.EntityID, error) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	s, ok := plant.Parse(species)
	if !ok {
		return 0, errSpawn
	}
	return g.spawnPlant(s, x, y, plant.DefOf(s).GrowTicks*pct/100)
}
