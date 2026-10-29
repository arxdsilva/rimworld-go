package ebiten

// planShown reports the Plan overlay is showing (default on, tool held).
func (r *Renderer) planShown() bool { return r.overlays().Plan.Shown() }
