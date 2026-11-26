// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package orders

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// assignCasket starts the closest marked sleeper casket's opening job.
func (o *Orders) assignCasket(id entity.EntityID) bool {
	h := o.h
	px, py := h.Pos(id)
	var tried []entity.EntityID
	for range 8 {
		var best entity.EntityID
		bestDist := 1 << 30
		for _, bid := range h.BuildingIDs() {
			b, ok := h.Building(bid)
			if !ok || b.Kind != construction.SleeperCasket || !b.Deconstruct ||
				!h.Sealed(bid) || !h.Free(id, bid, job.Deconstruct) ||
				slices.Contains(tried, bid) {
				continue
			}
			if d := dist(px, py, b.X, b.Y); d < bestDist || (d == bestDist && bid < best) {
				best, bestDist = bid, d
			}
		}
		if best == 0 {
			break
		}
		b, _ := h.Building(best)
		if tx, ty, ok := h.Spot(id, b.X, b.Y); ok && h.StartJob(id, job.Record{
			Kind: job.Deconstruct, Target: best, TX: tx, TY: ty, WorkLeft: b.WorkLeft,
		}) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}
