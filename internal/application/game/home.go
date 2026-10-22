package game

// The Home area (SIM-021): tiles the colony keeps clean. Painted with
// DesignateHome; grows around every new building; a new colony starts
// with one around its landing spot.

// homeState is the Home area, one byte per tile.
type homeState struct {
	cells []uint8
	ver   uint32 // bumped on every change (WorldView.HomeVer)
}

// paintHome adds (on) or clears the Home area over the rectangle.
func (g *Game) paintHome(x, y, x2, y2 int, on bool) {} // SIM-021 stub

// fillHomeView copies the Home area into the view.
func (g *Game) fillHomeView(w *WorldView) {} // SIM-021 stub

// exportHome and importHome save and restore the Home area; an older save
// (nil) gets a Home area around its buildings.
func (g *Game) exportHome() []uint8 { return nil } // SIM-021 stub

func (g *Game) importHome(cells []uint8) {} // SIM-021 stub
