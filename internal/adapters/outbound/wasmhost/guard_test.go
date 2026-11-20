// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The guard keeps wazero inside the adapter and the adapter unwired (AC-8),
// in the pattern of scriptbench/guard_test.go (copied, not imported).

const (
	guardWazero  = "github.com/tetratelabs/wazero"
	guardHostPkg = "rimworld-go/internal/adapters/outbound/wasmhost"
	guardHostDir = "internal/adapters/outbound/wasmhost"
)

// guardUnder reports whether slash path p is dir or inside it.
func guardUnder(p, dir string) bool { return p == dir || strings.HasPrefix(p, dir+"/") }

// guardImport is one import of one non-test file, the file relative to
// the root in slash form.
type guardImport struct{ file, path string }

// guardImports lists the imports of the non-test .go files under root. It
// skips testdata and the directories the go tool ignores (. and _
// prefixes, so .git and run scratch too).
func guardImports(t *testing.T, root string) []guardImport {
	t.Helper()
	var out []guardImport
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if path != root && (n == "testdata" || strings.HasPrefix(n, ".") || strings.HasPrefix(n, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			t.Errorf("parse %s: %v", path, perr)
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			out = append(out, guardImport{filepath.ToSlash(rel), p})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// guardViolations applies the two rules: wazero only under wasmhost and
// not in wasmrewrite or wasmtest; wasmhost imported only from inside
// wasmhost. It also returns the files allowed to import wazero.
func guardViolations(ims []guardImport) (bad []string, wazeroUsers []string) {
	for _, im := range ims {
		dir := filepath.ToSlash(filepath.Dir(im.file))
		switch {
		case guardUnder(im.path, guardWazero):
			ok := guardUnder(dir, guardHostDir) &&
				!guardUnder(dir, guardHostDir+"/wasmrewrite") && !guardUnder(dir, guardHostDir+"/wasmtest")
			if !ok {
				bad = append(bad, im.file+" imports "+im.path)
			} else {
				wazeroUsers = append(wazeroUsers, im.file)
			}
		case guardUnder(im.path, guardHostPkg) && !guardUnder(dir, guardHostDir):
			bad = append(bad, im.file+" imports "+im.path)
		}
	}
	slices.Sort(bad)
	return bad, wazeroUsers
}

func guardRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

// C7: in the repo, wazero is imported only by wasmhost's own non-test
// files (and some are found, so the scan is not vacuous), and nothing
// outside wasmhost imports it.
func TestGuardWazeroStaysInTheAdapterAndNothingWiresIt(t *testing.T) {
	bad, users := guardViolations(guardImports(t, guardRoot(t)))
	if len(bad) != 0 {
		t.Errorf("guard violations:\n%s", strings.Join(bad, "\n"))
	}
	if !slices.Contains(users, guardHostDir+"/host.go") {
		t.Errorf("the scan did not see host.go import wazero (saw %v)", users)
	}
}

// C7: the guard sees a planted import. The tree has one file per rule and
// edge; test files and testdata are not flagged, and wasmhostx is not
// wasmhost.
func TestGuardSelfTestFlagsPlantedImports(t *testing.T) {
	root := t.TempDir()
	plant := func(rel, imp string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		src := "package x\n\nimport _ " + strconv.Quote(imp) + "\n"
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plant(guardHostDir+"/ok.go", guardWazero)
	plant(guardHostDir+"/sub/ok.go", guardWazero+"/api")
	plant(guardHostDir+"/inner.go", guardHostPkg+"/wasmrewrite")
	plant(guardHostDir+"/wasmrewrite/bad.go", guardWazero+"/api")
	plant(guardHostDir+"/wasmtest/bad.go", guardWazero)
	plant("internal/adapters/outbound/wasmhostx/bad.go", guardWazero)
	plant("cmd/game/bad.go", guardWazero+"/experimental")
	plant("internal/compose/game/bad.go", guardHostPkg)
	plant("internal/compose/game/bad2.go", guardHostPkg+"/wasmtest")
	plant("internal/compose/game/ok_test.go", guardHostPkg)
	plant("internal/x/testdata/ok.go", guardWazero)
	plant(".loops/scratch/ok.go", guardWazero)
	plant("internal/x/near.go", "github.com/tetratelabs/wazerox")

	bad, users := guardViolations(guardImports(t, root))
	want := []string{
		"cmd/game/bad.go imports " + guardWazero + "/experimental",
		guardHostDir + "/wasmrewrite/bad.go imports " + guardWazero + "/api",
		guardHostDir + "/wasmtest/bad.go imports " + guardWazero,
		"internal/adapters/outbound/wasmhostx/bad.go imports " + guardWazero,
		"internal/compose/game/bad.go imports " + guardHostPkg,
		"internal/compose/game/bad2.go imports " + guardHostPkg + "/wasmtest",
	}
	slices.Sort(want)
	if !slices.Equal(bad, want) {
		t.Errorf("violations:\n%s\nwant:\n%s", strings.Join(bad, "\n"), strings.Join(want, "\n"))
	}
	slices.Sort(users)
	if wantUsers := []string{guardHostDir + "/ok.go", guardHostDir + "/sub/ok.go"}; !slices.Equal(users, wantUsers) {
		t.Errorf("allowed wazero users %v, want %v", users, wantUsers)
	}
}
