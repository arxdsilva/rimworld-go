package lang

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"testing"

	"rimworld-go/pkg/msgfmt"
)

// The simulation's messages and letters reach the HUD as msgfmt.Text
// (lang.Msg): the catalog keys their formats, the arguments by exact
// key, and the titles by exact key. simKeys finds those keys in the game
// tree so the English template holds them.

// What a poster's argument is to the catalog.
const (
	roleFormat = iota + 1 // a printf format: keyed as msgfmt.Key(format)
	roleTitle             // a letter or threat title: keyed as is
	rolePlain             // finished letter text: keyed as msgfmt.Plain(text)
	roleArg               // a format argument: the HUD translates it by exact key
)

// poster is a function or method that posts a message or letter: the
// role of each argument it keys, and from where its variadic format
// arguments start (-1: none).
type poster struct {
	at       map[int]int
	argsFrom int
}

// seedPosters are the game's own constructors (climate.go, world_state.go;
// game is the root package's import path). Hosts and helpers that forward a
// parameter to one of them are found by discoverPosters, so a sub-package's
// h.Note("…") is keyed too.
func seedPosters(game string) map[string]poster {
	return map[string]poster{
		funcKey(game, "letterf"): {at: map[int]int{2: roleTitle, 4: roleFormat}, argsFrom: 5},
		"method:message/*":       {at: map[int]int{2: roleFormat}, argsFrom: 3},
		"method:threat/*":        {at: map[int]int{0: roleTitle, 3: roleFormat}, argsFrom: 4},
		"method:worldNote/*":     {at: map[int]int{0: roleFormat}, argsFrom: 1},
		methodKey("letter", 4):   {at: map[int]int{1: roleTitle, 2: rolePlain}, argsFrom: -1},
	}
}

// callTargets are the keys call may resolve to, exact first.
func callTargets(f *srcFile, call *ast.CallExpr) []string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return []string{funcKey(f.path, fn.Name)}
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			if ip, ok := f.imports[x.Name]; ok {
				return []string{funcKey(ip, fn.Sel.Name)}
			}
		}
		return []string{methodKey(fn.Sel.Name, len(call.Args)), "method:" + fn.Sel.Name + "/*"}
	}
	return nil
}

func posterFor(f *srcFile, call *ast.CallExpr, posters map[string]poster) (poster, bool) {
	for _, k := range callTargets(f, call) {
		if p, ok := posters[k]; ok {
			return p, true
		}
	}
	return poster{}, false
}

// roleOf is the role of call's argument i under p (0: none).
func (p poster) roleOf(i int) int {
	if r, ok := p.at[i]; ok {
		return r
	}
	if p.argsFrom >= 0 && i >= p.argsFrom {
		return roleArg
	}
	return 0
}

// discoverPosters adds every game function that forwards one of its
// parameters to a poster's keyed argument, until nothing new turns up.
func discoverPosters(files []*srcFile, posters map[string]poster) {
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, d := range f.ast.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil && addForwarder(f, fd, posters) {
					changed = true
				}
			}
		}
	}
}

func addForwarder(f *srcFile, fd *ast.FuncDecl, posters map[string]poster) bool {
	params, strParams, n, variadic := map[string]int{}, map[int]bool{}, 0, false
	for _, fld := range fd.Type.Params.List {
		_, variadic = fld.Type.(*ast.Ellipsis)
		id, isStr := fld.Type.(*ast.Ident)
		for _, nm := range fld.Names {
			params[nm.Name] = n
			strParams[n] = isStr && id.Name == "string"
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
	cur, had := posters[key]
	if !had {
		cur = poster{at: map[int]int{}, argsFrom: -1}
	}
	added := false
	ast.Inspect(fd.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		p, ok := posterFor(f, call, posters)
		if !ok {
			return true
		}
		for i, a := range call.Args {
			role := p.roleOf(i)
			id, isID := a.(*ast.Ident)
			if role == 0 || !isID {
				continue
			}
			j, ok := params[id.Name]
			if !ok {
				continue
			}
			if variadic && j == n-1 && call.Ellipsis.IsValid() {
				// First wins: same-named hosts with other layouts must not flip it.
				if cur.argsFrom < 0 {
					cur.argsFrom, added = j, true
				}
				continue
			}
			if role != roleArg && !strParams[j] {
				continue // an id or number passed on is no text
			}
			if _, has := cur.at[j]; !has {
				cur.at[j], added = role, true
			}
		}
		return true
	})
	if added {
		posters[key] = cur
	}
	return added
}

// ifacePosters are the posting methods each package's consumer-owned
// interfaces declare (raid's Threat(title, x, y, format, args...), maps'
// Letter(caution, title, focus, format, args...)): pkg -> name -> poster.
// A call in that package is read by its own interface's signature, so
// same-named hosts with different layouts never mix. Parameters named
// format or f (a string) are formats, title a title, text finished text;
// a variadic after the format holds its arguments.
func ifacePosters(files []*srcFile) map[string]map[string]poster {
	out := map[string]map[string]poster{}
	for _, f := range files {
		ast.Inspect(f.ast, func(n ast.Node) bool {
			it, ok := n.(*ast.InterfaceType)
			if !ok {
				return true
			}
			for _, m := range it.Methods.List {
				ft, ok := m.Type.(*ast.FuncType)
				if !ok || len(m.Names) != 1 {
					continue
				}
				if p, ok := sigPoster(ft); ok {
					if out[f.pkg] == nil {
						out[f.pkg] = map[string]poster{}
					}
					out[f.pkg][m.Names[0].Name] = p
				}
			}
			return true
		})
	}
	return out
}

// sigPoster reads a method signature's roles by parameter name; ok is
// false when it posts nothing (no format, title or text).
func sigPoster(ft *ast.FuncType) (poster, bool) {
	p := poster{at: map[int]int{}, argsFrom: -1}
	i, format := 0, false
	for _, fld := range ft.Params.List {
		id, isStr := fld.Type.(*ast.Ident)
		isStr = isStr && id.Name == "string"
		_, variadic := fld.Type.(*ast.Ellipsis)
		for range max(1, len(fld.Names)) {
			name := ""
			if len(fld.Names) > 0 {
				name = fld.Names[i-countBefore(ft, fld)].Name
			}
			switch {
			case variadic && format:
				p.argsFrom = i
			case isStr && (name == "format" || name == "f"):
				p.at[i], format = roleFormat, true
			case isStr && name == "title":
				p.at[i] = roleTitle
			case isStr && name == "text":
				p.at[i] = rolePlain
			}
			i++
		}
	}
	return p, len(p.at) > 0
}

// countBefore is how many parameters come before fld in ft.
func countBefore(ft *ast.FuncType, fld *ast.Field) int {
	n := 0
	for _, x := range ft.Params.List {
		if x == fld {
			return n
		}
		n += max(1, len(x.Names))
	}
	return n
}

// simKeys is every catalog key the game tree posts: literal (or constant,
// or +-joined) formats, titles, plain letter texts and literal arguments of
// the posters; msgfmt.Of/Plain calls and MessageView/LetterView titles
// written directly; and game/sense's Why* refusal reasons (gear tab tips).
// built lists the posting sites whose format the scan cannot read, and
// finished those that post finished text ("%s" with a printed argument,
// or a letter's text): both reach the catalog as msgfmt.Plain(text) at
// run time, a key no template holds, so they stay English.
func simKeys(files []*srcFile, game string) (keys map[string]bool, built, finished []string) {
	posters := seedPosters(game)
	discoverPosters(files, posters)
	byPkg := ifacePosters(files)
	consts := stringConsts(files)
	keys = map[string]bool{}
	for _, f := range files {
		var fd *ast.FuncDecl // the function being scanned (nil: package level)
		// lits are the values e can hold; ok is false when one of them is
		// built at run time. A local variable holds every literal it is
		// assigned in the function (cause, how := …, "%s froze"; how = …).
		var lits func(e ast.Expr, depth int) ([]string, bool)
		lits = func(e ast.Expr, depth int) ([]string, bool) {
			switch x := e.(type) {
			case *ast.BasicLit:
				s, ok := strLit(x)
				return []string{s}, ok
			case *ast.ParenExpr:
				return lits(x.X, depth)
			case *ast.BinaryExpr:
				if x.Op == token.ADD {
					l, ok1 := lits(x.X, depth)
					r, ok2 := lits(x.Y, depth)
					if len(l) == 1 && len(r) == 1 {
						return []string{l[0] + r[0]}, ok1 && ok2
					}
				}
			case *ast.Ident:
				if s, ok := consts[f.pkg][x.Name]; ok {
					return []string{s}, true
				}
				if fd == nil || depth > 2 {
					return nil, false
				}
				vals := localValues(fd, x.Name)
				out, ok := []string{}, len(vals) > 0
				for _, v := range vals {
					l, vok := lits(v, depth+1)
					out, ok = append(out, l...), ok && vok
				}
				return out, ok
			case *ast.SelectorExpr:
				if pk, ok := x.X.(*ast.Ident); ok {
					if ip, ok := f.imports[pk.Name]; ok && strings.HasPrefix(ip, game+"/") {
						s, ok := consts[strings.TrimPrefix(ip, game+"/")][x.Sel.Name]
						return []string{s}, ok
					}
				}
			}
			return nil, false
		}
		add := func(role int, e ast.Expr) {
			vals, ok := lits(e, 0)
			at := f.fset.Position(e.Pos()).String()
			switch {
			case ok && role == roleFormat && slices.Equal(vals, []string{"%s"}):
				finished = append(finished, at)
			case ok || isParam(fd, e):
			case role == rolePlain, role == roleFormat && isPlainCall(f, e):
				finished = append(finished, at)
			case role == roleFormat:
				built = append(built, at)
			}
			for _, s := range vals {
				ok := true
				switch role {
				case roleFormat:
					s, ok = msgfmt.Key(s)
				case rolePlain:
					s = msgfmt.Plain(s)
				}
				if ok && hasLetter(s) {
					keys[s] = true
				}
			}
		}
		visit := func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if isMsgfmt(f, x, "Of") && len(x.Args) > 0 {
					add(roleFormat, x.Args[0])
					for _, a := range x.Args[1:] {
						add(roleArg, a)
					}
				} else if isMsgfmt(f, x, "Plain") && len(x.Args) == 1 {
					add(rolePlain, x.Args[0])
				} else if p, ok := callPoster(f, x, byPkg, posters); ok {
					for i, a := range x.Args {
						if r := p.roleOf(i); r != 0 {
							add(r, a)
						}
					}
				}
			case *ast.CompositeLit:
				if isView(x.Type) {
					for _, el := range x.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Title" {
								add(roleTitle, kv.Value)
							}
						}
					}
				}
			}
			return true
		}
		for _, d := range f.ast.Decls {
			fd, _ = d.(*ast.FuncDecl)
			ast.Inspect(d, visit)
		}
	}
	for name, v := range consts["sense"] {
		if strings.HasPrefix(name, "Why") && hasLetter(v) {
			keys[v] = true
		}
	}
	return keys, built, finished
}

// isPlainCall reports whether e is msgfmt.Plain(…): finished text.
func isPlainCall(f *srcFile, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && isMsgfmt(f, call, "Plain")
}

// localValues are the expressions fd assigns to the variable name (by
// name only: a shadowed namesake adds its values too, which only adds keys).
func localValues(fd *ast.FuncDecl, name string) []ast.Expr {
	var out []ast.Expr
	if fd.Body == nil {
		return nil
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		var lhs, rhs []ast.Expr
		switch x := n.(type) {
		case *ast.AssignStmt:
			lhs, rhs = x.Lhs, x.Rhs
		case *ast.ValueSpec:
			for _, id := range x.Names {
				lhs = append(lhs, id)
			}
			rhs = x.Values
		default:
			return true
		}
		if len(lhs) != len(rhs) {
			return true
		}
		for i, l := range lhs {
			if id, ok := l.(*ast.Ident); ok && id.Name == name {
				out = append(out, rhs[i])
			}
		}
		return true
	})
	return out
}

// isParam reports whether e is one of fd's parameters: the function is a
// forwarder, and its callers' formats are scanned instead.
func isParam(fd *ast.FuncDecl, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	if !ok || fd == nil {
		return false
	}
	for _, fld := range fd.Type.Params.List {
		for _, n := range fld.Names {
			if n.Name == id.Name {
				return true
			}
		}
	}
	return false
}

// callPoster is the poster call resolves to: a method its own package's
// interface declares first, else a game constructor or forwarder.
func callPoster(f *srcFile, call *ast.CallExpr, byPkg map[string]map[string]poster, posters map[string]poster) (poster, bool) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if x, isPkg := sel.X.(*ast.Ident); !isPkg || f.imports[x.Name] == "" {
			if p, ok := byPkg[f.pkg][sel.Sel.Name]; ok {
				return p, true
			}
		}
	}
	return posterFor(f, call, posters)
}

func isMsgfmt(f *srcFile, call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && strings.HasSuffix(f.imports[x.Name], "/pkg/msgfmt")
}

// isView reports whether a composite literal builds a MessageView or a
// LetterView (game.MessageView from outside the package).
func isView(e ast.Expr) bool {
	name := ""
	switch x := e.(type) {
	case *ast.Ident:
		name = x.Name
	case *ast.SelectorExpr:
		name = x.Sel.Name
	}
	return name == "MessageView" || name == "LetterView"
}

// TestSimKeysSeeEveryForm: the posting shapes simKeys reads.
func TestSimKeysSeeEveryForm(t *testing.T) {
	src := `package x
import (
	"fmt"
	"rimworld-go/pkg/msgfmt"
)
const fmtTamed = "%s tamed a %s."
func (g *Game) message(x, y int, format string, args ...any) {}
func (g *Game) threat(title string, x, y int, format string, args ...any) {}
func (g *Game) worldNote(format string, args ...any) { g.message(0, 0, format, args...) }
func (g *Game) letter(kind, title, text string, focus int) {}
func letterf(g *Game, kind, title string, focus int, format string, args ...any) {}
type host struct{ g *Game }
func (h host) Note(text string) { h.g.worldNote("%s", text) }
func (h host) Letter(danger bool, title, text string, focus int) { h.g.letter("info", title, text, focus) }
func (h host) Threat(title string, x, y int, text string) { h.g.threat(title, x, y, "%s", text) }
func f(g *Game, h host, name string) {
	g.message(1, 2, "%s ate %d meals (%.1f%%).", name, 3, 4.5)
	g.message(1, 2, fmtTamed, name, "boar")
	g.message(1, 2, "%s", name)
	g.message(1, 2, dynamic, name)
	g.worldNote("The caravan has left.")
	g.threat("Raid", 1, 2, "Raiders from %s.", name)
	g.letter("good", "Wanderers", "Two wanderers joined: 100% sure.", 0)
	g.letter("good", "Built", fmt.Sprintf("%s finished.", name), 0)
	h.Note("Nobody can join the caravan.")
	h.Letter(false, "Caravan arrived", text, 0)
	h.Threat("Manhunters", 1, 2, "Animals went mad.")
	g.post(MessageView{Title: "Direct", Msg: msgfmt.Of("Direct %d post", 2)})
	_ = msgfmt.Make(msgfmt.Plain("Plain 5%"), nil)
	letterf(g, "good", "Inspired", 0, "%s is inspired: %s.", name, "creativity")
}`
	f := parseOne(t, src)
	f.imports["msgfmt"] = "rimworld-go/pkg/msgfmt"
	got, _, _ := simKeys([]*srcFile{f}, f.path)
	want := []string{"%s ate %s meals (%s%%).", "%s tamed a %s.", "boar", "The caravan has left.", "Raid", "Raiders from %s.",
		"Wanderers", "Two wanderers joined: 100%% sure.", "Built", "Nobody can join the caravan.", "Caravan arrived",
		"Manhunters", "Animals went mad.", "Direct", "Direct %s post", "Plain 5%%",
		"Inspired", "%s is inspired: %s.", "creativity"}
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Fatalf("keys\n%q\nwant\n%q", keys, want)
	}
}

// TestSimKeysReadHostSignatures checks sub-package posts: each package's
// own interface decides the roles (two Letter layouts never mix),
// constants resolve in the package or through its import, and formats
// joined with + fold into one key.
func TestSimKeysReadHostSignatures(t *testing.T) {
	const game = "example.com/g"
	sub := func(pkg, src string) *srcFile {
		f := parseOne(t, src)
		f.pkg, f.path, f.rel = pkg, game+"/"+pkg, pkg+"/x.go"
		return f
	}
	orders := sub("orders", `package orders
const ReasonDefenders = "%s holds the line."`)
	raid := sub("raid", `package raid
import "example.com/g/orders"
const caveMessage = "A cave opened near %s."
type Host interface {
	Message(x, y int, format string, args ...any)
	Threat(title string, x, y int, format string, args ...any)
	Letter(title string, x, y int, danger bool, format string, args ...any)
}
func f(h Host, name string, cold bool) {
	built := strings.Repeat(name, 2)
	how := "%s froze."
	if !cold {
		how = "%s burned."
	}
	h.Message(1, 2, how, name)
	h.Message(1, 2, "%s", built)
	h.Message(1, 2, caveMessage, name)
	h.Message(1, 2, orders.ReasonDefenders, name)
	h.Threat("Siege", 1, 2, "Sappers dig toward "+"%s.", name)
	h.Letter("Ship part", 1, 2, true, "It crashed near %s.", "the hills")
	h.Message(1, 2, built)
}`)
	visit := sub("visit", `package visit
type Host interface {
	Letter(kind, title string, focus int, format string, args ...any)
}
func f(h Host) { h.Letter("good", "Visitors", 0, "%d traders arrived.", 3) }`)
	got, built, finished := simKeys([]*srcFile{orders, raid, visit}, game)
	want := []string{"%s froze.", "%s burned.", "A cave opened near %s.", "%s holds the line.", "Siege", "Sappers dig toward %s.",
		"Ship part", "It crashed near %s.", "the hills", "Visitors", "%s traders arrived."}
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Fatalf("keys\n%q\nwant\n%q", keys, want)
	}
	if len(built) != 1 || len(finished) != 1 {
		t.Fatalf("built %q, finished %q: want one of each", built, finished)
	}
}
