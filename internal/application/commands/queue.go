// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package commands

import (
	"slices"

	"rimworld-go/internal/domain/sim/command"
)

// Route is what Take does with a due command.
type Route uint8

const (
	// Keep leaves the due command queued, in order, for a later Take.
	Keep Route = iota
	// Apply removes the command and returns it to the caller to apply.
	Apply
	// Drop removes the command without returning it.
	Drop
)

// Queue holds pending commands sorted by (Tick, Seq). The zero value is an
// empty queue ready to use. It is not safe for concurrent use.
type Queue struct {
	items []command.Command
	last  uint64
}

// Push submits c at tick now. A command of Kind None is dropped. Otherwise
// its Tick becomes max(c.Tick, now), so an unset or past tick is due at the
// next Take, and its Seq becomes the next sequence number (the caller's Seq
// is ignored). c is inserted in (Tick, Seq) order by a scan from the tail, so
// an in-order push is an append and no sort runs.
func (q *Queue) Push(c command.Command, now uint64) {
	if c.Kind == command.None {
		return
	}
	q.last++
	c.Tick, c.Seq = max(c.Tick, now), q.last
	i := len(q.items)
	for i > 0 && q.items[i-1].Tick > c.Tick { // c's Seq is the largest
		i--
	}
	q.items = slices.Insert(q.items, i, c)
}

// Take walks the queue in (Tick, Seq) order and calls route for each due
// command (Tick <= now) only. Apply commands are removed and returned in
// order with their envelope cleared (Tick = Seq = 0); Drop commands are
// removed; Keep commands and commands not yet due stay queued in order. The
// returned slice does not alias the queue, so it stays valid while the
// caller pushes. On an empty queue Take returns nil at once without
// allocating and without calling route; callers may also check Len first.
func (q *Queue) Take(now uint64, route func(command.Command) Route) []command.Command {
	if len(q.items) == 0 {
		return nil
	}
	var out []command.Command
	kept := q.items[:0] // compacts in place: kept never passes the read index
	for i, c := range q.items {
		if c.Tick > now { // sorted: the rest are not due either
			kept = append(kept, q.items[i:]...)
			break
		}
		switch route(c) {
		case Apply:
			c.Tick, c.Seq = 0, 0
			out = append(out, c)
		case Keep:
			kept = append(kept, c)
		}
	}
	clear(q.items[len(kept):]) // drop references held by removed payloads
	q.items = kept
	return out
}

// Items returns the queue's backing slice, not a copy, in (Tick, Seq)
// order, for saving and load-time fixups. A caller may edit the payload
// fields of the elements in place and the queue sees the edits; it must not
// change Tick or Seq, reorder, append to or keep the slice across a Push,
// Take or Restore.
func (q *Queue) Items() []command.Command { return q.items }

// Len reports the number of queued commands.
func (q *Queue) Len() int { return len(q.items) }

// Last reports the last sequence number issued (0 before the first Push).
func (q *Queue) Last() uint64 { return q.last }

// Restore replaces the queue with saved items, kept in slice order, and the
// saved last sequence number. Items with Seq 0 (saves from before #8) are
// stamped last+1, last+2, … in slice order, and Last becomes the maximum of
// the saved last and every item's Seq.
func (q *Queue) Restore(items []command.Command, last uint64) {
	q.items, q.last = items, last
	for _, c := range items {
		q.last = max(q.last, c.Seq)
	}
	for i := range items {
		if items[i].Seq == 0 {
			q.last++
			items[i].Seq = q.last
		}
	}
}
