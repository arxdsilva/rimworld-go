// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/stat/statmod"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// extMods are three stat_mods: a trait's, a bio condition's and
// injury's, each on its own stat.
func extMods() statmod.Book {
	return statmod.Book{Defs: []statmod.Def{
		{Key: "m.steady", Stat: stat.WorkSpeed, Mul: 500, When: statmod.Cond{Trait: "core.steady"}},
		{Key: "m.near", Stat: stat.ShootAcc, Add: -7, When: statmod.Cond{Condition: pawngen.Nearsighted.Key()}},
		{Key: "m.hurt", Stat: stat.MoodOffset, Add: -10, When: statmod.Cond{Injured: true}},
	}}
}

// sheetOf is id's stat sheet (Base for none).
func sheetOf(g *Game, id entity.EntityID) stat.Sheet {
	if fx := g.sk.stats[id]; fx != nil {
		return fx.sheet
	}
	return stat.Base()
}

// sheetWithout is id's sheet rebuilt with no stat_mods (then restored).
func sheetWithout(g *Game, id entity.EntityID) stat.Sheet {
	book := g.sk.content.StatMods
	g.sk.content.StatMods = statmod.Book{}
	g.rebuildStats(id)
	s := sheetOf(g, id)
	g.sk.content.StatMods = book
	g.rebuildStats(id)
	return s
}

// setBio gives id bio p with only the given traits and conditions.
func setBio(g *Game, id entity.EntityID, ts []trait.ID, cs []pawngen.Condition) {
	p, _ := g.bioFor(id)
	p.Traits, p.Conditions = ts, cs
	g.giveBio(id, p)
}

// TestExtStatLayer: a stat_mod is the sheet's last layer while its fact
// holds (a trait key, a bio condition key, injured) and is gone with it.
func TestExtStatLayer(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 16, Height: 16, Content: Content{StatMods: extMods()}})
	ada := spawnColonists(t, g, 1, 2)[0]
	setBio(g, ada, nil, nil)
	if got, want := sheetOf(g, ada), sheetWithout(g, ada); got != want {
		t.Fatalf("no fact holds, yet the sheet moved: %v vs %v", got, want)
	}
	if traitsOf(g).Key(trait.Steady) != "core.steady" {
		t.Fatalf("fixture: trait key %q", traitsOf(g).Key(trait.Steady))
	}
	setBio(g, ada, []trait.ID{trait.Steady}, []pawngen.Condition{pawngen.Nearsighted})
	got, plain := sheetOf(g, ada), sheetWithout(g, ada)
	if got[stat.WorkSpeed] != plain[stat.WorkSpeed]/2 || got[stat.ShootAcc] != plain[stat.ShootAcc]-7 || got[stat.MoodOffset] != plain[stat.MoodOffset] {
		t.Fatalf("trait and condition: work %d (plain %d), aim %d (plain %d), mood %d (plain %d)",
			got[stat.WorkSpeed], plain[stat.WorkSpeed], got[stat.ShootAcc], plain[stat.ShootAcc], got[stat.MoodOffset], plain[stat.MoodOffset])
	}
	b := addWound(t, g, ada, body.LegL, 12, body.Cut)
	if statKeyOf(keyOf(b)) == healthyStatKey {
		t.Fatal("fixture: the wound moved no stat")
	}
	if got, plain := sheetOf(g, ada), sheetWithout(g, ada); got[stat.MoodOffset] != plain[stat.MoodOffset]-10 {
		t.Fatalf("injured: mood %d, plain %d", got[stat.MoodOffset], plain[stat.MoodOffset])
	}
	v, _ := g.vital.Get(ada)
	b.Wounds = nil
	g.bodyChanged(ada, b, &v)
	setBio(g, ada, nil, nil)
	if got, plain := sheetOf(g, ada), sheetWithout(g, ada); got != plain {
		t.Fatalf("every fact gone, the sheet still moved: %v vs %v", got, plain)
	}
}

// TestExtStatModsSurviveLoad: a healthy pawn with a matching trait has
// the same sheet after save, load and 600 ticks as in a continuous run,
// and a later bio change rebuilds it.
func TestExtStatModsSurviveLoad(t *testing.T) {
	t.Parallel()
	c := Content{StatMods: extMods()}
	g := New(Config{Seed: 3, Width: 16, Height: 16, Content: c})
	ada := spawnColonists(t, g, 1, 2)[0]
	setBio(g, ada, []trait.ID{trait.Steady}, nil)
	if sheetOf(g, ada) == sheetWithout(g, ada) || statKeyOf(keyOf(g.bodies[ada])) != healthyStatKey {
		t.Fatal("fixture: a healthy pawn whose trait moves its sheet")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "stat"); err != nil {
		t.Fatal(err)
	}
	h, _, err := LoadContent(bytes.NewReader(buf.Bytes()), c)
	if err != nil {
		t.Fatal(err)
	}
	g.Step(600)
	h.Step(600)
	if a, b := sheetOf(g, ada), sheetOf(h, ada); a != b {
		t.Fatalf("continuous %v, loaded %v", a, b)
	}
	setBio(h, ada, nil, []pawngen.Condition{pawngen.Nearsighted})
	if got, plain := sheetOf(h, ada), sheetWithout(h, ada); got[stat.WorkSpeed] != plain[stat.WorkSpeed] || got[stat.ShootAcc] != plain[stat.ShootAcc]-7 {
		t.Fatalf("after the bio change: %v, plain %v", got, plain)
	}
}
