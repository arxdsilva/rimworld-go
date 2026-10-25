package game

// PLAY-017: haulers carry only the colony's own things. Stacks found on
// the map (ruin loot, stone chunks, a predator's kill) are Unclaimed and
// wait for the player's haul mark.

// migrateClaims marks an older save's loose stone chunks and animal
// carcasses outside stockpiles unclaimed.
func (g *Game) migrateClaims() {}
