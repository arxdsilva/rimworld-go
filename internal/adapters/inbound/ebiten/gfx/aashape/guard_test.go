package aashape

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

// vectorAAAllowed are the functions ("file.go:func", slash paths below the
// ebiten dir) that may still draw through vector's anti-aliased path, with
// the reason. Each one costs a stencil pass when its destination is next
// used, so the list only shrinks.
var vectorAAAllowed = map[string]string{
	// a tessellated route would double-blend where two routes cross; that
	// needs a look in game before it moves (the circles already did)
	"render/overlay/paths_draw.go:DrawPaths": "colonist route polylines (vector.StrokePath)",
}

// No ebiten client code anti-aliases through vector (a literal true
// antialias argument, AntiAlias: true, or .AntiAlias = true): anti-aliased
// shapes go through gfx (FillCircle & co. with aa, PxDisc & co.), which
// tessellate them here.
func TestNoVectorAntiAlias(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string][]string{}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (d.Name() == "fontdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		for k, v := range vectorAA(t, p, filepath.ToSlash(rel)) {
			found[k] = append(found[k], v...)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if vectorAAAllowed[k] == "" {
			t.Errorf("%s anti-aliases through vector (%s): draw through gfx (aa=true, or PxDisc/PxRing/PxLine/PxFrame)", k, strings.Join(found[k], ", "))
		}
	}
	for k := range vectorAAAllowed {
		if found[k] == nil {
			t.Errorf("stale: vectorAAAllowed %q no longer anti-aliases through vector", k)
		}
	}
	if len(found) == 0 {
		t.Fatal("scanned nothing: is the ebiten dir two levels up?")
	}
}

// vectorAA is, per "file:func" of the file at p, where it asks vector for
// anti-aliasing.
func vectorAA(t *testing.T, p, rel string) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, p, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	vec := ""
	for _, im := range af.Imports {
		if path, _ := strconv.Unquote(im.Path.Value); strings.HasSuffix(path, "/ebiten/v2/vector") {
			vec = "vector"
			if im.Name != nil {
				vec = im.Name.Name
			}
		}
	}
	out := map[string][]string{}
	if vec == "" {
		return out
	}
	isTrue := func(e ast.Expr) bool { id, ok := e.(*ast.Ident); return ok && id.Name == "true" }
	for _, d := range af.Decls {
		name := "<pkg>"
		if fd, ok := d.(*ast.FuncDecl); ok {
			name = fd.Name.Name
		}
		key := rel + ":" + name
		at := func(n ast.Node, what string) {
			out[key] = append(out[key], "line "+strconv.Itoa(fset.Position(n.Pos()).Line)+": "+what)
		}
		ast.Inspect(d, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == vec && len(x.Args) > 0 && isTrue(x.Args[len(x.Args)-1]) {
						at(x, vec+"."+sel.Sel.Name+"(…, true)")
					}
				}
			case *ast.KeyValueExpr:
				if k, ok := x.Key.(*ast.Ident); ok && k.Name == "AntiAlias" && isTrue(x.Value) {
					at(x, "AntiAlias: true")
				}
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					if sel, ok := l.(*ast.SelectorExpr); ok && sel.Sel.Name == "AntiAlias" && i < len(x.Rhs) && isTrue(x.Rhs[i]) {
						at(x, ".AntiAlias = true")
					}
				}
			}
			return true
		})
	}
	return out
}

// The scan sees every spelling, under any import name, and only those.
func TestNoVectorAntiAliasCatches(t *testing.T) {
	src := `package p
import vec "github.com/hajimehoshi/ebiten/v2/vector"
func a() { vec.FillCircle(d, 1, 2, 3, c, true) }
func b() { vec.StrokePath(d, p, nil, &vec.DrawPathOptions{AntiAlias: true}) }
func c() { var o vec.DrawPathOptions; o.AntiAlias = true }
func ok() { vec.FillCircle(d, 1, 2, 3, c, false); vec.StrokeLine(d, 0, 0, 1, 1, 1, c, aa) }
`
	p := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	got := vectorAA(t, p, "p.go")
	for _, k := range []string{"p.go:a", "p.go:b", "p.go:c"} {
		if len(got[k]) != 1 {
			t.Errorf("%s: %v, want one finding", k, got[k])
		}
	}
	if got["p.go:ok"] != nil {
		t.Errorf("ok: %v, want none", got["p.go:ok"])
	}
}
