package arch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// simTrees are the deterministic core: one simulation owner, no goroutine
// per pawn or job (AGENTS.md). Parallel work goes through
// internal/platform/pool, the one place that starts goroutines for the
// simulation, behind a port the stage declares (game/pathstage.Pool).
var simTrees = []string{"internal/application", "internal/domain", "pkg"}

// TestNoGoStatementsInSim: no go statement in the non-test code of the
// application, domain or pkg trees.
func TestNoGoStatementsInSim(t *testing.T) {
	root := moduleRoot(t)
	var found []string
	for _, top := range simTrees {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			fset := token.NewFileSet()
			af, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, pos := range goStmts(af) {
				rel, _ := filepath.Rel(root, path)
				found = append(found, filepath.ToSlash(rel)+":"+strconv.Itoa(fset.Position(pos).Line))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) > 0 {
		sort.Strings(found)
		t.Fatalf("go statements in simulation code (run parallel work on internal/platform/pool through a port):\n%s", strings.Join(found, "\n"))
	}
}

// goStmts is where af starts goroutines.
func goStmts(af *ast.File) []token.Pos {
	var at []token.Pos
	ast.Inspect(af, func(n ast.Node) bool {
		if g, ok := n.(*ast.GoStmt); ok {
			at = append(at, g.Pos())
		}
		return true
	})
	return at
}

// TestNoGoStatementsSeesOne: the walk finds a go statement, nested too.
func TestNoGoStatementsSeesOne(t *testing.T) {
	af, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\nfunc f() { go f(); func() { go func() {}() }() }\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(goStmts(af)); n != 2 {
		t.Fatalf("found %d go statements, want 2", n)
	}
}
