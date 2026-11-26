// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package visit

import (
	"rimworld-go/internal/application/game/leave"
	"rimworld-go/internal/domain/ai/job"
)

// A visitor tries every map edge before breaking a wall. The search runs
// only when its previous move failed or its retry expires.
// escape runs the normal walk-out job, or the same building-damage job
// used by raiders when every walkable exit is cut off.
func (v Visits) escape(r *Rec, x, y int, j job.Record, now uint64) bool {
	h := v.h
	w, ht := h.Size()
	gr, member := v.GroupOf(r.ID)
	traveler := member && gr.Kind == KindTraveler
	atExit := min(x, y, w-1-x, ht-1-y) <= Inset
	if traveler {
		atExit = x == gr.ExitX && y == gr.ExitY
	}
	if atExit || !traveler && j.Kind == job.Move && j.State == job.Complete {
		v.goHome(r.ID)
		return false
	}
	if j.Kind == job.Breach && j.Active() {
		h.Breach(r.ID, j)
		return true
	}
	if j.Active() || now < r.RetryAt {
		return true
	}

	if traveler {
		if h.StartJob(r.ID, job.Record{Kind: job.Move, TX: gr.ExitX, TY: gr.ExitY}) ||
			h.StartBreach(r.ID, gr.ExitX, gr.ExitY) {
			return true
		}
		r.RetryAt = now + retry
		return true
	}
	if leave.Try(h, r.ID, x, y, Inset, j) {
		return true
	}
	if now >= r.LeaveAt+giveUp {
		if leave.BreachRoute(h, r.ID, x, y, Inset) {
			r.RetryAt = now + retry
			return true
		}
		v.goHome(r.ID)
		return false
	}
	r.RetryAt = now + retry
	return true
}
