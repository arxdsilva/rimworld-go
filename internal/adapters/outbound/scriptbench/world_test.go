// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import "rimworld-go/internal/application/scripting"

// colony is a fake home map of n colonists, ids 1..n, with deterministic
// fields. Pawns returns a slice built once, as the game's reader does.
type colony struct {
	n  int
	ps []scripting.Pawn
}

var (
	_ scripting.World      = (*colony)(nil)
	_ scripting.PawnReader = (*colony)(nil)
)

func newColony(n int) *colony {
	c := &colony{n: n, ps: make([]scripting.Pawn, n)}
	for i := range c.ps {
		id := uint64(i + 1)
		c.ps[i] = scripting.Pawn{
			ID: id, Name: "colonist", X: i % 100, Y: i / 100,
			Food: int(id * 37 % 101), Rest: int(id * 53 % 101), Mood: int(id * 17 % 101),
			Downed: id%11 == 0, Drafted: id%7 == 0,
		}
	}
	return c
}

func (c *colony) Day() int                     { return 4 }
func (c *colony) Hour() int                    { return 6 }
func (c *colony) Season() string               { return "summer" }
func (c *colony) Colonists() int               { return c.n }
func (c *colony) Wealth() int                  { return 1234 }
func (c *colony) ItemCount(string) (int, bool) { return 0, false }
func (c *colony) Pawns() []scripting.Pawn      { return c.ps }
func (c *colony) Pawn(id uint64) (scripting.Pawn, bool) {
	if id < 1 || id > uint64(c.n) {
		return scripting.Pawn{}, false
	}
	return c.ps[id-1], true
}
