// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

const InfestationGap = 20 * clock.TicksPerDay

// Weight: a warm, dark site is favored; cold and light reduce the
// chance rather than unconditionally disqualifying an otherwise valid cell.
// Factors are original tuning; the -17/-8 thresholds follow the gap spec.
func InfestationWeight(temp, light, roomCells int) int {
	if temp <= -170 || roomCells == 1 {
		return 0
	}
	weight := max(1, 1000-light)
	if temp < -80 {
		weight = max(1, weight*(temp+170)/90)
	}
	return weight
}

func NearColonyStructure(x, y int, ids []entity.EntityID, get func(entity.EntityID) (construction.Building, bool)) bool {
	for _, id := range ids {
		b, ok := get(id)
		if !ok || b.Ruin || b.Ancient || b.Hostile || construction.DefOf(b.Kind).Natural {
			continue
		}
		dx, dy := b.X-x, b.Y-y
		if dx*dx+dy*dy <= 30*30 {
			return true
		}
	}
	return false
}

// The rare storyteller event uses a fresh live eligibility pass and one
// weighted draw, with no random draws at all when there is no valid site.
func (hv *Hives) infestStrict(now uint64) (int, bool) {
	type candidate struct{ x, y, weight int }
	var cells []candidate
	total := 0
	w, h := hv.h.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if weight := hv.n.InfestationWeight(x, y); weight > 0 && hv.apart(x, y) {
				cells = append(cells, candidate{x, y, weight})
				total += weight
			}
		}
	}
	if total == 0 {
		return 0, false
	}
	pick := hv.h.Roll(total)
	for _, c := range cells {
		if pick < c.weight {
			return hv.raise(c.x, c.y, now)
		}
		pick -= c.weight
	}
	return 0, false
}
