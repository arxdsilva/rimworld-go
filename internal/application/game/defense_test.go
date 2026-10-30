package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/cover"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

func TestDoorsAreShutToRaidersAndAnimals(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	hut(t, g, 2, 2, 8, 8)
	hm := g.hostileMap()
	if !g.grid.Walkable(5, 8) || hm.Walkable(5, 8) {
		t.Fatal("the door is walkable for colonists but shut to hostiles")
	}
	if _, ok := g.raidPathTo(t, 5, 5); ok {
		t.Fatal("no hostile path into a closed hut")
	}
}

// raidPathTo plans a hostile path from the map corner.
func (g *Game) raidPathTo(t *testing.T, x, y int) ([]Position, bool) {
	t.Helper()
	g.Raid(1)
	rid := g.raiderIDs[len(g.raiderIDs)-1]
	g.positions.Set(rid, Position{X: 15, Y: 10})
	p, ok := g.pathTo(rid, x, y)
	out := make([]Position, len(p))
	for i, q := range p {
		out[i] = Position{X: q.X, Y: q.Y}
	}
	g.removeRaider(rid)
	return out, ok
}

func TestRaiderBreaksIntoAClosedHut(t *testing.T) {
	g := New(Config{Seed: 5, Width: 24, Height: 14})
	hut(t, g, 2, 2, 8, 8)
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.draft[ada] = true // stay put inside
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.equip[rid] = item.Club
	g.positions.Set(rid, Position{X: 20, Y: 5})
	broke := false
	for i := 0; i < 200 && !broke; i++ {
		g.Step(50)
		g.SetNeeds(ada, needs.Full, needs.Full)
		for _, m := range g.World().Messages {
			if strings.Contains(m.Text, "smashed down") {
				broke = true
			}
		}
	}
	if !broke {
		j, _ := g.jobs.Get(rid)
		t.Fatalf("the raider should smash its way in (job %v %v)", j.Kind, j.State)
	}
}

func TestSandbagsGiveCover(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	if !g.BuildNow("sandbags", "wood", "south", 9, 5) {
		t.Fatal("build sandbags")
	}
	behind, shooter := Position{X: 10, Y: 5}, Position{X: 2, Y: 5}
	if c := g.combat.Sight.Cover(shooter.X, shooter.Y, behind.X, behind.Y); c != cover.Permille(cover.Sandbags) { // DEF-3: permille, the cover domain's
		t.Fatalf("cover %d", c)
	}
	if c := g.combat.Sight.Cover(18, 5, behind.X, behind.Y); c != 0 {
		t.Fatalf("no cover from the open side, got %d", c)
	}
	if !g.grid.Walkable(9, 5) {
		t.Fatal("sandbags can be stepped over")
	}
}

func TestSpikeTrapSpringsOnRaiderAndIsRelaid(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 10})
	if !g.BuildNow("spike trap", "wood", "south", 10, 5) {
		t.Fatal("build trap")
	}
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.positions.Set(rid, Position{X: 10, Y: 5})
	for i := 0; i < 20 && g.combat.Traps.At(10, 5) != 0; i++ {
		g.combat.Traps.Spring(rid, 10, 5)
	}
	if g.combat.Traps.At(10, 5) != 0 {
		t.Fatal("the trap should have sprung")
	}
	if v, _ := g.vital.Get(rid); v.Injury == 0 && v.Status == health.Alive { // CBT-3: five wooden spikes of about 8, through armor
		t.Fatalf("injury %d", v.Injury)
	}
	if g.layerFrameAt(10, 5, construction.LayerObject) == 0 {
		t.Fatal("a sprung trap is re-laid as a blueprint")
	}
}

func TestBuildersRepairDamage(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 10})
	if !g.BuildNow("wall", "wood", "south", 10, 5) {
		t.Fatal("build wall")
	}
	var wall entity.EntityID
	for _, id := range g.bldgIDs {
		wall = id
	}
	g.damageBuilding(wall, 50)
	if b, _ := g.bldgs.Get(wall); b.Damage != 50 {
		t.Fatal("damage recorded")
	}
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	for i := 0; i < 40; i++ {
		g.Step(100)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	if b, _ := g.bldgs.Get(wall); b.Damage != 0 {
		t.Fatalf("wall should be repaired, damage %d", b.Damage)
	}
	g.damageBuilding(wall, construction.MaxHP(construction.Wall, item.Wood))
	if _, ok := g.bldgs.Get(wall); ok || !g.grid.Walkable(10, 5) {
		t.Fatal("a wall at zero hp is destroyed and the tile opens up")
	}
	if g.layerFrameAt(10, 5, construction.LayerObject) == 0 {
		t.Fatal("a smashed building leaves a blueprint to rebuild it")
	}
}
