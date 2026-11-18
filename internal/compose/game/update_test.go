// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import "testing"

func TestUpdateCheckIsOffForDevBuildsAndDevRuns(t *testing.T) {
	t.Parallel()
	if updateChecker("dev", false) != nil {
		t.Error("a dev build looked for updates")
	}
	if updateChecker("v0.1.21", true) != nil {
		t.Error("a developer run looked for updates")
	}
}
