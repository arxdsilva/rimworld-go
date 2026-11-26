// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package command

import "testing"

func TestRenameNameRoundTrip(t *testing.T) {
	f, n, l, ok := UnpackRename(RenameName("Ada", "", "O'Neil-Smith"))
	if !ok || f != "Ada" || n != "" || l != "O'Neil-Smith" {
		t.Fatalf("%q %q %q %v", f, n, l, ok)
	}
	if _, _, _, ok := UnpackRename("Ada"); ok {
		t.Fatal("one part is not a rename")
	}
}
