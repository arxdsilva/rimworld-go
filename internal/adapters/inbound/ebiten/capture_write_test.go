// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteShotCreatesPrivateDirAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shots", "a.png")
	if err := writeShot(path, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("shot missing or empty: %v", err)
	}
	st, err := os.Stat(filepath.Dir(path))
	if err != nil || st.Mode().Perm()&^0o750 != 0 || st.Mode().Perm()&0o700 != 0o700 {
		t.Fatalf("dir mode %v err %v, want 0o750 or tighter by umask", st.Mode().Perm(), err)
	}
}

func TestWriteShotReportsUnwritablePaths(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if writeShot(filepath.Join(blocker, "x", "a.png"), image.NewRGBA(image.Rect(0, 0, 1, 1))) == nil {
		t.Fatal("MkdirAll under a file: want error")
	}
	if writeShot(dir, image.NewRGBA(image.Rect(0, 0, 1, 1))) == nil {
		t.Fatal("Create on a directory: want error")
	}
}
