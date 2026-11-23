// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// Pain per point of wound damage, in hundredths of a permille. wiki: Injury,
// Pain, Health Difficulties: most wounds hurt 1.25 % of the pain scale per
// HP (12.5 permille), burns 1.875 %, scars 0.625 %. Tended wounds hurt as
// open ones do (tending stops bleeding, not pain). A fresh stump hurts as a
// wound of the part's MaxHP until tended (original tuning on the wiki's
// fresh missing part). The wiki's crack rate (1.0 %) is not modelled: there
// is no fracture wound kind.
const (
	painStd  = 1250
	painBurn = 1875
	painScar = 625
)

// painRate is the pain per HP of a live (non-scar) wound of kind k.
func painRate(k WoundKind) int32 {
	if k == Burn {
		return painBurn
	}
	return painStd
}
