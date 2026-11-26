// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package pathing

func RouteCost(q Query, px, py int, p []Point) int {
	c, _ := q.(Coster)
	cost := 0
	for _, s := range p {
		cost += Octile(px, py, s.X, s.Y)
		if c != nil {
			cost += max(c.ExtraCost(s.X, s.Y), 0)
		}
		px, py = s.X, s.Y
	}
	return cost
}
