package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestInspectorShowsWaitingFor(t *testing.T) {
	names := map[entity.EntityID]string{7: "Dustbuck"}
	lookup := func(id entity.EntityID) string { return names[id] }
	if got := waitingForLine(7, lookup); got != "waiting for Dustbuck" {
		t.Fatalf("got %q", got)
	}
	if got := waitingForLine(0, lookup); got != "" {
		t.Fatalf("unblocked should show nothing, got %q", got)
	}
	if got := waitingForLine(9, lookup); got != "waiting for something" {
		t.Fatalf("unknown blocker got %q", got)
	}
}
