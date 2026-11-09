package arch

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The packaged-assets guard (.agents/specs/RELEASES.md). Every path the game
// finds through internal/platform/assetroot at run time must ship in the
// releases: packaging/assets.txt lists what every packager copies, and a
// lookup path must start with one of its lines. A lookup whose path is not
// a constant fails too, because the guard could not check it.

const (
	assetList      = "packaging/assets.txt"
	assetrootPkg   = module + "/internal/platform/assetroot"
	assetTreeRoots = "internal cmd pkg"
)

func TestPackagedAssetsCoverRuntime(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	list := readAssetList(t, filepath.Join(root, filepath.FromSlash(assetList)))
	for _, entry := range list {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry))); err != nil {
			t.Errorf("%s lists %s, which is not in the repository", assetList, entry)
		}
	}
	lookups, err := assetLookups(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lookups) < 4 {
		t.Fatalf("found %d assetroot lookups; the scan is broken", len(lookups))
	}
	for _, l := range lookups {
		if l.err != "" {
			t.Errorf("%s: %s", l.at, l.err)
			continue
		}
		if !coveredBy(l.path, list) {
			t.Errorf("%s: the game looks for %s, which no line of %s ships; add it there", l.at, l.path, assetList)
		}
	}
}

// TestAssetLookupsSeeEachForm: literals, FromSlash, Join, a package
// constant, an imported one and FindParent's root plus marker.
func TestAssetLookupsSeeEachForm(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(rel, src string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pkg/names/names.go", "package names\nconst Rel = \"metadata/audio/audio.json\"\n")
	write("internal/a/a.go", `package a
import (
	"path/filepath"
	"rimworld-go/internal/platform/assetroot"
	"rimworld-go/pkg/names"
)
const splash = "assets/source/x.png"
var dyn = "assets/" + "y"
func f() {
	assetroot.Find(filepath.FromSlash("assets/lang"), true)
	assetroot.Find(filepath.Join("assets", "palettes"), true)
	assetroot.Find(splash, false)
	assetroot.FindParent("assets", filepath.FromSlash(names.Rel), false)
	assetroot.Find(dyn, true)
}
`)
	write("internal/a/a_test.go", "package a\nimport \"rimworld-go/internal/platform/assetroot\"\nfunc g() { assetroot.Find(\"nope\", true) }\n")
	got, err := assetLookups(dir)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, l := range got {
		if l.err != "" {
			paths = append(paths, "error")
			continue
		}
		paths = append(paths, l.path)
	}
	want := "assets/lang assets/palettes assets/source/x.png assets/metadata/audio/audio.json error"
	if strings.Join(paths, " ") != want {
		t.Fatalf("lookups %q, want %q", paths, want)
	}
	if !coveredBy("assets/lang", []string{"assets/lang"}) || !coveredBy("assets/metadata/audio/audio.json", []string{"assets/metadata"}) || coveredBy("assets/language", []string{"assets/lang"}) {
		t.Fatal("coveredBy must match whole path segments")
	}
}

// assetLookup is one assetroot.Find or FindParent call: its constant path,
// or why it has none.
type assetLookup struct {
	at, path, err string
}

func readAssetList(t *testing.T, p string) []string {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, strings.TrimSuffix(line, "/"))
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("%s lists nothing", p)
	}
	return out
}

// coveredBy: p is an entry or lies under one.
func coveredBy(p string, list []string) bool {
	for _, e := range list {
		if p == e || strings.HasPrefix(p, e+"/") {
			return true
		}
	}
	return false
}

// goPackage is one directory's non-test files and its constants.
type goPackage struct {
	files  map[string]*ast.File
	consts map[string]constDef
}

// constDef is a constant's value and the imports of the file declaring it.
type constDef struct {
	val     ast.Expr
	imports map[string]string
}

// assetLookups parses the non-test Go files under root's code trees and
// returns every assetroot lookup, by package and file.
func assetLookups(root string) ([]assetLookup, error) {
	pkgs := map[string]*goPackage{} // by import path
	for _, top := range strings.Fields(assetTreeRoots) {
		base := filepath.Join(root, top)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			af, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			imp := module + "/" + path.Dir(filepath.ToSlash(rel))
			pk := pkgs[imp]
			if pk == nil {
				pk = &goPackage{files: map[string]*ast.File{}, consts: map[string]constDef{}}
				pkgs[imp] = pk
			}
			pk.files[filepath.ToSlash(rel)] = af
			collectConsts(af, pk.consts)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var out []assetLookup
	for _, imp := range sortedKeys(pkgs) {
		pk := pkgs[imp]
		for _, name := range sortedKeys(pk.files) {
			out = append(out, fileLookups(name, pk.files[name], pk, pkgs)...)
		}
	}
	return out, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// fileImports maps af's local import names to their paths.
func fileImports(af *ast.File) map[string]string {
	out := map[string]string{}
	for _, is := range af.Imports {
		p, _ := strconv.Unquote(is.Path.Value)
		local := path.Base(p)
		if is.Name != nil {
			local = is.Name.Name
		}
		out[local] = p
	}
	return out
}

// collectConsts records af's package-level constants that have a value.
func collectConsts(af *ast.File, into map[string]constDef) {
	imports := fileImports(af)
	for _, decl := range af.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if i < len(vs.Values) {
					into[n.Name] = constDef{vs.Values[i], imports}
				}
			}
		}
	}
}

// fileLookups finds af's assetroot.Find and FindParent calls; FindParent's
// path is its root joined with its marker.
func fileLookups(name string, af *ast.File, pk *goPackage, pkgs map[string]*goPackage) []assetLookup {
	ev := constEval{pk: pk, pkgs: pkgs, imports: fileImports(af)}
	var out []assetLookup
	ast.Inspect(af, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := ev.qualified(call.Fun)
		if !ok || (fn != assetrootPkg+".Find" && fn != assetrootPkg+".FindParent") || len(call.Args) < 2 {
			return true
		}
		args := call.Args[:1]
		if strings.HasSuffix(fn, ".FindParent") {
			args = call.Args[:2]
		}
		at := fmt.Sprintf("%s:%s", name, path.Base(fn))
		parts := make([]string, 0, len(args))
		for _, a := range args {
			s, err := ev.eval(a)
			if err != nil {
				out = append(out, assetLookup{at: at, err: err.Error()})
				return true
			}
			parts = append(parts, s)
		}
		out = append(out, assetLookup{at: at, path: path.Join(parts...)})
		return true
	})
	return out
}

// constEval reduces a path argument to a slash-separated string, in the
// scope of package pk and one file's imports.
type constEval struct {
	pk      *goPackage
	pkgs    map[string]*goPackage
	imports map[string]string
}

// qualified is pkg.Name's import path plus name.
func (c constEval) qualified(e ast.Expr) (string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || c.imports[x.Name] == "" {
		return "", false
	}
	return c.imports[x.Name] + "." + sel.Sel.Name, true
}

func (c constEval) eval(e ast.Expr) (string, error) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strconv.Unquote(v.Value)
		}
	case *ast.ParenExpr:
		return c.eval(v.X)
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			l, err := c.eval(v.X)
			if err != nil {
				return "", err
			}
			r, err := c.eval(v.Y)
			return l + r, err
		}
	case *ast.Ident:
		if def, ok := c.pk.consts[v.Name]; ok {
			return constEval{c.pk, c.pkgs, def.imports}.eval(def.val)
		}
	case *ast.SelectorExpr:
		if q, ok := c.qualified(v); ok {
			imp := strings.TrimSuffix(q, "."+v.Sel.Name)
			if other := c.pkgs[imp]; other != nil {
				if def, ok := other.consts[v.Sel.Name]; ok {
					return constEval{other, c.pkgs, def.imports}.eval(def.val)
				}
			}
		}
	case *ast.CallExpr:
		return c.evalCall(v)
	}
	return "", fmt.Errorf("the path %s is not a constant the guard can read; use a string constant", exprString(e))
}

// evalCall handles filepath.FromSlash, ToSlash, Clean and Join.
func (c constEval) evalCall(call *ast.CallExpr) (string, error) {
	fn, _ := c.qualified(call.Fun)
	parts := make([]string, 0, len(call.Args))
	for _, a := range call.Args {
		s, err := c.eval(a)
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
	}
	switch fn {
	case "path/filepath.FromSlash", "path/filepath.ToSlash", "path/filepath.Clean":
		return path.Clean(parts[0]), nil
	case "path/filepath.Join":
		return path.Join(parts...), nil
	}
	return "", fmt.Errorf("the path %s is not a constant the guard can read; use a string constant", exprString(call))
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.CallExpr:
		return exprString(v.Fun) + "(...)"
	}
	return fmt.Sprintf("%T", e)
}
