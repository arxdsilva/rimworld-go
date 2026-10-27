package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
)

// TestCapacityTipPegLeg: the tooltip names the capacity, the implant and
// the result.
func TestCapacityTipPegLeg(t *testing.T) {
	var b body.Body
	b.Missing |= 1 << uint(body.LegL)
	b.SetImplant(body.LegL, item.PegLeg)
	tip := capacityTip(&b, body.Moving)
	t.Log("\n" + tip)
	if !strings.HasPrefix(tip, "Moving\n") || !strings.Contains(tip, "peg leg") || !strings.Contains(tip, "\n= ") {
		t.Fatalf("tip %q", tip)
	}
}
