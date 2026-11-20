// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
)

// Each stage's error return in Prepare. The NaN stage's only error is a
// Walk failure, which cannot happen after Check walked every body.
func TestPrepareStageErrors(t *testing.T) {
	good := richModule()
	if _, err := wasmrewrite.Prepare(good[:10], testConfig()); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("decode stage: %v", err)
	}
	if _, err := wasmrewrite.Prepare(oneFunc(0x08, 0, 0x0B), testConfig()); err == nil || !strings.Contains(err.Error(), "exception") {
		t.Errorf("check stage: %v", err)
	}
	c := testConfig()
	c.MaxWeight = 10 // far under any function's weight
	if _, err := wasmrewrite.Prepare(good, c); err == nil || !strings.Contains(err.Error(), wasmrewrite.RuleWeight) {
		t.Errorf("meter stage: %v", err)
	}
	c = testConfig()
	c.FuelExport = ""
	if _, err := wasmrewrite.Prepare(good, c); err == nil || !strings.Contains(err.Error(), "reserve") {
		t.Errorf("reserve stage: %v", err)
	}
}
