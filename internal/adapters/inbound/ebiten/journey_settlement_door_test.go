// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/worldview"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

// attackedBase is a real attack on a neutral base, its map in view. With
// tech set, the base is the nearest neutral one of a faction at that tech
// level (only an industrial base has sentry turrets, mapgen.Settlement).
func attackedBase(t *testing.T, tech string) *journey {
	t.Helper()
	var j *journey
	if tech == "" {
		j, _, _ = neutralBase(t)
	} else {
		j = neutralBaseOf(t, tech)
	}
	j.clickID("world.act.attack")
	if !j.clickID("modal." + worldview.AttackID + ".ok") {
		t.Fatal("the confirm refused Attack")
	}
	j.g.Step(60)
	var away game.MapID
	for _, m := range j.g.Maps() {
		if m.Kind == "settlement" {
			away = m.ID
		}
	}
	if away == 0 {
		t.Fatalf("the attack opened no settlement map: %+v", j.g.Maps())
	}
	j.key("F8", 0)
	j.update(worldFly)
	j.g.ViewMap(away)
	// The renderer turns the read-model cache on for the map it starts on;
	// the game keeps that flag per map, so a map opened later builds every
	// view afresh on each call (about 0.8 s an Update under -race here).
	// Turn it on for the settlement too, as the renderer means to.
	j.g.SetViewCache(true)
	j.update(2)
	return j
}

// neutralBaseOf is neutralBase at the nearest not-hostile settlement of a
// faction whose tech level is tech, chosen by the faction, not by the seed.
func neutralBaseOf(t *testing.T, tech string) *journey {
	t.Helper()
	j, p := worldColony(t)
	ofTech := map[string]bool{}
	for _, f := range j.g.Factions() {
		if f.Tech == tech {
			ofTech[f.Name] = true
		}
	}
	var s game.WorldSiteView
	bd, home := -1, j.g.WorldMap().Tile
	var sc planet.HopScratch
	for _, v := range j.g.WorldSites() {
		if v.Relation == "hostile" || !ofTech[v.Faction] {
			continue
		}
		if d := p.HopsWith(&sc, home, v.Tile, 200); d >= 0 && (bd < 0 || d < bd) {
			s, bd = v, d
		}
	}
	if bd < 0 {
		t.Fatalf("no reachable %s base that is not hostile in %+v", tech, j.g.WorldSites())
	}
	c := formCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
	game.DevArrive(j.g, entity.EntityID(c.ID), s.Tile)
	worldOpen(j)
	worldFind(j, s)
	worldPick(j, "world.obj.caravan."+strconv.FormatUint(c.ID, 10), "caravan:"+strconv.FormatUint(c.ID, 10))
	return j
}

// TestJourneyEnemyDoorRefusesClaim (MAP-17): a real attack on a base lays its
// doors shut; selecting one says why colonists can't pass, offers only Claim,
// and refuses that with a reason while its defenders stand.
func TestJourneyEnemyDoorRefusesClaim(t *testing.T) {
	t.Parallel()
	j := attackedBase(t, "")
	var door game.BuildingView
	for _, b := range j.g.World().Buildings {
		if b.Hostile && b.Kind == "door" {
			door = b
			break
		}
	}
	if door.ID == 0 {
		t.Fatal("the settlement has no enemy door")
	}
	if door.HoldOpen {
		t.Fatal("an enemy door lies held open")
	}
	wdSelect(j, door.X, door.Y)
	j.update(2)
	if lines := bldLines(j.a.insp.Building().BuildingPane(door.ID)); !strings.Contains(lines, inspector.EnemyDoorLine) {
		t.Fatalf("the pane says %q", lines)
	}
	if ids := bldCmdIDs(j.a.insp.Building().BuildingCommands(door.ID)); len(ids) != 1 || ids[0] != inspector.DoorClaimID {
		t.Fatalf("buttons %v, want Claim alone", ids)
	}
	claim, ok := bldCmd(j.a.insp.Building().BuildingCommands(door.ID), inspector.DoorClaimID)
	if !ok || !claim.Disabled || !strings.Contains(claim.Reason, "defenders") {
		t.Fatalf("Claim before the defeat: %+v (found %v)", claim, ok)
	}
	var shown []string
	found := false
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		shown = append(shown, b.ID)
		if b.ID == inspector.DoorClaimID {
			found = true
			if !b.Disabled {
				t.Fatalf("the Claim on screen is live: %+v", b)
			}
		}
	}
	if !found {
		t.Fatalf("no Claim on screen: %q", shown)
	}
	j.reset()
	if j.clickID(inspector.DoorClaimID) {
		t.Fatal("the refused Claim took a click")
	}
	if _, sent := j.lastCmd(command.DesignateClaim); sent {
		t.Fatal("the refused Claim sent its command")
	}
}

// TestEnemyDoorClaimSendsTheOrder (MAP-17): with no defender standing, Claim
// sends the Claim order on that one tile.
func TestEnemyDoorClaimSendsTheOrder(t *testing.T) {
	t.Parallel()
	const id = entity.EntityID(7)
	s := bldStub(game.BuildingView{ID: id, Kind: "door", X: 4, Y: 6, W: 1, H: 1, Hostile: true})
	a, _ := bldApp(t, s, id)
	b, ok := bldCmd(a.insp.Building().BuildingCommands(id), inspector.DoorClaimID)
	if !ok || b.Disabled {
		t.Fatalf("Claim with nobody left: %+v (found %v)", b, ok)
	}
	pressCmd(a, b)
	if len(s.cmds) == 0 {
		t.Fatal("Claim sent nothing")
	}
	c := s.cmds[len(s.cmds)-1]
	if c.Kind != command.DesignateClaim || c.X != 4 || c.Y != 6 || c.X2 != 4 || c.Y2 != 6 || !c.Enabled {
		t.Fatalf("Claim sent %+v", c)
	}
}

// TestJourneyClaimAfterTheDefeat (MAP-17, MAP-21; wiki: Door, Faction base): the
// defenders fall, and the door and the turret each take a click on Claim. The
// door stops being hostile, the turret goes on with no grid.
func TestJourneyClaimAfterTheDefeat(t *testing.T) {
	t.Parallel()
	j := attackedBase(t, research.Industrial.String())
	for _, p := range j.g.Snapshot() {
		if p.Hostile {
			j.g.Injure(p.ID, 100000)
		}
	}
	j.update(2)
	claim := func(kind func(game.BuildingView) bool, id string) entity.EntityID {
		t.Helper()
		var b game.BuildingView
		for _, v := range j.g.World().Buildings {
			if v.Hostile && kind(v) {
				b = v
				break
			}
		}
		if b.ID == 0 {
			t.Fatal("the base has no such piece")
		}
		wdSelect(j, b.X, b.Y)
		j.update(2)
		if !j.clickID(id) {
			t.Fatalf("Claim on the %s took no click", b.Kind)
		}
		j.update(2)
		return b.ID
	}
	door := claim(func(b game.BuildingView) bool { return b.Kind == "door" }, inspector.DoorClaimID)
	gun := claim(func(b game.BuildingView) bool { return b.Kind != "door" }, inspector.TurretClaimID)
	for _, v := range j.g.World().Buildings {
		switch v.ID {
		case door:
			if v.Hostile {
				t.Fatalf("door %+v", v)
			}
		case gun:
			if v.Hostile || !v.OffGrid || !v.Powered {
				t.Fatalf("turret %+v", v)
			}
			if lines := bldLines(j.a.insp.Building().BuildingPane(gun)); !strings.Contains(lines, inspector.OffGridLine) {
				t.Fatalf("the pane says %q", lines)
			}
		}
	}
}
