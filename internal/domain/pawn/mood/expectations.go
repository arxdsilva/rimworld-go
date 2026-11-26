// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import "rimworld-go/internal/domain/pawn/expectations"

// ExpectationsForWealth selects a state thought from the shared wealth bands.
func ExpectationsForWealth(wealth int) Kind {
	return ExpectationsExtremelyLow + Kind(expectations.ForWealth(wealth))
}
