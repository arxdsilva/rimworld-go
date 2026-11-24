// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package quest

import (
	"testing"

	qd "rimworld-go/internal/domain/quest"
)

func TestKindNameBounds(t *testing.T) {
	t.Parallel()
	if got := KindName(qd.NumKinds - 1); got != "Truce talks" {
		t.Errorf("last kind: %q", got)
	}
	if got := KindName(qd.NumKinds); got != "" {
		t.Errorf("NumKinds is out of range: %q", got)
	}
	if got := KindName(0); got != "" {
		t.Errorf("none: %q", got)
	}
}
