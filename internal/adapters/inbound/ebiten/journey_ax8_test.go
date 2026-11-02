package ebiten

// journey_ax8_test.go (AX-8): the escape ship through the real UI loop.
// Every ship tool is laid from the Architect's Ship category; on a built
// ship a colonist boards the stasis casket from the right-click menu,
// wakes and boards again; the reactor starts behind its warning, a
// countdown raid comes and is beaten (dev hooks), the reactor wakes, the
// alerts follow, and Launch ship ends the story: the ending screen opens
// and the save is marked complete.

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// ax8Research finishes the ship's projects (and what they need).
func ax8Research(j *journey) {
	for _, p := range []research.Project{research.ShipFrames, research.ShipStasis, research.ShipMinds,
		research.ShipReactors, research.ShipDrives, research.ShipSensors} {
		rsFinish(j, int(p))
	}
}

// ax8Ship builds the wiki's minimum ship around (x, y): beam, reactor,
// core, sensor, casket and three engines; it returns the casket's and
// the reactor's cells.
func ax8Ship(j *journey, x, y int) (casket, reactor [2]int) {
	j.t.Helper()
	for _, p := range []struct {
		kind   string
		dx, dy int
	}{
		{"ship structural beam", 2, 0}, {"ship reactor", 4, 0}, {"ship computer core", 0, 0}, {"ship sensor cluster", 0, 2},
		{"ship stasis casket", 1, 4}, {"ship engine", 2, 7}, {"ship engine", 5, 7}, {"ship engine", 8, 7},
	} {
		wdBuild(j, p.kind, x+p.dx, y+p.dy)
	}
	return [2]int{x + 1, y + 4}, [2]int{x + 4, y}
}

// ax8Alerts are the HUD's alert keys now.
func ax8Alerts(j *journey) []string {
	var keys []string
	for _, a := range hud.BuildAlerts(j.g.Snapshot(), j.world()) {
		keys = append(keys, a.Key)
	}
	return keys
}

// ax8Board selects pawn and boards it into the casket at c through the
// right-click menu.
func ax8Board(t *testing.T, j *journey, pawn uint64, c [2]int) {
	t.Helper()
	careSelect(j, pawn)
	if rows := careRightClick(j, c[0], c[1]); !slices.Contains(rows, "Board the ship stasis casket") {
		t.Fatalf("casket menu %q lacks the board order", rows)
	}
	j.pick("Board the ship stasis casket")
	if cmd, ok := j.lastCmd(command.BoardShip); !ok || cmd.Pawn != pawn {
		t.Fatalf("BoardShip %+v ok=%v", cmd, ok)
	}
	j.key("Digit3", 0)
	j.until(func() bool { return ax8Sleeper(j, c) != "" }, 4000)
	j.key("Space", 0)
}

func ax8Sleeper(j *journey, c [2]int) string {
	for _, b := range j.world().Buildings {
		if b.X == c[0] && b.Y == c[1] && b.Kind == "ship stasis casket" {
			return b.AX.Sleeper
		}
	}
	return ""
}

func TestJourneyAX8EscapeShip(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(48, 36), jPawns([2]int{12, 24}, [2]int{15, 24}))
	ax8Research(j)
	for _, tool := range []struct {
		id, kind string
		x, y     int
	}{
		{"ship_beam", "ship structural beam", 30, 4}, {"ship_casket", "ship stasis casket", 33, 4},
		{"ship_core", "ship computer core", 33, 8}, {"ship_sensor", "ship sensor cluster", 33, 12},
		{"ship_engine", "ship engine", 30, 16}, {"ship_reactor", "ship reactor", 38, 4},
	} {
		ax4Lay(t, j, "ship", tool.id, tool.kind, 0, tool.x, tool.y)
	}

	casket, reactor := ax8Ship(j, 12, 6)
	casket2 := [2]int{14, 12} // a second casket under the beam, facing east
	if !j.g.BuildNow("ship stasis casket", "steel", "east", casket2[0], casket2[1]) {
		t.Fatal("no second casket")
	}
	pawn, downed := uint64(j.g.Snapshot()[0].ID), uint64(j.g.Snapshot()[1].ID)
	rid := wdSelect(j, reactor[0], reactor[1])
	lines := wbPaneLines(j, rid)
	for _, want := range []string{"reactor asleep", "launch report:", "the reactor has not been started", "nobody sleeps in a ship casket"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("reactor pane %q lacks %q", lines, want)
		}
	}
	if j.clickID(inspector.LaunchShipBtnID) {
		t.Fatal("Launch ship is enabled on an unready ship")
	}
	// the connectivity overlay: the selected part's ship is highlighted
	j.update(1)
	if sel := j.a.renderer.overlays().ShipSel; sel == 0 || sel != j.world().AX.ShipID {
		t.Fatalf("ship overlay on %d, want the ship %d", sel, j.world().AX.ShipID)
	}

	// board, wake from the casket's pane, board again
	ax8Board(t, j, pawn, casket)
	cid := wdSelect(j, casket[0], casket[1])
	if lines := wbPaneLines(j, cid); !strings.Contains(lines, "sleeper:") {
		t.Fatalf("casket pane %q", lines)
	}
	if !j.clickID(inspector.WakeSleeperBtnID) {
		t.Fatal("no Wake sleeper button")
	}
	if _, ok := j.lastCmd(command.LeaveShip); !ok {
		t.Fatal("Wake sleeper sent no LeaveShip")
	}
	j.update(2)
	if ax8Sleeper(j, casket) != "" {
		t.Fatal("the sleeper did not wake")
	}

	// a downed colonist is carried into the second casket (game/travel's
	// carry, extended to ship caskets)
	careDown(j, downed)
	careSelect(j, pawn)
	j.settle()
	name := j.pawn(downed).Name
	if rows := careRightClick(j, j.pawn(downed).X, j.pawn(downed).Y); !slices.Contains(rows, "Carry "+name+" to ship stasis casket") {
		t.Fatalf("downed colonist's menu %q lacks the ship casket carry", rows)
	}
	j.pick("Carry " + name + " to ship stasis casket")
	if c, ok := j.lastCmd(command.EnterCasket); !ok || c.Worker != downed {
		t.Fatalf("EnterCasket %+v ok=%v", c, ok)
	}
	ax8Run(j, func() bool { return ax8Sleeper(j, casket2) != "" }, 6000)

	ax8Board(t, j, pawn, casket)

	// the reactor starts behind its warning
	wdSelect(j, reactor[0], reactor[1])
	if !j.clickID(inspector.StartReactorBtnID) {
		t.Fatal("no Start reactor button")
	}
	if j.a.uiModalTop() != inspector.ReactorWarnModalID {
		t.Fatalf("no warning before the reactor starts: top %q", j.a.uiModalTop())
	}
	if !j.clickID("modal." + inspector.ReactorWarnModalID + "." + inspector.ReactorWarnOKID) {
		t.Fatal("no Start reactor in the warning")
	}
	if c, ok := j.lastCmd(command.StartReactor); !ok || c.Target != uint64(rid) {
		t.Fatalf("StartReactor %+v ok=%v", c, ok)
	}
	j.update(2)
	if lines := wbPaneLines(j, rid); !strings.Contains(lines, "reactor waking: 15 days") {
		t.Fatalf("reactor pane %q", lines)
	}
	if !slices.Contains(ax8Alerts(j), hud.ShipWakingAlertKey) {
		t.Fatalf("alerts %q lack the waking reactor", ax8Alerts(j))
	}

	// a countdown raid comes (dev hook: skip to it) and is beaten
	game.DevWakeShip(j.g, ship.FirstRaid)
	j.key("Digit3", 0)
	ax8Run(j, func() bool { return ax8Raiders(j) > 0 }, 1500) // the raid's letter pauses the game on its way
	for _, p := range j.g.Snapshot() {
		if p.Hostile {
			j.g.Injure(p.ID, 10000)
		}
	}

	// the reactor wakes (dev hook: skip the countdown)
	game.DevWakeShip(j.g, ship.CountdownTicks)
	ax8Run(j, func() bool { return j.world().AX.Countdown == 0 }, 1500)
	if w := j.world(); len(w.AX.LaunchIssues) != 0 || !slices.Contains(ax8Alerts(j), hud.ShipReadyAlertKey) {
		t.Fatalf("an awake ship reports %q, alerts %q", w.AX.LaunchIssues, ax8Alerts(j))
	}

	// launch: the ship leaves, the ending screen opens, the save says so
	wdSelect(j, reactor[0], reactor[1])
	if !j.clickID(inspector.LaunchShipBtnID) {
		t.Fatal("Launch ship is disabled on a ready ship")
	}
	if j.a.uiModalTop() != inspector.LaunchModalID {
		t.Fatalf("no confirmation before the launch: top %q", j.a.uiModalTop())
	}
	if !j.clickID("modal." + inspector.LaunchModalID + "." + inspector.LaunchOKID) {
		t.Fatal("no Launch in the confirmation")
	}
	if c, ok := j.lastCmd(command.LaunchShip); !ok || c.Target != uint64(rid) {
		t.Fatalf("LaunchShip %+v ok=%v", c, ok)
	}
	j.key("Digit1", 0)
	j.until(func() bool { return j.world().AX.ShipLaunched }, 50)
	j.update(2)
	j.key("Space", 0)
	w := j.world()
	if w.AX.Ending != ship.EndingShip || len(w.AX.Escaped) != 2 {
		t.Fatalf("after the launch: %+v", w.AX)
	}
	if _, _, _, _, ok := overlay.LaunchFlare(w, uint64(j.g.Tick()), false); !ok {
		t.Fatal("no launch flare where the ship stood")
	}
	if j.a.uiModalTop() != hud.EndingModalID {
		t.Fatalf("the ending screen did not open: top %q", j.a.uiModalTop())
	}
	if !j.clickID("modal."+hud.EndingModalID+"."+hud.EndingWatchID) || j.a.uiModalTop() == hud.EndingModalID {
		t.Fatal("Keep watching did not close the ending screen")
	}
	if info := j.g.Info("c"); info.Ending != ship.EndingShip {
		t.Fatalf("save header ending %q", info.Ending)
	}
}

// ax8Run runs the colony at speed 3 until cond holds (at most max
// Updates), pressing the speed key again whenever a threat paused it, and
// pauses it again.
func ax8Run(j *journey, cond func() bool, max int) {
	j.t.Helper()
	for i := 0; i < max && !cond(); i++ {
		if j.a.renderer.speed == speedPaused {
			j.update(1) // let the pause settle before the speed key
			j.key("Digit3", 0)
		}
		j.update(1)
	}
	if !cond() {
		j.t.Fatalf("condition not met after %d Updates (tick %d)", max, j.g.Tick())
	}
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0)
	}
}

// ax8Raiders counts the hostile people on the map.
func ax8Raiders(j *journey) int {
	n := 0
	for _, p := range j.g.Snapshot() {
		if p.Hostile && p.Status != "dead" {
			n++
		}
	}
	return n
}

// TestAX8ShipArt: every ship part's sprite spans its footprint in each
// facing it is drawn in, the occupied casket has its sprite, every glow
// the renderer draws exists, and the mind core and the launch flare have
// their art.
func TestAX8ShipArt(t *testing.T) {
	t.Parallel()
	c := realArt(t)
	for _, k := range []construction.Kind{construction.ShipBeam, construction.ShipCasket, construction.ShipCore,
		construction.ShipReactor, construction.ShipEngine, construction.ShipSensor} {
		d := construction.DefOf(k)
		for f := construction.South; f <= construction.West; f++ {
			if !d.Rotatable && f != construction.South {
				continue
			}
			want := d.Size
			if f == construction.East || f == construction.West {
				want = [2]int{d.Size[1], d.Size[0]}
			}
			keys := []string{world.BuildSprite(d.Name, f.String(), 0, false, false)}
			if k == construction.ShipCasket {
				keys = append(keys, world.ShipKey(game.BuildingView{Kind: d.Name, Facing: f.String(), AX: axview.Building{Sleeper: "Ada"}}))
			}
			for i, key := range keys {
				if m := c.MetaOf(key); m == nil || m.Footprint != want {
					t.Errorf("%s %s: sprite %q footprint %+v, want %v", d.Name, f, key, m, want)
				}
				if glows := k != construction.ShipCasket || i == 1; glows && !c.Has(overlay.GlowKey(key)) {
					t.Errorf("%s %s: no glow layer %q", d.Name, f, overlay.GlowKey(key))
				}
			}
		}
	}
	for _, key := range []string{world.ItemSprite("mind core"), "effect/launch_flare"} {
		if !c.Has(key) {
			t.Errorf("no art %q", key)
		}
	}
}
