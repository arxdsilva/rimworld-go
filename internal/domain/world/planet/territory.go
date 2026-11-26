// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package planet

// Territories returns a detached faction ID per land tile, or -1 when
// unclaimed. A settlement's nearest overland hop region belongs to its
// faction; ties use the stable settlement order. Water and unseeded
// islands stay unclaimed. This original territorial rule is derived
// from current bases, not another independently saved owner.
func Territories(p *Planet) []int {
	if p == nil {
		return nil
	}
	owner := make([]int, p.TileCount())
	for i := range owner {
		owner[i] = -1
	}
	queue := make([]int32, 0, len(owner))
	for _, tile := range p.SettlementTiles() {
		f, _ := p.SettlementFaction(int(tile))
		if f < 0 || p.Factions[f].Defeated || !p.Land(int(tile)) {
			continue
		}
		owner[tile] = p.Factions[f].ID
		queue = append(queue, tile)
	}
	for head := 0; head < len(queue); head++ {
		t := queue[head]
		for _, nb := range p.Neighbours(int(t)) {
			if owner[nb] < 0 && p.Land(int(nb)) {
				owner[nb] = owner[t]
				queue = append(queue, nb)
			}
		}
	}
	return owner
}
