package game

import (
	"testing"

	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/plant"
)

// TestShellsCookOffInGameFire (CBT-5): a stack of mortar shells on a
// burning cell goes off once its fuse runs out: the stack leaves the
// store and the ground index, an HE burst is stamped for the renderer,
// and a rifle beside it is worn by the blast.
func TestShellsCookOffInGameFire(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 20, 20)
	shells, err := g.SpawnStack(item.MortarShells, 5, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	rifle, err := g.SpawnStack(item.Rifle, 1, 11, 10)
	if err != nil {
		t.Fatal(err)
	}
	g.startFire(10, 10, 30)
	fireStep(g)
	if !g.combat.Traps.Burning(shells) {
		t.Fatal("the fire did not set the shells off")
	}
	for i := 0; i < explosion.CookOffFuse+1; i++ {
		g.combat.Turrets.Phase(uint64(g.clock.Now()))
		g.clock.Advance(1)
	}
	if _, ok := g.stacks.Get(shells); ok || g.itemAt(10, 10) != 0 {
		t.Fatal("the shells are still there")
	}
	bombs := 0
	for _, b := range g.wl.Bursts {
		if b.Blast == explosion.Bomb && b.X == 10 && b.Y == 10 {
			bombs++
		}
	}
	if bombs != 1 {
		t.Fatalf("%d bomb bursts on the stack, want 1", bombs)
	}
	if st, _ := g.stacks.Get(rifle); st.Wear == 0 {
		t.Fatal("the rifle beside it took nothing")
	}
}

// TestFragTearsPlantAndSmokeIsViewed (CBT-5): a frag grenade kills a
// crop it lands on (bomb x4 against plants) and a smoke shell's cloud is
// on the world view until it thins.
func TestFragTearsPlantAndSmokeIsViewed(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 20, 20)
	g.ClearPlants(0, 0, 19, 19)
	if _, err := g.spawnPlant(plant.Potato, 5, 5, 50); err != nil {
		t.Fatal(err)
	}
	g.combat.Blasts.Explode(0, weapon.FragGrenade, 5, 5, weapon.DefOf(weapon.FragGrenade).Blast)
	if g.plantAt(5, 5) != 0 {
		t.Fatal("the crop survived a grenade at its feet")
	}
	g.combat.Blasts.Explode(0, weapon.MortarSmoke, 10, 10, weapon.DefOf(weapon.MortarSmoke).Blast)
	if s := g.World().Smoke; len(s) != 1 || s[0].X != 10 || s[0].Until != uint64(g.clock.Now())+explosion.SmokeTicks {
		t.Fatalf("smoke view %+v", s)
	}
}
