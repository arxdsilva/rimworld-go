// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package spatial

import "rimworld-go/internal/domain/sim/entity"

// OccupancyBatch records which blocking layers changed across a rebuild.
// The per-cell bits keep the old layer presence and also mark visited cells.
type OccupancyBatch struct {
	previous []uint8
	touched  []int
	active   bool
}

// Resize discards stale positions when the map size changes.
func (b *OccupancyBatch) Resize(n int) {
	b.previous = make([]uint8, n)
	b.touched = b.touched[:0]
	b.active = false
}

// Begin snapshots occupied cells before both layers are cleared.
func (b *OccupancyBatch) Begin(marked []int, blockers, partners []entity.EntityID) {
	b.touched = b.touched[:0]
	for _, i := range marked {
		if b.previous[i] != 0 {
			continue
		}
		was := uint8(4)
		if blockers[i] != 0 {
			was |= 1
		}
		if partners[i] != 0 {
			was |= 2
		}
		b.previous[i] = was
		b.touched = append(b.touched, i)
	}
	b.active = true
}

// End applies only net changes in the two occupancy layers.
func (b *OccupancyBatch) End(marked []int, blockers, partners []entity.EntityID, near, pnear []uint8, w, h int) {
	for _, i := range marked {
		if b.previous[i] == 0 {
			b.previous[i] = 4
			b.touched = append(b.touched, i)
		}
	}
	b.active = false
	for _, i := range b.touched {
		was := b.previous[i]
		if (was&1 != 0) != (blockers[i] != 0) {
			BumpNear(near, w, h, i, was&1 != 0)
		}
		if (was&2 != 0) != (partners[i] != 0) {
			BumpNear(pnear, w, h, i, was&2 != 0)
		}
		b.previous[i] = 0
	}
}

func (b *OccupancyBatch) Active() bool { return b.active }

// BumpNear updates the 3x3 neighbor count when a cell changes occupancy.
func BumpNear(near []uint8, w, h, i int, down bool) {
	var delta uint8 = 1
	if down {
		delta = 255
	}
	x, y := i%w, i/w
	for ny := max(y-1, 0); ny <= min(y+1, h-1); ny++ {
		for nx := max(x-1, 0); nx <= min(x+1, w-1); nx++ {
			near[ny*w+nx] += delta
		}
	}
}
