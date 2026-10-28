package arch

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// The SOLID guard (.cursor/SOLID_GO.md §8). It parses every non-test Go
// file under internal/ and cmd/ (syntax only, no type checking, so it
// runs in a couple of seconds) and measures, per package:
//
//   - methods: methods per receiver type            (limit 40)
//   - fields:  fields of a struct with ≥5 methods   (limit 25)
//   - iface:   explicit methods per interface        (limit 8; an embedded interface counts 1)
//   - lines:   non-test lines per package            (limit 12000)
//   - files:   non-test files per package            (limit 60)
//   - init:    func init() declarations              (none)
//   - global:  package-level vars assigned at runtime (none; sync/atomic values exempt)
//   - imports: forbidden import edges                 (none)
//
// solid_budget.txt records the debt the tree carries today: one line per
// violation, `rule key ceiling` for the numeric rules and `rule key` for
// the others, with # comments. The budget only shrinks: a measurement
// over its ceiling fails ("grew"), a ceiling well above the measurement
// fails ("stale, lower it"), a paid debt whose line remains fails, and a
// new violation without a line fails with the line to paste. Never raise
// a line; split the type or package instead (SOLID_GO.md §10).
//
// Known blind spots of a syntax-only scan: methods a type gains through an
// alias in another package are not counted, an embedded interface hides
// its methods (by design: composing small interfaces is the ISP path),
// and build-tagged files all count.

const module = "rimworld-go"

var limits = map[string]int{
	"methods": 40,
	"fields":  25,
	"iface":   8,
	"lines":   12000,
	"files":   60,
}

// forbiddenImports are the dependency edges no package may have: from a
// package whose path has the prefix `from`, to an import that `to`
// matches. HEXAGONAL_ARCHITECTURE.md §2 (direction) and §6 (adapters'
// sub-packages are consumers of the root, never the other way).
var forbiddenImports = []struct {
	from string
	to   func(path string) bool
	why  string
}{
	{module + "/internal/domain/", prefix(module + "/internal/adapters/"), "domain must not know adapters"},
	{module + "/internal/domain/", prefix(module + "/internal/application/"), "domain must not know the application layer"},
	{module + "/internal/domain/", prefix(module + "/internal/platform/"), "domain must not know the platform"},
	{module + "/internal/domain/", prefix(module + "/internal/ports/"), "domain must not know the ports"},
	{module + "/internal/domain/", prefix("github.com/hajimehoshi/ebiten"), "domain is headless"},
	{module + "/internal/domain/", exactOrSub("os"), "domain reads no files, env or clock"},
	{module + "/internal/application/", prefix(module + "/internal/adapters/"), "application must not know adapters"},
	{module + "/internal/ports/", prefix(module + "/internal/adapters/"), "ports must not know adapters"},
	{module + "/internal/adapters/outbound/", prefix(module + "/internal/adapters/inbound/"), "outbound adapters must not know the UI"},
	{module + "/internal/adapters/inbound/ebiten/", func(p string) bool { return p == module+"/internal/adapters/inbound/ebiten" }, "an ebiten sub-package is a consumer of the root, never its importer"},
}

func prefix(p string) func(string) bool {
	return func(s string) bool { return strings.HasPrefix(s, p) }
}
func exactOrSub(p string) func(string) bool {
	return func(s string) bool { return s == p || strings.HasPrefix(s, p+"/") }
}

// measure is one measured violation: a numeric rule's value, or 0 for the
// presence rules.
type measure struct {
	rule, key string
	value     int
}

func (m measure) id() string { return m.rule + " " + m.key }

// pkgScan is what the parser gathers for one package.
type pkgScan struct {
	path    string
	files   int
	lines   int
	methods map[string]int    // receiver type -> methods
	fields  map[string]int    // struct type -> fields
	ifaces  map[string]int    // interface type -> methods
	inits   []string          // file.go
	globals []string          // package var assigned in a function
	imports map[string]string // import path -> first file importing it
}

func TestSOLID(t *testing.T) {
	root := moduleRoot(t)
	pkgs := scanTree(t, root)
	var found []measure
	for _, p := range pkgs {
		for typ, n := range p.methods {
			if n > limits["methods"] {
				found = append(found, measure{"methods", p.path + "." + typ, n})
			}
			if f := p.fields[typ]; n >= 5 && f > limits["fields"] {
				found = append(found, measure{"fields", p.path + "." + typ, f})
			}
		}
		for typ, n := range p.ifaces {
			if n > limits["iface"] {
				found = append(found, measure{"iface", p.path + "." + typ, n})
			}
		}
		if p.lines > limits["lines"] {
			found = append(found, measure{"lines", p.path, p.lines})
		}
		if p.files > limits["files"] {
			found = append(found, measure{"files", p.path, p.files})
		}
		for _, f := range p.inits {
			found = append(found, measure{"init", p.path + "/" + f, 0})
		}
		for _, g := range p.globals {
			found = append(found, measure{"global", p.path + "." + g, 0})
		}
		for imp := range p.imports {
			for _, rule := range forbiddenImports {
				if strings.HasPrefix(p.path, rule.from) && rule.to(imp) {
					found = append(found, measure{"imports", p.path + "->" + imp, 0})
				}
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].id() < found[j].id() })

	budget, order := readBudget(t, filepath.Join(root, "internal", "arch", "solid_budget.txt"))
	if bad := judge(found, budget, order); len(bad) > 0 {
		t.Errorf("SOLID guard (.cursor/SOLID_GO.md §8): %d violation(s)\n\t%s", len(bad), strings.Join(bad, "\n\t"))
	}
}

// judge is the ratchet: it compares what the scan found with the budget
// and returns one line per violation. A measurement over the limit needs
// a line at or above it; a line well above the measurement (more than 2
// or 5 %) is stale; a line whose debt is paid must go; a presence rule's
// line must still be observed.
func judge(found []measure, budget map[string]int, order []string) []string {
	var bad []string
	seen := map[string]bool{}
	for _, m := range found {
		seen[m.id()] = true
		ceiling, ok := budget[m.id()]
		switch {
		case !ok && m.value > 0:
			bad = append(bad, fmt.Sprintf("%s: %s is %d, limit %d: split it (SOLID_GO.md §10); if it must wait, add the line\n\t\t%s %s %d", m.rule, m.key, m.value, limits[m.rule], m.rule, m.key, m.value))
		case !ok:
			bad = append(bad, fmt.Sprintf("%s: %s: %s", m.rule, m.key, explain(m.rule)))
		case m.value > ceiling:
			bad = append(bad, fmt.Sprintf("%s: %s grew to %d, budget %d (limit %d): split it (SOLID_GO.md §10), do not raise the budget", m.rule, m.key, m.value, ceiling, limits[m.rule]))
		case m.value > 0 && ceiling > m.value+max(2, ceiling/20):
			bad = append(bad, fmt.Sprintf("budget: %s %s is %d, ceiling %d is stale; lower it to %d", m.rule, m.key, m.value, ceiling, m.value))
		}
	}
	for _, id := range order {
		if !seen[id] {
			bad = append(bad, fmt.Sprintf("budget: %s no longer violates; delete its line", id))
		}
	}
	return bad
}

func explain(rule string) string {
	switch rule {
	case "init":
		return "func init() registers state; register from the composition root instead (SOLID_GO.md §4)"
	case "global":
		return "a package-level var is assigned at runtime; make it a field of its owner or a constructor option (SOLID_GO.md §9)"
	case "imports":
		return "forbidden import direction (HEXAGONAL_ARCHITECTURE.md §2, §6)"
	}
	return rule
}

// readBudget parses solid_budget.txt: `rule key [ceiling]  # note` lines.
// It returns the ceilings by "rule key" and the ids in file order.
func readBudget(t *testing.T, path string) (map[string]int, []string) {
	t.Helper()
	budget := map[string]int{}
	var order []string
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return budget, nil
		}
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		fs := strings.Fields(line)
		if len(fs) == 0 {
			continue
		}
		if len(fs) < 2 || len(fs) > 3 {
			t.Fatalf("%s:%d: want `rule key [ceiling]`, got %q", path, n, sc.Text())
		}
		if _, ok := limits[fs[0]]; !ok && fs[0] != "init" && fs[0] != "global" && fs[0] != "imports" {
			t.Fatalf("%s:%d: unknown rule %q", path, n, fs[0])
		}
		id := fs[0] + " " + fs[1]
		if _, dup := budget[id]; dup {
			t.Fatalf("%s:%d: %s listed twice", path, n, id)
		}
		ceiling := 0
		if len(fs) == 3 {
			if ceiling, err = strconv.Atoi(fs[2]); err != nil || ceiling <= limits[fs[0]] {
				t.Fatalf("%s:%d: ceiling %q must be a number above the limit %d", path, n, fs[2], limits[fs[0]])
			}
		} else if _, numeric := limits[fs[0]]; numeric {
			t.Fatalf("%s:%d: %s needs a ceiling", path, n, id)
		}
		budget[id] = ceiling
		order = append(order, id)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return budget, order
}

func moduleRoot(t *testing.T) string {
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
			t.Fatal("module root not found")
		}
		dir = up
	}
}

// scanTree parses every package under internal/ and cmd/, one goroutine
// per package.
func scanTree(t *testing.T, root string) []*pkgScan {
	t.Helper()
	dirs, tests := map[string][]string{}, map[string][]string{}
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".go") {
				dir := filepath.Dir(path)
				if strings.HasSuffix(path, "_test.go") {
					tests[dir] = append(tests[dir], path)
				} else {
					dirs[dir] = append(dirs[dir], path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		out  []*pkgScan
		errs []string
	)
	for dir, files := range dirs {
		wg.Add(1)
		go func(dir string, files []string) {
			defer wg.Done()
			rel, _ := filepath.Rel(root, dir)
			p, err := scanPackage(module+"/"+filepath.ToSlash(rel), files, tests[dir])
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err.Error())
				return
			}
			out = append(out, p)
		}(dir, files)
	}
	wg.Wait()
	if len(errs) > 0 {
		sort.Strings(errs)
		t.Fatalf("parse:\n%s", strings.Join(errs, "\n"))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

// scanPackage measures one package. Its test files are parsed only for
// the global rule: a test that assigns a package var at runtime is the
// same hazard as production code doing it (parallel tests race on it).
func scanPackage(path string, files, testFiles []string) (*pkgScan, error) {
	p := &pkgScan{path: path, methods: map[string]int{}, fields: map[string]int{}, ifaces: map[string]int{}, imports: map[string]string{}}
	sort.Strings(files)
	fset := token.NewFileSet()
	var parsed, parsedTests []*ast.File
	for _, file := range testFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		af, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		parsedTests = append(parsedTests, af)
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		af, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		p.files++
		p.lines += strings.Count(string(src), "\n")
		parsed = append(parsed, af)
		base := filepath.Base(file)
		for _, imp := range af.Imports {
			ip, _ := strconv.Unquote(imp.Path.Value)
			if _, ok := p.imports[ip]; !ok {
				p.imports[ip] = base
			}
		}
		for _, d := range af.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil {
					p.methods[receiverType(d.Recv)]++
				} else if d.Name.Name == "init" {
					p.inits = append(p.inits, base)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok {
						continue
					}
					switch tt := ts.Type.(type) {
					case *ast.StructType:
						p.fields[ts.Name.Name] = countFields(tt.Fields)
					case *ast.InterfaceType:
						p.ifaces[ts.Name.Name] = countFields(tt.Methods)
					}
				}
			}
		}
	}
	p.globals = assignedGlobals(parsed, parsedTests)
	return p, nil
}

// receiverType is the bare type name of a method receiver: *T, T, T[K].
func receiverType(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return "?"
	}
	e := recv.List[0].Type
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.Ident:
			return x.Name
		default:
			return "?"
		}
	}
}

// countFields counts names in a field list; an embedded field or
// interface counts one.
func countFields(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	n := 0
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			n++
		} else {
			n += len(f.Names)
		}
	}
	return n
}

// assignedGlobals lists the package-level vars declared in files that
// some function body (in files or tests) assigns to, increments, or
// indexes into for assignment. Exempt: `_`, sync/atomic values, and a
// `var (...)` group that holds one (the mutex or Once guarding a cache
// is declared beside it: SOLID_GO.md §9). A name the function declares
// itself (parameter, :=, var, range) shadows the global and does not
// count.
func assignedGlobals(files, tests []*ast.File) []string {
	globals := map[string]bool{}
	for _, af := range files {
		for _, d := range af.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			guarded := false
			for _, s := range gd.Specs {
				if syncValue(s.(*ast.ValueSpec)) {
					guarded = true
				}
			}
			if guarded {
				continue
			}
			for _, s := range gd.Specs {
				for _, n := range s.(*ast.ValueSpec).Names {
					if n.Name != "_" {
						globals[n.Name] = true
					}
				}
			}
		}
	}
	if len(globals) == 0 {
		return nil
	}
	hit := map[string]bool{}
	for _, af := range append(append([]*ast.File(nil), files...), tests...) {
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			local := localNames(fd)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.AssignStmt:
					if s.Tok == token.DEFINE {
						return true
					}
					for _, l := range s.Lhs {
						if name := rootIdent(l); name != "" && globals[name] && !local[name] {
							hit[name] = true
						}
					}
				case *ast.IncDecStmt:
					if name := rootIdent(s.X); name != "" && globals[name] && !local[name] {
						hit[name] = true
					}
				}
				return true
			})
		}
	}
	var out []string
	for name := range hit {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// syncValue reports a var whose type or initializer comes from sync or
// atomic (a mutex, a Once*, a Pool, an atomic counter), or an anonymous
// struct holding one: state those packages make safe to share.
func syncValue(vs *ast.ValueSpec) bool {
	if from(vs.Type) {
		return true
	}
	for _, v := range vs.Values {
		if from(v) {
			return true
		}
	}
	return false
}

func from(e ast.Expr) bool {
	for e != nil {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.CallExpr:
			e = x.Fun
		case *ast.CompositeLit:
			e = x.Type
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.StructType:
			for _, f := range x.Fields.List {
				if from(f.Type) {
					return true
				}
			}
			return false
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			return ok && (id.Name == "sync" || id.Name == "atomic")
		default:
			return false
		}
	}
	return false
}

// rootIdent is the identifier an assignment target starts from: x in
// x, x.f, x[i], *x.
func rootIdent(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x.Name
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		default:
			return ""
		}
	}
}

// localNames are the identifiers a function declares: parameters,
// results, receiver, := targets, var declarations and range variables.
func localNames(fd *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	add := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				names[n.Name] = true
			}
		}
	}
	add(fd.Recv)
	add(fd.Type.Params)
	add(fd.Type.Results)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if s.Tok == token.DEFINE {
				for _, l := range s.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						names[id.Name] = true
					}
				}
			}
		case *ast.RangeStmt:
			if s.Tok == token.DEFINE {
				for _, e := range []ast.Expr{s.Key, s.Value} {
					if id, ok := e.(*ast.Ident); ok {
						names[id.Name] = true
					}
				}
			}
		case *ast.DeclStmt:
			if gd, ok := s.Decl.(*ast.GenDecl); ok {
				for _, sp := range gd.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok {
						for _, id := range vs.Names {
							names[id.Name] = true
						}
					}
				}
			}
		case *ast.FuncLit:
			add(s.Type.Params)
			add(s.Type.Results)
		}
		return true
	})
	return names
}
