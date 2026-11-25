// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// ExitGround provides a nearby exit and the fallback reachable map border.
type ExitGround interface {
	WalkableNear(x, y, radius int) (int, int, bool)
	Edge() (int, int, bool)
}

// TryExit starts a caravan departure at the nearest available tile, then
// a reachable map border. Deferred searches keep their original pending
// exit so the simulation's path budget is respected.
func TryExit(ground ExitGround, id entity.EntityID, ex, ey int,
	deferred func() bool, start func(entity.EntityID, job.Record) bool,
) bool {
	if x, y, ok := ground.WalkableNear(ex, ey, 3); ok {
		if start(id, job.Record{Kind: job.ExitMap, TX: x, TY: y}) {
			return true
		}
		if deferred() {
			return false
		}
	}
	x, y, ok := ground.Edge()
	return ok && start(id, job.Record{Kind: job.ExitMap, TX: x, TY: y})
}
