// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

// MaterialEra identifies the verified paid-material catalogue. Zero means
// current; older unverified costs must not produce newly priced refunds.
type MaterialEra uint8

const (
	MaterialCurrent MaterialEra = iota
	MaterialV20
	MaterialUnverified
)

// MaterialBasis is compact and follows a building through minified cargo.
// Same-kind swaps buy new primary material; a new recipe buys both.
type MaterialBasis struct{ Primary, Secondary MaterialEra }
