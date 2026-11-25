// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mining

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/geology"
)

func OreMetal(o geology.Ore) item.Kind {
	switch o {
	case geology.Silver:
		return item.Silver
	case geology.Gold:
		return item.Gold
	case geology.Machinery:
		return item.Components // buried machinery (NG-1 §4.8)
	case geology.Plasteel:
		return item.Plasteel
	case geology.Uranium:
		return item.Uranium
	case geology.Jade:
		return item.Jade
	}
	return item.Steel
}
