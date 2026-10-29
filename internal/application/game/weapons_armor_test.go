package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// DEF-2 whole-game tests: the weapon and armor catalogue, explosions and
// the raid kit (the unit tests are in combat/blast_test.go and the domain
// packages).

// TestBurstWeaponFiresSeveralShots: a colonist with an assault rifle fires
// a burst of three shots ten ticks apart, so several are in flight at once.
func TestBurstWeaponFiresSeveralShots(t *testing.T) {
	g, att, _ := attackSetup(t, item.AssaultRifle, 20)
	var fired []uint64
	peak := 0
	for i := 0; i < 400 && len(fired) < 3; i++ {
		g.Step(1)
		w := g.World()
		peak = max(peak, len(w.Shots))
		for _, tr := range w.Traces {
			if tr.Attacker == att && tr.Stage == projectile.Fired && tr.Weapon == "assault rifle" && (len(fired) == 0 || uint64(g.clock.Now())-uint64(tr.Age) > fired[len(fired)-1]) {
				fired = append(fired, uint64(g.clock.Now())-uint64(tr.Age))
			}
		}
	}
	if len(fired) != 3 || fired[1]-fired[0] != 10 || fired[2]-fired[1] != 10 {
		t.Fatalf("burst shots at %v", fired)
	}
	if peak < 2 {
		t.Fatalf("at most %d shot in flight", peak)
	}
}

// hurtPct is the colonist's health, percent.
func hurtPct(g *Game, id entity.EntityID) int { return pawnView(g, id).Health }

// blastSeen steps g until a burst of kind blast is on the world view.
func blastSeen(t *testing.T, g *Game, blast string) BurstView {
	t.Helper()
	for i := 0; i < 600; i++ {
		g.Step(1)
		for _, b := range g.World().Bursts {
			if b.Blast == blast {
				return b
			}
		}
	}
	t.Fatalf("no %s blast", blast)
	return BurstView{}
}

// TestGrenadeExplosionDamagesWithinRadius: a thrown frag grenade bursts
// on its target and wounds the colonist beside it, not one further off.
func TestGrenadeExplosionDamagesWithinRadius(t *testing.T) {
	g, _, tgt := attackSetup(t, item.FragGrenades, 6)
	beside := sleeper(g, 12, 5, "Cy")
	far := sleeper(g, 15, 5, "Di")
	b := blastSeen(t, g, "bomb")
	if b.X != 11 || b.Y != 5 || b.Radius != 19 || b.Turret || b.Species != "" {
		t.Fatalf("burst %+v", b)
	}
	if hurtPct(g, tgt) >= 100 || hurtPct(g, beside) >= 100 {
		t.Fatalf("target %d%%, beside %d%%", hurtPct(g, tgt), hurtPct(g, beside))
	}
	if hurtPct(g, far) != 100 {
		t.Fatalf("four tiles off lost health: %d%%", hurtPct(g, far))
	}
}

// TestMolotovIgnites: a molotov sets its landing cell alight.
func TestMolotovIgnites(t *testing.T) {
	g, _, _ := attackSetup(t, item.MolotovCocktails, 5)
	b := blastSeen(t, g, "flame")
	if g.fireIndex(b.X, b.Y) < 0 {
		t.Fatalf("no fire on (%d, %d)", b.X, b.Y)
	}
}

// TestPlateArmorDeflectsKnife: knife blows on a colonist in steel plate
// armor deal far less than on one in everyday clothes.
func TestPlateArmorDeflectsKnife(t *testing.T) {
	dealt := func(plate bool) int {
		g := New(Config{Seed: 6, Width: 30, Height: 12})
		att, _ := g.SpawnNamedPawn(5, 5, "Ada")
		total := 0
		for n := 0; n < 40; n++ {
			tgt := sleeper(g, 6, 5, "Bo")
			if plate {
				g.apparel[tgt] = worn{On: [craft.Slots]craft.Piece{item.Body: {Kind: item.PlateArmor}}}
			}
			for i := 0; i < 6; i++ {
				before := len(g.traces)
				g.combat.Strikes.Strike(att, tgt, weapon.Knife)
				if len(g.traces) > before && g.traces[len(g.traces)-1].Hit {
					total += g.traces[len(g.traces)-1].Damage
				}
			}
			g.positions.Set(tgt, Position{X: 29, Y: 11})
			g.invalidateCreatures()
		}
		return total
	}
	bare, plated := dealt(false), dealt(true)
	if bare == 0 || plated*10 >= bare*7 {
		t.Fatalf("plate armor let %d through against %d bare", plated, bare)
	}
}

// TestGearReadModel: the pawn view carries the wielded weapon's numbers
// and each worn garment's armor by its stuff and quality.
func TestGearReadModel(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.Arm(id, item.AssaultRifle)
	g.apparel[id] = worn{On: [craft.Slots]craft.Piece{
		item.Body: {Kind: item.FlakVest, Quality: quality.Stored(quality.Good)},
		item.Head: {Kind: item.SimpleHelmet},
		item.Legs: {Kind: item.FlakPants},
	}}
	p := pawnView(g, id)
	if p.WeaponStats != weapon.Stats(weapon.AssaultRifle) || p.WeaponStats.Burst != 3 {
		t.Fatalf("weapon stats %+v", p.WeaponStats)
	}
	if !reflect.DeepEqual(p.Apparel, []string{"flak vest", "simple helmet", "flak pants"}) || len(p.ApparelArmor) != 3 {
		t.Fatalf("apparel %v armor %v", p.Apparel, p.ApparelArmor)
	}
	if p.ApparelArmor[0].Sharp != 1150 || p.ApparelArmor[1].Sharp != 450 || p.ApparelArmor[2].Sharp != 550 {
		t.Fatalf("armor %+v", p.ApparelArmor)
	}
}

// TestRaidKitByTech: the storyteller arms tribes with bows and blades and
// industrial raiders with the firearms and grenades of the catalogue.
func TestRaidKitByTech(t *testing.T) {
	seen := map[faction.Tech]map[item.Kind]bool{faction.Neolithic: {}, faction.Industrial: {}}
	for seed := uint64(1); seed <= 20; seed++ {
		g := New(Config{Seed: seed, Width: 10, Height: 8})
		for tech := range seen {
			for _, k := range g.raidKitOf(1<<20, tech) {
				seen[tech][k] = true
			}
		}
	}
	if !seen[faction.Neolithic][item.ShortBow] || seen[faction.Neolithic][item.Rifle] || seen[faction.Neolithic][item.AssaultRifle] {
		t.Fatalf("tribal kits %v", seen[faction.Neolithic])
	}
	if !seen[faction.Industrial][item.AssaultRifle] || !seen[faction.Industrial][item.FragGrenades] {
		t.Fatalf("industrial kits %v", seen[faction.Industrial])
	}
}

// TestSmokeAndStunSurviveASave: smoke clouds and an EMP-stunned turret's
// stun are in the save.
func TestSmokeAndStunSurviveASave(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 10})
	_, _ = g.SpawnNamedPawn(2, 2, "Ada")
	g.combat.Blasts.Explode(0, weapon.Fists, 8, 5, explosion.Def{Kind: explosion.Smoke, Radius: 2400})
	g.combat.Turrets.Stun(77, uint64(g.clock.Now())+500)
	var buf bytes.Buffer
	if err := g.Save(&buf, "smoke"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.combat.Blasts.Export(), g.combat.Blasts.Export()) || len(h.combat.Blasts.Export()) != 1 {
		t.Fatalf("smoke %v, want %v", h.combat.Blasts.Export(), g.combat.Blasts.Export())
	}
	if h.combat.Turrets.Aim(77).StunnedUntil != g.combat.Turrets.Aim(77).StunnedUntil {
		t.Fatal("the stun is lost")
	}
}

// TestNoHuntingWithExplosives: a colonist holding frag grenades takes no
// hunt job (it goes for a gun if one lies free), and a hunt order is
// refused for want of a ranged weapon.
func TestNoHuntingWithExplosives(t *testing.T) {
	g := New(Config{Seed: 65, Width: 40, Height: 16})
	bareGround(g)
	p, _ := g.SpawnNamedPawn(2, 8, "Hunter")
	g.equip[p] = item.FragGrenades
	buck, _ := g.SpawnAnimal("dustbuck", 8, 8)
	setAnimal(g, buck, func(a *animal.State) { a.Designated = true })
	g.combat.Hunting.Assign(p)
	if j, _ := g.jobs.Get(p); j.Kind == job.Hunt {
		t.Fatal("a grenadier went hunting")
	}
	if _, why := g.forcedJob(p, command.OrderHunt, 8, 8, construction.LayerObject); why != ReasonNoRanged {
		t.Fatalf("hunt order: %q, want %q", why, ReasonNoRanged)
	}
	g.equip[p] = item.Rifle
	if !g.combat.Hunting.Assign(p) {
		t.Fatal("with a rifle it hunts")
	}
}
