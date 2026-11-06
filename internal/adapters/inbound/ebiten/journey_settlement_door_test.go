package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/worldview"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// attackedBase is a real attack on a neutral base, its map in view.
func attackedBase(t *testing.T) *journey {
	t.Helper()
	j, _, _ := neutralBase(t)
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

// TestJourneyEnemyDoorRefusesClaim (MAP-17): a real attack on a base lays its
// doors shut; selecting one says why colonists can't pass, offers only Claim,
// and refuses that with a reason while its defenders stand.
func TestJourneyEnemyDoorRefusesClaim(t *testing.T) {
	t.Parallel()
	j := attackedBase(t)
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
	j := attackedBase(t)
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
