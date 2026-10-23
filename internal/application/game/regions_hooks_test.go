package game

import "os"

// SetRegionsOff switches the region answers off (plain searches), for the
// external equivalence test on the grown fixture.
func (g *Game) SetRegionsOff(off bool) { g.regs.off = off }

// SetExactLongPaths turns corridor searches and the long-search budget
// off (the region answers stay on).
func (g *Game) SetExactLongPaths(on bool) { g.regs.exactLong = on }

func init() {
	if os.Getenv("RIMWORLD_REGIONS_OFF") != "" {
		regionsOffAll = true
	}
}
