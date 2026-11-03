package ebiten

// journey_wanderers_test.go: new wanderers after a game over (MAP-19) on a
// Setup game: the button is off with a reason until eight hours have
// passed, the alert counts down and reopens the screen, the picker rerolls
// and Start sends NewWanderers.

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// goneAlert is the "Nobody left" alert, if it is up.
func goneAlert(j *journey) (hud.Alert, bool) {
	for _, al := range j.a.chrome.Alerts().Current() {
		if al.Key == hud.GoneAlertKey {
			return al, true
		}
	}
	return hud.Alert{}, false
}

// TestJourneyWanderersNewGroup: the whole path, from the game over to the
// colonists walking in.
func TestJourneyWanderersNewGroup(t *testing.T) {
	t.Parallel()
	j, _ := worldColony(t)
	abandonColony(j)
	if j.a.uiModalTop() != hud.GoneModalID {
		t.Fatalf("top %q, want the everyone-gone screen", j.a.uiModalTop())
	}
	j.draw(2)
	btn := goneID(hud.GoneWanderersID)
	why, off := gizmoWhy(j, btn)
	if !off || !strings.HasPrefix(why, "In ") {
		t.Fatalf("New wanderers: disabled %v, reason %q, want a countdown", off, why)
	}
	if j.clickID(btn) {
		t.Fatal("a disabled New wanderers took the click")
	}

	// Keep watching: the alert is the way back.
	j.clickID(goneID(hud.GoneWatchID))
	j.update(2)
	al, ok := goneAlert(j)
	if !ok || !strings.Contains(al.Label, "Nobody left") || !strings.Contains(al.Label, "in ") || al.Explain == "" {
		t.Fatalf("alert %+v ok %v", al, ok)
	}
	if !j.clickID("alert."+hud.GoneAlertKey) || j.a.uiModalTop() != hud.GoneModalID {
		t.Fatalf("the alert opened %q, want the everyone-gone screen", j.a.uiModalTop())
	}
	j.clickID(goneID(hud.GoneWatchID))

	// Eight hours on: the option opens.
	j.g.Step(8*clock.TicksPerHour + 60)
	j.update(2)
	if !j.world().AX.WanderersOK {
		t.Fatalf("not open after eight hours: %+v", j.world().AX)
	}
	if !j.clickID("alert." + hud.GoneAlertKey) {
		t.Fatal("the alert refused the click")
	}
	j.draw(2)
	if why, off := gizmoWhy(j, btn); off {
		t.Fatalf("New wanderers still off: %q", why)
	}
	if !j.clickID(btn) || j.a.uiModalTop() != dialogs.WanderersID {
		t.Fatalf("New wanderers opened %q, want the picker", j.a.uiModalTop())
	}
	j.draw(2)

	// Pick: reroll the second person (or the first on a group of one).
	slot := min(1, j.world().AX.WanderersMax-1)
	before := j.a.deal.WandererSeeds()
	if !j.clickID(dialogs.WandererRowID(slot)) || !j.clickID(dialogs.WandererRerollID(slot)) {
		t.Fatal("the slot's buttons are off")
	}
	after := j.a.deal.WandererSeeds()
	if len(before) == 0 || before[slot] == after[slot] {
		t.Fatalf("reroll kept the person: %v -> %v", before, after)
	}
	j.draw(2)
	j.reset()
	if !j.clickID("modal." + dialogs.WanderersID + "." + dialogs.WanderersStartID) {
		t.Fatal("Start is off")
	}
	c, ok := j.lastCmd(command.NewWanderers)
	if !ok {
		t.Fatal("NewWanderers was not sent")
	}
	var want []string
	for _, s := range after {
		want = append(want, strconv.FormatUint(s, 10))
	}
	if c.Name != strings.Join(want, ",") {
		t.Fatalf("NewWanderers seeds %q, want %q", c.Name, strings.Join(want, ","))
	}
	if j.a.uiModalTop() == dialogs.WanderersID {
		t.Fatal("the picker stays open after Start")
	}
	j.g.Step(60)
	j.update(2)
	if len(j.g.Snapshot()) == 0 || j.world().AX.EveryoneGone {
		t.Fatalf("nobody came: gone %v, pawns %d", j.world().AX.EveryoneGone, len(j.g.Snapshot()))
	}
	if _, ok := goneAlert(j); ok {
		t.Fatal("the Nobody left alert stays up with colonists back")
	}
}

// TestJourneyWanderersCancel: Cancel closes the picker without a command.
func TestJourneyWanderersCancel(t *testing.T) {
	t.Parallel()
	j, _ := worldColony(t)
	abandonColony(j)
	j.g.Step(8*clock.TicksPerHour + 60)
	j.update(2)
	j.draw(2)
	if !j.clickID(goneID(hud.GoneWanderersID)) || j.a.uiModalTop() != dialogs.WanderersID {
		t.Fatalf("picker not open: %q", j.a.uiModalTop())
	}
	j.draw(2)
	j.reset()
	if !j.clickID("modal." + dialogs.WanderersID + "." + dialogs.WanderersCancelID) {
		t.Fatal("Cancel is off")
	}
	if _, ok := j.lastCmd(command.NewWanderers); ok || j.a.uiModalTop() == dialogs.WanderersID {
		t.Fatal("Cancel sent a command or left the picker open")
	}
}
