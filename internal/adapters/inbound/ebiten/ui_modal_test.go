// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// modalGeometry places a w x h modal centred on the screen, shrunk to fit.
func modalGeometry(sw, sh int, m modal) (frame, title, body, footer gfx.Rect) {
	return uikit.ModalGeometry(sw, sh, uikit.Modal{W: m.W, H: m.H, Buttons: m.Buttons})
}
