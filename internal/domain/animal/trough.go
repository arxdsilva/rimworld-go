// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import "rimworld-go/internal/domain/economy/item"

const (
	TroughPlants uint8 = iota
	TroughMeat
	TroughAny
)

func TroughFeeds(f uint8, k item.Kind) bool {
	plants := PlantFeed(k) || k == item.Straw
	if k == item.Kibble {
		return f <= TroughAny
	}
	switch f {
	case TroughPlants:
		return plants
	case TroughMeat:
		return k == item.Meat || k == item.Fish
	case TroughAny:
		return plants || k == item.Meat || k == item.Fish
	}
	return false
}
