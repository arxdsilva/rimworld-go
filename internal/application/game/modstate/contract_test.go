// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modstate_test

import (
	"testing"

	"rimworld-go/internal/application/game/modstate"
)

type fixedSource []modstate.Blob

func (s fixedSource) SaveBlobs() []modstate.Blob { return s }

var _ modstate.Source = fixedSource(nil)

// TestContractLimits pins the save limits the PLG-063 contract fixed.
func TestContractLimits(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"MaxData", modstate.MaxData, 64<<10 + 16},
		{"MaxBlobs", modstate.MaxBlobs, 256},
		{"MaxName", modstate.MaxName, 256},
		{"MaxNotes", modstate.MaxNotes, 16},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
