package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// powerView is the building view with that id.
func powerView(t *testing.T, j *journey, id entity.EntityID) game.BuildingView {
	t.Helper()
	for _, b := range j.world().Buildings {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("building %d vanished", id)
	return game.BuildingView{}
}

// TestJourneyPowerReconnect (PWR-4): a heater wired to the near grid with a
// second grid in range gets a Reconnect button; a click wires it to the other
// grid at once. A heater with one grid in range has the button disabled with
// its reason, and a click on it sends nothing.
func TestJourneyPowerReconnect(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	// Grid A: a conduit and a battery at x 10-11; grid B at x 22-23.
	for _, p := range []struct {
		kind string
		x, y int
	}{{"power conduit", 11, 2}, {"battery", 10, 2}, {"power conduit", 22, 2}, {"battery", 23, 2}, {"heater", 16, 2}, {"heater", 11, 6}} {
		wdBuild(j, p.kind, p.x, p.y)
	}
	j.update(2)
	hot := wdBuilding(j, 16, 2)
	lone := wdBuilding(j, 11, 6)
	gridA := powerView(t, j, wdBuilding(j, 11, 2)).PowerNet
	gridB := powerView(t, j, wdBuilding(j, 22, 2)).PowerNet
	if gridA == 0 || gridB == 0 || gridA == gridB {
		t.Fatalf("the two grids are %d and %d, want two nets", gridA, gridB)
	}
	before := powerView(t, j, hot)
	if !before.HasLink || !before.PowerRelink || before.PowerNet != gridA {
		t.Fatalf("the heater is %+v, want linked to grid %d with another in range", before, gridA)
	}

	wdSelect(j, 16, 2)
	j.update(2)
	btn, ok := bldCmd(j.a.insp.Building().BuildingCommands(hot), inspector.PowerReconnectCmdID)
	if !ok || btn.Disabled {
		t.Fatalf("Reconnect with a grid in range: %+v (found %v)", btn, ok)
	}
	j.reset()
	if !j.clickID(inspector.PowerReconnectCmdID) {
		t.Fatal("Reconnect took no click")
	}
	c, sent := j.lastCmd(command.ReconnectPower)
	if !sent || c.Target != uint64(hot) {
		t.Fatalf("Reconnect sent %+v (%v), want Target %d", c, sent, hot)
	}
	j.update(2)
	j.g.Step(2)
	j.update(2)
	after := powerView(t, j, hot)
	if !after.HasLink || after.PowerNet != gridB || after.LinkX < 22 {
		t.Fatalf("after Reconnect the heater is %+v, want wired to grid %d at x 22", after, gridB)
	}

	wdSelect(j, 11, 6)
	j.update(2)
	if v := powerView(t, j, lone); !v.HasLink || v.PowerRelink {
		t.Fatalf("the lone heater is %+v, want a link and no other grid", v)
	}
	b, ok := bldCmd(j.a.insp.Building().BuildingCommands(lone), inspector.PowerReconnectCmdID)
	if !ok || !b.Disabled || !strings.Contains(b.Reason, "No other grid") {
		t.Fatalf("Reconnect with one grid in range: %+v (found %v)", b, ok)
	}
	onScreen := false
	for _, h := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if h.ID == inspector.PowerReconnectCmdID {
			onScreen = true
			if !h.Disabled {
				t.Fatalf("the Reconnect on screen is live: %+v", h)
			}
		}
	}
	if !onScreen {
		t.Fatal("no Reconnect on screen for the lone heater")
	}
	j.reset()
	if j.clickID(inspector.PowerReconnectCmdID) {
		t.Fatal("the refused Reconnect took a click")
	}
	if _, sent := j.lastCmd(command.ReconnectPower); sent {
		t.Fatal("the refused Reconnect sent its command")
	}
}
