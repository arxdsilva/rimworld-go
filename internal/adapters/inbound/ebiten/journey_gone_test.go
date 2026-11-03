package ebiten

import (
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/worldview"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// journey_gone_test.go: the everyone-gone screen (MAP-14) and attacks on
// settlements that are not hostile (MAP-15).

// goneID is a button of the everyone-gone screen.
func goneID(btn string) string { return "modal." + hud.GoneModalID + "." + btn }

// abandonColony presses Abandon on our colony in the world view and
// confirms it.
func abandonColony(j *journey) {
	j.t.Helper()
	worldOpen(j)
	if !j.clickID("world.act.abandon") || j.a.uiModalTop() != worldview.AbandonID {
		j.t.Fatalf("Abandon opened modal %q, want the confirm", j.a.uiModalTop())
	}
	if !j.clickID("modal." + worldview.AbandonID + ".ok") {
		j.t.Fatal("the Abandon confirm refused its button")
	}
	if _, ok := j.lastCmd(command.AbandonMap); !ok {
		j.t.Fatal("AbandonMap was not sent")
	}
	j.g.Step(60)
	j.update(2)
}

// TestJourneyEveryoneGoneScreen: abandoning the last colony with nothing on
// the road ends the game's story: the confirm warns of it, the screen opens,
// Keep watching closes it for good, and Main menu leaves for the menu.
func TestJourneyEveryoneGoneScreen(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		btn  string
	}{{"watch", hud.GoneWatchID}, {"menu", hud.GoneMenuID}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			j, _ := worldColony(t)
			worldOpen(j)
			if !j.clickID("world.act.abandon") {
				why, _ := gizmoWhy(j, "world.act.abandon")
				t.Fatalf("Abandon is disabled on the last colony: %q", why)
			}
			j.clickID("modal." + worldview.AbandonID + ".cancel")
			abandonColony(j)
			if !j.world().AX.EveryoneGone {
				t.Fatal("EveryoneGone is false after abandoning the last colony")
			}
			if j.a.uiModalTop() != hud.GoneModalID {
				t.Fatalf("the everyone-gone screen did not open: top %q", j.a.uiModalTop())
			}
			j.draw(2)
			if w, ok := j.a.uiSt().Find(goneID(hud.GoneLoadID)); !ok || w.Disabled {
				t.Fatalf("Load game: found %v, disabled %v", ok, w.Disabled)
			}
			if !j.clickID(goneID(c.btn)) {
				t.Fatalf("%s refused the click", c.btn)
			}
			if c.btn == hud.GoneMenuID {
				if j.a.nav.screen() != ports.ScreenMainMenu {
					t.Fatalf("Main menu left on screen %s", j.a.nav.screen())
				}
				return
			}
			// No flap: it stays true, the screen stays closed.
			j.update(120)
			if j.a.uiModalTop() == hud.GoneModalID {
				t.Fatal("the screen reopened while everyone is still gone")
			}
		})
	}
}

// TestJourneyEveryoneGoneLoadGame: Load game opens the Load screen.
func TestJourneyEveryoneGoneLoadGame(t *testing.T) {
	t.Parallel()
	j, _ := worldColony(t)
	abandonColony(j)
	if j.a.uiModalTop() != hud.GoneModalID {
		t.Fatalf("top %q, want the everyone-gone screen", j.a.uiModalTop())
	}
	if w, _ := j.a.uiSt().Find(goneID(hud.GoneLoadID)); w.Disabled {
		t.Skipf("Load is off in this test app: %q", w.Reason)
	}
	j.clickID(goneID(hud.GoneLoadID))
	if j.a.nav.screen() != ports.ScreenLoad {
		t.Fatalf("Load game opened %s, want the load screen", j.a.nav.screen())
	}
}

// TestJourneyEveryoneGoneNotWithACaravan: a caravan on the road still holds
// a colonist, so abandoning the colony does not end the game, and the
// confirm does not say it would.
func TestJourneyEveryoneGoneNotWithACaravan(t *testing.T) {
	t.Parallel()
	j, p := worldColony(t)
	s, _ := nearestSite(j, p, false)
	formCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
	worldOpen(j)
	for range 3 { // the caravan shares the colony's tile while it forms: a slower click cycles
		if j.a.world.Selected() == "map:0" {
			break
		}
		j.settle()
		j.clickID("world.obj.map.0")
	}
	if got := j.a.world.Selected(); got != "map:0" {
		t.Fatalf("selected %q, want the colony", got)
	}
	if !j.clickID("world.act.abandon") {
		t.Fatal("Abandon is disabled")
	}
	j.clickID("modal." + worldview.AbandonID + ".ok")
	j.g.Step(60)
	j.update(2)
	if j.world().AX.EveryoneGone || j.a.uiModalTop() == hud.GoneModalID {
		t.Fatalf("a caravan is out: EveryoneGone %v, top %q", j.world().AX.EveryoneGone, j.a.uiModalTop())
	}
}

// TestJourneyEveryoneGoneWhenAllDie: when the last colonists die the screen
// opens by itself, on a real game.
func TestJourneyEveryoneGoneWhenAllDie(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}))
	if j.world().AX.EveryoneGone || j.a.uiModalTop() == hud.GoneModalID {
		t.Fatal("the screen is up with colonists alive")
	}
	for i := 0; i < 400 && !j.world().AX.EveryoneGone; i++ {
		for _, p := range j.g.Snapshot() {
			if p.Status != "dead" {
				j.g.Injure(p.ID, 500)
			}
		}
		j.g.Step(10)
	}
	if !j.world().AX.EveryoneGone {
		t.Fatalf("EveryoneGone is false with %d colonists listed", len(j.g.Colonists()))
	}
	j.update(2)
	if j.a.uiModalTop() != hud.GoneModalID {
		t.Fatalf("the screen did not open: top %q", j.a.uiModalTop())
	}
	if !j.clickID(goneID(hud.GoneWatchID)) {
		t.Fatal("Keep watching refused the click")
	}
	j.update(60)
	if j.a.uiModalTop() == hud.GoneModalID {
		t.Fatal("the screen reopened while it is still true")
	}
}

// neutralBase is the nearest settlement that is not hostile, with a caravan
// standing on it and selected in the world view.
func neutralBase(t *testing.T) (j *journey, s game.WorldSiteView, c game.CaravanView) {
	t.Helper()
	j, p := worldColony(t)
	s, _ = nearestSite(j, p, false)
	c = formCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
	game.DevArrive(j.g, entity.EntityID(c.ID), s.Tile)
	worldOpen(j)
	worldFind(j, s)
	worldPick(j, "world.obj.caravan."+strconv.FormatUint(c.ID, 10), "caravan:"+strconv.FormatUint(c.ID, 10))
	return j, s, c
}

// goodwillOf is faction name's goodwill.
func goodwillOf(j *journey, name string) int {
	j.t.Helper()
	for _, f := range j.g.Factions() {
		if f.Name == name {
			return f.Goodwill
		}
	}
	j.t.Fatalf("no faction %q", name)
	return 0
}

// TestJourneyWorldAttackNeutralBase: Attack is on at a base that is not
// hostile and asks first, naming the cost; Cancel changes nothing and sends
// nothing; Attack sends AttackSettlement and the faction falls out with us.
func TestJourneyWorldAttackNeutralBase(t *testing.T) {
	t.Parallel()
	j, s, c := neutralBase(t)
	before := goodwillOf(j, s.Faction)
	if why, off := gizmoWhy(j, "world.act.attack"); off {
		t.Fatalf("Attack is disabled at a %s base: %q", s.Relation, why)
	}
	if !j.clickID("world.act.attack") || j.a.uiModalTop() != worldview.AttackID {
		t.Fatalf("Attack opened modal %q, want the confirm", j.a.uiModalTop())
	}
	if _, sent := j.lastCmd(command.AttackSettlement); sent {
		t.Fatal("the attack left before it was confirmed")
	}
	j.clickID("modal." + worldview.AttackID + ".cancel")
	if _, sent := j.lastCmd(command.AttackSettlement); sent || j.a.uiModalTop() != "" {
		t.Fatalf("Cancel: attack sent %v, modal %q", sent, j.a.uiModalTop())
	}
	// The press was the dialog's: the caravan stays selected.
	if !j.clickID("world.act.attack") {
		t.Fatal("Cancel's click reached the globe and changed the selection")
	}
	j.clickID("modal." + worldview.AttackID + ".cancel")
	j.g.Step(60)
	if got := goodwillOf(j, s.Faction); got != before || siteAtTile(j, s.Tile).Relation == "hostile" {
		t.Fatalf("Cancel moved goodwill %d -> %d or relation to %q", before, got, siteAtTile(j, s.Tile).Relation)
	}
	j.clickID("world.act.attack")
	if !j.clickID("modal." + worldview.AttackID + ".ok") {
		t.Fatal("the confirm refused Attack")
	}
	if cmd, ok := j.lastCmd(command.AttackSettlement); !ok || cmd.Target != c.ID {
		t.Fatalf("AttackSettlement %+v (sent %v), want caravan %d", cmd, ok, c.ID)
	}
	j.g.Step(60)
	j.update(2)
	if got := goodwillOf(j, s.Faction); got >= before {
		t.Fatalf("goodwill %d -> %d after the attack, want it down", before, got)
	}
	if r := siteAtTile(j, s.Tile).Relation; r != "hostile" {
		t.Fatalf("relation %q after the attack, want hostile", r)
	}
}

// TestJourneyWorldPodsAttackNeutralBase: pods landing in Attack mode on a
// settlement that is not hostile ask first.
func TestJourneyWorldPodsAttackNeutralBase(t *testing.T) {
	t.Parallel()
	j, pod, _ := podsReady(t, false, false)
	var s game.WorldSiteView
	for _, c := range j.g.WorldSites() {
		if c.Relation != "hostile" && hasWidget(j.a, "world.pod.target."+strconv.Itoa(c.Tile)) {
			s = c
			break
		}
	}
	if s.Name == "" {
		t.Skip("no non-hostile settlement in the pods' range")
	}
	j.clickID("world.pod.target." + strconv.Itoa(s.Tile))
	if !j.clickID("world.pod.attackedge") || j.a.uiModalTop() != worldview.AttackID {
		t.Fatalf("Attack: edge opened modal %q, want the confirm", j.a.uiModalTop())
	}
	if _, sent := j.lastCmd(command.LaunchPods); sent {
		t.Fatal("the pods left before the confirm")
	}
	j.clickID("modal." + worldview.AttackID + ".cancel")
	if _, sent := j.lastCmd(command.LaunchPods); sent || j.a.uiModalTop() != "" {
		t.Fatalf("Cancel: launch sent %v, modal %q", sent, j.a.uiModalTop())
	}
	if !hasWidget(j.a, "world.pod.attackedge") {
		j.clickID("world.pod.target." + strconv.Itoa(s.Tile))
	}
	j.clickID("world.pod.attackedge")
	if !j.clickID("modal." + worldview.AttackID + ".ok") {
		t.Fatal("the confirm refused Attack")
	}
	cmd, ok := j.lastCmd(command.LaunchPods)
	if !ok || cmd.Mode != command.PodArriveAttackEdge || cmd.Count != s.Tile || cmd.Target != uint64(pod) {
		t.Fatalf("LaunchPods %+v (sent %v), want pod %d to %d in attack mode", cmd, ok, pod, s.Tile)
	}
}
