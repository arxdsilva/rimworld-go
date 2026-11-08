package arch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// The cmd guard (SOLID_GO.md §8a, HEXAGONAL_ARCHITECTURE.md §6). Every
// cmd/<pkg> is one wiring-only main.go: it parses its arguments through a
// package, builds the adapters and application types, hands them over as
// interfaces, calls one Run and exits. Logic lives in packages that follow
// the SOLID limits (internal/adapters/inbound/cli/<tool>,
// internal/compose/<tool>, …).
//
// The rules, per cmd/<pkg>:
//   - the directory holds exactly one entry, main.go (no other files,
//     tests, sub-directories or non-Go files);
//   - main.go declares only func main (imports aside);
//   - it has at most cmdMaxLines lines (cmdLineCap names the exceptions);
//   - its body has no for, range, switch, select or go statements, no
//     function literals, and no string literal over cmdMaxString runes.
//
// cmdTODO lists the packages that still break a rule. It only shrinks: a
// listed package that now passes fails until its line is deleted, and an
// unlisted package that breaks a rule fails.

const (
	cmdMaxLines  = 80
	cmdMaxString = 40
)

// cmdLineCap raises the line cap for a package, with its reason.
var cmdLineCap = map[string]int{
	"game": 120, // the game wires the most adapters: window, audio, saves, content, mods, probes.
}

// cmdTODO: packages not yet cut to wiring. Delete a line in the commit that
// moves its package; never add one.
var cmdTODO = map[string]bool{
	"game": true,
}

func TestCmdIsWiringOnly(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	found, err := cmdViolations(filepath.Join(root, "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range judgeCmd(found, cmdTODO) {
		t.Error(bad)
	}
}

// Only cmd/ and the composition root itself import internal/compose.
func TestOnlyCmdImportsCompose(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	composeRoot := module + "/internal/compose"
	for _, p := range scanTree(t, root) {
		if strings.HasPrefix(p.path, module+"/cmd/") || exactOrSub(composeRoot)(p.path) {
			continue
		}
		for imp, file := range p.imports {
			if exactOrSub(composeRoot)(imp) {
				t.Errorf("%s imports %s (%s): only cmd/ builds the composition root", p.path, imp, file)
			}
		}
	}
}

// The judge only lets the TODO list shrink.
func TestCmdTODOOnlyShrinks(t *testing.T) {
	t.Parallel()
	todo := map[string]bool{"game": true, "tool": true}
	found := map[string][]string{"game": {"main.go: 300 lines"}}

	bad := judgeCmd(found, todo)
	if len(bad) != 1 || !strings.Contains(bad[0], "cmd/tool is wiring only now") {
		t.Fatalf("a listed package that passes must be delisted, got %v", bad)
	}
	bad = judgeCmd(map[string][]string{"game": {"x"}, "tool": {"x"}, "new": {"main.go: has func helper"}}, todo)
	if len(bad) != 1 || !strings.Contains(bad[0], "cmd/new") || !strings.Contains(bad[0], "func helper") {
		t.Fatalf("a new unlisted violation must fail with its reason, got %v", bad)
	}
	if bad := judgeCmd(map[string][]string{"game": {"x"}, "tool": {"x"}}, todo); len(bad) != 0 {
		t.Fatalf("a tree that matches the list must pass, got %v", bad)
	}
}

// The scanner names each rule it sees broken.
func TestCmdViolationsSeesEachRule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ok/main.go", "package main\n\nimport (\n\t\"os\"\n\n\t\"rimworld-go/internal/adapters/inbound/cli/x\"\n)\n\nfunc main() {\n\tos.Exit(cli.Run(os.Args[1:]))\n}\n")
	write("extra/main.go", "package main\n\nfunc main() {}\n")
	write("extra/flags.go", "package main\n")
	write("sub/main.go", "package main\n\nfunc main() {}\n")
	write("sub/render/r.go", "package render\n")
	write("helper/main.go", "package main\n\nvar x = 1\n\nfunc main() { help() }\n\nfunc help() {}\n")
	write("loops/main.go", "package main\n\nfunc main() {\n\tfor range 3 {\n\t}\n\tswitch {\n\t}\n\tgo func() {}()\n\t_ = \"a string literal that is far longer than forty runes\"\n}\n")
	write("long/main.go", "package main\n\nfunc main() {\n"+strings.Repeat("\tprintln()\n", cmdMaxLines)+"}\n")

	found, err := cmdViolations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found["ok"]) != 0 {
		t.Errorf("a wiring-only main must pass, got %v", found["ok"])
	}
	want := map[string][]string{
		"extra":  {"flags.go"},
		"sub":    {"render/"},
		"helper": {"func help", "var declaration"},
		"loops":  {"for/range", "switch", "go statement", "function literal", "string literal"},
		"long":   {"lines"},
	}
	for pkg, needles := range want {
		got := strings.Join(found[pkg], "; ")
		for _, n := range needles {
			if !strings.Contains(got, n) {
				t.Errorf("cmd/%s: want %q in %q", pkg, n, got)
			}
		}
	}
}

// cmdViolations returns, per package directory under dir, the rules it
// breaks.
func cmdViolations(dir string) (map[string][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	found := map[string][]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		reasons, err := cmdPackage(filepath.Join(dir, e.Name()), e.Name())
		if err != nil {
			return nil, err
		}
		if len(reasons) > 0 {
			found[e.Name()] = reasons
		}
	}
	return found, nil
}

func cmdPackage(dir, name string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var reasons []string
	hasMain := false
	for _, e := range entries {
		switch {
		case e.IsDir():
			reasons = append(reasons, "holds "+e.Name()+"/ (only main.go belongs in cmd/<pkg>)")
		case e.Name() == "main.go":
			hasMain = true
		default:
			reasons = append(reasons, "holds "+e.Name()+" (only main.go belongs in cmd/<pkg>)")
		}
	}
	if !hasMain {
		return append(reasons, "has no main.go"), nil
	}
	main, err := mainRules(filepath.Join(dir, "main.go"), name)
	if err != nil {
		return nil, err
	}
	return append(reasons, main...), nil
}

func mainRules(path, name string) ([]string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reasons []string
	limit := cmdMaxLines
	if n, ok := cmdLineCap[name]; ok {
		limit = n
	}
	if n := strings.Count(string(src), "\n"); n > limit {
		reasons = append(reasons, fmt.Sprintf("main.go: %d lines (limit %d)", n, limit))
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil || d.Name.Name != "main" {
				reasons = append(reasons, "main.go: has func "+d.Name.Name+" (only func main)")
			}
		case *ast.GenDecl:
			if d.Tok != token.IMPORT {
				reasons = append(reasons, "main.go: has a "+d.Tok.String()+" declaration (only func main)")
			}
		}
	}
	seen := map[string]bool{}
	note := func(pos token.Pos, what string) {
		if !seen[what] {
			seen[what] = true
			reasons = append(reasons, fmt.Sprintf("main.go:%d: %s (logic belongs in a package)", fset.Position(pos).Line, what))
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ImportSpec:
			return false // import paths are not string literals the rule means
		case *ast.ForStmt, *ast.RangeStmt:
			note(n.Pos(), "a for/range statement")
		case *ast.SwitchStmt, *ast.TypeSwitchStmt:
			note(n.Pos(), "a switch statement")
		case *ast.SelectStmt:
			note(n.Pos(), "a select statement")
		case *ast.GoStmt:
			note(n.Pos(), "a go statement")
		case *ast.FuncLit:
			note(n.Pos(), "a function literal")
		case *ast.BasicLit:
			if n.Kind == token.STRING && utf8.RuneCountInString(n.Value)-2 > cmdMaxString {
				note(n.Pos(), fmt.Sprintf("a string literal over %d runes", cmdMaxString))
			}
		}
		return true
	})
	return reasons, nil
}

// judgeCmd compares what the scan found with the TODO list.
func judgeCmd(found map[string][]string, todo map[string]bool) []string {
	var bad []string
	for pkg, reasons := range found {
		if !todo[pkg] {
			bad = append(bad, fmt.Sprintf("cmd/%s is not wiring only: %s", pkg, strings.Join(reasons, "; ")))
		}
	}
	for pkg := range todo {
		if _, ok := found[pkg]; !ok {
			bad = append(bad, fmt.Sprintf("cmd/%s is wiring only now: delete its cmdTODO line in internal/arch/cmd_test.go", pkg))
		}
	}
	sort.Strings(bad)
	return bad
}
