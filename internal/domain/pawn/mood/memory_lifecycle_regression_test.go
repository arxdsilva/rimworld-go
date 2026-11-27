// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import "testing"

func TestForgetExpired(t *testing.T) {
	m := Memories{{Kind: Grief, Subject: 42, Expires: 10, Value: -20, Label: "grief"}}
	want := m[0]
	got := ForgetOne(m, 10, func(int) int { t.Fatal("roll for expired memory"); return 0 })
	if len(got) != 1 || got[0] != want {
		t.Fatalf("expired set changed: got=%+v want=%+v", got, want)
	}
}
func TestGriefNotConditional(t *testing.T) {
	m := cb.AddLabeledUntil(nil, Grief, 42, -20, "grief", 0, 100)
	got := NullifyConditional(cb.AppendActiveAt(nil, m, 10), 10000, 1, 42, 1)
	if len(got) != 1 {
		t.Fatalf("unexpired event memory disappeared from active thoughts: got=%+v saved=%+v", got, m)
	}
}
