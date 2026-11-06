package lang

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// srcFile is one parsed production file of the ebiten client.
type srcFile struct {
	rel     string            // slash path below the ebiten dir: "hud/global.go"
	pkg     string            // its directory: "hud", "." for the root
	path    string            // its package's import path
	imports map[string]string // local name -> import path
	fset    *token.FileSet
	ast     *ast.File
}

// repoRoot walks up from the test's directory to go.mod.
func repoRoot(t *testing.T) string {
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
			t.Fatal("go.mod not found above the test")
		}
		dir = up
	}
}

// module reads the module path from go.mod.
func module(t *testing.T, root string) string {
	t.Helper()
	f, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m, ok := strings.CutPrefix(sc.Text(), "module "); ok {
			return strings.TrimSpace(m)
		}
	}
	t.Fatal("go.mod has no module line")
	return ""
}

const ebitenRel = "internal/adapters/inbound/ebiten"

// parseClient parses every non-test Go file under the ebiten client.
func parseClient(t *testing.T) []*srcFile {
	t.Helper()
	root := repoRoot(t)
	base := filepath.Join(root, ebitenRel)
	importBase := module(t, root) + "/" + ebitenRel
	var out []*srcFile
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		af, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(base, p)
		rel = filepath.ToSlash(rel)
		f := &srcFile{rel: rel, pkg: path.Dir(rel), fset: fset, ast: af, imports: map[string]string{}}
		f.path = importBase
		if f.pkg != "." {
			f.path += "/" + f.pkg
		}
		f.imports = importNames(af)
		out = append(out, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

// parseOne parses src as the file "x/x.go" of package path "example.com/x".
func parseOne(t *testing.T, src string) *srcFile {
	t.Helper()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "x.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	return &srcFile{rel: "x/x.go", pkg: "x", path: "example.com/x", fset: fset, ast: af, imports: importNames(af)}
}

// importNames maps each import's local name to its path ("ebiten/v2" is
// named ebiten, as Go names major-version paths).
func importNames(af *ast.File) map[string]string {
	out := map[string]string{}
	for _, im := range af.Imports {
		ip, _ := strconv.Unquote(im.Path.Value)
		name := path.Base(ip)
		if len(name) > 1 && name[0] == 'v' && strings.Trim(name[1:], "0123456789") == "" {
			name = path.Base(path.Dir(ip))
		}
		if im.Name != nil {
			name = im.Name.Name
		}
		out[name] = ip
	}
	return out
}

// strLit unquotes a string literal; ok is false for anything else.
func strLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	return s, err == nil
}

// translateKey is the source key a T/Tf call carries: x.T(lit),
// x.Tf(lit, …), lang.Tf(tr, lit, …) or Tf(tr, lit, …) inside lang.
func translateKey(f *srcFile, call *ast.CallExpr) (string, bool) {
	var name string
	pkgFunc := false
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		name, pkgFunc = fn.Name, true
	case *ast.SelectorExpr:
		name = fn.Sel.Name
		if x, ok := fn.X.(*ast.Ident); ok && strings.HasSuffix(f.imports[x.Name], "/ebiten/lang") {
			pkgFunc = true
		}
	default:
		return "", false
	}
	switch {
	case name == "T" && !pkgFunc && len(call.Args) == 1:
		return strLit(call.Args[0])
	case name == "Tf" && pkgFunc && len(call.Args) >= 2:
		return strLit(call.Args[1])
	case name == "Tf" && !pkgFunc && len(call.Args) >= 1:
		return strLit(call.Args[0])
	}
	return "", false
}

// isTranslate reports whether call is a T or Tf call, literal or not.
func isTranslate(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == "Tf"
	case *ast.SelectorExpr:
		return fn.Sel.Name == "T" || fn.Sel.Name == "Tf"
	}
	return false
}

// hasLetter reports whether s holds any letter (pure punctuation, digits
// and format verbs need no translation).
func hasLetter(s string) bool {
	return strings.IndexFunc(stripVerbs(s), unicode.IsLetter) >= 0
}

// stripVerbs drops fmt verbs so "%d%%" and "%s" count as letterless.
func stripVerbs(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && strings.IndexByte("+-# 0123456789.*[]", s[j]) >= 0 {
			j++
		}
		i = j
	}
	return b.String()
}
