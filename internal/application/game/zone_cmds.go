package game

// PLAY-020 zone management (contract stubs: ZONESIM implements). Every
// handler ignores an unknown zone id.

// cmdSetZoneSow allows (on) or stops sowing in zone id.
func (g *Game) cmdSetZoneSow(id uint16, on bool) {}

// cmdSetZoneCut allows (on) or stops growers cutting in zone id.
func (g *Game) cmdSetZoneCut(id uint16, on bool) {}

// cmdSetZoneHidden hides (on) or shows zone id's overlay.
func (g *Game) cmdSetZoneHidden(id uint16, on bool) {}

// cmdRenameZone names zone id ("" = the default name).
func (g *Game) cmdRenameZone(id uint16, name string) {}

// cmdSetZoneCrop makes grow zone id grow species.
func (g *Game) cmdSetZoneCrop(id uint16, species uint8) {}

// cmdExpandZone adds the valid cells of x0..x1, y0..y1 to zone id.
func (g *Game) cmdExpandZone(id uint16, x0, y0, x1, y1 int) {}

// cmdRemoveZoneCells removes zone id's cells in x0..x1, y0..y1.
func (g *Game) cmdRemoveZoneCells(id uint16, x0, y0, x1, y1 int) {}

// cmdDeleteZone removes zone id.
func (g *Game) cmdDeleteZone(id uint16) {}
