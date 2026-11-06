package lang

import (
	"fmt"
	"go/ast"
	"slices"
)

// sink is a function or method whose listed arguments end up on screen.
// from >= 0 marks a variadic tail: every argument from that index on.
type sink struct {
	args []int
	from int
}

// Sink keys: "func:<import path>.<Name>" for a package function and
// "method:<Name>/<arity>" for a method on any receiver ("/*" variadic);
// a method is matched by name and arity because the guard reads syntax
// only.
func funcKey(pkg, name string) string         { return "func:" + pkg + "." + name }
func methodKey(name string, arity int) string { return fmt.Sprintf("method:%s/%d", name, arity) }

// seedSinks are the text-drawing entry points of gfx and uikit. Helpers
// that pass a string parameter on to one of them are found by
// discoverSinks, so a feature's own drawText(dst, s, …) counts too.
func seedSinks(client string) map[string]sink {
	gfx, ui := client+"/gfx", client+"/uikit"
	s := map[string]sink{}
	add := func(key string, args ...int) { s[key] = sink{args: args, from: -1} }
	for name, args := range map[string][]int{
		"Text": {1}, "WorldText": {1}, "Ellipsize": {0}, "WrapPx": {0}, "Measure": {0}, "TextW": {0},
	} {
		add(funcKey(gfx, name), args...)
	}
	for name, args := range map[string][]int{
		"DrawCentered": {1}, "DrawLabelClip": {4}, "DrawFitSmall": {1}, "DrawLabel": {3},
		"DebugPrintAt": {1}, "DrawListRow": {2}, "DrawProgress": {2, 4}, "AsButton": {2},
		"StyledButton": {3}, "RowLabel": {2}, "TextLines": {4}, "Choice": {3, 4, 6},
		"Picker": {2, 3}, "Segmented": {3, 5}, "TabRects": {1}, "FitText": {0}, "TipLayout": {0},
	} {
		add(funcKey(ui, name), args...)
	}
	// uikit.Kit, uikit.Form and ports.Hinter methods.
	add(methodKey("Button", 4), 2)
	add(methodKey("Label", 4), 2)
	add(methodKey("Check", 5), 2)
	add(methodKey("Radio", 5), 2)
	add(methodKey("Tip", 3), 2)
	add(methodKey("Confirm", 6), 1, 2, 3)
	add(methodKey("Tabs", 5), 2)
	add(methodKey("Deny", 1), 0)
	add(methodKey("Hint", 1), 0)
	add(methodKey("SectionTitle", 2), 1)
	add(methodKey("Slider", 8), 2, 6, 7)
	add(methodKey("Check", 6), 2, 4, 5)
	add(methodKey("Stepper", 7), 2, 6)
	return s
}

// textFields are composite-literal keys whose string values are shown
// (button specs, options, table columns, tooltips).
var textFields = map[string]bool{
	"Label": true, "Labels": true, "Title": true, "Tip": true, "Tips": true, "Reason": true,
	"Body": true, "Text": true, "Hint": true, "Desc": true, "Caption": true, "Disabled": true,
}

// sinkFor resolves call to its sink key, trying the variadic method key
// after the exact one.
func sinkFor(f *srcFile, call *ast.CallExpr, sinks map[string]sink) (sink, bool) {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		s, ok := sinks[funcKey(f.path, fn.Name)]
		return s, ok
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			if ip, ok := f.imports[x.Name]; ok {
				s, ok := sinks[funcKey(ip, fn.Sel.Name)]
				return s, ok
			}
		}
		if s, ok := sinks[methodKey(fn.Sel.Name, len(call.Args))]; ok {
			return s, true
		}
		s, ok := sinks["method:"+fn.Sel.Name+"/*"]
		return s, ok
	}
	return sink{}, false
}

// shown lists the argument expressions of call that s draws.
func (s sink) shown(call *ast.CallExpr) []ast.Expr {
	var out []ast.Expr
	for _, i := range s.args {
		if i < len(call.Args) {
			out = append(out, call.Args[i])
		}
	}
	if s.from >= 0 {
		for i := s.from; i < len(call.Args); i++ {
			out = append(out, call.Args[i])
		}
	}
	return out
}

// discoverSinks adds every client function that passes one of its
// string parameters straight to a sink, until nothing new turns up.
func discoverSinks(files []*srcFile, sinks map[string]sink) {
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, d := range f.ast.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if ok && fd.Body != nil && addWrapper(f, fd, sinks) {
					changed = true
				}
			}
		}
	}
}

// addWrapper makes fd a sink for each parameter it forwards to one;
// it reports whether it added anything.
func addWrapper(f *srcFile, fd *ast.FuncDecl, sinks map[string]sink) bool {
	params, n, variadic := map[string]int{}, 0, false
	for _, fld := range fd.Type.Params.List {
		_, variadic = fld.Type.(*ast.Ellipsis)
		for _, nm := range fld.Names {
			params[nm.Name] = n
			n++
		}
		if len(fld.Names) == 0 {
			n++
		}
	}
	if len(params) == 0 {
		return false
	}
	key := funcKey(f.path, fd.Name.Name)
	if fd.Recv != nil {
		key = methodKey(fd.Name.Name, n)
		if variadic {
			key = "method:" + fd.Name.Name + "/*"
		}
	}
	cur, had := sinks[key]
	if !had {
		cur.from = -1
	}
	added := false
	ast.Inspect(fd.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		s, ok := sinkFor(f, call, sinks)
		if !ok {
			return true
		}
		for _, e := range s.shown(call) {
			id, ok := e.(*ast.Ident)
			if !ok {
				continue
			}
			i, ok := params[id.Name]
			if !ok || slices.Contains(cur.args, i) || (cur.from >= 0 && i >= cur.from) {
				continue
			}
			if variadic && i == n-1 {
				cur.from = i
			} else {
				cur.args = append(cur.args, i)
			}
			added = true
		}
		return true
	})
	if added {
		sinks[key] = cur
	}
	return added
}
