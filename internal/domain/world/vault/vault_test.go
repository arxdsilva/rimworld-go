// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package vault

import "testing"

func TestNames(t *testing.T) {
	if Grateful.String() != "grateful" || Remains.String() != "remains" || NumOccupants.String() != "" {
		t.Fatal("occupant names")
	}
	if Breached.String() != "breached" || State(99).String() != "" {
		t.Fatal("state names")
	}
}
