// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestCommsTextsNamePoweredBeacon(t *testing.T) {
	t.Parallel()
	s, _ := newFake()
	if h := commsHeader(s, game.CommsView{OperatorName: "Ada", BeaconSilver: 5}); !strings.Contains(h, "powered trade beacon") {
		t.Errorf("header %q", h)
	}
	if r := noSilverReason(s); !strings.Contains(r, "powered") {
		t.Errorf("no-silver reason %q", r)
	}
}
