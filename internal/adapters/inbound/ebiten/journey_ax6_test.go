package ebiten

// journey_ax6_test.go (AX-6): recreation through the real UI loop: every
// buildable recreation card lays its blueprint, the found-only big screen
// and telescope cards are locked, and a colonist with free time plays at
// a chess table by itself while the table's pane shows its joy kind and
// user and the colonist's Needs tab rows show what it enjoys.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/domain/sim/command"
)

// TestJourneyAX6RecreationCards: ring toss, ur board, chess, billiards
// and poker tables and both buildable televisions are laid from the
// Recreation category; the big screen and telescope cards are locked
// (found in ruins, never built) and send nothing.
func TestJourneyAX6RecreationCards(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(40, 32), jPawns([2]int{8, 4}))
	j.g.CompleteResearch() // chess, billiards, poker: Upholstery; televisions: their projects
	for _, c := range []struct {
		tool, kind string
		x, y       int
	}{
		{"ring_toss", "ring toss", 12, 8}, {"ur_board", "ur board", 15, 8}, {"chess_table", "chess table", 18, 8},
		{"billiards_table", "billiards table", 22, 8}, {"poker_table", "poker table", 27, 8},
		{"tube_tv", "tube television", 12, 16}, {"flatscreen_tv", "flatscreen television", 18, 16},
	} {
		ax4Lay(t, j, "recreation", c.tool, c.kind, 0, c.x, c.y)
	}
	j.reset()
	j.openArch("recreation", "")
	for _, id := range []string{"big_screen_tv", "telescope"} {
		if j.clickID("arch.tool." + id) {
			t.Fatalf("the %s card can be armed", id)
		}
	}
	if len(j.cmdsOf(command.DesignateBuild)) != 0 {
		t.Fatal("a found-only card sent a blueprint")
	}
}

// TestJourneyAX6PlaysChess: a colonist with free time walks to the chess
// table and plays; selecting the table shows "Recreation  cerebral" and
// its user, and the colonist's Needs tab rows say it enjoys cerebral fun
// and, after the game, its cerebral tolerance.
func TestJourneyAX6PlaysChess(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(30, 24), jPawns([2]int{8, 8}))
	wdBuild(j, "chess table", 14, 8)
	pid := j.g.Snapshot()[0].ID
	j.key("Digit3", 0)
	j.until(func() bool {
		return j.pawn(uint64(pid)).AX.Joy == "cerebral" && j.pawn(uint64(pid)).Acting == "play chess"
	}, 30000)
	j.key("Space", 0)
	id := wdSelect(j, 14, 8)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "recreation  cerebral") || !strings.Contains(lines, "in use  1 colonist") {
		t.Fatalf("chess table pane %q", lines)
	}
	rows := strings.Join(inspector.JoyRows(j.pawn(uint64(pid))), "\n")
	if !strings.Contains(rows, "Enjoying  cerebral") {
		t.Fatalf("needs tab rows %q, want what it enjoys", rows)
	}
	j.key("Digit3", 0)
	j.until(func() bool { return len(j.pawn(uint64(pid)).AX.Tolerance) > 0 }, 3000)
	if rows := strings.Join(inspector.JoyRows(j.pawn(uint64(pid))), "\n"); !strings.Contains(rows, "Tolerance  cerebral") {
		t.Fatalf("needs tab rows %q, want its cerebral tolerance", rows)
	}
}
