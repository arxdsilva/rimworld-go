package game

// Cleaning (SIM-021): colonists with cleaning work sweep dirt and stains
// off Home-area tiles, nearest first. A tile is queued at filth.CleanStart
// and cleaned to 0, so a half-cleaned tile is finished later rather than
// dropped (hysteresis).

// wireCleaning switches cleaning on, at the default priority, for every
// colonist of a save made before cleaning existed.
func (g *Game) wireCleaning() {} // SIM-021 stub
