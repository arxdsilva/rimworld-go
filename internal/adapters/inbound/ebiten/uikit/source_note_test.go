// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package uikit

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
)

func TestSourceNote(t *testing.T) {
	en := lang.English{}
	names := func(id string) string { return map[string]string{"a": "Alpha Pack"}[id] }
	cases := []struct {
		added   string
		patched []string
		want    string
	}{
		{"", nil, ""},
		{"a", nil, "Added by Alpha Pack"},
		{"", []string{"a", "b"}, "Changed by Alpha Pack, b"},
		{"b", []string{"a"}, "Added by b\nChanged by Alpha Pack"},
	}
	for _, c := range cases {
		if got := SourceNote(en, c.added, c.patched, names); got != c.want {
			t.Errorf("SourceNote(%q,%v) = %q, want %q", c.added, c.patched, got, c.want)
		}
	}
	if got := SourceNote(en, "x", nil, nil); got != "Added by x" {
		t.Errorf("ids without names = %q", got)
	}
}
