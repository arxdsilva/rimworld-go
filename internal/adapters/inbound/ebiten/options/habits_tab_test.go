package options

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/command"
)

// habitFake is a fakeShell that records the commands the Habits tab sends
// and serves the colony's rules.
type habitFake struct {
	*fakeShell
	rules gameplay.Settings
	sent  []command.Command
}

func (h *habitFake) Rules() gameplay.Settings { return h.rules }
func (h *habitFake) Send(c command.Command)   { h.sent = append(h.sent, c) }

func TestHabitsRowsCoverEveryToggle(t *testing.T) {
	t.Parallel()
	seen := map[gameplay.Toggle]int{}
	limits := map[gameplay.Limit]int{}
	for _, g := range habitGroups() {
		if g.Title == "" {
			t.Errorf("a group has no heading")
		}
		for _, r := range g.Toggles {
			seen[r.T]++
			if r.Label == "" || r.Tip == "" || strings.Contains(r.Tip, "\n") {
				t.Errorf("toggle %d: label %q and tip %q must both be set, the tip on one line", r.T, r.Label, r.Tip)
			}
		}
		for _, r := range g.Limits {
			limits[r.L]++
			if r.Label == "" || r.Tip == "" {
				t.Errorf("limit %d has no label or tip", r.L)
			}
		}
	}
	for _, tg := range gameplay.AllToggles() {
		if seen[tg] != 1 {
			t.Errorf("toggle %d is on %d rows, want 1", tg, seen[tg])
		}
	}
	for l := gameplay.Limit(0); l < gameplay.NumLimits; l++ {
		if limits[l] != 1 {
			t.Errorf("limit %d is on %d rows, want 1", l, limits[l])
		}
	}
}

func TestHabitsTabIsListedInAndOutOfAColony(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	has := func() bool {
		for _, tab := range s.TabList() {
			if tab == "habits" {
				return true
			}
		}
		return false
	}
	if !has() {
		t.Error("no habits tab outside a colony: it should be there, saying why it is off")
	}
	f.env.Running = true
	if !has() {
		t.Error("no habits tab in a colony")
	}
	s.Tab = "habits"
	if got := s.CurrentTab(); got != "habits" {
		t.Errorf("tab %q, want habits", got)
	}
}

func TestUnlimitedReadsAsUnlimited(t *testing.T) {
	t.Parallel()
	if got := HabitLimitText(0, "Unlimited"); got != "Unlimited" {
		t.Errorf("0 reads %q", got)
	}
	if got := HabitLimitText(0, ""); got != "0" {
		t.Errorf("a plain 0 reads %q", got)
	}
	if got := HabitLimitText(7, "Unlimited"); got != "7" {
		t.Errorf("7 reads %q", got)
	}
}

func TestBackToDefaultsSendsOnlyWhatDiffers(t *testing.T) {
	t.Parallel()
	h := &habitFake{fakeShell: &fakeShell{}}
	s := New(h)
	h.rules = gameplay.Settings{}.With(gameplay.OutdoorsJoy, false).With(gameplay.PoliteWander, true).WithLimit(gameplay.TidyBench, 12)
	if s.habitsAreDefault(h.rules) {
		t.Fatal("changed rules read as default")
	}
	s.habitsReset(h.rules)
	if len(h.sent) != 3 {
		t.Fatalf("sent %d commands %+v, want 3", len(h.sent), h.sent)
	}
	// Applied on top of the changed rules the commands give the defaults.
	end := h.rules
	for _, c := range h.sent {
		if c.Kind == command.SetGameplayToggle {
			end = end.With(gameplay.Toggle(c.Index), c.Enabled)
		} else {
			end = end.WithLimit(gameplay.Limit(c.Index), c.Count)
		}
	}
	if !s.habitsAreDefault(end) {
		t.Errorf("after the commands the rules are %+v, want the defaults", end)
	}
}

func TestLimitCommandsAreClamped(t *testing.T) {
	t.Parallel()
	h := &habitFake{fakeShell: &fakeShell{}}
	s := New(h)
	s.setLimit(gameplay.TidyStation, 500)
	s.setLimit(gameplay.TidyStation, -3)
	if h.sent[0].Count != gameplay.MaxLimit || h.sent[1].Count != 0 {
		t.Errorf("counts %d, %d, want %d and 0", h.sent[0].Count, h.sent[1].Count, gameplay.MaxLimit)
	}
}

// TestHabitsTabHasNoLegacyRows: the tab's widgets are its own; a row here
// would paint another tab's controls over them.
func TestHabitsTabHasNoLegacyRows(t *testing.T) {
	t.Parallel()
	if rows := OptionRows("habits"); rows != nil {
		t.Fatalf("OptionRows(habits) = %d rows, want none", len(rows))
	}
}
