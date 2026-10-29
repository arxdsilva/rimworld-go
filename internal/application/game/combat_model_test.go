package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// DEF-1 whole-game tests: shots in flight, hit chance, armor, friendly
// fire and saves (the unit tests are in combat/shot_test.go).

// settleShots flies every shot in flight to its end.
func settleShots(t *testing.T, g *Game) {
	t.Helper()
	for i := 0; g.combat.Shots.Len() > 0; i++ {
		if i > 200 {
			t.Fatal("a shot never landed")
		}
		g.combat.Shots.Step()
	}
}

// sleeper is a colonist asleep on (x, y): it stays put.
func sleeper(g *Game, x, y int, name string) entity.EntityID {
	id, _ := g.SpawnNamedPawn(x, y, name)
	g.jobs.Set(id, job.Record{Kind: job.Sleep, State: job.Do, WorkLeft: 1 << 20, TX: x, TY: y})
	return id
}

func TestRifleShotFliesInTheGame(t *testing.T) {
	g, att, tgt := attackSetup(t, item.Rifle, 12)
	var fired TraceView
	for i := 0; i < 400 && fired.Stage != projectile.Fired; i++ {
		g.Step(1)
		for _, tr := range g.World().Traces {
			if tr.Attacker == att && tr.Stage == projectile.Fired {
				fired = tr
			}
		}
	}
	if fired.Stage != projectile.Fired || fired.FromX != 5 || fired.Weapon != "rifle" || fired.Damage != 0 {
		t.Fatalf("no shot fired: %+v", fired)
	}
	w := g.World()
	if len(w.Shots) != 1 || w.Shots[0].Owner != att || w.Shots[0].Target != tgt || w.Shots[0].FromX != 5 ||
		w.Shots[0].X <= 5000 || w.Shots[0].VX != weapon.DefOf(weapon.Rifle).Speed {
		t.Fatalf("the shot is in flight on the world view: %+v", w.Shots)
	}
	var land TraceView
	for i := 0; i < 30 && land.Stage != projectile.Landed; i++ {
		g.Step(1)
		for _, tr := range g.World().Traces {
			if tr.Attacker == att && tr.Stage == projectile.Landed {
				land = tr
			}
		}
	}
	if land.Stage != projectile.Landed || land.FromX != 5 || len(g.World().Shots) != 0 {
		t.Fatalf("the shot landed: %+v", land)
	}
	if land.Hit && land.Target == tgt && pawnView(g, tgt).HitDamage != weapon.DefOf(weapon.Rifle).Damage {
		t.Fatal("a hit stamps the victim on landing")
	}
}

func TestHitChanceQuery(t *testing.T) {
	g := New(Config{Seed: 2, Width: 40, Height: 10})
	att, _ := g.SpawnNamedPawn(2, 5, "Ada")
	g.Arm(att, item.Rifle)
	lv := int(g.skillOf(att, skill.Shooting).Level)
	for _, dist := range []int{3, 12, 25} {
		tgt := sleeper(g, 2+dist, 5, "Bo")
		r, ok := g.HitChance(att, tgt)
		band := weapon.DefOf(weapon.Rifle).Bands.Accuracy(dist)
		if !ok || r.Band != band || r.Shooter != attack.ShooterFactor(attack.ShooterPerTile(lv), dist) ||
			r.Size != 1000 || r.Light != attack.NoLightPenalty || r.Weather != attack.ClearFactor ||
			r.Chance != r.Total() || r.Chance >= band || !r.InSight || r.Dist != dist {
			t.Fatalf("at %d tiles: %+v", dist, r)
		}
		g.positions.Set(tgt, Position{X: 39, Y: 0}) // out of the way
		g.invalidateCreatures()
	}
	if _, ok := g.HitChance(att, 9999); ok {
		t.Fatal("no such target")
	}
}

func TestApparelArmorInTheGame(t *testing.T) {
	g := New(Config{Seed: 6, Width: 30, Height: 12})
	att, _ := g.SpawnNamedPawn(5, 5, "Ada")
	outcome := map[int]int{} // damage dealt by a hit -> count
	for n := 0; n < 60; n++ {
		tgt := sleeper(g, 6, 5, "Bo")
		g.apparel[tgt] = worn{On: [craft.Slots]craft.Piece{item.Body: {Kind: item.Duster}}}
		for i := 0; i < 8; i++ {
			before := len(g.traces)
			g.combat.Strikes.Strike(att, tgt, weapon.Knife)
			if len(g.traces) > before {
				if tr := g.traces[len(g.traces)-1]; tr.Hit {
					outcome[tr.Damage]++
				}
			}
		}
		g.positions.Set(tgt, Position{X: 29, Y: 11}) // the next one takes its place
		g.invalidateCreatures()
	}
	// a knife (12 sharp, penetration 18 %) on a duster (sharp 24.3 %,
	// torso and legs): A = 6.3 %, so about 3 % deflected, 3 % halved
	knife := weapon.DefOf(weapon.Knife).Damage
	if outcome[0] == 0 || outcome[knife/2] == 0 || outcome[knife] == 0 || len(outcome) != 3 {
		t.Fatalf("deflected, halved and passed blows expected: %v", outcome)
	}
	if 10*(outcome[0]+outcome[knife/2]) > outcome[knife] {
		t.Fatalf("most blows pass a 6 percent armor: %v", outcome)
	}
}

func TestFriendlyFireInTheGame(t *testing.T) {
	g := New(Config{Seed: 9, Width: 40, Height: 10})
	att, _ := g.SpawnNamedPawn(2, 5, "Ada")
	g.Arm(att, item.Rifle)
	ally := sleeper(g, 2+attack.FriendlySafe+3, 5, "Cy")
	tgt := sleeper(g, 20, 5, "Bo")
	struck := false
	for i := 0; i < 100 && !struck; i++ {
		before := len(g.traces)
		g.combat.Strikes.Strike(att, tgt, weapon.Rifle)
		settleShots(t, g)
		for _, tr := range g.traces[before:] {
			struck = struck || tr.Stage == projectile.Landed && tr.Hit && tr.Target == ally
		}
	}
	if !struck || g.VitalOf(ally).Injury == 0 {
		t.Fatal("an ally in the line of fire is struck sometimes")
	}
}

func TestShotInFlightSurvivesASave(t *testing.T) {
	g, att, _ := attackSetup(t, item.Rifle, 20)
	for i := 0; i < 400 && g.combat.Shots.Len() == 0; i++ {
		g.Step(1)
	}
	g.Step(2)
	if g.combat.Shots.Len() != 1 {
		t.Fatal("a shot should be in flight")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "flight"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.World().Shots, h.World().Shots) {
		t.Fatalf("shots after load: %+v vs %+v", g.World().Shots, h.World().Shots)
	}
	g.Step(30)
	h.Step(30)
	landed := func(g *Game) []TraceView {
		var out []TraceView
		for _, tr := range g.World().Traces {
			if tr.Attacker == att && tr.Stage == projectile.Landed {
				out = append(out, tr)
			}
		}
		return out
	}
	if a, b := landed(g), landed(h); len(a) != 1 || !reflect.DeepEqual(a, b) {
		t.Fatalf("the shot lands the same after a load:\n%+v\n%+v", a, b)
	}
}
