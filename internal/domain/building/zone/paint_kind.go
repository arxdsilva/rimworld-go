// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package zone

// PaintKind admits the four user-paintable zone kinds.
func PaintKind(name string) (Kind, bool) {
	switch name {
	case "stockpile":
		return KindStockpile, true
	case "grow":
		return KindGrow, true
	case "pen":
		return KindPen, true
	case "dump":
		return KindDump, true
	}
	return 0, false
}
