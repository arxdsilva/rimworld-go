// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package commands

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

// cmd is a command told apart by Pawn, for a target tick and a caller Seq
// the queue must ignore.
func cmd(pawn, tick, seq uint64) command.Command {
	return command.Command{Kind: command.SetWorkPriority, Pawn: pawn, Tick: tick, Seq: seq}
}

type envelope struct{ pawn, tick, seq uint64 }

func envelopes(cs []command.Command) []envelope {
	var out []envelope
	for _, c := range cs {
		out = append(out, envelope{c.Pawn, c.Tick, c.Seq})
	}
	return out
}

func applyAll(command.Command) Route { return Apply }

// TestQueueOrder: Push stamps and orders by (Tick, Seq); Take releases the
// due commands in that order with the envelope cleared, keeps or drops
// them as routed, and leaves the commands not yet due.
func TestQueueOrder(t *testing.T) {
	t.Parallel()
	byPawn := func(m map[uint64]Route) func(command.Command) Route {
		return func(c command.Command) Route { return m[c.Pawn] }
	}
	cases := []struct {
		name   string
		push   []command.Command
		pushAt uint64
		takeAt uint64
		route  func(command.Command) Route
		want   []envelope // returned
		left   []envelope // still queued
	}{
		{
			name: "equal tick in submit order, caller Seq ignored",
			push: []command.Command{cmd(1, 5, 99), cmd(2, 5, 7), cmd(3, 5, 0)}, pushAt: 5, takeAt: 5,
			route: applyAll,
			want:  []envelope{{1, 0, 0}, {2, 0, 0}, {3, 0, 0}},
		},
		{
			name: "unset and past ticks are clamped to now and due",
			push: []command.Command{cmd(1, 0, 0), cmd(2, 3, 0)}, pushAt: 10, takeAt: 10,
			route: applyAll,
			want:  []envelope{{1, 0, 0}, {2, 0, 0}},
		},
		{
			name: "a future tick waits; an earlier tick pushed later goes first",
			push: []command.Command{cmd(1, 20, 0), cmd(2, 12, 0), cmd(3, 0, 0)}, pushAt: 10, takeAt: 12,
			route: applyAll,
			want:  []envelope{{3, 0, 0}, {2, 0, 0}},
			left:  []envelope{{1, 20, 1}},
		},
		{
			name: "keep stays in order, drop is removed, route sees only due commands",
			push: []command.Command{cmd(1, 0, 0), cmd(2, 0, 0), cmd(3, 0, 0), cmd(4, 0, 0), cmd(5, 9, 0)}, pushAt: 4, takeAt: 4,
			route: byPawn(map[uint64]Route{1: Keep, 2: Apply, 3: Drop, 4: Keep}),
			want:  []envelope{{2, 0, 0}},
			left:  []envelope{{1, 4, 1}, {4, 4, 4}, {5, 9, 5}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var q Queue
			for _, c := range tc.push {
				q.Push(c, tc.pushAt)
			}
			route := func(c command.Command) Route {
				if c.Tick > tc.takeAt {
					t.Errorf("route called for a command not due: %+v", c)
				}
				return tc.route(c)
			}
			got := q.Take(tc.takeAt, route)
			if !slices.Equal(envelopes(got), tc.want) {
				t.Errorf("Take = %v, want %v", envelopes(got), tc.want)
			}
			if !slices.Equal(envelopes(q.Items()), tc.left) {
				t.Errorf("left %v, want %v", envelopes(q.Items()), tc.left)
			}
			if q.Len() != len(tc.left) || q.Last() != uint64(len(tc.push)) {
				t.Errorf("Len %d Last %d", q.Len(), q.Last())
			}
		})
	}
}

// TestQueuePushNoneDropped: a command of Kind None is never queued and
// takes no sequence number.
func TestQueuePushNoneDropped(t *testing.T) {
	t.Parallel()
	var q Queue
	q.Push(command.Command{}, 3)
	if q.Len() != 0 || q.Last() != 0 {
		t.Fatalf("Len %d Last %d", q.Len(), q.Last())
	}
}

// TestQueueTakeResultSurvivesPush: the slice Take returns does not alias
// the queue, so pushes made while applying it change nothing in it, and
// the pushed command is due at the next Take.
func TestQueueTakeResultSurvivesPush(t *testing.T) {
	t.Parallel()
	var q Queue
	q.Push(cmd(1, 0, 0), 2)
	q.Push(cmd(2, 0, 0), 2)
	got := q.Take(2, applyAll)
	q.Push(cmd(3, 0, 0), 2)
	q.Push(cmd(4, 0, 0), 2)
	if !slices.Equal(envelopes(got), []envelope{{1, 0, 0}, {2, 0, 0}}) {
		t.Fatalf("result changed by a push: %v", envelopes(got))
	}
	if next := envelopes(q.Take(2, applyAll)); !slices.Equal(next, []envelope{{3, 0, 0}, {4, 0, 0}}) {
		t.Fatalf("next Take %v", next)
	}
}

// TestQueueRestoreOldSave: saved items keep their slice order; items with
// Seq 0 (saves from before #8) are stamped after the saved last in slice
// order, and Last is the largest Seq. An old save's (0, 0) commands are due
// at the next Take in their saved order.
func TestQueueRestoreOldSave(t *testing.T) {
	t.Parallel()
	t.Run("old save", func(t *testing.T) {
		t.Parallel()
		var q Queue
		q.Restore([]command.Command{cmd(7, 0, 0), cmd(8, 0, 0), cmd(9, 0, 0)}, 0)
		if want := []envelope{{7, 0, 1}, {8, 0, 2}, {9, 0, 3}}; !slices.Equal(envelopes(q.Items()), want) || q.Last() != 3 {
			t.Fatalf("restored %v last %d", envelopes(q.Items()), q.Last())
		}
		if got := envelopes(q.Take(100, applyAll)); !slices.Equal(got, []envelope{{7, 0, 0}, {8, 0, 0}, {9, 0, 0}}) {
			t.Fatalf("Take %v", got)
		}
		q.Push(cmd(10, 0, 0), 100)
		if q.Items()[0].Seq != 4 {
			t.Fatalf("next push stamped %d, want 4", q.Items()[0].Seq)
		}
	})
	t.Run("new save", func(t *testing.T) {
		t.Parallel()
		var q Queue
		q.Restore([]command.Command{cmd(1, 5, 4), cmd(2, 9, 6)}, 3)
		if want := []envelope{{1, 5, 4}, {2, 9, 6}}; !slices.Equal(envelopes(q.Items()), want) || q.Last() != 6 {
			t.Fatalf("restored %v last %d, want last 6 (the largest Seq)", envelopes(q.Items()), q.Last())
		}
		q.Restore(nil, 11)
		if q.Len() != 0 || q.Last() != 11 {
			t.Fatalf("empty restore: Len %d Last %d", q.Len(), q.Last())
		}
	})
}

// TestQueueEmptyTakeAllocs: the command phase runs every map pass, so an
// empty queue costs no allocation and never calls route; nor does a queue
// whose commands are all in the future.
func TestQueueEmptyTakeAllocs(t *testing.T) {
	var q Queue
	called := false
	route := func(command.Command) Route { called = true; return Apply }
	if n := testing.AllocsPerRun(100, func() { _ = q.Take(1, route) }); n != 0 || called {
		t.Fatalf("empty Take: %v allocs, route called %v", n, called)
	}
	if q.Take(1, route) != nil {
		t.Fatal("empty Take returns nil")
	}
	q.Push(cmd(1, 50, 0), 1)
	if n := testing.AllocsPerRun(100, func() { _ = q.Take(1, route) }); n != 0 || called || q.Len() != 1 {
		t.Fatalf("future-only Take: %v allocs, route called %v, Len %d", n, called, q.Len())
	}
}
