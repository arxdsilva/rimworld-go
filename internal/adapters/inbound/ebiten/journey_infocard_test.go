package ebiten

// journey_infocard_test.go: the read-only Info window (GEAR-006), opened
// through the shell as the Gear tab's Info button does. It opens over the
// map without taking the selection or the inspector, shows the item, and
// both Esc and the Close button shut it.

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/infocard"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

func TestJourneyInfoCardOpensAndEscapeCloses(t *testing.T) {
	j := newJourney(t)
	pid := j.a.shell.Pawns()[0].ID
	j.a.renderer.Select(pid)
	j.update(2)

	j.a.shell.Do(ports.OpenInfoCard{Kind: "flak vest"})
	j.update(2)
	j.draw(2)
	if j.a.uiModalTop() != infocard.ID {
		t.Fatalf("top modal %q, want the Info window", j.a.uiModalTop())
	}
	if id, ok := j.a.shell.Selected(); !ok || id != pid {
		t.Fatalf("opening the card dropped the selection: %v %v", id, ok)
	}
	if !j.a.insp.PaneShown() {
		t.Fatal("opening the card closed the inspector")
	}

	j.key("Escape", 0)
	j.update(1)
	if j.a.uiModalTop() == infocard.ID {
		t.Fatal("Escape left the card open")
	}
	if id, ok := j.a.shell.Selected(); !ok || id != pid {
		t.Fatal("Escape that closed the card also cleared the selection")
	}
}

func TestJourneyInfoCardCloseButton(t *testing.T) {
	j := newJourney(t)
	j.a.shell.Do(ports.OpenInfoCard{Kind: "rifle"})
	j.update(2)
	if j.a.uiModalTop() != infocard.ID {
		t.Fatal("the card did not open")
	}
	if !j.clickID("modal." + infocard.ID + "." + infocard.CloseID) {
		t.Fatal("no Close button")
	}
	j.update(1)
	if j.a.uiModalTop() == infocard.ID {
		t.Fatal("Close left the card open")
	}
}

// TestJourneyInfoCardForAMapItemThatIsGone: the card for a stack that has gone says so.
func TestJourneyInfoCardForAMapItemThatIsGone(t *testing.T) {
	j := newJourney(t)
	j.a.shell.Do(ports.OpenInfoCard{ID: 999999})
	j.update(2)
	j.draw(2)
	if j.a.uiModalTop() != infocard.ID {
		t.Fatal("a vanished item still opens a card that says so")
	}
}
