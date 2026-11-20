// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const luaImport = "github.com/yuin/gopher-lua"

// luaImporters lists the non-test .go files under root (skipping .git and
// testdata) that import gopher-lua.
func luaImporters(t testing.TB, root string) []string {
	t.Helper()
	var bad []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, im := range f.Imports {
			if p, _ := strconv.Unquote(im.Path.Value); p == luaImport || strings.HasPrefix(p, luaImport+"/") {
				bad = append(bad, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return bad
}

func moduleRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		up := filepath.Dir(dir)
		if up == dir {
			t.Fatal("go.mod not found")
		}
		dir = up
	}
}

// TestGopherLuaOnlyInBenchTests: gopher-lua never reaches the game binary.
func TestGopherLuaOnlyInBenchTests(t *testing.T) {
	t.Parallel()
	if bad := luaImporters(t, moduleRoot(t)); len(bad) != 0 {
		t.Fatalf("non-test files import gopher-lua: %v", bad)
	}
}

// TestGuardSeesAnImport proves the scan: a synthetic tree with a non-test
// importer is flagged; a test file and a testdata file are not.
func TestGuardSeesAnImport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := "package x\n\nimport _ \"" + luaImport + "\"\n"
	for name, flagged := range map[string]bool{"a/bad.go": true, "a/ok_test.go": false, "testdata/t.go": false, ".git/g.go": false} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = flagged
	}
	got := luaImporters(t, root)
	if len(got) != 1 || filepath.Base(got[0]) != "bad.go" {
		t.Fatalf("flagged %v, want only a/bad.go", got)
	}
}
