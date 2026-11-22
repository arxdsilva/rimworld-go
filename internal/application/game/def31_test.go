// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
)

// DEF 3.1 whole-game tests: log kinds (History Threats tab), records
// from blasts, damage taken and the v19 save.

// def31Bomb is a blast that lands its full damage on whoever stands at
// its centre.
var def31Bomb = explosion.Def{Kind: explosion.Bomb, Radius: 900, Damage: 900, Pen: 1000}

// TestDef31LogKinds: a raid coming and drop pods are threat entries; a
// raider killed and a colonist struck down are fight entries; the read
// model's Threat mirrors the kind.
func TestDef31LogKinds(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 40, 30, 20, 15)
	bea, err := g.SpawnNamedPawn(30, 25, "Bea")
	if err != nil {
		t.Fatal(err)
	}
	g.RaidArmed(item.Club)
	g.RaidDropPods(item.Club)
	if len(g.raiderIDs) != 1 {
		t.Fatalf("one raider walked in, got %d", len(g.raiderIDs))
	}
	b, _ := g.positions.Get(bea)
	for i := 0; i < 20 && g.standingPawn(bea); i++ {
		g.combat.Blasts.Explode(0, weapon.FragGrenade, b.X, b.Y, def31Bomb) // an IED under Bea
	}
	rid := g.raiderIDs[0]
	p, _ := g.positions.Get(rid)
	dead := func() bool { v, _ := g.vital.Get(rid); return v.Status == health.Dead }
	for i := 0; i < 20 && !dead(); i++ { // a blast may take a limb, not a life
		g.combat.Blasts.Explode(ada, weapon.FragGrenade, p.X, p.Y, def31Bomb)
	}
	g.Step(1) // the raider AI logs the death
	kinds := map[string]combat.LogKind{"came": combat.LogThreat, "Drop pods!": combat.LogThreat,
		"was killed.": combat.LogFight, "Bea was struck down.": combat.LogFight}
	seen := map[string]bool{}
	view := g.World().CombatLog
	for i, e := range g.combat.Log.Entries() {
		for sub, k := range kinds {
			if strings.Contains(e.Text, sub) {
				seen[sub] = true
				if e.Kind != k || view[i].Threat != (k == combat.LogThreat) || view[i].Text != e.Text {
					t.Errorf("%q: kind %d threat %v, want kind %d", e.Text, e.Kind, view[i].Threat, k)
				}
			}
		}
	}
	if len(seen) != len(kinds) {
		t.Fatalf("logged %v of %v: %+v", seen, kinds, view)
	}
	if v := g.combat.Gear.View(ada); v.Kills != 1 || v.Damage == 0 {
		t.Fatalf("the bomb's thrower is credited once: %+v", v)
	}
	bv, _ := g.vital.Get(bea)
	beaDead := bv.Status == health.Dead
	if s := g.combat.Turrets.Export(); s != nil {
		for _, k := range s.Gear {
			if k.ID == rid || k.ID == bea && beaDead {
				t.Fatalf("a dead fighter's kit is kept: %+v", k)
			}
		}
	}
}

// TestDef31DamageTakenView: the damage a colonist took reaches its
// PawnView beside the damage its attacker dealt.
func TestDef31DamageTakenView(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 30, 20, 15, 10)
	g.RaidArmed(item.Club)
	rid := g.raiderIDs[0]
	g.combat.Gear.Count(rid, ada, 7, false)
	g.combat.Gear.Count(0, ada, 5, false) // an IED
	v := pawnView(g, ada)
	if v.ID != ada || v.DamageTaken != 12 || v.DamageDealt != 0 {
		t.Fatalf("ada took %d, dealt %d", v.DamageTaken, v.DamageDealt)
	}
	if g.combat.Gear.View(rid).Damage != 7 || g.combat.Gear.LastBy(ada) != rid {
		t.Fatalf("raider's records %+v", g.combat.Gear.View(rid))
	}
}

// TestDef31SaveV18Loads: v19 keeps damage taken and log kinds; a v18
// save (neither field) loads with unsorted entries and no damage taken.
func TestDef31SaveV18Loads(t *testing.T) {
	t.Parallel()
	g, ada := def4Game(t, 30, 20, 15, 10)
	g.combat.Gear.Count(0, ada, 9, false)
	g.combat.Log.Alarm("A raid of 3 came: assault.")
	g.combat.Log.Fight("Raider Rook was killed.")
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	l := h.World().CombatLog
	if v := pawnView(h, ada); v.DamageTaken != 9 || len(l) != 2 || !l[0].Threat || l[1].Threat {
		t.Fatalf("v19: took %d, log %+v", v.DamageTaken, l)
	}
	old := loadAsVersionWith(t, g, 18, func(s *saveState) {
		for i := range s.Turrets.Log {
			s.Turrets.Log[i].Kind = 0
		}
		for i := range s.Turrets.Gear {
			s.Turrets.Gear[i].K.DamageTaken = 0
		}
	})
	for _, e := range old.combat.Log.Entries() {
		if e.Kind != combat.LogUnsorted {
			t.Fatalf("a v18 entry has a kind: %+v", e)
		}
	}
	l = old.World().CombatLog
	if v := pawnView(old, ada); v.DamageTaken != 0 || len(l) != 2 || l[0].Threat || l[1].Threat {
		t.Fatalf("v18: took %d, log %+v", v.DamageTaken, l)
	}
}
