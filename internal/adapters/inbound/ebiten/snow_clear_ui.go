package ebiten

// The snow clearing overlay's tool hook (the overlay itself is
// render/overlay, snow_clear_draw.go).

// snowClearTools are the two snow clearing Architect tools.
var snowClearTools = [2]string{"snow_clear", "unsnow_clear"}

// snowClearToolID reports id is one of the two snow clearing tools.
func snowClearToolID(id string) bool { return id == snowClearTools[0] || id == snowClearTools[1] }

// syncSnowClearForce shows the overlay while a snow clearing tool is held.
func (a *App) syncSnowClearForce() {
	if a.renderer != nil {
		a.renderer.overlays().SnowClear.Forced = a.archOpen && snowClearToolID(a.archTool)
	}
}
