// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

// planShown reports the Plan overlay is showing (default on, tool held).
func (r *Renderer) planShown() bool { return r.overlays().Plan.Shown() }
