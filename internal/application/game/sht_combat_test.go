// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// SHT-006/007/008/102 whole-game tests (the unit tests are in
// combat/sht_test.go).

// TestSHTCombatInGame (a): the hit card on a colonist lying downed shows
// the downed factor by distance, and its headline is the chain's last
// step (the fired shot rolls against the same report: combat
// TestReportDownedTarget).
func TestSHTCombatInGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 40, Height: 10})
	sel, _ := g.SpawnNamedPawn(2, 5, "Ada")
	g.Arm(sel, item.Rifle)
	for _, dist := range []int{3, 4, 5, 12} {
		downed := sleeper(g, 2+dist, 5, "Bo")
		v, _ := g.vital.Get(downed)
		v.Status = health.Downed
		g.vital.Set(downed, v)
		r, ok := g.HitChance(sel, downed)
		if !ok || r.Downed != attack.DownedFactor(dist) || r.Chance != r.Chain()[attack.StepCover] || r.Chance != r.Total() || r.Chance > 1000 {
			t.Fatalf("downed at %d: ok %v %+v", dist, ok, r)
		}
		v.Status = health.Dead
		g.vital.Set(downed, v)
		if _, ok := g.HitChance(sel, downed); ok {
			t.Fatalf("a dead colonist at %d gets no card", dist)
		}
		g.positions.Set(downed, Position{X: 39, Y: 0}) // out of the way
		g.invalidateCreatures()
	}
}

// TestSHTCombatFriendlyFire (b): the same seed and 200 shots past an ally
// 8 tiles out: the difficulty's friendly fire at 0 % spares it from every
// aimed shot, so it is struck strictly less than at 100 %. Both counts are
// pinned for this seed (the aimed-only rule itself: combat
// TestFriendlyFireScale).
func TestSHTCombatFriendlyFire(t *testing.T) {
	t.Parallel()
	hits := func(pct int) int {
		g := New(Config{Seed: 9, Width: 40, Height: 10})
		d := difficulty.Of(difficulty.Seasoned)
		d.FriendlyFirePct = pct
		g.ng.setup = &Setup{Preset: difficulty.Seasoned, Difficulty: d, Storyteller: "steady"}
		att, _ := g.SpawnNamedPawn(2, 5, "Ada")
		g.Arm(att, item.Rifle)
		ally := sleeper(g, 2+attack.InterceptNear+3, 5, "Cy") // 171 permille at 100 %
		tgt := sleeper(g, 20, 5, "Bo")
		n := 0
		for range 200 {
			healPawn(g, ally)
			healPawn(g, tgt)
			ally = standIn(g, ally, "Cy")
			tgt = standIn(g, tgt, "Bo")
			before := len(g.traces)
			g.combat.Strikes.Strike(att, tgt, weapon.Rifle)
			settleShots(t, g)
			for _, tr := range g.traces[before:] {
				if tr.Stage == projectile.Landed && tr.Hit && tr.Target == ally {
					n++
				}
			}
		}
		return n
	}
	full, none := hits(100), hits(0)
	if none >= full {
		t.Fatalf("friendly fire 0 %%: the ally is struck %d times, at 100 %% %d", none, full)
	}
	if full != pinFFFull || none != pinFFNone {
		t.Fatalf("seed 9: %d hits at 100 %%, %d at 0 %%; pinned %d and %d", full, none, pinFFFull, pinFFNone)
	}
}

// TestSHTCombatFriendlyFireSparesOnlyColonists (SHT-007, N2): friendly
// fire scales a colonist's aimed shot only on a colonist in the line. A
// visitor or a leaving raider (on the raider roster, not hostile) in the
// same place is struck exactly as often at FF 0 as at FF 100, shot for
// shot; a colonist there is struck less at FF 0.
func TestSHTCombatFriendlyFireSparesOnlyColonists(t *testing.T) {
	t.Parallel()
	const ix = 2 + attack.InterceptNear + 3 // 8 tiles out
	put := map[string]func(g *Game) entity.EntityID{
		"visitor": func(g *Game) entity.EntityID {
			h := visitArrivals{g}
			id := h.NewPerson(0, nil)
			h.Place(id, 0, ix, 5, uint64(g.clock.Now()))
			return id
		},
		"leaving raider": func(g *Game) entity.EntityID {
			g.RaidArmed(item.Rifle)
			id := g.raiderIDs[0]
			r := g.raiders[id]
			r.Leaving = true
			g.raiders[id] = r
			g.positions.Set(id, Position{X: ix, Y: 5})
			g.invalidateCreatures()
			return id
		},
		"colonist": func(g *Game) entity.EntityID { return sleeper(g, ix, 5, "Cy") },
	}
	run := func(kind string, pct int) (int, []projectile.Trace) {
		g := New(Config{Seed: 9, Width: 40, Height: 10})
		d := difficulty.Of(difficulty.Seasoned)
		d.FriendlyFirePct = pct
		g.ng.setup = &Setup{Preset: difficulty.Seasoned, Difficulty: d, Storyteller: "steady"}
		att, _ := g.SpawnNamedPawn(2, 5, "Ada")
		g.Arm(att, item.Rifle)
		tgt := sleeper(g, 20, 5, "Bo")
		in := put[kind](g)
		n, from := 0, len(g.traces)
		for range 120 {
			for _, id := range []entity.EntityID{in, tgt} {
				v, _ := g.vital.Get(id)
				v.Injury, v.Status = 0, health.Alive
				g.vital.Set(id, v)
			}
			before := len(g.traces)
			g.combat.Strikes.Strike(att, tgt, weapon.Rifle)
			settleShots(t, g)
			for _, tr := range g.traces[before:] {
				if tr.Stage == projectile.Landed && tr.Hit && tr.Target == in {
					n++
				}
			}
		}
		return n, g.traces[from:]
	}
	for _, kind := range []string{"visitor", "leaving raider"} {
		n0, tr0 := run(kind, 0)
		n100, tr100 := run(kind, 100)
		if n100 == 0 || n0 != n100 || !reflect.DeepEqual(tr0, tr100) {
			t.Errorf("%s in the line: struck %d times at FF 0, %d at FF 100; want identical shots", kind, n0, n100)
		}
	}
	n0, _ := run("colonist", 0)
	n100, _ := run("colonist", 100)
	if n0 >= n100 {
		t.Fatalf("a colonist in the line: struck %d times at FF 0, not fewer than %d at FF 100", n0, n100)
	}
}

// TestSHTCombatDownedInLine (SHT-006 in game): a colonist lying downed
// 12 tiles down the line is in the creature index and takes some of the
// shots past it, at a quarter of a standing one's chance. The count is
// pinned for this seed.
func TestSHTCombatDownedInLine(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 40, Height: 10})
	att, _ := g.SpawnNamedPawn(2, 5, "Ada")
	g.Arm(att, item.Rifle)
	lying := sleeper(g, 2+attack.InterceptFull, 5, "Cy")
	tgt := sleeper(g, 25, 5, "Bo")
	n := 0
	for range 200 {
		lying = downIn(g, lying, "Cy")
		healPawn(g, tgt)
		tgt = standIn(g, tgt, "Bo")
		before := len(g.traces)
		g.combat.Strikes.Strike(att, tgt, weapon.Rifle)
		settleShots(t, g)
		for _, tr := range g.traces[before:] {
			if tr.Stage == projectile.Landed && tr.Hit && tr.Target == lying {
				n++
			}
		}
	}
	if n == 0 || n != pinDownedHits {
		t.Fatalf("seed 9: the downed colonist is struck %d times in 200 shots, pinned %d", n, pinDownedHits)
	}
}

// pinDownedHits is TestSHTCombatDownedInLine's count (seed 9).
const pinDownedHits = 13

// downIn keeps id lying downed and unhurt in its place (a fresh one if it
// died).
func downIn(g *Game, id entity.EntityID, name string) entity.EntityID {
	if v, _ := g.vital.Get(id); v.Status == health.Dead {
		p, _ := g.positions.Get(id)
		g.positions.Set(id, Position{X: 39, Y: 9})
		g.invalidateCreatures()
		id = sleeper(g, p.X, p.Y, name)
	}
	v, _ := g.vital.Get(id)
	v.Injury, v.Status = 0, health.Downed
	g.vital.Set(id, v)
	return id
}

// standIn is id if it still stands, else a fresh sleeper in its place
// (the fallen one carried off the line).
func standIn(g *Game, id entity.EntityID, name string) entity.EntityID {
	if g.standingPawn(id) {
		return id
	}
	p, _ := g.positions.Get(id)
	g.positions.Set(id, Position{X: 39, Y: 9})
	g.invalidateCreatures()
	return sleeper(g, p.X, p.Y, name)
}

// The pinned ally hit counts of TestSHTCombatFriendlyFire (seed 9).
const (
	pinFFFull = 36
	pinFFNone = 25 // wild shots only: never scaled
)

// TestSHTCombatRaidAim (SHT-102): the raiders' first aim goes through the
// same scaling as the colonists': a quick-draw colonist's gun aims in half
// its warmup, a melee weapon never scales, and a raider with no aiming
// stat aims in the warmup.
func TestSHTCombatRaidAim(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 6, Width: 30, Height: 12})
	q := spawnGen(t, g, 2, 2, genPawn("Quick", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.QuickDraw} }))
	rifle, knife := weapon.DefOf(weapon.Rifle), weapon.DefOf(weapon.Knife)
	fight := raidFight{g}
	if got := fight.AimTicks(q, rifle); got != rifle.Warmup/2 || got == rifle.Warmup {
		t.Fatalf("quick draw rifle aim %d, want %d", got, rifle.Warmup/2)
	}
	if got := fight.AimTicks(q, knife); got != knife.Warmup {
		t.Fatalf("quick draw knife aim %d, want %d", got, knife.Warmup)
	}
	g.RaidArmed(item.Rifle)
	var rid entity.EntityID
	for _, id := range g.raiderIDs {
		if weaponOf(g, id) == weapon.Rifle {
			rid = id
			break
		}
	}
	if rid == 0 {
		t.Fatal("no rifle raider")
	}
	if got := fight.AimTicks(rid, rifle); got != rifle.Warmup {
		t.Fatalf("a raider with no aiming stat aims %d, want %d", got, rifle.Warmup)
	}
}
