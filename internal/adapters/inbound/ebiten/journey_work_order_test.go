package ebiten

// journey_work_order_test.go: the Work tab draws its columns in work.Order,
// the order a colonist breaks ties in (WRK-002).

import (
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	windowspkg "rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/command"
)

// TestJourneyWorkColumnsFollowOrder: on screen Hauling sits right of
// Growing, and clicking a colonist's Haul cell sends the Haul priority.
func TestJourneyWorkColumnsFollowOrder(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := uint64(j.g.Snapshot()[0].ID)
	j.key("F1", 0)
	j.update(1)
	cellID := func(ty work.Type) string {
		return ids.PrefixWorkCell + strconv.FormatUint(id, 10) + "." + strconv.Itoa(int(ty))
	}
	xOf := func(ty work.Type) float32 {
		for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
			if b.ID == cellID(ty) {
				return b.X
			}
		}
		t.Fatalf("no cell %s on the Work tab", cellID(ty))
		return 0
	}
	if xOf(work.Haul) <= xOf(work.Grow) || xOf(work.Research) <= xOf(work.Clean) || xOf(work.Firefight) >= xOf(work.Cook) {
		t.Fatalf("Haul %v Grow %v Research %v Clean %v", xOf(work.Haul), xOf(work.Grow), xOf(work.Research), xOf(work.Clean))
	}
	want := windowspkg.NextPriority(windowspkg.WorkPriority(j.pawn(id), int(work.Haul)))
	if !j.clickID(cellID(work.Haul)) {
		t.Fatalf("cell %s is disabled", cellID(work.Haul))
	}
	c, ok := j.lastCmd(command.SetWorkPriority)
	if !ok || c.Pawn != id || c.Work != command.WorkHaul || c.Count != want {
		t.Fatalf("SetWorkPriority %+v (sent %v), want hauling priority %d", c, ok, want)
	}
}
