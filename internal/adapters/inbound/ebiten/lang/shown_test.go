package lang

import (
	"go/ast"
	"go/token"
)

// How a literal reaches the screen.
const (
	viaCall   = iota // a sink's argument
	viaField         // a Label/Tip/… field of a composite literal
	viaVar           // a local assigned a literal, then drawn
	viaReturn        // returned by a function whose result is drawn
	viaKinds
)

// rawText is one untranslated literal headed for the screen.
type rawText struct {
	file string // slash path below the ebiten dir
	pkg  string
	line int
	text string
	via  int
}

// collector walks one function at a time; it remembers the literals
// assigned to each local and the callees whose results are drawn.
type collector struct {
	f       *srcFile
	sinks   map[string]sink
	locals  map[string][]ast.Expr
	pkgVals map[string][]ast.Expr // package-level consts and vars
	sources map[string]bool
	seen    map[litPos]bool
	out     *[]rawText
}

// litPos names a literal across files (each file has its own FileSet).
type litPos struct {
	file string
	pos  token.Pos
}

// emit records every shown literal in e once.
func (c *collector) emit(e ast.Expr, via int) {
	for _, bl := range c.literals(e, via, map[string]bool{}) {
		at := litPos{c.f.rel, bl.lit.Pos()}
		if c.seen[at] {
			continue
		}
		c.seen[at] = true
		if s, _ := strLit(bl.lit); hasLetter(s) {
			*c.out = append(*c.out, rawText{c.f.rel, c.f.pkg, c.f.fset.Position(bl.lit.Pos()).Line, s, bl.via})
		}
	}
}

type viaLit struct {
	lit *ast.BasicLit
	via int
}

// literals lists the shown string literals in e: a literal, either side
// of a +, fmt.Sprintf's arguments, the elements of a []string{…}, what a
// local was assigned. A T/Tf call is translated and yields nothing; any
// other call's returns are counted when its callee is walked (sources).
func (c *collector) literals(e ast.Expr, via int, inLocal map[string]bool) []viaLit {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			return []viaLit{{x, via}}
		}
	case *ast.ParenExpr:
		return c.literals(x.X, via, inLocal)
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return append(c.literals(x.X, via, inLocal), c.literals(x.Y, via, inLocal)...)
		}
	case *ast.Ident:
		if inLocal[x.Name] {
			return nil
		}
		inLocal[x.Name] = true
		vals, ok := c.locals[x.Name]
		if !ok {
			vals = c.pkgVals[x.Name]
		}
		var out []viaLit
		for _, v := range vals {
			out = append(out, c.literals(v, viaVar, inLocal)...)
		}
		return out
	case *ast.CallExpr:
		if isTranslate(x) {
			return nil
		}
		if c.isSprintf(x) {
			var out []viaLit
			for _, a := range x.Args {
				out = append(out, c.literals(a, via, inLocal)...)
			}
			return out
		}
		if k := c.calleeKey(x); k != "" {
			c.sources[k] = true
		}
	case *ast.CompositeLit:
		var out []viaLit
		for _, el := range x.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok && textFields[k.Name] {
					continue // the field rule counts it
				}
				el = kv.Value
			}
			out = append(out, c.literals(el, via, inLocal)...)
		}
		return out
	}
	return nil
}

func (c *collector) isSprintf(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && c.f.imports[id.Name] == "fmt"
}

// calleeKey is the sink-style key of call's function ("" for a call
// through an expression the guard cannot name).
func (c *collector) calleeKey(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return funcKey(c.f.path, fn.Name)
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			if ip, ok := c.f.imports[x.Name]; ok {
				return funcKey(ip, fn.Sel.Name)
			}
		}
		return methodKey(fn.Sel.Name, len(call.Args))
	}
	return ""
}

// declKey is fd's key in the same form as calleeKey.
func declKey(f *srcFile, fd *ast.FuncDecl) string {
	if fd.Recv == nil {
		return funcKey(f.path, fd.Name.Name)
	}
	n := 0
	for _, fld := range fd.Type.Params.List {
		n += max(1, len(fld.Names))
	}
	return methodKey(fd.Name.Name, n)
}

// localsOf maps each name assigned in body to the expressions it was
// given (a := "x"; a = "y"; var a = "z").
func localsOf(body ast.Node) map[string][]ast.Expr {
	out := map[string][]ast.Expr{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) != len(s.Rhs) {
				return true
			}
			for i, l := range s.Lhs {
				if id, ok := l.(*ast.Ident); ok {
					out[id.Name] = append(out[id.Name], s.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if i < len(s.Values) {
					out[id.Name] = append(out[id.Name], s.Values[i])
				}
			}
		}
		return true
	})
	return out
}

// untranslated walks the client for shown literals not wrapped in T/Tf.
func untranslated(files []*srcFile, sinks map[string]sink) []rawText {
	var out []rawText
	sources, seen := map[string]bool{}, map[litPos]bool{}
	pkgVals := map[string]map[string][]ast.Expr{}
	for _, f := range files {
		if pkgVals[f.pkg] == nil {
			pkgVals[f.pkg] = map[string][]ast.Expr{}
		}
		for _, d := range f.ast.Decls {
			if gd, ok := d.(*ast.GenDecl); ok {
				for name, v := range localsOf(gd) {
					pkgVals[f.pkg][name] = append(pkgVals[f.pkg][name], v...)
				}
			}
		}
	}
	var funcs []func() bool // the returns pass, one per FuncDecl
	for _, f := range files {
		if f.pkg == "lang" {
			continue
		}
		c := &collector{f: f, sinks: sinks, pkgVals: pkgVals[f.pkg], sources: sources, seen: seen, out: &out}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if kv, ok := n.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok && textFields[k.Name] {
					c.emit(kv.Value, viaField)
				}
			}
			return true
		})
		for _, d := range f.ast.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			fc := *c
			fc.locals = localsOf(fd.Body)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if s, ok := sinkFor(f, call, sinks); ok {
						for _, e := range s.shown(call) {
							fc.emit(e, viaCall)
						}
					}
				}
				return true
			})
			key, done := declKey(f, fd), false
			funcs = append(funcs, func() bool {
				if done || !sources[key] {
					return false
				}
				done = true
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					switch r := n.(type) {
					case *ast.FuncLit:
						return false // its returns are the closure's
					case *ast.ReturnStmt:
						if len(r.Results) == 1 {
							fc.emit(r.Results[0], viaReturn)
						}
					}
					return true
				})
				return true
			})
		}
	}
	// A drawn function may return another's result: walk to a fixpoint.
	for more := true; more; {
		more = false
		for _, fn := range funcs {
			if fn() {
				more = true
			}
		}
	}
	return out
}

// allLiterals counts, per package, every string literal with a letter
// outside imports and struct tags: the upper bound the guard's
// shown-text count is read against.
func allLiterals(files []*srcFile) map[string]int {
	out := map[string]int{}
	for _, f := range files {
		skip := map[*ast.BasicLit]bool{}
		for _, im := range f.ast.Imports {
			skip[im.Path] = true
		}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Field:
				if x.Tag != nil {
					skip[x.Tag] = true
				}
			case *ast.BasicLit:
				if s, ok := strLit(x); ok && !skip[x] && hasLetter(s) {
					out[f.pkg]++
				}
			}
			return true
		})
	}
	return out
}
