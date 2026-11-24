// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// Every declared table column is drawn (QA-002): the machinery of the
// guard that every column site of the ebiten tree has a column test. A
// column site is a non-test declaration that
//   (a) is a func or method whose results include []uikit.TableCell (bare
//       []TableCell inside package uikit), a nested func literal included;
//   (b) holds a uikit.TableCol composite literal ([]uikit.TableCol{…},
//       [N]uikit.TableCol{…}, uikit.TableCol{…}); or
//   (c) calls X.Table(...) with 6 arguments (uikit.Kit.Table);
// keyed "<rel path>:<Func|Recv.Func|var name>" by its top-level declaration.
// A column test is a Test func, registered for a site, whose own body calls
// uikit.ColumnScreens and uikit.ColumnGaps or uikit.TableColumnGaps.

// columnSites are the column sites of files (slash paths relative to root,
// from packageFiles), sorted and unique.
func columnSites(t testing.TB, root string, files []string) []string {
	t.Helper()
	src := make(map[string]string, len(files))
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		src[rel] = string(b)
	}
	return columnSitesIn(src)
}

// columnSitesIn are the column sites of the sources (rel path -> source),
// sorted and unique; _test.go files are skipped and a file that does not
// parse is listed as "<rel>: parse error: …" so it cannot hide a site.
func columnSitesIn(src map[string]string) []string {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for rel, s := range src {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, rel, s, 0)
		if err != nil {
			seen[rel+": parse error: "+err.Error()] = true
			continue
		}
		inUikit := f.Name.Name == "uikit"
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if columnSiteIn(d, inUikit) {
					seen[rel+":"+columnFuncKey(d)] = true
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok {
						for _, name := range columnVarSites(vs, inUikit) {
							seen[rel+":"+name] = true
						}
					}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// columnVarSites are the names of vs whose values hold a column site.
func columnVarSites(vs *ast.ValueSpec, inUikit bool) []string {
	var out []string
	for i, v := range vs.Values {
		if !columnSiteIn(v, inUikit) {
			continue
		}
		name := vs.Names[0].Name // var a, b = f(): the values are the first name's
		if len(vs.Values) == len(vs.Names) {
			name = vs.Names[i].Name
		}
		out = append(out, name)
	}
	return out
}

// columnSiteIn reports a column site anywhere in n.
func columnSiteIn(n ast.Node, inUikit bool) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			found = found || columnCellResults(n.Type, inUikit)
		case *ast.FuncLit:
			found = found || columnCellResults(n.Type, inUikit)
		case *ast.CompositeLit:
			found = found || columnTableColType(n.Type)
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			found = found || ok && sel.Sel.Name == "Table" && len(n.Args) == 6
		}
		return !found
	})
	return found
}

// columnCellResults reports a result of type []uikit.TableCell.
func columnCellResults(ft *ast.FuncType, inUikit bool) bool {
	if ft == nil || ft.Results == nil {
		return false
	}
	for _, r := range ft.Results.List {
		if at, ok := r.Type.(*ast.ArrayType); ok && at.Len == nil && columnUikitType(at.Elt, "TableCell", inUikit) {
			return true
		}
	}
	return false
}

// columnTableColType reports uikit.TableCol, []uikit.TableCol or
// [N]uikit.TableCol (any length, [...] too).
func columnTableColType(e ast.Expr) bool {
	if at, ok := e.(*ast.ArrayType); ok {
		e = at.Elt
	}
	return columnUikitType(e, "TableCol", false)
}

// columnUikitType reports e is uikit.<name> (or a bare <name> inside uikit).
func columnUikitType(e ast.Expr, name string, inUikit bool) bool {
	switch e := e.(type) {
	case *ast.SelectorExpr:
		x, ok := e.X.(*ast.Ident)
		return ok && x.Name == "uikit" && e.Sel.Name == name
	case *ast.Ident:
		return inUikit && e.Name == name
	}
	return false
}

// columnFuncKey is "Func" or "Recv.Func".
func columnFuncKey(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	e := d.Recv.List[0].Type
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
			continue
		case *ast.IndexExpr:
			e = x.X
			continue
		case *ast.IndexListExpr:
			e = x.X
			continue
		case *ast.Ident:
			return x.Name + "." + d.Name.Name
		}
		return d.Name.Name
	}
}

// columnGuardProblems are the guard's findings, sorted: a site with no
// column test and no TODO, a registry or TODO entry whose site is gone, a
// TODO entry that also has a test or has no reason, and a registered test
// that check (nil: skipped) rejects.
func columnGuardProblems(sites []string, tests, todo map[string]string, check func(ref string) string) []string {
	var out []string
	live := make(map[string]bool, len(sites))
	for _, s := range sites {
		live[s] = true
		if tests[s] == "" && todo[s] == "" {
			out = append(out, s+": no column test (add a column test and register it in columnTests, or list it in columnTestTODO with a reason)")
		}
	}
	for site, ref := range tests {
		if !live[site] {
			out = append(out, site+": columnTests entry for a site that is gone (remove it)")
		}
		if check == nil {
			continue
		}
		if p := check(ref); p != "" {
			out = append(out, site+": column test "+ref+": "+p)
		}
	}
	for site, why := range todo {
		switch {
		case !live[site]:
			out = append(out, site+": columnTestTODO entry for a site that is gone (remove it)")
		case tests[site] != "":
			out = append(out, site+": columnTestTODO entry has a column test now (remove it from columnTestTODO)")
		case strings.TrimSpace(why) == "":
			out = append(out, site+": columnTestTODO entry needs a reason")
		}
	}
	sort.Strings(out)
	return out
}

// columnTestRefProblem checks ref ("<rel>/<file>_test.go:TestName",
// relative to root): the file parses and the test's own body calls the
// column checkers (columnTestRefProblemIn). "" when it does.
func columnTestRefProblem(root, ref string) string {
	i := strings.LastIndex(ref, ":")
	if i < 0 || i == len(ref)-1 {
		return "a column test ref is \"<rel>/<file>_test.go:TestName\""
	}
	file, name := ref[:i], ref[i+1:]
	if !strings.HasSuffix(file, "_test.go") {
		return file + " is not a _test.go file"
	}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return "cannot read " + file + ": " + err.Error()
	}
	f, err := parser.ParseFile(token.NewFileSet(), file, src, 0)
	if err != nil {
		return "cannot parse " + file + ": " + err.Error()
	}
	return columnTestRefProblemIn(f, name)
}

// columnTestRefProblemIn checks that file has func name(t *testing.T) whose
// own body (nested closures such as t.Run included, other funcs of the
// file not) calls ColumnScreens and ColumnGaps or TableColumnGaps.
func columnTestRefProblemIn(file *ast.File, name string) string {
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if d, ok := d.(*ast.FuncDecl); ok && d.Recv == nil && d.Name.Name == name {
			fn = d
		}
	}
	if fn == nil {
		return name + " is not in the file"
	}
	if !strings.HasPrefix(name, "Test") || !columnTakesT(fn.Type) || fn.Body == nil {
		return name + " is not a func " + name + "(t *testing.T)"
	}
	var calls []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, calleeName(c))
		}
		return true
	})
	screens := slices.Contains(calls, "ColumnScreens")
	gaps := slices.Contains(calls, "ColumnGaps") || slices.Contains(calls, "TableColumnGaps")
	switch {
	case !screens && !gaps:
		return name + " calls neither ColumnScreens nor ColumnGaps/TableColumnGaps in its own body"
	case !screens:
		return name + " does not call ColumnScreens in its own body"
	case !gaps:
		return name + " does not call ColumnGaps or TableColumnGaps in its own body"
	}
	return ""
}

// columnTakesT reports the signature (t *testing.T).
func columnTakesT(ft *ast.FuncType) bool {
	if ft.Params == nil || len(ft.Params.List) != 1 || len(ft.Params.List[0].Names) > 1 {
		return false
	}
	star, ok := ft.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "testing" && sel.Sel.Name == "T"
}

// The scan finds each site kind, keyed by its top-level declaration, and
// skips non-sites and test files.
func TestColumnGuardFindsSites(t *testing.T) {
	t.Parallel()
	src := map[string]string{
		"windows/a.go": `package windows

import "rimworld-go/internal/adapters/inbound/ebiten/uikit"

func aCells(dst []uikit.TableCell) []uikit.TableCell { return dst }

func (w *Animals) cells() (n int, c []uikit.TableCell) { return }

func (g Gen[T]) genCells() []uikit.TableCell { return nil }

var scenarioCols = []uikit.TableCol{{ID: "a"}}

var one, two = 1, [2]uikit.TableCol{}

func factions() { _ = uikit.TableCol{ID: "x"} }

func arr() { _ = [...]uikit.TableCol{{}} }

func (s *Screen) draw(k *uikit.Kit) { k.Table("id", r, cols, n, &sel, 0) }

func (s Screen) drawVia(a *App) { a.uiSt().Table("id", r, cols, n, &sel, 0) }

func nested() {
	f := func() []uikit.TableCell { return nil }
	_ = f
}

// not sites
func notSite(c []uikit.TableCell) []string { return nil }
func list(k *uikit.Kit) { k.Table("a", b, c) }
func callback(f func() []uikit.TableCell) {}
func ptrs() []*uikit.TableCell { return nil }
func one2() uikit.TableCell { return uikit.TableCell{} }
var declared []uikit.TableCol
type holder struct{ f func() []uikit.TableCell }
`,
		"windows/bare.go": "package windows\n\nfunc bare() []TableCell { return nil }\n",
		"uikit/t.go":      "package uikit\n\nfunc TableCells(dst []TableCell) []TableCell { return dst }\n",
		"windows/a_test.go": `package windows

func testCells() []uikit.TableCell { return nil }
`,
	}
	got := columnSitesIn(src)
	want := []string{
		"uikit/t.go:TableCells",
		"windows/a.go:Animals.cells",
		"windows/a.go:Gen.genCells",
		"windows/a.go:Screen.draw",
		"windows/a.go:Screen.drawVia",
		"windows/a.go:aCells",
		"windows/a.go:arr",
		"windows/a.go:factions",
		"windows/a.go:nested",
		"windows/a.go:scenarioCols",
		"windows/a.go:two",
	}
	if !slices.Equal(got, want) {
		t.Errorf("columnSitesIn =\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
	bad := columnSitesIn(map[string]string{"windows/bad.go": "package windows\nfunc ("})
	if len(bad) != 1 || !strings.HasPrefix(bad[0], "windows/bad.go: parse error") {
		t.Errorf("a file that does not parse = %q, want one parse error entry", bad)
	}
}

// AC-4's failing proof: a site left unregistered is named, with the stale
// registry and TODO entries, a TODO without a reason and a rejected ref.
func TestColumnGuardFlagsMissingTest(t *testing.T) {
	t.Parallel()
	sites := []string{"w/a.go:aCells", "w/b.go:bCells", "w/c.go:cCells", "w/d.go:dCells", "w/e.go:eCells"}
	tests := map[string]string{
		"w/a.go:aCells":  "w/a_test.go:TestA",
		"w/c.go:cCells":  "w/c_test.go:TestC",
		"w/d.go:dCells":  "w/d_test.go:TestBad",
		"w/gone.go:gone": "w/gone_test.go:TestGone",
	}
	todo := map[string]string{
		"w/c.go:cCells": "drawn by the lane",
		"w/old.go:old":  "a site that went away",
		"w/e.go:eCells": " ",
	}
	check := func(ref string) string {
		if ref == "w/d_test.go:TestBad" {
			return "TestBad does not call ColumnScreens in its own body"
		}
		return ""
	}
	got := columnGuardProblems(sites, tests, todo, check)
	want := []string{
		"w/b.go:bCells: no column test (add a column test and register it in columnTests, or list it in columnTestTODO with a reason)",
		"w/c.go:cCells: columnTestTODO entry has a column test now (remove it from columnTestTODO)",
		"w/d.go:dCells: column test w/d_test.go:TestBad: TestBad does not call ColumnScreens in its own body",
		"w/e.go:eCells: columnTestTODO entry needs a reason",
		"w/gone.go:gone: columnTests entry for a site that is gone (remove it)",
		"w/old.go:old: columnTestTODO entry for a site that is gone (remove it)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("columnGuardProblems =\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
	ok := columnGuardProblems(sites[:1], map[string]string{"w/a.go:aCells": "w/a_test.go:TestA"}, nil, check)
	if ok != nil {
		t.Errorf("a fully registered tree has problems: %q", ok)
	}
}

// A registered test must call the checkers itself: a sibling test of the
// same file that does is not enough (plan-review B4).
func TestColumnTestRefNeedsTheCheckInItsBody(t *testing.T) {
	t.Parallel()
	const src = `package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

func TestA(t *testing.T) {
	for _, s := range uikit.ColumnScreens() {
		t.Run("x", func(t *testing.T) { _ = uikit.ColumnGaps(win, ids, nil, cells(s)) })
	}
}

func TestB(t *testing.T) { t.Log("checks nothing") }

func TestOnlyGaps(t *testing.T) { _ = uikit.ColumnGaps(win, ids, nil, nil) }

func TestOnlyScreens(t *testing.T) { _ = uikit.ColumnScreens() }

func TestTable(t *testing.T) {
	for range uikit.ColumnScreens() {
		_ = uikit.TableColumnGaps(win, r, cols, 0)
	}
}

func helperChecks(t *testing.T) { _, _ = uikit.ColumnScreens(), uikit.ColumnGaps(win, ids, nil, nil) }

func TestTwoArgs(t *testing.T, x int) { _, _ = uikit.ColumnScreens(), uikit.ColumnGaps(win, ids, nil, nil) }

func (s suite) TestM(t *testing.T) { _, _ = uikit.ColumnScreens(), uikit.ColumnGaps(win, ids, nil, nil) }
`
	f, err := parser.ParseFile(token.NewFileSet(), "x_test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"TestA":           "",
		"TestTable":       "",
		"TestB":           "TestB calls neither ColumnScreens nor ColumnGaps/TableColumnGaps in its own body",
		"TestOnlyGaps":    "TestOnlyGaps does not call ColumnScreens in its own body",
		"TestOnlyScreens": "TestOnlyScreens does not call ColumnGaps or TableColumnGaps in its own body",
		"helperChecks":    "helperChecks is not a func helperChecks(t *testing.T)",
		"TestTwoArgs":     "TestTwoArgs is not a func TestTwoArgs(t *testing.T)",
		"TestM":           "TestM is not in the file",
		"TestMissing":     "TestMissing is not in the file",
	} {
		if got := columnTestRefProblemIn(f, name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}

// The scan and the ref check run on the real tree: every file parses, the
// sites are sorted and unique, and the uikit demo table's test passes.
func TestColumnGuardScansTheTree(t *testing.T) {
	t.Parallel()
	sites := columnSites(t, ".", packageFiles(t, "."))
	if !slices.Contains(sites, "uikit/table.go:TableCells") {
		t.Errorf("the scan misses uikit/table.go:TableCells: %q", sites)
	}
	for i, s := range sites {
		if strings.Contains(s, ": parse error") {
			t.Errorf("%s", s)
		}
		if i > 0 && sites[i-1] >= s {
			t.Errorf("sites not sorted and unique at %q, %q", sites[i-1], s)
		}
	}
	if p := columnTestRefProblem(".", "uikit/columns_test.go:TestTableCellsDrawEveryColumn"); p != "" {
		t.Errorf("the uikit column test is rejected: %s", p)
	}
	for ref, want := range map[string]string{
		"uikit/columns_test.go:TestColumnScreens": "TestColumnScreens does not call ColumnGaps or TableColumnGaps in its own body",
		"uikit/columns.go:ColumnGaps":             "uikit/columns.go is not a _test.go file",
		"uikit/columns_test.go":                   "a column test ref is \"<rel>/<file>_test.go:TestName\"",
		"uikit/columns_test.go:":                  "a column test ref is \"<rel>/<file>_test.go:TestName\"",
	} {
		if got := columnTestRefProblem(".", ref); got != want {
			t.Errorf("%s: %q, want %q", ref, got, want)
		}
	}
	if got := columnTestRefProblem(".", "uikit/no_such_test.go:TestX"); !strings.HasPrefix(got, "cannot read uikit/no_such_test.go") {
		t.Errorf("a missing file: %q", got)
	}
}

// columnTests registers the column test of every column site of the tree
// (QA-002). Several sites of one window share a test. A map literal, never
// filled at run time.
func columnTests() map[string]string {
	return map[string]string{
		"dialogs/trade.go:tradeCells":                              "dialogs/trade_columns_test.go:TestTradeColumnsDrawn",
		"dialogs/trade.go:tradeColumns":                            "dialogs/trade_columns_test.go:TestTradeColumnsDrawn",
		"inspector/bio_ui.go:bioColumns":                           "inspector/bio_columns_test.go:TestBioSkillColumnsDrawn",
		"inspector/bio_ui.go:bioSkillCells":                        "inspector/bio_columns_test.go:TestBioSkillColumnsDrawn",
		"options/saves/dialog.go:dialogCols":                       "options/saves/dialog_columns_test.go:TestSaveListColumnsDrawn",
		"options/saves/dialog.go:drawDialog":                       "options/saves/dialog_columns_test.go:TestSaveListColumnsDrawn",
		"uikit/table.go:TableCells":                                "uikit/columns_test.go:TestTableCellsDrawEveryColumn",
		"windows/animals.go:colonyCells":                           "windows/animals_columns_test.go:TestColonyColumnsDrawn",
		"windows/animals.go:colonyCellsFor":                        "windows/animals_columns_test.go:TestColonyColumnsDrawn",
		"windows/animals.go:colonyColumns":                         "windows/animals_columns_test.go:TestColonyColumnsDrawn",
		"windows/apparel_policy.go:assignCells":                    "windows/assign_columns_test.go:TestAssignColumnsDrawn",
		"windows/apparel_policy.go:assignColumns":                  "windows/assign_columns_test.go:TestAssignColumnsDrawn",
		"windows/autoslaughter.go:autoCells":                       "windows/autoslaughter_columns_test.go:TestAutoSlaughterColumnsDrawn",
		"windows/autoslaughter.go:autoColumns":                     "windows/autoslaughter_columns_test.go:TestAutoSlaughterColumnsDrawn",
		"windows/records.go:recordCells":                           "windows/records_columns_test.go:TestRecordColumnsDrawn",
		"windows/records.go:recordColumns":                         "windows/records_columns_test.go:TestRecordColumnsDrawn",
		"windows/work.go:workCells":                                "windows/work_columns_test.go:TestWorkColumnsDrawn",
		"wizard/screen_factions.go:landingPage.factionColumns":     "wizard/tables_columns_test.go:TestFactionColumnsDrawn",
		"wizard/screen_factions.go:landingPage.factionsTable":      "wizard/tables_columns_test.go:TestFactionColumnsDrawn",
		"wizard/screen_pawns.go:pawnSkillCells":                    "wizard/pawns_columns_test.go:TestPawnSkillColumnsDrawn",
		"wizard/screen_pawns.go:pawnSkillColumns":                  "wizard/pawns_columns_test.go:TestPawnSkillColumnsDrawn",
		"wizard/screen_scenario.go:scenarioFileColumns":            "wizard/tables_columns_test.go:TestScenarioFileColumnsDrawn",
		"wizard/screen_scenario.go:scenarioPage.openScenarioFiles": "wizard/tables_columns_test.go:TestScenarioFileColumnsDrawn",
		"wizard/screen_scenario_editor.go:editorPage.editorLoad":   "wizard/tables_columns_test.go:TestScenarioFileColumnsDrawn",
	}
}

// columnTestTODO lists sites that cannot be tested yet, each with its reason
// and owning issue. It is expected to stay empty.
func columnTestTODO() map[string]string {
	return map[string]string{}
}

// Every column site of the tree has a column test whose own body checks that
// every declared column is drawn at both ColumnScreens (QA-002, AC-4).
func TestEveryTableWindowHasAColumnTest(t *testing.T) {
	t.Parallel()
	sites := columnSites(t, ".", packageFiles(t, "."))
	check := func(ref string) string { return columnTestRefProblem(".", ref) }
	if p := columnGuardProblems(sites, columnTests(), columnTestTODO(), check); len(p) > 0 {
		t.Errorf("column guard problems:\n\t%s", strings.Join(p, "\n\t"))
	}
}

// The AC-4 failing proof on the live tree: dropping any one registration
// makes the guard fail and name exactly that site.
func TestColumnGuardFailsWithoutARegistration(t *testing.T) {
	t.Parallel()
	sites := columnSites(t, ".", packageFiles(t, "."))
	full := columnTests()
	if len(full) == 0 {
		t.Fatal("columnTests is empty")
	}
	for site := range full {
		less := make(map[string]string, len(full))
		for k, v := range full {
			if k != site {
				less[k] = v
			}
		}
		got := columnGuardProblems(sites, less, columnTestTODO(), nil)
		if len(got) != 1 || !strings.HasPrefix(got[0], site+": no column test") {
			t.Errorf("without %s: problems = %q, want exactly that site named", site, got)
		}
	}
}
